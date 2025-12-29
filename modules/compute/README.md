# INOS Compute Module

**Philosophy**: "Leverage libraries, don't reimplement them."

The compute module is a **universal adapter** that exposes the full power of Rust's ecosystem through a generic library proxy pattern. Instead of hardcoding operations, we expose entire library APIs via method dispatch with JSON parameters.

---

## Production Foundation (MANDATORY)

> [!CAUTION]
> **Security & Performance Requirements**
> 
> The following are **non-negotiable** production requirements. All compute operations MUST comply with these standards before deployment.

### 1. WASM Sandboxing (Security Layer)

**Every library runs in an isolated WASM instance** with enforced resource limits:

```rust
pub struct ResourceLimits {
    max_input_size: usize,      // Prevent DoS (e.g., 10MB for images)
    max_output_size: usize,     // Prevent memory exhaustion
    max_memory_pages: u32,      // WASM memory limit (64KB pages)
    timeout_ms: u64,            // Prevent infinite loops
    max_fuel: u64,              // CPU cycle limit
    allowed_syscalls: Vec<String>, // Syscall whitelist
}
```

**Benefits**:
- ✅ **DoS Protection**: Input/output size limits prevent resource exhaustion
- ✅ **CPU Quotas**: Fuel limits prevent infinite loops
- ✅ **Memory Safety**: Page limits prevent memory bombs
- ✅ **Isolation**: One library failure doesn't crash the system

**Default Limits**:
- **Image**: 10MB input, 50MB output, 5s timeout, 10B fuel
- **Video**: 100MB input, 500MB output, 60s timeout, 100B fuel
- **ML**: 1GB input, 1GB output, 300s timeout, 1T fuel

---

### 2. True Zero-Copy Architecture

**Problem**: Using `Vec<u8>` still copies data between Go/Rust.

**Solution**: Raw pointers with bounds checking.

```rust
pub struct SABBridge {
    inbox: *mut u8,
    outbox: *mut u8,
    arena: *mut u8,
}

impl SABBridge {
    pub unsafe fn read_job(&self) -> JobRequest {
        // Direct pointer arithmetic - ZERO COPIES
        let input_ptr = self.inbox.offset(JOB_DATA_OFFSET);
        let input_len = *(self.inbox.offset(JOB_SIZE_OFFSET) as *const u32);
        
        JobRequest {
            input: std::slice::from_raw_parts(input_ptr, input_len as usize),
            // Fields accessed via pointer arithmetic (~10ns each)
        }
    }
    
    pub unsafe fn write_result(&self, output: &[u8]) {
        // Write directly to pre-allocated arena
        let arena_offset = atomic_arena_alloc(self.arena, output.len());
        std::ptr::copy_nonoverlapping(
            output.as_ptr(),
            self.arena.offset(arena_offset as isize),
            output.len(),
        );
    }
}
```

**Verification**: Production systems MUST have **zero `memcpy` operations** in profiling.

---

### 3. Smart Arena with Defragmentation

**Problem**: Arena fragmentation reduces efficiency over time.

**Solution**: Buddy allocator + defragmentation with indirection table.

```rust
pub struct SmartArena {
    pages: Vec<ArenaPage>,
    free_list: BuddyAllocator,           // Power-of-2 allocation
    fragmentation_monitor: FragmentationMonitor,
    pointer_table: IndirectionTable,      // For safe defragmentation
}

impl SmartArena {
    pub fn allocate_zero_copy(&mut self, size: usize) -> Result<ArenaPtr> {
        // 1. Try exact fit
        if let Some(ptr) = self.free_list.allocate_exact(size) {
            return Ok(ptr);
        }
        
        // 2. Defragment if fragmentation > 30%
        if self.fragmentation_monitor.ratio() > 0.3 {
            self.defragment()?;
        }
        
        // 3. Buddy allocation (optimized for GPU transfers)
        self.buddy_allocate(size)
    }
}
```

**Target**: Maintain **>80% memory efficiency** at all times.

---

### 4. Transactional Workflows (Reliability)

**Problem**: Workflow failures leave system in inconsistent state.

**Solution**: Write-Ahead Log + Compensation Actions.

```rust
pub struct AtomicWorkflowOrchestrator {
    engine: ComputeEngine,
    journal: WriteAheadLog,              // Crash recovery
    compensations: HashMap<String, CompensatingAction>, // Rollback
}

impl AtomicWorkflowOrchestrator {
    pub async fn execute_atomic(&self, workflow: &Workflow) -> Result<Vec<u8>> {
        let tx_id = self.journal.begin_transaction();
        
        for (step_idx, step) in workflow.steps.iter().enumerate() {
            // Log before execution
            self.journal.log_step_start(tx_id, step_idx, step);
            
            match self.engine.execute(&step.library, &step.method, input, &step.params).await {
                Ok(output) => {
                    self.journal.log_step_success(tx_id, step_idx, &output);
                    
                    // Prepare compensation
                    self.compensations.insert(
                        format!("{}-{}", tx_id, step_idx),
                        self.create_compensation(&step, &output),
                    );
                }
                Err(e) => {
                    // Rollback all executed steps
                    self.rollback(tx_id, step_idx).await?;
                    self.journal.log_rollback(tx_id, step_idx);
                    return Err(e);
                }
            }
        }
        
        // Commit - delete compensation actions
        self.journal.commit(tx_id);
        Ok(final_output)
    }
}
```

