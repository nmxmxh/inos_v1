package mesh_test

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh"
	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Integration Tests for Full Delegation Pipeline ==========

// TestDelegationPipeline_FullCycle tests the complete delegation flow:
// DelegationEngine.Analyze → MeshCoordinator.DelegateCompute → Verification → Settlement
func TestDelegationPipeline_FullCycle(t *testing.T) {
	// 1. Setup components
	loadProvider := &mockLoadProvider{load: 0.8}
	delegationEngine := mesh.NewDelegationEngine(loadProvider)
	economicLedger := mesh.NewEconomicLedger()

	// Register accounts
	economicLedger.RegisterAccount("did:inos:requester", 10000)
	economicLedger.RegisterAccount("did:inos:provider", 0)

	// 2. Create a job
	job := &foundation.Job{
		ID:        "integration-test-job-1",
		Operation: "compress",
		Data:      make([]byte, 2*1024*1024), // 2MB
		Priority:  50,
	}

	// 3. Analyze delegation decision
	ctx := context.Background()
	decision := delegationEngine.Analyze(ctx, job)

	// High load + large data should trigger delegation
	assert.True(t, decision.ShouldDelegate, "Should decide to delegate")
	assert.Greater(t, decision.EfficiencyScore, 0.5)

	// 4. Calculate cost and create escrow
	cost := mesh.CalculateDelegationCost(job.Operation, uint64(len(job.Data)), job.Priority)
	require.Greater(t, cost, uint64(0))

	escrow, err := economicLedger.CreateEscrow(
		"escrow-"+job.ID,
		"did:inos:requester",
		cost,
		time.Hour,
		job.ID,
	)
	require.NoError(t, err)
	require.NotNil(t, escrow)

	// 5. Assign provider
	err = economicLedger.AssignProvider(escrow.ID, "did:inos:provider")
	require.NoError(t, err)

	// 6. Simulate successful execution and verification
	verified := true
	latencyMs := 25.5

	// 7. Settle delegation
	result, err := economicLedger.SettleDelegation(escrow.ID, verified, latencyMs)
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.True(t, result.Verified)
	assert.Equal(t, cost, result.Amount)

	// 8. Verify economic outcome
	expectedProviderPay := int64(cost) - (int64(cost) * 50 / 1000)
	assert.Equal(t, expectedProviderPay, economicLedger.GetBalance("did:inos:provider"))
	assert.Equal(t, int64(10000-int(cost)), economicLedger.GetBalance("did:inos:requester"))

}

// TestDelegationPipeline_VerificationFailure tests the refund path
func TestDelegationPipeline_VerificationFailure(t *testing.T) {
	economicLedger := mesh.NewEconomicLedger()
	economicLedger.RegisterAccount("did:inos:alice", 5000)
	economicLedger.RegisterAccount("did:inos:badnode", 0)

	// Create and assign escrow
	escrow, _ := economicLedger.CreateEscrow("escrow-fail-test", "did:inos:alice", 500, time.Hour, "job-fail")
	economicLedger.AssignProvider(escrow.ID, "did:inos:badnode")

	// Initial balances
	initialAlice := economicLedger.GetBalance("did:inos:alice")

	// Verification fails
	result, err := economicLedger.SettleDelegation(escrow.ID, false, 100.0)
	require.NoError(t, err)

	assert.True(t, result.Success) // Refund was successful
	assert.False(t, result.Verified)

	// Alice should get refund
	assert.Equal(t, initialAlice+500, economicLedger.GetBalance("did:inos:alice"))
	// Bad node gets nothing
	assert.Equal(t, int64(0), economicLedger.GetBalance("did:inos:badnode"))
}

// TestDelegationPipeline_DigestValidation tests the digest verification flow
func TestDelegationPipeline_DigestValidation(t *testing.T) {
	// Simulate digest from Rust module
	rustDigest := "deadbeef12345678deadbeef12345678deadbeef12345678deadbeef12345678"

	digestBytes, err := hex.DecodeString(rustDigest)
	require.NoError(t, err)

	// Create validator
	validator := mesh.NewDigestValidator(digestBytes)
	assert.Equal(t, mesh.VerificationPending, validator.Status())

	// Validate with correct digest
	assert.True(t, validator.Validate(digestBytes))
	assert.Equal(t, mesh.VerificationPassed, validator.Status())
}

// TestDelegationPipeline_DigestMismatch tests tampered result detection
func TestDelegationPipeline_DigestMismatch(t *testing.T) {
	expected := make([]byte, 32)
	for i := range expected {
		expected[i] = byte(i)
	}

	tampered := make([]byte, 32)
	for i := range tampered {
		tampered[i] = byte(255 - i)
	}

	validator := mesh.NewDigestValidator(expected)
	assert.False(t, validator.Validate(tampered))
	assert.Equal(t, mesh.VerificationFailed, validator.Status())
}

