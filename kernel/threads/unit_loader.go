//go:build wasm

package threads

import (
	"unsafe"

	kruntime "github.com/nmxmxh/inos_v1/kernel/runtime"
	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/registry"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor/units"
	"github.com/nmxmxh/inos_v1/kernel/utils"
)

// UnitLoader handles the instantiation of unit supervisors
type UnitLoader struct {
	replica         []byte
	patterns        *pattern.TieredPatternStorage
	knowledge       *intelligence.KnowledgeGraph
	registry        *registry.ModuleRegistry
	credits         *supervisor.CreditSupervisor
	identity        *units.IdentitySupervisor
	metricsProvider units.MetricsProvider
	delegator       foundation.MeshDelegator
	role            kruntime.RoleConfig
	sabSize         uint32
}

// NewUnitLoader creates a new unit loader
func NewUnitLoader(replica []byte, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, registry *registry.ModuleRegistry, credits *supervisor.CreditSupervisor, identity *units.IdentitySupervisor, metricsProvider units.MetricsProvider, role kruntime.RoleConfig, delegator foundation.MeshDelegator) *UnitLoader {
	return &UnitLoader{
		replica:         replica,
		sabSize:         uint32(len(replica)),
		patterns:        patterns,
		knowledge:       knowledge,
		registry:        registry,
		credits:         credits,
		identity:        identity,
		metricsProvider: metricsProvider,
		role:            role,
		delegator:       delegator,
	}
}

// LoadUnits creates all unit supervisors sharing a single SAB bridge
// Returns a map of supervisor name to Supervisor interface AND the shared bridge
func (ul *UnitLoader) LoadUnits() (map[string]interface{}, *supervisor.SABBridge) {
	bridge := ul.GetBridge()
	loaded := make(map[string]interface{})

	// 1. Refresh registry from authoritative Global SAB
	bridge.RefreshRegistry()

	// 2. Load from local replica
	if err := ul.registry.LoadFromSAB(); err != nil {
		// Log error but continue with what we have
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
		loaded["boids_supervisor"] = units.NewBoidsSupervisor(bridge, ul.role, ul.patterns, ul.knowledge, nil, ul.metricsProvider, ul.delegator)
	}

	// 4. EXPLICIT: Create AnalyticsSupervisor
	if _, exists := loaded["analytics_supervisor"]; !exists {
		loaded["analytics_supervisor"] = units.NewAnalyticsSupervisor(bridge, ul.patterns, ul.knowledge, ul.metricsProvider, ul.delegator)
	}

	// 5. EXPLICIT: Ensure DataSupervisor exists for generic compute/data tasks
	if _, exists := loaded["data"]; !exists {
		loaded["data"] = units.NewDataSupervisor(bridge, ul.patterns, ul.knowledge, nil, ul.delegator)
	}

	// 6. EXPLICIT: Ensure GPUSupervisor exists for GPU-accelerated tasks
	// Even if not yet in registry, this allows early submission with default capabilities
	if _, exists := loaded["gpu"]; !exists {
		loaded["gpu"] = units.NewGPUSupervisor(bridge, ul.patterns, ul.knowledge, nil, ul.delegator)
	}

	return loaded, bridge
}

// GetBridge creates or returns a shared bridge
func (ul *UnitLoader) GetBridge() *supervisor.SABBridge {
	return supervisor.NewSABBridge(
		ul.replica,
		sab_layout.OFFSET_INBOX_BASE,
		sab_layout.OFFSET_OUTBOX_HOST_BASE,
		sab_layout.OFFSET_OUTBOX_KERNEL_BASE,
		sab_layout.IDX_SYSTEM_EPOCH,
	)
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
		println("DEBUG: Instantiating GPUSupervisor")
		utils.Info("Creating specialized GPUSupervisor unit")
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
		return units.NewBoidsSupervisor(bridge, ul.role, ul.patterns, ul.knowledge, capabilities, ul.metricsProvider, ul.delegator)
	case "driver":
		return units.NewDriverSupervisor(bridge, ul.credits, ul.patterns, ul.knowledge, capabilities, ul.delegator)
	case "identity":
		if ul.identity != nil {
			return ul.identity
		}
		return units.NewIdentitySupervisor(bridge, ul.patterns, ul.knowledge, unsafe.Pointer(&ul.replica[0]), ul.sabSize, sab_layout.OFFSET_IDENTITY_REGISTRY, ul.credits, nil, ul.delegator)
	case "analytics":
		return units.NewAnalyticsSupervisor(bridge, ul.patterns, ul.knowledge, ul.metricsProvider, ul.delegator)
	default:
		// Fallback: Generic Supervisor for new/unknown modules
		return supervisor.NewUnifiedSupervisor(name, capabilities, ul.patterns, ul.knowledge, ul.delegator, bridge, nil)
	}
}
