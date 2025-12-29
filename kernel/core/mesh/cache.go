package mesh

import (
	"container/list"
	"sync"
	"time"
)

// ChunkPeerMapping tracks which peers have which chunks
type ChunkPeerMapping struct {
	ChunkHash   string
	PeerIDs     []string
	LastUpdated time.Time
	Confidence  float32 // 0.0-1.0, based on gossip confirmations
}

// ChunkCache implements an LRU cache for chunk-to-peers mappings
type ChunkCache struct {
	maxSize   int
	ttl       time.Duration
	mu        sync.RWMutex
	cache     map[string]*list.Element
	lruList   *list.List
	hits      uint64
	misses    uint64
	evictions uint64
}

type cacheEntry struct {
	key     string
	mapping *ChunkPeerMapping
}

// NewChunkCache creates a new chunk cache
func NewChunkCache(maxSize int, ttl time.Duration) *ChunkCache {
	return &ChunkCache{
		maxSize: maxSize,
		ttl:     ttl,
		cache:   make(map[string]*list.Element),
		lruList: list.New(),
	}
}

// Get retrieves a chunk mapping from cache
func (cc *ChunkCache) Get(chunkHash string) (*ChunkPeerMapping, bool) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	elem, exists := cc.cache[chunkHash]
	if !exists {
		cc.misses++
		return nil, false
	}

	entry := elem.Value.(*cacheEntry)

	// Check if expired
	if time.Since(entry.mapping.LastUpdated) > cc.ttl {
		cc.lruList.Remove(elem)
		delete(cc.cache, chunkHash)
		cc.evictions++
		cc.misses++
		return nil, false
	}

	// Move to front (most recently used)
	cc.lruList.MoveToFront(elem)
	cc.hits++
	return entry.mapping, true
}

// Put adds or updates a chunk mapping in cache
func (cc *ChunkCache) Put(chunkHash string, peerIDs []string, confidence float32) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	mapping := &ChunkPeerMapping{
		ChunkHash:   chunkHash,
		PeerIDs:     peerIDs,
		LastUpdated: time.Now(),
		Confidence:  confidence,
	}

	// Check if already exists
	if elem, exists := cc.cache[chunkHash]; exists {
		// Update existing
		entry := elem.Value.(*cacheEntry)
		entry.mapping = mapping
		cc.lruList.MoveToFront(elem)
		return
	}

	// Add new entry
	entry := &cacheEntry{
		key:     chunkHash,
		mapping: mapping,
	}
	elem := cc.lruList.PushFront(entry)
	cc.cache[chunkHash] = elem

	// Evict if over capacity
	if cc.lruList.Len() > cc.maxSize {
		oldest := cc.lruList.Back()
		if oldest != nil {
			cc.lruList.Remove(oldest)
			oldEntry := oldest.Value.(*cacheEntry)
			delete(cc.cache, oldEntry.key)
			cc.evictions++
		}
	}
}

// AddPeer adds a peer to an existing chunk mapping
func (cc *ChunkCache) AddPeer(chunkHash, peerID string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	elem, exists := cc.cache[chunkHash]
	if !exists {
		// Create new mapping
		mapping := &ChunkPeerMapping{
			ChunkHash:   chunkHash,
			PeerIDs:     []string{peerID},
			LastUpdated: time.Now(),
			Confidence:  0.5,
		}
		entry := &cacheEntry{
			key:     chunkHash,
			mapping: mapping,
		}
		elem := cc.lruList.PushFront(entry)
		cc.cache[chunkHash] = elem
		return
	}

	// Add to existing
	entry := elem.Value.(*cacheEntry)

	// Check if peer already exists
	for _, existingPeer := range entry.mapping.PeerIDs {
		if existingPeer == peerID {
			// Already exists, just update timestamp
			entry.mapping.LastUpdated = time.Now()
			cc.lruList.MoveToFront(elem)
			return
		}
	}

	// Add new peer
	entry.mapping.PeerIDs = append(entry.mapping.PeerIDs, peerID)
	entry.mapping.LastUpdated = time.Now()
	entry.mapping.Confidence = min(entry.mapping.Confidence+0.1, 1.0)
	cc.lruList.MoveToFront(elem)
}

// Remove removes a chunk from cache
func (cc *ChunkCache) Remove(chunkHash string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if elem, exists := cc.cache[chunkHash]; exists {
		cc.lruList.Remove(elem)
		delete(cc.cache, chunkHash)
	}
}

// Clear removes all entries
func (cc *ChunkCache) Clear() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.cache = make(map[string]*list.Element)
	cc.lruList = list.New()
}

// GetMetrics returns cache metrics
func (cc *ChunkCache) GetMetrics() CacheMetrics {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	total := cc.hits + cc.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(cc.hits) / float64(total)
	}

	return CacheMetrics{
		Hits:      cc.hits,
		Misses:    cc.misses,
		Evictions: cc.evictions,
		HitRate:   hitRate,
		Size:      cc.lruList.Len(),
		MaxSize:   cc.maxSize,
	}
}

// CleanupExpired removes expired entries
func (cc *ChunkCache) CleanupExpired() int {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	now := time.Now()
	removed := 0

	// Iterate from back (oldest) to front
	for elem := cc.lruList.Back(); elem != nil; {
		entry := elem.Value.(*cacheEntry)

		if now.Sub(entry.mapping.LastUpdated) > cc.ttl {
			prev := elem.Prev()
			cc.lruList.Remove(elem)
			delete(cc.cache, entry.key)
			cc.evictions++
			removed++
			elem = prev
		} else {
			// Since list is ordered by access time, we can break
			break
		}
	}

	return removed
}

// CacheMetrics holds cache performance metrics
type CacheMetrics struct {
	Hits      uint64
	Misses    uint64
	Evictions uint64
	HitRate   float64
	Size      int
	MaxSize   int
}

func min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
