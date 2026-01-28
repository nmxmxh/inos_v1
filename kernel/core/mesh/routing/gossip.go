package routing

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
	"github.com/yasserelgammal/rate-limiter/limiter"
	"github.com/yasserelgammal/rate-limiter/store"
)

// GossipManager handles epidemic propagation with anti-entropy and rate limiting
type GossipManager struct {
	nodeID    string
	publicKey ed25519.PublicKey
	signKey   ed25519.PrivateKey

	// Local state - Merkle tree for anti-entropy
	state        *MerkleTree
	stateMu      sync.RWMutex
	stateVersion uint64

	// Message tracking
	messages   map[string]*common.GossipMessage // Local messages we've created
	messagesMu sync.RWMutex

	// Peer connections
	peers   []string
	peersMu sync.RWMutex

	// Deduplication with Bloom filter and TTL-based cleanup
	seenFilter     *bloom.BloomFilter
	seenTimestamps map[string]time.Time
	seenMu         sync.RWMutex
	seenTTL        time.Duration

	// Transport
	transport common.Transport

	// Rate limiting (Token Bucket)
	limiter      *limiter.TokenBucket
	limiterStore store.Store
	rateMu       sync.RWMutex

	// Message queue for backpressure
	messageQueue chan QueuedGossipMessage
	queueSize    int

	// Handlers
	handlers   map[string]GossipHandler
	handlersMu sync.RWMutex

	// Metrics
	metrics        GossipMetrics
	metricsMu      sync.RWMutex
	latencyHistory []time.Duration

	// Configuration
	config GossipConfig

	// Lifecycle
	shutdown chan struct{}
	running  atomic.Bool
	logger   *slog.Logger

	// Merkle sync state
	syncState map[string]*MerkleSyncState
	syncMu    sync.RWMutex
}

// GossipConfig holds gossip configuration
type GossipConfig struct {
	Fanout              int           `json:"fanout"`                // Number of peers to gossip to each round
	PushFactor          int           `json:"push_factor"`           // Push messages to this many peers
	PullFactor          int           `json:"pull_factor"`           // Pull from this many peers each round
	RoundInterval       time.Duration `json:"round_interval"`        // Time between gossip rounds
	AntiEntropyInterval time.Duration `json:"anti_entropy_interval"` // Time between anti-entropy syncs
	MessageTTL          time.Duration `json:"message_ttl"`           // Time messages stay in seen cache
	MaxHops             int           `json:"max_hops"`              // Maximum propagation hops
	MaxMessageSize      int           `json:"max_message_size"`      // Maximum message size in bytes
	QueueSize           int           `json:"queue_size"`            // Size of message queue
	RateLimit           struct {
		MessagesPerSecond float64 `json:"messages_per_second"`
		BurstSize         int     `json:"burst_size"`
	} `json:"rate_limit"`
	BloomFilter struct {
		ExpectedElements  uint    `json:"expected_elements"`
		FalsePositiveRate float64 `json:"false_positive_rate"`
	} `json:"bloom_filter"`
}

// DefaultGossipConfig returns production-ready defaults
func DefaultGossipConfig() GossipConfig {
	config := GossipConfig{
		Fanout:              3,
		PushFactor:          2,
		PullFactor:          1,
		RoundInterval:       1 * time.Second,
		AntiEntropyInterval: 30 * time.Second,
		MessageTTL:          1 * time.Hour,
		MaxHops:             10,
		MaxMessageSize:      10 * 1024 * 1024, // 10MB
		QueueSize:           1000,
	}

	config.RateLimit.MessagesPerSecond = 100.0
	config.RateLimit.BurstSize = 1000

	config.BloomFilter.ExpectedElements = 100000
	config.BloomFilter.FalsePositiveRate = 0.01

	return config
}

// GossipMetrics tracks gossip performance
type GossipMetrics struct {
	MessagesSent          uint64    `json:"messages_sent"`
	MessagesReceived      uint64    `json:"messages_received"`
	MessagesDropped       uint64    `json:"messages_dropped"`
	DuplicateMessages     uint64    `json:"duplicate_messages"`
	PropagationLatencyP50 float64   `json:"propagation_latency_p50_ms"`
	PropagationLatencyP95 float64   `json:"propagation_latency_p95_ms"`
	QueueLength           uint32    `json:"queue_length"`
	StateSize             uint64    `json:"state_size"`
	SyncOperations        uint64    `json:"sync_operations"`
	FailedSignatures      uint64    `json:"failed_signatures"`
	RateLimited           uint64    `json:"rate_limited"`
	StartTime             time.Time `json:"start_time"`
}

// QueuedGossipMessage represents a message in the gossip queue
type QueuedGossipMessage struct {
	Message   *common.GossipMessage
	Targets   []string
	Priority  int
	Timestamp time.Time
	Result    chan error
}

// MerkleTree implements Merkle tree for anti-entropy with fixed buckets for stability
type MerkleTree struct {
	Root    []byte
	Buckets [256][]string // Message IDs sorted in each bucket
	Layers  [][][]byte    // Layers[0] = 256 bucket hashes, Layers[h] = parents
}

// MerkleLeaf represents a leaf in the Merkle tree
type MerkleLeaf struct {
	Key   string
	Value []byte
	Hash  []byte
}

// MerkleSyncState tracks anti-entropy sync with a peer
type MerkleSyncState struct {
	PeerID     string
	TheirRoot  []byte
	SyncHeight int
	LastSync   time.Time
	InProgress bool
}

// Merkle depth limit to prevent stack overflow attacks
const MaxMerkleSyncDepth = 32

func getShortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// GossipHandler processes gossip messages
type GossipHandler func(*common.GossipMessage) error

// NewGossipManager creates a production-ready gossip manager
func NewGossipManager(nodeID string, transport common.Transport, logger *slog.Logger) (*GossipManager, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Generate signing keys
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	config := DefaultGossipConfig()

	// Initialize bloom filter
	bf := bloom.NewWithEstimates(
		config.BloomFilter.ExpectedElements,
		config.BloomFilter.FalsePositiveRate,
	)

	gossip := &GossipManager{
		nodeID:         nodeID,
		publicKey:      publicKey,
		signKey:        privateKey,
		state:          NewMerkleTree(),
		messages:       make(map[string]*common.GossipMessage),
		seenFilter:     bf,
		seenTimestamps: make(map[string]time.Time),
		seenTTL:        config.MessageTTL,
		transport:      transport,
		messageQueue:   make(chan QueuedGossipMessage, config.QueueSize),
		queueSize:      config.QueueSize,
		handlers:       make(map[string]GossipHandler),
		config:         config,
		shutdown:       make(chan struct{}),
		logger:         logger.With("component", "gossip", "node_id", getShortID(nodeID)),
		syncState:      make(map[string]*MerkleSyncState),
	}

	// Initialize rate limiter
	gossip.limiterStore = store.NewMemoryStore(time.Minute)
	gossip.limiter, _ = limiter.NewTokenBucket(
		limiter.Config{
			Rate:     int64(config.RateLimit.MessagesPerSecond),
			Duration: time.Second,
			Burst:    int64(config.RateLimit.BurstSize),
		},
		gossip.limiterStore,
	)

	// Initialize metrics
	gossip.metrics.StartTime = time.Now()

	// Register default handlers
	gossip.registerDefaultHandlers()

	// Register RPC handlers (NEW)
	gossip.registerRPCHandlers()

	return gossip, nil
}

