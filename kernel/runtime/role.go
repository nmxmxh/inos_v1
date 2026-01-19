package runtime

import (
	"time"

	system "github.com/nmxmxh/inos_v1/kernel/gen/system/v1"
	"github.com/nmxmxh/inos_v1/kernel/utils"
)

// RoleConfig defines the mesh behavior for a specific role
type RoleConfig struct {
	Role          system.Runtime_RuntimeRole
	GossipFanout  int           // Number of peers to gossip to
	BatchInterval time.Duration // Interval for batching mutations
	CanRelay      bool          // Can act as a relay for others
	MaxPeers      int           // Maximum peer connections
}

// AssignRole determines the role based on capabilities
func AssignRole(caps RuntimeCapabilities) RoleConfig {
	// Thresholds
	// High atomic overhead suggests V8/Chromium (OS locks) -> Neuron
	// Low atomic overhead suggests SpiderMonkey/Firefox -> Synapse
	// Very low compute -> Sentry

	const AtomicsThreshold = 2000 * time.Nanosecond // 2us
	const ComputeThreshold = 0.5                    // Relative score

	var role system.Runtime_RuntimeRole
	var config RoleConfig

	// Heuristics
	if caps.AtomicsOverhead < AtomicsThreshold && caps.ComputeScore > ComputeThreshold {
		// Fast atomics + good compute = Synapse (Firefox / Native)
		role = system.Runtime_RuntimeRole_synapse
		config = RoleConfig{
			Role:          role,
			GossipFanout:  6,
			BatchInterval: 10 * time.Millisecond,
			CanRelay:      true,
			MaxPeers:      50,
		}
	} else if caps.ComputeScore > ComputeThreshold {
		// Good compute, but slower atomics = Neuron (Chromium / V8)
		role = system.Runtime_RuntimeRole_neuron
		config = RoleConfig{
			Role:          role,
			GossipFanout:  3,
			BatchInterval: 50 * time.Millisecond, // Batch more to hide overhead
			CanRelay:      false,
			MaxPeers:      20,
		}
	} else {
		// Low compute = Sentry (Safari / Mobile)
		role = system.Runtime_RuntimeRole_sentry
		config = RoleConfig{
			Role:          role,
			GossipFanout:  2,
			BatchInterval: 100 * time.Millisecond,
			CanRelay:      false,
			MaxPeers:      10,
		}
	}

	utils.Info("Runtime: Role Assigned",
		utils.String("role", role.String()),
		utils.Int("fanout", config.GossipFanout),
		utils.Int64("batch_ms", config.BatchInterval.Milliseconds()),
	)

	return config
}
