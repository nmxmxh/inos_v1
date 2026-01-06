package supervisor

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"syscall/js"
	"time"
	"unsafe"

	compute "github.com/nmxmxh/inos_v1/kernel/gen/compute/v1"
	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	capnp "zombiezen.com/go/capnproto2"
)

// SABBridge provides non-blocking SAB communication
type SABBridge struct {
	sab          unsafe.Pointer // Pointer to SAB
	sabSize      uint32         // Actual capacity
	inboxOffset  uint32
	outboxOffset uint32
	epochOffset  uint32

	pollTimeout time.Duration
	pendingJobs map[string]chan *foundation.Result
	mu          sync.RWMutex
}

// NewSABBridge creates a new SAB bridge
func NewSABBridge(sab unsafe.Pointer, size, inboxOffset, outboxOffset, epochOffset uint32) *SABBridge {
	return &SABBridge{
		sab:          sab,
		sabSize:      size,
		inboxOffset:  inboxOffset,
		outboxOffset: outboxOffset,
		epochOffset:  epochOffset,
		pollTimeout:  100 * time.Millisecond,
		pendingJobs:  make(map[string]chan *foundation.Result),
	}
}

// RegisterJob adds a job to the pending registry and returns a result channel
func (sb *SABBridge) RegisterJob(jobID string) chan *foundation.Result {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	ch := make(chan *foundation.Result, 1) // Buffered to prevent blocking resolver
	sb.pendingJobs[jobID] = ch
	return ch
}

// ResolveJob resolves a pending job with a result
func (sb *SABBridge) ResolveJob(jobID string, result *foundation.Result) {
	sb.mu.Lock()
	ch, exists := sb.pendingJobs[jobID]
	if exists {
		delete(sb.pendingJobs, jobID)
	}
	sb.mu.Unlock()

	if exists {
		ch <- result
		close(ch)
	}
}

// WriteJob writes a job to SAB inbox (non-blocking)
func (sb *SABBridge) WriteJob(job *foundation.Job) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	// Serialize job to Cap'n Proto message
	data, err := sb.serializeJob(job)
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	// Write to inbox
	if err := sb.writeToSAB(sb.inboxOffset, data); err != nil {
		return fmt.Errorf("failed to write to inbox: %w", err)
	}

	return nil
}

// PollCompletion waits for job completion using signal-based blocking (zero CPU)
// Uses Atomics.wait instead of polling for true zero-CPU waiting
func (sb *SABBridge) PollCompletion(timeout time.Duration) (bool, error) {
	startEpoch := sb.readEpoch()
	timeoutMs := float64(timeout.Milliseconds())

	// Use signal-based waiting (Atomics.wait)
	result := sb.WaitForEpochChange(
		sab_layout.IDX_OUTBOX_DIRTY,
		int32(startEpoch),
		timeoutMs,
	)

	// Check if epoch actually changed
	currentEpoch := sb.readEpoch()
	if currentEpoch > startEpoch {
		return true, nil
	}

	// Result 2 = timed out, 0/1 = epoch changed but we double check above
	_ = result
	return false, nil
}

// ReadOutboxRaw reads raw bytes from SAB outbox
func (sb *SABBridge) ReadOutboxRaw() ([]byte, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	// Read from outbox
	return sb.readFromSAB(sb.outboxOffset, 1024*1024) // 1MB Limit
}

// ReadResult reads result from SAB outbox (Legacy/JobResult path)
func (sb *SABBridge) ReadResult() (*foundation.Result, error) {
	data, err := sb.ReadOutboxRaw()
	if err != nil {
		return nil, err
	}
	// Deserialize result
	result := sb.DeserializeResult(data)
	return result, nil
}

// Helper: Read epoch value (atomic)
func (sb *SABBridge) readEpoch() uint32 {
	ptr := unsafe.Add(sb.sab, sb.epochOffset)
	return atomic.LoadUint32((*uint32)(ptr))
}

// ReadOutboxSequence reads the atomic sequence counter for the Outbox
// Corresponds to IDX_OUTBOX_DIRTY (Index 2) in Atomic Flags region
func (sb *SABBridge) ReadOutboxSequence() uint32 {
	// Offset 0x000000 is Atomic Flags Base
	// Use standardized index from layout
	ptr := unsafe.Add(sb.sab, sab_layout.IDX_OUTBOX_DIRTY*4)
	return atomic.LoadUint32((*uint32)(ptr))
}

