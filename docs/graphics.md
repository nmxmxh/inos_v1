# INOS Graphics & Animation Methodology

This document outlines the architectural strategy used to achieve industrial-grade animation performance (e.g., 10,000+ entities @ 60fps) within the INOS distributed runtime.

---

## Core Pillars

### 1. Zero-Copy Shared Memory (SAB)
The primary bottleneck in web-based compute is data serialization/deserialization. INOS bypasses this entirely using `SharedArrayBuffer` (SAB).
- **One Truth**: Data exists in a fixed memory location.
- **Concurrent Access**: Go (Logic), Rust (Math), and JS (Rendering) all point to the same byte offsets.
- **No Transfers**: We never "send" data between layers; we only signal that data has changed via an **Epoch Counter**.

### 2. Signal-Based Architecture (Zero-CPU Blocking)
To eliminate polling overhead entirely:
- **Atomics.wait**: Components block on epoch changes using `Atomics.wait()` instead of `setInterval`.
- **Zero-CPU Idle**: When no activity occurs, CPU usage is 0% (no spinning).
- **Instant Wake**: When an epoch changes, waiting threads wake immediately.

```
┌────────────────────────────────────────────────────┐
│         SIGNAL-BASED DISCOVERY LOOP                │
├────────────────────────────────────────────────────┤
│                                                    │
│  Rust WASM                         Go Kernel       │
│  ┌──────────────┐    Atomics.notify  ┌──────────┐  │
│  │ Module       │  ─────────────────► │ Discovery│  │
│  │ Registration │     (epoch++)      │ Loop     │  │
│  │              │                    │ BLOCKS   │  │
│  └──────────────┘                    └──────────┘  │
│                                                    │
│  Zero-CPU while idle. Instant response on signal. │
└────────────────────────────────────────────────────┘
```

**Key Epoch Indices (from `layout.rs`):**
| Index | Name | Purpose |
|-------|------|---------|
| 12 | `IDX_BIRD_EPOCH` | Bird physics complete |
| 13 | `IDX_MATRIX_EPOCH` | Matrix generation complete |
| 14 | `IDX_PINGPONG_ACTIVE` | Active buffer selector |
| 15 | `IDX_REGISTRY_EPOCH` | Module registration signal |
| 16 | `IDX_EVOLUTION_EPOCH` | Boids evolution complete |
| 19 | `IDX_ECONOMY_EPOCH` | Credit settlement needed |

### 3. Ping-Pong Buffers (Zero Contention)
To prevent read/write conflicts between layers:
- **Dual Buffers**: Physics writes to Buffer A while rendering reads Buffer B.
- **Epoch Flip**: On physics complete, increment epoch. Buffer selection = `epoch % 2`.
- **Lock-Free**: No mutexes, no blocking, no torn reads.

```
┌─────────────────────────────────────────────────────┐
│              PING-PONG BUFFER ARCHITECTURE           │
├─────────────────────────────────────────────────────┤
│                                                      │
│   Frame N (epoch=100):          Frame N+1 (epoch=101):
│   ┌─────────┐  ┌─────────┐      ┌─────────┐  ┌─────────┐
│   │Buffer A │  │Buffer B │      │Buffer A │  │Buffer B │
│   │ WRITE   │  │  READ   │ ──►  │  READ   │  │ WRITE   │
│   │(physics)│  │(render) │      │(render) │  │(physics)│
│   └─────────┘  └─────────┘      └─────────┘  └─────────┘
│                                                      │
│   isBufferA = (matrixEpoch % 2 === 0)               │
└─────────────────────────────────────────────────────┘
```

**Buffer Layout (from `layout.rs`):**
| Buffer | Offset | Size | Purpose |
|--------|--------|------|---------|
| Bird A | `0x162000` | 2.36MB | Population state (10k × 236B) |
| Bird B | `0x3C2000` | 2.36MB | Population state |
| Matrix A | `0x622000` | 5.12MB | Instance matrices (10k × 8 × 64B) |
| Matrix B | `0xB22000` | 5.12MB | Instance matrices |

### 4. Persistent Scratch Buffers (Zero Allocation)
To prevent GC pressure and frame stutters:
- **Once-allocated**: Scratch buffers are allocated once at module init.
- **Mutex-protected**: A `Mutex<PersistentScratch>` ensures thread safety without heap churn.
- **Reused per-frame**: Same memory is reused for every physics step.

