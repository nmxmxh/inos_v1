package optimization

import (
	"math"
	"math/rand"
)

// BayesianOptimizer implements Bayesian optimization with Gaussian Process
type BayesianOptimizer struct {
	observations []Observation
	kernel       *RBFKernel
	noise        float64
}

type Observation struct {
	Parameters map[string]float64
	Value      float64
}

// RBFKernel implements Radial Basis Function kernel
type RBFKernel struct {
	lengthScale float64
	variance    float64
}

func NewBayesianOptimizer() *BayesianOptimizer {
	return &BayesianOptimizer{
		observations: make([]Observation, 0),
		kernel:       &RBFKernel{lengthScale: 1.0, variance: 1.0},
		noise:        0.01,
	}
}

// Optimize performs Bayesian optimization
func (bo *BayesianOptimizer) Optimize(
	objective func(params map[string]float64) float64,
	bounds map[string]Bounds,
	iterations int,
) map[string]float64 {
	// Initialize with random samples
	for i := 0; i < 5; i++ {
		params := bo.randomSample(bounds)
		value := objective(params)
		bo.observations = append(bo.observations, Observation{
			Parameters: params,
			Value:      value,
		})
	}

	// Optimization loop
	for i := 0; i < iterations; i++ {
		// Find next point to evaluate using acquisition function
		nextParams := bo.acquireNext(bounds)

		// Evaluate objective
		value := objective(nextParams)

		// Add observation
		bo.observations = append(bo.observations, Observation{
			Parameters: nextParams,
			Value:      value,
		})
	}

	// Return best parameters found
	return bo.getBest()
}

// Acquire next point using Expected Improvement
func (bo *BayesianOptimizer) acquireNext(bounds map[string]Bounds) map[string]float64 {
	bestEI := -math.MaxFloat64
	var bestParams map[string]float64

	// Sample candidate points
	numCandidates := 100
	for i := 0; i < numCandidates; i++ {
		candidate := bo.randomSample(bounds)

		// Calculate Expected Improvement
		ei := bo.expectedImprovement(candidate)

		if ei > bestEI {
			bestEI = ei
			bestParams = candidate
		}
	}

	return bestParams
}

// Expected Improvement acquisition function
func (bo *BayesianOptimizer) expectedImprovement(params map[string]float64) float64 {
	// Predict mean and variance using Gaussian Process
	mean, variance := bo.predict(params)

	// Current best value
	bestValue := bo.getBestValue()

	// Calculate improvement
	improvement := bestValue - mean
	stdDev := math.Sqrt(variance)

	if stdDev == 0 {
		return 0
	}

	// Z-score
	z := improvement / stdDev

	// Expected Improvement = improvement * Φ(z) + stdDev * φ(z)
	// where Φ is CDF and φ is PDF of standard normal
	ei := improvement*normalCDF(z) + stdDev*normalPDF(z)

	return ei
}

// Predict using Gaussian Process
func (bo *BayesianOptimizer) predict(params map[string]float64) (mean, variance float64) {
	if len(bo.observations) == 0 {
		return 0, 1.0
	}

	// Simplified GP prediction
	// In production, use proper GP with kernel matrix inversion

	// Calculate kernel values with all observations
	weights := make([]float64, len(bo.observations))
	totalWeight := 0.0

	for i, obs := range bo.observations {
		k := bo.kernel.evaluate(params, obs.Parameters)
		weights[i] = k
		totalWeight += k
	}

	// Weighted mean
	mean = 0
	for i, obs := range bo.observations {
		if totalWeight > 0 {
			mean += (weights[i] / totalWeight) * obs.Value
		}
	}

	// Variance (simplified)
	variance = 1.0 - totalWeight/(totalWeight+bo.noise)
	if variance < 0.01 {
		variance = 0.01
	}

	return mean, variance
}

// RBF Kernel evaluation
func (rbf *RBFKernel) evaluate(x1, x2 map[string]float64) float64 {
	// Calculate squared Euclidean distance
	distSq := 0.0
	for key, val1 := range x1 {
		val2 := x2[key]
		diff := val1 - val2
		distSq += diff * diff
	}

	// RBF kernel: σ² * exp(-d²/(2l²))
	return rbf.variance * math.Exp(-distSq/(2*rbf.lengthScale*rbf.lengthScale))
}

// Helper functions

func (bo *BayesianOptimizer) randomSample(bounds map[string]Bounds) map[string]float64 {
	params := make(map[string]float64)
	for key, bound := range bounds {
		params[key] = bound.Min + rand.Float64()*(bound.Max-bound.Min)
	}
	return params
}

func (bo *BayesianOptimizer) getBest() map[string]float64 {
	if len(bo.observations) == 0 {
		return nil
	}

	bestIdx := 0
	bestValue := bo.observations[0].Value

	for i, obs := range bo.observations {
		if obs.Value > bestValue {
			bestValue = obs.Value
			bestIdx = i
		}
	}

	return bo.observations[bestIdx].Parameters
}

func (bo *BayesianOptimizer) getBestValue() float64 {
	if len(bo.observations) == 0 {
		return -math.MaxFloat64
	}

	bestValue := bo.observations[0].Value
	for _, obs := range bo.observations {
		if obs.Value > bestValue {
			bestValue = obs.Value
		}
	}

	return bestValue
}

// Normal distribution functions
func normalPDF(x float64) float64 {
	return math.Exp(-0.5*x*x) / math.Sqrt(2*math.Pi)
}

func normalCDF(x float64) float64 {
	// Approximation of CDF using error function
	return 0.5 * (1.0 + math.Erf(x/math.Sqrt2))
}
