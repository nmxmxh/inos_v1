// SAB Memory Layout Constants for Rust modules
// SOURCED DIRECTLY FROM protocols/schemas/system/v1/sab_layout.capnp

use crate::sab_layout_capnp as sab;

// ========== SYSTEM BASE OFFSET ==========
// Go Kernel binary + heap occupy 0-16MB.
// All offsets below are ABSOLUTE and ALREADY INCLUDE this base.
pub const OFFSET_SYSTEM_BASE: usize = sab::OFFSET_SYSTEM_BASE as usize;

/// Total SAB size configurations (including 16MB Go zone)
pub const SAB_SIZE_DEFAULT: usize = sab::SAB_SIZE_DEFAULT as usize;
pub const SAB_SIZE_MIN: usize = sab::SAB_SIZE_MIN as usize;
pub const SAB_SIZE_MAX: usize = sab::SAB_SIZE_MAX as usize;

// ========== SYSTEM REGIONS (Absolute Addresses) ==========

/// Atomic Flags Region (128 bytes - 32 x i32)
pub const OFFSET_ATOMIC_FLAGS: usize = sab::OFFSET_ATOMIC_FLAGS as usize;
pub const SIZE_ATOMIC_FLAGS: usize = sab::SIZE_ATOMIC_FLAGS as usize;

/// Supervisor Allocation Table (176 bytes)
pub const OFFSET_SUPERVISOR_ALLOC: usize = sab::OFFSET_SUPERVISOR_ALLOC as usize;
pub const SIZE_SUPERVISOR_ALLOC: usize = sab::SIZE_SUPERVISOR_ALLOC as usize;

// Registry locking (16 bytes before registry)
pub const OFFSET_REGISTRY_LOCK: usize = sab::OFFSET_REGISTRY_LOCK as usize;
pub const SIZE_REGISTRY_LOCK: usize = sab::SIZE_REGISTRY_LOCK as usize;

/// Module Registry (6KB)
pub const OFFSET_MODULE_REGISTRY: usize = sab::OFFSET_MODULE_REGISTRY as usize;
pub const SIZE_MODULE_REGISTRY: usize = sab::SIZE_MODULE_REGISTRY as usize;
pub const MODULE_ENTRY_SIZE: usize = sab::MODULE_ENTRY_SIZE as usize;
pub const MAX_MODULES_INLINE: usize = sab::MAX_MODULES_INLINE as usize;
pub const MAX_MODULES_TOTAL: usize = sab::MAX_MODULES_TOTAL as usize;

/// Bloom filter (256 bytes after registry)
pub const OFFSET_BLOOM_FILTER: usize = sab::OFFSET_BLOOM_FILTER as usize;
pub const SIZE_BLOOM_FILTER: usize = sab::SIZE_BLOOM_FILTER as usize;

/// Supervisor Headers (4KB)
pub const OFFSET_SUPERVISOR_HEADERS: usize = sab::OFFSET_SUPERVISOR_HEADERS as usize;
pub const SIZE_SUPERVISOR_HEADERS: usize = sab::SIZE_SUPERVISOR_HEADERS as usize;
pub const MAX_SUPERVISORS_INLINE: usize = sab::MAX_SUPERVISORS_INLINE as usize;

/// Syscall Table (4KB)
pub const OFFSET_SYSCALL_TABLE: usize = sab::OFFSET_SYSCALL_TABLE as usize;
pub const SIZE_SYSCALL_TABLE: usize = sab::SIZE_SYSCALL_TABLE as usize;

/// Economics Region (16KB)
pub const OFFSET_ECONOMICS: usize = sab::OFFSET_ECONOMICS as usize;
pub const SIZE_ECONOMICS: usize = sab::SIZE_ECONOMICS as usize;

/// Identity Registry (16KB)
pub const OFFSET_IDENTITY_REGISTRY: usize = sab::OFFSET_IDENTITY_REGISTRY as usize;
pub const SIZE_IDENTITY_REGISTRY: usize = sab::SIZE_IDENTITY_REGISTRY as usize;

