//go:build wasm

package units

import (
	"context"
	"fmt"
	"sync"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
)

const (
	IDENTITY_METADATA_SIZE = 64
	IDENTITY_ENTRY_SIZE    = 128 // Aligned to cache line
	IDENTITY_MAX_ENTRIES   = 127 // Fits 16KB with header
)

// Identity Offsets
const (
	OFFSET_IDENTITY_METADATA = 0
	OFFSET_IDENTITY_ENTRIES  = IDENTITY_METADATA_SIZE
)

// IdentitySupervisor manages the Identity Registry in SAB
type IdentitySupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge supervisor.SABInterface

	sabPtr     unsafe.Pointer
	sabSize    uint32
	baseOffset uint32

	// Local cache for performance
	dids map[string]uint32 // DID -> SAB Offset

	mu sync.RWMutex
}

func NewIdentitySupervisor(
	bridge supervisor.SABInterface,
	patterns *pattern.TieredPatternStorage,
	knowledge *intelligence.KnowledgeGraph,
	sabPtr unsafe.Pointer,
	sabSize, offset uint32,
	delegator foundation.MeshDelegator,
) *IdentitySupervisor {
	capabilities := []string{"identity.resolve", "identity.register", "identity.verify", "identity.attest"}
	return &IdentitySupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("identity", capabilities, patterns, knowledge, delegator),
		bridge:            bridge,
		sabPtr:            sabPtr,
		sabSize:           sabSize,
		baseOffset:        offset,
		dids:              make(map[string]uint32),
	}
}

func (is *IdentitySupervisor) Start(ctx context.Context) error {
	return is.UnifiedSupervisor.Start(ctx)
}

// ResolveDID returns the account info for a given DID, or creates a system wallet entry
func (is *IdentitySupervisor) ResolveDID(did string) (uint32, error) {
	is.mu.RLock()
	offset, exists := is.dids[did]
	is.mu.RUnlock()

	if exists {
		return offset, nil
	}

	// If not found, check if it's did:inos:system
	if did == "did:inos:system" {
		return is.RegisterDID(did, nil)
	}

	return 0, fmt.Errorf("DID not found: %s", did)
}

// RegisterDID allocates space in SAB for a new DID
func (is *IdentitySupervisor) RegisterDID(did string, publicKey []byte) (uint32, error) {
	is.mu.Lock()
	defer is.mu.Unlock()

	if len(is.dids) >= IDENTITY_MAX_ENTRIES {
		return 0, fmt.Errorf("identity registry full")
	}

	if offset, exists := is.dids[did]; exists {
		return offset, nil
	}

	index := uint32(len(is.dids))
	offset := is.baseOffset + OFFSET_IDENTITY_ENTRIES + (index * IDENTITY_ENTRY_SIZE)
	is.dids[did] = offset

	entry := &foundation.IdentityEntry{
		Status: 0, // active
	}
	copy(entry.Did[:], did)
	if publicKey != nil {
		copy(entry.PublicKey[:], publicKey)
	}

	return offset, is.writeEntry(offset, entry)
}

func (is *IdentitySupervisor) writeEntry(offset uint32, entry *foundation.IdentityEntry) error {
	if offset+IDENTITY_ENTRY_SIZE > is.sabSize {
		return fmt.Errorf("offset out of bounds")
	}

	ptr := unsafe.Add(is.sabPtr, offset)
	data := unsafe.Slice((*byte)(ptr), IDENTITY_ENTRY_SIZE)
	copy(data[0:64], entry.Did[:])
	copy(data[64:97], entry.PublicKey[:])
	data[97] = entry.Status

	return nil
}

// ExecuteJob overrides base ExecuteJob for identity-specific tasks
func (is *IdentitySupervisor) ExecuteJob(job *foundation.Job) *foundation.Result {
	switch job.Operation {
	case "resolve":
		did, ok := job.Parameters["did"].(string)
		if !ok {
			return &foundation.Result{JobID: job.ID, Error: "missing did parameter"}
		}
		offset, err := is.ResolveDID(did)
		if err != nil {
			return &foundation.Result{JobID: job.ID, Error: err.Error()}
		}
		return &foundation.Result{JobID: job.ID, Data: []byte(fmt.Sprintf("%d", offset))}

	case "register":
		did, ok := job.Parameters["did"].(string)
		if !ok {
			return &foundation.Result{JobID: job.ID, Error: "missing did parameter"}
		}
		publicKey, _ := job.Parameters["public_key"].([]byte)
		offset, err := is.RegisterDID(did, publicKey)
		if err != nil {
			return &foundation.Result{JobID: job.ID, Error: err.Error()}
		}
		return &foundation.Result{JobID: job.ID, Data: []byte(fmt.Sprintf("%d", offset))}

	default:
		return &foundation.Result{
			JobID: job.ID,
			Error: "unsupported identity operation: " + job.Operation,
		}
	}
}
