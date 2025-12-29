@0xd1c2b3a4e5f67890;

using Base = import "/base/v1/base.capnp";

# ===========================================================================
# THE REALITY CONTRACT
# A zero-copy, cache-aware, multi-scale physics protocol for 1B+ node mesh.
# Every field optimized for BLAKE3 hashing and merkle proof generation.
# ===========================================================================

interface Science {
  # Primary execution with deterministic hashing for global deduplication
  execute @0 (request :ScienceRequest) -> (result :ScienceResult);
  
  # Request pre-computation hash (for mesh deduplication)
  computeRequestHash @1 (request :ScienceRequest) -> (hash :Data);
  
  # Multi-scale coupled simulation (quantum+continuum+kinetic in one call)
  executeCoupled @2 (request :CoupledRequest) -> (result :CoupledResult);
  
  # Validate a previous result against current mesh state (for PoS consensus)
  validateResult @3 (resultHash :Data) -> (validation :ValidationProof);

  # Mosaic P2P Protocol (Application Level)
  # Messages sent via Kernel Syscall::sendMessage
  struct MosaicMessage {
    union {
      allocationQuery @0 :AllocationQuery;
      allocationResponse @1 :AllocationResponse;
      chunkResponse @2 :StateChunk; # Response to a generic fetchChunk request
    }
  }
}

# ---------------------------------------------------------------------------
# MOSAIC: Distributed Voxel State
# ---------------------------------------------------------------------------

struct AllocationQuery {
  voxelRange @0 :VoxelRange;
  strategy @1 :Text;
}

struct AllocationResponse {
  recommendedPeers @0 :List(Data); # List<PeerID>
  confidence @1 :Float32;
}

struct StateChunk {
  id @0 :Data;
  voxelRange @1 :VoxelRange;
  data @2 :Data;
  checksum @3 :Data;
  version @4 :UInt64;
  compressed @5 :Bool;
}

struct VoxelRange {
  min @0 :List(Int32); # [x, y, z]
  max @1 :List(Int32);
}

# ---------------------------------------------------------------------------
# CORE TYPES: Reality as Data
# ---------------------------------------------------------------------------

struct DataRef {
  # Cap'n Proto zero-copy for mesh-scale efficiency
  union {
    inline @0 :Data;           # < 1MB (fits in L1 cache)
    hash @1 :Data;             # BLAKE3(32) for mesh storage
    merkleProof @2 :MerkleProof;  # For partial validation
  }
}

struct MerkleProof {
  rootHash @0 :Data;           # Global state root
  leafHash @1 :Data;           # Our data's hash
  path @2 :List(Data);         # Sibling hashes up to root
  indices @3 :List(UInt8);     # Left/right flags (0=left,1=right)
}

struct DeterministicHash {
  # Everything in the mesh is content-addressable
  inputHash @0 :Data;          # BLAKE3(request.serialize())
  methodHash @1 :Data;         # BLAKE3(library + method + version)
  paramsHash @2 :Data;         # BLAKE3(params.serialize())
  resultHash @3 :Data;         # BLAKE3(result.serialize())
  
  # Merkle tree position in global computation log
  epoch @4 :UInt64;            # Global mesh epoch
  shard @5 :UInt32;            # Which of 1M shards computed this
  proofOfWork @6 :Data;        # Nonce for Proof-of-Simulation
}

# ---------------------------------------------------------------------------
# SCALE-AWARE EXECUTION
# ---------------------------------------------------------------------------

struct ScienceRequest {
  # Execution context
  library @0 :Library;
  method @1 :Text;
  params @2 :ScienceParams;
  inputData @3 :DataRef;
  metadata @4 :Base.Base.Metadata;
  
  # Mesh optimization
  computationId @5 :Data;      # DeterministicHash.inputHash
  cachePolicy @6 :CachePolicy;
  priority @7 :Priority;
  scaleHint @8 :SimulationScale;
  
  # For cross-node verification
  expectedResultHash @9 :Data;  # If known (for validation)
  verificationSeed @10 :UInt64; # RNG seed for spot-checking
}

enum Library {
  atomic @0;    # Molecular Dynamics (groan_rs)
  continuum @1; # Finite Element Analysis (fenris)
  kinetic @2;   # Rigid Body Physics (rapier3d)
  math @3;      # Linear Algebra (nalgebra)
}

struct SimulationScale {
  spatial @0 :Float64;        # Characteristic length (meters)
  temporal @1 :Float64;       # Characteristic time (seconds)
  energy @2 :Float64;         # Characteristic energy (Joules)
  fidelity @3 :FidelityLevel; # How "real" should this be?
}