// ReadSystemEpoch reads the global system epoch counter (Index 7)
func (sb *SABBridge) ReadSystemEpoch() uint64 {
	ptr := unsafe.Add(sb.sab, sab_layout.IDX_SYSTEM_EPOCH*4)
	// Even though it's an i32/u32 in Atomic Flags, we treat it as uint64 for the economy loop
	return uint64(atomic.LoadUint32((*uint32)(ptr)))
}

// ReadAtomicI32 reads an atomic i32 value at the given epoch index
// Used by signal-based loops to check current epoch value
func (sb *SABBridge) ReadAtomicI32(epochIndex uint32) int32 {
	ptr := unsafe.Add(sb.sab, epochIndex*4)
	return int32(atomic.LoadUint32((*uint32)(ptr)))
}

// WriteInbox writes raw data to SAB Inbox (for Kernel -> Module return path)
// Thread-safe.
func (sb *SABBridge) WriteInbox(data []byte) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.writeToSAB(sb.inboxOffset, data)
}

// SignalInbox atomically increments the Inbox Sequence Counter/Flag
// Corresponds to IDX_INBOX_DIRTY (Index 1) in Atomic Flags region
func (sb *SABBridge) SignalInbox() {
	// Use standardized index from layout
	ptr := unsafe.Add(sb.sab, sab_layout.IDX_INBOX_DIRTY*4)
	atomic.AddUint32((*uint32)(ptr), 1)
	// Notify any waiters (Rust modules blocking on Atomics.wait)
	sb.NotifyEpochWaiters(sab_layout.IDX_INBOX_DIRTY)
}

// WaitForEpochChange blocks until the epoch at the given index changes from expectedValue.
// Uses Atomics.wait for true zero-CPU waiting (only works in Worker context).
// Returns: 0 = "ok" (value changed), 1 = "not-equal" (already different), 2 = "timed-out"
func (sb *SABBridge) WaitForEpochChange(epochIndex uint32, expectedValue int32, timeoutMs float64) int {
	// We need to use the actual SAB from the JavaScript side
	atomicsObj := js.Global().Get("Atomics")
	if atomicsObj.IsUndefined() {
		return 2 // Atomics not available, treat as timeout
	}

	// Get the Int32Array view that was set up during initialization
	// This usually exists in global scope if the host provided it
	int32View := js.Global().Get("__INOS_SAB_INT32__")
	if int32View.IsUndefined() {
		return 2
	}

	// Call Atomics.wait(int32Array, index, expectedValue, timeout)
	result := atomicsObj.Call("wait", int32View, epochIndex, expectedValue, timeoutMs)
	resultStr := result.String()

	switch resultStr {
	case "ok":
		return 0
	case "not-equal":
		return 1
	case "timed-out":
		return 2
	default:
		return 2
	}
}

// NotifyEpochWaiters wakes up threads waiting on the given epoch index.
// Returns the number of waiters that were notified.
func (sb *SABBridge) NotifyEpochWaiters(epochIndex uint32) int {
	atomicsObj := js.Global().Get("Atomics")
	if atomicsObj.IsUndefined() {
		return 0
	}

	int32View := js.Global().Get("__INOS_SAB_INT32__")
	if int32View.IsUndefined() {
		return 0
	}

	// Call Atomics.notify(int32Array, index, count) - notify all waiters
	result := atomicsObj.Call("notify", int32View, epochIndex, js.ValueOf(nil)) // nil = notify all
	return result.Int()
}

