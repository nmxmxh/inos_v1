package pattern

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"unsafe"

	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
)

// Storage tiers
const (
	Tier1Hot       = 1 // SAB, 1024 patterns, <100ns
	Tier2Warm      = 2 // Arena, 10K patterns, <1µs
	Tier3Cold      = 3 // Persistent, unlimited, <10ms
	Tier4Ephemeral = 4 // RAM, LRU cache
)

// TieredPatternStorage manages patterns across multiple storage tiers
type TieredPatternStorage struct {
	sabPtr     unsafe.Pointer
	sabSize    uint32
	baseOffset uint32

	// Storage tiers
	tier1 *HotPatternCache
	tier2 *WarmPatternStore
	tier3 *PersistentPatternStore
	tier4 *EphemeralPatternCache

	// Metadata and indices
	metadata *PatternMetadataStore
	indices  *PatternIndices
	bloom    *BloomFilter

	// Statistics
	stats StorageStats

	// ID generation
	nextID uint64

	mu sync.RWMutex
}

// HotPatternCache stores frequently accessed patterns in SAB
type HotPatternCache struct {
	sabPtr     unsafe.Pointer
	sabSize    uint32
	baseOffset uint32
	capacity   uint32 // 1024 patterns
	entries    []PatternEntry
	lru        *LRUList
	accessMap  map[uint64]uint32 // ID → slot
	hitRate    float32
	mu         sync.RWMutex
}

// PatternEntry is a compact pattern entry (64 bytes)
type PatternEntry struct {
	Header   PatternHeader
	DataHash uint32 // Hash of pattern data
	Tier     uint8  // Current storage tier
	Next     uint32 // Next pattern in linked list
	DataPtr  uint32 // Pointer to data in appropriate tier
}

// WarmPatternStore stores less frequently accessed patterns in arena
type WarmPatternStore struct {
	arena    []byte
	capacity uint32 // 10K patterns
	entries  map[uint64]*EnhancedPattern
	mu       sync.RWMutex
}

// PersistentPatternStore stores cold patterns (simple file-based)
type PersistentPatternStore struct {
	filename string
	patterns map[uint64]*EnhancedPattern
	mu       sync.RWMutex
}

func NewPersistentPatternStore(filename string) *PersistentPatternStore {
	pps := &PersistentPatternStore{
		filename: filename,
		patterns: make(map[uint64]*EnhancedPattern),
	}
	// Attempt to load on startup
	_ = pps.load()
	return pps
}

func (pps *PersistentPatternStore) load() error {
	data, err := os.ReadFile(pps.filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &pps.patterns)
}

func (pps *PersistentPatternStore) save() error {
	data, err := json.Marshal(pps.patterns)
	if err != nil {
		return err
	}
	return os.WriteFile(pps.filename, data, 0644)
}

// EphemeralPatternCache stores temporary patterns in RAM
type EphemeralPatternCache struct {
	capacity uint32
	patterns map[uint64]*EnhancedPattern
	lru      *LRUList
	mu       sync.RWMutex
}

// LRUList implements a simple LRU list
type LRUList struct {
	head  *LRUNode
	tail  *LRUNode
	nodes map[uint64]*LRUNode
	size  int
}

type LRUNode struct {
	patternID uint64
	prev      *LRUNode
	next      *LRUNode
}

// StorageStats tracks storage statistics
type StorageStats struct {
	Tier1Count    uint32
	Tier2Count    uint32
	Tier3Count    uint32
	Tier4Count    uint32
	TotalPatterns uint64
	CacheHits     uint64
	CacheMisses   uint64
	Promotions    uint64
	Demotions     uint64
	Evictions     uint64
}

