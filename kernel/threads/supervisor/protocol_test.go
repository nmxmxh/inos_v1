package supervisor

import (
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlowController_Thresholds(t *testing.T) {
	sab := make([]byte, 1024)
	fc := NewFlowController(sab)

	fc.RegisterSupervisor(1, 100)

	// 1. Initial state (not congested)
	assert.True(t, fc.CanSend(1))

	// 2. Over 80% threshold
	fc.UpdateQueueDepth(1, 85)
	assert.False(t, fc.CanSend(1))

	// 3. Check hysteresis (remains congested until < 50%)
	fc.UpdateQueueDepth(1, 60)
	assert.False(t, fc.CanSend(1))

	fc.UpdateQueueDepth(1, 40)
	assert.True(t, fc.CanSend(1))
}

func TestFlowController_Stats(t *testing.T) {
	fc := NewFlowController(nil)
	fc.RegisterSupervisor(1, 100)
	fc.RegisterSupervisor(2, 200)

	fc.UpdateQueueDepth(1, 10)
	fc.UpdateQueueDepth(2, 50)

	stats := fc.GetStats()
	assert.Equal(t, 2, stats.TotalSupervisors)
	assert.Equal(t, float32(30.0), stats.AvgQueueDepth)
	assert.Equal(t, uint32(50), stats.MaxQueueDepth)
}

func TestProtocol_SendReceive(t *testing.T) {
	sab := make([]byte, 1024*1024)
	// Base offsets matching SAB layout
	inbox := foundation.NewMessageQueue(sab, 1024, 256)
	outbox := foundation.NewMessageQueue(sab, 2048, 256)

	p := NewProtocol(sab, 1, outbox, inbox)
	require.NotNil(t, p)

	// Test non-blocking send (timeout 0)
	data := []byte("hello")
	err := p.SendWithGuarantee(2, 1, data, 0)
	assert.NoError(t, err)

	// Since we sent to outbox, let's "receive" it from the other side (simplified)
	// For testing ReceiveWithAck, we need data in the INBOX.
	// We'll manually enqueue data into inbox.
	payloadOffset, _ := inbox.EnqueueZeroCopy(1, 0, uint16(len(data)))
	copy(sab[payloadOffset:], data)
	headerOffset := payloadOffset - foundation.MESSAGE_HEADER_SIZE
	inbox.FinalizeMessage(headerOffset, data)

	msgType, received, err := p.ReceiveWithAck()
	assert.NoError(t, err)
	assert.Equal(t, uint8(1), msgType)
	assert.Equal(t, data, received)
}

func TestProtocol_AckTimeout(t *testing.T) {
	sab := make([]byte, 1024*1024)
	outbox := foundation.NewMessageQueue(sab, 1024, 256)
	p := NewProtocol(sab, 1, outbox, nil)

	// Test blocking send with timeout
	// This will timeout because no one calls NotifyAck
	err := p.SendWithGuarantee(2, 1, []byte("data"), 10*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestProtocol_NotifyAck(t *testing.T) {
	sab := make([]byte, 1024*1024)
	outbox := foundation.NewMessageQueue(sab, 1024, 256)
	p := NewProtocol(sab, 1, outbox, nil)

	// Run send in goroutine
	done := make(chan error, 1)
	go func() {
		done <- p.SendWithGuarantee(2, 1, []byte("data"), 100*time.Millisecond)
	}()

	// Give it a moment to register the pending ack
	time.Sleep(10 * time.Millisecond)

	// Notify success manually (msgID is likely 2 because NewProtocol starts at 1 and increments once)
	p.ackManager.NotifyAck(2, true)

	err := <-done
	assert.NoError(t, err)

	stats := p.ackManager.GetStats()
	assert.Equal(t, uint64(1), stats.AcksReceived)
}
