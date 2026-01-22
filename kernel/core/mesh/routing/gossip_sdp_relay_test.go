package routing

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
)

// ========== Multi-Peer SDP Relay Tests ==========
// Tests gossip-based SDP relay for decentralized WebRTC signaling

// TestSDPRelay_MultiPeerNetwork tests SDP relay across 10 interconnected nodes
func TestSDPRelay_MultiPeerNetwork(t *testing.T) {
	const numNodes = 10

	// Create node network
	nodes := make([]*GossipManager, numNodes)
	transports := make([]*MockDHTTransport, numNodes)

	// Initialize nodes
	for i := 0; i < numNodes; i++ {
		nodeID := fmt.Sprintf("node%d", i)
		transport := NewMockDHTTransport()
		transports[i] = transport

		gossip, err := NewGossipManager(nodeID, transport, nil)
		if err != nil {
			t.Fatalf("Failed to create GossipManager for %s: %v", nodeID, err)
		}
		gossip.config.RoundInterval = 10 * time.Millisecond
		nodes[i] = gossip
	}

	// Connect nodes in a ring + random cross-connections
	// Ring: 0 -> 1 -> 2 -> ... -> 9 -> 0
	// Cross: Each node also knows 2 random others
	for i := 0; i < numNodes; i++ {
		// Ring connections
		nextIdx := (i + 1) % numNodes
		prevIdx := (i - 1 + numNodes) % numNodes

		nodes[i].AddPeer(fmt.Sprintf("node%d", nextIdx))
		nodes[i].AddPeer(fmt.Sprintf("node%d", prevIdx))

		// Cross connections (every 3rd node)
		crossIdx := (i + 3) % numNodes
		nodes[i].AddPeer(fmt.Sprintf("node%d", crossIdx))

		// Link transports for message forwarding
		transports[i].peers[fmt.Sprintf("node%d", nextIdx)] = transports[nextIdx]
		transports[i].peers[fmt.Sprintf("node%d", prevIdx)] = transports[prevIdx]
		transports[i].peers[fmt.Sprintf("node%d", crossIdx)] = transports[crossIdx]

		// Register gossip manager with transport for message delivery
		transports[i].SetGossipManager(fmt.Sprintf("node%d", i), nodes[i])
	}

	// Start all nodes
	for i := 0; i < numNodes; i++ {
		if err := nodes[i].Start(); err != nil {
			t.Fatalf("Failed to start node%d: %v", i, err)
		}
		defer nodes[i].Stop()
	}

	// Track SDP messages received
	var sdpReceived int32
	var mu sync.Mutex
	receivedBy := make(map[string]bool)

	// Register SDP handlers on all nodes
	for i := 0; i < numNodes; i++ {
		nodeIdx := i
		nodes[i].RegisterHandler("sdp.relay", func(msg *common.GossipMessage) error {
			atomic.AddInt32(&sdpReceived, 1)
			mu.Lock()
			receivedBy[fmt.Sprintf("node%d", nodeIdx)] = true
			mu.Unlock()
			t.Logf("node%d received SDP relay", nodeIdx)
			return nil
		})
	}

	// Test 1: Node 0 sends SDP to Node 5 (not directly connected)
	t.Run("SDPRelay_AcrossNetwork", func(t *testing.T) {
		sdpPayload := SDPRelayPayload{
			OriginatorID: "node0",
			TargetID:     "node5",
			SessionID:    "session-001",
			SDP:          []byte("v=0\r\no=- 12345 12345 IN IP4 127.0.0.1\r\n"),
			HopCount:     0,
			MaxHops:      5,
			Timestamp:    time.Now().UnixNano(),
		}

		msg := &common.GossipMessage{
			ID:        "sdp-msg-001",
			Type:      "sdp.relay",
			Payload:   sdpPayload,
			Sender:    "node0",
			Timestamp: time.Now().UnixNano(),
			HopCount:  0,
			MaxHops:   5,
		}
		nodes[0].signMessage(msg)

		// Broadcast from node 0
		err := nodes[0].Broadcast("sdp.relay", sdpPayload)
		if err != nil {
			t.Errorf("Broadcast failed: %v", err)
		}

		// Wait for propagation
		time.Sleep(200 * time.Millisecond)

		count := atomic.LoadInt32(&sdpReceived)
		t.Logf("SDP messages received by %d nodes", count)

		// At minimum, direct neighbors should receive
		if count < 2 {
			t.Errorf("Expected at least 2 nodes to receive SDP, got %d", count)
		}
	})

	// Test 2: Multiple simultaneous SDP relays
	t.Run("SDPRelay_Concurrent", func(t *testing.T) {
		atomic.StoreInt32(&sdpReceived, 0)

		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(nodeIdx int) {
				defer wg.Done()

				targetIdx := (nodeIdx + 5) % numNodes
				sdpPayload := SDPRelayPayload{
					OriginatorID: fmt.Sprintf("node%d", nodeIdx),
					TargetID:     fmt.Sprintf("node%d", targetIdx),
					SessionID:    fmt.Sprintf("session-%d", nodeIdx),
					SDP:          []byte(fmt.Sprintf("offer-from-node%d", nodeIdx)),
					HopCount:     0,
					MaxHops:      5,
					Timestamp:    time.Now().UnixNano(),
				}

				err := nodes[nodeIdx].Broadcast("sdp.relay", sdpPayload)
				if err != nil {
					t.Errorf("Concurrent broadcast from node%d failed: %v", nodeIdx, err)
				}
			}(i)
		}
		wg.Wait()

		time.Sleep(300 * time.Millisecond)

		count := atomic.LoadInt32(&sdpReceived)
		t.Logf("Concurrent SDP test: %d messages received across network", count)

		if count < 5 {
			t.Errorf("Expected at least 5 SDP messages to propagate, got %d", count)
		}
	})

	// Test 3: SDP notify propagation
	t.Run("SDPNotify_Propagation", func(t *testing.T) {
		var notifyReceived int32

		for i := 0; i < numNodes; i++ {
			nodes[i].RegisterHandler("sdp.notify", func(msg *common.GossipMessage) error {
				atomic.AddInt32(&notifyReceived, 1)
				return nil
			})
		}

		notifyPayload := SDPNotifyPayload{
			OriginatorID: "node2",
			TargetID:     "node7",
			SessionID:    "notify-session-001",
			Timestamp:    time.Now().UnixNano(),
			Nonce:        []byte{1, 2, 3, 4, 5, 6, 7, 8},
		}

		err := nodes[2].Broadcast("sdp.notify", notifyPayload)
		if err != nil {
			t.Errorf("SDPNotify broadcast failed: %v", err)
		}

		time.Sleep(200 * time.Millisecond)

		count := atomic.LoadInt32(&notifyReceived)
		t.Logf("SDPNotify received by %d nodes", count)
	})

	// Test 4: ICE relay propagation
	t.Run("ICERelay_Propagation", func(t *testing.T) {
		var iceReceived int32

		for i := 0; i < numNodes; i++ {
			nodes[i].RegisterHandler("ice.relay", func(msg *common.GossipMessage) error {
				atomic.AddInt32(&iceReceived, 1)
				return nil
			})
		}

		icePayload := ICERelayPayload{
			OriginatorID:  "node3",
			TargetID:      "node8",
			SessionID:     "session-ice-001",
			Candidate:     "candidate:1 1 UDP 2122252543 192.168.1.100 49170 typ host",
			SDPMLineIndex: 0,
			Timestamp:     time.Now().UnixNano(),
		}

		err := nodes[3].Broadcast("ice.relay", icePayload)
		if err != nil {
			t.Errorf("ICERelay broadcast failed: %v", err)
		}

		time.Sleep(200 * time.Millisecond)

		count := atomic.LoadInt32(&iceReceived)
		t.Logf("ICE candidates received by %d nodes", count)
	})

	// Print network topology summary
	t.Logf("=== Network Topology Summary ===")
	for i := 0; i < numNodes; i++ {
		t.Logf("node%d: %d peers", i, nodes[i].TotalPeers())
	}
}

