package mesh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// MeshCoordinator orchestrates shared compute and storage across the global mesh.
// It bridges local compute (SAB) with remote resources (WebRTC P2P).
type MeshCoordinator struct {
	nodeID string
	region string

	// Core mesh components
	transport  Transport
	storage    StorageProvider
	dht        *DHT
	gossip     *GossipManager
	reputation *ReputationManager

	// Resource management
	allocator *AdaptiveAllocator

	// Local state
	localChunks   map[string]struct{} // Chunks we possess
	localChunksMu sync.RWMutex

	// Circuit breakers for unhealthy peers
	circuitBreakers map[string]*CircuitBreaker
	cbMu            sync.RWMutex

	// Caches
	peerCache     map[string]PeerCacheEntry
	peerCacheMu   sync.RWMutex
	peerCacheTTL  time.Duration
	chunkCache    *ChunkCache
	demandTracker *DemandTracker

	// Monitoring
	metrics      MeshMetrics
	metricsMu    sync.RWMutex
	healthTicker *time.Ticker
	shutdown     chan struct{}

	// Configuration
	config CoordinatorConfig
	logger *slog.Logger
}

// CoordinatorConfig holds mesh coordinator settings
type CoordinatorConfig struct {
	PeerSelectionWeights struct {
		Reputation float32 `json:"reputation"`
		Latency    float32 `json:"latency"`
		Bandwidth  float32 `json:"bandwidth"`
		Region     float32 `json:"region"`
		Freshness  float32 `json:"freshness"`
	} `json:"peer_selection_weights"`

	LookupTimeout  time.Duration `json:"lookup_timeout"`
	MaxRetries     int           `json:"max_retries"`
	CircuitBreaker struct {
		FailureThreshold int           `json:"failure_threshold"`
		ResetTimeout     time.Duration `json:"reset_timeout"`
		HalfOpenMax      int           `json:"half_open_max"`
	} `json:"circuit_breaker"`

	CacheTTL            time.Duration `json:"cache_ttl"`
	HealthCheckPeriod   time.Duration `json:"health_check_period"`
	MetricsUpdatePeriod time.Duration `json:"metrics_update_period"`
}

// PeerCacheEntry caches peer information
type PeerCacheEntry struct {
	Capability  *PeerCapability
	LastUpdated time.Time
	QueryCount  uint32
	SuccessRate float32
}

// CircuitBreaker prevents cascading failures
type CircuitBreaker struct {
	peerID           string
	failures         int
	successes        int
	state            BreakerState
	lastFailure      time.Time
	resetTimeout     time.Duration
	failureThreshold int
	mu               sync.RWMutex
}

type BreakerState int

const (
	BreakerClosed BreakerState = iota
	BreakerOpen
	BreakerHalfOpen
)

// DefaultCoordinatorConfig returns production defaults
func DefaultCoordinatorConfig() CoordinatorConfig {
	config := CoordinatorConfig{
		LookupTimeout:       10 * time.Second,
		MaxRetries:          3,
		CacheTTL:            5 * time.Minute,
		HealthCheckPeriod:   30 * time.Second,
		MetricsUpdatePeriod: 10 * time.Second,
	}

	config.PeerSelectionWeights.Reputation = 0.40
	config.PeerSelectionWeights.Latency = 0.25
	config.PeerSelectionWeights.Bandwidth = 0.20
	config.PeerSelectionWeights.Region = 0.10
	config.PeerSelectionWeights.Freshness = 0.05

	config.CircuitBreaker.FailureThreshold = 5
	config.CircuitBreaker.ResetTimeout = 30 * time.Second
	config.CircuitBreaker.HalfOpenMax = 3

	return config
}

