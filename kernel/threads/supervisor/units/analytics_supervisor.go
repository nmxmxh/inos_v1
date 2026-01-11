package units

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/nmxmxh/inos_v1/kernel/utils"
)

// MetricsProvider interface for getting/updating aggregated mesh data
type MetricsProvider interface {
	GetGlobalMetrics() common.MeshMetrics
	ReportComputeActivity(ops float64, gflops float64)
}

// AnalyticsSupervisor aggregates global mesh metrics for the dashboard
type AnalyticsSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge          supervisor.SABInterface
	metricsProvider MetricsProvider
}

func NewAnalyticsSupervisor(bridge supervisor.SABInterface, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, metricsProvider MetricsProvider, delegator foundation.MeshDelegator) *AnalyticsSupervisor {
	capabilities := []string{"analytics.aggregate", "analytics.broadcast"}
	return &AnalyticsSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("analytics", capabilities, patterns, knowledge, delegator),
		bridge:            bridge,
		metricsProvider:   metricsProvider,
	}
}

func (s *AnalyticsSupervisor) Start(ctx context.Context) error {
	utils.Info("Analytics supervisor started (Perpetual Mode)")

	// Run aggregation loop
	go s.aggregationLoop(ctx)

	return s.UnifiedSupervisor.Start(ctx)
}

func (s *AnalyticsSupervisor) aggregationLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.updateGlobalMetrics()
		}
	}
}

func (s *AnalyticsSupervisor) updateGlobalMetrics() {
	if s.metricsProvider == nil {
		return
	}

	// 1. Get real aggregated global metrics
	metrics := s.metricsProvider.GetGlobalMetrics()

	// 2. Prepare SAB buffer
	// Structure: [TotalStorage(8), TotalCompute(8), GlobalOps(8), NodeCount(4)]
	var buf [28]byte

	// Total Storage (Bytes)
	binary.LittleEndian.PutUint64(buf[0:8], metrics.TotalStorageBytes)

	// Total Compute (GFLOPS)
	// We use Uint64 for SAB slot, but metrics has float32. Convert for SAB storage.
	binary.LittleEndian.PutUint64(buf[8:16], uint64(metrics.TotalComputeGFLOPS))

	// Global Ops/Sec
	binary.LittleEndian.PutUint64(buf[16:24], uint64(metrics.GlobalOpsPerSec))

	// Node Count
	binary.LittleEndian.PutUint32(buf[24:28], metrics.ActiveNodeCount)

	// 3. Write to SAB
	if err := s.bridge.WriteRaw(sab_layout.OFFSET_GLOBAL_ANALYTICS, buf[:]); err != nil {
		utils.Error("Failed to write global analytics to SAB", utils.Err(err))
		return
	}

	// 4. Signal epoch update
	s.signalGlobalEpoch()
}

func (s *AnalyticsSupervisor) signalGlobalEpoch() {
	offset := sab_layout.OFFSET_ATOMIC_FLAGS + sab_layout.IDX_GLOBAL_METRICS_EPOCH*4

	var buf [4]byte
	if err := s.bridge.ReadAt(offset, buf[:]); err != nil {
		return
	}
	currentEpoch := binary.LittleEndian.Uint32(buf[:])

	newEpoch := currentEpoch + 1
	binary.LittleEndian.PutUint32(buf[:], newEpoch)

	s.bridge.WriteRaw(offset, buf[:])
}
