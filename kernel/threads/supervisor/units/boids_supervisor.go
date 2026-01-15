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

	"github.com/nmxmxh/inos_v1/kernel/gen/system/v1"
	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/nmxmxh/inos_v1/kernel/utils"
	capnp "zombiezen.com/go/capnproto2"
)

const (
	// SAB layout constants - MUST MATCH RUST (sdk::layout::BIRD_STRIDE)
	FloatsPerBird = 59  // position(3) + velocity(3) + rotation(4) + angular(1) + wings(3) + fitness(1) + weights(44)
	BytesPerBird  = 236 // 59 floats * 4 bytes = 236 (matches Rust BIRD_STRIDE)
	MaxBirds      = 10000

	// Evolution parameters - TUNED FOR REAL SELECTION PRESSURE
	DefaultMutationRate   = 0.6             // Increased for more exploration
	DefaultCrossoverRate  = 0.5             // Reduced for less blending
	DefaultTournamentSize = 5               // Increased for stronger selection
	EvolutionInterval     = 3 * time.Second // Calibrated for stability
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
	ReadAtomicI32(epochIndex uint32) int32                                    // Atomic read
	WaitForEpochAsync(epochIndex uint32, expectedValue int32) <-chan struct{} // Zero-latency wait
	WriteRaw(offset uint32, data []byte) error
	SignalInbox()
	IsReady() bool // Check if SAB is initialized
}

// BoidsSupervisor manages distributed learning for bird simulation
// Executes genetic algorithm, coordinates P2P learning, signals epochs
type BoidsSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge          supervisor.SABInterface
	metricsProvider MetricsProvider // For reporting latent compute stats

	// Learning configuration
	mu             sync.RWMutex
	birdCount      int
	generation     int
	mutationRate   float64
	crossoverRate  float64
	tournamentSize int

	// Evolution state
	lastEvolutionTime     time.Time
	evolutionInterval     time.Duration
	lastExecutionDuration time.Duration // Actual CPU time of last cycle

	// P2P mesh boost
	meshNodesActive int

	// Optimization: Reusable buffer for population reads to avoid GC pressure
	populationBuf []byte
	birdChunkBuf  []byte // Reusable buffer for single bird writes

	// Delegation for mesh-aware evolution
	delegator foundation.MeshDelegator
}

// NewBoidsSupervisor creates a supervisor for learning birds
func NewBoidsSupervisor(bridge supervisor.SABInterface, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string, metricsProvider MetricsProvider, delegator foundation.MeshDelegator) *BoidsSupervisor {
	if capabilities == nil {
		capabilities = []string{"boids.physics", "boids.evolution"}
	}
	s := &BoidsSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("boids", capabilities, patterns, knowledge, delegator, bridge, nil),
		bridge:            bridge,
		metricsProvider:   metricsProvider,
		birdCount:         1000, // Default
		tournamentSize:    3,
		crossoverRate:     0.7,
		mutationRate:      DefaultMutationRate,
		meshNodesActive:   0,
		evolutionInterval: EvolutionInterval,
		birdChunkBuf:      make([]byte, 180),
	}
	return s
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
	// Read bird count from SAB - if not yet set by Rust, use default
	targetAddr := sab_layout.OFFSET_ATOMIC_FLAGS + sab_layout.IDX_BIRD_COUNT*4
	var buf [4]byte
	if err := s.bridge.ReadAt(targetAddr, buf[:]); err == nil {
		count := binary.LittleEndian.Uint32(buf[:])
		if count > 0 && count <= MaxBirds {
			s.birdCount = int(count)
			utils.Info("Boids bird count detected", utils.Int("count", s.birdCount))
			return
		}
	}

	// Default to 1000 - Rust will update SAB when population is initialized
	s.birdCount = 1000
	utils.Debug("Using default bird count", utils.Int("count", s.birdCount))
}

