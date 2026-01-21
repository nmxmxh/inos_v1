@0xf1a2b3c4d5e6f7a8;
# SAB Memory Layout Constants
# This schema defines the complete SharedArrayBuffer memory layout for INOS v1.9+
# All constants are shared across Go (Kernel), Rust (Modules), and JavaScript (Frontend)

# ========== SYSTEM BASE OFFSET ==========
# Go uses Split Memory Twin pattern with explicit js.CopyBytesToGo bridging.
# No reservation zone needed - all offsets start at 0.
const offsetSystemBase :UInt32 = 0x00000000; # No Go reservation (was 16MB)

# ========== SAB SIZE LIMITS ==========

# Tier-based SAB sizing (matched to identity.capnp ResourceTier)
# Sizes are ACTUAL SAB size (no Go reservation)
# Light (Mobile/IoT): 32MB - basic entity simulation (<5k entities)
# Moderate (Laptop): 64MB - full boids + culling (10k entities)
# Heavy (Workstation): 128MB - multi-LOD + SoA (50k entities)  
# Dedicated (Server): 256MB+ - GPU-driven pipeline (100k+ entities)

const sabSizeDefault :UInt32 = 33554432;    # 32MB (32 * 1024 * 1024) - Light tier
const sabSizeMin     :UInt32 = 33554432;    # 32MB minimum
const sabSizeMax     :UInt32 = 1073741824;  # 1GB maximum

# Tier-specific SAB sizes (actual memory needed)
const sabSizeLight    :UInt32 = 33554432;   # 32MB
const sabSizeModerate :UInt32 = 67108864;   # 64MB
const sabSizeHeavy    :UInt32 = 134217728;  # 128MB
const sabSizeDedicated :UInt32 = 268435456; # 256MB

# ========== MEMORY REGION OFFSETS (ABSOLUTE) ==========
# All offsets start at 0 - no Go reservation zone

# Metadata Region (0x000000 - 0x001000)
const offsetAtomicFlags      :UInt32 = 0x00000000; # Epoch counters and atomic flags (Expanded to 1KB)
const sizeAtomicFlags        :UInt32 = 0x000400;   # 1KB (256 x i32)

const offsetSupervisorAlloc  :UInt32 = 0x00000400; # Dynamic epoch allocation table
const sizeSupervisorAlloc    :UInt32 = 0x0000B0;   # 176 bytes

const offsetRegistryLock     :UInt32 = 0x000004B0; # Global mutex for registry operations
const sizeRegistryLock       :UInt32 = 0x000010;   # 16 bytes

# Module Registry (0x000500 - 0x001D00)
const offsetModuleRegistry   :UInt32 = 0x00000500; # Module metadata and capabilities
const sizeModuleRegistry     :UInt32 = 0x001800;   # 6KB
const moduleEntrySize        :UInt32 = 96;         # Enhanced 96-byte entries
const maxModulesInline       :UInt32 = 64;         # 64 modules inline
const maxModulesTotal        :UInt32 = 1024;       # Total with arena overflow

const offsetBloomFilter      :UInt32 = 0x00001D00; # Fast module capability lookup
const sizeBloomFilter        :UInt32 = 0x000100;   # 256 bytes

# Supervisor Headers (0x002000 - 0x006000)
const offsetSupervisorHeaders :UInt32 = 0x00002000; # Supervisor state headers (Expanded for v2.0)
const sizeSupervisorHeaders   :UInt32 = 0x004000;   # 16KB (Supports 128 x 128b)
const supervisorHeaderSize    :UInt32 = 128;        
const maxSupervisorsInline    :UInt32 = 128;        
const maxSupervisorsTotal     :UInt32 = 256;        # Supports arena overflow

# Syscall Table (0x006000 - 0x007000)
const offsetSyscallTable     :UInt32 = 0x00006000; # Pending system call metadata
const sizeSyscallTable       :UInt32 = 0x001000;   # 4KB

# Mesh Metrics Region (0x007000 - 0x007100)
const offsetMeshMetrics      :UInt32 = 0x00007000; # Mesh network telemetry
const sizeMeshMetrics        :UInt32 = 0x000100;   # 256 bytes

# Global Analytics Region (0x007100 - 0x007200)
const offsetGlobalAnalytics  :UInt32 = 0x00007100; # Aggregated mesh metrics
const sizeGlobalAnalytics    :UInt32 = 0x000100;   # 256 bytes

# Economics Region (0x007200 - 0x00B000)
const offsetEconomics        :UInt32 = 0x00007200; # Credit accounts and resource metrics
const sizeEconomics          :UInt32 = 0x003E00;   # ~15.5KB

