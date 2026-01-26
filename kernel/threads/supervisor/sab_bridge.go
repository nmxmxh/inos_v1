//go:build wasm

package supervisor

import (
	"encoding/binary"
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
	"github.com/nmxmxh/inos_v1/kernel/utils"
	capnp "zombiezen.com/go/capnproto2"
)

// SABBridge provides non-blocking SAB communication
type SABBridge struct {
	sab                unsafe.Pointer // Pointer to local replica
	replica            []byte         // Backing buffer for replica
	sabSize            uint32         // Actual capacity
	inboxOffset        uint32
	outboxHostOffset   uint32
	outboxKernelOffset uint32
	epochOffset        uint32

	pollTimeout time.Duration
	pendingJobs map[string]chan *foundation.Result
	mu          sync.RWMutex

	// Cached JS values to prevent memory leak from repeated Get() calls
	jsAtomics     js.Value
	jsInt32View   js.Value
	jsUint8View   js.Value // Cached Uint8Array view of SAB
	isWorker      bool     // Cached worker status
	jsInitialized bool
	jsSabOffset   uint32

	// Optimization: Fixed-size LRU cache for subarrays (prevents memory leak)
	viewCache     map[uint64]js.Value
	viewCacheKeys []uint64 // LRU order: oldest at front
	viewCacheMax  int      // Max cache size (default 64)

	// Optimization: Scratch buffer for headers to avoid small allocations
	scratchBuf [8]byte

	// Profiling metrics
	profilingEnabled  bool
	waitAsyncHits     uint64
	waitAsyncMisses   uint64
	waitAsyncCalls    uint64
	waitAsyncTimeouts uint64
	totalReadTime     int64 // Nanoseconds
	totalWriteTime    int64 // Nanoseconds

	// GC Pressure Management: Track wait calls to yield for finalizer cleanup
	waitCallCount uint64

	// Epoch-Diffused Cleanup: Track activity instead of time
	lastCleanupEpoch int32
	cleanupThreshold int32

	epochLoggerOnce sync.Once

	epochWatcherEnabled uint32
	epochWaitersMu      sync.Mutex
	epochWaiters        map[uint32]chan int32

	// Stability Monitor: Tracks frame-to-frame latency to detect throttling
	lastFrameTime time.Time
	frameLatency  time.Duration
}

const defaultViewCacheMax = 64

// NewSABBridge creates a new SAB bridge
func NewSABBridge(replica []byte, inboxOffset, outboxHostOffset, outboxKernelOffset, epochOffset uint32) *SABBridge {
	size := uint32(len(replica))
	var sab unsafe.Pointer
	if size > 0 {
		sab = unsafe.Pointer(&replica[0])
	}

	bridge := &SABBridge{
		sab:                sab,
		replica:            replica,
		sabSize:            size,
		inboxOffset:        inboxOffset,
		outboxHostOffset:   outboxHostOffset,
		outboxKernelOffset: outboxKernelOffset,
		epochOffset:        epochOffset,
		pollTimeout:        100 * time.Millisecond,
		pendingJobs:        make(map[string]chan *foundation.Result),
		viewCache:          make(map[uint64]js.Value),
		viewCacheKeys:      make([]uint64, 0, defaultViewCacheMax),
		viewCacheMax:       defaultViewCacheMax,
		cleanupThreshold:   100, // Cleanup every 100 epochs of activity
		epochWaiters:       make(map[uint32]chan int32),
	}

	// Cache JS values once to prevent memory leak
	bridge.initJSCache()
	bridge.startEpochLogger()

	return bridge
}

// IsReady returns true if the bridge has been initialized with memory capacity.
func (sb *SABBridge) IsReady() bool {
	return sb.sabSize > 0
}

// Size returns the SAB capacity in bytes.
func (sb *SABBridge) Size() uint32 {
	return sb.sabSize
}

