//go:build wasm

package threads

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unsafe"

	kruntime "github.com/nmxmxh/inos_v1/kernel/runtime"
	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/registry"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor/units"
	"github.com/nmxmxh/inos_v1/kernel/utils"
)

// Supervisor implements the hierarchical actor model for thread management
// NOTE: This package must NOT import core or core/mesh to avoid import cycles
type Supervisor struct {
	mu sync.RWMutex

	// Configuration (interfaces to avoid import cycles)
	config SupervisorConfig
	logger *utils.Logger

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Child supervisors (actor hierarchy)
	children map[string]*ChildSupervisor

	// Message queues for each child
	matchmakerQueue chan JobMatchRequest
	watcherQueue    chan HealthCheckRequest
	adjusterQueue   chan ThrottleRequest

	// Statistics
	stats SupervisorStats

	// Shared State
	sab       unsafe.Pointer // Pointer to local replica
	replica   []byte         // Backing buffer for replica
	sabSize   uint32
	bridge    *supervisor.SABBridge
	patterns  *pattern.TieredPatternStorage
	knowledge *intelligence.KnowledgeGraph
	registry  *registry.ModuleRegistry
	units     map[string]interface{}

	// Phase 17: Economy & Identity
	credits  *supervisor.CreditSupervisor
	identity *units.IdentitySupervisor
	social   *supervisor.SocialGraphSupervisor
}

type SupervisorConfig struct {
	Scheduler       any // core.Scheduler
	MeshCoordinator any // mesh.MeshCoordinator
	Logger          *utils.Logger
	SAB             unsafe.Pointer // SharedArrayBuffer pointer
	MaxWorkers      int
	Role            kruntime.RoleConfig
}

// SupervisorStats holds supervisor statistics
type SupervisorStats struct {
	ActiveThreads    int
	TotalMessages    uint64
	FailedThreads    int
	RestartedThreads int
}

// ChildSupervisor represents a supervised thread
type ChildSupervisor struct {
	name        string
	startFunc   func(context.Context) error
	restarts    int
	maxRestarts int
	lastRestart time.Time
}

// Message types for inter-thread communication
type JobMatchRequest struct {
	JobID        string
	Requirements interface{}
	ResponseChan chan JobMatchResponse
}

type JobMatchResponse struct {
	NodeID string
	Error  error
}

type HealthCheckRequest struct {
	NodeID       string
	ResponseChan chan HealthCheckResponse
}

type HealthCheckResponse struct {
	IsHealthy bool
	Load      float64
}

type ThrottleRequest struct {
	ResourceID   string
	CurrentLoad  float64
	ResponseChan chan ThrottleResponse
}

type ThrottleResponse struct {
	ShouldThrottle bool
	NewRate        float64
}

// NewRootSupervisor creates the root supervisor
func NewRootSupervisor(ctx context.Context, config SupervisorConfig) *Supervisor {
	supervisorCtx, cancel := context.WithCancel(ctx)

	logger := config.Logger
	if logger == nil {
		logger = utils.DefaultLogger("supervisor")
	}

	return &Supervisor{
		config:          config,
		logger:          logger,
		ctx:             supervisorCtx,
		cancel:          cancel,
		children:        make(map[string]*ChildSupervisor),
		matchmakerQueue: make(chan JobMatchRequest, 100),
		watcherQueue:    make(chan HealthCheckRequest, 100),
		adjusterQueue:   make(chan ThrottleRequest, 100),
	}
}

// Start starts the supervisor hierarchy
func (s *Supervisor) Start() {
	s.logger.Info("Starting hierarchical supervisor")

	// Spawn all children CONCURRENTLY to avoid lock starvation
	// Go WASM uses cooperative scheduling, so sequential lock acquisition can starve
	go s.spawnChild("matchmaker", s.runMatchmaker, 3)
	go s.spawnChild("watcher", s.runWatcher, 3)
	go s.spawnChild("adjuster", s.runAdjuster, 3)

	s.logger.Info("Supervisor hierarchy started")
}

// GetSABPointer returns the base address of the SharedArrayBuffer
func (s *Supervisor) GetSABPointer() unsafe.Pointer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sab
}

