package threads

import (
	"fmt"
	"sync"
)

// Slab allocator for tiny objects (8B-256B)
// Uses fixed-size object pools with bitmap tracking

const (
	SLAB_PAGE_SIZE = 4096 // 4KB per slab page

	// Size classes (10 classes)
	SIZE_8   = 0
	SIZE_16  = 1
	SIZE_24  = 2
	SIZE_32  = 3
	SIZE_48  = 4
	SIZE_64  = 5
	SIZE_96  = 6
	SIZE_128 = 7
	SIZE_192 = 8
	SIZE_256 = 9
)

var sizeClassSizes = [10]uint32{8, 16, 24, 32, 48, 64, 96, 128, 192, 256}

type SlabAllocator struct {
	sab        []byte
	baseOffset uint32
	totalSize  uint32

	// One cache per size class
	caches [10]*SlabCache

	mu sync.RWMutex
}

type SlabCache struct {
	objectSize uint32
	slabs      []*SlabPage

	// Statistics
	allocated uint32
	capacity  uint32

	mu sync.Mutex
}

type SlabPage struct {
	offset     uint32 // Offset in SAB
	freeCount  uint16
	totalCount uint16
	bitmap     uint64 // Free object bitmap (max 64 objects per page)
}

func NewSlabAllocator(sab []byte, baseOffset, totalSize uint32) *SlabAllocator {
	sa := &SlabAllocator{
		sab:        sab,
		baseOffset: baseOffset,
		totalSize:  totalSize,
	}

	// Initialize caches for each size class
	for i := 0; i < 10; i++ {
		sa.caches[i] = &SlabCache{
			objectSize: sizeClassSizes[i],
			slabs:      make([]*SlabPage, 0, 16),
		}
	}

	return sa
}

// Allocate allocates an object of the given size
func (sa *SlabAllocator) Allocate(size uint32) (uint32, error) {
	if size > 256 {
		return 0, fmt.Errorf("size %d too large for slab allocator", size)
	}

	// Find size class
	sizeClass := sa.getSizeClass(size)
	cache := sa.caches[sizeClass]

	return cache.allocate(sa)
}

// Free frees an object at the given offset
func (sa *SlabAllocator) Free(offset uint32) error {
	// Determine which slab page this belongs to
	slab, cache := sa.findSlab(offset)
	if slab == nil {
		return fmt.Errorf("invalid offset %d", offset)
	}

	return cache.free(slab, offset)
}

// Helper: Get size class for requested size
func (sa *SlabAllocator) getSizeClass(size uint32) int {
	for i, classSize := range sizeClassSizes {
		if size <= classSize {
			return i
		}
	}
	return SIZE_256
}

// Helper: Find slab page containing offset
func (sa *SlabAllocator) findSlab(offset uint32) (*SlabPage, *SlabCache) {
	for _, cache := range sa.caches {
		cache.mu.Lock()
		for _, slab := range cache.slabs {
			if offset >= slab.offset && offset < slab.offset+SLAB_PAGE_SIZE {
				cache.mu.Unlock()
				return slab, cache
			}
		}
		cache.mu.Unlock()
	}
	return nil, nil
}

// SlabCache methods

func (sc *SlabCache) allocate(sa *SlabAllocator) (uint32, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Find slab with free objects
	for _, slab := range sc.slabs {
		if slab.freeCount > 0 {
			return sc.allocateFromSlab(slab)
		}
	}

	// Need new slab page
	slab, err := sc.allocateNewSlab(sa)
	if err != nil {
		return 0, err
	}

	return sc.allocateFromSlab(slab)
}

func (sc *SlabCache) allocateFromSlab(slab *SlabPage) (uint32, error) {
	// Find first free bit in bitmap
	for i := uint16(0); i < slab.totalCount; i++ {
		if (slab.bitmap & (1 << i)) != 0 {
			// Found free object
			slab.bitmap &^= (1 << i) // Clear bit
			slab.freeCount--
			sc.allocated++

			offset := slab.offset + uint32(i)*sc.objectSize
			return offset, nil
		}
	}

	return 0, fmt.Errorf("slab has no free objects")
}

func (sc *SlabCache) allocateNewSlab(sa *SlabAllocator) (*SlabPage, error) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	// Calculate how many slabs we already have
	totalSlabSize := uint32(len(sc.slabs)) * SLAB_PAGE_SIZE
	if totalSlabSize >= sa.totalSize {
		return nil, fmt.Errorf("slab allocator out of memory")
	}

	// Allocate new slab page
	slabOffset := sa.baseOffset + totalSlabSize
	objectsPerPage := uint16(SLAB_PAGE_SIZE / sc.objectSize)

	slab := &SlabPage{
		offset:     slabOffset,
		freeCount:  objectsPerPage,
		totalCount: objectsPerPage,
		bitmap:     (1 << objectsPerPage) - 1, // All bits set (all free)
	}

	sc.slabs = append(sc.slabs, slab)
	sc.capacity += uint32(objectsPerPage)

	return slab, nil
}

func (sc *SlabCache) free(slab *SlabPage, offset uint32) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Calculate object index within slab
	relativeOffset := offset - slab.offset
	if relativeOffset%sc.objectSize != 0 {
		return fmt.Errorf("invalid offset alignment")
	}

	objectIndex := uint16(relativeOffset / sc.objectSize)
	if objectIndex >= slab.totalCount {
		return fmt.Errorf("object index out of range")
	}

	// Check if already free
	if (slab.bitmap & (1 << objectIndex)) != 0 {
		return fmt.Errorf("double free detected at offset %d", offset)
	}

	// Mark as free
	slab.bitmap |= (1 << objectIndex)
	slab.freeCount++
	sc.allocated--

	return nil
}

// Statistics

type SlabStats struct {
	SizeClass   int
	ObjectSize  uint32
	Allocated   uint32
	Capacity    uint32
	SlabCount   int
	Utilization float32
}

func (sa *SlabAllocator) GetStats() []SlabStats {
	stats := make([]SlabStats, 10)

	for i, cache := range sa.caches {
		cache.mu.Lock()
		utilization := float32(0)
		if cache.capacity > 0 {
			utilization = float32(cache.allocated) / float32(cache.capacity) * 100
		}

		stats[i] = SlabStats{
			SizeClass:   i,
			ObjectSize:  cache.objectSize,
			Allocated:   cache.allocated,
			Capacity:    cache.capacity,
			SlabCount:   len(cache.slabs),
			Utilization: utilization,
		}
		cache.mu.Unlock()
	}

	return stats
}

// FreeEmptySlabs frees slab pages that are completely empty
func (sa *SlabAllocator) FreeEmptySlabs() uint32 {
	freed := uint32(0)

	for _, cache := range sa.caches {
		cache.mu.Lock()

		// Keep only non-empty slabs
		kept := make([]*SlabPage, 0, len(cache.slabs))
		for _, slab := range cache.slabs {
			if slab.freeCount < slab.totalCount {
				// Slab has allocated objects, keep it
				kept = append(kept, slab)
			} else {
				// Slab is empty, free it
				freed += SLAB_PAGE_SIZE
				cache.capacity -= uint32(slab.totalCount)
			}
		}

		cache.slabs = kept
		cache.mu.Unlock()
	}

	return freed
}
