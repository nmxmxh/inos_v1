package mesh_test

import (
	"sync"
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSharedStorage_ParallelRetrieval simulates fetching a large file (e.g. 50MB)
// sharded into 1MB chunks across multiple peers.
func TestSharedStorage_ParallelRetrieval(t *testing.T) {
	// 1. Setup Economics for the flow
	el := mesh.NewEconomicLedger()
	requester := "did:inos:requester"
	el.RegisterAccount(requester, 10000)

	// 2. Define Storage job parameters
	const totalSize = 50 * 1024 * 1024 // 50MB
	const chunkSize = 1024 * 1024      // 1MB
	const shardCount = totalSize / chunkSize

	// Expected cost for storage retrieval
	cost := mesh.CalculateDelegationCost("storage_fetch", uint64(totalSize), 50)

	// Create shared escrow for the whole file
	escrowID := "escrow-shared-storage-1"
	_, err := el.CreateSharedEscrow(escrowID, requester, cost, int(shardCount), time.Hour)

	require.NoError(t, err)

	// 3. Simulate Parallel Retrieval
	var wg sync.WaitGroup
	var mu sync.Mutex
	collectedData := make([][]byte, shardCount)

	// We'll simulate 5 unique peers providing the data (10 shards each)
	peers := []string{"peer-a", "peer-b", "peer-c", "peer-d", "peer-e"}
	for _, p := range peers {
		el.RegisterAccount(p, 0)
	}

	for i := 0; i < shardCount; i++ {
		wg.Add(1)
		go func(shardIdx int) {
			defer wg.Done()

			// Simulate network latency
			time.Sleep(time.Millisecond * 5)

			// Select a peer
			peerID := peers[shardIdx%len(peers)]

			// 4. Register contribution for this shard
			// Shared storage is settled when all shards are verified
			err := el.RegisterWorkerContribution(escrowID, peerID, shardIdx, uint64(chunkSize), true, 12.5)
			if err != nil {
				t.Errorf("Failed to register shard %d: %v", shardIdx, err)
				return
			}

			// Simulate data payload
			shardData := make([]byte, chunkSize)
			shardData[0] = byte(shardIdx)

			mu.Lock()
			collectedData[shardIdx] = shardData
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// 5. Settle the Shared Escrow
	result, err := el.SettleSharedEscrow(escrowID)
	require.NoError(t, err)
	assert.False(t, result.Refunded)
	assert.Equal(t, int(shardCount), result.ShardsVerified)

	// 6. Verify Economic Payouts
	// Total cost / shardCount = credits per shard
	// Payout is 95% of total
	protocolFee := int64(cost) * 50 / 1000
	totalWorkerPool := int64(cost) - protocolFee
	expectedPerPeer := totalWorkerPool / int64(len(peers))

	for _, p := range peers {
		balance := el.GetBalance(p)
		// Small variance allowed due to integer division
		assert.InDelta(t, expectedPerPeer, balance, 1.0, "Peer %s should receive proportional payout", p)
	}

	// 7. Verify Integrity of "Aggregated" file
	assert.Len(t, collectedData, shardCount)
	for i, data := range collectedData {
		assert.Equal(t, byte(i), data[0], "Shard %d has corrupted data alignment", i)
	}
}

// TestSharedStorage_PartialFailure simulates one peer failing verification
func TestSharedStorage_PartialFailure(t *testing.T) {
	el := mesh.NewEconomicLedger()
	requester := "did:inos:alice"
	el.RegisterAccount(requester, 5000)

	escrowID := "escrow-failing-storage"
	_, _ = el.CreateSharedEscrow(escrowID, requester, 1000, 2, time.Hour)

	// Worker 1: Successful
	_ = el.RegisterWorkerContribution(escrowID, "worker-1", 0, 512, true, 10.0)
	// Worker 2: Massive failure (malicious or offline)
	_ = el.RegisterWorkerContribution(escrowID, "worker-2", 1, 512, false, 500.0)

	// Settlement should still occur but worker-2 gets nothing
	result, err := el.SettleSharedEscrow(escrowID)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ShardsVerified)

	// worker-1 should get its proportional 50% of the 950 pool
	// (512 / (512+0)) * 950 = 950?
	// Wait, the logic used in SettleSharedEscrow uses totalVerifiedSize.
	// So worker-1 is (512 / 512) * pool = 100% of the SUCCESSFUL pool.
	assert.Equal(t, int64(950), el.GetBalance("worker-1"))
	assert.Equal(t, int64(0), el.GetBalance("worker-2"))
}