// GetBridge returns the shared SAB bridge
func (s *Supervisor) GetBridge() *supervisor.SABBridge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bridge
}

// GetCredits returns the economic supervisor
func (s *Supervisor) GetCredits() *supervisor.CreditSupervisor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.credits
}

// GetSystemLoad implements mesh.SystemLoadProvider via RootSupervisor
func (s *Supervisor) GetSystemLoad() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Heuristic: Active threads vs Max threads
	active := float64(len(s.children))
	max := float64(s.config.MaxWorkers)
	if max == 0 {
		max = 1.0
	}
	load := active / max
	if load > 1.0 {
		load = 1.0
	}
	return load
}

// InitializeCompute initializes the compute units with the provided SAB
func (s *Supervisor) InitializeCompute(sab unsafe.Pointer, size uint32) error {
	s.mu.Lock()

	// Guard: Don't initialize twice
	if s.units != nil {
		s.mu.Unlock()
		return nil
	}

	// Allocate local replica for Synchronized Memory Twin
	// This ensures Go operates on a stable private snapshot
	s.replica = make([]byte, size)
	s.sab = unsafe.Pointer(&s.replica[0])
	s.sabSize = size

	// Registry
	s.logger.Info("Creating ModuleRegistry", utils.Uint64("sab_addr", uint64(uintptr(s.sab))), utils.Uint64("size", uint64(s.sabSize)))
	s.registry = registry.NewModuleRegistry(s.sab, s.sabSize)
	s.logger.Info("Initializing registry region",
		utils.Uint64("offset", uint64(sab_layout.OFFSET_MODULE_REGISTRY)),
		utils.Int("size", int(sab_layout.SIZE_MODULE_REGISTRY)))

	// Patterns
	s.logger.Info("Creating TieredPatternStorage")
	s.patterns = pattern.NewTieredPatternStorage(s.sab, s.sabSize, sab_layout.OFFSET_PATTERN_EXCHANGE, sab_layout.SIZE_PATTERN_EXCHANGE)

	// Knowledge
	s.logger.Info("Creating KnowledgeGraph")
	s.knowledge = intelligence.NewKnowledgeGraph(s.sab, s.sabSize, sab_layout.OFFSET_COORDINATION, sab_layout.SIZE_COORDINATION)

	// Load modules from registry happens later in loader.LoadUnits()
	s.logger.Info("Supervisor regions established")

	// Initialize Core System Supervisors
	s.credits = supervisor.NewCreditSupervisor(s.sab, s.sabSize, uint32(sab_layout.OFFSET_ECONOMICS))

	// Inject CreditSupervisor into MeshCoordinator for unified economic authority
	if m, ok := s.config.MeshCoordinator.(interface {
		SetEconomicVault(foundation.EconomicVault)
	}); ok {
		m.SetEconomicVault(s.credits)
	}

	s.social = supervisor.NewSocialGraphSupervisor(s.sab, s.sabSize, uint32(sab_layout.OFFSET_SOCIAL_GRAPH))
	s.identity = units.NewIdentitySupervisor(
		s.bridge,
		s.patterns,
		s.knowledge,
		s.sab,
		s.sabSize,
		uint32(sab_layout.OFFSET_IDENTITY_REGISTRY),
		s.credits,
		s.social,
		nil,
	)

	s.logger.Info("Core regions established",
		utils.Uint64("identity_offset", uint64(sab_layout.OFFSET_IDENTITY_REGISTRY)),
		utils.Uint64("social_offset", uint64(sab_layout.OFFSET_SOCIAL_GRAPH)),
		utils.Uint64("economics_offset", uint64(sab_layout.OFFSET_ECONOMICS)))

	// Register Core System DIDs
	if _, err := s.identity.RegisterDID("did:inos:system", nil); err != nil {
		s.logger.Error("Failed to register system DID", utils.Err(err))
	}
	if _, err := s.identity.RegisterDID("did:inos:nmxmxh", nil); err != nil {
		s.logger.Error("Failed to register nmxmxh DID", utils.Err(err))
	}

	s.logger.Info("Initializing compute units with shared SAB")

	// Cast MeshCoordinator for metrics sharing
	var mp units.MetricsProvider
	var md foundation.MeshDelegator
	if m, ok := s.config.MeshCoordinator.(units.MetricsProvider); ok {
		mp = m
	}
	if d, ok := s.config.MeshCoordinator.(foundation.MeshDelegator); ok {
		md = d
	}

	loader := NewUnitLoader(s.replica, s.patterns, s.knowledge, s.registry, s.credits, s.identity, mp, s.config.Role, md)
	loadedUnits, bridge := loader.LoadUnits()
	s.bridge = bridge
	s.units = loadedUnits

	// Synchronize initial system state
	if s.bridge != nil {
		s.bridge.RefreshEconomics()
	}

	// CRITICAL: Release lock BEFORE spawning children to avoid recursive lock
	// Child spawning acquires its own lock, so we must not hold this one
	s.mu.Unlock()

	// Start supervisors for initially discovered units (CONCURRENT - failures isolated)
	for name, unit := range loadedUnits {
		if starter, ok := unit.(interface{ Start(context.Context) error }); ok {
			go func(n string, st interface{ Start(context.Context) error }) {
				s.spawnChild(n, st.Start, 5)
			}(name, starter)
		}
	}

	// Spawn Background Loops (CONCURRENT)
	go s.spawnChild("discovery_loop", s.runDiscoveryLoop, 1)
	go s.spawnChild("signal_listener", s.runSignalListener, 100)
	go s.spawnChild("economy_loop", s.runEconomyLoop, 10)
	go s.spawnChild("metrics_loop", s.runMetricsLoop, 1)

	return nil
}

