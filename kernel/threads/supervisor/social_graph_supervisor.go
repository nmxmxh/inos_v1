package supervisor

import (
	"fmt"
	"sync"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/sab"
)

const (
	SOCIAL_ENTRY_SIZE = 1024 // 128 (approx) + 15 * 64 = 1088. Let's make it 1024 for now and adjust.
	// Actually: 64 (Owner) + 64 (Referrer) + 15 * 64 (CloseIds) = 1152 bytes.
	// Let's use 2048 to be safe and allow expansion.
	SOCIAL_ACCOUNT_SIZE = 1248 // 64+64 + 15*64 + padding
	MAX_SOCIAL_ENTRIES  = 128
)

// SocialGraphSupervisor manages the Social Graph in SAB
type SocialGraphSupervisor struct {
	sab        []byte
	baseOffset uint32

	entries map[string]uint32 // DID -> SAB Offset
	mu      sync.RWMutex
}

func NewSocialGraphSupervisor(sabData []byte) *SocialGraphSupervisor {
	return &SocialGraphSupervisor{
		sab:        sabData,
		baseOffset: sab.OFFSET_SOCIAL_GRAPH,
		entries:    make(map[string]uint32),
	}
}

// GetReferrer returns the referrer DID for a user
func (ss *SocialGraphSupervisor) GetReferrer(did string) (string, error) {
	ss.mu.RLock()
	offset, exists := ss.entries[did]
	ss.mu.RUnlock()

	if !exists {
		return "did:inos:nmxmxh", nil // Default
	}

	entry, err := ss.readEntry(offset)
	if err != nil {
		return "did:inos:nmxmxh", nil
	}

	referrer := parseDID(entry.ReferrerDid[:])
	if referrer == "" {
		return "did:inos:nmxmxh", nil
	}

	return referrer, nil
}

// GetCloseIdentities returns the list of close IDs for a user
func (ss *SocialGraphSupervisor) GetCloseIdentities(did string) ([]string, error) {
	ss.mu.RLock()
	offset, exists := ss.entries[did]
	ss.mu.RUnlock()

	if !exists {
		return []string{}, nil
	}

	entry, err := ss.readEntry(offset)
	if err != nil {
		return []string{}, nil
	}

	var res []string
	for _, cid := range entry.CloseIds {
		idStr := parseDID(cid[:])
		if idStr != "" {
			res = append(res, idStr)
		}
	}
	return res, nil
}

// AddCloseIdentity adds a close ID relationship
func (ss *SocialGraphSupervisor) AddCloseIdentity(did, closeID string) error {
	ss.mu.Lock()
	offset, exists := ss.entries[did]
	ss.mu.Unlock()

	if !exists {
		return fmt.Errorf("user not found: %s", did)
	}

	entry, err := ss.readEntry(offset)
	if err != nil {
		return err
	}

	// Find free slot
	for i := 0; i < 15; i++ {
		// Clean the potential null-terminated string/padding
		cur := ""
		for j := 0; j < 64; j++ {
			if entry.CloseIds[i][j] == 0 {
				break
			}
			cur += string(entry.CloseIds[i][j])
		}

		if cur == "" || cur == closeID {
			copy(entry.CloseIds[i][:], closeID)
			return ss.writeEntry(offset, entry)
		}
	}

	return fmt.Errorf("close identity slots full")
}

// RegisterSocialEntry initializes a social entry for a new DID
func (ss *SocialGraphSupervisor) RegisterSocialEntry(did, referrer string) (uint32, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if offset, exists := ss.entries[did]; exists {
		return offset, nil
	}

	if len(ss.entries) >= MAX_SOCIAL_ENTRIES {
		return 0, fmt.Errorf("social graph full")
	}

	if referrer == "" {
		referrer = "did:inos:nmxmxh"
	}

	index := uint32(len(ss.entries))
	offset := ss.baseOffset + (index * SOCIAL_ACCOUNT_SIZE)
	ss.entries[did] = offset

	entry := &foundation.SocialEntry{}
	copy(entry.OwnerDid[:], did)
	copy(entry.ReferrerDid[:], referrer)

	return offset, ss.writeEntry(offset, entry)
}

func (ss *SocialGraphSupervisor) writeEntry(offset uint32, entry *foundation.SocialEntry) error {
	if offset+SOCIAL_ACCOUNT_SIZE > uint32(len(ss.sab)) {
		return fmt.Errorf("offset out of bounds")
	}

	data := ss.sab[offset : offset+SOCIAL_ACCOUNT_SIZE]
	copy(data[0:64], entry.OwnerDid[:])
	copy(data[64:128], entry.ReferrerDid[:])

	for i := 0; i < 15; i++ {
		copy(data[128+(i*64):128+((i+1)*64)], entry.CloseIds[i][:])
	}

	return nil
}

func (ss *SocialGraphSupervisor) readEntry(offset uint32) (*foundation.SocialEntry, error) {
	if offset+SOCIAL_ACCOUNT_SIZE > uint32(len(ss.sab)) {
		return nil, fmt.Errorf("offset out of bounds")
	}

	data := ss.sab[offset : offset+SOCIAL_ACCOUNT_SIZE]
	entry := &foundation.SocialEntry{}
	copy(entry.OwnerDid[:], data[0:64])
	copy(entry.ReferrerDid[:], data[64:128])

	for i := 0; i < 15; i++ {
		copy(entry.CloseIds[i][:], data[128+(i*64):128+((i+1)*64)])
	}

	return entry, nil
}

func parseDID(data []byte) string {
	for i, b := range data {
		if b == 0 {
			return string(data[:i])
		}
	}
	return string(data)
}
