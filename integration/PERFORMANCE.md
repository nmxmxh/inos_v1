# INOS Performance Architecture

**Last Validated**: 2026-01-19  
**Test Environment**: MacBook Pro M1, 16GB RAM  
**Kernel Version**: 2.5 (Zero-Polling) | **Test Suite**: 118 E2E tests (Playwright)

---

## The Story of Zero-Copy: How INOS Achieves 10-100x Performance

INOS is not a framework. It is a **nervous system**—a tri-layer architecture where Go, Rust, and TypeScript share a single memory buffer and react to atomic signals in microseconds. This document explains *why* this architecture is fast and *how* we validate it through continuous E2E testing.

The core insight is simple: **the fastest data transfer is no transfer at all**. By eliminating serialization, copying, and message-passing between components, INOS achieves performance that rivals native applications while running entirely in the browser.

Following the **Phase 4 Convergence**, we have eliminated the final 50ms polling bottleneck in the Go Bridge, moving to a pure signal-driven architecture. This document provides the high-fidelity metrics validating this 100x leap in responsiveness.

---

## Chapter 1: The SharedArrayBuffer Foundation

Every INOS kernel allocates a **64MB SharedArrayBuffer (SAB)** at boot. This buffer is the single source of truth—Go orchestrates policy in it, Rust computes physics into it, and TypeScript reads from it to render at 60 FPS. No component ever copies data to another; they share the same memory.

### Memory Layout

The SAB is divided into fixed regions, each with a specific purpose:

| Region | Offset | Size | Purpose |
|--------|--------|------|---------|
| **Atomic Flags** | `0x000000` | 128B | Epoch counters, mutexes, system signals |
| **Module Registry** | `0x000140` | 6KB | 64 registered WASM modules |
| **Supervisor Headers** | `0x002000` | 4KB | 32 supervisor thread metadata |
| **Economics + Identity** | `0x004000` | 32KB | Credit balances, DIDs, reputation |
| **Mesh Metrics** | `0x004100` | 4KB | Peer count, latency, throughput |
| **Pattern Exchange** | `0x010000` | 64KB | Evolved behavioral patterns |
| **Inbox / Outbox** | `0x050000` | 1MB | Job queues (ring buffers) |
| **Dynamic Arena** | `0x150000` | ~30MB | Boids, matrices, compute scratch |

### Native Memory Throughput (Rust SDK)

We measure the raw cost of writing to the SAB from the WASM environment using the `sdk` crate.

| Metric | Result | Target | Status |
|--------|--------|--------|--------|
| **SAB Write Latency** | **18ns/op** | <25ns | ✅ EXCELLENT |
| **SAB Read Latency** | **18ns/op** | <25ns | ✅ EXCELLENT |
| **Total Bandwidth** | **21.38 GB/s** | >10 GB/s | ✅ EXCEEDED |
| **Concurrent Safe Access** | **79.8M ops/sec** | >50M | ✅ EXCEEDED |

---

## Chapter 2: The Zero-Copy Pipeline

Traditional web applications pay a **"copy tax"** every time data moves between components. When Module A sends data to Module B:

1. Module A serializes data to JSON/Protobuf
2. The runtime copies the serialized bytes
3. Module B deserializes into its own memory

For a physics simulation with 10,000 entities at 60 FPS, this copying alone can exceed the 16ms frame budget. INOS eliminates these steps:

```
Network → SAB (Inbox) → Rust (Process) → SAB (Outbox) → JS (Render)
```

At every stage, components read and write directly to SAB memory. The "transfer" is a pointer offset, completed in nanoseconds.

### Test Validation: Zero-Copy Performance (E2E)

```
Test: Zero-Copy Performance: SAB vs. StructuredClone Benchmark
Source: deep_validation.spec.js:40
```

| Browser | Legacy (structuredClone 5MB) | INOS (SAB Direct) | Speedup Ratio |
|---------|-----------------------------|-------------------|---------------|
| **Chromium** | 29.83ms | **0.69ms** | **43.2x** |
| **Firefox** | 12.30ms | **0.98ms** | **12.6x** |

> **Architectural Insight**: Chromium's speedup is higher because its `structuredClone` incurs more overhead for complex objects. INOS provides a deterministic ~0.7ms baseline across all engines, effectively "flattening" the browser performance curve.

---

