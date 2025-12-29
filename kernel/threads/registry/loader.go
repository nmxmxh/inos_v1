package registry

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"sync"
)

// Constants from sab/layout.go
const (
	OFFSET_MODULE_REGISTRY = 0x000100
	MODULE_ENTRY_SIZE      = 96
	MAX_MODULES_INLINE     = 64
)

// EnhancedModuleEntry matches Rust definition (96 bytes)
type EnhancedModuleEntry struct {
	Signature       uint64
	IDHash          uint32
	VersionMajor    uint8
	VersionMinor    uint8
	VersionPatch    uint8
	Flags           uint8
	Timestamp       uint64
	DataOffset      uint32
	DataSize        uint32
	ResourceFlags   uint16
	MinMemoryMB     uint16
	MinGPUMemoryMB  uint16
	MinCPUCores     uint8
	Reserved1       uint8
	BaseCost        uint16
	PerMBCost       uint8
	PerSecondCost   uint16
	Reserved2       uint8
	DepTableOffset  uint32
	DepCount        uint16
	MaxVersionMajor uint8
	MinVersionMajor uint8
	CapTableOffset  uint32
	CapCount        uint16
	Reserved3       [2]byte
	ModuleID        [12]byte
	QuickHash       uint32
}

// RegisteredModule represents a loaded module with full metadata
type RegisteredModule struct {
	ID              string
	Version         VersionTriple
	Capabilities    []CapabilitySpec
	Dependencies    []DependencySpec
	ResourceProfile ResourceProfile
	CostModel       CostModel
	Slot            int
	Timestamp       uint64
}

type CapabilitySpec struct {
	ID          string
	RequiresGPU bool
	MinMemoryMB uint16
}

type VersionTriple struct {
	Major uint8
	Minor uint8
	Patch uint8
}

type DependencySpec struct {
	ModuleID     string
	MinVersion   VersionTriple
	MaxVersion   VersionTriple
	Optional     bool
	Alternatives []string
}

type ResourceProfile struct {
	Flags          uint16
	MinMemoryMB    uint16
	MinGPUMemoryMB uint16
	MinCPUCores    uint8
}

type CostModel struct {
	BaseCost      uint16
	PerMBCost     uint8
	PerSecondCost uint16
}

// ModuleRegistry manages module discovery with version awareness
type ModuleRegistry struct {
	sab     []byte
	modules map[string]*RegisteredModule
	byHash  map[uint32]*RegisteredModule
	mu      sync.RWMutex
}

// NewModuleRegistry creates a new module registry
func NewModuleRegistry(sab []byte) *ModuleRegistry {
	return &ModuleRegistry{
		sab:     sab,
		modules: make(map[string]*RegisteredModule),
		byHash:  make(map[uint32]*RegisteredModule),
	}
}

// LoadFromSAB loads all registered modules from SAB
func (mr *ModuleRegistry) LoadFromSAB() error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	loadedCount := 0

	// Scan all inline slots
	for slot := 0; slot < MAX_MODULES_INLINE; slot++ {
		entry, err := mr.readEnhancedEntry(slot)
		if err != nil {
			continue
		}

		// Skip invalid entries
		if entry.Signature != 0x494E4F5352454749 || entry.IDHash == 0 {
			continue
		}

		// Skip inactive entries
		if (entry.Flags & 0b0010) == 0 {
			continue
		}

		// Parse module
		module := mr.parseModule(entry, slot)
		mr.modules[module.ID] = module
		mr.byHash[entry.IDHash] = module
		loadedCount++
	}

	// Validate dependencies
	if err := mr.validateDependencies(); err != nil {
		return fmt.Errorf("dependency validation failed: %w", err)
	}

	return nil
}

// GetModule returns a module by ID
func (mr *ModuleRegistry) GetModule(moduleID string) (*RegisteredModule, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	module, exists := mr.modules[moduleID]
	if !exists {
		return nil, fmt.Errorf("module %s not found", moduleID)
	}

	return module, nil
}

// ListModules returns all registered modules
func (mr *ModuleRegistry) ListModules() []*RegisteredModule {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	modules := make([]*RegisteredModule, 0, len(mr.modules))
	for _, m := range mr.modules {
		modules = append(modules, m)
	}

	return modules
}

