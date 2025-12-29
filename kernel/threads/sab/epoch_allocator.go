package sab

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"sync"
	"sync/atomic"
)

// EpochAllocator manages dynamic allocation of epoch indices for supervisors
// Supports up to 120 supervisors with automatic expansion
type EpochAllocator struct {
	sab        []byte
	allocTable *SupervisorAllocTable
	mu         sync.Mutex
}

// SupervisorAllocTable tracks epoch index allocations in SAB
type SupervisorAllocTable struct {
	// Bitmap of used indices (16 bytes = 128 bits for indices 0-127)
	UsedBitmap [16]uint8

	// Next available index hint (atomic)
	NextIndex uint32

	// Lock for allocation (atomic CAS)
	Lock uint32

	// Number of allocated indices
	AllocatedCount uint32

	// Mapping: supervisorID hash -> epoch index
	Allocations map[uint32]uint8
}

// NewEpochAllocator creates a new epoch allocator
func NewEpochAllocator(sab []byte) (*EpochAllocator, error) {
	if len(sab) < int(OFFSET_SUPERVISOR_ALLOC+SIZE_SUPERVISOR_ALLOC) {
		return nil, errors.New("SAB too small for supervisor allocation table")
	}

	ea := &EpochAllocator{
		sab: sab,
		allocTable: &SupervisorAllocTable{
			NextIndex:   SUPERVISOR_POOL_BASE,
			Allocations: make(map[uint32]uint8),
		},
	}

	// Load existing allocation table from SAB
	if err := ea.loadFromSAB(); err != nil {
		return nil, err
	}

	// Mark system epochs (0-7) as used
	for i := uint32(0); i < SUPERVISOR_POOL_BASE; i++ {
		ea.allocTable.markIndexUsed(i)
	}

	return ea, nil
}

// AllocateEpoch allocates an epoch index for a supervisor
// Returns the allocated index or error if pool is exhausted
func (ea *EpochAllocator) AllocateEpoch(supervisorID string) (uint32, error) {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	// Hash supervisor ID
	hash := crc32.ChecksumIEEE([]byte(supervisorID))

	// Check if already allocated
	if idx, exists := ea.allocTable.Allocations[hash]; exists {
		return uint32(idx), nil
	}

	// Find free index in pool (8-127)
	index, err := ea.findFreeIndex()
	if err != nil {
		return 0, err
	}

	// Mark index as used
	ea.allocTable.markIndexUsed(index)
	ea.allocTable.Allocations[hash] = uint8(index)
	ea.allocTable.AllocatedCount++

	// Update next index hint
	atomic.StoreUint32(&ea.allocTable.NextIndex, index+1)

	// Write to SAB
	if err := ea.writeToSAB(); err != nil {
		// Rollback allocation
		ea.allocTable.markIndexFree(index)
		delete(ea.allocTable.Allocations, hash)
		ea.allocTable.AllocatedCount--
		return 0, err
	}

	return index, nil
}

// FreeEpoch frees an epoch index
func (ea *EpochAllocator) FreeEpoch(supervisorID string) error {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	// Hash supervisor ID
	hash := crc32.ChecksumIEEE([]byte(supervisorID))

	// Check if allocated
	idx, exists := ea.allocTable.Allocations[hash]
	if !exists {
		return errors.New("supervisor not allocated")
	}

	// Mark index as free
	ea.allocTable.markIndexFree(uint32(idx))
	delete(ea.allocTable.Allocations, hash)
	ea.allocTable.AllocatedCount--

	// Update next index hint if this is lower
	if uint32(idx) < atomic.LoadUint32(&ea.allocTable.NextIndex) {
		atomic.StoreUint32(&ea.allocTable.NextIndex, uint32(idx))
	}

	// Write to SAB
	return ea.writeToSAB()
}

// GetEpochIndex returns the epoch index for a supervisor
func (ea *EpochAllocator) GetEpochIndex(supervisorID string) (uint32, error) {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	hash := crc32.ChecksumIEEE([]byte(supervisorID))
	idx, exists := ea.allocTable.Allocations[hash]
	if !exists {
		return 0, errors.New("supervisor not allocated")
	}

	return uint32(idx), nil
}

// GetAllocatedCount returns the number of allocated epoch indices
func (ea *EpochAllocator) GetAllocatedCount() uint32 {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	return ea.allocTable.AllocatedCount
}

// GetAvailableCount returns the number of available epoch indices
func (ea *EpochAllocator) GetAvailableCount() uint32 {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	return SUPERVISOR_POOL_SIZE - ea.allocTable.AllocatedCount
}

// IsIndexUsed checks if an index is used
func (ea *EpochAllocator) IsIndexUsed(index uint32) bool {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	return ea.allocTable.isIndexUsed(index)
}

