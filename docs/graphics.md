# INOS Graphics & Animation Methodology

This document outlines the architectural strategy used to achieve industrial-grade animation performance (e.g., 1500+ entities @ 60fps) within the INOS distributed runtime.

## Core Pillars

### 1. Zero-Copy Shared Memory (SAB)
The primary bottleneck in web-based compute is data serialization/deserialization. INOS bypasses this entirely using `SharedArrayBuffer` (SAB).
- **One Truth**: Data exists in a fixed memory location.
- **Concurrent Access**: Go (Logic), Rust (Math), and JS (Rendering) all point to the same byte offsets.
- **No Transfers**: We never "send" data between layers; we only signal that data has changed via an **Epoch Counter**.

### 2. Side-Channel Signaling (The Pulse)
To avoid the overhead of constant event listeners:
- Components poll a single **Atomic Flag** (the System Epoch) in the SAB.
- Only when this flag changes does the renderer update its buffers.
- This creates a de-coupled "Heartbeat" where compute can run faster or slower than rendering without causing frames to drop.

### 3. FFI Bulk I/O
Inter-language calls (FFI) are expensive.
- ** methodology**: Instead of calling a function for every entity or every property, we perform **Bulk Transfers**.
- **The 3000x Win**: Refactoring from per-float updates to a single `read_all` and `write_all` call reduced our FFI overhead by 3 orders of magnitude.

### 4. Instanced Rendering
To minimize draw calls on the GPU:
- We use **Instanced Mesh** technology.
- A single "Geometry" (the bird) is sent to the GPU once.
- The GPU then reads the positions, rotations, and animation states for all 1500 birds directly from the SAB-backed attributes in a single pass.

### 5. Memory Barrier & View Persistence
To prevent "Death by Garbage Collection" (GC):
- **WASM-JS Bridge**: Many bridges create new objects (Buffer views, arrays) on every frame.
- **The Finding**: Creating 1500 `new Float32Array(sab, offset, length)` views per frame generates **90,000+ objects/sec**, causing a steady JS heap climb and eventual frame stutters.
- **The Solution**: Cache these views at initialization using `useRef` (React) or persistent variables. Re-use a single large `TypedArray` and index into it mathematically.

### 6. Atomic Initialization Pattern
To prevent race conditions and redundant resource spawns:
- **Problem**: Concurrent component mounts or rapid hot-reloads can trigger `initializeKernel` multiple times before the first call settles, resulting in multiple `SharedArrayBuffer` instances and WASM processes.
- **Solution**: Use an "Atomic Promise" lock stored globally (e.g., `window.__INOS_INIT_PROMISE__`). If a promise already exists, return it instead of spawning a new process.
- **Implementation**: See [kernel.ts](file:///Users/okhai/Desktop/OVASABI%20STUDIOS/inos_v1/frontend/src/wasm/kernel.ts).

### 8. Context Versioning (Zombie Killing)
To ensure old code doesn't haunt the new context:
- **Problem**: Hot-reloading replaces the JS bundle but doesn't necessarily stop old intervals or RAF loops from previous "Windows".
- **Solution**: Increment a global `window.__INOS_CONTEXT_ID__` on every boot. All loops must check if their local ID matches the global ID. If not, they self-destruct.
- **Cross-Module Enforcement**: Rust modules must also capture the context ID during `init_with_sab` and check it in hot entry points (e.g., `compute_poll`). Use `sdk::is_context_valid()`.

### 9. Shutdown & Lifecycle Management
To prevent "Orphaned Goroutines" in detached contexts:
- **Problem**: Go goroutines (like the boids evolution loop) do not automatically stop when a React component unmounts.
- **Solution**: Implement a `shutdownKernel()` signal that stores a shutdown flag in the SAB. The Go process should poll this flag and exit cleanly.

### 9. Explicit Resource Disposal
To prevent "Detached Node" leaks:
- **Problem**: 3D geometries and materials are not always automatically garbage collected if held in `useMemo`.
- **Solution**: Use `useEffect` cleanup to call `.dispose()` on all geometries and materials.

## Memory Leak Antipatterns
Avoid these common pitfalls in the `useFrame` or `requestAnimationFrame` hooks:
1. **Sub-vies**: `new Float32Array(buffer, offset, count)` — creates a new JS object.
2. **Object Spread**: `{ ...position }` — creates a new literal.
3. **Array Methods**: `.map()`, `.filter()` on every frame — creates new arrays.
4. **Three.js Constructors**: `new THREE.Color()`, `new THREE.Matrix4()` — expensive allocations.
5. **Dangling Intervals**: Starting `setInterval` in a store without a global "active" flag or cleanup.

## Reference Implementation
- **Kernel Bridge**: `kernel/threads/supervisor/sab_bridge.go`
- **SDK Memory Management**: `modules/sdk/src/sab.rs`
- **Simulation Loop**: `modules/compute/src/units/boids.rs`
- **Singleton Kernel**: `frontend/src/wasm/kernel.ts`
- **Zero-Allocation Frontend**: `frontend/app/components/ArchitecturalBoids.tsx`

## Performance Checklist
- [ ] **Singleton**: Is the Kernel memory guarded by a global singleton check?
- [ ] **Disposal**: Are all 3D geometries and materials explicitly disposed on unmount?
- [ ] **Allocations**: Is the JS Heap allocation rate 0KB/sec during active animation?
- [ ] **Persistence**: Are all `TypedArray` views and `Matrix` scratchpads pre-allocated?
- [ ] **Instancing**: Is the GPU using instancing to handle Entity counts > 100?
- [ ] **FFI**: Are we calling WASM functions for individual entities? (Antipattern).
