//go:build wasm

package supervisor

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"runtime"
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

	// Cached JS values to prevent memory leak from repeated Get() calls
	jsAtomics     js.Value
	jsInt32View   js.Value
	jsUint8View   js.Value // Cached Uint8Array view of SAB
	isWorker      bool     // Cached worker status
	jsInitialized bool

	// Optimization: Fixed-size LRU cache for subarrays (prevents memory leak)
	viewCache     map[uint64]js.Value
	viewCacheKeys []uint64 // LRU order: oldest at front
	viewCacheMax  int      // Max cache size (default 64)

	// Optimization: Scratch buffer for headers to avoid small allocations
	scratchBuf [8]byte

	// Profiling metrics
	profilingEnabled bool
	waitAsyncHits    uint64
	waitAsyncMisses  uint64
	totalReadTime    int64 // Nanoseconds
	totalWriteTime   int64 // Nanoseconds

	// GC Pressure Management: Track wait calls to yield for finalizer cleanup
	waitCallCount uint64
}

const defaultViewCacheMax = 64

// NewSABBridge creates a new SAB bridge
func NewSABBridge(sab unsafe.Pointer, size, inboxOffset, outboxOffset, epochOffset uint32) *SABBridge {
	bridge := &SABBridge{
		sab:           sab,
		sabSize:       size,
		inboxOffset:   inboxOffset,
		outboxOffset:  outboxOffset,
		epochOffset:   epochOffset,
		pollTimeout:   100 * time.Millisecond,
		pendingJobs:   make(map[string]chan *foundation.Result),
		viewCache:     make(map[uint64]js.Value),
		viewCacheKeys: make([]uint64, 0, defaultViewCacheMax),
		viewCacheMax:  defaultViewCacheMax,
	}

	// Cache JS values once to prevent memory leak
	bridge.initJSCache()

	return bridge
}

// IsReady returns true if the bridge has been initialized with memory capacity.
func (sb *SABBridge) IsReady() bool {
	return sb.sabSize > 0
}

// initJSCache initializes cached JS values (called once)
func (sb *SABBridge) initJSCache() {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.jsInitialized {
		return
	}

	// Only attempt JS calls if we're in a WASM environment
	// (Prevents panic during host-side testing if syscall/js is mocked/stubbed)
	defer func() {
		if r := recover(); r != nil {
			// Failed to Get globals, likely host-side test
			sb.jsInitialized = true
		}
	}()

	sb.jsAtomics = js.Global().Get("Atomics")
	sb.jsInt32View = js.Global().Get("__INOS_SAB_INT32__")

	// Cache Uint8Array view of the SAME buffer
	if !sb.jsInt32View.IsUndefined() {
		buffer := sb.jsInt32View.Get("buffer")
		sb.jsUint8View = js.Global().Get("Uint8Array").New(buffer)
	}

	// Cache worker context status
	sb.isWorker = sb.detectWorkerContext()

	sb.jsInitialized = true
}

// RegisterJob adds a job to the pending registry and returns a result channel
func (sb *SABBridge) RegisterJob(jobID string) chan *foundation.Result {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	ch := make(chan *foundation.Result, 1)
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

	data, err := sb.serializeJob(job)
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	if err := sb.writeToSAB(sb.inboxOffset, data); err != nil {
		return fmt.Errorf("failed to write to inbox: %w", err)
	}

	return nil
}

// PollCompletion waits for job completion using signal-based blocking
func (sb *SABBridge) PollCompletion(timeout time.Duration) (bool, error) {
	startEpoch := sb.readEpoch()
	timeoutMs := float64(timeout.Milliseconds())

	result := sb.WaitForEpochChange(
		sab_layout.IDX_OUTBOX_DIRTY,
		int32(startEpoch),
		timeoutMs,
	)

	currentEpoch := sb.readEpoch()
	if currentEpoch > startEpoch {
		return true, nil
	}

	_ = result
	return false, nil
}

// ReadOutboxRaw reads raw bytes from SAB outbox
func (sb *SABBridge) ReadOutboxRaw() ([]byte, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.readFromSAB(sb.outboxOffset, 1024*1024)
}