// NewMeshCoordinator creates a mesh coordinator for shared compute
func NewMeshCoordinator(nodeID, region string, transport Transport, logger *slog.Logger) *MeshCoordinator {
	if logger == nil {
		logger = slog.Default()
	}

	config := DefaultCoordinatorConfig()

	coord := &MeshCoordinator{
		nodeID:          nodeID,
		region:          region,
		transport:       transport,
		localChunks:     make(map[string]struct{}),
		circuitBreakers: make(map[string]*CircuitBreaker),
		peerCache:       make(map[string]PeerCacheEntry),
		peerCacheTTL:    config.CacheTTL,
		shutdown:        make(chan struct{}),
		config:          config,
		logger:          logger.With("component", "mesh_coordinator", "node_id", nodeID[:8]),
	}

	// Initialize subsystems
	coord.dht = NewDHT(nodeID, transport, logger)
	coord.reputation = NewReputationManager(3*24*time.Hour, nil, logger)

	var err error
	coord.gossip, err = NewGossipManager(nodeID, transport, logger)
	if err != nil {
		logger.Error("failed to initialize gossip", "error", err)
	}

	// Initialize adaptive allocator
	coord.allocator = NewAdaptiveAllocator(5, 700, 0.375, 0.50)

	// Initialize chunk cache (10000 entries, 5 minute TTL)
	coord.chunkCache = NewChunkCache(10000, 5*time.Minute)

	// Initialize demand tracker
	coord.demandTracker = NewDemandTracker()

	// Register gossip handlers for mesh coordination
	coord.registerGossipHandlers()

	return coord
}

// SetStorage sets the local storage provider
func (m *MeshCoordinator) SetStorage(storage StorageProvider) {
	m.storage = storage
}

// Start begins mesh coordination
func (m *MeshCoordinator) Start() error {
	m.logger.Info("starting mesh coordinator")

	// Start subsystems
	if err := m.dht.Start(); err != nil {
		return fmt.Errorf("failed to start DHT: %w", err)
	}

	if err := m.gossip.Start(); err != nil {
		return fmt.Errorf("failed to start Gossip: %w", err)
	}

	// Start background processes
	m.healthTicker = time.NewTicker(m.config.HealthCheckPeriod)

	go m.metricsLoop()
	go m.healthLoop()
	go m.cacheCleanupLoop()

	m.logger.Info("mesh coordinator started - ready for shared compute")
	return nil
}

// Stop gracefully shuts down
func (m *MeshCoordinator) Stop() error {
	m.logger.Info("stopping mesh coordinator")

	close(m.shutdown)

	if m.healthTicker != nil {
		m.healthTicker.Stop()
	}

	m.gossip.Stop()
	m.dht.Stop()

	m.logger.Info("mesh coordinator stopped")
	return nil
}

// GetNodeCount returns the number of active nodes in the mesh (including self)
func (m *MeshCoordinator) GetNodeCount() int {
	stats := m.transport.GetStats()
	if activeConns, ok := stats["active_connections"].(uint32); ok {
		return int(activeConns) + 1
	}
	return 1
}

// GetTelemetry returns detailed mesh telemetry
func (m *MeshCoordinator) GetTelemetry() map[string]interface{} {
	stats := m.transport.GetStats()

	// Calculate average latency
	var totalLatency float32
	var peerCount int
	m.peerCacheMu.RLock()
	for _, entry := range m.peerCache {
		if entry.Capability != nil {
			totalLatency += entry.Capability.LatencyMs
			peerCount++
		}
	}
	m.peerCacheMu.RUnlock()

	avgLatency := float32(0)
	if peerCount > 0 {
		avgLatency = totalLatency / float32(peerCount)
	}

	return map[string]interface{}{
		"node_count":        m.GetNodeCount(),
		"sector_id":         m.GetSectorID(),
		"active_peers":      peerCount,
		"avg_latency_ms":    avgLatency,
		"bytes_sent":        stats["bytes_sent"],
		"bytes_received":    stats["bytes_received"],
		"messages_sent":     stats["messages_sent"],
		"messages_received": stats["messages_received"],
		"region":            m.region,
	}
}

// GetSectorID returns the sector identifier for this node
func (m *MeshCoordinator) GetSectorID() int {
	hash := 0
	for _, c := range m.nodeID {
		hash = (hash * 31) + int(c)
	}
	return (hash & 0x7FFFFFFF) % 256
}

// SendMessage sends a generic message to a target peer via the transport
func (m *MeshCoordinator) SendMessage(ctx context.Context, targetPeerID string, payload interface{}) error {
	m.logger.Debug("routing message to peer", "target", targetPeerID[:8])
	return m.transport.SendMessage(ctx, targetPeerID, payload)
}

// ========== SHARED COMPUTE ORCHESTRATION ==========

