package units

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/nmxmxh/inos_v1/kernel/utils"
)

const (
	// SAB layout constants
	BoidSABOffset = 0x400000 // 4MB offset for boid data
	FloatsPerBird = 59       // position(3) + velocity(3) + rotation(4) + angular(1) + wings(3) + fitness(1) + weights(44)
	BytesPerBird  = FloatsPerBird * 4
	MaxBirds      = 2048

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

	utils.Info("Boids supervisor started")
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

// SetBirdCount updates the active bird count
func (s *BoidsSupervisor) SetBirdCount(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if count > MaxBirds {
		count = MaxBirds
	}

	s.birdCount = count
	utils.Info("Bird count updated", utils.Int("count", count))
}

// SetMeshNodes updates the P2P mesh node count for learning boost
func (s *BoidsSupervisor) SetMeshNodes(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.meshNodesActive = count
	boost := 1.0 + math.Log2(float64(count+1))*0.5

	utils.Info("Mesh nodes updated", utils.Int("count", count), utils.Float64("boost", boost))
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

	// Log evolution start
	utils.Info("Starting Evolution Cycle", utils.Int("gen", s.generation+1))
	startTime := time.Now()

	// Execute genetic algorithm
	if err := s.evolveGeneration(); err != nil {
		utils.Error("Evolution failed", utils.Err(err))
		return
	}

	s.generation++
	s.lastEvolutionTime = time.Now()
	duration := time.Since(startTime)

	// Log evolution complete
	utils.Info("Evolution Cycle Complete",
		utils.Int("gen", s.generation),
		utils.Duration("took", duration))

	// Signal epoch update to frontend
	s.SignalEpoch()
}

// evolveGeneration executes genetic algorithm on bird population in SAB
func (s *BoidsSupervisor) evolveGeneration() error {
	utils.Debug("Reading population from SAB", utils.Int("count", s.birdCount))

	// 1. Read current population from SAB
	population, err := s.ReadPopulation()
	if err != nil {
		return fmt.Errorf("failed to read population: %w", err)
	}

	// Sort by fitness (descending)
	SortByFitness(population)

	// Calculate statistics
	avgFit := avgFitness(population)
	minFit := population[len(population)-1].Fitness
	maxFit := population[0].Fitness

	// Log fitness statistics
	utils.Info("Generation Statistics",
		utils.Int("gen", s.generation),
		utils.Float64("best", maxFit),
		utils.Float64("avg", avgFit),
		utils.Float64("min", minFit))

	// Selection: keep top 25%
	survivalCount := max(2, s.birdCount/4)
	survivors := population[:survivalCount]

	// Generate new population through crossover + mutation
	newPopulation := make([]BirdGenes, s.birdCount)

	for i := 0; i < s.birdCount; i++ {
		// Yield execution every 50 birds to prevent UI stutter
		if i%50 == 0 {
			runtime.Gosched()
		}

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

	utils.Debug("Writing evolved population to SAB",
		utils.Int("elites", len(survivors)),
		utils.Int("offspring", s.birdCount-len(survivors)))

	// Write new population back to SAB
	if err := s.WritePopulation(newPopulation); err != nil {
		return fmt.Errorf("failed to write population: %w", err)
	}

	return nil
}

// avgFitness calculates average fitness of population
func avgFitness(pop []BirdGenes) float64 {
	if len(pop) == 0 {
		return 0
	}
	sum := 0.0
	for _, b := range pop {
		sum += b.Fitness
	}
	return sum / float64(len(pop))
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
// Added "Neural Glitches": Rare extreme mutation events
func (s *BoidsSupervisor) Mutate(genes BirdGenes) BirdGenes {
	// Adjust mutation rate based on mesh nodes (more nodes = less mutation)
	effectiveMutationRate := s.mutationRate / (1.0 + math.Log2(float64(s.meshNodesActive+1)))

	// --- NEURAL GLITCH: Chaos Mutation ---
	// 0.1% chance to become a "Chaos Boid" with totally random weights
	if rand.Float64() < 0.001 {
		for i := 0; i < 44; i++ {
			genes.Weights[i] = rand.Float32()*10.0 - 5.0
		}
		return genes
	}

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

// ReadPopulation reads all bird genes from SAB in a single BULK OPERATION
func (s *BoidsSupervisor) ReadPopulation() ([]BirdGenes, error) {
	population := make([]BirdGenes, s.birdCount)

	// --- BULK READ: 1 interop call instead of 2*N ---
	totalSize := uint32(s.birdCount * BytesPerBird)
	data, err := s.bridge.ReadRaw(uint32(s.boidDataOffset), totalSize)
	if err != nil {
		return nil, fmt.Errorf("bulk read failed: %w", err)
	}

	for i := 0; i < s.birdCount; i++ {
		birdBase := i * BytesPerBird

		// Fitness at float index 14
		fitnessOffset := 14 * 4
		fitness := math.Float32frombits(binary.LittleEndian.Uint32(data[birdBase+fitnessOffset : birdBase+fitnessOffset+4]))

		// Weights at float index 15-58 (44 floats)
		weightsOffset := 15 * 4
		var weights [44]float32
		for j := 0; j < 44; j++ {
			bits := binary.LittleEndian.Uint32(data[birdBase+weightsOffset+j*4 : birdBase+weightsOffset+(j+1)*4])
			weights[j] = math.Float32frombits(bits)
		}

		population[i] = BirdGenes{
			Weights: weights,
			Fitness: float64(fitness),
			BirdID:  i,
		}
	}

	return population, nil
}

// WritePopulation writes evolved genes back to SAB in a single BULK OPERATION
func (s *BoidsSupervisor) WritePopulation(population []BirdGenes) error {
	totalSize := s.birdCount * BytesPerBird
	bulkData := make([]byte, totalSize)

	// Since we are writing to an existing population, we might want to read existing data first
	// if we were only updating some fields. But here we update fitness and weights for EVERY bird.
	// However, we must NOT stomp on bird position/velocity.
	// SO: We MUST read the current data first, patch it, then write back.

	currentData, err := s.bridge.ReadRaw(uint32(s.boidDataOffset), uint32(totalSize))
	if err != nil {
		return fmt.Errorf("bulk read for patch failed: %w", err)
	}
	copy(bulkData, currentData)

	// Apply "Species Drift" - subtle global shift in weights for all birds
	drift := float32(rand.NormFloat64() * 0.01)

	for i, bird := range population {
		birdBase := i * BytesPerBird

		// Patch fitness
		fitnessOffset := 14 * 4
		binary.LittleEndian.PutUint32(bulkData[birdBase+fitnessOffset:], math.Float32bits(float32(bird.Fitness)))

		// Patch weights with drift
		weightsOffset := 15 * 4
		for w := 0; w < 44; w++ {
			val := bird.Weights[w] + drift
			binary.LittleEndian.PutUint32(bulkData[birdBase+weightsOffset+w*4:], math.Float32bits(val))
		}
	}

	// --- BULK WRITE: 1 interop call ---
	if err := s.bridge.WriteRaw(uint32(s.boidDataOffset), bulkData); err != nil {
		return fmt.Errorf("bulk write failed: %w", err)
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
		utils.Error("Failed to read epoch", utils.Err(err))
		return
	}
	currentEpoch := binary.LittleEndian.Uint32(epochBytes)

	// Increment
	newEpoch := currentEpoch + 1
	runtime.Gosched() // Yield execution
	newBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(newBytes, newEpoch)

	// Write back
	if err := s.bridge.WriteRaw(offset, newBytes); err != nil {
		utils.Error("Failed to write epoch", utils.Err(err))
		return
	}

	utils.Debug("Epoch signaled", utils.Uint64("old", uint64(currentEpoch)), utils.Uint64("new", uint64(newEpoch)))
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
