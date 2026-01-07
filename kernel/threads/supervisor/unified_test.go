package supervisor_test

import (
	"context"
	"testing"
	"time"

	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create test SAB with pattern storage and knowledge graph
func createTestEnvironment() ([]byte, *pattern.TieredPatternStorage, *intelligence.KnowledgeGraph) {
	sabSize := uint32(sab.SAB_SIZE_DEFAULT)
	testSAB := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&testSAB[0])
	patterns := pattern.NewTieredPatternStorage(sabPtr, sabSize, sab.OFFSET_PATTERN_EXCHANGE, sab.MAX_PATTERNS_INLINE)
	knowledge := intelligence.NewKnowledgeGraph(sabPtr, sabSize, sab.OFFSET_COORDINATION, 1024)
	return testSAB, patterns, knowledge
}

// ========== SUCCESS CASES ==========

// TestUnifiedSupervisor_Creation validates supervisor creation
func TestUnifiedSupervisor_Creation(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	testCases := []struct {
		name         string
		capabilities []string
	}{
		{"Audio", []string{"encode", "decode", "fft"}},
		{"Crypto", []string{"hash", "sign", "encrypt"}},
		{"GPU", []string{"matmul", "shader", "compute"}},
		{"Data", []string{"compress", "parse", "transform"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sup := supervisor.NewUnifiedSupervisor(tc.name, tc.capabilities, patterns, knowledge)

			assert.NotNil(t, sup)
			assert.Equal(t, tc.capabilities, sup.Capabilities())
			assert.True(t, sup.SupportsOperation(tc.capabilities[0]))
		})
	}
}

// TestUnifiedSupervisor_StartStop validates lifecycle
func TestUnifiedSupervisor_StartStop(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("test", []string{"test"}, patterns, knowledge)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start
	err := sup.Start(ctx)
	assert.NoError(t, err)

	// Verify running
	health := sup.Health()
	assert.NotNil(t, health)

	// Stop
	err = sup.Stop()
	assert.NoError(t, err)
}

// TestUnifiedSupervisor_JobSubmission validates job submission
func TestUnifiedSupervisor_JobSubmission(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("test", []string{"test"}, patterns, knowledge)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := sup.Start(ctx)
	require.NoError(t, err)
	defer sup.Stop()

	// Submit job
	job := &foundation.Job{
		ID:        "test-job-1",
		Type:      "test",
		Operation: "test",
		Data:      []byte("test data"),
		Parameters: map[string]interface{}{
			"param1": "value1",
		},
		Deadline: time.Now().Add(1 * time.Second),
	}

	resultChan, err := sup.Submit(job)
	assert.NoError(t, err)
	assert.NotNil(t, resultChan)

	// Wait for result (with timeout)
	select {
	case result := <-resultChan:
		assert.NotNil(t, result)
	case <-time.After(2 * time.Second):
		t.Error("Job execution timeout")
	}
}

// TestUnifiedSupervisor_BatchSubmission validates batch job submission
func TestUnifiedSupervisor_BatchSubmission(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("test", []string{"test"}, patterns, knowledge)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := sup.Start(ctx)
	require.NoError(t, err)
	defer sup.Stop()

	// Create batch of jobs
	jobs := make([]*foundation.Job, 5)
	for i := 0; i < 5; i++ {
		jobs[i] = &foundation.Job{
			ID:        string(rune('a' + i)),
			Type:      "test",
			Operation: "test",
			Data:      []byte("test"),
			Deadline:  time.Now().Add(5 * time.Second),
		}
	}

	resultChan, err := sup.SubmitBatch(jobs)
	assert.NoError(t, err)
	assert.NotNil(t, resultChan)

	// Wait for results
	select {
	case results := <-resultChan:
		assert.Equal(t, 5, len(results))
	case <-time.After(6 * time.Second):
		t.Error("Batch execution timeout")
	}
}

