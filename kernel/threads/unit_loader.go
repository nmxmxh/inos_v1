//go:build wasm

package threads

import (
	"unsafe"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/registry"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor/units"
)

// UnitLoader handles the instantiation of unit supervisors
type UnitLoader struct {
	sab             unsafe.Pointer
	patterns        *pattern.TieredPatternStorage
	knowledge       *intelligence.KnowledgeGraph
	registry        *registry.ModuleRegistry
	credits         *supervisor.CreditSupervisor
	metricsProvider units.MetricsProvider
	delegator       foundation.MeshDelegator
	sabSize         uint32
}

// NewUnitLoader creates a new unit loader
func NewUnitLoader(sab unsafe.Pointer, size uint32, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, registry *registry.ModuleRegistry, credits *supervisor.CreditSupervisor, metricsProvider units.MetricsProvider, delegator foundation.MeshDelegator) *UnitLoader {
	return &UnitLoader{
		sab:             sab,
		sabSize:         size,
		patterns:        patterns,
		knowledge:       knowledge,
		registry:        registry,
		credits:         credits,
		metricsProvider: metricsProvider,
		delegator:       delegator,
	}
}

// LoadUnits creates all unit supervisors sharing a single SAB bridge
// Returns a map of supervisor name to Supervisor interface AND the shared bridge
func (ul *UnitLoader) LoadUnits() (map[string]interface{}, *supervisor.SABBridge) {
	bridge := ul.GetBridge()
	loaded := make(map[string]interface{})

	// 1. Refresh registry from SAB (ensure we have latest definitions)
	if err := ul.registry.LoadFromSAB(); err != nil {
		// Log error but continue with what we have? Or fail?
	}

	// 2. Discover modules dynamically
	modules := ul.registry.ListModules()

	for _, module := range modules {
		loaded[module.ID] = ul.InstantiateUnit(bridge, module)
	}

	// 3. EXPLICIT: Create BoidsSupervisor
	// 'boids' is registered as a CAPABILITY of 'compute', not as a module ID
	// So the switch case above never matches - we must create it explicitly
	if _, exists := loaded["boids_supervisor"]; !exists {
		loaded["boids_supervisor"] = units.NewBoidsSupervisor(bridge, ul.patterns, ul.knowledge, nil, ul.metricsProvider, ul.delegator)
	}

	// 4. EXPLICIT: Create AnalyticsSupervisor
	if _, exists := loaded["analytics_supervisor"]; !exists {
		loaded["analytics_supervisor"] = units.NewAnalyticsSupervisor(bridge, ul.patterns, ul.knowledge, ul.metricsProvider, ul.delegator)
	}

	// 5. EXPLICIT: Create RobotSupervisor (Moonshot)
	if _, exists := loaded["robot_supervisor"]; !exists {
		loaded["robot_supervisor"] = units.NewRobotSupervisor(bridge, ul.patterns, ul.knowledge, ul.metricsProvider, ul.delegator)
	}

	return loaded, bridge
}

// GetBridge creates or returns a shared bridge
func (ul *UnitLoader) GetBridge() *supervisor.SABBridge {
	return supervisor.NewSABBridge(ul.sab, ul.sabSize, sab_layout.OFFSET_INBOX_BASE, sab_layout.OFFSET_OUTBOX_BASE, sab_layout.IDX_SYSTEM_EPOCH)
}

// InstantiateUnit creates a specific supervisor for a module
func (ul *UnitLoader) InstantiateUnit(bridge *supervisor.SABBridge, module *registry.RegisteredModule) interface{} {
	name := module.ID
	var capabilities []string
	for _, cap := range module.Capabilities {
		capabilities = append(capabilities, cap.ID)
	}

	// Instantiate specialized supervisors based on ID
	switch name {
	case "storage":
		return units.NewStorageSupervisor(bridge, ul.patterns, ul.knowledge, capabilities, ul.delegator)
	case "gpu":
		return units.NewGPUSupervisor(bridge, ul.patterns, ul.knowledge, capabilities, ul.delegator)
	case "audio":
		return units.NewAudioSupervisor(bridge, ul.patterns, ul.knowledge, capabilities, ul.delegator)
	case "image":
		return units.NewImageSupervisor(bridge, ul.patterns, ul.knowledge, capabilities, ul.delegator)
	case "crypto":
		return units.NewCryptoSupervisor(bridge, ul.patterns, ul.knowledge, capabilities, ul.delegator)
	case "data":
		return units.NewDataSupervisor(bridge, ul.patterns, ul.knowledge, capabilities, ul.delegator)
	case "boids":
		return units.NewBoidsSupervisor(bridge, ul.patterns, ul.knowledge, capabilities, ul.metricsProvider, ul.delegator)
	case "driver":
		return units.NewDriverSupervisor(bridge, ul.credits, ul.patterns, ul.knowledge, capabilities, ul.delegator)
	case "identity":
		return units.NewIdentitySupervisor(bridge, ul.patterns, ul.knowledge, ul.sab, ul.sabSize, sab_layout.OFFSET_IDENTITY_REGISTRY, ul.delegator)
	case "analytics":
		return units.NewAnalyticsSupervisor(bridge, ul.patterns, ul.knowledge, ul.metricsProvider, ul.delegator)
	case "robot":
		return units.NewRobotSupervisor(bridge, ul.patterns, ul.knowledge, ul.metricsProvider, ul.delegator)
	default:
		// Fallback: Generic Supervisor for new/unknown modules
		return supervisor.NewUnifiedSupervisor(name, capabilities, ul.patterns, ul.knowledge, ul.delegator)
	}
}
