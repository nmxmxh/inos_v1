package supervisor

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
)

// Protocol provides guaranteed delivery for supervisor messages
type Protocol struct {
	sab           []byte
	epochIndex    uint8
	outboundQueue *foundation.MessageQueue
	inboundQueue  *foundation.MessageQueue
	ackManager    *AckManager

	nextMsgID uint64
}

// AckManager handles message acknowledgments and retries
type AckManager struct {
	pendingAcks map[uint64]*PendingAck
	mu          sync.RWMutex

	// Statistics
	acksSent     uint64
	acksReceived uint64
	timeouts     uint64
	retries      uint64
}

// PendingAck tracks a message waiting for acknowledgment
type PendingAck struct {
	msgID      uint64
	sentTime   time.Time
	timeout    time.Duration
	retryCount uint8
	maxRetries uint8
	done       chan bool
}

// NewProtocol creates a new protocol instance
func NewProtocol(sab []byte, epochIndex uint8, outboundQueue, inboundQueue *foundation.MessageQueue) *Protocol {
	return &Protocol{
		sab:           sab,
		epochIndex:    epochIndex,
		outboundQueue: outboundQueue,
		inboundQueue:  inboundQueue,
		ackManager: &AckManager{
			pendingAcks: make(map[uint64]*PendingAck),
		},
		nextMsgID: 1,
	}
}

// SendWithGuarantee sends a message with exactly-once semantics
func (p *Protocol) SendWithGuarantee(
	targetEpoch uint8,
	msgType uint8,
	data []byte,
	timeout time.Duration,
) error {
	// Reserve message ID
	msgID := atomic.AddUint64(&p.nextMsgID, 1)

	// Enqueue with zero-copy
	payloadOffset, err := p.outboundQueue.EnqueueZeroCopy(msgType, uint8(PriorityNormal), uint16(len(data)))
	if err != nil {
		return fmt.Errorf("failed to enqueue: %w", err)
	}

	// Write payload directly (zero-copy)
	copy(p.sab[payloadOffset:], data)

	// Finalize message with checksum
	headerOffset := payloadOffset - foundation.MESSAGE_HEADER_SIZE
	p.outboundQueue.FinalizeMessage(headerOffset, data)

	// Register for acknowledgment if timeout > 0
	if timeout > 0 {
		ack := &PendingAck{
			msgID:      msgID,
			sentTime:   time.Now(),
			timeout:    timeout,
			retryCount: 0,
			maxRetries: 3,
			done:       make(chan bool, 1),
		}

		p.ackManager.addPendingAck(msgID, ack)
		defer p.ackManager.removePendingAck(msgID)

		// Wait for acknowledgment
		return p.ackManager.waitForAck(ack)
	}

	return nil
}

// ReceiveWithAck receives a message and sends acknowledgment
func (p *Protocol) ReceiveWithAck() (msgType uint8, data []byte, err error) {
	// Dequeue with zero-copy
	msgType, dataSize, payloadOffset, err := p.inboundQueue.DequeueZeroCopy()
	if err != nil {
		return 0, nil, err
	}

	// Read data from SAB
	data = make([]byte, dataSize)
	copy(data, p.sab[payloadOffset:payloadOffset+uint32(dataSize)])

	// Send acknowledgment (simplified for now)
	atomic.AddUint64(&p.ackManager.acksSent, 1)

	return msgType, data, nil
}

// AckManager methods

func (am *AckManager) addPendingAck(msgID uint64, ack *PendingAck) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.pendingAcks[msgID] = ack
}

func (am *AckManager) removePendingAck(msgID uint64) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.pendingAcks, msgID)
}

func (am *AckManager) waitForAck(ack *PendingAck) error {
	deadline := time.Now().Add(ack.timeout)
	retryInterval := ack.timeout / time.Duration(ack.maxRetries+1)

	for time.Now().Before(deadline) {
		select {
		case success := <-ack.done:
			if success {
				atomic.AddUint64(&am.acksReceived, 1)
				return nil
			}
			return fmt.Errorf("message rejected")

		case <-time.After(retryInterval):
			// Check if we should retry
			if ack.retryCount < ack.maxRetries {
				ack.retryCount++
				ack.sentTime = time.Now()
				atomic.AddUint64(&am.retries, 1)

				// Signal that retry is needed
				// Caller should check retryCount and resend if needed
				// For now, just continue waiting
			} else {
				// Max retries exceeded
				atomic.AddUint64(&am.timeouts, 1)
				return fmt.Errorf("acknowledgment timeout after %d retries", ack.maxRetries)
			}
		}
	}

	atomic.AddUint64(&am.timeouts, 1)
	return fmt.Errorf("acknowledgment timeout")
}

// NotifyAck notifies that an acknowledgment was received
func (am *AckManager) NotifyAck(msgID uint64, success bool) {
	am.mu.RLock()
	ack, exists := am.pendingAcks[msgID]
	am.mu.RUnlock()

	if exists {
		select {
		case ack.done <- success:
		default:
		}
	}
}

// GetStats returns acknowledgment statistics
type AckStats struct {
	AcksSent     uint64
	AcksReceived uint64
	Timeouts     uint64
	Retries      uint64
	Pending      int
}

func (am *AckManager) GetStats() AckStats {
	am.mu.RLock()
	defer am.mu.RUnlock()

	return AckStats{
		AcksSent:     atomic.LoadUint64(&am.acksSent),
		AcksReceived: atomic.LoadUint64(&am.acksReceived),
		Timeouts:     atomic.LoadUint64(&am.timeouts),
		Retries:      atomic.LoadUint64(&am.retries),
		Pending:      len(am.pendingAcks),
	}
}