// initJSCache initializes cached JS values (called once)
func (sb *SABBridge) initJSCache() {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	// Only attempt JS calls if we're in a WASM environment
	defer func() {
		if r := recover(); r != nil {
			sb.jsInitialized = true
		}
	}()

	sabOffsetVal := js.Global().Get("__INOS_SAB_OFFSET__")
	currentOffset := uint32(0)
	if sabOffsetVal.Type() == js.TypeNumber {
		currentOffset = uint32(sabOffsetVal.Int())
	}

	if sb.jsInitialized && sb.jsSabOffset == currentOffset &&
		!sb.jsInt32View.IsUndefined() && !sb.jsUint8View.IsUndefined() {
		return
	}

	sb.jsAtomics = js.Global().Get("Atomics")
	sb.jsSabOffset = currentOffset

	sab := js.Global().Get("__INOS_SAB__")
	view := js.Global().Get("__INOS_SAB_INT32__")
	if !view.IsUndefined() {
		byteOffset := view.Get("byteOffset")
		if byteOffset.Type() == js.TypeNumber && uint32(byteOffset.Int()) != currentOffset {
			view = js.Undefined()
		}
	}

	if view.IsUndefined() && !sab.IsUndefined() {
		// Covver the entire SAB for atomic access to any region (Inboxes, Outboxes, etc.)
		length := int(sb.sabSize / 4)
		view = js.Global().Get("Int32Array").New(sab, int(currentOffset), length)
		js.Global().Set("__INOS_SAB_INT32__", view)
	}

	sb.jsInt32View = view

	if !view.IsUndefined() {
		buffer := view.Get("buffer")
		if !buffer.IsUndefined() {
			if sb.sabSize > 0 {
				sb.jsUint8View = js.Global().Get("Uint8Array").New(buffer, int(currentOffset), int(sb.sabSize))
			} else {
				sb.jsUint8View = js.Global().Get("Uint8Array").New(buffer, int(currentOffset))
			}
		}
	}

	// Cache result string values for comparisons
	initJsResultValues()

	// Cache worker context status
	sb.isWorker = sb.detectWorkerContext()
	if sb.isWorker {
		// Proactively enable epoch watcher in worker context to avoid 50ms polling floor.
		// kernel.worker.ts starts dedicated JS threads for this.
		atomic.StoreUint32(&sb.epochWatcherEnabled, 1)
	}
	sb.jsInitialized = true
	if !sb.jsInt32View.IsUndefined() {
		utils.Info("JS Cache Initialized", utils.Uint64("offset", uint64(sb.jsSabOffset)), utils.Int("len", sb.jsInt32View.Length()))
	} else {
		utils.Warn("JS Cache Failure: Int32View is undefined")
	}
}

func (sb *SABBridge) startEpochLogger() {
	sb.epochLoggerOnce.Do(func() {
		// Startup Trace: Log epochs for the first 30 seconds to debug initial stalling
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			stopTime := time.Now().Add(30 * time.Second)

			for range ticker.C {
				if time.Now().After(stopTime) && !sb.profilingEnabled {
					return
				}
				birdEpoch := sb.ReadAtomicI32(sab_layout.IDX_BIRD_EPOCH)
				evoEpoch := sb.ReadAtomicI32(sab_layout.IDX_EVOLUTION_EPOCH)
				systemEpoch := sb.ReadAtomicI32(sab_layout.IDX_SYSTEM_EPOCH)

				utils.Info("[TRACE] Initial Heartbeat State",
					utils.Int("bird_epoch", int(birdEpoch)),
					utils.Int("evolution_epoch", int(evoEpoch)),
					utils.Int("system_epoch", int(systemEpoch)))
			}
		}()
	})
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

// MaybeCleanup checks if enough epochs have passed since last cleanup and runs cleanup if needed.
// This is epoch-driven (activity-based) NOT time-driven.
func (sb *SABBridge) MaybeCleanup(currentEpoch int32) {
	epochDelta := currentEpoch - sb.lastCleanupEpoch
	if epochDelta >= sb.cleanupThreshold {
		sb.cleanupStaleJobs()
		sb.FlushViewCache()
		sb.lastCleanupEpoch = currentEpoch
	}
}

