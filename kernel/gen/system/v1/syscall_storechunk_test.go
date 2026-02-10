package system

import (
	"testing"

	capnp "zombiezen.com/go/capnproto2"
)

func TestSyscallStoreChunkResultReplicasUInt16(t *testing.T) {
	_, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		t.Fatalf("failed to create capnp message: %v", err)
	}

	res, err := NewRootSyscall_StoreChunkResult(seg)
	if err != nil {
		t.Fatalf("failed to allocate StoreChunkResult: %v", err)
	}

	const expected uint16 = 700
	res.SetReplicas(expected)
	if got := res.Replicas(); got != expected {
		t.Fatalf("replica wire type regression: got=%d want=%d", got, expected)
	}
}
