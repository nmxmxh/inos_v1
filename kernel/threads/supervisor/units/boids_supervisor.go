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
	// SAB layout constants - MUST MATCH RUST (sdk::layout::BIRD_STRIDE)
	FloatsPerBird = 59  // position(3) + velocity(3) + rotation(4) + angular(1) + wings(3) + fitness(1) + weights(44)
	BytesPerBird  = 236 // 59 floats * 4 bytes = 236 (matches Rust BIRD_STRIDE)
	MaxBirds      = 10000

	// Evolution parameters - TUNED FOR VISIBLE VARIANCE
	DefaultMutationRate   = 0.4 // Reduced to 0.4 for stability while keeping diversity
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
	ReadAt(offset uint32, dest []byte) error                                  // Zero-allocation optimized read
	WaitForEpochAsync(epochIndex uint32, expectedValue int32) <-chan struct{} // Zero-latency wait
	WriteRaw(offset uint32, data []byte) error
	SignalInbox()
	IsReady() bool // Check if SAB is initialized
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

	// Optimization: Reusable buffer for population reads to avoid GC pressure
	populationBuf []byte
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

// autoDetectBirdCount reads the bird count from SAB atomic flags (Zero-Latency)
func (s *BoidsSupervisor) autoDetectBirdCount() {
	// 1. Check if already set
	targetAddr := sab_layout.OFFSET_ATOMIC_FLAGS + sab_layout.IDX_BIRD_COUNT*4
	data, err := s.bridge.ReadRaw(targetAddr, 4)
	if err == nil {
		count := binary.LittleEndian.Uint32(data)
		if count > 0 && count <= MaxBirds {
			s.birdCount = int(count)
			utils.Info("Boids bird count detected instantly", utils.Int("count", s.birdCount))
			return
		}
	}

	// 2. Wait for signal (0 Latency)
	utils.Info("Waiting for bird count signal...")

	// Wait for IDX_BIRD_COUNT to change from 0 (or current invalid value)
	// We wait for ANY change.
	done := s.bridge.WaitForEpochAsync(sab_layout.IDX_BIRD_COUNT, 0)

	select {
	case <-done:
		// Signal received! Read immediately.
		data, err := s.bridge.ReadRaw(targetAddr, 4)
		if err == nil {
			count := binary.LittleEndian.Uint32(data)
			s.birdCount = int(count)
			utils.Info("Boids bird count detected via signal", utils.Int("count", s.birdCount))
		}
	case <-time.After(5 * time.Second):
		utils.Warn("Timeout waiting for bird count signal, using default", utils.Int("default", 1000))
		s.birdCount = 1000
	}
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
// TUNED FOR VISIBLE VARIANCE
func (s *BoidsSupervisor) Mutate(genes BirdGenes) BirdGenes {
	// Adjust mutation rate based on mesh nodes (more nodes = less mutation)
	effectiveMutationRate := s.mutationRate / (1.0 + math.Log2(float64(s.meshNodesActive+1)))

	// --- NEURAL GLITCH: Chaos Mutation ---
	// 5% chance to become a "Chaos Boid" with totally random weights (was 1%)
	if rand.Float64() < 0.05 {
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
	// If Writer is Active on A (0), we read B (Stable)
	// If Writer is Active on B (1), we read A (Stable)
	if active == 0 {
		offset = uint32(sab_layout.OFFSET_BIRD_BUFFER_B)
	}

	utils.Info("DEBUG: ReadPopulation",
		utils.Int("active", int(active)),
		utils.String("offset", fmt.Sprintf("0x%X", offset)),
		utils.Int("count", s.birdCount))

	// --- BULK READ: 1 interop call instead of 2*N ---
	// Resize reuse buffer if needed
	totalSize := uint32(s.birdCount * BytesPerBird)
	if uint32(cap(s.populationBuf)) < totalSize {
		s.populationBuf = make([]byte, totalSize)
	}
	s.populationBuf = s.populationBuf[:totalSize]

	// Use Zero-Allocation ReadAt
	if err := s.bridge.ReadAt(offset, s.populationBuf); err != nil {
		utils.Warn("Failed to read boids population from SAB", utils.Err(err))
		return nil, fmt.Errorf("bulk read failed at offset %x: %w", offset, err)
	}

	for i := 0; i < s.birdCount; i++ {
		birdBase := i * BytesPerBird

		// Fitness at float index 14
		fitnessOffset := 14 * 4
		fitness := math.Float32frombits(binary.LittleEndian.Uint32(s.populationBuf[birdBase+fitnessOffset : birdBase+fitnessOffset+4]))

		// Weights at float index 15-58 (44 floats)
		weightsOffset := 15 * 4
		var weights [44]float32
		for j := 0; j < 44; j++ {
			bits := binary.LittleEndian.Uint32(s.populationBuf[birdBase+weightsOffset+j*4 : birdBase+weightsOffset+(j+1)*4])
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

	// Determine Active Buffer
	activeEpochBytes, _ := s.bridge.ReadRaw(uint32(sab_layout.OFFSET_ATOMIC_FLAGS+sab_layout.IDX_PINGPONG_ACTIVE*4), 4)
	active := binary.LittleEndian.Uint32(activeEpochBytes)

	// Target the STABLE buffer (Reader Buffer, Source of next frame)
	// If active is 0 (Buffer A is Active/Readable), then Rust is writing B.
	// We update A so that when Rust flips next frame, it reads our new weights from A.
	// This avoids race conditions as we only touch the buffer Rust has finished with.
	offset := uint32(sab_layout.OFFSET_BIRD_BUFFER_A)
	if active == 1 {
		offset = uint32(sab_layout.OFFSET_BIRD_BUFFER_B)
	}

	currentData, err := s.bridge.ReadRaw(offset, uint32(totalSize))
	if err != nil {
		return fmt.Errorf("bulk read for patch failed at offset %x: %w", offset, err)
	}
	copy(bulkData, currentData)

	// Apply "Magical Pulse" - Time-based oscillation for "Breathing" effect
	// Time since epoch logic or just wall clock
	t := float64(time.Now().UnixNano()) / 1e9

	// Separation Pulse: Oscillates from -2.0 to +2.0 over 10 seconds
	sepPulse := float32(math.Sin(t*0.6) * 2.0)

	// Cohesion Pulse: Inverse oscillation (when Sep is high, Coh is low)
	cohPulse := float32(math.Cos(t*0.6) * 2.0)

	// Alignment Pulse: Faster shimmer
	aliPulse := float32(math.Sin(t*2.0) * 0.5)

	// Spotlight: Randomly pick trick performers (5% of flock = 50 birds)
	trickTargets := make(map[int]bool)
	targetCount := max(1, s.birdCount/20) // 5%
	for i := 0; i < targetCount; i++ {
		trickTargets[rand.Intn(s.birdCount)] = true
	}

	for i, bird := range population {
		birdBase := i * BytesPerBird

		// Patch fitness
		fitnessOffset := 14 * 4
		binary.LittleEndian.PutUint32(bulkData[birdBase+fitnessOffset:], math.Float32bits(float32(bird.Fitness)))

		// Patch weights with Pulse
		weightsOffset := 15 * 4

		// Apply Pulses to Genes 0, 1, 2
		bird.Weights[0] += sepPulse
		bird.Weights[1] += aliPulse
		bird.Weights[2] += cohPulse

		// TRICK SYSTEM: Spotlight Logic
		// Reset Trick Weight (w3) to 0 first (unless mutated natural talent)
		bird.Weights[3] = 0.0

		// If this is a chosen performer, activate Trick
		if trickTargets[i] {
			if rand.Float64() > 0.5 {
				bird.Weights[3] = 5.0 // Hover
			} else {
				bird.Weights[3] = -5.0 // Barrel Roll
			}
		}

		for w := 0; w < 44; w++ {
			val := bird.Weights[w]
			binary.LittleEndian.PutUint32(bulkData[birdBase+weightsOffset+w*4:], math.Float32bits(val))
		}
	}

	// --- BULK WRITE: 1 interop call ---
	if err := s.bridge.WriteRaw(offset, bulkData); err != nil {
		return fmt.Errorf("bulk write failed at offset %x: %w", offset, err)
	}

	// Debug log first bird's new weights
	if len(population) > 0 {
		utils.Info("Gen Evolved & Written",
			utils.String("target_offset", fmt.Sprintf("0x%X", offset)),
			utils.Float64("pulse_sep", float64(sepPulse)),
			utils.Float64("b0_w0", float64(population[0].Weights[0])),
			utils.Float64("b0_w3_trick", float64(population[0].Weights[3])))
	}

	return nil
}

// SignalEpoch increments the epoch flag to trigger frontend reactivity
func (s *BoidsSupervisor) SignalEpoch() {
	// Evolution Epoch at Idx 16
	offset := sab_layout.OFFSET_ATOMIC_FLAGS + sab_layout.IDX_EVOLUTION_EPOCH*4

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
