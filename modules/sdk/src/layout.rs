// SAB Memory Layout Constants for Rust modules
// Must match kernel/threads/sab_layout.go exactly

/// Total SAB size configurations
pub const SAB_SIZE_DEFAULT: usize = 16 * 1024 * 1024; // 16MB
pub const SAB_SIZE_MIN: usize = 4 * 1024 * 1024; // 4MB
pub const SAB_SIZE_MAX: usize = 64 * 1024 * 1024; // 64MB

// ========== METADATA REGION (0x000000 - 0x000100) ==========

/// Atomic Flags Region (64 bytes - 16 x i32)
pub const OFFSET_ATOMIC_FLAGS: usize = 0x000000;
pub const SIZE_ATOMIC_FLAGS: usize = 0x000040;

/// Supervisor Allocation Table (192 bytes)
pub const OFFSET_SUPERVISOR_ALLOC: usize = 0x000040;
pub const SIZE_SUPERVISOR_ALLOC: usize = 0x0000B0; // 176 bytes (Ends at 0xF0)

// Registry locking (16 bytes before registry)
pub const OFFSET_REGISTRY_LOCK: usize = 0x0000F0;
pub const SIZE_REGISTRY_LOCK: usize = 0x000010;

// ========== MODULE REGISTRY (0x000100 - 0x001900) ==========

/// Enhanced module entries with overflow to arena
pub const OFFSET_MODULE_REGISTRY: usize = 0x000100;
pub const SIZE_MODULE_REGISTRY: usize = 0x001800; // 6KB (Phase 2 enhanced)
pub const MODULE_ENTRY_SIZE: usize = 96; // Enhanced 96-byte entries
pub const MAX_MODULES_INLINE: usize = 64; // 64 modules inline
pub const MAX_MODULES_TOTAL: usize = 1024; // Total with arena overflow

// Bloom filter (256 bytes after registry)
pub const OFFSET_BLOOM_FILTER: usize = 0x001900;
pub const SIZE_BLOOM_FILTER: usize = 0x000100;

// ========== SUPERVISOR HEADERS (0x002000 - 0x003000) ==========

/// Compact supervisor headers with state in arena
/// MOVED: 0x001000 overlapped with Registry (0x000100+0x001800=0x001900)
pub const OFFSET_SUPERVISOR_HEADERS: usize = 0x002000;
pub const SIZE_SUPERVISOR_HEADERS: usize = 0x001000; // 4KB
pub const SUPERVISOR_HEADER_SIZE: usize = 128; // Compact 128-byte headers
pub const MAX_SUPERVISORS_INLINE: usize = 32; // 32 supervisors inline
pub const MAX_SUPERVISORS_TOTAL: usize = 256; // Total with arena overflow

// ========== SYSCALL TABLE (0x003000 - 0x004000) ==========

/// Pending syscall metadata (DeepSeek Architecture)
pub const OFFSET_SYSCALL_TABLE: usize = 0x003000;
pub const SIZE_SYSCALL_TABLE: usize = 0x001000; // 4KB

// ========== PATTERN EXCHANGE (0x010000 - 0x020000) ==========

/// Pattern storage with LRU eviction to arena
pub const OFFSET_PATTERN_EXCHANGE: usize = 0x010000;
pub const SIZE_PATTERN_EXCHANGE: usize = 0x010000; // 64KB
pub const PATTERN_ENTRY_SIZE: usize = 64; // Compact 64-byte patterns
pub const MAX_PATTERNS_INLINE: usize = 1024; // 1024 patterns inline
pub const MAX_PATTERNS_TOTAL: usize = 16384; // Total with arena overflow

// ========== JOB HISTORY (0x020000 - 0x040000) ==========

/// Circular buffer with overflow to arena
pub const OFFSET_JOB_HISTORY: usize = 0x020000;
pub const SIZE_JOB_HISTORY: usize = 0x020000; // 128KB

