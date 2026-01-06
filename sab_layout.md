# INOS SAB Layout Architecture

**Version**: 2.0  
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
│   └─ Registry           └─ ML Inference           └─ Inputs     │
│                                                                  │
│   ALL READ/WRITE TO SAME ABSOLUTE OFFSETS                       │
│   SYNCHRONIZED VIA ATOMIC EPOCH COUNTERS                        │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. Memory Map Overview

### 1.1 Linear Memory Base Address

**Critical Requirement**: All layers must use `address 0` as the SAB base.

| Layer | Implementation | Base Address |
|-------|----------------|--------------|
| **Go** | `unsafe.Pointer(uintptr(0))` | `0x000000` |
| **Rust** | `SafeSAB::new(&sab)` with `base_offset=0` | `0x000000` |
| **JS** | `new Int32Array(window.__INOS_SAB__)` | `0x000000` |

### 1.2 Region Allocation

```
SAB Memory Map (16MB Default - 1GB Max)
├── 0x000000 - 0x00003F: Atomic Flags (64B)
├── 0x000040 - 0x0000EF: Supervisor Alloc (176B)
├── 0x0000F0 - 0x0000FF: Registry Lock (16B)
├── 0x000100 - 0x001FFF: Module Registry (6KB)
├── 0x002000 - 0x002FFF: Supervisor Headers (4KB)
├── 0x003000 - 0x003FFF: Syscall Table (4KB)
├── 0x004000 - 0x007FFF: Economics Region (16KB)
├── 0x008000 - 0x00BFFF: Identity Registry (16KB)
├── 0x00C000 - 0x00FFFF: Social Graph (16KB)
├── 0x010000 - 0x01FFFF: Pattern Exchange (64KB)
├── 0x020000 - 0x03FFFF: Job History (128KB)
├── 0x040000 - 0x04FFFF: Coordination (64KB)
├── 0x050000 - 0x14FFFF: Inbox/Outbox (1MB)
└── 0x150000 - END:      Arena (Dynamic)
    ├── 0x150000 - 0x150FFF: Diagnostics (4KB)
    ├── 0x160000 - 0x160FFF: Bird State (4KB)
    ├── 0x161000 - 0x16103F: Ping-Pong Control (64B)
    ├── 0x162000 - 0x3C1FFF: Bird Buffer A (2.36MB)
    ├── 0x3C2000 - 0x621FFF: Bird Buffer B (2.36MB)
    ├── 0x622000 - 0xB21FFF: Matrix Buffer A (5.12MB)
    └── 0xB22000 - END:      Matrix Buffer B (5.12MB)
```

---

## 2. Epoch-Based Signaling

### 2.1 Epoch Index Allocation

Located at `OFFSET_ATOMIC_FLAGS` (0x000000), each epoch is a 4-byte `i32`.

