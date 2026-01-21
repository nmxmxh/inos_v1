package units

import (
	"context"
	"encoding/binary"
	"math"
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
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("analytics", capabilities, patterns, knowledge, delegator, bridge, nil),
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
	// v1.10+: Epoch-driven aggregation (zero CPU when idle)
	const aggregationThreshold int32 = 10 // Update every 10 epochs
	var lastEpoch int32 = 0
	var aggEpoch int32 = 0

	for {
		// If bridge is nil (e.g., in tests), fall back to time-based
		if s.bridge == nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
				s.updateGlobalMetrics()
			}
			continue
		}

		// Epoch-driven: Wait for activity with a 2-second heartbeat fallback
		select {
		case <-ctx.Done():
			return
		case <-s.bridge.WaitForEpochAsync(sab_layout.IDX_SYSTEM_EPOCH, lastEpoch):
			currentEpoch := s.bridge.ReadAtomicI32(sab_layout.IDX_SYSTEM_EPOCH)
			if currentEpoch-aggEpoch >= aggregationThreshold {
				s.updateGlobalMetrics()
				aggEpoch = currentEpoch
			}
			lastEpoch = currentEpoch
		case <-time.After(2 * time.Second):
			// Heartbeat: Force update if no epoch activity
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

	// Total Compute (GFLOPS) - Use Float64 for precision
	binary.LittleEndian.PutUint64(buf[8:16], math.Float64bits(float64(metrics.TotalComputeGFLOPS)))

	// Global Ops/Sec - Use Float64 for precision
	binary.LittleEndian.PutUint64(buf[16:24], math.Float64bits(float64(metrics.GlobalOpsPerSec)))

	// Node Count - Report exactly what the provider gives us
	nodeCount := metrics.ActiveNodeCount
	binary.LittleEndian.PutUint32(buf[24:28], nodeCount)

	// 3. Write to SAB
	if err := s.bridge.WriteRaw(sab_layout.OFFSET_GLOBAL_ANALYTICS, buf[:]); err != nil {
		utils.Error("Failed to write global analytics to SAB", utils.Err(err))
		return
	}

	// 4. Signal epoch update
	s.signalGlobalEpoch()
}

func (s *AnalyticsSupervisor) signalGlobalEpoch() {
	s.bridge.SignalEpoch(sab_layout.IDX_GLOBAL_METRICS_EPOCH)
}
