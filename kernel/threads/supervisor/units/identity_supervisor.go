//go:build wasm

package units

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
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

const (
	IDENTITY_STATUS_ACTIVE       = 0
	IDENTITY_STATUS_UNDER_RECOVER = 1
	IDENTITY_STATUS_REVOKED      = 2
	IDENTITY_STATUS_SYSTEM       = 3
)

const (
	identityMetaVersionOffset         = 0
	identityMetaEntryCountOffset      = 4
	identityMetaDefaultIdentityOffset = 8
	identityMetaDefaultAccountOffset  = 12
	identityMetaDefaultSocialOffset   = 16
	identityMetaDefaultStatusOffset   = 20
	identityMetaVersion               = 1
)

// IdentitySupervisor manages the Identity Registry in SAB
type IdentitySupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge supervisor.SABInterface
	credits *supervisor.CreditSupervisor
	social  *supervisor.SocialGraphSupervisor

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
	credits *supervisor.CreditSupervisor,
	social *supervisor.SocialGraphSupervisor,
	delegator foundation.MeshDelegator,
) *IdentitySupervisor {
	capabilities := []string{"identity.resolve", "identity.register", "identity.verify", "identity.attest"}
	return &IdentitySupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("identity", capabilities, patterns, knowledge, delegator, bridge, nil),
		bridge:            bridge,
		credits:           credits,
		social:            social,
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

	accountOffset, err := is.resolveAccountOffset(did)
	if err != nil {
		return 0, err
	}

	socialOffset, err := is.resolveSocialOffset(did)
	if err != nil {
		return 0, err
	}

	status := identityStatusForDID(did)
	tier := is.resolveTier()
	entry := &foundation.IdentityEntry{
		Status:            status,
		AccountOffset:     accountOffset,
		SocialOffset:      socialOffset,
		RecoveryThreshold: 1,
		TotalShares:       1,
		Tier:              tier,
	}
	copy(entry.Did[:], did)
	if publicKey != nil {
		copy(entry.PublicKey[:], publicKey)
	}

	if err := is.writeEntry(offset, entry); err != nil {
		return 0, err
	}

	is.writeMetadata(uint32(len(is.dids)), entry)
	is.signalEconomyEpoch()

	return offset, nil
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
	binary.LittleEndian.PutUint32(data[98:102], entry.AccountOffset)
	binary.LittleEndian.PutUint32(data[102:106], entry.SocialOffset)
	data[106] = entry.RecoveryThreshold
	data[107] = entry.TotalShares
	data[108] = entry.Tier
	data[109] = entry.Flags

	return nil
}

func (is *IdentitySupervisor) readEntry(offset uint32) (*foundation.IdentityEntry, error) {
	if offset+IDENTITY_ENTRY_SIZE > is.sabSize {
		return nil, fmt.Errorf("offset out of bounds")
	}

	ptr := unsafe.Add(is.sabPtr, offset)
	data := unsafe.Slice((*byte)(ptr), IDENTITY_ENTRY_SIZE)
	entry := &foundation.IdentityEntry{}
	copy(entry.Did[:], data[0:64])
	copy(entry.PublicKey[:], data[64:97])
	entry.Status = data[97]
	entry.AccountOffset = binary.LittleEndian.Uint32(data[98:102])
	entry.SocialOffset = binary.LittleEndian.Uint32(data[102:106])
	entry.RecoveryThreshold = data[106]
	entry.TotalShares = data[107]
	entry.Tier = data[108]
	entry.Flags = data[109]
	return entry, nil
}

func (is *IdentitySupervisor) SetRecoveryState(did string, underRecovery bool, threshold, totalShares uint8) error {
	is.mu.RLock()
	offset, exists := is.dids[did]
	is.mu.RUnlock()

	if !exists {
		return fmt.Errorf("DID not found: %s", did)
	}

	entry, err := is.readEntry(offset)
	if err != nil {
		return err
	}

	if underRecovery {
		entry.Status = IDENTITY_STATUS_UNDER_RECOVER
	} else if entry.Status == IDENTITY_STATUS_UNDER_RECOVER {
		entry.Status = IDENTITY_STATUS_ACTIVE
	}
	if threshold > 0 {
		entry.RecoveryThreshold = threshold
	}
	if totalShares > 0 {
		entry.TotalShares = totalShares
	}

	if err := is.writeEntry(offset, entry); err != nil {
		return err
	}
	is.writeMetadata(uint32(len(is.dids)), entry)
	is.signalEconomyEpoch()
	return nil
}

