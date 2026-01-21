# Ping-Pong Buffer Architecture

**Zero-allocation, lock-free memory synchronization for INOS distributed compute.**

> [!IMPORTANT]
> **Architectural Pattern for ALL High-Frequency Data**
> This is not just for boids - ping-pong buffers are the **foundational pattern** for any hot-path data exchange in INOS. This includes:
> - **Entity Simulation** (boids, particles, agents)
> - **GPU Compute Results** (matrix transformations, physics state, shader outputs)
> - **Audio/Video Streams** (DSP buffers, frame data)
> - **Real-time Metrics** (telemetry, profiling, diagnostics)
> - **P2P Message Queues** (gossip payloads, sync batches)
> 
> Any data that updates at >30Hz should use ping-pong buffers to avoid GC pressure.

## The Problem

**Per-frame allocations in hot paths across ALL compute units:**

```rust
// Example 1: Boids simulation (60fps)
let mut population_data = vec![0u8; count * 236];  // 2.3MB @ 10k birds

// Example 2: GPU matrix transforms (60fps)
let mut matrix_output = vec![0u8; count * 512];    // 5MB @ 10k entities

// Example 3: Audio DSP (48kHz sampling)
let mut audio_buffer = vec![f32; 1024 * channels]; // Every 21ms

// Example 4: P2P gossip payloads (variable)
let mut message_batch = vec![0u8; batch_size];     // Every sync round
```

**Impact at scale**:
- **Boids @ 10k entities**: ~47MB/sec → GC stalls
- **Audio @ 48kHz**: ~200KB/sec → Xruns and dropouts  
- **P2P @ 100 peers**: Variable bursts → Unpredictable latency

**Deeper Problem**: Go WASM and Rust WASM have separate linear memories. They can't directly share memory - the SAB is the **bridge** that connects them.

---

## The Solution: Circular Ring Topology

The SharedArrayBuffer is the **central hub**. Go, Rust, and JS form a **circular ring** around it, each reading/writing to designated regions with epoch-based synchronization.

```
                         ╔═══════════════════════╗
                         ║   SharedArrayBuffer   ║
                         ║      (Central Hub)    ║
                         ║                       ║
                         ║   ┌───────┬───────┐   ║
                         ║   │ Buf A │ Buf B │   ║
                         ║   └───────┴───────┘   ║
                         ╚═══════════╤═══════════╝
                                     │
              ┌──────────────────────┼──────────────────────┐
              │                      │                      │
              ▼                      ▼                      ▼
       ┌─────────────┐        ┌─────────────┐        ┌─────────────┐
       │   GO WASM   │───────►│ RUST WASM   │───────►│ JavaScript  │
       │   Kernel    │        │  Compute    │        │   Render    │
       │             │        │             │        │             │
       │ • Evolution │        │ • Boids     │        │ • Three.js  │
       │ • Genetics  │        │ • Math      │        │ • GPU       │
       └─────────────┘        └─────────────┘        └─────────────┘
              ▲                                             │
              │                                             │
              └─────────────────────────────────────────────┘
                           (Epoch Signaling)

    ⚡ All three runtimes read/write to the SAME SAB
    ⚡ Ping-pong buffers enable concurrent access without locks
    ⚡ Epoch counter provides frame-perfect synchronization
```

### The Circular Flow (Generic Pattern)

**For Entity Simulation (Boids, N-body, Agent systems):**
1. **Go Kernel** writes entity state to Buffer A (even epoch) or B (odd epoch)  
2. **Rust Compute** reads from active buffer, computes transforms/physics  
3. **JavaScript** reads results from SAB, uploads to GPU  
4. **Epoch increments** → roles flip, cycle continues

**For Audio/Video Streams:**
1. **Rust DSP** writes processed frames to inactive buffer  
2. **JavaScript Audio API** reads from active buffer  
3. **Go Kernel** monitors Buffer A vs B for dropped frames  
4. **Epoch increments** → seamless buffer swap

**For P2P Message Queues:**
1. **Go Kernel** accumulates gossip messages in inactive buffer  
2. **Rust Crypto** reads from active buffer, signs batch  
3. **JavaScript WebRTC** sends signed batch to peers  
4. **Epoch increments** → next batch begins

