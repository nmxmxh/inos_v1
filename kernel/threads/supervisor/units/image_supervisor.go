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

// ImageSupervisor supervises the image module
type ImageSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge supervisor.SABInterface
}

func NewImageSupervisor(bridge supervisor.SABInterface, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string, delegator foundation.MeshDelegator) *ImageSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{"image.process", "image.filter", "image.recognize", "image.segment", "image.generate"}
	}
	return &ImageSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("image", capabilities, patterns, knowledge, delegator, bridge),
		bridge:            bridge,
	}
}

func (s *ImageSupervisor) Start(ctx context.Context) error {
	return s.UnifiedSupervisor.Start(ctx)
}

// ExecuteJob overrides base ExecuteJob for image-specific tasks
func (s *ImageSupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	if !s.SupportsOperation(job.Operation) {
		return &foundation.Result{
			JobID: job.ID,
			Error: "image capability not supported: " + job.Operation,
		}
	}

	dispatchJob := &foundation.Job{
		ID:         job.ID,
		Type:       "image",
		Operation:  job.Operation,
		Parameters: job.Parameters,
		Data:       job.Data,
	}

	resultChan := s.bridge.RegisterJob(job.ID)

	if err := s.bridge.WriteJob(dispatchJob); err != nil {
		return &foundation.Result{
			JobID: job.ID,
			Error: "image dispatch failed: " + err.Error(),
		}
	}

	s.bridge.SignalInbox()

	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()

	select {
	case res := <-resultChan:
		return res
	case <-timer.C:
		return &foundation.Result{
			JobID: job.ID,
			Error: "image operation timed out",
		}
	}
}