// registerRPCHandlers registers RPC handlers for the transport layer
func (g *GossipManager) registerRPCHandlers() {
	g.transport.RegisterRPCHandler("merkle.root", func(ctx context.Context, peerID string, _ json.RawMessage) (interface{}, error) {
		g.stateMu.RLock()
		defer g.stateMu.RUnlock()
		return g.state.Root, nil
	})

	g.transport.RegisterRPCHandler("merkle.hashes", func(ctx context.Context, peerID string, _ json.RawMessage) (interface{}, error) {
		return g.getAllMessageHashes(), nil
	})

	g.transport.RegisterRPCHandler("merkle.children", func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error) {
		var hashStr string
		if err := json.Unmarshal(args, &hashStr); err != nil {
			return nil, err
		}
		hash, err := base64.StdEncoding.DecodeString(hashStr)
		if err != nil {
			return nil, err
		}

		g.stateMu.RLock()
		defer g.stateMu.RUnlock()
		children := g.state.GetChildren(hash)

		// Encode to base64 for JSON
		encoded := make([]string, len(children))
		for i, c := range children {
			encoded[i] = base64.StdEncoding.EncodeToString(c)
		}
		return encoded, nil
	})

	g.transport.RegisterRPCHandler("merkle.bucket_ids", func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error) {
		var hashStr string
		if err := json.Unmarshal(args, &hashStr); err != nil {
			return nil, err
		}
		hash, err := base64.StdEncoding.DecodeString(hashStr)
		if err != nil {
			return nil, err
		}

		g.stateMu.RLock()
		defer g.stateMu.RUnlock()

		// Find bucket by hash
		for i, h := range g.state.Layers[0] {
			if bytes.Equal(h, hash) {
				return g.state.Buckets[i], nil
			}
		}

		return nil, errors.New("bucket not found")
	})

	g.transport.RegisterRPCHandler("gossip.pull", func(ctx context.Context, peerID string, _ json.RawMessage) (interface{}, error) {
		// Return summary of all messages (IDs)
		return g.getAllMessageIDs(), nil
	})

	g.transport.RegisterRPCHandler("gossip.messages", func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error) {
		var ids []string
		if err := json.Unmarshal(args, &ids); err != nil {
			return nil, err
		}
		return g.getMessagesByIDs(ids), nil
	})

	g.transport.RegisterRPCHandler("gossip.by_hash", func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error) {
		var hashes []string
		if err := json.Unmarshal(args, &hashes); err != nil {
			return nil, err
		}
		return g.getMessagesByHashes(hashes), nil
	})
}

// Start begins the gossip loops
func (g *GossipManager) Start() error {
	if g.running.Load() {
		return errors.New("gossip manager already running")
	}

	g.running.Store(true)
	g.logger.Info("starting gossip manager")

	// Start background workers
	go g.messageProcessor()
	go g.gossipLoop()
	go g.antiEntropyLoop()
	go g.metricsLoop()
	go g.cleanupLoop()
	go g.merkleAnnouncementLoop()

	g.logger.Info("gossip manager started")
	return nil
}

// Stop gracefully shuts down the gossip manager
func (g *GossipManager) Stop() {
	if !g.running.Load() {
		return
	}

	g.logger.Info("stopping gossip manager")
	close(g.shutdown)
	g.running.Store(false)
	g.logger.Info("gossip manager stopped")
}

// RegisterHandler registers a handler for a message type
func (g *GossipManager) RegisterHandler(msgType string, handler GossipHandler) {
	g.handlersMu.Lock()
	g.handlers[msgType] = handler
	g.handlersMu.Unlock()
}

// AnnounceChunk announces a chunk to the network
func (g *GossipManager) AnnounceChunk(chunkHash string) error {
	msg := &common.GossipMessage{
		Type:      "chunk_announce",
		Sender:    g.nodeID,
		Timestamp: time.Now().UnixNano(),
		Payload: map[string]interface{}{
			"chunk_hash": chunkHash,
			"node_id":    g.nodeID,
			"timestamp":  time.Now().Unix(),
		},
		HopCount: 0,
		MaxHops:  g.config.MaxHops,
	}

	// Sign the message
	if err := g.signMessage(msg); err != nil {
		return fmt.Errorf("failed to sign message: %w", err)
	}

	// Store locally
	msgID := g.computeMessageID(msg)
	g.messagesMu.Lock()
	g.messages[msgID] = msg
	g.messagesMu.Unlock()

	// Update Merkle tree
	g.updateStateWithMessage(msgID, msg)

	// Queue for gossip
	return g.queueMessage(msg, nil) // Broadcast to all
}

// AnnouncePeerCapability announces peer capabilities
func (g *GossipManager) AnnouncePeerCapability(capability *common.PeerCapability) error {
	msg := &common.GossipMessage{
		Type:      "peer_capability",
		Sender:    g.nodeID,
		Timestamp: time.Now().UnixNano(),
		Payload:   capability,
		HopCount:  0,
		MaxHops:   g.config.MaxHops,
	}

	if err := g.signMessage(msg); err != nil {
		return fmt.Errorf("failed to sign message: %w", err)
	}

	return g.queueMessage(msg, nil)
}

// ReceiveMessage processes an incoming gossip message
func (g *GossipManager) ReceiveMessage(sender string, msg *common.GossipMessage) error {
	start := time.Now()

	// Update metrics
	g.metricsMu.Lock()
	g.metrics.MessagesReceived++
	g.metricsMu.Unlock()

	// Check rate limiting
	if !g.checkRateLimit(sender) {
		g.metricsMu.Lock()
		g.metrics.RateLimited++
		g.metricsMu.Unlock()
		g.logger.Debug("rate limited", "sender", getShortID(sender))
		return errors.New("rate limited")
	}

	// Check deduplication
	msgID := g.computeMessageID(msg)
	if g.isDuplicate(msgID) {
		g.metricsMu.Lock()
		g.metrics.DuplicateMessages++
		g.metricsMu.Unlock()
		return errors.New("duplicate message")
	}

	// Verify signature
	if err := g.verifyMessage(msg); err != nil {
		g.metricsMu.Lock()
		g.metrics.FailedSignatures++
		g.metricsMu.Unlock()
		g.logger.Warn("failed to verify signature",
			"sender", getShortID(msg.Sender),
			"type", msg.Type,
			"error", err)
		return fmt.Errorf("signature verification failed: %w", err)
	}

	// Check hop count
	if msg.HopCount >= msg.MaxHops {
		g.logger.Debug("message exceeded max hops",
			"sender", getShortID(msg.Sender),
			"hops", msg.HopCount)
		return errors.New("max hops exceeded")
	}

	// Mark as seen
	g.markSeen(msgID)

	// Process message
	if err := g.processMessage(msg); err != nil {
		return fmt.Errorf("failed to process message: %w", err)
	}

	// Update propagation latency (if timestamp is in payload)
	var timestamp int64
	if payload, ok := msg.Payload.(map[string]interface{}); ok {
		if ts, ok := payload["timestamp"].(float64); ok { // JSON unmarshal uses float64
			timestamp = int64(ts)
		} else if ts, ok := payload["timestamp"].(int64); ok {
			timestamp = ts
		}
	} else if _, ok := msg.Payload.([]byte); ok {
		// For binary payloads, timestamp is the message's own timestamp usually
		timestamp = msg.Timestamp
	}

	if timestamp > 0 {
		latency := time.Since(time.Unix(0, timestamp)) // msg.Timestamp uses UnixNano
		g.recordPropagationLatency(latency)
	}

	// Forward if not at max hops
	if msg.HopCount < msg.MaxHops-1 {
		msg.HopCount++
		go g.forwardMessage(msg)
	}

	g.logger.Debug("message processed",
		"sender", getShortID(sender),
		"type", msg.Type,
		"hops", msg.HopCount,
		"latency", time.Since(start))

	return nil
}