// Helper: Write to Ring Buffer (Inbox)
func (sb *SABBridge) writeToSAB(baseOffset uint32, data []byte) error {
	// Ring Buffer Header: [Head(4) | Tail(4)]
	// Data follows header.
	const HeaderSize = 8
	// Total size for Inbox/Outbox is 512KB (0x80000)
	const RegionSize = 0x80000
	const DataCapacity = RegionSize - HeaderSize

	headPtr := (*uint32)(unsafe.Add(sb.sab, baseOffset))
	tailPtr := (*uint32)(unsafe.Add(sb.sab, baseOffset+4))

	head := atomic.LoadUint32(headPtr)
	tail := atomic.LoadUint32(tailPtr)

	// Calculate available space
	var available uint32
	if tail >= head {
		available = DataCapacity - (tail - head) - 1
	} else {
		available = (head - tail) - 1
	}

	msgLen := uint32(len(data))
	totalLen := 4 + msgLen // [Len(4)][Data...]

	if available < totalLen {
		return fmt.Errorf("ring buffer full: needed %d, available %d", totalLen, available)
	}

	// Write Length (4 bytes)
	lenBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBytes, msgLen)
	if err := sb.writeRawRing(baseOffset, HeaderSize, DataCapacity, tail, lenBytes); err != nil {
		return err
	}
	tail = (tail + 4) % DataCapacity

	// Write Data
	if err := sb.writeRawRing(baseOffset, HeaderSize, DataCapacity, tail, data); err != nil {
		return err
	}
	tail = (tail + msgLen) % DataCapacity

	// Update Tail atomically
	atomic.StoreUint32(tailPtr, tail)

	return nil
}

// Helper: Circular write of raw bytes
func (sb *SABBridge) writeRawRing(baseOffset, headerSize, capacity, writeIdx uint32, data []byte) error {
	dataPtr := unsafe.Add(sb.sab, baseOffset+headerSize)

	toWrite := uint32(len(data))
	firstChunk := capacity - writeIdx
	if toWrite < firstChunk {
		firstChunk = toWrite
	}
	secondChunk := toWrite - firstChunk

	// First chunk
	dest1 := unsafe.Add(dataPtr, writeIdx)
	copy(unsafe.Slice((*byte)(dest1), firstChunk), data[:firstChunk])

	// Second chunk (wrap)
	if secondChunk > 0 {
		dest2 := dataPtr // Start of data region
		copy(unsafe.Slice((*byte)(dest2), secondChunk), data[firstChunk:])
	}
	return nil
}

// Helper: Read from Ring Buffer (Outbox)
func (sb *SABBridge) readFromSAB(baseOffset uint32, maxSize int) ([]byte, error) {
	const HeaderSize = 8
	const RegionSize = sab_layout.SIZE_INBOX_TOTAL
	const DataCapacity = RegionSize - HeaderSize

	headPtr := (*uint32)(unsafe.Add(sb.sab, baseOffset))
	tailPtr := (*uint32)(unsafe.Add(sb.sab, baseOffset+4))

	head := atomic.LoadUint32(headPtr)
	tail := atomic.LoadUint32(tailPtr)

	if head == tail {
		return nil, nil // Empty
	}

	// Available bytes to read
	var available uint32
	if tail >= head {
		available = tail - head
	} else {
		available = DataCapacity - (head - tail)
	}

	if available < 4 {
		return nil, nil // Partial write? Wait.
	}

	// Peek Length
	lenBytes := make([]byte, 4)
	sb.readRawRing(baseOffset, HeaderSize, DataCapacity, head, lenBytes)
	msgLen := binary.LittleEndian.Uint32(lenBytes)

	if int(msgLen) > maxSize {
		return nil, fmt.Errorf("message size %d exceeds limit %d", msgLen, maxSize)
	}

	if available < 4+msgLen {
		return nil, nil // Wait for full message
	}

	// Advance Head past Length
	// We don't update atomic Head yet, safe to just compute local next index
	dataHead := (head + 4) % DataCapacity

	// Read Data
	data := make([]byte, msgLen)
	sb.readRawRing(baseOffset, HeaderSize, DataCapacity, dataHead, data)

	// Update Head atomically
	newHead := (dataHead + msgLen) % DataCapacity
	atomic.StoreUint32(headPtr, newHead)

	return data, nil
}

// Helper: Circular read
func (sb *SABBridge) readRawRing(baseOffset, headerSize, capacity, readIdx uint32, out []byte) {
	dataPtr := unsafe.Add(sb.sab, baseOffset+headerSize)

	toRead := uint32(len(out))
	firstChunk := capacity - readIdx
	if toRead < firstChunk {
		firstChunk = toRead
	}
	secondChunk := toRead - firstChunk

	// First chunk
	src1 := unsafe.Add(dataPtr, readIdx)
	copy(out[:firstChunk], unsafe.Slice((*byte)(src1), firstChunk))

	// Second chunk
	if secondChunk > 0 {
		src2 := dataPtr
		copy(out[firstChunk:], unsafe.Slice((*byte)(src2), secondChunk))
	}
}

