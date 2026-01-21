package foundation

import (
	"encoding/binary"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"unsafe"

	"github.com/stretchr/testify/assert"
)

// TestEpoch_Persistence verifies that epoch state is preserved in SAB
// and can be recovered by a new EnhancedEpoch instance shared on the same memory.
func TestEpoch_Persistence(t *testing.T) {
	sabSize := uint32(1024)
	sab := make([]byte, sabSize)

	// Phase 1: Initialize and Increment
	{
		epoch1 := NewEnhancedEpoch(unsafe.Pointer(&sab[0]), sabSize, 0)
		epoch1.Increment()
		epoch1.Increment()
		assert.Equal(t, uint32(2), epoch1.GetValue())
	}

	// Verify raw SAB (offset 0 -> 4 bytes)
	val := binary.LittleEndian.Uint32(sab[0:4])
	assert.Equal(t, uint32(2), val)

	// Phase 2: "Reboot" - New Instance
	{
		epoch2 := NewEnhancedEpoch(unsafe.Pointer(&sab[0]), sabSize, 0)
		// Should read existing value 2
		assert.Equal(t, uint32(2), epoch2.GetValue())
		assert.Equal(t, uint32(2), epoch2.lastValue) // Should init lastValue from SAB

		// Increment
		epoch2.Increment()
		assert.Equal(t, uint32(3), epoch2.GetValue())
	}

	// Verify raw SAB again
	val = binary.LittleEndian.Uint32(sab[0:4])
	assert.Equal(t, uint32(3), val)
}

// TestEpoch_HighContention stresses the epoch with concurrent readers and writers
func TestEpoch_HighContention(t *testing.T) {
	sabSize := uint32(1024)
	sab := make([]byte, sabSize)
	epoch := NewEnhancedEpoch(unsafe.Pointer(&sab[0]), sabSize, 0)

	// Configuration
	writers := 5
	readers := 20
	iterations := 1000

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Tracking
	var totalIncrements int64
	var totalWakes int64

	// Coordinate writers completion
	var writersWg sync.WaitGroup
	writersWg.Add(writers)

	// Channel to signal readers to stop
	writersDone := make(chan struct{})

	// Writers
	for i := 0; i < writers; i++ {
		wg.Add(1) // Add to main waitgroup for writers too
		go func() {
			defer wg.Done()
			defer writersWg.Done()
			<-start
			for j := 0; j < iterations; j++ {
				epoch.Increment()
				atomic.AddInt64(&totalIncrements, 1)
				// Small sleep to allow readers to interleave
				time.Sleep(time.Microsecond)
			}
		}()
	}

	// Monitor writers and close done channel
	go func() {
		writersWg.Wait()
		close(writersDone)
	}()

	// Readers
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			localEpoch := NewEnhancedEpoch(unsafe.Pointer(&sab[0]), sabSize, 0) // Simulate separate components reading same SAB

			for {
				select {
				case <-writersDone:
					return
				default:
					changed, _ := localEpoch.WaitForChange(10 * time.Millisecond)
					if changed {
						atomic.AddInt64(&totalWakes, 1)
					}
				}
			}
		}()
	}

	close(start)
	wg.Wait()

	finalVal := epoch.GetValue()
	assert.Equal(t, uint32(totalIncrements), finalVal)
	t.Logf("Total Increments: %d, Total Wakes: %d, Final Value: %d", totalIncrements, totalWakes, finalVal)

	assert.Greater(t, totalWakes, int64(0), "Readers should have woken up")
}
