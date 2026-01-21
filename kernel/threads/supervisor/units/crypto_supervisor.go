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

// CryptoSupervisor supervises the crypto module
type CryptoSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge supervisor.SABInterface
}

func NewCryptoSupervisor(bridge supervisor.SABInterface, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string, delegator foundation.MeshDelegator) *CryptoSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{"crypto.hash", "crypto.sign", "crypto.verify", "crypto.encrypt", "crypto.decrypt", "crypto.keygen"}
	}
	return &CryptoSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("crypto", capabilities, patterns, knowledge, delegator, bridge, nil),
		bridge:            bridge,
	}
}

func (s *CryptoSupervisor) Start(ctx context.Context) error {
	return s.UnifiedSupervisor.Start(ctx)
}

// ExecuteJob overrides base ExecuteJob for crypto-specific tasks
func (s *CryptoSupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	if !s.SupportsOperation(job.Operation) {
		return &foundation.Result{
			JobID: job.ID,
			Error: "crypto capability not supported: " + job.Operation,
		}
	}

	dispatchJob := &foundation.Job{
		ID:         job.ID,
		Type:       "crypto",
		Operation:  job.Operation,
		Parameters: job.Parameters,
		Data:       job.Data,
	}

	resultChan := s.bridge.RegisterJob(job.ID)

	if err := s.bridge.WriteJob(dispatchJob); err != nil {
		return &foundation.Result{
			JobID: job.ID,
			Error: "crypto dispatch failed: " + err.Error(),
		}
	}

	s.bridge.SignalInbox()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case res := <-resultChan:
		return res
	case <-timer.C:
		return &foundation.Result{
			JobID: job.ID,
			Error: "crypto operation timed out",
		}
	}
}
