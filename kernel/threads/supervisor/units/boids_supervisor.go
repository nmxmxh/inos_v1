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
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/nmxmxh/inos_v1/kernel/utils"
)

const (
	// SAB layout constants - MUST MATCH RUST
	FloatsPerBird = 58  // position(3) + velocity(3) + rotation(4) + padding... = 58 floats in Rust
	BytesPerBird  = 232 // MUST match Rust: BYTES_PER_BIRD = 232 (NOT 236!)
	MaxBirds      = 10000

	// Evolution parameters - TUNED FOR VISIBLE VARIANCE
	DefaultMutationRate   = 0.3 // Increased from 0.1 for more visible changes
	DefaultCrossoverRate  = 0.7
	DefaultTournamentSize = 3
	EvolutionInterval     = 2 * time.Second // Faster evolution for demo (was 5s)
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

	// Evolution state
	lastEvolutionTime time.Time
	evolutionInterval time.Duration

	// P2P mesh boost
	meshNodesActive int
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
		mutationRate:      DefaultMutationRate,
		crossoverRate:     DefaultCrossoverRate,
		tournamentSize:    DefaultTournamentSize,
		evolutionInterval: EvolutionInterval,
		lastEvolutionTime: time.Now(),
	}
}

// Start begins the learning supervision loop - BLOCKS until context cancelled
func (s *BoidsSupervisor) Start(ctx context.Context) error {
	// Auto-detect bird count from SAB epoch (IDX_BOIDS_COUNT)
	s.autoDetectBirdCount()

	utils.Info("Boids supervisor started", utils.Int("bird_count", s.birdCount))

	// Run learning loop - this BLOCKS until ctx.Done()
	// (spawnChild expects the function to block)
	s.learningLoop(ctx)

	return nil
}

// autoDetectBirdCount reads the bird count from SAB atomic flags
func (s *BoidsSupervisor) autoDetectBirdCount() {
	// Read from IDX_BOIDS_COUNT (index 15 in atomic flags)
	offset := uint32(sab_layout.OFFSET_ATOMIC_FLAGS + sab_layout.IDX_BOIDS_COUNT*4)
	countBytes, err := s.bridge.ReadRaw(offset, 4)
	if err != nil {
		utils.Warn("Failed to read bird count from SAB, using default", utils.Err(err))
		s.birdCount = 1000 // Default fallback
		return
	}

	count := int(binary.LittleEndian.Uint32(countBytes))
	if count > 0 && count <= MaxBirds {
		s.birdCount = count
	} else {
		// Fallback: read from frontend default
		s.birdCount = 1000
	}

	utils.Info("Auto-detected bird count from SAB", utils.Int("count", s.birdCount))
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

	// TODO: Rust bird struct doesn't include neural weights yet
	// Go expects 236 bytes/bird (with 44 neural weights)
	// But Rust only has 232 bytes/bird
	// Skip evolution until layouts are aligned
	if BytesPerBird != 232 {
		// Layout mismatch - skip evolution
		return
	}

	// For now, just log that evolution is disabled until Rust is updated
	// Once Rust adds the weight fields, remove this check
	if s.generation == 0 {
		utils.Warn("BoidsSupervisor: Evolution disabled - Rust bird struct missing neural weights",
			utils.Int("go_bytes_per_bird", BytesPerBird),
			utils.String("todo", "Add weights[44] to Rust bird struct"))
		s.generation = -1 // Mark as disabled so we only log once
	}
	return // Skip evolution until Rust layout includes weights

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
// TUNED FOR VISIBLE VARIANCE
func (s *BoidsSupervisor) Mutate(genes BirdGenes) BirdGenes {
	// Adjust mutation rate based on mesh nodes (more nodes = less mutation)
	effectiveMutationRate := s.mutationRate / (1.0 + math.Log2(float64(s.meshNodesActive+1)))

	// --- NEURAL GLITCH: Chaos Mutation ---
	// 1% chance to become a "Chaos Boid" with totally random weights (was 0.1%)
	if rand.Float64() < 0.01 {
		utils.Debug("Neural Glitch! Chaos boid created")
		for i := 0; i < 44; i++ {
			genes.Weights[i] = rand.Float32()*10.0 - 5.0
		}
		return genes
	}

	for i := 0; i < 44; i++ {
		if rand.Float64() < effectiveMutationRate {
			// Larger Gaussian noise for visible effect (0.5 instead of 0.2)
			genes.Weights[i] += float32(rand.NormFloat64() * 0.5)

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

	// Determine active buffer from ping-pong epoch
	activeEpochBytes, err := s.bridge.ReadRaw(uint32(sab_layout.OFFSET_ATOMIC_FLAGS+sab_layout.IDX_PINGPONG_ACTIVE*4), 4)
	if err != nil {
		return nil, fmt.Errorf("failed to read active buffer epoch: %w", err)
	}
	active := binary.LittleEndian.Uint32(activeEpochBytes)

	offset := uint32(sab_layout.OFFSET_BIRD_BUFFER_A)
	if active == 1 {
		offset = uint32(sab_layout.OFFSET_BIRD_BUFFER_B)
	}

	// --- BULK READ: 1 interop call instead of 2*N ---
	totalSize := uint32(s.birdCount * BytesPerBird)
	data, err := s.bridge.ReadRaw(offset, totalSize)
	if err != nil {
		return nil, fmt.Errorf("bulk read failed at offset %x: %w", offset, err)
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

	// Target the INACTIVE buffer for writing the next generation
	// If active is 0 (Buffer A), we write to Buffer B
	activeEpochBytes, _ := s.bridge.ReadRaw(uint32(sab_layout.OFFSET_ATOMIC_FLAGS+sab_layout.IDX_PINGPONG_ACTIVE*4), 4)
	active := binary.LittleEndian.Uint32(activeEpochBytes)

	offset := uint32(sab_layout.OFFSET_BIRD_BUFFER_B)
	if active == 1 {
		offset = uint32(sab_layout.OFFSET_BIRD_BUFFER_A)
	}

	currentData, err := s.bridge.ReadRaw(offset, uint32(totalSize))
	if err != nil {
		return fmt.Errorf("bulk read for patch failed at offset %x: %w", offset, err)
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
	if err := s.bridge.WriteRaw(offset, bulkData); err != nil {
		return fmt.Errorf("bulk write failed at offset %x: %w", offset, err)
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
