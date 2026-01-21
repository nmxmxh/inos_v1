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
	// Proper hash mixing for each seed
	// Use different hash functions for each seed to avoid collisions
	h := id

	// Mix in the seed
	h ^= uint64(seed) * 0x9e3779b97f4a7c15 // Golden ratio

	// MurmurHash3-style mixing
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33

	return uint32(h % uint64(bf.size))
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
