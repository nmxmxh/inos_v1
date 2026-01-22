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
