package supervisor

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
)

// Economics constants
const (
	ECONOMICS_METADATA_SIZE = 64
	ECONOMICS_ACCOUNT_SIZE  = 128 // Unified v1.9 (Account struct size)
	ECONOMICS_METRICS_SIZE  = 64
	ECONOMICS_MAX_ACCOUNTS  = 64
	ECONOMICS_MAX_METRICS   = 64
)

const (
	ResourceTierLight     uint8 = 0
	ResourceTierModerate  uint8 = 1
	ResourceTierHeavy     uint8 = 2
	ResourceTierDedicated uint8 = 3
)

// Economics Offsets within the Economics region
const (
	OFFSET_ECONOMICS_METADATA = 0
	OFFSET_ECONOMICS_ACCOUNTS = ECONOMICS_METADATA_SIZE
	OFFSET_ECONOMICS_METRICS  = OFFSET_ECONOMICS_ACCOUNTS + (ECONOMICS_MAX_ACCOUNTS * ECONOMICS_ACCOUNT_SIZE)
)

// CreditSupervisor manages the economic state in SAB
type CreditSupervisor struct {
	sabPtr     unsafe.Pointer
	sabSize    uint32
	baseOffset uint32
	capacity   uint32

	rates foundation.EconomicRates

	// Local cache for performance
	accounts  sync.Map // ID (string) -> Offset (uint32)
	nextIndex uint32   // Atomic counter for account allocation
}

// NewCreditSupervisor creates a new credit supervisor managing SAB economics
func NewCreditSupervisor(sabPtr unsafe.Pointer, sabSize, baseOffset uint32) *CreditSupervisor {
	cs := &CreditSupervisor{
		sabPtr:     sabPtr,
		sabSize:    sabSize,
		baseOffset: baseOffset,
		capacity:   ECONOMICS_MAX_ACCOUNTS,
		rates: foundation.EconomicRates{
			ComputeRate:        1.0,
			BandwidthRate:      0.001,
			StorageRate:        0.0001,
			UptimeRate:         0.1,
			LocalityBonus:      0.5,
			SyscallCost:        0.01,
			ReplicationCost:    1.0,
			SchedulingCost:     0.5,
			PressureMultiplier: 0.1,
		},
	}
	// Note: sync.Map doesn't need explicit initialization
	return cs
}

// RegisterAccount allocates space in SAB for a new account
func (cs *CreditSupervisor) RegisterAccount(id string) (uint32, error) {
	if val, ok := cs.accounts.Load(id); ok {
		return val.(uint32), nil
	}

	// Atomic allocation check
	currentIndex := atomic.LoadUint32(&cs.nextIndex)
	if currentIndex >= ECONOMICS_MAX_ACCOUNTS {
		return 0, fmt.Errorf("max accounts reached")
	}

	// Calculate and store
	index := atomic.AddUint32(&cs.nextIndex, 1) - 1
	offset := cs.baseOffset + OFFSET_ECONOMICS_ACCOUNTS + (index * ECONOMICS_ACCOUNT_SIZE)

	actual, loaded := cs.accounts.LoadOrStore(id, offset)
	if loaded {
		return actual.(uint32), nil
	}

	// Initialize account in SAB (lock-free write once)
	acc := &foundation.CreditAccount{
		Balance:           0,
		EarnedTotal:       0,
		SpentTotal:        0,
		LastActivityEpoch: 0,
		ReputationScore:   0.5,
		DeviceCount:       1,
		UptimeScore:       1.0,
		Tier:              resolveResourceTier(cs.sabSize),
		Threshold:         1,
		TotalShares:       1,
	}
	return offset, cs.writeAccount(offset, acc)
}

// GetOrCreateAccountOffset returns the SAB offset for a DID-backed account.
func (cs *CreditSupervisor) GetOrCreateAccountOffset(did string) (uint32, error) {
	if val, ok := cs.accounts.Load(did); ok {
		return val.(uint32), nil
	}
	return cs.RegisterAccount(did)
}

func (cs *CreditSupervisor) DefaultTier() uint8 {
	return resolveResourceTier(cs.sabSize)
}

// OnEpoch settle metrics and update accounts
// GetAccount retrieves an account by ID
func (cs *CreditSupervisor) GetAccount(id string) (foundation.CreditAccount, error) {
	val, ok := cs.accounts.Load(id)
	if !ok {
		return foundation.CreditAccount{}, fmt.Errorf("account not found: %s", id)
	}

	offset := val.(uint32)
	acc, err := cs.readAccount(offset)
	if err != nil {
		return foundation.CreditAccount{}, err
	}
	return *acc, nil
}

