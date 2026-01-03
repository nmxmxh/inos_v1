package mesh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
)

// MockStorage implements StorageProvider for testing
type MockStorage struct {
	chunks map[string][]byte
}

func (m *MockStorage) StoreChunk(ctx context.Context, hash string, data []byte) error {
	m.chunks[hash] = data
	return nil
}

func (m *MockStorage) FetchChunk(ctx context.Context, hash string) ([]byte, error) {
	if data, ok := m.chunks[hash]; ok {
		return data, nil
	}
	return nil, errors.New("not found")
}

func (m *MockStorage) HasChunk(ctx context.Context, hash string) (bool, error) {
	_, ok := m.chunks[hash]
	return ok, nil
}

// MockTransport implements Transport for testing
type MockTransport struct {
	nodeID      string
	rpcHandlers map[string]func(args interface{}) (interface{}, error)
	sentMsgs    []map[string]interface{}
	mu          sync.RWMutex
}

func (m *MockTransport) Start(ctx context.Context) error                  { return nil }
func (m *MockTransport) Stop() error                                      { return nil }
func (m *MockTransport) Connect(ctx context.Context, peerID string) error { return nil }
func (m *MockTransport) Disconnect(peerID string) error                   { return nil }
func (m *MockTransport) IsConnected(peerID string) bool                   { return true }
func (m *MockTransport) GetConnectedPeers() []string                      { return []string{} }

func (m *MockTransport) SendRPC(ctx context.Context, peerID string, method string, args interface{}, reply interface{}) error {
	m.mu.RLock()
	handler, ok := m.rpcHandlers[method]
	m.mu.RUnlock()

	if !ok {
		return errors.New("no handler for method " + method)
	}

	result, err := handler(args)
	if err != nil {
		return err
	}

	// Marshal/Unmarshal to simulate wire
	data, _ := json.Marshal(result)
	return json.Unmarshal(data, reply)
}

func (m *MockTransport) StreamRPC(ctx context.Context, peerID string, method string, args interface{}, writer io.Writer) (int64, error) {
	if method == "chunk.fetch" {
		data := []byte("direct-data")
		n, err := writer.Write(data)
		return int64(n), err
	}
	return 0, nil
}

func (m *MockTransport) SendMessage(ctx context.Context, peerID string, message interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if msg, ok := message.(map[string]interface{}); ok {
		m.sentMsgs = append(m.sentMsgs, msg)
	}
	return nil
}

func (m *MockTransport) Broadcast(topic string, message interface{}) error { return nil }
func (m *MockTransport) RegisterRPCHandler(method string, handler func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error)) {
}
func (m *MockTransport) GetPeerCapabilities(peerID string) (*common.PeerCapability, error) {
	return &common.PeerCapability{PeerID: peerID, LatencyMs: 10}, nil
}
func (m *MockTransport) UpdateLocalCapabilities(cap *common.PeerCapability) error { return nil }
func (m *MockTransport) Advertise(ctx context.Context, key, value string) error   { return nil }
func (m *MockTransport) FindPeers(ctx context.Context, key string) ([]common.PeerInfo, error) {
	return []common.PeerInfo{}, nil
}
func (m *MockTransport) GetConnectionMetrics() common.ConnectionMetrics {
	return common.ConnectionMetrics{}
}
func (m *MockTransport) GetHealth() common.TransportHealth {
	return common.TransportHealth{Status: "healthy"}
}
func (m *MockTransport) GetStats() map[string]interface{} {
	return map[string]interface{}{"active_connections": uint32(0)}
}
func (m *MockTransport) Ping(ctx context.Context, peerID string) error { return nil }

func (m *MockTransport) FindNode(ctx context.Context, peerID, targetID string) ([]common.PeerInfo, error) {
	return []common.PeerInfo{}, nil
}
func (m *MockTransport) FindValue(ctx context.Context, peerID, chunkHash string) ([]string, []common.PeerInfo, error) {
	return []string{}, []common.PeerInfo{}, nil
}
func (m *MockTransport) Store(ctx context.Context, peerID string, key string, value []byte) error {
	return nil
}

