# INOS SAB Layout Architecture

**Version**: 2.2  
**Status**: Architectural Specification  
**Philosophy**: Memory-as-Communication-Medium with Epoch-Based Synchronization

---

## Executive Summary

The SharedArrayBuffer (SAB) is INOS's **single source of truth** for all cross-layer communication. It replaces traditional message passing with **zero-copy memory views** and **atomic epoch signaling**.

**The Three-Layer Unification:**
```
┌─────────────────────────────────────────────────────────────────┐
│                 WebAssembly Linear Memory (SAB)                  │
├─────────────────────────────────────────────────────────────────┤
│   Go Kernel (Brain)     Rust Modules (Muscles)    JS (Sensors)  │
│   ├─ Supervisors        ├─ Compute Units          ├─ Rendering  │
│   ├─ Coordinators       ├─ Physics Engines        ├─ UI State   │
│   └─ Registry           └─ Crypto/Storage         └─ Inputs     │
│                                                                  │
│   ALL READ/WRITE TO SAME ABSOLUTE OFFSETS                       │
│   SYNCHRONIZED VIA ATOMIC EPOCH COUNTERS                        │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. Memory Map Overview

### 1.1 Linear Memory Base Address

All layers use absolute offsets starting at `0x000000`. Go uses the "Memory Twin" pattern with explicit `js.CopyBytesToGo` bridging for stable decision-making.

| Layer | Implementation | Base Address |
|-------|----------------|--------------|
| **Go** | Memory Twin (bridged copy) | N/A (isolated) |
| **Rust** | `SafeSAB::new(&sab)` | `0x000000` |
| **JS** | `new Int32Array(window.__INOS_SAB__)` | `0x000000` |

### 1.2 Region Allocation (32MB Default, 1GB Max)

```
SAB Memory Map (Absolute Addresses - v2.2)
├── 0x000000 - 0x00007F: Atomic Flags (128B)
├── 0x000080 - 0x00012F: Supervisor Alloc (176B)
├── 0x000130 - 0x00013F: Registry Lock (16B)
├── 0x000140 - 0x00193F: Module Registry (6KB)
├── 0x001940 - 0x001A3F: Bloom Filter (256B)
├── 0x002000 - 0x002FFF: Supervisor Headers (4KB)
├── 0x003000 - 0x003FFF: Syscall Table (4KB)
├── 0x004000 - 0x007FFF: Economics Region (16KB)
├── 0x008000 - 0x00BFFF: Identity Registry (16KB)
├── 0x00C000 - 0x00FFFF: Social Graph (16KB)
├── 0x010000 - 0x01FFFF: Pattern Exchange (64KB)
├── 0x020000 - 0x03FFFF: Job History (128KB)
├── 0x040000 - 0x04FFFF: Coordination (64KB)
├── 0x050000 - 0x14FFFF: Inbox/Outbox (1MB)
│   ├── 0x050000 - 0x0CFFFF: Inbox (512KB)
│   └── 0x0D0000 - 0x14FFFF: Outbox (512KB)
└── 0x150000 - END:      Arena (Dynamic)
    ├── 0x150000 - 0x150FFF: Diagnostics (4KB)
    ├── 0x15F000 - 0x15FFFF: Region Guard Table (4KB)
    ├── 0x160000 - 0x160FFF: Bird State (4KB)
    ├── 0x161000 - 0x16103F: Ping-Pong Control (64B)
    ├── 0x162000 - 0x3C1FFF: Bird Buffer A (2.36MB)
    ├── 0x3C2000 - 0x621FFF: Bird Buffer B (2.36MB)
    ├── 0x622000 - 0xB21FFF: Matrix Buffer A (5.12MB)
    └── 0xB22000 - END:      Matrix Buffer B (5.12MB)