// DistributeChunk distributes a chunk across the mesh for shared storage
func (m *MeshCoordinator) DistributeChunk(ctx context.Context, chunkHash string, data []byte) (int, error) {
	start := time.Now()

	// 1. Calculate optimal replicas based on size and demand
	demandScore := m.demandTracker.GetDemandScore(chunkHash)
	replicas := 3 // m.allocator.CalculateReplicas(...) - Defaulting to 3 if allocator logic is complex or internal
	// Re-reading code: m.allocator.CalculateReplicas IS callled in original. I should keep it.
	replicas = m.allocator.CalculateReplicas(Resource{
		Size:        uint64(len(data)),
		Type:        "chunk",
		DemandScore: demandScore,
	})

	m.logger.Debug("distributing chunk",
		"chunk", chunkHash[:8],
		"size", len(data),
		"replicas", replicas)

	// 2. Find candidate peers via DHT
	closestPeers := m.dht.FindNode(chunkHash)

	// 3. Score and select best peers
	scored := m.scorePeers(closestPeers)
	selected := scored[:minInt(replicas, len(scored))]

	// 4. Send chunk to selected peers in parallel
	var wg sync.WaitGroup
	errors := make(chan error, len(selected))

	for _, peer := range selected {
		wg.Add(1)
		go func(p PeerInfo) {
			defer wg.Done()

			if err := m.sendChunkToPeer(ctx, p.ID, chunkHash, data); err != nil {
				errors <- fmt.Errorf("peer %s: %w", p.ID[:8], err)
			}
		}(peer)
	}

	wg.Wait()
	close(errors)

	// 5. Store in local DHT
	if err := m.dht.Store(chunkHash, m.nodeID, 3600); err != nil {
		m.logger.Warn("failed to store in DHT", "error", err)
	}

	// 6. Announce via gossip
	m.gossip.AnnounceChunk(chunkHash)

	// 7. Store locally
	if m.storage != nil {
		if err := m.storage.StoreChunk(ctx, chunkHash, data); err != nil {
			m.logger.Warn("failed to store chunk locally", "error", err)
		}
	}

	m.localChunksMu.Lock()
	m.localChunks[chunkHash] = struct{}{}
	m.localChunksMu.Unlock()

	// 8. Update chunk cache with all peers that received it
	peerIDs := make([]string, len(selected))
	for i, peer := range selected {
		peerIDs[i] = peer.ID
	}
	m.chunkCache.Put(chunkHash, peerIDs, 1.0)

	m.logger.Info("chunk distributed",
		"chunk", chunkHash[:8],
		"replicas", replicas,
		"duration", time.Since(start))

	return replicas, nil
}

// FetchChunk retrieves a chunk from the mesh for shared compute
func (m *MeshCoordinator) FetchChunk(ctx context.Context, chunkHash string) ([]byte, error) {
	start := time.Now()

	if m.storage != nil {
		if has, err := m.storage.HasChunk(ctx, chunkHash); err == nil && has {
			data, err := m.storage.FetchChunk(ctx, chunkHash)
			if err == nil {
				m.logger.Debug("chunk fetched from local storage", "chunk", chunkHash[:8])
				return data, nil
			}
			m.logger.Warn("failed to fetch locally even though HasChunk returned true", "error", err)
		}
	}

	// Check if we have it in our local index
	m.localChunksMu.RLock()
	_, hasLocal := m.localChunks[chunkHash]
	m.localChunksMu.RUnlock()

	if hasLocal && m.storage == nil {
		m.logger.Debug("chunk marked as local but storage provider missing", "chunk", chunkHash[:8])
	}

	// Track demand
	m.demandTracker.RecordAccess(chunkHash)

	// Find peers with this chunk
	var lastErr error
	for attempt := 0; attempt < m.config.MaxRetries; attempt++ {
		peer, err := m.FindBestPeerForChunk(ctx, chunkHash)
		if err != nil {
			lastErr = err
			continue
		}

		// Fetch from peer
		data, err := m.fetchFromPeer(ctx, chunkHash, peer)
		if err == nil {
			latency := time.Since(start)
			m.recordFetchSuccess(chunkHash, peer.PeerID, latency)

			m.logger.Debug("chunk fetched",
				"chunk", chunkHash[:8],
				"peer", peer.PeerID[:8],
				"size", len(data),
				"latency", latency)

			return data, nil
		}

		m.recordFetchFailure(chunkHash, peer.PeerID, err)
		lastErr = err

		// Exponential backoff
		if attempt < m.config.MaxRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}
	}

	return nil, fmt.Errorf("failed to fetch chunk after %d attempts: %w", m.config.MaxRetries, lastErr)
}