// runDiscoveryLoop waits for module registration signals (zero-CPU blocking)
// Replaces polling with Atomics.wait on IDX_REGISTRY_EPOCH
// Submit routes a job to the appropriate unit supervisor and returns a result channel
func (s *Supervisor) Submit(job *foundation.Job) (<-chan *foundation.Result, error) {
	s.mu.RLock()
	unit, ok := s.units[job.Type]
	s.mu.RUnlock()

	if !ok {
		if job.Type == "compute" {
			s.mu.RLock()
			unit, ok = s.units["data"]
			s.mu.RUnlock()
		}
		if !ok {
			return nil, fmt.Errorf("unit not found: %s", job.Type)
		}
	}

	// Most unit supervisors embed UnifiedSupervisor which has Submit()
	if submitter, ok := unit.(interface {
		Submit(*foundation.Job) (<-chan *foundation.Result, error)
	}); ok {
		return submitter.Submit(job)
	}

	// Fallback: Use ExecuteJob directly if the unit doesn't have a queue
	if exec, ok := unit.(interface {
		ExecuteJob(*foundation.Job) *foundation.Result
	}); ok {
		resChan := make(chan *foundation.Result, 1)
		go func() {
			res := exec.ExecuteJob(job)
			resChan <- res
			close(resChan)
		}()
		return resChan, nil
	}

	return nil, fmt.Errorf("unit does not support job submission: %s", job.Type)
}

