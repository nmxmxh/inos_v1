package intelligence

import (
	"fmt"
	"math"
	"sync"

	"github.com/cdipaolo/goml/cluster"
)

const (
	FeatureCount = 3 // Standard feature count matching learning engine
)

// NoveltyScorer uses unsupervised learning to detect novel state patterns
// It implements the "Gaming is allowed if it leads to creativity" philosophy
type NoveltyScorer struct {
	model *cluster.KMeans

	// Observations for periodic retraining
	observations [][]float64
	maxObs       int

	mu sync.RWMutex
}

// NewNoveltyScorer creates a scorer that detects novelty via clustering drift
func NewNoveltyScorer(clusters int) *NoveltyScorer {
	// Initialize with dummy data matching cluster count and feature count
	dummyData := make([][]float64, clusters)
	for i := 0; i < clusters; i++ {
		dummyData[i] = make([]float64, FeatureCount)
	}

	return &NoveltyScorer{
		model:        cluster.NewKMeans(clusters, 10, dummyData),
		observations: make([][]float64, 0, 1000),
		maxObs:       1000,
	}
}

// Score calculates a novelty score (0-1) for a state vector
// Higher score means the pattern is further from known clusters
func (ns *NoveltyScorer) Score(features []float64) (float32, error) {
	if len(features) != FeatureCount {
		return 0, fmt.Errorf("invalid feature length: expected %d, got %d", FeatureCount, len(features))
	}

	ns.mu.Lock()
	defer ns.mu.Unlock()

	// 1. Record observation
	if len(ns.observations) < ns.maxObs {
		ns.observations = append(ns.observations, features)
	}

	// 2. Predict closest cluster
	// KMeans.Predict returns the centroid of the closest cluster
	centroid, err := ns.model.Predict(features)
	if err != nil {
		return 0, err
	}

	// 3. Calculate distance to centroid
	dist := ns.euclideanDistance(features, centroid)

	// 4. Normalize score based on a sigmoid mapping of distance
	// Higher distance -> Higher score
	score := 1.0 - (1.0 / (1.0 + math.Exp(dist-2.0)))

	return float32(score), nil
}

// Retrain improves the model based on accumulated observations
func (ns *NoveltyScorer) Retrain() error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if len(ns.observations) < 10 {
		return nil // Not enough data
	}

	// Update the model's training set
	err := ns.model.UpdateTrainingSet(ns.observations)
	if err != nil {
		return err
	}

	// Re-initialize and learn
	err = ns.model.Learn()
	if err != nil {
		return err
	}

	return nil
}

func (ns *NoveltyScorer) euclideanDistance(a, b []float64) float64 {
	sum := 0.0
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	for i := 0; i < limit; i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return math.Sqrt(sum)
}
