package units

import (
	"context"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
)

// PhysicsSupervisor supervises the physics module (Composite)
type PhysicsSupervisor struct {
	*supervisor.UnifiedSupervisor

	// SAB bridge for WASM communication
	bridge *supervisor.SABBridge
}

// NewPhysicsSupervisor creates a new physics supervisor
func NewPhysicsSupervisor(bridge *supervisor.SABBridge, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string) *PhysicsSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{"physics.simulate", "physics.collision", "physics.particles", "physics.fluid", "physics.cloth"}
	}

	ps := &PhysicsSupervisor{
		bridge: bridge,
	}
	ps.UnifiedSupervisor = supervisor.NewUnifiedSupervisor("physics", capabilities, patterns, knowledge)

	return ps
}

// Start starts the physics supervisor and its children
func (ps *PhysicsSupervisor) Start(ctx context.Context) error {
	return ps.UnifiedSupervisor.Start(ctx)
}

// ExecuteJob override for physics-specific dispatching
func (ps *PhysicsSupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	// 1. Dispatch to GPU if parallelizable
	// 2. Dispatch to storage for persistence
	// 3. Dispatch to data for post-processing
	return ps.UnifiedSupervisor.ExecuteJob(job)
}
