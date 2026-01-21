package learning_test

import (
	"testing"
	"time"

	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence/learning"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/stretchr/testify/assert"
)

// Helper function to create test environment
func createTestEnvironment() (*pattern.TieredPatternStorage, *intelligence.KnowledgeGraph) {
	testSAB := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabPtr := unsafe.Pointer(&testSAB[0])
	patterns := pattern.NewTieredPatternStorage(sabPtr, uint32(len(testSAB)), sab.OFFSET_PATTERN_EXCHANGE, sab.MAX_PATTERNS_INLINE)
	knowledge := intelligence.NewKnowledgeGraph(sabPtr, uint32(len(testSAB)), sab.OFFSET_COORDINATION, 1024)
	return patterns, knowledge
}

// Mock dispatcher for testing
type mockDispatcher struct {
	lastJob *foundation.Job
	result  *foundation.Result
}

func (m *mockDispatcher) ExecuteJob(job *foundation.Job) *foundation.Result {
	m.lastJob = job
	if m.result != nil {
		return m.result
	}
	return &foundation.Result{
		JobID:   job.ID,
		Success: true,
		Data:    []byte("mock result"),
	}
}

func (m *mockDispatcher) Dispatch(engine foundation.EngineType, operation string, parameters map[string]interface{}) (interface{}, error) {
	return nil, nil
}

// ========== SUCCESS CASES ==========

// TestLearningEngine_Creation validates engine creation
func TestLearningEngine_Creation(t *testing.T) {
	patterns, knowledge := createTestEnvironment()
	dispatcher := &mockDispatcher{}

	engine := learning.NewEnhancedLearningEngine(patterns, knowledge, dispatcher)

	assert.NotNil(t, engine)
}

// TestLearningEngine_Predict validates prediction functionality
func TestLearningEngine_Predict(t *testing.T) {
	patterns, knowledge := createTestEnvironment()
	dispatcher := &mockDispatcher{
		result: &foundation.Result{
			JobID:   "pred",
			Success: true,
			Data:    []byte("prediction result"),
		},
	}

	engine := learning.NewEnhancedLearningEngine(patterns, knowledge, dispatcher)

	ctx := &learning.PredictionContext{
		Type: foundation.PredictionLatency,
		Features: map[string]float32{
			"input_size": 1024.0,
		},
		Timeout: 1 * time.Second,
	}

	prediction, err := engine.Predict(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, prediction)
}

// TestLearningEngine_Learn validates learning functionality
func TestLearningEngine_Learn(t *testing.T) {
	patterns, knowledge := createTestEnvironment()
	dispatcher := &mockDispatcher{}

	engine := learning.NewEnhancedLearningEngine(patterns, knowledge, dispatcher)

	observation := &learning.Observation{
		Features:  map[string]float32{"f1": 1.0},
		Label:     true,
		Timestamp: time.Now(),
		Success:   true,
	}

	err := engine.Learn(observation)
	assert.NoError(t, err)

	// Wait for background channel processing
	time.Sleep(50 * time.Millisecond)

	// Verify data was stored in KnowledgeGraph
	nodes, err := knowledge.FindByType(foundation.NodeTypePrediction)
	assert.NoError(t, err)
	assert.Len(t, nodes, 1)
}

// TestLearningEngine_ModelConvergence verifies that predictions adapt to observations
func TestLearningEngine_ModelConvergence(t *testing.T) {
	patterns, knowledge := createTestEnvironment()
	dispatcher := &mockDispatcher{}
	engine := learning.NewEnhancedLearningEngine(patterns, knowledge, dispatcher)

	// 1. Initial prediction (should be baseline)
	ctx := &learning.PredictionContext{
		Type:     foundation.PredictionLatency,
		Features: map[string]float32{"f1": 10.0},
	}
	p1, _ := engine.Predict(ctx)

	// 2. Train with "high cost" for feature f1=10
	// 2. Train with "high cost" for feature f1=10
	for i := 0; i < 100; i++ {
		engine.Learn(&learning.Observation{
			Features: map[string]float32{"f1": 10.0},
			Success:  true, // Label 1.0
		})
	}

	// Wait for convergence
	time.Sleep(500 * time.Millisecond)

	// 3. New prediction for same feature
	p2, _ := engine.Predict(ctx)

	// 4. Verify p2 > p1 (adapting to success=1.0 which we used as cost proxy)
	assert.Greater(t, p2.Value.(float64), p1.Value.(float64), "Prediction should increase after training with high-value observations")
}

// TestLearningEngine_PredictResources validates resource prediction
func TestLearningEngine_PredictResources(t *testing.T) {
	patterns, knowledge := createTestEnvironment()
	dispatcher := &mockDispatcher{}

	engine := learning.NewEnhancedLearningEngine(patterns, knowledge, dispatcher)

	prediction := engine.PredictResources(1, []byte("input"))
	assert.NotNil(t, prediction)
	assert.GreaterOrEqual(t, prediction.CPU, float32(1.0))
	assert.Greater(t, prediction.Confidence, float32(0.0))
}

// TestLearningEngine_Stats validates stats collection
func TestLearningEngine_Stats(t *testing.T) {
	patterns, knowledge := createTestEnvironment()
	dispatcher := &mockDispatcher{}

	engine := learning.NewEnhancedLearningEngine(patterns, knowledge, dispatcher)

	// Make some predictions
	ctx := &learning.PredictionContext{
		Type:     foundation.PredictionLatency,
		Features: map[string]float32{"test": 1.0},
		Timeout:  1 * time.Second,
	}

	for i := 0; i < 5; i++ {
		_, _ = engine.Predict(ctx)
	}

	stats := engine.GetStats()
	assert.Equal(t, uint64(5), stats.PredictionsMade)
}

// ========== FAILURE CASES ==========

// TestLearningEngine_PredictWithoutKnowledge validates nil knowledge handling
func TestLearningEngine_PredictWithoutKnowledge(t *testing.T) {
	patterns, _ := createTestEnvironment()

	engine := learning.NewEnhancedLearningEngine(patterns, nil, &mockDispatcher{})

	ctx := &learning.PredictionContext{
		Type:     foundation.PredictionLatency,
		Features: map[string]float32{"test": 1.0},
		Timeout:  1 * time.Second,
	}

	prediction, err := engine.Predict(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, prediction)
}

// ========== EDGE CASES ==========

// TestLearningEngine_ConcurrentPredictions validates concurrent prediction handling
func TestLearningEngine_ConcurrentPredictions(t *testing.T) {
	patterns, knowledge := createTestEnvironment()
	engine := learning.NewEnhancedLearningEngine(patterns, knowledge, &mockDispatcher{})

	// Run 100 concurrent predictions
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			ctx := &learning.PredictionContext{
				Type:     foundation.PredictionLatency,
				Features: map[string]float32{"test": 1.0},
				Timeout:  1 * time.Second,
			}
			_, _ = engine.Predict(ctx)
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	stats := engine.GetStats()
	assert.Equal(t, uint64(100), stats.PredictionsMade)
}
