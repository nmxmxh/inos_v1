//go:build wasm

package units

import (
	"context"

	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
)

// DriverSupervisor supervises the hardware driver module
type DriverSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge  *supervisor.SABBridge
	credits *supervisor.CreditSupervisor
}

func NewDriverSupervisor(bridge *supervisor.SABBridge, credits *supervisor.CreditSupervisor, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string) *DriverSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{"driver.device_management", "driver.hardware_abstraction", "driver.interrupt_handling", "driver.dma_coordination"}
	}
	// Note: DriverSupervisor shares the bridge now, but might have used index 7.
	// We need to confirm if 'bridge' supports multiple indices or if we route via operation.
	// For now, we inject the shared bridge to maintain consistency.
	return &DriverSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("driver", capabilities, patterns, knowledge),
		bridge:            bridge,
		credits:           credits,
	}
}

func (s *DriverSupervisor) Start(ctx context.Context) error {
	return s.UnifiedSupervisor.Start(ctx)
}
