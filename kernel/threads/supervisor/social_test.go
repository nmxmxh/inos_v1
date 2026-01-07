package supervisor_test

import (
	"testing"
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSocialGraphSupervisor_Basic(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)
	baseOffset := uint32(100)
	sgs := supervisor.NewSocialGraphSupervisor(unsafe.Pointer(&sab[0]), sabSize, baseOffset)
	require.NotNil(t, sgs)

	// 1. Register Social Entry
	user := "did:inos:user"
	referrer := "did:inos:referrer"
	offset, err := sgs.RegisterSocialEntry(user, referrer)
	assert.NoError(t, err)
	assert.Greater(t, offset, baseOffset)

	// 2. Get Referrer
	ref, err := sgs.GetReferrer(user)
	assert.NoError(t, err)
	assert.Equal(t, referrer, ref)

	// 3. Default Referrer for new user
	ref, err = sgs.GetReferrer("nonexistent")
	assert.NoError(t, err)
	assert.Equal(t, "did:inos:nmxmxh", ref)
}

func TestSocialGraphSupervisor_CloseIdentities(t *testing.T) {
	sabSize := uint32(1024 * 1024)
	sab := make([]byte, sabSize)
	baseOffset := uint32(100)
	sgs := supervisor.NewSocialGraphSupervisor(unsafe.Pointer(&sab[0]), sabSize, baseOffset)

	user := "did:inos:user"
	sgs.RegisterSocialEntry(user, "")

	closeIDs := []string{"did:inos:friend1", "did:inos:friend2"}
	for _, cid := range closeIDs {
		err := sgs.AddCloseIdentity(user, cid)
		assert.NoError(t, err)
	}

	// Verify
	res, err := sgs.GetCloseIdentities(user)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))
	assert.Contains(t, res, "did:inos:friend1")
	assert.Contains(t, res, "did:inos:friend2")
}