// cleanupStaleJobs removes pending jobs that have been waiting too long.
// Since we're epoch-driven, we estimate staleness by job count rather than time.
func (sb *SABBridge) cleanupStaleJobs() {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	// In epoch-driven model, we don't track timestamps.
	// Instead, if cleanup runs frequently enough, stale jobs are those
	// still pending after several cleanup cycles.
	// For simplicity, we clear all pending jobs older than this call.
	// If needed, add per-job epoch tracking for finer control.

	// This clears jobs that have been pending for 100+ epochs of activity
	staleCount := 0
	for jobID, ch := range sb.pendingJobs {
		select {
		case ch <- &foundation.Result{JobID: jobID, Success: false, Error: "epoch timeout (cleanup)"}:
		default:
		}
		close(ch)
		delete(sb.pendingJobs, jobID)
		staleCount++
	}

	if staleCount > 0 {
		runtime.Gosched() // Yield to allow GC
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

	if err := sb.writeToSAB(sb.inboxOffset, sab_layout.SIZE_INBOX_TOTAL, data); err != nil {
		return fmt.Errorf("failed to write to inbox: %w", err)
	}

	return nil
}

// PollCompletion waits for job completion using signal-based blocking
func (sb *SABBridge) PollCompletion(timeout time.Duration) (bool, error) {
	startEpoch := sb.readEpoch()
	timeoutMs := float64(timeout.Milliseconds())

	result := sb.WaitForEpochChange(
		sab_layout.IDX_OUTBOX_HOST_DIRTY,
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

func (sb *SABBridge) ReadOutboxRaw() ([]byte, error) {
	return sb.readFromSAB(sb.outboxKernelOffset, sab_layout.SIZE_OUTBOX_KERNEL_TOTAL)
}

func (sb *SABBridge) ReadOutboxHostRaw() ([]byte, error) {
	return sb.readFromSAB(sb.outboxHostOffset, sab_layout.SIZE_OUTBOX_HOST_TOTAL)
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
	return sb.atomicLoad(sb.epochOffset)
}

func (sb *SABBridge) ReadOutboxSequence() uint32 {
	return sb.atomicLoad(sab_layout.IDX_OUTBOX_KERNEL_DIRTY)
}

func (sb *SABBridge) ReadSystemEpoch() uint64 {
	return uint64(sb.atomicLoad(sab_layout.IDX_SYSTEM_EPOCH))
}

func (sb *SABBridge) ReadAtomicI32(epochIndex uint32) int32 {
	val := int32(sb.atomicLoad(epochIndex))
	// Log only occasionally to avoid flooding
	if epochIndex == 12 && val%100 == 0 && val > 0 {
		utils.Debug("ReadAtomicI32", utils.Any("idx", epochIndex), utils.Any("val", val))
	}
	return val
}

func (sb *SABBridge) WriteInbox(data []byte) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.writeToSAB(sb.inboxOffset, sab_layout.SIZE_INBOX_TOTAL, data)
}

func (sb *SABBridge) WriteOutbox(data []byte) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	// Default to Host outbox (backwards compat / standard use case)
	if err := sb.writeToSAB(sb.outboxHostOffset, sab_layout.SIZE_OUTBOX_HOST_TOTAL, data); err != nil {
		return err
	}
	sb.SignalEpoch(sab_layout.IDX_OUTBOX_HOST_DIRTY)
	return nil
}

func (sb *SABBridge) WriteOutboxKernel(data []byte) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if err := sb.writeToSAB(sb.outboxKernelOffset, sab_layout.SIZE_OUTBOX_KERNEL_TOTAL, data); err != nil {
		return err
	}
	sb.SignalEpoch(sab_layout.IDX_OUTBOX_KERNEL_DIRTY)
	return nil
}

func (sb *SABBridge) SignalInbox() {
	sb.SignalEpoch(sab_layout.IDX_INBOX_DIRTY)
}

