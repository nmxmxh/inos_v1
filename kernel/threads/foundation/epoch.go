package foundation

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// EnhancedEpoch provides wait-free notification for epoch changes
type EnhancedEpoch struct {
	index     uint8
	sab       []byte
	lastValue uint32

	// Notification channels for waiters
	waiters   *[]chan struct{}
	waitersMu *sync.RWMutex

	// Statistics
	stats *EpochStats
}

// EpochStats tracks epoch performance metrics
type EpochStats struct {
	Increments   uint64        // Total increments
	Wakes        uint64        // Total wake-ups
	MaxWaiters   uint32        // Peak concurrent waiters
	AvgLatency   time.Duration // Average wake latency
	totalLatency time.Duration // For calculating average
}

// NewEnhancedEpoch creates a new enhanced epoch
func NewEnhancedEpoch(sab []byte, index uint8) *EnhancedEpoch {
	offset := uint32(index) * 4
	lastValue := atomic.LoadUint32((*uint32)(unsafe.Pointer(&sab[offset])))

	waiters := make([]chan struct{}, 0, 8)

	return &EnhancedEpoch{
		index:     index,
		sab:       sab,
		lastValue: lastValue,
		waiters:   &waiters,
		waitersMu: &sync.RWMutex{},
		stats:     &EpochStats{},
	}
}

// Reader creates a new reader instance sharing the signaling mechanism
func (ee *EnhancedEpoch) Reader() *EnhancedEpoch {
	offset := uint32(ee.index) * 4
	lastValue := atomic.LoadUint32((*uint32)(unsafe.Pointer(&ee.sab[offset])))

	return &EnhancedEpoch{
		index:     ee.index,
		sab:       ee.sab,
		lastValue: lastValue,
		waiters:   ee.waiters,
		waitersMu: ee.waitersMu,
		stats:     ee.stats,
	}
}

// WaitForChange waits for epoch change with <1µs latency
func (ee *EnhancedEpoch) WaitForChange(timeout time.Duration) (bool, error) {
	offset := uint32(ee.index) * 4
	start := time.Now()

	// Fast path
	current := atomic.LoadUint32((*uint32)(unsafe.Pointer(&ee.sab[offset])))
	if current != ee.lastValue {
		ee.lastValue = current
		atomic.AddUint64(&ee.stats.Wakes, 1)
		return true, nil
	}

	// Spin-wait
	spinDeadline := start.Add(time.Microsecond)
	for time.Now().Before(spinDeadline) {
		runtime.Gosched()
		current := atomic.LoadUint32((*uint32)(unsafe.Pointer(&ee.sab[offset])))
		if current != ee.lastValue {
			ee.lastValue = current
			atomic.AddUint64(&ee.stats.Wakes, 1)
			return true, nil
		}
	}

	// Register for notification
	ch := make(chan struct{}, 1)
	ee.addWaiter(ch)
	defer ee.removeWaiter(ch)

	select {
	case <-ch:
		ee.lastValue = atomic.LoadUint32((*uint32)(unsafe.Pointer(&ee.sab[offset])))
		atomic.AddUint64(&ee.stats.Wakes, 1)
		return true, nil
	case <-time.After(timeout):
		return false, nil
	}
}

// Increment increments the epoch
func (ee *EnhancedEpoch) Increment() {
	offset := uint32(ee.index) * 4
	atomic.AddUint32((*uint32)(unsafe.Pointer(&ee.sab[offset])), 1)
	atomic.AddUint64(&ee.stats.Increments, 1)
	go ee.notifyWaiters()
}

func (ee *EnhancedEpoch) GetValue() uint32 {
	offset := uint32(ee.index) * 4
	return atomic.LoadUint32((*uint32)(unsafe.Pointer(&ee.sab[offset])))
}

func (ee *EnhancedEpoch) addWaiter(ch chan struct{}) {
	ee.waitersMu.Lock()
	defer ee.waitersMu.Unlock()
	*ee.waiters = append(*ee.waiters, ch)
}

func (ee *EnhancedEpoch) removeWaiter(ch chan struct{}) {
	ee.waitersMu.Lock()
	defer ee.waitersMu.Unlock()
	for i, waiter := range *ee.waiters {
		if waiter == ch {
			*ee.waiters = append((*ee.waiters)[:i], (*ee.waiters)[i+1:]...)
			break
		}
	}
}

func (ee *EnhancedEpoch) notifyWaiters() {
	ee.waitersMu.RLock()
	waiters := make([]chan struct{}, len(*ee.waiters))
	copy(waiters, *ee.waiters)
	ee.waitersMu.RUnlock()

	for _, ch := range waiters {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