// ReadResult reads result from SAB outbox
func (sb *SABBridge) ReadResult() (*foundation.Result, error) {
	data, err := sb.ReadOutboxRaw()
	if err != nil {
		return nil, err
	}
	return sb.DeserializeResult(data), nil
}

func (sb *SABBridge) readEpoch() uint32 {
	ptr := unsafe.Add(sb.sab, sb.epochOffset)
	return atomic.LoadUint32((*uint32)(ptr))
}

func (sb *SABBridge) ReadOutboxSequence() uint32 {
	ptr := unsafe.Add(sb.sab, sab_layout.IDX_OUTBOX_DIRTY*4)
	return atomic.LoadUint32((*uint32)(ptr))
}

func (sb *SABBridge) ReadSystemEpoch() uint64 {
	ptr := unsafe.Add(sb.sab, sab_layout.IDX_SYSTEM_EPOCH*4)
	return uint64(atomic.LoadUint32((*uint32)(ptr)))
}

func (sb *SABBridge) ReadAtomicI32(epochIndex uint32) int32 {
	ptr := unsafe.Add(sb.sab, epochIndex*4)
	return int32(atomic.LoadUint32((*uint32)(ptr)))
}

func (sb *SABBridge) WriteInbox(data []byte) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.writeToSAB(sb.inboxOffset, data)
}

func (sb *SABBridge) SignalInbox() {
	ptr := unsafe.Add(sb.sab, sab_layout.IDX_INBOX_DIRTY*4)
	atomic.AddUint32((*uint32)(ptr), 1)
	sb.NotifyEpochWaiters(sab_layout.IDX_INBOX_DIRTY)
}

// SignalEpoch increments the epoch at the given index and notifies waiters
func (sb *SABBridge) SignalEpoch(index uint32) {
	ptr := unsafe.Add(sb.sab, index*4)
	atomic.AddUint32((*uint32)(ptr), 1)
	sb.NotifyEpochWaiters(index)
}

// WaitForEpochAsync returns a channel that closes when the epoch changes (Zero-Latency, Non-Blocking)
func (sb *SABBridge) WaitForEpochAsync(epochIndex uint32, expectedValue int32) <-chan struct{} {
	ch := make(chan struct{})

	if !sb.jsInitialized {
		sb.initJSCache()
	}

	// Fast path: Check if value already changed
	current := sb.ReadAtomicI32(epochIndex)
	if current != expectedValue {
		close(ch)
		return ch
	}

	// Async Wait (requires Atomics.waitAsync or fallback to polling)
	go func() {
		defer close(ch)

		// 1. Try waitAsync if available
		if !sb.jsAtomics.IsUndefined() {
			waitAsync := sb.jsAtomics.Get("waitAsync")
			if !waitAsync.IsUndefined() {
				// Promise-based wait
				// Atomics.waitAsync(typedArray, index, value) -> { async: boolean, value: "ok" | "not-equal" | "timed-out" | Promise }
				result := waitAsync.Invoke(sb.jsInt32View, epochIndex, expectedValue)

				isAsync := result.Get("async").Bool()
				if isAsync {
					atomic.AddUint64(&sb.waitAsyncHits, 1)
					promise := result.Get("value")

					// Create blocking channel for the promise callback
					done := make(chan struct{})

					var cb js.Func
					cb = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
						close(done)
						cb.Release()
						return nil
					})

					// promise.then(() => done)
					promise.Call("then", cb)

					// Wait for promise resolution (blocks this goroutine, but yields to runtime)
					<-done
					return
				}
				// If not async, it means value changed rapidly or error, returns immediately
				return
			}
		}

		// 2. Fallback: Efficient Polling (100ms) if waitAsync missing
		atomic.AddUint64(&sb.waitAsyncMisses, 1)
		// relaxed for "no heat" architecture
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if sb.ReadAtomicI32(epochIndex) != expectedValue {
				return
			}
		}
	}()

	return ch
}