---

## Supervisor Orchestration

The **Supervisor** acts as the arbiter of the ring, ensuring data integrity:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    SUPERVISOR VERIFICATION                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│   Question: "I don't have X in SAB-A, do you have it in SAB-B?"    │
│                                                                     │
│   ┌───────────────┐                                                 │
│   │  SUPERVISOR   │                                                 │
│   │  (Arbiter)    │                                                 │
│   └───────┬───────┘                                                 │
│           │                                                         │
│    ┌──────┴──────┐                                                  │
│    ▼             ▼                                                  │
│  ┌─────┐      ┌─────┐                                               │
│  │Buf A│◄────►│Buf B│  Comparison / Verification                    │
│  └─────┘      └─────┘                                               │
│                                                                     │
│  • Detects write races                                              │
│  • Validates epoch consistency                                      │
│  • Triggers recovery on mismatch                                    │
└─────────────────────────────────────────────────────────────────────┘
```

### Supervisor Responsibilities

| Responsibility | How It's Achieved |
|----------------|-------------------|
| **Buffer Selection** | Reads epoch, returns correct buffer reference |
| **Write Ordering** | Ensures writers complete before epoch flip |
| **Integrity Check** | Optional CRC32 at buffer header for validation |
| **Stall Detection** | Watchdog timer on epoch staleness |
| **Recovery** | Re-sync from authoritative buffer on corruption |

---

## The Memory Bridge Problem

Go WASM and Rust WASM have **separate linear memories**. They cannot directly share pointers or memory regions. The SAB solves this by being a neutral ground that both can read/write via explicit copy operations.

```
┌─────────────────────────────────────────────────────────────────────┐
│                    MEMORY BRIDGE ARCHITECTURE                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│   ┌──────────────────┐                    ┌──────────────────┐     │
│   │   GO LINEAR MEM  │                    │  RUST LINEAR MEM │     │
│   │   (wasm_exec)    │                    │   (wasm32)       │     │
│   │                  │                    │                  │     │
│   │  ┌────────────┐  │    ┌─────────┐     │  ┌────────────┐  │     │
│   │  │ Bird State │──┼───►│   SAB   │◄────┼──│ SafeSAB    │  │     │
│   │  └────────────┘  │    │ (Bridge)│     │  └────────────┘  │     │
│   │                  │    └─────────┘     │                  │     │
│   └──────────────────┘                    └──────────────────┘     │
│                                                                     │
│   Problem: Go cannot call Rust functions, Rust cannot read Go mem  │
│   Solution: Both copy to/from SAB at designated offsets            │
│   Coordination: Epoch-based signaling prevents read/write races    │
└─────────────────────────────────────────────────────────────────────┘
```

### Why Memory Patching Was Needed

The `patch_wasm_memory.js` script patches WASM binaries to enable `SharedArrayBuffer` support:

```javascript
// Sets memory flags to Shared+HasMax (0x03)
buffer[memOffset] = 0x03;
```

This is required because:
1. Default WASM memory is NOT shared
2. SharedArrayBuffer requires the `shared` flag in the memory section
3. Go's `wasm_exec.js` doesn't set this flag by default
4. Without patching, Atomics operations fail

---

## SAB Memory Layout (v1.9+)

```
Offset (Abs) | Size    | Purpose
════════════|═════════|══════════════════════════════════════════
0x01000000  | 64B     | Atomic Flags Region (16 x i32)
            |         |   [IDX 12] Bird Epoch (u32)
            |         |   [IDX 13] Matrix Epoch (u32)
            |         |   [IDX 14] Active Buffer Flag (0=A, 1=B)
            |         |
