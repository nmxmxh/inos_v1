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

# Metadata Region (0x000000 - 0x000100)
const offsetAtomicFlags      :UInt32 = 0x00000000; # Epoch counters and atomic flags
const sizeAtomicFlags        :UInt32 = 0x000080;   # 128 bytes (32 x i32)

const offsetSupervisorAlloc  :UInt32 = 0x00000080; # Dynamic epoch allocation table
const sizeSupervisorAlloc    :UInt32 = 0x0000B0;   # 176 bytes (Ends at 0x130)

const offsetRegistryLock     :UInt32 = 0x00000130; # Global mutex for registry operations
const sizeRegistryLock       :UInt32 = 0x000010;   # 16 bytes

# Module Registry (0x000140 - 0x001940)
const offsetModuleRegistry   :UInt32 = 0x00000140; # Module metadata and capabilities
const sizeModuleRegistry     :UInt32 = 0x001800;   # 6KB
const moduleEntrySize        :UInt32 = 96;         # Enhanced 96-byte entries
const maxModulesInline       :UInt32 = 64;         # 64 modules inline
const maxModulesTotal        :UInt32 = 1024;       # Total with arena overflow

const offsetBloomFilter      :UInt32 = 0x00001940; # Fast module capability lookup
const sizeBloomFilter        :UInt32 = 0x000100;   # 256 bytes

# Supervisor Headers (0x002000 - 0x003000)
const offsetSupervisorHeaders :UInt32 = 0x00002000; # Supervisor state headers
const sizeSupervisorHeaders   :UInt32 = 0x001000;   # 4KB
const supervisorHeaderSize    :UInt32 = 128;        # Compact 128-byte headers
const maxSupervisorsInline    :UInt32 = 32;         # 32 supervisors inline
const maxSupervisorsTotal     :UInt32 = 256;        # Total with arena overflow

# Syscall Table (0x003000 - 0x004000)
const offsetSyscallTable     :UInt32 = 0x00003000; # Pending system call metadata
const sizeSyscallTable       :UInt32 = 0x001000;   # 4KB

# Mesh Metrics Region (0x004000 - 0x004100)
const offsetMeshMetrics      :UInt32 = 0x00004000; # Mesh network telemetry
const sizeMeshMetrics        :UInt32 = 0x000100;   # 256 bytes

# Global Analytics Region (0x004100 - 0x004200)
const offsetGlobalAnalytics  :UInt32 = 0x00004100; # Aggregated mesh metrics
const sizeGlobalAnalytics    :UInt32 = 0x000100;   # 256 bytes

# Economics Region (0x004200 - 0x008000)
const offsetEconomics        :UInt32 = 0x00004200; # Credit accounts and resource metrics
const sizeEconomics          :UInt32 = 0x003E00;   # ~15.5KB

# Identity Registry (0x008000 - 0x00C000)
const offsetIdentityRegistry :UInt32 = 0x00008000; # DIDs, device binding, TSS metadata
const sizeIdentityRegistry   :UInt32 = 0x004000;   # 16KB

# Social Graph (0x00C000 - 0x010000)
const offsetSocialGraph      :UInt32 = 0x0000C000; # Referrals, close IDs, social yield
const sizeSocialGraph        :UInt32 = 0x004000;   # 16KB

# Pattern Exchange (0x010000 - 0x020000)
const offsetPatternExchange  :UInt32 = 0x00010000; # Learned patterns and optimizations
const sizePatternExchange    :UInt32 = 0x010000;   # 64KB
const patternEntrySize       :UInt32 = 64;         # Compact 64-byte patterns
const maxPatternsInline      :UInt32 = 1024;       # 1024 patterns inline
const maxPatternsTotal       :UInt32 = 16384;      # Total with arena overflow

# Job History (0x020000 - 0x040000)
const offsetJobHistory       :UInt32 = 0x00020000; # Job execution history (circular buffer)
const sizeJobHistory         :UInt32 = 0x020000;   # 128KB

