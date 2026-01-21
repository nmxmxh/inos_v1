package routing

import (
	"fmt"
	"testing"
)

func BenchmarkMerkleTree_AddMessage(b *testing.B) {
	mt := NewMerkleTree()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mt.AddMessage(fmt.Sprintf("msg_%d", i))
	}
}

func BenchmarkMerkleTree_Rebuild(b *testing.B) {
	mt := NewMerkleTree()
	for i := 0; i < 1000; i++ {
		mt.AddMessage(fmt.Sprintf("msg_%d", i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mt.rebuild()
	}
}

func BenchmarkMerkleTree_GetChildren(b *testing.B) {
	mt := NewMerkleTree()
	for i := 0; i < 1000; i++ {
		mt.AddMessage(fmt.Sprintf("msg_%d", i))
	}
	root := mt.Root
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mt.GetChildren(root)
	}
}
