package pattern

import (
	"fmt"
	"testing"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/testutil"
)

func TestDebugCounting(t *testing.T) {
	sabSize := uint32(4 * 1024 * 1024)
	sabData := testutil.NewMockSABBuilder(int(sabSize)).Build()
	storage := NewTieredPatternStorage(unsafe.Pointer(&sabData[0]), sabSize, 0x10000, 1024)

	for i := 0; i < 10; i++ {
		pattern := &EnhancedPattern{
			Header: PatternHeader{
				Magic:      PATTERN_MAGIC,
				Type:       0,
				Confidence: 80,
			},
		}
		err := storage.WritePattern(pattern)
		if err != nil {
			fmt.Printf("Write %d failed: %v\n", i, err)
		} else {
			fmt.Printf("Write %d success, ID=%d\n", i, pattern.Header.ID)
		}
		stats := storage.GetStats()
		fmt.Printf("  TotalPatterns=%d\n", stats.TotalPatterns)
	}
}
