//go:build wasm

package units

import (
	"context"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
)

// GPUSupervisor supervises the GPU module
type GPUSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge supervisor.SABInterface
}

func NewGPUSupervisor(bridge supervisor.SABInterface, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string, delegator foundation.MeshDelegator) *GPUSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{
			"gpu.compute", "gpu.shader", "gpu.render", "gpu.tensor", "gpu.cuda_proxy",
			"gpu.boids", "instance_matrix_gen",
		}
	}
	return &GPUSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("gpu", capabilities, patterns, knowledge, delegator, bridge, nil),
		bridge:            bridge,
	}
}

func (s *GPUSupervisor) Start(ctx context.Context) error {
	return s.UnifiedSupervisor.Start(ctx)
}

// ExecuteJob overrides base ExecuteJob for GPU-specific tasks
func (s *GPUSupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	if !s.SupportsOperation(job.Operation) {
		return &foundation.Result{
			JobID: job.ID,
			Error: "gpu capability not supported: " + job.Operation,
		}
	}

	// 1. Construct Dispatch Job
	dispatchJob := &foundation.Job{
		ID:         job.ID,
		Type:       "gpu", // Routes to GPUUnit in Rust
		Operation:  job.Operation,
		Parameters: job.Parameters,
		Data:       job.Data,
	}

	// 2. Register job with bridge for reactive completion
	resultChan := s.bridge.RegisterJob(job.ID)

	// 3. Dispatch to Rust Muscle (via SAB)
	if err := s.bridge.WriteJob(dispatchJob); err != nil {
		return &foundation.Result{
			JobID: job.ID,
			Error: "gpu dispatch failed: " + err.Error(),
		}
	}

	// 4. Signal inbox dirty (Wait-Free Trigger)
	s.bridge.SignalInbox()

	// 5. Wait for result asynchronously (via channel)
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()

	select {
	case res := <-resultChan:
		return res
	case <-timer.C:
		return &foundation.Result{
			JobID: job.ID,
			Error: "gpu operation timed out",
		}
	}
}