// NewTieredPatternStorage creates a new tiered pattern storage
func NewTieredPatternStorage(sabPtr unsafe.Pointer, sabSize, baseOffset, capacity uint32) *TieredPatternStorage {
	return &TieredPatternStorage{
		sabPtr:     sabPtr,
		sabSize:    sabSize,
		baseOffset: baseOffset,
		tier1: &HotPatternCache{
			sabPtr:     sabPtr,
			sabSize:    sabSize,
			baseOffset: baseOffset,
			capacity:   1024,
			entries:    make([]PatternEntry, 1024),
			lru:        NewLRUList(),
			accessMap:  make(map[uint64]uint32),
		},
		tier2: &WarmPatternStore{
			capacity: 10000,
			entries:  make(map[uint64]*EnhancedPattern),
		},
		tier3: NewPersistentPatternStore("patterns.json"),
		tier4: &EphemeralPatternCache{
			capacity: 256,
			patterns: make(map[uint64]*EnhancedPattern),
			lru:      NewLRUList(),
		},
		metadata: NewPatternMetadataStore(),
		indices:  NewPatternIndices(),
		bloom:    NewBloomFilter(256),
	}
}

// Query searches for patterns matching the query
func (tps *TieredPatternStorage) Query(query *PatternQuery) ([]*EnhancedPattern, error) {
	tps.mu.RLock()
	defer tps.mu.RUnlock()

	var results []*EnhancedPattern
	var candidateIDs []uint64

	// optimized search using indices
	if len(query.Tags) > 0 {
		for _, tag := range query.Tags {
			candidateIDs = append(candidateIDs, tps.indices.FindByTag(tag)...)
		}
	}

	if len(query.Types) > 0 {
		for _, t := range query.Types {
			candidateIDs = append(candidateIDs, tps.indices.FindByType(t)...)
		}
	} else if query.MinConfidence > 0 {
		candidateIDs = tps.indices.FindByConfidence(query.MinConfidence)
	} else if len(query.Sources) > 0 {
		for _, s := range query.Sources {
			candidateIDs = append(candidateIDs, tps.indices.FindBySource(s)...)
		}
	}

	// De-duplicate candidates (simplified)
	seen := make(map[uint64]bool)
	for _, id := range candidateIDs {
		if seen[id] {
			continue
		}
		seen[id] = true

		// Read pattern
		if p, err := tps.ReadPattern(id); err == nil {
			// Apply remaining filters (like TimeRange) manually
			if query.TimeRange != nil {
				ts := int64(p.Header.Timestamp)
				if ts < query.TimeRange.Start.UnixNano() || ts > query.TimeRange.End.UnixNano() {
					continue
				}
			}
			results = append(results, p)
		}
	}

	return results, nil
}

// WritePattern writes a pattern to storage
func (tps *TieredPatternStorage) WritePattern(pattern *EnhancedPattern) error {
	tps.mu.Lock()
	defer tps.mu.Unlock()

	// Generate ID if not set
	if pattern.Header.ID == 0 {
		pattern.Header.ID = tps.generatePatternID()
	}

	// Check if pattern already exists (after ID is set)
	isNew := !tps.bloom.Contains(pattern.Header.ID)

	// Add to bloom filter
	tps.bloom.Add(pattern.Header.ID)

	// Write to tier 1 (hot cache)
	if err := tps.tier1.Write(pattern); err != nil {
		// If tier 1 is full, write to tier 2
		if err := tps.tier2.Write(pattern); err != nil {
			// If tier 2 is full, write to tier 3
			return tps.tier3.Write(pattern)
		}
	}

	// Update indices
	tps.indices.Add(pattern)

	// Update stats (only increment for new patterns)
	if isNew {
		atomic.AddUint64(&tps.stats.TotalPatterns, 1)
	}

	return nil
}