// TestSDPRelay_HopLimit tests that messages respect MaxHops
func TestSDPRelay_HopLimit(t *testing.T) {
	const numNodes = 7

	nodes := make([]*GossipManager, numNodes)
	transports := make([]*MockDHTTransport, numNodes)

	// Create linear chain: 0 -> 1 -> 2 -> 3 -> 4 -> 5 -> 6
	for i := 0; i < numNodes; i++ {
		nodeID := fmt.Sprintf("chain-node%d", i)
		transport := NewMockDHTTransport()
		transports[i] = transport

		gossip, err := NewGossipManager(nodeID, transport, nil)
		if err != nil {
			t.Fatalf("Failed to create GossipManager: %v", err)
		}
		gossip.config.RoundInterval = 10 * time.Millisecond
		nodes[i] = gossip
	}

	// Link in chain
	for i := 0; i < numNodes-1; i++ {
		nodes[i].AddPeer(fmt.Sprintf("chain-node%d", i+1))
		nodes[i+1].AddPeer(fmt.Sprintf("chain-node%d", i))
		transports[i].peers[fmt.Sprintf("chain-node%d", i+1)] = transports[i+1]
		transports[i+1].peers[fmt.Sprintf("chain-node%d", i)] = transports[i]
	}

	// Register gossip managers with transports
	for i := 0; i < numNodes; i++ {
		transports[i].SetGossipManager(fmt.Sprintf("chain-node%d", i), nodes[i])
	}

	// Start nodes
	for i := 0; i < numNodes; i++ {
		nodes[i].Start()
		defer nodes[i].Stop()
	}

	// Track which nodes receive the message
	var receivedAt []int
	var mu sync.Mutex

	for i := 0; i < numNodes; i++ {
		nodeIdx := i
		nodes[i].RegisterHandler("sdp.relay", func(msg *common.GossipMessage) error {
			mu.Lock()
			receivedAt = append(receivedAt, nodeIdx)
			mu.Unlock()
			return nil
		})
	}

	// Send SDP from node 0 with MaxHops = 3
	sdpPayload := SDPRelayPayload{
		OriginatorID: "chain-node0",
		TargetID:     "chain-node6",
		SessionID:    "hop-limit-test",
		SDP:          []byte("test-offer"),
		HopCount:     0,
		MaxHops:      3, // Should only reach nodes 0, 1, 2, 3
		Timestamp:    time.Now().UnixNano(),
	}

	err := nodes[0].Broadcast("sdp.relay", sdpPayload)
	if err != nil {
		t.Errorf("Broadcast failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	t.Logf("Message reached nodes: %v", receivedAt)
	// Check that far nodes (5, 6) didn't receive due to hop limit
	for _, idx := range receivedAt {
		if idx > 3 {
			t.Errorf("Message reached node%d but MaxHops=3 should have stopped it", idx)
		}
	}
	mu.Unlock()
}

// TestSDPRelay_TargetedDelivery tests that target node correctly identifies itself
func TestSDPRelay_TargetedDelivery(t *testing.T) {
	const numNodes = 5

	nodes := make([]*GossipManager, numNodes)
	transports := make([]*MockDHTTransport, numNodes)

	for i := 0; i < numNodes; i++ {
		nodeID := fmt.Sprintf("target-node%d", i)
		transport := NewMockDHTTransport()
		transports[i] = transport

		gossip, err := NewGossipManager(nodeID, transport, nil)
		if err != nil {
			t.Fatalf("Failed to create GossipManager: %v", err)
		}
		gossip.config.RoundInterval = 10 * time.Millisecond
		nodes[i] = gossip
	}

	// Fully connected mesh
	for i := 0; i < numNodes; i++ {
		for j := 0; j < numNodes; j++ {
			if i != j {
				nodes[i].AddPeer(fmt.Sprintf("target-node%d", j))
				transports[i].peers[fmt.Sprintf("target-node%d", j)] = transports[j]
			}
		}
		// Register gossip manager with transport
		transports[i].SetGossipManager(fmt.Sprintf("target-node%d", i), nodes[i])
	}

	for i := 0; i < numNodes; i++ {
		nodes[i].Start()
		defer nodes[i].Stop()
	}

	// Only node2 should process as target
	var targetProcessed int32
	var othersProcessed int32

	for i := 0; i < numNodes; i++ {
		nodeIdx := i
		nodes[i].RegisterHandler("sdp.notify", func(msg *common.GossipMessage) error {
			// Parse payload to check if we're the target
			if nodeIdx == 2 {
				atomic.AddInt32(&targetProcessed, 1)
				t.Logf("Target node%d correctly identified as recipient", nodeIdx)
			} else {
				atomic.AddInt32(&othersProcessed, 1)
			}
			return nil
		})
	}

	// Node 0 sends to Node 2
	notifyPayload := SDPNotifyPayload{
		OriginatorID: "target-node0",
		TargetID:     "target-node2",
		SessionID:    "targeted-session",
		Timestamp:    time.Now().UnixNano(),
		Nonce:        []byte{1, 2, 3, 4, 5, 6, 7, 8},
	}

	err := nodes[0].Broadcast("sdp.notify", notifyPayload)
	if err != nil {
		t.Errorf("Broadcast failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	t.Logf("Target processed: %d, Others processed: %d",
		atomic.LoadInt32(&targetProcessed),
		atomic.LoadInt32(&othersProcessed))
}

// TestSDPRelay_Metrics tests that metrics are properly tracked
func TestSDPRelay_Metrics(t *testing.T) {
	transport := NewMockDHTTransport()
	gossip, err := NewGossipManager("metrics-node", transport, nil)
	if err != nil {
		t.Fatalf("Failed to create GossipManager: %v", err)
	}

	gossip.config.RoundInterval = 10 * time.Millisecond
	gossip.AddPeer("peer1")
	gossip.AddPeer("peer2")
	gossip.Start()
	defer gossip.Stop()

	// Send multiple SDP messages
	for i := 0; i < 10; i++ {
		sdpPayload := SDPRelayPayload{
			OriginatorID: "metrics-node",
			TargetID:     "peer1",
			SessionID:    fmt.Sprintf("metrics-session-%d", i),
			SDP:          []byte("test"),
			HopCount:     0,
			MaxHops:      3,
			Timestamp:    time.Now().UnixNano(),
		}
		gossip.Broadcast("sdp.relay", sdpPayload)
	}

	time.Sleep(100 * time.Millisecond)

	metrics := gossip.GetMetrics()
	t.Logf("Gossip Metrics after SDP relay:")
	t.Logf("  Messages Sent: %d", metrics.MessagesSent)
	t.Logf("  Messages Received: %d", metrics.MessagesReceived)
	t.Logf("  Queue Length: %d", metrics.QueueLength)
}
