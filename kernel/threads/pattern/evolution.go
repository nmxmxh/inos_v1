package pattern

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// PatternEvolutionManager manages pattern evolution using genetic algorithms
type PatternEvolutionManager struct {
	storage   *TieredPatternStorage
	feedback  *FeedbackCollector
	mutator   *PatternMutator
	selector  *PatternSelector
	evaluator *PatternEvaluator
	history   *EvolutionHistory

	generation uint32
	mu         sync.RWMutex
}

// FeedbackCollector collects feedback on pattern applications
type FeedbackCollector struct {
	feedbacks map[uint64][]*PatternFeedback
	mu        sync.RWMutex
}

// PatternFeedback tracks a single pattern application
type PatternFeedback struct {
	PatternID   uint64
	Timestamp   time.Time
	Success     bool
	Improvement float32
	Latency     time.Duration
	Cost        float32
}

// PatternMutator applies mutations to patterns
type PatternMutator struct {
	mutationRate float32
	operators    []MutationOperator
	rand         *rand.Rand
}

// MutationOperator defines a mutation operation
type MutationOperator interface {
	Mutate(pattern *EnhancedPattern) *EnhancedPattern
	Probability() float32
}

// PatternSelector selects patterns for evolution
type PatternSelector struct {
	storage  *TieredPatternStorage
	strategy SelectionStrategy
}

type SelectionStrategy int

const (
	SelectionRoulette SelectionStrategy = iota
	SelectionTournament
	SelectionElite
)

// PatternEvaluator evaluates pattern fitness
type PatternEvaluator struct {
	weights FitnessWeights
}

type FitnessWeights struct {
	SuccessRate float32
	Improvement float32
	Frequency   float32
	Recency     float32
}

// EvolutionHistory tracks evolution history
type EvolutionHistory struct {
	generations map[uint32][]*EnhancedPattern
	mu          sync.RWMutex
}

// NewPatternEvolutionManager creates a new evolution manager
func NewPatternEvolutionManager(storage *TieredPatternStorage) *PatternEvolutionManager {
	return &PatternEvolutionManager{
		storage:  storage,
		feedback: NewFeedbackCollector(),
		mutator: &PatternMutator{
			mutationRate: 0.1,
			operators:    createMutationOperators(),
			rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		},
		selector: &PatternSelector{
			storage:  storage,
			strategy: SelectionTournament,
		},
		evaluator: &PatternEvaluator{
			weights: FitnessWeights{
				SuccessRate: 0.4,
				Improvement: 0.3,
				Frequency:   0.2,
				Recency:     0.1,
			},
		},
		history:    NewEvolutionHistory(),
		generation: 0,
	}
}

// Evolve runs one generation of evolution
func (pem *PatternEvolutionManager) Evolve() ([]*EnhancedPattern, error) {
	pem.mu.Lock()
	defer pem.mu.Unlock()

	// 1. Collect feedback
	allFeedback := pem.feedback.GetAll()
	if len(allFeedback) == 0 {
		return nil, fmt.Errorf("no feedback available")
	}

	// 2. Evaluate patterns
	evaluations := pem.evaluator.Evaluate(allFeedback)

	// 3. Select patterns for evolution
	candidates := pem.selector.Select(evaluations, 10)

	// 4. Apply mutations
	evolved := make([]*EnhancedPattern, 0)
	for _, candidate := range candidates {
		if pem.mutator.ShouldMutate() {
			mutated := pem.mutator.Mutate(candidate)
			mutated.Header.Version++
			mutated.Header.Flags |= FlagEvolved
			mutated.Body.Links.EvolvedFrom = append(mutated.Body.Links.EvolvedFrom, candidate.Header.ID)
			evolved = append(evolved, mutated)
		}
	}

	// 5. Store evolved patterns
	for _, pattern := range evolved {
		if err := pem.storage.WritePattern(pattern); err != nil {
			return nil, err
		}
	}

	// 6. Record generation
	pem.generation++
	pem.history.Record(pem.generation, evolved)

	return evolved, nil
}

// RecordFeedback records feedback for a pattern
func (pem *PatternEvolutionManager) RecordFeedback(feedback *PatternFeedback) {
	pem.feedback.Add(feedback)
}

// FeedbackCollector methods

func NewFeedbackCollector() *FeedbackCollector {
	return &FeedbackCollector{
		feedbacks: make(map[uint64][]*PatternFeedback),
	}
}

func (fc *FeedbackCollector) Add(feedback *PatternFeedback) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.feedbacks[feedback.PatternID] = append(fc.feedbacks[feedback.PatternID], feedback)
}

func (fc *FeedbackCollector) GetAll() map[uint64][]*PatternFeedback {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	// Return copy
	result := make(map[uint64][]*PatternFeedback)
	for id, feedbacks := range fc.feedbacks {
		result[id] = feedbacks
	}

	return result
}

// PatternMutator methods

func (pm *PatternMutator) ShouldMutate() bool {
	return pm.rand.Float32() < pm.mutationRate
}

