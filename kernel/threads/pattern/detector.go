package pattern

import (
	"fmt"
	"sync"
	"time"
)

// PatternDetector coordinates multiple detection algorithms
type PatternDetector struct {
	detectors  []PatternDetectorAlgorithm
	correlator *PatternCorrelator
	validator  *PatternValidator
	publisher  *PatternPublisher

	observations *ObservationStore
	mu           sync.RWMutex
}

// PatternDetectorAlgorithm defines the interface for detection algorithms
type PatternDetectorAlgorithm interface {
	Detect(obs []Observation) []*PatternCandidate
	Confidence() float32
	Type() PatternType
	Complexity() uint8
}

// Observation represents a single observation
type Observation struct {
	Timestamp time.Time
	Success   bool
	Latency   time.Duration
	Cost      float32
	Metadata  map[string]interface{}
}

// PatternCandidate is a potential pattern
type PatternCandidate struct {
	Type       PatternType
	Confidence uint8
	Data       []byte
	Evidence   []Observation
	Metadata   map[string]interface{}
}

// ObservationStore stores observations for pattern detection
type ObservationStore struct {
	observations map[string]*ObservationWindow
	mu           sync.RWMutex
}

// ObservationWindow tracks observations in a time window
type ObservationWindow struct {
	samples    []Observation
	windowSize int
	minSamples int
	startTime  time.Time
}

// NewPatternDetector creates a new pattern detector
func NewPatternDetector(validator *PatternValidator, publisher *PatternPublisher) *PatternDetector {
	return &PatternDetector{
		detectors: []PatternDetectorAlgorithm{
			NewStatisticalDetector(),
			NewTemporalDetector(),
			NewSequenceDetector(),
		},
		correlator:   NewPatternCorrelator(),
		validator:    validator,
		publisher:    publisher,
		observations: NewObservationStore(),
	}
}

// Observe records an observation
func (pd *PatternDetector) Observe(key string, obs Observation) {
	pd.observations.Add(key, obs)
}

// DetectPatterns analyzes observations and detects patterns
func (pd *PatternDetector) DetectPatterns() []*EnhancedPattern {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	var allCandidates []*PatternCandidate

	// Run all detection algorithms
	for _, detector := range pd.detectors {
		observations := pd.observations.GetAll()
		candidates := detector.Detect(observations)
		allCandidates = append(allCandidates, candidates...)
	}

	// Correlate candidates
	correlated := pd.correlator.Correlate(allCandidates)

	// Validate patterns
	var validPatterns []*EnhancedPattern
	for _, pattern := range correlated {
		if pd.validator.Validate(pattern, nil) {
			validPatterns = append(validPatterns, pattern)
		}
	}

	return validPatterns
}

// ObservationStore methods

func NewObservationStore() *ObservationStore {
	return &ObservationStore{
		observations: make(map[string]*ObservationWindow),
	}
}

func (os *ObservationStore) Add(key string, obs Observation) {
	os.mu.Lock()
	defer os.mu.Unlock()

	window, exists := os.observations[key]
	if !exists {
		window = &ObservationWindow{
			samples:    make([]Observation, 0),
			windowSize: 1000,
			minSamples: 10,
			startTime:  time.Now(),
		}
		os.observations[key] = window
	}

	window.samples = append(window.samples, obs)

	// Keep window size limited
	if len(window.samples) > window.windowSize {
		window.samples = window.samples[1:]
	}
}

func (os *ObservationStore) GetAll() []Observation {
	os.mu.RLock()
	defer os.mu.RUnlock()

	var all []Observation
	for _, window := range os.observations {
		all = append(all, window.samples...)
	}

	return all
}

func (os *ObservationStore) Get(key string) []Observation {
	os.mu.RLock()
	defer os.mu.RUnlock()

	window, exists := os.observations[key]
	if !exists {
		return nil
	}

	return window.samples
}

// PatternCorrelator correlates pattern candidates
type PatternCorrelator struct {
	patterns map[uint64]*EnhancedPattern
	mu       sync.RWMutex
}

