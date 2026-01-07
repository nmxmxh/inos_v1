package supervisor

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
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
	sabPtr     unsafe.Pointer
	sabSize    uint32
	baseOffset uint32

	// Local cache for performance
	dids map[string]uint32 // DID -> SAB Offset

	mu sync.RWMutex
}

func NewIdentitySupervisor(sabPtr unsafe.Pointer, sabSize, offset uint32) *IdentitySupervisor {
	return &IdentitySupervisor{
		sabPtr:     sabPtr,
		sabSize:    sabSize,
		baseOffset: offset,
		dids:       make(map[string]uint32),
	}
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