// processMessage handles a message based on its type
func (g *GossipManager) processMessage(msg *common.GossipMessage) error {
	g.handlersMu.RLock()
	handler, exists := g.handlers[msg.Type]
	g.handlersMu.RUnlock()

	if !exists {
		// Default handler for unknown types
		g.logger.Debug("no handler for message type", "type", msg.Type)
		return nil
	}

	return handler(msg)
}

// forwardMessage forwards a message to fanout peers
func (g *GossipManager) forwardMessage(msg *common.GossipMessage) {
	// Get random peers to forward to
	peers := g.getRandomPeers(g.config.Fanout)
	if len(peers) == 0 {
		return
	}

	// Queue for sending
	g.queueMessage(msg, peers)
}

// Broadcast propagates a message to the entire network
func (g *GossipManager) Broadcast(topic string, payload interface{}) error {
	msg := &common.GossipMessage{
		ID:        fmt.Sprintf("msg_%d_%d", time.Now().UnixNano(), rand.Uint64()),
		Type:      topic,
		Payload:   payload,
		Sender:    g.nodeID,
		Timestamp: time.Now().UnixNano(),
		TTL:       g.config.MaxHops,
		MaxHops:   g.config.MaxHops,
	}

	// Sign the message using consistent signatureData
	if g.signKey != nil {
		data := g.signatureData(msg)
		sig := ed25519.Sign(g.signKey, data)
		msg.Signature = sig
		msg.PublicKey = g.publicKey
	}

	return g.queueMessage(msg, nil)
}

// queueMessage adds a message to the send queue
func (g *GossipManager) queueMessage(msg *common.GossipMessage, targets []string) error {
	if !g.running.Load() {
		return errors.New("gossip manager not running")
	}

	queued := QueuedGossipMessage{
		Message:   msg,
		Targets:   targets,
		Priority:  g.getMessagePriority(msg.Type),
		Timestamp: time.Now(),
		Result:    make(chan error, 1),
	}

	select {
	case g.messageQueue <- queued:
		// Update queue length metric
		g.metricsMu.Lock()
		g.metrics.QueueLength = uint32(len(g.messageQueue))
		g.metricsMu.Unlock()

		// Wait for result OR shutdown
		select {
		case err := <-queued.Result:
			return err
		case <-g.shutdown:
			return errors.New("gossip manager shutting down")
		}
	default:
		// Queue full, apply backpressure
		g.metricsMu.Lock()
		g.metrics.MessagesDropped++
		g.metricsMu.Unlock()
		return errors.New("gossip queue full")
	}
}

// messageProcessor processes queued messages
func (g *GossipManager) messageProcessor() {
	for {
		select {
		case <-g.shutdown:
			for len(g.messageQueue) > 0 {
				select {
				case queued := <-g.messageQueue:
					if queued.Result != nil {
						queued.Result <- errors.New("gossip manager shutting down")
					}
				default:
					goto done
				}
			}
		done:
			return
		case queued := <-g.messageQueue:
			g.sendMessageToPeers(queued)

			// Update queue length
			g.metricsMu.Lock()
			g.metrics.QueueLength = uint32(len(g.messageQueue))
			g.metricsMu.Unlock()
		}
	}
}

// sendMessageToPeers sends a message to specified peers
func (g *GossipManager) sendMessageToPeers(queued QueuedGossipMessage) {
	var err error

	targets := queued.Targets
	if len(targets) == 0 {
		// No specific targets - select random peers based on fanout
		targets = g.getRandomPeers(g.config.Fanout)
	}

	if len(targets) == 0 {
		// No peers available
		g.logger.Debug("no peers available to send message to")
		if queued.Result != nil {
			queued.Result <- nil
		}
		return
	}

	// Send to each target peer
	var wg sync.WaitGroup
	errs := make(chan error, len(targets))
	successCount := int32(0)

	for _, peer := range targets {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if sendErr := g.transport.SendMessage(ctx, p, queued.Message); sendErr != nil {
				errs <- fmt.Errorf("peer %s: %w", getShortID(p), sendErr)
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}(peer)
	}

	wg.Wait()
	close(errs)

	// Collect errors
	for e := range errs {
		if err == nil {
			err = e
		} else {
			err = fmt.Errorf("%v; %v", err, e)
		}
	}

	// Update metrics - count as sent if at least one peer received it
	if atomic.LoadInt32(&successCount) > 0 {
		g.metricsMu.Lock()
		g.metrics.MessagesSent++
		g.metricsMu.Unlock()
	}

	// Notify sender - return error only if ALL sends failed
	if queued.Result != nil {
		if atomic.LoadInt32(&successCount) > 0 {
			queued.Result <- nil
		} else {
			queued.Result <- err
		}
	}
}

// gossipLoop runs the main gossip protocol
func (g *GossipManager) gossipLoop() {
	ticker := time.NewTicker(g.config.RoundInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.shutdown:
			return
		case <-ticker.C:
			g.gossipRound()
		}
	}
}

// gossipRound performs one round of gossip
func (g *GossipManager) gossipRound() {
	// Push: Send recent messages to random peers
	g.pushGossip()

	// Pull: Request messages from random peers
	g.pullGossip()

	// Cleanup old messages
	g.cleanupOldMessages()
}

// pushGossip pushes recent messages to random peers
func (g *GossipManager) pushGossip() {
	// Get recent messages
	recent := g.getRecentMessages(10) // Last 10 messages
	if len(recent) == 0 {
		return
	}

	// Get random peers
	peers := g.getRandomPeers(g.config.PushFactor)
	if len(peers) == 0 {
		return
	}

	// Send each message to each peer
	for _, msg := range recent {
		for _, peer := range peers {
			go func(p string, m *common.GossipMessage) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				g.transport.SendMessage(ctx, p, m)
			}(peer, msg)
		}
	}
}

// pullGossip pulls messages from random peers
func (g *GossipManager) pullGossip() {
	peers := g.getRandomPeers(g.config.PullFactor)
	if len(peers) == 0 {
		return
	}

	for _, peer := range peers {
		go func(p string) {
			// Request recent message IDs from peer
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var response struct {
				MessageIDs []string `json:"message_ids"`
			}

			if err := g.transport.SendRPC(ctx, p, "gossip.pull", nil, &response); err != nil {
				g.logger.Debug("pull request failed", "peer", getShortID(p), "error", err)
				return
			}

			// Request missing messages
			g.requestMissingMessages(p, response.MessageIDs)
		}(peer)
	}
}

