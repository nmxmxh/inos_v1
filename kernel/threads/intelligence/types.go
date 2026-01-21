package intelligence

import (
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
)

// Re-export or alias if needed, but better to use foundation directly.
// For now, we will just remove the definitions here so users must use foundation.
// OR we can keep them as aliases if possible in Go (but Go doesn't have type aliases across packages easily for constants).
// So we will remove them.

// Decision context for intelligent decisions
type DecisionContext struct {
	Type        foundation.DecisionType
	Input       interface{}
	Constraints map[string]interface{}
	Timeout     time.Duration
}

// Job status
type JobStatus int

const (
	JobStatusPending JobStatus = iota
	JobStatusRunning
	JobStatusCompleted
	JobStatusFailed
	JobStatusCancelled
)

// Node status for DAG
type NodeStatus int

const (
	NodeStatusWaiting NodeStatus = iota
	NodeStatusReady
	NodeStatusRunning
	NodeStatusCompleted
	NodeStatusFailed
)

// Alert severity
type AlertSeverity int

const (
	AlertInfo AlertSeverity = iota
	AlertWarning
	AlertError
	AlertCritical
)

// Health aspects
type HealthAspect int

const (
	HealthCPU HealthAspect = iota
	HealthMemory
	HealthDisk
	HealthNetwork
	HealthLatency
)

// Metric types
type MetricType int

const (
	MetricCounter MetricType = iota
	MetricGauge
	MetricHistogram
)

// Health status
type HealthStatus int

const (
	HealthHealthy HealthStatus = iota
	HealthDegraded
	HealthUnhealthy
	HealthCritical
)

// Aggregation methods for ensemble
type AggregationMethod int

const (
	AggregationVoting AggregationMethod = iota
	AggregationWeighted
	AggregationStacking
)

// Objective targets
type ObjectiveTarget int

const (
	ObjectiveMinimize ObjectiveTarget = iota
	ObjectiveMaximize
)

// Optimization methods
type OptimizationMethod int

const (
	MethodGridSearch OptimizationMethod = iota
	MethodRandomSearch
	MethodBayesian
	MethodGenetic
	MethodGradient
)

// Scheduling algorithms
type SchedulingAlgorithm int

const (
	AlgorithmFIFO SchedulingAlgorithm = iota
	AlgorithmEDF
	AlgorithmSJF
	AlgorithmPriority
	AlgorithmPredictive
)

// Detection methods
type DetectionMethod int

const (
	DetectionSignature DetectionMethod = iota
	DetectionBehavioral
	DetectionAnomaly
	DetectionML
)

// Monitor types
type MonitorType int

const (
	MonitorMetrics MonitorType = iota
	MonitorLogs
	MonitorTraces
	MonitorSynthetic
)

// Validation types
type ValidationType int

const (
	ValidationSize ValidationType = iota
	ValidationFormat
	ValidationRange
	ValidationPattern
)