// TestUnifiedSupervisor_Learning validates learning functionality
func TestUnifiedSupervisor_Learning(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("test", []string{"test"}, patterns, knowledge)

	job := &foundation.Job{
		ID:        "learn-job",
		Type:      "test",
		Operation: "test",
	}

	result := &foundation.Result{
		JobID:   "learn-job",
		Success: true,
		Data:    []byte("result"),
	}

	err := sup.Learn(job, result)
	assert.NoError(t, err)
}

// TestUnifiedSupervisor_Optimization validates optimization
func TestUnifiedSupervisor_Optimization(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("test", []string{"test"}, patterns, knowledge)

	job := &foundation.Job{
		ID:        "opt-job",
		Type:      "test",
		Operation: "test",
		Parameters: map[string]interface{}{
			"quality": 0.5,
		},
	}

	optResult, err := sup.Optimize(job)
	assert.NoError(t, err)
	assert.NotNil(t, optResult)
}

// TestUnifiedSupervisor_Prediction validates prediction
func TestUnifiedSupervisor_Prediction(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("test", []string{"test"}, patterns, knowledge)

	job := &foundation.Job{
		ID:        "pred-job",
		Type:      "test",
		Operation: "test",
	}

	prediction, err := sup.Predict(job)
	assert.NoError(t, err)
	assert.NotNil(t, prediction)
}

// TestUnifiedSupervisor_Metrics validates metrics collection
func TestUnifiedSupervisor_Metrics(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("test", []string{"test"}, patterns, knowledge)

	metrics := sup.Metrics()
	assert.NotNil(t, metrics)
}

// ========== FAILURE CASES ==========

// TestUnifiedSupervisor_InvalidJob validates invalid job handling
func TestUnifiedSupervisor_InvalidJob(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("test", []string{"test"}, patterns, knowledge)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := sup.Start(ctx)
	require.NoError(t, err)
	defer sup.Stop()

	// Submit invalid job (nil)
	_, err = sup.Submit(nil)
	assert.Error(t, err)

	// Submit job with invalid operation
	invalidJob := &foundation.Job{
		ID:        "invalid",
		Type:      "nonexistent",
		Operation: "invalid",
		Deadline:  time.Now().Add(1 * time.Second),
	}

	resultChan2, err := sup.Submit(invalidJob)
	assert.NoError(t, err)
	select {
	case result := <-resultChan2:
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "Capability not supported")
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for invalid job result")
	}
}

// TestUnifiedSupervisor_ExpiredDeadline validates deadline handling
func TestUnifiedSupervisor_ExpiredDeadline(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("test", []string{"test"}, patterns, knowledge)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := sup.Start(ctx)
	require.NoError(t, err)
	defer sup.Stop()

	// Submit job with already expired deadline
	expiredJob := &foundation.Job{
		ID:        "expired",
		Type:      "test",
		Operation: "test",
		Deadline:  time.Now().Add(-1 * time.Second), // Already expired
	}

	resultChan, err := sup.Submit(expiredJob)
	if err == nil {
		select {
		case result := <-resultChan:
			assert.False(t, result.Success)
		case <-time.After(2 * time.Second):
			t.Error("Timeout waiting for expired job result")
		}
	}
}

// ========== EDGE CASES ==========

// TestUnifiedSupervisor_ConcurrentSubmissions validates concurrent job handling
func TestUnifiedSupervisor_ConcurrentSubmissions(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("test", []string{"test"}, patterns, knowledge)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := sup.Start(ctx)
	require.NoError(t, err)
	defer sup.Stop()

	// Submit 100 jobs concurrently
	numJobs := 100
	resultChans := make([]<-chan *foundation.Result, numJobs)

	for i := 0; i < numJobs; i++ {
		job := &foundation.Job{
			ID:        string(rune(i)),
			Type:      "test",
			Operation: "test",
			Deadline:  time.Now().Add(5 * time.Second),
		}

		ch, err := sup.Submit(job)
		assert.NoError(t, err)
		resultChans[i] = ch
	}

	// Collect all results
	successCount := 0
	for _, ch := range resultChans {
		select {
		case result := <-ch:
			if result.Success {
				successCount++
			}
		case <-time.After(6 * time.Second):
			t.Error("Timeout waiting for concurrent job")
		}
	}

	// At least some should succeed
	assert.Greater(t, successCount, 0)
}

