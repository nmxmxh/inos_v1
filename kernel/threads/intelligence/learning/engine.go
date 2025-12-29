package learning

import (
	"fmt"
	"sync"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
)

// Dispatcher allows the engine to request job execution from its supervisor
type Dispatcher interface {
	ExecuteJob(job *foundation.Job) *foundation.Result
}

// EnhancedLearningEngine is now a lightweight overseer
// It delegates actual learning tasks to the Rust 'ml' module via the Unit Proxy system
type EnhancedLearningEngine struct {
	// Integration
	patterns   *pattern.TieredPatternStorage
	knowledge  *intelligence.KnowledgeGraph
	dispatcher Dispatcher

	// Stats
	stats LearningStats
	mu    sync.RWMutex
}

type LearningStats struct {
	PredictionsMade   uint64
	AvgPredictionTime time.Duration
}

// PredictionContext contains context for predictions
type PredictionContext struct {
	Type     foundation.PredictionType
	Features map[string]float32
	Timeout  time.Duration
}

// Prediction result
type Prediction struct {
	Value      interface{}
	Confidence float32
}

func NewEnhancedLearningEngine(
	patterns *pattern.TieredPatternStorage,
	knowledge *intelligence.KnowledgeGraph,
	dispatcher Dispatcher,
) *EnhancedLearningEngine {
	return &EnhancedLearningEngine{
		patterns:   patterns,
		knowledge:  knowledge,
		dispatcher: dispatcher,
	}
}

// Predict delegates to the ML module via the dispatcher
func (ele *EnhancedLearningEngine) Predict(context *PredictionContext) (*Prediction, error) {
	if ele.dispatcher == nil {
		return &Prediction{Confidence: 0}, fmt.Errorf("dispatcher not initialized")
	}

	// 1. Construct Job for ML module
	job := &foundation.Job{
		ID:        fmt.Sprintf("pred_%d", time.Now().UnixNano()),
		Type:      "ml",
		Operation: "inference.predict",
		Data:      nil, // Input data if any
		Parameters: map[string]interface{}{
			"prediction_type": context.Type,
			"features":        context.Features,
		},
		Deadline: time.Now().Add(context.Timeout),
	}

	// 2. Dispatch and wait for result
	result := ele.dispatcher.ExecuteJob(job)

	if !result.Success {
		return nil, fmt.Errorf("prediction failed: %s", result.Error)
	}

	// 3. Update stats
	ele.mu.Lock()
	ele.stats.PredictionsMade++
	ele.mu.Unlock()

	return &Prediction{
		Value:      result.Data,
		Confidence: 1.0, // Should extract from result data if available
	}, nil
}

// Learn delegates to the ML module
func (ele *EnhancedLearningEngine) Learn(observation interface{}) error {
	if ele.dispatcher == nil {
		return fmt.Errorf("dispatcher not initialized")
	}

	// Construct Job for ML module training
	job := &foundation.Job{
		ID:        fmt.Sprintf("learn_%d", time.Now().UnixNano()),
		Type:      "ml",
		Operation: "training.learn",
		Data:      nil, // We could serialize observation here
		Parameters: map[string]interface{}{
			"observation": observation,
		},
	}

	// Dispatch and ignore result for async training (or wait if needed)
	_ = ele.dispatcher.ExecuteJob(job)
	return nil
}

// PredictResources stub
func (ele *EnhancedLearningEngine) PredictResources(moduleID uint32, input []byte) *ResourcePrediction {
	return &ResourcePrediction{CPU: 1.0, Memory: 1024, GPU: 0, Confidence: 1.0}
}

// PredictLatency stub
func (ele *EnhancedLearningEngine) PredictLatency(moduleID uint32, supervisor uint8) time.Duration {
	return 0
}

// PredictFailure stub
func (ele *EnhancedLearningEngine) PredictFailure(moduleID uint32, context interface{}) float32 {
	return 0.0
}

func (ele *EnhancedLearningEngine) GetStats() LearningStats {
	ele.mu.RLock()
	defer ele.mu.RUnlock()
	return ele.stats
}

type ResourcePrediction struct {
	CPU        float32
	Memory     float32
	GPU        float32
	Confidence float32
}

// Observation represents a learning observation
type Observation struct {
	Features  map[string]float32
	Label     bool
	Timestamp time.Time
	Success   bool
}