// learningLoop executes genetic algorithm based on physics epochs (zero CPU when idle)
func (s *BoidsSupervisor) learningLoop(ctx context.Context) {
	// Evolution trigger: every N physics frames (60fps * 3 seconds = ~180 frames)
	const evolutionFrameThreshold int32 = 180

	utils.Info("Boids learning loop starting (Epoch Mode)", utils.Int("threshold_frames", int(evolutionFrameThreshold)))

	// Track physics epochs (Rust increments IDX_BIRD_EPOCH after every physics step)
	var lastPhysicsEpoch int32 = 0
	var lastEvolutionEpoch int32 = 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.bridge.WaitForEpochAsync(sab_layout.IDX_BIRD_EPOCH, lastPhysicsEpoch):
			// Physics frame completed - Rust incremented the bird epoch
			currentEpoch := s.bridge.ReadAtomicI32(sab_layout.IDX_BIRD_EPOCH)

			// Check if enough frames have passed since last evolution
			framesSinceEvolution := currentEpoch - lastEvolutionEpoch
			if framesSinceEvolution < 0 {
				// Epoch wrapped around - reset baseline
				lastEvolutionEpoch = currentEpoch
				framesSinceEvolution = 0
			}

			if framesSinceEvolution >= evolutionFrameThreshold && s.birdCount > 0 && s.bridge.IsReady() {
				s.checkEvolution()
				lastEvolutionEpoch = currentEpoch
			}

			lastPhysicsEpoch = currentEpoch
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

	// Note: Evolution frequency is controlled by the epoch-driven learningLoop
	// (every N physics frames), so no time check needed here.

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
	s.lastExecutionDuration = duration // Store for adaptive logic

	// Log evolution complete
	utils.Info("Evolution Cycle Complete",
		utils.Int("gen", s.generation),
		utils.Duration("took", duration),
		utils.Bool("local_optimized", s.meshNodesActive > 0 && duration < 16*time.Millisecond))

	// Report Latent Compute to Mesh Metrics
	// Ops = Birds * Genes (44) * ~50 FLOPs (Selection/Crossover/Mutation)
	latentOps := float64(s.birdCount) * 44.0 * 50.0
	seconds := duration.Seconds()
	if seconds > 0 {
		opsPerSec := latentOps / seconds
		gflops := opsPerSec / 1e9
		if s.metricsProvider != nil {
			s.metricsProvider.ReportComputeActivity(opsPerSec, gflops)
		}
	}

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

	// Selection: keep top 25% (elites)
	survivalCount := max(2, s.birdCount/4)
	survivors := population[:survivalCount]

	// Generate new population through crossover + mutation
	newPopulation := make([]BirdGenes, s.birdCount)
	copy(newPopulation[:survivalCount], survivors) // Keep elites

	// Offload to mesh ONLY if local processing is struggling (>16ms/frame)
	offspringNeeded := s.birdCount - survivalCount
	offloadCount := 0

	// Adaptive Scaling:
	// If the last evolution took < 16ms (60fps budget), keep it 100% local.
	// Only offload if we are dropping frames or if explicitly requested via high load.
	if s.meshNodesActive > 0 && s.delegator != nil && s.lastExecutionDuration > 16*time.Millisecond {
		offloadCount = offspringNeeded / 5 // Offload 20% to mesh if slow
	}
	localCount := offspringNeeded - offloadCount

	var wg sync.WaitGroup
	var mu sync.Mutex

	// 2. Parallel Local Generation
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < localCount; i++ {
			if i%50 == 0 {
				runtime.Gosched() // Yield to JS event loop to prevent blocking
			}
			p1 := s.TournamentSelect(survivors)
			p2 := s.TournamentSelect(survivors)
			child := s.Crossover(p1, p2)
			child = s.Mutate(child)
			child.BirdID = survivalCount + i
			child.Fitness = 0

			mu.Lock()
			newPopulation[survivalCount+i] = child
			mu.Unlock()
		}
	}()

	// 3. Mesh Delegation (Async)
	if offloadCount > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Serialize survivors using binary encoding for "water-like" fluidity
			packedParents := serializeGenesBinary(survivors)

			// Wrap in Universal Resource Protocol
			msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
			if err != nil {
				utils.Error("Failed to create capnp message", utils.Err(err))
				return
			}
			resource, err := system.NewRootResource(seg)
			if err != nil {
				utils.Error("Failed to create resource struct", utils.Err(err))
				return
			}

			resource.SetId(fmt.Sprintf("boids-evolve-%d", s.generation))
			resource.SetPriority(200) // High priority for compute
			resource.SetTimestamp(uint64(time.Now().UnixNano()))
			resource.SetInline(packedParents)

			// Add metadata for mesh context
			meta, _ := resource.NewMetadata()
			meta.SetContentType("application/x-inos-boids-genes")

			// Encode resource to bytes
			resourceData, err := msg.Marshal()
			if err != nil {
				utils.Error("Failed to marshal resource", utils.Err(err))
				return
			}

			job := &foundation.Job{
				ID:        fmt.Sprintf("boids-evolve-%d", s.generation),
				Type:      "compute",
				Operation: "boids.evolve_batch",
				Data:      resourceData,
				Parameters: map[string]interface{}{
					"count":   offloadCount,
					"base_id": survivalCount + localCount,
				},
			}

			result, err := s.delegator.DelegateJob(context.Background(), job)
			if err != nil {
				utils.Warn("Mesh delegation failed, falling back to local for batch", utils.Err(err))
				// Fallback: execute locally
				for i := 0; i < offloadCount; i++ {
					p1 := s.TournamentSelect(survivors)
					p2 := s.TournamentSelect(survivors)
					child := s.Crossover(p1, p2)
					child = s.Mutate(child)
					child.BirdID = survivalCount + localCount + i

					mu.Lock()
					newPopulation[survivalCount+localCount+i] = child
					mu.Unlock()
				}
				return
			}

			// Deserialize Resource result
			resMsg, err := capnp.Unmarshal(result.Data)
			if err != nil {
				utils.Error("Failed to unmarshal result resource", utils.Err(err))
				return
			}
			res, err := system.ReadRootResource(resMsg)
			if err != nil {
				utils.Error("Failed to read root resource", utils.Err(err))
				return
			}

			inlineData, err := res.Inline()
			if err != nil {
				utils.Error("Failed to get inline data from resource", utils.Err(err))
				return
			}

			remoteGenes := deserializeGenesBinary(inlineData)
			mu.Lock()
			for i, genes := range remoteGenes {
				if survivalCount+localCount+i < s.birdCount {
					newPopulation[survivalCount+localCount+i] = genes
				}
			}
			mu.Unlock()
			utils.Info("Mesh delegation successful (Binary Resource)", utils.Int("genes_received", len(remoteGenes)))
		}()
	}

	wg.Wait()

	utils.Debug("Writing evolved population to SAB",
		utils.Int("elites", survivalCount),
		utils.Int("local", localCount),
		utils.Int("remote", offloadCount))

	// Write new population back to SAB
	if err := s.WritePopulation(newPopulation); err != nil {
		return fmt.Errorf("failed to write population: %w", err)
	}

	return nil
}

