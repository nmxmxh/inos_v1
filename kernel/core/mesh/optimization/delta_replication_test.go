package optimization

import (
	"bytes"
	"testing"
)

func TestDeltaReplication_BuildMerkleTree(t *testing.T) {
	dr := NewDeltaReplication(1024)

	data := []byte("Hello, World! This is test data for merkle tree construction.")
	tree := dr.BuildMerkleTree("content1", data)

	if tree == nil {
		t.Fatal("Expected merkle tree, got nil")
	}

	if tree.Root == "" {
		t.Error("Expected non-empty root hash")
	}

	if len(tree.Leaves) == 0 {
		t.Error("Expected leaves in tree")
	}

	t.Logf("Tree: root=%s, leaves=%d, depth=%d", tree.Root[:8], len(tree.Leaves), tree.Depth)
}

func TestDeltaReplication_IdenticalContent(t *testing.T) {
	dr := NewDeltaReplication(1024)

	data := []byte("Identical content for both trees")
	tree1 := dr.BuildMerkleTree("content1", data)
	tree2 := dr.BuildMerkleTree("content2", data)

	// Identical content should have same root
	if tree1.Root != tree2.Root {
		t.Errorf("Expected identical roots for same content: %s != %s", tree1.Root[:8], tree2.Root[:8])
	}

	// Diff should be empty
	diff := dr.DiffTrees(tree1, tree2)
	if len(diff) != 0 {
		t.Errorf("Expected no diff for identical content, got %d differences", len(diff))
	}
}

func TestDeltaReplication_DifferentContent(t *testing.T) {
	dr := NewDeltaReplication(1024)

	data1 := []byte("Original content")
	data2 := []byte("Modified content")

	tree1 := dr.BuildMerkleTree("content1", data1)
	tree2 := dr.BuildMerkleTree("content2", data2)

	// Different content should have different roots
	if tree1.Root == tree2.Root {
		t.Error("Expected different roots for different content")
	}

	// Diff should show differences
	diff := dr.DiffTrees(tree1, tree2)
	if len(diff) == 0 {
		t.Error("Expected differences for different content")
	}

	t.Logf("Differences: %d chunks", len(diff))
}

func TestDeltaReplication_PartialModification(t *testing.T) {
	dr := NewDeltaReplication(100) // Small chunks for testing

	// Create original data (300 bytes = 3 chunks)
	original := make([]byte, 300)
	for i := range original {
		original[i] = byte(i % 256)
	}

	// Modify only middle chunk
	modified := make([]byte, 300)
	copy(modified, original)
	for i := 100; i < 200; i++ {
		modified[i] = byte((i + 50) % 256)
	}

	tree1 := dr.BuildMerkleTree("original", original)
	tree2 := dr.BuildMerkleTree("modified", modified)

	diff := dr.DiffTrees(tree1, tree2)

	// Should only detect middle chunk as different
	if len(diff) != 1 {
		t.Errorf("Expected 1 different chunk, got %d", len(diff))
	}

	if len(diff) > 0 && diff[0] != 1 {
		t.Errorf("Expected chunk index 1, got %d", diff[0])
	}
}

func TestDeltaReplication_BandwidthSavings(t *testing.T) {
	dr := NewDeltaReplication(1024)

	totalSize := 10240         // 10KB
	diffIndices := []int{0, 5} // 2 chunks different

	transferred, savings := dr.CalculateBandwidthSavings(totalSize, diffIndices)

	expectedTransferred := 2 * 1024 // 2 chunks
	if transferred != expectedTransferred {
		t.Errorf("Expected %d bytes transferred, got %d", expectedTransferred, transferred)
	}

	expectedSavings := 0.8 // 80% savings
	if savings < expectedSavings-0.01 || savings > expectedSavings+0.01 {
		t.Errorf("Expected ~%f savings, got %f", expectedSavings, savings)
	}

	t.Logf("Bandwidth: %d bytes transferred, %.1f%% savings", transferred, savings*100)
}

func TestDeltaReplication_GetChunks(t *testing.T) {
	dr := NewDeltaReplication(1024)

	data := make([]byte, 5120) // 5 chunks
	for i := range data {
		data[i] = byte(i % 256)
	}

	tree := dr.BuildMerkleTree("content1", data)
	chunks := dr.GetChunks(tree, []int{1, 3})

	if len(chunks) != 2 {
		t.Errorf("Expected 2 chunks, got %d", len(chunks))
	}

	if chunks[0].Index != 1 {
		t.Errorf("Expected chunk index 1, got %d", chunks[0].Index)
	}
	if chunks[1].Index != 3 {
		t.Errorf("Expected chunk index 3, got %d", chunks[1].Index)
	}
}

