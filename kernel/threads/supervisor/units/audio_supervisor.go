//go:build wasm

package units

import (
	"context"

	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
)

// AudioSupervisor supervises the audio module
type AudioSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge *supervisor.SABBridge
}

func NewAudioSupervisor(bridge *supervisor.SABBridge, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string) *AudioSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{"audio.encode", "audio.decode", "audio.fft", "audio.filter", "audio.resample"}
	}
	return &AudioSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("audio", capabilities, patterns, knowledge),
		bridge:            bridge,
	}
}

func (s *AudioSupervisor) Start(ctx context.Context) error {
	return s.UnifiedSupervisor.Start(ctx)
}
