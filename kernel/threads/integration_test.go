package threads

import (
	"context"
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_LoopOfIntelligence(t *testing.T) {
	// 1. Setup Environment
	sab := make([]byte, 2*1024*1024)
	// knowledge base offset (e.g. at 1MB)
	storage := pattern.NewTieredPatternStorage(sab, 0, 1024*1024)
	knowledge := intelligence.NewKnowledgeGraph(sab, 1024*1024, 1024)

	// Initialize Supervisor with storage and knowledge
	us := supervisor.NewUnifiedSupervisor(
		"test-node",
		[]string{"compute", "ml"},
		storage,
		knowledge,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := us.Start(ctx)
	require.NoError(t, err)
	defer us.Stop()

	// 2. Step 1: Pattern Discovery
	// We simulate a series of successes that the PatternDetector should pick up.
	pd := pattern.NewPatternDetector(
		pattern.NewPatternValidator(),
		pattern.NewPatternPublisher(storage),
	)

	for i := 0; i < 15; i++ {
		pd.Observe("compute-op", pattern.Observation{
			Success:   true,
			Latency:   50 * time.Millisecond,
			Timestamp: time.Now(),
		})
	}

	// Trigger detection - this writes a pattern to the shared storage
	patterns := pd.DetectPatterns()
	require.NotEmpty(t, patterns, "Should have detected a new pattern")

	// 3. Step 2: Learning & Evolution
	// The supervisor's learning engine updates models based on job results.
	job1 := &foundation.Job{
		ID:        "j1",
		Operation: "compute",
		Features:  map[string]float64{"moduleID": 1.0, "size": 100.0, "priority": 1.0},
	}
	res1 := &foundation.Result{Success: true}

	err = us.Learn(job1, res1)
	assert.NoError(t, err, "Learning should succeed")

	// 4. Step 3: Predictive Scheduling
	// Submit a job and ensure it processes correctly using the evolved models.
	job2 := &foundation.Job{
		ID:        "j-predictive",
		Operation: "compute",
		Priority:  int(foundation.PriorityHigh),
		Deadline:  time.Now().Add(5 * time.Second),
		Features:  map[string]float64{"moduleID": 1.0, "size": 200.0, "priority": 2.0},
	}

	resChan, err := us.Submit(job2)
	require.NoError(t, err)

	select {
	case res := <-resChan:
		assert.True(t, res.Success)
		assert.Equal(t, job2.ID, res.JobID)
	case <-time.After(2 * time.Second):
		t.Fatal("Job execution timed out")
	}

	// 5. Verify Metrics & Health
	metrics := us.Metrics()
	assert.Equal(t, uint64(1), metrics.JobsCompleted)
	assert.Greater(t, metrics.AverageLatency, time.Duration(0))

	health := us.Health()
	assert.True(t, health.Healthy)
}
