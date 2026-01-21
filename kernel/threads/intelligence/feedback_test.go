package intelligence

import (
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeedbackLoopManager_Registration(t *testing.T) {
	fm := NewFeedbackLoopManager()

	err := fm.RegisterLoop("l1", foundation.EngineLearning, foundation.EngineOptimization, 100*time.Millisecond, 0.5)
	require.NoError(t, err)
}

func TestFeedbackLoopManager_Processing(t *testing.T) {
	fm := NewFeedbackLoopManager()

	// Register loop: Learning -> Opt
	_ = fm.RegisterLoop("l1", foundation.EngineLearning, foundation.EngineOptimization, 100*time.Millisecond, 0.8)

	// Send feedback
	fb := &FeedbackMessage{
		Source:    foundation.EngineLearning,
		Target:    foundation.EngineOptimization,
		Type:      foundation.FeedbackAccuracy,
		Value:     0.95,
		Timestamp: time.Now(),
		Metadata:  map[string]interface{}{"info": "high accuracy"},
	}

	err := fm.SendFeedback("l1", fb)
	require.NoError(t, err)

	// Process feedback (aggregator -> adjuster)
	fm.ProcessFeedback()

	// Verify adjustments
	adjs := fm.adjuster.GetAdjustments()
	require.NotEmpty(t, adjs)

	// Value 0.95 * Gain 0.8 = 0.76
	assert.InDelta(t, 0.76, adjs[0].NewValue, 0.001)
	assert.Equal(t, foundation.FeedbackAccuracy.String(), adjs[0].Parameter)
}

func TestFeedbackLoopManager_Metrics(t *testing.T) {
	fm := NewFeedbackLoopManager()
	_ = fm.RegisterLoop("l1", foundation.EngineLearning, foundation.EngineOptimization, 100*time.Millisecond, 0.5)

	// Check empty
	adjs := fm.adjuster.GetAdjustments()
	assert.Empty(t, adjs)
}