// FetchChunkDirect retrieves a chunk and writes it directly to the writer (Zero-Copy)
func (m *MeshCoordinator) FetchChunkDirect(ctx context.Context, chunkHash string, writer io.Writer) (int64, error) {
	// 1. Check local storage first
	if m.storage != nil {
		if has, err := m.storage.HasChunk(ctx, chunkHash); err == nil && has {
			data, err := m.storage.FetchChunk(ctx, chunkHash)
			if err == nil {
				n, err := writer.Write(data)
				return int64(n), err
			}
		}
	}

	// 2. Find best peer
	peer, err := m.FindBestPeerForChunk(ctx, chunkHash)
	if err != nil {
		return 0, err
	}

	// 3. Use StreamRPC for direct piping from network to writer
	return m.transport.StreamRPC(ctx, peer.PeerID, "chunk.fetch", map[string]string{
		"chunk_hash": chunkHash,
	}, writer)
}

// FindBestPeerForChunk finds the optimal peer for fetching a chunk
func (m *MeshCoordinator) FindBestPeerForChunk(ctx context.Context, chunkHash string) (*PeerCapability, error) {
	// Check circuit breaker
	if m.isCircuitBreakerOpen(chunkHash) {
		return nil, fmt.Errorf("circuit breaker open for chunk %s", chunkHash[:8])
	}

	// Try cache first
	if cached := m.getCachedPeers(chunkHash); len(cached) > 0 {
		bestPeer, err := m.selectBestPeer(cached)
		if err == nil {
			return bestPeer, nil
		}
	}

	// DHT lookup
	lkCtx, lkCancel := context.WithTimeout(ctx, m.config.LookupTimeout)
	defer lkCancel()

	peerIDs, err := m.dht.FindPeers(chunkHash)
	if err != nil {
		return nil, err
	}

	if len(peerIDs) == 0 {
		return nil, errors.New("chunk not found in mesh")
	}

	// Fetch capabilities in parallel
	peers, err := m.fetchPeerCapabilities(lkCtx, peerIDs)
	if err != nil {
		return nil, err
	}

	if len(peers) == 0 {
		return nil, errors.New("no peers with valid capabilities")
	}

	// Cache results
	m.cachePeers(chunkHash, peers)

	// Select best peer
	bestPeer, err := m.selectBestPeer(peers)
	if err != nil {
		return nil, err
	}

	m.recordLookupSuccess(chunkHash, bestPeer.PeerID)
	return bestPeer, nil
}

// ========== PEER SELECTION ==========

func (m *MeshCoordinator) selectBestPeer(peers []*PeerCapability) (*PeerCapability, error) {
	if len(peers) == 0 {
		return nil, errors.New("no peers to select from")
	}

	type scoredPeer struct {
		peer  *PeerCapability
		score float32
	}

	scoredPeers := make([]scoredPeer, len(peers))
	for i, peer := range peers {
		score := m.calculatePeerScore(peer)
		scoredPeers[i] = scoredPeer{peer: peer, score: score}
	}

	// Sort by score descending
	sort.Slice(scoredPeers, func(i, j int) bool {
		return scoredPeers[i].score > scoredPeers[j].score
	})

	m.logger.Debug("peer selection",
		"top_peer", scoredPeers[0].peer.PeerID[:8],
		"top_score", scoredPeers[0].score,
		"total_peers", len(peers))

	return scoredPeers[0].peer, nil
}

func (m *MeshCoordinator) calculatePeerScore(peer *PeerCapability) float32 {
	var score float32
	weights := m.config.PeerSelectionWeights

	// 1. Reputation
	reputation, _ := m.reputation.GetTrustScore(peer.PeerID)
	score += float32(reputation) * weights.Reputation

	// 2. Latency (inverse)
	latencyScore := m.calculateLatencyScore(peer.LatencyMs)
	score += latencyScore * weights.Latency

	// 3. Bandwidth
	bandwidthScore := m.calculateBandwidthScore(peer.BandwidthKbps)
	score += bandwidthScore * weights.Bandwidth

	// 4. Region proximity
	regionScore := m.calculateRegionScore(peer.Region)
	score += regionScore * weights.Region

	// 5. Freshness
	freshnessScore := m.calculateFreshnessScore(peer.LastSeen)
	score += freshnessScore * weights.Freshness

	return score
}

