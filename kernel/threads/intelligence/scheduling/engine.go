package scheduling

import (
	"container/heap"
	"sync"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
)

// SchedulingEngine coordinates predictive scheduling
type SchedulingEngine struct {
	predictor *TimeSeriesPredictor
	scheduler *DeadlineScheduler
	allocator *ResourceAllocator
	dag       *DAGExecutor

	// Statistics
	jobsScheduled   uint64
	deadlinesMet    uint64
	deadlinesMissed uint64

	mu sync.RWMutex
}

// Job represents a schedulable job
type Job struct {
	ID           string
	Priority     foundation.Priority
	Deadline     time.Time
	Duration     time.Duration
	Resources    ResourceRequirements
	Dependencies []string

	// Predicted metrics
	PredictedLatency time.Duration
	FailureRisk      float64
}

type ResourceRequirements struct {
	CPU    float64 // CPU cores
	Memory uint64  // Bytes
	GPU    float64 // GPU fraction
}

func NewSchedulingEngine() *SchedulingEngine {
	return &SchedulingEngine{
		predictor: NewTimeSeriesPredictor(),
		scheduler: NewDeadlineScheduler(),
		allocator: NewResourceAllocator(),
		dag:       NewDAGExecutor(),
	}
}

// Schedule schedules a job
func (se *SchedulingEngine) Schedule(job *Job) *foundation.Decision {
	se.mu.Lock()
	se.jobsScheduled++
	se.mu.Unlock()

	// Predict job metrics
	job.PredictedLatency = se.predictor.PredictLatency(job)
	job.FailureRisk = se.predictor.PredictFailureRisk(job)

	// Check if deadline can be met
	canMeetDeadline := time.Now().Add(job.PredictedLatency).Before(job.Deadline)

	// Allocate resources
	allocation := se.allocator.Allocate(job.Resources)

	// Create schedule decision
	decision := &foundation.Decision{
		Type:       foundation.DecisionScheduling,
		Confidence: 1.0,
		Value: &ScheduleDecision{
			JobID:           job.ID,
			ScheduledTime:   time.Now(),
			Allocation:      allocation,
			CanMeetDeadline: canMeetDeadline,
			Priority:        se.calculatePriority(job),
		},
	}

	// Add to scheduler
	se.scheduler.Add(job, decision.Value.(*ScheduleDecision).Priority)

	return decision
}

// ScheduleDAG schedules a DAG of jobs
func (se *SchedulingEngine) ScheduleDAG(jobs []*Job) *DAGSchedule {
	return se.dag.Schedule(jobs)
}

// PredictLoad predicts future load
func (se *SchedulingEngine) PredictLoad(horizon time.Duration) []LoadPrediction {
	return se.predictor.PredictLoad(horizon)
}

// GetStats returns scheduling statistics
func (se *SchedulingEngine) GetStats() SchedulingStats {
	se.mu.RLock()
	defer se.mu.RUnlock()

	deadlineMeetRate := float64(0)
	if se.jobsScheduled > 0 {
		deadlineMeetRate = float64(se.deadlinesMet) / float64(se.jobsScheduled)
	}

	return SchedulingStats{
		JobsScheduled:    se.jobsScheduled,
		DeadlinesMet:     se.deadlinesMet,
		DeadlinesMissed:  se.deadlinesMissed,
		DeadlineMeetRate: deadlineMeetRate,
	}
}

type ScheduleDecision struct {
	JobID           string
	ScheduledTime   time.Time
	Allocation      *ResourceAllocation
	CanMeetDeadline bool
	Priority        float64
}

type SchedulingStats struct {
	JobsScheduled    uint64
	DeadlinesMet     uint64
	DeadlinesMissed  uint64
	DeadlineMeetRate float64
}

type LoadPrediction struct {
	Time       time.Time
	CPU        float64
	Memory     float64
	GPU        float64
	Confidence float64
}

// Helper: Calculate dynamic priority
func (se *SchedulingEngine) calculatePriority(job *Job) float64 {
	// EDF-based priority: closer deadline = higher priority
	timeToDeadline := time.Until(job.Deadline).Seconds()

	if timeToDeadline <= 0 {
		return 1000000.0 // Extremely high priority for overdue
	}

	// Priority = base_priority / time_to_deadline
	basePriority := float64(job.Priority)
	priority := basePriority * 1000.0 / timeToDeadline

	// Adjust for failure risk
	priority *= (1.0 + job.FailureRisk)

	return priority
}

// Priority queue for deadline scheduling
type JobQueue []*QueuedJob

type QueuedJob struct {
	Job      *Job
	Priority float64
	Index    int
}

func (pq JobQueue) Len() int { return len(pq) }

func (pq JobQueue) Less(i, j int) bool {
	// Higher priority first
	return pq[i].Priority > pq[j].Priority
}

func (pq JobQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

func (pq *JobQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*QueuedJob)
	item.Index = n
	*pq = append(*pq, item)
}

func (pq *JobQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.Index = -1
	*pq = old[0 : n-1]
	return item
}

// DeadlineScheduler implements EDF scheduling
type DeadlineScheduler struct {
	queue JobQueue
	mu    sync.Mutex
}

func NewDeadlineScheduler() *DeadlineScheduler {
	ds := &DeadlineScheduler{
		queue: make(JobQueue, 0),
	}
	heap.Init(&ds.queue)
	return ds
}

func (ds *DeadlineScheduler) Add(job *Job, priority float64) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	queuedJob := &QueuedJob{
		Job:      job,
		Priority: priority,
	}

	heap.Push(&ds.queue, queuedJob)
}

func (ds *DeadlineScheduler) GetNext() *Job {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.queue.Len() == 0 {
		return nil
	}

	queuedJob := heap.Pop(&ds.queue).(*QueuedJob)
	return queuedJob.Job
}