// ListAllocations returns all current allocations
func (ea *EpochAllocator) ListAllocations() map[uint32]uint8 {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	// Create copy to avoid race conditions
	allocations := make(map[uint32]uint8, len(ea.allocTable.Allocations))
	for k, v := range ea.allocTable.Allocations {
		allocations[k] = v
	}

	return allocations
}

// Helper: Find free index in pool
func (ea *EpochAllocator) findFreeIndex() (uint32, error) {
	// Start from next index hint
	start := atomic.LoadUint32(&ea.allocTable.NextIndex)

	// Search from hint to end of pool
	for i := start; i < SUPERVISOR_POOL_BASE+SUPERVISOR_POOL_SIZE; i++ {
		if !ea.allocTable.isIndexUsed(i) {
			return i, nil
		}
	}

	// Wrap around and search from beginning
	for i := uint32(SUPERVISOR_POOL_BASE); i < start; i++ {
		if !ea.allocTable.isIndexUsed(i) {
			return i, nil
		}
	}

	return 0, errors.New("epoch pool exhausted")
}

// Helper: Load allocation table from SAB
func (ea *EpochAllocator) loadFromSAB() error {
	offset := OFFSET_SUPERVISOR_ALLOC

	// Read bitmap
	copy(ea.allocTable.UsedBitmap[:], ea.sab[offset:offset+16])
	offset += 16

	// Read next index
	ea.allocTable.NextIndex = binary.LittleEndian.Uint32(ea.sab[offset : offset+4])
	offset += 4

	// Read allocated count
	ea.allocTable.AllocatedCount = binary.LittleEndian.Uint32(ea.sab[offset : offset+4])
	offset += 4

	// Read allocations (hash -> index pairs)
	// Format: [count:4] [hash:4 index:1] [hash:4 index:1] ...
	count := binary.LittleEndian.Uint32(ea.sab[offset : offset+4])
	offset += 4

	ea.allocTable.Allocations = make(map[uint32]uint8, count)
	for i := uint32(0); i < count && i < SUPERVISOR_POOL_SIZE; i++ {
		hash := binary.LittleEndian.Uint32(ea.sab[offset : offset+4])
		offset += 4
		index := ea.sab[offset]
		offset++

		ea.allocTable.Allocations[hash] = index
	}

	return nil
}

// Helper: Write allocation table to SAB
func (ea *EpochAllocator) writeToSAB() error {
	offset := OFFSET_SUPERVISOR_ALLOC

	// Write bitmap
	copy(ea.sab[offset:offset+16], ea.allocTable.UsedBitmap[:])
	offset += 16

	// Write next index
	binary.LittleEndian.PutUint32(ea.sab[offset:offset+4], ea.allocTable.NextIndex)
	offset += 4

	// Write allocated count
	binary.LittleEndian.PutUint32(ea.sab[offset:offset+4], ea.allocTable.AllocatedCount)
	offset += 4

	// Write allocations
	count := uint32(len(ea.allocTable.Allocations))
	binary.LittleEndian.PutUint32(ea.sab[offset:offset+4], count)
	offset += 4

	for hash, index := range ea.allocTable.Allocations {
		binary.LittleEndian.PutUint32(ea.sab[offset:offset+4], hash)
		offset += 4
		ea.sab[offset] = index
		offset++
	}

	return nil
}

// SupervisorAllocTable methods

// markIndexUsed marks an index as used in the bitmap
func (t *SupervisorAllocTable) markIndexUsed(index uint32) {
	byteIndex := index / 8
	bitIndex := index % 8
	t.UsedBitmap[byteIndex] |= (1 << bitIndex)
}

// markIndexFree marks an index as free in the bitmap
func (t *SupervisorAllocTable) markIndexFree(index uint32) {
	byteIndex := index / 8
	bitIndex := index % 8
	t.UsedBitmap[byteIndex] &^= (1 << bitIndex)
}

// isIndexUsed checks if an index is used
func (t *SupervisorAllocTable) isIndexUsed(index uint32) bool {
	byteIndex := index / 8
	bitIndex := index % 8
	return (t.UsedBitmap[byteIndex] & (1 << bitIndex)) != 0
}

// GetStats returns allocation statistics
type EpochAllocStats struct {
	TotalCapacity  uint32
	AllocatedCount uint32
	AvailableCount uint32
	UtilizationPct float32
	NextIndex      uint32
}

func (ea *EpochAllocator) GetStats() EpochAllocStats {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	allocated := ea.allocTable.AllocatedCount
	total := uint32(SUPERVISOR_POOL_SIZE)

	return EpochAllocStats{
		TotalCapacity:  total,
		AllocatedCount: allocated,
		AvailableCount: total - allocated,
		UtilizationPct: float32(allocated) / float32(total) * 100.0,
		NextIndex:      atomic.LoadUint32(&ea.allocTable.NextIndex),
	}
}
