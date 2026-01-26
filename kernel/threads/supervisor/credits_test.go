package supervisor_test

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreditSupervisor_Basic(t *testing.T) {
	sabSize := uint32(1024 * 1024) // 1MB for test
	sab := make([]byte, sabSize)
	baseOffset := uint32(1024)

	cs := supervisor.NewCreditSupervisor(unsafe.Pointer(&sab[0]), sabSize, baseOffset)
	require.NotNil(t, cs)

	// 1. Register Account
	id := "user1"
	offset, err := cs.RegisterAccount(id)
	assert.NoError(t, err)
	assert.Greater(t, offset, baseOffset)

	// Verify account state in SAB
	data := sab[offset : offset+128]
	balance := int64(binary.LittleEndian.Uint64(data[0:8]))
	assert.Equal(t, int64(0), balance)

	reputation := math.Float32frombits(binary.LittleEndian.Uint32(data[32:36]))
	assert.Equal(t, float32(0.5), reputation)
}

func TestCreditSupervisor_OnEpoch(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)
	baseOffset := uint32(1024)
	cs := supervisor.NewCreditSupervisor(unsafe.Pointer(&sab[0]), sabSize, baseOffset)

	// 1. Register Account
	id := "user1"
	offset, _ := cs.RegisterAccount(id)

	// 2. Setup Metrics in SAB
	// Metrics are at baseOffset + OFFSET_ECONOMICS_METRICS
	metricsOffset := baseOffset + 8256 // 64 (Metadata) + (64 * 128 (Accounts))

	// Write some usage metrics
	binary.LittleEndian.PutUint64(sab[metricsOffset:metricsOffset+8], 1000)   // ComputeCyclesUsed
	binary.LittleEndian.PutUint64(sab[metricsOffset+8:metricsOffset+16], 500) // BytesServed
	binary.LittleEndian.PutUint64(sab[metricsOffset+36:metricsOffset+44], 10) // SyscallCount

	// 3. Settle Epoch
	err := cs.OnEpoch(1)
	assert.NoError(t, err)

	// 4. Verify Account Update
	// ComputeRate=1.0, BandwidthRate=0.001, SyscallCost=0.01
	// Earned: 1000*1.0 + 500*0.001 = 1000.5
	// Spent: 10*0.01 = 0.1
	// Delta: 1000.4 -> rounded to 1000 in balance update if int64 cast

	// Re-read account
	data := sab[offset : offset+128]
	newBalance := int64(binary.LittleEndian.Uint64(data[0:8]))
	assert.Greater(t, newBalance, int64(0))

	// 5. Verify Metrics Reset
	assert.Equal(t, uint64(0), binary.LittleEndian.Uint64(sab[metricsOffset:metricsOffset+8]))
}

func TestCreditSupervisor_PendingCredits(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)
	cs := supervisor.NewCreditSupervisor(unsafe.Pointer(&sab[0]), sabSize, 0)

	id := "user1"
	_, err := cs.RegisterAccount(id)
	require.NoError(t, err)

	err = cs.GrantBonus(id, 100)
	require.NoError(t, err)

	acc, err := cs.GetAccount(id)
	require.NoError(t, err)
	assert.Equal(t, int64(0), acc.Balance)
	assert.Equal(t, int64(100), acc.PendingBalance)

	cs.FinalizePending(42)
	acc, err = cs.GetAccount(id)
	require.NoError(t, err)
	assert.Equal(t, int64(100), acc.Balance)
	assert.Equal(t, int64(0), acc.PendingBalance)
}

func TestCreditSupervisor_Bounds(t *testing.T) {
	sab := make([]byte, 100) // Too small
	cs := supervisor.NewCreditSupervisor(unsafe.Pointer(&sab[0]), 100, 0)

	_, err := cs.RegisterAccount("test")
	assert.Error(t, err)
}

func TestCreditSupervisor_MaxAccounts(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)
	cs := supervisor.NewCreditSupervisor(unsafe.Pointer(&sab[0]), sabSize, 0)

	for i := 0; i < 64; i++ {
		_, err := cs.RegisterAccount(fmt.Sprintf("user%d", i))
		require.NoError(t, err)
	}

	_, err := cs.RegisterAccount("overflow")
	assert.Error(t, err)
	assert.Equal(t, "max accounts reached", err.Error())
}

func TestCreditSupervisor_Multipliers(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)
	cs := supervisor.NewCreditSupervisor(unsafe.Pointer(&sab[0]), sabSize, 0)

	// Use writeAccount manually to set higher device count
	offset, _ := cs.RegisterAccount("user1")

	// Craft a higher-tier account
	data := sab[offset : offset+128]
	binary.LittleEndian.PutUint16(data[36:38], 10) // 10 devices -> 1.01 multiplier

	// Metrics
	metricsOffset := uint32(8256)
	binary.LittleEndian.PutUint64(sab[metricsOffset:metricsOffset+8], 1000000) // 1M cycles

	err := cs.OnEpoch(2)
	assert.NoError(t, err)

	newBalance := int64(binary.LittleEndian.Uint64(data[0:8]))
	// Without multiplier: 1,000,000
	// With 10 devices (1.01x): 1,010,000
	assert.Equal(t, int64(1010000), newBalance)
}

func TestCreditSupervisor_ProtocolFee(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)
	cs := supervisor.NewCreditSupervisor(unsafe.Pointer(&sab[0]), sabSize, 0)

	// Register accounts
	worker := "did:inos:worker"
	treasury := "did:inos:treasury"
	creator := "did:inos:nmxmxh"
	referrer := "did:inos:referrer"

	cs.RegisterAccount(worker)
	cs.RegisterAccount(treasury)
	cs.RegisterAccount(creator)
	cs.RegisterAccount(referrer)

	// Distribute 1000 credits
	err := cs.DistributePoUWYield(worker, referrer, nil, 1000)
	assert.NoError(t, err)
	cs.FinalizePending(1)

	// Verify Splits:
	// Worker: 95% = 950
	// Treasury: 3.5% = 35
	// Creator: 0.5% = 5
	// Referrer: 0.5% = 5
	// CloseIDs: 0.5% (but nil in this test) -> goes to Treasury?
	// Actually current logic: if no close IDs, remains in balance or goes to common pool.
	// Let's check logic: s.DistributePoUWYield splits it.

	verifyBalance := func(did string, expected int64) {
		acc, _ := cs.GetAccount(did)
		assert.Equal(t, expected, acc.Balance, "Balance mismatch for "+did)
	}

	verifyBalance(worker, 950)
	verifyBalance(treasury, 35+5) // 35 (treasury) + 5 (unallocated close IDs)
	verifyBalance(creator, 5)
	verifyBalance(referrer, 5)
}