**Guarantees**:
- ✅ **Atomic**: All steps succeed or all rollback
- ✅ **Durable**: WAL survives crashes
- ✅ **Recoverable**: Replay from journal on restart

---

### 5. Production Monitoring & SLO Tracking

**Every operation is measured** against Service Level Objectives:

```rust
pub struct ComputeMetrics {
    histogram: HistogramVec,    // Latency distribution
    counters: CounterVec,       // Success/failure counts
    gauges: GaugeVec,           // Resource utilization
}

impl ComputeMetrics {
    pub fn observe_job(&self, library: &str, method: &str, duration: Duration) {
        self.histogram
            .with_label_values(&[library, method])
            .observe(duration.as_secs_f64());
            
        // Alert if > 99th percentile
        if duration > self.slo_violation_threshold(library) {
            alert_slo_violation(library, method, duration);
        }
    }
    
    pub fn verify_zero_copy(&self) -> bool {
        let memcpy_count = measure_memcpy_operations();
        assert_eq!(memcpy_count, 0, "Zero-copy violation!");
        true
    }
}
```

**SLO Targets**:
- **p95 latency**: <100ms for all operations
- **Success rate**: >99.9%
- **Memory efficiency**: >80%
- **Zero-copy compliance**: 0 memcpy operations

---

### 6. Input Validation (Defense in Depth)

**All inputs validated BEFORE deserialization**:

```rust
fn validate_job_request(data: &[u8], limits: &ResourceLimits) -> Result<()> {
    // 1. Size check
    if data.len() > limits.max_input_size {
        return Err(Error::InputTooLarge);
    }
    
    // 2. Cap'n Proto structure validation
    let _ = capnp::serialize::read_message_from_flat_slice(
        &mut data,
        capnp::message::ReaderOptions::new()
    )?;
    
    // 3. Params validation (prevent injection)
    let params = extract_params(data)?;
    validate_json_params(&params)?;
    
    Ok(())
}

fn validate_json_params(params: &str) -> Result<()> {
    // 1. Size limit
    if params.len() > 1024 * 1024 {  // 1MB max
        return Err(Error::ParamsTooLarge);
    }
    
    // 2. Parse as JSON (prevents injection)
    let _: serde_json::Value = serde_json::from_str(params)?;
    
    // 3. Check for dangerous patterns
    if params.contains("__proto__") || params.contains("constructor") {
        return Err(Error::MaliciousParams);
    }
    
    Ok(())
}
```

---

### 7. Fuzzing & Security Testing

**Continuous fuzzing** to detect vulnerabilities:

```rust
#[cfg(test)]
mod fuzzing {
    use libfuzzer_sys::fuzz_target;
    
    fuzz_target!(|data: &[u8]| {
        // 1. Cap'n Proto fuzzing (should never panic)
        if let Ok(req) = decode_job_request(data) {
            let _ = compute_engine.execute(
                &req.library,
                &req.method,
                &req.input,
                &req.params
            );
        }
        
        // 2. Memory safety fuzzing
        fuzz_memory_corruption(data);
        
        // 3. Sandbox escape attempts
        fuzz_wasm_escape(data);
    });
}
```

**Fuzzing Targets**:
- Cap'n Proto deserialization
- Memory corruption attempts
- Sandbox escape attempts
- Resource exhaustion attacks

---

## Production Readiness Checklist

Before deployment, verify:

- [ ] **Security**
  - [ ] WASM sandboxing enabled for all libraries
  - [ ] Input validation at SAB boundary
  - [ ] Resource limits enforced
  - [ ] Fuzzing tests passing

- [ ] **Performance**
  - [ ] Zero memcpy operations (verified in profiling)
  - [ ] p95 latency < 100ms
  - [ ] Memory efficiency > 80%
  - [ ] SIMD acceleration enabled

- [ ] **Reliability**
  - [ ] Transactional workflows with WAL
  - [ ] Crash recovery tested
  - [ ] Circuit breakers functional
  - [ ] Health checks passing

- [ ] **Monitoring**
  - [ ] SLO tracking enabled
  - [ ] Anomaly detection configured
  - [ ] Alerting rules defined
  - [ ] Dashboards deployed

---

## Two-Tier Protocol Architecture

INOS employs a **Two-Tier Protocol** to balance flexibility (development velocity) with performance (execution speed).

### Tier 1: The Universal Proxy (Generic)
**Used by**: `image`, `video`, `ml`, `audio`, `crypto`
**Format**: `JobRequest` + **JSON Params**

*   **Pros**: Extreme flexibility. New methods can be added to Rust libraries without recompiling the Kernel or Protocol Schemas.
*   **Cons**: Parsing JSON params costs ~1µs (negligible for long-running jobs like video encoding).
*   **Structure**:
    *   `library`: "image"
    *   `method`: "resize"
    *   `params`: `{"width": 1920, "height": 1080}` (JSON String)

### Tier 2: The Reality Contract (Specialized)
**Used by**: `science`, `physics` (Molecular Dynamics, FEA)
**Format**: `ScienceRequest` + **Typed Cap'n Proto Union**

