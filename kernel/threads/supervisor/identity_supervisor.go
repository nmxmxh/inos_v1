package supervisor

import (
	"fmt"
	"sync"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/sab"
)

const (
	IDENTITY_ENTRY_SIZE  = 128 // Aligned to cache line
	IDENTITY_MAX_ENTRIES = 128
)

// IdentitySupervisor manages the Identity Registry in SAB
type IdentitySupervisor struct {
	sab        []byte
	baseOffset uint32

	// Local cache for performance
	dids map[string]uint32 // DID -> SAB Offset

	mu sync.RWMutex
}

func NewIdentitySupervisor(sabData []byte) *IdentitySupervisor {
	return &IdentitySupervisor{
		sab:        sabData,
		baseOffset: sab.OFFSET_IDENTITY_REGISTRY,
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
	offset := is.baseOffset + (index * IDENTITY_ENTRY_SIZE)
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
	if offset+IDENTITY_ENTRY_SIZE > uint32(len(is.sab)) {
		return fmt.Errorf("offset out of bounds")
	}

	data := is.sab[offset : offset+IDENTITY_ENTRY_SIZE]
	copy(data[0:64], entry.Did[:])
	copy(data[64:97], entry.PublicKey[:])
	data[97] = entry.Status

	return nil
}

func (is *IdentitySupervisor) readEntry(offset uint32) (*foundation.IdentityEntry, error) {
	if offset+IDENTITY_ENTRY_SIZE > uint32(len(is.sab)) {
		return nil, fmt.Errorf("offset out of bounds")
	}

	data := is.sab[offset : offset+IDENTITY_ENTRY_SIZE]
	entry := &foundation.IdentityEntry{
		Status: data[97],
	}
	copy(entry.Did[:], data[0:64])
	copy(entry.PublicKey[:], data[64:97])

	return entry, nil
}
