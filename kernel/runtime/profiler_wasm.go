//go:build js && wasm

package runtime

import (
	"syscall/js"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/utils"
)

// Profiler measures the runtime environment

// Profiler measures the runtime environment
type Profiler struct {
	console js.Value
}

func NewProfiler() *Profiler {
	return &Profiler{
		console: js.Global().Get("console"),
	}
}

// Profile executes all benchmarks and returns the capabilities
func (p *Profiler) Profile() RuntimeCapabilities {
	utils.Info("Profiler: Starting runtime capability analysis...")

	caps := RuntimeCapabilities{
		ComputeScore:    p.measureCompute(),
		AtomicsOverhead: p.measureAtomics(),
		// NetworkLatency requires async bridge calls, we'll estimate or mock for now
		// In a real implementation, this would ping the JS side over WebRTC
		NetworkLatency: 5 * time.Millisecond,
		IsHeadless:     p.detectHeadless(),
		HasGpu:         p.detectGpu(),
		HasSimd:        p.detectSimd(),
	}

	utils.Info("Profiler: Analysis complete",
		utils.Float64("compute_score", caps.ComputeScore),
		utils.Int64("atomics_ns", caps.AtomicsOverhead.Nanoseconds()),
		utils.Bool("headless", caps.IsHeadless),
		utils.Bool("gpu", caps.HasGpu),
		utils.Bool("simd", caps.HasSimd),
	)

	return caps
}

// measureCompute runs a Sieve of Eratosthenes benchmark
func (p *Profiler) measureCompute() float64 {
	start := time.Now()
	// Run primes up to 100,000 to gauge integer performance
	count := 0
	n := 100000
	isPrime := make([]bool, n+1)
	for i := 2; i <= n; i++ {
		isPrime[i] = true
	}
	for p := 2; p*p <= n; p++ {
		if isPrime[p] {
			for i := p * p; i <= n; i += p {
				isPrime[i] = false
			}
		}
	}
	for i := 2; i <= n; i++ {
		if isPrime[i] {
			count++
		}
	}
	duration := time.Since(start)

	// Normalize: Lower duration is better. Arbitrary baseline: 10ms = 1.0
	// If it takes 20ms, score is 0.5. If 5ms, score is 2.0.
	baseline := 10 * time.Millisecond
	score := float64(baseline) / float64(duration)
	return score
}

// measureAtomics measures the overhead of syscall/js calls (proxy for Atomics)
func (p *Profiler) measureAtomics() time.Duration {
	// We measure the cost of a simple JS call roundtrip
	// This proxies the "overhead" of talking to the main thread via SAB/Atomics
	start := time.Now()
	iterations := 1000
	global := js.Global()
	for i := 0; i < iterations; i++ {
		_ = global.Get("undefined")
	}
	total := time.Since(start)
	return total / time.Duration(iterations)
}

func (p *Profiler) detectHeadless() bool {
	navigator := js.Global().Get("navigator")
	if !navigator.Truthy() {
		return true // Likely non-browser WASM host
	}
	userAgent := navigator.Get("userAgent").String()
	webdriver := navigator.Get("webdriver")

	return webdriver.Truthy() || userAgent == ""
}

func (p *Profiler) detectGpu() bool {
	navigator := js.Global().Get("navigator")
	if !navigator.Truthy() {
		return false
	}
	gpu := navigator.Get("gpu")
	return gpu.Truthy()
}

func (p *Profiler) detectSimd() bool {
	// Simple SIMD detection: check if WebAssembly.validate with a SIMD instruction works
	// Or check the 'features' in the browser if available.
	// For now, a simple heuristic or checking for a known SIMD global is enough.
	// Most modern browsers supporting SharedArrayBuffer also support SIMD.
	wasm := js.Global().Get("WebAssembly")
	if !wasm.Truthy() {
		return false
	}
	// We'll assume SIMD is present if WebAssembly is present and we're in a modern enough environment
	// (Actual validation would require a small WASM binary with SIMD opcodes)
	return true
}
