package internal

import (
	"fmt"
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
)

// TestAdaptiveAllocator tests replica calculation and distribution
func TestAdaptiveAllocator(t *testing.T) {
	aa := NewAdaptiveAllocator(5, 700, 0.375, 0.5)

	tests := []struct {
		name     string
		resource common.Resource
		expected int
	}{
		{"SmallFile", common.Resource{Size: 500 * KB, DemandScore: 0.0, CreditBudget: 100}, 5},
		{"MediumFile", common.Resource{Size: 5 * MB, DemandScore: 0.0, CreditBudget: 100}, 10},
		{"LargeFileHighDemand", common.Resource{Size: 50 * MB, DemandScore: 1.0, CreditBudget: 100}, 61}, // (15 + 15)*2*1
		{"HugeFile", common.Resource{Size: 5 * GB, DemandScore: 0.5, CreditBudget: 1000}, 457},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replicas := aa.CalculateReplicas(tt.resource)
			if replicas < aa.minReplicas || replicas > aa.maxReplicas {
				t.Errorf("Replicas %d out of bounds [%d, %d]", replicas, aa.minReplicas, aa.maxReplicas)
			}
		})
	}

	// Test Chunk Distribution
	dist := aa.CalculateChunkDistribution(10 * MB)
	if dist.NumChunks != 10 {
		t.Errorf("Expected 10 chunks, got %d", dist.NumChunks)
	}

	distHuge := aa.CalculateChunkDistribution(20 * GB)
	if distHuge.ChunkSize != 4*MB {
		t.Errorf("Expected 4MB chunk size for huge file, got %d", distHuge.ChunkSize)
	}
}

// TestChunkCache tests LRU and TTL logic
func TestChunkCache(t *testing.T) {
	cache := NewChunkCache(3, 100*time.Millisecond)

	// Test Put and Get
	cache.Put("c1", []string{"p1"}, 0.5)
	cache.Put("c2", []string{"p1", "p2"}, 0.8)
	cache.Put("c3", []string{"p3"}, 0.9)

	if m, ok := cache.Get("c1"); !ok || m.Confidence != 0.5 {
		t.Error("Failed to get c1")
	}

	// Test LRU Eviction
	cache.Put("c4", []string{"p4"}, 0.7) // Should evict c2 (c1 was just touched)
	if _, ok := cache.Get("c2"); ok {
		t.Error("c2 should have been evicted")
	}

	// Test TTL Expiration
	time.Sleep(150 * time.Millisecond)
	if _, ok := cache.Get("c1"); ok {
		t.Error("c1 should have expired")
	}

	// Test AddPeer
	cache.AddPeer("c5", "p5")
	if m, ok := cache.Get("c5"); !ok || len(m.PeerIDs) != 1 {
		t.Error("Failed to add peer to new mapping")
	}
	cache.AddPeer("c5", "p6")
	if m, ok := cache.Get("c5"); !ok || len(m.PeerIDs) != 2 {
		t.Error("Failed to add second peer")
	}

	// Test CleanupExpired
	removed := cache.CleanupExpired()
	if removed == 0 {
		// c1-c4 should have been removed by Get or CleanupExpired
	}

	// Test Clear
	cache.Clear()
	if metrics := cache.GetMetrics(); metrics.Size != 0 {
		t.Errorf("Expected size 0 after clear, got %d", metrics.Size)
	}
}

// TestDemandTracker tests access tracking and decay
func TestDemandTracker(t *testing.T) {
	dt := NewDemandTracker()
	dt.decayInterval = 50 * time.Millisecond

	// Record accesses
	for i := 0; i < 10; i++ {
		dt.RecordAccess("chunk1")
	}

	score := dt.GetDemandScore("chunk1")
	if score != 0.5 { // 6-20 accesses = 0.5
		t.Errorf("Expected score 0.5, got %f", score)
	}

	// Test Recency Factor
	dt.mu.Lock()
	dt.accessCounts["chunk1"].LastAccess = time.Now().Add(-2 * time.Hour)
	dt.mu.Unlock()

	scoreLong := dt.GetDemandScore("chunk1")
	if scoreLong >= score {
		t.Errorf("Score should decrease with time, got %f", scoreLong)
	}

	// Test Decay
	time.Sleep(100 * time.Millisecond)
	dt.RecordAccess("chunk2") // This should trigger decayAll

	scoreDecayed := dt.GetDemandScore("chunk1")
	// RecentAccesses 10 -> 5. Score for 5 is 0.2
	if scoreDecayed > 0.3 {
		t.Errorf("Score should have decayed, got %f", scoreDecayed)
	}

	// Test GetStats
	stats := dt.GetStats()
	if stats["total_tracked"].(int) == 0 {
		t.Error("Expected total_tracked > 0")
	}

	// Test Cleanup
	removed := dt.Cleanup(0) // Remove everything
	if removed == 0 {
		t.Error("Expected entries to be removed during Cleanup")
	}
}

// TestMeshError tests error creation and wrapping
func TestMeshError(t *testing.T) {
	cause := fmt.Errorf("underlying error")
	err := ErrPeerUnreachable("peer1", cause)

	if err.Code != ErrCodePeerUnreachable {
		t.Errorf("Expected code %s, got %s", ErrCodePeerUnreachable, err.Code)
	}

	if err.Context["peer_id"] != "peer1" {
		t.Errorf("Expected peer_id peer1 in context")
	}

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Error("Failed to unwrap cause")
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Empty error message")
	}

	// Test remaining constructors
	_ = ErrChunkNotFound("c1")
	_ = ErrCircuitOpen("p1")
	_ = ErrTimeout("op", "1s")
	_ = ErrInsufficientPeers(3, 1)
	_ = ErrSignatureInvalid("m1")
	_ = ErrLowReputation("p1", 0.1, 0.5)
}
