# INOS TODO

## GPU Module Enhancements

### Hardware-Specific Shader Optimization
**Priority**: Medium | **Impact**: +30-50% performance on AMD, +20-30% on Apple

```javascript
// JS Layer: Detect GPU vendor at initialization
const adapter = await navigator.gpu.requestAdapter();
const info = await adapter.requestAdapterInfo();

const vendor = info.vendor.toLowerCase();
let gpuType = 'unknown';
if (vendor.includes('nvidia')) gpuType = 'nvidia';
else if (vendor.includes('amd')) gpuType = 'amd';
else if (vendor.includes('apple')) gpuType = 'apple';

// Pass to Rust
wasmModule.set_gpu_vendor(gpuType);
```

**Implementation Steps**:
1. Add `set_gpu_vendor()` method to `GpuJob`
2. Create hardware-specific shader variants:
   - `matmul_nvidia.wgsl` (32x32 tiles, current)
   - `matmul_amd.wgsl` (64x64 tiles, more shared mem)
   - `matmul_apple.wgsl` (16x16 tiles, less shared mem)
3. Load appropriate shader based on detected vendor
4. Benchmark to verify performance gains

**Expected Performance**:
- NVIDIA: +10-20% (already near-optimal)
- AMD: +30-50% (64x64 tiles better utilize wave size)
- Apple: +20-30% (16x16 reduces shared mem pressure)

**Files to Modify**:
- `modules/compute/src/jobs/gpu.rs` - Add vendor detection
- `modules/compute/src/jobs/gpu_shaders/` - Add vendor-specific variants
- `modules/sdk/src/lib.rs` - Add JS detection code

---

## Other TODOs

### Audio Module
- [ ] Implement SAB-native zero-copy processing
- [ ] Add epoch signaling for reactive mutation
- [ ] Benchmark WASM SIMD FFT performance

### Compute Module
- [x] Add ML module (separate from GPU - `modules/ml`)
- [x] Implement Ring Buffer Architecture (Phase 15)
- [x] Refactor Science/Mining/Compute to new Reactor API
- [ ] Add Video transcoding module
- [ ] Implement workflow orchestration

### P2P Mesh
- [x] Implement chunking for large resources (See `modules/ml/src/p2p/chunks.rs`)
- [x] Implement `sendMessage` syscall for authenticated peer messaging
- [x] Refactor science module P2P bridge to use syscalls instead of raw SAB writes
- [ ] Add Brotli compression for WASM modules
- [x] Test mesh replication across nodes (In progress via `MeshCoordinator`)

### Syscalls
- [x] Implement `sendMessage` syscall in `syscall.capnp`
- [x] Add Go kernel handler in `signal_loop.go`
- [x] Implement Rust SDK `SyscallClient::send_message()`
- [x] Integrate with `MeshCoordinator` for message routing
- [ ] Add timeout handling for undelivered messages
- [ ] Implement priority queues for QoS
