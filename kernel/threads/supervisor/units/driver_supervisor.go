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

// DriverSupervisor supervises the hardware driver module
type DriverSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge  supervisor.SABInterface
	credits *supervisor.CreditSupervisor
}

func NewDriverSupervisor(bridge supervisor.SABInterface, credits *supervisor.CreditSupervisor, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string, delegator foundation.MeshDelegator) *DriverSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{"driver.device_management", "driver.hardware_abstraction", "driver.interrupt_handling", "driver.dma_coordination"}
	}
	// Note: DriverSupervisor shares the bridge now, but might have used index 7.
	// We need to confirm if 'bridge' supports multiple indices or if we route via operation.
	// For now, we inject the shared bridge to maintain consistency.
	return &DriverSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("driver", capabilities, patterns, knowledge, delegator, bridge, nil),
		bridge:            bridge,
		credits:           credits,
	}
}

func (s *DriverSupervisor) Start(ctx context.Context) error {
	return s.UnifiedSupervisor.Start(ctx)
}

// ExecuteJob overrides base ExecuteJob for driver-specific tasks
func (s *DriverSupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	if !s.SupportsOperation(job.Operation) {
		return &foundation.Result{
			JobID: job.ID,
			Error: "driver capability not supported: " + job.Operation,
		}
	}

	dispatchJob := &foundation.Job{
		ID:         job.ID,
		Type:       "driver",
		Operation:  job.Operation,
		Parameters: job.Parameters,
		Data:       job.Data,
	}

	resultChan := s.bridge.RegisterJob(job.ID)

	if err := s.bridge.WriteJob(dispatchJob); err != nil {
		return &foundation.Result{
			JobID: job.ID,
			Error: "driver dispatch failed: " + err.Error(),
		}
	}

	s.bridge.SignalInbox()

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	select {
	case res := <-resultChan:
		return res
	case <-timer.C:
		return &foundation.Result{
			JobID: job.ID,
			Error: "driver operation timed out",
		}
	}
}