func TestDeltaReplication_ApplyDelta(t *testing.T) {
	dr := NewDeltaReplication(100)

	// Original data
	original := []byte("Original data that will be partially modified")

	// Create modified version
	modified := []byte("Original data that was successfully modified")

	// Build trees
	tree1 := dr.BuildMerkleTree("original", original)
	tree2 := dr.BuildMerkleTree("modified", modified)

	// Get diff
	diff := dr.DiffTrees(tree1, tree2)
	deltaChunks := dr.GetChunks(tree2, diff)

	// Apply delta
	result := dr.ApplyDelta(original, deltaChunks)

	// Trim result to modified length for comparison
	if len(result) > len(modified) {
		result = result[:len(modified)]
	}

	// Result should match modified
	if !bytes.Equal(result, modified) {
		t.Errorf("Delta application failed:\nExpected: %s\nGot: %s", string(modified), string(result))
	}
}

func TestDeltaReplication_LargeContent(t *testing.T) {
	dr := NewDeltaReplication(4096)

	// Create 1MB of data
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	tree := dr.BuildMerkleTree("large", data)

	expectedChunks := 1024 * 1024 / 4096 // 256 chunks
	if len(tree.Leaves) != expectedChunks {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, len(tree.Leaves))
	}

	t.Logf("Large content: %d bytes, %d chunks, depth=%d", len(data), len(tree.Leaves), tree.Depth)
}

func TestDeltaReplication_EmptyContent(t *testing.T) {
	dr := NewDeltaReplication(1024)

	data := []byte{}
	tree := dr.BuildMerkleTree("empty", data)

	if tree == nil {
		t.Fatal("Expected tree for empty content")
	}

	if len(tree.Leaves) != 0 {
		t.Errorf("Expected 0 leaves for empty content, got %d", len(tree.Leaves))
	}
}

func TestDeltaReplication_SingleChunk(t *testing.T) {
	dr := NewDeltaReplication(1024)

	data := []byte("Small data")
	tree := dr.BuildMerkleTree("small", data)

	if len(tree.Leaves) != 1 {
		t.Errorf("Expected 1 leaf for small content, got %d", len(tree.Leaves))
	}

	if tree.Depth != 0 {
		t.Errorf("Expected depth 0 for single leaf, got %d", tree.Depth)
	}
}

func TestDeltaReplication_Metrics(t *testing.T) {
	dr := NewDeltaReplication(2048)

	dr.BuildMerkleTree("content1", []byte("data1"))
	dr.BuildMerkleTree("content2", []byte("data2"))

	metrics := dr.GetMetrics()

	if metrics["cached_trees"] != 2 {
		t.Errorf("Expected 2 cached trees, got %v", metrics["cached_trees"])
	}

	if metrics["chunk_size"] != 2048 {
		t.Errorf("Expected chunk size 2048, got %v", metrics["chunk_size"])
	}
}

func TestDeltaReplication_ConcurrentAccess(t *testing.T) {
	dr := NewDeltaReplication(1024)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			data := make([]byte, 5000)
			for j := range data {
				data[j] = byte((id + j) % 256)
			}

			contentHash := string(rune(id))
			dr.BuildMerkleTree(contentHash, data)
			dr.GetMerkleTree(contentHash)

			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	metrics := dr.GetMetrics()
	if metrics["cached_trees"].(int) != 10 {
		t.Errorf("Expected 10 cached trees, got %v", metrics["cached_trees"])
	}
}

func BenchmarkDeltaReplication_BuildTree(b *testing.B) {
	dr := NewDeltaReplication(4096)
	data := make([]byte, 100*1024) // 100KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dr.BuildMerkleTree("content", data)
	}
}

func BenchmarkDeltaReplication_DiffTrees(b *testing.B) {
	dr := NewDeltaReplication(4096)

	data1 := make([]byte, 100*1024)
	data2 := make([]byte, 100*1024)
	copy(data2, data1)
	data2[50000] = 0xFF // Single byte difference

	tree1 := dr.BuildMerkleTree("content1", data1)
	tree2 := dr.BuildMerkleTree("content2", data2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dr.DiffTrees(tree1, tree2)
	}
}

func BenchmarkDeltaReplication_ApplyDelta(b *testing.B) {
	dr := NewDeltaReplication(4096)

	original := make([]byte, 100*1024)
	modified := make([]byte, 100*1024)
	copy(modified, original)
	modified[50000] = 0xFF

	tree1 := dr.BuildMerkleTree("original", original)
	tree2 := dr.BuildMerkleTree("modified", modified)

	diff := dr.DiffTrees(tree1, tree2)
	chunks := dr.GetChunks(tree2, diff)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dr.ApplyDelta(original, chunks)
	}
}
