package supervisor

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
)

// Economics constants
const (
	ECONOMICS_METADATA_SIZE = 64
	ECONOMICS_ACCOUNT_SIZE  = 128 // Unified v1.9 (Account struct size)
	ECONOMICS_METRICS_SIZE  = 64
	ECONOMICS_MAX_ACCOUNTS  = 1024
	ECONOMICS_MAX_METRICS   = 256
)

// Economics Offsets within the Economics region
const (
	OFFSET_ECONOMICS_METADATA = 0
	OFFSET_ECONOMICS_ACCOUNTS = ECONOMICS_METADATA_SIZE
	OFFSET_ECONOMICS_METRICS  = OFFSET_ECONOMICS_ACCOUNTS + (ECONOMICS_MAX_ACCOUNTS * ECONOMICS_ACCOUNT_SIZE)
)

// CreditSupervisor manages the economic state in SAB
type CreditSupervisor struct {
	sab        []byte
	baseOffset uint32
	capacity   uint32

	rates foundation.EconomicRates

	// Local cache for performance
	accounts map[string]uint32 // ID -> SAB Offset

	mu sync.RWMutex
}

// NewCreditSupervisor creates a new credit supervisor managing SAB economics
func NewCreditSupervisor(sabData []byte, baseOffset uint32) *CreditSupervisor {
	return &CreditSupervisor{
		sab:        sabData,
		baseOffset: baseOffset,
		capacity:   ECONOMICS_MAX_ACCOUNTS,
		accounts:   make(map[string]uint32),
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
}

// RegisterAccount allocates space in SAB for a new account
func (cs *CreditSupervisor) RegisterAccount(id string) (uint32, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if len(cs.accounts) >= ECONOMICS_MAX_ACCOUNTS {
		return 0, fmt.Errorf("max accounts reached")
	}

	if offset, exists := cs.accounts[id]; exists {
		return offset, nil
	}

	index := uint32(len(cs.accounts))
	offset := cs.baseOffset + OFFSET_ECONOMICS_ACCOUNTS + (index * ECONOMICS_ACCOUNT_SIZE)
	cs.accounts[id] = offset

	// Initialize account in SAB
	acc := &foundation.CreditAccount{
		Balance:           0,
		EarnedTotal:       0,
		SpentTotal:        0,
		LastActivityEpoch: 0,
		ReputationScore:   0.5,
		DeviceCount:       1,
		UptimeScore:       1.0,
	}
	return offset, cs.writeAccount(offset, acc)
}

// OnEpoch settle metrics and update accounts
// GetAccount retrieves an account by ID
func (cs *CreditSupervisor) GetAccount(id string) (foundation.CreditAccount, error) {
	cs.mu.RLock()
	offset, exists := cs.accounts[id]
	cs.mu.RUnlock()

	if !exists {
		return foundation.CreditAccount{}, fmt.Errorf("account not found: %s", id)
	}

	acc, err := cs.readAccount(offset)
	if err != nil {
		return foundation.CreditAccount{}, err
	}
	return *acc, nil
}

func (cs *CreditSupervisor) OnEpoch(epoch uint64) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// For each metrics entry in SAB (256 slots)
	for i := uint32(0); i < ECONOMICS_MAX_METRICS; i++ {
		metricsOffset := cs.baseOffset + OFFSET_ECONOMICS_METRICS + (i * ECONOMICS_METRICS_SIZE)
		metrics, err := cs.readMetrics(metricsOffset)
		if err != nil || metrics.ComputeCyclesUsed == 0 {
			continue
		}

		// Update registered accounts based on metrics
		for id, offset := range cs.accounts {
			acc, err := cs.readAccount(offset)
			if err != nil {
				continue
			}

			// Apply multiplier based on device count (v1.1 Principle)
			multiplier := 1.0 + (float64(acc.DeviceCount) * 0.001)

			// Calculate delta (v1.0 Principle)
			delta := float64(cs.economic_tick(metrics, 1.0/12.0)) * multiplier

			// Update balance
			acc.Balance += int64(delta)
			if delta > 0 {
				acc.EarnedTotal += uint64(delta)
			} else {
				acc.SpentTotal += uint64(math.Abs(delta))
			}
			acc.LastActivityEpoch = epoch

			cs.writeAccount(offset, acc)
			fmt.Printf("Settled account %s: delta %f, new balance %d\n", id, delta, acc.Balance)
		}

		// 2. Process UBI Drip for all accounts (from Treasury)
		cs.ProcessUBIDrip(epoch)

		cs.resetMetrics(metricsOffset)
	}

	return nil
}

