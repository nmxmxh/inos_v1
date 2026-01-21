package intelligence

import (
	"fmt"
	"sync"
	"testing"

	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKnowledgeGraph_BasicOps(t *testing.T) {
	// Setup SAB (1MB)
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)
	kg := NewKnowledgeGraph(unsafe.Pointer(&sab[0]), sabSize, 0, 100)

	// Add Node
	data := []byte("test-data")
	err := kg.AddNode("node1", foundation.NodeTypePattern, 0.9, data)
	require.NoError(t, err)

	// Get Node
	node, err := kg.GetNode("node1")
	require.NoError(t, err)
	t.Logf("Node Details: ID=%d, Type=%d, Confidence=%f, Magic=%x", node.ID, node.Type, node.Confidence, node.Magic)
	assert.Equal(t, uint64(KNOWLEDGE_MAGIC), node.Magic)
	assert.Equal(t, uint64(1), node.ID)
	assert.Equal(t, uint16(foundation.NodeTypePattern), node.Type)
	assert.Equal(t, float32(0.9), node.Confidence)
	assert.Equal(t, uint32(len(data)), node.DataSize)

	// Update Node
	err = kg.AddNode("node1", foundation.NodeTypePattern, 0.95, data) // Update conf
	require.NoError(t, err)

	nodeUpdated, err := kg.GetNode("node1")
	require.NoError(t, err)
	assert.Equal(t, float32(0.95), nodeUpdated.Confidence)
	assert.Equal(t, node.Version+1, nodeUpdated.Version)
}

func TestKnowledgeGraph_Edges(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)
	kg := NewKnowledgeGraph(unsafe.Pointer(&sab[0]), sabSize, 0, 100)

	// Add Nodes
	_ = kg.AddNode("nodeA", foundation.NodeTypePattern, 0.8, nil)
	_ = kg.AddNode("nodeB", foundation.NodeTypeMetric, 0.9, nil)

	// Add Edge
	err := kg.AddEdge("nodeA", "nodeB", foundation.RelationCauses, 0.75)
	assert.NoError(t, err)

	// Add Invalid Edge
	err = kg.AddEdge("nodeA", "nodeMissing", foundation.RelationCauses, 0.5)
	assert.Error(t, err)
}

func TestKnowledgeGraph_Query(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)
	kg := NewKnowledgeGraph(unsafe.Pointer(&sab[0]), sabSize, 0, 100)

	// Add mixed nodes
	_ = kg.AddNode("p1", foundation.NodeTypePattern, 0.9, nil)
	_ = kg.AddNode("p2", foundation.NodeTypePattern, 0.4, nil)
	_ = kg.AddNode("m1", foundation.NodeTypeMetric, 0.8, nil)

	// Query by Type
	results, err := kg.Query(fmt.Sprintf("type:%d", foundation.NodeTypePattern))
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Query by ID
	results, err = kg.Query("id:m1")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, float32(0.8), results[0].Confidence)

	// Query by Confidence (>0.5)
	// Currently Query engine "confidence" parser uses hardcoded 0.5 threshold if not parsed?
	// The implementation has `threshold := float32(0.5) // Parse from value` comment.
	// But let's check if it actually parses or just ignores value.
	// Code: `threshold := float32(0.5)` - it seems hardcoded in current impl!
	// We test what exists.
	results, err = kg.Query("confidence:0.8") // Value currently ignored by impl
	require.NoError(t, err)
	// Should return p1(0.9) and m1(0.8). p2(0.4) is below 0.5 default.
	assert.Len(t, results, 2)
}

func TestKnowledgeGraph_Concurrency(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)
	kg := NewKnowledgeGraph(unsafe.Pointer(&sab[0]), sabSize, 0, 1000)

	var wg sync.WaitGroup
	workers := 10
	iterations := 50

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				nodeID := fmt.Sprintf("node_%d_%d", id, j)
				err := kg.AddNode(nodeID, foundation.NodeTypePattern, 0.5, nil)
				assert.NoError(t, err)

				// Read back occasionally
				if j%5 == 0 {
					_, err := kg.GetNode(nodeID)
					assert.NoError(t, err)
				}
			}
		}(i)
	}

	wg.Wait()

	stats := kg.GetStats()
	assert.Equal(t, uint32(workers*iterations), stats.NodeCount)
}

func TestKnowledgeGraph_Persistence(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)

	// Phase 1: Create and Populate
	{
		kg1 := NewKnowledgeGraph(unsafe.Pointer(&sab[0]), sabSize, 0, 100)
		err := kg1.AddNode("persist_node", foundation.NodeTypeRule, 0.99, []byte("rules"))
		require.NoError(t, err)
	}

	// Phase 2: "Reboot" - New KG instance on same SAB
	{
		kg2 := NewKnowledgeGraph(unsafe.Pointer(&sab[0]), sabSize, 0, 100)
		// Manually populate index for existing node (since we don't have LoadFromSAB yet)
		kg2.nodeIndex["persist_node"] = 0

		node, err := kg2.GetNode("persist_node")
		require.NoError(t, err)
		assert.Equal(t, uint64(KNOWLEDGE_MAGIC), node.Magic)
		assert.Equal(t, uint64(1), node.ID)
	}
}

func TestKnowledgeGraph_Errors(t *testing.T) {
	// 1. Capacity Error
	{
		sabSize := uint32(1024 * 1024)
		sab := make([]byte, sabSize)
		kg := NewKnowledgeGraph(unsafe.Pointer(&sab[0]), sabSize, 0, 2) // Cap 2

		err := kg.AddNode("n1", 0, 0.5, nil)
		require.NoError(t, err)
		err = kg.AddNode("n2", 0, 0.5, nil)
		require.NoError(t, err)

		err = kg.AddNode("n3", 0, 0.5, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "full")
	}

	// 2. Query Errors
	{
		sabSize := uint32(1024 * 1024)
		sab := make([]byte, sabSize)
		kg := NewKnowledgeGraph(unsafe.Pointer(&sab[0]), sabSize, 0, 10)

		// Invalid format
		_, err := kg.Query("invalidformat")
		assert.Error(t, err)

		// Unsupported key
		_, err = kg.Query("weird:value")
		assert.Error(t, err)

		// Non-existent ID
		_, err = kg.Query("id:missing")
		assert.Error(t, err)
	}

	// 3. Corrupted Data (Magic Mismatch)
	{
		sabSize := uint32(1024 * 1024)
		sab := make([]byte, sabSize)
		kg := NewKnowledgeGraph(unsafe.Pointer(&sab[0]), sabSize, 0, 10)

		err := kg.AddNode("nodeX", 0, 0.5, nil)
		require.NoError(t, err)

		// Corrupt magic at offset 0
		sab[0] = 0xFF

		_, err = kg.GetNode("nodeX")
		assert.Error(t, err)
		// Capnp unmarshal error might be different, but it should fail
	}
}