func (s *Supervisor) runDiscoveryLoop(ctx context.Context) error {
	// Cast MeshCoordinator for metrics sharing
	var mp units.MetricsProvider
	var md foundation.MeshDelegator
	if m, ok := s.config.MeshCoordinator.(units.MetricsProvider); ok {
		mp = m
	}
	if d, ok := s.config.MeshCoordinator.(foundation.MeshDelegator); ok {
		md = d
	}
	loader := NewUnitLoader(s.replica, s.patterns, s.knowledge, s.registry, s.credits, s.identity, mp, s.config.Role, md)
	var lastRegistryEpoch int32 = 0

	for {
		// Check for shutdown
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		s.mu.RLock()
		reg := s.registry
		bridge := s.bridge
		s.mu.RUnlock()

		if reg == nil || bridge == nil {
			// Not initialized yet, wait a bit
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}

		// Wait for registry epoch change using Atomics.wait (zero CPU)
		// This blocks until Rust signals a module registration or timeout
		result := bridge.WaitForEpochChange(
			sab_layout.IDX_REGISTRY_EPOCH,
			lastRegistryEpoch,
			2000.0, // 2s max wait for shutdown responsiveness
		)

		// Update last epoch regardless of result
		currentEpoch := bridge.ReadAtomicI32(sab_layout.IDX_REGISTRY_EPOCH)
		if currentEpoch != lastRegistryEpoch {
			lastRegistryEpoch = currentEpoch

			// Activity detected - scan for new modules
			if err := reg.LoadFromSAB(); err != nil {
				continue
			}

			modules := reg.ListModules()
			for _, mod := range modules {
				s.mu.RLock()
				_, exists := s.units[mod.ID]
				s.mu.RUnlock()

				if !exists {
					s.logger.Info("Discovered new module (signal-based)", utils.String("id", mod.ID))

					// Instantiate and start supervisor
					unit := loader.InstantiateUnit(bridge, mod)

					s.mu.Lock()
					s.units[mod.ID] = unit
					s.mu.Unlock()

					if starter, ok := unit.(interface{ Start(context.Context) error }); ok {
						s.spawnChild(mod.ID, starter.Start, 5)
					}
				}
			}
		} else if result == 1 {
			// Timeout - no activity, loop continues
			continue
		}
	}
}

// runMetricsLoop periodically persists bridge metrics to SAB for external diagnostics
// v1.10+: Epoch-driven (zero CPU when idle)
func (s *Supervisor) runMetricsLoop(ctx context.Context) error {
	const metricsThreshold int32 = 10 // Update every 10 epochs
	var lastEpoch int32 = 0
	var metricsEpoch int32 = 0

	for {
		s.mu.RLock()
		bridge := s.bridge
		s.mu.RUnlock()

		// Fallback to time-based if no bridge
		if bridge == nil {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(1 * time.Second):
				// Skip metrics write when no bridge
			}
			continue
		}

		// Epoch-driven: Wait for activity
		select {
		case <-ctx.Done():
			return nil
		case <-bridge.WaitForEpochAsync(sab_layout.IDX_SYSTEM_EPOCH, lastEpoch):
			currentEpoch := bridge.ReadAtomicI32(sab_layout.IDX_SYSTEM_EPOCH)
			if currentEpoch-metricsEpoch >= metricsThreshold {
				bridge.WriteMetricsToSAB()
				metricsEpoch = currentEpoch
			}
			lastEpoch = currentEpoch
		}
	}
}

// Stop stops the supervisor hierarchy
func (s *Supervisor) Stop() {
	s.logger.Info("Stopping supervisor hierarchy")
	s.cancel()
	s.wg.Wait()
	close(s.matchmakerQueue)
	close(s.watcherQueue)
	close(s.adjusterQueue)
	s.logger.Info("Supervisor hierarchy stopped")
}

// spawnChild spawns a child supervisor with automatic restart
func (s *Supervisor) spawnChild(name string, startFunc func(context.Context) error, maxRestarts int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	child := &ChildSupervisor{
		name:        name,
		startFunc:   startFunc,
		maxRestarts: maxRestarts,
	}

	s.children[name] = child
	s.wg.Add(1)
	go s.superviseChild(child)
}

// superviseChild supervises a child thread with automatic restart
func (s *Supervisor) superviseChild(child *ChildSupervisor) {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Child supervisor stopping", utils.String("name", child.name))
			return
		default:
			err := s.runChildWithRecovery(child)

			if err != nil {
				s.logger.Error("Child supervisor failed", utils.String("name", child.name), utils.Err(err))

				if child.restarts >= child.maxRestarts {
					s.logger.Error("Child supervisor exceeded max restarts",
						utils.String("name", child.name),
						utils.Int("restarts", child.restarts))
					s.mu.Lock()
					s.stats.FailedThreads++
					s.mu.Unlock()
					return
				}

				backoff := time.Duration(child.restarts+1) * time.Second
				s.logger.Warn("Restarting child supervisor",
					utils.String("name", child.name),
					utils.Duration("backoff", backoff))

				time.Sleep(backoff)
				child.restarts++
				child.lastRestart = time.Now()

				s.mu.Lock()
				s.stats.RestartedThreads++
				s.mu.Unlock()
			}
		}
	}
}