# Identity Registry (0x00B000 - 0x00F000)
const offsetIdentityRegistry :UInt32 = 0x0000B000; # DIDs, device binding, TSS metadata
const sizeIdentityRegistry   :UInt32 = 0x004000;   # 16KB

# Social Graph (0x00F000 - 0x013000)
const offsetSocialGraph      :UInt32 = 0x0000F000; # Referrals, close IDs, social yield
const sizeSocialGraph        :UInt32 = 0x004000;   # 16KB

# Pattern Exchange (0x013000 - 0x023000)
const offsetPatternExchange  :UInt32 = 0x00013000; # Learned patterns and optimizations
const sizePatternExchange    :UInt32 = 0x010000;   # 64KB
const patternEntrySize       :UInt32 = 64;         # Compact 64-byte patterns
const maxPatternsInline      :UInt32 = 1024;       # 1024 patterns inline
const maxPatternsTotal       :UInt32 = 16384;      # Total with arena overflow

# Job History (0x023000 - 0x043000)
const offsetJobHistory       :UInt32 = 0x00023000; # Job execution history (circular buffer)
const sizeJobHistory         :UInt32 = 0x020000;   # 128KB

# Coordination State (0x043000 - 0x053000)
const offsetCoordination     :UInt32 = 0x00043000; # Cross-unit coordination state
const sizeCoordination       :UInt32 = 0x010000;   # 64KB

# Inbox/Outbox (0x053000 - 0x153000)
const offsetInboxOutbox      :UInt32 = 0x00053000; # Job request/result communication
const sizeInboxOutbox        :UInt32 = 0x100000;   # 1MB total
const offsetInboxBase        :UInt32 = 0x00053000; # Inbox start
const sizeInboxTotal         :UInt32 = 0x080000;   # 512KB

# Two Outboxes to prevent Kernel/Host race conditions
const offsetOutboxHostBase   :UInt32 = 0x000D3000; # Outbox for Kernel -> Host (Results)
const sizeOutboxHostTotal    :UInt32 = 0x040000;   # 256KB
const offsetOutboxKernelBase :UInt32 = 0x00113000; # Outbox for Module -> Kernel (Syscalls)
const sizeOutboxKernelTotal  :UInt32 = 0x040000;   # 256KB

# Arena (0x153000 - end)
const offsetArena            :UInt32 = 0x00153000; # Dynamic allocation for overflow and large data

# Arena Internal Layout
const offsetArenaMetadata    :UInt32 = 0x00153000; # Arena metadata
const sizeArenaMetadata      :UInt32 = 0x010000;   # 64KB reserved for metadata

const offsetDiagnostics      :UInt32 = 0x00153000; # Diagnostics region
const sizeDiagnostics        :UInt32 = 0x001000;   # 4KB

const offsetBridgeMetrics    :UInt32 = 0x00153800; # Performance metrics for SAB bridge
const sizeBridgeMetrics      :UInt32 = 0x000100;   # 256 bytes

const offsetArenaRequestQueue  :UInt32 = 0x00154000; # Async allocation requests
const offsetArenaResponseQueue :UInt32 = 0x00155000; # Async allocation responses
const arenaQueueEntrySize      :UInt32 = 64;
const maxArenaRequests         :UInt32 = 64;

# Mesh Event Stream (Ring Buffer)
const offsetMeshEventQueue   :UInt32 = 0x00156000; # Event payload slots
const sizeMeshEventQueue     :UInt32 = 0x000D000;  # 52KB (52 slots * 1KB)
const meshEventSlotSize      :UInt32 = 0x000400;  # 1KB per slot
const meshEventSlotCount     :UInt32 = 52;        # Power-of-two not required (monotonic counters)

# Bird Animation State
const offsetBirdState        :UInt32 = 0x00163000; # Bird state metadata
const sizeBirdState          :UInt32 = 0x001000;   # 4KB

# ========== PING-PONG BUFFERS (Arena) ==========

# Control Block
const offsetPingpongControl  :UInt32 = 0x00164000; # Ping-pong coordination
const sizePingpongControl    :UInt32 = 0x000040;   # 64 bytes

# Bird Population Data (Dual Buffers)
const offsetBirdBufferA      :UInt32 = 0x00165000;
const offsetBirdBufferB      :UInt32 = 0x003C5000;
const sizeBirdBuffer         :UInt32 = 2360000;    # 10000 * 236
const birdStride             :UInt32 = 236;

# Matrix Output Data (Dual Buffers)
const offsetMatrixBufferA    :UInt32 = 0x00625000;
const offsetMatrixBufferB    :UInt32 = 0x00B25000;
const sizeMatrixBuffer       :UInt32 = 5120000;    # 10000 * 8 * 64
const matrixStride           :UInt32 = 64;