// antiEntropyLoop runs anti-entropy synchronization
func (g *GossipManager) antiEntropyLoop() {
	ticker := time.NewTicker(g.config.AntiEntropyInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.shutdown:
			return
		case <-ticker.C:
			g.performAntiEntropy()
		}
	}
}

// SetFanout updates the gossip fanout parameter
func (g *GossipManager) SetFanout(fanout int) {
	if fanout < 1 {
		fanout = 1
	}
	g.config.Fanout = fanout
	g.logger.Info("updated gossip fanout", "fanout", fanout)
}

// performAntiEntropy performs anti-entropy with a random peer
func (g *GossipManager) performAntiEntropy() {
	// Get random peer
	peers := g.getRandomPeers(1)
	if len(peers) == 0 {
		return
	}
	peer := peers[0]

	// Check if we're already syncing with this peer
	g.syncMu.RLock()
	syncState, exists := g.syncState[peer]
	g.syncMu.RUnlock()

	if exists && syncState.InProgress {
		g.logger.Debug("sync already in progress", "peer", getShortID(peer))
		return
	}

	// Start sync
	g.syncMu.Lock()
	g.syncState[peer] = &MerkleSyncState{
		PeerID:     peer,
		InProgress: true,
		LastSync:   time.Now(),
	}
	g.syncMu.Unlock()

	// Perform Merkle tree sync
	go g.syncWithPeer(peer)
}

// syncWithPeer performs Merkle tree sync with a peer
func (g *GossipManager) syncWithPeer(peerID string) {
	defer func() {
		g.syncMu.Lock()
		if state, exists := g.syncState[peerID]; exists {
			state.InProgress = false
		}
		g.syncMu.Unlock()
	}()

	g.logger.Debug("starting anti-entropy sync", "peer", getShortID(peerID))

	// Exchange Merkle roots
	g.stateMu.RLock()
	ourRoot := g.state.Root
	g.stateMu.RUnlock()

	var theirRoot []byte
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := g.transport.SendRPC(ctx, peerID, "merkle.root", nil, &theirRoot); err != nil {
		g.logger.Debug("failed to get peer root", "peer", getShortID(peerID), "error", err)
		return
	}

	// Compare roots
	if string(ourRoot) == string(theirRoot) {
		g.logger.Debug("states are synchronized", "peer", getShortID(peerID))
		return
	}

	// Roots differ, perform tree reconciliation
	g.reconcileMerkleTrees(peerID, ourRoot, theirRoot)

	g.metricsMu.Lock()
	g.metrics.SyncOperations++
	g.metricsMu.Unlock()

	g.logger.Debug("anti-entropy sync completed", "peer", getShortID(peerID))
}

// reconcileMerkleTrees performs recursive Merkle tree reconciliation
func (g *GossipManager) reconcileMerkleTrees(peerID string, ourRoot, theirRoot []byte) {
	if bytes.Equal(ourRoot, theirRoot) {
		return
	}

	g.logger.Debug("starting recursive merkle reconciliation", "peer", getShortID(peerID))

	// Find differing leaves
	missingHashes, extraHashes, err := g.diffMerkleTreesInteractive(peerID, ourRoot, theirRoot)
	if err != nil {
		g.logger.Debug("recursive reconciliation failed, falling back to full hash exchange", "error", err)
		g.reconcileMerkleTreesSimplified(peerID)
		return
	}

	if len(missingHashes) > 0 {
		g.requestMessagesByHash(peerID, missingHashes)
	}

	if len(extraHashes) > 0 {
		g.sendMessagesByHash(peerID, extraHashes)
	}
}

// reconcileMerkleTreesSimplified is the fallback hash exchange reconciliation
func (g *GossipManager) reconcileMerkleTreesSimplified(peerID string) {
	ourHashes := g.getAllMessageHashes()
	var theirHashes []string
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := g.transport.SendRPC(ctx, peerID, "merkle.hashes", nil, &theirHashes); err != nil {
		g.logger.Debug("failed to get peer hashes", "peer", getShortID(peerID), "error", err)
		return
	}

	// Find differences
	ourSet := make(map[string]bool)
	for _, h := range ourHashes {
		ourSet[h] = true
	}

	theirSet := make(map[string]bool)
	for _, h := range theirHashes {
		theirSet[h] = true
	}

	// Request messages we're missing
	var missing []string
	for h := range theirSet {
		if !ourSet[h] {
			missing = append(missing, h)
		}
	}

	if len(missing) > 0 {
		g.requestMessagesByHash(peerID, missing)
	}

	// Send messages they're missing
	var toSend []string
	for h := range ourSet {
		if !theirSet[h] {
			toSend = append(toSend, h)
		}
	}

	if len(toSend) > 0 {
		g.sendMessagesByHash(peerID, toSend)
	}
}

// diffMerkleTreesInteractive performs the interactive walk to find differences
func (g *GossipManager) diffMerkleTreesInteractive(peerID string, ourRoot, theirRoot []byte) ([]string, []string, error) {
	var missing []string
	var extra []string

	// Snapshot the current Merkle tree state for this reconciliation
	// This prevents race conditions when the tree is modified concurrently
	g.stateMu.RLock()
	stateSnapshot := &MerkleTree{
		Root:    make([]byte, len(g.state.Root)),
		Layers:  make([][][]byte, len(g.state.Layers)),
		Buckets: g.state.Buckets, // Share bucket references (read-only during reconciliation)
	}
	copy(stateSnapshot.Root, g.state.Root)
	for i, layer := range g.state.Layers {
		stateSnapshot.Layers[i] = make([][]byte, len(layer))
		for j, hash := range layer {
			stateSnapshot.Layers[i][j] = make([]byte, len(hash))
			copy(stateSnapshot.Layers[i][j], hash)
		}
	}
	g.stateMu.RUnlock()

	type nodePair struct {
		ourHash   []byte
		theirHash []byte
	}

	queue := []nodePair{{ourHash: ourRoot, theirHash: theirRoot}}
	depth := 0

	for len(queue) > 0 {
		// Defense: prevent Merkle sync depth bombs
		if depth > MaxMerkleSyncDepth {
			g.logger.Warn("merkle sync depth exceeded", "peer", getShortID(peerID))
			return nil, nil, fmt.Errorf("merkle sync depth limit exceeded")
		}
		depth++

		pair := queue[0]
		queue = queue[1:]

		if bytes.Equal(pair.ourHash, pair.theirHash) {
			continue
		}

		// If it's a leaf hash (bucket), we reconcile the bucket's content
		if pair.ourHash != nil && g.isLeafHashFromSnapshot(stateSnapshot, pair.ourHash) {
			// Bucket differs, get message IDs from both sides
			ourBucketIDs := g.getBucketIDsByHashFromSnapshot(stateSnapshot, pair.ourHash)

			var theirBucketIDs []string
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := g.transport.SendRPC(ctx, peerID, "merkle.bucket_ids", base64.StdEncoding.EncodeToString(pair.theirHash), &theirBucketIDs); err != nil {
				g.logger.Debug("failed to fetch bucket IDs", "peer", getShortID(peerID), "error", err)
				cancel()
				continue
			}
			cancel()

			// Find missing/extra within this bucket
			m, e := g.diffBuckets(ourBucketIDs, theirBucketIDs)
			missing = append(missing, m...)
			extra = append(extra, e...)
			continue
		}

		// Request children for this level (use snapshot)
		ourChildren := stateSnapshot.GetChildren(pair.ourHash)

		var theirChildrenEncoded []string

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := g.transport.SendRPC(ctx, peerID, "merkle.children", base64.StdEncoding.EncodeToString(pair.theirHash), &theirChildrenEncoded); err != nil {
			cancel()
			return nil, nil, err
		}
		cancel()

		// Decode their children
		theirChildren := make([][]byte, len(theirChildrenEncoded))
		for i, enc := range theirChildrenEncoded {
			tc, _ := base64.StdEncoding.DecodeString(enc)
			theirChildren[i] = tc
		}

		// Compare children
		maxLen := len(ourChildren)
		if len(theirChildren) > maxLen {
			maxLen = len(theirChildren)
		}

		for i := 0; i < maxLen; i++ {
			var oc, tc []byte
			if i < len(ourChildren) {
				oc = ourChildren[i]
			}
			if i < len(theirChildren) {
				tc = theirChildren[i]
			}

			if !bytes.Equal(oc, tc) {
				queue = append(queue, nodePair{ourHash: oc, theirHash: tc})
			}
		}
	}

	return missing, extra, nil
}

