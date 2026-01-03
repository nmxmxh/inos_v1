package optimization

import (
	"fmt"
	"testing"
)

func TestSchedulerScoring_BasicScoring(t *testing.T) {
	ss := NewSchedulerScoring()

	score := ss.ScoreNode(
		"node1",
		50.0,     // latency
		10,       // cost
		0.9,      // capability match
		0.8,      // reputation
		"abc123", // node geohash
		"abc456", // local geohash
	)

	if score.TotalScore <= 0 || score.TotalScore > 1 {
		t.Errorf("Expected score between 0 and 1, got %f", score.TotalScore)
	}

	t.Logf("Score breakdown: total=%f, latency=%f, cost=%f, cap=%f, rep=%f, geo=%f",
		score.TotalScore, score.LatencyScore, score.CostScore,
		score.CapabilityScore, score.ReputationScore, score.GeohashScore)
}

func TestSchedulerScoring_LatencyScoring(t *testing.T) {
	ss := NewSchedulerScoring()

	tests := []struct {
		latency       float64
		expectedRange [2]float64
	}{
		{10.0, [2]float64{0.9, 1.0}},   // Low latency = high score (1/1.05 = 0.95)
		{100.0, [2]float64{0.6, 0.7}},  // Medium latency = medium score (1/1.5 = 0.66)
		{500.0, [2]float64{0.2, 0.4}},  // High latency = low score (1/3.5 = 0.28)
		{1000.0, [2]float64{0.0, 0.1}}, // Very high latency = very low score (0.01)
	}

	for _, tt := range tests {
		score := ss.calculateLatencyScore(tt.latency)
		if score < tt.expectedRange[0] || score > tt.expectedRange[1] {
			t.Errorf("Latency %f: expected score in range [%f, %f], got %f",
				tt.latency, tt.expectedRange[0], tt.expectedRange[1], score)
		}
	}
}

func TestSchedulerScoring_CostScoring(t *testing.T) {
	ss := NewSchedulerScoring()

	tests := []struct {
		cost          uint64
		expectedRange [2]float64
	}{
		{0, [2]float64{1.0, 1.0}},    // Free = perfect score
		{10, [2]float64{0.8, 1.0}},   // Low cost = high score
		{100, [2]float64{0.4, 0.6}},  // Medium cost = medium score
		{1000, [2]float64{0.0, 0.1}}, // High cost = low score
	}

	for _, tt := range tests {
		score := ss.calculateCostScore(tt.cost)
		if score < tt.expectedRange[0] || score > tt.expectedRange[1] {
			t.Errorf("Cost %d: expected score in range [%f, %f], got %f",
				tt.cost, tt.expectedRange[0], tt.expectedRange[1], score)
		}
	}
}

func TestSchedulerScoring_GeohashProximity(t *testing.T) {
	ss := NewSchedulerScoring()

	tests := []struct {
		nodeHash      string
		localHash     string
		expectedScore float64
	}{
		{"abc123", "abc456", 0.9}, // 3 matching chars -> 39km -> score 0.9
		{"abc123", "ab9999", 0.7}, // 2 matching chars -> 156km -> score 0.7
		{"abc123", "a99999", 0.2}, // 1 matching char -> 1250km -> score 0.2
		{"abc123", "xyz789", 0.1}, // 0 matching chars -> 5000km -> score 0.1
		{"abc123", "abc123", 1.0}, // Exact match -> score 1.0
	}

	for _, tt := range tests {
		score := ss.calculateGeohashScore(tt.nodeHash, tt.localHash)
		if score != tt.expectedScore {
			t.Errorf("Geohash %s vs %s: expected %f, got %f",
				tt.nodeHash, tt.localHash, tt.expectedScore, score)
		}
	}
}

func TestSchedulerScoring_WeightAdjustment(t *testing.T) {
	ss := NewSchedulerScoring()

	// Set custom weights
	ss.SetWeights(0.5, 0.2, 0.1, 0.1, 0.1)

	// Weights should be normalized
	total := ss.latencyWeight + ss.costWeight + ss.capabilityWeight +
		ss.reputationWeight + ss.geohashWeight

	if total < 0.99 || total > 1.01 {
		t.Errorf("Expected weights to sum to 1.0, got %f", total)
	}

	if ss.latencyWeight < 0.49 || ss.latencyWeight > 0.51 {
		t.Errorf("Expected latency weight ~0.5, got %f", ss.latencyWeight)
	}
}

