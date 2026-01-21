package optimization

import (
	"math"
	"math/rand"
	"sort"
)

// NSGA2Optimizer implements Non-dominated Sorting Genetic Algorithm II
type NSGA2Optimizer struct {
	populationSize int
	generations    int
	crossoverRate  float64
	mutationRate   float64
}

func NewNSGA2Optimizer(populationSize, generations int) *NSGA2Optimizer {
	return &NSGA2Optimizer{
		populationSize: populationSize,
		generations:    generations,
		crossoverRate:  0.9,
		mutationRate:   0.1,
	}
}

// Optimize performs NSGA-II optimization
func (nsga *NSGA2Optimizer) Optimize(problem *OptimizationProblem) []*Solution {
	// Initialize population
	population := nsga.initializePopulation(problem)

	// Evaluate initial population
	for _, sol := range population {
		nsga.evaluate(sol, problem)
	}

	// Evolution loop
	for gen := 0; gen < nsga.generations; gen++ {
		// Fast non-dominated sorting
		fronts := nsga.fastNonDominatedSort(population)

		// Calculate crowding distance for each front
		for _, front := range fronts {
			nsga.calculateCrowdingDistance(front)
		}

		// Create offspring through selection, crossover, and mutation
		offspring := nsga.createOffspring(population, problem)

		// Combine parent and offspring
		combined := append(population, offspring...)

		// Select next generation
		population = nsga.selectNextGeneration(combined, problem)
	}

	// Return Pareto front (rank 0)
	fronts := nsga.fastNonDominatedSort(population)
	if len(fronts) > 0 {
		return fronts[0]
	}
	return nil
}

// Fast non-dominated sorting (NSGA-II core algorithm)
func (nsga *NSGA2Optimizer) fastNonDominatedSort(population []*Solution) [][]*Solution {
	// Initialize domination counts and dominated sets
	dominationCount := make(map[*Solution]int)
	dominatedSet := make(map[*Solution][]*Solution)

	fronts := make([][]*Solution, 0)
	currentFront := make([]*Solution, 0)

	// For each solution
	for _, p := range population {
		dominationCount[p] = 0
		dominatedSet[p] = make([]*Solution, 0)

		// Compare with all other solutions
		for _, q := range population {
			if p == q {
				continue
			}

			if dominates(p, q) {
				// p dominates q
				dominatedSet[p] = append(dominatedSet[p], q)
			} else if dominates(q, p) {
				// q dominates p
				dominationCount[p]++
			}
		}

		// If p is not dominated by any solution
		if dominationCount[p] == 0 {
			p.Rank = 0
			currentFront = append(currentFront, p)
		}
	}

	fronts = append(fronts, currentFront)

	// Build subsequent fronts
	i := 0
	for len(fronts[i]) > 0 {
		nextFront := make([]*Solution, 0)

		for _, p := range fronts[i] {
			for _, q := range dominatedSet[p] {
				dominationCount[q]--
				if dominationCount[q] == 0 {
					q.Rank = i + 1
					nextFront = append(nextFront, q)
				}
			}
		}

		if len(nextFront) > 0 {
			fronts = append(fronts, nextFront)
			i++
		} else {
			break
		}
	}

	return fronts
}

// Calculate crowding distance for diversity preservation
func (nsga *NSGA2Optimizer) calculateCrowdingDistance(front []*Solution) {
	if len(front) == 0 {
		return
	}

	numObjectives := len(front[0].Objectives)

	// Initialize distances to 0
	for _, sol := range front {
		sol.Distance = 0
	}

	// For each objective
	for m := 0; m < numObjectives; m++ {
		// Sort by objective m
		sort.Slice(front, func(i, j int) bool {
			return front[i].Objectives[m] < front[j].Objectives[m]
		})

		// Boundary solutions get infinite distance
		front[0].Distance = math.Inf(1)
		front[len(front)-1].Distance = math.Inf(1)

		// Calculate range
		objRange := front[len(front)-1].Objectives[m] - front[0].Objectives[m]

		if objRange == 0 {
			continue
		}

		// Calculate crowding distance for intermediate solutions
		for i := 1; i < len(front)-1; i++ {
			distance := (front[i+1].Objectives[m] - front[i-1].Objectives[m]) / objRange
			front[i].Distance += distance
		}
	}
}

// Create offspring through selection, crossover, and mutation
func (nsga *NSGA2Optimizer) createOffspring(population []*Solution, problem *OptimizationProblem) []*Solution {
	offspring := make([]*Solution, 0, nsga.populationSize)

	for len(offspring) < nsga.populationSize {
		// Binary tournament selection
		parent1 := nsga.tournamentSelect(population)
		parent2 := nsga.tournamentSelect(population)

		// Crossover
		var child1, child2 *Solution
		if rand.Float64() < nsga.crossoverRate {
			child1, child2 = nsga.crossover(parent1, parent2, problem)
		} else {
			child1 = nsga.copySolution(parent1)
			child2 = nsga.copySolution(parent2)
		}

		// Mutation
		if rand.Float64() < nsga.mutationRate {
			nsga.mutate(child1, problem)
		}
		if rand.Float64() < nsga.mutationRate {
			nsga.mutate(child2, problem)
		}

		// Evaluate offspring
		nsga.evaluate(child1, problem)
		nsga.evaluate(child2, problem)

		offspring = append(offspring, child1)
		if len(offspring) < nsga.populationSize {
			offspring = append(offspring, child2)
		}
	}

	return offspring
}