func TestMeshCoordinator_Lifecycle(t *testing.T) {
	nodeID := "test-node-1"
	tr := &MockTransport{nodeID: nodeID}

	coord := NewMeshCoordinator(nodeID, "us-east", tr, nil)

	// Test SetStorage
	storage := &MockStorage{chunks: make(map[string][]byte)}
	coord.SetStorage(storage)

	// Test Start
	err := coord.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}

	// Test Telemetry & Node Count
	if coord.GetNodeCount() != 1 {
		t.Errorf("Expected 1 node (self), got %d", coord.GetNodeCount())
	}

	telemetry := coord.GetTelemetry()
	if telemetry["region"] != "us-east" {
		t.Errorf("Expected region us-east, got %v", telemetry["region"])
	}

	sector := coord.GetSectorID()
	if sector < 0 || sector > 255 {
		t.Errorf("Invalid sector ID: %d", sector)
	}

	// Test Stop
	err = coord.Stop()
	if err != nil {
		t.Errorf("Failed to stop coordinator: %v", err)
	}
}

func TestMeshCoordinator_PeerSelection(t *testing.T) {
	nodeID := "test-node-1"
	tr := &MockTransport{nodeID: nodeID}
	coord := NewMeshCoordinator(nodeID, "us-east", tr, nil)

	peers := []*common.PeerCapability{
		{
			PeerID:        "peer-1",
			Region:        "us-east",
			LatencyMs:     10,
			BandwidthKbps: 100,
		},
		{
			PeerID:        "peer-2",
			Region:        "eu-west",
			LatencyMs:     100,
			BandwidthKbps: 50,
		},
		{
			PeerID:        "peer-3",
			Region:        "us-east",
			LatencyMs:     20,
			BandwidthKbps: 200,
		},
	}

	best, err := coord.selectBestPeer(peers)
	if err != nil {
		t.Fatalf("selectBestPeer failed: %v", err)
	}

	if best.PeerID != "peer-3" && best.PeerID != "peer-1" {
		t.Errorf("Unexpected best peer: %s", best.PeerID)
	}
}

func TestMeshCoordinator_CircuitBreaker(t *testing.T) {
	nodeID := "test-node-1"
	tr := &MockTransport{nodeID: nodeID}
	coord := NewMeshCoordinator(nodeID, "us-east", tr, nil)

	peerID := "bad-peer"

	// Initially closed
	if coord.isCircuitBreakerOpenForPeer(peerID) {
		t.Error("Breaker should be closed initially")
	}

	// Trigger failures to open breaker
	for i := 0; i < 6; i++ {
		coord.updateCircuitBreaker(peerID, false)
	}

	if !coord.isCircuitBreakerOpenForPeer(peerID) {
		t.Error("Breaker should be open after 5 failures")
	}

	// Wait for reset timeout (simulated or real)
	resource := "peer:" + peerID
	coord.cbMu.Lock()
	cb := coord.circuitBreakers[resource]
	cb.lastFailure = time.Now().Add(-2 * time.Minute)
	coord.cbMu.Unlock()

	// Should be half-open now on next check
	if coord.isCircuitBreakerOpenForPeer(peerID) {
		t.Error("Breaker should be half-open (not closed, but not fully open)")
	}

	// Success should close it
	coord.updateCircuitBreaker(peerID, true)
	if coord.isCircuitBreakerOpenForPeer(peerID) {
		t.Error("Breaker should be closed after success")
	}
}

func TestMeshCoordinator_DetailedTelemetry(t *testing.T) {
	nodeID := "test-node-1"
	tr := &MockTransport{nodeID: nodeID}
	coord := NewMeshCoordinator(nodeID, "us-east", tr, nil)

	// Record some successes and failures
	coord.recordFetchSuccess("hash1", "peer1", 100*time.Millisecond)
	coord.recordFetchFailure("hash2", "peer2", fmt.Errorf("failed"))
	coord.recordRPCFailure("peer3", "method", fmt.Errorf("rpc-fail"))

	telemetry := coord.GetTelemetry()
	// Just check if it doesn't panic and returns something
	if telemetry["node_count"] == nil {
		t.Error("Telemetry missing node_count")
	}

	// Test Scoring Logic
	peer := &common.PeerCapability{
		PeerID:        "peer1",
		Region:        "us-east",
		LatencyMs:     10,
		BandwidthKbps: 1000,
		Reputation:    0.8,
	}

	score := coord.calculatePeerScore(peer)
	if score <= 0 {
		t.Errorf("Expected positive score, got %f", score)
	}

	// Peer in different region should have lower score
	peerDiff := &common.PeerCapability{
		PeerID:        "peer2",
		Region:        "eu-west",
		LatencyMs:     10,
		BandwidthKbps: 1000,
		Reputation:    0.8,
	}
	scoreDiff := coord.calculatePeerScore(peerDiff)
	if scoreDiff >= score {
		t.Errorf("Expected lower score for different region, got %f >= %f", scoreDiff, score)
	}
}