func TestSchedulerScoring_LatencyTracking(t *testing.T) {
	ss := NewSchedulerScoring()

	// Record latencies
	ss.RecordLatency("node1", 50.0)
	ss.RecordLatency("node1", 60.0)
	ss.RecordLatency("node1", 70.0)

	avg := ss.GetAverageLatency("node1")
	expected := 60.0

	if avg != expected {
		t.Errorf("Expected average latency %f, got %f", expected, avg)
	}
}

func TestSchedulerScoring_SuccessRateTracking(t *testing.T) {
	ss := NewSchedulerScoring()

	// Record successes and failures
	ss.RecordSuccess("node1")
	ss.RecordSuccess("node1")
	ss.RecordSuccess("node1")
	ss.RecordFailure("node1")

	rate := ss.GetSuccessRate("node1")
	expected := 0.75 // 3/4

	if rate != expected {
		t.Errorf("Expected success rate %f, got %f", expected, rate)
	}
}

func TestSchedulerScoring_UnknownNode(t *testing.T) {
	ss := NewSchedulerScoring()

	// Unknown node should have neutral success rate
	rate := ss.GetSuccessRate("unknown")
	if rate != 0.5 {
		t.Errorf("Expected neutral success rate 0.5 for unknown node, got %f", rate)
	}

	// Unknown node should have 0 average latency
	avg := ss.GetAverageLatency("unknown")
	if avg != 0 {
		t.Errorf("Expected 0 average latency for unknown node, got %f", avg)
	}
}

func TestSchedulerScoring_LatencyHistoryLimit(t *testing.T) {
	ss := NewSchedulerScoring()

	// Record more than 100 latencies
	for i := 0; i < 150; i++ {
		ss.RecordLatency("node1", float64(i))
	}

	ss.mu.RLock()
	historyLen := len(ss.nodeLatencies["node1"])
	ss.mu.RUnlock()

	if historyLen > 100 {
		t.Errorf("Expected latency history limited to 100, got %d", historyLen)
	}
}

func TestSchedulerScoring_CompareNodes(t *testing.T) {
	ss := NewSchedulerScoring()

	// Node 1: Low latency, high cost
	score1 := ss.ScoreNode("node1", 10.0, 100, 0.8, 0.7, "abc123", "abc456")

	// Node 2: High latency, low cost
	score2 := ss.ScoreNode("node2", 200.0, 10, 0.8, 0.7, "abc123", "abc456")

	// Node 3: Balanced
	score3 := ss.ScoreNode("node3", 50.0, 50, 0.8, 0.7, "abc123", "abc456")

	t.Logf("Node 1 (low latency, high cost): %f", score1.TotalScore)
	t.Logf("Node 2 (high latency, low cost): %f", score2.TotalScore)
	t.Logf("Node 3 (balanced): %f", score3.TotalScore)

	// With default weights (latency=0.3, cost=0.2), low latency should win
	if score1.TotalScore < score2.TotalScore {
		t.Error("Expected low latency node to score higher with default weights")
	}
}

func TestSchedulerScoring_Metrics(t *testing.T) {
	ss := NewSchedulerScoring()

	ss.RecordLatency("node1", 50.0)
	ss.RecordLatency("node2", 60.0)

	metrics := ss.GetMetrics()

	if metrics["tracked_nodes"] != 2 {
		t.Errorf("Expected 2 tracked nodes, got %v", metrics["tracked_nodes"])
	}
}

func TestSchedulerScoring_ConcurrentAccess(t *testing.T) {
	ss := NewSchedulerScoring()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			nodeID := fmt.Sprintf("node%d", id%3)
			for j := 0; j < 100; j++ {
				ss.RecordLatency(nodeID, float64(j))
				ss.RecordSuccess(nodeID)
				ss.GetAverageLatency(nodeID)
				ss.GetSuccessRate(nodeID)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic
	metrics := ss.GetMetrics()
	if metrics["tracked_nodes"].(int) != 3 {
		t.Errorf("Expected 3 tracked nodes, got %v", metrics["tracked_nodes"])
	}
}

func BenchmarkSchedulerScoring_ScoreNode(b *testing.B) {
	ss := NewSchedulerScoring()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ss.ScoreNode("node1", 50.0, 10, 0.9, 0.8, "abc123", "abc456")
	}
}

func BenchmarkSchedulerScoring_RecordLatency(b *testing.B) {
	ss := NewSchedulerScoring()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ss.RecordLatency("node1", float64(i%1000))
	}
}

func BenchmarkSchedulerScoring_ConcurrentScoring(b *testing.B) {
	ss := NewSchedulerScoring()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			nodeID := fmt.Sprintf("node%d", i%10)
			ss.ScoreNode(nodeID, float64(i%500), uint64(i%100), 0.8, 0.7, "abc123", "abc456")
			i++
		}
	})
}
