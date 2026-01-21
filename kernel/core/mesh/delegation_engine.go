package mesh

import (
	"context"
	"sync"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
)

// DelegationDecision represents the result of the delegation engine's analysis
type DelegationDecision struct {
	ShouldDelegate     bool
	TargetType         DelegationTargetType
	PeerScoreThreshold float32
	FallbackTimeout    time.Duration
	EstimatedCost      float64
	EfficiencyScore    float64
}

type DelegationTargetType int

const (
	TargetLocal DelegationTargetType = iota
	TargetMeshLocal
	TargetMeshRemote
	TargetDedicatedHW
)

// SystemLoadProvider provides current system load metrics
type SystemLoadProvider interface {
	GetSystemLoad() float64
}

// DelegationEngine handles AI-driven decision making for offloading tasks
type DelegationEngine struct {
	loadProvider SystemLoadProvider
	// TODO: Add CostModel and LoadPredictor when available in kernel

	networkLatency float64 // Rolling average of mesh latency
	localLoad      float64 // Current system load (0-1)
	mu             sync.RWMutex
}

// NewDelegationEngine creates a new delegation engine
func NewDelegationEngine(loadProvider SystemLoadProvider) *DelegationEngine {
	return &DelegationEngine{
		loadProvider:   loadProvider,
		networkLatency: 50.0, // Default 50ms
	}
}

// Analyze evaluates whether a job should be delegated
func (de *DelegationEngine) Analyze(ctx context.Context, job *foundation.Job) DelegationDecision {
	de.mu.RLock()
	defer de.mu.RUnlock()

	efficiency := de.predictEfficiency(job)

	return DelegationDecision{
		ShouldDelegate:     efficiency > 0.7,
		TargetType:         de.selectTargetType(job, efficiency),
		PeerScoreThreshold: de.calculateMinScore(job),
		FallbackTimeout:    500 * time.Millisecond,
		EfficiencyScore:    efficiency,
	}
}

// predictEfficiency uses multi-factor analysis to estimate delegation benefit
func (de *DelegationEngine) predictEfficiency(job *foundation.Job) float64 {
	// 1. Data Transfer Efficiency (Size vs Latency)
	dataSize := float64(len(job.Data))
	transferCost := dataSize / (1024 * 1024) * de.networkLatency // Simplified cost model
	transferEfficiency := 1.0 / (1.0 + 0.001*transferCost)

	// 2. Compute Speedup (Local Load vs Remote Potential)
	// If local load is high, speedup potential is high
	computeSpeedup := de.localLoad
	if de.loadProvider != nil {
		computeSpeedup = de.loadProvider.GetSystemLoad()
	}

	// 3. TODO: Resource Intensity (Battery/Thermal aware)
	// For now, use a constant placeholder
	energyEfficiency := 0.5

	// 4. Opportunity Cost (Local Task Priority)
	// High priority tasks should stay local unless load is critical
	priorityFactor := 1.0
	if job.Priority > 200 {
		priorityFactor = 0.2
	}

	// Weighted average
	return (transferEfficiency * 0.4) + (computeSpeedup * 0.3) + (energyEfficiency * 0.2) + (priorityFactor * 0.1)
}

func (de *DelegationEngine) selectTargetType(_ *foundation.Job, efficiency float64) DelegationTargetType {
	if efficiency < 0.3 {
		return TargetLocal
	}
	if de.networkLatency < 10 {
		return TargetMeshLocal
	}
	return TargetMeshRemote
}

func (de *DelegationEngine) calculateMinScore(job *foundation.Job) float32 {
	if job.Priority > 200 {
		return 0.9 // High priority needs highly reputable peers
	}
	return 0.6
}

// UpdateMetrics updates the engine's internal state for decision making
func (de *DelegationEngine) UpdateMetrics(load float64, latency float64) {
	de.mu.Lock()
	defer de.mu.Unlock()

	// EMA for metrics
	alpha := 0.2
	de.localLoad = (1-alpha)*de.localLoad + alpha*load
	de.networkLatency = (1-alpha)*de.networkLatency + alpha*latency
}
