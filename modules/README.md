# Layer 3: The Modules (Compute Capsules)

**Philosophy**: "Compute at the photon."
**Technology**: Rust (WASM) + WebGPU (WGPU) + Cap'n Proto.

This directory contains the "Muscle" of the system. Unlike the Kernel (written in Go), which manages orchestration and resources, these Modules (written in Rust) perform the actual heavy lifting. They are detachable, cached workloads that can run on any capable node in the mesh.

## 📂 The Capsules

| Module | Purpose | Key Libraries |
|--------|---------|---------------|
| `sdk` | **The Bridge**. Common utilities for SAB access, logging, and signaling. | `wasm-bindgen`, `js-sys`, `capnp` |
| `compute` | **General Purpose**. WebGPU accelerated compute shaders. | `wgpu` |
| `ml` | **Intelligence**. Distributed inference and training. | `burn`, `candle-core` |
| `mining` | **Economy**. Background Yield & Arbitrage. Mining Bitcoin/SHA-256 during idle cycles to subsidize node costs. | `sha2`, `blake3` |
| `physics` | **Simulation**. Deterministic rigid-body dynamics. | `rapier3d` |
| `storage` | **Data Backbone**. Compression (`brotli`) and CAS (`blake3`). | `brotli`, `blake3` |
| `drivers/` | **Hardware**. Hardware control via `web-sys`. | `web-sys` |

## 🏗 Architecture & Contracts

Modules are "Stateless Workers" but "Stateful Implementations". They adhere to two strict contracts:

1.  **The Work Contract (`compute/v1/capsule.capnp`)**:
    *   Accept a `JobRequest` (with Credit Budget).
    *   Read input from `SharedArrayBuffer` at `inputOffset`.
    *   Write output to `outputOffset`.
    *   Return a `JobResult` (with Cost used).

2.  **The Lifecycle Contract (`system/v1/orchestration.capnp`)**:
    *   Respond to `LifecycleCmd` (e.g., Pause/Resume).
    *   Emit `HealthHeartbeat` every 1s to the Kernel.

### The Reactive Loop (Generic Compute)

Modules utilize a standardized loop provided by `sdk` that ensures **Credit Economy Enforcement** and **Zero-Copy I/O**.

```rust
// modules/compute/src/lib.rs

#[wasm_bindgen]
pub fn start_worker(ctx: &IdentityContext) {
    loop {
        // 1. Efficient Wait
        if !sdk::signal::check_inbox() {
            sdk::thread::yield_now();
            continue;
        }

        // 2. Zero-Copy Read
        let job = sdk::inbox::read::<JobRequest>();

        // 3. Economic Enforcement (Added in v1.8)
        if job.budget < ESTIMATED_COST {
            sdk::outbox::write_error(Error::InsufficientCredits);
            continue;
        }

        // 4. Execution
        let result = process(job);

        // 5. Write & Signal
        sdk::outbox::write(result);
        sdk::signal::raise_outbox();
    }
}
```

## 🔋 Detailed Implementations

### Physics Kernel
*Focus: Determinism & Shared World*
```rust
// modules/physics/src/lib.rs
// Uses rapier3d for consistency across x86/ARM/WASM
impl PhysicsWorld {
    pub fn step(&mut self, dt: f32, ptr_in: *const u8, ptr_out: *mut u8) {
        let inputs = unsafe { parse_inputs(ptr_in) };
        self.integrate_forces(inputs, dt);
        unsafe { write_outputs(ptr_out, &self.bodies) };
    }
}
```

### Mining Kernel
*Focus: Raw Performance (SIMD)*
```rust
// modules/mining/src/lib.rs
// Uses explicit chunking for parallelism
pub fn mine_block(header: &[u8], target: u32) -> Option<u64> {
    for nonce in start..end {
        if check_difficulty(sha256(header, nonce), target) {
            return Some(nonce);
        }
    }
    None
}
```

## 📚 Stack & Libraries

*   **`wgpu`**: The "Universal Hardware Interface". Allows capsules to run compute shaders on Metal/Vulkan/DX12.
*   **`burn`**: The "Intelligence Engine". Runs Tensor operations via `wgpu`, enabling distributed inference.
*   **`rapier3d`**: The "Consistency Engine". Guarantees bit-exact determinism for world states.
*   **`capnp`**: The "Memory Lens". Provides typed views over raw SharedArrayBuffer bytes.

## 🧪 Verification Standards

1.  **Determinism**: `Physics` must produce bit-exact output across all architectures.
2.  **Performance**: `Mining`/`ML` must be within 90% of native speed (validating SIMD usage).
3.  **Memory Safety**: Fuzz test `sdk` raw pointer logic.
4.  **Budget Compliance**: **MUST** stop execution if credits run out.
