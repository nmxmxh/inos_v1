package units

import (
	"context"
	"encoding/binary"
	"math"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/nmxmxh/inos_v1/kernel/utils"
)

const (
	PhaseEntropic = 0
	PhaseEmergent = 1
	PhaseMorphic  = 2
)

// RobotSupervisor orchestrates the Morphic Lattice (Moonshot) simulation
type RobotSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge          supervisor.SABInterface
	metricsProvider MetricsProvider
}

func NewRobotSupervisor(bridge supervisor.SABInterface, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, metricsProvider MetricsProvider, delegator foundation.MeshDelegator) *RobotSupervisor {
	capabilities := []string{"robot.kinematics", "robot.syntropy", "robot.telemetry"}
	return &RobotSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("robot", capabilities, patterns, knowledge, delegator),
		bridge:            bridge,
		metricsProvider:   metricsProvider,
	}
}

func (s *RobotSupervisor) Start(ctx context.Context) error {
	utils.Info("Robot supervisor started (Syntropy Loop Active)")

	// Initialize robot state in SAB
	s.initRobotState()

	// Run orchestration loop
	go s.orchestrationLoop(ctx)

	return s.UnifiedSupervisor.Start(ctx)
}

func (s *RobotSupervisor) initRobotState() {
	// [Epoch(8), Phase(4), Syntropy(4), Score1(4), Score2(4), Score3(4), Score4(4)] = 32 bytes
	data := make([]byte, 32)
	binary.LittleEndian.PutUint64(data[0:8], 0) // Initial epoch
	binary.LittleEndian.PutUint32(data[8:12], PhaseEntropic)

	if err := s.bridge.WriteRaw(sab_layout.OFFSET_ROBOT_STATE, data); err != nil {
		utils.Error("Failed to initialize robot state in SAB", utils.Err(err))
	}
}

func (s *RobotSupervisor) orchestrationLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	// physicsTicker := time.NewTicker(16 * time.Millisecond) // DISABLED: Causes Kernel OOM due to allocation spam
	defer ticker.Stop()
	// defer physicsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.updateSyntropy()
		}
	}
}

func (s *RobotSupervisor) updateSyntropy() {
	if s.metricsProvider == nil {
		return
	}

	// 1. Get mesh metrics to drive syntropy
	metrics := s.metricsProvider.GetGlobalMetrics()

	// 2. Calculate Syntropy Score (0.0 to 1.0)
	// Factors: Active Nodes, GFLOPS stability, and Ops/Sec
	nodeFactor := math.Min(float64(metrics.ActiveNodeCount)/10.0, 1.0)         // Max syntropy at 10 nodes
	computeFactor := math.Min(float64(metrics.TotalComputeGFLOPS)/1000.0, 1.0) // Max syntropy at 1 TFLOPS

	syntropy := (nodeFactor + computeFactor) / 2.0

	// 3. Apply Local/Peer Balance (Morphic Optimization)
	// User Requirement: "balance between local and peer, if local can provice faster, optimise towards"
	// We optimize the syntropy score source based on local capability.
	localWeight := 1.0
	if metrics.TotalComputeGFLOPS < 100 {
		// Weak local node: Rely more on "peer influence" (simulated ideal state)
		localWeight = 0.4
	} else if metrics.TotalComputeGFLOPS < 500 {
		// Moderate local node: Balanced approach
		localWeight = 0.7
	}
	// Strong local node (>500 GFLOPS): dominate with local calculation (localWeight = 1.0)

	// In a real implementation, "peerSyntropy" would come from the mesh (gossip).
	// For now, we simulate peer supercomputer influence as a stable attractor (0.95 ideal).
	peerSyntropy := 0.95
	syntropy = syntropy*localWeight + peerSyntropy*(1.0-localWeight)

	// --- GO ML: LEARNING LOOP SIMULATION ---
	// Simulate online learning: The supervisor optimizes the lattice parameters over time.
	// This represents the "P2P ML" aspect where the network converges on an optimal topology.
	epochMod := time.Now().Unix() % 100
	learningCurve := 1.0 - (1.0 / (1.0 + float64(epochMod)*0.05)) // Sigmoid-like convergence
	modelAccuracy := 0.8 + (learningCurve * 0.19)                 // 0.8 -> 0.99 range

	// 4. Determine Phase
	phase := PhaseEntropic
	if syntropy > 0.8 {
		phase = PhaseMorphic
	} else if syntropy > 0.3 {
		phase = PhaseEmergent
	}

	// 5. Report Latent Compute (RobotUnit runs 60Hz autonomously)
	// 512 Nodes * 1024 Filaments * ~55k Ops/Frame * 60 FPS = 3.3 GFLOPS (approx)
	// We report this constant load if we are in Morphic/Emergent phase
	if phase >= PhaseEmergent && s.metricsProvider != nil {
		opsPerSec := 3.3 * 1e9 // 3.3 GFLOPS
		s.metricsProvider.ReportComputeActivity(opsPerSec, 3.3)
	}

	// 4. Update SAB
	data := make([]byte, 24) // [Phase(4), Syntropy(4), Meta1..4(16)]
	binary.LittleEndian.PutUint32(data[0:4], uint32(phase))
	binary.LittleEndian.PutUint32(data[4:8], math.Float32bits(float32(syntropy)))

	// Fill meta scores with metric components for visualization
	binary.LittleEndian.PutUint32(data[8:12], math.Float32bits(float32(nodeFactor)))
	binary.LittleEndian.PutUint32(data[12:16], math.Float32bits(float32(computeFactor)))

	// META3: Model Accuracy (ML Score)
	binary.LittleEndian.PutUint32(data[16:20], math.Float32bits(float32(modelAccuracy)))

	// Write at offset 8 (after epoch)
	if err := s.bridge.WriteRaw(sab_layout.OFFSET_ROBOT_STATE+8, data); err != nil {
		utils.Error("Failed to update robot syntropy in SAB", utils.Err(err))
	}

	// 5. Signal Epoch Update if phase changed or significant syntropy shift
	s.signalRobotEpoch()
}

func (s *RobotSupervisor) signalRobotEpoch() {
	offset := sab_layout.OFFSET_ATOMIC_FLAGS + sab_layout.IDX_ROBOT_EPOCH*4

	epochBytes, err := s.bridge.ReadRaw(offset, 4)
	if err != nil {
		return
	}
	currentEpoch := binary.LittleEndian.Uint32(epochBytes)

	newEpoch := currentEpoch + 1
	newBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(newBytes, newEpoch)

	s.bridge.WriteRaw(offset, newBytes)

	// Also update the logical epoch in the RobotState region
	stateEpochBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(stateEpochBuf, uint64(newEpoch))
	s.bridge.WriteRaw(sab_layout.OFFSET_ROBOT_STATE, stateEpochBuf)
}

func (s *RobotSupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	// Handle manual phase overrides or force training steps
	switch job.Operation {
	case "set_phase":
		p, _ := job.Parameters["phase"].(float64)
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, uint32(p))
		s.bridge.WriteRaw(sab_layout.OFFSET_ROBOT_STATE+8, data)
		s.signalRobotEpoch()
		return &foundation.Result{JobID: job.ID, Success: true}
	default:
		return s.UnifiedSupervisor.ExecuteJob(job)
	}
}
