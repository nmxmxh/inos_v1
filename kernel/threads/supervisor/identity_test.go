package supervisor_test

import (
	"testing"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor/units"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentitySupervisor_Basic(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sabData := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&sabData[0])
	baseOffset := uint32(100)

	bridge := supervisor.NewSABBridge(sabPtr, sabSize, sab.OFFSET_INBOX_BASE, sab.OFFSET_OUTBOX_BASE, sab.IDX_SYSTEM_EPOCH)
	patterns := pattern.NewTieredPatternStorage(sabPtr, sabSize, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(sabPtr, sabSize, 0, 1024)

	is := units.NewIdentitySupervisor(bridge, patterns, knowledge, sabPtr, sabSize, baseOffset, nil)
	require.NotNil(t, is)

	// 1. Register DID
	did := "did:inos:testuser"
	pubKey := []byte{0x01, 0x02, 0x03}
	offset, err := is.RegisterDID(did, pubKey)
	assert.NoError(t, err)
	assert.Greater(t, offset, baseOffset)

	// 2. Resolve DID
	resolved, err := is.ResolveDID(did)
	assert.NoError(t, err)
	assert.Equal(t, offset, resolved)

	// 3. Resolve non-existent DID
	_, err = is.ResolveDID("did:inos:nonexistent")
	assert.Error(t, err)
}

func TestIdentitySupervisor_SystemWallets(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sabData := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&sabData[0])
	baseOffset := uint32(100)

	bridge := supervisor.NewSABBridge(sabPtr, sabSize, sab.OFFSET_INBOX_BASE, sab.OFFSET_OUTBOX_BASE, sab.IDX_SYSTEM_EPOCH)
	patterns := pattern.NewTieredPatternStorage(sabPtr, sabSize, 0, 1024)
	knowledge := intelligence.NewKnowledgeGraph(sabPtr, sabSize, 0, 1024)

	is := units.NewIdentitySupervisor(bridge, patterns, knowledge, sabPtr, sabSize, baseOffset, nil)

	// Register treasury and nmxmxh
	is.RegisterDID("did:inos:treasury", nil)
	is.RegisterDID("did:inos:nmxmxh", nil)

	offset, err := is.ResolveDID("did:inos:treasury")
	assert.NoError(t, err)
	assert.Greater(t, offset, baseOffset)
}
