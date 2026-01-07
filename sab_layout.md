# INOS SAB Layout Architecture

**Version**: 2.1  
**Status**: Architectural Specification (Unified Absolute Addressing)  
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
│   └─ Registry           └─ ML Inference           └─ Inputs     │
│                                                                  │
│   ALL READ/WRITE TO SAME ABSOLUTE OFFSETS                       │
│   SYNCHRONIZED VIA ATOMIC EPOCH COUNTERS                        │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. Memory Map Overview

### 1.1 Linear Memory Base Address

**Critical Requirement**: All layers must use `address 0` as the SAB base. All offsets are absolute.

| Layer | Implementation | Base Address |
|-------|----------------|--------------|
| **Go** | `unsafe.Pointer(uintptr(0))` | `0x000000` |
| **Rust** | `SafeSAB::new(&sab)` | Absolute |
| **JS** | `new Int32Array(window.__INOS_SAB__)` | Absolute |

### 1.2 Region Allocation (48MB Default - 1GB Max)

The first 16MB (`0x00000000` to `0x01000000`) are reserved for the Go Kernel binary and heap. All INOS system regions start after this zone.

```
SAB Memory Map (Absolute Addresses)
├── 0x00000000 - 0x01000000: Go Kernel Reservation Zone (16MB)
│
├── 0x01000000 - 0x0100007F: Atomic Flags (128B)
├── 0x01000080 - 0x0100012F: Supervisor Alloc (176B)
├── 0x01000130 - 0x0100013F: Registry Lock (16B)
├── 0x01000140 - 0x0100193F: Module Registry (6KB)
├── 0x01001940 - 0x01001A3F: Bloom Filter (256B)
├── 0x01002000 - 0x01002FFF: Supervisor Headers (4KB)
├── 0x01003000 - 0x01003FFF: Syscall Table (4KB)
├── 0x01004000 - 0x01007FFF: Economics Region (16KB)
├── 0x01008000 - 0x0100BFFF: Identity Registry (16KB)
├── 0x0100C000 - 0x0100FFFF: Social Graph (16KB)
├── 0x01010000 - 0x0101FFFF: Pattern Exchange (64KB)
├── 0x01020000 - 0x0103FFFF: Job History (128KB)
├── 0x01040000 - 0x0104FFFF: Coordination (64KB)
├── 0x01050000 - 0x0114FFFF: Inbox/Outbox (1MB)
└── 0x01150000 - END:      Arena (Dynamic)
    ├── 0x01150000 - 0x01150FFF: Diagnostics (4KB)
    ├── 0x01160000 - 0x01160FFF: Bird State (4KB)
    ├── 0x01161000 - 0x0116103F: Ping-Pong Control (64B)
    ├── 0x01162000 - 0x013C1FFF: Bird Buffer A (2.36MB)
    ├── 0x013C2000 - 0x01621FFF: Bird Buffer B (2.36MB)
    ├── 0x01622000 - 0x01B21FFF: Matrix Buffer A (5.12MB)
    └── 0x01B22000 - END:      Matrix Buffer B (5.12MB)
```

---

## 2. Epoch-Based Signaling

Located at `OFFSET_ATOMIC_FLAGS` (`0x01000000`), each epoch is a 4-byte `i32`.

### 2.1 Epoch Index Allocation

| Index | Name | Signaler | Waiter | Purpose |
|-------|------|----------|--------|---------|
| 0 | `IDX_KERNEL_READY` | Go | JS | Kernel boot complete |
| 1 | `IDX_INBOX_DIRTY` | Go | Rust | New work available |
| 2 | `IDX_OUTBOX_DIRTY` | Rust | Go | Results ready |
| 3 | `IDX_PANIC_STATE` | Any | All | System panic |
| 12 | `IDX_BIRD_EPOCH` | Rust | JS | Bird physics complete |
| 13 | `IDX_MATRIX_EPOCH` | Rust | JS | Matrix generation complete |
| 14 | `IDX_PINGPONG_ACTIVE` | Rust | JS | Active buffer (0=A, 1=B) |
| 15 | `IDX_REGISTRY_EPOCH` | Rust | Go | Module registration |

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
| **Bird Buffers** | `0x01162000` | `0x013C2000` | 2.36MB | Entity state |
| **Matrix Buffers** | `0x01622000` | `0x01B22000` | 5.12MB | GPU transforms |

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
- [x] Absolute offsets including 16MB base
- [x] Rust SDK refactored to use generated schema
- [x] Go kernel bindings regenerated
- [x] Frontend constants regenerated and verified
- [x] Manual offset additions removed from Go/Rust code