// GetBalance implements foundation.EconomicVault
func (cs *CreditSupervisor) GetBalance(did string) (int64, error) {
	acc, err := cs.GetAccount(did)
	if err != nil {
		return 0, err
	}
	return acc.Balance, nil
}

// GrantBonus implements foundation.EconomicVault
func (cs *CreditSupervisor) GrantBonus(did string, amount int64) error {
	cs.settleAccount(did, amount, true)
	return nil
}

// RegisterSABAccount wraps RegisterAccount to satisfy foundation.EconomicVault
func (cs *CreditSupervisor) RegisterSABAccount(did string) error {
	_, err := cs.RegisterAccount(did)
	return err
}

func (cs *CreditSupervisor) OnEpoch(epoch uint64) error {
	// For each metrics entry in SAB (256 slots)
	for i := uint32(0); i < ECONOMICS_MAX_METRICS; i++ {
		metricsOffset := cs.baseOffset + OFFSET_ECONOMICS_METRICS + (i * ECONOMICS_METRICS_SIZE)
		metrics, err := cs.readMetrics(metricsOffset)
		if err != nil || metrics.ComputeCyclesUsed == 0 {
			continue
		}

		// Update registered accounts based on metrics
		cs.accounts.Range(func(key, value any) bool {
			id := key.(string)
			offset := value.(uint32)

			acc, err := cs.readAccount(offset)
			if err != nil {
				return true
			}

			// Apply multiplier based on device count (v1.1 Principle)
			multiplier := 1.0 + (float64(acc.DeviceCount) * 0.001)

			// Calculate delta (v1.0 Principle)
			delta := float64(cs.economic_tick(metrics, 1.0/12.0)) * multiplier

			if delta != 0 {
				cs.settleAccount(id, int64(delta), delta > 0)
			}
			return true
		})

		// 2. Process UBI Drip for all accounts (from Treasury)
		cs.ProcessUBIDrip(epoch)

		cs.resetMetrics(metricsOffset)
	}

	return nil
}

// ProcessUBIDrip distributes credits from did:inos:treasury to all accounts
func (cs *CreditSupervisor) ProcessUBIDrip(epoch uint64) {
	// 1. Get Treasury balance
	val, ok := cs.accounts.Load("did:inos:treasury")
	if !ok {
		return
	}
	treasuryOffset := val.(uint32)
	treasury, err := cs.readAccount(treasuryOffset)
	if err != nil || treasury.Balance <= 0 {
		return
	}

	// 2. Calculate baseline drip (e.g., 1 credit per epoch)
	baselineDrip := int64(1)

	cs.accounts.Range(func(key, value any) bool {
		id := key.(string)
		offset := value.(uint32)

		if id == "did:inos:treasury" || id == "did:inos:nmxmxh" {
			return true
		}

		acc, err := cs.readAccount(offset)
		if err != nil {
			return true
		}

		// Apply device multiplier: 1.0 + (devices * 0.001)
		multiplier := 1.0 + (float64(acc.DeviceCount) * 0.001)
		drip := int64(float64(baselineDrip) * multiplier)

		if treasury.Balance >= drip {
			// Atomic transfer logic (simplified for drip)
			cs.settleAccount(id, drip, true)
			cs.settleAccount("did:inos:treasury", -drip, false)
		}
		return true
	})
}

// Internal Accessors

func (cs *CreditSupervisor) readAccount(offset uint32) (*foundation.CreditAccount, error) {
	if offset+ECONOMICS_ACCOUNT_SIZE > cs.sabSize {
		return nil, fmt.Errorf("offset out of bounds")
	}

	ptr := unsafe.Add(cs.sabPtr, offset)
	data := unsafe.Slice((*byte)(ptr), ECONOMICS_ACCOUNT_SIZE)
	return &foundation.CreditAccount{
		Balance:           int64(binary.LittleEndian.Uint64(data[0:8])),
		EarnedTotal:       binary.LittleEndian.Uint64(data[8:16]),
		SpentTotal:        binary.LittleEndian.Uint64(data[16:24]),
		LastActivityEpoch: binary.LittleEndian.Uint64(data[24:32]),
		ReputationScore:   math.Float32frombits(binary.LittleEndian.Uint32(data[32:36])),
		DeviceCount:       binary.LittleEndian.Uint16(data[36:38]),
		UptimeScore:       math.Float32frombits(binary.LittleEndian.Uint32(data[38:42])),
		LastUbiClaim:      int64(binary.LittleEndian.Uint64(data[42:50])),
		ReferrerLockedAt:  int64(binary.LittleEndian.Uint64(data[50:58])),
		ReferrerChangedAt: int64(binary.LittleEndian.Uint64(data[58:66])),
		FromCreator:       binary.LittleEndian.Uint64(data[66:74]),
		FromReferrals:     binary.LittleEndian.Uint64(data[74:82]),
		FromCloseIds:      binary.LittleEndian.Uint64(data[82:90]),
		Threshold:         data[90],
		TotalShares:       data[91],
		Tier:              data[92],
	}, nil
}

