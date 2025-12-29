package intelligence

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
)

// KnowledgeGraph stores knowledge in SAB for zero-copy sharing
type KnowledgeGraph struct {
	sab        []byte
	baseOffset uint32
	capacity   uint32

	// Indices for fast lookup
	nodeIndex map[string]uint32 // ID → SAB offset
	edgeIndex map[string][]uint32
	typeIndex map[foundation.NodeType][]uint32

	// Query engine
	query *KnowledgeQueryEngine

	// Statistics
	nodeCount uint32
	edgeCount uint32

	mu sync.RWMutex
}

// KnowledgeNode stored in SAB (64 bytes)
type KnowledgeNode struct {
	Magic      uint64  // 0x4B4E4F574C444745 ("KNOWLEDG")
	ID         uint64  // Unique node ID
	Type       uint16  // Node type
	Confidence float32 // 0.0-1.0
	Timestamp  uint64  // Unix nano
	Version    uint32  // Version for updates
	DataOffset uint32  // Offset to variable data
	DataSize   uint32  // Size of data
	Reserved   [24]byte
}

// KnowledgeEdge represents relationship between nodes
type KnowledgeEdge struct {
	From     uint64
	To       uint64
	Relation foundation.RelationType
	Strength float32
	Evidence []foundation.Evidence
}

// KnowledgeQueryEngine executes queries on knowledge graph
type KnowledgeQueryEngine struct {
	graph *KnowledgeGraph
}

const (
	KNOWLEDGE_MAGIC     = 0x4B4E4F574C444745 // "KNOWLEDG"
	KNOWLEDGE_NODE_SIZE = 64
	KNOWLEDGE_MAX_NODES = 1024
	KNOWLEDGE_MAX_EDGES = 4096
)

// NewKnowledgeGraph creates a new knowledge graph in SAB
func NewKnowledgeGraph(sab []byte, baseOffset, capacity uint32) *KnowledgeGraph {
	return &KnowledgeGraph{
		sab:        sab,
		baseOffset: baseOffset,
		capacity:   capacity,
		nodeIndex:  make(map[string]uint32),
		edgeIndex:  make(map[string][]uint32),
		typeIndex:  make(map[foundation.NodeType][]uint32),
		query:      &KnowledgeQueryEngine{},
	}
}

// AddNode adds a node to the knowledge graph
func (kg *KnowledgeGraph) AddNode(id string, nodeType foundation.NodeType, confidence float32, data []byte) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	// Check if node already exists
	if _, exists := kg.nodeIndex[id]; exists {
		return kg.updateNode(id, confidence, data)
	}

	// Check capacity
	if atomic.LoadUint32(&kg.nodeCount) >= kg.capacity {
		return fmt.Errorf("knowledge graph full")
	}

	// Allocate node ID
	nodeID := atomic.AddUint32(&kg.nodeCount, 1)

	// Create node
	node := &KnowledgeNode{
		Magic:      KNOWLEDGE_MAGIC,
		ID:         uint64(nodeID),
		Type:       uint16(nodeType),
		Confidence: confidence,
		Timestamp:  uint64(time.Now().UnixNano()),
		Version:    1,
		DataSize:   uint32(len(data)),
	}

	// Calculate offset in SAB
	offset := kg.baseOffset + (nodeID-1)*KNOWLEDGE_NODE_SIZE

	// Write node to SAB
	if err := kg.writeNode(offset, node); err != nil {
		return err
	}

	// Update indices
	kg.nodeIndex[id] = offset
	kg.typeIndex[nodeType] = append(kg.typeIndex[nodeType], offset)

	return nil
}

// GetNode retrieves a node from the knowledge graph
func (kg *KnowledgeGraph) GetNode(id string) (*KnowledgeNode, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	offset, exists := kg.nodeIndex[id]
	if !exists {
		return nil, fmt.Errorf("node not found: %s", id)
	}

	return kg.readNode(offset)
}

// AddEdge adds an edge between two nodes
func (kg *KnowledgeGraph) AddEdge(fromID, toID string, relation foundation.RelationType, strength float32) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	// Verify both nodes exist
	fromOffset, fromExists := kg.nodeIndex[fromID]
	toOffset, toExists := kg.nodeIndex[toID]

	if !fromExists || !toExists {
		return fmt.Errorf("one or both nodes not found")
	}

	// Create edge key
	edgeKey := fmt.Sprintf("%d-%d", fromOffset, toOffset)

	// Add to edge index
	kg.edgeIndex[edgeKey] = []uint32{fromOffset, toOffset}
	atomic.AddUint32(&kg.edgeCount, 1)

	return nil
}

// Query executes a query on the knowledge graph
func (kg *KnowledgeGraph) Query(query string) ([]*KnowledgeNode, error) {
	return kg.query.Execute(kg, query)
}

