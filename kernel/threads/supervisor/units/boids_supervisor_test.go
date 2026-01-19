package units

import (
	"context"
	"encoding/binary"
	"testing"

	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
)

// MockSABBridge for testing
type MockSABBridge struct {
	data map[uint32][]byte
}

func NewMockSABBridge() *MockSABBridge {
	return &MockSABBridge{
		data: make(map[uint32][]byte),
	}
}

func (m *MockSABBridge) ReadRaw(offset, size uint32) ([]byte, error) {
	if data, ok := m.data[offset]; ok && uint32(len(data)) >= size {
		return data[:size], nil
	}
	// Return zeroed buffer if not found
	return make([]byte, size), nil
}

func (m *MockSABBridge) ReadAt(offset uint32, dest []byte) error {
	size := uint32(len(dest))
	if data, ok := m.data[offset]; ok && uint32(len(data)) >= size {
		copy(dest, data[:size])
		return nil
	}
	// Zero out if not found (or partial)
	for i := range dest {
		dest[i] = 0
	}
	return nil
}

func (m *MockSABBridge) WaitForEpochAsync(epochIndex uint32, expectedValue int32) <-chan struct{} {
	ch := make(chan struct{})
	// For tests, return immediately to simulate signal
	// Or we could check m.data but simple is better for unit tests
	close(ch)
	return ch
}

func (m *MockSABBridge) WriteRaw(offset uint32, data []byte) error {
	m.data[offset] = make([]byte, len(data))
	copy(m.data[offset], data)
	return nil
}

func (m *MockSABBridge) SignalInbox() {
	// No-op
}

func (m *MockSABBridge) RegisterJob(jobID string) chan *foundation.Result {
	return make(chan *foundation.Result, 1)
}

func (m *MockSABBridge) ResolveJob(jobID string, result *foundation.Result) {
	// No-op for testing
}

func (m *MockSABBridge) WriteJob(job *foundation.Job) error {
	return nil
}

type MockMeshDelegator struct{}

func (m *MockMeshDelegator) DelegateJob(ctx context.Context, job *foundation.Job) (*foundation.Result, error) {
	return &foundation.Result{JobID: job.ID, Success: true}, nil
}

func (m *MockSABBridge) SignalEpoch(index uint32) {
	// Simple implementation for testing: increment value at correct SAB offset
	offset := uint32(sab_layout.OFFSET_ATOMIC_FLAGS + index*4)
	current := m.ReadAtomicI32(index)
	newData := make([]byte, 4)
	binary.LittleEndian.PutUint32(newData, uint32(current+1))
	m.data[offset] = newData
}

func (m *MockSABBridge) ReadAtomicI32(index uint32) int32 {
	offset := uint32(sab_layout.OFFSET_ATOMIC_FLAGS + index*4)
	if data, ok := m.data[offset]; ok && len(data) >= 4 {
		return int32(binary.LittleEndian.Uint32(data[:4]))
	}
	return 0
}

func (m *MockSABBridge) IsReady() bool {
	return true // Always ready in tests
}

func (m *MockSABBridge) WriteResult(result *foundation.Result) error {
	// No-op for testing - store if needed for verification
	return nil
}

func (m *MockSABBridge) ReadResult() (*foundation.Result, error) {
	return nil, nil
}

func TestNewBoidsSupervisor(t *testing.T) {
	bridge := NewMockSABBridge()
	// Create dummy SAB for patterns/knowledge
	sabSize := uint32(1024)
	dummySAB := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&dummySAB[0])
	patterns := pattern.NewTieredPatternStorage(sabPtr, sabSize, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(sabPtr, sabSize, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil, nil, &MockMeshDelegator{})

	if supervisor == nil {
		t.Fatal("Expected supervisor to be created")
	}

	if supervisor.mutationRate != DefaultMutationRate {
		t.Errorf("Expected mutationRate %.2f, got %.2f", DefaultMutationRate, supervisor.mutationRate)
	}
}

