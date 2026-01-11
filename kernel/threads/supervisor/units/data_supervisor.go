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

// DataSupervisor supervises the data module
type DataSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge supervisor.SABInterface
}

func NewDataSupervisor(bridge supervisor.SABInterface, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string, delegator foundation.MeshDelegator) *DataSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{"data.transform", "data.filter", "data.aggregate", "data.validate", "data.query"}
	}
	return &DataSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("data", capabilities, patterns, knowledge, delegator),
		bridge:            bridge,
	}
}

func (s *DataSupervisor) Start(ctx context.Context) error {
	return s.UnifiedSupervisor.Start(ctx)
}

// ExecuteJob overrides base ExecuteJob for data-specific tasks
func (s *DataSupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	if !s.SupportsOperation(job.Operation) {
		return &foundation.Result{
			JobID: job.ID,
			Error: "data capability not supported: " + job.Operation,
		}
	}

	dispatchJob := &foundation.Job{
		ID:         job.ID,
		Type:       "data",
		Operation:  job.Operation,
		Parameters: job.Parameters,
		Data:       job.Data,
	}

	resultChan := s.bridge.RegisterJob(job.ID)

	if err := s.bridge.WriteJob(dispatchJob); err != nil {
		return &foundation.Result{
			JobID: job.ID,
			Error: "data dispatch failed: " + err.Error(),
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
			Error: "data operation timed out",
		}
	}
}
