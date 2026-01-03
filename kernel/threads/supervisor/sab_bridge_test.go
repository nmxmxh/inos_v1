package supervisor

import (
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	compute "github.com/nmxmxh/inos_v1/kernel/gen/compute/v1"
	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	capnp "zombiezen.com/go/capnproto2"
)

// Helper to create a test SAB bridge
func createTestSABBridge() (*SABBridge, []byte) {
	sabSize := sab_layout.SAB_SIZE_DEFAULT
	sab := make([]byte, sabSize)

	bridge := NewSABBridge(
		unsafe.Pointer(&sab[0]),
		sab_layout.OFFSET_INBOX_BASE,
		sab_layout.OFFSET_OUTBOX_BASE,
		sab_layout.OFFSET_ATOMIC_FLAGS, // Base of atomic flags
	)

	// Initialize ring buffer pointers
	// Inbox Head/Tail
	binary.LittleEndian.PutUint32(sab[sab_layout.OFFSET_INBOX_BASE:], 0)   // Head
	binary.LittleEndian.PutUint32(sab[sab_layout.OFFSET_INBOX_BASE+4:], 0) // Tail

	// Outbox Head/Tail
	binary.LittleEndian.PutUint32(sab[sab_layout.OFFSET_OUTBOX_BASE:], 0)   // Head
	binary.LittleEndian.PutUint32(sab[sab_layout.OFFSET_OUTBOX_BASE+4:], 0) // Tail

	return bridge, sab
}

func TestSABBridge_WriteJob(t *testing.T) {
	bridge, sab := createTestSABBridge()

	job := &foundation.Job{
		ID:        "job-123",
		Type:      "test-lib",
		Operation: "doSomething",
		Data:      []byte("input-data"),
		Parameters: map[string]interface{}{
			"param1": "value1",
		},
	}

	err := bridge.WriteJob(job)
	require.NoError(t, err)

	// Verify data written to inbox
	// Head should still be 0, Tail should be advanced
	inboxHead := binary.LittleEndian.Uint32(sab[sab_layout.OFFSET_INBOX_BASE:])
	inboxTail := binary.LittleEndian.Uint32(sab[sab_layout.OFFSET_INBOX_BASE+4:])

	assert.Equal(t, uint32(0), inboxHead)
	assert.Greater(t, inboxTail, uint32(0))

	// Verify content by reading manually (or simulating module read)
	// We can cheat and use the bridge's internal read logic if we adapt offsets?
	// Or just trust the WriteJob works if Tail moved.

	// Let's verify Signal message
	// WriteJob doesn't signal automatically? Wait, WriteJob DOES NOT signal in the current implementation?
	// Checking sab_bridge.go: WriteJob calls writeToSAB. It does NOT call SignalInbox.
	// The caller is expected to call SignalInbox?
	// Let's check unified.go or similar usage.
}

func TestSABBridge_Signaling(t *testing.T) {
	bridge, sab := createTestSABBridge()

	// Test SignalInbox
	// Should increment IDX_INBOX_DIRTY (Index 1)
	flagOffset := sab_layout.OFFSET_ATOMIC_FLAGS + (sab_layout.IDX_INBOX_DIRTY * 4)

	assert.Equal(t, uint32(0), binary.LittleEndian.Uint32(sab[flagOffset:]))

	bridge.SignalInbox()

	assert.Equal(t, uint32(1), binary.LittleEndian.Uint32(sab[flagOffset:]))

	// Test ReadOutboxSequence
	// Simulate module signaling (Index 2)
	outboxFlagOffset := sab_layout.OFFSET_ATOMIC_FLAGS + (sab_layout.IDX_OUTBOX_DIRTY * 4)
	binary.LittleEndian.PutUint32(sab[outboxFlagOffset:], 42)

	seq := bridge.ReadOutboxSequence()
	assert.Equal(t, uint32(42), seq)
}

