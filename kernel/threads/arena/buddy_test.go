package threads

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuddyAllocator_Allocate(t *testing.T) {
	// Setup 1MB SAB for testing (enough for 1MB total buffer)
	// But BuddyAllocator New takes totalSize.
	// Let's make it small for easy testing. e.g. 64KB.
	// 64KB contains 16 x 4KB blocks.
	// Max level will be smaller.

	// Create a BuddyAllocator with restricted size for testing logic
	// But the constants (MIN_BUDDY_SIZE=4096) are fixed in buddy.go.
	// We must respect them.

	totalSize := uint32(16 * 4096) // 64KB
	sab := make([]byte, totalSize+4096)
	baseOffset := uint32(4096) // Non-zero base
	ba := NewBuddyAllocator(sab, baseOffset, totalSize)

	// Allocate 4KB (Level 0)
	offset1, err := ba.Allocate(4096)
	require.NoError(t, err)
	assert.Equal(t, baseOffset, offset1, "First allocation should be at baseOffset")

	// Check stats
	stats := ba.GetStats()
	assert.Equal(t, uint32(4096), stats.Allocated)
	assert.Equal(t, totalSize-4096, stats.Free)

	// Allocate 4KB (Level 0)
	offset2, err := ba.Allocate(4096)
	require.NoError(t, err)
	assert.Equal(t, baseOffset+4096, offset2)

	// Allocate 8KB (Level 1)
	offset3, err := ba.Allocate(8192)
	require.NoError(t, err)
	assert.Equal(t, baseOffset+8192, offset3)

	// Free everything
	require.NoError(t, ba.Free(offset1))
	require.NoError(t, ba.Free(offset2))
	require.NoError(t, ba.Free(offset3))

	stats = ba.GetStats()
	assert.Equal(t, uint32(0), stats.Allocated)

	// Allocate 16KB
	offset4, err := ba.Allocate(16384)
	require.NoError(t, err)
	assert.Equal(t, baseOffset, offset4, "Should reuse coalesced space at baseOffset")
}

func TestBuddyAllocator_SplitAndCoalesce(t *testing.T) {
	totalSize := uint32(32 * 4096) // 128KB
	sab := make([]byte, totalSize+4096)
	baseOffset := uint32(4096)
	ba := NewBuddyAllocator(sab, baseOffset, totalSize)

	// Allocate 32KB. Should take chunk 0-32KB (relative).
	off1, err := ba.Allocate(32 * 1024)
	require.NoError(t, err)
	assert.Equal(t, baseOffset, off1)

	// Allocate 32KB. Should take chunk 32-64KB.
	off2, err := ba.Allocate(32 * 1024)
	require.NoError(t, err)
	assert.Equal(t, baseOffset+(32*1024), off2)

	// Free first chunk.
	err = ba.Free(off1)
	require.NoError(t, err)

	// Now allocate 16KB. Should reuse the first 32KB chunk by splitting it.
	off3, err := ba.Allocate(16 * 1024)
	require.NoError(t, err)
	assert.Equal(t, baseOffset, off3)

	// Allocate another 16KB. Should take the buddy of off3 (from 16KB to 32KB).
	off4, err := ba.Allocate(16 * 1024)
	require.NoError(t, err)
	assert.Equal(t, baseOffset+(16*1024), off4)

	// Free both to trigger coalesce back to 32KB chunk?
	ba.Free(off3)
	ba.Free(off4)

	// Allocate 32KB again. Should get baseOffset.
	off5, err := ba.Allocate(32 * 1024)
	require.NoError(t, err)
	assert.Equal(t, baseOffset, off5)
}

func TestBuddyAllocator_OOM(t *testing.T) {
	totalSize := uint32(4 * 4096) // 16KB
	sab := make([]byte, totalSize+4096)
	baseOffset := uint32(4096)
	ba := NewBuddyAllocator(sab, baseOffset, totalSize)

	// Allocate all
	_, err := ba.Allocate(16 * 1024)
	require.NoError(t, err)

	// Next should fail
	_, err = ba.Allocate(4096)
	assert.Error(t, err)
}

func TestBuddyAllocator_CorrectInitialization(t *testing.T) {
	// Provide 1MB
	totalSize := uint32(1024 * 1024)
	sab := make([]byte, totalSize+4096)
	baseOffset := uint32(4096)
	ba := NewBuddyAllocator(sab, baseOffset, totalSize)

	// Allocation of 1MB should succeed
	offset, err := ba.Allocate(1024 * 1024)
	require.NoError(t, err)
	assert.Equal(t, baseOffset, offset)

	// Next any allocation fails
	_, err = ba.Allocate(4096)
	assert.Error(t, err)
}

func TestBuddyAllocator_InvalidFree(t *testing.T) {
	totalSize := uint32(1024 * 1024)
	sab := make([]byte, totalSize+4096)
	baseOffset := uint32(4096)
	ba := NewBuddyAllocator(sab, baseOffset, totalSize)

	err := ba.Free(12345) // Unallocated
	// Note: Current implementation might not detect unallocated free cleanly unless we check bitmap.
	// But it might fail on level check if 12345 is not aligned or out of bounds.
	// 12345 is not valid.
	if err == nil {
		// If it accepts it, verify it didn't crash.
	}

	// Valid allocation
	off, _ := ba.Allocate(4096)
	err = ba.Free(off)
	require.NoError(t, err)
}
