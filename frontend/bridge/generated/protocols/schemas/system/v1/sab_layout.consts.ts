/**
 * Cap'n Proto Constants - Auto-Generated
 * 
 * Source: protocols/schemas/system/v1/sab_layout.capnp
 * File ID: 0xf1a2b3c4d5e6f7a8
 * 
 * DO NOT EDIT MANUALLY - Regenerate with: make proto
 * 
 * @generated
 */

/** No Go reservation (was 16MB) */
export const OFFSET_SYSTEM_BASE = 0 as const;

/** 32MB (32 * 1024 * 1024) - Light tier */
export const SAB_SIZE_DEFAULT = 0x2000000 as const;

/** 32MB minimum */
export const SAB_SIZE_MIN = 0x2000000 as const;

/** 1GB maximum */
export const SAB_SIZE_MAX = 0x40000000 as const;

/** 32MB */
export const SAB_SIZE_LIGHT = 0x2000000 as const;

/** 64MB */
export const SAB_SIZE_MODERATE = 0x4000000 as const;

/** 128MB */
export const SAB_SIZE_HEAVY = 0x8000000 as const;

/** 256MB */
export const SAB_SIZE_DEDICATED = 0x10000000 as const;

/** Epoch counters and atomic flags */
export const OFFSET_ATOMIC_FLAGS = 0 as const;

/** 128 bytes (32 x i32) */
export const SIZE_ATOMIC_FLAGS = 128 as const;

/** Dynamic epoch allocation table */
export const OFFSET_SUPERVISOR_ALLOC = 128 as const;

/** 176 bytes (Ends at 0x130) */
export const SIZE_SUPERVISOR_ALLOC = 176 as const;

/** Global mutex for registry operations */
export const OFFSET_REGISTRY_LOCK = 304 as const;

/** 16 bytes */
export const SIZE_REGISTRY_LOCK = 16 as const;

/** Module metadata and capabilities */
export const OFFSET_MODULE_REGISTRY = 320 as const;

/** 6KB */
export const SIZE_MODULE_REGISTRY = 0x001800 as const;

/** Enhanced 96-byte entries */
export const MODULE_ENTRY_SIZE = 96 as const;

/** 64 modules inline */
export const MAX_MODULES_INLINE = 64 as const;

/** Total with arena overflow */
export const MAX_MODULES_TOTAL = 1024 as const;

/** Fast module capability lookup */
export const OFFSET_BLOOM_FILTER = 0x001940 as const;

/** 256 bytes */
export const SIZE_BLOOM_FILTER = 256 as const;

/** Supervisor state headers */
export const OFFSET_SUPERVISOR_HEADERS = 0x002000 as const;

/** 4KB */
export const SIZE_SUPERVISOR_HEADERS = 0x001000 as const;

/** Compact 128-byte headers */
export const SUPERVISOR_HEADER_SIZE = 128 as const;

/** 32 supervisors inline */
export const MAX_SUPERVISORS_INLINE = 32 as const;

/** Total with arena overflow */
export const MAX_SUPERVISORS_TOTAL = 256 as const;

/** Pending system call metadata */
export const OFFSET_SYSCALL_TABLE = 0x003000 as const;

/** 4KB */
export const SIZE_SYSCALL_TABLE = 0x001000 as const;

/** Mesh network telemetry */
export const OFFSET_MESH_METRICS = 0x004000 as const;

/** 256 bytes */
export const SIZE_MESH_METRICS = 256 as const;

/** Aggregated mesh metrics */
export const OFFSET_GLOBAL_ANALYTICS = 0x004100 as const;

/** 256 bytes */
export const SIZE_GLOBAL_ANALYTICS = 256 as const;

/** Credit accounts and resource metrics */
export const OFFSET_ECONOMICS = 0x004200 as const;

/** ~15.5KB */
export const SIZE_ECONOMICS = 0x003E00 as const;

/** DIDs, device binding, TSS metadata */
export const OFFSET_IDENTITY_REGISTRY = 0x008000 as const;

/** 16KB */
export const SIZE_IDENTITY_REGISTRY = 0x004000 as const;

