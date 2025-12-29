package supervisor

import (
	"sync"
	"sync/atomic"
	"time"
)

// FlowController manages backpressure and congestion control
type FlowController struct {
	sab         []byte
	supervisors map[uint8]*SupervisorState
	mu          sync.RWMutex
}

// SupervisorState tracks supervisor load and capacity
type SupervisorState struct {
	epochIndex     uint8
	queueDepth     uint32
	processingRate uint32 // messages/ms
	capacity       uint32
	lastUpdate     time.Time
	isCongested    uint32 // 0 = not congested, 1 = congested (for atomic ops)
}

// NewFlowController creates a new flow controller
func NewFlowController(sab []byte) *FlowController {
	return &FlowController{
		sab:         sab,
		supervisors: make(map[uint8]*SupervisorState),
	}
}

// RegisterSupervisor registers a supervisor for flow control
func (fc *FlowController) RegisterSupervisor(epochIndex uint8, capacity uint32) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.supervisors[epochIndex] = &SupervisorState{
		epochIndex:     epochIndex,
		queueDepth:     0,
		processingRate: 0,
		capacity:       capacity,
		lastUpdate:     time.Now(),
		isCongested:    0, // 0 = not congested
	}
}

// CanSend checks if we can send to target supervisor
func (fc *FlowController) CanSend(targetEpoch uint8) bool {
	fc.mu.RLock()
	state, exists := fc.supervisors[targetEpoch]
	fc.mu.RUnlock()

	if !exists {
		return true // Unknown supervisor, allow
	}

	// Check queue depth (80% capacity threshold)
	if state.queueDepth > state.capacity*8/10 {
		atomic.StoreUint32(&state.isCongested, 1)
		return false
	}

	// Check if congested
	if atomic.LoadUint32(&state.isCongested) != 0 {
		// Allow if queue has drained below 50%
		if state.queueDepth < state.capacity/2 {
			atomic.StoreUint32(&state.isCongested, 0)
			return true
		}
		return false
	}

	return true
}

// UpdateCongestion updates congestion state based on feedback
func (fc *FlowController) UpdateCongestion(
	srcEpoch, dstEpoch uint8,
	latency time.Duration,
	success bool,
) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	state, exists := fc.supervisors[dstEpoch]
	if !exists {
		return
	}

	// Update based on success/failure
	if !success {
		// Mark as congested
		atomic.StoreUint32(&state.isCongested, 1)
	} else if latency < time.Microsecond {
		// Fast response, likely not congested
		atomic.StoreUint32(&state.isCongested, 0)
	}

	state.lastUpdate = time.Now()
}

// UpdateQueueDepth updates queue depth for a supervisor
func (fc *FlowController) UpdateQueueDepth(epochIndex uint8, depth uint32) {
	fc.mu.RLock()
	state, exists := fc.supervisors[epochIndex]
	fc.mu.RUnlock()

	if exists {
		atomic.StoreUint32(&state.queueDepth, depth)
	}
}

// GetSupervisorState returns current state of a supervisor
func (fc *FlowController) GetSupervisorState(epochIndex uint8) *SupervisorState {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	state, exists := fc.supervisors[epochIndex]
	if !exists {
		return nil
	}

	// Return copy with converted isCongested
	return &SupervisorState{
		epochIndex:     state.epochIndex,
		queueDepth:     atomic.LoadUint32(&state.queueDepth),
		processingRate: atomic.LoadUint32(&state.processingRate),
		capacity:       state.capacity,
		lastUpdate:     state.lastUpdate,
		isCongested:    atomic.LoadUint32(&state.isCongested),
	}
}

// FlowStats returns flow control statistics
type FlowStats struct {
	TotalSupervisors int
	CongestedCount   int
	AvgQueueDepth    float32
	MaxQueueDepth    uint32
}

func (fc *FlowController) GetStats() FlowStats {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	stats := FlowStats{
		TotalSupervisors: len(fc.supervisors),
	}

	totalDepth := uint32(0)
	for _, state := range fc.supervisors {
		depth := atomic.LoadUint32(&state.queueDepth)
		totalDepth += depth

		if depth > stats.MaxQueueDepth {
			stats.MaxQueueDepth = depth
		}

		if atomic.LoadUint32(&state.isCongested) != 0 {
			stats.CongestedCount++
		}
	}

	if len(fc.supervisors) > 0 {
		stats.AvgQueueDepth = float32(totalDepth) / float32(len(fc.supervisors))
	}

	return stats
}