// ReadPattern reads a pattern by ID
func (tps *TieredPatternStorage) ReadPattern(id uint64) (*EnhancedPattern, error) {
	tps.mu.RLock()

	// Check bloom filter first
	if !tps.bloom.Contains(id) {
		tps.mu.RUnlock()
		atomic.AddUint64(&tps.stats.CacheMisses, 1)
		return nil, fmt.Errorf("pattern %d not found", id)
	}

	// Try tier 1 (hot)
	if pattern, err := tps.tier1.Read(id); err == nil {
		tps.mu.RUnlock()
		atomic.AddUint64(&tps.stats.CacheHits, 1)
		return pattern, nil
	}

	// Try tier 2 (warm)
	if pattern, err := tps.tier2.Read(id); err == nil {
		tps.mu.RUnlock()
		// Promote to tier 1 (need write lock)
		tps.mu.Lock()
		tps.promote(pattern, Tier1Hot)
		tps.mu.Unlock()
		atomic.AddUint64(&tps.stats.CacheHits, 1)
		return pattern, nil
	}

	// Try tier 3 (cold)
	if pattern, err := tps.tier3.Read(id); err == nil {
		tps.mu.RUnlock()
		// Promote to tier 2 (need write lock)
		tps.mu.Lock()
		tps.promote(pattern, Tier2Warm)
		tps.mu.Unlock()
		return pattern, nil
	}

	// Try tier 4 (ephemeral)
	if pattern, err := tps.tier4.Read(id); err == nil {
		tps.mu.RUnlock()
		return pattern, nil
	}

	tps.mu.RUnlock()
	atomic.AddUint64(&tps.stats.CacheMisses, 1)
	return nil, fmt.Errorf("pattern %d not found in any tier", id)
}

// SyncFromSAB scans the SAB for external patterns (from Rust modules)
func (tps *TieredPatternStorage) SyncFromSAB() error {
	tps.mu.Lock()
	defer tps.mu.Unlock()
	return tps.tier1.SyncFromSAB(tps.bloom)
}

// Helper: Generate pattern ID
func (tps *TieredPatternStorage) generatePatternID() uint64 {
	// Use a separate counter for ID generation (not TotalPatterns)
	// IDs start from current time to avoid collisions
	return uint64(1000000) + atomic.AddUint64(&tps.nextID, 1)
}

// Helper: Promote pattern to higher tier
func (tps *TieredPatternStorage) promote(pattern *EnhancedPattern, toTier uint8) error {
	atomic.AddUint64(&tps.stats.Promotions, 1)

	// Simple promotion: Write to new tier, remove from old (if supported)
	switch toTier {
	case Tier1Hot:
		_ = tps.tier1.Write(pattern)
	case Tier2Warm:
		_ = tps.tier2.Write(pattern)
	}

	// We don't strictly delete from lower tiers immediately in this model
	// as lower tiers act as backing store.
	// But for T2->T1, we might want to keep T2 as backup?
	// For now, simple duplication is fine as "Promotion".
	return nil
}

// HotPatternCache methods

func (hpc *HotPatternCache) Write(pattern *EnhancedPattern) error {
	hpc.mu.Lock()
	defer hpc.mu.Unlock()

	// Check capacity
	if uint32(len(hpc.accessMap)) >= hpc.capacity {
		// Evict LRU
		if err := hpc.evictLRU(); err != nil {
			return err
		}
	}

	// Find slot
	slot := uint32(len(hpc.accessMap))
	hpc.accessMap[pattern.Header.ID] = slot

	// Calculate hash
	hash := hpc.calculateHash(pattern.Body.Data.Payload)

	// Allocate space in Arena for payload
	payloadSize := uint32(len(pattern.Body.Data.Payload))
	dataPtr := uint32(0)
	if payloadSize > 0 {
		ptr, err := hpc.allocateArena(payloadSize)
		if err == nil {
			dataPtr = ptr
			// Write payload to Arena (Zero-copy)
			destPtr := unsafe.Add(hpc.sabPtr, dataPtr)
			dest := unsafe.Slice((*byte)(destPtr), payloadSize)
			copy(dest, pattern.Body.Data.Payload)
		}
	}

	// Write entry to shadow cache
	hpc.entries[slot] = PatternEntry{
		Header:   pattern.Header,
		DataHash: hash,
		Tier:     Tier1Hot,
		DataPtr:  dataPtr,
	}

	// Write to SAB (Source of Truth)
	hpc.writeToSAB(slot, pattern.Header, dataPtr, uint16(payloadSize))

	// Update LRU
	hpc.lru.Add(pattern.Header.ID)

	return nil
}

