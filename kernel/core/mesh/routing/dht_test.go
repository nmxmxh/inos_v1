package routing

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
	"github.com/stretchr/testify/assert"
)

// MockTransport for DHT testing - fully implements common.Transport
type MockDHTTransport struct {
	mu        sync.RWMutex
	responses map[string]interface{}
	calls     []string
	handlers  map[string]func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error)
	peers     map[string]*MockDHTTransport // Simulate network
}

func NewMockDHTTransport() *MockDHTTransport {
	return &MockDHTTransport{
		responses: make(map[string]interface{}),
		calls:     make([]string, 0),
		handlers:  make(map[string]func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error)),
		peers:     make(map[string]*MockDHTTransport),
	}
}

func (m *MockDHTTransport) Start(ctx context.Context) error { return nil }
func (m *MockDHTTransport) Stop() error                     { return nil }
func (m *MockDHTTransport) Advertise(ctx context.Context, key string, value string) error {
	m.mu.Lock()
	m.calls = append(m.calls, fmt.Sprintf("advertise:%s:%s", key, value))
	m.mu.Unlock()
	return nil
}
func (m *MockDHTTransport) FindPeers(ctx context.Context, key string) ([]common.PeerInfo, error) {
	return nil, nil
}
func (m *MockDHTTransport) SendRPC(ctx context.Context, peerID string, method string, args interface{}, reply interface{}) error {
	m.mu.Lock()
	m.calls = append(m.calls, fmt.Sprintf("rpc:%s:%s", peerID, method))

	// Check if we have a peer mocked
	target, ok := m.peers[peerID]
	m.mu.Unlock()

	if ok {
		target.mu.RLock()
		handler, exists := target.handlers[method]
		target.mu.RUnlock()

		if exists {
			argBytes, _ := json.Marshal(args)
			res, err := handler(ctx, "caller", argBytes)
			if err != nil {
				return err
			}
			resBytes, _ := json.Marshal(res)
			return json.Unmarshal(resBytes, reply)
		}
	}

	return nil
}
func (m *MockDHTTransport) StreamRPC(ctx context.Context, peerID string, method string, args interface{}, writer io.Writer) (int64, error) {
	return 0, nil
}
func (m *MockDHTTransport) SendMessage(ctx context.Context, peerID string, msg interface{}) error {
	m.mu.Lock()
	m.calls = append(m.calls, fmt.Sprintf("msg:%s", peerID))
	m.mu.Unlock()
	return nil
}
func (m *MockDHTTransport) Broadcast(topic string, message interface{}) error { return nil }
func (m *MockDHTTransport) RegisterRPCHandler(method string, handler func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error)) {
	m.mu.Lock()
	m.handlers[method] = handler
	m.mu.Unlock()
}
func (m *MockDHTTransport) FindNode(ctx context.Context, peerID, targetID string) ([]common.PeerInfo, error) {
	return nil, nil
}
func (m *MockDHTTransport) FindValue(ctx context.Context, peerID, chunkHash string) ([]string, []common.PeerInfo, error) {
	return nil, nil, nil
}
func (m *MockDHTTransport) Store(ctx context.Context, peerID string, key string, value []byte) error {
	return nil
}
func (m *MockDHTTransport) Ping(ctx context.Context, peerID string) error { return nil }
func (m *MockDHTTransport) GetPeerCapabilities(peerID string) (*common.PeerCapability, error) {
	return nil, nil
}
func (m *MockDHTTransport) UpdateLocalCapabilities(capabilities *common.PeerCapability) error {
	return nil
}
func (m *MockDHTTransport) GetConnectionMetrics() common.ConnectionMetrics {
	return common.ConnectionMetrics{}
}
func (m *MockDHTTransport) GetHealth() common.TransportHealth {
	return common.TransportHealth{}
}
func (m *MockDHTTransport) GetStats() map[string]interface{} {
	return make(map[string]interface{})
}

