package routing

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"
)

// MockStore for Reputation testing
type MockReputationStore struct {
	mu     sync.RWMutex
	scores map[string]ReputationScore
}

func (m *MockReputationStore) SaveScores(scores map[string]ReputationScore) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scores = make(map[string]ReputationScore)
	for k, v := range scores {
		m.scores[k] = v
	}
	return nil
}

func (m *MockReputationStore) LoadScores() (map[string]ReputationScore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.scores == nil {
		return make(map[string]ReputationScore), nil
	}
	res := make(map[string]ReputationScore)
	for k, v := range m.scores {
		res[k] = v
	}
	return res, nil
}

// TestReputation_NewManager tests reputation manager initialization
func TestReputation_NewManager(t *testing.T) {
	rm := NewReputationManager(24*time.Hour, nil, nil)

	if rm == nil {
		t.Fatal("NewReputationManager returned nil")
	}

	if rm.decayHalfLife != 24*time.Hour {
		t.Errorf("Expected half-life 24h, got %v", rm.decayHalfLife)
	}
}

// TestReputation_Report tests basic reporting
func TestReputation_Report(t *testing.T) {
	rm := NewReputationManager(24*time.Hour, nil, nil)
	peerID := "peer1"

	// Initial score should be default (0.5)
	score, confidence := rm.GetTrustScore(peerID)
	if score != 0.5 || confidence != 0.0 {
		t.Errorf("Expected initial score 0.5/0.0, got %f/%f", score, confidence)
	}

	// Report multiple successes to overcome confidence threshold (total/2+1)
	for i := 0; i < 5; i++ {
		rm.Report(peerID, true, 50.0) // 50ms is perfect latency
	}
	score, confidence = rm.GetTrustScore(peerID)

	if score <= 0.5 {
		t.Errorf("Expected score to increase after success, got %f", score)
	}
	if confidence <= 0.0 {
		t.Errorf("Expected confidence to increase after multiple interactions, got %f", confidence)
	}
}

// TestReputation_ReportPenalty tests specific penalties
func TestReputation_ReportPenalty(t *testing.T) {
	rm := NewReputationManager(24*time.Hour, nil, nil)
	peerID := "peer1"

	rm.Report(peerID, true, 10.0)
	scoreBefore, _ := rm.GetTrustScore(peerID)

	rm.ReportPenalty(peerID, PenaltyTimeout)
	scoreAfter, _ := rm.GetTrustScore(peerID)

	if scoreAfter >= scoreBefore {
		t.Errorf("Expected score to decrease after penalty, got %f (was %f)", scoreAfter, scoreBefore)
	}
}

// TestReputation_PoRReport tests Proof of Retrievability reporting
func TestReputation_PoRReport(t *testing.T) {
	rm := NewReputationManager(24*time.Hour, nil, nil)
	peerID := "peer1"

	rm.PoRReport(peerID, true, 2.0) // difficulty 2.0
	score, _ := rm.GetTrustScore(peerID)

	if score <= 0.5 {
		t.Errorf("Expected high boost from successful PoR, got %f", score)
	}
}

// TestReputation_Decay tests score decay over time
func TestReputation_Decay(t *testing.T) {
	// Very short half-life for testing
	rm := NewReputationManager(100*time.Millisecond, nil, nil)
	peerID := "peer1"

	rm.Report(peerID, true, 10.0)
	scoreBefore, _ := rm.GetTrustScore(peerID)

	// Wait for decay
	time.Sleep(250 * time.Millisecond)

	scoreAfter, _ := rm.GetTrustScore(peerID)
	if scoreAfter >= scoreBefore {
		t.Errorf("Expected score to decay toward 0.5, got %f (was %f)", scoreAfter, scoreBefore)
	}
}

// TestReputation_ConcurrentOperations tests concurrent access
func TestReputation_ConcurrentOperations(t *testing.T) {
	rm := NewReputationManager(24*time.Hour, nil, nil)
	numGoroutines := 20
	numReports := 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numReports; j++ {
				rm.Report(fmt.Sprintf("peer%d", id), true, 10.0)
				_, _ = rm.GetTrustScore(fmt.Sprintf("peer%d", id))
			}
		}(i)
	}
	wg.Wait()

	metrics := rm.GetMetrics()
	if metrics["total_peers"] != numGoroutines {
		t.Errorf("Expected %d peers in metrics, got %v", numGoroutines, metrics["total_peers"])
	}
}

// TestReputation_IsTrusted tests trust logic
func TestReputation_IsTrusted(t *testing.T) {
	rm := NewReputationManager(24*time.Hour, nil, nil)
	peerID := "peer1"

	if rm.IsTrusted(peerID) {
		t.Error("New peer should not be trusted initially")
	}

	// Build trust
	for i := 0; i < 10; i++ {
		rm.Report(peerID, true, 10.0)
	}

	if !rm.IsTrusted(peerID) {
		t.Errorf("Peer should be trusted after 10 successes, score/conf: %v", fmt.Sprint(rm.GetTrustScore(peerID)))
	}
}

// TestReputation_Persistence tests Snapshot and loading
func TestReputation_Persistence(t *testing.T) {
	store := &MockReputationStore{}
	rm := NewReputationManager(24*time.Hour, store, nil)
	peerID := "peer1"

	rm.Report(peerID, true, 10.0)
	err := rm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Create new manager with same store
	rm2 := NewReputationManager(24*time.Hour, store, nil)
	score1, _ := rm.GetTrustScore(peerID)
	score2, _ := rm2.GetTrustScore(peerID)

	if math.Abs(score1-score2) > 0.0001 {
		t.Errorf("Loaded score %f mismatch with saved score %f", score2, score1)
	}
}

// Benchmarks

func BenchmarkReputation_Report(b *testing.B) {
	rm := NewReputationManager(24*time.Hour, nil, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.Report("peer1", true, 10.0)
	}
}

func BenchmarkReputation_GetTrustScore(b *testing.B) {
	rm := NewReputationManager(24*time.Hour, nil, nil)
	rm.Report("peer1", true, 10.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rm.GetTrustScore("peer1")
	}
}

func TestReputation_Metrics(t *testing.T) {
	mgr := NewReputationManager(24*time.Hour, nil, nil)

	mgr.Report("peer1", true, 1.0)
	mgr.Report("peer2", true, 0.5)

	// Test GetAverageScore
	avg := mgr.GetAverageScore()
	if avg < 0.4 || avg > 0.6 {
		t.Errorf("Expected average score around 0.5, got %f", avg)
	}

	// Test GetTopPeers
	top := mgr.GetTopPeers(1)
	if len(top) != 1 {
		t.Errorf("Expected 1 top peer, got %d", len(top))
	} else if top[0] != "peer1" {
		t.Errorf("Expected peer1 to be top, got %s", top[0])
	}
}