// WriteRaw writes raw bytes to SAB at the specified offset (no length prefix)
// Use this for Zero-Copy data transfer to Arena regions
func (sb *SABBridge) WriteRaw(offset uint32, data []byte) error {
	if offset+uint32(len(data)) > sb.sabSize {
		return fmt.Errorf("out of bounds write: 0x%x + %d > 0x%x", offset, len(data), sb.sabSize)
	}
	ptr := unsafe.Add(sb.sab, offset)
	copy(unsafe.Slice((*byte)(ptr), len(data)), data)
	return nil
}

// ReadRaw reads raw bytes from SAB at the specified offset (no length prefix)
// Checks boundaries implicitly by construction of slice, but caller should validate offset
func (sb *SABBridge) ReadRaw(offset uint32, size uint32) ([]byte, error) {
	if offset+size > sb.sabSize {
		return nil, fmt.Errorf("out of bounds read: 0x%x + %d > 0x%x", offset, size, sb.sabSize)
	}
	ptr := unsafe.Add(sb.sab, offset)
	data := make([]byte, size)
	copy(data, unsafe.Slice((*byte)(ptr), size))
	return data, nil
}

// ValidateArenaOffset checks if the given offset and size fall within the Arena region
func (sb *SABBridge) ValidateArenaOffset(offset, size uint32) error {
	if offset < sab_layout.OFFSET_ARENA {
		return fmt.Errorf("offset 0x%x is below Arena start 0x%x", offset, sab_layout.OFFSET_ARENA)
	}
	// Note: We use the actual SAB size here if available, or the default
	if offset+size > sab_layout.SAB_SIZE_MAX {
		return fmt.Errorf("offset+size 0x%x exceeds maximum SAB size 0x%x", offset+size, sab_layout.SAB_SIZE_MAX)
	}
	return nil
}

// Helper: Serialize job to Cap'n Proto message
func (sb *SABBridge) serializeJob(job *foundation.Job) ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	// Create root struct: Compute.JobRequest
	req, err := compute.NewRootCompute_JobRequest(seg)
	if err != nil {
		return nil, err
	}

	// Set fields
	if err := req.SetJobId(job.ID); err != nil {
		return nil, err
	}
	if err := req.SetLibrary(job.Type); err != nil {
		return nil, err
	}
	if err := req.SetMethod(job.Operation); err != nil {
		return nil, err
	}

	// Convert params map to JSON string to match schema (params @4 :Text)
	// This maintains compatibility with the engine's expectation of JSON params
	// even though the envelope is Cap'n Proto
	paramsJSON := []byte("{}")
	if job.Parameters != nil {
		// Best effort JSON marshalling
		if b, err := json.Marshal(job.Parameters); err == nil {
			paramsJSON = b
		}
	}
	if err := req.SetParams(paramsJSON); err != nil {
		return nil, err
	}

	if err := req.SetInput(job.Data); err != nil {
		return nil, err
	}

	// Simple defaults for now
	req.SetBudget(1000)
	req.SetPriority(128)
	req.SetTimeout(5000)

	return msg.Marshal()
}

// DeserializeResult deserializes result from Cap'n Proto message
func (sb *SABBridge) DeserializeResult(data []byte) *foundation.Result {
	if len(data) == 0 {
		return &foundation.Result{Success: false, Error: "empty result data"}
	}

	// Read message
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		// Fallback for legacy simple results if needed, or just error
		// This handles the transition period if an old module writes a simple result
		return &foundation.Result{Success: false, Error: fmt.Sprintf("capnp unmarshal failed: %v", err)}
	}

	// Read root: Compute.JobResult
	res, err := compute.ReadRootCompute_JobResult(msg)
	if err != nil {
		return &foundation.Result{Success: false, Error: fmt.Sprintf("invalid root struct: %v", err)}
	}

	jobId, _ := res.JobId()
	status := res.Status()
	success := status == compute.Compute_Status_success

	output, _ := res.Output()
	errStr, _ := res.ErrorMessage()

	// If failed, prefer the error message
	finalError := ""
	if !success {
		if errStr != "" {
			finalError = errStr
		} else {
			finalError = fmt.Sprintf("job failed with status: %v", status)
		}
	}

	return &foundation.Result{
		JobID:   jobId,
		Success: success,
		Data:    output,
		Error:   finalError,
	}
}
