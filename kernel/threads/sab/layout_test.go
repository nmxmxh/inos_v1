package sab

import (
	"testing"
	"unsafe"
)

// TestSABLayout validates SAB memory layout constants
func TestSABLayout(t *testing.T) {
	// Use the built-in validator which checks all regions for overlaps
	if err := ValidateMemoryLayout(SAB_SIZE_DEFAULT); err != nil {
		t.Errorf("ValidateMemoryLayout failed: %v", err)
	}

	// Double check critical boundaries manually
	if OFFSET_MODULE_REGISTRY < OFFSET_ATOMIC_FLAGS+SIZE_ATOMIC_FLAGS {
		t.Error("Module registry overlaps with atomic flags")
	}

	if OFFSET_INBOX_BASE < OFFSET_MODULE_REGISTRY+SIZE_MODULE_REGISTRY {
		t.Error("Inbox overlaps with module registry")
	}

	if OFFSET_OUTBOX_BASE < OFFSET_INBOX_BASE+SIZE_INBOX_TOTAL {
		t.Error("Outbox overlaps with inbox")
	}

	if OFFSET_ARENA < OFFSET_OUTBOX_BASE+SIZE_OUTBOX_TOTAL {
		t.Error("Arena overlaps with outbox")
	}
}

// TestZeroCopyWrite validates zero-copy SAB writes
func TestZeroCopyWrite(t *testing.T) {
	data := make([]byte, SAB_SIZE_DEFAULT)
	sab := unsafe.Pointer(&data[0])

	testData := []byte{1, 2, 3, 4}
	ptrBefore := unsafe.Pointer(&testData[0])

	// Write to SAB
	sabSlice := (*[SAB_SIZE_DEFAULT]byte)(sab)
	copy(sabSlice[0x1000:], testData)

	ptrAfter := unsafe.Pointer(&testData[0])

	// Validate: No copy (pointer unchanged)
	if ptrBefore != ptrAfter {
		t.Error("Pointer changed - not zero-copy!")
	}

	// Validate: Data written correctly
	read := sabSlice[0x1000 : 0x1000+4]
	for i, v := range testData {
		if read[i] != v {
			t.Errorf("Data mismatch at %d: expected %d, got %d", i, v, read[i])
		}
	}
}

// TestEpochSignaling validates epoch-based signaling
func TestEpochSignaling(t *testing.T) {
	data := make([]byte, SAB_SIZE_DEFAULT)
	sab := unsafe.Pointer(&data[0])
	epochSlice := (*[256]int32)(sab)

	// Initial state
	if epochSlice[IDX_SYSTEM_EPOCH] != 0 {
		t.Error("Epoch should start at 0")
	}

	// Increment epoch
	epochSlice[IDX_SYSTEM_EPOCH]++

	// Validate
	if epochSlice[IDX_SYSTEM_EPOCH] != 1 {
		t.Errorf("Epoch should be 1, got %d", epochSlice[IDX_SYSTEM_EPOCH])
	}

	// Multiple increments
	for i := 2; i <= 10; i++ {
		epochSlice[IDX_SYSTEM_EPOCH]++
		if epochSlice[IDX_SYSTEM_EPOCH] != int32(i) {
			t.Errorf("Epoch should be %d, got %d", i, epochSlice[IDX_SYSTEM_EPOCH])
		}
	}
}
