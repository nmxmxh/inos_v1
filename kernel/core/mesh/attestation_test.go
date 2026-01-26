package mesh

import (
	"bytes"
	"testing"

	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
)

type testSABBridge struct {
	data []byte
}

func (t *testSABBridge) WriteRaw(offset uint32, data []byte) error { return nil }
func (t *testSABBridge) ReadRaw(offset uint32, size uint32) ([]byte, error) {
	start := int(offset)
	end := start + int(size)
	if start < 0 || end > len(t.data) {
		return nil, nil
	}
	out := make([]byte, size)
	copy(out, t.data[start:end])
	return out, nil
}
func (t *testSABBridge) SignalEpoch(index uint32)           {}
func (t *testSABBridge) GetAddress(data []byte) (uint32, bool) { return 0, false }
func (t *testSABBridge) Size() uint32                       { return uint32(len(t.data)) }
func (t *testSABBridge) AtomicLoad(index uint32) uint32     { return 0 }
func (t *testSABBridge) AtomicAdd(index uint32, delta uint32) uint32 {
	return 0
}

func TestAttestationRegionsWithinBounds(t *testing.T) {
	regions := defaultAttestationRegions(sab_layout.SAB_SIZE_DEFAULT)
	if len(regions) == 0 {
		t.Fatal("expected attestation regions to be defined")
	}
	if err := validateKnownRegions(regions, sab_layout.SAB_SIZE_DEFAULT); err != nil {
		t.Fatalf("expected regions to be valid: %v", err)
	}
	for _, region := range regions {
		if region.Length == 0 {
			t.Fatalf("region %s has zero length", region.Name)
		}
		if region.Length > attestationKnownRegionMaxBytes {
			t.Fatalf("region %s exceeds max length", region.Name)
		}
	}
}

func TestHashKnownRegionsDeterministic(t *testing.T) {
	sab := make([]byte, 8192)
	for i := range sab {
		sab[i] = byte(i % 251)
	}
	bridge := &testSABBridge{data: sab}
	regions := []AttestationRegion{
		{Name: "flags", Offset: 0, Length: 64},
		{Name: "registry", Offset: 128, Length: 128},
	}

	hashesA, err := hashKnownRegions(bridge, regions)
	if err != nil {
		t.Fatalf("hashKnownRegions failed: %v", err)
	}
	hashesB, err := hashKnownRegions(bridge, regions)
	if err != nil {
		t.Fatalf("hashKnownRegions failed: %v", err)
	}

	if !bytes.Equal(hashesA["flags"], hashesB["flags"]) || !bytes.Equal(hashesA["registry"], hashesB["registry"]) {
		t.Fatal("expected deterministic region hashes")
	}
}
