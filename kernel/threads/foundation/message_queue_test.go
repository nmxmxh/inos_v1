package foundation

import (
	"encoding/binary"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageQueue_EnqueueDequeue(t *testing.T) {
	sabSize := 1024 * 1024
	sab := make([]byte, sabSize)
	baseOffset := uint32(256) // Ensure enough space for head/tail pointers
	capacity := uint32(4)     // Small capacity to test wrapping

	mq := NewMessageQueue(sab, baseOffset, capacity)

	// Enqueue simple message
	payloadOffset, err := mq.EnqueueZeroCopy(1, 10, 5)
	require.NoError(t, err)

	// Write payload
	copy(sab[payloadOffset:], []byte("HELLO"))

	// Dequeue
	msgType, size, readOffset, err := mq.DequeueZeroCopy()
	require.NoError(t, err)
	assert.Equal(t, uint8(1), msgType)
	assert.Equal(t, uint16(5), size)

	// Read payload
	payload := make([]byte, size)
	copy(payload, sab[readOffset:readOffset+uint32(size)])
	assert.Equal(t, "HELLO", string(payload))
}

func TestMessageQueue_QueueFull(t *testing.T) {
	sabSize := 1024 * 1024
	sab := make([]byte, sabSize)
	baseOffset := uint32(256)
	capacity := uint32(2) // Capacity 2 means we can store 1 item? Or 2? usually capacity-1

	mq := NewMessageQueue(sab, baseOffset, capacity)

	// Fill queue
	// Tail increments. If nextTail == head, full.
	// Starts at 0. Enqueue 1 -> tail=1.
	// Enqueue 2 -> tail=0 (nextTail=1 == head? No head=0. Wait 1==0 false.)
	// Wait, nextTail = (tail + 1) & mask.
	// 0 -> 1.
	// 1 -> 0.
	// If capacity is 2. Mask is 1.
	// 0 -> 1.
	// 1 -> 0. (0 == 0 head). FULL.
	// So capacity 2 can hold 1 item.

	_, err := mq.EnqueueZeroCopy(1, 1, 10)
	require.NoError(t, err)

	_, err = mq.EnqueueZeroCopy(2, 1, 10)
	assert.Error(t, err)
	assert.Equal(t, "queue full", err.Error())
}

func TestMessageQueue_QueueEmpty(t *testing.T) {
	sabSize := 1024 * 1024
	sab := make([]byte, sabSize)
	baseOffset := uint32(256)
	capacity := uint32(4)

	mq := NewMessageQueue(sab, baseOffset, capacity)

	_, _, _, err := mq.DequeueZeroCopy()
	assert.Error(t, err)
	assert.Equal(t, "queue empty", err.Error())
}

func TestMessageQueue_FinalizeMessage(t *testing.T) {
	sabSize := 1024 * 1024
	sab := make([]byte, sabSize)
	baseOffset := uint32(256)
	capacity := uint32(4)

	mq := NewMessageQueue(sab, baseOffset, capacity)

	data := []byte{1, 2, 3, 4}
	offset, err := mq.EnqueueZeroCopy(1, 1, uint16(len(data)))
	require.NoError(t, err)

	// Write data
	copy(sab[offset:], data)

	// Finalize (update checksum in header)
	headerOffset := offset - 32 // MESSAGE_HEADER_SIZE
	mq.FinalizeMessage(headerOffset, data)

	// Verify checksum manually
	expectedChecksum := uint16(1 + 2 + 3 + 4)
	storedChecksum := binary.LittleEndian.Uint16(sab[headerOffset+24:])
	assert.Equal(t, expectedChecksum, storedChecksum)
}

func TestMessageQueue_PointersLocation(t *testing.T) {
	sabSize := 1024 * 1024
	sab := make([]byte, sabSize)
	baseOffset := uint32(256)
	capacity := uint32(16)

	mq := NewMessageQueue(sab, baseOffset, capacity)

	// Manually inspect pointers
	headPtr := (*uint32)(unsafe.Pointer(&sab[baseOffset-8]))
	tailPtr := (*uint32)(unsafe.Pointer(&sab[baseOffset-4]))

	assert.Equal(t, uint32(0), *headPtr)
	assert.Equal(t, uint32(0), *tailPtr)

	mq.EnqueueZeroCopy(1, 1, 10)
	assert.Equal(t, uint32(1), *tailPtr)

	mq.DequeueZeroCopy()
	assert.Equal(t, uint32(1), *headPtr)
}

func TestMessageQueue_WrapAround(t *testing.T) {
	sabSize := 1024 * 1024
	sab := make([]byte, sabSize)
	baseOffset := uint32(256)
	capacity := uint32(4) // Max items = 3

	mq := NewMessageQueue(sab, baseOffset, capacity)

	// 1. Fill to capacity (3 items)
	// Tail: 0 -> 1 -> 2 -> 3
	_, err := mq.EnqueueZeroCopy(1, 0, 10)
	require.NoError(t, err)
	_, err = mq.EnqueueZeroCopy(2, 0, 10)
	require.NoError(t, err)
	_, err = mq.EnqueueZeroCopy(3, 0, 10)
	require.NoError(t, err)

	// Verify Full
	_, err = mq.EnqueueZeroCopy(4, 0, 10)
	assert.Error(t, err)
	assert.Equal(t, "queue full", err.Error())

	// 2. Consume one item
	// Head: 0 -> 1
	msgType, _, _, err := mq.DequeueZeroCopy()
	require.NoError(t, err)
	assert.Equal(t, uint8(1), msgType) // Sequence 1? MsgType passed as 1.

	// 3. Enqueue one item (Wrap around)
	// Tail: 3 -> 0
	_, err = mq.EnqueueZeroCopy(5, 0, 10)
	require.NoError(t, err)

	// Verify pointers wrapping
	// Head should be 1. Tail should be 0.
	headPtr := (*uint32)(unsafe.Pointer(&sab[baseOffset-8]))
	tailPtr := (*uint32)(unsafe.Pointer(&sab[baseOffset-4]))

	assert.Equal(t, uint32(1), *headPtr)
	assert.Equal(t, uint32(0), *tailPtr)

	// 4. Verify FIFO order preserved
	// Next items: 2, 3, 5
	msgType, _, _, _ = mq.DequeueZeroCopy()
	assert.Equal(t, uint8(2), msgType)

	msgType, _, _, _ = mq.DequeueZeroCopy()
	assert.Equal(t, uint8(3), msgType)

	msgType, _, _, _ = mq.DequeueZeroCopy()
	assert.Equal(t, uint8(5), msgType)

	// Empty now
	_, _, _, err = mq.DequeueZeroCopy()
	assert.Error(t, err)
}
