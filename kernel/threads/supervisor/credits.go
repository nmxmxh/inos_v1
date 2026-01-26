package supervisor

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
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

const (
	accountBalanceOffset           = 0
	accountEarnedTotalOffset       = 8
	accountSpentTotalOffset        = 16
	accountLastActivityEpochOffset = 24
	accountReputationOffset        = 32
	accountDeviceCountOffset       = 36
	accountUptimeScoreOffset       = 38
	accountLastUbiClaimOffset      = 42
	accountReferrerLockedAtOffset  = 50
	accountReferrerChangedAtOffset = 58
	accountFromCreatorOffset       = 66
	accountFromReferralsOffset     = 74
	accountFromCloseIdsOffset      = 82
	accountThresholdOffset         = 90
	accountTotalSharesOffset       = 91
	accountTierOffset              = 92
	accountPendingBalanceOffset    = 96
	accountPendingEpochOffset      = 104
	accountPendingEarnedOffset     = 112
	accountPendingSpentOffset      = 120
)

const (
	economicsSealEpochOffset = 0
	economicsSealHashOffset  = 8
	economicsSealHashSize    = 32
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

	cs.FinalizePending(epoch)
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
		Balance:           int64(binary.LittleEndian.Uint64(data[accountBalanceOffset : accountBalanceOffset+8])),
		EarnedTotal:       binary.LittleEndian.Uint64(data[accountEarnedTotalOffset : accountEarnedTotalOffset+8]),
		SpentTotal:        binary.LittleEndian.Uint64(data[accountSpentTotalOffset : accountSpentTotalOffset+8]),
		LastActivityEpoch: binary.LittleEndian.Uint64(data[accountLastActivityEpochOffset : accountLastActivityEpochOffset+8]),
		ReputationScore:   math.Float32frombits(binary.LittleEndian.Uint32(data[accountReputationOffset : accountReputationOffset+4])),
		DeviceCount:       binary.LittleEndian.Uint16(data[accountDeviceCountOffset : accountDeviceCountOffset+2]),
		UptimeScore:       math.Float32frombits(binary.LittleEndian.Uint32(data[accountUptimeScoreOffset : accountUptimeScoreOffset+4])),
		LastUbiClaim:      int64(binary.LittleEndian.Uint64(data[accountLastUbiClaimOffset : accountLastUbiClaimOffset+8])),
		ReferrerLockedAt:  int64(binary.LittleEndian.Uint64(data[accountReferrerLockedAtOffset : accountReferrerLockedAtOffset+8])),
		ReferrerChangedAt: int64(binary.LittleEndian.Uint64(data[accountReferrerChangedAtOffset : accountReferrerChangedAtOffset+8])),
		FromCreator:       binary.LittleEndian.Uint64(data[accountFromCreatorOffset : accountFromCreatorOffset+8]),
		FromReferrals:     binary.LittleEndian.Uint64(data[accountFromReferralsOffset : accountFromReferralsOffset+8]),
		FromCloseIds:      binary.LittleEndian.Uint64(data[accountFromCloseIdsOffset : accountFromCloseIdsOffset+8]),
		Threshold:         data[accountThresholdOffset],
		TotalShares:       data[accountTotalSharesOffset],
		Tier:              data[accountTierOffset],
		PendingBalance:    int64(binary.LittleEndian.Uint64(data[accountPendingBalanceOffset : accountPendingBalanceOffset+8])),
		PendingEpoch:      binary.LittleEndian.Uint64(data[accountPendingEpochOffset : accountPendingEpochOffset+8]),
		PendingEarned:     binary.LittleEndian.Uint64(data[accountPendingEarnedOffset : accountPendingEarnedOffset+8]),
		PendingSpent:      binary.LittleEndian.Uint64(data[accountPendingSpentOffset : accountPendingSpentOffset+8]),
	}, nil
}