// SignalEpoch increments the epoch at the given index and notifies waiters
func (sb *SABBridge) SignalEpoch(index uint32) {
	sb.atomicAdd(index, 1)
	sb.NotifyEpochWaiters(index)

	if index != sab_layout.IDX_SYSTEM_EPOCH && shouldSignalSystemEpoch(index) {
		sb.atomicAdd(sab_layout.IDX_SYSTEM_EPOCH, 1)
		sb.NotifyEpochWaiters(sab_layout.IDX_SYSTEM_EPOCH)
	}
}

// GetAddress returns the SAB offset of the data if it's backed by the SAB
func (sb *SABBridge) GetAddress(data []byte) (uint32, bool) {
	if len(data) == 0 {
		return 0, false
	}

	// Address of the slice data in linear memory
	ptr := uintptr(unsafe.Pointer(&data[0]))
	sabBase := uintptr(sb.sab)

	if ptr >= sabBase && ptr < sabBase+uintptr(sb.sabSize) {
		return uint32(ptr - sabBase), true
	}

	return 0, false
}

// WaitForEpochAsync returns a channel that closes when the epoch changes (Zero-Latency, Non-Blocking)
func (sb *SABBridge) WaitForEpochAsync(epochIndex uint32, expectedValue int32) <-chan struct{} {
	ch := make(chan struct{})

	if !sb.jsInitialized || sb.jsInt32View.IsUndefined() {
		sb.initJSCache()
	}

	// Fast path: Check if value already changed
	current := sb.ReadAtomicI32(epochIndex)
	if current != expectedValue {
		close(ch)
		return ch
	}

	atomic.AddUint64(&sb.waitAsyncCalls, 1)

	// Async wrapper for blocking Atomics.wait (worker-only)
	go func() {
		defer close(ch)
		result := sb.WaitForEpochChange(epochIndex, expectedValue, 1000)
		if result == 0 || result == 1 {
			atomic.AddUint64(&sb.waitAsyncHits, 1)
			return
		}
		atomic.AddUint64(&sb.waitAsyncTimeouts, 1)
	}()

	return ch
}

// Cached string values for zero-allocation result comparison
var (
	jsOkValue       js.Value
	jsNotEqualValue js.Value
	jsResultsInit   bool
)

func initJsResultValues() {
	if jsResultsInit {
		return
	}
	jsOkValue = js.ValueOf("ok")
	jsNotEqualValue = js.ValueOf("not-equal")
	jsResultsInit = true
}

