package intelligence_test

import (
	"testing"

	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/stretchr/testify/assert"
)

func TestNoveltyScorer_Score(t *testing.T) {
	scorer := intelligence.NewNoveltyScorer(2)

	// 1. Train with some "normal" data (near origin)
	for i := 0; i < 50; i++ {
		_, _ = scorer.Score([]float64{0.1, 0.1, 0.1})
	}

	// Retrain to establish centroids
	err := scorer.Retrain()
	assert.NoError(t, err)

	// 2. Score a "similar" pattern
	scoreLow, err := scorer.Score([]float64{0.15, 0.15, 0.15})
	assert.NoError(t, err)

	// 3. Score a "novel" pattern (far from origin)
	scoreHigh, err := scorer.Score([]float64{10.0, 10.0, 10.0})
	assert.NoError(t, err)

	// 4. Verify higher score for novel pattern
	assert.Greater(t, scoreHigh, scoreLow, "Novel pattern should have a higher score than a known pattern")
}
