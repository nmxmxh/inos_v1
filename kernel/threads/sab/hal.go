package sab

import "errors"

// MemoryProvider abstracts access to shared memory for SAB.
// Implementations may be backed by mmap, SharedArrayBuffer, or in-memory buffers.
type MemoryProvider interface {
	Size() uint32
	ReadAt(offset uint32, dest []byte) error
	WriteAt(offset uint32, src []byte) error
	AtomicLoad32(offset uint32) (uint32, error)
	AtomicStore32(offset uint32, val uint32) error
	AtomicAdd32(offset uint32, delta uint32) (uint32, error)
	Close() error
}

var ErrOutOfBounds = errors.New("offset out of bounds")
var ErrMisaligned = errors.New("offset is not 4-byte aligned")