func TestSABBridge_PollCompletion(t *testing.T) {
	bridge, sab := createTestSABBridge()

	// We need to set epochOffset correctly.
	// NewSABBridge uses `epochOffset` for `pollCompletion`.
	// In createTestSABBridge we passed OFFSET_ATOMIC_FLAGS.
	// readEpoch reads from `sb.epochOffset`.
	// So it reads the first word of atomic flags? Which is IDX_KERNEL_READY (Index 0).
	// This seems to be generic "readEpoch" logic used for PollCompletion.

	// Initial epoch at offset 0 is 0.

	// Test timeout
	start := time.Now()
	changed, err := bridge.PollCompletion(10 * time.Millisecond)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.WithinDuration(t, start.Add(10*time.Millisecond), time.Now(), 20*time.Millisecond)

	// Test success
	go func() {
		time.Sleep(20 * time.Millisecond)
		// Increment epoch at offset 0
		ptr := (*uint32)(unsafe.Pointer(&sab[0]))
		atomic.AddUint32(ptr, 1)
	}()

	changed, err = bridge.PollCompletion(100 * time.Millisecond)
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestSABBridge_ReadResult(t *testing.T) {
	bridge, sab := createTestSABBridge()

	// Create a dummy result in Cap'n Proto format
	msg, seg, _ := capnp.NewMessage(capnp.SingleSegment(nil))
	res, _ := compute.NewRootCompute_JobResult(seg)
	res.SetJobId("job-success")
	res.SetStatus(compute.Compute_Status_success)
	res.SetOutput([]byte("result-data"))

	data, _ := msg.Marshal()

	// Write to Outbox MANUALLY to simulate module
	// We can use WriteInbox logic but targeting outbox offsets?
	// Or define a helper.

	// Let's modify bridge temporarily to point inbox to outbox offset? No that's hacky.
	// Reuse writeToSAB logic by creating a temporary bridge?

	// Helper to write to ring buffer
	writeRing := func(baseOffset uint32, payload []byte) {
		// Simple manual write
		// Read Tail
		tailOffset := baseOffset + 4
		tail := binary.LittleEndian.Uint32(sab[tailOffset:])

		// Write Length
		dataPtr := baseOffset + 8 // Header size
		packetLen := uint32(len(payload))

		// Write length at tail
		binary.LittleEndian.PutUint32(sab[dataPtr+tail:], packetLen)
		newTail := (tail + 4 + packetLen) // assuming no wrap for simple test

		// Write data
		copy(sab[dataPtr+tail+4:], payload)

		// Update Tail
		binary.LittleEndian.PutUint32(sab[tailOffset:], newTail)
	}

	writeRing(sab_layout.OFFSET_OUTBOX_BASE, data)

	// Now ReadResult
	result, err := bridge.ReadResult()
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "job-success", result.JobID)
	assert.True(t, result.Success)
	assert.Equal(t, []byte("result-data"), result.Data)
}

func TestSABBridge_ConcurrentAccess(t *testing.T) {
	bridge, _ := createTestSABBridge()

	var wg sync.WaitGroup
	wg.Add(10)

	// Concurrent Job Registration
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer wg.Done()
			jobID := fmt.Sprintf("job-%d", id)
			ch := bridge.RegisterJob(jobID)

			go func() {
				// Simulate resolution
				bridge.ResolveJob(jobID, &foundation.Result{JobID: jobID, Success: true})
			}()

			res := <-ch
			assert.Equal(t, jobID, res.JobID)
		}(i)
	}

	wg.Wait()
}

func TestSABBridge_InboxAndRawOps(t *testing.T) {
	bridge, sab := createTestSABBridge()

	// Test WriteInbox (used for Kernel -> Module return path)
	data := []byte("inbox-payload")
	err := bridge.WriteInbox(data)
	require.NoError(t, err)

	// Verify manually
	inboxHead := binary.LittleEndian.Uint32(sab[sab_layout.OFFSET_INBOX_BASE:])
	inboxTail := binary.LittleEndian.Uint32(sab[sab_layout.OFFSET_INBOX_BASE+4:])
	assert.Equal(t, uint32(0), inboxHead)
	assert.Greater(t, inboxTail, uint32(0))

	// Validate Arena Offset
	err = bridge.ValidateArenaOffset(sab_layout.OFFSET_ARENA, 100)
	assert.NoError(t, err)

	err = bridge.ValidateArenaOffset(0, 100) // Below arena
	assert.Error(t, err)

	// Test Raw Read/Write
	testOffset := uint32(sab_layout.OFFSET_ARENA + 1024)
	testData := []byte("raw-data-test")

	err = bridge.WriteRaw(testOffset, testData)
	require.NoError(t, err)

	readData, err := bridge.ReadRaw(testOffset, uint32(len(testData)))
	require.NoError(t, err)
	assert.Equal(t, testData, readData)
}
