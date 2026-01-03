package threads

import (
	"fmt"
	"sync"
)

// Buddy allocator for large blocks (4KB-1MB)
// Uses power-of-2 block sizes with automatic coalescing

const (
	MIN_BUDDY_SIZE   = 4096        // 4KB
	MAX_BUDDY_SIZE   = 1024 * 1024 // 1MB
	NUM_BUDDY_LEVELS = 9           // 4KB to 1MB
)

type BuddyAllocator struct {
	sab        []byte
	baseOffset uint32
	totalSize  uint32

	// Free lists for each level (0=4KB, 1=8KB, ..., 8=1MB)
	freeLists [NUM_BUDDY_LEVELS]uint32

	// Allocation bitmap (1 bit per 4KB block)
	bitmap    []uint64
	bitmapLen int

	// Level tracking (1 byte per 4KB block)
	blockLevels []uint8

	mu sync.RWMutex
}

func NewBuddyAllocator(sab []byte, baseOffset, totalSize uint32) *BuddyAllocator {
	// Calculate bitmap size (1 bit per min block)
	numBlocks := int(totalSize / MIN_BUDDY_SIZE)
	bitmapLen := (numBlocks + 63) / 64 // Round up to uint64s

	ba := &BuddyAllocator{
		sab:         sab,
		baseOffset:  baseOffset,
		totalSize:   totalSize,
		bitmap:      make([]uint64, bitmapLen),
		bitmapLen:   bitmapLen,
		blockLevels: make([]uint8, numBlocks),
	}

	// Initialize free lists with largest possible blocks
	remaining := totalSize
	currentOffset := baseOffset

	for remaining >= MIN_BUDDY_SIZE {
		// Find largest level that fits
		level := NUM_BUDDY_LEVELS - 1
		for level >= 0 {
			size := ba.levelToSize(level)
			if size <= remaining {
				ba.addToFreeList(currentOffset, level)
				currentOffset += size
				remaining -= size
				break
			}
			level--
		}
	}

	return ba
}

// Allocate allocates a block of at least the given size
func (ba *BuddyAllocator) Allocate(size uint32) (uint32, error) {
	if size > MAX_BUDDY_SIZE {
		return 0, fmt.Errorf("size %d too large for buddy allocator", size)
	}
	if size < MIN_BUDDY_SIZE {
		size = MIN_BUDDY_SIZE
	}

	ba.mu.Lock()
	defer ba.mu.Unlock()

	level := ba.sizeToLevel(size)
	offset := ba.findFreeBlock(level)

	if offset == 0 {
		return 0, fmt.Errorf("out of memory")
	}

	ba.markAllocated(offset, level)
	return offset, nil
}

// Free frees a block at the given offset
func (ba *BuddyAllocator) Free(offset uint32) error {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	level := ba.getBlockLevel(offset)
	if level < 0 {
		return fmt.Errorf("invalid offset %d", offset)
	}

	ba.markFree(offset, level)
	ba.coalesce(offset, level)

	return nil
}

// Helper: Convert size to level
func (ba *BuddyAllocator) sizeToLevel(size uint32) int {
	level := 0
	blockSize := uint32(MIN_BUDDY_SIZE)

	for blockSize < size && level < NUM_BUDDY_LEVELS-1 {
		blockSize *= 2
		level++
	}

	return level
}

// Helper: Convert level to size
func (ba *BuddyAllocator) levelToSize(level int) uint32 {
	return MIN_BUDDY_SIZE << uint(level)
}

// Helper: Find free block at level or split larger block
func (ba *BuddyAllocator) findFreeBlock(level int) uint32 {
	// Check if free block exists at this level
	if ba.freeLists[level] != 0 {
		offset := ba.freeLists[level]
		ba.freeLists[level] = ba.getNextFree(offset)
		return offset
	}

	// Try to split a larger block
	for l := level + 1; l < NUM_BUDDY_LEVELS; l++ {
		if ba.freeLists[l] != 0 {
			return ba.splitBlock(l, level)
		}
	}

	return 0
}

// Helper: Split block from higher level to target level
func (ba *BuddyAllocator) splitBlock(fromLevel, toLevel int) uint32 {
	// Get block from higher level
	offset := ba.freeLists[fromLevel]
	ba.freeLists[fromLevel] = ba.getNextFree(offset)

	// Split down to target level
	for level := fromLevel - 1; level >= toLevel; level-- {
		blockSize := ba.levelToSize(level)
		buddyOffset := offset + blockSize

		// Add buddy to free list
		ba.addToFreeList(buddyOffset, level)
	}

	return offset
}

// Helper: Coalesce with buddy
func (ba *BuddyAllocator) coalesce(offset uint32, level int) {
	for level < NUM_BUDDY_LEVELS-1 {
		blockSize := ba.levelToSize(level)
		// Calculate buddy using relative offset
		relOffset := offset - ba.baseOffset
		buddyRel := relOffset ^ blockSize
		buddyOffset := ba.baseOffset + buddyRel

		// Check if buddy is free
		if !ba.isFree(buddyOffset, level) {
			break
		}

		// Remove buddy from free list
		ba.removeFromFreeList(buddyOffset, level)

		// Merge with buddy
		if buddyOffset < offset {
			offset = buddyOffset
		}
		level++
	}

	// Add coalesced block to free list
	ba.addToFreeList(offset, level)
}

