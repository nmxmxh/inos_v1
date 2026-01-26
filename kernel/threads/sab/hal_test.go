package sab

import "testing"

func TestInMemoryProviderReadWrite(t *testing.T) {
	provider := NewInMemoryProvider(64)
	defer provider.Close()

	data := []byte{1, 2, 3, 4, 5}
	if err := provider.WriteAt(8, data); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	read := make([]byte, len(data))
	if err := provider.ReadAt(8, read); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	for i, v := range data {
		if read[i] != v {
			t.Fatalf("unexpected byte at %d: %d != %d", i, read[i], v)
		}
	}
}

func TestInMemoryProviderAtomic(t *testing.T) {
	provider := NewInMemoryProvider(16)
	defer provider.Close()

	if err := provider.AtomicStore32(4, 10); err != nil {
		t.Fatalf("store failed: %v", err)
	}
	val, err := provider.AtomicLoad32(4)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if val != 10 {
		t.Fatalf("expected 10, got %d", val)
	}
	newVal, err := provider.AtomicAdd32(4, 5)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if newVal != 15 {
		t.Fatalf("expected 15, got %d", newVal)
	}
}

func TestInMemoryProviderMisaligned(t *testing.T) {
	provider := NewInMemoryProvider(16)
	defer provider.Close()

	if _, err := provider.AtomicLoad32(2); err != ErrMisaligned {
		t.Fatalf("expected misaligned error, got %v", err)
	}
}
