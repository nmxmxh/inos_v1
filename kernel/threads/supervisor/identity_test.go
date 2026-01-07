package supervisor_test

import (
	"testing"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentitySupervisor_Basic(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)
	baseOffset := uint32(100)
	is := supervisor.NewIdentitySupervisor(unsafe.Pointer(&sab[0]), sabSize, baseOffset)
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
	sab := make([]byte, sabSize)
	baseOffset := uint32(100)
	is := supervisor.NewIdentitySupervisor(unsafe.Pointer(&sab[0]), sabSize, baseOffset)

	// Register treasury and nmxmxh
	is.RegisterDID("did:inos:treasury", nil)
	is.RegisterDID("did:inos:nmxmxh", nil)

	offset, err := is.ResolveDID("did:inos:treasury")
	assert.NoError(t, err)
	assert.Greater(t, offset, baseOffset)
}
