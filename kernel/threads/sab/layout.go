package sab

// SAB Memory Layout Constants
// This defines the complete SharedArrayBuffer memory layout for INOS v1.9+
// All regions are designed for dynamic expansion with overflow to arena

const (
	// Total SAB size (configurable, default 16MB)
	SAB_SIZE_DEFAULT = 16 * 1024 * 1024 // 16MB
	SAB_SIZE_MIN     = 4 * 1024 * 1024  // 4MB minimum
	SAB_SIZE_MAX     = 64 * 1024 * 1024 // 64MB maximum

	// ========== METADATA REGION (0x000000 - 0x000100) ==========
	// Atomic Flags Region (64 bytes - 16 x i32)
	OFFSET_ATOMIC_FLAGS = 0x000000
	SIZE_ATOMIC_FLAGS   = 0x000040 // 64 bytes

	// Supervisor Allocation Table (176 bytes)
	OFFSET_SUPERVISOR_ALLOC = 0x000040
	SIZE_SUPERVISOR_ALLOC   = 0x0000B0 // 176 bytes (Ends at 0xF0)

	// Registry locking (16 bytes before registry)
	OFFSET_REGISTRY_LOCK = 0x0000F0
	SIZE_REGISTRY_LOCK   = 0x000010 // 16 bytes

	// ========== MODULE REGISTRY (0x000100 - 0x001900) ==========
	// Enhanced module entries with overflow to arena
	OFFSET_MODULE_REGISTRY = 0x000100
	SIZE_MODULE_REGISTRY   = 0x001800 // 6KB (Phase 2 enhanced)
	MODULE_ENTRY_SIZE      = 96       // Enhanced 96-byte entries
	MAX_MODULES_INLINE     = 64       // 64 modules inline
	MAX_MODULES_TOTAL      = 1024     // Total with arena overflow

	// Bloom filter (256 bytes after registry)
	OFFSET_BLOOM_FILTER = 0x001900
	SIZE_BLOOM_FILTER   = 0x000100 // 256 bytes

	// ========== SUPERVISOR HEADERS (0x002000 - 0x003000) ==========
	// Compact supervisor headers with state in arena
	// MOVED: 0x001000 overlapped with Registry (0x000100+0x001800=0x001900)
	OFFSET_SUPERVISOR_HEADERS = 0x002000
	SIZE_SUPERVISOR_HEADERS   = 0x001000 // 4KB
	SUPERVISOR_HEADER_SIZE    = 128      // Compact 128-byte headers
	MAX_SUPERVISORS_INLINE    = 32       // 32 supervisors inline
	MAX_SUPERVISORS_TOTAL     = 256      // Total with arena overflow

	// ========== SYSCALL TABLE (0x003000 - 0x004000) ==========
	// Pending syscall metadata (DeepSeek Architecture)
	OFFSET_SYSCALL_TABLE = 0x003000
	SIZE_SYSCALL_TABLE   = 0x001000 // 4KB
	// ========== ECONOMICS (0x004000 - 0x010000) ==========
	// Credit accounts and resource metrics (Phase 17)
	OFFSET_ECONOMICS = 0x004000
	SIZE_ECONOMICS   = 0x004000 // 16KB

	// ========== IDENTITY REGISTRY (0x008000 - 0x00C000) ==========
	// DIDs, device binding, and TSS metadata (Phase 17.D)
	OFFSET_IDENTITY_REGISTRY = 0x008000
	SIZE_IDENTITY_REGISTRY   = 0x004000 // 16KB

	// ========== SOCIAL GRAPH (0x00C000 - 0x010000) ==========
	// Referrals, close IDs, and social yield (Phase 17.E)
	OFFSET_SOCIAL_GRAPH = 0x00C000
	SIZE_SOCIAL_GRAPH   = 0x004000 // 16KB

	// ========== PATTERN EXCHANGE (0x010000 - 0x020000) ==========
	// Pattern storage with LRU eviction to arena
	OFFSET_PATTERN_EXCHANGE = 0x010000
	SIZE_PATTERN_EXCHANGE   = 0x010000 // 64KB
	PATTERN_ENTRY_SIZE      = 64       // Compact 64-byte patterns
	MAX_PATTERNS_INLINE     = 1024     // 1024 patterns inline
	MAX_PATTERNS_TOTAL      = 16384    // Total with arena overflow

	// ========== JOB HISTORY (0x020000 - 0x040000) ==========
	// Circular buffer with overflow to arena
	OFFSET_JOB_HISTORY = 0x020000
	SIZE_JOB_HISTORY   = 0x020000 // 128KB

	// ========== COORDINATION STATE (0x040000 - 0x050000) ==========
	// Cross-unit coordination with dynamic expansion
	OFFSET_COORDINATION = 0x040000
	SIZE_COORDINATION   = 0x010000 // 64KB

	// ========== INBOX/OUTBOX (0x050000 - 0x150000) ==========
	// Job communication regions - Expanded for Slotted Architecture
	// 1MB total: 512KB Inbox + 512KB Outbox
	OFFSET_INBOX_OUTBOX = 0x050000
	SIZE_INBOX_OUTBOX   = 0x100000 // 1MB (was 512KB)

	// Sub-regions
	OFFSET_INBOX_BASE  = 0x050000
	SIZE_INBOX_TOTAL   = 0x080000 // 512KB
	OFFSET_OUTBOX_BASE = 0x0D0000 // 0x050000 + 512KB
	SIZE_OUTBOX_TOTAL  = 0x080000 // 512KB

	// ========== ARENA (0x150000 - end) ==========
	// Dynamic allocation region for overflow and large data
	OFFSET_ARENA = 0x150000
	// SIZE_ARENA calculated as: SAB_SIZE - OFFSET_ARENA

	// Internal Arena Layout (Phase 16)
	OFFSET_ARENA_METADATA = 0x150000
	SIZE_ARENA_METADATA   = 0x010000 // 64KB reserved for metadata

	// Async Request/Response Queues (DeepSeek Spec)
	// These allow modules to request larger allocations or complex operations
	OFFSET_ARENA_REQUEST_QUEUE  = 0x151000 // 0x150000 + 4KB
	OFFSET_ARENA_RESPONSE_QUEUE = 0x152000 // 0x150000 + 8KB
	ARENA_QUEUE_ENTRY_SIZE      = 64
	MAX_ARENA_REQUESTS          = 64

	// ========== EPOCH INDEX ALLOCATION ==========
	// Fixed system epochs (0-31 Reserved)
	IDX_KERNEL_READY  = 0
	IDX_INBOX_DIRTY   = 1 // Signal from Kernel to Module (Module watches this)
	IDX_OUTBOX_DIRTY  = 2 // Signal from Module to Kernel (Kernel watches this)
	IDX_PANIC_STATE   = 3
	IDX_SENSOR_EPOCH  = 4
	IDX_ACTOR_EPOCH   = 5
	IDX_STORAGE_EPOCH = 6
	IDX_SYSTEM_EPOCH  = 7

	// Phase 16: Extended System Epochs
	IDX_ARENA_ALLOCATOR = 8  // Atomic bump pointer for arena
	IDX_OUTBOX_MUTEX    = 9  // Mutex for outbox synchronization
	IDX_INBOX_MUTEX     = 10 // Mutex for inbox synchronization
	IDX_METRICS_EPOCH   = 11
	IDX_BOIDS_COUNT     = 12 // Current population count for Go supervisor discovery

	// Reserved for future system extensions (13-31)

	// Dynamic supervisor pool (32-127)
	SUPERVISOR_POOL_BASE = 32
	SUPERVISOR_POOL_SIZE = 96 // Supports 96 supervisors

	// Reserved for future expansion (128-255)
	RESERVED_POOL_BASE = 128
	RESERVED_POOL_SIZE = 128

	// ========== ALIGNMENT REQUIREMENTS ==========
	ALIGNMENT_CACHE_LINE = 64    // Cache line alignment
	ALIGNMENT_PAGE       = 4096  // Page alignment
	ALIGNMENT_LARGE      = 65536 // Large allocation alignment
)