// WaitForEpochChange blocks until the epoch at the given index changes from expectedValue.
// IMPORTANT: Each Atomics.wait() call creates a js.Value with a registered finalizer.
// In tight loops, this can exhaust the Go WASM runtime's finalizer table.
// We mitigate this by periodically yielding to allow GC to process finalizers.
func (sb *SABBridge) WaitForEpochChange(epochIndex uint32, expectedValue int32, timeoutMs float64) int {
	if !sb.jsInitialized {
		sb.initJSCache()
	}

	if !sb.isWorker {
		return sb.pollForEpochChange(epochIndex, expectedValue, timeoutMs)
	}

	if sb.jsAtomics.IsUndefined() || sb.jsInt32View.IsUndefined() {
		return 2
	}

	// GC Pressure Relief: Every 50 wait calls, yield to allow finalizer cleanup
	// This prevents finalizer table exhaustion in Go WASM
	count := atomic.AddUint64(&sb.waitCallCount, 1)
	if count%50 == 0 {
		runtime.Gosched()
	}

	// Call Atomics.wait - this creates a js.Value that will have a finalizer
	result := sb.jsAtomics.Call("wait", sb.jsInt32View, epochIndex, expectedValue, timeoutMs)

	// Use Type() check instead of String() to reduce additional js.Value allocations
	// js.TypeString = 7, but we need to compare the actual string value
	// Since we must extract the string, use it efficiently:
	resultStr := result.String()

	switch resultStr {
	case "ok":
		return 0
	case "not-equal":
		return 1
	default:
		return 2
	}
}

// detectWorkerContext detects if we're running in a Web Worker (Atomics.wait allowed)
func (sb *SABBridge) detectWorkerContext() bool {
	workerScope := js.Global().Get("WorkerGlobalScope")
	if workerScope.IsUndefined() {
		return false
	}
	self := js.Global().Get("self")
	if self.IsUndefined() {
		return false
	}
	return self.InstanceOf(workerScope)
}

// pollForEpochChange uses time.Sleep() polling as fallback
func (sb *SABBridge) pollForEpochChange(epochIndex uint32, expectedValue int32, timeoutMs float64) int {
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	for time.Now().Before(deadline) {
		if sb.ReadAtomicI32(epochIndex) != expectedValue {
			return 1
		}
		time.Sleep(50 * time.Millisecond)
	}
	return 2
}

// NotifyEpochWaiters wakes up threads waiting on the given epoch index.
func (sb *SABBridge) NotifyEpochWaiters(epochIndex uint32) int {
	if !sb.jsInitialized {
		sb.initJSCache()
	}

	if sb.jsAtomics.IsUndefined() || sb.jsInt32View.IsUndefined() {
		return 0
	}

	result := sb.jsAtomics.Call("notify", sb.jsInt32View, epochIndex)
	return result.Int()
}

// writeToSAB writes raw data to SAB Inbox
func (sb *SABBridge) writeToSAB(baseOffset uint32, data []byte) error {
	const HeaderSize = 8
	const RegionSize = sab_layout.SIZE_INBOX_TOTAL
	const DataCapacity = RegionSize - HeaderSize

	headPtr := (*uint32)(unsafe.Add(sb.sab, baseOffset))
	tailPtr := (*uint32)(unsafe.Add(sb.sab, baseOffset+4))

	head := atomic.LoadUint32(headPtr)
	tail := atomic.LoadUint32(tailPtr)

	var available uint32
	if tail >= head {
		available = DataCapacity - (tail - head) - 1
	} else {
		available = (head - tail) - 1
	}

	msgLen := uint32(len(data))
	totalLen := 4 + msgLen

	if available < totalLen {
		return fmt.Errorf("ring buffer full")
	}

	lenBytes := sb.scratchBuf[:4]
	binary.LittleEndian.PutUint32(lenBytes, msgLen)
	sb.writeRawRing(baseOffset, HeaderSize, DataCapacity, tail, lenBytes)
	tail = (tail + 4) % DataCapacity

	sb.writeRawRing(baseOffset, HeaderSize, DataCapacity, tail, data)
	tail = (tail + msgLen) % DataCapacity

	atomic.StoreUint32(tailPtr, tail)
	return nil
}