func getSHA256ID(s string) string {
	h := sha256.Sum256([]byte(s))
	// Use raw bytes as string, take first 20 bytes (160 bits)
	return string(h[:20])
}

// TestDHT_NewDHT tests DHT initialization
func TestDHT_NewDHT(t *testing.T) {
	transport := NewMockDHTTransport()
	dht := NewDHT(getSHA256ID("node1"), transport, nil)

	if dht.nodeID != getSHA256ID("node1") {
		t.Errorf("Expected nodeID '%s', got '%s'", getSHA256ID("node1"), dht.nodeID)
	}

	if len(dht.buckets) != 160 {
		t.Errorf("Expected 160 buckets, got %d", len(dht.buckets))
	}
}

// TestDHT_AddPeer tests adding peers to the routing table
func TestDHT_AddPeer(t *testing.T) {
	transport := NewMockDHTTransport()
	nodeID := getSHA256ID("node1")
	dht := NewDHT(nodeID, transport, nil)

	peerID := getSHA256ID("peer1")
	peer := common.PeerInfo{
		ID:      peerID,
		Address: "192.168.1.1:8080",
	}

	dht.AddPeer(peer)

	count := dht.TotalPeers()
	if count != 1 {
		t.Errorf("Expected 1 peer, got %d", count)
	}
}

// TestDHT_Store tests storing values in the DHT
func TestDHT_Store(t *testing.T) {
	transport := NewMockDHTTransport()
	dht := NewDHT(getSHA256ID("node1"), transport, nil)

	chunkHash := getSHA256ID("chunk1")
	peerID := getSHA256ID("peer1")
	ttl := int64(3600)

	err := dht.Store(chunkHash, peerID, ttl)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	peers, err := dht.FindPeers(chunkHash)
	if err != nil {
		t.Fatalf("FindPeers failed: %v", err)
	}

	found := false
	for _, p := range peers {
		if p == peerID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Stored peer not found in FindPeers")
	}
}

// TestDHT_FindNode tests finding closest nodes
func TestDHT_FindNode(t *testing.T) {
	transport := NewMockDHTTransport()
	dht := NewDHT(getSHA256ID("node1"), transport, nil)

	for i := 0; i < 50; i++ {
		peer := common.PeerInfo{
			ID:      getSHA256ID(fmt.Sprintf("peer%d", i)),
			Address: "addr",
		}
		dht.AddPeer(peer)
	}

	closest := dht.FindNode(getSHA256ID("some_target"))
	// dht.k is 20
	if len(closest) != 20 {
		t.Errorf("Expected 20 closest nodes, got %d", len(closest))
	}
}

// TestDHT_ConcurrentOperations tests concurrent DHT operations
func TestDHT_ConcurrentOperations(t *testing.T) {
	transport := NewMockDHTTransport()
	dht := NewDHT(getSHA256ID("node1"), transport, nil)
	dht.k = 100   // Increase K to accommodate all peers in bucket 0 for this test
	dht.alpha = 5 // Increase concurrency
	var wg sync.WaitGroup
	numGoroutines := 50

	// Concurrent AddPeer with SHA-256 IDs for even distribution
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			peer := common.PeerInfo{
				ID:      getSHA256ID(fmt.Sprintf("peer%d", id)),
				Address: "addr",
			}
			dht.AddPeer(peer)
		}(i)
	}
	wg.Wait()

	// Concurrent Store
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			_ = dht.Store(getSHA256ID(fmt.Sprintf("chunk%d", id)), getSHA256ID("node1"), 3600)
		}(i)
	}
	wg.Wait()

	if dht.TotalPeers() != uint32(numGoroutines) {
		t.Errorf("Expected %d peers, got %d", numGoroutines, dht.TotalPeers())
	}
}

