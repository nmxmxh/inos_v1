//go:build !js || !wasm

package sab

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// SharedMemoryProvider uses a memory-mapped file for shared access.
type SharedMemoryProvider struct {
	path string
	file *os.File
	data []byte
	size uint32
}

// SharedMemoryOptions configures shared memory creation/opening.
type SharedMemoryOptions struct {
	Path   string
	Size   uint32
	Create bool
}

// DefaultSharedMemoryPath returns the default shared memory path.
func DefaultSharedMemoryPath() string {
	if _, err := os.Stat("/dev/shm"); err == nil {
		return "/dev/shm/inos_sab"
	}
	return filepath.Join(os.TempDir(), "inos_sab")
}

// OpenSharedMemory opens or creates a shared memory mapping.
func OpenSharedMemory(opts SharedMemoryOptions) (*SharedMemoryProvider, error) {
	if opts.Path == "" {
		return nil, errors.New("shared memory path required")
	}

	path := filepath.Clean(opts.Path)
	flags := os.O_RDWR
	if opts.Create {
		flags |= os.O_CREATE
	}

	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open shared memory file: %w", err)
	}

	if opts.Create {
		if opts.Size == 0 {
			_ = file.Close()
			return nil, errors.New("shared memory size required when creating")
		}
		if err := file.Truncate(int64(opts.Size)); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("truncate shared memory file: %w", err)
		}
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat shared memory file: %w", err)
	}
	if info.Size() == 0 {
		_ = file.Close()
		return nil, errors.New("shared memory file has zero size")
	}
	size := uint32(info.Size())

	data, err := syscall.Mmap(int(file.Fd()), 0, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("mmap shared memory file: %w", err)
	}

	return &SharedMemoryProvider{
		path: path,
		file: file,
		data: data,
		size: size,
	}, nil
}

func (s *SharedMemoryProvider) Size() uint32 {
	return s.size
}

func (s *SharedMemoryProvider) ReadAt(offset uint32, dest []byte) error {
	if offset+uint32(len(dest)) > s.size {
		return ErrOutOfBounds
	}
	copy(dest, s.data[offset:offset+uint32(len(dest))])
	return nil
}

func (s *SharedMemoryProvider) WriteAt(offset uint32, src []byte) error {
	if offset+uint32(len(src)) > s.size {
		return ErrOutOfBounds
	}
	copy(s.data[offset:offset+uint32(len(src))], src)
	return nil
}

func (s *SharedMemoryProvider) AtomicLoad32(offset uint32) (uint32, error) {
	ptr, err := s.ptrAt(offset)
	if err != nil {
		return 0, err
	}
	return atomic.LoadUint32((*uint32)(ptr)), nil
}

func (s *SharedMemoryProvider) AtomicStore32(offset uint32, val uint32) error {
	ptr, err := s.ptrAt(offset)
	if err != nil {
		return err
	}
	atomic.StoreUint32((*uint32)(ptr), val)
	return nil
}

func (s *SharedMemoryProvider) AtomicAdd32(offset uint32, delta uint32) (uint32, error) {
	ptr, err := s.ptrAt(offset)
	if err != nil {
		return 0, err
	}
	return atomic.AddUint32((*uint32)(ptr), delta), nil
}

func (s *SharedMemoryProvider) Close() error {
	var err error
	if s.data != nil {
		if unmapErr := syscall.Munmap(s.data); unmapErr != nil {
			err = unmapErr
		}
		s.data = nil
	}
	if s.file != nil {
		if closeErr := s.file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		s.file = nil
	}
	return err
}

func (s *SharedMemoryProvider) ptrAt(offset uint32) (unsafe.Pointer, error) {
	if offset+4 > s.size {
		return nil, ErrOutOfBounds
	}
	if offset%4 != 0 {
		return nil, ErrMisaligned
	}
	return unsafe.Pointer(&s.data[offset]), nil
}