/** Referrals, close IDs, social yield */
export const OFFSET_SOCIAL_GRAPH = 0x00C000 as const;

/** 16KB */
export const SIZE_SOCIAL_GRAPH = 0x004000 as const;

/** Learned patterns and optimizations */
export const OFFSET_PATTERN_EXCHANGE = 0x010000 as const;

/** 64KB */
export const SIZE_PATTERN_EXCHANGE = 0x010000 as const;

/** Compact 64-byte patterns */
export const PATTERN_ENTRY_SIZE = 64 as const;

/** 1024 patterns inline */
export const MAX_PATTERNS_INLINE = 1024 as const;

/** Total with arena overflow */
export const MAX_PATTERNS_TOTAL = 0x004000 as const;

/** Job execution history (circular buffer) */
export const OFFSET_JOB_HISTORY = 0x020000 as const;

/** 128KB */
export const SIZE_JOB_HISTORY = 0x020000 as const;

/** Cross-unit coordination state */
export const OFFSET_COORDINATION = 0x040000 as const;

/** 64KB */
export const SIZE_COORDINATION = 0x010000 as const;

/** Job request/result communication */
export const OFFSET_INBOX_OUTBOX = 0x050000 as const;

/** 1MB total */
export const SIZE_INBOX_OUTBOX = 0x100000 as const;

/** Inbox start */
export const OFFSET_INBOX_BASE = 0x050000 as const;

/** 512KB */
export const SIZE_INBOX_TOTAL = 0x080000 as const;

/** Outbox for Kernel -> Host (Results) */
export const OFFSET_OUTBOX_HOST_BASE = 0x0D0000 as const;

/** 256KB */
export const SIZE_OUTBOX_HOST_TOTAL = 0x040000 as const;

/** Outbox for Module -> Kernel (Syscalls) */
export const OFFSET_OUTBOX_KERNEL_BASE = 0x110000 as const;

/** 256KB */
export const SIZE_OUTBOX_KERNEL_TOTAL = 0x040000 as const;

/** Dynamic allocation for overflow and large data */
export const OFFSET_ARENA = 0x150000 as const;

/** Arena metadata */
export const OFFSET_ARENA_METADATA = 0x150000 as const;

/** 64KB reserved for metadata */
export const SIZE_ARENA_METADATA = 0x010000 as const;

/** Diagnostics region */
export const OFFSET_DIAGNOSTICS = 0x150000 as const;

/** 4KB */
export const SIZE_DIAGNOSTICS = 0x001000 as const;

/** Performance metrics for SAB bridge */
export const OFFSET_BRIDGE_METRICS = 0x150800 as const;

/** 256 bytes */
export const SIZE_BRIDGE_METRICS = 256 as const;

/** Async allocation requests */
export const OFFSET_ARENA_REQUEST_QUEUE = 0x151000 as const;

/** Async allocation responses */
export const OFFSET_ARENA_RESPONSE_QUEUE = 0x152000 as const;

/** arenaQueueEntrySize */
export const ARENA_QUEUE_ENTRY_SIZE = 64 as const;

/** maxArenaRequests */
export const MAX_ARENA_REQUESTS = 64 as const;

/** Event payload slots */
export const OFFSET_MESH_EVENT_QUEUE = 0x153000 as const;

/** 52KB (52 slots * 1KB) */
export const SIZE_MESH_EVENT_QUEUE = 0x00C000 as const;

/** 1KB per slot */
export const MESH_EVENT_SLOT_SIZE = 1024 as const;

/** Power-of-two not required (monotonic counters) */
export const MESH_EVENT_SLOT_COUNT = 48 as const;

/** Guard table entries (within Arena metadata) */
export const OFFSET_REGION_GUARDS = 0x15F000 as const;

/** 4KB (256 entries * 16B) */
export const SIZE_REGION_GUARDS = 0x001000 as const;

export const REGION_GUARD_ENTRY_SIZE = 16 as const;
export const REGION_GUARD_COUNT = 256 as const;

/** Bird state metadata */
export const OFFSET_BIRD_STATE = 0x160000 as const;

