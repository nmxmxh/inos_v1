package threads

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHybridAllocator_Basics(t *testing.T) {
	// Setup 10MB SAB (1MB Slab + 8MB Buddy + 64KB Metadata)
	totalSize := uint32(10 * 1024 * 1024)
	sab := make([]byte, totalSize)
	ha := NewHybridAllocator(sab)

	// Test Slab Allocation (Size <= 256)
	req1 := AllocationRequest{
		Size:     32,
		Owner:    "test",
		Priority: 1,
	}
	off1, err := ha.Allocate(req1)
	require.NoError(t, err)
	assert.True(t, off1 >= OFFSET_ARENA+ARENA_METADATA_SIZE)
	assert.True(t, off1 < OFFSET_ARENA+ARENA_METADATA_SIZE+ARENA_SLAB_SIZE)

	// Test Buddy Allocation (Small 4KB)
	req2 := AllocationRequest{
		Size:     4096,
		Owner:    "test",
		Priority: 1,
	}
	off2, err := ha.Allocate(req2)
	require.NoError(t, err)
	buddyStart := uint32(OFFSET_ARENA + ARENA_METADATA_SIZE + ARENA_SLAB_SIZE)
	assert.True(t, off2 >= buddyStart)

	// Test Buddy Allocation (Large 64KB)
	req3 := AllocationRequest{
		Size:     64 * 1024,
		Owner:    "test",
		Priority: 1,
	}
	off3, err := ha.Allocate(req3)
	require.NoError(t, err)
	assert.True(t, off3 >= buddyStart)

	// Verify Stats
	stats := ha.GetStats()
	assert.Greater(t, stats.AllocCount, uint64(0))
	assert.Greater(t, stats.TotalAllocated, uint64(0))

	// Free
	require.NoError(t, ha.Free(off1))
	require.NoError(t, ha.Free(off2))
	require.NoError(t, ha.Free(off3))

	stats2 := ha.GetStats()
	assert.Equal(t, stats2.FreeCount, uint64(3))
}

func TestHybridAllocator_SlabReclamation(t *testing.T) {
	totalSize := uint32(10 * 1024 * 1024)
	sab := make([]byte, totalSize)
	ha := NewHybridAllocator(sab)

	// Allocate many tiny objects to fill a page
	// Page size 4KB. Object 32B.
	// 4096 / 32 = 128 objects per page.
	offsets := make([]uint32, 128)
	for i := 0; i < 128; i++ {
		off, err := ha.Allocate(AllocationRequest{Size: 32})
		require.NoError(t, err)
		offsets[i] = off
	}

	// Verify they are on the same page (mostly)
	pageStart := offsets[0] & ^uint32(4095)
	for _, off := range offsets {
		assert.Equal(t, pageStart, off&^uint32(4095), "Objects should clearly fill a page")
	}

	// Free all
	for _, off := range offsets {
		require.NoError(t, ha.Free(off))
	}

	// Stats should show empty slabs
	stats := ha.GetStats()
	// Check slab stats
	slabStat := stats.SlabStats[3] // Size 32 is index 3?
	// Map: 8(0), 16(1), 24(2), 32(3)
	assert.Equal(t, 3, slabStat.SizeClass)
	// Allocated should be 0
	assert.Equal(t, uint32(0), slabStat.Allocated)
	// Capacity should be 128 (one page)
	assert.Equal(t, uint32(128), slabStat.Capacity)

	// Reclaim
	freed := ha.FreeCache()
	assert.Equal(t, uint32(4096), freed)

	statsAfter := ha.GetStats()
	assert.Equal(t, uint32(0), statsAfter.SlabStats[3].Capacity)
}
