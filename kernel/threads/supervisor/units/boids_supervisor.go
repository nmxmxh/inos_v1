package units

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
)

const (
	// SAB layout constants
	BoidSABOffset = 0x400000 // 4MB offset for boid data
	FloatsPerBird = 58       // position(3) + velocity(3) + rotation(4) + angular(1) + wings(3) + fitness(1) + weights(44)
	BytesPerBird  = FloatsPerBird * 4
	MaxBirds      = 100

	// Evolution parameters
	DefaultMutationRate   = 0.1
	DefaultCrossoverRate  = 0.7
	DefaultTournamentSize = 3
	EvolutionInterval     = 5 * time.Second // Evolve every 5 seconds
)

// BirdGenes represents the neural network weights for a bird
type BirdGenes struct {
	Weights [44]float32 // 8x4 + 4x3 = 44 weights
	Fitness float64
	BirdID  int
}

// SABInterface defines the methods needed from the bridge
type SABInterface interface {
	ReadRaw(offset uint32, size uint32) ([]byte, error)
	WriteRaw(offset uint32, data []byte) error
	SignalInbox()
}

// BoidsSupervisor manages distributed learning for bird simulation
// Executes genetic algorithm, coordinates P2P learning, signals epochs
type BoidsSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge SABInterface

	// Learning configuration
	mu             sync.RWMutex
	birdCount      int
	generation     int
	mutationRate   float64
	crossoverRate  float64
	tournamentSize int

	// SAB offsets
	boidDataOffset  int // 0x400000
	epochFlagOffset int // IDX_BIRD_EPOCH

	// Evolution state
	lastEvolutionTime time.Time
	evolutionInterval time.Duration

	// P2P mesh boost
	meshNodesActive int

	// Internal fields for tests or logic
	sabOffset uint32
}

// NewBoidsSupervisor creates a supervisor for learning birds
func NewBoidsSupervisor(bridge SABInterface, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string) *BoidsSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{
			"boids.init_population",
			"boids.step_physics",
			"boids.step_learning",
			"boids.evolve_generation",
			"boids.forward_pass",
		}
	}

	return &BoidsSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("boids", capabilities, patterns, knowledge),
		bridge:            bridge,
		boidDataOffset:    BoidSABOffset,
		sabOffset:         uint32(BoidSABOffset),
		epochFlagOffset:   8, // IDX_BIRD_EPOCH (system flag index 8)
		mutationRate:      DefaultMutationRate,
		crossoverRate:     DefaultCrossoverRate,
		tournamentSize:    DefaultTournamentSize,
		evolutionInterval: EvolutionInterval,
		lastEvolutionTime: time.Now(),
	}
}

// Start begins the learning supervision loop
func (s *BoidsSupervisor) Start(ctx context.Context) error {
	// Start unified supervisor
	if err := s.UnifiedSupervisor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start unified supervisor: %w", err)
	}

	// Start learning loop
	go s.learningLoop(ctx)

	s.Log("INFO", "Boids supervisor started")
	return nil
}

// learningLoop monitors SAB and executes genetic algorithm
func (s *BoidsSupervisor) learningLoop(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond) // Check every 100ms
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			s.checkEvolution()
		}
	}
}

// Log logs a message
func (s *BoidsSupervisor) Log(level, msg string) {
	fmt.Printf("[%s] boids: %s\n", level, msg)
}

// SetBirdCount updates the active bird count
func (s *BoidsSupervisor) SetBirdCount(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if count > MaxBirds {
		count = MaxBirds
	}

	s.birdCount = count
	s.Log("INFO", fmt.Sprintf("Bird count set to %d", count))
}

// SetMeshNodes updates the P2P mesh node count for learning boost
func (s *BoidsSupervisor) SetMeshNodes(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.meshNodesActive = count
	boost := 1.0 + math.Log2(float64(count+1))*0.5

	s.Log("INFO", fmt.Sprintf("Mesh nodes: %d, learning boost: %.2fx", count, boost))
}

// checkEvolution determines if it's time to evolve and executes genetic algorithm
func (s *BoidsSupervisor) checkEvolution() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check conditions
	if s.birdCount == 0 {
		return // Nothing to do
	}

	if time.Since(s.lastEvolutionTime) < s.evolutionInterval {
		return // Not time yet
	}

	// Execute genetic algorithm
	if err := s.evolveGeneration(); err != nil {
		s.Log("ERROR", fmt.Sprintf("Evolution failed: %v", err))
		return
	}

	s.generation++
	s.lastEvolutionTime = time.Now()

	// Signal epoch update to frontend
	s.SignalEpoch()
}

// evolveGeneration executes genetic algorithm on bird population in SAB
func (s *BoidsSupervisor) evolveGeneration() error {
	// 1. Read current population from SAB
	population, err := s.ReadPopulation()
	if err != nil {
		return fmt.Errorf("failed to read population: %w", err)
	}

	// Sort by fitness (descending)
	SortByFitness(population)

	// Log best fitness
	if len(population) > 0 {
		s.Log("INFO", fmt.Sprintf("Best fitness: %.2f (bird %d)", population[0].Fitness, population[0].BirdID))
	}

	// Selection: keep top 25%
	survivalCount := max(2, s.birdCount/4)
	survivors := population[:survivalCount]

	// Generate new population through crossover + mutation
	newPopulation := make([]BirdGenes, s.birdCount)

	for i := 0; i < s.birdCount; i++ {
		if i < len(survivors) {
			// Keep elites
			newPopulation[i] = survivors[i]
		} else {
			// Breed new individual
			parent1 := s.TournamentSelect(survivors)
			parent2 := s.TournamentSelect(survivors)

			child := s.Crossover(parent1, parent2)
			child = s.Mutate(child)
			child.BirdID = i
			child.Fitness = 0 // Reset fitness for new generation

			newPopulation[i] = child
		}
	}

	// Write new population back to SAB
	if err := s.WritePopulation(newPopulation); err != nil {
		return fmt.Errorf("failed to write population: %w", err)
	}

	return nil
}

