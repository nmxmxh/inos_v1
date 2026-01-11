# INOS CORE ARCHITECTURE: MASTER BLUEPRINT (POST-OPTIMIZATION)

## 1. Architectural Foundation: Digital Twin & Absolute Memory
The system is unified on a **0-based Absolute Addressing Model**. Legacy relative offsets (`__INOS_SAB_OFFSET__`) are deprecated.

- **SharedArrayBuffer (SAB)**: Initialized as a single linear memory space.
- **Memory Map**: Defined by `sab_layout.capnp`.
    - `0x0000`: Atomic Flags & Epochs.
    - `0x162000`: Bird Buffer A (Ping-Pong).
    - `0x3C2000`: Bird Buffer B (Ping-Pong).
    - `0x8C2000`: Matrix Buffer A (Ping-Pong).
    - `0xB22000`: Matrix Buffer B (Ping-Pong).
- **Go "Digital Twin"**: Go maintains local memory buffers and synchronizes with SAB using `ReadAt` and `WriteAt` with absolute offsets.
- **Linear Memory Pruning**: The 16MB ghost zone in `layout.ts` (`MEMORY_PAGES`) is removed to align JS WASM memory directly with the SAB start.

## 2. Physics & Compute: "Flock-One" Engine (Rust)

### Spatial Grid 2.0 (High-Speed Neighborhood Search)
- **Eliminate HashMap**: Replaced `HashMap<(i32,i32,i32), Vec<usize>>` with a fixed-size `Vec<Vec<usize>>` grid.
- **Grid Specs**: `GRID_DIM = 40`, `WORLD_SIZE = 200.0`. Total cells = 64,000.
- **Optimization**: Cells use `Vec::with_capacity(32)` to prevent allocation churn.
- **Search**: 3x3x3 search logic with boundary checks.

### Matrix Constant Caching (Math Unit)
- **MathScratch**: A persistent struct storing pre-calculated constant matrices for boid parts (Body rotation, Head translation, etc.).
- **Zero-Copy Generation**: Matrices are written directly into the inactive matrix buffer in the SAB block-by-block.

### Procedural Synchronization
- **Epoch Tracking**: `EPOCH_COUNTER.fetch_add(1)` in `boids.rs` provides a global physics time basis (`t = epoch * 0.1`) for "ocean tide" procedural motion.
- **Signal-Based**: `compute_poll` is removed. Steps are driven by explicit JS `dispatch.execute` calls.

## 3. Infrastructure & Safety (JS/TS)

### WasmHeap & Dispatcher GC
- **Interning**: Objects are deduplicated and reference-counted.
- **GC Interval**: 5000ms periodic cleanup or `MAX_CACHE_SIZE` (1000) trigger.
- **Dispatcher LRU**: Encoder string cache capped at 100 entries with LRU eviction.
- **Bridge Safety**: `viewCache` capped at 500 entries in `bridge.ts`.

### Zero-Copy Rendering (Pointer Swapping)
- **Arena Views**: `getArenaView` returns a direct `Float32Array` view of the SAB matrix blocks.
- **Swap Logic**: `instanceMatrix.array` is pointed directly to the active matrix buffer. `instanceMatrix.version++` triggers GPU upload.

## 4. The Blockers (Frame 1 Hang Diagnosis)
- **Starvation**: Go Supervisor waking at 60Hz causes JS event loop starvation (cannot schedule Frame 2).
- **Deadlock**: Concurrent SAB access between Go `ReadAt` and JS `useFrame` during high-frequency flips.

---

## SURGICAL RE-IMPLEMENTATION PLAN (Post-Reset)

### Step 1: Memory & Infrastructure (Ground Layer)
- [ ] Apply 0-based addressing in `kernel.ts` and `layout.ts`.
- [ ] Update `bridge.go` to support size-only initialization (`initializeSharedMemory(size)`).
- [ ] Add `WasmHeap` and `Dispatcher` GC/Caching logic.

### Step 2: High-Performance Physics (Rust Layer)
- [ ] Re-introduce Spatial Grid 2.0 in `boids.rs`.
- [ ] Re-introduce `MathScratch` and pre-calculated matrices in `math.rs`.
- [ ] Ensure `EPOCH_COUNTER` is active for procedural movement.

### Step 3: Calibrated Supervisor (Go Layer)
- [ ] Set `EvolutionInterval = 3 * time.Second`.
- [ ] Restore correct Ping-Pong selection logic:
    - **ReadPopulation**: If `active=0` (Rust writing B), Go reads B (stable).
    - **WritePopulation**: If `active=0` (Rust reading A), Go writes A (stable).
- [ ] **STRICT**: Continue using a periodic ticker for evolution, NOT the physics epoch, to avoid main-thread starvation.

### Step 4: Graphics & Sync (UI Layer)
- [ ] Restore 8-part rendering in `InstancedBoidsRenderer.tsx`.
- [ ] Use copy-based rendering initially: `instanceMatrix.array.set(newData)`.
- [ ] Move to Pointer Swapping *only* if liveness is confirmed at 2s+ evolution cadence.
