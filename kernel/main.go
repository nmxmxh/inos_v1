//go:build js && wasm
// +build js,wasm

package main

import (
	"runtime"
	"runtime/debug"
	"syscall/js"
)

// Global singleton
var kernelInstance *Kernel

// Synchronized SAB Region Size
// Moved to Kernel struct for thread-safety and proper injection order

// Dynamic SAB Base Pointer (set by InjectSAB, exposed to JS)
var sabBasePtr uintptr

func main() {
	// 1. Create Kernel Instance
	kernelInstance = NewKernel()

	// 2. Export Functions
	js.Global().Set("initializeSharedMemory", js.FuncOf(jsInitializeSharedMemory))
	js.Global().Set("getSharedArrayBuffer", js.FuncOf(jsGetSharedArrayBuffer))
	js.Global().Set("getKernelStats", js.FuncOf(jsGetKernelStats))
	js.Global().Set("shutdown", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if kernelInstance != nil {
			kernelInstance.Shutdown()
		}
		return nil
	}))

	// Economics API
	economics := js.Global().Get("Object").New()
	economics.Set("getBalance", js.FuncOf(jsGetEconomicBalance))
	economics.Set("getStats", js.FuncOf(jsGetEconomicStats))
	economics.Set("grantBonus", js.FuncOf(jsGrantEconomicBonus))
	economics.Set("getAccountInfo", js.FuncOf(jsGetAccountInfo))
	js.Global().Set("economics", economics)

	// Kernel API
	kernel := js.Global().Get("Object").New()
	kernel.Set("submitJob", js.FuncOf(jsSubmitJob))
	kernel.Set("deserializeResult", js.FuncOf(jsDeserializeResult))
	kernel.Set("getStats", js.FuncOf(jsGetKernelStats))
	js.Global().Set("kernel", kernel)

	// Mesh API (Explicit Delegation)
	mesh := js.Global().Get("Object").New()
	mesh.Set("delegateJob", js.FuncOf(jsDelegateJob))
	js.Global().Set("mesh", mesh)

	// Expose SAB metadata to Host (Dynamic Grounding)
	js.Global().Set("getSystemSABAddress", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		// Return the actual SAB base pointer (set by InjectSAB at runtime)
		return js.ValueOf(int(sabBasePtr))
	}))
	js.Global().Set("getSystemSABSize", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if kernelInstance != nil {
			return js.ValueOf(int(kernelInstance.GetSABSize()))
		}
		return js.ValueOf(0)
	}))

	// Register Shutdown Hook (Main thread only)
	window := js.Global().Get("window")
	if !window.IsUndefined() && !window.IsNull() {
		window.Call("addEventListener", "beforeunload", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			kernelInstance.Shutdown()
			return nil
		}))
	}

	// Signal Boot Sequence (Reactive - waits for SAB)
	go func() {
		kernelInstance.Boot()
		// Reclaim memory after subsystems are initialized
		debug.FreeOSMemory()
	}()

	// Block Main Thread
	select {}
}

// detectOptimalConfig move back from main.go
func detectOptimalConfig() *KernelConfig {
	numCPU := runtime.NumCPU()
	jsCores := 0

	nav := js.Global().Get("navigator")
	if !nav.IsUndefined() && !nav.IsNull() {
		hwConcurrency := nav.Get("hardwareConcurrency")
		if !hwConcurrency.IsUndefined() && !hwConcurrency.IsNull() {
			jsCores = hwConcurrency.Int()
		}
	}

	cores := numCPU
	if jsCores > cores {
		cores = jsCores
	}

	workers := cores / 4
	if workers < 1 {
		workers = 1
	}
	if workers > 4 {
		workers = 4
	}

	return &KernelConfig{
		EnableThreading: true,
		MaxWorkers:      workers,
		LogLevel:        1, // INFO
	}
}