enum FidelityLevel {
  # Tradeoff between speed and accuracy
  heuristic @0;        # Fast, approximate (game physics)
  engineering @1;      # Industry standard (FEA, MD)
  research @2;         # Publication quality (DFT, CCSD)
  quantumExact @3;     # Full Schrödinger (exponential scaling)
  realityProof @4;     # With merkle proofs of correctness
}

enum CachePolicy {
  computeAlways @0;           # Ignore cache, recompute
  cacheIfExists @1;          # Use cache, compute if missing
  validateOnly @2;           # Just verify, don't compute
  storeOnly @3;              # Compute and cache, don't return
}

enum Priority {
  realtime @0;               # 0-10ms response (kinematics)
  interactive @1;            # 10-100ms (continuum solve)
  batch @2;                  # 100ms-10s (quantum chemistry)
  background @3;             # 10s+ (materials discovery)
}

# ---------------------------------------------------------------------------
# MULTI-SCALE COUPLED SIMULATION (The Holy Grail)
# ---------------------------------------------------------------------------

struct CoupledRequest {
  # A graph of computations across scales
  computations @0 :List(CoupledComputation);
  coupling @1 :List(ScaleCoupling);
  
  # How to reconcile results across scales
  reconciliation @2 :ReconciliationMethod;
  tolerance @3 :Float64;      # Maximum energy discrepancy allowed
  
  # For global mesh orchestration
  shardingStrategy @4 :ShardingStrategy;
  synchronizationEpoch @5 :UInt64;  # Global mesh epoch for sync
}

struct CoupledComputation {
  id @0 :UInt32;              # Local ID for this computation
  request @1 :ScienceRequest; # The actual computation
  scale @2 :SimulationScale;  # Where this lives in scale-space
  dependencies @3 :List(UInt32);  # Which other computations this needs
}

struct ScaleCoupling {
  fromId @0 :UInt32;          # Source computation
  toId @1 :UInt32;            # Target computation
  couplingType @2 :CouplingType;
  mapping @3 :DataRef;        # How to map results (interpolation, averaging)
}

enum CouplingType {
  quantumToContinuum @0;      # Electron density → Stress tensor
  continuumToKinetic @1;      # Stress → Forces on rigid bodies
  kineticToContinuum @2;      # Motion → Boundary conditions
  allToAll @3;                # Full bidirectional coupling
}

enum ReconciliationMethod {
  weakCoupling @0;            # One-way influence (fast)
  strongCoupling @1;          # Iterate to convergence
  monolithic @2;              # Solve all scales simultaneously (hard)
}

enum ShardingStrategy {
  byScale @0;                 # Each scale on different node groups
  byDomain @1;                # Spatial decomposition
  hybrid @2;                  # Both scale and domain
  adaptive @3;                # Mesh decides based on load
}

struct CoupledResult {
  # Individual results
  results @0 :List(IndividualResult);
  
  # Cross-scale consistency proof
  consistencyProof @1 :ConsistencyProof;
  
  # Global mesh state after computation
  globalStateHash @2 :Data;
  merkleRoot @3 :Data;
  
  # Performance metrics for mesh optimization
  nodeSeconds @4 :Float64;    # Total compute time across all nodes
  dataTransferred @5 :UInt64; # Bytes moved through mesh
  scaleCouplingTime @6 :Float64; # Time spent reconciling scales
}

struct IndividualResult {
  computationId @0 :UInt32;
  result @1 :ScienceResult;
  nodeId @2 :UInt64;          # Which mesh node computed this
  verificationHash @3 :Data;  # For Proof-of-Simulation
}

struct ConsistencyProof {
  # Proof that results are physically consistent across scales
  energyBalance @0 :Float64;  # Total energy conserved?
  forceMismatch @1 :Float64;  # Quantum vs continuum forces
  stressContinuity @2 :Float64; # Stress tensor smoothness
  
  # Merkle proof that these values were computed correctly
  proof @3 :MerkleProof;
}

# ---------------------------------------------------------------------------
# LIBRARY-SPECIFIC PARAMETERS (Enhanced for Mesh Scale)
# ---------------------------------------------------------------------------

struct ScienceParams {
  union {
    atomic @0 :AtomicParams;
    continuum @1 :ContinuumParams;
    kinetic @2 :KineticParams;
    math @3 :MathParams;
    json @4 :Text;
    
    # New: Cross-library operations
    crossScale @5 :CrossScaleParams;
    validation @6 :ValidationParams;
    optimization @7 :OptimizationParams;
    binary @8 :Data; # Tier 2 Polyglot: Raw binary payload
  }
}

# Placeholder structs for union fields to compile
struct CrossScaleParams {}
struct ValidationParams {}
struct OptimizationParams {}