// isLeafHashFromSnapshot checks if hash is a leaf using a snapshot
func (g *GossipManager) isLeafHashFromSnapshot(snapshot *MerkleTree, h []byte) bool {
	if len(snapshot.Layers) == 0 {
		return false
	}
	// Layers[0] contains the bucket hashes
	for _, bh := range snapshot.Layers[0] {
		if bytes.Equal(bh, h) {
			return true
		}
	}
	return false
}

// getBucketIDsByHashFromSnapshot returns message IDs in a bucket by its hash using a snapshot
func (g *GossipManager) getBucketIDsByHashFromSnapshot(snapshot *MerkleTree, h []byte) []string {
	for i, bh := range snapshot.Layers[0] {
		if bytes.Equal(bh, h) {
			return snapshot.Buckets[i]
		}
	}
	return nil
}

// diffBuckets finds differences between two sets of message IDs
func (g *GossipManager) diffBuckets(ourIDs, theirIDs []string) (missing, extra []string) {
	ourSet := make(map[string]bool)
	for _, id := range ourIDs {
		ourSet[id] = true
	}

	theirSet := make(map[string]bool)
	for _, id := range theirIDs {
		theirSet[id] = true
	}

	for id := range theirSet {
		if !ourSet[id] {
			missing = append(missing, id)
		}
	}

	for id := range ourSet {
		if !theirSet[id] {
			extra = append(extra, id)
		}
	}

	return missing, extra
}

// getRecentMessages returns recent messages
func (g *GossipManager) getRecentMessages(count int) []*common.GossipMessage {
	g.messagesMu.RLock()
	defer g.messagesMu.RUnlock()

	messages := make([]*common.GossipMessage, 0, count)
	for _, msg := range g.messages {
		messages = append(messages, msg)
		if len(messages) >= count {
			break
		}
	}

	return messages
}

// getAllMessageHashes returns all message hashes
func (g *GossipManager) getAllMessageHashes() []string {
	g.messagesMu.RLock()
	defer g.messagesMu.RUnlock()

	hashes := make([]string, 0, len(g.messages))
	for _, msg := range g.messages {
		// Use the message ID as the hash for now,
		// Or compute actual BLAKE3 if required
		hashes = append(hashes, msg.ID)
	}

	return hashes
}

// getAllMessageIDs returns all message IDs
func (g *GossipManager) getAllMessageIDs() []string {
	g.messagesMu.RLock()
	defer g.messagesMu.RUnlock()

	ids := make([]string, 0, len(g.messages))
	for id := range g.messages {
		ids = append(ids, id)
	}
	return ids
}

// getMessagesByIDs returns messages for the given IDs
func (g *GossipManager) getMessagesByIDs(ids []string) []*common.GossipMessage {
	g.messagesMu.RLock()
	defer g.messagesMu.RUnlock()

	result := make([]*common.GossipMessage, 0, len(ids))
	for _, id := range ids {
		if msg, exists := g.messages[id]; exists {
			result = append(result, msg)
		}
	}
	return result
}

// getMessagesByHashes returns messages for the given hashes
func (g *GossipManager) getMessagesByHashes(hashes []string) []*common.GossipMessage {
	// For now, hashes ARE the IDs in this implementation
	return g.getMessagesByIDs(hashes)
}

// requestMissingMessages requests messages we don't have
func (g *GossipManager) requestMissingMessages(peerID string, messageIDs []string) {
	var missing []string

	g.seenMu.RLock()
	for _, id := range messageIDs {
		if !g.seenFilter.Test([]byte(id)) {
			missing = append(missing, id)
		}
	}
	g.seenMu.RUnlock()

	if len(missing) == 0 {
		return
	}

	// Request missing messages
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var messages []*common.GossipMessage
	if err := g.transport.SendRPC(ctx, peerID, "gossip.messages", missing, &messages); err != nil {
		g.logger.Debug("failed to request messages", "peer", getShortID(peerID), "error", err)
		return
	}

	// Process received messages
	for _, msg := range messages {
		g.ReceiveMessage(peerID, msg)
	}
}

// requestMessagesByHash requests messages by their hash
func (g *GossipManager) requestMessagesByHash(peerID string, hashes []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var messages []*common.GossipMessage
	if err := g.transport.SendRPC(ctx, peerID, "gossip.by_hash", hashes, &messages); err != nil {
		g.logger.Debug("failed to request messages by hash", "peer", getShortID(peerID), "error", err)
		return
	}

	for _, msg := range messages {
		g.ReceiveMessage(peerID, msg)
	}
}

// sendMessagesByHash sends messages by their hash
func (g *GossipManager) sendMessagesByHash(peerID string, hashes []string) {
	g.messagesMu.RLock()
	messages := make([]*common.GossipMessage, 0, len(hashes))
	for _, hash := range hashes {
		if msg, exists := g.messages[hash]; exists {
			messages = append(messages, msg)
		}
	}
	g.messagesMu.RUnlock()

	if len(messages) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := g.transport.SendMessage(ctx, peerID, map[string]interface{}{
		"type":     "merkle.messages",
		"messages": messages,
	}); err != nil {
		g.logger.Debug("failed to send messages", "peer", getShortID(peerID), "error", err)
	}
}

// getRandomPeers returns random peers
func (g *GossipManager) getRandomPeers(count int) []string {
	g.peersMu.RLock()
	defer g.peersMu.RUnlock()

	if len(g.peers) <= count {
		return g.peers
	}

	// Fisher-Yates shuffle
	indices := rand.Perm(len(g.peers))
	selected := make([]string, 0, count)
	for i := 0; i < count && i < len(indices); i++ {
		selected = append(selected, g.peers[indices[i]])
	}

	return selected
}

// UpdatePeers updates the peer list
func (g *GossipManager) UpdatePeers(peers []string) {
	g.peersMu.Lock()
	g.peers = peers
	g.peersMu.Unlock()
}

// AddPeer adds a single peer to the gossip list
func (g *GossipManager) AddPeer(peerID string) {
	g.peersMu.Lock()
	defer g.peersMu.Unlock()
	for _, p := range g.peers {
		if p == peerID {
			return
		}
	}
	g.peers = append(g.peers, peerID)
}

