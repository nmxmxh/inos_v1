# INOS: Internet-Native Operating System

> **A state-centric distributed runtime with native economic incentives**

[![Version](https://img.shields.io/badge/version-2.0-blue.svg)](docs/spec.md)
[![License](https://img.shields.io/badge/license-BSL--1.1-orange.svg)](LICENSE)
[![Status](https://img.shields.io/badge/status-production--ready-green.svg)](docs/spec.md)

---

## 🌌 Overview

INOS is a **biological runtime** for the internet age. It replaces traditional message passing with **shared reality**, enabling zero-copy communication across Go, Rust, and JavaScript boundaries.

### The Core Innovation: Reactive Mutation
We eliminate serialization overhead by allowing components to share memory via **SharedArrayBuffer (SAB)**.
1.  **Mutate**: A worker updates the shared state.
2.  **Signal**: The worker increments an **Atomic Epoch Counter**.
3.  **React**: Subscribers detect the epoch change and synchronize instantly.

---

## 🗺️ Developer Navigation Map

```text
inos_v1/
├── kernel/             # Layer 2: Go WASM Kernel (The Brain)
│   ├── core/           # Scheduler, Mesh Coordinator, Memory management
│   ├── threads/        # WebWorker supervisors and unit loaders
│   └── transport/      # P2P stack (Gossip, DHT, WebRTC)
├── modules/            # Layer 3: Rust WASM Modules (The Muscle)
│   ├── sdk/            # Shared primitives (Epochs, SAB, Registry)
│   ├── compute/        # High-performance compute units (GPU, Image, etc.)
│   └── storage/        # Encrypted storage (ChaCha20, Brotli)
├── frontend/           # Layer 1: React + Vite Host (The Body)
│   ├── app/            # Feature-driven UI components
│   └── src/wasm/       # JS Bridge, Dispatcher, and SAB layout
├── protocols/          # Cap'n Proto schemas (The Language)
└── docs/               # Architectural directives and specifications
```

---

## 📡 Cap'n Proto: The Zero-Copy Language

INOS uses **Cap'n Proto** for all cross-boundary communication. Unlike JSON or Protobuf, Cap'n Proto requires **no parsing step**; data is accessed directly from the wire format.

### Why Cap'n Proto?
*   **Zero-Copy Reads**: Pointers into the message buffer, no deserialization.
*   **Type Safety**: Compile-time schema validation across Go, Rust, and TypeScript.
*   **Compact Binary Format**: Efficient for SAB regions and P2P mesh traffic.

### Schema Organization
All schemas live in `protocols/schemas/` with versioned subdirectories:

| Domain | Schemas | Purpose |
|:---|:---|:---|
| `system/v1` | `sab_layout`, `syscall`, `runtime` | Kernel internals, SAB region definitions |
| `compute/v1` | `capsule` | Job requests/results for Rust units |
| `p2p/v1` | `mesh`, `gossip`, `delegation` | Peer discovery, state propagation |
| `identity/v1` | `identity` | Cryptographic identity and attestation |

### Relevant Files
*   [`protocols/schemas/`](protocols/schemas/) — All `.capnp` schema definitions
*   [`scripts/gen-proto-go.sh`](scripts/gen-proto-go.sh) — Go code generation script
*   [`modules/sdk/src/protocols/`](modules/sdk/src/protocols/) — Rust bindings (via `build.rs`)

---

## 🚫 No wasm-bindgen: The Library Proxy Philosophy

INOS **deliberately avoids `wasm-bindgen`** for Rust modules. This architectural decision ensures:

1.  **Pure C ABI**: All exports use `extern "C"` with `#[no_mangle]`. No JS glue code generated.
2.  **Minimal Binary Size**: No runtime overhead from bindgen-generated wrappers.
3.  **SAB Compatibility**: Direct pointer manipulation into SharedArrayBuffer without JS object marshalling.
4.  **Predictable Memory**: Manual `compute_alloc` / `compute_free` lifecycle.

### The Library Proxy Pattern
Instead of exposing many `#[wasm_bindgen]` functions, we expose a **single generic dispatcher**:

```rust
#[no_mangle]
pub extern "C" fn compute_execute(
    library_ptr: *const u8, library_len: usize,
    method_ptr: *const u8, method_len: usize,
    input_ptr: *const u8, input_len: usize,
    params_ptr: *const u8, params_len: usize,
) -> *mut u8
```

The `ComputeEngine` internally routes to registered units (Image, Crypto, Boids, etc.).

### Relevant Files
*   [`modules/compute/src/lib.rs`](modules/compute/src/lib.rs) — `compute_execute` entry point
*   [`modules/compute/src/engine.rs`](modules/compute/src/engine.rs) — Unit registry and routing
*   [`modules/storage/src/lib.rs`](modules/storage/src/lib.rs) — Storage module (no wasm-bindgen)
*   [`modules/drivers/src/lib.rs`](modules/drivers/src/lib.rs) — I/O drivers (pure C ABI)

---

## 🧠 The Memory Twin: Go WASM Integrity

Go's WASM runtime cannot natively share `SharedArrayBuffer` as its linear memory. INOS solves this with a **Synchronized Memory Twin** architecture.

### The Problem
*   **Rust/JS**: Direct SAB access (true zero-copy).
*   **Go Kernel**: Operates on its own private linear memory.

### The Solution: Ephemeral Snapshot Isolation
The Kernel maintains a **Local Replica (Twin)** synchronized via explicit bridge calls:

```go
// Bulk copy from Global SAB → Local Twin
js.CopyBytesToGo(localTwin, view.Call("subarray", offset, offset+size))
```

### Benefits
*   **Snapshot Consistency**: Kernel operates on a stable snapshot, immune to tearing reads.
*   **Zero-Allocation Sync**: `ReadAt` pattern recycles buffers, minimizing GC pressure.
*   **Double-Buffered State**: Front Buffer (SAB, mutated by Rust/JS) ↔ Back Buffer (Go Twin, stable for logic).

### Relevant Files
*   [`docs/go_wasm_memory_integrity.md`](docs/go_wasm_memory_integrity.md) — Full architectural deep-dive
*   [`kernel/threads/bridge.go`](kernel/threads/bridge.go) — SAB bridge implementation
*   [`frontend/src/wasm/layout.ts`](frontend/src/wasm/layout.ts) — SAB region offsets and indices

---

## ⚡ The Unit Proxy Model (Rust ↔ JS)

INOS uses a standardized **Unit Proxy Model** to expose Rust performance to the JavaScript frontend without complex glue code.

1.  **Registry**: Rust units (Boids, Math, Crypto) register their capabilities in a global table.
2.  **Marshalling**: The `compute_execute` WASM export serves as a generic entry point.
3.  **Dispatch**: The frontend `Dispatcher` routes requests to dedicated background workers (`plug`) or executes them synchronously.
4.  **Zero-Copy**: Parameters and results stay in the SAB; only pointers and lengths cross the WASM boundary.

---

## 🧬 Communication Patterns

Based on the [ArchitecturalBoids](frontend/app/features/boids/ArchitecturalBoids.tsx) implementation:

*   **Decoupled Compute**: Physics (Boids) and Matrix generation (Math) run in autonomous workers.
*   **Pulse Signaling**: Workers wait on the high-precision `PulseWorker` (Zero-CPU idling).
*   **Epoch-Based GPU Sync**: React only flips the GPU buffer pointers when the `IDX_MATRIX_EPOCH` increments, ensuring tear-free rendering at 60FPS+.

---

## 🛠️ Capability Catalog

INOS provides a rich set of built-in functionalities exposed via the `dispatch` system:

| Unit | Capabilities |
|:---|:---|
| **Compute** | BLAKE3/SHA256 Hashing, Brotli Compression/Decompression |
| **GPU** | PBR Rendering, WGSL Execution, Particle Systems, SSAO/SSR |
| **Math** | Matrix/Vector SIMD-ready operations, Quaternions, Projections |
| **Boids** | Massively parallel flocking physics, Evolutionary scaling |
| **Audio** | FFT/Spectrogram, Spatial Audio, FLAC/WAV Encoding |
| **Data** | Parquet/JSON/CSV processing, SQL-like aggregations |
| **Storage** | Content-addressed storage, ChaCha20 encryption, P2P replication |

---

## 🚀 Getting Started

### Prerequisites
*   **Go 1.21+**, **Rust 1.75+**, **Node.js 20+**, **Cap'n Proto 1.0+**

### Quick Build
```bash
make setup    # Install tools and generate protocols
make build    # Build Kernel, Modules, and Frontend
cd frontend && npm run dev
```

---

## 📜 License

INOS is licensed under the **Business Source License 1.1 (BSL 1.1)**.

*   **Free for Individuals & Small Teams**: Permitted for entities with <$5M annual revenue and <50 employees.
*   **Commercial Use**: Requires a separate agreement for larger corporations ("Fat Checks").
*   **Eventual Open Source**: Becomes **MIT Licensed** on **2029-01-01**.

See [LICENSE](LICENSE) for full legal text.

---

*Built with 🧠 by The INOS Architects*
