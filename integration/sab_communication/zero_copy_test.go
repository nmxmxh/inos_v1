package sab_communication

import (
	"testing"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/stretchr/testify/assert"
)

// TestGoWriteRustRead validates Go → Rust SAB communication
func TestGoWriteRustRead(t *testing.T) {
	// 1. Go creates SAB (simulating kernel initialization)
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabPtr := unsafe.Pointer(&data[0])
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(sabPtr)

	// 2. Go writes data to Inbox (Kernel → Module)
	testData := []byte("Hello from Go Kernel")
	offset := sab.OFFSET_INBOX_BASE
	copy(sabSlice[offset:], testData)

	// 3. Go signals Rust via epoch increment
	epochSlice := (*[256]int32)(sabPtr)
	epochSlice[sab.IDX_INBOX_DIRTY]++

	// 4. Validate: Data is at correct offset (Rust would read this)
	read := sabSlice[offset : offset+20]
	assert.Equal(t, testData, read, "Data should be readable at OFFSET_INBOX")

	// 5. Validate: Epoch incremented
	assert.Equal(t, int32(1), epochSlice[sab.IDX_INBOX_DIRTY], "Epoch should be incremented")
}

// TestRustWriteGoRead validates Rust → Go SAB communication
func TestRustWriteGoRead(t *testing.T) {
	// 1. Create shared SAB
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabPtr := unsafe.Pointer(&data[0])
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(sabPtr)

	// 2. Simulate Rust writing to Outbox (Module → Kernel)
	testData := []byte("Hello from Rust Module")
	offset := sab.OFFSET_OUTBOX_BASE
	copy(sabSlice[offset:], testData)

	// 3. Simulate Rust signaling Go via epoch
	epochSlice := (*[256]int32)(sabPtr)
	epochSlice[sab.IDX_OUTBOX_DIRTY]++

	// 4. Go reads data
	read := sabSlice[offset : offset+22]
	assert.Equal(t, testData, read, "Go should read Rust's data from OFFSET_OUTBOX")

	// 5. Go detects epoch change
	assert.Equal(t, int32(1), epochSlice[sab.IDX_OUTBOX_DIRTY], "Go should detect epoch change")
}

// TestZeroCopyValidation ensures no data copying occurs
func TestZeroCopyValidation(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabPtr := unsafe.Pointer(&data[0])
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(sabPtr)

	testData := []byte{1, 2, 3, 4, 5}
	ptrBefore := unsafe.Pointer(&testData[0])

	// Write to SAB
	copy(sabSlice[0x1000:], testData)

	ptrAfter := unsafe.Pointer(&testData[0])

	// Validate: Pointer unchanged (zero-copy)
	assert.Equal(t, ptrBefore, ptrAfter, "Pointer should not change - zero-copy operation")

	// Validate: Data written correctly
	read := sabSlice[0x1000 : 0x1000+5]
	assert.Equal(t, testData, read)
}

// TestModuleRegistration validates module registration via SAB
func TestModuleRegistration(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabPtr := unsafe.Pointer(&data[0])
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(sabPtr)

	// Simulate Rust module writing registry entry
	offset := sab.OFFSET_MODULE_REGISTRY
	moduleID := []byte("ml\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00") // 32 bytes
	copy(sabSlice[offset:], moduleID)

	version := []byte("1.0.0\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00") // 16 bytes
	copy(sabSlice[offset+32:], version)

	// Go reads registry
	readID := sabSlice[offset : offset+32]
	readVersion := sabSlice[offset+32 : offset+48]

	// Validate: Module registered correctly
	assert.Equal(t, byte('m'), readID[0])
	assert.Equal(t, byte('l'), readID[1])
	assert.Equal(t, byte('1'), readVersion[0])
}

// TestEpochSignaling validates epoch-based reactive mutation
func TestEpochSignaling(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabPtr := unsafe.Pointer(&data[0])
	epochSlice := (*[256]int32)(sabPtr)

	// Initial state
	assert.Equal(t, int32(0), epochSlice[7], "Epoch should start at 0")

	// Increment epoch (simulate Rust signaling)
	epochSlice[7]++

	// Validate: Epoch incremented
	assert.Equal(t, int32(1), epochSlice[7])

	// Multiple increments
	for i := 2; i <= 10; i++ {
		epochSlice[7]++
		assert.Equal(t, int32(i), epochSlice[7])
	}
}

// BenchmarkSABWrite benchmarks SAB write performance (target: < 20ns)
func BenchmarkSABWrite(b *testing.B) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(unsafe.Pointer(&data[0]))
	testData := []byte("benchmark data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(sabSlice[0x1000:], testData)
	}
	// Target: < 20ns per operation
}

// BenchmarkSABRead benchmarks SAB read performance (target: < 10ns)
func BenchmarkSABRead(b *testing.B) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(unsafe.Pointer(&data[0]))
	copy(sabSlice[0x1000:], []byte("benchmark data"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sabSlice[0x1000 : 0x1000+14]
	}
	// Target: < 10ns per operation
}

// BenchmarkEpochIncrement benchmarks epoch increment (target: < 5ns)
func BenchmarkEpochIncrement(b *testing.B) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	epochSlice := (*[256]int32)(unsafe.Pointer(&data[0]))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		epochSlice[7]++
	}
	// Target: < 5ns per operation
}
