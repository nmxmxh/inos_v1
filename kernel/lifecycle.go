//go:build js && wasm
// +build js,wasm

package main

import (
	"context"
	"fmt"
	goruntime "runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh"
	"github.com/nmxmxh/inos_v1/kernel/core/mesh/transport"
	inosruntime "github.com/nmxmxh/inos_v1/kernel/runtime"
	"github.com/nmxmxh/inos_v1/kernel/threads"
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

// KernelConfig holds kernel configuration
type KernelConfig struct {
	EnableThreading bool
	MaxWorkers      int
	CacheSize       uint64
	LogLevel        utils.LogLevel
}

// Kernel is the root object managing the INOS runtime
type Kernel struct {
	state  atomic.Int32
	config *KernelConfig
	logger *utils.Logger

	// Core Components
	supervisor      *threads.Supervisor
	meshCoordinator *mesh.MeshCoordinator
	sabSize         atomic.Uint32
	meshIdentity    MeshIdentity
	roleConfig      inosruntime.RoleConfig

	// Lifecycle
	startTime time.Time
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Reactive Synchronization
	sabReady chan struct{}
}

// NewKernel creates a new kernel instance
func NewKernel() *Kernel {
	config := detectOptimalConfig()
	meshConfig := loadMeshConfig()

	logger := utils.NewLogger(utils.LoggerConfig{
		Level:      config.LogLevel,
		Component:  "kernel",
		Colorize:   true,
		ShowCaller: false,
	})

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize Mesh Components
	nodeID := meshConfig.Identity.NodeID
	tr, _ := transport.NewWebRTCTransport(nodeID, meshConfig.Transport, nil)
	m := mesh.NewMeshCoordinator(nodeID, meshConfig.Region, tr, nil)
	m.SetIdentity(meshConfig.Identity.DID, meshConfig.Identity.DeviceID, meshConfig.Identity.DisplayName)

	k := &Kernel{
		config:          config,
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
		meshCoordinator: m,
		meshIdentity:    meshConfig.Identity,
		sabReady:        make(chan struct{}),
	}

	k.setState(StateUninitialized)
	return k
}

// Boot wait for SAB injection then initializes subsystems
func (k *Kernel) Boot() {
	k.startTime = time.Now()
	defer k.recoverPanic()

	if !k.transitionState(StateUninitialized, StateBooting) {
		k.logger.Error("Invalid boot transition", utils.String("current", k.StateName()))
		return
	}

	k.logger.Info("INOS Kernel Boot Sequence",
		utils.String("version", "2.0"),
		utils.Int("cores", goruntime.NumCPU()),
		utils.Int("workers", k.config.MaxWorkers))

	if k.config.EnableThreading {
		goruntime.GOMAXPROCS(k.config.MaxWorkers)
	}

	// Signal kernel is ready for SAB injection
	k.setState(StateWaitingForSAB)
	k.notifyHost("kernel:waiting_for_sab", map[string]interface{}{
		"threading": k.config.EnableThreading,
		"workers":   k.config.MaxWorkers,
	})

	k.logger.Info("Kernel waiting for SAB injection...")

	// Phase 1.5: Runtime Profiling (Adaptive Mesh)
	// We run this parallel to waiting for SAB to save time,
	// or block here. Since it's CPU bound (compute test), we do it here.
	profiler := inosruntime.NewProfiler()
	caps := profiler.Profile()
	k.roleConfig = inosruntime.AssignRole(caps)

	// Phase 2: Reactive Synchronization
	// Wait for InjectSAB to signal the channel
	select {
	case <-k.sabReady:
		k.logger.Info("SAB signal received, initializing compute layer")
	case <-k.ctx.Done():
		return
	}

	// Now that we have the real SAB ptr (stored in supervisor during InjectSAB)
	// we can initialize the compute layer properly.
	size := k.GetSABSize()
	ptr := k.supervisor.GetSABPointer()

	// Check runtime stats

	k.logger.Info("Initializing compute layer",
		utils.Uint64("ptr_addr", uint64(uintptr(ptr))),
		utils.Uint64("size", uint64(size)))

	// Force memory growth is no longer needed - Go uses Split Memory Twin pattern
	// and accesses SAB via explicit js.CopyBytesToGo bridging

	if err := k.supervisor.InitializeCompute(ptr, size); err != nil {
		k.logger.Error("Failed to initialize compute layer", utils.Err(err))
	}

	k.logger.Info("Starting supervisor hierarchy")
	k.supervisor.Start()

	// Finalize Mesh Integration
	if k.meshCoordinator != nil {
		k.meshCoordinator.SetStorage(k.supervisor)
		// Inject SAB bridge for metrics reporting
		k.meshCoordinator.SetSABBridge(k.supervisor.GetBridge())
		// Inject monitor for delegation engine
		k.meshCoordinator.SetMonitor(k.supervisor)

		// Adaptive Mesh: Apply Role Configuration
		k.meshCoordinator.ApplyRoleConfig(k.roleConfig)

		if err := k.meshCoordinator.Start(k.ctx); err != nil {
			k.logger.Warn("Failed to start Mesh Coordinator", utils.Err(err))
		}
	}

	k.logger.Info("Kernel fully operational")
	k.notifyHost("kernel:fully_operational", map[string]interface{}{
		"threading": k.config.EnableThreading,
		"workers":   k.config.MaxWorkers,
		"role":      k.roleConfig.Role.String(),
	})
}

// InjectSAB performs the actual grounding of the kernel memory
func (k *Kernel) InjectSAB(ptr unsafe.Pointer, size uint32) error {
	defer k.recoverPanic()

	if KernelState(k.state.Load()) != StateWaitingForSAB {
		if KernelState(k.state.Load()) == StateRunning {
			k.logger.Warn("InjectSAB called but kernel already RUNNING")
			return nil
		}
		return fmt.Errorf("kernel not waiting for SAB (current: %s)", k.StateName())
	}

	if size == 0 {
		return fmt.Errorf("injected SAB size cannot be 0")
	}

	time.AfterFunc(0, func() {
		k.logger.Info("Injecting SharedArrayBuffer", utils.Uint64("size", uint64(size)))
	})
	k.sabSize.Store(size)

	// Initialize Root Supervisor with the real pointer immediately
	k.supervisor = threads.NewRootSupervisor(k.ctx, threads.SupervisorConfig{
		MeshCoordinator: k.meshCoordinator,
		Logger:          k.logger,
		SAB:             ptr,
		MaxWorkers:      k.config.MaxWorkers,
	})

	k.setState(StateRunning)
	time.AfterFunc(0, func() {
		k.notifyHost("kernel:running", nil)
	})

	// Trigger Boot sequence to continue
	close(k.sabReady)
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

// GetSABSize returns the injected SAB size
func (k *Kernel) GetSABSize() uint32 {
	return uint32(k.sabSize.Load())
}

// State Management
func (k *Kernel) setState(s KernelState) {
	k.state.Store(int32(s))
}

func (k *Kernel) transitionState(from, to KernelState) bool {
	return k.state.CompareAndSwap(int32(from), int32(to))
}

func (k *Kernel) StateName() string {
	return stateNames[KernelState(k.state.Load())]
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