# --- ATOMIC: Now with quantum chemistry ---
struct AtomicParams {
  union {
    # Basic operations
    loadGro @0 :Text;
    select @1 :Text;
    rmsd @2 :RmsdParams;
    centerOfMass @3 :Void;
    distance @4 :DistanceParams;
    
    # Quantum chemistry (groan_rs + extensions)
    computeEnergy @5 :QuantumEnergyParams;
    optimizeGeometry @6 :OptimizationParams;
    computeForces @7 :Void;
    molecularDynamics @8 :MDParams;
    
    # Electronic structure
    computeDensity @9 :DensityParams;
    computeOrbitals @10 :OrbitalParams;
    
    # For mesh distribution
    domainDecomposition @11 :DomainDecomposition;
  }
}

struct RmsdParams {
  reference @0 :Text;
  target @1 :Text;
}

struct DistanceParams {
  groupA @0 :Text;
  groupB @1 :Text;
}

struct QuantumEnergyParams {
  method @0 :Text;  # "dft", "hf", "mp2", "ccsd", "ci", "qmc"
  basisSet @1 :Text;
  functional @2 :Text;  # For DFT
  accuracy @3 :Float64;
  useSymmetry @4 :Bool;
}

struct MDParams {
  temperature @0 :Float64;
  pressure @1 :Float64;
  steps @2 :UInt64;
  timestep @3 :Float64;  # Femtoseconds
  thermostat @4 :Text;
  saveFrequency @5 :UInt64;
}

struct DensityParams {}
struct OrbitalParams {}

struct DomainDecomposition {
  # How to split this system across mesh nodes
  strategy @0 :Text;  # "spatial", "byMolecule", "byResidue"
  partitions @1 :UInt32;
  overlap @2 :Float64;  # Angstroms of overlap region
}

# --- CONTINUUM: Enhanced for multi-scale ---
struct ContinuumParams {
  union {
    generateMesh @0 :MeshParams;
    solveLinear @1 :SolveParams;
    computeStress @2 :Void;
    computeStrain @3 :Void;
    
    # Multi-scale extensions
    computeHomogenizedProperties @4 :HomogenizationParams;
    applyMicrostructure @5 :MicrostructureParams;
    
    # For coupling to atomic scale
    receiveQuantumForces @6 :CouplingParams;
    sendContinuumDeformation @7 :CouplingParams;
  }
}

struct MeshParams {
  resolution @0 :UInt32;
  domain @1 :Box3D;
}

struct SolveParams {
  maxIterations @0 :UInt32;
  tolerance @1 :Float32;
}

struct HomogenizationParams {
  rveSize @0 :Float64;        # Representative volume element size
  boundaryConditions @1 :Text; # "periodic", "displacement", "traction"
  computeTensor @2 :Text;     # "stiffness", "conductivity", "permeability"
}

struct MicrostructureParams {
  type @0 :Text;  # "polycrystal", "composite", "foam", "lattice"
  orientation @1 :List(Float32);  # Euler angles
  volumeFraction @2 :Float64;
}

struct CouplingParams {
  sourceScale @0 :SimulationScale;
  targetScale @1 :SimulationScale;
  mappingMethod @2 :Text;  # "interpolation", "averaging", "projection"
  tolerance @3 :Float64;
}

# --- KINETIC: Ready for billion-body simulation ---
struct KineticParams {
  union {
    createBody @0 :RigidBodyParams;
    step @1 :StepParams;
    castRay @2 :RayParams;
    
    # Mesh-scale optimizations
    createManyBodies @3 :ManyBodiesParams;
    broadPhase @4 :BroadPhaseParams;
    
    # Coupling to continuum
    applyContinuumForces @5 :ContinuumForces;
    sendKineticMotion @6 :MotionParams;
    stepParticles @7 :ParticleRealityParams;
  }
}

struct RigidBodyParams {
  position @0 :List(Float32);
  mass @1 :Float32;
  isStatic @2 :Bool;
}

struct StepParams {
  dt @0 :Float32;
  sabOffset @1 :UInt32;
  particleCount @2 :UInt32;
}

struct ParticleRealityParams {
  sabOffset @0 :UInt32;
  particleCount @1 :UInt32;
  dt @2 :Float32;
  gravity @3 :List(Float32); # [x, y, z]
}

struct RayParams {
  origin @0 :List(Float32);
  direction @1 :List(Float32);
  maxToi @2 :Float32;
}

