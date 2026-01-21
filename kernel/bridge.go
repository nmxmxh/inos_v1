//go:build js && wasm
// +build js,wasm

package main

import (
	"context"
	"syscall/js"
	"time"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/utils"
)

// notifyHost sends events to the JS environment
func (k *Kernel) notifyHost(event string, data map[string]interface{}) {
	payload := map[string]interface{}{
		"event":     event,
		"timestamp": time.Now().UnixNano(),
		"data":      data,
	}

	js.Global().Call("dispatchEvent",
		js.Global().Get("CustomEvent").New("inos:kernel", map[string]interface{}{
			"detail": payload,
		}),
	)
}

// --- JS Exports ---

func jsInitializeSharedMemory(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return js.ValueOf(map[string]interface{}{"error": "missing arguments: (offset, size)"})
	}

	if kernelInstance == nil {
		return js.ValueOf(map[string]interface{}{"error": "kernel instance missing"})
	}

	utils.Info("INOS Kernel Go Bridge Initializing (Synchronized Memory Twin)")

	// In the Twin pattern, we ignore the JS-provided offset (grounding pointer)
	// and allocate our own local replica of the requested size.
	size := uint32(args[1].Int())

	// FIX: Synchronous InjectSAB ensures the JS worker receives the success/error
	// strictly AFTER the kernel is ready for signaling.
	if err := kernelInstance.InjectSAB(nil, size); err != nil {
		kernelInstance.logger.Error("InjectSAB failed", utils.Err(err))
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}

	return js.ValueOf(map[string]interface{}{"success": true})
}

func jsGetKernelStats(this js.Value, args []js.Value) interface{} {
	if kernelInstance == nil {
		return js.ValueOf(nil)
	}

	uptime := time.Since(kernelInstance.startTime).String()
	particleCount := 0
	nodeCount := 1
	sector := 0

	if kernelInstance.supervisor != nil {
		sabPtr := kernelInstance.supervisor.GetSABPointer()
		if sabPtr != nil {
			// Read current count from standardized epoch index
			// Using raw pointer to avoid slice boundary checks
			ptr := unsafe.Add(sabPtr, sab_layout.OFFSET_ATOMIC_FLAGS+sab_layout.IDX_BIRD_COUNT*4)
			particleCount = int(*(*uint32)(ptr))
		}
	}

	meshStats := map[string]interface{}{}
	if kernelInstance.meshCoordinator != nil {
		nodeCount = kernelInstance.meshCoordinator.GetNodeCount()
		sector = kernelInstance.meshCoordinator.GetSectorID()
		meshStats = kernelInstance.meshCoordinator.GetTelemetry()
	}

	stats := map[string]interface{}{
		"nodes":     nodeCount,
		"particles": particleCount,
		"sector":    sector,
		"state":     kernelInstance.StateName(),
		"uptime":    uptime,
		"startedAt": kernelInstance.startTime.Format(time.RFC3339),
		"mesh":      meshStats,
	}

	if kernelInstance.supervisor != nil {
		supStats := kernelInstance.supervisor.GetStats()
		stats["supervisor"] = map[string]interface{}{
			"activeThreads": supStats.ActiveThreads,
			"totalMessages": supStats.TotalMessages,
			"failedThreads": supStats.FailedThreads,
		}
	} else {
		stats["supervisor"] = "not_started"
	}

	return js.ValueOf(stats)
}

func jsGetSharedArrayBuffer(this js.Value, args []js.Value) interface{} {
	if kernelInstance == nil || kernelInstance.supervisor == nil {
		return js.Null()
	}

	sab := kernelInstance.supervisor.GetSAB()
	if sab == nil {
		return js.Null()
	}

	sabConstructor := js.Global().Get("SharedArrayBuffer")
	sabJS := sabConstructor.New(len(sab))
	js.CopyBytesToJS(js.Global().Get("Uint8Array").New(sabJS), sab)

	return sabJS
}
func jsSubmitJob(this js.Value, args []js.Value) interface{} {
	println("DEBUG: jsSubmitJob called")
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing job argument"})
	}
	jobVal := args[0]

	job := &foundation.Job{
		ID:        jobVal.Get("id").String(),
		Type:      jobVal.Get("type").String(),
		Operation: jobVal.Get("op").String(),
	}

	if job.ID == "" {
		job.ID = utils.GenerateID()
	}

	// Data (Optional)
	if dataVal := jobVal.Get("data"); !dataVal.IsUndefined() && !dataVal.IsNull() {
		job.Data = make([]byte, dataVal.Get("length").Int())
		js.CopyBytesToGo(job.Data, dataVal)
	}

	// Parameters (Optional)
	if paramsVal := jobVal.Get("params"); !paramsVal.IsUndefined() && !paramsVal.IsNull() {
		job.Parameters = make(map[string]interface{})
		keys := js.Global().Get("Object").Call("keys", paramsVal)
		for i := 0; i < keys.Length(); i++ {
			k := keys.Index(i).String()
			v := paramsVal.Get(k)
			switch v.Type() {
			case js.TypeString:
				job.Parameters[k] = v.String()
			case js.TypeNumber:
				job.Parameters[k] = v.Float()
			case js.TypeBoolean:
				job.Parameters[k] = v.Bool()
			}
		}
	}

	// FIRE AND FORGET
	// Routes through supervisor hierarchy (Storage, Crypto, etc.)
	// Supervisors will decide whether to execute locally (Rust) or delegate to Mesh.
	utils.Info("JS submitted job", utils.String("job_id", job.ID), utils.String("type", job.Type), utils.String("op", job.Operation))
	// 2. Submit to supervisor
	go func() {
		if kernelInstance != nil && kernelInstance.supervisor != nil {
			resChan, err := kernelInstance.supervisor.Submit(job)
			if err != nil {
				utils.Warn("Submit job failed", utils.String("job_id", job.ID), utils.Err(err))
				// Report failure back to bridge
				if bridge := kernelInstance.supervisor.GetBridge(); bridge != nil {
					_ = bridge.WriteResult(&foundation.Result{
						JobID:   job.ID,
						Success: false,
						Error:   err.Error(),
					})
				}
				return
			}

			// Wait for result (Supervisor already writes to bridge, so we just wait)
			<-resChan
		}
	}()

	return js.ValueOf(map[string]interface{}{
		"success": true,
		"jobId":   job.ID,
		"status":  "submitted",
	})
}

