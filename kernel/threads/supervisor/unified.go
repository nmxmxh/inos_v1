package supervisor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence/health"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence/learning"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence/optimization"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence/scheduling"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence/security"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
)

// UnifiedSupervisor is the base supervisor implementation
type UnifiedSupervisor struct {
	name         string
	capabilities []string
	delegator    foundation.MeshDelegator // Mesh offloading capability
	bridge       SABInterface             // SAB bridge for epoch-based signaling

	// Intelligence engines (from Week 1-2)
	learning  *learning.EnhancedLearningEngine
	optimizer *optimization.OptimizationEngine
	scheduler *scheduling.SchedulingEngine
	security  *security.SecurityEngine
	healthMon *health.HealthMonitor

	// Channels
	channels *ChannelSet

	// State
	running       atomic.Bool
	jobsSubmitted atomic.Uint64
	jobsCompleted atomic.Uint64
	jobsFailed    atomic.Uint64

	// Queues and caches
	jobQueue    *JobQueue
	resultCache *ResultCache

	// Metrics
	latencies []time.Duration
	mu        sync.RWMutex

	// Epoch-Based Loop Tracking (v1.10+)
	lastSystemEpoch        int32 // Last seen system epoch
	lastCleanupEpoch       int32 // Epoch at last cleanup
	monitorEpochThreshold  int32 // Run monitor every N epochs (default 10)
	learningEpochThreshold int32 // Run learning every N epochs (default 1000)
	healthEpochThreshold   int32 // Run health every N epochs (default 100)

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewUnifiedSupervisor creates a new unified supervisor
func NewUnifiedSupervisor(
	name string,
	capabilities []string,
	patterns *pattern.TieredPatternStorage,
	knowledge *intelligence.KnowledgeGraph,
	delegator foundation.MeshDelegator,
	bridge SABInterface,
) *UnifiedSupervisor {
	us := &UnifiedSupervisor{
		name:         name,
		capabilities: capabilities,
		delegator:    delegator,
		bridge:       bridge,
		optimizer:    optimization.NewOptimizationEngine(),
		scheduler:    scheduling.NewSchedulingEngine(),
		security:     security.NewSecurityEngine(),
		healthMon:    health.NewHealthMonitor(),
		channels:     NewChannelSet(100),
		jobQueue:     NewJobQueue(),
		resultCache:  NewResultCache(),
		latencies:    make([]time.Duration, 0, 1000),
		// Epoch thresholds (activity-based, not time-based)
		monitorEpochThreshold:  10,   // ~10 operations between monitor checks
		learningEpochThreshold: 1000, // ~1000 operations between learning updates
		healthEpochThreshold:   100,  // ~100 operations between health checks
	}
	// Initialize engines with coordinator and bridge access
	us.learning = learning.NewEnhancedLearningEngine(patterns, knowledge, us)
	return us
}

// Start starts the supervisor
func (us *UnifiedSupervisor) Start(ctx context.Context) error {
	if us.running.Load() {
		return fmt.Errorf("supervisor already running")
	}

	us.ctx, us.cancel = context.WithCancel(ctx)
	us.running.Store(true)

	// Start goroutines for different responsibilities
	us.wg.Add(4)
	go us.monitorLoop()
	go us.scheduleLoop()
	go us.learningLoop()
	go us.healthLoop()

	// BLOCK until context cancelled (spawnChild expects blocking)
	<-us.ctx.Done()
	return nil
}

// Stop stops the supervisor
func (us *UnifiedSupervisor) Stop() error {
	if !us.running.Load() {
		return fmt.Errorf("supervisor not running")
	}

	us.cancel()
	us.running.Store(false)

	// Wait for goroutines to finish
	us.wg.Wait()

	// Close channels
	us.channels.Close()

	return nil
}

// Submit queues a job for execution
func (us *UnifiedSupervisor) Submit(job *foundation.Job) (<-chan *foundation.Result, error) {
	// Validate job
	if job == nil {
		return nil, fmt.Errorf("job cannot be nil")
	}

	if !us.running.Load() {
		return nil, fmt.Errorf("supervisor not running")
	}

	// Check if job is already expired
	if !job.Deadline.IsZero() && time.Now().After(job.Deadline) {
		resultChan := make(chan *foundation.Result, 1)
		resultChan <- &foundation.Result{
			JobID:   job.ID,
			Success: false,
			Error:   "job deadline already expired",
		}
		close(resultChan)
		return resultChan, nil
	}

	// Create result channel
	resultChan := make(chan *foundation.Result, 1)
	job.ResultChan = resultChan
	job.SubmittedAt = time.Now()

	// Increment counter
	us.jobsSubmitted.Add(1)

	// Non-blocking send to job channel
	select {
	case us.channels.Jobs <- job:
		return resultChan, nil
	case <-time.After(100 * time.Millisecond):
		return nil, fmt.Errorf("job queue full")
	}
}

// SubmitBatch submits multiple jobs
func (us *UnifiedSupervisor) SubmitBatch(jobs []*foundation.Job) (<-chan []*foundation.Result, error) {
	resultsChan := make(chan []*foundation.Result, 1)

	go func() {
		results := make([]*foundation.Result, len(jobs))
		var wg sync.WaitGroup

		for i, job := range jobs {
			wg.Add(1)
			go func(idx int, j *foundation.Job) {
				defer wg.Done()
				resChan, err := us.Submit(j)
				if err != nil {
					results[idx] = &foundation.Result{
						JobID:   j.ID,
						Success: false,
						Error:   err.Error(),
					}
					return
				}
				results[idx] = <-resChan
			}(i, job)
		}

		wg.Wait()
		resultsChan <- results
	}()

	return resultsChan, nil
}

// Learn learns from job execution
func (us *UnifiedSupervisor) Learn(job *foundation.Job, result *foundation.Result) error {
	// Delegate to learning engine
	features32 := make(map[string]float32)
	for k, v := range job.Features {
		features32[k] = float32(v)
	}

	observation := &learning.Observation{
		Features:  features32,
		Label:     result.Success,
		Timestamp: time.Now(),
		Success:   result.Success,
	}
	return us.learning.Learn(observation)
}

// Optimize optimizes job execution parameters
func (us *UnifiedSupervisor) Optimize(job *foundation.Job) (*foundation.OptimizationResult, error) {
	// Use optimization engine for parameter evolution
	// Map job parameters to float64 map if necessary
	params := make(map[string]float64)
	for k, v := range job.Parameters {
		if f, ok := v.(float64); ok {
			params[k] = f
		}
	}

	res := us.optimizer.EvolveParameters(nil, nil, 10) // Placeholder for actual optimization logic
	return &foundation.OptimizationResult{
		Parameters: res,
		Score:      1.0,
		Iterations: 10,
	}, nil
}

// Predict makes performance predictions
func (us *UnifiedSupervisor) Predict(job *foundation.Job) (*foundation.Prediction, error) {
	// Use scheduler's predictor or learning engine
	return &foundation.Prediction{
		Latency:     100 * time.Millisecond, // Predictive placeholder
		FailureRisk: 0.05,
		Confidence:  0.9,
	}, nil
}

// Schedule handles job scheduling
func (us *UnifiedSupervisor) Schedule(job *foundation.Job) error {
	// Use scheduling engine
	sJob := &scheduling.Job{
		ID:        job.ID,
		Priority:  foundation.Priority(job.Priority),
		Deadline:  job.Deadline,
		Resources: scheduling.ResourceRequirements{CPU: 1.0, Memory: 1024},
	}
	us.scheduler.Schedule(sJob)
	return nil
}

// Secure enforces security policies
func (us *UnifiedSupervisor) Secure(job *foundation.Job) error {
	// Use security engine for anomaly detection
	req := &security.SecurityRequest{
		Source:   job.Source,
		Data:     job.Data,
		Features: job.Features,
	}
	decision := us.security.Analyze(req)
	if !decision.Allow {
		return fmt.Errorf("security policy violation")
	}
	return nil
}

// Monitor executes health monitoring
func (us *UnifiedSupervisor) Monitor(ctx context.Context) error {
	metrics := us.collectHealthMetrics()
	analysis := us.healthMon.Analyze(metrics)
	if !analysis.Healthy {
		return fmt.Errorf("health degradation detected")
	}
	return nil
}

// Coordinate coordinates with other supervisors (Mesh Offloading)
func (us *UnifiedSupervisor) Coordinate(ctx context.Context, job *foundation.Job) (*foundation.Result, error) {
	if us.delegator == nil {
		return nil, fmt.Errorf("mesh delegator not available for supervisor: %s", us.name)
	}
	return us.delegator.DelegateJob(ctx, job)
}

func (us *UnifiedSupervisor) SharePatterns(patterns []*pattern.EnhancedPattern) error {
	// TODO: Delegate to EnhancedLearningEngine sharing mechanism
	return nil
}

// Health returns health status
func (us *UnifiedSupervisor) Health() *foundation.HealthStatus {
	// Base health reporting
	submitted := us.jobsSubmitted.Load()
	completed := us.jobsCompleted.Load()
	failed := us.jobsFailed.Load()

	errorRate := 0.0
	if submitted > 0 {
		errorRate = float64(failed) / float64(submitted)
	}

	return &foundation.HealthStatus{
		Healthy:       errorRate < 0.1,
		Issues:        make([]string, 0),
		LastCheck:     time.Now(),
		JobsProcessed: completed,
		ErrorRate:     errorRate,
	}
}

// Metrics returns supervisor metrics
func (us *UnifiedSupervisor) Metrics() *foundation.SupervisorMetrics {
	// Base metrics reporting
	us.mu.RLock()
	defer us.mu.RUnlock()

	avgLatency := time.Duration(0)
	if len(us.latencies) > 0 {
		total := time.Duration(0)
		for _, lat := range us.latencies {
			total += lat
		}
		avgLatency = total / time.Duration(len(us.latencies))
	}

	return &foundation.SupervisorMetrics{
		JobsSubmitted:  us.jobsSubmitted.Load(),
		JobsCompleted:  us.jobsCompleted.Load(),
		JobsFailed:     us.jobsFailed.Load(),
		AverageLatency: avgLatency,
		QueueDepth:     us.jobQueue.Len(),
	}
}

// Anomalies returns detected anomalies
func (us *UnifiedSupervisor) Anomalies() []string {
	// Pull from health monitor or security engine
	return make([]string, 0)
}

// Capabilities returns supervisor capabilities
func (us *UnifiedSupervisor) Capabilities() []string {
	return us.capabilities
}

// SupportsOperation checks if operation is supported
func (us *UnifiedSupervisor) SupportsOperation(op string) bool {
	for _, cap := range us.capabilities {
		if cap == op {
			return true
		}
	}
	return false
}

// Goroutine loops
// v1.10+: Epoch-driven loops replace time-based tickers
// Zero CPU when idle - activity-proportional maintenance

func (us *UnifiedSupervisor) monitorLoop() {
	defer us.wg.Done()

	var lastEpoch int32 = 0
	var monitorEpoch int32 = 0

	for {
		// If bridge is nil (e.g., in tests), fall back to time-based
		if us.bridge == nil {
			select {
			case <-us.ctx.Done():
				return
			case <-time.After(1 * time.Second):
				us.Monitor(us.ctx)
			}
			continue
		}

		// Epoch-driven: Wait for activity
		select {
		case <-us.ctx.Done():
			return
		case <-us.bridge.WaitForEpochAsync(sab_layout.IDX_SYSTEM_EPOCH, lastEpoch):
			// Epoch changed, update and check threshold
			currentEpoch := us.bridge.ReadAtomicI32(sab_layout.IDX_SYSTEM_EPOCH)
			if currentEpoch-monitorEpoch >= us.monitorEpochThreshold {
				us.Monitor(us.ctx)
				monitorEpoch = currentEpoch
			}
			lastEpoch = currentEpoch
		}
	}
}

func (us *UnifiedSupervisor) scheduleLoop() {
	defer us.wg.Done()

	for {
		select {
		case <-us.ctx.Done():
			return
		case job := <-us.channels.Jobs:
			// Process job
			us.processJob(job)
		}
	}
}

func (us *UnifiedSupervisor) learningLoop() {
	defer us.wg.Done()

	var lastEpoch int32 = 0
	var learnEpoch int32 = 0

	for {
		// If bridge is nil (e.g., in tests), fall back to time-based
		if us.bridge == nil {
			select {
			case <-us.ctx.Done():
				return
			case <-time.After(1 * time.Minute):
				// Periodic learning updates (placeholder)
			}
			continue
		}

		// Epoch-driven: Wait for activity
		select {
		case <-us.ctx.Done():
			return
		case <-us.bridge.WaitForEpochAsync(sab_layout.IDX_SYSTEM_EPOCH, lastEpoch):
			currentEpoch := us.bridge.ReadAtomicI32(sab_layout.IDX_SYSTEM_EPOCH)
			if currentEpoch-learnEpoch >= us.learningEpochThreshold {
				// Periodic learning updates
				// 1. Scan for new patterns from Rust modules (SAB)
				// 2. Analyze patterns and update models
				learnEpoch = currentEpoch
			}
			lastEpoch = currentEpoch
		}
	}
}

func (us *UnifiedSupervisor) healthLoop() {
	defer us.wg.Done()

	var lastEpoch int32 = 0
	var healthEpoch int32 = 0

	for {
		// If bridge is nil (e.g., in tests), fall back to time-based
		if us.bridge == nil {
			select {
			case <-us.ctx.Done():
				return
			case <-time.After(30 * time.Second):
				us.Monitor(us.ctx)
			}
			continue
		}

		// Epoch-driven: Wait for activity
		select {
		case <-us.ctx.Done():
			return
		case <-us.bridge.WaitForEpochAsync(sab_layout.IDX_SYSTEM_EPOCH, lastEpoch):
			currentEpoch := us.bridge.ReadAtomicI32(sab_layout.IDX_SYSTEM_EPOCH)
			if currentEpoch-healthEpoch >= us.healthEpochThreshold {
				us.Monitor(us.ctx)
				healthEpoch = currentEpoch
			}
			lastEpoch = currentEpoch
		}
	}
}

// Helper methods

func (us *UnifiedSupervisor) processJob(job *foundation.Job) {
	startTime := time.Now()

	// Security check
	if !us.validateJob(job) {
		us.jobsFailed.Add(1)
		job.ResultChan <- &foundation.Result{
			JobID:   job.ID,
			Success: false,
			Error:   "Security validation failed",
		}
		return
	}

	// Execute job (to be overridden by unit supervisors)
	result := us.ExecuteJob(job)

	// Record metrics
	latency := time.Since(startTime)
	us.recordLatency(latency)

	if result.Success {
		us.jobsCompleted.Add(1)
	} else {
		us.jobsFailed.Add(1)
	}

	// Send result
	job.ResultChan <- result
}

func (us *UnifiedSupervisor) validateJob(job *foundation.Job) bool {
	if err := us.Secure(job); err != nil {
		return false
	}
	return true
}

func (us *UnifiedSupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	// Base implementation - validate capabilities
	if !us.SupportsOperation(job.Operation) {
		return &foundation.Result{
			JobID:   job.ID,
			Success: false,
			Error:   fmt.Sprintf("Capability not supported: %s", job.Operation),
		}
	}

	return &foundation.Result{
		JobID:   job.ID,
		Success: true,
		Data:    job.Data,
		Latency: 0,
	}
}

func (us *UnifiedSupervisor) recordLatency(latency time.Duration) {
	us.mu.Lock()
	defer us.mu.Unlock()

	us.latencies = append(us.latencies, latency)

	// Keep only last 1000 latencies
	if len(us.latencies) > 1000 {
		us.latencies = us.latencies[1:]
	}
}

func (us *UnifiedSupervisor) collectHealthMetrics() *health.HealthMetrics {
	metrics := us.Metrics()

	return &health.HealthMetrics{
		CPU:        0.5, // Placeholder
		Memory:     0.6, // Placeholder
		Latency:    metrics.AverageLatency,
		ErrorRate:  float64(metrics.JobsFailed) / float64(metrics.JobsSubmitted),
		Throughput: float64(metrics.JobsCompleted),
		Timestamp:  time.Now(),
	}
}
