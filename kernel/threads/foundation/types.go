package foundation

import (
	"context"
	"time"
)

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

// MeshDelegator defines the interface for offloading tasks to the global mesh
type MeshDelegator interface {
	DelegateJob(ctx context.Context, job *Job) (*Result, error)
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

// CreditAccount in SAB for economic state (Unified v1.9)
// Size: 128 bytes (aligned to cache line x 2)
type CreditAccount struct {
	Balance           int64   // 8
	EarnedTotal       uint64  // 8
	SpentTotal        uint64  // 8
	LastActivityEpoch uint64  // 8
	ReputationScore   float32 // 4
	DeviceCount       uint16  // 2
	UptimeScore       float32 // 4
	LastUbiClaim      int64   // 8

	// Social-Economic Graph (Fixed Offsets for Zero-Copy)
	ReferrerLockedAt  int64 // 8
	ReferrerChangedAt int64 // 8

	// Yield Stats (Aggregated)
	FromCreator   uint64 // 8
	FromReferrals uint64 // 8
	FromCloseIds  uint64 // 8

	// Thresholds
	Threshold   uint8 // 1
	TotalShares uint8 // 1
	Tier        uint8 // 1

	// Pending credits (written by modules, finalized by supervisor)
	PendingBalance int64  // 8 (Net change)
	PendingEpoch   uint64 // 8
	PendingEarned  uint64 // 8 (Cumulative earned in epoch)
	PendingSpent   uint64 // 8 (Cumulative spent in epoch)

	// Alignment & Padding for 128 bytes
	Reserved [13]byte
}

// SocialEntry for the Social Graph region (maps to economy.capnp CloseIdentity fields)
type SocialEntry struct {
	OwnerDid    [64]byte // did:inos:<hash> (fixed size)
	ReferrerDid [64]byte
	CloseIds    [15][64]byte // Max 15 close IDs directly in SAB
	// Close ID metadata (epoch seconds, 0 = unset). Mirrors economy.capnp CloseIdentity fields.
	CloseIdAddedAt    [15]uint32
	CloseIdVerifiedAt [15]uint32
	Reserved          [40]byte
}

// IdentityEntry for the Identity Registry region (maps to identity.capnp + economy.capnp Wallet metadata)
type IdentityEntry struct {
	Did       [64]byte
	PublicKey [33]byte // Compressed Ed25519/X25519
	Status    uint8
	// Offsets into SAB regions (absolute offsets).
	AccountOffset uint32
	SocialOffset  uint32
	// Recovery + tier metadata (mirrors identity.capnp / economy.capnp).
	RecoveryThreshold uint8
	TotalShares       uint8
	Tier              uint8
	Flags             uint8
	Reserved          [18]byte
}

// ResourceMetrics for economic settlement
type ResourceMetrics struct {
	ComputeCyclesUsed   uint64  // CPU/GPU cycles consumed
	BytesServed         uint64  // Bandwidth used
	BytesStored         uint64  // Storage usage
	UptimeSeconds       uint64  // Availability
	LocalityScore       float32 // Network proximity (0-1)
	SyscallCount        uint64  // Kernel requests
	MemoryPressure      float32 // SAB usage ratio
	ReplicationPriority uint32  // Urgency
	SchedulingBias      int32   // Urgency offset
}

// EconomicRates for credit calculation
type EconomicRates struct {
	ComputeRate        float64
	BandwidthRate      float64
	StorageRate        float64
	UptimeRate         float64
	LocalityBonus      float64
	SyscallCost        float64
	ReplicationCost    float64
	SchedulingCost     float64
	PressureMultiplier float64
}

// EconomicVault defines the authority for economic state changes
type EconomicVault interface {
	GetBalance(did string) (int64, error)
	GrantBonus(did string, amount int64) error
}