# ========== EPOCH INDEX ALLOCATION ==========

# --- 1. CORE KERNEL SIGNALS (0-7) ---
const idxKernelReady         :UInt32 = 0;  # Kernel boot complete
const idxInboxDirty          :UInt32 = 1;  # Signal from Kernel to Module
const idxOutboxHostDirty     :UInt32 = 2;  # Signal from Kernel to Host (Results)
const idxOutboxKernelDirty   :UInt32 = 3;  # Signal from Module to Kernel (Syscalls)
const idxPanicState          :UInt32 = 4;  # System panic
const idxSensorEpoch         :UInt32 = 5;  # Sensor updates
const idxActorEpoch          :UInt32 = 6;  # Actor updates
const idxStorageEpoch        :UInt32 = 7;  # Storage updates

# --- 2. AUTONOMOUS SYSTEM PULSE (8-15) ---
const idxSystemPulse         :UInt32 = 8;  # Main thread increments each rAF (Heartbeat) - Optional
const idxSystemVisibility    :UInt32 = 9;  # 1=Visible, 0=Hidden/Suspended
const idxSystemPowerState    :UInt32 = 10; # Throttle control (HighPerf, Balanced, LowPower)
const idxOutboxMutex         :UInt32 = 9;  # Alias for Visibility (Compatibility)
const idxInboxMutex          :UInt32 = 10; # Alias for PowerState (Compatibility)
const idxSystemEpoch         :UInt32 = 11; # General system state change
const idxReservedPulse1      :UInt32 = 12;
const idxReservedPulse2      :UInt32 = 13;
const idxReservedPulse3      :UInt32 = 14;
const idxReservedPulse4      :UInt32 = 15;

# --- 3. RESOURCE & ECONOMY SIGNALS (16-23) ---
const idxEconomyEpoch        :UInt32 = 16; # Credit settlement needed
const idxMetricsEpoch        :UInt32 = 17; # Local metrics updated
const idxHealthEpoch         :UInt32 = 18; # Health status changed
const idxRegistryEpoch       :UInt32 = 19; # Module registration signal
const idxGlobalMetricsEpoch  :UInt32 = 20; # Mesh-wide diagnostics complete
const idxReservedRes1        :UInt32 = 21;
const idxReservedRes2        :UInt32 = 22;
const idxReservedRes3        :UInt32 = 23;

# --- 4. COMPUTE & PIPELINE SIGNALS (24-31) ---
const idxBirdEpoch           :UInt32 = 24; # Bird physics complete
const idxMatrixEpoch         :UInt32 = 25; # Matrix generation complete
const idxPingpongActive      :UInt32 = 26; # Active buffer (0=A, 1=B)
const idxEvolutionEpoch      :UInt32 = 27; # Boids evolution cycle complete
const idxLearningEpoch       :UInt32 = 28; # Pattern learning complete
const idxBirdCount           :UInt32 = 29; # Active bird count (mutable)
const idxContextIdHash       :UInt32 = 31; # Hash of initialization context ID

# --- 5. MESH & DELEGATION SIGNALS (32-47) ---
const idxDelegatedJobEpoch   :UInt32 = 32; # Remote job delegation complete
const idxUserJobEpoch        :UInt32 = 33; # Local user job complete
const idxDelegatedChunkEpoch :UInt32 = 34; # Remote chunk fetch/store complete
const idxMeshEventEpoch      :UInt32 = 35; # Mesh event stream updated
const idxMeshEventHead       :UInt32 = 36; # Consumer head (monotonic)
const idxMeshEventTail       :UInt32 = 37; # Producer tail (monotonic)
const idxMeshEventDropped    :UInt32 = 38; # Dropped event counter

# --- 6. WORKER CONTROL & DYNAMIC POOL (64-255) ---
const idxArenaAllocator      :UInt32 = 64; # Arena bump pointer (atomic)
const supervisorPoolBase     :UInt32 = 65; # Pool for individual worker ACKs/Signals
const supervisorPoolSize     :UInt32 = 128; # Supports 128 concurrent supervisor signals

# Reserved for future expansion (128-255)
const reservedPoolBase       :UInt32 = 128;
const reservedPoolSize       :UInt32 = 128;

# ========== ALIGNMENT REQUIREMENTS ==========

const alignmentCacheLine     :UInt32 = 64;    # Cache line alignment
const alignmentPage          :UInt32 = 4096;  # Page alignment
const alignmentLarge         :UInt32 = 65536; # Large allocation alignment
