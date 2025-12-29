package mesh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
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
	messages   map[string]*GossipMessage // Local messages we've created
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
	transport Transport

	// Rate limiting
	rateLimiters map[string]*RateLimiter
	rateMu       sync.RWMutex

	// Message queue for backpressure
	messageQueue chan QueuedGossipMessage
	queueSize    int

	// Handlers
	handlers   map[string]GossipHandler
	handlersMu sync.RWMutex

	// Metrics
	metrics   GossipMetrics
	metricsMu sync.RWMutex

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
	Message   *GossipMessage
	Targets   []string
	Priority  int
	Timestamp time.Time
	Result    chan error
}

// MerkleTree implements Merkle tree for anti-entropy
type MerkleTree struct {
	Root   []byte
	Leaves []MerkleLeaf
	Nodes  [][]byte
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

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	rate       float64 // tokens per second
	capacity   float64 // burst capacity
	tokens     float64 // current tokens
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		capacity:   float64(burst),
		tokens:     float64(burst),
		lastUpdate: time.Now(),
	}
}

// Allow checks if a request is allowed
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastUpdate).Seconds()
	rl.tokens += elapsed * rl.rate

	if rl.tokens > rl.capacity {
		rl.tokens = rl.capacity
	}

	if rl.tokens < 1.0 {
		return false
	}

	rl.tokens--
	rl.lastUpdate = now
	return true
}

// GossipHandler processes gossip messages
type GossipHandler func(*GossipMessage) error