func (m *MeshCoordinator) calculateLatencyScore(latencyMs float32) float32 {
	if latencyMs <= 0 {
		return 1.0
	}
	if latencyMs >= 1000 {
		return 0.01
	}
	return float32(1.0 / (1.0 + 0.01*float64(latencyMs)))
}

func (m *MeshCoordinator) calculateBandwidthScore(bandwidthKbps float32) float32 {
	if bandwidthKbps <= 0 {
		return 0.0
	}
	score := bandwidthKbps / 1000000.0
	if score > 1.0 {
		return 1.0
	}
	return score
}

func (m *MeshCoordinator) calculateRegionScore(peerRegion string) float32 {
	if peerRegion == "" {
		return 0.5
	}
	if peerRegion == m.region {
		return 1.0
	}
	if len(peerRegion) >= 2 && len(m.region) >= 2 && peerRegion[:2] == m.region[:2] {
		return 0.7
	}
	if len(peerRegion) >= 1 && len(m.region) >= 1 && peerRegion[0] == m.region[0] {
		return 0.4
	}
	return 0.1
}

func (m *MeshCoordinator) calculateFreshnessScore(lastSeen int64) float32 {
	now := time.Now().UnixNano()
	age := time.Duration(now - lastSeen)

	if age < time.Minute {
		return 1.0
	}
	if age < time.Hour {
		return 0.5
	}
	if age < 24*time.Hour {
		return 0.2
	}
	return 0.1
}

func (m *MeshCoordinator) scorePeers(peers []PeerInfo) []PeerInfo {
	type scored struct {
		peer  PeerInfo
		score float32
	}

	scoredList := make([]scored, 0, len(peers))
	for _, peer := range peers {
		if peer.Capabilities != nil {
			score := m.calculatePeerScore(peer.Capabilities)
			scoredList = append(scoredList, scored{peer: peer, score: score})
		}
	}

	sort.Slice(scoredList, func(i, j int) bool {
		return scoredList[i].score > scoredList[j].score
	})

	result := make([]PeerInfo, len(scoredList))
	for i, s := range scoredList {
		result[i] = s.peer
	}

	return result
}

// ========== HELPER METHODS ==========

func (m *MeshCoordinator) sendChunkToPeer(ctx context.Context, peerID, chunkHash string, data []byte) error {
	return m.transport.SendMessage(ctx, peerID, map[string]interface{}{
		"type":       "chunk_store",
		"chunk_hash": chunkHash,
		"data":       data,
	})
}

func (m *MeshCoordinator) fetchFromPeer(ctx context.Context, chunkHash string, peer *PeerCapability) ([]byte, error) {
	if m.isCircuitBreakerOpenForPeer(peer.PeerID) {
		return nil, fmt.Errorf("circuit breaker open for peer %s", peer.PeerID[:8])
	}

	var result struct {
		Data []byte `json:"data"`
		Size int    `json:"size"`
	}

	err := m.transport.SendRPC(ctx, peer.PeerID, "chunk.fetch", map[string]string{
		"chunk_hash": chunkHash,
	}, &result)

	if err != nil {
		m.recordRPCFailure(peer.PeerID, "chunk.fetch", err)
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, errors.New("empty response from peer")
	}

	return result.Data, nil
}

func (m *MeshCoordinator) fetchPeerCapabilities(_ context.Context, peerIDs []string) ([]*PeerCapability, error) {
	var wg sync.WaitGroup
	results := make(chan *PeerCapability, len(peerIDs))
	sem := make(chan struct{}, 10) // Limit concurrency

	for _, peerID := range peerIDs {
		wg.Add(1)
		go func(pid string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if cached := m.getCachedPeer(pid); cached != nil {
				results <- cached
				return
			}

			cap, err := m.transport.GetPeerCapabilities(pid)
			if err != nil {
				return
			}

			if err := cap.Validate(); err != nil {
				return
			}

			m.cachePeer(pid, cap)
			results <- cap
		}(peerID)
	}

	wg.Wait()
	close(results)

	var peers []*PeerCapability
	for cap := range results {
		peers = append(peers, cap)
	}

	return peers, nil
}

