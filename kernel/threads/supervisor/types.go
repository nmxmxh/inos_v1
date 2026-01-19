package supervisor

import (
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
)

// Shared types and constants for supervisor package

// Message types
const (
	MSG_JOB_REQUEST   = 1
	MSG_JOB_COMPLETE  = 2
	MSG_RESOURCE_REQ  = 3
	MSG_PATTERN_SHARE = 4
	MSG_HEALTH_CHECK  = 5
)

// Priority levels
type MessagePriority uint8

const (
	PriorityCritical   MessagePriority = 0 // System health, OOM
	PriorityHigh       MessagePriority = 1 // Job requests, responses
	PriorityNormal     MessagePriority = 2 // Pattern sharing, coordination
	PriorityLow        MessagePriority = 3 // Statistics, monitoring
	PriorityBackground MessagePriority = 4 // Garbage collection, cleanup
)

// Allocation flags
type AllocFlags uint16

const (
	FlagUrgent      AllocFlags = 1 << 0 // Urgent message
	FlagAckRequired AllocFlags = 1 << 1 // Requires acknowledgment
	FlagRetry       AllocFlags = 1 << 2 // Retry on failure
	FlagOrdered     AllocFlags = 1 << 3 // Maintain order
)

// SABInterface defines the methods needed from the bridge for unit supervisors
type SABInterface interface {
	ReadRaw(offset uint32, size uint32) ([]byte, error)
	ReadAt(offset uint32, dest []byte) error                                  // Zero-allocation optimized read
	ReadAtomicI32(epochIndex uint32) int32                                    // Atomic read
	WaitForEpochAsync(epochIndex uint32, expectedValue int32) <-chan struct{} // Zero-latency wait
	WriteRaw(offset uint32, data []byte) error
	SignalInbox()
	SignalEpoch(index uint32)
	IsReady() bool                                    // Check if SAB is initialized
	RegisterJob(jobID string) chan *foundation.Result // Register job for completion
	ResolveJob(jobID string, result *foundation.Result)
	WriteJob(job *foundation.Job) error          // Write job to inbox
	WriteResult(result *foundation.Result) error // Write result to outbox
	ReadResult() (*foundation.Result, error)     // Read result from outbox
	GetFrameLatency() time.Duration
}
