package threads

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// HybridAllocator coordinates Slab and Buddy allocators
// Routes allocations based on size

const (
	// From sab/layout.go
	OFFSET_ARENA = 0x150000 // Standardized Phase 16

	// Arena layout (Resized for 16MB total SAB)
	// Metadata + Queues take 0x010000 (64KB)
	ARENA_METADATA_SIZE = 64 * 1024
	ARENA_SLAB_SIZE     = 1 * 1024 * 1024 // 1MB for tiny objects
	ARENA_BUDDY_SIZE    = 8 * 1024 * 1024 // 8MB for larger blocks

	// Allocation priorities
	PRIORITY_NORMAL   = 0
	PRIORITY_HIGH     = 1
	PRIORITY_CRITICAL = 2
)

type HybridAllocator struct {
	sab []byte

	// Sub-allocators
	slab  *SlabAllocator
	buddy *BuddyAllocator

	// Statistics
	totalAllocated uint64
	totalFreed     uint64
	allocCount     uint64
	freeCount      uint64

	mu sync.RWMutex
}

type AllocationRequest struct {
	Size      uint32
	Owner     string
	Priority  uint8
	Alignment uint32
	Flags     AllocFlags
}

type AllocFlags uint32

const (
	FlagPersistent AllocFlags = 1 << 0 // Survives module unload
	FlagShared     AllocFlags = 1 << 1 // Shareable across modules
	FlagZeroed     AllocFlags = 1 << 2 // Zero on allocation
	FlagGuarded    AllocFlags = 1 << 3 // Add guard pages
)

func NewHybridAllocator(sabBytes []byte) *HybridAllocator {
	// Calculate offsets
	slabOffset := OFFSET_ARENA + ARENA_METADATA_SIZE
	buddyOffset := slabOffset + ARENA_SLAB_SIZE

	ha := &HybridAllocator{
		sab:   sabBytes,
		slab:  NewSlabAllocator(sabBytes, uint32(slabOffset), ARENA_SLAB_SIZE),
		buddy: NewBuddyAllocator(sabBytes, uint32(buddyOffset), ARENA_BUDDY_SIZE),
	}

	return ha
}

// Allocate allocates memory based on size
func (ha *HybridAllocator) Allocate(req AllocationRequest) (uint32, error) {
	var offset uint32
	var err error

	// Route to appropriate allocator
	if req.Size <= 256 {
		offset, err = ha.slab.Allocate(req.Size)
	} else if req.Size < MIN_BUDDY_SIZE {
		// Use buddy for sizes between 256B and 4KB
		offset, err = ha.buddy.Allocate(MIN_BUDDY_SIZE)
	} else {
		offset, err = ha.buddy.Allocate(req.Size)
	}

	if err != nil {
		return 0, err
	}

	// Zero memory if requested
	if req.Flags&FlagZeroed != 0 {
		ha.zeroMemory(offset, req.Size)
	}

	// Update statistics
	atomic.AddUint64(&ha.totalAllocated, uint64(req.Size))
	atomic.AddUint64(&ha.allocCount, 1)

	return offset, nil
}

// Free frees memory at the given offset
func (ha *HybridAllocator) Free(offset uint32) error {
	// Determine which allocator owns this offset
	slabStart := OFFSET_ARENA + ARENA_METADATA_SIZE
	slabEnd := slabStart + ARENA_SLAB_SIZE
	buddyStart := slabEnd

	var err error
	if offset >= uint32(slabStart) && offset < uint32(slabEnd) {
		err = ha.slab.Free(offset)
	} else if offset >= uint32(buddyStart) {
		err = ha.buddy.Free(offset)
	} else {
		return fmt.Errorf("invalid offset %d", offset)
	}

	if err == nil {
		atomic.AddUint64(&ha.freeCount, 1)
	}

	return err
}

// Helper: Zero memory
func (ha *HybridAllocator) zeroMemory(offset, size uint32) {
	for i := uint32(0); i < size; i++ {
		ha.sab[offset+i] = 0
	}
}

// Statistics

type HybridStats struct {
	TotalAllocated uint64
	TotalFreed     uint64
	AllocCount     uint64
	FreeCount      uint64

	SlabStats  []SlabStats
	BuddyStats BuddyStats

	OverallFragmentation float32
}

func (ha *HybridAllocator) GetStats() HybridStats {
	ha.mu.RLock()
	defer ha.mu.RUnlock()

	slabStats := ha.slab.GetStats()
	buddyStats := ha.buddy.GetStats()

	// Calculate overall fragmentation
	totalAllocated := uint64(0)
	totalCapacity := uint64(ARENA_SLAB_SIZE + ARENA_BUDDY_SIZE)

	for _, s := range slabStats {
		totalAllocated += uint64(s.Allocated * s.ObjectSize)
	}
	totalAllocated += uint64(buddyStats.Allocated)

	fragmentation := float32(0)
	if totalCapacity > 0 {
		utilization := float32(totalAllocated) / float32(totalCapacity)
		fragmentation = (1 - utilization) * 100
	}

	return HybridStats{
		TotalAllocated:       atomic.LoadUint64(&ha.totalAllocated),
		TotalFreed:           atomic.LoadUint64(&ha.totalFreed),
		AllocCount:           atomic.LoadUint64(&ha.allocCount),
		FreeCount:            atomic.LoadUint64(&ha.freeCount),
		SlabStats:            slabStats,
		BuddyStats:           buddyStats,
		OverallFragmentation: fragmentation,
	}
}

// FreeCache frees cached memory (for OOM recovery)
func (ha *HybridAllocator) FreeCache() uint32 {
	return ha.slab.FreeEmptySlabs()
}