// MemoryRegion describes a region in the SAB
type MemoryRegion struct {
	Name    string
	Offset  uint32
	Size    uint32
	Purpose string
	// Expansion configuration
	CanExpand      bool   // Can overflow to arena
	MaxInline      uint32 // Max entries in inline region
	MaxTotal       uint32 // Max entries including arena
	OverflowOffset uint32 // Offset in arena for overflow (0 if not allocated)
}

// GetAllRegions returns all defined memory regions
func GetAllRegions(sabSize uint32) []MemoryRegion {
	arenaSize := sabSize - OFFSET_ARENA

	return []MemoryRegion{
		{
			Name:      "AtomicFlags",
			Offset:    OFFSET_ATOMIC_FLAGS,
			Size:      SIZE_ATOMIC_FLAGS,
			Purpose:   "Epoch counters and atomic flags",
			CanExpand: false,
			MaxInline: 16, // 16 i32 flags
			MaxTotal:  16,
		},
		{
			Name:      "SupervisorAlloc",
			Offset:    OFFSET_SUPERVISOR_ALLOC,
			Size:      SIZE_SUPERVISOR_ALLOC,
			Purpose:   "Dynamic epoch allocation table",
			CanExpand: false,
			MaxInline: 1,
			MaxTotal:  1,
		},
		{
			Name:      "RegistryLock",
			Offset:    OFFSET_REGISTRY_LOCK,
			Size:      SIZE_REGISTRY_LOCK,
			Purpose:   "Global mutex for registry operations",
			CanExpand: false,
			MaxInline: 1,
			MaxTotal:  1,
		},
		{
			Name:      "ModuleRegistry",
			Offset:    OFFSET_MODULE_REGISTRY,
			Size:      SIZE_MODULE_REGISTRY,
			Purpose:   "Module metadata and capabilities",
			CanExpand: true,
			MaxInline: MAX_MODULES_INLINE,
			MaxTotal:  MAX_MODULES_TOTAL,
		},
		{
			Name:      "SupervisorHeaders",
			Offset:    OFFSET_SUPERVISOR_HEADERS,
			Size:      SIZE_SUPERVISOR_HEADERS,
			Purpose:   "Supervisor state headers",
			CanExpand: true,
			MaxInline: MAX_SUPERVISORS_INLINE,
			MaxTotal:  MAX_SUPERVISORS_TOTAL,
		},
		{
			Name:      "SyscallTable",
			Offset:    OFFSET_SYSCALL_TABLE,
			Size:      SIZE_SYSCALL_TABLE,
			Purpose:   "Pending system call metadata",
			CanExpand: false,
			MaxInline: 0,
			MaxTotal:  0,
		},
		{
			Name:      "PatternExchange",
			Offset:    OFFSET_PATTERN_EXCHANGE,
			Size:      SIZE_PATTERN_EXCHANGE,
			Purpose:   "Learned patterns and optimizations",
			CanExpand: true,
			MaxInline: MAX_PATTERNS_INLINE,
			MaxTotal:  MAX_PATTERNS_TOTAL,
		},
		{
			Name:      "JobHistory",
			Offset:    OFFSET_JOB_HISTORY,
			Size:      SIZE_JOB_HISTORY,
			Purpose:   "Job execution history (circular buffer)",
			CanExpand: true,
			MaxInline: 0, // Calculated based on entry size
			MaxTotal:  0, // Unlimited with arena overflow
		},
		{
			Name:      "Coordination",
			Offset:    OFFSET_COORDINATION,
			Size:      SIZE_COORDINATION,
			Purpose:   "Cross-unit coordination state",
			CanExpand: true,
			MaxInline: 0,
			MaxTotal:  0,
		},
		{
			Name:      "Economics",
			Offset:    OFFSET_ECONOMICS,
			Size:      SIZE_ECONOMICS,
			Purpose:   "Credit accounts and resource metrics",
			CanExpand: true,
			MaxInline: 128, // 128 accounts inline
			MaxTotal:  1024,
		},
		{
			Name:      "InboxOutbox",
			Offset:    OFFSET_INBOX_OUTBOX,
			Size:      SIZE_INBOX_OUTBOX,
			Purpose:   "Job request/result communication",
			CanExpand: false, // Fixed size for predictable latency
			MaxInline: 0,
			MaxTotal:  0,
		},
		{
			Name:      "Arena",
			Offset:    OFFSET_ARENA,
			Size:      arenaSize,
			Purpose:   "Dynamic allocation for overflow and large data",
			CanExpand: false, // Arena is the expansion target
			MaxInline: 0,
			MaxTotal:  0,
		},
	}
}

