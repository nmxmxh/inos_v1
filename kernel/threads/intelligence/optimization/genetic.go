package optimization

import (
	"math/rand"
)

// GeneticAlgorithm implements genetic algorithm optimization
type GeneticAlgorithm struct {
	crossoverRate  float64
	mutationRate   float64
	populationSize int
	eliteSize      int
}

func NewGeneticAlgorithm(crossoverRate, mutationRate float64) *GeneticAlgorithm {
	return &GeneticAlgorithm{
		crossoverRate:  crossoverRate,
		mutationRate:   mutationRate,
		populationSize: 50,
		eliteSize:      5,
	}
}

// Evolve performs genetic algorithm evolution
func (ga *GeneticAlgorithm) Evolve(
	fitness func(params map[string]float64) float64,
	bounds map[string]Bounds,
	generations int,
) map[string]float64 {
	// Initialize population
	population := ga.initializePopulation(bounds)

	// Evaluate fitness
	fitnesses := make([]float64, len(population))
	for i, individual := range population {
		fitnesses[i] = fitness(individual)
	}

	// Evolution loop
	for gen := 0; gen < generations; gen++ {
		// Create new generation
		newPopulation := make([]map[string]float64, 0, ga.populationSize)

		// Elitism - keep best individuals
		elite := ga.selectElite(population, fitnesses)
		newPopulation = append(newPopulation, elite...)

		// Generate offspring
		for len(newPopulation) < ga.populationSize {
			// Selection
			parent1 := ga.tournamentSelection(population, fitnesses)
			parent2 := ga.tournamentSelection(population, fitnesses)

			// Crossover
			var child map[string]float64
			if rand.Float64() < ga.crossoverRate {
				child = ga.uniformCrossover(parent1, parent2)
			} else {
				child = ga.copyIndividual(parent1)
			}

			// Mutation
			if rand.Float64() < ga.mutationRate {
				ga.gaussianMutation(child, bounds)
			}

			newPopulation = append(newPopulation, child)
		}

		// Replace population
		population = newPopulation

		// Re-evaluate fitness
		for i, individual := range population {
			fitnesses[i] = fitness(individual)
		}
	}

	// Return best individual
	return ga.getBest(population, fitnesses)
}

// Tournament selection
func (ga *GeneticAlgorithm) tournamentSelection(population []map[string]float64, fitnesses []float64) map[string]float64 {
	tournamentSize := 3
	bestIdx := rand.Intn(len(population))
	bestFitness := fitnesses[bestIdx]

	for i := 1; i < tournamentSize; i++ {
		idx := rand.Intn(len(population))
		if fitnesses[idx] > bestFitness {
			bestIdx = idx
			bestFitness = fitnesses[idx]
		}
	}

	return population[bestIdx]
}

// Uniform crossover
func (ga *GeneticAlgorithm) uniformCrossover(parent1, parent2 map[string]float64) map[string]float64 {
	child := make(map[string]float64)

	for key := range parent1 {
		if rand.Float64() < 0.5 {
			child[key] = parent1[key]
		} else {
			child[key] = parent2[key]
		}
	}

	return child
}

// Gaussian mutation
func (ga *GeneticAlgorithm) gaussianMutation(individual map[string]float64, bounds map[string]Bounds) {
	for key, value := range individual {
		if rand.Float64() < 1.0/float64(len(individual)) {
			// Gaussian perturbation
			bound := bounds[key]
			sigma := (bound.Max - bound.Min) * 0.1 // 10% of range
			perturbation := rand.NormFloat64() * sigma

			newValue := value + perturbation

			// Clamp to bounds
			if newValue < bound.Min {
				newValue = bound.Min
			} else if newValue > bound.Max {
				newValue = bound.Max
			}

			individual[key] = newValue
		}
	}
}

// Select elite individuals
func (ga *GeneticAlgorithm) selectElite(population []map[string]float64, fitnesses []float64) []map[string]float64 {
	// Create index array
	indices := make([]int, len(population))
	for i := range indices {
		indices[i] = i
	}

	// Sort by fitness (descending)
	for i := 0; i < len(indices)-1; i++ {
		for j := i + 1; j < len(indices); j++ {
			if fitnesses[indices[j]] > fitnesses[indices[i]] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	// Select top elite
	elite := make([]map[string]float64, ga.eliteSize)
	for i := 0; i < ga.eliteSize && i < len(indices); i++ {
		elite[i] = ga.copyIndividual(population[indices[i]])
	}

	return elite
}

// Helper functions

func (ga *GeneticAlgorithm) initializePopulation(bounds map[string]Bounds) []map[string]float64 {
	population := make([]map[string]float64, ga.populationSize)

	for i := 0; i < ga.populationSize; i++ {
		individual := make(map[string]float64)
		for key, bound := range bounds {
			individual[key] = bound.Min + rand.Float64()*(bound.Max-bound.Min)
		}
		population[i] = individual
	}

	return population
}

func (ga *GeneticAlgorithm) copyIndividual(individual map[string]float64) map[string]float64 {
	copy := make(map[string]float64)
	for key, value := range individual {
		copy[key] = value
	}
	return copy
}

func (ga *GeneticAlgorithm) getBest(population []map[string]float64, fitnesses []float64) map[string]float64 {
	bestIdx := 0
	bestFitness := fitnesses[0]

	for i, fitness := range fitnesses {
		if fitness > bestFitness {
			bestFitness = fitness
			bestIdx = i
		}
	}

	return population[bestIdx]
}
