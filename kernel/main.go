//go:build js && wasm
// +build js,wasm

package main

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"syscall/js"
	"time"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh"
	"github.com/nmxmxh/inos_v1/kernel/core/mesh/transport"
	"github.com/nmxmxh/inos_v1/kernel/threads"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/utils"
)

// KernelState represents the lifecycle state of the kernel
type KernelState int32

const (
	StateUninitialized KernelState = iota
	StateBooting
	StateWaitingForSAB
	StateRunning
	StateStopping
	StateStopped
	StatePanic
)

var stateNames = map[KernelState]string{
	StateUninitialized: "UNINITIALIZED",
	StateBooting:       "BOOTING",
	StateWaitingForSAB: "WAITING_FOR_SAB",
	StateRunning:       "RUNNING",
	StateStopping:      "STOPPING",
	StateStopped:       "STOPPED",
	StatePanic:         "PANIC",
}

// Global singleton
var kernelInstance *Kernel

// Synchronized SAB Region (Static allocation in Go heap)
// Total size: 16MB. This region will be exposed to Rust modules.
const SystemSABSize = sab_layout.SAB_SIZE_DEFAULT

var systemSAB [SystemSABSize]byte

// Kernel is the root object managing the INOS runtime
type Kernel struct {
	state  atomic.Int32
	config *KernelConfig
	logger *utils.Logger

	// Core Components
	supervisor      *threads.Supervisor
	meshCoordinator *mesh.MeshCoordinator

	// Lifecycle
	startTime time.Time
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// KernelConfig holds kernel configuration
type KernelConfig struct {
	EnableThreading bool
	MaxWorkers      int
	CacheSize       uint64
	LogLevel        utils.LogLevel
}

// NewKernel creates a new kernel instance
func NewKernel() *Kernel {
	// Detect configuration
	config := detectOptimalConfig()

	// Initialize Logger
	logger := utils.NewLogger(utils.LoggerConfig{
		Level:      config.LogLevel,
		Component:  "kernel",
		Colorize:   true,
		ShowCaller: false,
	})

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize Mesh Components (Phase 16 Integration)
	nodeID := utils.GenerateID()
	tr, _ := transport.NewWebRTCTransport(nodeID, transport.DefaultTransportConfig(), nil)
	m := mesh.NewMeshCoordinator(nodeID, "global", tr, nil)

	k := &Kernel{
		config:          config,
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
		meshCoordinator: m,
	}

	k.setState(StateUninitialized)
	return k
}

// Boot starts the kernel capability negotiation and preparation
// It does NOT start the supervisor yet; it waits for SAB injection
func (k *Kernel) Boot() {
	k.startTime = time.Now()
	defer k.recoverPanic()

	if !k.transitionState(StateUninitialized, StateBooting) {
		k.logger.Error("Invalid boot transition", utils.String("current", k.StateName()))
		return
	}

	k.logger.Info("INOS Kernel Boot Sequence",
		utils.String("version", "1.9"),
		utils.Int("cores", runtime.NumCPU()),
		utils.Int("workers", k.config.MaxWorkers))

	if k.config.EnableThreading {
		runtime.GOMAXPROCS(k.config.MaxWorkers)
	}

	// Initialize Root Supervisor using the statically allocated SAB region
	k.supervisor = threads.NewRootSupervisor(k.ctx, threads.SupervisorConfig{
		MeshCoordinator: k.meshCoordinator,
		Logger:          k.logger,
		SAB:             unsafe.Pointer(&systemSAB[0]),
	})

	// Note: Compute initialization now happens via InjectSAB or when frontend provides SAB
	// We no longer initialize with the static SAB here to avoid double initialization

	k.setState(StateRunning)
	k.logger.Info("Kernel Running with internal static SAB")
	k.notifyHost("kernel:ready", map[string]interface{}{
		"threading": k.config.EnableThreading,
		"workers":   k.config.MaxWorkers,
	})
}

// InjectSAB receives the SharedArrayBuffer from the Host and starts the Supervisor
func (k *Kernel) InjectSAB(ptr unsafe.Pointer, size uint32) error {
	defer k.recoverPanic()

	currentState := KernelState(k.state.Load())
	if currentState != StateWaitingForSAB {
		// Just warn if already running, don't error out hard to prevent boot loops in dev
		if currentState == StateRunning {
			k.logger.Warn("InjectSAB called but kernel already RUNNING. Ignoring.")
			return nil
		}
		return fmt.Errorf("kernel not waiting for SAB (current: %s)", k.StateName())
	}

	k.logger.Info("Injecting SharedArrayBuffer", utils.Uint64("size", uint64(size)))

	// Initialize Compute Layer via Supervisor
	// This wires up the patterns, knowledge graph, and unit supervisors
	if err := k.supervisor.InitializeCompute(ptr, size); err != nil {
		k.logger.Error("Failed to initialize compute layer", utils.Err(err))
		return err
	}

	// Start Supervisor Hierarchy
	k.supervisor.Start()

	// Finalize Mesh Integration (Phase 16)
	if k.meshCoordinator != nil {
		// Wire Supervisor as StorageProvider
		k.meshCoordinator.SetStorage(k.supervisor)
		// Start Mesh Coordinator
		if err := k.meshCoordinator.Start(k.ctx); err != nil {
			k.logger.Warn("Failed to start Mesh Coordinator", utils.Err(err))
		}
	}

	k.setState(StateRunning)
	k.logger.Info("Kernel Running", utils.String("mode", "ACTIVE"))
	k.notifyHost("kernel:ready", map[string]interface{}{
		"threading": k.config.EnableThreading,
		"workers":   k.config.MaxWorkers,
	})

	return nil
}

// Shutdown initiates a graceful shutdown
func (k *Kernel) Shutdown() {
	k.setState(StateStopping)
	k.logger.Info("Kernel Shutting Down...")

	if k.supervisor != nil {
		k.supervisor.Stop()
	}

	k.cancel()
	k.wg.Wait()

	k.setState(StateStopped)
	k.logger.Info("Kernel Stopped")
	k.notifyHost("kernel:shutdown", nil)
}

// Helper: State Management
func (k *Kernel) setState(s KernelState) {
	k.state.Store(int32(s))
}

func (k *Kernel) transitionState(from, to KernelState) bool {
	return k.state.CompareAndSwap(int32(from), int32(to))
}

func (k *Kernel) StateName() string {
	return stateNames[KernelState(k.state.Load())]
}

// Helper: Host Notification (JS Bridge)
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

// Helper: Global Panic Recovery
func (k *Kernel) recoverPanic() {
	if r := recover(); r != nil {
		k.setState(StatePanic)
		stack := string(debug.Stack())
		k.logger.Error("KERNEL PANIC",
			utils.Any("reason", r),
			utils.String("stack", stack))

		k.notifyHost("kernel:panic", map[string]interface{}{
			"reason": fmt.Sprintf("%v", r),
			"stack":  stack,
		})
	}
}

// --- JS Exports ---

func jsInitializeSharedMemory(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return js.ValueOf(map[string]interface{}{"error": "missing arguments: (ptr, size)"})
	}

	if kernelInstance == nil {
		return js.ValueOf(map[string]interface{}{"error": "kernel instance missing"})
	}

	// Expecting address as int and size as int
	// Use indirection to bypass unsafeptr check for WASM SAB
	// GUARDIAN: If pointer is 0 (WASM base), we handle it carefully.
	// WASM memory IS the address space.
	// The address 'addr' from JS is the absolute offset in linear memory.

	// We convert the integer address to an unsafe.Pointer.
	// This is required for interfacing with host memory in WASM.
	ptr := unsafe.Pointer(uintptr(args[0].Int())) //nolint:all

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

	// Dynamically calculate particle count from SAB
	// Particles are stored at offset 0x1000 (4096), each particle is 6 floats (24 bytes)
	particleCount := 0
	nodeCount := 1
	sector := 0

	if kernelInstance.supervisor != nil {
		sab := kernelInstance.supervisor.GetSAB()
		if len(sab) > int(sab_layout.OFFSET_PATTERN_EXCHANGE) {
			// Calculate based on available space in SAB
			// Assuming particles use the region from OFFSET_PATTERN_EXCHANGE to end of SAB
			availableBytes := len(sab) - int(sab_layout.OFFSET_PATTERN_EXCHANGE)
			particleCount = availableBytes / 24 // 6 floats * 4 bytes per float

			// Cap at reasonable maximum
			if particleCount > 100000 {
				particleCount = 100000
			}
		}
	}

	// Get mesh stats
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

	// Add Supervisor Stats if available
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

// jsGetSharedArrayBuffer exports the SharedArrayBuffer for module registration
func jsGetSharedArrayBuffer(this js.Value, args []js.Value) interface{} {
	if kernelInstance == nil || kernelInstance.supervisor == nil {
		return js.Null()
	}

	sab := kernelInstance.supervisor.GetSAB()
	if sab == nil {
		return js.Null()
	}

	// Create a new SharedArrayBuffer from the Go slice
	// This allows Rust modules to write to the same memory
	sabConstructor := js.Global().Get("SharedArrayBuffer")
	sabJS := sabConstructor.New(len(sab))

	// Copy the current SAB content to the JS SharedArrayBuffer
	js.CopyBytesToJS(js.Global().Get("Uint8Array").New(sabJS), sab)

	return sabJS
}

// --- Entrypoint ---

func main() {
	// 1. Create Kernel Instance
	kernelInstance = NewKernel()

	// 2. Export Functions
	js.Global().Set("initializeSharedMemory", js.FuncOf(jsInitializeSharedMemory))
	js.Global().Set("getSharedArrayBuffer", js.FuncOf(jsGetSharedArrayBuffer))
	js.Global().Set("getKernelStats", js.FuncOf(jsGetKernelStats))

	// Zero-copy exports for synchronized SAB
	js.Global().Set("getSystemSABAddress", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return js.ValueOf(int(uintptr(unsafe.Pointer(&systemSAB[0]))))
	}))
	js.Global().Set("getSystemSABSize", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return js.ValueOf(SystemSABSize)
	}))

	// Register Shutdown Hook
	js.Global().Get("window").Call("addEventListener", "beforeunload", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		kernelInstance.Shutdown()
		return nil
	}))

	// 4. Register Shutdown Polling (Atomic Signal from Host)
	go func() {
		// Wait for SAB to be initialized
		for kernelInstance.StateName() != "RUNNING" {
			time.Sleep(100 * time.Millisecond)
		}

		// Polling loop for Atomic shutdown
		for {
			// IDX 0 of the system SAB is the shutdown flag
			if systemSAB[0] == 1 {
				kernelInstance.logger.Info("!!! ATOMIC SHUTDOWN SIGNAL RECEIVED !!!")
				kernelInstance.Shutdown()
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}()

	// 5. Boot (Wait for SAB)
	go kernelInstance.Boot()

	// 5. Block Main Thread (WASM requires this)
	select {}
}

// --- Config Detection (Moved from old main) ---
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
		LogLevel:        utils.INFO,
	}
}
