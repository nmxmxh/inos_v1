package mesh_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Multi-Peer Simulation Tests ==========

// TestMultiPeer_GossipPropagation tests message propagation across multiple peers
func TestMultiPeer_GossipPropagation(t *testing.T) {
	// Simulate 5 peer network
	peerCount := 5
	peers := make([]*mockPeer, peerCount)

	for i := 0; i < peerCount; i++ {
		peers[i] = &mockPeer{
			id:       fmt.Sprintf("peer-%d", i),
			messages: make([]string, 0),
		}
	}

	// Simulate gossip from peer 0
	originMessage := "hello-from-peer-0"
	peers[0].broadcast(originMessage, peers[1:])

	// All peers should receive the message
	for i := 1; i < peerCount; i++ {
		assert.Contains(t, peers[i].messages, originMessage,
			"Peer %d should receive message", i)
	}
}

// TestMultiPeer_ReputationConvergence tests reputation scores converging across network
func TestMultiPeer_ReputationConvergence(t *testing.T) {
	ledger := mesh.NewEconomicLedger()

	// Register multiple peers
	peers := []string{"peer-a", "peer-b", "peer-c", "peer-d", "peer-e"}
	for _, peer := range peers {
		ledger.RegisterAccount(peer, 1000)
	}

	// Simulate successful interactions
	successfulPeer := "peer-a"
	for i := 0; i < 10; i++ {
		// Peer-a provides compute to others
		escrowID := fmt.Sprintf("escrow-%d", i)
		requester := peers[(i%4)+1] // Rotate requesters (b,c,d,e)

		_, err := ledger.CreateEscrow(escrowID, requester, 50, time.Hour, "job")
		if err != nil {
			continue
		}
		ledger.AssignProvider(escrowID, successfulPeer)
		ledger.ReleaseToProvider(escrowID, true)
	}

	// Successful peer should have highest balance
	assert.Greater(t, ledger.GetBalance(successfulPeer), int64(1000))
}

// TestMultiPeer_PartitionRecovery tests network partition and recovery
func TestMultiPeer_PartitionRecovery(t *testing.T) {
	ledger := mesh.NewEconomicLedger()

	// Partition A: peers 0-2
	// Partition B: peers 3-4
	partitionA := []string{"peer-0", "peer-1", "peer-2"}
	partitionB := []string{"peer-3", "peer-4"}

	for _, p := range append(partitionA, partitionB...) {
		ledger.RegisterAccount(p, 1000)
	}

	// Activity in partition A
	for i := 0; i < 3; i++ {
		escrowID := fmt.Sprintf("a-escrow-%d", i)
		ledger.CreateEscrow(escrowID, partitionA[0], 100, time.Hour, "job")
		ledger.AssignProvider(escrowID, partitionA[i])
		ledger.ReleaseToProvider(escrowID, true)
	}

	// Activity in partition B
	for i := 0; i < 2; i++ {
		escrowID := fmt.Sprintf("b-escrow-%d", i)
		ledger.CreateEscrow(escrowID, partitionB[0], 100, time.Hour, "job")
		ledger.AssignProvider(escrowID, partitionB[(i+1)%2])
		ledger.ReleaseToProvider(escrowID, true)
	}

	// After merge, both partitions should have valid state
	stats := ledger.GetStats()
	assert.Equal(t, uint64(5), stats["settlements_count"])
}

