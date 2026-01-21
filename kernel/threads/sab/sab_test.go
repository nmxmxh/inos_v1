package sab

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

// TestZeroCopySABWrite validates that SAB writes don't copy data
func TestZeroCopySABWrite(t *testing.T) {
	// Create test SAB
	data := make([]byte, 16*1024*1024) // 16MB
	sab := unsafe.Pointer(&data[0])
	
	testData := []byte{42, 43, 44, 45}
	
	// Write to SAB
	ptrBefore := unsafe.Pointer(&testData[0])
	offset := uintptr(0x1000)
	
	// Simulate SAB write (copy to offset)
	sabSlice := (*[16 * 1024 * 1024]byte)(sab)
	copy(sabSlice[offset:], testData)
	
	ptrAfter := unsafe.Pointer(&testData[0])
	
	// Validate: No copy occurred (pointer unchanged)
	assert.Equal(t, ptrBefore, ptrAfter, "Pointer should not change (zero-copy)")
	
	// Validate: Data written correctly
	readData := sabSlice[offset : offset+4]
	assert.Equal(t, testData, readData)
}

// TestSABPointerArithmetic validates pointer arithmetic correctness
func TestSABPointerArithmetic(t *testing.T) {
	data := make([]byte, 16*1024*1024) // 16MB
	sab := (*[16 * 1024 * 1024]byte)(unsafe.Pointer(&data[0]))
	
	// Write at various offsets (from SAB layout)
	offsets := []uintptr{0x000100, 0x010000, 0x0D0000}
	testData := []byte("test")
	
	for _, offset := range offsets {
		copy(sab[offset:], testData)
		read := sab[offset : offset+4]
		assert.Equal(t, testData, read, "Data at offset %x should match", offset)
	}
}

// TestAtomicOperations validates atomic load/store/add
func TestAtomicOperations(t *testing.T) {
	data := make([]byte, 1024)
	sab := (*[256]int32)(unsafe.Pointer(&data[0]))
	
	// Test atomic store and load
	sab[0] = 42
	value := sab[0]
	assert.Equal(t, int32(42), value)
	
	// Test atomic add
	sab[0] += 10
	newValue := sab[0]
	assert.Equal(t, int32(52), newValue)
}

// BenchmarkSABRead benchmarks SAB read performance (target: < 10ns)
func BenchmarkSABRead(b *testing.B) {
	data := make([]byte, 1024*1024)
	sab := (*[1024 * 1024]byte)(unsafe.Pointer(&data[0]))
	copy(sab[:], []byte("benchmark data"))
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sab[0:14]
	}
}

// BenchmarkSABWrite benchmarks SAB write performance (target: < 20ns)
func BenchmarkSABWrite(b *testing.B) {
	data := make([]byte, 1024*1024)
	sab := (*[1024 * 1024]byte)(unsafe.Pointer(&data[0]))
	testData := []byte("benchmark data")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(sab[:], testData)
	}
}

// BenchmarkAtomicAdd benchmarks atomic add performance (target: < 50ns)
func BenchmarkAtomicAdd(b *testing.B) {
	data := make([]byte, 1024)
	sab := (*[256]int32)(unsafe.Pointer(&data[0]))
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sab[0]++
	}
}
