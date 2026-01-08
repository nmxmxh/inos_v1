//go:build js && wasm
// +build js,wasm

package main

import (
	"syscall/js"
	"time"
	"unsafe"

	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
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
		return js.ValueOf(map[string]interface{}{"error": "missing arguments: (ptr, size)"})
	}

	if kernelInstance == nil {
		return js.ValueOf(map[string]interface{}{"error": "kernel instance missing"})
	}

	// Address from JS is absolute offset in linear memory.
	// Using raw pointer arithmetic to avoid unsafe.Slice(nil,...) panic.
	ptrVal := uintptr(args[0].Int())
	ptr := unsafe.Pointer(ptrVal) //nolint:govet,unsafeptr // intentional: FFI boundary from JS (Foreign Pointer)

	// Store base pointer for Dynamic Grounding (exposed via getSystemSABAddress)
	sabBasePtr = ptrVal

	if err := kernelInstance.InjectSAB(ptr, uint32(args[1].Int())); err != nil {
		return js.ValueOf(map[string]interface{}{"success": false, "error": err.Error()})
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
			ptr := unsafe.Add(sabPtr, sab_layout.OFFSET_ATOMIC_FLAGS+sab_layout.IDX_BOIDS_COUNT*4)
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
