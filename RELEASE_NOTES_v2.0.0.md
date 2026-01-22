# INOS v2.0.0 — The Zero-Copy Distributed Runtime

We're excited to publicly release **INOS (Internet-Native Operating System)** — a production-ready distributed runtime that eliminates serialization overhead through SharedArrayBuffer-based reactive mutation.

---

## 🌟 Highlights

### Zero-Copy Architecture
Traditional distributed systems serialize data at every boundary. INOS components share memory directly via `SharedArrayBuffer`, signaling changes through atomic epoch counters. The result: **O(1) performance** regardless of payload size.

### Hybrid Polyglot Runtime
| Layer | Technology | Role |
|:---|:---|:---|
| **Kernel** | Go (WASM) | Orchestration, policy, P2P mesh coordination |
| **Modules** | Rust (WASM) | High-performance compute (GPU, Crypto, Physics) |
| **Host** | TypeScript + React | Sensors, rendering, Web API bridge |

### No wasm-bindgen Philosophy
All Rust modules use **pure C ABI exports** (`extern "C"` + `#[no_mangle]`). This eliminates JS glue code, reduces binary size, and enables direct SAB pointer manipulation.

### Memory Twin Pattern
Go's WASM runtime can't natively share SAB. We solved this with **Ephemeral Snapshot Isolation** — the Kernel operates on a stable local replica synchronized via explicit bridge calls, immune to tearing reads from 60Hz+ Rust compute threads.

---

## 📦 What's Included

### Kernel (`kernel/`)
- **Mesh Coordinator**: DHT-based peer discovery, gossip state propagation
- **Intelligence Engine**: Adaptive scheduling, security policies, workflow optimization
- **Unit Supervisors**: Lifecycle management for Boids, GPU, Crypto, Storage, and more

### Modules (`modules/`)
- **Compute**: 8 production units (GPU, Image, Audio, Data, Crypto, Physics, Math, Boids)
- **Storage**: ChaCha20 encryption, Brotli compression, content-addressed chunks
- **SDK**: SAB primitives, epoch signaling, BLAKE3 hashing, Cap'n Proto bindings

### Frontend (`frontend/`)
- **React + Three.js**: Zero-copy GPU buffer rendering at 60FPS+
- **Dispatcher**: Routes compute requests to dedicated background workers
- **SAB Layout**: Typed region definitions for all shared memory zones

### Protocols (`protocols/`)
- **17 Cap'n Proto schemas** covering system, compute, P2P, identity, and economy domains

---

## 🔧 Technical Specifications

| Metric | Value |
|:---|:---|
| **SAB Size** | 16MB default (configurable) |
| **Epoch Latency** | <1μs (Atomics.waitAsync) |
| **GPU Sync** | Double-buffered, tear-free |
| **Compression** | Brotli (Level 11 for storage) |
| **Hashing** | BLAKE3 (SIMD-accelerated) |
| **Encryption** | ChaCha20-Poly1305 |

---

## 🚀 Getting Started

```bash
# Clone and build
git clone https://github.com/nmxmxh/inos_v1.git
cd inos_v1
make setup && make build

# Run the demo
cd frontend && npm run dev
```

Open `http://localhost:5173` to see 1000+ boids flocking in real-time via zero-copy GPU rendering.

---

## 📜 License

**Business Source License 1.1 (BSL 1.1)**

- ✅ Free for individuals, researchers, and small teams (<$5M revenue, <50 employees)
- 💼 Commercial use by larger entities requires a separate agreement
- 🔓 Converts to **MIT License** on **2029-01-01**

---

## 🙏 Acknowledgments

INOS was built using **Post-AI Development Methodology** — where AI handles boilerplate and humans direct architecture. This project demonstrates that focused architectural vision, amplified by AI tooling, can achieve what traditionally requires large teams.

---

## 📚 Documentation

- [Specification (spec.md)](docs/spec.md) — Complete v2.0 architecture
- [Memory Twin Deep-Dive](docs/go_wasm_memory_integrity.md) — Go WASM SAB bridging
- [Cap'n Proto Schemas](protocols/schemas/) — All protocol definitions

---

**Built with 🧠 by The INOS Architects**
