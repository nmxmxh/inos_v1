package optimization

import (
	"sort"
	"sync"
)

// OptimizationEngine coordinates multi-objective optimization
type OptimizationEngine struct {
	nsga2    *NSGA2Optimizer
	bayesian *BayesianOptimizer
	genetic  *GeneticAlgorithm
	rollout  *RolloutStrategy

	// Statistics
	optimizationsRun uint64
	improvements     float64

	mu sync.RWMutex
}

// Objective represents an optimization objective
type Objective struct {
	Name      string
	Minimize  bool // true = minimize, false = maximize
	Weight    float64
	Evaluator func(solution *Solution) float64
}

// Solution represents a candidate solution
type Solution struct {
	Parameters map[string]float64
	Objectives []float64
	Rank       int     // Pareto rank
	Distance   float64 // Crowding distance
	Fitness    float64 // Overall fitness
}

// OptimizationProblem defines the optimization problem
type OptimizationProblem struct {
	Objectives  []*Objective
	Constraints []Constraint
	Bounds      map[string]Bounds
}

type Constraint struct {
	Name      string
	Evaluator func(solution *Solution) bool
}

type Bounds struct {
	Min float64
	Max float64
}

func NewOptimizationEngine() *OptimizationEngine {
	return &OptimizationEngine{
		nsga2:    NewNSGA2Optimizer(100, 200), // population=100, generations=200
		bayesian: NewBayesianOptimizer(),
		genetic:  NewGeneticAlgorithm(0.8, 0.1), // crossover=0.8, mutation=0.1
		rollout:  NewRolloutStrategy(),
	}
}

// Optimize performs multi-objective optimization
func (oe *OptimizationEngine) Optimize(problem *OptimizationProblem) []*Solution {
	oe.mu.Lock()
	oe.optimizationsRun++
	oe.mu.Unlock()

	// Use NSGA-II for multi-objective optimization
	paretoFront := oe.nsga2.Optimize(problem)

	return paretoFront
}

// OptimizeHyperparameters uses Bayesian optimization for hyperparameter tuning
func (oe *OptimizationEngine) OptimizeHyperparameters(
	objective func(params map[string]float64) float64,
	bounds map[string]Bounds,
	iterations int,
) map[string]float64 {
	return oe.bayesian.Optimize(objective, bounds, iterations)
}

// EvolveParameters uses genetic algorithm for parameter evolution
func (oe *OptimizationEngine) EvolveParameters(
	fitness func(params map[string]float64) float64,
	bounds map[string]Bounds,
	generations int,
) map[string]float64 {
	return oe.genetic.Evolve(fitness, bounds, generations)
}

// PlanRollout creates a safe rollout strategy
func (oe *OptimizationEngine) PlanRollout(
	baseline, candidate *Solution,
	trafficPercentages []float64,
) *RolloutPlan {
	return oe.rollout.Plan(baseline, candidate, trafficPercentages)
}

// GetStats returns optimization statistics
func (oe *OptimizationEngine) GetStats() OptimizationStats {
	oe.mu.RLock()
	defer oe.mu.RUnlock()

	return OptimizationStats{
		OptimizationsRun: oe.optimizationsRun,
		Improvements:     oe.improvements,
	}
}

type OptimizationStats struct {
	OptimizationsRun uint64
	Improvements     float64
}

// Helper: Calculate dominated count (for Pareto ranking)
func dominates(s1, s2 *Solution) bool {
	// s1 dominates s2 if s1 is better in at least one objective and not worse in any
	betterInOne := false
	for i := range s1.Objectives {
		if s1.Objectives[i] > s2.Objectives[i] {
			return false // s1 is worse in this objective
		}
		if s1.Objectives[i] < s2.Objectives[i] {
			betterInOne = true
		}
	}
	return betterInOne
}

// Helper: Sort solutions by crowding distance (descending)
func sortByCrowdingDistance(solutions []*Solution) {
	sort.Slice(solutions, func(i, j int) bool {
		return solutions[i].Distance > solutions[j].Distance
	})
}
