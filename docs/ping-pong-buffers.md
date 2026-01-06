# Ping-Pong Buffer Architecture

Zero-allocation, lock-free memory synchronization for INOS distributed compute.

## The Problem

Current per-frame allocations in hot paths:

```rust
// boids.rs - Every frame
let mut population_data = vec![0u8; count * 236];  // ~236KB @ 1k birds

// math.rs - Every frame  
let mut input_data = vec![0u8; input_size];   // ~236KB
let mut output_data = vec![0u8; output_size]; // ~512KB
```

**At 10k+ entities**: ~47MB/sec allocations → GC pressure, frame stutters.

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

### The Circular Flow

1. **Go Kernel** writes bird state to Buffer A (even epoch) or B (odd epoch)
2. **Rust Compute** reads from active buffer, computes matrices, writes to matrix buffer
3. **JavaScript** reads matrices from SAB, uploads to GPU
4. **Epoch increments** → roles flip, cycle continues

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
Offset      | Size    | Purpose
════════════|═════════|══════════════════════════════════════════
0x0000      | 64B     | Atomic Flags Region (16 x i32)
            |         |   [IDX 12] Bird Epoch (u32)
            |         |   [IDX 13] Matrix Epoch (u32)
            |         |   [IDX 14] Active Buffer Flag (0=A, 1=B)
            |         |
0x162000    | ~2.3MB  | Bird State Buffer A (10k birds × 236 bytes)
0x3C2000    | ~2.3MB  | Bird State Buffer B (10k birds × 236 bytes)
0x622000    | 640KB   | Matrix Output Buffer A (10k × 64 bytes)
0x6C2000    | 640KB   | Matrix Output Buffer B (10k × 64 bytes)
```

**Total SAB Size**: 16MB (Default). Ping-pong regions occupy ~6MB in the Arena.

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
// threads/supervisor/sab_bridge.go
func (b *SABBridge) FlipBuffers() {
    epoch := atomic.AddUint32(&b.epoch, 1)
    binary.LittleEndian.PutUint32(b.sab[0:4], epoch)
    
    // Write to INACTIVE buffer (will become active next flip)
    writeBuffer := b.getBirdBuffer(epoch + 1)
    copy(writeBuffer, b.computedBirdState)
}

func (b *SABBridge) getBirdBuffer(epoch uint32) []byte {
    if epoch % 2 == 0 {
        return b.sab[0x0100:0x240100]  // Buffer A
    }
    return b.sab[0x240100:0x480200]    // Buffer B
}
```

**Rust Module (Reader/Writer)**:
```rust
// sdk/src/sab.rs
impl SafeSAB {
    pub fn get_read_buffer(&self, region: Region) -> &[u8] {
        let epoch = self.read_epoch();
        let base = if epoch % 2 == 0 { 
            region.buffer_a_offset 
        } else { 
            region.buffer_b_offset 
        };
        unsafe { std::slice::from_raw_parts(self.ptr.add(base), region.size) }
    }
    
    pub fn get_write_buffer(&self, region: Region) -> &mut [u8] {
        let epoch = self.read_epoch();
        let base = if epoch % 2 == 0 { 
            region.buffer_b_offset  // Write to OPPOSITE
        } else { 
            region.buffer_a_offset 
        };
        unsafe { std::slice::from_raw_parts_mut(self.ptr.add(base), region.size) }
    }
}
```

**JavaScript (Reader)**:
```typescript
// src/wasm/sab.ts
const getActiveMatrixBuffer = (sab: SharedArrayBuffer): Float32Array => {
  const epoch = new Uint32Array(sab, 0x08, 1)[0]; // Matrix epoch
  const offset = epoch % 2 === 0 ? 0x480200 : 0x980200;
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
