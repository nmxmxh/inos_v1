package supervisor

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
