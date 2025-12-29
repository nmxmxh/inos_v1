package units

import (
	"context"

	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
)

// GPUSupervisor supervises the GPU module
type GPUSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge *supervisor.SABBridge
}

func NewGPUSupervisor(bridge *supervisor.SABBridge, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string) *GPUSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{"gpu.compute", "gpu.shader", "gpu.render", "gpu.tensor", "gpu.cuda_proxy"}
	}
	return &GPUSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("gpu", capabilities, patterns, knowledge),
		bridge:            bridge,
	}
}

func (s *GPUSupervisor) Start(ctx context.Context) error {
	return s.UnifiedSupervisor.Start(ctx)
}