func (cs *CreditSupervisor) writeAccount(offset uint32, acc *foundation.CreditAccount) error {
	if offset+ECONOMICS_ACCOUNT_SIZE > cs.sabSize {
		return fmt.Errorf("offset out of bounds")
	}

	ptr := unsafe.Add(cs.sabPtr, offset)
	data := unsafe.Slice((*byte)(ptr), ECONOMICS_ACCOUNT_SIZE)
	binary.LittleEndian.PutUint64(data[accountBalanceOffset:accountBalanceOffset+8], uint64(acc.Balance))
	binary.LittleEndian.PutUint64(data[accountEarnedTotalOffset:accountEarnedTotalOffset+8], acc.EarnedTotal)
	binary.LittleEndian.PutUint64(data[accountSpentTotalOffset:accountSpentTotalOffset+8], acc.SpentTotal)
	binary.LittleEndian.PutUint64(data[accountLastActivityEpochOffset:accountLastActivityEpochOffset+8], acc.LastActivityEpoch)
	binary.LittleEndian.PutUint32(data[accountReputationOffset:accountReputationOffset+4], math.Float32bits(acc.ReputationScore))
	binary.LittleEndian.PutUint16(data[accountDeviceCountOffset:accountDeviceCountOffset+2], acc.DeviceCount)
	binary.LittleEndian.PutUint32(data[accountUptimeScoreOffset:accountUptimeScoreOffset+4], math.Float32bits(acc.UptimeScore))
	binary.LittleEndian.PutUint64(data[accountLastUbiClaimOffset:accountLastUbiClaimOffset+8], uint64(acc.LastUbiClaim))
	binary.LittleEndian.PutUint64(data[accountReferrerLockedAtOffset:accountReferrerLockedAtOffset+8], uint64(acc.ReferrerLockedAt))
	binary.LittleEndian.PutUint64(data[accountReferrerChangedAtOffset:accountReferrerChangedAtOffset+8], uint64(acc.ReferrerChangedAt))
	binary.LittleEndian.PutUint64(data[accountFromCreatorOffset:accountFromCreatorOffset+8], acc.FromCreator)
	binary.LittleEndian.PutUint64(data[accountFromReferralsOffset:accountFromReferralsOffset+8], acc.FromReferrals)
	binary.LittleEndian.PutUint64(data[accountFromCloseIdsOffset:accountFromCloseIdsOffset+8], acc.FromCloseIds)
	data[accountThresholdOffset] = acc.Threshold
	data[accountTotalSharesOffset] = acc.TotalShares
	data[accountTierOffset] = acc.Tier
	binary.LittleEndian.PutUint64(data[accountPendingBalanceOffset:accountPendingBalanceOffset+8], uint64(acc.PendingBalance))
	binary.LittleEndian.PutUint64(data[accountPendingEpochOffset:accountPendingEpochOffset+8], acc.PendingEpoch)
	binary.LittleEndian.PutUint64(data[accountPendingEarnedOffset:accountPendingEarnedOffset+8], acc.PendingEarned)
	binary.LittleEndian.PutUint64(data[accountPendingSpentOffset:accountPendingSpentOffset+8], acc.PendingSpent)
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

	// Atomic update of pending balance in SAB (sealed credits).
	ptr := unsafe.Add(cs.sabPtr, offset+accountPendingBalanceOffset)
	atomic.AddInt64((*int64)(ptr), delta)

	// Proper use of isEarned: Bifurcate pending accumulators for accurate metrics
	if isEarned {
		earnedPtr := (*uint64)(unsafe.Add(cs.sabPtr, offset+accountPendingEarnedOffset))
		if delta > 0 {
			atomic.AddUint64(earnedPtr, uint64(delta))
		}
	} else {
		spentPtr := (*uint64)(unsafe.Add(cs.sabPtr, offset+accountPendingSpentOffset))
		if delta < 0 {
			atomic.AddUint64(spentPtr, uint64(-delta))
		} else if delta > 0 {
			// Spending a positive adjustment (e.g. refunding a spend)
			// In production, we'd handle this via the appropriate counter
			atomic.AddUint64(spentPtr, uint64(delta))
		}
	}

	epochPtr := (*uint64)(unsafe.Add(cs.sabPtr, offset+accountPendingEpochOffset))
	atomic.StoreUint64(epochPtr, uint64(time.Now().Unix()))
}

