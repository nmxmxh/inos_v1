package optimization

import (
	
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// DeltaReplication manages efficient content replication using merkle trees
type DeltaReplication struct {
	mu sync.RWMutex

	// Merkle trees for content: contentHash -> tree
	trees map[string]*ContentMerkleTree

	// Chunk size for splitting content
	chunkSize int
}

// NewDeltaReplication creates a new delta replication manager
func NewDeltaReplication(chunkSize int) *DeltaReplication {
	if chunkSize <= 0 {
		chunkSize = 4096 // Default 4KB chunks
	}

	return &DeltaReplication{
		trees:     make(map[string]*ContentMerkleTree),
		chunkSize: chunkSize,
	}
}

// BuildMerkleTree builds a merkle tree for content
func (dr *DeltaReplication) BuildMerkleTree(contentHash string, data []byte) *ContentMerkleTree {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	// Split data into chunks
	var leaves []ContentMerkleLeaf
	for i := 0; i < len(data); i += dr.chunkSize {
		end := i + dr.chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[i:end]
		hash := hashData(chunk)

		leaves = append(leaves, ContentMerkleLeaf{
			Index: len(leaves),
			Hash:  hash,
			Data:  chunk,
		})
	}

	// Build tree from leaves
	tree := &ContentMerkleTree{
		Leaves: leaves,
		Depth:  calculateDepth(len(leaves)),
	}

	// Calculate root hash
	tree.Root = dr.calculateRoot(leaves)

	// Cache tree
	dr.trees[contentHash] = tree

	return tree
}

// GetMerkleTree returns the cached merkle tree for content
func (dr *DeltaReplication) GetMerkleTree(contentHash string) *ContentMerkleTree {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	return dr.trees[contentHash]
}

// DiffTrees compares two merkle trees and returns differing chunk indices
func (dr *DeltaReplication) DiffTrees(tree1, tree2 *ContentMerkleTree) []int {
	if tree1 == nil || tree2 == nil {
		return nil
	}

	// If roots match, trees are identical
	if tree1.Root == tree2.Root {
		return nil
	}

	var diffIndices []int

	// Compare leaves
	maxLeaves := len(tree1.Leaves)
	if len(tree2.Leaves) > maxLeaves {
		maxLeaves = len(tree2.Leaves)
	}

	for i := 0; i < maxLeaves; i++ {
		var hash1, hash2 string

		if i < len(tree1.Leaves) {
			hash1 = tree1.Leaves[i].Hash
		}
		if i < len(tree2.Leaves) {
			hash2 = tree2.Leaves[i].Hash
		}

		if hash1 != hash2 {
			diffIndices = append(diffIndices, i)
		}
	}

	return diffIndices
}

// CalculateBandwidthSavings calculates bandwidth saved by delta replication
func (dr *DeltaReplication) CalculateBandwidthSavings(totalSize int, diffIndices []int) (int, float64) {
	if len(diffIndices) == 0 {
		return 0, 1.0 // 100% savings
	}

	bytesTransferred := len(diffIndices) * dr.chunkSize
	if bytesTransferred > totalSize {
		bytesTransferred = totalSize
	}

	savings := float64(totalSize-bytesTransferred) / float64(totalSize)
	return bytesTransferred, savings
}

// GetChunks returns the chunks that need to be transferred
func (dr *DeltaReplication) GetChunks(tree *ContentMerkleTree, indices []int) []ContentMerkleLeaf {
	var chunks []ContentMerkleLeaf

	for _, idx := range indices {
		if idx >= 0 && idx < len(tree.Leaves) {
			chunks = append(chunks, tree.Leaves[idx])
		}
	}

	return chunks
}

// ApplyDelta applies delta chunks to existing content
func (dr *DeltaReplication) ApplyDelta(existingData []byte, deltaChunks []ContentMerkleLeaf) []byte {
	// Create result buffer
	result := make([]byte, len(existingData))
	copy(result, existingData)

	// Apply delta chunks
	for _, chunk := range deltaChunks {
		offset := chunk.Index * dr.chunkSize
		if offset < len(result) {
			end := offset + len(chunk.Data)
			if end > len(result) {
				// Extend result if needed
				result = append(result, make([]byte, end-len(result))...)
			}
			copy(result[offset:], chunk.Data)
		}
	}

	return result
}

// GetMetrics returns delta replication metrics
func (dr *DeltaReplication) GetMetrics() map[string]interface{} {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	return map[string]interface{}{
		"cached_trees": len(dr.trees),
		"chunk_size":   dr.chunkSize,
	}
}

// Helper functions

func hashData(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (dr *DeltaReplication) calculateRoot(leaves []ContentMerkleLeaf) string {
	if len(leaves) == 0 {
		return ""
	}

	// Collect leaf hashes
	hashes := make([]string, len(leaves))
	for i, leaf := range leaves {
		hashes[i] = leaf.Hash
	}

	// Build tree bottom-up
	for len(hashes) > 1 {
		var nextLevel []string

		for i := 0; i < len(hashes); i += 2 {
			if i+1 < len(hashes) {
				// Combine two hashes
				combined := hashes[i] + hashes[i+1]
				nextLevel = append(nextLevel, hashData([]byte(combined)))
			} else {
				// Odd number, promote single hash
				nextLevel = append(nextLevel, hashes[i])
			}
		}

		hashes = nextLevel
	}

	return hashes[0]
}

func calculateDepth(leafCount int) int {
	depth := 0
	count := leafCount

	for count > 1 {
		count = (count + 1) / 2
		depth++
	}

	return depth
}