// ========== COORDINATION STATE (0x040000 - 0x050000) ==========

/// Cross-unit coordination with dynamic expansion
pub const OFFSET_COORDINATION: usize = 0x040000;
pub const SIZE_COORDINATION: usize = 0x010000; // 64KB

// ========== INBOX/OUTBOX (0x050000 - 0x150000) ==========

/// Job communication regions - Expanded for Slotted Architecture
/// 1MB total: 512KB Inbox + 512KB Outbox
pub const OFFSET_INBOX_OUTBOX: usize = 0x050000;
pub const SIZE_INBOX_OUTBOX: usize = 0x100000; // 1MB (was 512KB)

// Split into Inbox (Host->Module) and Outbox (Module->Host)
pub const SIZE_INBOX: usize = 0x80000; // 512KB
pub const SIZE_OUTBOX: usize = 0x80000; // 512KB
pub const OFFSET_SAB_INBOX: usize = OFFSET_INBOX_OUTBOX;
pub const OFFSET_SAB_OUTBOX: usize = OFFSET_INBOX_OUTBOX + SIZE_INBOX;

// ========== ARENA (0x150000 - end) ==========

/// Dynamic allocation region for overflow and large data
pub const OFFSET_ARENA: usize = 0x150000;
// SIZE_ARENA calculated as: SAB_SIZE - OFFSET_ARENA

/// Internal Arena Layout (Phase 16)
pub const OFFSET_ARENA_METADATA: usize = 0x150000;
pub const SIZE_ARENA_METADATA: usize = 0x010000; // 64KB reserved for metadata

/// Diagnostics Region (Heartbeats, Pulses, Health)
pub const OFFSET_DIAGNOSTICS: usize = 0x150000;
pub const SIZE_DIAGNOSTICS: usize = 0x001000; // 4KB

/// Async Request/Response Queues (DeepSeek Spec)
pub const OFFSET_ARENA_REQUEST_QUEUE: usize = 0x151000; // 0x150000 + 4KB
pub const OFFSET_ARENA_RESPONSE_QUEUE: usize = 0x152000; // 0x150000 + 8KB
pub const ARENA_QUEUE_ENTRY_SIZE: usize = 64;
pub const MAX_ARENA_REQUESTS: usize = 64;

// ========== BIRD STATE (Arena Reserved) ==========
pub const OFFSET_BIRD_STATE: usize = 0x160000; // Offset into SAB (Arena region)
pub const SIZE_BIRD_STATE: usize = 0x001000; // 4KB for bird telemetry and state

// ========== EPOCH INDEX ALLOCATION ==========

/// Fixed system epochs (0-31 Reserved)
pub const IDX_KERNEL_READY: u32 = 0;
pub const IDX_INBOX_DIRTY: u32 = 1; // Signal from Kernel to Module
pub const IDX_OUTBOX_DIRTY: u32 = 2; // Signal from Module to Kernel
pub const IDX_PANIC_STATE: u32 = 3;
pub const IDX_SENSOR_EPOCH: u32 = 4;
pub const IDX_ACTOR_EPOCH: u32 = 5;
pub const IDX_STORAGE_EPOCH: u32 = 6;
pub const IDX_SYSTEM_EPOCH: u32 = 7;

/// Phase 16: Extended System Epochs
pub const IDX_ARENA_ALLOCATOR: u32 = 8; // Arena bump pointer (bytes used)
pub const IDX_OUTBOX_MUTEX: u32 = 9; // Mutex for outbox synchronization
pub const IDX_INBOX_MUTEX: u32 = 10; // Mutex for inbox synchronization
pub const IDX_METRICS_EPOCH: u32 = 11;
pub const IDX_BIRD_EPOCH: u32 = 12; // High-frequency bird state updates

/// Dynamic supervisor pool (32-127)
pub const SUPERVISOR_POOL_BASE: u32 = 32;
pub const SUPERVISOR_POOL_SIZE: u32 = 96;