// allocateArena implements a simple atomic bump allocator for the SAB Arena
func (hpc *HotPatternCache) allocateArena(size uint32) (uint32, error) {
	// IDX_ARENA_ALLOCATOR = 8 (Index in Atomic Flags)
	// Atomic Flags start at OFFSET_ATOMIC_FLAGS (0x01000000)
	// 0x01000000 + (8 * 4) = 0x01000020
	atomicOffset := sab_layout.OFFSET_ATOMIC_FLAGS + (sab_layout.IDX_ARENA_ALLOCATOR * 4)
	atomicPtr := unsafe.Add(hpc.sabPtr, atomicOffset)
	ptr := (*uint32)(atomicPtr)

	// Atomic add to get current bump pointer
	relativeOffset := atomic.AddUint32(ptr, size) - size

	// Calculate absolute offset
	absOffset := sab_layout.OFFSET_ARENA + relativeOffset

	// Boundary check
	if absOffset+size > hpc.sabSize {
		// Rollback on failure (best effort)
		atomic.AddUint32(ptr, -size)
		return 0, fmt.Errorf("arena overflow")
	}

	return absOffset, nil
}

// SyncFromSAB scans all slots in the SAB hot cache
func (hpc *HotPatternCache) SyncFromSAB(bloom *BloomFilter) error {
	hpc.mu.Lock()
	defer hpc.mu.Unlock()

	// 1024 slots * 64 bytes
	// We scan for valid magic bytes
	for slot := uint32(0); slot < hpc.capacity; slot++ {
		header := hpc.readFromSAB(slot)
		if header == nil {
			continue
		}

		// Check if we already have this pattern ID
		_, exists := hpc.accessMap[header.ID]
		if !exists {
			// Read DataPtr (last 4 bytes of 64B slot) at OFFSET_PATTERN_EXCHANGE + (slot*64) + 60
			entryOffset := hpc.baseOffset + (slot * 64)
			ptr := unsafe.Add(hpc.sabPtr, entryOffset+60)
			dataPtr := binary.LittleEndian.Uint32(unsafe.Slice((*byte)(ptr), 4))

			// New pattern discovered from Rust!
			// Import it into our local cache
			hpc.accessMap[header.ID] = slot
			hpc.entries[slot] = PatternEntry{
				Header:   *header,
				DataHash: 0,
				Tier:     Tier1Hot,
				DataPtr:  dataPtr,
			}
			hpc.lru.Add(header.ID)

			// Add to bloom filter
			if bloom != nil {
				bloom.Add(header.ID)
			}
		}
	}
	return nil
}

// Helper: Serialize header to SAB
func (hpc *HotPatternCache) writeToSAB(slot uint32, header PatternHeader, dataPtr uint32, payloadSize uint16) {
	offset := hpc.baseOffset + (slot * 64) // 64 byte entries
	if offset+64 > hpc.sabSize {
		return // Guard against overflow
	}

	ptr := unsafe.Add(hpc.sabPtr, offset)
	data := unsafe.Slice((*byte)(ptr), 64)

	binary.LittleEndian.PutUint64(data[0:8], header.Magic)
	binary.LittleEndian.PutUint64(data[8:16], header.ID)
	binary.LittleEndian.PutUint16(data[16:18], header.Version)
	binary.LittleEndian.PutUint16(data[18:20], uint16(header.Type))
	data[20] = header.Complexity
	data[21] = header.Confidence
	binary.LittleEndian.PutUint32(data[22:26], header.SourceHash)
	binary.LittleEndian.PutUint64(data[26:34], header.Timestamp)
	binary.LittleEndian.PutUint64(data[34:42], header.Expiration)
	binary.LittleEndian.PutUint32(data[42:46], math.Float32bits(header.Weight))
	binary.LittleEndian.PutUint32(data[46:50], header.AccessCount)
	binary.LittleEndian.PutUint32(data[50:54], math.Float32bits(header.SuccessRate))
	binary.LittleEndian.PutUint16(data[54:56], uint16(header.Flags))

	// Store payload size (bytes 56-58)
	binary.LittleEndian.PutUint16(data[56:58], payloadSize)

	// Store DataPtr at end of slot (bytes 60-64)
	binary.LittleEndian.PutUint32(data[60:64], dataPtr)
}

