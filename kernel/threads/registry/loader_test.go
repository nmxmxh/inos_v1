package registry

import (
	"encoding/binary"
	"testing"

	"github.com/nmxmxh/inos_v1/kernel/threads/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleRegistry_LoadFromSAB(t *testing.T) {
	// Create mock SAB with modules
	builder := testutil.NewMockSABBuilder(1024 * 1024)
	builder.AddModule("storage", [3]uint8{1, 0, 0}, nil, nil)
	builder.AddModule("compute", [3]uint8{2, 1, 0}, nil, nil)
	sab := builder.Build()

	mr := NewModuleRegistry(sab)
	err := mr.LoadFromSAB()
	require.NoError(t, err)

	modules := mr.ListModules()
	assert.Len(t, modules, 2)

	storage, err := mr.GetModule("storage")
	require.NoError(t, err)
	assert.Equal(t, uint8(1), storage.Version.Major)
	assert.Equal(t, uint8(0), storage.Version.Minor)

	compute, err := mr.GetModule("compute")
	require.NoError(t, err)
	assert.Equal(t, uint8(2), compute.Version.Major)
	assert.Equal(t, uint8(1), compute.Version.Minor)
}

func TestModuleRegistry_LoadEmpty(t *testing.T) {
	builder := testutil.NewMockSABBuilder(1024 * 1024)
	// No modules added
	sab := builder.Build()

	mr := NewModuleRegistry(sab)
	err := mr.LoadFromSAB()
	require.NoError(t, err)

	assert.Empty(t, mr.ListModules())
}

func TestModuleRegistry_DependencyResolution(t *testing.T) {
	builder := testutil.NewMockSABBuilder(1024 * 1024)
	// Chain: C -> B -> A
	builder.AddModule("mod_a", [3]uint8{1, 0, 0}, nil, nil)
	builder.AddModule("mod_b", [3]uint8{1, 0, 0}, nil, []string{"mod_a"})
	builder.AddModule("mod_c", [3]uint8{1, 0, 0}, nil, []string{"mod_b"})
	sab := builder.Build()

	mr := NewModuleRegistry(sab)
	err := mr.LoadFromSAB()
	require.NoError(t, err)

	order, err := mr.GetDependencyOrder()
	require.NoError(t, err)

	// Expected Order: mod_a, mod_b, mod_c
	// Note: Verify indices
	idxA := indexOf(order, "mod_a")
	idxB := indexOf(order, "mod_b")
	idxC := indexOf(order, "mod_c")

	assert.True(t, idxA < idxB, "mod_a should come before mod_b")
	assert.True(t, idxB < idxC, "mod_b should come before mod_c")
}

func TestModuleRegistry_CircularDependencyDetection(t *testing.T) {
	builder := testutil.NewMockSABBuilder(1024 * 1024)
	// Ring: A -> B -> A
	builder.AddModule("circular_a", [3]uint8{1, 0, 0}, nil, []string{"circular_b"})
	builder.AddModule("circular_b", [3]uint8{1, 0, 0}, nil, []string{"circular_a"})
	sab := builder.Build()

	mr := NewModuleRegistry(sab)
	err := mr.LoadFromSAB()
	require.NoError(t, err)

	_, err = mr.GetDependencyOrder()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency detected")
}

func indexOf(slice []string, val string) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return -1
}

func TestModuleRegistry_CapabilityParsing(t *testing.T) {
	builder := testutil.NewMockSABBuilder(1024 * 1024)
	builder.AddModule("gpu_mod", [3]uint8{1, 0, 0}, []string{"compute_heavy", "rendering"}, nil)
	sab := builder.Build()

	mr := NewModuleRegistry(sab)
	err := mr.LoadFromSAB()
	require.NoError(t, err)

	mod, err := mr.GetModule("gpu_mod")
	require.NoError(t, err)

	assert.Len(t, mod.Capabilities, 2)
	assert.Equal(t, "compute_heavy", mod.Capabilities[0].ID)
	// AddModule logic sets default memory to 128
	assert.Equal(t, uint16(128), mod.Capabilities[0].MinMemoryMB)
}

func TestModuleRegistry_VersionIncompatibility(t *testing.T) {
	// Test Version Logic manually since MockSABBuilder defaults to broad range (0-255)
	// We can manually write to the Mock SAB if needed, OR test the helper directly?
	// But `validateDependencies` calls it.
	// We can add a module, then hack the SAB to restrict the version range?
	// MockSABBuilder returns `Build()` which is byte slice. We can modify it.

	// Better: Use `AddModule` then modify the dep table in the SAB before loading.
	// But we don't know exact offsets easily without `allocateArena` tracking.
	// Actually, `MockSABBuilder` is opaque.

	// BUT, we can test `isVersionCompatible` helper via internal test if we expose it?
	// It is unexported `isVersionCompatible`.
	// Since we are in `package registry`, we can test it directly!

	v100 := VersionTriple{1, 0, 0}
	v200 := VersionTriple{2, 0, 0}

	// Case 1: Exact match
	assert.True(t, isVersionCompatible(v100, v100, v100))

	// Case 2: Range
	assert.True(t, isVersionCompatible(VersionTriple{1, 5, 0}, v100, v200))

	// Case 3: Too low
	assert.False(t, isVersionCompatible(VersionTriple{0, 9, 9}, v100, v200))

	// Case 4: Too high
	assert.False(t, isVersionCompatible(VersionTriple{2, 1, 0}, v100, v200))
}

func TestModuleRegistry_Stats(t *testing.T) {
	builder := testutil.NewMockSABBuilder(1024 * 1024)
	builder.AddModule("mod_1", [3]uint8{1, 0, 0}, nil, nil)
	sab := builder.Build()

	mr := NewModuleRegistry(sab)
	mr.LoadFromSAB()

	stats := mr.GetStats()
	assert.Equal(t, 1, stats.LoadedModules)
	assert.Equal(t, 64, stats.TotalModules) // MAX_MODULES_INLINE
	assert.False(t, stats.HasCircularDeps)
}

func TestModuleRegistry_HashMismatch(t *testing.T) {
	// Scenario: Registry entry has corrupted IDHash, but ModuleID string is correct.
	// Dependency lookup by Hash fails in map, should fallback to string scan.

	builder := testutil.NewMockSABBuilder(1024 * 1024)
	builder.AddModule("target_mod", [3]uint8{1, 0, 0}, nil, nil)
	builder.AddModule("dependent", [3]uint8{1, 0, 0}, nil, []string{"target_mod"})
	sab := builder.Build()

	// Corrupt "target_mod" hash in SAB
	// "target_mod" is first module (slot 0).
	// Hash is at offset 8.
	// 0x000100 + (0 * 96) + 8 = 0x108
	// Write garbage hash
	offset := 0x000100 + 8
	binary.LittleEndian.PutUint32(sab[offset:], 0xDEADBEEF)

	mr := NewModuleRegistry(sab)
	err := mr.LoadFromSAB()
	require.NoError(t, err)

	// If fallback works, this succeeds. If failed, "dependent" would complain about missing "target_mod"
	// because `readDependencyTable` calls `reverseHashLookup`.

	// Verify "target_mod" is loaded but under wrong hash in map?
	// Actually `LoadFromSAB` puts it in `byHash` with the read hash (DEADBEEF).
	// `reverseHashLookup` searches for `crc32("target_mod")`.
	// `byHash` check fails.
	// Fallback loop scans modules. Finds "target_mod". Computes crc32. Matches. Returns ID.

	depOrder, err := mr.GetDependencyOrder()
	require.NoError(t, err)
	assert.Len(t, depOrder, 2)
}
