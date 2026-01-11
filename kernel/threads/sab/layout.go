package sab

import (
	system "github.com/nmxmxh/inos_v1/kernel/gen/system/v1"
)

// SAB Memory Layout Constants
// This file now acts as a wrapper that re-exports constants from the generated
// Cap'n Proto schema (protocols/schemas/system/v1/sab_layout.capnp).
// This maintains backwards compatibility while ensuring all constants come from
// a single source of truth.
//
// All regions are designed for dynamic expansion with overflow to arena.

const (
	// ========== SYSTEM BASE OFFSET ==========
	// Go Kernel binary + heap occupy 0-16MB. All SAB offsets are relative to this.
	OFFSET_SYSTEM_BASE = system.OffsetSystemBase // 16MB

	// ========== SAB SIZE LIMITS ==========
	SAB_SIZE_DEFAULT = system.SabSizeDefault // 48MB (includes 16MB Go zone)
	SAB_SIZE_MIN     = system.SabSizeMin     // 48MB minimum
	SAB_SIZE_MAX     = system.SabSizeMax     // 1GB

	// ========== METADATA REGION (0x000000 - 0x000100) ==========
	// Atomic Flags Region (64 bytes - 16 x i32)
	OFFSET_ATOMIC_FLAGS = system.OffsetAtomicFlags
	SIZE_ATOMIC_FLAGS   = system.SizeAtomicFlags

	// Supervisor Allocation Table (176 bytes)
	OFFSET_SUPERVISOR_ALLOC = system.OffsetSupervisorAlloc
	SIZE_SUPERVISOR_ALLOC   = system.SizeSupervisorAlloc

	// Registry locking (16 bytes before registry)
	OFFSET_REGISTRY_LOCK = system.OffsetRegistryLock
	SIZE_REGISTRY_LOCK   = system.SizeRegistryLock

	// ========== MODULE REGISTRY (0x000100 - 0x001900) ==========
	OFFSET_MODULE_REGISTRY = system.OffsetModuleRegistry
	SIZE_MODULE_REGISTRY   = system.SizeModuleRegistry
	MODULE_ENTRY_SIZE      = system.ModuleEntrySize
	MAX_MODULES_INLINE     = system.MaxModulesInline
	MAX_MODULES_TOTAL      = system.MaxModulesTotal

	// Bloom filter (256 bytes after registry)
	OFFSET_BLOOM_FILTER = system.OffsetBloomFilter
	SIZE_BLOOM_FILTER   = system.SizeBloomFilter

	// ========== SUPERVISOR HEADERS (0x002000 - 0x003000) ==========
	OFFSET_SUPERVISOR_HEADERS = system.OffsetSupervisorHeaders
	SIZE_SUPERVISOR_HEADERS   = system.SizeSupervisorHeaders
	SUPERVISOR_HEADER_SIZE    = system.SupervisorHeaderSize
	MAX_SUPERVISORS_INLINE    = system.MaxSupervisorsInline
	MAX_SUPERVISORS_TOTAL     = system.MaxSupervisorsTotal

	// ========== SYSCALL TABLE (0x003000 - 0x004000) ==========
	OFFSET_SYSCALL_TABLE = system.OffsetSyscallTable
	SIZE_SYSCALL_TABLE   = system.SizeSyscallTable

	// ========== MESH METRICS (0x004000 - 0x004100) ==========
	OFFSET_MESH_METRICS = system.OffsetMeshMetrics
	SIZE_MESH_METRICS   = system.SizeMeshMetrics

	// ========== ECONOMICS (0x004100 - 0x008000) ==========
	OFFSET_ECONOMICS = system.OffsetEconomics
	SIZE_ECONOMICS   = system.SizeEconomics

	// ========== IDENTITY REGISTRY (0x008000 - 0x00C000) ==========
	OFFSET_IDENTITY_REGISTRY = system.OffsetIdentityRegistry
	SIZE_IDENTITY_REGISTRY   = system.SizeIdentityRegistry

	// ========== SOCIAL GRAPH (0x00C000 - 0x010000) ==========
	OFFSET_SOCIAL_GRAPH = system.OffsetSocialGraph
	SIZE_SOCIAL_GRAPH   = system.SizeSocialGraph

	// ========== GLOBAL ANALYTICS (0x010000 - 0x010100) ==========
	OFFSET_GLOBAL_ANALYTICS = system.OffsetGlobalAnalytics
	SIZE_GLOBAL_ANALYTICS   = system.SizeGlobalAnalytics

	// ========== PATTERN EXCHANGE (0x010000 - 0x020000) ==========
	OFFSET_PATTERN_EXCHANGE = system.OffsetPatternExchange
	SIZE_PATTERN_EXCHANGE   = system.SizePatternExchange
	PATTERN_ENTRY_SIZE      = system.PatternEntrySize
	MAX_PATTERNS_INLINE     = system.MaxPatternsInline
	MAX_PATTERNS_TOTAL      = system.MaxPatternsTotal

	// ========== JOB HISTORY (0x020000 - 0x040000) ==========
	OFFSET_JOB_HISTORY = system.OffsetJobHistory
	SIZE_JOB_HISTORY   = system.SizeJobHistory

	// ========== COORDINATION STATE (0x040000 - 0x050000) ==========
	OFFSET_COORDINATION = system.OffsetCoordination
	SIZE_COORDINATION   = system.SizeCoordination

	// ========== INBOX/OUTBOX (0x050000 - 0x150000) ==========
	OFFSET_INBOX_OUTBOX = system.OffsetInboxOutbox
	SIZE_INBOX_OUTBOX   = system.SizeInboxOutbox

	// Sub-regions
	OFFSET_INBOX_BASE  = system.OffsetInboxBase
	SIZE_INBOX_TOTAL   = system.SizeInboxTotal
	OFFSET_OUTBOX_BASE = system.OffsetOutboxBase
	SIZE_OUTBOX_TOTAL  = system.SizeOutboxTotal

	// ========== ARENA (0x150000 - end) ==========
	OFFSET_ARENA          = system.OffsetArena
	OFFSET_ARENA_METADATA = system.OffsetArenaMetadata
	SIZE_ARENA_METADATA   = system.SizeArenaMetadata

	// Diagnostics Region
	OFFSET_DIAGNOSTICS = system.OffsetDiagnostics
	SIZE_DIAGNOSTICS   = system.SizeDiagnostics

	OFFSET_BRIDGE_METRICS = system.OffsetDiagnostics + 0x800
	SIZE_BRIDGE_METRICS   = 0x100

	// Async Request/Response Queues
	OFFSET_ARENA_REQUEST_QUEUE  = system.OffsetArenaRequestQueue
	OFFSET_ARENA_RESPONSE_QUEUE = system.OffsetArenaResponseQueue
	ARENA_QUEUE_ENTRY_SIZE      = system.ArenaQueueEntrySize
	MAX_ARENA_REQUESTS          = system.MaxArenaRequests

	// Bird Animation State
	OFFSET_BIRD_STATE = system.OffsetBirdState
	SIZE_BIRD_STATE   = system.SizeBirdState

	// ========== PING-PONG BUFFERS (Arena) ==========
	OFFSET_PINGPONG_CONTROL = system.OffsetPingpongControl
	SIZE_PINGPONG_CONTROL   = system.SizePingpongControl

	// Bird Population Data (Dual Buffers)
	OFFSET_BIRD_BUFFER_A = system.OffsetBirdBufferA
	OFFSET_BIRD_BUFFER_B = system.OffsetBirdBufferB
	SIZE_BIRD_BUFFER     = system.SizeBirdBuffer
	BIRD_STRIDE          = system.BirdStride

	// Matrix Output Data (Dual Buffers)
	OFFSET_MATRIX_BUFFER_A = system.OffsetMatrixBufferA
	OFFSET_MATRIX_BUFFER_B = system.OffsetMatrixBufferB
	SIZE_MATRIX_BUFFER     = system.SizeMatrixBuffer
	MATRIX_STRIDE          = system.MatrixStride

	// ========== ROBOT / LATTICE STATE (Moonshot) ==========
	OFFSET_ROBOT_STATE     = system.OffsetRobotState
	SIZE_ROBOT_STATE       = system.SizeRobotState
	OFFSET_ROBOT_NODES     = system.OffsetRobotNodes
	SIZE_ROBOT_NODES       = system.SizeRobotNodes
	OFFSET_ROBOT_FILAMENTS = system.OffsetRobotFilaments
	SIZE_ROBOT_FILAMENTS   = system.SizeRobotFilaments

	// ========== EPOCH INDEX ALLOCATION ==========
	// Fixed system epochs (0-31 Reserved)
	IDX_KERNEL_READY  = system.IdxKernelReady
	IDX_INBOX_DIRTY   = system.IdxInboxDirty
	IDX_OUTBOX_DIRTY  = system.IdxOutboxDirty
	IDX_PANIC_STATE   = system.IdxPanicState
	IDX_SENSOR_EPOCH  = system.IdxSensorEpoch
	IDX_ACTOR_EPOCH   = system.IdxActorEpoch
	IDX_STORAGE_EPOCH = system.IdxStorageEpoch
	IDX_SYSTEM_EPOCH  = system.IdxSystemEpoch

	// Phase 16: Extended System Epochs
	IDX_ARENA_ALLOCATOR = system.IdxArenaAllocator
	IDX_OUTBOX_MUTEX    = system.IdxOutboxMutex
	IDX_INBOX_MUTEX     = system.IdxInboxMutex
	IDX_METRICS_EPOCH   = system.IdxMetricsEpoch
	IDX_BOIDS_COUNT     = system.IdxBirdEpoch // Note: Schema uses IdxBirdEpoch, aliased here
	IDX_MATRIX_EPOCH    = system.IdxMatrixEpoch
	IDX_PINGPONG_ACTIVE = system.IdxPingpongActive

	// Signal-Based Architecture Epochs (15-20)
	IDX_REGISTRY_EPOCH       = system.IdxRegistryEpoch
	IDX_EVOLUTION_EPOCH      = system.IdxEvolutionEpoch
	IDX_HEALTH_EPOCH         = system.IdxHealthEpoch
	IDX_LEARNING_EPOCH       = system.IdxLearningEpoch
	IDX_ECONOMY_EPOCH        = system.IdxEconomyEpoch
	IDX_BIRD_COUNT           = system.IdxBirdCount // Index 20
	IDX_GLOBAL_METRICS_EPOCH = system.IdxGlobalMetricsEpoch
	IDX_ROBOT_EPOCH          = system.IdxRobotEpoch

	// Dynamic supervisor pool (32-127)
	SUPERVISOR_POOL_BASE = system.SupervisorPoolBase
	SUPERVISOR_POOL_SIZE = system.SupervisorPoolSize

	// Reserved for future expansion (128-255)
	RESERVED_POOL_BASE = system.ReservedPoolBase
	RESERVED_POOL_SIZE = system.ReservedPoolSize

	// ========== ALIGNMENT REQUIREMENTS ==========
	ALIGNMENT_CACHE_LINE = system.AlignmentCacheLine
	ALIGNMENT_PAGE       = system.AlignmentPage
	ALIGNMENT_LARGE      = system.AlignmentLarge
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
			Name:      "MeshMetrics",
			Offset:    OFFSET_MESH_METRICS,
			Size:      SIZE_MESH_METRICS,
			Purpose:   "Mesh network telemetry",
			CanExpand: false,
			MaxInline: 1,
			MaxTotal:  1,
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
			Name:      "RobotState",
			Offset:    OFFSET_ROBOT_STATE,
			Size:      SIZE_ROBOT_STATE + SIZE_ROBOT_NODES + SIZE_ROBOT_FILAMENTS,
			Purpose:   "Moonshot Morphic Lattice simulation state",
			CanExpand: false,
			MaxInline: 1,
			MaxTotal:  1,
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