// GetAvailableBalance returns balance minus pending spends.
func (cs *CreditSupervisor) GetAvailableBalance(did string) (int64, error) {
	acc, err := cs.GetAccount(did)
	if err != nil {
		return 0, err
	}
	if acc.PendingBalance < 0 {
		return acc.Balance + acc.PendingBalance, nil
	}
	return acc.Balance, nil
}

// ReservePending locks credits as pending spend.
func (cs *CreditSupervisor) ReservePending(did string, amount uint64) error {
	cs.settleAccount(did, -int64(amount), false)
	return nil
}

// ReleasePending credits a provider as pending earn.
func (cs *CreditSupervisor) ReleasePending(did string, amount uint64) error {
	cs.settleAccount(did, int64(amount), true)
	return nil
}

// RefundPending returns escrowed credits to the requester.
func (cs *CreditSupervisor) RefundPending(did string, amount uint64) error {
	cs.settleAccount(did, int64(amount), true)
	return nil
}

// FinalizePending applies pending credits to balances and writes a seal hash.
func (cs *CreditSupervisor) FinalizePending(epoch uint64) {
	cs.accounts.Range(func(key, value any) bool {
		offset := value.(uint32)
		pendingPtr := (*int64)(unsafe.Add(cs.sabPtr, offset+accountPendingBalanceOffset))
		pending := atomic.SwapInt64(pendingPtr, 0)
		if pending == 0 {
			return true
		}

		balancePtr := (*int64)(unsafe.Add(cs.sabPtr, offset+accountBalanceOffset))
		atomic.AddInt64(balancePtr, pending)

		// Accurate Epoch Finalization: Use dedicated pending counters
		pePtr := (*uint64)(unsafe.Add(cs.sabPtr, offset+accountPendingEarnedOffset))
		psPtr := (*uint64)(unsafe.Add(cs.sabPtr, offset+accountPendingSpentOffset))

		pe := atomic.SwapUint64(pePtr, 0)
		ps := atomic.SwapUint64(psPtr, 0)

		if pe > 0 {
			earnedPtr := (*uint64)(unsafe.Add(cs.sabPtr, offset+accountEarnedTotalOffset))
			atomic.AddUint64(earnedPtr, pe)
		}
		if ps > 0 {
			spentPtr := (*uint64)(unsafe.Add(cs.sabPtr, offset+accountSpentTotalOffset))
			atomic.AddUint64(spentPtr, ps)
		}

		lastEpochPtr := (*uint64)(unsafe.Add(cs.sabPtr, offset+accountLastActivityEpochOffset))
		atomic.StoreUint64(lastEpochPtr, epoch)
		return true
	})

	cs.writeSeal(epoch)
}

func (cs *CreditSupervisor) writeSeal(epoch uint64) {
	if cs.baseOffset+ECONOMICS_METADATA_SIZE > cs.sabSize {
		return
	}
	accountsOffset := cs.baseOffset + OFFSET_ECONOMICS_ACCOUNTS
	accountsSize := uint32(ECONOMICS_MAX_ACCOUNTS * ECONOMICS_ACCOUNT_SIZE)
	if accountsOffset+accountsSize > cs.sabSize {
		return
	}

	ptr := unsafe.Add(cs.sabPtr, accountsOffset)
	data := unsafe.Slice((*byte)(ptr), accountsSize)
	hash := sha256.Sum256(data)

	metaPtr := unsafe.Add(cs.sabPtr, cs.baseOffset+OFFSET_ECONOMICS_METADATA)
	meta := unsafe.Slice((*byte)(metaPtr), ECONOMICS_METADATA_SIZE)
	binary.LittleEndian.PutUint64(meta[economicsSealEpochOffset:economicsSealEpochOffset+8], epoch)
	copy(meta[economicsSealHashOffset:economicsSealHashOffset+economicsSealHashSize], hash[:])
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