func (is *IdentitySupervisor) resolveAccountOffset(did string) (uint32, error) {
	if is.credits == nil {
		return 0, fmt.Errorf("credits supervisor not available")
	}
	return is.credits.GetOrCreateAccountOffset(did)
}

func (is *IdentitySupervisor) resolveSocialOffset(did string) (uint32, error) {
	if is.social == nil {
		return 0, fmt.Errorf("social graph not available")
	}
	return is.social.RegisterSocialEntry(did, "")
}

func (is *IdentitySupervisor) resolveTier() uint8 {
	if is.credits == nil {
		return 0
	}
	return is.credits.DefaultTier()
}

func (is *IdentitySupervisor) writeMetadata(entryCount uint32, entry *foundation.IdentityEntry) {
	ptr := unsafe.Add(is.sabPtr, is.baseOffset+OFFSET_IDENTITY_METADATA)
	data := unsafe.Slice((*byte)(ptr), IDENTITY_METADATA_SIZE)

	binary.LittleEndian.PutUint32(data[identityMetaVersionOffset:], identityMetaVersion)
	binary.LittleEndian.PutUint32(data[identityMetaEntryCountOffset:], entryCount)

	defaultOffset := binary.LittleEndian.Uint32(data[identityMetaDefaultIdentityOffset:])
	if defaultOffset == 0 || entry.Status == IDENTITY_STATUS_SYSTEM {
		binary.LittleEndian.PutUint32(data[identityMetaDefaultIdentityOffset:], is.baseOffset+OFFSET_IDENTITY_ENTRIES+uint32((entryCount-1)*IDENTITY_ENTRY_SIZE))
		binary.LittleEndian.PutUint32(data[identityMetaDefaultAccountOffset:], entry.AccountOffset)
		binary.LittleEndian.PutUint32(data[identityMetaDefaultSocialOffset:], entry.SocialOffset)
		data[identityMetaDefaultStatusOffset] = entry.Status
	}
}

func (is *IdentitySupervisor) signalEconomyEpoch() {
	if is.bridge == nil {
		return
	}
	is.bridge.SignalEpoch(sab_layout.IDX_ECONOMY_EPOCH)
}

func identityStatusForDID(did string) uint8 {
	if did == "did:inos:system" {
		return IDENTITY_STATUS_SYSTEM
	}
	return IDENTITY_STATUS_ACTIVE
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

	case "add_close_id":
		did, ok := job.Parameters["did"].(string)
		if !ok {
			return &foundation.Result{JobID: job.ID, Error: "missing did parameter"}
		}
		closeID, ok := job.Parameters["close_id"].(string)
		if !ok {
			return &foundation.Result{JobID: job.ID, Error: "missing close_id parameter"}
		}
		if is.social == nil {
			return &foundation.Result{JobID: job.ID, Error: "social graph unavailable"}
		}
		if err := is.social.AddCloseIdentity(did, closeID); err != nil {
			return &foundation.Result{JobID: job.ID, Error: err.Error()}
		}
		is.signalEconomyEpoch()
		return &foundation.Result{JobID: job.ID, Data: []byte("ok")}

	case "set_recovery":
		did, ok := job.Parameters["did"].(string)
		if !ok {
			return &foundation.Result{JobID: job.ID, Error: "missing did parameter"}
		}
		underRecovery, _ := job.Parameters["under_recovery"].(bool)
		threshold := toUint8(job.Parameters["threshold"])
		totalShares := toUint8(job.Parameters["total_shares"])
		if err := is.SetRecoveryState(did, underRecovery, threshold, totalShares); err != nil {
			return &foundation.Result{JobID: job.ID, Error: err.Error()}
		}
		return &foundation.Result{JobID: job.ID, Data: []byte("ok")}

	default:
		return &foundation.Result{
			JobID: job.ID,
			Error: "unsupported identity operation: " + job.Operation,
		}
	}
}

func toUint8(val interface{}) uint8 {
	switch v := val.(type) {
	case uint8:
		return v
	case uint16:
		return uint8(v)
	case uint32:
		return uint8(v)
	case int:
		return uint8(v)
	case int32:
		return uint8(v)
	case int64:
		return uint8(v)
	case float64:
		return uint8(v)
	default:
		return 0
	}
}
