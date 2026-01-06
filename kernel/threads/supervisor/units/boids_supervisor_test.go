package units

import (
	"encoding/binary"
	"testing"

	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
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

func (m *MockSABBridge) WriteRaw(offset uint32, data []byte) error {
	m.data[offset] = make([]byte, len(data))
	copy(m.data[offset], data)
	return nil
}

func (m *MockSABBridge) SignalInbox() {
	// No-op
}

func TestNewBoidsSupervisor(t *testing.T) {
	bridge := NewMockSABBridge()
	// Create dummy SAB for patterns/knowledge
	dummySAB := make([]byte, 1024)
	patterns := pattern.NewTieredPatternStorage(dummySAB, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(dummySAB, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil)

	if supervisor == nil {
		t.Fatal("Expected supervisor to be created")
	}

	if supervisor.mutationRate != DefaultMutationRate {
		t.Errorf("Expected mutationRate %.2f, got %.2f", DefaultMutationRate, supervisor.mutationRate)
	}
}

func TestSetBirdCount(t *testing.T) {
	bridge := NewMockSABBridge()
	dummySAB := make([]byte, 1024)
	patterns := pattern.NewTieredPatternStorage(dummySAB, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(dummySAB, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil)

	// Test normal count
	supervisor.SetBirdCount(12)
	if supervisor.birdCount != 12 {
		t.Errorf("Expected birdCount 12, got %d", supervisor.birdCount)
	}

	// Test exceeding max
	supervisor.SetBirdCount(3000)
	if supervisor.birdCount != MaxBirds {
		t.Errorf("Expected birdCount capped at %d, got %d", MaxBirds, supervisor.birdCount)
	}
}

func TestSetMeshNodes(t *testing.T) {
	bridge := NewMockSABBridge()
	dummySAB := make([]byte, 1024)
	patterns := pattern.NewTieredPatternStorage(dummySAB, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(dummySAB, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil)

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
	dummySAB := make([]byte, 1024)
	patterns := pattern.NewTieredPatternStorage(dummySAB, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(dummySAB, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil)

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
	dummySAB := make([]byte, 1024)
	patterns := pattern.NewTieredPatternStorage(dummySAB, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(dummySAB, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil)

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
	dummySAB := make([]byte, 1024)
	patterns := pattern.NewTieredPatternStorage(dummySAB, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(dummySAB, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil)

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
	dummySAB := make([]byte, 1024)
	patterns := pattern.NewTieredPatternStorage(dummySAB, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(dummySAB, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil)

	// Mock initial epoch at index 0 (byte offset 0)
	epochOffset := uint32(0)
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
	dummySAB := make([]byte, 1024)
	patterns := pattern.NewTieredPatternStorage(dummySAB, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(dummySAB, 0, 1024)

	supervisor := NewBoidsSupervisor(bridge, patterns, knowledge, nil)

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
