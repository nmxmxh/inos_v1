package units

import (
	"context"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
)

// ScienceSupervisor supervises the science module
type ScienceSupervisor struct {
	*supervisor.UnifiedSupervisor

	// SAB bridge for WASM communication
	bridge *supervisor.SABBridge
}

// NewScienceSupervisor creates a new science supervisor
func NewScienceSupervisor(bridge *supervisor.SABBridge, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string) *ScienceSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{"science.experiment", "science.simulate", "science.calculate", "science.analyze", "science.predict"}
	}

	return &ScienceSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("science", capabilities, patterns, knowledge),
		bridge:            bridge,
	}
}

// Start starts the science supervisor and its children
func (ss *ScienceSupervisor) Start(ctx context.Context) error {
	return ss.UnifiedSupervisor.Start(ctx)
}

// ExecuteJob override for science-specific workflows
func (ss *ScienceSupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	// 1. Dispatch to Physics for simulation
	// 2. Dispatch to Storage for results
	// 3. Dispatch to Data for analysis
	return ss.UnifiedSupervisor.ExecuteJob(job)
}
