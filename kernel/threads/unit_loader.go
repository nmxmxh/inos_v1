package threads

import (
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/registry"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor/units"
)

// UnitLoader handles the instantiation of unit supervisors
type UnitLoader struct {
	sab       unsafe.Pointer
	patterns  *pattern.TieredPatternStorage
	knowledge *intelligence.KnowledgeGraph
	registry  *registry.ModuleRegistry
}

// NewUnitLoader creates a new unit loader
func NewUnitLoader(sab unsafe.Pointer, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, registry *registry.ModuleRegistry) *UnitLoader {
	return &UnitLoader{
		sab:       sab,
		patterns:  patterns,
		knowledge: knowledge,
		registry:  registry,
	}
}

// LoadUnits creates all unit supervisors sharing a single SAB bridge
// Returns a map of supervisor name to Supervisor interface AND the shared bridge
func (ul *UnitLoader) LoadUnits() (map[string]interface{}, *supervisor.SABBridge) {
	// Use standardized layout constants for cross-layer consistency
	sharedBridge := supervisor.NewSABBridge(ul.sab, sab_layout.OFFSET_INBOX_BASE, sab_layout.OFFSET_OUTBOX_BASE, sab_layout.IDX_SYSTEM_EPOCH)

	loaded := make(map[string]interface{})

	// 1. Refresh registry from SAB (ensure we have latest definitions)
	if err := ul.registry.LoadFromSAB(); err != nil {
		// Log error but continue with what we have? Or fail?
		// For now, assume pre-loaded or warn.
	}

	// 2. Discover modules dynamically
	modules := ul.registry.ListModules()

	for _, module := range modules {
		name := module.ID
		var capabilities []string
		for _, cap := range module.Capabilities {
			capabilities = append(capabilities, cap.ID)
		}

		// Instantiate specialized supervisors based on ID
		switch name {
		case "ml":
			loaded[name] = units.NewMLSupervisor(sharedBridge, ul.patterns, ul.knowledge, capabilities)
		case "storage":
			loaded[name] = units.NewStorageSupervisor(sharedBridge, ul.patterns, ul.knowledge, capabilities)
		case "gpu":
			loaded[name] = units.NewGPUSupervisor(sharedBridge, ul.patterns, ul.knowledge, capabilities)
		case "science":
			loaded[name] = units.NewScienceSupervisor(sharedBridge, ul.patterns, ul.knowledge, capabilities)
		case "mining":
			loaded[name] = units.NewMiningSupervisor(sharedBridge, ul.patterns, ul.knowledge, capabilities)
		case "audio":
			loaded[name] = units.NewAudioSupervisor(sharedBridge, ul.patterns, ul.knowledge, capabilities)
		case "image":
			loaded[name] = units.NewImageSupervisor(sharedBridge, ul.patterns, ul.knowledge, capabilities)
		case "crypto":
			loaded[name] = units.NewCryptoSupervisor(sharedBridge, ul.patterns, ul.knowledge, capabilities)
		case "data":
			loaded[name] = units.NewDataSupervisor(sharedBridge, ul.patterns, ul.knowledge, capabilities)
		case "physics":
			loaded[name] = units.NewPhysicsSupervisor(sharedBridge, ul.patterns, ul.knowledge, capabilities)
		case "driver":
			loaded[name] = units.NewDriverSupervisor(sharedBridge, ul.patterns, ul.knowledge, capabilities)
		default:
			// Fallback: Generic Supervisor for new/unknown modules
			// This enables true dynamic extensibility without code changes
			loaded[name] = supervisor.NewUnifiedSupervisor(name, capabilities, ul.patterns, ul.knowledge)
		}
	}

	return loaded, sharedBridge
}