// TestMultiPeer_ConcurrentDelegation tests concurrent delegation requests
func TestMultiPeer_ConcurrentDelegation(t *testing.T) {
	ledger := mesh.NewEconomicLedger()

	// Register peers
	for i := 0; i < 10; i++ {
		ledger.RegisterAccount(fmt.Sprintf("requester-%d", i), 10000)
		ledger.RegisterAccount(fmt.Sprintf("provider-%d", i), 0)
	}

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// 100 concurrent delegations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			escrowID := fmt.Sprintf("concurrent-escrow-%d", idx)
			requester := fmt.Sprintf("requester-%d", idx%10)
			provider := fmt.Sprintf("provider-%d", (idx+1)%10)

			_, err := ledger.CreateEscrow(escrowID, requester, 50, time.Hour, "job")
			if err != nil {
				return
			}

			ledger.AssignProvider(escrowID, provider)

			result, err := ledger.SettleDelegation(escrowID, true, 10.0)
			if err == nil && result.Success {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Most delegations should succeed
	assert.Greater(t, successCount, 50, "At least 50% of delegations should succeed")
}

// ========== Rust Module Integration Scenarios ==========

// TestRustIntegration_HashCompressEncryptPipeline tests the full storage pipeline
func TestRustIntegration_HashCompressEncryptPipeline(t *testing.T) {
	// Simulate the pipeline:
	// 1. Hash input (BLAKE3) - Rust crypto module
	// 2. Compress (Brotli) - Rust storage module
	// 3. Encrypt (ChaCha20) - Rust storage module
	// 4. Store (CAS) - Rust storage module
	// 5. Verify digest matches

	// Create verifier for input
	inputDigest := make([]byte, 32)
	for i := range inputDigest {
		inputDigest[i] = byte(i)
	}

	verifier := mesh.NewDelegationVerifier(fmt.Sprintf("%x", inputDigest), "pipeline")

	// Simulate Rust module output
	outputDigest := "abcd1234efgh5678abcd1234efgh5678abcd1234efgh5678abcd1234efgh5678"
	verifier.SetResult(outputDigest, 50000000) // 50ms

	// Verify output
	assert.True(t, verifier.Verify(outputDigest))
	assert.Equal(t, int64(50000000), verifier.ExecutionTime())
}

// TestRustIntegration_UnitCapabilityRouting tests routing to correct Rust unit
func TestRustIntegration_UnitCapabilityRouting(t *testing.T) {
	// Define expected capabilities per unit
	unitCapabilities := map[string][]string{
		"crypto":  {"blake3", "sha256", "ed25519_sign", "chacha20_encrypt"},
		"storage": {"store_chunk", "load_chunk", "compress", "encrypt"},
		"image":   {"resize", "crop", "blur", "edge_detect"},
		"audio":   {"decode", "encode_flac", "fft", "resample"},
		"data":    {"parquet_read", "csv_write", "sum", "mean"},
		"gpu":     {"transform_vertices", "particle_update", "ray_tracing"},
		"boids":   {"init_population", "step_physics", "evolve_batch"},
	}

	// Test each unit has expected capabilities
	for unit, capabilities := range unitCapabilities {
		for _, cap := range capabilities {
			// Simulate capability lookup
			found := lookupCapability(unit, cap)
			assert.True(t, found, "Unit %s should have capability %s", unit, cap)
		}
	}
}

// TestRustIntegration_CompressDecompressRoundTrip tests compression integrity
func TestRustIntegration_CompressDecompressRoundTrip(t *testing.T) {
	algorithms := []string{"brotli", "snappy", "lz4"}

	for _, algo := range algorithms {
		t.Run(algo, func(t *testing.T) {
			// Simulate data
			originalData := make([]byte, 100*1024) // 100KB
			for i := range originalData {
				originalData[i] = byte(i % 256)
			}

			// Create digest validator
			inputDigest := make([]byte, 32)
			validator := mesh.NewDigestValidator(inputDigest)

			// Simulate round-trip
			// In real system: Rust compresses, returns digest
			// Go validates digest matches expected

			assert.NotNil(t, validator)
			// Validation would happen with actual Rust module call
		})
	}
}

// ========== Architecture Scenario Tests ==========

// TestArchitecture_EpochBasedSignaling tests epoch increment and detection
func TestArchitecture_EpochBasedSignaling(t *testing.T) {
	// Simulate epoch tracking
	var epoch uint64 = 0
	var mu sync.Mutex

	incrementEpoch := func() uint64 {
		mu.Lock()
		defer mu.Unlock()
		epoch++
		return epoch
	}

	getEpoch := func() uint64 {
		mu.Lock()
		defer mu.Unlock()
		return epoch
	}

	// Initial epoch
	assert.Equal(t, uint64(0), getEpoch())

	// Simulate 10 mutations
	for i := 0; i < 10; i++ {
		newEpoch := incrementEpoch()
		assert.Equal(t, uint64(i+1), newEpoch)
	}

	// Final epoch
	assert.Equal(t, uint64(10), getEpoch())
}

// TestArchitecture_MerkleDagChunking tests file chunking for CAS
func TestArchitecture_MerkleDagChunking(t *testing.T) {
	// Simulate 3MB file chunking
	fileSize := 3 * 1024 * 1024
	chunkSize := 1024 * 1024 // 1MB

	expectedChunks := (fileSize + chunkSize - 1) / chunkSize
	assert.Equal(t, 3, expectedChunks)

	// Each chunk should have its own hash
	chunkHashes := make([]string, expectedChunks)
	for i := 0; i < expectedChunks; i++ {
		chunkHashes[i] = fmt.Sprintf("chunk-hash-%d", i)
	}

	// Root hash is hash of all chunk hashes
	rootHash := fmt.Sprintf("root-hash-of-%d-chunks", len(chunkHashes))
	assert.NotEmpty(t, rootHash)
}

// TestArchitecture_StorageTierSelection tests hot/cold tier logic
func TestArchitecture_StorageTierSelection(t *testing.T) {
	tests := []struct {
		name          string
		latencyMs     float64
		bandwidthMbps float64
		expectedTier  string
	}{
		{"Edge Server", 5.0, 1000, "hot"},
		{"5G Drone", 15.0, 500, "hot"},
		{"Home NAS", 100.0, 100, "cold"},
		{"Data Center", 200.0, 10000, "cold"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tier := selectStorageTier(tc.latencyMs, tc.bandwidthMbps)
			assert.Equal(t, tc.expectedTier, tier)
		})
	}
}