// runChildWithRecovery runs a child with panic recovery
func (s *Supervisor) runChildWithRecovery(child *ChildSupervisor) (err error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("Child supervisor panicked", utils.String("name", child.name))
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic: %v", r)
			}
		}
	}()

	return child.startFunc(s.ctx)
}

// ========== StorageProvider Implementation ==========

// StoreChunk stores a data chunk via the StorageSupervisor
func (s *Supervisor) StoreChunk(ctx context.Context, hash string, data []byte) error {
	s.mu.RLock()
	unit, ok := s.units["storage"]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("storage unit not found")
	}

	ss, ok := unit.(*units.StorageSupervisor)
	if !ok {
		return fmt.Errorf("invalid storage unit type")
	}

	job := &foundation.Job{
		ID:        utils.GenerateID(),
		Type:      "storage",
		Operation: "store",
		Parameters: map[string]interface{}{
			"hash":     hash,
			"priority": "high",
		},
		Data: data,
	}

	result := ss.ExecuteJob(job)
	if result.Error != "" {
		return fmt.Errorf("storage error: %s", result.Error)
	}

	return nil
}

// FetchChunk retrieves a data chunk via the StorageSupervisor
func (s *Supervisor) FetchChunk(ctx context.Context, hash string) ([]byte, error) {
	s.mu.RLock()
	unit, ok := s.units["storage"]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("storage unit not found")
	}

	ss, ok := unit.(*units.StorageSupervisor)
	if !ok {
		return nil, fmt.Errorf("invalid storage unit type")
	}

	job := &foundation.Job{
		ID:        utils.GenerateID(),
		Type:      "storage",
		Operation: "load",
		Parameters: map[string]interface{}{
			"hash": hash,
		},
	}

	result := ss.ExecuteJob(job)
	if result.Error != "" {
		return nil, fmt.Errorf("storage error: %s", result.Error)
	}

	return result.Data, nil
}

// HasChunk checks if a chunk exists via the StorageSupervisor
func (s *Supervisor) HasChunk(ctx context.Context, hash string) (bool, error) {
	// For now, we use "query" or just try a "load" with minimal data?
	// Actually, the StorageUnit in Rust might support a specific 'has' check.
	// Looking at storage_supervisor.go, it supports 'query'.
	// Let's use Load but we don't need the data.
	// OR we can add a 'has_chunk' method to StorageUnit/Supervisor.
	// For production grade, let's just check if it's in the pattern storage if it's there.

	// Actually, the best way is to ask the supervisor to check.
	// Let's use a "query" operation.
	s.mu.RLock()
	unit, ok := s.units["storage"]
	s.mu.RUnlock()

	if !ok {
		return false, fmt.Errorf("storage unit not found")
	}

	ss, ok := unit.(*units.StorageSupervisor)
	if !ok {
		return false, fmt.Errorf("invalid storage unit type")
	}

	job := &foundation.Job{
		ID:        utils.GenerateID(),
		Type:      "storage",
		Operation: "query",
		Parameters: map[string]interface{}{
			"hash": hash,
		},
	}

	result := ss.ExecuteJob(job)
	if result.Error != "" {
		return false, fmt.Errorf("storage error: %s", result.Error)
	}

	// If result contains the hash, it exists.
	// The Rust side should return something indicating existence.
	// For now, assume if no error and we got a result, it might exist.
	// TODO: Refine 'has_chunk' semantics in storage module.
	return result.Error == "", nil
}

// GetStats returns supervisor statistics
func (s *Supervisor) GetStats() SupervisorStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := s.stats
	stats.ActiveThreads = len(s.children)
	return stats
}

// GetSAB returns the SharedArrayBuffer for stats calculation
func (s *Supervisor) GetSAB() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sab == nil || s.sabSize == 0 {
		return nil
	}

	// Convert unsafe.Pointer to []byte slice safely
	return unsafe.Slice((*byte)(s.sab), s.sabSize)
}