```rust
static SCRATCH: Lazy<Mutex<PersistentScratch>> = Lazy::new(|| {
    Mutex::new(PersistentScratch {
        population: vec![0u8; MAX_BIRDS * BIRD_STRIDE],
        positions: vec![[0.0, 0.0, 0.0]; MAX_BIRDS],
        velocities: vec![[0.0, 0.0, 0.0]; MAX_BIRDS],
        grid: SpatialHashGrid::new(CELL_SIZE, GRID_SIZE),
        neighbor_cache: vec![],
    })
});
```

### 5. FFI Bulk I/O
Inter-language calls (FFI) are expensive.
- **Bulk Transfers**: Instead of per-float calls, read/write entire buffers at once.
- **The 3000x Win**: Refactoring from per-float updates to `read_raw`/`write_raw` reduced FFI overhead by 3 orders of magnitude.

### 6. Instanced Rendering
To minimize draw calls on the GPU:
- **Instanced Mesh**: A single geometry is sent to the GPU once.
- **Matrix Injection**: GPU reads 4x4 transforms for all entities from SAB-backed attributes.
- **8-Part Birds**: Each bird = 8 meshes (body, head, beak, wings, tail). 1000 birds = 8000 instances.

### 7. Memory Barrier & View Persistence
To prevent "Death by Garbage Collection":
- **Cached Views**: All `TypedArray` views are cached at initialization.
- **No Per-Frame Allocations**: The animation loop creates zero new objects.

### 8. Context Versioning (Zombie Killing)
To ensure old code doesn't haunt the new context:
- **Global Context ID**: `window.__INOS_CONTEXT_ID__` incremented on every boot.
- **Self-Destruct**: All loops check if their local ID matches global. If not, they exit.

---

## GPU-Ready Matrix Generation

### The Optimal Architecture

```
┌─────────────────────────────────────────────────────┐
│                MATRIX PIPELINE                       │
├─────────────────────────────────────────────────────┤
│                                                      │
│   Rust BoidUnit        Rust MathUnit        Three.js │
│  ┌──────────────┐     ┌──────────────┐     ┌───────┐│
│  │ Physics      │ SAB │ Matrix Gen   │ SAB │ Copy  ││
│  │ step_physics │────►│ 8 parts ×    │────►│ to    ││
│  │              │     │ 4x4 matrices │     │ GPU   ││
│  └──────────────┘     └──────────────┘     └───────┘│
│      ~2ms                  ~1ms              ~0.2ms  │
└─────────────────────────────────────────────────────┘
```

**In JavaScript (zero math):**
```typescript
useFrame(() => {
  dispatch.execute('boids', 'step_physics', { bird_count: 1000, dt: delta });
  dispatch.execute('math', 'compute_instance_matrices', { count: 1000 });
  
  // Just copy – no computation
  const matrixBase = (matrixEpoch % 2 === 0) ? 0x622000 : 0xB22000;
  const sabView = new Float32Array(sab, matrixBase, 1000 * 16);
  bodiesRef.current.instanceMatrix.array.set(sabView);
  bodiesRef.current.instanceMatrix.needsUpdate = true;
});
```

---

## Optimization Opportunities

### 1. GPU-Driven Rendering Pipeline
**Current**: CPU generates matrices → GPU instances  
**Opportunity**: Move matrix generation to compute shaders

- Store raw boid data (position, velocity, orientation) in SAB-backed texture
- Compute shader reads directly from SAB via `WEBGL_multi_draw`
- Eliminate CPU→GPU matrix copy entirely
- **Potential**: 100% CPU-offload for >10k entities

### 2. Hierarchical Epoch System
**Current**: Flat epoch indices  
**Opportunity**: Tree-structured dependency graph

```
IDX_PHYSICS_READY → triggers → IDX_MATRIX_READY → triggers → IDX_RENDER_READY
```

- Enables partial updates (only changed entities)
- Automatic dependency resolution
- Debug visualization of pipeline bottlenecks

### 3. Structure-of-Arrays (SoA) for SIMD
**Current**: Array of Structs (AoS)  
**Opportunity**: SoA for 4x SIMD speedup

```rust
// Current: Array of Structs
struct Bird { pos: [f32;3], vel: [f32;3], ... }

// Proposed: Struct of Arrays
struct Birds {
  positions_x: [f32; MAX_BIRDS],
  positions_y: [f32; MAX_BIRDS],
  positions_z: [f32; MAX_BIRDS],
}
```

- Enables SIMD vectorization in Rust
- Better cache locality for matrix generation
- GPU-friendly for compute shaders

### 4. WebGPU Migration Path
**Current**: Three.js with WebGL  
**Opportunity**: Hybrid WebGPU/WebGL with graceful degradation