// ValidateMemoryLayout checks for overlaps and invalid configurations
func ValidateMemoryLayout(sabSize uint32) error {
	if sabSize < SAB_SIZE_MIN {
		return &LayoutError{
			Code:    "SAB_TOO_SMALL",
			Message: "SAB size must be at least 4MB",
		}
	}

	if sabSize > SAB_SIZE_MAX {
		return &LayoutError{
			Code:    "SAB_TOO_LARGE",
			Message: "SAB size must not exceed 64MB",
		}
	}

	regions := GetAllRegions(sabSize)

	// Check for overlaps
	for i := 0; i < len(regions); i++ {
		for j := i + 1; j < len(regions); j++ {
			r1, r2 := regions[i], regions[j]

			// Check if regions overlap
			if r1.Offset < r2.Offset+r2.Size && r1.Offset+r1.Size > r2.Offset {
				return &LayoutError{
					Code:    "REGION_OVERLAP",
					Message: "Region " + r1.Name + " overlaps with " + r2.Name,
				}
			}
		}
	}

	// Validate arena starts after all fixed regions
	if OFFSET_ARENA >= sabSize {
		return &LayoutError{
			Code:    "INVALID_ARENA",
			Message: "Arena offset exceeds SAB size",
		}
	}

	return nil
}