// runMatchmaker runs the matchmaker thread
func (s *Supervisor) runMatchmaker(ctx context.Context) error {
	s.logger.Info("Matchmaker thread started")

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Matchmaker thread stopping")
			return nil
		case req := <-s.matchmakerQueue:
			s.logger.Debug("Processing job match request", utils.String("job_id", req.JobID))
			req.ResponseChan <- JobMatchResponse{NodeID: "node-1", Error: nil}
			s.mu.Lock()
			s.stats.TotalMessages++
			s.mu.Unlock()
		}
	}
}

// runWatcher runs the watcher thread
// v1.10+: Epoch-driven with channel listening
func (s *Supervisor) runWatcher(ctx context.Context) error {
	s.logger.Info("Watcher thread started")
	const watcherThreshold int32 = 100 // Health check every 100 epochs
	var lastEpoch int32 = 0
	var watchEpoch int32 = 0

	for {
		s.mu.RLock()
		bridge := s.bridge
		s.mu.RUnlock()

		// Fallback to time-based if no bridge
		if bridge == nil {
			select {
			case <-ctx.Done():
				s.logger.Info("Watcher thread stopping")
				return nil
			case <-time.After(1 * time.Second):
				s.logger.Debug("Performing periodic health check")
				s.mu.Lock()
				s.stats.TotalMessages++
				s.mu.Unlock()
			case req := <-s.watcherQueue:
				s.logger.Debug("Processing health check request", utils.String("node_id", req.NodeID))
				req.ResponseChan <- HealthCheckResponse{IsHealthy: true, Load: 0.3}
				s.mu.Lock()
				s.stats.TotalMessages++
				s.mu.Unlock()
			}
			continue
		}

		// Epoch-driven: Wait for activity or queue messages
		select {
		case <-ctx.Done():
			s.logger.Info("Watcher thread stopping")
			return nil
		case <-bridge.WaitForEpochAsync(sab_layout.IDX_SYSTEM_EPOCH, lastEpoch):
			currentEpoch := bridge.ReadAtomicI32(sab_layout.IDX_SYSTEM_EPOCH)
			if currentEpoch-watchEpoch >= watcherThreshold {
				s.logger.Debug("Performing periodic health check")
				s.mu.Lock()
				s.stats.TotalMessages++
				s.mu.Unlock()
				watchEpoch = currentEpoch
			}
			lastEpoch = currentEpoch
		case req := <-s.watcherQueue:
			s.logger.Debug("Processing health check request", utils.String("node_id", req.NodeID))
			req.ResponseChan <- HealthCheckResponse{IsHealthy: true, Load: 0.3}
			s.mu.Lock()
			s.stats.TotalMessages++
			s.mu.Unlock()
		}
	}
}

// runAdjuster runs the adjuster thread
// v1.10+: Epoch-driven with channel listening
func (s *Supervisor) runAdjuster(ctx context.Context) error {
	s.logger.Info("Adjuster thread started")
	const adjusterThreshold int32 = 50 // Adjust every 50 epochs
	var lastEpoch int32 = 0
	var adjustEpoch int32 = 0

	for {
		s.mu.RLock()
		bridge := s.bridge
		s.mu.RUnlock()

		// Fallback to time-based if no bridge
		if bridge == nil {
			select {
			case <-ctx.Done():
				s.logger.Info("Adjuster thread stopping")
				return nil
			case <-time.After(500 * time.Millisecond):
				s.logger.Debug("Checking load and adjusting throttling")
				s.mu.Lock()
				s.stats.TotalMessages++
				s.mu.Unlock()
			case req := <-s.adjusterQueue:
				s.processAdjusterRequest(req)
			}
			continue
		}

		// Epoch-driven: Wait for activity or queue messages
		select {
		case <-ctx.Done():
			s.logger.Info("Adjuster thread stopping")
			return nil
		case <-bridge.WaitForEpochAsync(sab_layout.IDX_SYSTEM_EPOCH, lastEpoch):
			currentEpoch := bridge.ReadAtomicI32(sab_layout.IDX_SYSTEM_EPOCH)
			if currentEpoch-adjustEpoch >= adjusterThreshold {
				s.logger.Debug("Checking load and adjusting throttling")
				s.mu.Lock()
				s.stats.TotalMessages++
				s.mu.Unlock()
				adjustEpoch = currentEpoch
			}
			lastEpoch = currentEpoch
		case req := <-s.adjusterQueue:
			s.processAdjusterRequest(req)
		}
	}
}

