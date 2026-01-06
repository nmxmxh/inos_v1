//go:build js && wasm
// +build js,wasm

package main

import (
	"context"
	"fmt"
	"reflect"
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

// Synchronized SAB Region
// This region is at the BASE of WebAssembly linear memory (address 0)
// to ensure Go, Rust, and JS all access the same absolute offsets.
var SystemSABSize int

// systemSAB is a byte slice view pointing to the START of linear memory
// It does NOT allocate new memory - it views existing WASM memory at offset 0
var systemSAB []byte

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

	// Signal kernel is ready for SAB injection
	k.setState(StateWaitingForSAB)

	// NOTE: We used to write to IDX_KERNEL_READY SAB byte here, but it conflicts with
	// the shutdown signal (both use offset 0). Since JS now waits for
	// initializeSharedMemory function availability, we no longer need the SAB write.

	k.notifyHost("kernel:waiting_for_sab", map[string]interface{}{
		"threading": k.config.EnableThreading,
		"workers":   k.config.MaxWorkers,
	})

	// NOW do heavy initialization (supervisor setup)
	// This runs in background while JS can inject SAB
	sabBasePtr := unsafe.Pointer(uintptr(0))
	k.supervisor = threads.NewRootSupervisor(k.ctx, threads.SupervisorConfig{
		MeshCoordinator: k.meshCoordinator,
		Logger:          k.logger,
		SAB:             sabBasePtr,
	})

	// Initialize Compute Layer with linear memory base
	if err := k.supervisor.InitializeCompute(sabBasePtr, uint32(SystemSABSize)); err != nil {
		k.logger.Error("Failed to initialize compute layer with linear memory base", utils.Err(err))
	}

	k.logger.Info("Kernel boot complete - waiting for SAB injection to start supervisor")
}

// InjectSAB receives the SharedArrayBuffer from the Host and starts the Supervisor
// IMPORTANT: This is NON-BLOCKING to align with INOS architecture.
// JS should not wait for Go kernel operations.
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

	k.logger.Info("Injecting SharedArrayBuffer (using linear memory base)", utils.Uint64("size", uint64(size)))

	// Transition to RUNNING immediately - don't block JS
	k.setState(StateRunning)
	k.logger.Info("Kernel Running", utils.String("mode", "ACTIVE"))

	// Notify host immediately that kernel is ready
	k.notifyHost("kernel:running", nil)

	// Start supervisor operations in background goroutine
	// This prevents blocking the JS main thread
	go func() {
		defer k.recoverPanic()

		// Wait for Boot() to finish InitializeCompute (which holds the lock)
		// Use a simple retry loop instead of blocking
		maxRetries := 100
		for i := 0; i < maxRetries; i++ {
			if k.supervisor != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		if k.supervisor == nil {
			k.logger.Error("Supervisor not initialized after timeout")
			return
		}

		k.logger.Info("Starting supervisor hierarchy (background)")

		// Start Supervisor Hierarchy
		k.supervisor.Start()

		k.logger.Info("Supervisor hierarchy started")

		// Finalize Mesh Integration (Phase 16)
		if k.meshCoordinator != nil {
			k.meshCoordinator.SetStorage(k.supervisor)
			if err := k.meshCoordinator.Start(k.ctx); err != nil {
				k.logger.Warn("Failed to start Mesh Coordinator", utils.Err(err))
			}
		}

		k.logger.Info("Kernel fully operational")
		k.notifyHost("kernel:fully_operational", map[string]interface{}{
			"threading": k.config.EnableThreading,
			"workers":   k.config.MaxWorkers,
		})
	}()

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
		if len(sab) > int(sab_layout.OFFSET_ATOMIC_FLAGS+sab_layout.IDX_BOIDS_COUNT*4) {
			// Read current count from standardized epoch index
			ptr := unsafe.Add(unsafe.Pointer(&sab[0]), sab_layout.OFFSET_ATOMIC_FLAGS+sab_layout.IDX_BOIDS_COUNT*4)
			particleCount = int(*(*uint32)(ptr))
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

	// Determine dynamic SAB size from environment or Tier
	sizeFromJS := js.Global().Get("window").Get("__INOS_SAB_SIZE__")
	if !sizeFromJS.IsUndefined() && !sizeFromJS.IsNull() {
		SystemSABSize = sizeFromJS.Int()
	} else {
		// Fallback to default if not set by frontend yet
		SystemSABSize = int(sab_layout.SAB_SIZE_DEFAULT)
	}

	// Double check bounds
	if SystemSABSize > int(sab_layout.SAB_SIZE_MAX) {
		SystemSABSize = int(sab_layout.SAB_SIZE_MAX)
	}
	if SystemSABSize < int(sab_layout.SAB_SIZE_MIN) {
		SystemSABSize = int(sab_layout.SAB_SIZE_MIN)
	}

	// Create a byte slice view at LINEAR MEMORY BASE (address 0)
	// This ensures Go, Rust, and JS all use the SAME absolute offsets
	var memoryBase uintptr = 0
	header := (*reflect.SliceHeader)(unsafe.Pointer(&systemSAB))
	header.Data = memoryBase
	header.Len = SystemSABSize
	header.Cap = SystemSABSize

	// Zero-copy exports for synchronized SAB
	// Address is 0 (linear memory base) so all layers agree on offsets
	js.Global().Set("getSystemSABAddress", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return js.ValueOf(0) // Linear memory base
	}))
	js.Global().Set("getSystemSABSize", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return js.ValueOf(SystemSABSize)
	}))

	// Register Shutdown Hook
	js.Global().Get("window").Call("addEventListener", "beforeunload", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		kernelInstance.Shutdown()
		return nil
	}))

	// CRITICAL: Yield to JS event loop after registering functions
	// time.Sleep actually yields to JS in Go WASM; runtime.Gosched() only yields to Go goroutines
	time.Sleep(time.Millisecond)

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
