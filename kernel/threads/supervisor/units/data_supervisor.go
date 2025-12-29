package units

import (
	"context"

	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
)

// DataSupervisor supervises the data module
type DataSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge *supervisor.SABBridge
}

func NewDataSupervisor(bridge *supervisor.SABBridge, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string) *DataSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{"data.compress", "data.decompress", "data.parse", "data.transform", "data.validate"}
	}
	return &DataSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("data", capabilities, patterns, knowledge),
		bridge:            bridge,
	}
}

func (s *DataSupervisor) Start(ctx context.Context) error {
	return s.UnifiedSupervisor.Start(ctx)
}
