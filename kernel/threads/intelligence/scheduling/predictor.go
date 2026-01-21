package scheduling

import (
	"math"
	"time"
)

// TimeSeriesPredictor predicts future metrics using time series analysis
type TimeSeriesPredictor struct {
	latencyHistory []float64
	loadHistory    []LoadSample
	windowSize     int
}

type LoadSample struct {
	Time   time.Time
	CPU    float64
	Memory float64
	GPU    float64
}

func NewTimeSeriesPredictor() *TimeSeriesPredictor {
	return &TimeSeriesPredictor{
		latencyHistory: make([]float64, 0),
		loadHistory:    make([]LoadSample, 0),
		windowSize:     100,
	}
}

// PredictLatency predicts job latency using exponential smoothing
func (tsp *TimeSeriesPredictor) PredictLatency(job *Job) time.Duration {
	if len(tsp.latencyHistory) == 0 {
		// No history, use job's estimated duration
		return job.Duration
	}

	// Exponential smoothing
	alpha := 0.3 // Smoothing factor
	forecast := tsp.latencyHistory[len(tsp.latencyHistory)-1]

	// Adjust for trend
	if len(tsp.latencyHistory) >= 2 {
		trend := tsp.latencyHistory[len(tsp.latencyHistory)-1] -
			tsp.latencyHistory[len(tsp.latencyHistory)-2]
		forecast += alpha * trend
	}

	// Adjust for job priority (higher priority jobs get more resources)
	priorityFactor := 1.0 / (1.0 + float64(job.Priority)/10.0)
	forecast *= priorityFactor

	return time.Duration(forecast) * time.Millisecond
}

// PredictFailureRisk predicts probability of job failure
func (tsp *TimeSeriesPredictor) PredictFailureRisk(job *Job) float64 {
	// Simple risk model based on resource requirements
	risk := 0.0

	// CPU risk
	if job.Resources.CPU > 8.0 {
		risk += 0.1
	}

	// Memory risk
	memoryGB := float64(job.Resources.Memory) / (1024 * 1024 * 1024)
	if memoryGB > 16.0 {
		risk += 0.1
	}

	// GPU risk
	if job.Resources.GPU > 0.8 {
		risk += 0.15
	}

	// Deadline pressure risk
	timeToDeadline := time.Until(job.Deadline)
	if timeToDeadline < job.Duration*2 {
		risk += 0.2 // Tight deadline
	}

	// Cap at 1.0
	if risk > 1.0 {
		risk = 1.0
	}

	return risk
}

// PredictLoad predicts future load using ARIMA-like approach
func (tsp *TimeSeriesPredictor) PredictLoad(horizon time.Duration) []LoadPrediction {
	if len(tsp.loadHistory) == 0 {
		return nil
	}

	predictions := make([]LoadPrediction, 0)
	steps := int(horizon.Minutes())

	// Get recent trend
	recentSamples := tsp.getRecentSamples(10)
	if len(recentSamples) == 0 {
		return nil
	}

	// Calculate trends
	cpuTrend := tsp.calculateTrend(recentSamples, func(s LoadSample) float64 { return s.CPU })
	memoryTrend := tsp.calculateTrend(recentSamples, func(s LoadSample) float64 { return s.Memory })
	gpuTrend := tsp.calculateTrend(recentSamples, func(s LoadSample) float64 { return s.GPU })

	// Get current values
	current := recentSamples[len(recentSamples)-1]

	// Forecast
	for i := 0; i < steps; i++ {
		futureTime := time.Now().Add(time.Duration(i+1) * time.Minute)

		// Simple linear extrapolation with dampening
		dampening := 1.0 / (1.0 + float64(i)*0.1)

		prediction := LoadPrediction{
			Time:       futureTime,
			CPU:        current.CPU + cpuTrend*float64(i+1)*dampening,
			Memory:     current.Memory + memoryTrend*float64(i+1)*dampening,
			GPU:        current.GPU + gpuTrend*float64(i+1)*dampening,
			Confidence: 1.0 / (1.0 + float64(i)*0.2), // Decreasing confidence
		}

		// Clamp values
		prediction.CPU = math.Max(0, math.Min(1.0, prediction.CPU))
		prediction.Memory = math.Max(0, math.Min(1.0, prediction.Memory))
		prediction.GPU = math.Max(0, math.Min(1.0, prediction.GPU))

		predictions = append(predictions, prediction)
	}

	return predictions
}

// RecordLatency records observed latency
func (tsp *TimeSeriesPredictor) RecordLatency(latency time.Duration) {
	tsp.latencyHistory = append(tsp.latencyHistory, float64(latency.Milliseconds()))

	// Keep window size
	if len(tsp.latencyHistory) > tsp.windowSize {
		tsp.latencyHistory = tsp.latencyHistory[1:]
	}
}

// RecordLoad records observed load
func (tsp *TimeSeriesPredictor) RecordLoad(sample LoadSample) {
	tsp.loadHistory = append(tsp.loadHistory, sample)

	// Keep window size
	if len(tsp.loadHistory) > tsp.windowSize {
		tsp.loadHistory = tsp.loadHistory[1:]
	}
}

// Helper functions

func (tsp *TimeSeriesPredictor) getRecentSamples(n int) []LoadSample {
	if len(tsp.loadHistory) == 0 {
		return nil
	}

	start := len(tsp.loadHistory) - n
	if start < 0 {
		start = 0
	}

	return tsp.loadHistory[start:]
}

func (tsp *TimeSeriesPredictor) calculateTrend(samples []LoadSample, extractor func(LoadSample) float64) float64 {
	if len(samples) < 2 {
		return 0
	}

	// Simple linear regression for trend
	n := float64(len(samples))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i, sample := range samples {
		x := float64(i)
		y := extractor(sample)

		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Slope = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	slope := (n*sumXY - sumX*sumY) / denominator
	return slope
}
