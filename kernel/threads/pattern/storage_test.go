package pattern

import (
	"testing"

	"unsafe"

	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/threads/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPatternStorage_WriteAndRead tests basic write/read operations
func TestPatternStorage_WriteAndRead(t *testing.T) {
	// Create SAB
	sabData := testutil.NewMockSABBuilder(int(sab_layout.SAB_SIZE_DEFAULT)).Build()
	storage := NewTieredPatternStorage(unsafe.Pointer(&sabData[0]), uint32(len(sabData)), sab_layout.OFFSET_PATTERN_EXCHANGE, 1024)

	// Create test pattern
	pattern := &EnhancedPattern{
		Header: PatternHeader{
			Magic:      PATTERN_MAGIC,
			Type:       0, // TypeSequence
			Confidence: 95,
			Complexity: 3,
		},
		Body: PatternBody{
			Data: PatternData{
				Payload: []byte("test pattern data"),
				Size:    17,
			},
		},
	}

	// Write pattern
	err := storage.WritePattern(pattern)
	require.NoError(t, err)

	// Read back
	retrieved, err := storage.ReadPattern(pattern.Header.ID)
	require.NoError(t, err)
	assert.Equal(t, pattern.Header.Magic, retrieved.Header.Magic)
	assert.Equal(t, pattern.Header.Type, retrieved.Header.Type)
	assert.Equal(t, pattern.Header.Confidence, retrieved.Header.Confidence)
}

// TestPatternStorage_TierPromotion tests LRU eviction when tier 1 full
func TestPatternStorage_TierPromotion(t *testing.T) {
	sabData := testutil.NewMockSABBuilder(int(sab_layout.SAB_SIZE_DEFAULT)).Build()
	storage := NewTieredPatternStorage(unsafe.Pointer(&sabData[0]), uint32(len(sabData)), sab_layout.OFFSET_PATTERN_EXCHANGE, 1024)

	// Create pattern
	pattern := &EnhancedPattern{
		Header: PatternHeader{
			Magic:      PATTERN_MAGIC,
			Type:       0, // TypeSequence
			Confidence: 85,
		},
		Body: PatternBody{
			Data: PatternData{
				Payload: []byte("tier test"),
				Size:    9,
			},
		},
	}

	// Fill tier 1 to capacity (1024 patterns)
	firstPatternID := uint64(0)
	for i := 0; i < 1024; i++ {
		p := &EnhancedPattern{
			Header: PatternHeader{
				Magic:      PATTERN_MAGIC,
				Type:       0, // TypeSequence
				Confidence: 80,
			},
		}
		err := storage.WritePattern(p)
		require.NoError(t, err)
		if i == 0 {
			firstPatternID = p.Header.ID
		}
	}

	// Write one more pattern (should trigger LRU eviction)
	err := storage.WritePattern(pattern)
	require.NoError(t, err)

	// Verify we can read the new pattern
	retrieved, err := storage.ReadPattern(pattern.Header.ID)
	require.NoError(t, err)
	assert.Equal(t, pattern.Header.ID, retrieved.Header.ID)

	// Verify first pattern was evicted (LRU)
	_, err = storage.ReadPattern(firstPatternID)
	assert.Error(t, err) // Should not be found
}

// TestPatternStorage_LRUEviction tests LRU eviction when tier 1 full
func TestPatternStorage_LRUEviction(t *testing.T) {
	sabData := testutil.NewMockSABBuilder(int(sab_layout.SAB_SIZE_DEFAULT)).Build()
	storage := NewTieredPatternStorage(unsafe.Pointer(&sabData[0]), uint32(len(sabData)), sab_layout.OFFSET_PATTERN_EXCHANGE, 1024)

	// Fill tier 1 to capacity
	patterns := make([]*EnhancedPattern, 1025) // One more than capacity
	for i := 0; i < 1025; i++ {
		patterns[i] = &EnhancedPattern{
			Header: PatternHeader{
				Magic:      PATTERN_MAGIC,
				Type:       0, // TypeSequence
				Confidence: 80,
			},
		}
		err := storage.WritePattern(patterns[i])
		require.NoError(t, err)
	}

	// Tier 1 should be at capacity
	stats := storage.GetStats()
	assert.LessOrEqual(t, stats.Tier1Count, uint32(1024))
}

// TestPatternStorage_BloomFilter tests bloom filter prevents unnecessary lookups
func TestPatternStorage_BloomFilter(t *testing.T) {
	sabData := testutil.NewMockSABBuilder(int(sab_layout.SAB_SIZE_DEFAULT)).Build()
	storage := NewTieredPatternStorage(unsafe.Pointer(&sabData[0]), uint32(len(sabData)), sab_layout.OFFSET_PATTERN_EXCHANGE, 1024)

	// Try to read non-existent pattern
	_, err := storage.ReadPattern(99999)
	assert.Error(t, err)

	// Cache misses should increment
	stats := storage.GetStats()
	assert.Greater(t, stats.CacheMisses, uint64(0))
}

// TestPatternStorage_SABSynchronization tests SyncFromSAB for Rust-written patterns
func TestPatternStorage_SABSynchronization(t *testing.T) {
	// Create SAB with pre-written pattern (simulating Rust write)
	builder := testutil.NewMockSABBuilder(int(sab_layout.SAB_SIZE_DEFAULT))
	builder.AddPattern(12345, 0, 90, nil) // ID=12345, Type=Sequence, Confidence=90
	sabData := builder.Build()

	storage := NewTieredPatternStorage(unsafe.Pointer(&sabData[0]), uint32(len(sabData)), sab_layout.OFFSET_PATTERN_EXCHANGE, 1024)

	// Sync from SAB
	err := storage.SyncFromSAB()
	require.NoError(t, err)

	// Should be able to read the pattern
	retrieved, err := storage.ReadPattern(12345)
	require.NoError(t, err)
	assert.Equal(t, uint64(12345), retrieved.Header.ID)
	assert.Equal(t, uint16(0), uint16(retrieved.Header.Type))
	assert.Equal(t, uint8(90), retrieved.Header.Confidence)
}

// TestPatternStorage_MultiplePatterns tests writing multiple patterns
func TestPatternStorage_MultiplePatterns(t *testing.T) {
	sabData := testutil.NewMockSABBuilder(int(sab_layout.SAB_SIZE_DEFAULT)).Build()
	storage := NewTieredPatternStorage(unsafe.Pointer(&sabData[0]), uint32(len(sabData)), sab_layout.OFFSET_PATTERN_EXCHANGE, 1024)

	// Write 100 patterns
	for i := 0; i < 100; i++ {
		pattern := &EnhancedPattern{
			Header: PatternHeader{
				Magic:      PATTERN_MAGIC,
				Type:       0, // TypeSequence
				Confidence: uint8(50 + i%50),
			},
		}
		err := storage.WritePattern(pattern)
		require.NoError(t, err)
	}

	// Verify stats
	stats := storage.GetStats()
	assert.Equal(t, uint64(100), stats.TotalPatterns)
}

// TestPatternStorage_Query tests pattern querying
func TestPatternStorage_Query(t *testing.T) {
	sabData := testutil.NewMockSABBuilder(int(sab_layout.SAB_SIZE_DEFAULT)).Build()
	storage := NewTieredPatternStorage(unsafe.Pointer(&sabData[0]), uint32(len(sabData)), sab_layout.OFFSET_PATTERN_EXCHANGE, 1024)

	// Write patterns with different types
	for i := 0; i < 10; i++ {
		pattern := &EnhancedPattern{
			Header: PatternHeader{
				Magic:      PATTERN_MAGIC,
				Type:       PatternType(i % 3), // 3 different types
				Confidence: 80,
			},
		}
		err := storage.WritePattern(pattern)
		require.NoError(t, err)
	}

	// Query by type
	query := &PatternQuery{
		Types: []PatternType{0}, // TypeSequence
	}
	results, err := storage.Query(query)
	require.NoError(t, err)
	assert.Greater(t, len(results), 0)
}

// TestPatternStorage_ConcurrentAccess tests thread-safe operations
func TestPatternStorage_ConcurrentAccess(t *testing.T) {
	sabData := testutil.NewMockSABBuilder(int(sab_layout.SAB_SIZE_DEFAULT)).Build()
	storage := NewTieredPatternStorage(unsafe.Pointer(&sabData[0]), uint32(len(sabData)), sab_layout.OFFSET_PATTERN_EXCHANGE, 1024)

	// Write some patterns
	for i := 0; i < 10; i++ {
		pattern := &EnhancedPattern{
			Header: PatternHeader{
				Magic:      PATTERN_MAGIC,
				Type:       0, // TypeSequence
				Confidence: 80,
			},
		}
		_ = storage.WritePattern(pattern)
	}

	// Concurrent reads
	done := make(chan bool)
	for i := 0; i < 20; i++ {
		go func(id uint64) {
			_, _ = storage.ReadPattern(id)
			_ = storage.GetStats()
			done <- true
		}(uint64(i % 10))
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

// TestPatternStorage_Stats tests statistics tracking
func TestPatternStorage_Stats(t *testing.T) {
	sabData := testutil.NewMockSABBuilder(int(sab_layout.SAB_SIZE_DEFAULT)).Build()
	storage := NewTieredPatternStorage(unsafe.Pointer(&sabData[0]), uint32(len(sabData)), sab_layout.OFFSET_PATTERN_EXCHANGE, 1024)

	// Initial stats
	stats := storage.GetStats()
	assert.Equal(t, uint64(0), stats.TotalPatterns)
	assert.Equal(t, uint64(0), stats.CacheHits)

	// Write pattern
	pattern := &EnhancedPattern{
		Header: PatternHeader{
			Magic:      PATTERN_MAGIC,
			Type:       0, // TypeSequence
			Confidence: 85,
		},
	}
	err := storage.WritePattern(pattern)
	require.NoError(t, err)

	// Read pattern (should increment cache hits)
	_, err = storage.ReadPattern(pattern.Header.ID)
	require.NoError(t, err)

	// Verify stats updated
	stats = storage.GetStats()
	assert.Equal(t, uint64(1), stats.TotalPatterns)
	assert.Greater(t, stats.CacheHits, uint64(0))
}

// TestPatternStorage_InvalidPattern tests handling of invalid patterns
func TestPatternStorage_InvalidPattern(t *testing.T) {
	sabData := testutil.NewMockSABBuilder(int(sab_layout.SAB_SIZE_DEFAULT)).Build()
	storage := NewTieredPatternStorage(unsafe.Pointer(&sabData[0]), uint32(len(sabData)), sab_layout.OFFSET_PATTERN_EXCHANGE, 1024)

	// Try to read non-existent pattern
	_, err := storage.ReadPattern(0)
	assert.Error(t, err)

	// Try to read from empty storage
	_, err = storage.ReadPattern(12345)
	assert.Error(t, err)
}

// TestPatternStorage_LargePayload tests patterns with large payloads
func TestPatternStorage_LargePayload(t *testing.T) {
	sabData := testutil.NewMockSABBuilder(int(sab_layout.SAB_SIZE_DEFAULT)).Build()
	storage := NewTieredPatternStorage(unsafe.Pointer(&sabData[0]), uint32(len(sabData)), sab_layout.OFFSET_PATTERN_EXCHANGE, 1024)

	// Create pattern with large payload (10KB)
	largePayload := make([]byte, 10*1024)
	for i := range largePayload {
		largePayload[i] = byte(i % 256)
	}

	pattern := &EnhancedPattern{
		Header: PatternHeader{
			Magic:      PATTERN_MAGIC,
			Type:       0, // TypeSequence
			Confidence: 90,
		},
		Body: PatternBody{
			Data: PatternData{
				Payload: largePayload,
				Size:    uint16(len(largePayload)),
			},
		},
	}

	// Write pattern
	err := storage.WritePattern(pattern)
	require.NoError(t, err)

	// Read back and verify payload
	retrieved, err := storage.ReadPattern(pattern.Header.ID)
	require.NoError(t, err)
	assert.Equal(t, len(largePayload), len(retrieved.Body.Data.Payload))
}
