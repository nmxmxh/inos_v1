package pattern

import (
"fmt"
"testing"
)

func TestBloomFilterDebug(t *testing.T) {
	bf := NewBloomFilter(256)
	
	// Test with sequential IDs like the actual test
	for i := uint64(1000001); i <= 1000010; i++ {
		before := bf.Contains(i)
		bf.Add(i)
		after := bf.Contains(i)
		fmt.Printf("ID %d: before=%v, after=%v\n", i, before, after)
	}
	
	// Now check for false positives
	fmt.Println("\nChecking for false positives:")
	falsePositives := 0
	for i := uint64(1000011); i <= 1000110; i++ {
		if bf.Contains(i) {
			falsePositives++
			fmt.Printf("FALSE POSITIVE: ID %d\n", i)
		}
	}
	fmt.Printf("\nFalse positive rate: %d/100 = %.1f%%\n", falsePositives, float64(falsePositives))
}