// ProcessUBIDrip distributes credits from did:inos:treasury to all accounts
func (cs *CreditSupervisor) ProcessUBIDrip(epoch uint64) {
	// 1. Get Treasury balance
	treasuryOffset, exists := cs.accounts["did:inos:treasury"]
	if !exists {
		return
	}
	treasury, err := cs.readAccount(treasuryOffset)
	if err != nil || treasury.Balance <= 0 {
		return
	}

	// 2. Calculate baseline drip (e.g., 1 credit per epoch)
	baselineDrip := int64(1)

	for id, offset := range cs.accounts {
		if id == "did:inos:treasury" || id == "did:inos:nmxmxh" {
			continue
		}

		acc, err := cs.readAccount(offset)
		if err != nil {
			continue
		}

		// Apply device multiplier: 1.0 + (devices * 0.001)
		multiplier := 1.0 + (float64(acc.DeviceCount) * 0.001)
		drip := int64(float64(baselineDrip) * multiplier)

		if treasury.Balance >= drip {
			acc.Balance += drip
			acc.EarnedTotal += uint64(drip)
			acc.LastUbiClaim = int64(epoch)

			treasury.Balance -= drip
			cs.writeAccount(offset, acc)
		}
	}

	cs.writeAccount(treasuryOffset, treasury)
}

// Internal Accessors

func (cs *CreditSupervisor) readAccount(offset uint32) (*foundation.CreditAccount, error) {
	if offset+ECONOMICS_ACCOUNT_SIZE > uint32(len(cs.sab)) {
		return nil, fmt.Errorf("offset out of bounds")
	}

	data := cs.sab[offset : offset+ECONOMICS_ACCOUNT_SIZE]
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
	if offset+ECONOMICS_ACCOUNT_SIZE > uint32(len(cs.sab)) {
		return fmt.Errorf("offset out of bounds")
	}

	data := cs.sab[offset : offset+ECONOMICS_ACCOUNT_SIZE]
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
	if offset+ECONOMICS_METRICS_SIZE > uint32(len(cs.sab)) {
		return nil, fmt.Errorf("offset out of bounds")
	}

	data := cs.sab[offset : offset+ECONOMICS_METRICS_SIZE]
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
	data := cs.sab[offset : offset+ECONOMICS_METRICS_SIZE]
	for i := range data {
		data[i] = 0
	}
}

// DistributePoUWYield splits a job value according to the 5% protocol fee
func (cs *CreditSupervisor) DistributePoUWYield(workerId, referrerId string, closeIds []string, jobValue uint64) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

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
	offset, exists := cs.accounts[id]
	if !exists {
		// Auto-register for simplicity in Phase 17
		var err error
		offset, err = cs.registerAccountLocked(id)
		if err != nil {
			return
		}
	}

	acc, err := cs.readAccount(offset)
	if err != nil {
		return
	}

	acc.Balance += delta
	if isEarned && delta > 0 {
		acc.EarnedTotal += uint64(delta)
	} else if delta < 0 {
		acc.SpentTotal += uint64(math.Abs(float64(delta)))
	}

	cs.writeAccount(offset, acc)
}

// registerAccountLocked is the internal locked version of RegisterAccount
func (cs *CreditSupervisor) registerAccountLocked(id string) (uint32, error) {
	if len(cs.accounts) >= ECONOMICS_MAX_ACCOUNTS {
		return 0, fmt.Errorf("max accounts reached")
	}

	index := uint32(len(cs.accounts))
	offset := cs.baseOffset + OFFSET_ECONOMICS_ACCOUNTS + (index * ECONOMICS_ACCOUNT_SIZE)
	cs.accounts[id] = offset

	acc := &foundation.CreditAccount{
		ReputationScore: 0.5,
		DeviceCount:     1,
		UptimeScore:     1.0,
	}
	return offset, cs.writeAccount(offset, acc)
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
