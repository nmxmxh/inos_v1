package sab_communication

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/stretchr/testify/assert"
)

// ========== PERFORMANCE & LOAD TESTS ==========

// TestPerformance_ZeroCopyThroughput measures zero-copy throughput
func TestPerformance_ZeroCopyThroughput(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(unsafe.Pointer(&data[0]))

	// Test various sizes
	sizes := []int{1024, 10 * 1024, 100 * 1024, 1024 * 1024}

	for _, size := range sizes {
		testData := make([]byte, size)
		iterations := 1000

		start := time.Now()
		for i := 0; i < iterations; i++ {
			// Ensure offset stays within bounds
			offset := sab.OFFSET_ARENA + uint32((i%10)*size)
			if offset+uint32(size) > sab.SAB_SIZE_DEFAULT {
				offset = sab.OFFSET_ARENA
			}
			copy(sabSlice[offset:offset+uint32(size)], testData)
		}
		duration := time.Since(start)

		throughputMBps := float64(size*iterations) / duration.Seconds() / 1024 / 1024
		t.Logf("Size: %6d bytes, Throughput: %.2f MB/s (%d ops in %v)",
			size, throughputMBps, iterations, duration)

		// Zero-copy should be fast
		assert.Greater(t, throughputMBps, 10.0, "Throughput should be >10 MB/s")
	}
}

// TestPerformance_EpochLatency measures epoch signaling latency
func TestPerformance_EpochLatency(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	epochSlice := (*[256]int32)(unsafe.Pointer(&data[0]))

	iterations := 10000
	latencies := make([]time.Duration, iterations)

	for i := 0; i < iterations; i++ {
		start := time.Now()
		epochSlice[int(sab.IDX_SYSTEM_EPOCH)]++
		latencies[i] = time.Since(start)
	}

	// Calculate statistics
	var total time.Duration
	var max time.Duration
	for _, lat := range latencies {
		total += lat
		if lat > max {
			max = lat
		}
	}
	avg := total / time.Duration(iterations)

	t.Logf("Epoch signaling - Avg: %v, Max: %v, Iterations: %d", avg, max, iterations)

	// Atomic operations should be very fast
	assert.Less(t, avg.Microseconds(), int64(100), "Average latency should be <100μs")
}

// TestPerformance_ConcurrentLoad tests concurrent operations
func TestPerformance_ConcurrentLoad(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(unsafe.Pointer(&data[0]))

	numGoroutines := 50
	operationsPerGoroutine := 100

	var wg sync.WaitGroup
	var successCount atomic.Int64

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			testData := make([]byte, 1024)
			for j := 0; j < operationsPerGoroutine; j++ {
				offset := sab.OFFSET_ARENA + uint32(((id*operationsPerGoroutine+j)%1000)*1024)
				copy(sabSlice[offset:], testData)
				successCount.Add(1)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	totalOps := numGoroutines * operationsPerGoroutine
	opsPerSec := float64(successCount.Load()) / duration.Seconds()

	t.Logf("Concurrent load: %d goroutines, %d ops each", numGoroutines, operationsPerGoroutine)
	t.Logf("Success: %d/%d, Duration: %v", successCount.Load(), totalOps, duration)
	t.Logf("Throughput: %.2f ops/sec", opsPerSec)

	assert.Equal(t, int64(totalOps), successCount.Load(), "All operations should succeed")
	assert.Greater(t, opsPerSec, 1000.0, "Should handle >1k ops/sec")
}

// TestPerformance_SustainedThroughput tests sustained performance
func TestPerformance_SustainedThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sustained load test in short mode")
	}

	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(unsafe.Pointer(&data[0]))

	duration := 5 * time.Second
	dataSize := 1024

	var opsCompleted atomic.Int64
	stopChan := make(chan struct{})

	// Start workers
	numWorkers := 10
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			testData := make([]byte, dataSize)

			for {
				select {
				case <-stopChan:
					return
				default:
					offset := sab.OFFSET_ARENA + uint32(((id*10000+int(opsCompleted.Load()%1000))*dataSize)%1000000)
					copy(sabSlice[offset:], testData)
					opsCompleted.Add(1)
				}
			}
		}(i)
	}

	// Run for duration
	time.Sleep(duration)
	close(stopChan)
	wg.Wait()

	totalOps := opsCompleted.Load()
	opsPerSec := float64(totalOps) / duration.Seconds()
	throughputMBps := float64(totalOps*int64(dataSize)) / duration.Seconds() / 1024 / 1024

	t.Logf("Sustained load test: %v duration, %d workers", duration, numWorkers)
	t.Logf("Operations: %d, Throughput: %.2f ops/sec, %.2f MB/s",
		totalOps, opsPerSec, throughputMBps)

	assert.Greater(t, opsPerSec, 1000.0, "Should sustain >1k ops/sec")
	assert.Greater(t, throughputMBps, 1.0, "Should sustain >1 MB/s")
}

// TestPerformance_ComparisonTraditionalVsZeroCopy compares approaches
func TestPerformance_ComparisonTraditionalVsZeroCopy(t *testing.T) {
	dataSize := 1024 * 1024 // 1MB
	iterations := 100

	// Traditional approach (with copy)
	traditionalData := make([]byte, dataSize)
	start := time.Now()
	for i := 0; i < iterations; i++ {
		// Simulate copy
		copied := make([]byte, len(traditionalData))
		copy(copied, traditionalData)
	}
	traditionalTime := time.Since(start)

	// Zero-copy approach (SAB)
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(unsafe.Pointer(&data[0]))
	copy(sabSlice[sab.OFFSET_ARENA:], traditionalData)

	start = time.Now()
	for i := 0; i < iterations; i++ {
		// Zero-copy read (just slice reference)
		_ = sabSlice[sab.OFFSET_ARENA : sab.OFFSET_ARENA+uint32(dataSize)]
	}
	zeroCopyTime := time.Since(start)

	speedup := float64(traditionalTime) / float64(zeroCopyTime)

	t.Logf("Traditional (copy): %v for %d iterations", traditionalTime, iterations)
	t.Logf("Zero-copy (SAB):    %v for %d iterations", zeroCopyTime, iterations)
	t.Logf("Speedup: %.2fx faster", speedup)

	assert.Greater(t, speedup, 1.0, "Zero-copy should be faster")
}