/// Reserved for future expansion (128-255)
pub const RESERVED_POOL_BASE: u32 = 128;
pub const RESERVED_POOL_SIZE: u32 = 128;

// ========== ALIGNMENT REQUIREMENTS ==========

pub const ALIGNMENT_CACHE_LINE: usize = 64; // Cache line alignment
pub const ALIGNMENT_PAGE: usize = 4096; // Page alignment
pub const ALIGNMENT_LARGE: usize = 65536; // Large allocation alignment

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
        o if o < OFFSET_COORDINATION => "JobHistory",
        o if o < OFFSET_INBOX_OUTBOX => "Coordination",
        o if o < OFFSET_ARENA => "InboxOutbox",
        _ => "Arena",
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_arena_size_calculation() {
        assert_eq!(
            calculate_arena_size(SAB_SIZE_DEFAULT),
            SAB_SIZE_DEFAULT - OFFSET_ARENA
        );
        assert_eq!(calculate_arena_size(OFFSET_ARENA - 1), 0);
    }

    #[test]
    fn test_alignment() {
        assert_eq!(align_offset(0, 64), 0);
        assert_eq!(align_offset(1, 64), 64);
        assert_eq!(align_offset(63, 64), 64);
        assert_eq!(align_offset(64, 64), 64);
        assert_eq!(align_offset(65, 64), 128);
    }

    #[test]
    fn test_validate_offset() {
        assert!(validate_offset(0, 100, 1000).is_ok());
        assert!(validate_offset(900, 100, 1000).is_ok());
        assert!(validate_offset(901, 100, 1000).is_err());
        assert!(validate_offset(1000, 1, 1000).is_err());
    }

    #[test]
    fn test_region_names() {
        assert_eq!(get_region_name(0x000000), "AtomicFlags");
        assert_eq!(get_region_name(0x000040), "SupervisorAlloc");
        assert_eq!(get_region_name(0x000100), "ModuleRegistry");
        assert_eq!(get_region_name(0x002000), "SupervisorHeaders");
        assert_eq!(get_region_name(0x010000), "PatternExchange");
        assert_eq!(get_region_name(0x150000), "Arena");
    }

    #[test]
    fn test_no_region_overlaps() {
        // Verify regions don't overlap
        const { assert!(OFFSET_SUPERVISOR_ALLOC >= OFFSET_ATOMIC_FLAGS + SIZE_ATOMIC_FLAGS) };
        const { assert!(OFFSET_REGISTRY_LOCK >= OFFSET_SUPERVISOR_ALLOC + SIZE_SUPERVISOR_ALLOC) };
        const { assert!(OFFSET_MODULE_REGISTRY >= OFFSET_REGISTRY_LOCK + SIZE_REGISTRY_LOCK) };
        const { assert!(OFFSET_SUPERVISOR_HEADERS >= OFFSET_MODULE_REGISTRY + SIZE_MODULE_REGISTRY) };
        // Syscall Table Check
        const { assert!(OFFSET_SYSCALL_TABLE >= OFFSET_SUPERVISOR_HEADERS + SIZE_SUPERVISOR_HEADERS) };
        const { assert!(OFFSET_PATTERN_EXCHANGE >= OFFSET_SYSCALL_TABLE + SIZE_SYSCALL_TABLE) };

        const { assert!(OFFSET_JOB_HISTORY >= OFFSET_PATTERN_EXCHANGE + SIZE_PATTERN_EXCHANGE) };
        const { assert!(OFFSET_COORDINATION >= OFFSET_JOB_HISTORY + SIZE_JOB_HISTORY) };
        const { assert!(OFFSET_INBOX_OUTBOX >= OFFSET_COORDINATION + SIZE_COORDINATION) };
        const { assert!(OFFSET_ARENA >= OFFSET_INBOX_OUTBOX + SIZE_INBOX_OUTBOX) };
    }
}
