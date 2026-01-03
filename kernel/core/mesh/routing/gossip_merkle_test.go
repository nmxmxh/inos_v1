package routing

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
	"github.com/stretchr/testify/assert"
)

func TestGossipManager_MerkleSync(t *testing.T) {
	transport := NewMockDHTTransport()
	gossip, err := NewGossipManager("node1", transport, nil)
	assert.NoError(t, err)

	// Set short intervals
	gossip.config.RoundInterval = 10 * time.Millisecond
	gossip.config.AntiEntropyInterval = 50 * time.Millisecond

	// We must start it for some functions to work
	err = gossip.Start()
	assert.NoError(t, err)
	defer gossip.Stop()

	// Add a peer so broadcast has someone to send to
	gossip.UpdatePeers([]string{"peer2"})

	// 1. Populate the tree so root is not empty
	err = gossip.AnnounceChunk("chunk-initial")
	assert.NoError(t, err)

	gossip.stateMu.RLock()
	assert.NotEmpty(t, gossip.state.Root)
	initialRoot := gossip.state.Root
	gossip.stateMu.RUnlock()

	// 2. Test announceMerkleRoot
	gossip.announceMerkleRoot()

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	// Verify transport messages
	transport.mu.RLock()
	foundSync := false
	for _, call := range transport.calls {
		if call != "" {
			foundSync = true
		}
	}
	transport.mu.RUnlock()
	assert.True(t, foundSync, "Should have sent at least one gossip message")

	// 3. Test receiving merkle.sync
	theirRoot := []byte("different-root")
	msg := &common.GossipMessage{
		ID:        "sync-1",
		Type:      "merkle.sync",
		Sender:    "peer2",
		Timestamp: time.Now().UnixNano(),
		HopCount:  0,
		MaxHops:   10,
		Payload: map[string]interface{}{
			"root": base64.StdEncoding.EncodeToString(theirRoot),
		},
	}
	gossip.signMessage(msg)

	err = gossip.ReceiveMessage("peer2", msg)
	assert.NoError(t, err)

	// 4. Test AnnounceChunk (should update Merkle root)
	err = gossip.AnnounceChunk("chunk-123")
	assert.NoError(t, err)

	gossip.stateMu.RLock()
	newRoot := gossip.state.Root
	gossip.stateMu.RUnlock()
	assert.NotEqual(t, initialRoot, newRoot, "Root should change after chunk announcement")
}

func TestGossipManager_HealthAndMetrics(t *testing.T) {
	transport := NewMockDHTTransport()
	gossip, err := NewGossipManager("node1", transport, nil)
	assert.NoError(t, err)

	// Test GetHealthScore
	score := gossip.GetHealthScore()
	assert.True(t, score >= 0 && score <= 1.0)

	// Test cleanupOldMessages
	gossip.markSeen("msg-to-be-cleaned")
	gossip.cleanupOldMessages()
}