/** 4KB */
export const SIZE_BIRD_STATE = 0x001000 as const;

/** Ping-pong coordination */
export const OFFSET_PINGPONG_CONTROL = 0x161000 as const;

/** 64 bytes */
export const SIZE_PINGPONG_CONTROL = 64 as const;

/** offsetBirdBufferA */
export const OFFSET_BIRD_BUFFER_A = 0x162000 as const;

/** offsetBirdBufferB */
export const OFFSET_BIRD_BUFFER_B = 0x3C2000 as const;

/** 10000 * 236 */
export const SIZE_BIRD_BUFFER = 0x2402C0 as const;

/** birdStride */
export const BIRD_STRIDE = 236 as const;

/** offsetMatrixBufferA */
export const OFFSET_MATRIX_BUFFER_A = 0x622000 as const;

/** offsetMatrixBufferB */
export const OFFSET_MATRIX_BUFFER_B = 0xB22000 as const;

/** 10000 * 8 * 64 */
export const SIZE_MATRIX_BUFFER = 0x4E2000 as const;

/** matrixStride */
export const MATRIX_STRIDE = 64 as const;

/** Kernel boot complete */
export const IDX_KERNEL_READY = 0 as const;

/** Signal from Kernel to Module */
export const IDX_INBOX_DIRTY = 1 as const;

/** Signal from Kernel to Host (Results) */
export const IDX_OUTBOX_HOST_DIRTY = 2 as const;

/** System panic */
export const IDX_PANIC_STATE = 3 as const;

/** Sensor updates */
export const IDX_SENSOR_EPOCH = 4 as const;

/** Actor updates */
export const IDX_ACTOR_EPOCH = 5 as const;

/** Storage updates */
export const IDX_STORAGE_EPOCH = 6 as const;

/** System updates */
export const IDX_SYSTEM_EPOCH = 7 as const;

/** Global high-precision pulse */
export const IDX_SYSTEM_PULSE = 8 as const;

/** 1 = Visible, 0 = Hidden */
export const IDX_SYSTEM_VISIBILITY = 9 as const;

/** 0 = Low, 1 = Normal, 2 = High */
export const IDX_SYSTEM_POWER_STATE = 10 as const;

/** idxReservedPulse1 */
export const IDX_RESERVED_PULSE1 = 11 as const;

/** idxReservedPulse2 */
export const IDX_RESERVED_PULSE2 = 12 as const;

/** idxReservedPulse3 */
export const IDX_RESERVED_PULSE3 = 13 as const;

/** idxReservedPulse4 */
export const IDX_RESERVED_PULSE4 = 14 as const;

/** idxReservedPulse5 */
export const IDX_RESERVED_PULSE5 = 15 as const;

/** Arena bump pointer (atomic) */
export const IDX_ARENA_ALLOCATOR = 16 as const;

/** Mutex for outbox synchronization (unused) */
export const IDX_OUTBOX_MUTEX = 17 as const;

/** Mutex for inbox synchronization (unused) */
export const IDX_INBOX_MUTEX = 18 as const;

/** Metrics updated */
export const IDX_METRICS_EPOCH = 19 as const;

/** Bird physics complete */
export const IDX_BIRD_EPOCH = 20 as const;

/** Matrix generation complete */
export const IDX_MATRIX_EPOCH = 21 as const;

/** Active buffer (0=A, 1=B) */
export const IDX_PINGPONG_ACTIVE = 22 as const;

/** Module registration signal */
export const IDX_REGISTRY_EPOCH = 23 as const;

/** Boids evolution complete */
export const IDX_EVOLUTION_EPOCH = 24 as const;

/** Health metrics updated */
export const IDX_HEALTH_EPOCH = 25 as const;

/** Pattern learning complete */
export const IDX_LEARNING_EPOCH = 26 as const;

/** Credit settlement needed */
export const IDX_ECONOMY_EPOCH = 27 as const;

/** Active bird count (mutable) */
export const IDX_BIRD_COUNT = 28 as const;

/** Global diagnostics complete */
export const IDX_GLOBAL_METRICS_EPOCH = 29 as const;