// RemovePeer removes a peer from the gossip list
func (g *GossipManager) RemovePeer(peerID string) {
	g.peersMu.Lock()
	defer g.peersMu.Unlock()
	for i, p := range g.peers {
		if p == peerID {
			g.peers = append(g.peers[:i], g.peers[i+1:]...)
			return
		}
	}
}

// TotalPeers returns the number of peers currently in the gossip list
func (g *GossipManager) TotalPeers() int {
	g.peersMu.RLock()
	defer g.peersMu.RUnlock()
	return len(g.peers)
}

// checkRateLimit checks if a peer is rate limited
func (g *GossipManager) checkRateLimit(peerID string) bool {
	g.rateMu.RLock()
	defer g.rateMu.RUnlock()

	// Use the library's Allow method with peerID as key
	return g.limiter.Allow(peerID)
}

// computeMessageID computes a unique ID for a message
func (g *GossipManager) computeMessageID(msg *common.GossipMessage) string {
	h := sha256.New()
	h.Write([]byte(msg.Type))
	h.Write([]byte(msg.Sender))
	h.Write([]byte(fmt.Sprintf("%d", msg.Timestamp)))

	// Hash payload
	if payload, ok := msg.Payload.(map[string]interface{}); ok {
		for k, v := range payload {
			h.Write([]byte(k))
			h.Write([]byte(fmt.Sprintf("%v", v)))
		}
	} else {
		h.Write([]byte(fmt.Sprintf("%v", msg.Payload)))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// signMessage signs a gossip message
func (g *GossipManager) signMessage(msg *common.GossipMessage) error {
	// Create signature data
	data := g.signatureData(msg)

	// Attach public key
	msg.PublicKey = g.publicKey

	// Sign
	signature := ed25519.Sign(g.signKey, data)
	msg.Signature = signature

	return nil
}

// verifyMessage verifies a gossip message signature
func (g *GossipManager) verifyMessage(msg *common.GossipMessage) error {
	if len(msg.Signature) == 0 {
		return errors.New("message has no signature")
	}

	if len(msg.PublicKey) == 0 {
		return errors.New("message has no public key")
	}

	// Verify public key matches sender (if sender is derived from key)
	// For now, we just verify the signature against the attached key
	// In a real system, we'd verify Sender == Hash(PublicKey)

	data := g.signatureData(msg)

	if !ed25519.Verify(msg.PublicKey, data, msg.Signature) {
		return errors.New("invalid signature")
	}

	return nil
}

// signatureData creates data for signing/verification
func (g *GossipManager) signatureData(msg *common.GossipMessage) []byte {
	h := sha256.New()
	h.Write([]byte(msg.Type))
	h.Write([]byte(msg.Sender))
	h.Write([]byte(fmt.Sprintf("%d", msg.Timestamp)))
	h.Write([]byte(fmt.Sprintf("%d", msg.HopCount)))
	h.Write([]byte(fmt.Sprintf("%d", msg.MaxHops)))

	// Add payload
	if msg.Payload != nil {
		// Go's json.Marshal sorts map keys, which is deterministic for stable hashing
		data, _ := json.Marshal(msg.Payload)
		h.Write(data)
	}

	return h.Sum(nil)
}

// SignAttestation signs a mesh attestation payload using the gossip identity key.
func (g *GossipManager) SignAttestation(data []byte) ([]byte, ed25519.PublicKey, error) {
	if g.signKey == nil {
		return nil, nil, errors.New("gossip signing key not initialized")
	}
	signature := ed25519.Sign(g.signKey, data)
	return signature, g.publicKey, nil
}

// isDuplicate checks if we've seen a message
func (g *GossipManager) isDuplicate(msgID string) bool {
	g.seenMu.RLock()
	defer g.seenMu.RUnlock()

	return g.seenFilter.Test([]byte(msgID))
}

// markSeen marks a message as seen
func (g *GossipManager) markSeen(msgID string) {
	g.seenMu.Lock()
	g.seenFilter.Add([]byte(msgID))
	g.seenTimestamps[msgID] = time.Now()
	g.seenMu.Unlock()
}

// cleanupOldMessages removes old messages from the seen cache
func (g *GossipManager) cleanupOldMessages() {
	g.seenMu.Lock()
	defer g.seenMu.Unlock()

	now := time.Now()
	for msgID, timestamp := range g.seenTimestamps {
		if now.Sub(timestamp) > g.seenTTL {
			delete(g.seenTimestamps, msgID)
			// Production-grade bloom filter maintenance: reset periodically
			if len(g.seenTimestamps) > 10000 {
				g.resetSeenFilter()
			}
		}
	}

	// Reset bloom filter periodically (e.g., every hour)
	if len(g.seenTimestamps) == 0 {
		g.seenFilter = bloom.NewWithEstimates(
			g.config.BloomFilter.ExpectedElements,
			g.config.BloomFilter.FalsePositiveRate,
		)
	}
}

// updateStateWithMessage updates Merkle tree with a new message
func (g *GossipManager) updateStateWithMessage(msgID string, msg *common.GossipMessage) {
	g.stateMu.Lock()
	defer g.stateMu.Unlock()

	// Add message to Merkle tree (auto-routes to bucket)
	g.state.AddMessage(msgID)
	g.stateVersion++

	// Log for debugging if message type is provided
	if msg != nil && msg.Type != "" {
		g.logger.Debug("updated state with message", "type", msg.Type, "id", msgID)
	}
}

// getMessagePriority returns priority for message type
func (g *GossipManager) getMessagePriority(msgType string) int {
	switch msgType {
	case "chunk_announce":
		return 1 // High priority
	case "peer_capability":
		return 2 // Medium priority
	default:
		return 3 // Low priority
	}
}

// metricsLoop updates metrics periodically
func (g *GossipManager) metricsLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-g.shutdown:
			return
		case <-ticker.C:
			g.updateMetrics()
		}
	}
}

// updateMetrics updates gossip metrics
func (g *GossipManager) updateMetrics() {
	g.metricsMu.Lock()

	// Update state size
	g.stateMu.RLock()
	g.metrics.StateSize = uint64(len(g.messages))
	g.stateMu.RUnlock()

	// Update queue length
	g.metrics.QueueLength = uint32(len(g.messageQueue))

	g.metricsMu.Unlock()
}

// cleanupLoop performs periodic cleanup
func (g *GossipManager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-g.shutdown:
			return
		case <-ticker.C:
			g.cleanup()
		}
	}
}

// cleanup performs general cleanup
func (g *GossipManager) cleanup() {
	// Cleanup old sync states
	g.syncMu.Lock()
	for peer, state := range g.syncState {
		if time.Since(state.LastSync) > 10*time.Minute {
			delete(g.syncState, peer)
		}
	}
	g.syncMu.Unlock()

	// Cleanup old messages
	g.messagesMu.Lock()
	cutoff := time.Now().Add(-24 * time.Hour).UnixNano()
	for id, msg := range g.messages {
		if msg.Timestamp < cutoff {
			delete(g.messages, id)
		}
	}
	g.messagesMu.Unlock()

	// Reset seen filter if needed
	if len(g.seenTimestamps) > 10000 {
		g.resetSeenFilter()
	}
}