func (cs *CreditSupervisor) writeAccount(offset uint32, acc *foundation.CreditAccount) error {
	if offset+ECONOMICS_ACCOUNT_SIZE > cs.sabSize {
		return fmt.Errorf("offset out of bounds")
	}

	ptr := unsafe.Add(cs.sabPtr, offset)
	data := unsafe.Slice((*byte)(ptr), ECONOMICS_ACCOUNT_SIZE)
	binary.LittleEndian.PutUint64(data[0:8], uint64(acc.Balance))
	binary.LittleEndian.PutUint64(data[8:16], acc.EarnedTotal)
	binary.LittleEndian.PutUint64(data[16:24], acc.SpentTotal)
	binary.LittleEndian.PutUint64(data[24:32], acc.LastActivityEpoch)
	binary.LittleEndian.PutUint32(data[32:36], math.Float32bits(acc.ReputationScore))
	binary.LittleEndian.PutUint16(data[36:38], acc.DeviceCount)
	binary.LittleEndian.PutUint32(data[38:42], math.Float32bits(acc.UptimeScore))
	binary.LittleEndian.PutUint64(data[42:50], uint64(acc.LastUbiClaim))
	binary.LittleEndian.PutUint64(data[50:58], uint64(acc.ReferrerLockedAt))
	binary.LittleEndian.PutUint64(data[58:66], uint64(acc.ReferrerChangedAt))
	binary.LittleEndian.PutUint64(data[66:74], acc.FromCreator)
	binary.LittleEndian.PutUint64(data[74:82], acc.FromReferrals)
	binary.LittleEndian.PutUint64(data[82:90], acc.FromCloseIds)
	data[90] = acc.Threshold
	data[91] = acc.TotalShares
	data[92] = acc.Tier
	return nil
}

func (cs *CreditSupervisor) readMetrics(offset uint32) (*foundation.ResourceMetrics, error) {
	if offset+ECONOMICS_METRICS_SIZE > cs.sabSize {
		return nil, fmt.Errorf("offset out of bounds")
	}

	ptr := unsafe.Add(cs.sabPtr, offset)
	data := unsafe.Slice((*byte)(ptr), ECONOMICS_METRICS_SIZE)
	return &foundation.ResourceMetrics{
		ComputeCyclesUsed:   binary.LittleEndian.Uint64(data[0:8]),
		BytesServed:         binary.LittleEndian.Uint64(data[8:16]),
		BytesStored:         binary.LittleEndian.Uint64(data[16:24]),
		UptimeSeconds:       binary.LittleEndian.Uint64(data[24:32]),
		LocalityScore:       math.Float32frombits(binary.LittleEndian.Uint32(data[32:36])),
		SyscallCount:        binary.LittleEndian.Uint64(data[36:44]),
		MemoryPressure:      math.Float32frombits(binary.LittleEndian.Uint32(data[44:48])),
		ReplicationPriority: binary.LittleEndian.Uint32(data[48:52]),
		SchedulingBias:      int32(binary.LittleEndian.Uint32(data[52:56])),
	}, nil
}

func (cs *CreditSupervisor) resetMetrics(offset uint32) {
	ptr := unsafe.Add(cs.sabPtr, offset)
	data := unsafe.Slice((*byte)(ptr), ECONOMICS_METRICS_SIZE)
	for i := range data {
		data[i] = 0
	}
}

