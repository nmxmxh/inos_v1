package learning

import (
	"fmt"
	"sync"
	"time"

	"github.com/cdipaolo/goml/base"
	"github.com/cdipaolo/goml/linear"
	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
)

// EnhancedLearningEngine implements online learning for kernel heuristics
type EnhancedLearningEngine struct {
	patterns   *pattern.TieredPatternStorage
	knowledge  *intelligence.KnowledgeGraph
	dispatcher foundation.Dispatcher

	// Models
	costModel       *linear.LeastSquares
	reputationModel *linear.Logistic

	// Fast adaptive channels
	costChan       chan base.Datapoint
	reputationChan chan base.Datapoint
	errChan        chan error

	// Training data buffers
	costDataX       [][]float64
	costDataY       []float64
	reputationDataX [][]float64
	reputationDataY []float64

	// Stats
	stats LearningStats
	mu    sync.RWMutex
}

const (
	FeatureCount = 3 // moduleID, size, priority
)

func NewEnhancedLearningEngine(
	patterns *pattern.TieredPatternStorage,
	knowledge *intelligence.KnowledgeGraph,
	dispatcher foundation.Dispatcher,
) *EnhancedLearningEngine {
	// Initialize models with 3 features (moduleID, size, priority)
	dummyX := [][]float64{{0, 0, 0}}
	dummyY := []float64{0}

	costModel := linear.NewLeastSquares(base.BatchGA, 0.0001, 0, 1, dummyX, dummyY)
	reputationModel := linear.NewLogistic(base.BatchGA, 0.0001, 0, 1, dummyX, dummyY)

	ele := &EnhancedLearningEngine{
		patterns:        patterns,
		knowledge:       knowledge,
		dispatcher:      dispatcher,
		costModel:       costModel,
		reputationModel: reputationModel,
		costChan:        make(chan base.Datapoint, 100),
		reputationChan:  make(chan base.Datapoint, 100),
		errChan:         make(chan error, 10),
	}

	go ele.runLearningLoop()

	go func() {
		for err := range ele.errChan {
			if err != nil {
				// Avoid noisy logs in tests unless critical
			}
		}
	}()

	return ele
}

func (ele *EnhancedLearningEngine) mapFeatures(input map[string]float32) []float64 {
	features := make([]float64, FeatureCount)
	// Map specific keys to indices
	features[0] = float64(input["moduleID"])
	features[1] = float64(input["size"])
	features[2] = float64(input["priority"])
	return features
}

// Predict estimates a value based on the relevant model
func (ele *EnhancedLearningEngine) Predict(context *PredictionContext) (*Prediction, error) {
	ele.mu.Lock()
	defer ele.mu.Unlock()
	ele.stats.PredictionsMade++

	features := ele.mapFeatures(context.Features)

	switch context.Type {
	case foundation.PredictionLatency, foundation.PredictionResource:
		val, err := ele.costModel.Predict(features)
		if err != nil {
			return nil, err
		}
		return &Prediction{Value: val[0], Confidence: 0.8}, nil

	default:
		// Fallback to reputation model for classification
		val, err := ele.reputationModel.Predict(features)
		if err != nil {
			return ele.predictFromKnowledge(context)
		}
		return &Prediction{Value: val[0], Confidence: 0.7}, nil
	}
}

func (ele *EnhancedLearningEngine) predictFromKnowledge(_ *PredictionContext) (*Prediction, error) {
	if ele.knowledge == nil {
		return &Prediction{Value: 0.5, Confidence: 0.1}, nil
	}
	nodes, err := ele.knowledge.FindByType(foundation.NodeTypePrediction)
	if err != nil || len(nodes) == 0 {
		return &Prediction{Value: 0.5, Confidence: 0.5}, nil
	}
	var sum float64
	for _, node := range nodes {
		sum += float64(node.Confidence)
	}
	return &Prediction{Value: sum / float64(len(nodes)), Confidence: 0.7}, nil
}

// Learn updates models with new observation data via channels
func (ele *EnhancedLearningEngine) Learn(observation interface{}) error {
	obs, ok := observation.(*Observation)
	if !ok {
		return fmt.Errorf("invalid observation type")
	}

	features := ele.mapFeatures(obs.Features)

	label := 0.0
	if obs.Success {
		label = 1.0
	}

	point := base.Datapoint{
		X: features,
		Y: []float64{label},
	}

	// Send to online learners (non-blocking if possible)
	select {
	case ele.costChan <- point:
	default:
		// Queue full, skip to maintain performance
	}

	select {
	case ele.reputationChan <- point:
	default:
	}

	// Persist to KnowledgeGraph if available
	if ele.knowledge != nil {
		nodeID := fmt.Sprintf("obs_%d", time.Now().UnixNano())
		ele.knowledge.AddNode(nodeID, foundation.NodeTypePrediction, float32(label), nil)
	}

	return nil
}