// ========== CIRCUIT BREAKER ==========

func (m *MeshCoordinator) isCircuitBreakerOpen(resource string) bool {
	m.cbMu.RLock()
	cb, exists := m.circuitBreakers[resource]
	m.cbMu.RUnlock()

	if !exists {
		return false
	}

	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == BreakerOpen
}

func (m *MeshCoordinator) isCircuitBreakerOpenForPeer(peerID string) bool {
	return m.isCircuitBreakerOpen("peer:" + peerID)
}

func (m *MeshCoordinator) updateCircuitBreaker(peerID string, success bool) {
	resource := "peer:" + peerID

	m.cbMu.Lock()
	cb, exists := m.circuitBreakers[resource]
	if !exists {
		cb = &CircuitBreaker{
			peerID:           peerID,
			resetTimeout:     m.config.CircuitBreaker.ResetTimeout,
			failureThreshold: m.config.CircuitBreaker.FailureThreshold,
			state:            BreakerClosed,
		}
		m.circuitBreakers[resource] = cb
	}
	m.cbMu.Unlock()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case BreakerClosed:
		if !success {
			cb.failures++
			if cb.failures >= cb.failureThreshold {
				cb.state = BreakerOpen
				cb.lastFailure = time.Now()
				m.logger.Warn("circuit breaker opened", "peer", peerID[:8])
			}
		} else {
			cb.successes++
			if cb.successes >= 3 {
				cb.failures = 0
			}
		}

	case BreakerOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = BreakerHalfOpen
			cb.successes = 0
			cb.failures = 0
			m.logger.Info("circuit breaker half-open", "peer", peerID[:8])
		}

	case BreakerHalfOpen:
		if success {
			cb.successes++
			if cb.successes >= m.config.CircuitBreaker.HalfOpenMax {
				cb.state = BreakerClosed
				m.logger.Info("circuit breaker closed", "peer", peerID[:8])
			}
		} else {
			cb.state = BreakerOpen
			cb.lastFailure = time.Now()
			m.logger.Warn("circuit breaker re-opened", "peer", peerID[:8])
		}
	}
}

// ========== CACHE MANAGEMENT ==========

func (m *MeshCoordinator) getCachedPeers(chunkHash string) []*PeerCapability {
	mapping, found := m.chunkCache.Get(chunkHash)
	if !found {
		return nil
	}

	// Convert peer IDs to capabilities
	var capabilities []*PeerCapability
	for _, peerID := range mapping.PeerIDs {
		if cap := m.getCachedPeer(peerID); cap != nil {
			capabilities = append(capabilities, cap)
		}
	}

	return capabilities
}

func (m *MeshCoordinator) getCachedPeer(peerID string) *PeerCapability {
	m.peerCacheMu.RLock()
	entry, exists := m.peerCache[peerID]
	m.peerCacheMu.RUnlock()

	if !exists || time.Since(entry.LastUpdated) > m.peerCacheTTL {
		return nil
	}

	return entry.Capability
}

func (m *MeshCoordinator) cachePeer(peerID string, capability *PeerCapability) {
	m.peerCacheMu.Lock()
	m.peerCache[peerID] = PeerCacheEntry{
		Capability:  capability,
		LastUpdated: time.Now(),
	}
	m.peerCacheMu.Unlock()
}

func (m *MeshCoordinator) cachePeers(chunkHash string, peers []*PeerCapability) {
	if len(peers) == 0 {
		return
	}

	peerIDs := make([]string, len(peers))
	for i, peer := range peers {
		peerIDs[i] = peer.PeerID
		// Also cache individual peer capabilities
		m.cachePeer(peer.PeerID, peer)
	}

	// Cache with confidence based on number of peers
	confidence := float32(len(peers)) / 10.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	m.chunkCache.Put(chunkHash, peerIDs, confidence)
}

// ========== METRICS & MONITORING ==========

func (m *MeshCoordinator) metricsLoop() {
	ticker := time.NewTicker(m.config.MetricsUpdatePeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.updateMetrics()
		case <-m.shutdown:
			return
		}
	}
}