func (pm *PatternMutator) Mutate(pattern *EnhancedPattern) *EnhancedPattern {
	// Create copy
	mutated := *pattern

	// Apply random mutation operator
	if len(pm.operators) > 0 {
		op := pm.operators[pm.rand.Intn(len(pm.operators))]
		return op.Mutate(&mutated)
	}

	return &mutated
}

// Mutation operators

type ConfidenceMutation struct{}

func (cm *ConfidenceMutation) Mutate(pattern *EnhancedPattern) *EnhancedPattern {
	mutated := *pattern
	// Slightly adjust confidence
	adjustment := int8(rand.Intn(11) - 5) // -5 to +5
	newConfidence := int(mutated.Header.Confidence) + int(adjustment)
	if newConfidence < 0 {
		newConfidence = 0
	}
	if newConfidence > 100 {
		newConfidence = 100
	}
	mutated.Header.Confidence = uint8(newConfidence)
	return &mutated
}

func (cm *ConfidenceMutation) Probability() float32 {
	return 0.3
}

type WeightMutation struct{}

func (wm *WeightMutation) Mutate(pattern *EnhancedPattern) *EnhancedPattern {
	mutated := *pattern
	// Adjust weight
	adjustment := (rand.Float32() - 0.5) * 0.2 // -0.1 to +0.1
	mutated.Header.Weight += adjustment
	if mutated.Header.Weight < 0 {
		mutated.Header.Weight = 0
	}
	if mutated.Header.Weight > 1 {
		mutated.Header.Weight = 1
	}
	return &mutated
}

func (wm *WeightMutation) Probability() float32 {
	return 0.3
}

type ComplexityMutation struct{}

func (cm *ComplexityMutation) Mutate(pattern *EnhancedPattern) *EnhancedPattern {
	mutated := *pattern
	// Adjust complexity
	if rand.Float32() < 0.5 && mutated.Header.Complexity > 1 {
		mutated.Header.Complexity--
	} else if mutated.Header.Complexity < 10 {
		mutated.Header.Complexity++
	}
	return &mutated
}

func (cm *ComplexityMutation) Probability() float32 {
	return 0.2
}

func createMutationOperators() []MutationOperator {
	return []MutationOperator{
		&ConfidenceMutation{},
		&WeightMutation{},
		&ComplexityMutation{},
	}
}

// PatternSelector methods

func (ps *PatternSelector) Select(evaluations map[uint64]float32, count int) []*EnhancedPattern {
	// Convert to slice for sorting
	type scoredPattern struct {
		pattern *EnhancedPattern
		score   float32
	}

	var scored []scoredPattern
	for id, score := range evaluations {
		pattern, err := ps.storage.ReadPattern(id)
		if err != nil {
			// If pattern missing, skip
			continue // Log error?
		}
		scored = append(scored, scoredPattern{pattern, score})
	}

	// Sort by score (descending)
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Select top N
	result := make([]*EnhancedPattern, 0, count)
	for i := 0; i < count && i < len(scored); i++ {
		result = append(result, scored[i].pattern)
	}

	return result
}

// PatternEvaluator methods

func (pe *PatternEvaluator) Evaluate(feedback map[uint64][]*PatternFeedback) map[uint64]float32 {
	scores := make(map[uint64]float32)

	for patternID, feedbacks := range feedback {
		if len(feedbacks) == 0 {
			continue
		}

		// Calculate metrics
		successes := 0
		totalImprovement := float32(0)
		for _, fb := range feedbacks {
			if fb.Success {
				successes++
			}
			totalImprovement += fb.Improvement
		}

		successRate := float32(successes) / float32(len(feedbacks))
		avgImprovement := totalImprovement / float32(len(feedbacks))
		frequency := float32(len(feedbacks))
		recency := pe.calculateRecency(feedbacks)

		// Calculate fitness score
		score := pe.weights.SuccessRate*successRate +
			pe.weights.Improvement*avgImprovement +
			pe.weights.Frequency*frequency/100.0 +
			pe.weights.Recency*recency

		scores[patternID] = score
	}

	return scores
}

func (pe *PatternEvaluator) calculateRecency(feedbacks []*PatternFeedback) float32 {
	if len(feedbacks) == 0 {
		return 0
	}

	// Find most recent feedback
	mostRecent := feedbacks[0].Timestamp
	for _, fb := range feedbacks {
		if fb.Timestamp.After(mostRecent) {
			mostRecent = fb.Timestamp
		}
	}

	// Calculate recency score (1.0 = now, 0.0 = 24h ago)
	age := time.Since(mostRecent)
	if age > 24*time.Hour {
		return 0
	}

	return 1.0 - float32(age.Hours()/24.0)
}

// EvolutionHistory methods

func NewEvolutionHistory() *EvolutionHistory {
	return &EvolutionHistory{
		generations: make(map[uint32][]*EnhancedPattern),
	}
}

func (eh *EvolutionHistory) Record(generation uint32, patterns []*EnhancedPattern) {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	eh.generations[generation] = patterns
}

func (eh *EvolutionHistory) Get(generation uint32) []*EnhancedPattern {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	return eh.generations[generation]
}