func TestMeshCoordinator_ChunkOrchestration(t *testing.T) {
	nodeID := "test-node-1"
	tr := &MockTransport{
		nodeID:      nodeID,
		rpcHandlers: make(map[string]func(args interface{}) (interface{}, error)),
	}

	coord := NewMeshCoordinator(nodeID, "us-east", tr, nil)
	storage := &MockStorage{chunks: make(map[string][]byte)}
	coord.SetStorage(storage)

	// Mock peer response for chunk fetch
	tr.mu.Lock()
	tr.rpcHandlers["chunk.fetch"] = func(args interface{}) (interface{}, error) {
		return map[string]interface{}{
			"data": []byte("chunk-data"),
			"size": 10,
		}, nil
	}
	tr.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Test DistributeChunk
	chunkHash := "test-chunk-hash"
	data := []byte("chunk-data")

	// Add some peers to DHT first to avoid empty selection
	coord.dht.AddPeer(common.PeerInfo{
		ID:           "peer-1",
		Capabilities: &common.PeerCapability{PeerID: "peer-1", Reputation: 0.9, Region: "us-east"},
	})
	coord.dht.AddPeer(common.PeerInfo{
		ID:           "peer-2",
		Capabilities: &common.PeerCapability{PeerID: "peer-2", Reputation: 0.8, Region: "us-east"},
	})
	coord.dht.AddPeer(common.PeerInfo{
		ID:           "peer-3",
		Capabilities: &common.PeerCapability{PeerID: "peer-3", Reputation: 0.7, Region: "us-east"},
	})

	replicas, err := coord.DistributeChunk(ctx, chunkHash, data)
	if err != nil {
		t.Fatalf("DistributeChunk failed: %v", err)
	}
	if replicas == 0 {
		t.Error("Expected at least 1 replica")
	}

	// Verify local storage
	if has, _ := storage.HasChunk(ctx, chunkHash); !has {
		t.Error("Chunk not stored locally")
	}

	// 2. Test FetchChunk (from local storage)
	fetched, err := coord.FetchChunk(ctx, chunkHash)
	if err != nil {
		t.Fatalf("FetchChunk local failed: %v", err)
	}
	if string(fetched) != string(data) {
		t.Errorf("Expected %s, got %s", string(data), string(fetched))
	}

	// 3. Test FetchChunk (from peer)
	remoteHash := "remote-chunk-hash"
	// Ensure we have a peer in DHT that "has" this chunk
	coord.dht.Store(remoteHash, "peer-1", 3600)

	// Manually inject peer capability for peer-1 to ensure scoring works
	coord.cachePeer("peer-1", &common.PeerCapability{PeerID: "peer-1", LatencyMs: 5})

	fetchedRemote, err := coord.FetchChunk(ctx, remoteHash)
	if err != nil {
		t.Fatalf("FetchChunk remote failed: %v", err)
	}
	if string(fetchedRemote) != "chunk-data" {
		t.Errorf("Expected chunk-data, got %s", string(fetchedRemote))
	}
}