func (m *MeshCoordinator) updateMetrics() {
	m.metricsMu.Lock()
	defer m.metricsMu.Unlock()

	m.metrics.TotalPeers = m.dht.TotalPeers()
	m.metrics.DHTEntries = m.dht.getEntryCount()
	m.metrics.AvgReputation = float32(m.reputation.GetAverageScore())
	m.metrics.GossipRatePerSec = m.gossip.GetMessageRate()

	connMetrics := m.transport.GetConnectionMetrics()
	m.metrics.ConnectedPeers = connMetrics.ActiveConnections
	m.metrics.BytesSent = connMetrics.BytesSent
	m.metrics.BytesReceived = connMetrics.BytesReceived
	m.metrics.P50LatencyMs = connMetrics.LatencyP50
	m.metrics.P95LatencyMs = connMetrics.LatencyP95

	m.localChunksMu.RLock()
	m.metrics.LocalChunks = uint32(len(m.localChunks))
	m.localChunksMu.RUnlock()

	m.metrics.TotalChunksAvailable = m.dht.GetTotalChunksCount()
}

func (m *MeshCoordinator) healthLoop() {
	for {
		select {
		case <-m.healthTicker.C:
			m.performHealthChecks()
		case <-m.shutdown:
			return
		}
	}
}

func (m *MeshCoordinator) performHealthChecks() {
	if !m.dht.IsHealthy() {
		m.logger.Warn("DHT health check failed")
	}
	if !m.gossip.IsHealthy() {
		m.logger.Warn("gossip health check failed")
	}
}

func (m *MeshCoordinator) cacheCleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredCache()
		case <-m.shutdown:
			return
		}
	}
}

func (m *MeshCoordinator) cleanupExpiredCache() {
	m.peerCacheMu.Lock()
	defer m.peerCacheMu.Unlock()

	now := time.Now()
	for peerID, entry := range m.peerCache {
		if now.Sub(entry.LastUpdated) > m.peerCacheTTL {
			delete(m.peerCache, peerID)
		}
	}
}

// ========== GOSSIP HANDLERS ==========

func (m *MeshCoordinator) registerGossipHandlers() {
	m.gossip.RegisterHandler("chunk_announce", func(msg *GossipMessage) error {
		payload, ok := msg.Payload.(map[string]interface{})
		if !ok {
			return errors.New("invalid payload type for chunk_announce")
		}

		if chunkHash, ok := payload["chunk_hash"].(string); ok {
			m.dht.Store(chunkHash, msg.Sender, 1800)
		}
		return nil
	})

	m.gossip.RegisterHandler("peer_capability", func(msg *GossipMessage) error {
		payload, ok := msg.Payload.(map[string]interface{})
		if !ok {
			return errors.New("invalid payload type for peer_capability")
		}

		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		var capability PeerCapability
		if err := json.Unmarshal(data, &capability); err != nil {
			return err
		}

		m.cachePeer(capability.PeerID, &capability)
		return nil
	})
}

// ========== METRICS RECORDING ==========

func (m *MeshCoordinator) recordLookupSuccess(_ string, _ string) {
	m.metricsMu.Lock()
	m.metrics.ChunkFetchSuccessRate = m.calculateSuccessRate(m.metrics.ChunkFetchSuccessRate, true)
	m.metricsMu.Unlock()
}

func (m *MeshCoordinator) recordFetchSuccess(_ string, peerID string, latency time.Duration) {
	m.reputation.Report(peerID, true, float64(latency.Milliseconds()))
	m.updateCircuitBreaker(peerID, true)
}

func (m *MeshCoordinator) recordFetchFailure(_ string, peerID string, _ error) {
	m.reputation.Report(peerID, false, 0)
	m.updateCircuitBreaker(peerID, false)
}

func (m *MeshCoordinator) recordRPCFailure(_ string, peerID string, _ error) {
	m.reputation.Report(peerID, false, 0)
}

func (m *MeshCoordinator) calculateSuccessRate(currentRate float32, success bool) float32 {
	alpha := 0.1
	if success {
		return float32(alpha*1.0 + (1-alpha)*float64(currentRate))
	}
	return float32((1 - alpha) * float64(currentRate))
}

// GetMetrics returns current metrics
func (m *MeshCoordinator) GetMetrics() MeshMetrics {
	m.metricsMu.RLock()
	defer m.metricsMu.RUnlock()
	return m.metrics
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