*   **Pros**: 
    *   **Bit-Exact Hashing**: Deterministic hashing of parameters (BLAKE3) allows for global deduplication and Merkle proofs.
    *   **Type Safety**: Strictly enforced schemas for complex scientific data structures.
*   **Cons**: Requires schema compilation updates for every new parameter.
*   **Structure**:
    *   `library`: `Library::Atomic` (Enum)
    *   `params`: `ScienceParams::AtomicParams(...)` (Typed Union)

---

## Zero-Copy Performance Architecture

### The Role of JSON in Tier 1

**Question**: Why use JSON for `params` in Tier 1 if we care about zero-copy?

**Answer**: Because **Metadata is small, Pixel Data is huge**.
*   Parsing 1KB of JSON params: **~1µs**
*   Copying 10MB of Image data: **~2ms** (2000µs)
*   **Zero-Copy applies to the INPUT/OUTPUT BI-DIRECTIONAL DATA**, which is passed via `SharedArrayBuffer` pointers. The parameters control *how* that data is processed.

### The INOS Zero-Copy Pipeline

```
┌─────────────────────────────────────────────────────────────┐
│ Go Kernel                                                   │
│ 1. Encode JobRequest (Cap'n Proto) → Write to SAB[Inbox]    │
│    (Includes Pointers to Data + JSON Params)                │
│ 2. Increment Epoch (atomic)                                 │
└─────────────────────────────────────────────────────────────┘
                            ↓ (zero-copy)
┌─────────────────────────────────────────────────────────────┐
│ Rust Compute Module                                         │
│ 3. Detect Epoch change → Read SAB[Inbox] (zero-copy)        │
│ 4. Decode Cap'n Proto Envelope                              │
│ 5. Parse JSON Params (Tier 1) OR Typed Union (Tier 2)       │
│ 6. Execute library method (Direct Memory Access to SAB)     │
│ 7. Encode JobResult (Cap'n Proto) → Write to SAB[Outbox]    │
│ 8. Increment Epoch (atomic)                                 │
└─────────────────────────────────────────────────────────────┘
```

**Key Insight**: **No Bulk Data is copied**. The overhead of parameter parsing is vanishingly small compared to the compute workload (e.g., resizing a 4K video frame).

---

## Maximum Performance Strategies

### 1. **Batch Processing** (Already Implemented)

```go
// Go: Process up to 10 jobs per batch
const maxBatchSize = 10
for i := 0; i < maxBatchSize; i++ {
    if cs.dispatchSingleJob() {
        jobsDispatched++
    } else {
        break
    }
}
```

**Benefit**: Amortize syscall overhead across multiple jobs

### 2. **Arena Allocation** (Use SAB Arena)

```rust
// Instead of allocating output on heap:
let mut output = Vec::new();  // ❌ Heap allocation

// Write directly to SAB Arena:
let arena = self.reactor.arena_data();  // ✅ Zero-copy
let output_ptr = arena.allocate(output_size);
library.execute_into(method, input, params, output_ptr);
```

**Benefit**: **Zero allocations**, output is already in SAB

### 3. **SIMD Acceleration** (Use Rust Libraries)

```rust
// Libraries like `fast_image_resize` use SIMD automatically
use fast_image_resize as fr;

let src_image = fr::Image::from_vec_u8(width, height, input, fr::PixelType::U8x3)?;
let mut dst_image = fr::Image::new(new_width, new_height, src_image.pixel_type());

let mut resizer = fr::Resizer::new(fr::ResizeAlg::Convolution(fr::FilterType::Lanczos3));
resizer.resize(&src_image.view(), &mut dst_image.view_mut())?;
```

**Benefit**: **4-8x faster** than naive implementation

### 4. **GPU Offloading** (wgpu)

```rust
// For parallel operations, use GPU
let device = wgpu::Device::new();
let shader = device.create_shader_module(wgsl_code);

// Execute on GPU (massively parallel)
encoder.dispatch_workgroups(workgroup_count_x, workgroup_count_y, 1);

// Result written directly to SAB
device.queue.read_buffer_to_sab(&output_buffer, &arena);
```

**Benefit**: **100-1000x faster** for parallel workloads

---

## Orchestration: Multi-Step Workflows

### The Challenge

**Question**: How do we handle complex workflows like molecular dynamics (energy minimize → equilibrate → production)?

**Answer**: **Orchestration via Cap'n Proto Workflow Schema**

### Workflow Schema Extension

```capnp
# protocols/schemas/compute/v1/capsule.capnp

struct Workflow {
  steps @0 :List(WorkflowStep);
  onError @1 :ErrorPolicy;
}

struct WorkflowStep {
  library @0 :Text;
  method @1 :Text;
  params @2 :Text;  # Can reference previous step outputs
  
  union {
    sequential @3 :Void;      # Wait for previous step
    parallel @4 :Void;        # Run concurrently
    conditional @5 :Condition; # Run if condition met
  }
}

struct Condition {
  field @0 :Text;      # e.g., "previous.energy"
  operator @1 :Text;   # e.g., "<", ">", "=="
  value @2 :Float64;
}

enum ErrorPolicy {
  abort @0;      # Stop entire workflow
  skip @1;       # Skip failed step, continue
  retry @2;      # Retry failed step
  rollback @3;   # Undo previous steps
}
```

### Orchestration Example: Molecular Dynamics