| Index | Name | Signaler | Waiter | Purpose |
|-------|------|----------|--------|---------|
| 0 | `IDX_KERNEL_READY` | Go | JS | Kernel boot complete |
| 1 | `IDX_INBOX_DIRTY` | Go | Rust | New work available |
| 2 | `IDX_OUTBOX_DIRTY` | Rust | Go | Results ready |
| 3 | `IDX_PANIC_STATE` | Any | All | System panic |
| 4-7 | *Reserved* | — | — | — |
| 8 | `IDX_ARENA_ALLOCATOR` | Rust | Go | Arena bump pointer |
| 9 | `IDX_OUTBOX_MUTEX` | — | — | Mutex for outbox |
| 10 | `IDX_INBOX_MUTEX` | — | — | Mutex for inbox |
| 11 | `IDX_METRICS_EPOCH` | Rust | Go | Metrics updated |
| 12 | `IDX_BIRD_EPOCH` | Rust | JS | Bird physics complete |
| 13 | `IDX_MATRIX_EPOCH` | Rust | JS | Matrix generation complete |
| 14 | `IDX_PINGPONG_ACTIVE` | Rust | JS | Active buffer (0=A, 1=B) |
| 15 | `IDX_REGISTRY_EPOCH` | Rust | Go | Module registration |
| 16 | `IDX_EVOLUTION_EPOCH` | Rust | Go | Boids evolution |
| 17 | `IDX_HEALTH_EPOCH` | Rust | Go | Health metrics |
| 18 | `IDX_LEARNING_EPOCH` | Rust | Go | Pattern learning |
| 19 | `IDX_ECONOMY_EPOCH` | Rust | Go | Credit settlement |
| 20-31 | *Reserved* | — | — | Future signals |
| 32-127 | Supervisor Pool | Dynamic | Dynamic | Per-supervisor epochs |

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
func (s *SABBridge) WaitForEpochChange(idx uint32, lastEpoch int32, timeoutMs float64) int32 {
    result := Atomics.Wait(view, idx, lastEpoch, timeoutMs)
    return Atomics.Load(view, idx)
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

| Buffer Set | Size | Purpose | Layout |
|------------|------|---------|--------|
| **Bird Buffers** | 2 × 2.36MB | Entity state (pos, vel, orientation) | 10k × 236 bytes |
| **Matrix Buffers** | 2 × 5.12MB | GPU-ready transforms | 10k × 8 parts × 64 bytes |

### 3.3 Selection Logic

```typescript
const epoch = Atomics.load(flags, IDX_MATRIX_EPOCH);
const isBufferA = (epoch % 2 === 0);
const matrixOffset = isBufferA ? 0x622000 : 0xB22000;
```

---

## 4. Integration with Economic Layer

### 4.1 Economics Region (0x004000 - 0x007FFF)

| Offset | Size | Structure | Purpose |
|--------|------|-----------|---------|
| 0x004000 | 4KB | `CreditLedger` | Real-time balances |
| 0x005000 | 4KB | `TransactionQueue` | Pending settlements |
| 0x006000 | 4KB | `YieldAccumulator` | Creator/Referrer/CloseID yields |
| 0x007000 | 4KB | `UBIState` | UBI drip tracking |

### 4.2 Social Graph Region (0x00C000 - 0x00FFFF)

| Offset | Size | Structure | Purpose |
|--------|------|-----------|---------|
| 0x00C000 | 8KB | `ReferralGraph` | DID → Referrer mapping |
| 0x00E000 | 8KB | `CloseIdentityList` | DID → Close IDs (max 15) |

---

## 5. Integration with Identity Layer

### 5.1 Identity Registry (0x008000 - 0x00BFFF)

| Field | Size | Purpose |
|-------|------|---------|
| `did_hash` | 32B | BLAKE3 of DID |
| `device_count` | 2B | Linked devices |
| `reputation` | 4B | Trust score |
| `tier` | 1B | Resource tier |
| `flags` | 1B | Status flags |
| `reserved` | 24B | Padding |
| **Total** | 64B | Per-identity entry (256 max) |

---

## 6. Graphics Pipeline Integration

### 6.1 Rendering Data Flow

```
Frame N:
┌──────────────┐      ┌──────────────┐      ┌──────────────┐
│ Rust Boids   │─SAB─►│ Rust Matrix  │─SAB─►│ JS Renderer  │
│ step_physics │      │ compute_mats │      │ GPU upload   │
└──────────────┘      └──────────────┘      └──────────────┘
     Buffer A              Buffer A             Read A
                                                (epoch=100)
Frame N+1:
     Buffer B              Buffer B             Read B
                                                (epoch=101)
```

### 6.2 Performance Targets

| Metric | Target | Current |
|--------|--------|---------|
| Physics step | <2ms | ~1.5ms |
| Matrix generation | <1ms | ~0.8ms |
| GPU upload | <0.5ms | ~0.3ms |
| Total frame | <4ms | ~2.6ms |

---

## 7. Future Architecture: Cap'n Proto Unification

### 7.1 Proposed Schema

```capnp
# protocols/schemas/system/v1/sab_layout.capnp
@0xa1b2c3d4e5f67890;

# Region offsets as constants
const offsetAtomicFlags :UInt32 = 0x000000;
const offsetModuleRegistry :UInt32 = 0x000100;
const offsetEconomics :UInt32 = 0x004000;
const offsetIdentity :UInt32 = 0x008000;
const offsetSocialGraph :UInt32 = 0x00C000;
const offsetArena :UInt32 = 0x150000;
const offsetBirdBufferA :UInt32 = 0x162000;
const offsetMatrixBufferA :UInt32 = 0x622000;
```

### 7.2 Benefits

| Current | With Cap'n Proto |
|---------|------------------|
| Manual sync across 3 files | Single schema generates all |
| Drift risk | Type-checked consistency |
| Hard to verify | Schema validation at build |

---

## 8. Implementation TODO

### 8.1 Phase 1: Cap'n Proto Schema (Foundation Complete ✅)

**Existing Infrastructure:**
- ✅ `make proto` → runs `gen-proto-go.sh` for Go generation
- ✅ `modules/build.rs` handles Rust generation automatically
- ✅ `protocols/schemas/system/v1/` exists with `syscall.capnp` and `orchestration.capnp`
- ✅ 369 OFFSET/IDX usages across codebase (needs consolidation)

**TODO:**
- [ ] **Create `sab_layout.capnp`** at `protocols/schemas/system/v1/sab_layout.capnp`
  - [ ] Define all region offsets as `const` values
  - [ ] Define all epoch indices as `const` values
  - [ ] Define buffer sizes and alignments
  - [ ] Add inline documentation for each region

### 8.2 Phase 2: Build System Integration

**TODO:**
- [ ] **Update `scripts/gen-proto-go.sh`** to include `sab_layout.capnp`
- [ ] **Update `modules/sdk/build.rs`** to compile `sab_layout.capnp`
- [ ] **Add `make proto-sab`** target to Makefile (or include in `make proto`)
- [ ] **Verify generation** works for both Go and Rust

### 8.3 Phase 3: Migrate Layout Files

**Current files to replace:**
| File | Lines | OFFSET/IDX Count |
|------|-------|------------------|
| `kernel/threads/sab/layout.go` | ~400 | ~180 |
| `modules/sdk/src/layout.rs` | ~252 | ~120 |

**TODO:**
- [ ] **Generate `layout_gen.go`** from schema
- [ ] **Generate `layout_gen.rs`** from schema
- [ ] **Update imports** in Go (change `sab_layout.` to `sab_layout_gen.`)
- [ ] **Update imports** in Rust (change `layout::` to `layout_gen::`)
- [ ] **Verify compilation** with `make kernel-build` and `make modules-build`
- [ ] **Delete manual files** after verification

### 8.4 Phase 4: Graphics Pipeline Optimizations

**TODO:**
- [ ] Implement SoA memory layout for SIMD (4x physics speedup)
- [ ] Add hierarchical epoch dependencies (partial updates)
- [ ] Implement spatial culling pipeline (reduce draw calls)
- [ ] Add WebGPU backend with WebGL fallback
- [ ] Implement temporal coherence (70-90% matrix reduction)
- [ ] Add multi-LOD system (100k entities)

### 8.5 Phase 5: Advanced Features

**TODO:**
- [ ] GPU-driven pipeline via compute shaders
- [ ] Distributed rendering for multi-display
- [ ] Predictive buffer warming for VR/120Hz
- [ ] Audio buffer integration in SAB
- [ ] Debug metrics region for self-optimization

---

## 9. Validation Checklist

### Memory Layout Consistency
- [ ] All 3 layers (Go/Rust/JS) use address 0 as base
- [ ] All OFFSET constants match across generated files
- [ ] All IDX constants match across generated files
- [ ] Ping-pong buffer offsets are correctly aligned

### Signal Protocol
- [ ] Go discovery loop uses `Atomics.wait` on `IDX_REGISTRY_EPOCH`
- [ ] Rust modules call `signal_registry_change` after registration
- [ ] All polling loops converted to signal-based

### Build Verification
- [ ] `make proto` generates without errors
- [ ] `make kernel-build` compiles successfully
- [ ] `make modules-build` compiles successfully
- [ ] Frontend loads all modules correctly

---

## 10. Reference Files

| Component | Path |
|-----------|------|
| **Schema (NEW)** | `protocols/schemas/system/v1/sab_layout.capnp` |
| **Proto Gen Script** | `scripts/gen-proto-go.sh` |
| **Rust Build** | `modules/sdk/build.rs` |
| Rust Layout (manual) | `modules/sdk/src/layout.rs` |
| Go Layout (manual) | `kernel/threads/sab/layout.go` |
| SAB Bridge | `kernel/threads/supervisor/sab_bridge.go` |
| Ping-Pong | `modules/sdk/src/pingpong.rs` |
| Boids Physics | `modules/compute/src/units/boids.rs` |
| Matrix Compute | `modules/compute/src/units/math.rs` |
| Kernel Init | `kernel/main.go` |
| Module Loader | `frontend/src/wasm/module-loader.ts` |
