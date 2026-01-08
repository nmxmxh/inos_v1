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

# Economics Region (0x004000 - 0x008000)
const offsetEconomics        :UInt32 = 0x00004000; # Credit accounts and resource metrics
const sizeEconomics          :UInt32 = 0x004000;   # 16KB

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
const offsetOutboxBase       :UInt32 = 0x000D0000; # Outbox start
const sizeOutboxTotal        :UInt32 = 0x080000;   # 512KB

# Arena (0x150000 - end)
const offsetArena            :UInt32 = 0x00150000; # Dynamic allocation for overflow and large data

# Arena Internal Layout
const offsetArenaMetadata    :UInt32 = 0x00150000; # Arena metadata
const sizeArenaMetadata      :UInt32 = 0x010000;   # 64KB reserved for metadata

const offsetDiagnostics      :UInt32 = 0x00150000; # Diagnostics region
const sizeDiagnostics        :UInt32 = 0x001000;   # 4KB

const offsetArenaRequestQueue  :UInt32 = 0x00151000; # Async allocation requests
const offsetArenaResponseQueue :UInt32 = 0x00152000; # Async allocation responses
const arenaQueueEntrySize      :UInt32 = 64;
const maxArenaRequests         :UInt32 = 64;

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

# Fixed system epochs (0-31 Reserved)
const idxKernelReady         :UInt32 = 0;  # Kernel boot complete
const idxInboxDirty          :UInt32 = 1;  # Signal from Kernel to Module
const idxOutboxDirty         :UInt32 = 2;  # Signal from Module to Kernel
const idxPanicState          :UInt32 = 3;  # System panic
const idxSensorEpoch         :UInt32 = 4;  # Sensor updates
const idxActorEpoch          :UInt32 = 5;  # Actor updates
const idxStorageEpoch        :UInt32 = 6;  # Storage updates
const idxSystemEpoch         :UInt32 = 7;  # System updates

# Extended System Epochs
const idxArenaAllocator      :UInt32 = 8;  # Arena bump pointer (atomic)
const idxOutboxMutex         :UInt32 = 9;  # Mutex for outbox synchronization
const idxInboxMutex          :UInt32 = 10; # Mutex for inbox synchronization
const idxMetricsEpoch        :UInt32 = 11; # Metrics updated
const idxBirdEpoch           :UInt32 = 12; # Bird physics complete (was idxBoidsCount in Go)
const idxMatrixEpoch         :UInt32 = 13; # Matrix generation complete
const idxPingpongActive      :UInt32 = 14; # Active buffer (0=A, 1=B)

# Signal-Based Architecture Epochs
const idxRegistryEpoch       :UInt32 = 15; # Module registration signal
const idxEvolutionEpoch      :UInt32 = 16; # Boids evolution complete
const idxHealthEpoch         :UInt32 = 17; # Health metrics updated
const idxLearningEpoch       :UInt32 = 18; # Pattern learning complete
const idxEconomyEpoch        :UInt32 = 19; # Credit settlement needed
const idxBirdCount           :UInt32 = 20; # Active bird count (mutable)

# Context Verification (Zero-Copy)
const idxContextIdHash       :UInt32 = 31; # Hash of initialization context ID

# Dynamic supervisor pool (32-127)
const supervisorPoolBase     :UInt32 = 32;
const supervisorPoolSize     :UInt32 = 96;  # Supports 96 supervisors

# Reserved for future expansion (128-255)
const reservedPoolBase       :UInt32 = 128;
const reservedPoolSize       :UInt32 = 128;

# ========== ALIGNMENT REQUIREMENTS ==========

const alignmentCacheLine     :UInt32 = 64;    # Cache line alignment
const alignmentPage          :UInt32 = 4096;  # Page alignment
const alignmentLarge         :UInt32 = 65536; # Large allocation alignment
