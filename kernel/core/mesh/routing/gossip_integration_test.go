package routing

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
)

// TestGossipManager_Integration tests the full gossip loop
func TestGossipManager_Integration(t *testing.T) {
	transport := NewMockDHTTransport()
	gossip, err := NewGossipManager("node1", transport, nil)
	if err != nil {
		t.Fatalf("Failed to create GossipManager: %v", err)
	}

	// Set short intervals for testing
	gossip.config.RoundInterval = 10 * time.Millisecond
	gossip.config.AntiEntropyInterval = 100 * time.Millisecond

	err = gossip.Start()
	if err != nil {
		t.Fatalf("Failed to start GossipManager: %v", err)
	}
	defer gossip.Stop()

	// 1. Test Broadcast
	msgType := "system.alert"
	msgData := map[string]string{"foo": "bar"}

	err = gossip.Broadcast(msgType, msgData)
	if err != nil {
		t.Errorf("Broadcast failed: %v", err)
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	metrics := gossip.GetMetrics()
	if metrics.MessagesSent == 0 && gossip.TotalPeers() > 0 {
		// If we have peers, messages should be sent
		t.Log("Messages queued for broadcast")
	}

	// 2. Test ReceiveMessage (Simulation of incoming gossip)
	peerID := "peer2"
	incomingMsg := &common.GossipMessage{
		ID:        "msg_abc",
		Type:      msgType,
		Payload:   []byte(`{"hello":"world"}`),
		Sender:    peerID,
		Timestamp: time.Now().UnixNano(),
		HopCount:  1,
		MaxHops:   10,
	}
	gossip.signMessage(incomingMsg)

	// Register a handler
	var wg sync.WaitGroup
	wg.Add(1)
	gossip.RegisterHandler(msgType, func(msg *common.GossipMessage) error {
		wg.Done()
		return nil
	})

	err = gossip.ReceiveMessage(peerID, incomingMsg)
	if err != nil {
		t.Errorf("ReceiveMessage failed: %v", err)
	}

	// Wait for async processing with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("Gossip handler was not called for received message within timeout")
	}

	metrics = gossip.GetMetrics()
	if metrics.MessagesReceived != 1 {
		t.Errorf("Expected 1 message received, got %d", metrics.MessagesReceived)
	}
}