## Chapter 3: The Zero-Polling Revolution

Previously, the Go-to-JS bridge relied on a 50ms polling loop for worker-thread synchronization. We have replaced this with **Zero-Polling Epoch Signaling** using `Atomics.wait` and `Atomics.notify`. In this model, threads "sleep" at the hardware level and are woken up instantly by memory signals.

### Cross-Layer Signaling Latency

| Layer Transition | Latency | Mechanism |
|------------------|---------|-----------|
| **Rust → SAB** | **9ns** | `atomic_store` + `notify` |
| **Go → SAB** | **155ns** | `Atomics.notify` (Bridge) |
| **JS → Go (Worker)** | **0.0005ms** | `Atomics.wait` Wakeup |

### Test Validation: Signaling Efficiency

```
Test: Signaling Efficiency: Atomics vs. setInterval Latency
Source: deep_validation.spec.js:88
```

| Method | Latency (20 iterations) | Improvement |
|--------|--------------------------|-------------|
| **Polling (setInterval 4ms)** | 849.48ms | Baseline |
| **Epoch Signal (Zero-Polling)** | **0.007ms** | **121,354x** |

### Signaling Jitter & Consistency

Consistency matters as much as speed. We measure jitter (standard deviation) to ensure signal stability:

| Browser | Avg Latency | Jitter (StdDev) | Max Latency |
|---------|-------------|-----------------|-------------|
| **Chromium** | 0.0002ms | 0.0010ms | 0.0050ms |
| **Firefox** | 0.0004ms | 0.0033ms | 0.0400ms |

---

## Chapter 4: The Economic Engine

The INOS economy runs on a high-frequency ledger implemented directly in the SAB's atomic region. It tracks credits, escrow, and reputation without ever locking the main thread.

### Economic Memory Layout

| Field | Offset from Economics Base | Size | Type |
|-------|---------------------------|------|------|
| Metadata | +0 | 64B | Header |
| Primary Balance | +64 | 8B | BigInt64 |
| Escrow | +72 | 8B | BigInt64 |
| Pending Rewards | +80 | 8B | BigInt64 |

### Settlement Performance (E2E Benchmarks)

```
Test: Escrow Signaling Latency: High-Resolution Measure
Source: economic_benchmarks.spec.js:23
```

| Browser | Avg. Settlement Latency | Throughput (reads/sec) |
|---------|-------------------------|------------------------|
| **Chromium** | **0.0005ms** | **5,714,281** |
| **Firefox** | **0.0040ms** | **1,666,667** |

### Atomic Balance Pulse: Loop Integrity

In 100% of stress tests (100 concurrent increments of 10 credits), the ledger maintains perfect consistency.
- **Expected final value**: 1000
- **Actual final value**: 1000 ✅
- **Integrity**: 100%

---

## Chapter 5: Kernel Throughput & Scale

The Go kernel orchestrates the entire mesh. With zero-polling, it can now handle jobs at native speeds without stalling worker execution.

### Kernel Job Throughput

```bash
# Internal Benchmark: TestUnifiedSupervisor_Throughput
Result: 78,400.63 jobs/second
```

This throughput allows a single INOS node to sustain over **6.7 million tasks per day** with zero idle CPU overhead.

### P2P Mesh Communication Patterns

| Pattern | Complexity | Latency (Avg) | Use Case |
|---------|------------|---------------|----------|
| **Local SAB** | O(1) | <1µs | Single-node compute |
| **Epoch Broadcast** | O(1) | <10µs | Local reactivity |
| **Gossip Propagate** | O(log n) | 12ms | State sync across mesh |
| **DHT Peer Discovery** | O(log n) | 142ms | Find content by hash |
| **WebRTC Negotiation** | O(1) | 5.4s - 21.5s | Handshake (local/WAN) |

---

## Chapter 6: Pipeline Saturation & Main Thread Protection

A critical design goal: **the main thread must never block.** Heavy compute runs in Web Workers, and the UI only reads completed results from SAB via `INOSBridge`.

### Concurrent Safety: Atomic CAS

```
Test: Atomic CAS Safety: Multi-Threaded Sync Simulation
Source: deep_validation.spec.js:179
```

| Metric | Target | Result |
|--------|--------|--------|
| **Concurrent Safe Operations** | 2,000 CAS | 2,000 ✅ |
| **Race Conditions** | 0 | 0 ✅ |

