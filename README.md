# INOS: Internet-Native Operating System

> **A state-centric distributed runtime with native economic incentives**

[![Version](https://img.shields.io/badge/version-1.9-blue.svg)](spec.md)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Status](https://img.shields.io/badge/status-specification-yellow.svg)](spec.md)

---

## 🌌 What is INOS?

INOS is not just another distributed system—it's a **biological runtime** for the internet age. Think of it as a globally distributed motherboard where:

*   **Nodes are Cells**: Disposable, specialized (GPU/Storage), self-healing
*   **Kernel is the Nervous System**: Go-based orchestration brain
*   **Economy is ATP**: Credits drive replication, maintenance, and compute
*   **Reactive Mutation is Reflexes**: Zero-copy signaling mimics biological nerve responses

### The Core Innovation: Reactive Mutation

We replace traditional message passing with **shared reality**:

```
Traditional:  Node A → (serialize) → network → (deserialize) → Node B
INOS:         Node A writes to SAB → Node B reads from same memory
```

**Result:** Zero serialization overhead, atomic consistency, O(1) performance.

---

## 🏗️ Architecture at a Glance

```
┌────────────────────────────────────────────────────────┐
│  Layer 3: The Modules (WASM)                           │
│  [Rust Compute/Storage] [React+Vite UI]                │
├────────────────────────────────────────────────────────┤
│  Layer 2: The Kernel (WASM)                            │
│  [Go Orchestration & Currency]                         │
├────────────────────────────────────────────────────────┤
│  Layer 1: The Hybrid Host (Native)                     │
│  [Nginx + Brotli] [JS Web API Bridge]                  │
└────────────────────────────────────────────────────────┘
```

| Layer | Technology | Role |
|:------|:-----------|:-----|
| **Ingress** | Nginx + Brotli | High-speed network termination |
| **UI** | React + Vite | User interaction & sensor access |
| **Kernel** | Go (WASM) | Orchestration, policy, economy |
| **Compute** | Rust (WASM) | GPU, Data, Crypto, Image, Audio, Physics, API Proxy |
| **Storage** | P2P Mesh | Content-addressed, economically incentivized |

---

## 🚀 Key Features

### Zero-Copy Pipelining & Synchronized Twins
We employ a hybrid memory strategy:
1.  **Rust/JS (Hot Path):** Zero-copy access to SharedArrayBuffer (SAB) for physics/rendering.
2.  **Kernel (Control Path):** Uses a **Synchronized Memory Twin** (Zero-Allocation Sync) to operate on a stable snapshot of the state.

```
Network → SAB (Inbox) → Rust (Process) → SAB (Arena) → JS (Render)
SAB (Arena) ⟳ [Active Sync] ➔ Kernel (Twin) ➔ Orchestration
```

### Layered Compression Integrity
*   **Pass 1 (Ingress):** Brotli-Fast for network efficiency
*   **Pass 2 (Storage):** Brotli-Max for storage density
*   **Anchor:** `Hash = BLAKE3(Compressed-1)` ensures global deduplication

### Economic Storage Mesh & Mosaic P2P (v1.9)
*   **Hierarchical Topology:** Seeds (Core) → Hubs (Aggregation) → Edges (Leaf) for massive scale.
*   **1MB State Chunking:** Optimal voxel-state distribution via QUIC-based P2P bridge.
*   **Geo-Aware Replication:** Redundant storage with latency-tier optimization.
*   **Hot Tier (Edge/CDN):** Earns credits for bandwidth (data retrieval).
*   **Cold Tier (Vault):** Earns credits for capacity (data retention).
*   **Dynamic Scaling:** Viral content automatically replicates to more nodes.

### Epoch-Based Signaling (v1.9)
Components signal state changes via atomic epoch counters:
```rust
Epoch += 1  // Signal mutation
// Kernel reacts when: Epoch > LastSeenEpoch
```

### Syscall Architecture (v2.0)
*   **Authenticated Communication**: Modules request kernel services via Cap'n Proto syscalls
*   **Zero-Copy Routing**: Messages routed through `MeshCoordinator` without serialization overhead
*   **Type Safety**: Cap'n Proto schemas ensure compile-time correctness across Go-Rust boundary
*   **Available Syscalls**: `fetchChunk`, `storeChunk`, `sendMessage`, `spawnThread`, `killThread`
*   **Security**: Every syscall includes `source_module_id` for identity verification and policy enforcement

---

---

## 📂 Project Structure

> **Note:** The definitive source of truth for architectural requirements is [spec.md](spec.md).

```
inos_v1/
├── kernel/              # Go WASM kernel (orchestration)
│   ├── core/           # Memory, scheduler, supervisor
│   ├── transport/      # P2P networking (DHT, WebRTC)
│   └── utils/          # Logging, error handling
├── modules/            # Rust WASM modules (compute)
│   ├── sdk/           # Shared utilities (SAB, Epoch, Credits)
│   ├── compute/       # Multi-unit compute (GPU, Data, Crypto, Image, Audio, Physics, API Proxy)
│   ├── storage/       # Encrypted storage (ChaCha20, Brotli)
│   ├── drivers/       # I/O Sockets (Sensors → Actors, library proxies)
│   └── diagnostics/   # System metrics and monitoring
├── frontend/           # React + Vite UI
├── protocols/          # Cap'n Proto schemas
│   └── schemas/        # Versioned protocol definitions
├── docs/               # Architecture documentation
└── deployment/         # Docker, Nginx configs
```

---

## 🛠️ Quick Start

### Prerequisites
*   **Go 1.21+** (for kernel)
*   **Rust 1.75+** (for modules)
*   **Node.js 20+** (for frontend)
*   **Cap'n Proto 1.0+** (for protocol generation)

### Build & Run

```bash
# 1. Build the kernel
make kernel

# 2. Build Rust modules
cd modules && cargo build --target wasm32-unknown-unknown --release

# 3. Start frontend dev server
cd frontend && yarn install && yarn dev

# 4. Open browser
open http://localhost:5173
```

---

## 📖 Documentation

*   **[Specification (spec.md)](spec.md)** - Complete v1.9 architecture
*   **[Supervisor Architecture (threads.md)](threads.md)** - SAB-native supervisor implementation
*   **[Rust Explainer (RUST_EXPLAINER.md)](RUST_EXPLAINER.md)** - Why Rust is the "muscle"
*   **[Cap'n Proto Guide (kernel/docs/CAPNPROTO.md)](kernel/docs/CAPNPROTO.md)** - Protocol integration
*   **[P2P Mesh Architecture (docs/P2P_MESH.md)](docs/P2P_MESH.md)** - Adaptive replication (5-700 nodes)

---

## 🧬 The Post-AI Development Paradigm

INOS is built using a novel methodology where:

*   **AI handles boilerplate**: 100% of Go/Rust/TS scaffolding is AI-generated
*   **Human directs architecture**: System design and coherence maintained by focused vision
*   **Validation is exhaustive**: Thousands of edge cases tested before deployment

**Result:** What traditionally requires large teams is orchestrated by amplified human intelligence.

---

## 🎯 Current Status

| Component | Status | Notes |
|:----------|:-------|:------|
| **Kernel (Go)** | ✅ Production | Scheduler, memory manager, transport, Mesh Coordinator, Intelligence |
| **SDK (Rust)** | ✅ Production | SAB, Epoch, Credits, Cap'n Proto, Identity |
| **Compute Module** | ✅ Production | GPU, Data, Crypto, Image, Audio, Physics, API Proxy (8 units) |
| **Storage Module** | ✅ Production | ChaCha20 encryption, Brotli compression, 15/17 tests passing |
| **Drivers Module** | 🏗️ In Progress | I/O Sockets (Sensors → Actors), library proxy pattern |
| **P2P Mesh** | ✅ Production | DHT, WebRTC, Gossip, Adaptive Replication |
| **Frontend** | ✅ Production | React + Three.js, WASM loader, zero-copy rendering |
| **Documentation** | ✅ Complete | ASSESSMENT.md, MIGRATION_GUIDE.md, DRIVERS_ARCHITECTURE.md |


---

## 🤝 Contributing

INOS is an intentional architecture. Contributions should align with the core philosophy:

1.  **Zero-copy first**: Avoid serialization wherever possible
2.  **Economic alignment**: Every resource has a cost and a reward
3.  **Deterministic execution**: Same input → same output, always
4.  **Biological metaphors**: Design systems that heal and adapt

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## 📜 License

MIT License - See [LICENSE](LICENSE) for details.

---

## 🌟 Vision

**INOS isn't just software—it's a digital immune system.**

By combining:
*   **Rust's muscle** (performance + safety)
*   **Go's brain** (orchestration + policy)
*   **JS's body** (sensors + UI)
*   **Economic incentives** (self-sustaining mesh)

We're building the foundational layer for the next generation of distributed applications—where computation, storage, and identity flow seamlessly across devices, from phones to drones to data centers.

**This is computing as a living system, not as a mechanical construct.**

---

*Built with 🧠 by The INOS Architects*