0x01162000  | 2.36MB  | Bird State Buffer A (10k birds × 236 bytes)
0x013C2000  | 2.36MB  | Bird State Buffer B (10k birds × 236 bytes)
0x01622000  | 5.12MB  | Matrix Output Buffer A (10k birds × 8 parts × 64 bytes)
0x01B22000  | 5.12MB  | Matrix Output Buffer B (10k birds × 8 parts × 64 bytes)
```

**Total SAB Size**: 32MB (Light Tier Default). Ping-pong regions occupy ~15MB in the Arena.

---

## Epoch-Based Flip Signaling

### The Flip Protocol

```
1. Go Kernel increments System Epoch
2. All readers check: (epoch % 2) to determine active buffer
3. Writers write to OPPOSITE buffer (epoch % 2 == 0 → write B)
4. No explicit locks - atomic epoch is the synchronization primitive
```

### Implementation

**Go Kernel (Writer)**:
```go
// kernel/threads/supervisor/sab_bridge.go
func (b *SABBridge) FlipBuffers() {
    epoch := atomic.AddUint32(&b.epoch, 1)
    
    // Layout constants already include the 16MB base
    b.WriteRaw(OFFSET_BIRD_EPOCH, epoch)
    
    // Write to INACTIVE buffer (will become active next flip)
    // selection logic handles absolute addressing
    writeBuffer := b.getBirdBuffer(epoch + 1)
    copy(writeBuffer, b.computedBirdState)
}
```

**Rust Module (Reader/Writer)**:
```rust
// sdk/src/layout.rs
pub const OFFSET_BIRD_BUFFER_A: usize = 0x01162000;
pub const OFFSET_BIRD_BUFFER_B: usize = 0x013C2000;

// sdk/src/sab.rs
impl SafeSAB {
    pub fn get_read_buffer(&self, region: Region) -> &[u8] {
        let epoch = self.read_epoch();
        let base = if epoch % 2 == 0 { 
            region.buffer_a_offset // Absolute offset A
        } else { 
            region.buffer_b_offset // Absolute offset B
        };
        self.as_slice(base, region.size)
    }
}
```

**JavaScript (Reader)**:
```typescript
// src/wasm/sab.ts
const getActiveMatrixBuffer = (sab: SharedArrayBuffer): Float32Array => {
  const epoch = Atomics.load(typedArray, IDX_MATRIX_EPOCH);
  const offset = epoch % 2 === 0 ? CONSTS.OFFSET_MATRIX_BUFFER_A : CONSTS.OFFSET_MATRIX_BUFFER_B;
  return new Float32Array(sab, offset, BIRD_COUNT * 8 * 16);
};
```

---

## Implementation Phases

### Phase 1: SAB Layout Expansion
- [x] Update `layout.rs` / `layout.go` with dual buffer regions
- [x] Modify `kernel.ts` to initialize `__INOS_SAB_INT32__`
- [x] Update `SafeSAB::new()` to validate new layout

### Phase 2: SDK Buffer Accessors
- [x] Add `PingPongBuffer` to `sdk/src/pingpong.rs`
- [x] Define epoch indices in `layout.rs`
- [x] Add atomic notify interop

### Phase 3: Boids Migration
- [x] Replace `vec![]` allocations with `PingPongBuffer`
- [x] Operate directly on SAB slices (zero-copy)
- [x] Add epoch-aware flip at frame boundary

### Phase 4: Matrix Generation Migration
- [x] Update `math.rs` to use dual matrix buffers
- [x] JS reads from active buffer, Rust writes to inactive
- [x] Validate with 10k+ entity stress test (Ready for frontend testing)

### Phase 5: Go Kernel Integration
- [ ] Update evolution loop to use flip protocol
- [ ] Ensure genetic algorithm writes to inactive buffer
- [ ] Sync frame timing with Rust/JS

---

## Verification Checklist

- [ ] Zero allocations in hot path (`vec![]` removed)
- [ ] No frame tearing (epoch consistency)
- [ ] 10k entities @ 60fps (performance target)
- [ ] Memory stable over time (no GC spikes)
- [ ] All tests pass

---

## Related Files

| Layer | File | Changes Required |
|-------|------|------------------|
| Schema | `protocols/sab.capnp` | Add buffer region definitions |
| Go | `kernel/threads/supervisor/sab_bridge.go` | Flip protocol, dual buffers |
| SDK | `modules/sdk/src/sab.rs` | Buffer accessors, epoch helpers |
| Compute | `modules/compute/src/units/boids.rs` | Use buffer accessors |
| Compute | `modules/compute/src/units/math.rs` | Use buffer accessors |
| Frontend | `frontend/src/wasm/sab.ts` | Active buffer selection |
| Frontend | `frontend/app/components/ArchitecturalBoids.tsx` | Use typed buffer views |
