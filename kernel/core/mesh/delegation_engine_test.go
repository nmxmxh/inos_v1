package mesh

import (
	"context"
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSystemLoadProvider implements SystemLoadProvider for testing
type mockSystemLoadProvider struct {
	load float64
}

func (m *mockSystemLoadProvider) GetSystemLoad() float64 {
	return m.load
}

// ========== DelegationEngine Tests ==========

func TestNewDelegationEngine(t *testing.T) {
	provider := &mockSystemLoadProvider{load: 0.5}
	engine := NewDelegationEngine(provider)

	require.NotNil(t, engine)
	assert.NotNil(t, engine.loadProvider)
	assert.Equal(t, 50.0, engine.networkLatency) // Default 50ms
}

func TestDelegationEngine_Analyze_LowLoad(t *testing.T) {
	provider := &mockSystemLoadProvider{load: 0.2}
	engine := NewDelegationEngine(provider)

	job := &foundation.Job{
		ID:        "test-job-1",
		Operation: "compress",
		Data:      make([]byte, 1024), // 1KB
		Priority:  100,
	}

	decision := engine.Analyze(context.Background(), job)

	// Low load contributes to lower efficiency score
	// With transferEfficiency ~1.0 (small data), computeSpeedup 0.2, energyEfficiency 0.5, priorityFactor 1.0
	// Expected efficiency = 0.4*1.0 + 0.3*0.2 + 0.2*0.5 + 0.1*1.0 = 0.4+0.06+0.1+0.1 = 0.66
	// This is below 0.7 threshold, so ShouldDelegate should be false
	assert.False(t, decision.ShouldDelegate, "Low load should not trigger delegation")
	// selectTargetType depends on efficiency and network latency (default 50ms)
	// efficiency 0.66 >= 0.3, and networkLatency 50 >= 10, so TargetMeshRemote
	assert.Equal(t, TargetMeshRemote, decision.TargetType)
}

func TestDelegationEngine_Analyze_HighLoad(t *testing.T) {
	provider := &mockSystemLoadProvider{load: 0.9}
	engine := NewDelegationEngine(provider)

	job := &foundation.Job{
		ID:        "test-job-2",
		Operation: "hash",
		Data:      make([]byte, 1024*1024), // 1MB
		Priority:  50,
	}

	decision := engine.Analyze(context.Background(), job)

	// High load should favor delegation
	assert.True(t, decision.ShouldDelegate, "High load should trigger delegation")
	assert.Greater(t, decision.EfficiencyScore, 0.6)
}

func TestDelegationEngine_Analyze_HighPriority(t *testing.T) {
	provider := &mockSystemLoadProvider{load: 0.8}
	engine := NewDelegationEngine(provider)

	// High priority jobs should stay local unless absolutely necessary
	job := &foundation.Job{
		ID:        "critical-job",
		Operation: "encrypt",
		Data:      make([]byte, 512),
		Priority:  250, // High priority
	}

	decision := engine.Analyze(context.Background(), job)

	// High priority should require higher reputation peers
	assert.Equal(t, float32(0.9), decision.PeerScoreThreshold)
}

func TestDelegationEngine_UpdateMetrics(t *testing.T) {
	provider := &mockSystemLoadProvider{load: 0.5}
	engine := NewDelegationEngine(provider)

	// Initial state
	assert.Equal(t, 50.0, engine.networkLatency)
	assert.Equal(t, 0.0, engine.localLoad)

	// Update metrics
	engine.UpdateMetrics(0.8, 30.0)

	// Should apply EMA (alpha=0.2)
	// localLoad = (1-0.2)*0 + 0.2*0.8 = 0.16
	// networkLatency = (1-0.2)*50 + 0.2*30 = 40 + 6 = 46
	assert.InDelta(t, 0.16, engine.localLoad, 0.01)
	assert.InDelta(t, 46.0, engine.networkLatency, 0.01)

	// Update again
	engine.UpdateMetrics(0.9, 20.0)

	// localLoad = (1-0.2)*0.16 + 0.2*0.9 = 0.128 + 0.18 = 0.308
	// networkLatency = (1-0.2)*46 + 0.2*20 = 36.8 + 4 = 40.8
	assert.InDelta(t, 0.308, engine.localLoad, 0.01)
	assert.InDelta(t, 40.8, engine.networkLatency, 0.01)
}

func TestDelegationEngine_predictEfficiency(t *testing.T) {
	tests := []struct {
		name          string
		load          float64
		dataSize      int
		priority      int
		minEfficiency float64
		maxEfficiency float64
	}{
		{
			name:          "Small data, low load",
			load:          0.2,
			dataSize:      1024,
			priority:      50,
			minEfficiency: 0.4,
			maxEfficiency: 0.7,
		},
		{
			name:          "Large data, high load",
			load:          0.9,
			dataSize:      10 * 1024 * 1024, // 10MB
			priority:      50,
			minEfficiency: 0.6,
			maxEfficiency: 1.0,
		},
		{
			name:          "High priority, high load",
			load:          0.9,
			dataSize:      1024,
			priority:      250,
			minEfficiency: 0.5,
			maxEfficiency: 0.9, // Priority factor 0.2 still leaves high efficiency due to high load
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockSystemLoadProvider{load: tc.load}
			engine := NewDelegationEngine(provider)

			job := &foundation.Job{
				ID:       "test",
				Data:     make([]byte, tc.dataSize),
				Priority: tc.priority,
			}

			efficiency := engine.predictEfficiency(job)
			assert.GreaterOrEqual(t, efficiency, tc.minEfficiency)
			assert.LessOrEqual(t, efficiency, tc.maxEfficiency)
		})
	}
}

func TestDelegationEngine_selectTargetType(t *testing.T) {
	provider := &mockSystemLoadProvider{load: 0.5}
	engine := NewDelegationEngine(provider)

	job := &foundation.Job{ID: "test"}

	// Low efficiency -> local
	target := engine.selectTargetType(job, 0.2)
	assert.Equal(t, TargetLocal, target)

	// Medium efficiency with high latency -> remote
	engine.networkLatency = 100.0
	target = engine.selectTargetType(job, 0.5)
	assert.Equal(t, TargetMeshRemote, target)

	// Medium efficiency with low latency -> local mesh
	engine.networkLatency = 5.0
	target = engine.selectTargetType(job, 0.5)
	assert.Equal(t, TargetMeshLocal, target)
}

func TestDelegationEngine_calculateMinScore(t *testing.T) {
	provider := &mockSystemLoadProvider{load: 0.5}
	engine := NewDelegationEngine(provider)

	// Normal priority
	job := &foundation.Job{ID: "test", Priority: 100}
	score := engine.calculateMinScore(job)
	assert.Equal(t, float32(0.6), score)

	// High priority
	job.Priority = 250
	score = engine.calculateMinScore(job)
	assert.Equal(t, float32(0.9), score)
}

func TestDelegationEngine_ConcurrentAccess(t *testing.T) {
	provider := &mockSystemLoadProvider{load: 0.5}
	engine := NewDelegationEngine(provider)

	// Concurrent metric updates and analyses
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			engine.UpdateMetrics(float64(i%10)/10.0, float64(20+i%30))
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			job := &foundation.Job{
				ID:       "concurrent-test",
				Data:     make([]byte, 1024),
				Priority: 50,
			}
			_ = engine.Analyze(context.Background(), job)
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Wait for both
	<-done
	<-done

	// No panic means success
	assert.True(t, true)
}