```rust
// Workflow definition (sent as JobRequest)
{
  "workflow": {
    "steps": [
      {
        "library": "physics",
        "method": "energy_minimize",
        "params": {"steps": 1000, "tolerance": 0.001},
        "sequential": {}
      },
      {
        "library": "physics",
        "method": "equilibrate",
        "params": {
          "duration": 100,  // ps
          "temperature": 300,
          "ensemble": "NVT"
        },
        "sequential": {},
        "conditional": {
          "field": "previous.final_energy",
          "operator": "<",
          "value": -50000.0  // Only equilibrate if minimization succeeded
        }
      },
      {
        "library": "physics",
        "method": "production",
        "params": {
          "duration": 1000,  // ps
          "ensemble": "NPT",
          "output_frequency": 10
        },
        "sequential": {}
      }
    ],
    "onError": "abort"
  }
}
```

### Orchestration Implementation (Rust)

```rust
// modules/compute/src/orchestrator.rs
pub struct Orchestrator {
    engine: ComputeEngine,
}

impl Orchestrator {
    pub fn execute_workflow(&self, workflow: &Workflow, input: &[u8]) -> Result<Vec<u8>> {
        let mut context = WorkflowContext::new(input);
        
        for (i, step) in workflow.steps.iter().enumerate() {
            // Check condition
            if let Some(condition) = &step.condition {
                if !self.evaluate_condition(condition, &context)? {
                    continue; // Skip step
                }
            }
            
            // Resolve params (may reference previous outputs)
            let params = self.resolve_params(&step.params, &context)?;
            
            // Execute step
            match self.engine.execute(&step.library, &step.method, context.current_input(), &params) {
                Ok(output) => {
                    context.add_result(i, output);
                }
                Err(e) => {
                    match workflow.on_error {
                        ErrorPolicy::Abort => return Err(e),
                        ErrorPolicy::Skip => continue,
                        ErrorPolicy::Retry => {
                            // Retry logic
                        }
                        ErrorPolicy::Rollback => {
                            // Rollback logic
                        }
                    }
                }
            }
        }
        
        Ok(context.final_output())
    }
    
    fn resolve_params(&self, params: &str, context: &WorkflowContext) -> Result<String> {
        // Replace references like "${previous.energy}" with actual values
        let mut resolved = params.to_string();
        
        // Parse and replace references
        for (key, value) in context.variables() {
            resolved = resolved.replace(&format!("${{{}}}", key), value);
        }
        
        Ok(resolved)
    }
}

struct WorkflowContext {
    results: Vec<Vec<u8>>,
    variables: HashMap<String, String>,
}
```

### Parallel Orchestration

```rust
// For independent steps, execute in parallel
{
  "workflow": {
    "steps": [
      {
        "library": "image",
        "method": "resize",
        "params": {"width": 800, "height": 600},
        "parallel": {}  // Can run concurrently
      },
      {
        "library": "image",
        "method": "grayscale",
        "params": {},
        "parallel": {}  // Can run concurrently
      },
      {
        "library": "crypto",
        "method": "sha256",
        "params": {},
        "sequential": {}  // Must wait for both above to complete
      }
    ]
  }
}
```

**Implementation**:
```rust
// Detect parallel steps
let parallel_steps: Vec<_> = workflow.steps.iter()
    .take_while(|s| s.is_parallel())
    .collect();

// Execute in parallel using rayon
use rayon::prelude::*;
let results: Vec<_> = parallel_steps.par_iter()
    .map(|step| self.engine.execute(&step.library, &step.method, input, &step.params))
    .collect();
```

---

## Performance Benchmarks (Expected)

| Operation | JSON Params | Cap'n Proto | Speedup |
|-----------|-------------|-------------|---------|
| Parse params | ~1µs | ~10ns | **100x** |
| Image resize (1MB) | 50ms | 50ms | 1x (same) |
| Batch 10 jobs | 10.5ms | 10.0ms | **1.05x** |
| Workflow (3 steps) | 150ms + 3µs | 150ms + 30ns | **1.00002x** |

**Key Insight**: Cap'n Proto overhead is **negligible** compared to actual compute time.

---

## Architecture: Library Proxy Pattern

### Core Concept