// ExecuteJob handles remote evolution requests (Mesh Role)
func (s *BoidsSupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	if job.Operation != "boids.evolve_batch" {
		return &foundation.Result{
			JobID: job.ID,
			Error: "unsupported boids operation: " + job.Operation,
		}
	}

	// Unmarshal Resource request
	resMsg, err := capnp.Unmarshal(job.Data)
	if err != nil {
		return &foundation.Result{JobID: job.ID, Error: "failed to unmarshal request resource: " + err.Error()}
	}
	res, err := system.ReadRootResource(resMsg)
	if err != nil {
		return &foundation.Result{JobID: job.ID, Error: "failed to read root resource: " + err.Error()}
	}

	inlineData, err := res.Inline()
	if err != nil {
		return &foundation.Result{JobID: job.ID, Error: "failed to get inline data: " + err.Error()}
	}

	survivors := deserializeGenesBinary(inlineData)
	count, _ := job.Parameters["count"].(float64)
	baseID, _ := job.Parameters["base_id"].(float64)

	offspring := make([]BirdGenes, int(count))
	for i := 0; i < int(count); i++ {
		p1 := s.TournamentSelect(survivors)
		p2 := s.TournamentSelect(survivors)
		child := s.Crossover(p1, p2)
		child = s.Mutate(child)
		child.BirdID = int(baseID) + i
		child.Fitness = 0
		offspring[i] = child
	}

	// Wrap response in Resource
	packedOffspring := serializeGenesBinary(offspring)
	outMsg, outSeg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return &foundation.Result{JobID: job.ID, Error: "failed to create out message: " + err.Error()}
	}
	outRes, err := system.NewRootResource(outSeg)
	if err != nil {
		return &foundation.Result{JobID: job.ID, Error: "failed to create out resource: " + err.Error()}
	}
	outRes.SetId(job.ID + "-res")
	outRes.SetInline(packedOffspring)
	outData, err := outMsg.Marshal()
	if err != nil {
		return &foundation.Result{JobID: job.ID, Error: "failed to marshal out resource: " + err.Error()}
	}

	return &foundation.Result{
		JobID:   job.ID,
		Success: true,
		Data:    outData,
	}
}