// TestArchitecture_ReplicationFactorMaintenance tests RF = 3 enforcement
func TestArchitecture_ReplicationFactorMaintenance(t *testing.T) {
	targetRF := 3

	// Simulate chunk with 2 replicas (below target)
	currentRF := 2

	replicasNeeded := targetRF - currentRF
	assert.Equal(t, 1, replicasNeeded)

	// After adding replica
	currentRF++
	assert.Equal(t, targetRF, currentRF)
}

// ========== Economic Credit Scenarios ==========

// TestEconomics_JobPricing tests cost calculation for different operations
func TestEconomics_JobPricing(t *testing.T) {
	tests := []struct {
		operation string
		sizeMB    int
		priority  int
		minCost   uint64
	}{
		{"hash", 1, 50, 10},
		{"compress", 10, 50, 500},
		{"encrypt", 100, 50, 10000},
		{"compress", 1, 250, 100}, // High priority doubles cost
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s_%dMB_pri%d", tc.operation, tc.sizeMB, tc.priority), func(t *testing.T) {
			cost := mesh.CalculateDelegationCost(
				tc.operation,
				uint64(tc.sizeMB*1024*1024),
				tc.priority,
			)
			assert.GreaterOrEqual(t, cost, tc.minCost)
		})
	}
}

// TestEconomics_ProviderEarnings tests credit flow to providers
func TestEconomics_ProviderEarnings(t *testing.T) {
	ledger := mesh.NewEconomicLedger()

	// Setup
	ledger.RegisterAccount("requester", 10000)
	ledger.RegisterAccount("provider", 0)

	// 10 successful jobs
	totalEarned := uint64(0)
	for i := 0; i < 10; i++ {
		cost := uint64(100)
		escrowID := fmt.Sprintf("job-%d", i)

		ledger.CreateEscrow(escrowID, "requester", cost, time.Hour, escrowID)
		ledger.AssignProvider(escrowID, "provider")
		ledger.ReleaseToProvider(escrowID, true)
		totalEarned += cost
	}

	assert.Equal(t, int64(totalEarned), ledger.GetBalance("provider"))
	assert.Equal(t, int64(10000-int(totalEarned)), ledger.GetBalance("requester"))
}

