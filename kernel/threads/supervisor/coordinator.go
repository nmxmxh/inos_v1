package supervisor

import (
	"fmt"
	"sync"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
)

// Coordinator manages supervisor-to-supervisor communication with load balancing
type Coordinator struct {
	sab          []byte
	supervisorID string
	epochIndex   uint8

	// Communication
	protocol    *Protocol
	epoch       *foundation.EnhancedEpoch
	flowControl *FlowController

	// Peer management
	peers        map[string]*PeerInfo
	peerSelector *PeerSelector
	mu           sync.RWMutex
}

// PeerInfo tracks information about a peer supervisor
type PeerInfo struct {
	supervisorID string
	epochIndex   uint8
	capabilities []string
	loadFactor   float32 // 0.0-1.0 (1.0 = fully loaded)
	latency      time.Duration
	lastSeen     time.Time
}

// PeerSelector selects peers based on strategy
type PeerSelector struct {
	strategy     SelectionStrategy
	lastSelected int
}

// SelectionStrategy defines peer selection algorithm
type SelectionStrategy int

const (
	StrategyRoundRobin SelectionStrategy = iota
	StrategyLeastLoaded
	StrategyLowestLatency
	StrategyCapabilityMatch
)

// NewCoordinator creates a new coordinator
func NewCoordinator(
	sab []byte,
	supervisorID string,
	epochIndex uint8,
	protocol *Protocol,
	epoch *foundation.EnhancedEpoch,
	flowControl *FlowController,
) *Coordinator {
	return &Coordinator{
		sab:          sab,
		supervisorID: supervisorID,
		epochIndex:   epochIndex,
		protocol:     protocol,
		epoch:        epoch,
		flowControl:  flowControl,
		peers:        make(map[string]*PeerInfo),
		peerSelector: &PeerSelector{
			strategy: StrategyLeastLoaded,
		},
	}
}

// RegisterPeer registers a peer supervisor
func (c *Coordinator) RegisterPeer(peerID string, epochIndex uint8, capabilities []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.peers[peerID] = &PeerInfo{
		supervisorID: peerID,
		epochIndex:   epochIndex,
		capabilities: capabilities,
		loadFactor:   0.0,
		latency:      0,
		lastSeen:     time.Now(),
	}
}

// RouteMessage selects best peer for message and sends it
func (c *Coordinator) RouteMessage(
	msgType uint8,
	data []byte,
	priority MessagePriority,
) (string, error) {
	// Get capable peers
	capablePeers := c.getCapablePeers(msgType)
	if len(capablePeers) == 0 {
		return "", fmt.Errorf("no capable peers for message type %d", msgType)
	}

	// Select peer based on strategy
	selectedPeer := c.peerSelector.Select(capablePeers)
	if selectedPeer == nil {
		return "", fmt.Errorf("no suitable peer available")
	}

	// Check flow control
	if !c.flowControl.CanSend(selectedPeer.epochIndex) {
		return "", fmt.Errorf("peer %s is congested", selectedPeer.supervisorID)
	}

	// Send message
	start := time.Now()
	err := c.protocol.SendWithGuarantee(
		selectedPeer.epochIndex,
		msgType,
		data,
		time.Second,
	)
	latency := time.Since(start)

	// Update peer statistics
	c.updatePeerStats(selectedPeer.supervisorID, latency, err == nil)

	return selectedPeer.supervisorID, err
}

// Helper: Get peers capable of handling message type
func (c *Coordinator) getCapablePeers(msgType uint8) []*PeerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Map message types to required capabilities
	requiredCapability := c.getRequiredCapability(msgType)

	capable := make([]*PeerInfo, 0, len(c.peers))
	for _, peer := range c.peers {
		// Check if peer has required capability
		if c.hasCapability(peer, requiredCapability) {
			capable = append(capable, peer)
		}
	}

	return capable
}

// Helper: Map message type to required capability
func (c *Coordinator) getRequiredCapability(msgType uint8) string {
	switch msgType {
	case MSG_JOB_REQUEST:
		return "compute" // Any compute capability
	case MSG_JOB_COMPLETE:
		return "" // All supervisors can receive completions
	case MSG_RESOURCE_REQ:
		return "resource_management"
	case MSG_PATTERN_SHARE:
		return "learning"
	case MSG_HEALTH_CHECK:
		return "" // All supervisors support health checks
	default:
		return "" // Unknown message type, allow all
	}
}

// Helper: Check if peer has capability
func (c *Coordinator) hasCapability(peer *PeerInfo, capability string) bool {
	// Empty capability means all peers are capable
	if capability == "" {
		return true
	}

	// Check if peer has the specific capability
	for _, cap := range peer.capabilities {
		if cap == capability {
			return true
		}

		// Check for wildcard capabilities (e.g., "compute" matches "compute:ml", "compute:gpu")
		if len(cap) > len(capability) &&
			cap[:len(capability)] == capability &&
			cap[len(capability)] == ':' {
			return true
		}
	}

	return false
}

// Helper: Update peer statistics
func (c *Coordinator) updatePeerStats(peerID string, latency time.Duration, success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	peer, exists := c.peers[peerID]
	if !exists {
		return
	}

	// Update latency (moving average)
	if peer.latency == 0 {
		peer.latency = latency
	} else {
		peer.latency = (peer.latency*3 + latency) / 4
	}

	// Update load factor based on success
	if !success {
		peer.loadFactor = min(peer.loadFactor+0.1, 1.0)
	} else if latency < time.Microsecond {
		peer.loadFactor = max(peer.loadFactor-0.05, 0.0)
	}

	peer.lastSeen = time.Now()

	// Update flow control
	c.flowControl.UpdateCongestion(c.epochIndex, peer.epochIndex, latency, success)
}

// PeerSelector methods

func (ps *PeerSelector) Select(peers []*PeerInfo) *PeerInfo {
	if len(peers) == 0 {
		return nil
	}

	switch ps.strategy {
	case StrategyRoundRobin:
		ps.lastSelected = (ps.lastSelected + 1) % len(peers)
		return peers[ps.lastSelected]

	case StrategyLeastLoaded:
		var selected *PeerInfo
		minLoad := float32(1.1)
		for _, peer := range peers {
			if peer.loadFactor < minLoad {
				minLoad = peer.loadFactor
				selected = peer
			}
		}
		return selected

	case StrategyLowestLatency:
		var selected *PeerInfo
		minLatency := time.Hour
		for _, peer := range peers {
			if peer.latency < minLatency || peer.latency == 0 {
				minLatency = peer.latency
				selected = peer
			}
		}
		return selected

	default:
		return peers[0]
	}
}

// Helper functions
func min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// GetPeerStats returns statistics for all peers
type PeerStats struct {
	PeerID     string
	EpochIndex uint8
	LoadFactor float32
	Latency    time.Duration
	LastSeen   time.Time
}

func (c *Coordinator) GetPeerStats() []PeerStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := make([]PeerStats, 0, len(c.peers))
	for _, peer := range c.peers {
		stats = append(stats, PeerStats{
			PeerID:     peer.supervisorID,
			EpochIndex: peer.epochIndex,
			LoadFactor: peer.loadFactor,
			Latency:    peer.latency,
			LastSeen:   peer.lastSeen,
		})
	}

	return stats
}
