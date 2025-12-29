package units

import (
	"context"
	"sync"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
)

// MLSupervisor supervises the ML module using sub-units
type MLSupervisor struct {
	*supervisor.UnifiedSupervisor

	// SAB bridge for WASM communication
	bridge *supervisor.SABBridge

	// Model cache management
	activeModels map[string]bool
	mu           sync.RWMutex
}

// NewMLSupervisor creates a new ML supervisor
func NewMLSupervisor(
	bridge *supervisor.SABBridge,
	patterns *pattern.TieredPatternStorage,
	knowledge *intelligence.KnowledgeGraph,
	capabilities []string,
) *MLSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{
			"ml.inference", "ml.training", "ml.model_management",
			"tensor.ops", "layers.build", "training.step", "inference.predict",
			"model.load", "model.evict", "gpu.allocate",
		}
	}

	ms := &MLSupervisor{
		bridge:       bridge,
		activeModels: make(map[string]bool),
	}
	// MLSupervisor uses UnifiedSupervisor directly (Composite removed)
	ms.UnifiedSupervisor = supervisor.NewUnifiedSupervisor("ml", capabilities, patterns, knowledge)

	return ms
}

// ExecuteJob override with model lifecycle management
func (ms *MLSupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	// 1. Check if model needs loading
	if modelID, ok := job.Parameters["model_id"].(string); ok {
		ms.ensureModelLoaded(modelID)
	}

	// 2. Register job with bridge for reactive completion
	resultChan := ms.bridge.RegisterJob(job.ID)

	// 3. Submit to WASM bridge
	if err := ms.bridge.WriteJob(job); err != nil {
		return &foundation.Result{
			JobID: job.ID, Success: false, Error: err.Error(),
		}
	}

	// 4. Wait for result (ML jobs can take longer)
	timeout := 30 * time.Second
	if job.Deadline.After(time.Now()) {
		if d := time.Until(job.Deadline); d > 0 {
			timeout = d
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case res := <-resultChan:
		return res
	case <-timer.C:
		return &foundation.Result{
			JobID: job.ID, Success: false, Error: "ML inference timeout (reactive)",
		}
	}
}

func (ms *MLSupervisor) ensureModelLoaded(modelID string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.activeModels[modelID] {
		return
	}

	// Note: Real load triggered via bridge command in production
	ms.activeModels[modelID] = true
}

// Start starts the ML supervisor and its children
func (ms *MLSupervisor) Start(ctx context.Context) error {
	return ms.UnifiedSupervisor.Start(ctx)
}
