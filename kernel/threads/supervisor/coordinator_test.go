package supervisor

import (
	"testing"
	"time"

	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoordinator_Basic(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)

	// Initialize MessageQueues
	inbox := foundation.NewMessageQueue(unsafe.Pointer(&sab[0]), sabSize, 1024, 256)
	outbox := foundation.NewMessageQueue(unsafe.Pointer(&sab[0]), sabSize, 2048, 256)

	protocol := NewProtocol(sab, 1, outbox, inbox)
	epoch := &foundation.EnhancedEpoch{}
	flow := NewFlowController(sab)

	coord := NewCoordinator(sab, "sup-1", 1, protocol, epoch, flow)
	require.NotNil(t, coord)

	// 1. Register Peers
	coord.RegisterPeer("peer-1", 2, []string{"compute:ml", "learning"})
	coord.RegisterPeer("peer-2", 3, []string{"compute:gpu"})

	// 2. Test GetCapablePeers
	capable := coord.getCapablePeers(MSG_JOB_REQUEST)
	assert.Len(t, capable, 2)

	capableLearning := coord.getCapablePeers(MSG_PATTERN_SHARE)
	assert.Len(t, capableLearning, 1)
	assert.Equal(t, "peer-1", capableLearning[0].supervisorID)
}

func TestCoordinator_SelectionStrategies(t *testing.T) {
	sab := make([]byte, 1024*1024)
	coord := NewCoordinator(sab, "sup-1", 1, nil, nil, nil)

	p1 := &PeerInfo{supervisorID: "p1", loadFactor: 0.8, latency: 100 * time.Millisecond}
	p2 := &PeerInfo{supervisorID: "p2", loadFactor: 0.2, latency: 200 * time.Millisecond}
	peers := []*PeerInfo{p1, p2}

	// 1. Least Loaded
	coord.peerSelector.strategy = StrategyLeastLoaded
	selected := coord.peerSelector.Select(peers)
	assert.Equal(t, "p2", selected.supervisorID)

	// 2. Lowest Latency
	coord.peerSelector.strategy = StrategyLowestLatency
	selected = coord.peerSelector.Select(peers)
	assert.Equal(t, "p1", selected.supervisorID)

	// 3. Round Robin
	coord.peerSelector.strategy = StrategyRoundRobin
	s1 := coord.peerSelector.Select(peers)
	s2 := coord.peerSelector.Select(peers)
	assert.NotEqual(t, s1.supervisorID, s2.supervisorID)
}

func TestCoordinator_CapabilityMatching(t *testing.T) {
	coord := &Coordinator{}
	peer := &PeerInfo{
		capabilities: []string{"compute:ml", "storage:ssd", "networking"},
	}

	assert.True(t, coord.hasCapability(peer, "compute"))
	assert.True(t, coord.hasCapability(peer, "compute:ml"))
	assert.False(t, coord.hasCapability(peer, "compute:gpu"))
	assert.True(t, coord.hasCapability(peer, "storage"))
	assert.True(t, coord.hasCapability(peer, "")) // All match empty
}

func TestCoordinator_StatsUpdate(t *testing.T) {
	sab := make([]byte, 1024*1024)
	flow := NewFlowController(sab)
	coord := NewCoordinator(sab, "sup-1", 1, nil, nil, flow)

	coord.RegisterPeer("p1", 2, nil)
	flow.RegisterSupervisor(2, 100)

	// Update stats
	coord.updatePeerStats("p1", 50*time.Millisecond, true)

	stats := coord.GetPeerStats()
	require.Len(t, stats, 1)
	assert.Equal(t, 50*time.Millisecond, stats[0].Latency)

	// Test moving average
	coord.updatePeerStats("p1", 150*time.Millisecond, true)
	stats = coord.GetPeerStats()
	// (50*3 + 150) / 4 = 75
	assert.Equal(t, 75*time.Millisecond, stats[0].Latency)
}

func TestCoordinator_RouteMessage(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)
	inbox := foundation.NewMessageQueue(unsafe.Pointer(&sab[0]), sabSize, 1024, 256)
	outbox := foundation.NewMessageQueue(unsafe.Pointer(&sab[0]), sabSize, 2048, 256)
	protocol := NewProtocol(sab, 1, outbox, inbox)
	flow := NewFlowController(sab)

	coord := NewCoordinator(sab, "sup-1", 1, protocol, &foundation.EnhancedEpoch{}, flow)

	// 1. No capable peers
	_, err := coord.RouteMessage(MSG_JOB_REQUEST, []byte("data"), PriorityNormal)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no capable peers")

	// 2. Register a capable peer
	coord.RegisterPeer("p1", 2, []string{"compute"})
	flow.RegisterSupervisor(2, 100)

	// Background Ack notifier for nextMsgID
	// msgID starts at 1, increments to 2 for first send
	go func() {
		time.Sleep(10 * time.Millisecond)
		protocol.ackManager.NotifyAck(2, true)
	}()

	peerID, err := coord.RouteMessage(MSG_JOB_REQUEST, []byte("data"), PriorityNormal)
	assert.NoError(t, err)
	assert.Equal(t, "p1", peerID)

	// 3. Test congestion (over 80% default threshold in flow_control.go)
	flow.UpdateQueueDepth(2, 85)
	_, err = coord.RouteMessage(MSG_JOB_REQUEST, []byte("data"), PriorityNormal)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "congested")
}