// GetRegionInfo returns information about the region containing the given offset
func GetRegionInfo(offset uint32, sabSize uint32) (*MemoryRegion, error) {
	regions := GetAllRegions(sabSize)

	for _, region := range regions {
		if offset >= region.Offset && offset < region.Offset+region.Size {
			return &region, nil
		}
	}

	return nil, &LayoutError{
		Code:    "INVALID_OFFSET",
		Message: "Offset does not belong to any region",
	}
}

// IsValidOffset checks if an offset and size are within bounds
func IsValidOffset(offset, size, sabSize uint32) bool {
	return offset+size <= sabSize
}

// LayoutError represents a memory layout error
type LayoutError struct {
	Code    string
	Message string
}

func (e *LayoutError) Error() string {
	return e.Code + ": " + e.Message
}

// CalculateArenaSize returns the size of the arena region
func CalculateArenaSize(sabSize uint32) uint32 {
	if sabSize < OFFSET_ARENA {
		return 0
	}
	return sabSize - OFFSET_ARENA
}

// AlignOffset aligns an offset to the specified alignment
func AlignOffset(offset, alignment uint32) uint32 {
	return (offset + alignment - 1) & ^(alignment - 1)
}

// GetExpansionInfo returns information about region expansion capabilities
type ExpansionInfo struct {
	RegionName     string
	InlineUsed     uint32
	InlineCapacity uint32
	ArenaUsed      uint32
	ArenaCapacity  uint32
	CanExpand      bool
	UtilizationPct float32
}

// CalculateUtilization calculates region utilization percentage
func CalculateUtilization(used, capacity uint32) float32 {
	if capacity == 0 {
		return 0
	}
	return float32(used) / float32(capacity) * 100.0
}