// Helper: Deserialize header from SAB
func (hpc *HotPatternCache) readFromSAB(slot uint32) *PatternHeader {
	offset := hpc.baseOffset + (slot * 64)
	if offset+64 > hpc.sabSize {
		return nil
	}

	ptr := unsafe.Add(hpc.sabPtr, offset)
	data := unsafe.Slice((*byte)(ptr), 64)
	header := &PatternHeader{}

	header.Magic = binary.LittleEndian.Uint64(data[0:8])
	if header.Magic != PATTERN_MAGIC {
		return nil // Invalid or empty
	}

	header.ID = binary.LittleEndian.Uint64(data[8:16])
	header.Version = binary.LittleEndian.Uint16(data[16:18])
	header.Type = PatternType(binary.LittleEndian.Uint16(data[18:20]))
	header.Complexity = data[20]
	header.Confidence = data[21]
	header.SourceHash = binary.LittleEndian.Uint32(data[22:26])
	header.Timestamp = binary.LittleEndian.Uint64(data[26:34])
	header.Expiration = binary.LittleEndian.Uint64(data[34:42])
	header.Weight = math.Float32frombits(binary.LittleEndian.Uint32(data[42:46]))
	header.AccessCount = binary.LittleEndian.Uint32(data[46:50])
	header.SuccessRate = math.Float32frombits(binary.LittleEndian.Uint32(data[50:54]))
	header.Flags = PatternFlags(binary.LittleEndian.Uint16(data[54:56]))

	return header
}

func (hpc *HotPatternCache) Read(id uint64) (*EnhancedPattern, error) {
	hpc.mu.RLock()
	defer hpc.mu.RUnlock()

	slot, exists := hpc.accessMap[id]
	if !exists {
		return nil, fmt.Errorf("pattern %d not in hot cache", id)
	}

	// Update LRU
	hpc.lru.Touch(id)

	// Return pattern
	entry := hpc.entries[slot]
	pattern := &EnhancedPattern{
		Header: entry.Header,
	}

	// Read Payload if DataPtr is set
	if entry.DataPtr > 0 {
		// Read size from SAB slot (bytes 56-58 contain the size)
		entryOffset := hpc.baseOffset + (slot * 64)
		ptr := unsafe.Add(hpc.sabPtr, entryOffset+56)
		// Actually let's use a better way to read uint16
		sizeBytes := unsafe.Slice((*byte)(ptr), 2)
		valSize := binary.LittleEndian.Uint16(sizeBytes)

		if valSize > 0 {
			// Read payload from arena using DataPtr
			payload := make([]byte, valSize)
			ptrData := unsafe.Add(hpc.sabPtr, entry.DataPtr)
			src := unsafe.Slice((*byte)(ptrData), valSize)
			copy(payload, src)
			pattern.Body.Data.Payload = payload
			pattern.Body.Data.Size = valSize
		}
	}

	return pattern, nil
}

func (hpc *HotPatternCache) evictLRU() error {
	// Get LRU pattern
	id := hpc.lru.RemoveLRU()
	if id == 0 {
		return fmt.Errorf("no patterns to evict")
	}

	// Remove from access map
	delete(hpc.accessMap, id)

	return nil
}

func (hpc *HotPatternCache) calculateHash(data []byte) uint32 {
	h := fnv.New32a()
	h.Write(data)
	return h.Sum32()
}

// WarmPatternStore methods

func (wps *WarmPatternStore) Write(pattern *EnhancedPattern) error {
	wps.mu.Lock()
	defer wps.mu.Unlock()

	if uint32(len(wps.entries)) >= wps.capacity {
		return fmt.Errorf("warm store full")
	}

	wps.entries[pattern.Header.ID] = pattern
	return nil
}

