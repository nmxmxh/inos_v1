package transport

import (
	"encoding/json"
	"errors"
	"sync"
)

// GossipSignalingChannel implements SignalingChannel using the Gossip mesh.
// This allows every node to act as a signaling server for others.
type GossipSignalingChannel struct {
	nodeID    string
	messages  chan []byte
	broadcast func(topic string, payload interface{}) error
	closed    chan struct{}
	closeOnce sync.Once
}

// NewGossipSignalingChannel creates a new Gossip-based signaling channel.
func NewGossipSignalingChannel(nodeID string, broadcast func(topic string, payload interface{}) error) *GossipSignalingChannel {
	return &GossipSignalingChannel{
		nodeID:    nodeID,
		messages:  make(chan []byte, 200),
		broadcast: broadcast,
		closed:    make(chan struct{}),
	}
}

// Send broadcasts a signaling message to the mesh.
func (g *GossipSignalingChannel) Send(message interface{}) error {
	select {
	case <-g.closed:
		return errors.New("channel closed")
	default:
		return g.broadcast("webrtc.signaling", message)
	}
}

// Receive returns a message received from the mesh.
func (g *GossipSignalingChannel) Receive() ([]byte, error) {
	select {
	case msg := <-g.messages:
		return msg, nil
	case <-g.closed:
		return nil, errors.New("channel closed")
	}
}

// Close shuts down the channel.
func (g *GossipSignalingChannel) Close() error {
	g.closeOnce.Do(func() {
		close(g.closed)
	})
	return nil
}

// IsConnected returns true as long as the mesh broadcast function is available.
func (g *GossipSignalingChannel) IsConnected() bool {
	select {
	case <-g.closed:
		return false
	default:
		return true
	}
}

// HandleIncoming is called by the MeshCoordinator when a webrtc.signaling gossip message is received.
func (g *GossipSignalingChannel) HandleIncoming(payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	select {
	case g.messages <- data:
	case <-g.closed:
	default:
		// Channel full, drop message
	}
}