// processAdjusterRequest handles a throttle request
func (s *Supervisor) processAdjusterRequest(req ThrottleRequest) {
	s.logger.Debug("Processing throttle request",
		utils.String("resource_id", req.ResourceID),
		utils.Float64("current_load", req.CurrentLoad))

	shouldThrottle := req.CurrentLoad > 0.8
	newRate := 1.0
	if shouldThrottle {
		newRate = 0.5
	}

	req.ResponseChan <- ThrottleResponse{ShouldThrottle: shouldThrottle, NewRate: newRate}
	s.mu.Lock()
	s.stats.TotalMessages++
	s.mu.Unlock()
}

// MatchJob sends a job matching request to the matchmaker
func (s *Supervisor) MatchJob(jobID string, requirements interface{}) (string, error) {
	responseChan := make(chan JobMatchResponse, 1)

	select {
	case s.matchmakerQueue <- JobMatchRequest{JobID: jobID, Requirements: requirements, ResponseChan: responseChan}:
	case <-time.After(1 * time.Second):
		return "", utils.TimeoutError("match job")
	}

	select {
	case resp := <-responseChan:
		return resp.NodeID, resp.Error
	case <-time.After(5 * time.Second):
		return "", utils.TimeoutError("match job response")
	}
}

// CheckHealth sends a health check request to the watcher
func (s *Supervisor) CheckHealth(nodeID string) (bool, float64, error) {
	responseChan := make(chan HealthCheckResponse, 1)

	select {
	case s.watcherQueue <- HealthCheckRequest{NodeID: nodeID, ResponseChan: responseChan}:
	case <-time.After(1 * time.Second):
		return false, 0, utils.TimeoutError("check health")
	}

	select {
	case resp := <-responseChan:
		return resp.IsHealthy, resp.Load, nil
	case <-time.After(5 * time.Second):
		return false, 0, utils.TimeoutError("check health response")
	}
}

// RequestThrottle sends a throttle request to the adjuster
func (s *Supervisor) RequestThrottle(resourceID string, currentLoad float64) (bool, float64, error) {
	responseChan := make(chan ThrottleResponse, 1)

	select {
	case s.adjusterQueue <- ThrottleRequest{ResourceID: resourceID, CurrentLoad: currentLoad, ResponseChan: responseChan}:
	case <-time.After(1 * time.Second):
		return false, 0, utils.TimeoutError("request throttle")
	}

	select {
	case resp := <-responseChan:
		return resp.ShouldThrottle, resp.NewRate, nil
	case <-time.After(5 * time.Second):
		return false, 0, utils.TimeoutError("request throttle response")
	}
}

// runEconomyLoop waits for economy epoch signals (zero-CPU blocking)
// Replaces polling with Atomics.wait on IDX_ECONOMY_EPOCH
func (s *Supervisor) runEconomyLoop(ctx context.Context) error {
	s.logger.Info("Economy Loop started (signal-based)")

	var lastEpoch int32 = 0

	for {
		// Check for shutdown
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Wait for economy epoch change using Atomics.wait (zero CPU)
		_ = s.bridge.WaitForEpochChange(
			sab_layout.IDX_ECONOMY_EPOCH,
			lastEpoch,
			5000.0, // 5s max wait for shutdown responsiveness
		)

		// Read current epoch
		currentEpoch := s.bridge.ReadAtomicI32(sab_layout.IDX_ECONOMY_EPOCH)
		if currentEpoch != lastEpoch {
			s.logger.Debug("Economy epoch change detected, settling",
				utils.Int64("old", int64(lastEpoch)),
				utils.Int64("new", int64(currentEpoch)))

			if err := s.credits.OnEpoch(uint64(currentEpoch)); err != nil {
				s.logger.Error("Failed to settle economics", utils.Err(err))
			}

			lastEpoch = currentEpoch
		}
	}
}