// NewGossipManager creates a production-ready gossip manager
func NewGossipManager(nodeID string, transport Transport, logger *slog.Logger) (*GossipManager, error) {
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
		messages:       make(map[string]*GossipMessage),
		seenFilter:     bf,
		seenTimestamps: make(map[string]time.Time),
		seenTTL:        config.MessageTTL,
		transport:      transport,
		rateLimiters:   make(map[string]*RateLimiter),
		messageQueue:   make(chan QueuedGossipMessage, config.QueueSize),
		queueSize:      config.QueueSize,
		handlers:       make(map[string]GossipHandler),
		config:         config,
		shutdown:       make(chan struct{}),
		logger:         logger.With("component", "gossip", "node_id", nodeID[:8]),
		syncState:      make(map[string]*MerkleSyncState),
	}

	// Initialize metrics
	gossip.metrics.StartTime = time.Now()

	// Register default handlers
	gossip.registerDefaultHandlers()

	return gossip, nil
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
	msg := &GossipMessage{
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
func (g *GossipManager) AnnouncePeerCapability(capability *PeerCapability) error {
	msg := &GossipMessage{
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
func (g *GossipManager) ReceiveMessage(sender string, msg *GossipMessage) error {
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
		g.logger.Debug("rate limited", "sender", sender[:8])
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
			"sender", msg.Sender[:8],
			"type", msg.Type,
			"error", err)
		return fmt.Errorf("signature verification failed: %w", err)
	}

	// Check hop count
	if msg.HopCount >= msg.MaxHops {
		g.logger.Debug("message exceeded max hops",
			"sender", msg.Sender[:8],
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
	if payload, ok := msg.Payload.(map[string]interface{}); ok {
		if ts, ok := payload["timestamp"].(int64); ok {
			latency := time.Since(time.Unix(ts, 0))
			g.recordPropagationLatency(latency)
		}
	}

	// Forward if not at max hops
	if msg.HopCount < msg.MaxHops-1 {
		msg.HopCount++
		go g.forwardMessage(msg)
	}

	g.logger.Debug("message processed",
		"sender", sender[:8],
		"type", msg.Type,
		"hops", msg.HopCount,
		"latency", time.Since(start))

	return nil
}

// processMessage handles a message based on its type
func (g *GossipManager) processMessage(msg *GossipMessage) error {
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
func (g *GossipManager) forwardMessage(msg *GossipMessage) {
	// Get random peers to forward to
	peers := g.getRandomPeers(g.config.Fanout)
	if len(peers) == 0 {
		return
	}

	// Queue for sending
	g.queueMessage(msg, peers)
}

// queueMessage adds a message to the send queue
func (g *GossipManager) queueMessage(msg *GossipMessage, targets []string) error {
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
		return <-queued.Result
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

	if len(queued.Targets) == 0 {
		// Broadcast to all peers
		peers := g.getAllPeers()
		if len(peers) > 0 {
			err = g.transport.Broadcast("gossip", queued.Message)
		}
	} else {
		// Send to specific peers
		var wg sync.WaitGroup
		errs := make(chan error, len(queued.Targets))

		for _, peer := range queued.Targets {
			wg.Add(1)
			go func(p string) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if sendErr := g.transport.SendMessage(ctx, p, queued.Message); sendErr != nil {
					errs <- fmt.Errorf("peer %s: %w", p[:8], sendErr)
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
	}

	// Update metrics
	if err == nil {
		g.metricsMu.Lock()
		g.metrics.MessagesSent++
		g.metricsMu.Unlock()
	}

	// Notify sender
	if queued.Result != nil {
		queued.Result <- err
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
			go func(p string, m *GossipMessage) {
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
				g.logger.Debug("pull request failed", "peer", p[:8], "error", err)
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
		g.logger.Debug("sync already in progress", "peer", peer[:8])
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

	g.logger.Debug("starting anti-entropy sync", "peer", peerID[:8])

	// Exchange Merkle roots
	g.stateMu.RLock()
	ourRoot := g.state.Root
	g.stateMu.RUnlock()

	var theirRoot []byte
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := g.transport.SendRPC(ctx, peerID, "merkle.root", nil, &theirRoot); err != nil {
		g.logger.Debug("failed to get peer root", "peer", peerID[:8], "error", err)
		return
	}

	// Compare roots
	if string(ourRoot) == string(theirRoot) {
		g.logger.Debug("states are synchronized", "peer", peerID[:8])
		return
	}

	// Roots differ, perform tree reconciliation
	g.reconcileMerkleTrees(peerID, ourRoot, theirRoot)

	g.metricsMu.Lock()
	g.metrics.SyncOperations++
	g.metricsMu.Unlock()

	g.logger.Debug("anti-entropy sync completed", "peer", peerID[:8])
}

// reconcileMerkleTrees reconciles differences using Merkle tree
func (g *GossipManager) reconcileMerkleTrees(peerID string, _, _ []byte) {
	// This is a simplified Merkle tree reconciliation
	// In production, you'd walk the tree to find differences

	// For now, we'll do a simple hash exchange
	ourHashes := g.getAllMessageHashes()

	var theirHashes []string
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := g.transport.SendRPC(ctx, peerID, "merkle.hashes", nil, &theirHashes); err != nil {
		g.logger.Debug("failed to get peer hashes", "peer", peerID[:8], "error", err)
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

// getRecentMessages returns recent messages
func (g *GossipManager) getRecentMessages(count int) []*GossipMessage {
	g.messagesMu.RLock()
	defer g.messagesMu.RUnlock()

	messages := make([]*GossipMessage, 0, count)
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
	for msgID := range g.messages {
		hashes = append(hashes, msgID)
	}

	return hashes
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

	var messages []*GossipMessage
	if err := g.transport.SendRPC(ctx, peerID, "gossip.messages", missing, &messages); err != nil {
		g.logger.Debug("failed to request messages", "peer", peerID[:8], "error", err)
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

	var messages []*GossipMessage
	if err := g.transport.SendRPC(ctx, peerID, "gossip.by_hash", hashes, &messages); err != nil {
		g.logger.Debug("failed to request messages by hash", "peer", peerID[:8], "error", err)
		return
	}

	for _, msg := range messages {
		g.ReceiveMessage(peerID, msg)
	}
}

// sendMessagesByHash sends messages by their hash
func (g *GossipManager) sendMessagesByHash(peerID string, hashes []string) {
	g.messagesMu.RLock()
	messages := make([]*GossipMessage, 0, len(hashes))
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
		g.logger.Debug("failed to send messages", "peer", peerID[:8], "error", err)
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

// getAllPeers returns all peers
func (g *GossipManager) getAllPeers() []string {
	g.peersMu.RLock()
	defer g.peersMu.RUnlock()

	peers := make([]string, len(g.peers))
	copy(peers, g.peers)
	return peers
}

// UpdatePeers updates the peer list
func (g *GossipManager) UpdatePeers(peers []string) {
	g.peersMu.Lock()
	g.peers = peers
	g.peersMu.Unlock()

	// Update rate limiters
	g.updateRateLimiters(peers)
}

// updateRateLimiters updates rate limiters for peers
func (g *GossipManager) updateRateLimiters(peers []string) {
	g.rateMu.Lock()
	defer g.rateMu.Unlock()

	// Create new rate limiters for new peers
	for _, peer := range peers {
		if _, exists := g.rateLimiters[peer]; !exists {
			g.rateLimiters[peer] = NewRateLimiter(
				g.config.RateLimit.MessagesPerSecond,
				g.config.RateLimit.BurstSize,
			)
		}
	}

	// Clean up old rate limiters
	for peer := range g.rateLimiters {
		found := false
		for _, p := range peers {
			if p == peer {
				found = true
				break
			}
		}
		if !found {
			delete(g.rateLimiters, peer)
		}
	}
}

// checkRateLimit checks if a peer is rate limited
func (g *GossipManager) checkRateLimit(peerID string) bool {
	g.rateMu.RLock()
	limiter, exists := g.rateLimiters[peerID]
	g.rateMu.RUnlock()

	if !exists {
		// Create new limiter
		g.rateMu.Lock()
		limiter = NewRateLimiter(
			g.config.RateLimit.MessagesPerSecond,
			g.config.RateLimit.BurstSize,
		)
		g.rateLimiters[peerID] = limiter
		g.rateMu.Unlock()
	}

	return limiter.Allow()
}

// computeMessageID computes a unique ID for a message
func (g *GossipManager) computeMessageID(msg *GossipMessage) string {
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
func (g *GossipManager) signMessage(msg *GossipMessage) error {
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
func (g *GossipManager) verifyMessage(msg *GossipMessage) error {
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
func (g *GossipManager) signatureData(msg *GossipMessage) []byte {
	h := sha256.New()
	h.Write([]byte(msg.Type))
	h.Write([]byte(msg.Sender))
	h.Write([]byte(fmt.Sprintf("%d", msg.Timestamp)))
	h.Write([]byte(fmt.Sprintf("%d", msg.HopCount)))
	h.Write([]byte(fmt.Sprintf("%d", msg.MaxHops)))

	// Add payload
	if msg.Payload != nil {
		// In production, use canonical JSON marshaling
		h.Write([]byte(fmt.Sprintf("%v", msg.Payload)))
	}

	return h.Sum(nil)
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
			// Note: Bloom filter doesn't support deletion
			// In production, use a counting bloom filter or periodic reset
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
func (g *GossipManager) updateStateWithMessage(msgID string, msg *GossipMessage) {
	g.stateMu.Lock()
	defer g.stateMu.Unlock()

	// Add leaf to Merkle tree
	leaf := MerkleLeaf{
		Key:   msgID,
		Value: []byte(fmt.Sprintf("%v", msg.Payload)),
		Hash:  []byte(msgID),
	}

	g.state.AddLeaf(leaf)
	g.stateVersion++
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
}

// recordPropagationLatency records message propagation latency
func (g *GossipManager) recordPropagationLatency(latency time.Duration) {
	// In production, store in a circular buffer and calculate percentiles
	// For now, just update a running average
	g.metricsMu.Lock()

	// Simple P50/P95 estimation (in production, use proper percentile calculation)
	latencyMs := float64(latency.Milliseconds())
	g.metrics.PropagationLatencyP50 = (g.metrics.PropagationLatencyP50 + latencyMs) / 2
	g.metrics.PropagationLatencyP95 = math.Max(g.metrics.PropagationLatencyP95, latencyMs)

	g.metricsMu.Unlock()
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
	g.RegisterHandler("chunk_announce", func(msg *GossipMessage) error {
		// Handle chunk announcement
		if payload, ok := msg.Payload.(map[string]interface{}); ok {
			chunkHash, _ := payload["chunk_hash"].(string)
			nodeID, _ := payload["node_id"].(string)

			g.logger.Debug("received chunk announcement",
				"chunk", chunkHash[:8],
				"from", nodeID[:8])

			// In production, update DHT with this information
		}
		return nil
	})

	g.RegisterHandler("peer_capability", func(msg *GossipMessage) error {
		// Update peer capabilities
		// In production, update mesh coordinator
		return nil
	})

	g.RegisterHandler("merkle.sync", func(msg *GossipMessage) error {
		// Handle Merkle sync requests
		return g.handleMerkleSync(msg)
	})
}

// handleMerkleSync handles Merkle tree synchronization requests
func (g *GossipManager) handleMerkleSync(_ *GossipMessage) error {
	// Implement Merkle sync protocol
	// This would handle requests for roots, hashes, and messages
	return nil
}

// ========== MerkleTree Implementation ==========

// NewMerkleTree creates a new Merkle tree
func NewMerkleTree() *MerkleTree {
	return &MerkleTree{
		Leaves: make([]MerkleLeaf, 0),
		Nodes:  make([][]byte, 0),
	}
}

// AddLeaf adds a leaf to the Merkle tree
func (mt *MerkleTree) AddLeaf(leaf MerkleLeaf) {
	mt.Leaves = append(mt.Leaves, leaf)
	mt.rebuild()
}

// rebuild rebuilds the Merkle tree
func (mt *MerkleTree) rebuild() {
	if len(mt.Leaves) == 0 {
		mt.Root = nil
		mt.Nodes = nil
		return
	}

	// Compute leaf hashes
	hashes := make([][]byte, len(mt.Leaves))
	for i, leaf := range mt.Leaves {
		if leaf.Hash != nil {
			hashes[i] = leaf.Hash
		} else {
			h := sha256.New()
			h.Write([]byte(leaf.Key))
			h.Write(leaf.Value)
			hashes[i] = h.Sum(nil)
		}
	}

	// Build tree
	mt.Nodes = hashes
	for len(mt.Nodes) > 1 {
		var level [][]byte
		for i := 0; i < len(mt.Nodes); i += 2 {
			if i+1 < len(mt.Nodes) {
				h := sha256.New()
				h.Write(mt.Nodes[i])
				h.Write(mt.Nodes[i+1])
				level = append(level, h.Sum(nil))
			} else {
				// Odd number of nodes, duplicate last
				h := sha256.New()
				h.Write(mt.Nodes[i])
				h.Write(mt.Nodes[i])
				level = append(level, h.Sum(nil))
			}
		}
		mt.Nodes = level
	}

	if len(mt.Nodes) > 0 {
		mt.Root = mt.Nodes[0]
	}
}

// GetProof returns a Merkle proof for a leaf
func (mt *MerkleTree) GetProof(leafKey string) ([][]byte, error) {
	// Find leaf index
	idx := -1
	for i, leaf := range mt.Leaves {
		if leaf.Key == leafKey {
			idx = i
			break
		}
	}

	if idx == -1 {
		return nil, errors.New("leaf not found")
	}

	// Generate proof
	var proof [][]byte
	level := mt.getLeafHashes()
	position := idx

	for len(level) > 1 {
		if position%2 == 0 {
			// Right sibling
			if position+1 < len(level) {
				proof = append(proof, level[position+1])
			} else {
				proof = append(proof, level[position]) // Duplicate
			}
		} else {
			// Left sibling
			proof = append(proof, level[position-1])
		}

		// Move up
		position = position / 2
		level = mt.getNextLevel(level)
	}

	return proof, nil
}

// getLeafHashes returns leaf hashes
func (mt *MerkleTree) getLeafHashes() [][]byte {
	hashes := make([][]byte, len(mt.Leaves))
	for i, leaf := range mt.Leaves {
		if leaf.Hash != nil {
			hashes[i] = leaf.Hash
		} else {
			h := sha256.New()
			h.Write([]byte(leaf.Key))
			h.Write(leaf.Value)
			hashes[i] = h.Sum(nil)
		}
	}
	return hashes
}

// getNextLevel computes next level of Merkle tree
func (mt *MerkleTree) getNextLevel(level [][]byte) [][]byte {
	var next [][]byte
	for i := 0; i < len(level); i += 2 {
		h := sha256.New()
		h.Write(level[i])
		if i+1 < len(level) {
			h.Write(level[i+1])
		} else {
			h.Write(level[i]) // Duplicate
		}
		next = append(next, h.Sum(nil))
	}
	return next
}

// MerkleProofNode represents a node in a Merkle proof with direction
type MerkleProofNode struct {
	Hash   []byte
	IsLeft bool // true if this sibling is on the left, false if on the right
}

// GenerateProofWithDirection generates a Merkle proof with direction information
func (mt *MerkleTree) GenerateProofWithDirection(leafKey string) ([]MerkleProofNode, error) {
	// Find leaf index
	leafIndex := -1
	for i, leaf := range mt.Leaves {
		if leaf.Key == leafKey {
			leafIndex = i
			break
		}
	}

	if leafIndex == -1 {
		return nil, errors.New("leaf not found")
	}

	var proof []MerkleProofNode
	currentLevel := mt.getLeafHashes()
	currentIndex := leafIndex

	// Traverse up the tree
	for len(currentLevel) > 1 {
		// Determine sibling index and direction
		var siblingIndex int
		var isLeft bool

		if currentIndex%2 == 0 {
			// Current node is left child, sibling is right
			siblingIndex = currentIndex + 1
			isLeft = false // Sibling is on the right
		} else {
			// Current node is right child, sibling is left
			siblingIndex = currentIndex - 1
			isLeft = true // Sibling is on the left
		}

		// Add sibling to proof if it exists
		if siblingIndex < len(currentLevel) {
			proof = append(proof, MerkleProofNode{
				Hash:   currentLevel[siblingIndex],
				IsLeft: isLeft,
			})
		}

		// Move to parent level
		currentLevel = mt.getNextLevel(currentLevel)
		currentIndex = currentIndex / 2
	}

	return proof, nil
}

// VerifyProof verifies a Merkle proof (backward compatible - assumes left-to-right order)
func (mt *MerkleTree) VerifyProof(leafKey string, leafValue []byte, proof [][]byte) bool {
	// Compute leaf hash
	h := sha256.New()
	h.Write([]byte(leafKey))
	h.Write(leafValue)
	hash := h.Sum(nil)

	// Recompute root using lexicographic ordering
	// This ensures deterministic hash computation
	for _, sibling := range proof {
		h := sha256.New()

		// Use lexicographic comparison to determine order
		// This makes the proof verification deterministic
		if bytes.Compare(hash, sibling) < 0 {
			// Current hash is smaller, put it first (left)
			h.Write(hash)
			h.Write(sibling)
		} else {
			// Sibling is smaller, put it first (left)
			h.Write(sibling)
			h.Write(hash)
		}

		hash = h.Sum(nil)
	}

	// Compare with root using bytes.Equal for proper comparison
	return bytes.Equal(hash, mt.Root)
}

// VerifyProofWithDirection verifies a Merkle proof with explicit direction information
func (mt *MerkleTree) VerifyProofWithDirection(leafKey string, leafValue []byte, proof []MerkleProofNode) bool {
	// Compute leaf hash
	h := sha256.New()
	h.Write([]byte(leafKey))
	h.Write(leafValue)
	hash := h.Sum(nil)

	// Recompute root using direction information
	for _, node := range proof {
		h := sha256.New()

		if node.IsLeft {
			// Sibling is on the left
			h.Write(node.Hash)
			h.Write(hash)
		} else {
			// Sibling is on the right
			h.Write(hash)
			h.Write(node.Hash)
		}

		hash = h.Sum(nil)
	}

	// Compare with root
	return bytes.Equal(hash, mt.Root)
}