// GetDependencyOrder returns modules in dependency order with version checking
func (mr *ModuleRegistry) GetDependencyOrder() ([]string, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	// Build dependency graph
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	for id := range mr.modules {
		inDegree[id] = 0
		graph[id] = []string{}
	}

	// Add edges with version checking
	for id, module := range mr.modules {
		for _, dep := range module.Dependencies {
			// Check if dependency exists
			depModule, exists := mr.modules[dep.ModuleID]
			if !exists {
				if dep.Optional {
					continue
				}

				// Try alternatives
				foundAlt := false
				for _, alt := range dep.Alternatives {
					if _, exists := mr.modules[alt]; exists {
						graph[alt] = append(graph[alt], id)
						inDegree[id]++
						foundAlt = true
						break
					}
				}

				if !foundAlt {
					return nil, fmt.Errorf(
						"unsatisfied dependency: %s requires %s@%d.%d.%d",
						id, dep.ModuleID,
						dep.MinVersion.Major, dep.MinVersion.Minor, dep.MinVersion.Patch,
					)
				}
				continue
			}

			// Check version compatibility
			if !isVersionCompatible(depModule.Version, dep.MinVersion, dep.MaxVersion) {
				return nil, fmt.Errorf(
					"version incompatibility: %s requires %s@%d.%d.%d but found %d.%d.%d",
					id, dep.ModuleID,
					dep.MinVersion.Major, dep.MinVersion.Minor, dep.MinVersion.Patch,
					depModule.Version.Major, depModule.Version.Minor, depModule.Version.Patch,
				)
			}

			// Add edge
			graph[dep.ModuleID] = append(graph[dep.ModuleID], id)
			inDegree[id]++
		}
	}

	// Kahn's algorithm for topological sort
	queue := []string{}
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	result := []string{}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		result = append(result, id)

		for _, neighbor := range graph[id] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(result) != len(mr.modules) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return result, nil
}

// Helper: Check version compatibility
func isVersionCompatible(actual, min, max VersionTriple) bool {
	// Check minimum version
	if actual.Major < min.Major {
		return false
	}
	if actual.Major == min.Major && actual.Minor < min.Minor {
		return false
	}
	if actual.Major == min.Major && actual.Minor == min.Minor && actual.Patch < min.Patch {
		return false
	}

	// Check maximum version
	if actual.Major > max.Major {
		return false
	}
	if actual.Major == max.Major && actual.Minor > max.Minor {
		return false
	}
	if actual.Major == max.Major && actual.Minor == max.Minor && actual.Patch > max.Patch {
		return false
	}

	return true
}

// Helper: Read enhanced entry from SAB
func (mr *ModuleRegistry) readEnhancedEntry(slot int) (*EnhancedModuleEntry, error) {
	offset := OFFSET_MODULE_REGISTRY + (slot * MODULE_ENTRY_SIZE)

	if offset+MODULE_ENTRY_SIZE > len(mr.sab) {
		return nil, fmt.Errorf("offset out of bounds")
	}

	entry := &EnhancedModuleEntry{}
	data := mr.sab[offset : offset+MODULE_ENTRY_SIZE]

	entry.Signature = binary.LittleEndian.Uint64(data[0:8])
	entry.IDHash = binary.LittleEndian.Uint32(data[8:12])
	entry.VersionMajor = data[12]
	entry.VersionMinor = data[13]
	entry.VersionPatch = data[14]
	entry.Flags = data[15]
	entry.Timestamp = binary.LittleEndian.Uint64(data[16:24])
	entry.DataOffset = binary.LittleEndian.Uint32(data[24:28])
	entry.DataSize = binary.LittleEndian.Uint32(data[28:32])
	entry.ResourceFlags = binary.LittleEndian.Uint16(data[32:34])
	entry.MinMemoryMB = binary.LittleEndian.Uint16(data[34:36])
	entry.MinGPUMemoryMB = binary.LittleEndian.Uint16(data[36:38])
	entry.MinCPUCores = data[38]
	entry.BaseCost = binary.LittleEndian.Uint16(data[40:42])
	entry.PerMBCost = data[42]
	entry.PerSecondCost = binary.LittleEndian.Uint16(data[43:45])
	entry.DepTableOffset = binary.LittleEndian.Uint32(data[48:52])
	entry.DepCount = binary.LittleEndian.Uint16(data[52:54])
	entry.MaxVersionMajor = data[54]
	entry.MinVersionMajor = data[55]
	entry.CapTableOffset = binary.LittleEndian.Uint32(data[56:60])
	entry.CapCount = binary.LittleEndian.Uint16(data[60:62])
	copy(entry.ModuleID[:], data[64:76])
	entry.QuickHash = binary.LittleEndian.Uint32(data[76:80])

	return entry, nil
}

// Helper: Parse module from entry
func (mr *ModuleRegistry) parseModule(entry *EnhancedModuleEntry, slot int) *RegisteredModule {
	// Extract module ID
	nullPos := 12
	for i, b := range entry.ModuleID {
		if b == 0 {
			nullPos = i
			break
		}
	}
	moduleID := string(entry.ModuleID[:nullPos])

	// Read dependencies from arena if present
	dependencies := []DependencySpec{}
	if entry.DepCount > 0 && entry.DepTableOffset > 0 {
		dependencies = mr.readDependencyTable(entry.DepTableOffset, entry.DepCount)
	}

	// Read capabilities from arena if present
	capabilities := []CapabilitySpec{}
	if entry.CapCount > 0 && entry.CapTableOffset > 0 {
		capabilities = mr.readCapabilityTable(entry.CapTableOffset, entry.CapCount)
	}

	return &RegisteredModule{
		ID: moduleID,
		Version: VersionTriple{
			Major: entry.VersionMajor,
			Minor: entry.VersionMinor,
			Patch: entry.VersionPatch,
		},
		Capabilities: capabilities,
		Dependencies: dependencies,
		ResourceProfile: ResourceProfile{
			Flags:          entry.ResourceFlags,
			MinMemoryMB:    entry.MinMemoryMB,
			MinGPUMemoryMB: entry.MinGPUMemoryMB,
			MinCPUCores:    entry.MinCPUCores,
		},
		CostModel: CostModel{
			BaseCost:      entry.BaseCost,
			PerMBCost:     entry.PerMBCost,
			PerSecondCost: entry.PerSecondCost,
		},
		Slot:      slot,
		Timestamp: entry.Timestamp,
	}
}