func (sb *SABBridge) writeRawRing(baseOffset, headerSize, capacity, writeIdx uint32, data []byte) {
	dataPtr := unsafe.Add(sb.sab, baseOffset+headerSize)
	toWrite := uint32(len(data))
	firstChunk := capacity - writeIdx
	if toWrite < firstChunk {
		firstChunk = toWrite
	}
	secondChunk := toWrite - firstChunk

	copy(unsafe.Slice((*byte)(unsafe.Add(dataPtr, uintptr(writeIdx))), firstChunk), data[:firstChunk])
	if secondChunk > 0 {
		copy(unsafe.Slice((*byte)(dataPtr), secondChunk), data[firstChunk:])
	}
}

func (sb *SABBridge) readFromSAB(baseOffset uint32, maxSize int) ([]byte, error) {
	const HeaderSize = 8
	const RegionSize = sab_layout.SIZE_INBOX_TOTAL
	const DataCapacity = RegionSize - HeaderSize

	headPtr := (*uint32)(unsafe.Add(sb.sab, baseOffset))
	tailPtr := (*uint32)(unsafe.Add(sb.sab, baseOffset+4))

	head := atomic.LoadUint32(headPtr)
	tail := atomic.LoadUint32(tailPtr)

	if head == tail {
		return nil, nil
	}

	var available uint32
	if tail >= head {
		available = tail - head
	} else {
		available = DataCapacity - (head - tail)
	}

	if available < 4 {
		return nil, nil
	}

	lenBytes := make([]byte, 4)
	sb.readRawRing(baseOffset, HeaderSize, DataCapacity, head, lenBytes)
	msgLen := binary.LittleEndian.Uint32(lenBytes)

	if int(msgLen) > maxSize {
		return nil, fmt.Errorf("message too large")
	}

	if available < 4+msgLen {
		return nil, nil
	}

	dataHead := (head + 4) % DataCapacity
	data := make([]byte, msgLen)
	sb.readRawRing(baseOffset, HeaderSize, DataCapacity, dataHead, data)

	atomic.StoreUint32(headPtr, (dataHead+msgLen)%DataCapacity)
	return data, nil
}

func (sb *SABBridge) readRawRing(baseOffset, headerSize, capacity, readIdx uint32, out []byte) {
	dataPtr := unsafe.Add(sb.sab, baseOffset+headerSize)
	toRead := uint32(len(out))
	firstChunk := capacity - readIdx
	if toRead < firstChunk {
		firstChunk = toRead
	}
	secondChunk := toRead - firstChunk

	copy(out[:firstChunk], unsafe.Slice((*byte)(unsafe.Add(dataPtr, uintptr(readIdx))), firstChunk))
	if secondChunk > 0 {
		copy(out[firstChunk:], unsafe.Slice((*byte)(dataPtr), secondChunk))
	}
}

// WriteRaw writes raw bytes to SAB at the specified offset
func (sb *SABBridge) WriteRaw(offset uint32, data []byte) error {
	if offset+uint32(len(data)) > sb.sabSize {
		return fmt.Errorf("out of bounds write")
	}
	if len(data) == 0 {
		return nil
	}

	// Address 0 safety
	if offset == 0 {
		view := sb.getJsUint8View()
		if !view.IsUndefined() {
			view.SetIndex(0, data[0])
			if len(data) > 1 {
				ptr := unsafe.Add(sb.sab, 1)
				copy(unsafe.Slice((*byte)(ptr), len(data)-1), data[1:])
			}
			return nil
		}
	}

	// FORCE JS INTEROP for correct SAB access
	// Go linear memory != SAB memory in this environment
	var startTime time.Time
	if sb.profilingEnabled {
		startTime = time.Now()
	}

	subView := sb.getCachedView(offset, uint32(len(data)))
	if !subView.IsUndefined() {
		copied := js.CopyBytesToJS(subView, data)

		if sb.profilingEnabled {
			atomic.AddInt64(&sb.totalWriteTime, int64(time.Since(startTime)))
		}

		if copied != len(data) {
			return fmt.Errorf("failed to copy all bytes to JS: expected %d, got %d", len(data), copied)
		}
		return nil
	}

	// Fallback (only works if memory is unified)
	ptr := unsafe.Add(sb.sab, offset)
	copy(unsafe.Slice((*byte)(ptr), len(data)), data)
	return nil
}

