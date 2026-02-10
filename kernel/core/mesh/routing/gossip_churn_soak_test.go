package routing

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
)

const (
	churnSoakNodeCount     = 16
	churnSoakRounds        = 120
	churnFlapsPerRound     = 3
	churnBroadcastInterval = 8 * time.Millisecond
	churnFlapDowntime      = 3 * time.Millisecond
)

func TestGossipManager_MultiNodeChurnSoak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-node churn soak in short mode")
	}

	nodes, transports := newChurnSoakNetwork(t, churnSoakNodeCount)
	startChurnSoakNetwork(t, nodes)
	t.Cleanup(func() {
		for _, node := range nodes {
			node.Stop()
		}
	})

	topic := "mesh.churn.soak"
	received := make([]int64, len(nodes))
	for i := range nodes {
		idx := i
		nodes[i].RegisterHandler(topic, func(_ *common.GossipMessage) error {
			atomic.AddInt64(&received[idx], 1)
			return nil
		})
	}

	rng := rand.New(rand.NewSource(42))

	for round := 0; round < churnSoakRounds; round++ {
		for flap := 0; flap < churnFlapsPerRound; flap++ {
			a := rng.Intn(len(nodes))
			b := (a + 1 + rng.Intn(len(nodes)-1)) % len(nodes)
			disconnectChurnSoakLink(nodes, transports, a, b)
			time.Sleep(churnFlapDowntime)
			connectChurnSoakLink(nodes, transports, a, b)
		}

		sender := rng.Intn(len(nodes))
		payload := map[string]interface{}{
			"round":     round,
			"sender":    churnSoakNodeID(sender),
			"timestamp": time.Now().UnixNano(),
		}
		if err := nodes[sender].Broadcast(topic, payload); err != nil {
			t.Fatalf("broadcast failed at round %d from %s: %v", round, churnSoakNodeID(sender), err)
		}

		time.Sleep(churnBroadcastInterval)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		allReceived := true
		for i := range received {
			if atomic.LoadInt64(&received[i]) == 0 {
				allReceived = false
				break
			}
		}
		if allReceived || time.Now().After(deadline) {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	var totalReceived int64
	var starved []string
	for i := range received {
		got := atomic.LoadInt64(&received[i])
		totalReceived += got
		if got == 0 {
			starved = append(starved, churnSoakNodeID(i))
		}
	}

	if len(starved) > 0 {
		t.Fatalf("soak churn starved nodes with no deliveries: %v (total_received=%d)", starved, totalReceived)
	}
	if totalReceived < int64(churnSoakRounds) {
		t.Fatalf("expected aggregate deliveries >= rounds (%d), got %d", churnSoakRounds, totalReceived)
	}
}

func newChurnSoakNetwork(t *testing.T, nodeCount int) ([]*GossipManager, []*MockDHTTransport) {
	t.Helper()

	nodes := make([]*GossipManager, nodeCount)
	transports := make([]*MockDHTTransport, nodeCount)

	for i := 0; i < nodeCount; i++ {
		nodeID := churnSoakNodeID(i)
		transport := NewMockDHTTransport()
		gossip, err := NewGossipManager(nodeID, transport, nil)
		if err != nil {
			t.Fatalf("failed to create GossipManager for %s: %v", nodeID, err)
		}

		// Faster rounds and high rate limits keep soak deterministic under churn.
		gossip.config.RoundInterval = 15 * time.Millisecond
		gossip.config.AntiEntropyInterval = 120 * time.Millisecond
		gossip.config.Fanout = 4
		gossip.config.PushFactor = 3
		gossip.config.PullFactor = 2
		gossip.config.RateLimit.MessagesPerSecond = 10000
		gossip.config.RateLimit.BurstSize = 10000

		nodes[i] = gossip
		transports[i] = transport
		transport.SetGossipManager(nodeID, gossip)
	}

	// Baseline topology: ring + skip links for redundancy during flaps.
	for i := 0; i < nodeCount; i++ {
		connectChurnSoakLink(nodes, transports, i, (i+1)%nodeCount)
		connectChurnSoakLink(nodes, transports, i, (i+4)%nodeCount)
	}

	return nodes, transports
}

func startChurnSoakNetwork(t *testing.T, nodes []*GossipManager) {
	t.Helper()
	for i, node := range nodes {
		if err := node.Start(); err != nil {
			t.Fatalf("failed to start %s: %v", churnSoakNodeID(i), err)
		}
	}
}

func connectChurnSoakLink(nodes []*GossipManager, transports []*MockDHTTransport, a, b int) {
	if a == b {
		return
	}

	peerA := churnSoakNodeID(a)
	peerB := churnSoakNodeID(b)

	nodes[a].AddPeer(peerB)
	nodes[b].AddPeer(peerA)

	transports[a].mu.Lock()
	transports[a].peers[peerB] = transports[b]
	transports[a].mu.Unlock()

	transports[b].mu.Lock()
	transports[b].peers[peerA] = transports[a]
	transports[b].mu.Unlock()
}

func disconnectChurnSoakLink(nodes []*GossipManager, transports []*MockDHTTransport, a, b int) {
	if a == b {
		return
	}

	peerA := churnSoakNodeID(a)
	peerB := churnSoakNodeID(b)

	nodes[a].RemovePeer(peerB)
	nodes[b].RemovePeer(peerA)

	transports[a].mu.Lock()
	delete(transports[a].peers, peerB)
	transports[a].mu.Unlock()

	transports[b].mu.Lock()
	delete(transports[b].peers, peerA)
	transports[b].mu.Unlock()
}

func churnSoakNodeID(index int) string {
	return fmt.Sprintf("soak-node-%02d", index)
}