func (wps *WarmPatternStore) Read(id uint64) (*EnhancedPattern, error) {
	wps.mu.RLock()
	defer wps.mu.RUnlock()

	pattern, exists := wps.entries[id]
	if !exists {
		return nil, fmt.Errorf("pattern %d not in warm store", id)
	}

	return pattern, nil
}

// PersistentPatternStore methods

func (pps *PersistentPatternStore) Write(pattern *EnhancedPattern) error {
	pps.mu.Lock()
	defer pps.mu.Unlock()

	pps.patterns[pattern.Header.ID] = pattern
	// Sync to disk
	return pps.save()
}

func (pps *PersistentPatternStore) Read(id uint64) (*EnhancedPattern, error) {
	pps.mu.RLock()
	defer pps.mu.RUnlock()

	pattern, exists := pps.patterns[id]
	if !exists {
		return nil, fmt.Errorf("pattern %d not in persistent store", id)
	}

	return pattern, nil
}

// EphemeralPatternCache methods

func (epc *EphemeralPatternCache) Write(pattern *EnhancedPattern) error {
	epc.mu.Lock()
	defer epc.mu.Unlock()

	if uint32(len(epc.patterns)) >= epc.capacity {
		// Evict LRU
		id := epc.lru.RemoveLRU()
		delete(epc.patterns, id)
	}

	epc.patterns[pattern.Header.ID] = pattern
	epc.lru.Add(pattern.Header.ID)

	return nil
}

func (epc *EphemeralPatternCache) Read(id uint64) (*EnhancedPattern, error) {
	epc.mu.RLock()
	defer epc.mu.RUnlock()

	pattern, exists := epc.patterns[id]
	if !exists {
		return nil, fmt.Errorf("pattern %d not in ephemeral cache", id)
	}

	epc.lru.Touch(id)
	return pattern, nil
}

// LRUList methods

func NewLRUList() *LRUList {
	return &LRUList{
		nodes: make(map[uint64]*LRUNode),
	}
}

func (lru *LRUList) Add(id uint64) {
	if _, exists := lru.nodes[id]; exists {
		lru.Touch(id)
		return
	}

	node := &LRUNode{patternID: id}
	lru.nodes[id] = node

	if lru.head == nil {
		lru.head = node
		lru.tail = node
	} else {
		node.next = lru.head
		lru.head.prev = node
		lru.head = node
	}

	lru.size++
}

func (lru *LRUList) Touch(id uint64) {
	node, exists := lru.nodes[id]
	if !exists {
		return
	}

	if node == lru.head {
		return
	}

	// Detach
	if node.prev != nil {
		node.prev.next = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	}
	if node == lru.tail {
		lru.tail = node.prev
	}

	// Move to head
	node.next = lru.head
	node.prev = nil
	if lru.head != nil {
		lru.head.prev = node
	}
	lru.head = node

	if lru.tail == nil {
		lru.tail = node
	}
}

func (lru *LRUList) RemoveLRU() uint64 {
	if lru.tail == nil {
		return 0
	}

	id := lru.tail.patternID
	delete(lru.nodes, id)

	if lru.tail.prev != nil {
		lru.tail = lru.tail.prev
		lru.tail.next = nil
	} else {
		lru.head = nil
		lru.tail = nil
	}

	lru.size--
	return id
}

// GetStats returns storage statistics
func (tps *TieredPatternStorage) GetStats() StorageStats {
	return StorageStats{
		Tier1Count:    uint32(len(tps.tier1.accessMap)),
		Tier2Count:    uint32(len(tps.tier2.entries)),
		Tier3Count:    uint32(len(tps.tier3.patterns)),
		Tier4Count:    uint32(len(tps.tier4.patterns)),
		TotalPatterns: atomic.LoadUint64(&tps.stats.TotalPatterns),
		CacheHits:     atomic.LoadUint64(&tps.stats.CacheHits),
		CacheMisses:   atomic.LoadUint64(&tps.stats.CacheMisses),
		Promotions:    atomic.LoadUint64(&tps.stats.Promotions),
		Demotions:     atomic.LoadUint64(&tps.stats.Demotions),
		Evictions:     atomic.LoadUint64(&tps.stats.Evictions),
	}
}