// TestUnifiedSupervisor_CapabilityCheck validates capability checking
func TestUnifiedSupervisor_CapabilityCheck(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	capabilities := []string{"encode", "decode", "transform"}
	sup := supervisor.NewUnifiedSupervisor("test", capabilities, patterns, knowledge)

	// Test supported operations
	for _, cap := range capabilities {
		assert.True(t, sup.SupportsOperation(cap))
	}

	// Test unsupported operation
	assert.False(t, sup.SupportsOperation("nonexistent"))
}

// TestUnifiedSupervisor_HealthMonitoring validates health monitoring
func TestUnifiedSupervisor_HealthMonitoring(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("test", []string{"test"}, patterns, knowledge)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := sup.Start(ctx)
	require.NoError(t, err)
	defer sup.Stop()

	// Monitor health
	err = sup.Monitor(ctx)
	assert.NoError(t, err)

	// Check health status
	health := sup.Health()
	assert.NotNil(t, health)
}

// TestUnifiedSupervisor_AnomalyDetection validates anomaly detection
func TestUnifiedSupervisor_AnomalyDetection(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("test", []string{"test"}, patterns, knowledge)

	anomalies := sup.Anomalies()
	assert.NotNil(t, anomalies)
}

// ========== INTEGRATION TESTS ==========

// TestUnifiedSupervisor_FullWorkflow validates complete workflow
func TestUnifiedSupervisor_FullWorkflow(t *testing.T) {
	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("workflow", []string{"test"}, patterns, knowledge)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Start supervisor
	err := sup.Start(ctx)
	require.NoError(t, err)
	defer sup.Stop()

	// 2. Submit job
	job := &foundation.Job{
		ID:        "workflow-job",
		Type:      "test",
		Operation: "test",
		Data:      []byte("workflow data"),
		Deadline:  time.Now().Add(5 * time.Second),
	}

	resultChan, err := sup.Submit(job)
	require.NoError(t, err)

	// 3. Wait for result
	var result *foundation.Result
	select {
	case result = <-resultChan:
		assert.NotNil(t, result)
	case <-time.After(6 * time.Second):
		t.Fatal("Workflow timeout")
	}

	// 4. Learn from result
	err = sup.Learn(job, result)
	assert.NoError(t, err)

	// 5. Optimize based on learning
	optResult, err := sup.Optimize(job)
	assert.NoError(t, err)
	assert.NotNil(t, optResult)

	// 6. Make prediction
	prediction, err := sup.Predict(job)
	assert.NoError(t, err)
	assert.NotNil(t, prediction)

	// 7. Check metrics
	metrics := sup.Metrics()
	assert.NotNil(t, metrics)

	// 8. Check health
	health := sup.Health()
	assert.NotNil(t, health)
}

// ========== PERFORMANCE TESTS ==========

// TestUnifiedSupervisor_Throughput validates job throughput
func TestUnifiedSupervisor_Throughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping throughput test in short mode")
	}

	_, patterns, knowledge := createTestEnvironment()

	sup := supervisor.NewUnifiedSupervisor("throughput", []string{"test"}, patterns, knowledge)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := sup.Start(ctx)
	require.NoError(t, err)
	defer sup.Stop()

	// Submit 1000 jobs
	numJobs := 1000
	start := time.Now()

	for i := 0; i < numJobs; i++ {
		job := &foundation.Job{
			ID:        string(rune(i)),
			Type:      "test",
			Operation: "test",
			Deadline:  time.Now().Add(30 * time.Second),
		}

		_, err := sup.Submit(job)
		assert.NoError(t, err)
	}

	elapsed := time.Since(start)
	throughput := float64(numJobs) / elapsed.Seconds()

	t.Logf("Throughput: %.2f jobs/sec", throughput)
	assert.Greater(t, throughput, 100.0, "Should handle > 100 jobs/sec")
}