// TournamentSelect selects a parent using tournament selection
func (s *BoidsSupervisor) TournamentSelect(population []BirdGenes) BirdGenes {
	best := population[rand.Intn(len(population))]

	for i := 1; i < s.tournamentSize; i++ {
		candidate := population[rand.Intn(len(population))]
		if candidate.Fitness > best.Fitness {
			best = candidate
		}
	}

	return best
}

// Crossover performs uniform crossover between two parents
func (s *BoidsSupervisor) Crossover(parent1, parent2 BirdGenes) BirdGenes {
	var child BirdGenes

	for i := 0; i < 44; i++ {
		if rand.Float64() < s.crossoverRate {
			child.Weights[i] = parent1.Weights[i]
		} else {
			child.Weights[i] = parent2.Weights[i]
		}
	}

	return child
}

// Mutate applies Gaussian mutation to genes
func (s *BoidsSupervisor) Mutate(genes BirdGenes) BirdGenes {
	// Adjust mutation rate based on mesh nodes (more nodes = less mutation)
	effectiveMutationRate := s.mutationRate / (1.0 + math.Log2(float64(s.meshNodesActive+1)))

	for i := 0; i < 44; i++ {
		if rand.Float64() < effectiveMutationRate {
			// Gaussian noise
			genes.Weights[i] += float32(rand.NormFloat64() * 0.2)

			// Clamp to reasonable range
			if genes.Weights[i] > 5.0 {
				genes.Weights[i] = 5.0
			} else if genes.Weights[i] < -5.0 {
				genes.Weights[i] = -5.0
			}
		}
	}

	return genes
}

// ReadPopulation reads all bird genes from SAB
func (s *BoidsSupervisor) ReadPopulation() ([]BirdGenes, error) {
	population := make([]BirdGenes, s.birdCount)

	for i := 0; i < s.birdCount; i++ {
		offset := s.boidDataOffset + i*BytesPerBird

		// Read fitness (offset 14 floats into bird data)
		fitnessOffset := uint32(offset + 14*4)
		fitnessBytes, err := s.bridge.ReadRaw(fitnessOffset, 4)
		if err != nil {
			return nil, fmt.Errorf("failed to read fitness at offset %d: %w", fitnessOffset, err)
		}
		fitness := math.Float32frombits(binary.LittleEndian.Uint32(fitnessBytes))

		// Read weights (offset 15-58 floats)
		weightsOffset := uint32(offset + 15*4)
		weightsBytes, err := s.bridge.ReadRaw(weightsOffset, 44*4)
		if err != nil {
			return nil, fmt.Errorf("failed to read weights at offset %d: %w", weightsOffset, err)
		}

		var weights [44]float32
		for j := 0; j < 44; j++ {
			weights[j] = math.Float32frombits(binary.LittleEndian.Uint32(weightsBytes[j*4 : (j+1)*4]))
		}

		population[i] = BirdGenes{
			Weights: weights,
			Fitness: float64(fitness), // Convert float32 to float64
			BirdID:  i,
		}
	}

	return population, nil
}

// WritePopulation writes evolved genes back to SAB
func (s *BoidsSupervisor) WritePopulation(population []BirdGenes) error {
	for i, bird := range population {
		offset := i * 58 * 4

		// Correct global offset logic
		globalOffset := uint32(s.boidDataOffset + offset)

		// Write fitness (index 14)
		fitnessBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(fitnessBytes, math.Float32bits(float32(bird.Fitness)))
		if err := s.bridge.WriteRaw(globalOffset+14*4, fitnessBytes); err != nil {
			return fmt.Errorf("failed to write fitness for bird %d: %w", i, err)
		}

		// Write weights (index 15-58)
		weightsBytes := make([]byte, 44*4)
		for w := 0; w < 44; w++ {
			bits := math.Float32bits(bird.Weights[w])
			binary.LittleEndian.PutUint32(weightsBytes[w*4:], bits)
		}

		if err := s.bridge.WriteRaw(globalOffset+15*4, weightsBytes); err != nil {
			return fmt.Errorf("failed to write weights for bird %d: %w", i, err)
		}
	}

	return nil
}

// SignalEpoch increments the epoch flag to trigger frontend reactivity
func (s *BoidsSupervisor) SignalEpoch() {
	// Standardized Global Epoch at 0x000000
	offset := uint32(0)

	// Read current epoch from offset
	epochBytes, err := s.bridge.ReadRaw(offset, 4)
	if err != nil {
		s.Log("ERROR", fmt.Sprintf("Failed to read epoch: %v", err))
		return
	}
	currentEpoch := binary.LittleEndian.Uint32(epochBytes)

	// Increment
	newEpoch := currentEpoch + 1
	newBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(newBytes, newEpoch)

	// Write back
	if err := s.bridge.WriteRaw(offset, newBytes); err != nil {
		s.Log("ERROR", fmt.Sprintf("Failed to write epoch: %v", err))
		return
	}

	s.Log("DEBUG", fmt.Sprintf("Epoch signaled: %d -> %d", currentEpoch, newEpoch))
}

// SortByFitness sorts bird population by fitness descending
func SortByFitness(population []BirdGenes) {
	sort.Slice(population, func(i, j int) bool {
		return population[i].Fitness > population[j].Fitness
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
