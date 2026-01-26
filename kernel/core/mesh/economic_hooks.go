package mesh

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
)

// EscrowStatus represents the state of an escrow
type EscrowStatus int

const (
	EscrowPending EscrowStatus = iota
	EscrowLocked
	EscrowReleased
	EscrowRefunded
	EscrowExpired
)

func (es EscrowStatus) String() string {
	switch es {
	case EscrowPending:
		return "pending"
	case EscrowLocked:
		return "locked"
	case EscrowReleased:
		return "released"
	case EscrowRefunded:
		return "refunded"
	case EscrowExpired:
		return "expired"
	default:
		return "unknown"
	}
}

// DelegationEscrow represents locked credits for a delegation job
type DelegationEscrow struct {
	ID          string       // Unique escrow identifier
	RequesterID string       // DID of the requester
	ProviderID  string       // DID of the provider (set when matched)
	Amount      uint64       // Credits locked
	Status      EscrowStatus // Current status
	CreatedAt   time.Time
	ExpiresAt   time.Time
	SettledAt   time.Time
	JobID       string // Associated job ID
}

// EconomicLedger manages credit escrow and settlement for delegated jobs
type EconomicLedger struct {
	escrows  map[string]*DelegationEscrow
	balances map[string]int64 // Account balances (DID -> credits)
	mu       sync.RWMutex

	// Authority for grounded state (optional)
	vault foundation.EconomicVault

	// Statistics
	totalEscrowed    uint64
	totalSettled     uint64
	totalRefunded    uint64
	settlementsCount uint64
}

// SealedCreditsVault adds pending credit support for escrow settlement.
type SealedCreditsVault interface {
	GetAvailableBalance(did string) (int64, error)
	ReservePending(did string, amount uint64) error
	ReleasePending(did string, amount uint64) error
	RefundPending(did string, amount uint64) error
}

// NewEconomicLedger creates a new economic ledger for delegation
func NewEconomicLedger() *EconomicLedger {
	return &EconomicLedger{
		escrows:  make(map[string]*DelegationEscrow),
		balances: make(map[string]int64),
	}
}

// SetVault sets the grounded economic authority
func (el *EconomicLedger) SetVault(vault foundation.EconomicVault) {
	el.mu.Lock()
	el.vault = vault
	balances := make(map[string]int64, len(el.balances))
	for did, balance := range el.balances {
		balances[did] = balance
	}
	el.mu.Unlock()

	if vault == nil {
		return
	}

	for did, balance := range balances {
		if balance <= 0 {
			continue
		}
		_ = vault.GrantBonus(did, balance)
	}
}

// RegisterAccount initializes an account with optional starting balance
func (el *EconomicLedger) RegisterAccount(did string, initialBalance int64) {
	el.mu.Lock()
	el.balances[did] = initialBalance
	v := el.vault
	el.mu.Unlock()

	if v != nil && initialBalance > 0 {
		v.GrantBonus(did, initialBalance)
	}
}

// EnsureAccount registers an account if it does not already exist.
func (el *EconomicLedger) EnsureAccount(did string, initialBalance int64) {
	el.mu.Lock()
	_, exists := el.balances[did]
	if !exists {
		el.balances[did] = initialBalance
	}
	v := el.vault
	el.mu.Unlock()

	if !exists && v != nil && initialBalance > 0 {
		v.GrantBonus(did, initialBalance)
	}
}

// GrantEarlyAdopterBonus grants a one-time bonus to a new user
func (el *EconomicLedger) GrantEarlyAdopterBonus(did string, bonus int64) {
	el.mu.Lock()
	el.balances[did] += bonus
	v := el.vault
	el.mu.Unlock()

	if v != nil {
		v.GrantBonus(did, bonus)
	}
}

