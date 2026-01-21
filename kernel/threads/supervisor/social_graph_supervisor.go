package supervisor

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
)

const (
	SOCIAL_METADATA_SIZE = 64
	// 64 (Owner) + 64 (Referrer) + 15 * 64 (CloseIds) + 15 * 4 (AddedAt) + 15 * 4 (VerifiedAt) = 1208 bytes.
	SOCIAL_ACCOUNT_SIZE = 1248 // Unified v1.9+ size
	MAX_SOCIAL_ENTRIES  = 12   // Fits 16KB with header
)

// Social Offsets
const (
	OFFSET_SOCIAL_METADATA = 0
	OFFSET_SOCIAL_ENTRIES  = SOCIAL_METADATA_SIZE
)

// SocialGraphSupervisor manages the Social Graph in SAB
type SocialGraphSupervisor struct {
	sabPtr     unsafe.Pointer
	sabSize    uint32
	baseOffset uint32

	entries map[string]uint32 // DID -> SAB Offset
	mu      sync.RWMutex
}

func NewSocialGraphSupervisor(sabPtr unsafe.Pointer, sabSize, offset uint32) *SocialGraphSupervisor {
	return &SocialGraphSupervisor{
		sabPtr:     sabPtr,
		sabSize:    sabSize,
		baseOffset: offset,
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
	closeOffset, closeExists := ss.entries[closeID]
	ss.mu.Unlock()

	if !exists {
		return fmt.Errorf("user not found: %s", did)
	}

	entry, err := ss.readEntry(offset)
	if err != nil {
		return err
	}

	// Find free slot
	now := uint32(time.Now().Unix())
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
			if entry.CloseIdAddedAt[i] == 0 {
				entry.CloseIdAddedAt[i] = now
			}
			if err := ss.writeEntry(offset, entry); err != nil {
				return err
			}
			if closeExists {
				ss.maybeVerifyPair(did, closeID, offset, closeOffset)
			}
			return nil
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
	offset := ss.baseOffset + OFFSET_SOCIAL_ENTRIES + (index * SOCIAL_ACCOUNT_SIZE)
	ss.entries[did] = offset

	entry := &foundation.SocialEntry{}
	copy(entry.OwnerDid[:], did)
	copy(entry.ReferrerDid[:], referrer)

	return offset, ss.writeEntry(offset, entry)
}

func (ss *SocialGraphSupervisor) writeEntry(offset uint32, entry *foundation.SocialEntry) error {
	if offset+SOCIAL_ACCOUNT_SIZE > ss.sabSize {
		return fmt.Errorf("offset out of bounds")
	}

	ptr := unsafe.Add(ss.sabPtr, offset)
	data := unsafe.Slice((*byte)(ptr), SOCIAL_ACCOUNT_SIZE)
	copy(data[0:64], entry.OwnerDid[:])
	copy(data[64:128], entry.ReferrerDid[:])

	for i := 0; i < 15; i++ {
		copy(data[128+(i*64):128+((i+1)*64)], entry.CloseIds[i][:])
	}

	base := 128 + (15 * 64)
	for i := 0; i < 15; i++ {
		start := base + (i * 4)
		data[start] = byte(entry.CloseIdAddedAt[i])
		data[start+1] = byte(entry.CloseIdAddedAt[i] >> 8)
		data[start+2] = byte(entry.CloseIdAddedAt[i] >> 16)
		data[start+3] = byte(entry.CloseIdAddedAt[i] >> 24)
	}

	base += 15 * 4
	for i := 0; i < 15; i++ {
		start := base + (i * 4)
		data[start] = byte(entry.CloseIdVerifiedAt[i])
		data[start+1] = byte(entry.CloseIdVerifiedAt[i] >> 8)
		data[start+2] = byte(entry.CloseIdVerifiedAt[i] >> 16)
		data[start+3] = byte(entry.CloseIdVerifiedAt[i] >> 24)
	}

	return nil
}

func (ss *SocialGraphSupervisor) readEntry(offset uint32) (*foundation.SocialEntry, error) {
	if offset+SOCIAL_ACCOUNT_SIZE > ss.sabSize {
		return nil, fmt.Errorf("offset out of bounds")
	}

	ptr := unsafe.Add(ss.sabPtr, offset)
	data := unsafe.Slice((*byte)(ptr), SOCIAL_ACCOUNT_SIZE)
	entry := &foundation.SocialEntry{}
	copy(entry.OwnerDid[:], data[0:64])
	copy(entry.ReferrerDid[:], data[64:128])

	for i := 0; i < 15; i++ {
		copy(entry.CloseIds[i][:], data[128+(i*64):128+((i+1)*64)])
	}

	base := 128 + (15 * 64)
	for i := 0; i < 15; i++ {
		start := base + (i * 4)
		entry.CloseIdAddedAt[i] = uint32(data[start]) |
			uint32(data[start+1])<<8 |
			uint32(data[start+2])<<16 |
			uint32(data[start+3])<<24
	}

	base += 15 * 4
	for i := 0; i < 15; i++ {
		start := base + (i * 4)
		entry.CloseIdVerifiedAt[i] = uint32(data[start]) |
			uint32(data[start+1])<<8 |
			uint32(data[start+2])<<16 |
			uint32(data[start+3])<<24
	}

	return entry, nil
}

func (ss *SocialGraphSupervisor) maybeVerifyPair(did, closeID string, didOffset, closeOffset uint32) {
	entry, err := ss.readEntry(didOffset)
	if err != nil {
		return
	}
	other, err := ss.readEntry(closeOffset)
	if err != nil {
		return
	}

	didIdx := findCloseIdIndex(entry, closeID)
	otherIdx := findCloseIdIndex(other, did)
	if didIdx == -1 || otherIdx == -1 {
		return
	}

	now := uint32(time.Now().Unix())
	if entry.CloseIdVerifiedAt[didIdx] == 0 {
		entry.CloseIdVerifiedAt[didIdx] = now
	}
	if other.CloseIdVerifiedAt[otherIdx] == 0 {
		other.CloseIdVerifiedAt[otherIdx] = now
	}

	_ = ss.writeEntry(didOffset, entry)
	_ = ss.writeEntry(closeOffset, other)
}

func findCloseIdIndex(entry *foundation.SocialEntry, did string) int {
	for i := 0; i < 15; i++ {
		if parseDID(entry.CloseIds[i][:]) == did {
			return i
		}
	}
	return -1
}

func parseDID(data []byte) string {
	for i, b := range data {
		if b == 0 {
			return string(data[:i])
		}
	}
	return string(data)
}
