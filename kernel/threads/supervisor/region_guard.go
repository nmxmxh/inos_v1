package supervisor

import (
	"fmt"

	"github.com/nmxmxh/inos_v1/kernel/threads/sab"
)

// Guard entry layout (4 x i32)
const (
	guardLock      = 0
	guardLastEpoch = 1
	guardViolations = 2
	guardLastOwner = 3
)

// RegionGuard enforces write ownership for a SAB region.
type RegionGuard struct {
	bridge     *SABBridge
	policy     sab.RegionPolicy
	owner      sab.RegionOwner
	startEpoch *uint32
	locked     bool
}

// AcquireRegionWrite enforces the region policy and (if required) obtains a write lock.
func (sb *SABBridge) AcquireRegionWrite(region sab.RegionId, owner sab.RegionOwner) (*RegionGuard, error) {
	policy := sab.PolicyFor(region)

	if policy.WriterMask&owner == 0 {
		sb.incrementRegionViolation(region)
		return nil, fmt.Errorf("guard: writer not allowed for region %d", region)
	}

	if uint32(region) >= sab.REGION_GUARD_COUNT {
		return nil, fmt.Errorf("guard: region id out of range: %d", region)
	}

	guard := &RegionGuard{
		bridge: sb,
		policy: policy,
		owner:  owner,
	}

	switch policy.Access {
	case sab.AccessReadOnly:
		sb.incrementRegionViolation(region)
		return nil, fmt.Errorf("guard: region is read-only")
	case sab.AccessSingleWriter:
		if !sb.guardCAS(region, guardLock, 0, uint32(owner)) {
			sb.incrementRegionViolation(region)
			return nil, fmt.Errorf("guard: region already locked")
		}
		guard.locked = true
	case sab.AccessMultiWriter:
		// No lock, but record last owner for telemetry
		sb.guardStore(region, guardLastOwner, uint32(owner))
	}

	if policy.EpochIndex != nil {
		current := sb.AtomicLoad(*policy.EpochIndex)
		guard.startEpoch = &current
	}

	return guard, nil
}

// AcquireRegionWriteGuard exposes a minimal interface for external packages (e.g., mesh).
func (sb *SABBridge) AcquireRegionWriteGuard(region sab.RegionId, owner sab.RegionOwner) (sab.RegionWriteGuard, error) {
	return sb.AcquireRegionWrite(region, owner)
}

// ValidateRegionRead checks read ownership without locking.
func (sb *SABBridge) ValidateRegionRead(region sab.RegionId, owner sab.RegionOwner) error {
	policy := sab.PolicyFor(region)
	if policy.ReaderMask&owner == 0 {
		sb.incrementRegionViolation(region)
		return fmt.Errorf("guard: reader not allowed for region %d", region)
	}
	return nil
}

// EnsureEpochAdvanced validates that the region epoch moved forward.
func (g *RegionGuard) EnsureEpochAdvanced() error {
	if g.policy.EpochIndex == nil || g.startEpoch == nil {
		return nil
	}
	current := g.bridge.AtomicLoad(*g.policy.EpochIndex)
	if current <= *g.startEpoch {
		g.bridge.incrementRegionViolation(g.policy.RegionID)
		return fmt.Errorf("guard: epoch not advanced for region %d", g.policy.RegionID)
	}
	g.bridge.guardStore(g.policy.RegionID, guardLastEpoch, current)
	return nil
}

// Release releases the region lock if held.
func (g *RegionGuard) Release() error {
	if !g.locked {
		return nil
	}
	if !g.bridge.guardCAS(g.policy.RegionID, guardLock, uint32(g.owner), 0) {
		g.bridge.incrementRegionViolation(g.policy.RegionID)
		return fmt.Errorf("guard: release failed (owner mismatch)")
	}
	g.locked = false
	return nil
}

func (sb *SABBridge) guardIndex(region sab.RegionId, field uint32) uint32 {
	entryWords := sab.REGION_GUARD_ENTRY_SIZE / 4
	return (sab.OFFSET_REGION_GUARDS / 4) + uint32(region)*entryWords + field
}

func (sb *SABBridge) guardLoad(region sab.RegionId, field uint32) uint32 {
	return sb.atomicLoadDirect(sb.guardIndex(region, field))
}

func (sb *SABBridge) guardStore(region sab.RegionId, field uint32, value uint32) {
	sb.atomicStoreDirect(sb.guardIndex(region, field), value)
}

func (sb *SABBridge) guardCAS(region sab.RegionId, field uint32, old, new uint32) bool {
	return sb.atomicCASDirect(sb.guardIndex(region, field), old, new)
}

func (sb *SABBridge) incrementRegionViolation(region sab.RegionId) {
	sb.atomicAddDirect(sb.guardIndex(region, guardViolations), 1)
}
