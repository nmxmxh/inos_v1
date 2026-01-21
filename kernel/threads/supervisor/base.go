package supervisor

import (
	"context"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
)

// BaseSupervisor defines the interface all supervisors must implement
type BaseSupervisor interface {
	// Lifecycle
	Start(ctx context.Context) error
	Stop() error

	// Job submission (non-blocking)
	Submit(job *foundation.Job) (<-chan *foundation.Result, error)
	SubmitBatch(jobs []*foundation.Job) (<-chan []*foundation.Result, error)

	// Intelligence integration (Cognitive Roles)
	Learn(job *foundation.Job, result *foundation.Result) error
	Optimize(job *foundation.Job) (*foundation.OptimizationResult, error)
	Predict(job *foundation.Job) (*foundation.Prediction, error)
	Schedule(job *foundation.Job) error
	Secure(job *foundation.Job) error
	Monitor(ctx context.Context) error

	// Collaboration
	Coordinate(job *foundation.Job, peer string) (*foundation.Result, error)
	SharePatterns(patterns []*foundation.Pattern) error

	// Health & observability
	Health() *foundation.HealthStatus
	Metrics() *foundation.SupervisorMetrics
	Anomalies() []string

	// Capabilities
	Capabilities() []string
	SupportsOperation(op string) bool
}
