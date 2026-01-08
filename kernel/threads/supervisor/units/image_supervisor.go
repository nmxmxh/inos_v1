//go:build wasm

package units

import (
	"context"

	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
)

// ImageSupervisor supervises the image module
type ImageSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge *supervisor.SABBridge
}

func NewImageSupervisor(bridge *supervisor.SABBridge, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string) *ImageSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{"image.resize", "image.filter", "image.convert", "image.crop", "image.optimize"}
	}
	return &ImageSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("image", capabilities, patterns, knowledge),
		bridge:            bridge,
	}
}

func (s *ImageSupervisor) Start(ctx context.Context) error {
	return s.UnifiedSupervisor.Start(ctx)
}
