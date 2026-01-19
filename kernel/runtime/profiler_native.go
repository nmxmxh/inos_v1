//go:build !js || !wasm

package runtime

import "time"

// Profiler mocks the runtime environment measurement on native
type Profiler struct{}

func NewProfiler() *Profiler {
	return &Profiler{}
}

func (p *Profiler) Profile() RuntimeCapabilities {
	return RuntimeCapabilities{
		ComputeScore:    1.0,
		NetworkLatency:  5 * time.Millisecond,
		AtomicsOverhead: 100 * time.Nanosecond,
		IsHeadless:      true,
	}
}