func TestSetBirdCount(t *testing.T) {
	bridge := NewMockSABBridge()
	sabSize := uint32(1024)
	dummySAB := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&dummySAB[0])
	patterns := pattern.NewTieredPatternStorage(sabPtr, sabSize, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(sabPtr, sabSize, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil, nil, &MockMeshDelegator{})

	// Test normal count
	supervisor.SetBirdCount(12)
	if supervisor.birdCount != 12 {
		t.Errorf("Expected birdCount 12, got %d", supervisor.birdCount)
	}

	// Test exceeding max
	supervisor.SetBirdCount(12000)
	if supervisor.birdCount != MaxBirds {
		t.Errorf("Expected birdCount capped at %d, got %d", MaxBirds, supervisor.birdCount)
	}
}

func TestSetMeshNodes(t *testing.T) {
	bridge := NewMockSABBridge()
	sabSize := uint32(1024)
	dummySAB := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&dummySAB[0])
	patterns := pattern.NewTieredPatternStorage(sabPtr, sabSize, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(sabPtr, sabSize, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil, nil, &MockMeshDelegator{})

	supervisor.SetMeshNodes(0)
	if supervisor.meshNodesActive != 0 {
		t.Errorf("Expected meshNodesActive 0, got %d", supervisor.meshNodesActive)
	}

	supervisor.SetMeshNodes(10)
	if supervisor.meshNodesActive != 10 {
		t.Errorf("Expected meshNodesActive 10, got %d", supervisor.meshNodesActive)
	}
}

func TestTournamentSelect(t *testing.T) {
	bridge := NewMockSABBridge()
	sabSize := uint32(1024)
	dummySAB := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&dummySAB[0])
	patterns := pattern.NewTieredPatternStorage(sabPtr, sabSize, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(sabPtr, sabSize, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil, nil, &MockMeshDelegator{})

	population := []BirdGenes{
		{Fitness: 10.0, BirdID: 0},
		{Fitness: 50.0, BirdID: 1},
		{Fitness: 30.0, BirdID: 2},
		{Fitness: 20.0, BirdID: 3},
	}

	// Run tournament selection many times
	selections := make(map[int]int)
	for i := 0; i < 1000; i++ {
		selected := supervisor.TournamentSelect(population)
		selections[selected.BirdID]++
	}

	// Bird with highest fitness (ID 1) should be selected most often
	if selections[1] < selections[0] {
		t.Error("Expected higher fitness bird to be selected more often")
	}
}

func TestCrossover(t *testing.T) {
	bridge := NewMockSABBridge()
	sabSize := uint32(1024)
	dummySAB := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&dummySAB[0])
	patterns := pattern.NewTieredPatternStorage(sabPtr, sabSize, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(sabPtr, sabSize, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil, nil, &MockMeshDelegator{})

	parent1 := BirdGenes{}
	parent2 := BirdGenes{}

	for i := 0; i < 44; i++ {
		parent1.Weights[i] = 1.0
		parent2.Weights[i] = -1.0
	}

	child := supervisor.Crossover(parent1, parent2)

	// Child should have mix of parent genes
	hasParent1Genes := false
	hasParent2Genes := false

	for i := 0; i < 44; i++ {
		if child.Weights[i] == 1.0 {
			hasParent1Genes = true
		}
		if child.Weights[i] == -1.0 {
			hasParent2Genes = true
		}
	}

	if !hasParent1Genes || !hasParent2Genes {
		t.Error("Expected child to have genes from both parents")
	}
}

func TestMutate(t *testing.T) {
	bridge := NewMockSABBridge()
	sabSize := uint32(1024)
	dummySAB := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&dummySAB[0])
	patterns := pattern.NewTieredPatternStorage(sabPtr, sabSize, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(sabPtr, sabSize, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil, nil, &MockMeshDelegator{})

	original := BirdGenes{}
	for i := 0; i < 44; i++ {
		original.Weights[i] = 2.0
	}

	mutated := supervisor.Mutate(original)

	// At least some genes should have mutated
	mutationCount := 0
	for i := 0; i < 44; i++ {
		if mutated.Weights[i] != original.Weights[i] {
			mutationCount++
		}

		// Check clamping
		if mutated.Weights[i] > 5.0 || mutated.Weights[i] < -5.0 {
			t.Errorf("Gene %d exceeds clamping bounds: %f", i, mutated.Weights[i])
		}
	}

	// With default mutation rate, expect some mutations
	if mutationCount == 0 {
		t.Error("Expected at least some genes to mutate")
	}
}