```typescript
class RenderBackend {
  async init() {
    if ('gpu' in navigator) {
      // WebGPU: Buffer-to-texture direct from SAB
      this.gpu = await this.initWebGPU();
    } else {
      // WebGL: Current instanced rendering
      this.gl = await this.initWebGL();
    }
  }
}
```

- WebGPU allows SAB direct mapping via `GPUBuffer` `MAP_WRITE`
- Compute shader boids entirely on GPU
- Maintain current WebGL as fallback

### 5. Temporal Coherence Optimization
**Current**: Full matrix generation every frame  
**Opportunity**: Exploit temporal coherence

- Store previous frame matrices
- Compute delta transforms (only update changed > threshold)
- Use `transformFeedback` for GPU-side interpolation
- **Potential**: 70-90% reduction in matrix updates for cohesive flocks

### 6. Spatial Culling Pipeline
**Opportunity**: Add culling as a Rust compute stage

- Generate visibility bitmask in scratch buffer
- Compact visible instances via prefix sum
- Render only visible entities
- Shares spatial grid with boids physics

### 7. Level-of-Detail (LOD) System
**Opportunity**: Multi-resolution rendering

- 3 LODs per bird mesh (high/med/low)
- Distance-based LOD selection in matrix generation
- Billboard sprites for distant birds
- **Potential**: 100k "birds" with same GPU budget

### 8. Predictive Buffer Warming
**Opportunity**: Use physics velocity to pre-warm buffers

- Predict next frame positions
- Pre-generate matrices in idle frames
- Double-buffer prediction to hide latency
- Especially valuable for VR/120Hz displays

### 9. Audio Integration
**Opportunity**: Synchronized audio rendering

- Extend SAB layout with audio buffers
- Web Audio API reads directly from shared memory
- Perfect audio-visual sync (critical for VR)
- Spatial audio based on boid positions

---

## Implementation Priority

### Immediate (1-2 weeks)
1. [x] Signal-based discovery loop (completed)
2. [ ] SoA memory layout for SIMD
3. [ ] Spatial culling pipeline

### Medium-term (1-2 months)
4. [ ] WebGPU backend with fallback
5. [ ] Temporal coherence optimization
6. [ ] Multi-LOD system

### Long-term (3-6 months)
7. [ ] GPU-driven pipeline
8. [ ] Distributed rendering
9. [ ] Predictive buffering

---

## Performance Metrics

| Stage | Target | Current | Optimized |
|-------|--------|---------|-----------|
| Physics (1k birds) | <2ms | ~1.5ms | <0.5ms (SIMD) |
| Matrix Gen (1k birds) | <1ms | ~0.8ms | <0.1ms (GPU) |
| GPU Upload | <0.5ms | ~0.3ms | 0ms (WebGPU) |
| **Total Frame** | <4ms | ~2.6ms | <1ms |

---

## Reference Implementation

| Component | File |
|-----------|------|
| Kernel Bridge | `kernel/threads/supervisor/sab_bridge.go` |
| SDK Memory | `modules/sdk/src/sab.rs` |
| Ping-Pong Buffers | `modules/sdk/src/pingpong.rs` |
| Layout Constants | `modules/sdk/src/layout.rs` |
| Boids Physics | `modules/compute/src/units/boids.rs` |
| Matrix Computation | `modules/compute/src/units/math.rs` |
| Signal Epochs | `kernel/threads/sab/layout.go` |
| Singleton Kernel | `frontend/src/wasm/kernel.ts` |
| Instanced Renderer | `frontend/app/components/ArchitecturalBoids.tsx` |
| SAB Layout Spec | `sab_layout.md` |

---

## Performance Checklist

- [ ] **Singleton**: Is the Kernel memory guarded by a global singleton check?
- [ ] **Linear Memory Base**: Is Go using address 0 (not heap allocation)?
- [ ] **Signal-Based**: Are loops using `Atomics.wait` instead of polling?
- [ ] **Ping-Pong**: Are buffers using epoch-based selection (no locks)?
- [ ] **Persistent Scratch**: Are Rust modules using `Lazy<Mutex<Scratch>>`?
- [ ] **Disposal**: Are all 3D resources explicitly disposed on unmount?
- [ ] **Zero Allocations**: Is JS Heap rate 0KB/sec during animation?
- [ ] **FFI Bulk I/O**: Are SAB ops using `read_raw`/`write_raw`?
- [ ] **Matrix Source**: Are matrices generated in Rust for >100 entities?
- [ ] **SoA Layout**: Is entity data arranged for SIMD (future)?
- [ ] **WebGPU Ready**: Is render backend abstracted for migration?