// Binary tournament selection
func (nsga *NSGA2Optimizer) tournamentSelect(population []*Solution) *Solution {
	i1 := rand.Intn(len(population))
	i2 := rand.Intn(len(population))

	sol1 := population[i1]
	sol2 := population[i2]

	// Select based on rank and crowding distance
	if sol1.Rank < sol2.Rank {
		return sol1
	} else if sol1.Rank > sol2.Rank {
		return sol2
	} else {
		// Same rank, select based on crowding distance
		if sol1.Distance > sol2.Distance {
			return sol1
		}
		return sol2
	}
}

// Simulated Binary Crossover (SBX)
func (nsga *NSGA2Optimizer) crossover(parent1, parent2 *Solution, problem *OptimizationProblem) (*Solution, *Solution) {
	child1 := &Solution{Parameters: make(map[string]float64)}
	child2 := &Solution{Parameters: make(map[string]float64)}

	eta := 20.0 // Distribution index

	for param := range parent1.Parameters {
		if rand.Float64() < 0.5 {
			// Perform SBX
			p1 := parent1.Parameters[param]
			p2 := parent2.Parameters[param]

			u := rand.Float64()
			var beta float64

			if u <= 0.5 {
				beta = math.Pow(2.0*u, 1.0/(eta+1.0))
			} else {
				beta = math.Pow(1.0/(2.0*(1.0-u)), 1.0/(eta+1.0))
			}

			child1.Parameters[param] = 0.5 * ((1.0+beta)*p1 + (1.0-beta)*p2)
			child2.Parameters[param] = 0.5 * ((1.0-beta)*p1 + (1.0+beta)*p2)

			// Ensure bounds
			if bounds, exists := problem.Bounds[param]; exists {
				child1.Parameters[param] = math.Max(bounds.Min, math.Min(bounds.Max, child1.Parameters[param]))
				child2.Parameters[param] = math.Max(bounds.Min, math.Min(bounds.Max, child2.Parameters[param]))
			}
		} else {
			// No crossover
			child1.Parameters[param] = parent1.Parameters[param]
			child2.Parameters[param] = parent2.Parameters[param]
		}
	}

	return child1, child2
}

// Polynomial mutation
func (nsga *NSGA2Optimizer) mutate(solution *Solution, problem *OptimizationProblem) {
	eta := 20.0 // Distribution index

	for param, value := range solution.Parameters {
		if rand.Float64() < 1.0/float64(len(solution.Parameters)) {
			bounds := problem.Bounds[param]
			delta := bounds.Max - bounds.Min

			u := rand.Float64()
			var deltaq float64

			if u < 0.5 {
				deltaq = math.Pow(2.0*u, 1.0/(eta+1.0)) - 1.0
			} else {
				deltaq = 1.0 - math.Pow(2.0*(1.0-u), 1.0/(eta+1.0))
			}

			solution.Parameters[param] = value + deltaq*delta

			// Ensure bounds
			solution.Parameters[param] = math.Max(bounds.Min, math.Min(bounds.Max, solution.Parameters[param]))
		}
	}
}

// Select next generation
func (nsga *NSGA2Optimizer) selectNextGeneration(combined []*Solution, _ *OptimizationProblem) []*Solution {
	// Sort into fronts
	fronts := nsga.fastNonDominatedSort(combined)

	// Calculate crowding distance
	for _, front := range fronts {
		nsga.calculateCrowdingDistance(front)
	}

	// Select solutions
	nextGen := make([]*Solution, 0, nsga.populationSize)

	for _, front := range fronts {
		if len(nextGen)+len(front) <= nsga.populationSize {
			// Add entire front
			nextGen = append(nextGen, front...)
		} else {
			// Sort by crowding distance and add best
			sortByCrowdingDistance(front)
			remaining := nsga.populationSize - len(nextGen)
			nextGen = append(nextGen, front[:remaining]...)
			break
		}
	}

	return nextGen
}

// Helper functions

func (nsga *NSGA2Optimizer) initializePopulation(problem *OptimizationProblem) []*Solution {
	population := make([]*Solution, nsga.populationSize)

	for i := 0; i < nsga.populationSize; i++ {
		solution := &Solution{Parameters: make(map[string]float64)}

		for param, bounds := range problem.Bounds {
			solution.Parameters[param] = bounds.Min + rand.Float64()*(bounds.Max-bounds.Min)
		}

		population[i] = solution
	}

	return population
}

func (nsga *NSGA2Optimizer) evaluate(solution *Solution, problem *OptimizationProblem) {
	solution.Objectives = make([]float64, len(problem.Objectives))

	for i, obj := range problem.Objectives {
		value := obj.Evaluator(solution)
		if obj.Minimize {
			solution.Objectives[i] = value
		} else {
			solution.Objectives[i] = -value
		}
	}
}

func (nsga *NSGA2Optimizer) copySolution(sol *Solution) *Solution {
	copied := &Solution{
		Parameters: make(map[string]float64),
		Objectives: make([]float64, len(sol.Objectives)),
		Rank:       sol.Rank,
		Distance:   sol.Distance,
		Fitness:    sol.Fitness,
	}

	for k, v := range sol.Parameters {
		copied.Parameters[k] = v
	}

	copy(copied.Objectives, sol.Objectives)

	return copied
}