func TestSortByFitness(t *testing.T) {
	population := []BirdGenes{
		{Fitness: 10.0, BirdID: 0},
		{Fitness: 50.0, BirdID: 1},
		{Fitness: 30.0, BirdID: 2},
		{Fitness: 20.0, BirdID: 3},
	}

	SortByFitness(population)

	// Should be sorted descending
	if population[0].Fitness != 50.0 {
		t.Errorf("Expected highest fitness first, got %.1f", population[0].Fitness)
	}

	if population[len(population)-1].Fitness != 10.0 {
		t.Errorf("Expected lowest fitness last, got %.1f", population[len(population)-1].Fitness)
	}

	// Verify sorted order
	for i := 0; i < len(population)-1; i++ {
		if population[i].Fitness < population[i+1].Fitness {
			t.Error("Population not properly sorted by fitness")
		}
	}
}

func TestSignalEpoch(t *testing.T) {
	bridge := NewMockSABBridge()
	sabSize := uint32(1024)
	dummySAB := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&dummySAB[0])
	patterns := pattern.NewTieredPatternStorage(sabPtr, sabSize, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(sabPtr, sabSize, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil, nil, &MockMeshDelegator{})

	// Mock initial epoch
	epochOffset := uint32(sab_layout.OFFSET_ATOMIC_FLAGS + sab_layout.IDX_EVOLUTION_EPOCH*4)
	initialEpoch := uint32(64)
	epochBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(epochBytes, initialEpoch)
	bridge.WriteRaw(epochOffset, epochBytes)

	// Signal epoch
	supervisor.SignalEpoch()

	// Check if incremented
	newBytes, _ := bridge.ReadRaw(epochOffset, 4)
	newEpoch := binary.LittleEndian.Uint32(newBytes)

	if newEpoch != initialEpoch+1 {
		t.Errorf("Expected epoch %d, got %d", initialEpoch+1, newEpoch)
	}
}

func TestMeshLearningBoost(t *testing.T) {
	bridge := NewMockSABBridge()
	sabSize := uint32(1024)
	dummySAB := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&dummySAB[0])
	patterns := pattern.NewTieredPatternStorage(sabPtr, sabSize, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(sabPtr, sabSize, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil, nil, &MockMeshDelegator{})

	original := BirdGenes{}
	for i := 0; i < 44; i++ {
		original.Weights[i] = 0.0
	}

	// Collect stats over many runs
	iterations := 1000
	mutationsNoMesh := 0
	mutationsWithMesh := 0

	// 1. Run with 0 mesh nodes
	supervisor.SetMeshNodes(0)
	for i := 0; i < iterations; i++ {
		mutated := supervisor.Mutate(original)
		mutationsNoMesh += countMutations(original, mutated)
	}

	// 2. Run with 100 mesh nodes
	supervisor.SetMeshNodes(100)
	for i := 0; i < iterations; i++ {
		mutated := supervisor.Mutate(original)
		mutationsWithMesh += countMutations(original, mutated)
	}

	// More mesh nodes -> lower mutation rate (stability)
	// Just verify that mutationsWithMesh < mutationsNoMesh
	if mutationsWithMesh >= mutationsNoMesh {
		t.Errorf("Expected mesh boost to reduce mutations (stability). NoMesh: %d, WithMesh: %d", mutationsNoMesh, mutationsWithMesh)
	}
}

func countMutations(original, mutated BirdGenes) int {
	count := 0
	for i := 0; i < 44; i++ {
		if original.Weights[i] != mutated.Weights[i] {
			count++
		}
	}
	return count
}