/// Social Graph (16KB)
pub const OFFSET_SOCIAL_GRAPH: usize = sab::OFFSET_SOCIAL_GRAPH as usize;
pub const SIZE_SOCIAL_GRAPH: usize = sab::SIZE_SOCIAL_GRAPH as usize;

/// Pattern Exchange (64KB)
pub const OFFSET_PATTERN_EXCHANGE: usize = sab::OFFSET_PATTERN_EXCHANGE as usize;
pub const SIZE_PATTERN_EXCHANGE: usize = sab::SIZE_PATTERN_EXCHANGE as usize;
pub const PATTERN_ENTRY_SIZE: usize = sab::PATTERN_ENTRY_SIZE as usize;

/// Job History (128KB)
pub const OFFSET_JOB_HISTORY: usize = sab::OFFSET_JOB_HISTORY as usize;
pub const SIZE_JOB_HISTORY: usize = sab::SIZE_JOB_HISTORY as usize;

/// Coordination State (64KB)
pub const OFFSET_COORDINATION: usize = sab::OFFSET_COORDINATION as usize;
pub const SIZE_COORDINATION: usize = sab::SIZE_COORDINATION as usize;

/// Inbox/Outbox (1MB)
pub const OFFSET_INBOX_OUTBOX: usize = sab::OFFSET_INBOX_OUTBOX as usize;
pub const SIZE_INBOX_OUTBOX: usize = sab::SIZE_INBOX_OUTBOX as usize;
pub const SIZE_INBOX: usize = sab::SIZE_INBOX_TOTAL as usize;
pub const SIZE_OUTBOX: usize = sab::SIZE_OUTBOX_KERNEL_TOTAL as usize;
pub const OFFSET_SAB_INBOX: usize = sab::OFFSET_INBOX_BASE as usize;
pub const OFFSET_SAB_OUTBOX: usize = sab::OFFSET_OUTBOX_KERNEL_BASE as usize;

// ========== ARENA REGIONS ==========

pub const OFFSET_ARENA: usize = sab::OFFSET_ARENA as usize;

/// Diagnostics Region (4KB)
pub const OFFSET_DIAGNOSTICS: usize = sab::OFFSET_DIAGNOSTICS as usize;
pub const SIZE_DIAGNOSTICS: usize = sab::SIZE_DIAGNOSTICS as usize;

pub const OFFSET_BRIDGE_METRICS: usize = OFFSET_DIAGNOSTICS + 0x800;
pub const SIZE_BRIDGE_METRICS: usize = 0x100;

/// Async Request/Response Queues
pub const OFFSET_ARENA_REQUEST_QUEUE: usize = sab::OFFSET_ARENA_REQUEST_QUEUE as usize;
pub const OFFSET_ARENA_RESPONSE_QUEUE: usize = sab::OFFSET_ARENA_RESPONSE_QUEUE as usize;
pub const ARENA_QUEUE_ENTRY_SIZE: usize = sab::ARENA_QUEUE_ENTRY_SIZE as usize;
pub const MAX_ARENA_REQUESTS: usize = sab::MAX_ARENA_REQUESTS as usize;

/// Mesh Event Ring Buffer (Arena metadata)
pub const OFFSET_MESH_EVENT_QUEUE: usize = sab::OFFSET_MESH_EVENT_QUEUE as usize;
pub const SIZE_MESH_EVENT_QUEUE: usize = sab::SIZE_MESH_EVENT_QUEUE as usize;
pub const MESH_EVENT_SLOT_SIZE: usize = sab::MESH_EVENT_SLOT_SIZE as usize;
pub const MESH_EVENT_SLOT_COUNT: usize = sab::MESH_EVENT_SLOT_COUNT as usize;

/// Bird Animation State (Arena)
pub const OFFSET_BIRD_STATE: usize = sab::OFFSET_BIRD_STATE as usize;
pub const SIZE_BIRD_STATE: usize = sab::SIZE_BIRD_STATE as usize;

// ---------- Ping-Pong Buffer Regions ----------

/// Control block for ping-pong coordination
pub const OFFSET_PINGPONG_CONTROL: usize = sab::OFFSET_PINGPONG_CONTROL as usize;
pub const SIZE_PINGPONG_CONTROL: usize = sab::SIZE_PINGPONG_CONTROL as usize;