func (sb *SABBridge) WaitForEpochChange(epochIndex uint32, expectedValue int32, timeoutMs float64) int {
	// Fast Path: Check if already changed (prevents blocking)
	if sb.ReadAtomicI32(epochIndex) != expectedValue {
		return 1
	}

	// In Go/WASM, Atomics.wait is a HARD BLOCK on the entire Go runtime.
	// We MUST use reactive notification or polling to allow goroutines to yield.
	if atomic.LoadUint32(&sb.epochWatcherEnabled) == 1 {
		return sb.waitForEpochNotification(epochIndex, expectedValue, timeoutMs)
	}

	return sb.pollForEpochChange(epochIndex, expectedValue, timeoutMs)
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

func (sb *SABBridge) waitForEpochNotification(epochIndex uint32, expectedValue int32, timeoutMs float64) int {
	ch := sb.getEpochWaiter(epochIndex)
	timer := time.NewTimer(time.Duration(timeoutMs) * time.Millisecond)
	defer timer.Stop()

	for {
		select {
		case value := <-ch:
			if value != expectedValue {
				return 1
			}
			expectedValue = value
		case <-timer.C:
			return 2
		}
	}
}

func (sb *SABBridge) getEpochWaiter(epochIndex uint32) chan int32 {
	sb.epochWaitersMu.Lock()
	defer sb.epochWaitersMu.Unlock()

	ch := sb.epochWaiters[epochIndex]
	if ch == nil {
		ch = make(chan int32, 1)
		sb.epochWaiters[epochIndex] = ch
	}
	return ch
}

// PushEpochChange is called by JS when an epoch index changes.
func (sb *SABBridge) PushEpochChange(epochIndex uint32, value int32) {
	atomic.StoreUint32(&sb.epochWatcherEnabled, 1)

	// Update Stability Monitor if this is a physics pulse (Index 12)
	if epochIndex == sab_layout.IDX_BIRD_EPOCH {
		now := time.Now()
		if !sb.lastFrameTime.IsZero() {
			sb.frameLatency = now.Sub(sb.lastFrameTime)
		}
		sb.lastFrameTime = now
	}

	ch := sb.getEpochWaiter(epochIndex)

	select {
	case ch <- value:
		return
	default:
	}

	select {
	case <-ch:
	default:
	}

	select {
	case ch <- value:
	default:
	}
}

// NotifyEpochWaiters wakes up threads waiting on the given epoch index.
func (sb *SABBridge) NotifyEpochWaiters(epochIndex uint32) int {
	if !sb.jsInitialized || sb.jsInt32View.IsUndefined() {
		sb.initJSCache()
	}

	if sb.jsAtomics.IsUndefined() || sb.jsInt32View.IsUndefined() {
		return 0
	}

	result := sb.jsAtomics.Call("notify", sb.jsInt32View, sb.atomicIndex(epochIndex))
	return result.Int()
}

func (sb *SABBridge) SignalOutboxHost() {
	sb.SignalEpoch(sab_layout.IDX_OUTBOX_HOST_DIRTY)
}

func (sb *SABBridge) SignalOutboxKernel() {
	sb.SignalEpoch(sab_layout.IDX_OUTBOX_KERNEL_DIRTY)
}

// writeToSAB writes raw data to SAB Inbox/Outbox using MPSC pattern
func (sb *SABBridge) writeToSAB(baseOffset, regionSize uint32, data []byte) error {
	const HeaderSize = 8
	DataCapacity := regionSize - HeaderSize

	msgLen := uint32(len(data))
	totalLen := 4 + msgLen

	// 1. Reserve space atomically (MPSC)
	var reservedTail uint32
	for {
		// Atomic operations expect INDICES (uint32 array index), not BYTE OFFSETS.
		// baseOffset is a byte offset, so we must divide by 4.
		// Atomic operations expect INDICES (uint32 array index).
		// We use atomicLoadDirect to avoid flag offsets.
		head := sb.atomicLoadDirect(baseOffset / 4)
		tail := sb.atomicLoadDirect((baseOffset + 4) / 4)

		var available uint32
		if tail >= head {
			available = DataCapacity - (tail - head) - 1
		} else {
			available = (head - tail) - 1
		}

		if available < totalLen {
			return fmt.Errorf("ring buffer full")
		}

		newTail := (tail + totalLen) % DataCapacity

		if baseOffset == sb.outboxHostOffset || baseOffset == sb.outboxKernelOffset {
			utils.Info("DEBUG: writeToSAB Outbox Attempt",
				utils.Uint64("base", uint64(baseOffset)),
				utils.Uint64("tail", uint64(tail)),
				utils.Uint64("newTail", uint64(newTail)),
				utils.Int("dataLen", len(data)),
			)
		}

		if sb.atomicCASDirect((baseOffset+4)/4, tail, newTail) {
			reservedTail = tail
			break
		}
		// Retry on contention
	}

	// 2. Write Data first (skipping the 4-byte length slot)
	dataStart := (reservedTail + 4) % DataCapacity
	sb.writeRawRing(baseOffset, HeaderSize, DataCapacity, dataStart, data)

	// 3. Commit: Write Length Header LAST
	lenBytes := sb.scratchBuf[:4]
	binary.LittleEndian.PutUint32(lenBytes, msgLen)
	sb.writeRawRing(baseOffset, HeaderSize, DataCapacity, reservedTail, lenBytes)

	// 4. Synchronize: Push local write to Global SAB
	// Crucial: Only push the data region, leave Head/Tail (Metadata) to Atomic management
	sb.commitToJS(baseOffset+HeaderSize, DataCapacity)

	return nil
}

func (sb *SABBridge) commitToJS(offset, size uint32) {
	if !sb.jsInitialized || sb.jsUint8View.IsUndefined() {
		sb.initJSCache()
	}
	if !sb.jsUint8View.IsUndefined() {
		// Bulk push from Go's local replica to JS's Global SharedArrayBuffer
		target := sb.jsUint8View.Call("subarray", offset, offset+size)
		js.CopyBytesToJS(target, sb.replica[offset:offset+size])
	}
}

func (sb *SABBridge) pullFromJS(offset, size uint32) {
	if !sb.jsInitialized || sb.jsUint8View.IsUndefined() {
		sb.initJSCache()
	}
	if !sb.jsUint8View.IsUndefined() {
		// Bulk pull from Global SAB to Go's local replica
		src := sb.jsUint8View.Call("subarray", offset, offset+size)
		js.CopyBytesToGo(sb.replica[offset:offset+size], src)
	}
}

func (sb *SABBridge) RefreshRegistry() {
	sb.pullFromJS(sab_layout.OFFSET_MODULE_REGISTRY, sab_layout.SIZE_MODULE_REGISTRY)
}

// pullRingRegion selectively pulls a specific region of the ring buffer from JS to Go
// Handles wrap-around automatically.
func (sb *SABBridge) pullRingRegion(baseOffset, headerSize, capacity, startIdx, length uint32) {
	dataBase := baseOffset + headerSize
	firstChunk := capacity - startIdx
	if length <= firstChunk {
		sb.pullFromJS(dataBase+startIdx, length)
	} else {
		// Wrap around: pull end first, then start
		sb.pullFromJS(dataBase+startIdx, firstChunk)
		sb.pullFromJS(dataBase, length-firstChunk)
	}
}

func (sb *SABBridge) RefreshEconomics() {
	sb.pullFromJS(sab_layout.OFFSET_ECONOMICS, sab_layout.SIZE_ECONOMICS)
}

func (sb *SABBridge) writeRawRing(baseOffset, headerSize, capacity, writeIdx uint32, data []byte) {
	// Debug tracing for ring buffer writes
	if sb.profilingEnabled || ((baseOffset == sb.outboxHostOffset || baseOffset == sb.outboxKernelOffset) && len(data) > 0) {
		previewLen := 16
		if len(data) < previewLen {
			previewLen = len(data)
		}
		utils.Debug("writeRawRing",
			utils.Uint64("base_offset", uint64(baseOffset)),
			utils.Uint64("write_idx", uint64(writeIdx)),
			utils.Int("len", len(data)),
			utils.String("preview", fmt.Sprintf("%x", data[:previewLen])))
	}
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

func (sb *SABBridge) readFromSAB(baseOffset, regionSize uint32) ([]byte, error) {
	const HeaderSize = 8
	DataCapacity := regionSize - HeaderSize

	// 1. Optimistic CAS Loop to claim the message
	var head, tail, msgLen, nextHead, dataHead uint32

	for {
		// Read Head/Tail from authoritative Global SAB
		// Atomic operations expect INDICES (uint32 array index), not BYTE OFFSETS.
		// Read Head/Tail directly using absolute indices
		head = sb.atomicLoadDirect(baseOffset / 4)
		tail = sb.atomicLoadDirect((baseOffset + 4) / 4)

		if head == tail {
			return nil, nil // Empty
		}

		// Calculate DataHead
		dataHead = (head + 4) % DataCapacity

		// PRODUCTION GRADE: Selective Pull (Header Only)
		// We pull strictly the 4 bytes needed for length, handling wrap-around.
		sb.pullRingRegion(baseOffset, HeaderSize, DataCapacity, head, 4)

		// Peek length from local replica
		lenBytes := sb.scratchBuf[:4]
		sb.readRawRing(baseOffset, HeaderSize, DataCapacity, head, lenBytes)
		msgLen = binary.LittleEndian.Uint32(lenBytes)

		if msgLen == 0 {
			// Producer reserved space but hasn't committed length yet.
			// This is a race with the Producer's Commit phase.
			// Return nil to back off and retry later.
			return nil, nil
		}

		if int(msgLen) > int(regionSize) {
			return nil, fmt.Errorf("message too large: %d", msgLen)
		}

		// Calculate NextHead
		nextHead = (dataHead + msgLen) % DataCapacity

		// ATOMIC CLAIM: Try to advance Head from 'head' to 'nextHead'
		// This is the Critical Section entry for MPMC consumers.
		// ATOMIC CLAIM: Try to advance Head directly
		if sb.atomicCASDirect(baseOffset/4, head, nextHead) {
			// Success! We claimed this message.
			break
		}
		// Failure: Another consumer advanced Head. Loop and try again with new Head.
	}

	// 2. Read Data
	// Now that we own the message, we selectively pull the payload.
	sb.pullRingRegion(baseOffset, HeaderSize, DataCapacity, dataHead, msgLen)

	data := make([]byte, msgLen)
	sb.readRawRing(baseOffset, HeaderSize, DataCapacity, dataHead, data)

	// 3. Clear Header in Relay/Replica (Local Hygiene)
	// We do NOT write this back to Global SAB because we already moved Head.
	zeroBytes := []byte{0, 0, 0, 0}
	sb.writeRawRing(baseOffset, HeaderSize, DataCapacity, head, zeroBytes)

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

	if !sb.jsInitialized || sb.jsUint8View.IsUndefined() {
		sb.initJSCache()
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

	if !sb.jsInitialized || sb.jsUint8View.IsUndefined() {
		sb.initJSCache()
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
	if !sb.jsInitialized || sb.jsUint8View.IsUndefined() {
		sb.initJSCache()
	}

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

// getJsUint8View returns the cached Uint8Array view of the SAB.
// CRITICAL: This method NO LONGER LOCKS to avoid recursive deadlock when called from locked methods.
func (sb *SABBridge) getJsUint8View() js.Value {
	// initJSCache is guaranteed to have run in constructor
	return sb.jsUint8View
}

func (sb *SABBridge) atomicOffset(index uint32) uint32 {
	return sab_layout.OFFSET_ATOMIC_FLAGS + index*4
}

func (sb *SABBridge) atomicIndex(index uint32) uint32 {
	return sb.atomicOffset(index) / 4
}

func (sb *SABBridge) atomicLoad(index uint32) uint32 {
	if !sb.jsInitialized || sb.jsInt32View.IsUndefined() {
		sb.initJSCache()
	}
	if !sb.jsAtomics.IsUndefined() && !sb.jsInt32View.IsUndefined() {
		val := sb.jsAtomics.Call("load", sb.jsInt32View, sb.atomicIndex(index))
		return uint32(val.Int())
	}
	ptr := unsafe.Add(sb.sab, sb.atomicOffset(index))
	val := atomic.LoadUint32((*uint32)(ptr))
	// Log fallback (this is usually a bug if sync is expected)
	utils.Warn("atomicLoad fallback to local memory", utils.Int("idx", int(index)), utils.Int("val", int(val)))
	return val
}

func (sb *SABBridge) atomicAdd(index uint32, delta uint32) uint32 {
	if !sb.jsInitialized || sb.jsInt32View.IsUndefined() {
		sb.initJSCache()
	}
	if !sb.jsAtomics.IsUndefined() && !sb.jsInt32View.IsUndefined() {
		val := sb.jsAtomics.Call("add", sb.jsInt32View, sb.atomicIndex(index), int32(delta))
		return uint32(val.Int())
	}
	ptr := unsafe.Add(sb.sab, sb.atomicOffset(index))
	return atomic.AddUint32((*uint32)(ptr), delta)
}

// atomicLoadDirect loads from the SAB at the absolute index (no flag offset)
func (sb *SABBridge) atomicLoadDirect(index uint32) uint32 {
	if !sb.jsInitialized || sb.jsInt32View.IsUndefined() {
		sb.initJSCache()
	}
	if !sb.jsAtomics.IsUndefined() && !sb.jsInt32View.IsUndefined() {
		val := sb.jsAtomics.Call("load", sb.jsInt32View, index)
		return uint32(val.Int())
	}
	// Fallback uses absolute index stored in sab pointer (assuming sab points to base 0)
	// But in split memory, sab IS the private heap.
	// We calculate ptr based on index * 4 (since index is int32 index)
	byteOffset := index * 4
	ptr := unsafe.Add(sb.sab, byteOffset)
	val := atomic.LoadUint32((*uint32)(ptr))
	return val
}

// atomicCASDirect performs CAS at the absolute index (no flag offset)
func (sb *SABBridge) atomicCASDirect(index uint32, old, new uint32) bool {
	if !sb.jsInitialized || sb.jsInt32View.IsUndefined() {
		sb.initJSCache()
	}
	if !sb.jsAtomics.IsUndefined() && !sb.jsInt32View.IsUndefined() {
		val := sb.jsAtomics.Call("compareExchange", sb.jsInt32View, index, int32(old), int32(new))
		return uint32(val.Int()) == old
	}
	// Fallback
	byteOffset := index * 4
	ptr := unsafe.Add(sb.sab, byteOffset)
	return atomic.CompareAndSwapUint32((*uint32)(ptr), old, new)
}

func shouldSignalSystemEpoch(index uint32) bool {
	// Universal Heartbeat: Signal system epoch for ANY change in the flags region
	// except for the system epoch itself (index 7) or pulse (index 8).
	// This ensures IDX_SYSTEM_EPOCH is the master frequency of the entire OS.
	return index != sab_layout.IDX_SYSTEM_EPOCH && index != sab_layout.IDX_SYSTEM_PULSE
}

// AtomicLoad exposes atomic read for custom SAB indices (flags region only).
func (sb *SABBridge) AtomicLoad(index uint32) uint32 {
	return sb.atomicLoad(index)
}

// AtomicAdd exposes atomic add for custom SAB indices (flags region only).
func (sb *SABBridge) AtomicAdd(index uint32, delta uint32) uint32 {
	return sb.atomicAdd(index, delta)
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

	// Structured parameters (using custom field for now if not mapped)
	params, _ := req.NewParams()
	custom, _ := params.NewCustomParams()

	// Check for known parameters in the map
	if pVal, ok := job.Parameters["shader_source"]; ok {
		if shader, ok := pVal.(string); ok {
			_ = custom.SetShaderSource(shader)
		}
	} else if pVal, ok := job.Parameters["params"]; ok {
		if pStr, ok := pVal.(string); ok {
			_ = custom.SetShaderSource(pStr)
		}
	}

	req.SetInput(job.Data)
	return msg.Marshal()
}

func (sb *SABBridge) serializeResult(result *foundation.Result) ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	res, err := compute.NewRootCompute_JobResult(seg)
	if err != nil {
		return nil, err
	}
	if err := res.SetJobId(result.JobID); err != nil {
		return nil, err
	}

	if result.Success {
		res.SetStatus(compute.Compute_Status_success)
	} else {
		res.SetStatus(compute.Compute_Status_failed)
	}

	if len(result.Data) > 0 {
		_ = res.SetOutput(result.Data)
	}
	if result.Error != "" {
		_ = res.SetErrorMessage(result.Error)
	}
	if result.Latency > 0 {
		res.SetExecutionTimeNs(uint64(result.Latency.Nanoseconds()))
	}

	return msg.Marshal()
}

func (sb *SABBridge) WriteResult(result *foundation.Result) error {
	data, err := sb.serializeResult(result)
	if err != nil {
		return err
	}
	sb.mu.Lock()
	defer sb.mu.Unlock()
	err = sb.writeToSAB(sb.outboxHostOffset, sab_layout.SIZE_OUTBOX_HOST_TOTAL, data)
	if err == nil {
		sb.SignalEpoch(sab_layout.IDX_OUTBOX_HOST_DIRTY)
	}
	return err
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

// GetFrameLatency returns the recently measured physics frame latency
func (sb *SABBridge) GetFrameLatency() time.Duration {
	return sb.frameLatency
}
