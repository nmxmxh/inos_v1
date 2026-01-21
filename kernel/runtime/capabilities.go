package runtime

import "time"

// RuntimeCapabilities holds the raw performance metrics of the node
type RuntimeCapabilities struct {
	ComputeScore    float64       // Normalized FLOPS / Integer performance
	NetworkLatency  time.Duration // Loopback WebRTC RTT estimate
	AtomicsOverhead time.Duration // Average overhead of Atomics.wait
	IsHeadless      bool          // Heuristic detection
}