// ReadRaw reads raw bytes from SAB at the specified offset
func (sb *SABBridge) ReadRaw(offset uint32, size uint32) ([]byte, error) {
	if offset+size > sb.sabSize {
		return nil, fmt.Errorf("out of bounds read")
	}
	if size == 0 {
		return []byte{}, nil
	}

	// FORCE JS INTEROP for correct SAB access
	dest := make([]byte, size)
	if err := sb.ReadAt(offset, dest); err != nil {
		return nil, err
	}
	return dest, nil
}

// ReadAt reads raw bytes from SAB into the provided buffer (Zero Allocation if buffer reused)
func (sb *SABBridge) ReadAt(offset uint32, dest []byte) error {
	size := uint32(len(dest))
	if offset+size > sb.sabSize {
		return fmt.Errorf("out of bounds read: off=%d len=%d cap=%d", offset, size, sb.sabSize)
	}
	if size == 0 {
		return nil
	}

	// Go's linear memory is likely distinct from the SAB in this environment.
	// We use CopyBytesToGo to copy from the shared SAB into Go memory.
	// Optimization: Use Cached View
	var startTime time.Time
	if sb.profilingEnabled {
		startTime = time.Now()
	}

	subView := sb.getCachedView(offset, size)
	if !subView.IsUndefined() {
		copied := js.CopyBytesToGo(dest, subView)

		if sb.profilingEnabled {
			atomic.AddInt64(&sb.totalReadTime, int64(time.Since(startTime)))
		}

		if uint32(copied) != size {
			return fmt.Errorf("failed to copy all bytes: expected %d, got %d", size, copied)
		}
		return nil
	}

	// Fallback to unsafe (read from local linear memory)
	ptr := unsafe.Add(sb.sab, offset)
	copy(dest, unsafe.Slice((*byte)(ptr), size))
	return nil
}

// ReadBatch reads multiple non-contiguous SAB regions into corresponding buffers.
// More efficient than multiple ReadAt calls when reading from several regions.
type ReadRegion struct {
	Offset uint32
	Dest   []byte
}

func (sb *SABBridge) ReadBatch(regions []ReadRegion) error {
	root := sb.getJsUint8View()
	if root.IsUndefined() {
		// Fallback: individual reads
		for _, r := range regions {
			if err := sb.ReadAt(r.Offset, r.Dest); err != nil {
				return err
			}
		}
		return nil
	}

	// Batch read using single root view (avoid multiple subarray allocations)
	for _, r := range regions {
		size := uint32(len(r.Dest))
		if r.Offset+size > sb.sabSize {
			return fmt.Errorf("out of bounds batch read: off=%d len=%d", r.Offset, size)
		}
		// Use cached subview
		subView := sb.getCachedView(r.Offset, size)
		js.CopyBytesToGo(r.Dest, subView)
	}
	return nil
}

func (sb *SABBridge) getJsUint8View() js.Value {
	if !sb.jsInitialized || sb.jsUint8View.IsUndefined() {
		sb.initJSCache()
	}
	return sb.jsUint8View
}