/// Bird Population Data (Dual Buffers)
pub const OFFSET_BIRD_BUFFER_A: usize = sab::OFFSET_BIRD_BUFFER_A as usize;
pub const OFFSET_BIRD_BUFFER_B: usize = sab::OFFSET_BIRD_BUFFER_B as usize;
pub const SIZE_BIRD_BUFFER: usize = sab::SIZE_BIRD_BUFFER as usize;
pub const BIRD_STRIDE: usize = sab::BIRD_STRIDE as usize;

/// Matrix Output Data (Dual Buffers)
pub const OFFSET_MATRIX_BUFFER_A: usize = sab::OFFSET_MATRIX_BUFFER_A as usize;
pub const OFFSET_MATRIX_BUFFER_B: usize = sab::OFFSET_MATRIX_BUFFER_B as usize;
pub const SIZE_MATRIX_BUFFER: usize = sab::SIZE_MATRIX_BUFFER as usize;
pub const MATRIX_STRIDE: usize = sab::MATRIX_STRIDE as usize;

// ========== EPOCH INDEX ALLOCATION ==========

pub const IDX_KERNEL_READY: u32 = sab::IDX_KERNEL_READY;
pub const IDX_INBOX_DIRTY: u32 = sab::IDX_INBOX_DIRTY;
pub const IDX_OUTBOX_HOST_DIRTY: u32 = sab::IDX_OUTBOX_HOST_DIRTY;
pub const IDX_OUTBOX_KERNEL_DIRTY: u32 = sab::IDX_OUTBOX_KERNEL_DIRTY;
pub const IDX_PANIC_STATE: u32 = sab::IDX_PANIC_STATE;
pub const IDX_SENSOR_EPOCH: u32 = sab::IDX_SENSOR_EPOCH;
pub const IDX_ACTOR_EPOCH: u32 = sab::IDX_ACTOR_EPOCH;
pub const IDX_STORAGE_EPOCH: u32 = sab::IDX_STORAGE_EPOCH;
pub const IDX_SYSTEM_EPOCH: u32 = sab::IDX_SYSTEM_EPOCH;

pub const IDX_ARENA_ALLOCATOR: u32 = sab::IDX_ARENA_ALLOCATOR;
pub const IDX_OUTBOX_MUTEX: u32 = sab::IDX_OUTBOX_MUTEX;
pub const IDX_INBOX_MUTEX: u32 = sab::IDX_INBOX_MUTEX;
pub const IDX_METRICS_EPOCH: u32 = sab::IDX_METRICS_EPOCH;
pub const IDX_BIRD_EPOCH: u32 = sab::IDX_BIRD_EPOCH;
pub const IDX_MATRIX_EPOCH: u32 = sab::IDX_MATRIX_EPOCH;
pub const IDX_PINGPONG_ACTIVE: u32 = sab::IDX_PINGPONG_ACTIVE;

pub const IDX_REGISTRY_EPOCH: u32 = sab::IDX_REGISTRY_EPOCH;
pub const IDX_EVOLUTION_EPOCH: u32 = sab::IDX_EVOLUTION_EPOCH;
pub const IDX_HEALTH_EPOCH: u32 = sab::IDX_HEALTH_EPOCH;
pub const IDX_LEARNING_EPOCH: u32 = sab::IDX_LEARNING_EPOCH;
pub const IDX_ECONOMY_EPOCH: u32 = sab::IDX_ECONOMY_EPOCH;
pub const IDX_BIRD_COUNT: u32 = sab::IDX_BIRD_COUNT;
pub const IDX_GLOBAL_METRICS_EPOCH: u32 = sab::IDX_GLOBAL_METRICS_EPOCH;

