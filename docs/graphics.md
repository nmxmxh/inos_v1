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
1. **Sub-views**: `new Float32Array(buffer, offset, count)` — creates a new JS object.
2. **Object Spread**: `{ ...position }` — creates a new literal.
3. **Array Methods**: `.map()`, `.filter()` on every frame — creates new arrays.
4. **Three.js Constructors**: `new THREE.Color()`, `new THREE.Matrix4()` — expensive allocations.
5. **Dangling Intervals**: Starting `setInterval` in a store without a global "active" flag or cleanup.

---

## 10. GPU-Ready Matrix Generation

### The Problem: JavaScript as a Bottleneck

Even with zero-copy SAB architecture, a subtle performance issue can arise:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    CURRENT ARCHITECTURE (Suboptimal)                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   Rust (WASM)              JavaScript (CPU)           GPU (WebGL)   │
│  ┌──────────────┐         ┌──────────────────┐       ┌───────────┐  │
│  │ Physics      │  SAB    │ Matrix Math      │  API  │ Render    │  │
│  │ calculate    │ ─────►  │ 8000 multiplies  │ ───►  │ instances │  │
│  │ positions    │         │ per frame!       │       │           │  │
│  └──────────────┘         └──────────────────┘       └───────────┘  │
│        Fast                     SLOW                     Fast       │
└─────────────────────────────────────────────────────────────────────┘
```

**Why this happens:**
- Rust computes **bird state**: position (x, y, z), velocity, rotation
- JavaScript reads state and creates **render matrices**: 4x4 transforms for each mesh part
- With 1000 birds × 8 mesh parts = **8000 matrix multiplications in JavaScript per frame**
- JavaScript matrix math runs on **CPU cores**, not GPU

### The Solution: Rust-Generated Render Matrices

```
┌─────────────────────────────────────────────────────────────────────┐
│                    OPTIMAL ARCHITECTURE                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   Rust (WASM)                   JavaScript           GPU (WebGL)    │
│  ┌────────────────────────┐    ┌────────────┐       ┌───────────┐   │
│  │ Physics + Matrix Gen   │    │ Copy SAB   │       │ Render    │   │
│  │ ───────────────────    │    │ to GPU     │       │ instances │   │
│  │ positions, rotations,  │SAB │ (zero math)│ API   │           │   │
│  │ AND 4x4 matrices       │───►│            │ ───►  │           │   │
│  └────────────────────────┘    └────────────┘       └───────────┘   │
│          Fast                    Minimal                Fast        │
└─────────────────────────────────────────────────────────────────────┘
```

**Benefits:**
1. **CPU Offload**: JavaScript becomes a thin data-copy layer
2. **Consistent Performance**: Rust WASM is 10-100x faster than JS for matrix math
3. **True Zero-Copy**: Matrices flow directly from SAB → GPU with no JavaScript computation
4. **Unified Compute**: All math happens in one place (Rust), easier to optimize

### Implementation Pattern

**In Rust (`boids.rs`):**
```rust
// Per-bird: 58 floats → expand to include 4x4 matrices
// New layout: position(3) + velocity(3) + rotation(4) + ... + body_matrix(16) + head_matrix(16) + ...

fn compute_render_matrix(pos: [f32; 3], rot: [f32; 4], part_offset: [f32; 3]) -> [f32; 16] {
    // Compute full 4x4 transform matrix in Rust
    // Write directly to SAB
}
```

**In JavaScript (`ArchitecturalBoids.tsx`):**
```typescript
useFrame(() => {
  // No matrix math! Just copy:
  const matrices = new Float32Array(sab, MATRIX_OFFSET, BIRD_COUNT * 16);
  bodiesRef.current.instanceMatrix.array.set(matrices);
  bodiesRef.current.instanceMatrix.needsUpdate = true;
});
```

### When to Use This Pattern

| Scenario | Recommended Approach |
|----------|---------------------|
| < 100 entities | JavaScript matrix math is fine |
| 100-1000 entities | Consider Rust matrices |
| > 1000 entities | **Must** use Rust matrices |
| Complex per-entity transforms | **Must** use Rust matrices |

---

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
- [ ] **Matrix Source**: Are matrices generated in Rust when entity count > 100?