// getCachedView returns a cached subarray view for the given range.
// Uses LRU eviction when cache exceeds viewCacheMax entries.
func (sb *SABBridge) getCachedView(offset, size uint32) js.Value {
	key := uint64(offset)<<32 | uint64(size)

	sb.mu.RLock()
	v, ok := sb.viewCache[key]
	sb.mu.RUnlock()
	if ok {
		return v
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	// Double check after acquiring write lock
	if v, ok = sb.viewCache[key]; ok {
		return v
	}

	root := sb.getJsUint8View()
	if root.IsUndefined() {
		return root
	}

	// LRU Eviction: Remove oldest entry if at capacity
	if len(sb.viewCacheKeys) >= sb.viewCacheMax {
		oldestKey := sb.viewCacheKeys[0]
		sb.viewCacheKeys = sb.viewCacheKeys[1:]
		delete(sb.viewCache, oldestKey)
	}

	// Create new view and add to cache
	v = root.Call("subarray", offset, offset+size)
	sb.viewCache[key] = v
	sb.viewCacheKeys = append(sb.viewCacheKeys, key)
	return v
}

// FlushViewCache clears the entire view cache.
// Call during epoch transitions or when memory pressure is high.
func (sb *SABBridge) FlushViewCache() {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	sb.viewCache = make(map[uint64]js.Value)
	sb.viewCacheKeys = sb.viewCacheKeys[:0]
}

// SetProfiling enables or disables bridge-level profiling
func (sb *SABBridge) SetProfiling(enabled bool) {
	sb.profilingEnabled = enabled
}

// GetProfilingStats returns current performance metrics
func (sb *SABBridge) GetProfilingStats() map[string]interface{} {
	return map[string]interface{}{
		"wait_async_hits":   atomic.LoadUint64(&sb.waitAsyncHits),
		"wait_async_misses": atomic.LoadUint64(&sb.waitAsyncMisses),
		"total_read_ns":     atomic.LoadInt64(&sb.totalReadTime),
		"total_write_ns":    atomic.LoadInt64(&sb.totalWriteTime),
	}
}

// WriteMetricsToSAB writes current metrics to the designated SAB region
func (sb *SABBridge) WriteMetricsToSAB() {
	if !sb.IsReady() {
		return
	}

	hits := atomic.LoadUint64(&sb.waitAsyncHits)
	misses := atomic.LoadUint64(&sb.waitAsyncMisses)
	readNs := atomic.LoadInt64(&sb.totalReadTime)
	writeNs := atomic.LoadInt64(&sb.totalWriteTime)

	// Layout (32 bytes):
	// [0:8]   waitAsyncHits
	// [8:16]  waitAsyncMisses
	// [16:24] totalReadTime
	// [24:32] totalWriteTime
	data := make([]byte, 32)
	binary.LittleEndian.PutUint64(data[0:8], hits)
	binary.LittleEndian.PutUint64(data[8:16], misses)
	binary.LittleEndian.PutUint64(data[16:24], uint64(readNs))
	binary.LittleEndian.PutUint64(data[24:32], uint64(writeNs))

	_ = sb.WriteRaw(sab_layout.OFFSET_BRIDGE_METRICS, data)
	// Signal metric update
	sb.SignalEpoch(sab_layout.IDX_METRICS_EPOCH)
}

func (sb *SABBridge) ValidateArenaOffset(offset, size uint32) error {
	if offset < sab_layout.OFFSET_ARENA {
		return fmt.Errorf("offset below arena")
	}
	return nil
}

func (sb *SABBridge) serializeJob(job *foundation.Job) ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	req, err := compute.NewRootCompute_JobRequest(seg)
	if err != nil {
		return nil, err
	}
	req.SetJobId(job.ID)
	req.SetLibrary(job.Type)
	req.SetMethod(job.Operation)
	params, _ := json.Marshal(job.Parameters)
	req.SetParams(params)
	req.SetInput(job.Data)
	return msg.Marshal()
}

func (sb *SABBridge) DeserializeResult(data []byte) *foundation.Result {
	if len(data) == 0 {
		return &foundation.Result{Success: false, Error: "no data"}
	}
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		return &foundation.Result{Success: false, Error: err.Error()}
	}
	res, _ := compute.ReadRootCompute_JobResult(msg)
	jobId, _ := res.JobId()
	output, _ := res.Output()
	errStr, _ := res.ErrorMessage()
	return &foundation.Result{
		JobID:   jobId,
		Success: res.Status() == compute.Compute_Status_success,
		Data:    output,
		Error:   errStr,
	}
}