// resetSeenFilter clears the seen filter and timestamps
func (g *GossipManager) resetSeenFilter() {
	g.seenMu.Lock()
	defer g.seenMu.Unlock()

	g.seenFilter = bloom.NewWithEstimates(100000, 0.01)
	g.seenTimestamps = make(map[string]time.Time)
	g.logger.Debug("seen filter reset")
}

// recordPropagationLatency records message propagation latency
func (g *GossipManager) recordPropagationLatency(latency time.Duration) {
	g.metricsMu.Lock()
	defer g.metricsMu.Unlock()

	// Maintain a sliding window of the last 1000 latencies
	g.latencyHistory = append(g.latencyHistory, latency)
	if len(g.latencyHistory) > 1000 {
		g.latencyHistory = g.latencyHistory[1:]
	}

	// Calculate P50 and P95
	if len(g.latencyHistory) > 0 {
		sorted := make([]time.Duration, len(g.latencyHistory))
		copy(sorted, g.latencyHistory)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i] < sorted[j]
		})

		g.metrics.PropagationLatencyP50 = float64(sorted[len(sorted)*5/10].Milliseconds())
		g.metrics.PropagationLatencyP95 = float64(sorted[len(sorted)*95/100].Milliseconds())
	}
}

// GetMetrics returns gossip metrics
func (g *GossipManager) GetMetrics() GossipMetrics {
	g.metricsMu.RLock()
	defer g.metricsMu.RUnlock()
	return g.metrics
}

// GetMessageRate returns messages per second
func (g *GossipManager) GetMessageRate() float32 {
	g.metricsMu.RLock()
	defer g.metricsMu.RUnlock()

	duration := time.Since(g.metrics.StartTime).Seconds()
	if duration <= 0 {
		return 0
	}

	return float32(float64(g.metrics.MessagesSent+g.metrics.MessagesReceived) / duration)
}

// IsHealthy checks if gossip is healthy
func (g *GossipManager) IsHealthy() bool {
	// Check if we have peers
	g.peersMu.RLock()
	hasPeers := len(g.peers) > 0
	g.peersMu.RUnlock()

	// Check if queue is not overloaded
	g.metricsMu.RLock()
	queueOK := g.metrics.QueueLength < uint32(g.queueSize/2)
	g.metricsMu.RUnlock()

	// Check if we're sending/receiving messages
	rate := g.GetMessageRate()
	rateOK := rate > 0.1 // At least 0.1 messages per second

	return hasPeers && queueOK && rateOK
}

// GetHealthScore returns a health score (0-1)
func (g *GossipManager) GetHealthScore() float32 {
	var score float32

	// Peer count component
	g.peersMu.RLock()
	peerScore := float32(len(g.peers)) / 100.0 // Normalize to 0-1
	if peerScore > 1.0 {
		peerScore = 1.0
	}
	g.peersMu.RUnlock()
	score += peerScore * 0.3

	// Queue health component
	g.metricsMu.RLock()
	queueUtilization := float32(g.metrics.QueueLength) / float32(g.queueSize)
	queueScore := 1.0 - queueUtilization
	g.metricsMu.RUnlock()
	score += queueScore * 0.3

	// Message rate component
	rate := g.GetMessageRate()
	rateScore := rate / 10.0 // Normalize (10 msg/sec = perfect score)
	if rateScore > 1.0 {
		rateScore = 1.0
	}
	score += rateScore * 0.4

	return score
}

// registerDefaultHandlers registers default message handlers
func (g *GossipManager) registerDefaultHandlers() {
	g.RegisterHandler("chunk_announce", func(msg *common.GossipMessage) error {
		// Handle chunk announcement
		if payload, ok := msg.Payload.(map[string]interface{}); ok {
			chunkHash, _ := payload["chunk_hash"].(string)
			nodeID, _ := payload["node_id"].(string)

			g.logger.Debug("received chunk announcement",
				"chunk", getShortID(chunkHash),
				"from", getShortID(nodeID))

			// Update DHT with this information
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			g.transport.Store(ctx, nodeID, chunkHash, []byte(nodeID))
			cancel()
		}
		return nil
	})

	g.RegisterHandler("peer_capability", func(msg *common.GossipMessage) error {
		// Update peer capabilities
		// Update mesh coordinator via transport advertise
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		g.transport.Advertise(ctx, "mesh.sync", g.nodeID)
		cancel()
		return nil
	})

	g.RegisterHandler("merkle.sync", func(msg *common.GossipMessage) error {
		// Handle Merkle sync requests
		return g.handleMerkleSync(msg)
	})

	// ========== SDP Relay Handlers for Decentralized WebRTC Signaling ==========

	g.RegisterHandler("sdp.notify", func(msg *common.GossipMessage) error {
		return g.handleSDPNotify(msg)
	})

	g.RegisterHandler("sdp.relay", func(msg *common.GossipMessage) error {
		return g.handleSDPRelay(msg)
	})

	g.RegisterHandler("ice.relay", func(msg *common.GossipMessage) error {
		return g.handleICERelay(msg)
	})
}

// ========== SDP Relay Handlers ==========

// SDPNotifyPayload is the payload for sdp.notify messages
type SDPNotifyPayload struct {
	OriginatorID string `json:"originator_id"`
	TargetID     string `json:"target_id"`
	SessionID    string `json:"session_id"`
	Timestamp    int64  `json:"timestamp"`
	Nonce        []byte `json:"nonce"`
}

// SDPRelayPayload is the payload for sdp.relay messages
type SDPRelayPayload struct {
	OriginatorID string `json:"originator_id"`
	TargetID     string `json:"target_id"`
	SessionID    string `json:"session_id"`
	SDP          []byte `json:"sdp"` // Encrypted SDP
	HopCount     uint8  `json:"hop_count"`
	MaxHops      uint8  `json:"max_hops"`
	Timestamp    int64  `json:"timestamp"`
	Signature    []byte `json:"signature"`
}

// ICERelayPayload is the payload for ice.relay messages
type ICERelayPayload struct {
	OriginatorID  string `json:"originator_id"`
	TargetID      string `json:"target_id"`
	SessionID     string `json:"session_id"`
	Candidate     string `json:"candidate"`
	SDPMLineIndex uint16 `json:"sdp_mline_index"`
	Timestamp     int64  `json:"timestamp"`
}

// handleSDPNotify handles sdp.notify messages - lightweight notification that SDP is available
func (g *GossipManager) handleSDPNotify(msg *common.GossipMessage) error {
	// Parse payload
	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		return nil
	}
	var notify SDPNotifyPayload
	if err := json.Unmarshal(payloadBytes, &notify); err != nil {
		return nil
	}

	// Am I the target?
	if notify.TargetID != g.nodeID {
		// Not for me, let gossip propagate naturally
		return nil
	}

	g.logger.Info("received SDP notify",
		"from", getShortID(notify.OriginatorID),
		"session", getShortID(notify.SessionID))

	// Notify transport layer to fetch SDP from DHT and handle handshake
	// The transport layer will call DHT.FindPeers(hashSDP(originator, target, session))
	if handler, exists := g.handlers["sdp.ready"]; exists {
		return handler(msg)
	}

	return nil
}