# Coordination State (0x040000 - 0x050000)
const offsetCoordination     :UInt32 = 0x00040000; # Cross-unit coordination state
const sizeCoordination       :UInt32 = 0x010000;   # 64KB

# Inbox/Outbox (0x050000 - 0x150000)
const offsetInboxOutbox      :UInt32 = 0x00050000; # Job request/result communication
const sizeInboxOutbox        :UInt32 = 0x100000;   # 1MB total
const offsetInboxBase        :UInt32 = 0x00050000; # Inbox start
const sizeInboxTotal         :UInt32 = 0x080000;   # 512KB

# Two Outboxes to prevent Kernel/Host race conditions
const offsetOutboxHostBase   :UInt32 = 0x000D0000; # Outbox for Kernel -> Host (Results)
const sizeOutboxHostTotal    :UInt32 = 0x040000;   # 256KB
const offsetOutboxKernelBase :UInt32 = 0x00110000; # Outbox for Module -> Kernel (Syscalls)
const sizeOutboxKernelTotal  :UInt32 = 0x040000;   # 256KB

# Arena (0x150000 - end)
const offsetArena            :UInt32 = 0x00150000; # Dynamic allocation for overflow and large data

# Arena Internal Layout
const offsetArenaMetadata    :UInt32 = 0x00150000; # Arena metadata
const sizeArenaMetadata      :UInt32 = 0x010000;   # 64KB reserved for metadata

const offsetDiagnostics      :UInt32 = 0x00150000; # Diagnostics region
const sizeDiagnostics        :UInt32 = 0x001000;   # 4KB

const offsetBridgeMetrics    :UInt32 = 0x00150800; # Performance metrics for SAB bridge
const sizeBridgeMetrics      :UInt32 = 0x000100;   # 256 bytes

const offsetArenaRequestQueue  :UInt32 = 0x00151000; # Async allocation requests
const offsetArenaResponseQueue :UInt32 = 0x00152000; # Async allocation responses
const arenaQueueEntrySize      :UInt32 = 64;
const maxArenaRequests         :UInt32 = 64;

# Mesh Event Stream (Ring Buffer)
const offsetMeshEventQueue   :UInt32 = 0x00153000; # Event payload slots
const sizeMeshEventQueue     :UInt32 = 0x000D000;  # 52KB (52 slots * 1KB)
const meshEventSlotSize      :UInt32 = 0x000400;  # 1KB per slot
const meshEventSlotCount     :UInt32 = 52;        # Power-of-two not required (monotonic counters)

# Bird Animation State
const offsetBirdState        :UInt32 = 0x00160000; # Bird state metadata
const sizeBirdState          :UInt32 = 0x001000;   # 4KB

# ========== PING-PONG BUFFERS (Arena) ==========

# Control Block
const offsetPingpongControl  :UInt32 = 0x00161000; # Ping-pong coordination
const sizePingpongControl    :UInt32 = 0x000040;   # 64 bytes

# Bird Population Data (Dual Buffers)
const offsetBirdBufferA      :UInt32 = 0x00162000;
const offsetBirdBufferB      :UInt32 = 0x003C2000;
const sizeBirdBuffer         :UInt32 = 2360000;    # 10000 * 236
const birdStride             :UInt32 = 236;

# Matrix Output Data (Dual Buffers)
const offsetMatrixBufferA    :UInt32 = 0x00622000;
const offsetMatrixBufferB    :UInt32 = 0x00B22000;
const sizeMatrixBuffer       :UInt32 = 5120000;    # 10000 * 8 * 64
const matrixStride           :UInt32 = 64;

# ========== EPOCH INDEX ALLOCATION ==========

# ========== EPOCH INDEX ALLOCATION ==========

