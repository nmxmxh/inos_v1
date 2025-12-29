package foundation

import "time"

// Priority levels
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
	PriorityEmergency
)

// Engine types
type EngineType int

const (
	EngineLearning EngineType = iota
	EngineOptimization
	EngineScheduling
	EngineSecurity
	EngineHealth
)

// Prediction types
type PredictionType int

const (
	PredictionLatency PredictionType = iota
	PredictionFailure
	PredictionLoad
	PredictionResource
)

// Decision types
type DecisionType int

const (
	DecisionRouting DecisionType = iota
	DecisionScheduling
	DecisionOptimization
	DecisionSecurity
)

// Knowledge node types
type NodeType uint16

const (
	NodeTypePattern NodeType = iota
	NodeTypeMetric
	NodeTypePrediction
	NodeTypeRule
	NodeTypeModel
)

// Relation types for knowledge edges
type RelationType int

const (
	RelationCauses RelationType = iota
	RelationCorrelates
	RelationDependsOn
	RelationSimilarTo
	RelationConflictsWith
)

// Job types
type JobType int

const (
	JobTypeCompute JobType = iota
	JobTypeML
	JobTypeGPU
	JobTypeScience
)

// Decision result structure
type Decision struct {
	Type       DecisionType
	Value      interface{}
	Confidence float32
	Reasoning  string
	Latency    time.Duration
}

// Workflow result
type WorkflowResult struct {
	Success  bool
	Output   interface{}
	Duration time.Duration
	Error    string
}

// Evidence represents supporting data for a node or relation
type Evidence struct {
	Source    string
	Field     string
	Value     float32
	Timestamp time.Time
}

// FeedbackType represents different types of intelligence feedback
type FeedbackType int

const (
	FeedbackPerformance FeedbackType = iota
	FeedbackAccuracy
	FeedbackLatency
	FeedbackCost
)

// String returns the string representation of FeedbackType
func (ft FeedbackType) String() string {
	switch ft {
	case FeedbackPerformance:
		return "performance"
	case FeedbackAccuracy:
		return "accuracy"
	case FeedbackLatency:
		return "latency"
	case FeedbackCost:
		return "cost"
	default:
		return "unknown"
	}
}

// Dispatcher allows the engine to request job execution from its supervisor
type Dispatcher interface {
	ExecuteJob(job *Job) *Result
}

// Job represents a unit of work
type Job struct {
	ID         string
	Type       string
	Operation  string
	Data       []byte
	Parameters map[string]interface{}
	Priority   int
	Deadline   time.Time
	Source     string

	// Prediction context
	Features         map[string]float64
	PredictedLatency time.Duration
	FailureRisk      float64

	// Internal
	ResultChan  chan *Result
	SubmittedAt time.Time
}

// Result represents job execution result
type Result struct {
	JobID       string
	Success     bool
	Data        []byte
	Error       string
	Latency     time.Duration
	Metrics     *ExecutionMetrics
	CompletedAt time.Time
}

// ExecutionMetrics tracks job execution metrics
type ExecutionMetrics struct {
	CPUTime    time.Duration
	MemoryUsed uint64
	GPUTime    time.Duration
	IOOps      uint64
}

// OptimizationResult contains optimization results
type OptimizationResult struct {
	Parameters map[string]float64
	Score      float64
	Iterations int
}

// Prediction contains predicted metrics
type Prediction struct {
	Latency       time.Duration
	FailureRisk   float64
	ResourceNeeds ResourceRequirements
	Confidence    float64
}

// ResourceRequirements specifies resource needs
type ResourceRequirements struct {
	CPU    float64
	Memory uint64
	GPU    float64
}

// Pattern represents a learned pattern
type Pattern struct {
	ID         string
	Type       string
	Frequency  float64
	Confidence float64
	Metadata   map[string]interface{}
}

// HealthStatus represents supervisor health
type HealthStatus struct {
	Healthy       bool
	Issues        []string
	LastCheck     time.Time
	Uptime        time.Duration
	JobsProcessed uint64
	ErrorRate     float64
}

// SupervisorMetrics contains supervisor metrics
type SupervisorMetrics struct {
	JobsSubmitted  uint64
	JobsCompleted  uint64
	JobsFailed     uint64
	AverageLatency time.Duration
	P99Latency     time.Duration
	Throughput     float64
	QueueDepth     int
	ActiveJobs     int
}

// ControlMessage for supervisor control
type ControlMessage struct {
	Type    string
	Payload interface{}
}

// HistoricalExecution represents past execution
type HistoricalExecution struct {
	JobType  string
	Latency  time.Duration
	Success  bool
	Features map[string]float64
}