// handleSDPRelay handles sdp.relay messages - full SDP forwarding
func (g *GossipManager) handleSDPRelay(msg *common.GossipMessage) error {
	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		return nil
	}
	var relay SDPRelayPayload
	if err := json.Unmarshal(payloadBytes, &relay); err != nil {
		return nil
	}

	// Check hop limit
	if relay.HopCount >= relay.MaxHops {
		g.logger.Debug("SDP relay exceeded max hops", "session", getShortID(relay.SessionID))
		return nil
	}

	// Am I the target?
	if relay.TargetID == g.nodeID {
		g.logger.Info("received SDP relay (target)",
			"from", getShortID(relay.OriginatorID),
			"session", getShortID(relay.SessionID))

		// Pass to transport for WebRTC handshake
		if handler, exists := g.handlers["sdp.offer"]; exists {
			return handler(msg)
		}
		return nil
	}

	// Am I directly connected to the target?
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := g.transport.Ping(ctx, relay.TargetID); err == nil {
		// Forward directly to target
		relay.HopCount++
		msg.Payload = relay
		return g.transport.SendMessage(ctx, relay.TargetID, msg)
	}

	// Otherwise, let natural gossip propagation handle it
	return nil
}

// handleICERelay handles ice.relay messages - ICE candidate forwarding
func (g *GossipManager) handleICERelay(msg *common.GossipMessage) error {
	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		return nil
	}
	var relay ICERelayPayload
	if err := json.Unmarshal(payloadBytes, &relay); err != nil {
		return nil
	}

	// Am I the target?
	if relay.TargetID != g.nodeID {
		return nil // Let gossip propagate
	}

	g.logger.Debug("received ICE relay",
		"from", getShortID(relay.OriginatorID),
		"session", getShortID(relay.SessionID))

	// Pass to transport for ICE handling
	if handler, exists := g.handlers["ice.candidate"]; exists {
		return handler(msg)
	}
	return nil
}

// handleMerkleSync handles Merkle tree synchronization requests
func (g *GossipManager) handleMerkleSync(msg *common.GossipMessage) error {
	// If it's from ourselves, ignore
	if msg.Sender == g.nodeID {
		return nil
	}

	// Payload should contain the sender's Merkle root
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return nil
	}

	theirRootStr, ok := payload["root"].(string)
	if !ok {
		return nil
	}

	theirRoot, err := base64.StdEncoding.DecodeString(theirRootStr)
	if err != nil {
		return nil
	}

	g.stateMu.RLock()
	ourRoot := g.state.Root
	g.stateMu.RUnlock()

	// If roots differ, trigger anti-entropy sync with this peer
	if !bytes.Equal(ourRoot, theirRoot) {
		g.logger.Debug("root mismatch detected via gossip, triggering sync",
			"peer", getShortID(msg.Sender),
			"our_root", getShortID(base64.StdEncoding.EncodeToString(ourRoot)),
			"their_root", getShortID(theirRootStr))

		// Run sync in background
		go g.reconcileMerkleTrees(msg.Sender, ourRoot, theirRoot)
	}

	return nil
}

// merkleAnnouncementLoop periodically broadcasts our Merkle root
func (g *GossipManager) merkleAnnouncementLoop() {
	ticker := time.NewTicker(g.config.AntiEntropyInterval * 2)
	defer ticker.Stop()

	for {
		select {
		case <-g.shutdown:
			return
		case <-ticker.C:
			g.announceMerkleRoot()
		}
	}
}

// announceMerkleRoot broadcasts our current Merkle root
func (g *GossipManager) announceMerkleRoot() {
	g.stateMu.RLock()
	root := g.state.Root
	g.stateMu.RUnlock()

	if len(root) == 0 {
		return
	}

	msg := &common.GossipMessage{
		ID:        fmt.Sprintf("merkle_sync_%d", time.Now().UnixNano()),
		Type:      "merkle.sync",
		Sender:    g.nodeID,
		Timestamp: time.Now().UnixNano(),
		TTL:       3, // Limited TTL for sync announcements
		Payload: map[string]interface{}{
			"root": base64.StdEncoding.EncodeToString(root),
		},
	}

	g.logger.Debug("announcing merkle root", "root", getShortID(base64.StdEncoding.EncodeToString(root)))
	g.Broadcast("merkle.sync", msg)
}

// ========== MerkleTree Implementation ==========

// NewMerkleTree creates a new stable bucket Merkle tree
func NewMerkleTree() *MerkleTree {
	mt := &MerkleTree{
		Layers: make([][][]byte, 0),
	}
	for i := range mt.Buckets {
		mt.Buckets[i] = make([]string, 0)
	}
	mt.rebuild() // Initial empty tree
	return mt
}

// AddMessage adds a message to the tree, routing it to the correct bucket
// Uses copy-on-write to ensure thread safety
func (mt *MerkleTree) AddMessage(msgID string) {
	h := sha256.Sum256([]byte(msgID))
	bucketIdx := h[0]

	// Create a copy of the bucket array for modification (copy-on-write)
	newBuckets := mt.Buckets
	bucket := make([]string, len(newBuckets[bucketIdx]))
	copy(bucket, newBuckets[bucketIdx])

	// Add to bucket if not already present
	found := false
	for _, id := range bucket {
		if id == msgID {
			found = true
			break
		}
	}

	if !found {
		bucket = append(bucket, msgID)
		sort.Strings(bucket)

		// Update the bucket atomically
		newBuckets[bucketIdx] = bucket
		mt.Buckets = newBuckets
		mt.rebuild()
	}
}

// rebuild rebuilds the stable bucket Merkle tree
func (mt *MerkleTree) rebuild() {
	// 1. Compute 256 bucket hashes (Layer 0)
	bucketHashes := make([][]byte, 256)
	for i := 0; i < 256; i++ {
		if len(mt.Buckets[i]) == 0 {
			bucketHashes[i] = make([]byte, 32) // Zero hash for empty bucket
		} else {
			h := sha256.New()
			for _, id := range mt.Buckets[i] {
				h.Write([]byte(id))
			}
			bucketHashes[i] = h.Sum(nil)
		}
	}

	// 2. Build tree levels (fixed depth)
	mt.Layers = [][][]byte{bucketHashes}
	current := bucketHashes

	for len(current) > 1 {
		var next [][]byte
		for i := 0; i < len(current); i += 2 {
			h := sha256.New()
			h.Write(current[i])
			if i+1 < len(current) {
				h.Write(current[i+1])
			} else {
				// Should not happen with 256 leaves (power of 2)
				h.Write(current[i])
			}
			next = append(next, h.Sum(nil))
		}
		mt.Layers = append(mt.Layers, next)
		current = next
	}

	if len(current) > 0 {
		mt.Root = current[0]
	}
}

// GetChildren returns the children hashes of a node hash
func (mt *MerkleTree) GetChildren(nodeHash []byte) [][]byte {
	if len(mt.Layers) <= 1 {
		return nil
	}

	// Find nodeHash in layers (except leaf layer)
	for h := len(mt.Layers) - 1; h > 0; h-- {
		layer := mt.Layers[h]
		for i, hsh := range layer {
			if bytes.Equal(hsh, nodeHash) {
				// Children are at layer h-1, indices 2*i and 2*i+1
				prevLayer := mt.Layers[h-1]
				children := [][]byte{prevLayer[2*i]}
				if 2*i+1 < len(prevLayer) {
					children = append(children, prevLayer[2*i+1])
				}
				return children
			}
		}
	}
	return nil
}