func NewPatternCorrelator() *PatternCorrelator {
	return &PatternCorrelator{
		patterns: make(map[uint64]*EnhancedPattern),
	}
}

func (pc *PatternCorrelator) Correlate(candidates []*PatternCandidate) []*EnhancedPattern {
	var enhanced []*EnhancedPattern

	for _, candidate := range candidates {
		// Convert candidate to enhanced pattern
		pattern := &EnhancedPattern{
			Header: PatternHeader{
				Magic:      PATTERN_MAGIC,
				Type:       candidate.Type,
				Confidence: candidate.Confidence,
				Timestamp:  uint64(time.Now().UnixNano()),
				Weight:     1.0,
				Flags:      FlagActive,
			},
			Body: PatternBody{
				Data: PatternData{
					Encoding: EncodingBinary,
					Size:     uint16(len(candidate.Data)),
					Payload:  candidate.Data,
				},
			},
		}

		enhanced = append(enhanced, pattern)
	}

	return enhanced
}

// PatternPublisher publishes validated patterns
type PatternPublisher struct {
	storage *TieredPatternStorage
	mu      sync.RWMutex
}

func NewPatternPublisher(storage *TieredPatternStorage) *PatternPublisher {
	return &PatternPublisher{
		storage: storage,
	}
}

func (pp *PatternPublisher) PublishPattern(pattern *EnhancedPattern) error {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	return pp.storage.WritePattern(pattern)
}

// PatternValidator validates patterns
type PatternValidator struct {
	minSamples    int
	minConfidence float32
	maxAge        time.Duration
}

func NewPatternValidator() *PatternValidator {
	return &PatternValidator{
		minSamples:    10,
		minConfidence: 0.7,
		maxAge:        24 * time.Hour,
	}
}

func (pv *PatternValidator) Validate(pattern *EnhancedPattern, observations []Observation) bool {
	// Check magic
	if pattern.Header.Magic != PATTERN_MAGIC {
		return false
	}

	// Check confidence
	if float32(pattern.Header.Confidence)/100.0 < pv.minConfidence {
		return false
	}

	// Check age
	age := time.Since(time.Unix(0, int64(pattern.Header.Timestamp)))
	if age > pv.maxAge {
		return false
	}

	return true
}

func (pv *PatternValidator) CalculateConfidence(observations []Observation) uint8 {
	if len(observations) == 0 {
		return 0
	}

	successes := 0
	for _, obs := range observations {
		if obs.Success {
			successes++
		}
	}

	confidence := float32(successes) / float32(len(observations))
	return uint8(confidence * 100)
}

func (pv *PatternValidator) CheckStatisticalSignificance(observations []Observation) bool {
	// Simple check: need at least minSamples
	return len(observations) >= pv.minSamples
}

// Helper: Create pattern from observations
func CreatePatternFromObservations(patternType PatternType, observations []Observation) (*EnhancedPattern, error) {
	if len(observations) == 0 {
		return nil, fmt.Errorf("no observations provided")
	}

	// Calculate metrics
	successes := 0
	totalLatency := time.Duration(0)
	totalCost := float32(0)

	for _, obs := range observations {
		if obs.Success {
			successes++
		}
		totalLatency += obs.Latency
		totalCost += obs.Cost
	}

	successRate := float32(successes) / float32(len(observations))
	avgLatency := totalLatency / time.Duration(len(observations))
	avgCost := totalCost / float32(len(observations))

	pattern := NewPattern(patternType, 0)
	pattern.Header.Confidence = uint8(successRate * 100)
	pattern.Header.SuccessRate = successRate
	pattern.Body.Metadata.Metrics.Applications = uint32(len(observations))
	pattern.Body.Metadata.Metrics.Successes = uint32(successes)
	pattern.Body.Metadata.Metrics.AvgLatency = avgLatency
	pattern.Body.Metadata.Metrics.CostSavings = avgCost

	return pattern, nil
}
