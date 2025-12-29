package intelligence

import (
	"sync"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
)

// FeedbackLoopManager manages feedback loops between engines
type FeedbackLoopManager struct {
	loops      map[string]*FeedbackLoop
	aggregator *FeedbackAggregator
	adjuster   *ModelAdjuster
	mu         sync.RWMutex
}

// FeedbackLoop represents a feedback connection between engines
type FeedbackLoop struct {
	ID      string
	Source  foundation.EngineType
	Target  foundation.EngineType
	Delay   time.Duration
	Gain    float32
	Active  bool
	Metrics FeedbackMetrics
}

type FeedbackMetrics struct {
	MessagesProcessed uint64
	AvgLatency        time.Duration
	LastUpdate        time.Time
}

// FeedbackAggregator aggregates feedback from multiple sources
type FeedbackAggregator struct {
	buffers map[string][]*FeedbackMessage
	mu      sync.RWMutex
}

type FeedbackMessage struct {
	Source    foundation.EngineType
	Target    foundation.EngineType
	Type      foundation.FeedbackType
	Value     float32
	Timestamp time.Time
	Metadata  map[string]interface{}
}

// ModelAdjuster adjusts models based on feedback
type ModelAdjuster struct {
	adjustments map[string]*Adjustment
	mu          sync.RWMutex
}

type Adjustment struct {
	Parameter string
	OldValue  float32
	NewValue  float32
	Reason    string
	Timestamp time.Time
}

// NewFeedbackLoopManager creates a new feedback loop manager
func NewFeedbackLoopManager() *FeedbackLoopManager {
	return &FeedbackLoopManager{
		loops: make(map[string]*FeedbackLoop),
		aggregator: &FeedbackAggregator{
			buffers: make(map[string][]*FeedbackMessage),
		},
		adjuster: &ModelAdjuster{
			adjustments: make(map[string]*Adjustment),
		},
	}
}

// RegisterLoop registers a new feedback loop
func (flm *FeedbackLoopManager) RegisterLoop(id string, source, target foundation.EngineType, delay time.Duration, gain float32) error {
	flm.mu.Lock()
	defer flm.mu.Unlock()

	flm.loops[id] = &FeedbackLoop{
		ID:     id,
		Source: source,
		Target: target,
		Delay:  delay,
		Gain:   gain,
		Active: true,
	}

	return nil
}

// SendFeedback sends feedback through a loop
func (flm *FeedbackLoopManager) SendFeedback(loopID string, message *FeedbackMessage) error {
	flm.mu.RLock()
	loop, exists := flm.loops[loopID]
	flm.mu.RUnlock()

	if !exists || !loop.Active {
		return nil // Silently ignore if loop doesn't exist or is inactive
	}

	// Add to aggregator buffer
	flm.aggregator.Add(loopID, message)

	return nil
}

// ProcessFeedback processes accumulated feedback
func (flm *FeedbackLoopManager) ProcessFeedback() {
	flm.mu.RLock()
	defer flm.mu.RUnlock()

	for loopID, loop := range flm.loops {
		if !loop.Active {
			continue
		}

		// Get aggregated feedback
		messages := flm.aggregator.GetAndClear(loopID)
		if len(messages) == 0 {
			continue
		}

		// Apply adjustments
		for _, msg := range messages {
			adjustment := flm.calculateAdjustment(loop, msg)
			if adjustment != nil {
				flm.adjuster.Apply(adjustment)
			}
		}

		// Update metrics
		loop.Metrics.MessagesProcessed += uint64(len(messages))
		loop.Metrics.LastUpdate = time.Now()
	}
}

// Helper: Calculate adjustment from feedback
func (flm *FeedbackLoopManager) calculateAdjustment(loop *FeedbackLoop, msg *FeedbackMessage) *Adjustment {
	// Apply gain to feedback value
	adjustmentValue := msg.Value * loop.Gain

	return &Adjustment{
		Parameter: msg.Type.String(),
		NewValue:  adjustmentValue,
		Reason:    "feedback_loop",
		Timestamp: time.Now(),
	}
}

// FeedbackAggregator methods

func (fa *FeedbackAggregator) Add(loopID string, message *FeedbackMessage) {
	fa.mu.Lock()
	defer fa.mu.Unlock()

	fa.buffers[loopID] = append(fa.buffers[loopID], message)
}

func (fa *FeedbackAggregator) GetAndClear(loopID string) []*FeedbackMessage {
	fa.mu.Lock()
	defer fa.mu.Unlock()

	messages := fa.buffers[loopID]
	fa.buffers[loopID] = nil

	return messages
}

// ModelAdjuster methods

func (ma *ModelAdjuster) Apply(adjustment *Adjustment) {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	ma.adjustments[adjustment.Parameter] = adjustment
}

func (ma *ModelAdjuster) GetAdjustments() []*Adjustment {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	adjustments := make([]*Adjustment, 0, len(ma.adjustments))
	for _, adj := range ma.adjustments {
		adjustments = append(adjustments, adj)
	}

	return adjustments
}