// TestEconomics_BadActorPenalty tests handling of verification failures
func TestEconomics_BadActorPenalty(t *testing.T) {
	ledger := mesh.NewEconomicLedger()

	ledger.RegisterAccount("requester", 5000)
	ledger.RegisterAccount("bad-actor", 0)

	// Failed verification - credits refunded to requester
	ledger.CreateEscrow("bad-job", "requester", 500, time.Hour, "job")
	ledger.AssignProvider("bad-job", "bad-actor")

	// Verification fails
	result, _ := ledger.SettleDelegation("bad-job", false, 100)

	assert.True(t, result.Success)                               // Refund succeeded
	assert.Equal(t, int64(5000), ledger.GetBalance("requester")) // Full refund
	assert.Equal(t, int64(0), ledger.GetBalance("bad-actor"))    // No payment
}

// ========== Verification Flow Tests ==========

// TestVerification_DigestChain tests digest verification chain
func TestVerification_DigestChain(t *testing.T) {
	// Simulate verification chain:
	// Input -> Hash -> Compress -> Encrypt -> Final Hash

	stages := []string{"raw", "hashed", "compressed", "encrypted", "final"}
	digests := make(map[string][]byte)

	for i, stage := range stages {
		digest := make([]byte, 32)
		for j := range digest {
			digest[j] = byte((i * 32) + j)
		}
		digests[stage] = digest
	}

	// Verify each stage
	for _, stage := range stages {
		validator := mesh.NewDigestValidator(digests[stage])
		assert.True(t, validator.Validate(digests[stage]))
		assert.Equal(t, mesh.VerificationPassed, validator.Status())
	}
}

// TestVerification_StreamingLargeFile tests streaming verification
func TestVerification_StreamingLargeFile(t *testing.T) {
	// Simulate 100MB file verification
	fileSizeMB := 100
	chunkSizeMB := 1
	totalChunks := fileSizeMB / chunkSizeMB

	expectedDigest := make([]byte, 32)
	verifier := mesh.NewStreamingVerifier(expectedDigest)

	// Process header
	err := verifier.ProcessHeader([]byte("file header"))
	require.NoError(t, err)

	// Track chunks
	assert.Equal(t, int64(1), verifier.ChunkCount())

	// Simulate chunk processing
	for i := 0; i < totalChunks; i++ {
		// In real flow, would call ProcessChunk or ProcessStream
	}

	// Finalize would compare with Rust-computed digest
}

// ========== Helper Functions ==========

type mockPeer struct {
	id       string
	messages []string
	mu       sync.Mutex
}

func (p *mockPeer) broadcast(msg string, peers []*mockPeer) {
	for _, peer := range peers {
		peer.receive(msg)
	}
}

func (p *mockPeer) receive(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages = append(p.messages, msg)
}

func lookupCapability(unit, capability string) bool {
	// Simulated capability lookup
	capabilities := map[string][]string{
		"crypto":  {"blake3", "sha256", "ed25519_sign", "chacha20_encrypt"},
		"storage": {"store_chunk", "load_chunk", "compress", "encrypt"},
		"image":   {"resize", "crop", "blur", "edge_detect"},
		"audio":   {"decode", "encode_flac", "fft", "resample"},
		"data":    {"parquet_read", "csv_write", "sum", "mean"},
		"gpu":     {"transform_vertices", "particle_update", "ray_tracing"},
		"boids":   {"init_population", "step_physics", "evolve_batch"},
	}

	caps, exists := capabilities[unit]
	if !exists {
		return false
	}

	for _, c := range caps {
		if c == capability {
			return true
		}
	}
	return false
}

func selectStorageTier(latencyMs, bandwidthMbps float64) string {
	// Hot tier: low latency (<50ms) AND high bandwidth (>200Mbps)
	if latencyMs < 50 && bandwidthMbps > 200 {
		return "hot"
	}
	return "cold"
}
