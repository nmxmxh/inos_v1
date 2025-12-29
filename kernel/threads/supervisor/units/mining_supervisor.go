package units

import (
	"context"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
)

// MiningSupervisor supervises the mining module using sub-units
type MiningSupervisor struct {
	*supervisor.UnifiedSupervisor

	// SAB bridge for WASM communication
	bridge *supervisor.SABBridge

	// Mining state
	hashrate      float64
	sharesFound   uint64
	lastShareTime time.Time
}

// NewMiningSupervisor creates a new mining supervisor
func NewMiningSupervisor(
	bridge *supervisor.SABBridge,
	patterns *pattern.TieredPatternStorage,
	knowledge *intelligence.KnowledgeGraph,
	capabilities []string,
) *MiningSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{
			"pow.hash", "pow.verify", "mining.share", "mining.config",
			"gpu.temp", "gpu.power",
		}
	}

	return &MiningSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("mining", capabilities, patterns, knowledge),
		bridge:            bridge,
	}
}

// ExecuteJob override for mining operations
func (ms *MiningSupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	// Submit via bridge
	if err := ms.bridge.WriteJob(job); err != nil {
		return &foundation.Result{
			JobID: job.ID, Success: false, Error: err.Error(),
		}
	}

	// Mining jobs can be long-running (POW)
	timeout := 10 * time.Second
	if d := time.Until(job.Deadline); d > 0 {
		timeout = d
	}

	completed, err := ms.bridge.PollCompletion(timeout)
	if err != nil || !completed {
		return &foundation.Result{
			JobID: job.ID, Success: false, Error: "Mining job timeout",
		}
	}

	res, err := ms.bridge.ReadResult()
	if err != nil {
		return &foundation.Result{
			JobID: job.ID, Success: false, Error: err.Error(),
		}
	}

	// Update stats if successful
	if res.Success && job.Operation == "mining.share" {
		ms.sharesFound++
		ms.lastShareTime = time.Now()
	}

	return res
}

// Start starts the mining supervisor and its children
func (ms *MiningSupervisor) Start(ctx context.Context) error {
	return ms.UnifiedSupervisor.Start(ctx)
}

// Stats returns current mining statistics
func (ms *MiningSupervisor) Stats() (float64, uint64, time.Time) {
	return ms.hashrate, ms.sharesFound, ms.lastShareTime
}