# --- 1. FIXED SYSTEM SIGNALS (0-7) ---
const idxKernelReady         :UInt32 = 0;  # Kernel boot complete
const idxInboxDirty          :UInt32 = 1;  # Signal from Kernel to Module
const idxOutboxHostDirty     :UInt32 = 2;  # Signal from Kernel to Host (Results)
const idxPanicState          :UInt32 = 3;  # System panic
const idxSensorEpoch         :UInt32 = 4;  # Sensor updates
const idxActorEpoch          :UInt32 = 5;  # Actor updates
const idxStorageEpoch        :UInt32 = 6;  # Storage updates
const idxSystemEpoch         :UInt32 = 7;  # System updates

# --- 2. AUTONOMOUS SYSTEM PULSE (8-15) ---
const idxSystemPulse         :UInt32 = 8;  # Global high-precision pulse
const idxSystemVisibility    :UInt32 = 9;  # 1 = Visible, 0 = Hidden
const idxSystemPowerState    :UInt32 = 10; # 0 = Low, 1 = Normal, 2 = High
const idxReservedPulse1      :UInt32 = 11;
const idxReservedPulse2      :UInt32 = 12;
const idxReservedPulse3      :UInt32 = 13;
const idxReservedPulse4      :UInt32 = 14;
const idxReservedPulse5      :UInt32 = 15;

# --- 3. EXTENDED SYSTEM SIGNALS (16-31) ---
const idxArenaAllocator      :UInt32 = 16; # Arena bump pointer (atomic)
const idxOutboxMutex         :UInt32 = 17; # Mutex for outbox synchronization (unused)
const idxInboxMutex          :UInt32 = 18; # Mutex for inbox synchronization (unused)
const idxMetricsEpoch        :UInt32 = 19; # Metrics updated
const idxBirdEpoch           :UInt32 = 20; # Bird physics complete
const idxMatrixEpoch         :UInt32 = 21; # Matrix generation complete
const idxPingpongActive      :UInt32 = 22; # Active buffer (0=A, 1=B)
const idxRegistryEpoch       :UInt32 = 23; # Module registration signal
const idxEvolutionEpoch      :UInt32 = 24; # Boids evolution complete
const idxHealthEpoch         :UInt32 = 25; # Health metrics updated
const idxLearningEpoch       :UInt32 = 26; # Pattern learning complete
const idxEconomyEpoch        :UInt32 = 27; # Credit settlement needed
const idxBirdCount           :UInt32 = 28; # Active bird count (mutable)
const idxGlobalMetricsEpoch  :UInt32 = 29; # Global diagnostics complete
const idxOutboxKernelDirty   :UInt32 = 30; # Signal from Module to Kernel (Syscalls)
const idxContextIdHash       :UInt32 = 31; # Hash of initialization context ID

# --- 4. MESH & DELEGATION SIGNALS (32-47) ---
const idxDelegatedJobEpoch   :UInt32 = 32; # Remote job delegation complete
const idxUserJobEpoch        :UInt32 = 33; # Local user job complete
const idxDelegatedChunkEpoch :UInt32 = 34; # Remote chunk fetch/store complete
const idxMeshEventEpoch      :UInt32 = 35; # Mesh event stream updated
const idxMeshEventHead       :UInt32 = 36; # Consumer head (monotonic)
const idxMeshEventTail       :UInt32 = 37; # Producer tail (monotonic)
const idxMeshEventDropped    :UInt32 = 38; # Dropped event counter

# --- 5. WORKER CONTROL & DYNAMIC POOL (64-255) ---
const supervisorPoolBase     :UInt32 = 64;
const supervisorPoolSize     :UInt32 = 128; # Supports 128 supervisors

# Reserved for future expansion (128-255)
const reservedPoolBase       :UInt32 = 128;
const reservedPoolSize       :UInt32 = 128;

# ========== ALIGNMENT REQUIREMENTS ==========

const alignmentCacheLine     :UInt32 = 64;    # Cache line alignment
const alignmentPage          :UInt32 = 4096;  # Page alignment
const alignmentLarge         :UInt32 = 65536; # Large allocation alignment