// TestGossipManager_Deduplication tests that same message is not processed twice
func TestGossipManager_Deduplication(t *testing.T) {
	transport := NewMockDHTTransport()
	gossip, err := NewGossipManager("node1", transport, nil)
	if err != nil {
		t.Fatalf("Failed to create GossipManager: %v", err)
	}

	gossip.Start()
	defer gossip.Stop()

	msgType := "test.dedup"

	// First instance
	incomingMsg1 := &common.GossipMessage{
		ID:        "duplicate_msg",
		Type:      msgType,
		Payload:   []byte("data"),
		Sender:    "peer2",
		Timestamp: 123456789,
		HopCount:  1,
		MaxHops:   10,
	}
	gossip.signMessage(incomingMsg1)

	// Second instance (identical to first, to simulate duplicate from another peer)
	incomingMsg2 := &common.GossipMessage{
		ID:        "duplicate_msg",
		Type:      msgType,
		Payload:   []byte("data"),
		Sender:    "peer2",
		Timestamp: 123456789,
		HopCount:  1,
		MaxHops:   10,
	}
	gossip.signMessage(incomingMsg2)

	var mu sync.Mutex
	handlerCount := 0
	gossip.RegisterHandler(msgType, func(msg *common.GossipMessage) error {
		mu.Lock()
		handlerCount++
		mu.Unlock()
		return nil
	})

	// Receive first message - should be accepted
	err1 := gossip.ReceiveMessage("peer2", incomingMsg1)
	if err1 != nil {
		t.Errorf("First ReceiveMessage failed: %v", err1)
	}

	// Receive second message - should be identified as duplicate
	err2 := gossip.ReceiveMessage("peer3", incomingMsg2)
	if err2 == nil {
		// Dedup logic in ReceiveMessage returns error
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := handlerCount
	mu.Unlock()

	if count != 1 {
		t.Errorf("Expected handler called once, got %d", count)
	}

	metrics := gossip.GetMetrics()
	if metrics.DuplicateMessages < 1 {
		t.Errorf("Expected at least 1 duplicate counted, got %d", metrics.DuplicateMessages)
	}
}

// TestGossipManager_PeerManagement tests adding/removing peers
func TestGossipManager_PeerManagement(t *testing.T) {
	gossip, _ := NewGossipManager("node1", NewMockDHTTransport(), nil)

	gossip.AddPeer("peer1")
	gossip.AddPeer("peer2")

	if gossip.TotalPeers() != 2 {
		t.Errorf("Expected 2 peers, got %d", gossip.TotalPeers())
	}

	gossip.RemovePeer("peer1")
	if gossip.TotalPeers() != 1 {
		t.Errorf("Expected 1 peer, got %d", gossip.TotalPeers())
	}
}

// TestGossipManager_RateLimiting tests message dropping when rate exceeded
func TestGossipManager_RateLimiting(t *testing.T) {
	gossip, _ := NewGossipManager("node1", NewMockDHTTransport(), nil)

	// Set very low rate
	gossip.config.RateLimit.MessagesPerSecond = 0.1
	gossip.config.RateLimit.BurstSize = 1

	// Initialize rate limiters by adding peers
	gossip.AddPeer("peer2")

	gossip.Start()
	defer gossip.Stop()

	msg := &common.GossipMessage{
		ID:        "m1",
		Type:      "t",
		Payload:   []byte("d"),
		Sender:    "peer2",
		Timestamp: time.Now().UnixNano(),
		MaxHops:   10,
	}
	gossip.signMessage(msg)

	// First one allowed
	err := gossip.ReceiveMessage("peer2", msg)
	if err != nil {
		t.Errorf("First message should be allowed, got error: %v", err)
	}

	// Second one immediately should be rate limited
	msg2 := &common.GossipMessage{
		ID:        "m2",
		Type:      "t",
		Payload:   []byte("d"),
		Sender:    "peer2",
		Timestamp: time.Now().UnixNano(),
		MaxHops:   10,
	}
	gossip.signMessage(msg2)
	err = gossip.ReceiveMessage("peer2", msg2)

	// It should return error immediately if rate limited in checkRateLimit
	if err == nil {
		t.Error("Expected second message to be rate limited/dropped")
	}

	metrics := gossip.GetMetrics()
	if metrics.RateLimited == 0 {
		t.Log("Rate limiting might be async, checking metrics...")
		time.Sleep(100 * time.Millisecond)
		metrics = gossip.GetMetrics()
		if metrics.RateLimited == 0 {
			t.Error("Expected rate limited count to be > 0")
		}
	}
}

// Mock transport with Peer discovery
type GossipMockTransport struct {
	MockDHTTransport
	peers []common.PeerInfo
}

func (m *GossipMockTransport) FindPeers(ctx context.Context, key string) ([]common.PeerInfo, error) {
	return m.peers, nil
}

// TestGossipManager_Health tests health and metrics reporting
func TestGossipManager_Health(t *testing.T) {
	gossip, _ := NewGossipManager("node1", NewMockDHTTransport(), nil)

	// Initially unhealthy (no peers, no rate)
	if gossip.IsHealthy() {
		t.Error("New GossipManager without peers should not be healthy")
	}

	// Make it healthy
	gossip.AddPeer("p1")
	gossip.metricsMu.Lock()
	gossip.metrics.MessagesSent = 100 // Simulate some activity
	gossip.metricsMu.Unlock()

	gossip.recordPropagationLatency(100 * time.Millisecond)

	rate := gossip.GetMessageRate()
	if rate < 0 {
		t.Errorf("Invalid message rate: %f", rate)
	}

	// Trigger metrics update
	gossip.updateMetrics()
}

// TestGossipManager_Maintenance tests cleanup and filter reset
func TestGossipManager_Maintenance(t *testing.T) {
	gossip, _ := NewGossipManager("node1", NewMockDHTTransport(), nil)

	// Manually add a message to local creations
	gossip.messagesMu.Lock()
	gossip.messages["msg1"] = &common.GossipMessage{
		ID:        "msg1",
		Timestamp: time.Now().Add(-48 * time.Hour).UnixNano(),
	}
	gossip.messagesMu.Unlock()

	// Run cleanup
	gossip.cleanup()

	gossip.messagesMu.RLock()
	// msg1 should be cleaned up (threshold is 24h)
	if _, ok := gossip.messages["msg1"]; ok {
		t.Error("msg1 should have been cleaned up from local messages")
	}
	gossip.messagesMu.RUnlock()

	// Test seen filter reset (manually trigger by filling)
	gossip.seenMu.Lock()
	for i := 0; i < 10001; i++ {
		gossip.seenTimestamps[fmt.Sprintf("m%d", i)] = time.Now()
	}
	gossip.seenMu.Unlock()

	gossip.cleanup()

	gossip.seenMu.RLock()
	if len(gossip.seenTimestamps) != 0 {
		t.Error("Seen timestamps should have been reset")
	}
	gossip.seenMu.RUnlock()
}

// TestGossipManager_UpdatePeers tests the peer list update logic
func TestGossipManager_UpdatePeers(t *testing.T) {
	gossip, _ := NewGossipManager("node1", NewMockDHTTransport(), nil)

	peers := []string{"p1", "p2", "p3"}
	gossip.UpdatePeers(peers)

	if gossip.TotalPeers() != 3 {
		t.Errorf("Expected 3 peers, got %d", gossip.TotalPeers())
	}
}

// TestGossipManager_Sync tests the anti-entropy sync trigger
func TestGossipManager_Sync(t *testing.T) {
	gossip, _ := NewGossipManager("node1", NewMockDHTTransport(), nil)
	gossip.AddPeer("p2")

	// Start sync manually
	gossip.performAntiEntropy()

	gossip.syncMu.RLock()
	if _, ok := gossip.syncState["p2"]; !ok {
		t.Error("Sync should have been initiated for p2")
	}
	gossip.syncMu.RUnlock()
}

// TestGossipManager_CapabilityAnnouncement tests announcing capabilities
func TestGossipManager_CapabilityAnnouncement(t *testing.T) {
	gossip, _ := NewGossipManager("node1", NewMockDHTTransport(), nil)
	gossip.Start()
	defer gossip.Stop()

	cap := &common.PeerCapability{
		PeerID:       "node1",
		Reputation:   1.0,
		LatencyMs:    10.0,
		Capabilities: []string{"gpu", "compute"},
	}

	err := gossip.AnnouncePeerCapability(cap)
	if err != nil {
		t.Errorf("AnnouncePeerCapability failed: %v", err)
	}
}

// TestGossipManager_StoppedQueue tests that methods return error when stopped
func TestGossipManager_StoppedQueue(t *testing.T) {
	gossip, _ := NewGossipManager("node1", NewMockDHTTransport(), nil)
	// Not starting

	cap := &common.PeerCapability{PeerID: "node1"}
	err := gossip.AnnouncePeerCapability(cap)
	if err == nil {
		t.Error("Expected error when calling AnnouncePeerCapability on stopped manager")
	}
}

// TestGossipManager_FullSync tests reconciliation between two nodes
func TestGossipManager_FullSync(t *testing.T) {
	t1 := NewMockDHTTransport()
	t2 := NewMockDHTTransport()

	// Link them
	t1.peers["node2"] = t2
	t2.peers["node1"] = t1

	g1, _ := NewGossipManager("node1", t1, nil)
	g2, _ := NewGossipManager("node2", t2, nil)

	g1.Start()
	g2.Start()
	defer g1.Stop()
	defer g2.Stop()

	// Add message to g2
	msg := &common.GossipMessage{ID: "m1", Timestamp: time.Now().UnixNano()}
	if msg.Timestamp == 0 {
		t.Error("Message timestamp should be set")
	}
	g2.stateMu.Lock()
	g2.state.AddMessage(msg.ID)
	g2.stateMu.Unlock()

	// node1 syncs with node2
	g1.reconcileMerkleTreesSimplified("node2")

	// In a real scenario, g1 would then request the message.
	// We just check if the RPC call was made.
	t1.mu.RLock()
	found := false
	for _, call := range t1.calls {
		if call == "rpc:node2:merkle.hashes" {
			found = true
			break
		}
	}
	t1.mu.RUnlock()

	if !found {
		t.Error("node1 did not request hashes from node2")
	}
}

// TestGossipManager_InteractiveSync tests the recursive Merkle tree sync
func TestGossipManager_InteractiveSync(t *testing.T) {
	t1 := NewMockDHTTransport()
	t2 := NewMockDHTTransport()

	t1.peers["node2"] = t2
	t2.peers["node1"] = t1

	g1, _ := NewGossipManager("node1", t1, nil)
	g2, _ := NewGossipManager("node2", t2, nil)

	g1.Start()
	g2.Start()
	defer g1.Stop()
	defer g2.Stop()

	// Add message to g2 only
	msgID := "m1"
	g2.stateMu.Lock()
	g2.state.AddMessage(msgID)
	g2.stateVersion++
	g2.stateMu.Unlock()

	// Exchange roots
	g1.stateMu.RLock()
	root1 := g1.state.Root
	g1.stateMu.RUnlock()

	g2.stateMu.RLock()
	root2 := g2.state.Root
	g2.stateMu.RUnlock()

	// They should differ
	if string(root1) == string(root2) {
		t.Fatal("Initial roots should differ")
	}

	// Trigger recursive reconciliation
	g1.reconcileMerkleTrees("node2", root1, root2)

	// Verify that merkle.children or merkle.bucket_ids was called
	t1.mu.RLock()
	found := false
	for _, call := range t1.calls {
		if call == "rpc:node2:merkle.children" || call == "rpc:node2:merkle.bucket_ids" {
			found = true
			break
		}
	}
	t1.mu.RUnlock()

	if !found {
		t.Error("node1 did not perform interactive Merkle walk with node2")
	}
}