// Mesh Delegation Epochs (P2P Coordination)
pub const IDX_DELEGATED_JOB_EPOCH: u32 = sab::IDX_DELEGATED_JOB_EPOCH;
pub const IDX_USER_JOB_EPOCH: u32 = sab::IDX_USER_JOB_EPOCH;
pub const IDX_DELEGATED_CHUNK_EPOCH: u32 = sab::IDX_DELEGATED_CHUNK_EPOCH;
pub const IDX_MESH_EVENT_EPOCH: u32 = sab::IDX_MESH_EVENT_EPOCH;
pub const IDX_MESH_EVENT_HEAD: u32 = sab::IDX_MESH_EVENT_HEAD;
pub const IDX_MESH_EVENT_TAIL: u32 = sab::IDX_MESH_EVENT_TAIL;
pub const IDX_MESH_EVENT_DROPPED: u32 = sab::IDX_MESH_EVENT_DROPPED;

pub const IDX_CONTEXT_ID_HASH: u32 = sab::IDX_CONTEXT_ID_HASH;

pub const SUPERVISOR_POOL_BASE: u32 = sab::SUPERVISOR_POOL_BASE;
pub const SUPERVISOR_POOL_SIZE: u32 = sab::SUPERVISOR_POOL_SIZE;

pub const RESERVED_POOL_BASE: u32 = sab::RESERVED_POOL_BASE;
pub const RESERVED_POOL_SIZE: u32 = sab::RESERVED_POOL_SIZE;

// ========== ALIGNMENT REQUIREMENTS ==========

pub const ALIGNMENT_CACHE_LINE: usize = sab::ALIGNMENT_CACHE_LINE as usize;
pub const ALIGNMENT_PAGE: usize = sab::ALIGNMENT_PAGE as usize;
pub const ALIGNMENT_LARGE: usize = sab::ALIGNMENT_LARGE as usize;

pub const fn should_signal_system_epoch(index: u32) -> bool {
    matches!(
        index,
        IDX_KERNEL_READY
            | IDX_INBOX_DIRTY
            | IDX_OUTBOX_HOST_DIRTY
            | IDX_OUTBOX_KERNEL_DIRTY
            | IDX_PANIC_STATE
            | IDX_SENSOR_EPOCH
            | IDX_ACTOR_EPOCH
            | IDX_STORAGE_EPOCH
            | IDX_ARENA_ALLOCATOR
            | IDX_METRICS_EPOCH
            | IDX_BIRD_EPOCH
            | IDX_MATRIX_EPOCH
            | IDX_REGISTRY_EPOCH
            | IDX_EVOLUTION_EPOCH
            | IDX_HEALTH_EPOCH
            | IDX_LEARNING_EPOCH
            | IDX_ECONOMY_EPOCH
            | IDX_BIRD_COUNT
            | IDX_GLOBAL_METRICS_EPOCH
            | IDX_DELEGATED_JOB_EPOCH
            | IDX_USER_JOB_EPOCH
            | IDX_DELEGATED_CHUNK_EPOCH
            | IDX_MESH_EVENT_EPOCH
    )
}

/// Calculate arena size for a given SAB size
pub const fn calculate_arena_size(sab_size: usize) -> usize {
    sab_size.saturating_sub(OFFSET_ARENA)
}

/// Align offset to specified alignment
pub const fn align_offset(offset: usize, alignment: usize) -> usize {
    (offset + alignment - 1) & !(alignment - 1)
}

/// Validate offset and size are within bounds
pub fn validate_offset(offset: usize, size: usize, sab_size: usize) -> Result<(), String> {
    if offset + size > sab_size {
        return Err(format!(
            "Offset {} + size {} exceeds SAB size {}",
            offset, size, sab_size
        ));
    }
    Ok(())
}

/// Get region name for an offset
pub fn get_region_name(offset: usize) -> &'static str {
    match offset {
        o if o < OFFSET_SUPERVISOR_ALLOC => "AtomicFlags",
        o if o < OFFSET_MODULE_REGISTRY => "SupervisorAlloc",
        o if o < OFFSET_SUPERVISOR_HEADERS => "ModuleRegistry",
        o if o < OFFSET_PATTERN_EXCHANGE => "SupervisorHeaders",
        o if o < OFFSET_JOB_HISTORY => "PatternExchange",
        o if o < OFFSET_INBOX_OUTBOX => "Coordination",
        o if o < OFFSET_ARENA => "InboxOutbox",
        _ => "Arena",
    }
}
