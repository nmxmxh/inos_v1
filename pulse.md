# INOS Autonomous System Pulse: Architecture & Specification

## 1. The Vision: Zero-Copy, Zero-Jank, Zero-rAF
Conventional web applications use `requestAnimationFrame` (rAF) on the main thread to drive animation and logic. This forces the browser into a rendering pipeline (style, layout, paint) even when no visual changes occur, and introduces significant microtask overhead.

INOS v2.0 eliminates rAF as a compute driver. Instead, it uses a **Hybrid Event-Driven Pulse** where workers are autonomous, self-timed, and synchronized via data-flow dependencies in the SharedArrayBuffer.

## 2. Core Principles
- **Decoupled Cycles**: Compute cycles (Physics/Math) run at their natural frequency, independent of the display refresh rate.
- **Atomic Parking**: Workers use `memory.atomic.wait32` to "park" their threads, consuming zero CPU until data is ready or a pulse occurs.
- **Significance-Based Scheduling**: Updates are throttled based on `IDX_SYSTEM_VISIBILITY` and distance-based LOD (Level of Detail).
- **GPU-Driven Data Flow**: Boid matrices and vertex data flow directly from Rust-managed SAB regions into WebGPU storage buffers without JS-side copying.

## 3. Signal Infrastructure (`sab_layout.capnp`)

The atomic flags region (expanded to 1KB) acts as the system's "Nervous System":

### 3.1 System Heartbeats
- `IDX_SYSTEM_PULSE`: A monotonic heartbeat (optional fallback).
- `IDX_SYSTEM_VISIBILITY`: 1 = Visible (High Perf), 0 = Hidden (Low Power/Deep Sleep).
- `IDX_SYSTEM_POWER_STATE`: Dynamic throttle control (Throttled, Balanced, Unlocked).

### 3.2 Worker Lifecycle (SAB-Native)
Each worker/capsule monitors its own control slots:
- `IDX_WORKER_PAUSE`: Managed by the Orchestrator to freeze execution.
- `IDX_WORKER_KILL`: Signal for graceful or forced termination.
- `IDX_WORKER_ACK`: The worker increments this when a work unit is complete, waking downstream dependencies.

## 4. Execution Flow: The Multi-Worker Pulse
The timing authority is moved to a dedicated **Pulse Worker**, freeing the **Compute Worker** for heavy-lifting.

```mermaid
graph LR
    A[Pulse Worker] -- IDX_SYSTEM_PULSE --> B[Compute Worker]
    A -- IDX_SYSTEM_VISIBILITY --> B
    C[Go Kernel] -- LifecycleCmd --> A
    B -- IDX_WORKER_ACK --> D[Renderer]
```

### 4.1 Pulse Worker (Orchestrator)
- Runs a high-precision `self.performance.now()` loop.
- Manages the **System Heartbeat**.
- Propagates visibility states (Visible/Hidden) to the signal bus.

### 4.2 Compute Worker (Autonomous)
- Blocks on `Atomics.wait(IDX_SYSTEM_PULSE)`.
- Executes Unit logic in its natural cadence.
- Increments `IDX_WORKER_ACK` for consumers.

## 6. Migration Roadmap
1. **Phase 1**: Implement `pulse.worker.ts` as the standalone timing authority.
2. **Phase 2**: Implement `AutonomousWorker` trait in Rust.
3. **Phase 3**: Establish the Hierarchical Epoch DAG for event-driven wakeups.
4. **Phase 4**: Full cutover to WebGPU-SAB aliasing.