// GetBalance returns the current balance for an account
func (el *EconomicLedger) GetBalance(did string) int64 {
	el.mu.RLock()
	v := el.vault
	balance := el.balances[did]
	el.mu.RUnlock()

	if v != nil {
		if vb, err := v.GetBalance(did); err == nil {
			return vb
		}
	}
	return balance
}

// CreateEscrow locks credits for a pending delegation job
func (el *EconomicLedger) CreateEscrow(
	escrowID string,
	requesterID string,
	amount uint64,
	ttl time.Duration,
	jobID string,
) (*DelegationEscrow, error) {
	el.mu.Lock()
	defer el.mu.Unlock()

	// Check if requester has sufficient balance
	balance := el.balances[requesterID]
	if sealed, ok := el.vault.(SealedCreditsVault); ok && el.vault != nil {
		if available, err := sealed.GetAvailableBalance(requesterID); err == nil {
			balance = available
		}
	}
	if balance < int64(amount) {
		return nil, fmt.Errorf("insufficient balance: have %d, need %d", balance, amount)
	}

	// Check for duplicate escrow
	if _, exists := el.escrows[escrowID]; exists {
		return nil, errors.New("escrow ID already exists")
	}

	// Lock the credits
	if sealed, ok := el.vault.(SealedCreditsVault); ok && el.vault != nil {
		if err := sealed.ReservePending(requesterID, amount); err != nil {
			return nil, err
		}
	} else {
		el.balances[requesterID] -= int64(amount)
	}

	escrow := &DelegationEscrow{
		ID:          escrowID,
		RequesterID: requesterID,
		Amount:      amount,
		Status:      EscrowLocked,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(ttl),
		JobID:       jobID,
	}

	el.escrows[escrowID] = escrow
	el.totalEscrowed += amount

	return escrow, nil
}

// AssignProvider updates the escrow with the matched provider
func (el *EconomicLedger) AssignProvider(escrowID, providerID string) error {
	el.mu.Lock()
	defer el.mu.Unlock()

	escrow, exists := el.escrows[escrowID]
	if !exists {
		return errors.New("escrow not found")
	}

	if escrow.Status != EscrowLocked {
		return fmt.Errorf("invalid escrow status: %s", escrow.Status)
	}

	escrow.ProviderID = providerID
	return nil
}

// ReleaseToProvider settles the escrow to the provider (success case)
func (el *EconomicLedger) ReleaseToProvider(escrowID string, verified bool) error {
	el.mu.Lock()
	defer el.mu.Unlock()

	escrow, exists := el.escrows[escrowID]
	if !exists {
		return errors.New("escrow not found")
	}

	if escrow.Status != EscrowLocked {
		return fmt.Errorf("invalid escrow status: %s", escrow.Status)
	}

	if escrow.ProviderID == "" {
		return errors.New("no provider assigned")
	}

	if !verified {
		return errors.New("verification failed, cannot release")
	}

	// Transfer credits to provider
	if sealed, ok := el.vault.(SealedCreditsVault); ok && el.vault != nil {
		if err := sealed.ReleasePending(escrow.ProviderID, escrow.Amount); err != nil {
			return err
		}
	} else {
		el.balances[escrow.ProviderID] += int64(escrow.Amount)
	}
	escrow.Status = EscrowReleased
	escrow.SettledAt = time.Now()

	el.totalSettled += escrow.Amount
	el.settlementsCount++

	return nil
}

// RefundToRequester returns escrowed credits to the requester (failure/timeout)
func (el *EconomicLedger) RefundToRequester(escrowID string) error {
	el.mu.Lock()
	defer el.mu.Unlock()

	escrow, exists := el.escrows[escrowID]
	if !exists {
		return errors.New("escrow not found")
	}

	if escrow.Status != EscrowLocked {
		return fmt.Errorf("invalid escrow status: %s", escrow.Status)
	}

	// Return credits to requester
	if sealed, ok := el.vault.(SealedCreditsVault); ok && el.vault != nil {
		if err := sealed.RefundPending(escrow.RequesterID, escrow.Amount); err != nil {
			return err
		}
	} else {
		el.balances[escrow.RequesterID] += int64(escrow.Amount)
	}
	escrow.Status = EscrowRefunded
	escrow.SettledAt = time.Now()

	el.totalRefunded += escrow.Amount

	return nil
}