// PredictResources predicts CPU and Memory requirements
func (ele *EnhancedLearningEngine) PredictResources(moduleID uint32, input []byte) *ResourcePrediction {
	feats := map[string]float32{
		"moduleID": float32(moduleID),
		"size":     float32(len(input)),
	}
	features := ele.mapFeatures(feats)

	ele.mu.RLock()
	val, _ := ele.costModel.Predict(features)
	ele.mu.RUnlock()

	// Ensure positive mapping
	cpu := float32(val[0])
	if cpu < 1.0 {
		cpu = 1.0
	}

	return &ResourcePrediction{
		CPU:        cpu,
		Memory:     1024 * cpu, // 1GB per core scaled
		Confidence: 0.7,
	}
}

// PredictLatency calculates expected latency
func (ele *EnhancedLearningEngine) PredictLatency(moduleID uint32, supervisor uint8) time.Duration {
	feats := map[string]float32{
		"moduleID": float32(moduleID),
		"priority": float32(supervisor),
	}
	features := ele.mapFeatures(feats)

	ele.mu.RLock()
	val, _ := ele.costModel.Predict(features)
	ele.mu.RUnlock()

	ms := val[0]
	if ms < 10 {
		ms = 10
	}
	return time.Duration(ms) * time.Millisecond
}

// PredictFailure estimates risk
func (ele *EnhancedLearningEngine) PredictFailure(moduleID uint32, context interface{}) float32 {
	feats := map[string]float32{
		"moduleID": float32(moduleID),
	}
	features := ele.mapFeatures(feats)

	ele.mu.RLock()
	val, _ := ele.reputationModel.Predict(features)
	ele.mu.RUnlock()

	prob := float32(val[0])
	if prob > 1.0 {
		prob = 1.0
	}
	if prob < 0.0 {
		prob = 0.0
	}

	return 1.0 - prob // Failure risk is inverse of success probability
}

func (ele *EnhancedLearningEngine) GetStats() LearningStats {
	ele.mu.RLock()
	defer ele.mu.RUnlock()
	return ele.stats
}

type LearningStats struct {
	PredictionsMade   uint64
	AvgPredictionTime time.Duration
}

type PredictionContext struct {
	Type     foundation.PredictionType
	Features map[string]float32
	Timeout  time.Duration
}

type Prediction struct {
	Value      interface{}
	Confidence float32
}

type ResourcePrediction struct {
	CPU        float32
	Memory     float32
	GPU        float32
	Confidence float32
}

type Observation struct {
	Features  map[string]float32
	Label     bool
	Timestamp time.Time
	Success   bool
}

func (ele *EnhancedLearningEngine) runLearningLoop() {
	const MaxHistory = 1000

	for {
		select {
		case point := <-ele.costChan:
			ele.mu.Lock()
			ele.costDataX = append(ele.costDataX, point.X)
			ele.costDataY = append(ele.costDataY, point.Y[0])

			// Cap history
			if len(ele.costDataX) > MaxHistory {
				ele.costDataX = ele.costDataX[1:]
				ele.costDataY = ele.costDataY[1:]
			}

			if err := ele.costModel.UpdateTrainingSet(ele.costDataX, ele.costDataY); err != nil {
				ele.errChan <- err
			} else if err := ele.costModel.Learn(); err != nil {
				ele.errChan <- err
			}
			ele.mu.Unlock()

		case point := <-ele.reputationChan:
			ele.mu.Lock()
			ele.reputationDataX = append(ele.reputationDataX, point.X)
			ele.reputationDataY = append(ele.reputationDataY, point.Y[0])

			// Cap history
			if len(ele.reputationDataX) > MaxHistory {
				ele.reputationDataX = ele.reputationDataX[1:]
				ele.reputationDataY = ele.reputationDataY[1:]
			}

			if err := ele.reputationModel.UpdateTrainingSet(ele.reputationDataX, ele.reputationDataY); err != nil {
				ele.errChan <- err
			} else if err := ele.reputationModel.Learn(); err != nil {
				ele.errChan <- err
			}
			ele.mu.Unlock()
		}
	}
}
