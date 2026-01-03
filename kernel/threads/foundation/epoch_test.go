package foundation

import (
	"encoding/binary"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnhancedEpoch_IncrementAndGet(t *testing.T) {
	sab := make([]byte, 1024)
	epoch := NewEnhancedEpoch(sab, 0)

	assert.Equal(t, uint32(0), epoch.GetValue())

	epoch.Increment()
	assert.Equal(t, uint32(1), epoch.GetValue())
	assert.Equal(t, uint64(1), epoch.stats.Increments)

	epoch.Increment()
	assert.Equal(t, uint32(2), epoch.GetValue())
	assert.Equal(t, uint64(2), epoch.stats.Increments)
}

func TestEnhancedEpoch_WaitForChange_FastPath(t *testing.T) {
	sab := make([]byte, 1024)
	epoch := NewEnhancedEpoch(sab, 0)

	// Simulate external update
	epoch.Increment()
	// epoch.lastValue is 0, current is 1 in SAB
	// WaitForChange should return immediately

	// Reset lastValue to 0 to simulate "we haven't seen the update yet"
	// But NewEnhancedEpoch reads current value.
	// So we need to call Increment from another goroutine or cheat.

	// Actually NewEnhancedEpoch reads initial value.
	// If we increment, SAB becomes 1. lastValue is 0?
	// No, Increment updates SAB.

	// Wait, EnhancedEpoch keeps `lastValue` locally.
	// `NewEnhancedEpoch` sets `extract lastValue` from SAB.

	// If I call Increment(), it updates SAB AND stats. It does NOT update `lastValue` of the struct?
	// Let's check implementation.
	// func (ee *EnhancedEpoch) Increment() { ... atomic.AddUint32(...) ... }
	// It does NOT update `lastValue`. `WaitForChange` updates `lastValue`.

	// So:
	// epoch.Increment() removed
	t.Logf("After Increment: SAB[0]=%d, lastValue=%d", binary.LittleEndian.Uint32(sab[0:4]), epoch.lastValue)
	// SAB is 1. epoch.lastValue is 0.

	changed, err := epoch.WaitForChange(time.Second)
	t.Logf("After WaitForChange: SAB[0]=%d, lastValue=%d, changed=%v", binary.LittleEndian.Uint32(sab[0:4]), epoch.lastValue, changed)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, uint32(1), epoch.lastValue)
}

func TestEnhancedEpoch_WaitForChange_Timeout(t *testing.T) {
	sab := make([]byte, 1024)
	epoch := NewEnhancedEpoch(sab, 0)

	changed, err := epoch.WaitForChange(10 * time.Millisecond)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestEnhancedEpoch_WaitForChange_SlowPath(t *testing.T) {
	sab := make([]byte, 1024)
	epoch := NewEnhancedEpoch(sab, 0)

	start := time.Now()
	go func() {
		time.Sleep(50 * time.Millisecond)
		epoch.Increment()
	}()

	changed, err := epoch.WaitForChange(1 * time.Second)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.WithinDuration(t, start.Add(50*time.Millisecond), time.Now(), 20*time.Millisecond)
}

func TestEnhancedEpoch_ConcurrentWaiters(t *testing.T) {
	sab := make([]byte, 1024)
	epoch := NewEnhancedEpoch(sab, 0)

	const triggers = 10
	const waiters = 5

	var wg sync.WaitGroup
	wg.Add(waiters)

	for i := 0; i < waiters; i++ {
		go func() {
			defer wg.Done()
			// Create local reader for thread-safety (stateful cursor sharing signal mechanism)
			localEpoch := epoch.Reader()
			for {
				changed, err := localEpoch.WaitForChange(1 * time.Second)
				assert.NoError(t, err)
				assert.True(t, changed)
				if localEpoch.GetValue() >= uint32(triggers) {
					return
				}
			}
		}()
	}

	for i := 0; i < triggers; i++ {
		time.Sleep(10 * time.Millisecond)
		epoch.Increment()
	}

	wg.Wait()
}