```

---

## 2. Epoch-Based Signaling

Located at `OFFSET_ATOMIC_FLAGS` (`0x000000`), each epoch is a 4-byte `i32`.

### 2.1 Epoch Index Allocation

| Index | Name | Signaler | Waiter | Purpose |
|-------|------|----------|--------|---------|
| 0 | `IDX_KERNEL_READY` | Go | JS | Kernel boot complete |
| 1 | `IDX_INBOX_DIRTY` | Go | Rust | Signal to module |
| 2 | `IDX_OUTBOX_DIRTY` | Rust | Go | Results ready |
| 3 | `IDX_PANIC_STATE` | Any | All | System panic |
| 4-7 | System Epochs | Various | Various | Sensor, Actor, Storage, System |
| 8 | `IDX_ARENA_ALLOCATOR` | Rust | Go | Arena bump pointer |
| 9-10 | Mutexes | Various | Various | Inbox/Outbox locks |
| 12 | `IDX_BIRD_EPOCH` | Rust | JS | Bird physics complete |
| 13 | `IDX_MATRIX_EPOCH` | Rust | JS | Matrix generation complete |
| 14 | `IDX_PINGPONG_ACTIVE` | Rust | JS | Active buffer (0=A, 1=B) |
| 15 | `IDX_REGISTRY_EPOCH` | Rust | Go | Module registration |
| 16-19 | Extended | Various | Various | Evolution, Health, Learning, Economy |
| 20 | `IDX_BIRD_COUNT` | Rust | JS | Active bird count |
| 31 | `IDX_CONTEXT_ID_HASH` | JS | All | Context verification |
| 32-127 | Supervisor Pool | Dynamic | Dynamic | 96 supervisor epochs |
| 128-255 | Reserved | — | — | Future expansion |

### 2.2 Signal Protocol

**Producer (Rust):**
```rust
fn signal_epoch_change(sab: &SafeSAB, idx: u32) {
    js_interop::atomic_add(&sab.barrier_view(), idx, 1);
    js_interop::atomic_notify(&sab.barrier_view(), idx, i32::MAX);
}
```

**Consumer (Go):**
```go
func (sb *SABBridge) WaitForEpochChange(idx uint32, lastEpoch int32, timeoutMs float64) int32 {
    // result := Atomics.Wait(view, idx, lastEpoch, timeoutMs)
    // return Atomics.Load(view, idx)
}
```

---

## 3. Ping-Pong Buffer Architecture

### 3.1 Purpose

Eliminates read/write contention between layers:
- **Producer** writes to inactive buffer
- **Consumer** reads from active buffer
- **Epoch flip** atomically switches roles

### 3.2 Buffer Pairs

| Buffer Set | Absolute Offset A | Absolute Offset B | Size | Purpose |
|------------|-------------------|-------------------|------|---------|
| **Bird Buffers** | `0x162000` | `0x3C2000` | 2.36MB | Entity state (10k × 236B) |
| **Matrix Buffers** | `0x622000` | `0xB22000` | 5.12MB | GPU transforms (10k × 512B) |

### 3.3 Region Guard Table (Cross-Layer Enforcement)

INOS reserves a small **Region Guard Table** inside Arena metadata to enforce
single-writer and authorized-writer policies across Go/Rust/JS. Each entry is a
fixed-size record that tracks:
- Authorized owner (Kernel, Module, Host, System)
- Active writer lock (atomic)
- Last observed epoch
- Violation counter (debug/telemetry)

This enables runtime detection of multi-writer overlap and mis-ordered epoch
signaling without changing the primary SAB layout.

---

## 4. Source of Truth: Cap'n Proto Schema

The layout is defined centrally in `protocols/schemas/system/v1/sab_layout.capnp`. 

> [!IMPORTANT]
> **Never hardcode offsets.** All layers must import the generated constants to ensure total harmony.
> - **Go**: `system.OffsetName`
> - **Rust**: `sdk::layout::OFFSET_NAME`
> - **JS**: `CONSTS.OFFSET_NAME`

### 4.1 Implementation Status ✅
- [x] Unified constants in `sab_layout.capnp`
- [x] Absolute offsets starting at 0 (no Go reservation)
- [x] Rust SDK refactored to use generated schema
- [x] Go kernel uses Memory Twin pattern
- [x] Frontend constants regenerated and verified

---

## 5. Device Tier SAB Sizes

| Tier | Profile | SAB Size | Entities | P2P Role |
|------|---------|----------|----------|----------|
| **Light** | Mobile/IoT | 32MB | <5k | Pulse |
| **Moderate** | Laptop | 64MB | 10k | Gossip |
| **Heavy** | Workstation | 128MB | 50k | Full DHT |
| **Dedicated** | Server | 256MB+ | 100k+ | Relay/Seed |

---

## 6. Go Memory Twin Pattern

Since Go WASM cannot directly share memory with SAB, INOS uses explicit bridging:

```go
// kernel/sab_bridge.go
func (sb *SABBridge) SyncFromSAB(offset, size uint32) {
    js.Global().Get("__INOS_BRIDGE__").Call("copyToGo", 
        sb.localBuffer, offset, size)
}
```

**Benefits:**
- **Snapshot Isolation**: Go operates on stable view, immune to high-frequency Rust updates
- **Zero GC Pressure**: Single pre-allocated buffer reused
- **Deterministic Timing**: Sync happens at explicit checkpoints, not continuously
