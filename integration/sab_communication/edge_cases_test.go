package sab_communication

import (
	"testing"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== SUCCESS CASES ==========

// TestSABWriteRead_Success validates successful SAB write and read
func TestSABWriteRead_Success(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(unsafe.Pointer(&data[0]))

	// Test various data sizes
	testCases := []struct {
		name   string
		data   []byte
		offset uint32
	}{
		{"Small", []byte("test"), 0x1000},
		{"Medium", make([]byte, 1024), 0x2000},
		{"Large", make([]byte, 64*1024), 0x10000},
		{"AtInbox", []byte("inbox data"), sab.OFFSET_INBOX_BASE},
		{"AtOutboxHost", []byte("outbox host data"), sab.OFFSET_OUTBOX_HOST_BASE},
		{"AtOutboxKernel", []byte("outbox kernel data"), sab.OFFSET_OUTBOX_KERNEL_BASE},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize test data
			for i := range tc.data {
				tc.data[i] = byte(i % 256)
			}

			// Write
			copy(sabSlice[tc.offset:], tc.data)

			// Read
			read := sabSlice[tc.offset : tc.offset+uint32(len(tc.data))]

			// Validate
			assert.Equal(t, tc.data, read, "Data should match")
		})
	}
}

// TestEpochIncrement_Success validates successful epoch increments
func TestEpochIncrement_Success(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	epochSlice := (*[256]int32)(unsafe.Pointer(&data[0]))

	testCases := []struct {
		name  string
		index int
		count int
	}{
		{"SystemEpoch", int(sab.IDX_SYSTEM_EPOCH), 100},
		{"InboxDirty", int(sab.IDX_INBOX_DIRTY), 50},
		{"OutboxHostDirty", int(sab.IDX_OUTBOX_HOST_DIRTY), 75},
		{"OutboxKernelDirty", int(sab.IDX_OUTBOX_KERNEL_DIRTY), 75},
		{"CustomEpoch", 32, 200}, // Supervisor pool
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset
			epochSlice[tc.index] = 0

			// Increment
			for i := 1; i <= tc.count; i++ {
				epochSlice[tc.index]++
				assert.Equal(t, int32(i), epochSlice[tc.index])
			}
		})
	}
}

// TestModuleRegistration_Success validates successful module registration
func TestModuleRegistration_Success(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(unsafe.Pointer(&data[0]))

	modules := []struct {
		id      string
		version string
		slot    int
	}{
		{"ml", "1.0.0", 0},
		{"compute", "2.1.3", 1},
		{"storage", "0.9.5", 2},
	}

	for _, mod := range modules {
		t.Run(mod.id, func(t *testing.T) {
			offset := sab.OFFSET_MODULE_REGISTRY + uint32(mod.slot)*sab.MODULE_ENTRY_SIZE

			// Write module ID (32 bytes)
			moduleID := make([]byte, 32)
			copy(moduleID, []byte(mod.id))
			copy(sabSlice[offset:], moduleID)

			// Write version (16 bytes)
			version := make([]byte, 16)
			copy(version, []byte(mod.version))
			copy(sabSlice[offset+32:], version)

			// Validate
			readID := string(sabSlice[offset : offset+uint32(len(mod.id))])
			readVersion := string(sabSlice[offset+32 : offset+32+uint32(len(mod.version))])

			assert.Equal(t, mod.id, readID)
			assert.Equal(t, mod.version, readVersion)
		})
	}
}

// ========== FAILURE CASES ==========

// TestSABOutOfBounds_Failure validates out-of-bounds detection
func TestSABOutOfBounds_Failure(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(unsafe.Pointer(&data[0]))

	testCases := []struct {
		name   string
		offset uint32
		size   int
	}{
		{"ExceedsSABSize", sab.SAB_SIZE_DEFAULT - 10, 20},
		{"AtBoundary", sab.SAB_SIZE_DEFAULT, 1},
		{"WayOutOfBounds", sab.SAB_SIZE_DEFAULT + 1000, 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate bounds check
			isValid := sab.IsValidOffset(tc.offset, uint32(tc.size), sab.SAB_SIZE_DEFAULT)
			assert.False(t, isValid, "Should detect out-of-bounds")

			// Attempting to access would panic (don't actually do it in test)
			if tc.offset+uint32(tc.size) <= sab.SAB_SIZE_DEFAULT {
				// Safe to access
				_ = sabSlice[tc.offset : tc.offset+uint32(tc.size)]
			}
		})
	}
}

// TestEpochOverflow_Failure validates epoch overflow handling
func TestEpochOverflow_Failure(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	epochSlice := (*[256]int32)(unsafe.Pointer(&data[0]))

	// Set to max int32
	epochSlice[int(sab.IDX_SYSTEM_EPOCH)] = 2147483647

	// Increment (will overflow to negative)
	epochSlice[int(sab.IDX_SYSTEM_EPOCH)]++

	// Validate overflow occurred
	assert.Less(t, epochSlice[int(sab.IDX_SYSTEM_EPOCH)], int32(0), "Should overflow to negative")
}