// Binary serialization helpers for "water-like" fluidity (Zero-Allocation friendly)

func serializeGenesBinary(genes []BirdGenes) []byte {
	// [ID(4) | Fitness(8) | Weights(44*4)] per bird
	// Size = 4 + 8 + 176 = 188 bytes per bird
	const stride = 188
	buf := make([]byte, len(genes)*stride)

	for i, g := range genes {
		offset := i * stride
		binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(g.BirdID))
		binary.LittleEndian.PutUint64(buf[offset+4:offset+12], math.Float64bits(g.Fitness))

		weightsOffset := offset + 12
		for w := 0; w < 44; w++ {
			binary.LittleEndian.PutUint32(buf[weightsOffset+w*4:weightsOffset+(w+1)*4], math.Float32bits(g.Weights[w]))
		}
	}
	return buf
}

func deserializeGenesBinary(data []byte) []BirdGenes {
	const stride = 188
	count := len(data) / stride
	genes := make([]BirdGenes, count)

	for i := 0; i < count; i++ {
		offset := i * stride
		id := binary.LittleEndian.Uint32(data[offset : offset+4])
		fitness := math.Float64frombits(binary.LittleEndian.Uint64(data[offset+4 : offset+12]))

		var weights [44]float32
		weightsOffset := offset + 12
		for w := 0; w < 44; w++ {
			bits := binary.LittleEndian.Uint32(data[weightsOffset+w*4 : weightsOffset+(w+1)*4])
			weights[w] = math.Float32frombits(bits)
		}

		genes[i] = BirdGenes{
			BirdID:  int(id),
			Fitness: fitness,
			Weights: weights,
		}
	}
	return genes
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

// TournamentSelect selects a parent using tournament selection with ELITISM
func (s *BoidsSupervisor) TournamentSelect(population []BirdGenes) BirdGenes {
	// ELITISM: 30% chance to select from top 10% performers
	// This ensures best traits propagate while maintaining diversity
	if rand.Float64() < 0.30 && len(population) > 20 {
		eliteCut := len(population) / 10 // Top 10%
		if eliteCut > 0 {
			return population[rand.Intn(eliteCut)]
		}
	}

	// Standard tournament for the rest
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
// ENHANCED: Stronger mutations for real diversity
func (s *BoidsSupervisor) Mutate(genes BirdGenes) BirdGenes {
	// Adjust mutation rate based on mesh nodes (more nodes = less mutation)
	effectiveMutationRate := s.mutationRate / (1.0 + math.Log2(float64(s.meshNodesActive+1)))

	// --- CHAOS BOID: 10% chance for complete randomization ---
	if rand.Float64() < 0.10 {
		utils.Debug("Neural Glitch! Chaos boid created")
		for i := 0; i < 44; i++ {
			genes.Weights[i] = rand.Float32()*20.0 - 10.0 // Wider range: -10 to +10
		}
		return genes
	}

	// --- FOCUS MUTATION: 15% chance to dramatically change ONE weight ---
	// This creates specialists rather than generalists
	if rand.Float64() < 0.15 {
		idx := rand.Intn(44)
		genes.Weights[idx] = rand.Float32()*16.0 - 8.0 // Dramatic single change
		return genes
	}

	// --- STANDARD GAUSSIAN MUTATION ---
	for i := 0; i < 44; i++ {
		if rand.Float64() < effectiveMutationRate {
			// Larger Gaussian noise (0.8 instead of 0.5)
			genes.Weights[i] += float32(rand.NormFloat64() * 0.8)

			// Clamp to wider range: ±10
			if genes.Weights[i] > 10.0 {
				genes.Weights[i] = 10.0
			} else if genes.Weights[i] < -10.0 {
				genes.Weights[i] = -10.0
			}
		}
	}

	return genes
}

// ReadPopulation reads all bird genes from SAB in a single BULK OPERATION
func (s *BoidsSupervisor) ReadPopulation() ([]BirdGenes, error) {
	population := make([]BirdGenes, s.birdCount)

	// Determine active buffer from ping-pong (Stable Read Buffer)
	active := s.bridge.ReadAtomicI32(sab_layout.IDX_PINGPONG_ACTIVE)

	offset := uint32(sab_layout.OFFSET_BIRD_BUFFER_A)
	if active == 1 {
		offset = uint32(sab_layout.OFFSET_BIRD_BUFFER_B)
	}

	// If active is neither 0 nor 1, we haven't initialized yet or have a corruption.
	// Default to A as it's the boot buffer.
	if active < 0 || active > 1 {
		offset = uint32(sab_layout.OFFSET_BIRD_BUFFER_A)
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

// WritePopulation writes evolved genes back to SAB
// CRITICAL FIX: We only write [Fitness + Weights] (Offset 56..236).
// We NEVER overwrite [Position/Velocity/Rotation] (Offset 0..56) because Rust is updating those at 60Hz.
// Overwriting them with old data cause "time travel" rubber-banding.
func (s *BoidsSupervisor) WritePopulation(population []BirdGenes) error {
	// Determine current READ buffer (Stable).
	// Writing to the READ buffer ensures Rust picks up new genes in its NEXT physics step.
	active := s.bridge.ReadAtomicI32(sab_layout.IDX_PINGPONG_ACTIVE)

	baseOffset := uint32(sab_layout.OFFSET_BIRD_BUFFER_A)
	if active == 1 {
		baseOffset = uint32(sab_layout.OFFSET_BIRD_BUFFER_B)
	}

	// Default to A if not initialized
	if active < 0 || active > 1 {
		baseOffset = uint32(sab_layout.OFFSET_BIRD_BUFFER_A)
	}

	utils.Info("Writing evolved population to stable buffer",
		utils.Int("active_idx", int(active)),
		utils.String("offset", fmt.Sprintf("0x%X", baseOffset)))

	s.mu.RLock()
	chunk := s.birdChunkBuf
	s.mu.RUnlock()

	for i, bird := range population {
		// Prepare data chunk (only Weights/Fitness)
		binary.LittleEndian.PutUint32(chunk[0:4], math.Float32bits(float32(bird.Fitness)))

		for j := 0; j < 44; j++ {
			binary.LittleEndian.PutUint32(chunk[4+j*4:8+j*4], math.Float32bits(bird.Weights[j]))
		}

		// Calculate target address (Index 14 floats)
		targetAddr := baseOffset + uint32(i*BytesPerBird) + 56

		// Write ONLY the weight chunk
		if err := s.bridge.WriteRaw(targetAddr, chunk); err != nil {
			return fmt.Errorf("weight write failed at bird %d: %w", i, err)
		}
	}

	// Debug log first bird's new weights
	if len(population) > 0 {
		utils.Info("Gen Evolved & Weights Patched",
			utils.String("target_buffer", fmt.Sprintf("0x%X", baseOffset)))
	}

	return nil
}

// SignalEpoch increments the epoch flag to trigger frontend reactivity
func (s *BoidsSupervisor) SignalEpoch() {
	// Evolution Epoch at Idx 16
	offset := sab_layout.OFFSET_ATOMIC_FLAGS + sab_layout.IDX_EVOLUTION_EPOCH*4

	// Read current epoch from offset using stack array
	var buf [4]byte
	if err := s.bridge.ReadAt(offset, buf[:]); err != nil {
		utils.Error("Failed to read epoch", utils.Err(err))
		return
	}
	currentEpoch := binary.LittleEndian.Uint32(buf[:])

	// Increment
	newEpoch := currentEpoch + 1
	runtime.Gosched() // Yield execution
	binary.LittleEndian.PutUint32(buf[:], newEpoch)

	// Write back
	if err := s.bridge.WriteRaw(offset, buf[:]); err != nil {
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
