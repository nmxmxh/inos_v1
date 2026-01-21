package common

import (
	"testing"
	"time"

	capnp "zombiezen.com/go/capnproto2"
)

// TestPeerCapability tests validation and helper methods
func TestPeerCapability(t *testing.T) {
	cap := &PeerCapability{
		PeerID:          "peer1",
		BandwidthKbps:   1000,
		LatencyMs:       50,
		Reputation:      0.8,
		Capabilities:    []string{"gpu", "storage"},
		LastSeen:        time.Now().UnixNano(),
		ConnectionState: ConnectionStateConnected,
	}

	// Test Validate
	if err := cap.Validate(); err != nil {
		t.Errorf("Validation failed for valid capability: %v", err)
	}

	invalid := &PeerCapability{PeerID: ""}
	if err := invalid.Validate(); err == nil {
		t.Error("Expected error for empty PeerID")
	}

	invalidRep := &PeerCapability{PeerID: "p", Reputation: 1.5}
	if err := invalidRep.Validate(); err == nil {
		t.Error("Expected error for invalid reputation")
	}

	// Test HasCapability
	if !cap.HasCapability("gpu") {
		t.Error("Expected HasCapability(gpu) to be true")
	}
	if cap.HasCapability("cpu") {
		t.Error("Expected HasCapability(cpu) to be false")
	}

	// Test IsOnline
	if !cap.IsOnline() {
		t.Error("Expected IsOnline to be true for recently seen connected peer")
	}

	cap.ConnectionState = ConnectionStateDisconnected
	if cap.IsOnline() {
		t.Error("Expected IsOnline to be false for disconnected peer")
	}
}

// TestModelMetadata tests validation logic
func TestModelMetadata(t *testing.T) {
	meta := &ModelMetadata{
		ModelID:     "model1",
		TotalChunks: 100,
		TotalSize:   100 * 1024 * 1024,
		LayerChunks: []LayerChunkMapping{
			{LayerIndex: 0, ChunkIndices: []uint32{0, 1, 2}},
		},
	}

	if err := meta.Validate(); err != nil {
		t.Errorf("Validation failed for valid metadata: %v", err)
	}

	invalidChunks := &ModelMetadata{
		ModelID:     "model1",
		TotalChunks: 2,
		LayerChunks: []LayerChunkMapping{
			{LayerIndex: 0, ChunkIndices: []uint32{0, 1, 5}}, // Index 5 > TotalChunks 2
		},
	}
	if err := invalidChunks.Validate(); err == nil {
		t.Error("Expected error for out-of-bounds chunk index")
	}
}

// TestConnectionStateString tests string representation
func TestConnectionStateString(t *testing.T) {
	tests := []struct {
		state    ConnectionState
		expected string
	}{
		{ConnectionStateDisconnected, "disconnected"},
		{ConnectionStateConnected, "connected"},
		{ConnectionStateConnecting, "connecting"},
		{ConnectionStateDegraded, "degraded"},
		{ConnectionStateFailed, "failed"},
		{ConnectionState(99), "unknown(99)"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, tt.state.String())
		}
	}
}

// TestGossipMessageStruct runs a basic sanity check on GossipMessage initialization
func TestGossipMessageStruct(t *testing.T) {
	msg := &GossipMessage{
		ID:       "m1",
		Type:     "t1",
		Sender:   "p1",
		Payload:  "data",
		HopCount: 1,
		MaxHops:  10,
	}

	if msg.ID != "m1" || msg.Type != "t1" || msg.Sender != "p1" || msg.Payload != "data" || msg.HopCount != 1 || msg.MaxHops != 10 {
		t.Errorf("Failed to initialize GossipMessage correctly: %+v", msg)
	}
}

// TestCapnpConversions tests PeerCapability and Envelope conversions
func TestCapnpConversions(t *testing.T) {
	// 1. PeerCapability Conversion
	cap := &PeerCapability{
		PeerID:          "peer1",
		BandwidthKbps:   1000.5,
		LatencyMs:       50.2,
		Reputation:      0.85,
		AvailableChunks: []string{"chunk1", "chunk2"},
		Capabilities:    []string{"gpu", "tpu"},
		LastSeen:        123456789,
		Region:          "us-east-1",
	}

	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		t.Fatalf("Failed to create capnp message: %v", err)
	}

	c, err := cap.ToCapnp(seg)
	if err != nil {
		t.Fatalf("ToCapnp failed: %v", err)
	}

	// Round trip
	cap2 := &PeerCapability{}
	if err := cap2.FromCapnp(c); err != nil {
		t.Fatalf("FromCapnp failed: %v", err)
	}

	if cap2.PeerID != cap.PeerID || cap2.Reputation != cap.Reputation {
		t.Errorf("Round trip failed: expected %+v, got %+v", cap, cap2)
	}

	// 2. Envelope Conversion
	env := &Envelope{
		ID:        "env1",
		Type:      "type1",
		Timestamp: 123456,
		Version:   "1.0",
		Payload:   []byte("test_data"),
		Metadata: EnvelopeMetadata{
			UserID:   "user1",
			DeviceID: "device1",
		},
	}

	_, seg2, _ := capnp.NewMessage(capnp.SingleSegment(nil))
	e, err := env.ToCapnp(seg2)
	if err != nil {
		t.Fatalf("Envelope ToCapnp failed: %v", err)
	}

	env2 := &Envelope{}
	if err := env2.FromCapnp(e); err != nil {
		t.Fatalf("Envelope FromCapnp failed: %v", err)
	}

	if env2.ID != env.ID || string(env2.Payload) != string(env.Payload) {
		t.Errorf("Envelope round trip failed")
	}

	_ = msg
}