/** Signal from Module to Kernel (Syscalls) */
export const IDX_OUTBOX_KERNEL_DIRTY = 30 as const;

/** Hash of initialization context ID */
export const IDX_CONTEXT_ID_HASH = 31 as const;

/** Remote job delegation complete */
export const IDX_DELEGATED_JOB_EPOCH = 32 as const;

/** Local user job complete */
export const IDX_USER_JOB_EPOCH = 33 as const;

/** Remote chunk fetch/store complete */
export const IDX_DELEGATED_CHUNK_EPOCH = 34 as const;

/** Mesh event stream updated */
export const IDX_MESH_EVENT_EPOCH = 35 as const;

/** Consumer head (monotonic) */
export const IDX_MESH_EVENT_HEAD = 36 as const;

/** Producer tail (monotonic) */
export const IDX_MESH_EVENT_TAIL = 37 as const;

/** Dropped event counter */
export const IDX_MESH_EVENT_DROPPED = 38 as const;

/** supervisorPoolBase */
export const SUPERVISOR_POOL_BASE = 64 as const;

/** Supports 128 supervisors */
export const SUPERVISOR_POOL_SIZE = 128 as const;

/** reservedPoolBase */
export const RESERVED_POOL_BASE = 128 as const;

/** reservedPoolSize */
export const RESERVED_POOL_SIZE = 128 as const;

/** Cache line alignment */
export const ALIGNMENT_CACHE_LINE = 64 as const;

/** Page alignment */
export const ALIGNMENT_PAGE = 0x001000 as const;

/** Large allocation alignment */
export const ALIGNMENT_LARGE = 0x010000 as const;

// ========== UNIFIED CONSTANTS OBJECT ==========