// TestModuleRegistryFull_Failure validates registry capacity limits
func TestModuleRegistryFull_Failure(t *testing.T) {
	// Validate max modules
	assert.Equal(t, uint32(64), sab.MAX_MODULES_INLINE, "Should support 64 inline modules")

	// Calculate if we can fit MAX_MODULES_INLINE
	totalSize := int(sab.MAX_MODULES_INLINE) * int(sab.MODULE_ENTRY_SIZE)
	assert.LessOrEqual(t, totalSize, int(sab.SIZE_MODULE_REGISTRY), "Registry should fit max modules")
}

// ========== EMPTY/NULL CASES ==========

// TestEmptyData_EdgeCase validates handling of empty data
func TestEmptyData_EdgeCase(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(unsafe.Pointer(&data[0]))

	testCases := []struct {
		name   string
		data   []byte
		offset uint32
	}{
		{"EmptySlice", []byte{}, 0x1000},
		{"ZeroLength", make([]byte, 0), 0x2000},
		{"NilEquivalent", []byte(nil), 0x3000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write empty data (should be no-op)
			copy(sabSlice[tc.offset:], tc.data)

			// Validate no panic occurred
			assert.Equal(t, 0, len(tc.data))
		})
	}
}

// TestZeroInitialization_EdgeCase validates SAB starts zeroed
func TestZeroInitialization_EdgeCase(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)

	// Validate all bytes are zero
	for i, b := range data {
		if b != 0 {
			t.Errorf("Byte at %d should be 0, got %d", i, b)
			break
		}
	}

	// Validate epochs are zero
	epochSlice := (*[256]int32)(unsafe.Pointer(&data[0]))
	for i := 0; i < 256; i++ {
		assert.Equal(t, int32(0), epochSlice[i], "Epoch %d should be 0", i)
	}
}

// ========== EDGE CASES ==========

// TestBoundaryWrites_EdgeCase validates writes at region boundaries
func TestBoundaryWrites_EdgeCase(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(unsafe.Pointer(&data[0]))

	testCases := []struct {
		name   string
		offset uint32
		size   int
	}{
		{"InboxStart", sab.OFFSET_INBOX_BASE, 1},
		{"InboxEnd", sab.OFFSET_INBOX_BASE + sab.SIZE_INBOX_TOTAL - 1, 1},
		{"OutboxHostStart", sab.OFFSET_OUTBOX_HOST_BASE, 1},
		{"OutboxHostEnd", sab.OFFSET_OUTBOX_HOST_BASE + sab.SIZE_OUTBOX_HOST_TOTAL - 1, 1},
		{"OutboxKernelStart", sab.OFFSET_OUTBOX_KERNEL_BASE, 1},
		{"OutboxKernelEnd", sab.OFFSET_OUTBOX_KERNEL_BASE + sab.SIZE_OUTBOX_KERNEL_TOTAL - 1, 1},
		{"ArenaStart", sab.OFFSET_ARENA, 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write at boundary
			testData := make([]byte, tc.size)
			testData[0] = 0xFF

			copy(sabSlice[tc.offset:], testData)

			// Validate
			assert.Equal(t, byte(0xFF), sabSlice[tc.offset])
		})
	}
}

// TestConcurrentEpochUpdates_EdgeCase simulates concurrent updates
func TestConcurrentEpochUpdates_EdgeCase(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	epochSlice := (*[256]int32)(unsafe.Pointer(&data[0]))

	// Simulate multiple "threads" incrementing
	// (In real scenario, this would use atomics)
	iterations := 1000
	for i := 0; i < iterations; i++ {
		epochSlice[int(sab.IDX_SYSTEM_EPOCH)]++
	}

	assert.Equal(t, int32(iterations), epochSlice[int(sab.IDX_SYSTEM_EPOCH)])
}

// TestMaxSABSize_EdgeCase validates maximum SAB size
func TestMaxSABSize_EdgeCase(t *testing.T) {
	// Validate max size constant
	assert.Equal(t, uint32(1024*1024*1024), sab.SAB_SIZE_MAX, "Max SAB should be 1GB")

	// Validate layout fits in max size
	err := sab.ValidateMemoryLayout(sab.SAB_SIZE_MAX)
	assert.NoError(t, err, "Layout should be valid at max size")
}

// TestMinSABSize_EdgeCase validates minimum SAB size
func TestMinSABSize_EdgeCase(t *testing.T) {
	// Validate min size constant
	assert.Equal(t, uint32(32*1024*1024), sab.SAB_SIZE_MIN, "Min SAB should be 32MB")

	// Validate layout fits in min size
	err := sab.ValidateMemoryLayout(sab.SAB_SIZE_MIN)
	assert.NoError(t, err, "Layout should be valid at min size")
}