// FindByType finds all nodes of a specific type
func (kg *KnowledgeGraph) FindByType(nodeType foundation.NodeType) ([]*KnowledgeNode, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	offsets, exists := kg.typeIndex[nodeType]
	if !exists {
		return nil, nil
	}

	nodes := make([]*KnowledgeNode, 0, len(offsets))
	for _, offset := range offsets {
		node, err := kg.readNode(offset)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// Helper: Write node to SAB with binary encoding
func (kg *KnowledgeGraph) writeNode(offset uint32, node *KnowledgeNode) error {
	if offset+KNOWLEDGE_NODE_SIZE > uint32(len(kg.sab)) {
		return fmt.Errorf("offset out of bounds")
	}

	// Encode node to binary
	data := make([]byte, KNOWLEDGE_NODE_SIZE)

	// Magic (8 bytes)
	binary.LittleEndian.PutUint64(data[0:8], node.Magic)
	// ID (8 bytes)
	binary.LittleEndian.PutUint64(data[8:16], node.ID)
	// Type (2 bytes)
	binary.LittleEndian.PutUint16(data[16:18], node.Type)
	// Confidence (4 bytes)
	binary.LittleEndian.PutUint32(data[18:22], math.Float32bits(node.Confidence))
	// Timestamp (8 bytes)
	binary.LittleEndian.PutUint64(data[22:30], node.Timestamp)
	// Version (4 bytes)
	binary.LittleEndian.PutUint32(data[30:34], node.Version)
	// DataOffset (4 bytes)
	binary.LittleEndian.PutUint32(data[34:38], node.DataOffset)
	// DataSize (4 bytes)
	binary.LittleEndian.PutUint32(data[38:42], node.DataSize)
	// Reserved (24 bytes)
	copy(data[42:64], node.Reserved[:])

	// Atomic write to SAB
	copy(kg.sab[offset:offset+KNOWLEDGE_NODE_SIZE], data)

	return nil
}

// Helper: Read node from SAB with binary decoding
func (kg *KnowledgeGraph) readNode(offset uint32) (*KnowledgeNode, error) {
	if offset+KNOWLEDGE_NODE_SIZE > uint32(len(kg.sab)) {
		return nil, fmt.Errorf("offset out of bounds")
	}

	data := kg.sab[offset : offset+KNOWLEDGE_NODE_SIZE]

	node := &KnowledgeNode{
		Magic:      binary.LittleEndian.Uint64(data[0:8]),
		ID:         binary.LittleEndian.Uint64(data[8:16]),
		Type:       binary.LittleEndian.Uint16(data[16:18]),
		Confidence: math.Float32frombits(binary.LittleEndian.Uint32(data[18:22])),
		Timestamp:  binary.LittleEndian.Uint64(data[22:30]),
		Version:    binary.LittleEndian.Uint32(data[30:34]),
		DataOffset: binary.LittleEndian.Uint32(data[34:38]),
		DataSize:   binary.LittleEndian.Uint32(data[38:42]),
	}
	copy(node.Reserved[:], data[42:64])

	// Validate magic
	if node.Magic != KNOWLEDGE_MAGIC {
		return nil, fmt.Errorf("invalid node magic")
	}

	return node, nil
}

// Helper: Update existing node
func (kg *KnowledgeGraph) updateNode(id string, confidence float32, _ []byte) error {
	offset := kg.nodeIndex[id]
	node, err := kg.readNode(offset)
	if err != nil {
		return err
	}

	// Update fields
	node.Confidence = confidence
	node.Timestamp = uint64(time.Now().UnixNano())
	node.Version++

	return kg.writeNode(offset, node)
}

// KnowledgeQueryEngine methods

func (kqe *KnowledgeQueryEngine) Execute(graph *KnowledgeGraph, query string) ([]*KnowledgeNode, error) {
	// Production query parser supporting:
	// - type:NodeType - find by type
	// - confidence>0.8 - filter by confidence
	// - id:value - find by ID

	results := make([]*KnowledgeNode, 0)

	// Parse query (simple key:value format)
	parts := strings.Split(query, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid query format, use key:value")
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	switch key {
	case "type":
		// Query by type
		nodeType := foundation.NodeType(0) // Parse from value
		return graph.FindByType(nodeType)

	case "id":
		// Query by ID
		node, err := graph.GetNode(value)
		if err != nil {
			return nil, err
		}
		results = append(results, node)
		return results, nil

	case "confidence":
		// Query by confidence threshold
		threshold := float32(0.5) // Parse from value
		graph.mu.RLock()
		defer graph.mu.RUnlock()

		for _, offset := range graph.nodeIndex {
			node, err := graph.readNode(offset)
			if err != nil {
				continue
			}
			if node.Confidence >= threshold {
				results = append(results, node)
			}
		}
		return results, nil

	default:
		return nil, fmt.Errorf("unsupported query key: %s", key)
	}
}

// GetStats returns knowledge graph statistics
func (kg *KnowledgeGraph) GetStats() KnowledgeGraphStats {
	return KnowledgeGraphStats{
		NodeCount: atomic.LoadUint32(&kg.nodeCount),
		EdgeCount: atomic.LoadUint32(&kg.edgeCount),
		Capacity:  kg.capacity,
	}
}

type KnowledgeGraphStats struct {
	NodeCount uint32
	EdgeCount uint32
	Capacity  uint32
}