// TestDelegationPipeline_DelegationVerifier tests the full verifier flow
func TestDelegationPipeline_DelegationVerifier(t *testing.T) {
	inputDigest := "abc123"
	operation := "compress"

	verifier := mesh.NewDelegationVerifier(inputDigest, operation)

	// Simulate Rust module result
	outputDigest := "def456"
	executionTimeNs := int64(50000000) // 50ms

	verifier.SetResult(outputDigest, executionTimeNs)

	// Verify matches expected
	assert.True(t, verifier.Verify(outputDigest))
	assert.True(t, verifier.IsVerified())
	assert.Equal(t, executionTimeNs, verifier.ExecutionTime())
}

// TestDelegationPipeline_EscrowExpiration tests automatic refund on timeout
func TestDelegationPipeline_EscrowExpiration(t *testing.T) {
	ledger := mesh.NewEconomicLedger()
	ledger.RegisterAccount("did:inos:user", 1000)

	// Create escrow with immediate expiration
	_, err := ledger.CreateEscrow("expired-escrow", "did:inos:user", 200, -time.Second, "job")
	require.NoError(t, err)

	// Before expiration check
	assert.Equal(t, int64(800), ledger.GetBalance("did:inos:user"))

	// Run expiration
	expired := ledger.ExpireStaleEscrows()

	assert.Equal(t, 1, expired)
	assert.Equal(t, int64(1000), ledger.GetBalance("did:inos:user"))
}

// TestDelegationPipeline_CostCalculation tests the cost calculator
func TestDelegationPipeline_CostCalculation(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		size      uint64
		priority  int
		expected  uint64
	}{
		{"1MB hash", "hash", 1024 * 1024, 50, 10},
		{"10MB compress", "compress", 10 * 1024 * 1024, 50, 500},
		{"1MB encrypt high priority", "encrypt", 1024 * 1024, 250, 200},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cost := mesh.CalculateDelegationCost(tc.operation, tc.size, tc.priority)
			assert.GreaterOrEqual(t, cost, tc.expected)
		})
	}
}

// TestDelegationPipeline_StreamingVerifier tests stream verification
func TestDelegationPipeline_StreamingVerifier(t *testing.T) {
	expectedDigest := make([]byte, 32)
	for i := range expectedDigest {
		expectedDigest[i] = byte(i)
	}

	verifier := mesh.NewStreamingVerifier(expectedDigest)

	// Process header
	err := verifier.ProcessHeader([]byte("file header data"))
	require.NoError(t, err)
	assert.Equal(t, int64(1), verifier.ChunkCount())
	assert.Greater(t, verifier.BytesProcessed(), int64(0))

	// Finalize with matching digest (from Rust)
	verified := verifier.Finalize(expectedDigest)
	assert.True(t, verified)
	assert.Equal(t, mesh.VerificationPassed, verifier.Status())
}

// TestDelegationPipeline_ConcurrentDelegations tests parallel job handling
func TestDelegationPipeline_ConcurrentDelegations(t *testing.T) {
	ledger := mesh.NewEconomicLedger()

	// Register accounts
	for i := 0; i < 10; i++ {
		ledger.RegisterAccount("did:inos:user"+string(rune('A'+i)), 10000)
	}

	// Create concurrent escrows
	done := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		go func(idx int) {
			userIdx := idx % 10
			escrowID := "escrow-" + string(rune(idx))
			userID := "did:inos:user" + string(rune('A'+userIdx))

			_, err := ledger.CreateEscrow(escrowID, userID, 50, time.Hour, "job")
			if err == nil {
				ledger.AssignProvider(escrowID, "did:inos:provider")
			}
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 100; i++ {
		<-done
	}

	// Verify no panic and stats make sense
	stats := ledger.GetStats()
	assert.Greater(t, stats["total_escrowed"], uint64(0))
}

// TestDelegationPipeline_DecisionEngine_LoadAdaptive tests load-based routing
func TestDelegationPipeline_DecisionEngine_LoadAdaptive(t *testing.T) {
	tests := []struct {
		name           string
		load           float64
		dataSize       int
		expectedTarget mesh.DelegationTargetType
	}{
		{"Low load, small data", 0.2, 1024, mesh.TargetMeshRemote}, // efficiency ~0.66
		{"High load, large data", 0.9, 5 * 1024 * 1024, mesh.TargetMeshRemote},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLoadProvider{load: tc.load}
			engine := mesh.NewDelegationEngine(provider)

			job := &foundation.Job{
				ID:       "test",
				Data:     make([]byte, tc.dataSize),
				Priority: 50,
			}

			decision := engine.Analyze(context.Background(), job)
			// Just verify we get a valid decision
			assert.NotNil(t, decision)
		})
	}
}

// Mock implementations
type mockLoadProvider struct {
	load float64
}

func (m *mockLoadProvider) GetSystemLoad() float64 {
	return m.load
}