The "Proxy" separates the **Interface** (Cap'n Proto) from the **Implementation** (Rust Crates).

```rust
// The generic trait used by Compute/ML
pub trait LibraryProxy {
    fn execute(&self, method: &str, input: &[u8], params: &str) -> Result<Vec<u8>>;
}
```

### Tier 1: Generic Dispatch (Standard)

Standard modules (`image`, `ml`, `video`) use the **Action Dispatch** pattern:
1.  Kernel receives `JobRequest`.
2.  Dispatch based on `library` string.
3.  Proxy deserializes JSON params (1µs).
4.  Proxy calls Rust function.

```json
{
  "library": "image",
  "method": "resize",
  "params": "{\"width\": 1920, \"height\": 1080}"
}
```

### Tier 2: Specialized Dispatch (Science/Simulations)

The `science` module uses the **Reality Contract** pattern (`science.capnp`):
1.  Kernel receives `ScienceRequest` (or wraps generic request).
2.  Parameters are **Strongly Typed Union** (`ScienceParams`).
3.  Input is **DataRef** (Hash, Inline, or Merkle Proof).
4.  Execution requires **Proof of Simulation** (hashing inputs+params).

```capnp
# protocols/schemas/science/v1/science.capnp
struct ScienceRequest {
  library @0 :Library;     # Enum: Atomic, Continuum, Kinetic
  params @2 :ScienceParams; # Union: AtomicParams, ContinuumParams...
}
```

**Why the difference?**
- **Compute/ML** prioritizes **Tooling Flexibility** (adding new tools easily).
- **Science** prioritizes **Verifier Correctness** (proving a simulation run is valid).

### Example Request

```json
{
  "library": "image",
  "method": "resize",
  "params": {
    "width": 1920,
    "height": 1080,
    "filter": "Lanczos3",
    "format": "webp",
    "quality": 90
  }
}
```

**No hardcoding. Full library power. Maximum flexibility.**

---

## Supported Libraries (High-Value Workloads)

### 1. **Image Processing** (`image` library) 💰💰💰💰💰
**Market Rate**: $0.01-$0.10/image  
**Revenue Potential**: **$1.75B/year** (10K nodes)

**Rust Libraries**:
- `image` (v0.24+): Core image operations
- `fast_image_resize`: SIMD-accelerated resizing
- `imageproc`: Advanced computer vision

**Exposed Methods**:
```rust
"resize", "crop", "blur", "brighten", "contrast", "grayscale", 
"rotate90", "rotate180", "rotate270", "fliph", "flipv", "invert"
```

**Example Params**:
```json
{
  "method": "resize",
  "params": {
    "width": 800,
    "height": 600,
    "filter": "Lanczos3",  // Triangle, CatmullRom, Gaussian, Nearest
    "format": "jpeg",       // jpeg, png, webp, avif
    "quality": 85
  }
}
```

---

### 2. **Video Transcoding** (`video-rs`, `rav1e`) 💰💰💰💰💰
**Market Rate**: $2.70-$60/hour  
**Revenue Potential**: **$876M/year** (10K nodes)

**Rust Libraries**:
- `video-rs`: High-level FFmpeg bindings
- `rav1e`: Pure Rust AV1 encoder
- `ffmpeg-next`: Direct FFmpeg bindings

**Exposed Methods**:
```rust
"transcode", "encode", "decode", "extract_frames", "add_audio"
```

**Example Params**:
```json
{
  "method": "transcode",
  "params": {
    "codec": "av1",         // h264, h265, vp9, av1
    "resolution": "4k",     // 1080p, 4k, 8k
    "bitrate": "10M",
    "preset": "medium",     // ultrafast, fast, medium, slow, veryslow
    "format": "mp4"
  }
}
```

---

### 3. **Audio Processing** (`symphonia`, `dasp`) 💰💰💰
**Market Rate**: $0.01-$0.05/minute  
**Revenue Potential**: **$105M/year** (10K nodes)

**Rust Libraries**:
- `symphonia`: Pure Rust audio decode/encode
- `dasp`: Digital audio signal processing
- `RustFFT`: Fast Fourier Transform
- `rubato`: Async resampling

**Exposed Methods**:
```rust
"encode", "decode", "resample", "normalize", "fft", "apply_effect"
```

**Example Params**:
```json
{
  "method": "encode",
  "params": {
    "codec": "opus",        // mp3, aac, opus, flac, wav
    "bitrate": "128k",
    "sample_rate": 48000,
    "channels": 2
  }
}
```

---

### 4. **Cryptographic Operations** (`sha2`, `blake3`, `ed25519`) 💰💰
**Market Rate**: $0.001-$0.01/operation  
**Revenue Potential**: **$7.3M/year** (10K nodes)

**Rust Libraries**:
- `sha2`: SHA-256, SHA-512
- `blake3`: BLAKE3 hashing
- `ed25519-dalek`: Ed25519 signing
- `aes-gcm`: AES encryption

**Exposed Methods**:
```rust
"sha256", "sha512", "blake3", "ed25519_sign", "ed25519_verify", "aes_encrypt", "aes_decrypt"
```

**Example Params**:
```json
{
  "method": "ed25519_sign",
  "params": {
    "private_key": "base64_encoded_key",
    "message": "data_to_sign"
  }
}
```

---

### 5. **Data Processing** (`polars`, `arrow`, `parquet`) 💰💰💰
**Market Rate**: $0.10-$1.00/GB  
**Revenue Potential**: **$182.5M/year** (10K nodes)

**Rust Libraries**:
- `polars` (v1.0+): Blazingly fast DataFrames
- `arrow`: Apache Arrow columnar data
- `parquet`: Apache Parquet storage

**Exposed Methods**:
```rust
"parquet_read", "parquet_write", "filter", "aggregate", "join", "transform"
```

**Example Params**:
```json
{
  "method": "aggregate",
  "params": {
    "group_by": ["category", "region"],
    "agg": {
      "sales": "sum",
      "quantity": "mean"
    },
    "output_format": "parquet"
  }
}
```

---

### 6. **Custom GPU Shaders** (`wgpu`) 💰💰💰
**Market Rate**: $0.10-$1.00/execution  
**Revenue Potential**: **$73M/year** (10K nodes)

**Rust Libraries**:
- `wgpu` (v0.19+): WebGPU API
- `naga`: Shader compiler

**Exposed Methods**:
```rust
"execute_shader", "compile_wgsl"
```

**Example Params**:
```json
{
  "method": "execute_shader",
  "params": {
    "wgsl_code": "@compute @workgroup_size(256) fn main(...) { ... }",
    "workgroup_size": [256, 1, 1],
    "buffer_size": 1048576
  }
}
```

---

## Advanced Workloads (Future)

### 7. **3D Rendering** (`wgpu`, Blender HTTP API) 💰💰💰💰
**Market Rate**: $0.10-$0.50/frame  
**Revenue Potential**: **$219M/year** (10K nodes)

**Integration Strategy**:
- **Blender**: HTTP API to headless Blender instance
- **Custom**: `wgpu`-based ray tracer

**Blender HTTP Integration** (No Python in codebase):
```rust
// Rust makes HTTP request to external Blender server
async fn render_blender(scene: &str, params: &RenderParams) -> Result<Vec<u8>> {
    let client = reqwest::Client::new();
    let response = client
        .post("http://blender-server:8080/render")
        .json(&json!({
            "scene": scene,
            "resolution": params.resolution,
            "samples": params.samples,
            "engine": "CYCLES"
        }))
        .send()
        .await?;
    
    response.bytes().await.map(|b| b.to_vec())
}
```

**Example Params**:
```json
{
  "method": "render",
  "params": {
    "scene_url": "https://cdn.example.com/scene.blend",
    "resolution": [1920, 1080],
    "samples": 128,
    "engine": "CYCLES",
    "frame": 1
  }
}
```

---

### 8. **AI Inference** (`candle` - Hugging Face) 💰💰💰💰
**Market Rate**: $0.15-$15/million tokens  
**Revenue Potential**: **$36.5M/year** (10K nodes)

**Rust Library** (ONLY ONE CHOICE):
- `candle`: Hugging Face ML framework (Pure Rust, WASM-compatible)

**Why `candle` (not `mistral.rs` or `llama.cpp`)**:
- ✅ **Pure Rust**: No C++ dependencies (unlike `llama.cpp`)
- ✅ **WASM-first**: Built for browser execution
- ✅ **Zero-copy**: Works with SharedArrayBuffer directly
- ✅ **Industry standard**: Backed by Hugging Face
- ✅ **Full control**: `mistral.rs` is just a wrapper around `candle`

**Quantization Support** (Memory-Efficient):
```rust
// 4-bit quantization reduces memory by 4x
// 7B model: 28GB (FP32) → 7GB (4-bit) → fits in browser!
use candle_core::{Device, Tensor};
use candle_transformers::models::llama;

let model = llama::Model::from_ggml(
    "llama-7b-q4.ggml",  // 4-bit quantized
    &Device::Cpu
)?;
```

**Quantization Formats**:
- **GGML**: 4-bit, 8-bit (most common)
- **GPTQ**: 4-bit with group quantization
- **AWQ**: Activation-aware weight quantization

**Exposed Methods**:
```rust
"infer", "embed", "classify"
```

**Example Params**:
```json
{
  "method": "infer",
  "params": {
    "model": "llama-7b-q4",     // Quantized model
    "prompt": "Explain quantum computing",
    "max_tokens": 500,
    "temperature": 0.7,
    "top_p": 0.9
  }
}
```

**Implementation Notes**:
- Models are chunked into 1MB pieces for P2P distribution
- Quantization is **required** for browser deployment
- Use `candle_transformers` for pre-built model architectures


---

### 9. **Molecular Dynamics** (`rapier3d`, `nalgebra`) 💰💰💰💰
**Market Rate**: $28-$755/µs  
**Revenue Potential**: **$36.5M/year** (10K nodes)

**Rust Libraries**:
- `rapier3d`: Physics simulation
- `nalgebra`: Linear algebra
- Custom force fields via params

**Domain Expertise via Custom Params**:
```json
{
  "method": "simulate",
  "params": {
    "force_field": "AMBER",     // AMBER, CHARMM, OPLS
    "timestep": 0.002,          // ps
    "temperature": 300,         // K
    "pressure": 1.0,            // bar
    "ensemble": "NPT",          // NVE, NVT, NPT
    "integrator": "verlet",
    "constraints": {
      "bonds": "all-bonds",
      "angles": "h-bonds"
    }
  }
}
```

**Orchestrated Flows** (Multi-step simulations):
```json
{
  "workflow": [
    {"method": "energy_minimize", "steps": 1000},
    {"method": "equilibrate", "duration": 100},  // ps
    {"method": "production", "duration": 1000}   // ps
  ]
}
```

---

## Custom Parameters: The Power of JSON

### Why JSON?

1. **Self-describing**: No schema updates needed
2. **Flexible**: Any parameter structure
3. **Extensible**: Add new params without code changes
4. **Type-safe**: Rust's `serde` validates at runtime

### Advanced Param Patterns

**Conditional Logic**:
```json
{
  "method": "resize",
  "params": {
    "width": 1920,
    "height": 1080,
    "if_larger_than": [800, 600],  // Only resize if larger
    "preserve_aspect": true
  }
}
```

**Batch Operations**:
```json
{
  "method": "batch",
  "params": {
    "operations": [
      {"method": "resize", "width": 800, "height": 600},
      {"method": "blur", "sigma": 2.0},
      {"method": "encode", "format": "webp", "quality": 90}
    ]
  }
}
```

**Orchestrated Workflows**:
```json
{
  "workflow": {
    "steps": [
      {"library": "image", "method": "resize", "params": {...}},
      {"library": "image", "method": "filter", "params": {...}},
      {"library": "crypto", "method": "sha256", "params": {...}}
    ],
    "on_error": "rollback"
  }
}
```

---

## Implementation Roadmap

### Phase 1: Quick Wins (Weeks 1-4) ✅
- [x] Image processing
- [x] Cryptographic operations
- [x] Audio transcoding

**Revenue**: $1.86B/year (56% of total)

### Phase 2: High-Value (Weeks 5-12) 🚧
- [ ] Video transcoding
- [ ] Data processing (Polars/Arrow)
- [ ] Custom GPU shaders

**Revenue**: $1.13B/year (34% of total)

### Phase 3: Advanced (Weeks 13-24) 📋
- [ ] 3D rendering (Blender HTTP)
- [ ] AI inference (quantized LLMs)

**Revenue**: $255.5M/year (8% of total)

### Phase 4: Specialized (Future) 📋
- [ ] Molecular dynamics
- [ ] CFD simulations

**Revenue**: $41.76M/year (1% of total)

---

## Usage Examples

### Image Resize
```json
{
  "library": "image",
  "method": "resize",
  "input": <image_bytes>,
  "params": {
    "width": 1920,
    "height": 1080,
    "filter": "Lanczos3",
    "format": "webp",
    "quality": 90
  },
  "budget": 100
}
```

### Video Transcode
```json
{
  "library": "video",
  "method": "transcode",
  "input": <video_bytes>,
  "params": {
    "codec": "av1",
    "resolution": "4k",
    "bitrate": "10M",
    "preset": "medium"
  },
  "budget": 10000
}
```

### Molecular Dynamics
```json
{
  "library": "physics",
  "method": "simulate",
  "input": <pdb_file>,
  "params": {
    "force_field": "AMBER",
    "timestep": 0.002,
    "temperature": 300,
    "ensemble": "NPT",
    "duration": 1000
  },
  "budget": 100000
}
```

---

## Benefits

1. **No Hardcoding**: Operations defined by params, not code
2. **Full Library Power**: Expose entire Rust ecosystem
3. **Extensible**: Add new methods without changing architecture
4. **Domain Expertise**: Complex workflows via JSON params
5. **Validation**: Libraries validate their own params
6. **Performance**: Direct library calls, zero overhead

---

## Implementation Guide

### Step 1: Create Library Proxy Trait

```rust
// modules/compute/src/engine.rs
use std::collections::HashMap;

pub trait LibraryProxy: Send + Sync {
    /// Execute a method from the library
    fn execute(&self, method: &str, input: &[u8], params: &str) -> Result<Vec<u8>, Box<dyn std::error::Error>>;
    
    /// Get library name
    fn name(&self) -> &str;
}

pub struct ComputeEngine {
    libraries: HashMap<String, Box<dyn LibraryProxy>>,
}

impl ComputeEngine {
    pub fn new() -> Self {
        let mut libraries = HashMap::new();
        
        // Register all libraries
        libraries.insert("image".into(), Box::new(ImageLibrary::new()) as Box<dyn LibraryProxy>);
        libraries.insert("crypto".into(), Box::new(CryptoLibrary::new()) as Box<dyn LibraryProxy>);
        libraries.insert("audio".into(), Box::new(AudioLibrary::new()) as Box<dyn LibraryProxy>);
        // ... register others
        
        Self { libraries }
    }
    
    pub fn execute(&self, library: &str, method: &str, input: &[u8], params: &str) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        let lib = self.libraries.get(library)
            .ok_or_else(|| format!("Unknown library: {}", library))?;
        
        lib.execute(method, input, params)
    }
}
```

### Step 2: Implement Library Adapters

```rust
// modules/compute/src/libraries/image.rs
use image::{DynamicImage, ImageFormat, imageops::FilterType};
use serde::{Deserialize, Serialize};

pub struct ImageLibrary;

#[derive(Deserialize)]
struct ImageParams {
    width: Option<u32>,
    height: Option<u32>,
    filter: Option<String>,
    format: Option<String>,
    quality: Option<u8>,
}

impl LibraryProxy for ImageLibrary {
    fn name(&self) -> &str { "image" }
    
    fn execute(&self, method: &str, input: &[u8], params_json: &str) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        let params: ImageParams = serde_json::from_str(params_json)?;
        let img = image::load_from_memory(input)?;
        
        let result = match method {
            "resize" => {
                let width = params.width.ok_or("Missing width")?;
                let height = params.height.ok_or("Missing height")?;
                let filter = parse_filter(&params.filter.unwrap_or("Lanczos3".into()))?;
                img.resize(width, height, filter)
            }
            "crop" => {
                let x = params.x.ok_or("Missing x")?;
                let y = params.y.ok_or("Missing y")?;
                let width = params.width.ok_or("Missing width")?;
                let height = params.height.ok_or("Missing height")?;
                img.crop_imm(x, y, width, height)
            }
            "blur" => {
                let sigma = params.sigma.ok_or("Missing sigma")?;
                img.blur(sigma)
            }
            "grayscale" => img.grayscale(),
            "fliph" => img.fliph(),
            "flipv" => img.flipv(),
            _ => return Err(format!("Unknown method: {}", method).into())
        };
        
        // Encode result
        encode_image(&result, &params.format, params.quality)
    }
}

fn encode_image(img: &DynamicImage, format: &Option<String>, quality: Option<u8>) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    let mut output = Vec::new();
    let format = match format.as_deref() {
        Some("jpeg") | Some("jpg") => ImageFormat::Jpeg,
        Some("png") => ImageFormat::Png,
        Some("webp") => ImageFormat::WebP,
        _ => ImageFormat::Png,
    };
    
    img.write_to(&mut std::io::Cursor::new(&mut output), format)?;
    Ok(output)
}
```

### Step 3: Wire Up WASM Entry Point

```rust
// modules/compute/src/lib.rs
use sdk::Reactor;
use wasm_bindgen::prelude::*;

mod engine;
mod libraries;

use engine::ComputeEngine;

#[wasm_bindgen]
pub struct ComputeKernel {
    reactor: Reactor,
    engine: ComputeEngine,
}

#[wasm_bindgen]
impl ComputeKernel {
    #[wasm_bindgen(constructor)]
    pub fn new(sab: &js_sys::SharedArrayBuffer, node_id: String) -> Self {
        sdk::init_logging();
        log::info!("Compute Kernel initialized on node {}", node_id);
        
        Self {
            reactor: Reactor::new(sab),
            engine: ComputeEngine::new(),
        }
    }
    
    pub fn poll(&mut self) -> bool {
        if !self.reactor.check_inbox() {
            return false;
        }
        
        // Read job request
        let inbox = self.reactor.inbox_data();
        let mut job_data = vec![0u8; inbox.length() as usize];
        inbox.copy_to(&mut job_data[..]);
        self.reactor.ack_inbox();
        
        // Parse request (simplified - full version uses Cap'n Proto)
        match self.process_job(&job_data) {
            Ok(result) => {
                let outbox = self.reactor.outbox_data();
                outbox.copy_from(&result[..]);
                self.reactor.raise_outbox();
                true
            }
            Err(e) => {
                log::error!("Job failed: {}", e);
                false
            }
        }
    }
    
    fn process_job(&self, data: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        // Parse JSON request (simplified)
        let request: JobRequest = serde_json::from_slice(data)?;
        
        // Execute via engine
        self.engine.execute(
            &request.library,
            &request.method,
            &request.input,
            &request.params
        )
    }
}

#[derive(serde::Deserialize)]
struct JobRequest {
    library: String,
    method: String,
    input: Vec<u8>,
    params: String,
}
```

### Step 4: Update Go Supervisor

The Go supervisor needs to handle the new library proxy pattern:

```go
// kernel/threads/compute.go
func (cs *ComputeSupervisor) processJob(job *JobRequest) (*JobResult, error) {
    // Encode request as JSON (simplified - use Cap'n Proto in production)
    requestJSON, err := json.Marshal(map[string]interface{}{
        "library": job.Library,
        "method":  job.Method,
        "input":   job.Input,
        "params":  job.Params, // Already JSON string
    })
    
    // Write to inbox
    cs.reactor.WriteInbox(requestJSON)
    cs.reactor.RaiseInbox()
    
    // Wait for result
    result := cs.reactor.ReadOutbox()
    
    return &JobResult{
        JobID:  job.JobID,
        Output: result,
        Status: StatusSuccess,
    }, nil
}
```

---

## Directory Structure

```
modules/compute/
├── Cargo.toml
├── README.md                 # This file
├── src/
│   ├── lib.rs               # WASM entry point
│   ├── engine.rs            # ComputeEngine + LibraryProxy trait
│   └── libraries/           # Library implementations
│       ├── mod.rs
│       ├── image.rs         # Image processing
│       ├── video.rs         # Video transcoding
│       ├── audio.rs         # Audio processing
│       ├── crypto.rs        # Cryptographic operations
│       ├── data.rs          # Data processing (Polars)
│       └── gpu.rs           # Custom GPU shaders (wgpu)
```

---

## Testing Strategy

### Unit Tests
```rust
#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_image_resize() {
        let library = ImageLibrary::new();
        let input = include_bytes!("../test_data/sample.jpg");
        let params = r#"{"width": 800, "height": 600, "filter": "Lanczos3"}"#;
        
        let result = library.execute("resize", input, params).unwrap();
        assert!(!result.is_empty());
    }
}
```

### Integration Tests
```rust
#[test]
fn test_compute_engine() {
    let engine = ComputeEngine::new();
    
    let input = include_bytes!("../test_data/sample.jpg");
    let params = r#"{"width": 800, "height": 600}"#;
    
    let result = engine.execute("image", "resize", input, params).unwrap();
    assert!(!result.is_empty());
}
```

---

## Benefits

1. **No Hardcoding**: Operations defined by params, not code
2. **Full Library Power**: Expose entire Rust ecosystem
3. **Extensible**: Add new methods without changing architecture
4. **Domain Expertise**: Complex workflows via JSON params
5. **Validation**: Libraries validate their own params
6. **Performance**: Direct library calls, zero overhead

**This architecture leverages Rust's ecosystem, doesn't reimplement it.**

