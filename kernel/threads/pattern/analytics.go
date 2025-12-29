package pattern

import (
	"sync"
	"sync/atomic"
	"time"
)

// PatternAnalytics provides analytics and monitoring for patterns
type PatternAnalytics struct {
	storage   *TieredPatternStorage
	collector *MetricsCollector
	analyzer  *PatternAnalyzer
	mu        sync.RWMutex
}

// MetricsCollector collects pattern metrics
type MetricsCollector struct {
	metrics AnalyticsMetrics
	mu      sync.RWMutex
}

// AnalyticsMetrics tracks comprehensive pattern analytics metrics
type AnalyticsMetrics struct {
	// Detection metrics
	PatternsDetected  uint64
	FalsePositives    uint64
	DetectionLatency  time.Duration
	DetectionAccuracy float32

	// Application metrics
	PatternsApplied    uint64
	SuccessRate        float32
	AvgImprovement     float32
	ApplicationLatency time.Duration

	// Evolution metrics
	PatternsEvolved  uint64
	EvolutionSuccess float32
	Generations      uint32
	MutationRate     float32

	// Storage metrics
	StorageUsage     StorageMetrics
	CacheHitRate     float32
	CompressionRatio float32

	// Performance metrics
	P50Latency time.Duration
	P95Latency time.Duration
	P99Latency time.Duration
}

// StorageMetrics tracks storage statistics
type StorageMetrics struct {
	Tier1Usage uint32
	Tier2Usage uint32
	Tier3Usage uint32
	Tier4Usage uint32
	TotalSize  uint64
}

// PatternAnalyzer analyzes pattern performance
type PatternAnalyzer struct {
	history []AnalyticsMetrics
	mu      sync.RWMutex
}

// NewPatternAnalytics creates a new pattern analytics system
func NewPatternAnalytics(storage *TieredPatternStorage) *PatternAnalytics {
	return &PatternAnalytics{
		storage: storage,
		collector: &MetricsCollector{
			metrics: AnalyticsMetrics{},
		},
		analyzer: &PatternAnalyzer{
			history: make([]AnalyticsMetrics, 0),
		},
	}
}

// CollectMetrics collects current metrics
func (pa *PatternAnalytics) CollectMetrics() AnalyticsMetrics {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	// Get storage stats
	storageStats := pa.storage.GetStats()

	// Calculate cache hit rate
	totalAccess := storageStats.CacheHits + storageStats.CacheMisses
	cacheHitRate := float32(0)
	if totalAccess > 0 {
		cacheHitRate = float32(storageStats.CacheHits) / float32(totalAccess)
	}

	metrics := AnalyticsMetrics{
		PatternsDetected: atomic.LoadUint64(&pa.collector.metrics.PatternsDetected),
		PatternsApplied:  atomic.LoadUint64(&pa.collector.metrics.PatternsApplied),
		PatternsEvolved:  atomic.LoadUint64(&pa.collector.metrics.PatternsEvolved),
		StorageUsage: StorageMetrics{
			Tier1Usage: storageStats.Tier1Count,
			Tier2Usage: storageStats.Tier2Count,
			Tier3Usage: storageStats.Tier3Count,
			Tier4Usage: storageStats.Tier4Count,
			TotalSize:  storageStats.TotalPatterns,
		},
		CacheHitRate: cacheHitRate,
	}

	return metrics
}

// RecordDetection records a pattern detection
func (pa *PatternAnalytics) RecordDetection(latency time.Duration, success bool) {
	atomic.AddUint64(&pa.collector.metrics.PatternsDetected, 1)
	if !success {
		atomic.AddUint64(&pa.collector.metrics.FalsePositives, 1)
	}
	// Update latency (simplified)
	pa.collector.mu.Lock()
	pa.collector.metrics.DetectionLatency = latency
	pa.collector.mu.Unlock()
}

// RecordApplication records a pattern application
func (pa *PatternAnalytics) RecordApplication(success bool, improvement float32, latency time.Duration) {
	atomic.AddUint64(&pa.collector.metrics.PatternsApplied, 1)

	pa.collector.mu.Lock()
	defer pa.collector.mu.Unlock()

	// Update success rate (moving average)
	applied := atomic.LoadUint64(&pa.collector.metrics.PatternsApplied)
	if success {
		pa.collector.metrics.SuccessRate = (pa.collector.metrics.SuccessRate*float32(applied-1) + 1.0) / float32(applied)
	} else {
		pa.collector.metrics.SuccessRate = (pa.collector.metrics.SuccessRate * float32(applied-1)) / float32(applied)
	}

	// Update improvement (moving average)
	pa.collector.metrics.AvgImprovement = (pa.collector.metrics.AvgImprovement*float32(applied-1) + improvement) / float32(applied)

	// Update latency
	pa.collector.metrics.ApplicationLatency = latency
}

