package pattern

import "sync"

// BloomFilter provides fast pattern existence checks
type BloomFilter struct {
	bits []byte
	size uint32
	k    uint8 // Number of hash functions
	mu   sync.RWMutex
}

// NewBloomFilter creates a new bloom filter
func NewBloomFilter(sizeBytes uint32) *BloomFilter {
	return &BloomFilter{
		bits: make([]byte, sizeBytes),
		size: sizeBytes * 8, // bits
		k:    3,             // 3 hash functions
	}
}

// Add adds a pattern ID to the bloom filter
func (bf *BloomFilter) Add(id uint64) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	for i := uint8(0); i < bf.k; i++ {
		hash := bf.hash(id, i)
		byteIndex := hash / 8
		bitIndex := hash % 8
		bf.bits[byteIndex] |= 1 << bitIndex
	}
}

// Contains checks if a pattern ID might exist
func (bf *BloomFilter) Contains(id uint64) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	for i := uint8(0); i < bf.k; i++ {
		hash := bf.hash(id, i)
		byteIndex := hash / 8
		bitIndex := hash % 8

		if (bf.bits[byteIndex] & (1 << bitIndex)) == 0 {
			return false
		}
	}

	return true
}

// Helper: Hash function
func (bf *BloomFilter) hash(id uint64, seed uint8) uint32 {
	// Simple hash function (FNV-1a variant)
	hash := uint64(2166136261 + uint64(seed))
	hash ^= id
	hash *= 16777619
	return uint32(hash % uint64(bf.size))
}

// Clear clears the bloom filter
func (bf *BloomFilter) Clear() {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	for i := range bf.bits {
		bf.bits[i] = 0
	}
}

// EstimateFalsePositiveRate estimates the false positive rate
func (bf *BloomFilter) EstimateFalsePositiveRate(n uint32) float64 {
	// n = number of elements
	// m = number of bits
	// k = number of hash functions
	// FPR ≈ (1 - e^(-kn/m))^k

	m := float64(bf.size)
	k := float64(bf.k)
	nf := float64(n)

	// Simplified approximation: (1 - (1 - 1/m)^(kn))^k
	// For small n/m, this approximates to (kn/m)^k
	if nf == 0 {
		return 0
	}

	// Very simplified: just return a rough estimate
	return (k * nf / m)
}
