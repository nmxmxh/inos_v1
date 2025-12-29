package supervisor

import (
	"sync"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
)

// Channel types for non-blocking communication

// ControlChannel for control messages
type ControlChannel chan foundation.ControlMessage

// MetricsChannel for metrics reporting
type MetricsChannel chan *foundation.SupervisorMetrics

// ChannelSet groups all supervisor channels
type ChannelSet struct {
	Jobs    chan *foundation.Job
	Results chan *foundation.Result
	Control ControlChannel
	Metrics MetricsChannel
}

// NewChannelSet creates a new channel set
func NewChannelSet(bufferSize int) *ChannelSet {
	return &ChannelSet{
		Jobs:    make(chan *foundation.Job, bufferSize),
		Results: make(chan *foundation.Result, bufferSize),
		Control: make(ControlChannel, bufferSize),
		Metrics: make(MetricsChannel, bufferSize),
	}
}

// Close closes all channels
func (cs *ChannelSet) Close() {
	close(cs.Jobs)
	close(cs.Results)
	close(cs.Control)
	close(cs.Metrics)
}

// JobQueue is a thread-safe job queue
type JobQueue struct {
	jobs []*foundation.Job
	mu   sync.RWMutex
}

// NewJobQueue creates a new job queue
func NewJobQueue() *JobQueue {
	return &JobQueue{
		jobs: make([]*foundation.Job, 0),
	}
}

// Enqueue adds a job to the queue
func (jq *JobQueue) Enqueue(job *foundation.Job) {
	jq.mu.Lock()
	defer jq.mu.Unlock()
	jq.jobs = append(jq.jobs, job)
}

// Dequeue removes and returns the first job
func (jq *JobQueue) Dequeue() *foundation.Job {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	if len(jq.jobs) == 0 {
		return nil
	}

	job := jq.jobs[0]
	jq.jobs = jq.jobs[1:]
	return job
}

// Peek returns the first job without removing it
func (jq *JobQueue) Peek() *foundation.Job {
	jq.mu.RLock()
	defer jq.mu.RUnlock()

	if len(jq.jobs) == 0 {
		return nil
	}

	return jq.jobs[0]
}

// Len returns queue length
func (jq *JobQueue) Len() int {
	jq.mu.RLock()
	defer jq.mu.RUnlock()
	return len(jq.jobs)
}

// ResultCache caches results for deduplication
type ResultCache struct {
	cache map[string]*foundation.Result
	mu    sync.RWMutex
}

// NewResultCache creates a new result cache
func NewResultCache() *ResultCache {
	return &ResultCache{
		cache: make(map[string]*foundation.Result),
	}
}

// Get retrieves a cached result
func (rc *ResultCache) Get(jobID string) (*foundation.Result, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	result, exists := rc.cache[jobID]
	return result, exists
}

// Set stores a result in cache
func (rc *ResultCache) Set(jobID string, result *foundation.Result) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cache[jobID] = result
}

// Delete removes a result from cache
func (rc *ResultCache) Delete(jobID string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	delete(rc.cache, jobID)
}

// Clear clears the cache
func (rc *ResultCache) Clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cache = make(map[string]*foundation.Result)
}

// Size returns the cache size
func (rc *ResultCache) Size() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return len(rc.cache)
}

// Values returns all cached results
func (rc *ResultCache) Values() []*foundation.Result {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	values := make([]*foundation.Result, 0, len(rc.cache))
	for _, v := range rc.cache {
		values = append(values, v)
	}
	return values
}