// ExpireStaleEscrows marks expired escrows and refunds them
func (el *EconomicLedger) ExpireStaleEscrows() int {
	el.mu.Lock()
	defer el.mu.Unlock()

	now := time.Now()
	expired := 0

	for id, escrow := range el.escrows {
		if escrow.Status == EscrowLocked && now.After(escrow.ExpiresAt) {
			// Refund automatically
			if sealed, ok := el.vault.(SealedCreditsVault); ok && el.vault != nil {
				_ = sealed.RefundPending(escrow.RequesterID, escrow.Amount)
			} else {
				el.balances[escrow.RequesterID] += int64(escrow.Amount)
			}
			escrow.Status = EscrowExpired
			escrow.SettledAt = now
			el.totalRefunded += escrow.Amount
			expired++
			_ = id // Track for logging
		}
	}

	return expired
}

// GetEscrow returns the escrow by ID
func (el *EconomicLedger) GetEscrow(escrowID string) (*DelegationEscrow, bool) {
	el.mu.RLock()
	defer el.mu.RUnlock()
	escrow, exists := el.escrows[escrowID]
	return escrow, exists
}

// GetStats returns ledger statistics
func (el *EconomicLedger) GetStats() map[string]interface{} {
	el.mu.RLock()
	defer el.mu.RUnlock()

	return map[string]interface{}{
		"total_escrowed":    el.totalEscrowed,
		"total_settled":     el.totalSettled,
		"total_refunded":    el.totalRefunded,
		"settlements_count": el.settlementsCount,
		"active_escrows":    len(el.escrows),
		"accounts":          len(el.balances),
	}
}

// CalculateDelegationCost estimates the cost for a delegation operation
func CalculateDelegationCost(operation string, dataSizeBytes uint64, priority int) uint64 {
	// Base cost per operation type (microcredits)
	baseCost := map[string]uint64{
		"hash":     10,
		"compress": 50,
		"encrypt":  100,
		"decrypt":  100,
		"custom":   200,
	}

	cost, exists := baseCost[operation]
	if !exists {
		cost = baseCost["custom"]
	}

	// Scale by data size (1 credit per MB)
	sizeMB := dataSizeBytes / (1024 * 1024)
	if sizeMB == 0 {
		sizeMB = 1
	}
	cost *= sizeMB

	// Priority multiplier
	if priority > 200 {
		cost = cost * 2 // High priority = 2x cost
	} else if priority > 100 {
		cost = cost * 150 / 100 // Medium priority = 1.5x
	}

	return cost
}

// SettlementResult contains the outcome of a delegation settlement
type SettlementResult struct {
	EscrowID   string
	ProviderID string
	Amount     uint64
	Success    bool
	Verified   bool
	LatencyMs  float64
	SettledAt  time.Time
	Error      string
}

// SettleDelegation performs full settlement after delegation completes
func (el *EconomicLedger) SettleDelegation(
	escrowID string,
	verified bool,
	latencyMs float64,
) (*SettlementResult, error) {
	escrow, exists := el.GetEscrow(escrowID)
	if !exists {
		return nil, errors.New("escrow not found")
	}

	result := &SettlementResult{
		EscrowID:   escrowID,
		ProviderID: escrow.ProviderID,
		Amount:     escrow.Amount,
		Verified:   verified,
		LatencyMs:  latencyMs,
	}

	if verified {
		err := el.ReleaseToProvider(escrowID, true)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			return result, err
		}
		result.Success = true
	} else {
		err := el.RefundToRequester(escrowID)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			return result, err
		}
		result.Success = true
	}

	result.SettledAt = time.Now()
	return result, nil
}