func TestMeshCoordinator_FetchChunkDirect(t *testing.T) {
	nodeID := "test-node-1"
	tr := &MockTransport{
		nodeID:      nodeID,
		rpcHandlers: make(map[string]func(args interface{}) (interface{}, error)),
	}

	coord := NewMeshCoordinator(nodeID, "us-east", tr, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	remoteHash := "direct-chunk-hash"
	coord.dht.Store(remoteHash, "peer-1", 3600)
	coord.cachePeer("peer-1", &common.PeerCapability{PeerID: "peer-1", LatencyMs: 5})

	var buf bytes.Buffer
	size, err := coord.FetchChunkDirect(ctx, remoteHash, &buf)
	if err != nil {
		t.Fatalf("FetchChunkDirect failed: %v", err)
	}
	if size != 11 {
		t.Errorf("Expected size 11, got %d", size)
	}
	if buf.String() != "direct-data" {
		t.Errorf("Expected direct-data, got %s", buf.String())
	}

	// Test local path in FetchChunkDirect
	coord.SetStorage(&MockStorage{chunks: map[string][]byte{"local-hash": []byte("local-data")}})
	var buf2 bytes.Buffer
	size2, err := coord.FetchChunkDirect(ctx, "local-hash", &buf2)
	if err != nil {
		t.Errorf("FetchChunkDirect local failed: %v", err)
	}
	if buf2.String() != "local-data" {
		t.Errorf("Expected local-data, got %s", buf2.String())
	}
	if size2 != 10 {
		t.Errorf("Expected size 10, got %d", size2)
	}
}

func signGossipMessage(msg *common.GossipMessage, priv ed25519.PrivateKey) {
	h := sha256.New()
	h.Write([]byte(msg.Type))
	h.Write([]byte(msg.Sender))
	h.Write([]byte(fmt.Sprintf("%d", msg.Timestamp)))
	h.Write([]byte(fmt.Sprintf("%d", msg.HopCount)))
	h.Write([]byte(fmt.Sprintf("%d", msg.MaxHops)))
	if msg.Payload != nil {
		data, _ := json.Marshal(msg.Payload)
		h.Write(data)
	}
	signData := h.Sum(nil)
	msg.Signature = ed25519.Sign(priv, signData)
}

func TestMeshCoordinator_GossipHandlers(t *testing.T) {
	nodeID := "test-node-1"
	tr := &MockTransport{nodeID: nodeID}
	coord := NewMeshCoordinator(nodeID, "us-east", tr, nil)

	// Create a key pair for signing gossip messages
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	// Test chunk_announce handler
	annMsg := &common.GossipMessage{
		ID:        "msg-1",
		Sender:    "peer-remote",
		Type:      "chunk_announce",
		Timestamp: time.Now().UnixNano(),
		HopCount:  0,
		MaxHops:   10,
		Payload: map[string]interface{}{
			"chunk_hash": "gossip-chunk-hash",
		},
		PublicKey: []byte(pub),
	}
	signGossipMessage(annMsg, priv)

	err := coord.gossip.ReceiveMessage("peer-remote", annMsg)
	if err != nil {
		t.Errorf("Failed to process chunk_announce: %v", err)
	}

	// Check if DHT now has the chunk
	peers, err := coord.dht.FindPeers("gossip-chunk-hash")
	if err != nil {
		t.Errorf("FindPeers failed: %v", err)
	}
	found := false
	for _, p := range peers {
		if p == "peer-remote" {
			found = true
			break
		}
	}
	if !found {
		t.Error("DHT did not store chunk from gossip announcement")
	}

	// Test peer_capability handler
	capMsg := &common.GossipMessage{
		ID:        "msg-2",
		Sender:    "peer-remote",
		Type:      "peer_capability",
		Timestamp: time.Now().UnixNano(),
		HopCount:  0,
		MaxHops:   10,
		Payload: map[string]interface{}{
			"peer_id":    "peer-remote",
			"region":     "eu-west",
			"latency_ms": float64(42),
		},
		PublicKey: []byte(pub),
	}
	signGossipMessage(capMsg, priv)

	err = coord.gossip.ReceiveMessage("peer-remote", capMsg)
	if err != nil {
		t.Errorf("Failed to process peer_capability: %v", err)
	}

	// Check if cache now has the peer
	cached := coord.getCachedPeer("peer-remote")
	if cached == nil || cached.Region != "eu-west" {
		t.Errorf("Peer cache not updated correctly: %v", cached)
	}
}

func TestMeshCoordinator_SendMessageAndMetrics(t *testing.T) {
	nodeID := "test-node-1"
	tr := &MockTransport{nodeID: nodeID}
	coord := NewMeshCoordinator(nodeID, "us-east", tr, nil)

	// Test SendMessage
	err := coord.SendMessage(context.Background(), "peer-1", "hello")
	if err != nil {
		t.Errorf("SendMessage failed: %v", err)
	}

	// Test GetMetrics
	_ = coord.GetMetrics()

	// Test manual updateMetrics
	coord.updateMetrics()
}

func TestMeshCoordinator_CleanupAndHealth(t *testing.T) {
	nodeID := "test-node-1"
	tr := &MockTransport{nodeID: nodeID}
	coord := NewMeshCoordinator(nodeID, "us-east", tr, nil)

	// Inject an expired cache entry
	coord.peerCacheMu.Lock()
	coord.peerCache["expired-peer"] = PeerCacheEntry{
		Capability:  &common.PeerCapability{PeerID: "expired-peer"},
		LastUpdated: time.Now().Add(-24 * time.Hour),
	}
	coord.peerCache["valid-peer"] = PeerCacheEntry{
		Capability:  &common.PeerCapability{PeerID: "valid-peer"},
		LastUpdated: time.Now(),
	}
	coord.peerCacheMu.Unlock()

	// Run cleanup
	coord.cleanupExpiredCache()

	coord.peerCacheMu.RLock()
	if _, ok := coord.peerCache["expired-peer"]; ok {
		t.Error("Expired peer not cleaned up")
	}
	if _, ok := coord.peerCache["valid-peer"]; !ok {
		t.Error("Valid peer incorrectly cleaned up")
	}
	coord.peerCacheMu.RUnlock()

	// Run health checks (should not panic)
	coord.performHealthChecks()
}