// DistributePoUWYield splits a job value according to the 5% protocol fee
func (cs *CreditSupervisor) DistributePoUWYield(workerId, referrerId string, closeIds []string, jobValue uint64) error {

	// 1. Calculate Splits
	protocolFee := uint64(float64(jobValue) * 0.05)
	workerReward := jobValue - protocolFee

	treasuryAmt := uint64(float64(protocolFee) * (3.5 / 5.0))
	creatorAmt := uint64(float64(protocolFee) * (0.5 / 5.0))
	referrerAmt := uint64(float64(protocolFee) * (0.5 / 5.0))
	closeIdTotalAmt := protocolFee - treasuryAmt - creatorAmt - referrerAmt

	// 2. Worker Reward (95%)
	cs.settleAccount(workerId, int64(workerReward), true)

	// 4. Creator nmxmxh (0.5%)
	cs.settleAccount("did:inos:nmxmxh", int64(creatorAmt), true)

	// 5. Referrer (0.5%) - Fallback to treasury if none
	if referrerId != "" {
		cs.settleAccount(referrerId, int64(referrerAmt), true)
	} else {
		treasuryAmt += referrerAmt
	}

	// 6. Close IDs (0.5% shared) - Fallback to treasury if none
	if len(closeIds) > 0 {
		perCloseId := closeIdTotalAmt / uint64(len(closeIds))
		for _, cid := range closeIds {
			cs.settleAccount(cid, int64(perCloseId), true)
		}
	} else {
		treasuryAmt += closeIdTotalAmt
	}

	// 3. Treasury (3.5% + fallbacks)
	cs.settleAccount("did:inos:treasury", int64(treasuryAmt), true)

	return nil
}

// settleAccount applies a delta to an account and updates totals
func (cs *CreditSupervisor) settleAccount(id string, delta int64, isEarned bool) {
	val, exists := cs.accounts.Load(id)
	var offset uint32
	if !exists {
		// Auto-register
		var err error
		offset, err = cs.RegisterAccount(id)
		if err != nil {
			return
		}
	} else {
		offset = val.(uint32)
	}

	// Atomic update of balance in SAB
	ptr := unsafe.Add(cs.sabPtr, offset)
	// Balance is at offset 0
	atomic.AddInt64((*int64)(ptr), delta)

	if delta > 0 && isEarned {
		// EarnedTotal is at offset 8
		atomic.AddUint64((*uint64)(unsafe.Add(ptr, 8)), uint64(delta))
	} else if delta < 0 {
		// SpentTotal is at offset 16
		atomic.AddUint64((*uint64)(unsafe.Add(ptr, 16)), uint64(math.Abs(float64(delta))))
	}
}

// economic_tick calculates the delta for an epoch based on metrics
func (cs *CreditSupervisor) economic_tick(metrics *foundation.ResourceMetrics, hoursSinceLast float64) int64 {
	earned := (float64(metrics.ComputeCyclesUsed) * cs.rates.ComputeRate) +
		(float64(metrics.BytesServed) * cs.rates.BandwidthRate) +
		(float64(metrics.BytesStored) * cs.rates.StorageRate * hoursSinceLast) +
		(float64(metrics.UptimeSeconds) * cs.rates.UptimeRate) +
		(float64(metrics.LocalityScore) * cs.rates.LocalityBonus)

	spent := (float64(metrics.SyscallCount)*cs.rates.SyscallCost)*
		(1.0+float64(metrics.MemoryPressure)) +
		(float64(metrics.ReplicationPriority) * cs.rates.ReplicationCost) +
		(float64(metrics.SchedulingBias) * cs.rates.SchedulingCost)

	return int64(earned - spent)
}

// GetStats returns aggregate economic statistics
func (cs *CreditSupervisor) GetStats() map[string]interface{} {
	var totalBalance int64
	var accountCount int

	cs.accounts.Range(func(key, value any) bool {
		offset := value.(uint32)
		acc, err := cs.readAccount(offset)
		if err == nil {
			totalBalance += acc.Balance
			accountCount++
		}
		return true
	})

	return map[string]interface{}{
		"active":         true,
		"account_count":  accountCount,
		"total_balance":  totalBalance,
		"pending_escrow": 0,
		"earnings_rate":  0.0,
	}
}

func resolveResourceTier(sabSize uint32) uint8 {
	switch {
	case sabSize >= sab_layout.SAB_SIZE_DEDICATED:
		return ResourceTierDedicated
	case sabSize >= sab_layout.SAB_SIZE_HEAVY:
		return ResourceTierHeavy
	case sabSize >= sab_layout.SAB_SIZE_MODERATE:
		return ResourceTierModerate
	default:
		return ResourceTierLight
	}
}