func jsDelegateJob(this js.Value, args []js.Value) interface{} {
	println("DEBUG: jsDelegateJob called")
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing job argument"})
	}
	jobVal := args[0]

	job := &foundation.Job{
		ID:        jobVal.Get("id").String(),
		Type:      jobVal.Get("type").String(),
		Operation: jobVal.Get("op").String(),
	}

	if job.ID == "" {
		job.ID = utils.GenerateID()
	}

	// Data (Optional)
	if dataVal := jobVal.Get("data"); !dataVal.IsUndefined() && !dataVal.IsNull() {
		job.Data = make([]byte, dataVal.Get("length").Int())
		js.CopyBytesToGo(job.Data, dataVal)
	}

	// EXPLICIT DELEGATION
	// Specifically forces Mesh Coordinator handling
	utils.Info("JS delegated job", utils.String("job_id", job.ID), utils.String("op", job.Operation))
	go func() {
		if kernelInstance == nil {
			return
		}

		if kernelInstance.meshCoordinator != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result, err := kernelInstance.meshCoordinator.DelegateJob(ctx, job)
			if err != nil {
				utils.Warn("Mesh delegation error", utils.String("job_id", job.ID), utils.Err(err))
			}
			if err == nil && result != nil {
				utils.Info("Mesh delegation successful, writing result", utils.String("job_id", job.ID))
				if kernelInstance.supervisor != nil {
					if bridge := kernelInstance.supervisor.GetBridge(); bridge != nil {
						err := bridge.WriteResult(result)
						if err != nil {
							utils.Error("Failed to write delegation result", utils.String("job_id", job.ID), utils.Err(err))
						}
					}
				}
				return
			}
		}

		if kernelInstance.supervisor != nil {
			utils.Info("Delegated job falling back to local execution", utils.String("job_id", job.ID))
			resChan, err := kernelInstance.supervisor.Submit(job)
			if err != nil {
				utils.Warn("Delegated job fallback failed", utils.String("job_id", job.ID), utils.Err(err))
				// Report failure
				if bridge := kernelInstance.supervisor.GetBridge(); bridge != nil {
					_ = bridge.WriteResult(&foundation.Result{
						JobID:   job.ID,
						Success: false,
						Error:   err.Error(),
					})
				}
				return
			}

			// Wait for result (Supervisor already writes to bridge)
			<-resChan
		}
	}()

	return js.ValueOf(map[string]interface{}{
		"success": true,
		"jobId":   job.ID,
		"status":  "delegated",
	})
}

// jsDeserializeResult deserializes raw Cap'n Proto bytes into a JS result object
func jsDeserializeResult(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing data argument"})
	}

	if kernelInstance == nil || kernelInstance.supervisor == nil {
		return js.ValueOf(map[string]interface{}{"error": "kernel or supervisor not initialized"})
	}

	data := make([]byte, args[0].Length())
	js.CopyBytesToGo(data, args[0])

	result := kernelInstance.supervisor.GetBridge().DeserializeResult(data)

	res := map[string]interface{}{
		"jobId":   result.JobID,
		"success": result.Success,
		"error":   result.Error,
	}

	if len(result.Data) > 0 {
		uint8Arr := js.Global().Get("Uint8Array").New(len(result.Data))
		js.CopyBytesToJS(uint8Arr, result.Data)
		res["data"] = uint8Arr
	}

	return js.ValueOf(res)
}
