//go:build wasm

package units

import (
	"context"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/nmxmxh/inos_v1/kernel/utils"
)

// StorageSupervisor supervises the storage module
type StorageSupervisor struct {
	*supervisor.UnifiedSupervisor

	// SAB bridge for WASM communication
	bridge supervisor.SABInterface
}

// NewStorageSupervisor creates a new storage supervisor
func NewStorageSupervisor(
	bridge supervisor.SABInterface,
	patterns *pattern.TieredPatternStorage,
	knowledge *intelligence.KnowledgeGraph,
	capabilities []string,
	delegator foundation.MeshDelegator,
) *StorageSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{
			"storage.cas", "storage.compress", "storage.replicate",
			"storage.deduplicate", "storage.verify", "storage.encrypt",
		}
	}

	return &StorageSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("storage", capabilities, patterns, knowledge, delegator, bridge, nil),
		bridge:            bridge,
	}
}

// Start starts the storage supervisor
func (ss *StorageSupervisor) Start(ctx context.Context) error {
	return ss.UnifiedSupervisor.Start(ctx)
}

// ExecuteJob overrides base ExecuteJob for storage-specific tasks
func (ss *StorageSupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	// 1. Check if delegation is preferred (e.g., large payload + local load high)
	start := time.Now()
	if ss.shouldDelegate(job) {
		ss.Logger.Info("Offloading storage task to mesh", utils.String("job_id", job.ID), utils.String("operation", job.Operation))
		result, err := ss.Coordinate(context.Background(), job)
		if err == nil {
			elapsed := time.Since(start)
			ss.RecordLatency(elapsed)
			ss.Logger.Info("Mesh delegation successful",
				utils.String("job_id", job.ID),
				utils.String("duration", elapsed.String()),
			)
			return result
		}
		ss.Logger.Warn("Mesh delegation failed, falling back to local", utils.String("job_id", job.ID), utils.Err(err))
	}

	// 2. Determine action and params based on Job Operation
	var method string
	params := make(map[string]interface{})

	switch job.Operation {
	case "store":
		method = "store_chunk"
		// Expect job.Parameters to contain "hash", "priority"
		if hash, ok := job.Parameters["hash"].(string); ok {
			params["content_hash"] = hash
		}
		if priority, ok := job.Parameters["priority"].(string); ok {
			params["priority"] = priority
		} else {
			params["priority"] = "medium"
		}
	case "load":
		method = "load_chunk"
		if hash, ok := job.Parameters["hash"].(string); ok {
			params["content_hash"] = hash
		}
	case "delete":
		method = "delete_chunk"
		if hash, ok := job.Parameters["hash"].(string); ok {
			params["content_hash"] = hash
		}
	case "query":
		method = "query_index"
		// Pass through filter params
		if priority, ok := job.Parameters["priority"].(string); ok {
			params["priority"] = priority
		}
		if modelID, ok := job.Parameters["model_id"].(string); ok {
			params["model_id"] = modelID
		}
	default:
		return &foundation.Result{
			JobID: job.ID,
			Error: "Unknown storage operation: " + job.Operation,
		}
	}

	// 2. Construct Dispatch Job
	dispatchJob := &foundation.Job{
		ID:         job.ID,
		Type:       "storage", // Routes to StorageUnit in Rust
		Operation:  method,
		Parameters: params,
		Data:       job.Data,
	}

	// 3. Register job with bridge for reactive completion
	resultChan := ss.bridge.RegisterJob(job.ID)

	// 4. Dispatch to Rust Muscle (via SAB)
	if err := ss.bridge.WriteJob(dispatchJob); err != nil {
		return &foundation.Result{
			JobID: job.ID,
			Error: "Dispatch failed (WriteJob): " + err.Error(),
		}
	}

	// 5. Wait for result asynchronously (via channel)
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	select {
	case res := <-resultChan:
		elapsed := time.Since(start)
		ss.RecordLatency(elapsed)
		ss.Logger.Info("Local storage operation completed",
			utils.String("job_id", job.ID),
			utils.String("duration", elapsed.String()),
		)
		return res
	case <-timer.C:
		return &foundation.Result{
			JobID: job.ID,
			Error: "Storage operation timed out (reactive)",
		}
	}
}

func (ss *StorageSupervisor) shouldDelegate(job *foundation.Job) bool {
	// Logic to determine if mesh offloading is efficient
	// - Is payload > 1MB?
	// - Is it a heavy operation (compress, store)?
	// - Is local CPU usage > 80%?

	isHeavy := job.Operation == "store" || job.Operation == "compress"
	isLarge := len(job.Data) > 1024*1024
	highLoad := ss.getSystemLoad() > 0.8

	return (isHeavy && isLarge) || (isLarge && highLoad)
}

func (ss *StorageSupervisor) getSystemLoad() float64 {
	// In a real implementation, this would read from the health monitor
	// For now, return a placeholder or use the healthMon if available
	if ss.UnifiedSupervisor != nil {
		// Mock load based on job counters
		submitted := float64(ss.Metrics().JobsSubmitted)
		completed := float64(ss.Metrics().JobsCompleted)
		if submitted > 0 {
			return (submitted - completed) / 10.0 // Very simplified
		}
	}
	return 0.2
}
