package sab

import (
	"sync/atomic"
	"unsafe"
)

// InMemoryProvider stores SAB data in a local byte slice.
type InMemoryProvider struct {
	data []byte
}

// NewInMemoryProvider creates an in-memory provider with the requested size.
func NewInMemoryProvider(size uint32) *InMemoryProvider {
	return &InMemoryProvider{
		data: make([]byte, size),
	}
}

func (m *InMemoryProvider) Size() uint32 {
	return uint32(len(m.data))
}

func (m *InMemoryProvider) ReadAt(offset uint32, dest []byte) error {
	if offset+uint32(len(dest)) > uint32(len(m.data)) {
		return ErrOutOfBounds
	}
	copy(dest, m.data[offset:offset+uint32(len(dest))])
	return nil
}

func (m *InMemoryProvider) WriteAt(offset uint32, src []byte) error {
	if offset+uint32(len(src)) > uint32(len(m.data)) {
		return ErrOutOfBounds
	}
	copy(m.data[offset:offset+uint32(len(src))], src)
	return nil
}

func (m *InMemoryProvider) AtomicLoad32(offset uint32) (uint32, error) {
	ptr, err := m.ptrAt(offset)
	if err != nil {
		return 0, err
	}
	return atomic.LoadUint32((*uint32)(ptr)), nil
}

func (m *InMemoryProvider) AtomicStore32(offset uint32, val uint32) error {
	ptr, err := m.ptrAt(offset)
	if err != nil {
		return err
	}
	atomic.StoreUint32((*uint32)(ptr), val)
	return nil
}

func (m *InMemoryProvider) AtomicAdd32(offset uint32, delta uint32) (uint32, error) {
	ptr, err := m.ptrAt(offset)
	if err != nil {
		return 0, err
	}
	return atomic.AddUint32((*uint32)(ptr), delta), nil
}

func (m *InMemoryProvider) Close() error {
	m.data = nil
	return nil
}

func (m *InMemoryProvider) ptrAt(offset uint32) (unsafe.Pointer, error) {
	if offset+4 > uint32(len(m.data)) {
		return nil, ErrOutOfBounds
	}
	if offset%4 != 0 {
		return nil, ErrMisaligned
	}
	return unsafe.Pointer(&m.data[offset]), nil
}