// RecordEvolution records a pattern evolution
func (pa *PatternAnalytics) RecordEvolution(generation uint32, success bool) {
	atomic.AddUint64(&pa.collector.metrics.PatternsEvolved, 1)

	pa.collector.mu.Lock()
	defer pa.collector.mu.Unlock()

	pa.collector.metrics.Generations = generation

	// Update evolution success rate
	evolved := atomic.LoadUint64(&pa.collector.metrics.PatternsEvolved)
	if success {
		pa.collector.metrics.EvolutionSuccess = (pa.collector.metrics.EvolutionSuccess*float32(evolved-1) + 1.0) / float32(evolved)
	} else {
		pa.collector.metrics.EvolutionSuccess = (pa.collector.metrics.EvolutionSuccess * float32(evolved-1)) / float32(evolved)
	}
}

// AnalyzePerformance analyzes pattern performance trends
func (pa *PatternAnalytics) AnalyzePerformance() *PerformanceAnalysis {
	pa.analyzer.mu.Lock()
	defer pa.analyzer.mu.Unlock()

	// Record current metrics
	current := pa.CollectMetrics()
	pa.analyzer.history = append(pa.analyzer.history, current)

	// Keep last 100 snapshots
	if len(pa.analyzer.history) > 100 {
		pa.analyzer.history = pa.analyzer.history[1:]
	}

	// Calculate trends
	analysis := &PerformanceAnalysis{
		CurrentMetrics: current,
		Trends:         pa.analyzer.calculateTrends(),
		Anomalies:      pa.analyzer.detectAnomalies(),
	}

	return analysis
}

type PerformanceAnalysis struct {
	CurrentMetrics AnalyticsMetrics
	Trends         PerformanceTrends
	Anomalies      []Anomaly
}

type PerformanceTrends struct {
	DetectionRate    float32 // Patterns/sec
	ApplicationRate  float32 // Applications/sec
	SuccessRateTrend float32 // Increasing/decreasing
	CacheHitTrend    float32 // Increasing/decreasing
}

type Anomaly struct {
	Type     string
	Severity string
	Message  string
	Value    float32
}

func (pa *PatternAnalyzer) calculateTrends() PerformanceTrends {
	if len(pa.history) < 2 {
		return PerformanceTrends{}
	}

	// Compare last two snapshots
	current := pa.history[len(pa.history)-1]
	previous := pa.history[len(pa.history)-2]

	return PerformanceTrends{
		DetectionRate:    float32(current.PatternsDetected - previous.PatternsDetected),
		ApplicationRate:  float32(current.PatternsApplied - previous.PatternsApplied),
		SuccessRateTrend: current.SuccessRate - previous.SuccessRate,
		CacheHitTrend:    current.CacheHitRate - previous.CacheHitRate,
	}
}

func (pa *PatternAnalyzer) detectAnomalies() []Anomaly {
	var anomalies []Anomaly

	if len(pa.history) < 10 {
		return anomalies
	}

	current := pa.history[len(pa.history)-1]

	// Detect low cache hit rate
	if current.CacheHitRate < 0.8 {
		anomalies = append(anomalies, Anomaly{
			Type:     "CACHE_PERFORMANCE",
			Severity: "WARNING",
			Message:  "Cache hit rate below 80%",
			Value:    current.CacheHitRate,
		})
	}

	// Detect low success rate
	if current.SuccessRate < 0.7 {
		anomalies = append(anomalies, Anomaly{
			Type:     "APPLICATION_PERFORMANCE",
			Severity: "WARNING",
			Message:  "Pattern success rate below 70%",
			Value:    current.SuccessRate,
		})
	}

	// Detect high false positive rate
	if current.PatternsDetected > 0 {
		fpRate := float32(current.FalsePositives) / float32(current.PatternsDetected)
		if fpRate > 0.1 {
			anomalies = append(anomalies, Anomaly{
				Type:     "DETECTION_ACCURACY",
				Severity: "WARNING",
				Message:  "False positive rate above 10%",
				Value:    fpRate,
			})
		}
	}

	return anomalies
}

// GetMetrics returns current metrics
func (pa *PatternAnalytics) GetMetrics() AnalyticsMetrics {
	return pa.CollectMetrics()
}

// ResetMetrics resets all metrics
func (pa *PatternAnalytics) ResetMetrics() {
	pa.collector.mu.Lock()
	defer pa.collector.mu.Unlock()

	pa.collector.metrics = AnalyticsMetrics{}
}
