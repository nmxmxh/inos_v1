package optimization

import (
	"math"
	"time"
)

// RolloutStrategy implements safe deployment strategies
type RolloutStrategy struct {
	minSampleSize int
	alpha         float64 // Significance level
}

type RolloutPlan struct {
	Stages           []*RolloutStage
	TotalDuration    time.Duration
	RollbackTriggers []RollbackTrigger
}

type RolloutStage struct {
	TrafficPercent float64
	Duration       time.Duration
	MinSamples     int
}

type RollbackTrigger struct {
	Metric    string
	Threshold float64
	Direction string // "increase" or "decrease"
}

func NewRolloutStrategy() *RolloutStrategy {
	return &RolloutStrategy{
		minSampleSize: 100,
		alpha:         0.05, // 95% confidence
	}
}

// Plan creates a rollout plan
func (rs *RolloutStrategy) Plan(
	baseline, candidate *Solution,
	trafficPercentages []float64,
) *RolloutPlan {
	plan := &RolloutPlan{
		Stages:           make([]*RolloutStage, len(trafficPercentages)),
		RollbackTriggers: rs.createRollbackTriggers(baseline, candidate),
	}

	totalDuration := time.Duration(0)

	for i, percent := range trafficPercentages {
		// Calculate required samples for statistical significance
		minSamples := rs.calculateSampleSize(percent)

		// Estimate duration based on traffic
		duration := rs.estimateDuration(percent, minSamples)

		plan.Stages[i] = &RolloutStage{
			TrafficPercent: percent,
			Duration:       duration,
			MinSamples:     minSamples,
		}

		totalDuration += duration
	}

	plan.TotalDuration = totalDuration

	return plan
}

// Evaluate performs A/B test statistical analysis
func (rs *RolloutStrategy) Evaluate(
	baselineMetrics, candidateMetrics []float64,
) *ABTestResult {
	result := &ABTestResult{
		SampleSizeBaseline:  len(baselineMetrics),
		SampleSizeCandidate: len(candidateMetrics),
	}

	if len(baselineMetrics) == 0 || len(candidateMetrics) == 0 {
		result.Significant = false
		return result
	}

	// Calculate means
	result.MeanBaseline = mean(baselineMetrics)
	result.MeanCandidate = mean(candidateMetrics)
	result.Improvement = (result.MeanCandidate - result.MeanBaseline) / result.MeanBaseline

	// Calculate standard deviations
	stdBaseline := stdDev(baselineMetrics, result.MeanBaseline)
	stdCandidate := stdDev(candidateMetrics, result.MeanCandidate)

	// Perform t-test
	result.PValue = rs.tTest(
		result.MeanBaseline, stdBaseline, len(baselineMetrics),
		result.MeanCandidate, stdCandidate, len(candidateMetrics),
	)

	result.Significant = result.PValue < rs.alpha

	return result
}

type ABTestResult struct {
	MeanBaseline        float64
	MeanCandidate       float64
	Improvement         float64
	PValue              float64
	Significant         bool
	SampleSizeBaseline  int
	SampleSizeCandidate int
}

// Two-sample t-test
func (rs *RolloutStrategy) tTest(
	mean1, std1 float64, n1 int,
	mean2, std2 float64, n2 int,
) float64 {
	// Calculate pooled standard error
	se := math.Sqrt((std1*std1)/float64(n1) + (std2*std2)/float64(n2))

	if se == 0 {
		return 1.0 // No difference
	}

	// Calculate t-statistic
	t := (mean2 - mean1) / se

	// Degrees of freedom (Welch's approximation)
	df := math.Pow(std1*std1/float64(n1)+std2*std2/float64(n2), 2) /
		(math.Pow(std1*std1/float64(n1), 2)/float64(n1-1) +
			math.Pow(std2*std2/float64(n2), 2)/float64(n2-1))

	// Calculate p-value (two-tailed)
	// Simplified - in production use proper t-distribution
	pValue := 2.0 * (1.0 - normalCDF(math.Abs(t)))

	_ = df // Mark as used

	return pValue
}

// Helper functions

func (rs *RolloutStrategy) calculateSampleSize(trafficPercent float64) int {
	// Calculate required sample size for statistical power
	// Simplified - in production use proper power analysis
	baseSize := float64(rs.minSampleSize)
	return int(baseSize / (trafficPercent / 100.0))
}

func (rs *RolloutStrategy) estimateDuration(trafficPercent float64, minSamples int) time.Duration {
	// Estimate time to collect required samples
	// Assume 100 requests per second baseline
	requestsPerSecond := 100.0
	trafficFraction := trafficPercent / 100.0

	secondsNeeded := float64(minSamples) / (requestsPerSecond * trafficFraction)

	return time.Duration(secondsNeeded) * time.Second
}

func (rs *RolloutStrategy) createRollbackTriggers(_, _ *Solution) []RollbackTrigger {
	triggers := []RollbackTrigger{
		{
			Metric:    "error_rate",
			Threshold: 0.05, // 5% increase
			Direction: "increase",
		},
		{
			Metric:    "latency_p99",
			Threshold: 0.20, // 20% increase
			Direction: "increase",
		},
		{
			Metric:    "success_rate",
			Threshold: 0.02, // 2% decrease
			Direction: "decrease",
		},
	}

	return triggers
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stdDev(values []float64, mean float64) float64 {
	if len(values) <= 1 {
		return 0
	}

	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values) - 1)

	return math.Sqrt(variance)
}