// Helper: Check if block is free
func (ba *BuddyAllocator) isFree(offset uint32, level int) bool {
	blockSize := ba.levelToSize(level)
	numBlocks := blockSize / MIN_BUDDY_SIZE

	blockIndex := (offset - ba.baseOffset) / MIN_BUDDY_SIZE

	// Check if block is within arena bounds
	totalBlocks := ba.totalSize / MIN_BUDDY_SIZE
	if blockIndex+numBlocks > totalBlocks {
		return false
	}

	// Check all constituent 4KB blocks are free
	for i := uint32(0); i < numBlocks; i++ {
		bitIndex := int(blockIndex + i)
		if bitIndex >= len(ba.bitmap)*64 {
			return false
		}

		// If any block is allocated, the whole block is not free
		word := ba.bitmap[bitIndex/64]
		mask := uint64(1 << (bitIndex % 64))
		if (word & mask) != 0 {
			return false
		}
	}
	return true
}

// Helper: Mark block as allocated
func (ba *BuddyAllocator) markAllocated(offset uint32, level int) {
	blockSize := ba.levelToSize(level)
	numBlocks := blockSize / MIN_BUDDY_SIZE

	blockIndex := (offset - ba.baseOffset) / MIN_BUDDY_SIZE
	for i := uint32(0); i < numBlocks; i++ {
		bitIndex := int(blockIndex + i)
		ba.bitmap[bitIndex/64] |= (1 << (bitIndex % 64))
		ba.blockLevels[bitIndex] = uint8(level)
	}
}

// Helper: Mark block as free
func (ba *BuddyAllocator) markFree(offset uint32, level int) {
	blockSize := ba.levelToSize(level)
	numBlocks := blockSize / MIN_BUDDY_SIZE

	blockIndex := (offset - ba.baseOffset) / MIN_BUDDY_SIZE
	for i := uint32(0); i < numBlocks; i++ {
		bitIndex := int(blockIndex + i)
		ba.bitmap[bitIndex/64] &^= (1 << (bitIndex % 64))
	}
}

// Helper: Add block to free list
func (ba *BuddyAllocator) addToFreeList(offset uint32, level int) {
	// Write next pointer at offset (in SAB)
	nextOffset := ba.freeLists[level]
	ba.writeU32(offset, nextOffset)
	if nextOffset == offset {
		panic(fmt.Sprintf("Creating cycle in addToFreeList! off %d L%d", offset, level))
	}

	ba.freeLists[level] = offset
}

// Helper: Remove block from free list
func (ba *BuddyAllocator) removeFromFreeList(offset uint32, level int) {
	if ba.freeLists[level] == offset {
		ba.freeLists[level] = ba.getNextFree(offset)
		return
	}

	// Walk free list to find predecessor
	current := ba.freeLists[level]
	for current != 0 {
		next := ba.getNextFree(current)
		if next == offset {
			// Found it, update link
			nextNext := ba.getNextFree(offset)
			ba.writeU32(current, nextNext)
			return
		}
		current = next
	}
}

// Helper: Get next free block pointer from SAB
func (ba *BuddyAllocator) getNextFree(offset uint32) uint32 {
	if offset == 0 || offset < ba.baseOffset || offset >= ba.baseOffset+ba.totalSize {
		return 0
	}

	// Read uint32 from SAB
	idx := offset
	return uint32(ba.sab[idx]) |
		uint32(ba.sab[idx+1])<<8 |
		uint32(ba.sab[idx+2])<<16 |
		uint32(ba.sab[idx+3])<<24
}

// Helper: Write uint32 to SAB
func (ba *BuddyAllocator) writeU32(offset, value uint32) {
	ba.sab[offset] = byte(value)
	ba.sab[offset+1] = byte(value >> 8)
	ba.sab[offset+2] = byte(value >> 16)
	ba.sab[offset+3] = byte(value >> 24)
}

// Helper: Get block level from offset
func (ba *BuddyAllocator) getBlockLevel(offset uint32) int {
	blockIndex := (offset - ba.baseOffset) / MIN_BUDDY_SIZE
	if blockIndex >= uint32(len(ba.blockLevels)) {
		return -1
	}
	return int(ba.blockLevels[blockIndex])
}

// Statistics

type BuddyStats struct {
	TotalSize     uint32
	Allocated     uint32
	Free          uint32
	Fragmentation float32
	LevelStats    [NUM_BUDDY_LEVELS]LevelStats
}

type LevelStats struct {
	Level      int
	BlockSize  uint32
	FreeBlocks int
}

func (ba *BuddyAllocator) GetStats() BuddyStats {
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	stats := BuddyStats{
		TotalSize: ba.totalSize,
	}

	// Count allocated blocks
	allocated := uint32(0)
	for i := 0; i < len(ba.bitmap)*64; i++ {
		if (ba.bitmap[i/64] & (1 << (i % 64))) != 0 {
			allocated += MIN_BUDDY_SIZE
		}
	}
	stats.Allocated = allocated
	stats.Free = ba.totalSize - allocated

	// Calculate fragmentation
	if stats.Free > 0 {
		// Count free blocks at each level
		totalFreeBlocks := 0
		for level := 0; level < NUM_BUDDY_LEVELS; level++ {
			count := 0
			offset := ba.freeLists[level]
			// fmt.Printf("DEBUG: GetStats L%d head: %d\n", level, offset)
			for offset != 0 {
				count++
				next := ba.getNextFree(offset)
				if next == offset {
					panic("Cycle detected")
				}
				offset = next
				if count > 10000 {
					panic("Infinite loop")
				}
			}
			stats.LevelStats[level] = LevelStats{
				Level:      level,
				BlockSize:  ba.levelToSize(level),
				FreeBlocks: count,
			}
			totalFreeBlocks += count
		}

		// Fragmentation = (free blocks - 1) / total free space
		if totalFreeBlocks > 1 {
			stats.Fragmentation = float32(totalFreeBlocks-1) / float32(stats.Free/MIN_BUDDY_SIZE) * 100
		}
	}

	return stats
}
