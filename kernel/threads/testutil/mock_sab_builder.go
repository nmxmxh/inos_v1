package testutil

import (
	"encoding/binary"
	"hash/crc32"
	"time"
	"unsafe"

	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
)

// MockSABBuilder helps create consistent SAB layouts for testing
type MockSABBuilder struct {
	sab          []byte
	moduleCount  int
	patternCount int
	arenaOffset  uint32
}

// NewMockSABBuilder creates a new builder
func NewMockSABBuilder(size int) *MockSABBuilder {
	// Ensure minimum size
	if size < int(sab_layout.SAB_SIZE_DEFAULT) {
		size = int(sab_layout.SAB_SIZE_DEFAULT)
	}
	return &MockSABBuilder{
		sab:         make([]byte, size),
		arenaOffset: sab_layout.OFFSET_ARENA,
	}
}

// AddModule adds a module entry to the registry
func (m *MockSABBuilder) AddModule(id string, version [3]uint8, caps []string, deps []string) *MockSABBuilder {
	slot := m.moduleCount
	offset := int(sab_layout.OFFSET_MODULE_REGISTRY) + (slot * 96) // 96 bytes per entry

	// Create entry data (EnhancedModuleEntry)
	// Signature: 0x494E4F5352454749
	binary.LittleEndian.PutUint64(m.sab[offset:], 0x494E4F5352454749)

	// Hash ID
	hash := crc32.ChecksumIEEE([]byte(id))
	binary.LittleEndian.PutUint32(m.sab[offset+8:], hash)

	// Version
	m.sab[offset+12] = version[0]
	m.sab[offset+13] = version[1]
	m.sab[offset+14] = version[2]

	// Flags (Active)
	m.sab[offset+15] = 0b0010

	// Timestamp
	binary.LittleEndian.PutUint64(m.sab[offset+16:], uint64(time.Now().UnixNano()))

	// ID String (null terminated, max 12 bytes in fixed slot)
	idBytes := []byte(id)
	if len(idBytes) > 12 {
		idBytes = idBytes[:12]
	}
	copy(m.sab[offset+64:], idBytes)

	// Simple Resource Defaults
	binary.LittleEndian.PutUint16(m.sab[offset+34:], 128) // MinMemoryMB
	m.sab[offset+38] = 1                                  // MinCPUCores

	// Write Dependencies if present
	if len(deps) > 0 {
		depCount := uint16(len(deps))
		// Table Size: 12 bytes header + (16 bytes * count)
		tableSize := 12 + (16 * int(depCount))

		arenaAddr := m.allocateArena(tableSize)

		// Set offsets in fixed entry
		binary.LittleEndian.PutUint32(m.sab[offset+48:], uint32(arenaAddr)) // DepTableOffset
		binary.LittleEndian.PutUint16(m.sab[offset+52:], depCount)

		// Write Table Header (12 bytes - currently unused/reserved in loader except offset+12 start)
		// But loader reads `entryOffset = offset + 12`.

		// Write Entries
		entryStart := arenaAddr + 12
		for i, depId := range deps {
			entryOffset := entryStart + (i * 16)

			// Hash ID
			depHash := crc32.ChecksumIEEE([]byte(depId))
			binary.LittleEndian.PutUint32(m.sab[entryOffset:], depHash)

			// Min Version (0.0.0)
			// Max Version (0.0.0 is open?)
			// Loader logic: isVersionCompatible.
			// If we want "any version", max should be high?
			// Let's set Min 0.0.0, Max 255.255.255
			m.sab[entryOffset+7] = 255 // MaxMajor
			m.sab[entryOffset+8] = 255 // MaxMinor
			m.sab[entryOffset+9] = 255 // MaxPatch

			// Flags (Optional=1 at bit 0). We assume required for now.
			m.sab[entryOffset+10] = 0 // Required
		}
	}

	// Write Capabilities if present
	if len(caps) > 0 {
		capCount := uint16(len(caps))
		// Table Size: 36 bytes per entry (no header for caps table in loader? offset points directly to entries?
		// Loader: `readCapabilityTable` -> `entryOffset := offset`. Yes.
		tableSize := 36 * int(capCount)

		arenaAddr := m.allocateArena(tableSize)

		// Set offsets
		binary.LittleEndian.PutUint32(m.sab[offset+56:], uint32(arenaAddr)) // CapTableOffset
		binary.LittleEndian.PutUint16(m.sab[offset+60:], capCount)

		// Write Entries
		for i, capId := range caps {
			entryOffset := arenaAddr + (i * 36)

			// ID: 32 bytes (null terminated)
			idBytes := []byte(capId)
			if len(idBytes) > 32 {
				idBytes = idBytes[:32]
			}
			copy(m.sab[entryOffset:], idBytes)

			// MinMemoryMB: 2 bytes (bytes 32-34)
			binary.LittleEndian.PutUint16(m.sab[entryOffset+32:], 128)

			// Flags: 1 byte (byte 34)
			// RequiresGPU = 1?
			if capId == "gpu_compute" {
				m.sab[entryOffset+34] = 1
			} else {
				m.sab[entryOffset+34] = 0
			}

			// Reserved: 1 byte (byte 35)
		}
	}

	m.moduleCount++
	return m
}

func (m *MockSABBuilder) allocateArena(size int) int {
	addr := int(m.arenaOffset)
	m.arenaOffset += uint32(size)
	// Simple alignment padding? Go logic doesn't strictly need it but good practice.
	// Align to 4 bytes
	padding := (4 - (m.arenaOffset % 4)) % 4
	m.arenaOffset += padding
	return addr
}

// AddPattern adds a pattern to the pattern exchange
func (m *MockSABBuilder) AddPattern(id uint64, patternType uint16, confidence uint8, payload []byte) *MockSABBuilder {
	slot := m.patternCount
	offset := int(sab_layout.OFFSET_PATTERN_EXCHANGE) + (slot * 64) // 64 bytes per entry

	// Magic
	binary.LittleEndian.PutUint64(m.sab[offset:], 0x5041545F45582D50) // "PAT_EX-P"

	// ID
	binary.LittleEndian.PutUint64(m.sab[offset+8:], id)

	// Type
	binary.LittleEndian.PutUint16(m.sab[offset+18:], patternType)

	// Confidence
	m.sab[offset+21] = confidence

	// Write payload if present
	if len(payload) > 0 {
		payloadSize := uint32(len(payload))
		arenaAddr := m.allocateArena(int(payloadSize))

		// Copy payload to arena
		copy(m.sab[arenaAddr:], payload)

		// Set size in header (bytes 56-58)
		binary.LittleEndian.PutUint16(m.sab[offset+56:], uint16(payloadSize))

		// Set DataPtr (bytes 60-64)
		binary.LittleEndian.PutUint32(m.sab[offset+60:], uint32(arenaAddr))
	} else {
		// Zero size
		binary.LittleEndian.PutUint16(m.sab[offset+56:], 0)
		// Zero DataPtr
		binary.LittleEndian.PutUint32(m.sab[offset+60:], 0)
	}

	m.patternCount++
	return m
}

// Build returns the SAB byte slice
func (m *MockSABBuilder) Build() []byte {
	return m.sab
}

// GetPointer returns unsafe pointer to SAB start
func (m *MockSABBuilder) GetPointer() unsafe.Pointer {
	return unsafe.Pointer(&m.sab[0])
}