// TestDHT_HealthAndState tests DHT health and state reporting
func TestDHT_HealthAndState(t *testing.T) {
	transport := NewMockDHTTransport()
	dht := NewDHT(getSHA256ID("node1"), transport, nil)

	// Test IsHealthy
	if !dht.IsHealthy() {
		t.Error("New DHT should be healthy")
	}

	// Test GetHealthScore
	score := dht.GetHealthScore()
	if score < 0 || score > 100 {
		t.Errorf("Invalid health score: %f", score)
	}

	// Test GetState
	state := dht.GetState().(map[string]interface{})
	if _, ok := state["peer_count"]; !ok {
		t.Error("State missing peer_count")
	}

	// Test GetTotalChunksCount
	count := dht.GetTotalChunksCount()
	if count != 0 {
		t.Errorf("Expected 0 chunks, got %d", count)
	}

	// Add some peers and data
	dht.AddPeer(common.PeerInfo{ID: getSHA256ID("p1")})
	_ = dht.Store(getSHA256ID("c1"), getSHA256ID("p1"), 3600)

	if dht.TotalPeers() != 1 {
		t.Error("Peer count mismatch after addition")
	}
	if dht.GetTotalChunksCount() != 1 {
		t.Error("Chunk count mismatch after store")
	}

	// Test EstimateNetworkSize
	size := dht.EstimateNetworkSize()
	if size < 1 {
		t.Errorf("Unexpected network size: %d", size)
	}
}

// MockDHTStore for persistence testing
type MockDHTStore struct {
	buckets [][]common.PeerInfo
	store   map[string][]string
}

func (m *MockDHTStore) SaveRoutingTable(buckets [][]common.PeerInfo) error {
	m.buckets = buckets
	return nil
}
func (m *MockDHTStore) SaveStore(store map[string][]string) error {
	m.store = store
	return nil
}
func (m *MockDHTStore) LoadRoutingTable() ([][]common.PeerInfo, error) {
	return m.buckets, nil
}
func (m *MockDHTStore) LoadStore() (map[string][]string, error) {
	return m.store, nil
}

// TestDHT_Lifecycle tests Start, Stop and Metrics
func TestDHT_Lifecycle(t *testing.T) {
	transport := NewMockDHTTransport()
	dht := NewDHT(getSHA256ID("node1"), transport, nil)

	if err := dht.Start(); err != nil {
		t.Errorf("Start failed: %v", err)
	}

	metrics := dht.GetMetrics()
	if metrics.SuccessfulLookups != 0 {
		t.Errorf("Expected 0 successful lookups, got %d", metrics.SuccessfulLookups)
	}

	dht.Stop()
}

// TestDHT_StatePersistence tests SaveState and LoadState
func TestDHT_StatePersistence(t *testing.T) {
	transport := NewMockDHTTransport()
	dht := NewDHT(getSHA256ID("node1"), transport, nil)

	dht.AddPeer(common.PeerInfo{ID: getSHA256ID("p1"), Address: "addr1"})

	store := &MockDHTStore{}
	err := dht.SaveState(store)
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	dht2 := NewDHT(getSHA256ID("node1"), transport, nil)
	err = dht2.LoadState(store)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if dht2.TotalPeers() != 1 {
		t.Error("LoadState did not restore peers")
	}
}

// Benchmarks

func BenchmarkDHT_AddPeer(b *testing.B) {
	transport := NewMockDHTTransport()
	dht := NewDHT(getSHA256ID("node1"), transport, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		peer := common.PeerInfo{
			ID:      getSHA256ID(fmt.Sprintf("peer%d", i)),
			Address: "addr",
		}
		dht.AddPeer(peer)
	}
}
func TestDHT_StateAndMetrics(t *testing.T) {
	dht := NewDHT("test-node", nil, nil)

	// Test GetEntryCount
	assert.Equal(t, uint32(0), dht.GetEntryCount())

	// Test EstimateNetworkSize
	size := dht.EstimateNetworkSize()
	assert.Equal(t, 1, size) // Default for < 2 peers

	// Test Refresh
	dht.Refresh() // Should not panic

	// Test Stop
	dht.Stop()
}