struct ManyBodiesParams {
  positions @0 :DataRef;     # Stream of positions (Nx3)
  velocities @1 :DataRef;    # Stream of velocities (Nx3)
  masses @2 :DataRef;        # Stream of masses (N)
  collisionGroups @3 :DataRef; # Bitmask for collision filtering
  batchSize @4 :UInt32;      # How many to create per RPC
}

struct BroadPhaseParams {
  algorithm @0 :Text;  # "sweepAndPrune", "grid", "hierarchy"
  cellSize @1 :Float32;
  parallel @2 :Bool;
}

struct ContinuumForces {
  forceField @0 :DataRef;    # Grid or mesh of forces
  interpolation @1 :Text;    # How to apply to rigid bodies
  updateFrequency @2 :UInt32;
}

struct MotionParams {}

# --- MATH: Now with quantum linear algebra ---
struct MathParams {
  union {
    # Basic operations
    matrixMultiply @0 :MatMulParams;
    dotProduct @1 :Void;
    inverse @2 :Void;
    eigenvalues @3 :Void;
    
    # Quantum-aware operations
    partialTrace @4 :PartialTraceParams;
    expectationValue @5 :ExpectationParams;
    commutator @6 :CommutatorParams;
    tensorProduct @7 :TensorProductParams;
    
    # For distributed mesh
    distributedSolve @8 :DistributedSolveParams;
    reduceOperation @9 :ReduceParams;
  }
}

struct MatMulParams {
  aShape @0 :List(UInt32);
  bShape @1 :List(UInt32);
}

struct PartialTraceParams {
  subsystemsToTrace @0 :List(UInt32);
  systemDimensions @1 :List(UInt32);  # Dimensions of each subsystem
}

struct ExpectationParams {
  observable @0 :DataRef;    # Operator as matrix
  state @1 :DataRef;         # Density matrix or wavefunction
  computeVariance @2 :Bool;
}

struct CommutatorParams {
  a @0 :DataRef;
  b @1 :DataRef;
  computeAntiCommutator @2 :Bool;
}

struct TensorProductParams {}

struct DistributedSolveParams {
  matrix @0 :DataRef;        # Distributed across nodes
  rhs @1 :DataRef;
  solver @2 :Text;           # "conjugateGradient", "multigrid", "direct"
  preconditioner @3 :Text;
  maxIterations @4 :UInt32;
  tolerance @5 :Float64;
}

struct ReduceParams {
  operation @0 :Text;  # "sum", "max", "min", "mean", "norm"
  data @1 :DataRef;
  rootNode @2 :UInt64;  # Which node gets final result
}

# ---------------------------------------------------------------------------
# RESULTS & VALIDATION
# ---------------------------------------------------------------------------

struct ScienceResult {
  # Core result
  status @0 :Status;
  data @1 :DataRef;
  errorMessage @2 :Text;
  
  # For mesh validation and caching
  computationProof @3 :ComputationProof;
  performance @4 :PerformanceMetrics;
  alternatives @5 :List(DataRef);  # Other valid results (quantum superposition)
}

struct ComputationProof {
  # Proof that this computation was done correctly
  inputHash @0 :Data;
  methodHash @1 :Data;
  paramsHash @2 :Data;
  resultHash @3 :Data;
  
  # Who computed it and how
  nodeId @4 :UInt64;
  shardId @5 :UInt32;
  epoch @6 :UInt64;
  
  # For Proof-of-Simulation consensus
  verificationData @7 :DataRef;  # Random subset recomputed by verifiers
  merkleProof @8 :MerkleProof;   # Proof this is in global state
}

struct PerformanceMetrics {
  computeTime @0 :Float64;    # Seconds on node
  memoryUsed @1 :UInt64;      # Bytes
  cacheHit @2 :Bool;
  scale @3 :SimulationScale;  # What scale was actually computed at
  accuracy @4 :Float64;       # Estimated vs exact (if known)
}

struct ValidationProof {
  # Result of validating someone else's computation
  originalHash @0 :Data;
  validationMethod @1 :Text;  # "full", "spot", "crossScale"
  isValid @2 :Bool;
  
  # What the validator computed
  validatorResult @3 :DataRef;
  validatorNode @4 :UInt64;
  
  # Incentive/dispute system
  stake @5 :UInt64;           # Credits staked on validation
  reward @6 :UInt64;          # Credits earned if correct
}

# ---------------------------------------------------------------------------
# COMMON TYPES
# ---------------------------------------------------------------------------

struct Box3D {
  min @0 :List(Float32);
  max @1 :List(Float32);
}

enum Status {
  success @0;
  error @1;
  invalidParams @2;
  cacheHit @3;                # Result came from cache
  validationRequired @4;      # Needs verification before trust
  scaleMismatch @5;          # Couldn't compute at requested fidelity
}