export const CONSTS = {
  OFFSET_SYSTEM_BASE,
  SAB_SIZE_DEFAULT,
  SAB_SIZE_MIN,
  SAB_SIZE_MAX,
  SAB_SIZE_LIGHT,
  SAB_SIZE_MODERATE,
  SAB_SIZE_HEAVY,
  SAB_SIZE_DEDICATED,
  OFFSET_ATOMIC_FLAGS,
  SIZE_ATOMIC_FLAGS,
  OFFSET_SUPERVISOR_ALLOC,
  SIZE_SUPERVISOR_ALLOC,
  OFFSET_REGISTRY_LOCK,
  SIZE_REGISTRY_LOCK,
  OFFSET_MODULE_REGISTRY,
  SIZE_MODULE_REGISTRY,
  MODULE_ENTRY_SIZE,
  MAX_MODULES_INLINE,
  MAX_MODULES_TOTAL,
  OFFSET_BLOOM_FILTER,
  SIZE_BLOOM_FILTER,
  OFFSET_SUPERVISOR_HEADERS,
  SIZE_SUPERVISOR_HEADERS,
  SUPERVISOR_HEADER_SIZE,
  MAX_SUPERVISORS_INLINE,
  MAX_SUPERVISORS_TOTAL,
  OFFSET_SYSCALL_TABLE,
  SIZE_SYSCALL_TABLE,
  OFFSET_MESH_METRICS,
  SIZE_MESH_METRICS,
  OFFSET_GLOBAL_ANALYTICS,
  SIZE_GLOBAL_ANALYTICS,
  OFFSET_ECONOMICS,
  SIZE_ECONOMICS,
  OFFSET_IDENTITY_REGISTRY,
  SIZE_IDENTITY_REGISTRY,
  OFFSET_SOCIAL_GRAPH,
  SIZE_SOCIAL_GRAPH,
  OFFSET_PATTERN_EXCHANGE,
  SIZE_PATTERN_EXCHANGE,
  PATTERN_ENTRY_SIZE,
  MAX_PATTERNS_INLINE,
  MAX_PATTERNS_TOTAL,
  OFFSET_JOB_HISTORY,
  SIZE_JOB_HISTORY,
  OFFSET_COORDINATION,
  SIZE_COORDINATION,
  OFFSET_INBOX_OUTBOX,
  SIZE_INBOX_OUTBOX,
  OFFSET_INBOX_BASE,
  SIZE_INBOX_TOTAL,
  OFFSET_OUTBOX_HOST_BASE,
  SIZE_OUTBOX_HOST_TOTAL,
  OFFSET_OUTBOX_KERNEL_BASE,
  SIZE_OUTBOX_KERNEL_TOTAL,
  OFFSET_ARENA,
  OFFSET_ARENA_METADATA,
  SIZE_ARENA_METADATA,
  OFFSET_DIAGNOSTICS,
  SIZE_DIAGNOSTICS,
  OFFSET_BRIDGE_METRICS,
  SIZE_BRIDGE_METRICS,
  OFFSET_ARENA_REQUEST_QUEUE,
  OFFSET_ARENA_RESPONSE_QUEUE,
  ARENA_QUEUE_ENTRY_SIZE,
  MAX_ARENA_REQUESTS,
  OFFSET_MESH_EVENT_QUEUE,
  SIZE_MESH_EVENT_QUEUE,
  MESH_EVENT_SLOT_SIZE,
  MESH_EVENT_SLOT_COUNT,
  OFFSET_REGION_GUARDS,
  SIZE_REGION_GUARDS,
  REGION_GUARD_ENTRY_SIZE,
  REGION_GUARD_COUNT,
  OFFSET_BIRD_STATE,
  SIZE_BIRD_STATE,
  OFFSET_PINGPONG_CONTROL,
  SIZE_PINGPONG_CONTROL,
  OFFSET_BIRD_BUFFER_A,
  OFFSET_BIRD_BUFFER_B,
  SIZE_BIRD_BUFFER,
  BIRD_STRIDE,
  OFFSET_MATRIX_BUFFER_A,
  OFFSET_MATRIX_BUFFER_B,
  SIZE_MATRIX_BUFFER,
  MATRIX_STRIDE,
  IDX_KERNEL_READY,
  IDX_INBOX_DIRTY,
  IDX_OUTBOX_HOST_DIRTY,
  IDX_PANIC_STATE,
  IDX_SENSOR_EPOCH,
  IDX_ACTOR_EPOCH,
  IDX_STORAGE_EPOCH,
  IDX_SYSTEM_EPOCH,
  IDX_SYSTEM_PULSE,
  IDX_SYSTEM_VISIBILITY,
  IDX_SYSTEM_POWER_STATE,
  IDX_RESERVED_PULSE1,
  IDX_RESERVED_PULSE2,
  IDX_RESERVED_PULSE3,
  IDX_RESERVED_PULSE4,
  IDX_RESERVED_PULSE5,
  IDX_ARENA_ALLOCATOR,
  IDX_OUTBOX_MUTEX,
  IDX_INBOX_MUTEX,
  IDX_METRICS_EPOCH,
  IDX_BIRD_EPOCH,
  IDX_MATRIX_EPOCH,
  IDX_PINGPONG_ACTIVE,
  IDX_REGISTRY_EPOCH,
  IDX_EVOLUTION_EPOCH,
  IDX_HEALTH_EPOCH,
  IDX_LEARNING_EPOCH,
  IDX_ECONOMY_EPOCH,
  IDX_BIRD_COUNT,
  IDX_GLOBAL_METRICS_EPOCH,
  IDX_OUTBOX_KERNEL_DIRTY,
  IDX_CONTEXT_ID_HASH,
  IDX_DELEGATED_JOB_EPOCH,
  IDX_USER_JOB_EPOCH,
  IDX_DELEGATED_CHUNK_EPOCH,
  IDX_MESH_EVENT_EPOCH,
  IDX_MESH_EVENT_HEAD,
  IDX_MESH_EVENT_TAIL,
  IDX_MESH_EVENT_DROPPED,
  SUPERVISOR_POOL_BASE,
  SUPERVISOR_POOL_SIZE,
  RESERVED_POOL_BASE,
  RESERVED_POOL_SIZE,
  ALIGNMENT_CACHE_LINE,
  ALIGNMENT_PAGE,
  ALIGNMENT_LARGE,
} as const;

export type ConstKeys = keyof typeof CONSTS;