// Helper: Read dependency table from arena
func (mr *ModuleRegistry) readDependencyTable(offset uint32, count uint16) []DependencySpec {
	dependencies := make([]DependencySpec, 0, count)

	// Read dependency entries (16 bytes each)
	entryOffset := offset + 12 // Skip header
	for i := uint16(0); i < count; i++ {
		data := mr.sab[entryOffset : entryOffset+16]

		moduleIDHash := binary.LittleEndian.Uint32(data[0:4])
		minMajor := data[4]
		minMinor := data[5]
		minPatch := data[6]
		maxMajor := data[7]
		maxMinor := data[8]
		maxPatch := data[9]
		flags := data[10]

		// Reverse lookup module ID from hash
		moduleID := mr.reverseHashLookup(moduleIDHash)
		if moduleID == "" {
			entryOffset += 16
			continue
		}

		dependencies = append(dependencies, DependencySpec{
			ModuleID: moduleID,
			MinVersion: VersionTriple{
				Major: minMajor,
				Minor: minMinor,
				Patch: minPatch,
			},
			MaxVersion: VersionTriple{
				Major: maxMajor,
				Minor: maxMinor,
				Patch: maxPatch,
			},
			Optional:     (flags & 0b0001) != 0,
			Alternatives: []string{},
		})

		entryOffset += 16
	}

	return dependencies
}

// Helper: Read capability table from arena
func (mr *ModuleRegistry) readCapabilityTable(offset uint32, count uint16) []CapabilitySpec {
	capabilities := make([]CapabilitySpec, 0, count)

	// Read capability entries (assume 36 bytes: 32 bytes ID + 2 flags + 2 reserved)
	// This matches the Rust layout implied in threads.md
	entrySize := uint32(36)
	entryOffset := offset

	for i := uint16(0); i < count; i++ {
		if entryOffset+entrySize > uint32(len(mr.sab)) {
			break
		}

		data := mr.sab[entryOffset : entryOffset+entrySize]

		// Extract ID (first 32 bytes, null terminated)
		nullPos := 32
		for j, b := range data[0:32] {
			if b == 0 {
				nullPos = j
				break
			}
		}
		id := string(data[0:nullPos])

		// Layout: [ID:32][MinMemoryMB:2][Flags:1][Reserved:1]
		minMemoryMB := binary.LittleEndian.Uint16(data[32:34])
		flags := data[34]

		capabilities = append(capabilities, CapabilitySpec{
			ID:          id,
			RequiresGPU: (flags & 0b00000001) != 0,
			MinMemoryMB: minMemoryMB,
		})

		entryOffset += entrySize
	}

	return capabilities
}

// Helper: Reverse hash lookup (known modules)
func (mr *ModuleRegistry) reverseHashLookup(hash uint32) string {
	knownModules := []string{
		"ml", "gpu", "storage", "crypto", "image",
		"audio", "data", "mining", "physics", "science",
	}

	for _, id := range knownModules {
		if crc32.ChecksumIEEE([]byte(id)) == hash {
			return id
		}
	}

	return ""
}

// Helper: Validate dependencies
func (mr *ModuleRegistry) validateDependencies() error {
	for _, module := range mr.modules {
		for _, dep := range module.Dependencies {
			if _, exists := mr.modules[dep.ModuleID]; !exists && !dep.Optional {
				// Check alternatives
				foundAlt := false
				for _, alt := range dep.Alternatives {
					if _, exists := mr.modules[alt]; exists {
						foundAlt = true
						break
					}
				}
				if !foundAlt {
					return fmt.Errorf(
						"module %s depends on %s which is not registered",
						module.ID, dep.ModuleID,
					)
				}
			}
		}
	}

	return nil
}

// GetStats returns registry statistics
type RegistryStats struct {
	TotalModules    int
	LoadedModules   int
	HasCircularDeps bool
}

func (mr *ModuleRegistry) GetStats() RegistryStats {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	_, err := mr.GetDependencyOrder()

	return RegistryStats{
		TotalModules:    MAX_MODULES_INLINE,
		LoadedModules:   len(mr.modules),
		HasCircularDeps: err != nil,
	}
}
