package sab

// Region Owner bitmask (shared across Go/Rust/JS)
type RegionOwner uint32

const (
	RegionOwnerKernel RegionOwner = 1 << 0
	RegionOwnerModule RegionOwner = 1 << 1
	RegionOwnerHost   RegionOwner = 1 << 2
	RegionOwnerSystem RegionOwner = 1 << 3
)

// AccessMode defines how a region is protected.
type AccessMode int

const (
	AccessReadOnly AccessMode = iota
	AccessSingleWriter
	AccessMultiWriter
)

// RegionId identifies guard-protected SAB regions.
type RegionId uint32

const (
	RegionInbox RegionId = iota
	RegionOutboxHost
	RegionOutboxKernel
	RegionMeshEventQueue
	RegionArenaRequestQueue
	RegionArenaResponseQueue
	RegionArena
)

// RegionPolicy declares who can access a region and how.
type RegionPolicy struct {
	RegionID   RegionId
	Access     AccessMode
	WriterMask RegionOwner
	ReaderMask RegionOwner
	EpochIndex *uint32
}

// RegionWriteGuard is a minimal interface for guarded writes.
type RegionWriteGuard interface {
	EnsureEpochAdvanced() error
	Release() error
}

// PolicyFor returns the canonical policy for a region.
func PolicyFor(region RegionId) RegionPolicy {
	switch region {
	case RegionInbox:
		return RegionPolicy{
			RegionID:   region,
			Access:     AccessSingleWriter,
			WriterMask: RegionOwnerKernel,
			ReaderMask: RegionOwnerModule | RegionOwnerHost,
			EpochIndex: ptrUint32(IDX_INBOX_DIRTY),
		}
	case RegionOutboxHost:
		return RegionPolicy{
			RegionID:   region,
			Access:     AccessSingleWriter,
			WriterMask: RegionOwnerKernel,
			ReaderMask: RegionOwnerHost,
			EpochIndex: ptrUint32(IDX_OUTBOX_HOST_DIRTY),
		}
	case RegionOutboxKernel:
		return RegionPolicy{
			RegionID:   region,
			Access:     AccessMultiWriter,
			WriterMask: RegionOwnerModule,
			ReaderMask: RegionOwnerKernel,
			EpochIndex: ptrUint32(IDX_OUTBOX_KERNEL_DIRTY),
		}
	case RegionMeshEventQueue:
		return RegionPolicy{
			RegionID:   region,
			Access:     AccessSingleWriter,
			WriterMask: RegionOwnerKernel,
			ReaderMask: RegionOwnerHost,
			EpochIndex: ptrUint32(IDX_MESH_EVENT_EPOCH),
		}
	case RegionArenaRequestQueue:
		return RegionPolicy{
			RegionID:   region,
			Access:     AccessSingleWriter,
			WriterMask: RegionOwnerModule,
			ReaderMask: RegionOwnerKernel,
			EpochIndex: ptrUint32(IDX_ARENA_ALLOCATOR),
		}
	case RegionArenaResponseQueue:
		return RegionPolicy{
			RegionID:   region,
			Access:     AccessSingleWriter,
			WriterMask: RegionOwnerKernel,
			ReaderMask: RegionOwnerModule,
			EpochIndex: nil,
		}
	case RegionArena:
		return RegionPolicy{
			RegionID:   region,
			Access:     AccessMultiWriter,
			WriterMask: RegionOwnerKernel | RegionOwnerModule,
			ReaderMask: RegionOwnerKernel | RegionOwnerModule | RegionOwnerHost,
			EpochIndex: nil,
		}
	default:
		return RegionPolicy{
			RegionID:   region,
			Access:     AccessReadOnly,
			WriterMask: 0,
			ReaderMask: 0,
			EpochIndex: nil,
		}
	}
}

func ptrUint32(v uint32) *uint32 {
	return &v
}