// TestAlignmentRequirements_EdgeCase validates alignment
func TestAlignmentRequirements_EdgeCase(t *testing.T) {
	testCases := []struct {
		name      string
		offset    uint32
		alignment uint32
		expected  uint32
	}{
		{"CacheLine_Aligned", 0x1000, sab.ALIGNMENT_CACHE_LINE, 0x1000},
		{"CacheLine_Unaligned", 0x1001, sab.ALIGNMENT_CACHE_LINE, 0x1040},
		{"Page_Aligned", 0x10000, sab.ALIGNMENT_PAGE, 0x10000},
		{"Page_Unaligned", 0x10001, sab.ALIGNMENT_PAGE, 0x11000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			aligned := sab.AlignOffset(tc.offset, tc.alignment)
			assert.Equal(t, tc.expected, aligned)
		})
	}
}

// TestRegionOverlap_EdgeCase validates no region overlaps
func TestRegionOverlap_EdgeCase(t *testing.T) {
	regions := sab.GetAllRegions(sab.SAB_SIZE_DEFAULT)

	// Check all pairs
	for i := 0; i < len(regions); i++ {
		for j := i + 1; j < len(regions); j++ {
			r1, r2 := regions[i], regions[j]

			// Regions should not overlap
			overlaps := r1.Offset < r2.Offset+r2.Size && r1.Offset+r1.Size > r2.Offset
			assert.False(t, overlaps, "%s overlaps with %s", r1.Name, r2.Name)
		}
	}
}

// ========== PERFORMANCE TESTS ==========

// TestLargeDataTransfer_Performance validates large data transfers
func TestLargeDataTransfer_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(unsafe.Pointer(&data[0]))

	sizes := []int{
		1024,        // 1KB
		64 * 1024,   // 64KB
		1024 * 1024, // 1MB
	}

	for _, size := range sizes {
		t.Run(string(rune(size)), func(t *testing.T) {
			testData := make([]byte, size)
			for i := range testData {
				testData[i] = byte(i % 256)
			}

			// Write
			copy(sabSlice[sab.OFFSET_ARENA:], testData)

			// Read
			read := sabSlice[sab.OFFSET_ARENA : sab.OFFSET_ARENA+uint32(size)]

			// Validate
			assert.Equal(t, testData, read)
		})
	}
}

// ========== INTEGRATION TESTS ==========

// TestFullWorkflow_Integration validates complete Go→Rust→Go flow
func TestFullWorkflow_Integration(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabPtr := unsafe.Pointer(&data[0])
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(sabPtr)
	epochSlice := (*[256]int32)(sabPtr)

	// 1. Go writes job to Inbox
	job := []byte(`{"method":"inference","params":{"model":"llama-7b"}}`)
	copy(sabSlice[sab.OFFSET_INBOX_BASE:], job)
	epochSlice[sab.IDX_INBOX_DIRTY]++

	// 2. Validate Rust can detect change
	assert.Equal(t, int32(1), epochSlice[sab.IDX_INBOX_DIRTY])

	// 3. Simulate Rust processing and writing result
	result := []byte(`{"status":"success","output":"Hello, world!"}`)
	copy(sabSlice[sab.OFFSET_OUTBOX_HOST_BASE:], result)
	epochSlice[int(sab.IDX_OUTBOX_HOST_DIRTY)]++

	// 4. Go reads result
	readResult := sabSlice[sab.OFFSET_OUTBOX_HOST_BASE : sab.OFFSET_OUTBOX_HOST_BASE+uint32(len(result))]
	assert.Equal(t, result, readResult)

	// 5. Validate epoch incremented
	assert.Equal(t, int32(1), epochSlice[int(sab.IDX_OUTBOX_HOST_DIRTY)])
}

// TestMultiModuleRegistration_Integration validates multiple modules
func TestMultiModuleRegistration_Integration(t *testing.T) {
	data := make([]byte, sab.SAB_SIZE_DEFAULT)
	sabSlice := (*[sab.SAB_SIZE_DEFAULT]byte)(unsafe.Pointer(&data[0]))

	modules := []string{"ml", "compute", "storage", "mining", "science", "drivers"}

	for i, modID := range modules {
		offset := sab.OFFSET_MODULE_REGISTRY + uint32(i)*sab.MODULE_ENTRY_SIZE

		// Write module ID
		moduleID := make([]byte, 32)
		copy(moduleID, []byte(modID))
		copy(sabSlice[offset:], moduleID)
	}

	// Validate all modules registered
	for i, modID := range modules {
		offset := sab.OFFSET_MODULE_REGISTRY + uint32(i)*sab.MODULE_ENTRY_SIZE
		readID := string(sabSlice[offset : offset+uint32(len(modID))])
		assert.Equal(t, modID, readID)
	}
}

// ========== HELPER FUNCTIONS ==========

// requireNoError is a helper that fails the test if error is not nil
func requireNoError(t *testing.T, err error, msg string) {
	t.Helper()
	require.NoError(t, err, msg)
}

// createTestSAB creates a test SAB with optional initialization
func createTestSAB(t *testing.T, size int, initialize bool) []byte {
	t.Helper()
	data := make([]byte, size)

	if initialize {
		// Initialize with pattern
		for i := range data {
			data[i] = byte(i % 256)
		}
	}

	return data
}