Lock-free `Atomics.compareExchange` ensures data integrity even under extreme concurrent access.

---

## Chapter 7: The Three-Layer Architecture

INOS uses three specialized language layers, each doing what it does best:

### Layer 1: The Host (TypeScript)
**Role**: Rendering, user interaction, DOM events. React components read SAB via `INOSBridge`.

### Layer 2: The Kernel (Go → WASM)
**Role**: Policy, scheduling, coordination. Supervisor threads manage module lifecycles and mesh synchronization.

### Layer 3: The Modules (Rust → WASM)
**Role**: High-performance compute. SIMD-accelerated physics, encryption, and compression modules operating directly on SAB.

### Inter-Layer Communication

```
┌─────────────────────────────────────────────────────────┐
│                    SharedArrayBuffer                    │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐            │
│  │  Atomics │   │   Jobs   │   │   Arena  │            │
│  │  (flags) │   │  (ring)  │   │  (data)  │            │
│  └────▲─────┘   └────▲─────┘   └────▲─────┘            │
│       │              │              │                   │
└───────┼──────────────┼──────────────┼───────────────────┘
        │              │              │
   ┌────┴────┐    ┌────┴────┐    ┌────┴────┐
   │   Go    │    │   Rust  │    │   TS    │
   │ Kernel  │──▶│ Modules │──▶│ Render  │
   └─────────┘    └─────────┘    └─────────┘
       ▲                              │
       └──────── Epoch Signal ────────┘
```

No function calls cross layer boundaries. All communication happens through SAB mutations and hardware-level signals.

---

## Chapter 8: Test Suite Overview

The INOS E2E test suite validates the entire architecture. 118 tests ensure that performance gains don't come at the cost of stability.

| Suite | Tests | Purpose |
|-------|-------|---------|
| `deep_validation.spec.js` | 10 | Core architecture (zero-copy, signaling, atomics) |
| `economic_benchmarks.spec.js` | 4 | Credit system stress testing |
| `mesh_network.spec.js` | 34 | P2P mesh operations |
| `mesh_telemetry.spec.js` | 5 | UI metrics and deep dives |
| `ml_pipeline.spec.js` | 4 | Compute job delegation |

---

## Chapter 9: Comparison with Traditional Architectures

| Metric | Traditional (Client-Server) | Traditional (postMessage) | INOS (Zero-Copy) |
|--------|----------------------------|----------------------------|-------------------|
| **Data Sync** | 50-200ms (Network) | 5-15ms (Copy Tax) | **<1ms (Signal)** |
| **Throughput** | ~50 MB/s | ~200 MB/s | **21+ GB/s** |
| **Scheduling** | Centralized | Event Loop | **Epoch Atomics** |
| **Idle Load** | Low | High (Polling) | **Zero (Atomic Wait)** |

---

## Chapter 10: Browser-Specific Characteristics

Different browser engines have different strengths. INOS provides a deterministic baseline across all of them.

### Chromium (v8 Engine)
- **Strength**: Lower signaling jitter (0.001ms).
- **Strength**: Higher SAB throughput (5.7M reads/sec).
- **Note**: Aggressively throttles background workers; `Atomics.wait` is critical to prevent starvation.

### Firefox (SpiderMonkey Engine)
- **Strength**: Faster WebRTC negotiation (~4x faster for local peers).
- **Strength**: Lower UI interaction latency (0.66ms vs Chrome's variability).
- **Note**: Slightly higher signaling jitter under heavy load.

---

## Appendix A: Running Benchmarks

```bash
# Full test suite (both browsers)
cd integration && npx playwright test --project=chromium --project=firefox

# Specific benchmark suite
npx playwright test deep_validation.spec.js --project=chromium
```

## Appendix B: Performance Thresholds

| Category | Metric | Achieved | Status |
|----------|--------|----------|--------|
| **Compute** | Zero-Copy Speedup | 43.2x | ✅ EXCEEDED |
| **Signaling** | Epoch Latency | 0.0005ms | ✅ EXCEEDED |
| **Economic** | Settlement Speed | 5.7M/s | ✅ EXCEEDED |
| **Mesh** | Job Throughput | 78k/s | ✅ EXCEEDED |

---

*This document is auto-generated from E2E test results and manually curated architectural descriptions.*
