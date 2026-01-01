pub mod flux;
pub mod math;
pub mod matter;
pub mod mesh;
pub mod ml;
pub mod mosaic;
pub mod types;

use log::{error, info};

use crate::flux::boundary::ScaleCoupling;
use crate::math::MathProxy;
use crate::matter::atomic::AtomicProxy;
use crate::matter::continuum::ContinuumProxy;
use crate::matter::kinetic::KineticProxy;
use crate::matter::ScienceProxy;
use crate::mesh::cache::ComputationCache;
use crate::ml::adaptive_allocator::{AdaptiveAllocator, AllocationStrategy, LoadPrediction};
use crate::mosaic::bridge::{BridgeConfig, P2PBridge, PeerID, VoxelRange};
use crate::mosaic::dispatch::ShardingStrategy;
use crate::types::*;
use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use std::cell::RefCell;
use std::collections::HashMap;
use std::rc::Rc;
use std::sync::atomic::{AtomicU64, Ordering};

use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;
use std::task::{Context, Poll, Waker};
// Note: wasm_bindgen types are available through web-sys
use web_sys::wasm_bindgen::{closure::Closure, JsValue};

// Import compute engine types for UnitProxy
use compute::engine::{ComputeError, ResourceLimits, UnitProxy};

// Import the capnp-generated types
pub use sdk::base_capnp;

pub mod science_capnp {
    #![allow(dead_code, unused_imports, unused_parens)]
    include!(concat!(env!("OUT_DIR"), "/science/v1/science_capnp.rs"));
}

// ----------------------------------------------------------------------------
// ENHANCED SCIENCE MODULE
// ----------------------------------------------------------------------------

// Redundant types moved to types.rs (ScaleMapping logic removed in v2.0)

#[derive(Serialize, Deserialize)]
pub struct CoupledComputation {
    id: u32,
    request: ScienceRequest,
    dependencies: Vec<u32>,
}

#[derive(Serialize, Deserialize)]
pub struct CoupledRequest {
    computations: Vec<CoupledComputation>,
    couplings: Vec<ScaleCoupling>,
    reconciliation: ReconciliationMethod,
    tolerance: f64,
    sharding_strategy: ShardingStrategy,
    synchronization_epoch: u64,
}

#[derive(Serialize, Deserialize)]
pub enum ReconciliationMethod {
    WeakCoupling,
    StrongCoupling,
    Monolithic,
}

#[derive(Serialize, Deserialize)]
pub struct ScienceRequest {
    library: String,
    method: String,
    input: Vec<u8>,
    params: Vec<u8>,
    scale_hint: SimulationScale,
    cache_policy: CachePolicy,
}

#[derive(Serialize, Deserialize)]
pub enum CachePolicy {
    ComputeAlways,
    CacheIfExists,
    ValidateOnly,
    StoreOnly,
}

// Custom Promise-to-Future adapter (replaces wasm-bindgen-futures::JsFuture)
struct PromiseFuture {
    _promise: js_sys::Promise,
    result: Rc<RefCell<Option<Result<JsValue, JsValue>>>>,
    waker: Rc<RefCell<Option<Waker>>>,
}

impl PromiseFuture {
    fn new(promise: js_sys::Promise) -> Self {
        let result = Rc::new(RefCell::new(None));
        let waker: Rc<RefCell<Option<Waker>>> = Rc::new(RefCell::new(None));

        let result_clone = result.clone();
        let waker_clone = waker.clone();

        let resolve = Closure::once(move |value: JsValue| {
            *result_clone.borrow_mut() = Some(Ok(value));
            if let Some(w) = waker_clone.borrow_mut().take() {
                w.wake();
            }
        });

        let result_clone = result.clone();
        let waker_clone = waker.clone();

        let reject = Closure::once(move |error: JsValue| {
            *result_clone.borrow_mut() = Some(Err(error));
            if let Some(w) = waker_clone.borrow_mut().take() {
                w.wake();
            }
        });

        let _ = promise.then2(&resolve, &reject);
        resolve.forget();
        reject.forget();

        Self {
            _promise: promise,
            result,
            waker,
        }
    }
}

impl Future for PromiseFuture {
    type Output = Result<JsValue, JsValue>;

    fn poll(self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<Self::Output> {
        if let Some(result) = self.result.borrow_mut().take() {
            Poll::Ready(result)
        } else {
            *self.waker.borrow_mut() = Some(cx.waker().clone());
            Poll::Pending
        }
    }
}

// Helper function to replace JsFuture::from()
pub async fn await_promise(promise: js_sys::Promise) -> Result<JsValue, JsValue> {
    PromiseFuture::new(promise).await
}

/// Initialize panic hooks and logging
/// Initialize panic hooks and logging
#[no_mangle]
pub extern "C" fn science_init_hooks() {
    // Custom panic hook using stable ABI to avoid console_error_panic_hook dependency
    std::panic::set_hook(Box::new(|info| {
        let msg = info.to_string();
        sdk::js_interop::console_error(&msg);
    }));

    // Initialize SDK logging (now uses stable ABI)
    sdk::init_logging();
}

/// Initialize science module with SharedArrayBuffer from global scope
#[no_mangle]
pub extern "C" fn science_init_with_sab(_buffer_ptr: *mut u8, _buffer_size: usize) -> i32 {
    // Use stable ABI to get global object
    let global = sdk::js_interop::get_global();
    let sab_key = sdk::js_interop::create_string("__INOS_SAB__");
    let sab_val = sdk::js_interop::reflect_get(&global, &sab_key);

    let offset_key = sdk::js_interop::create_string("__INOS_SAB_OFFSET__");
    let offset_val = sdk::js_interop::reflect_get(&global, &offset_key);

    let size_key = sdk::js_interop::create_string("__INOS_SAB_SIZE__");
    let size_val = sdk::js_interop::reflect_get(&global, &size_key);

    let id_key = sdk::js_interop::create_string("__INOS_MODULE_ID__");
    let id_val = sdk::js_interop::reflect_get(&global, &id_key);

    if let (Ok(val), Ok(off), Ok(sz)) = (sab_val, offset_val, size_val) {
        if !val.is_undefined() && !val.is_null() {
            let offset = sdk::js_interop::as_f64(&off).unwrap_or(0.0) as u32;
            let size = sdk::js_interop::as_f64(&sz).unwrap_or(0.0) as u32;
            let module_id = id_val.ok().and_then(|v| v.as_f64()).unwrap_or(0.0) as u32;

            // Create TWO SafeSAB references:
            // 1. Scoped view for module data
            let module_sab = sdk::sab::SafeSAB::new_shared_view(val.clone(), offset, size);
            // 2. Global SAB for registry writes
            let global_sab = sdk::sab::SafeSAB::new(val.clone());

            sdk::init_logging();
            info!("Science module initialized (ID: {}) with synchronized SAB bridge (Offset: 0x{:x}, Size: {}MB)", 
                module_id, offset, size / 1024 / 1024);

            // Register capabilities
            register_science(&global_sab);

            // Instantiate Global Module
            unsafe {
                let mut module = ScienceModule::new();
                module.set_sab(module_sab);
                GLOBAL_SCIENCE = Some(module);
            }

            return 1;
        }
    }
    0
}

/// Helper to register science capabilities
fn register_science(sab: &sdk::sab::SafeSAB) {
    use sdk::registry::*;
    let id = "science";
    let mut builder = ModuleEntryBuilder::new(id).version(1, 9, 0);
    // Register capabilities with scales
    builder = builder.capability("atomic", false, 512);
    builder = builder.capability("continuum", false, 512);
    builder = builder.capability("kinetic", false, 512);
    builder = builder.capability("math", false, 256);
    builder = builder.capability("simulation", false, 1024);

    match builder.build() {
        Ok((mut entry, _, caps)) => {
            if let Ok(offset) = write_capability_table(sab, &caps) {
                entry.cap_table_offset = offset;
            }
            if let Ok((slot, _)) = find_slot_double_hashing(sab, id) {
                let _ = write_enhanced_entry(sab, slot, &entry);
            }
        }
        Err(_) => {}
    }
}

/// Standardized Memory Allocator for WebAssembly
#[no_mangle]
pub extern "C" fn science_alloc(size: usize) -> *mut u8 {
    let mut buf = Vec::with_capacity(size);
    let ptr = buf.as_mut_ptr();
    std::mem::forget(buf);
    ptr
}

/// External poll entry point for JavaScript
#[no_mangle]
pub extern "C" fn science_poll() {
    unsafe {
        if let Some(ref module) = GLOBAL_SCIENCE {
            module.poll_reactive();
        }
    }
}

// Removed wasm_bindgen
pub struct ScienceModule {
    // Core proxies
    atomic: RefCell<AtomicProxy>,
    continuum: RefCell<ContinuumProxy>,
    kinetic: RefCell<KineticProxy>,
    math: RefCell<MathProxy>,

    // Multi-scale orchestration

    // Mesh-aware caching (Consolidated in Phase 2)
    cache: Rc<RefCell<ComputationCache>>,

    // Performance telemetry (Shared in Phase 2)
    telemetry: Rc<RefCell<Telemetry>>,

    // Node identity for Proof-of-Simulation
    node_id: u64,
    // Shard ID for distributed Proof-of-Simulation (v3.1 Feature)
    #[allow(dead_code)] // Currently unused in v1.9 single-node execution
    shard_id: u32,

    // Epoch for Reactive Mutation pattern (INOS v1.9)
    epoch: Arc<AtomicU64>,
    last_inbox_epoch: Arc<AtomicU64>,

    // P2P Bridge (Decoupled in Phase 2)
    bridge: RefCell<Option<Arc<P2PBridge>>>,

    // Shared SAB for direct memory access
    sab: Option<sdk::sab::SafeSAB>,
}

static mut GLOBAL_SCIENCE: Option<ScienceModule> = None;

// Shared Telemetry used from crate::types

// ----------------------------------------------------------------------------
// CORE IMPLEMENTATION
// ----------------------------------------------------------------------------

// Removed wasm_bindgen
impl ScienceModule {
    pub fn new() -> Self {
        sdk::init_logging();

        // Generate deterministic node identity from WASM instance
        let node_id = Self::generate_node_id();
        let shard_id = (node_id % 1_000_000) as u32; // For 1M shard mesh

        let cache = Rc::new(RefCell::new(ComputationCache::new()));
        let telemetry = Rc::new(RefCell::new(Telemetry::default()));
        let epoch = Arc::new(AtomicU64::new(0));

        Self {
            atomic: RefCell::new(AtomicProxy::new(cache.clone(), telemetry.clone())),
            continuum: RefCell::new(ContinuumProxy::new(cache.clone(), telemetry.clone())),
            kinetic: RefCell::new(KineticProxy::new(cache.clone(), telemetry.clone())),
            math: RefCell::new(MathProxy::new(cache.clone(), telemetry.clone())),
            cache,
            telemetry,
            node_id,
            shard_id,
            epoch,
            last_inbox_epoch: Arc::new(AtomicU64::new(0)),
            bridge: RefCell::new(None),
            sab: None,
        }
    }

    pub fn set_sab(&mut self, sab: sdk::sab::SafeSAB) {
        self.sab = Some(sab);
    }

    pub fn poll_reactive(&self) {
        // High-frequency task execution (Chain of Mutators)
        if let Some(ref sab) = self.sab {
            // 1. Process standard inbox signals (JobRequests)
            let _ = self.poll_inbox(sab);

            // 2. Continuous Bird Physics (High Frequency)
            self.update_bird_physics(sab);
        }
    }

    fn poll_inbox(&self, _sab: &sdk::sab::SafeSAB) -> Result<bool, String> {
        // Legacy/Generic inbox polling logic (placeholder)
        Ok(false)
    }

    fn update_bird_physics(&self, sab: &sdk::sab::SafeSAB) {
        use sdk::layout::{IDX_BIRD_EPOCH, OFFSET_BIRD_STATE, SIZE_BIRD_STATE};

        // Standard Zero-Copy Pattern: Read -> Mutate -> Write
        let mut data = match sab.read(OFFSET_BIRD_STATE, SIZE_BIRD_STATE) {
            Ok(d) => d,
            Err(_) => return,
        };

        // Direct C-style binding to the data
        let state = unsafe { &mut *(data.as_mut_ptr() as *mut BirdState) };

        // 1. Physics: Velocity towards interaction point (Attraction)
        let target = state.interaction_point;
        let pos = state.position;

        // Direction vector
        let dx = target[0] - pos[0];
        let dy = target[1] - pos[1];
        let dz = target[2] - pos[2];

        // Small damping force
        let strength = 0.05;
        state.velocity[0] += dx * strength;
        state.velocity[1] += dy * strength;
        state.velocity[2] += dz * strength;

        // Apply friction
        let friction = 0.95;
        state.velocity[0] *= friction;
        state.velocity[1] *= friction;
        state.velocity[2] *= friction;

        // Update position
        state.position[0] += state.velocity[0] * 0.01;
        state.position[1] += state.velocity[1] * 0.01;
        state.position[2] += state.velocity[2] * 0.01;

        // 2. Animation: Flap Phase
        state.flap_phase += 0.1 * state.energy.max(0.1);
        if state.flap_phase > 1.0 {
            state.flap_phase -= 1.0;
        }

        // 3. Write back and Notify Local JS
        if sab.write(OFFSET_BIRD_STATE, &data).is_ok() {
            // Flip the BIRD_EPOCH to signal JS that new coordinates are ready
            let flags = sab
                .int32_view(sdk::layout::OFFSET_ATOMIC_FLAGS, 16)
                .unwrap();
            sdk::js_interop::atomic_add(&flags.into(), IDX_BIRD_EPOCH, 1);
        }

        // 4. Automated P2P Gossip (Throttled to ~10Hz)
        // Triggered every 12 frames of 120Hz loop
        let current_epoch = self.epoch.fetch_add(1, Ordering::SeqCst);
        if current_epoch % 12 == 0 {
            // We use the SDK's automated Syscall path
            // This writes to the SAB Outbox; Go Kernel's signal_listener picks it up
            let _ = sdk::syscalls::SyscallClient::send_message(sab, "mesh:gossip:bird", &data);
        }
    }

    // Removed wasm_bindgen
    pub fn init_mosaic(&self, buffer: js_sys::SharedArrayBuffer) -> Result<(), String> {
        let sab = Arc::new(sdk::sab::SafeSAB::new(buffer.into()));

        // Configuration for the bridge
        let config = BridgeConfig {
            chunk_size: 1024 * 1024,
            redundancy_factor: 2,
            ml_integration: true,
        };

        // We use a stub allocator for now
        struct StubAllocator;
        #[async_trait]
        impl AdaptiveAllocator for StubAllocator {
            async fn predict_load(&self, _: &VoxelRange, _: AllocationStrategy) -> LoadPrediction {
                LoadPrediction {
                    predicted_load: 0.0,
                    confidence: 1.0,
                    recommended_strategy: AllocationStrategy::Balanced,
                }
            }
            async fn allocate(
                &self,
                _: &VoxelRange,
                _: AllocationStrategy,
            ) -> Result<Vec<PeerID>, String> {
                Ok(vec![])
            }
        }

        let allocator = Arc::new(StubAllocator);

        let bridge = P2PBridge::new(&self.node_id.to_le_bytes(), allocator, config, sab.clone())?;
        *self.bridge.borrow_mut() = Some(bridge);

        // Share SAB with proxies for zero-copy mutation
        self.kinetic.borrow_mut().set_sab(sab.clone());
        // self.atomic.borrow_mut().set_sab(sab.clone());
        // self.continuum.borrow_mut().set_sab(sab.clone());

        Ok(())
    }

    pub fn increment_epoch(&self) -> u64 {
        self.epoch.fetch_add(1, Ordering::SeqCst) + 1
    }

    // Removed wasm_bindgen
    pub fn get_epoch(&self) -> u64 {
        self.epoch.load(Ordering::SeqCst)
    }
}

impl Default for ScienceModule {
    fn default() -> Self {
        Self::new()
    }
}

// Removed wasm_bindgen
impl ScienceModule {
    /// Execute a Cap'n Proto encoded ScienceRequest.
    /// This is the primary entry point for the Go kernel to orchestrate physics workloads.
    // Removed wasm_bindgen
    /// Execute a Cap'n Proto encoded ScienceRequest.
    /// This is the primary entry point for the Go kernel to orchestrate physics workloads.
    // Removed wasm_bindgen
    /// Reactive Poll Loop
    /// Checks SAB Inbox for new messages, processes them, and writes to Outbox.
    /// Returns true if work was done.
    // Removed wasm_bindgen
    pub async fn poll(&self) -> Result<bool, String> {
        // Clone the Arc to avoid holding the RefCell borrow across await points
        let bridge_arc = {
            let bridge_ref = self.bridge.borrow();
            if bridge_ref.is_none() {
                return Ok(false);
            }
            bridge_ref.as_ref().unwrap().clone()
        };
        // RefCell borrow is dropped here

        let sab = &bridge_arc.sab;

        // 1. Check Inbox Signal (Index 1)
        use sdk::signal::IDX_INBOX_DIRTY;
        let flags = sab.int32_view(sdk::layout::OFFSET_ATOMIC_FLAGS, 8)?; // Need up to index 7

        let current_inbox_seq = sdk::js_interop::atomic_load(&flags, IDX_INBOX_DIRTY) as u64;

        let last_seq = self.last_inbox_epoch.load(Ordering::Relaxed);

        if current_inbox_seq <= last_seq {
            // No new messages
            // Also poll P2P bridge for network messages
            if let Ok(msgs) = bridge_arc.poll() {
                if !msgs.is_empty() {
                    log::info!("Processed {} P2P messages", msgs.len());
                    return Ok(true);
                }
            }
            return Ok(false);
        }

        // 2. Read Inbox (Ring Buffer)
        // We construct a transient RingBuffer wrapper around the shared SAB
        let inbox = sdk::ringbuffer::RingBuffer::new(
            sdk::sab::SafeSAB::new(sab.inner().clone()),
            sdk::layout::OFFSET_INBOX_OUTBOX as u32,
            (sdk::layout::SIZE_INBOX_OUTBOX / 2) as u32,
        );

        let request_bytes = match inbox.read_message()? {
            Some(msg) => msg,
            None => {
                // False alarm or partial message
                self.last_inbox_epoch
                    .store(current_inbox_seq, Ordering::Relaxed);
                return Ok(false);
            }
        };

        // 3. Process Request
        let result_vec = self.process_request_internal(&request_bytes).await?;

        // 4. Write Response to Outbox (Ring Buffer)
        let outbox = sdk::ringbuffer::RingBuffer::new(
            sdk::sab::SafeSAB::new(sab.inner().clone()),
            (sdk::layout::OFFSET_INBOX_OUTBOX + (sdk::layout::SIZE_INBOX_OUTBOX / 2)) as u32,
            (sdk::layout::SIZE_INBOX_OUTBOX / 2) as u32,
        );

        if !outbox.write_message(&result_vec)? {
            log::error!("Outbox full, dropping science result");
        } else {
            // Signal Kernel (IDX_OUTBOX_DIRTY)
            // SyscallClient logic is replaced by direct Atomic Signal for Ring Buffer
            use sdk::signal::IDX_OUTBOX_DIRTY;
            sdk::js_interop::atomic_add(&flags, IDX_OUTBOX_DIRTY, 1);
        }

        // 5. Update Local State
        self.last_inbox_epoch
            .store(current_inbox_seq, Ordering::Relaxed);

        Ok(true)
    }

    /// Execute a Cap'n Proto encoded ScienceRequest (Direct Call)
    // Removed wasm_bindgen
    pub async fn execute_raw(&self, request_data: Vec<u8>) -> Result<Vec<u8>, String> {
        self.process_request_internal(&request_data).await
    }

    /// Internal logic to process a request bytes -> response bytes
    /// Expects sdk::protocols::compute::JobRequest (Capsule Protocol)
    /// Internal logic to process a request bytes -> response bytes
    /// Expects sdk::protocols::compute::JobRequest (Capsule Protocol)
    async fn process_request_internal(&self, request_data: &[u8]) -> Result<Vec<u8>, String> {
        #[cfg(feature = "capnp")]
        {
            use capnp::message::ReaderOptions;
            use capnp::serialize;
            use sdk::protocols::compute::compute::job_request;

            // 1. Decode Request using Universal Job Protocol
            let reader = serialize::read_message(&mut &request_data[..], ReaderOptions::new())
                .map_err(|e: capnp::Error| format!("Capnp decode failed: {}", e))?;

            let request = reader
                .get_root::<job_request::Reader>()
                .map_err(|e: capnp::Error| e.to_string())?;

            // 2. Extract context
            let library_name = request
                .get_library()
                .map_err(|_| "Invalid library")?
                .to_str()
                .unwrap_or("unknown");
            let method = request
                .get_method()
                .map_err(|_| "Invalid method")?
                .to_str()
                .unwrap_or("unknown");

            // Params are now Data (bytes)
            let params = request.get_params().map_err(|_| "Invalid params")?;
            let input_data = request.get_input().unwrap_or(&[]);

            self.dispatch(library_name, method, input_data, params)
                .await
        }
        #[cfg(not(feature = "capnp"))]
        {
            let _ = request_data;
            Err("Cap'n Proto feature not enabled".to_string())
        }
    }

    /// Central dispatch logic
    async fn dispatch(
        &self,
        library: &str,
        method: &str,
        input: &[u8],
        params: &[u8],
    ) -> Result<Vec<u8>, String> {
        // Tier 2: Check for nested ScienceRequest
        if library == "science" && method == "execute" {
            // Decode ScienceRequest from params
            // Note: We need to import ScienceRequest reader
            // For now, this requires the schema to be compiled and accessible.
            // As this is a recursive step, let's defer full implementation until Phase 4 (Typed Proxies).
            // But we acknowledge the path.
            return Err("Tier 2 nested execution not yet fully implemented".to_string());
        }

        // Tier 1: Legacy string-based dispatch
        // We defer UTF-8 conversion to individual proxies if they need it (Polyglot)

        // 2.7. Check shared cache
        let scale = self.extract_scale_from_params(params).unwrap_or_default();
        let request_hash = self.compute_request_hash(library, method, input, params);

        if let Some(entry) = self.cache.borrow_mut().get(&request_hash, Some(&scale)) {
            self.telemetry.borrow_mut().cache_hits += 1;
            return self
                .encode_job_result(true, &entry.data, "")
                .map_err(|e| e.to_string());
        }
        self.telemetry.borrow_mut().cache_misses += 1;

        // 2.8. Bridge
        let bridge_opt = self.bridge.borrow().clone();
        if let Some(bridge) = bridge_opt {
            let _ = bridge
                .request_execution(
                    request_hash.clone(),
                    library.to_string(),
                    method.to_string(),
                    std::str::from_utf8(params).unwrap_or("{}").to_string(), // Bridge still expects string for now? Check P2PBridge.
                    scale,
                )
                .await;
        }

        // 3. Execute
        let result_or_err = match library {
            "atomic" | "science" => self.atomic.borrow_mut().execute(method, input, params),
            "continuum" => self.continuum.borrow_mut().execute(method, input, params),
            "kinetic" => self.kinetic.borrow_mut().execute(method, input, params),
            "math" => self.math.borrow_mut().execute(method, input, params),
            _ => Err(ScienceError::InvalidLibrary(library.to_string())),
        };

        match result_or_err {
            Ok(result_vec) => {
                // 4. Cache
                let proof = ComputationProof {
                    result_hash: self.compute_result_hash(&result_vec),
                    node_id: self.node_id,
                    shard_id: self.shard_id,
                    ..Default::default()
                };
                let timestamp = (js_sys::Date::now() / 1000.0) as u64;
                let entry = CacheEntry {
                    data: result_vec.clone(),
                    result_hash: proof.result_hash.clone(),
                    timestamp,
                    access_count: 1,
                    scale,
                    proof: proof.clone(),
                };
                self.cache.borrow_mut().put(request_hash, entry);
                self.increment_epoch();

                self.encode_job_result(true, &result_vec, "")
                    .map_err(|e| e.to_string())
            }
            Err(e) => self
                .encode_job_result(false, &[], &format!("{:?}", e))
                .map_err(|e| e.to_string()),
        }
    }

    #[cfg(feature = "capnp")]
    fn encode_job_result(
        &self,
        success: bool,
        data: &[u8],
        error_msg: &str,
    ) -> Result<Vec<u8>, String> {
        use capnp::serialize_packed;
        use sdk::protocols::compute::compute::{job_result, Status};

        let mut message = capnp::message::Builder::new_default();
        let mut root = message.init_root::<job_result::Builder>();

        if success {
            root.set_status(Status::Success);
        } else {
            root.set_status(Status::Failed);
        }

        // Output
        root.set_output(data);

        // Error message
        root.set_error_message(error_msg);

        // Metrics (Stub)
        // let mut metrics = root.init_metrics();
        // metrics.set_cpu_time_ns(0);

        let mut output_bytes = Vec::new();
        serialize_packed::write_message(&mut output_bytes, &message).map_err(|e| e.to_string())?;

        Ok(output_bytes)
    }

    fn generate_node_id() -> u64 {
        // In production, this would come from mesh consensus
        //`` For now, use a hash of instance memory + timestamp
        let mut hasher = blake3::Hasher::new();
        hasher.update(
            &((sdk::js_interop::get_now() / 1000) as u64).to_le_bytes(), // Unix epoch in seconds
        );
        let hash = hasher.finalize();
        u64::from_le_bytes(hash.as_bytes()[..8].try_into().unwrap())
    }
}

// ----------------------------------------------------------------------------
// UNIT PROXY IMPLEMENTATION
// ----------------------------------------------------------------------------

#[async_trait(?Send)]
impl UnitProxy for ScienceModule {
    fn service_name(&self) -> &str {
        "science"
    }

    fn name(&self) -> &str {
        "science"
    }

    fn actions(&self) -> Vec<&str> {
        vec![
            "atomic:select",
            "atomic:rmsd",
            "atomic:centerOfMass",
            "atomic:distance",
            "continuum:generateMesh",
            "continuum:solveLinear",
            "continuum:computeStress",
            "kinetic:createBody",
            "kinetic:step",
            "kinetic:castRay",
            "math:matrixMultiply",
            "math:dotProduct",
            "math:inverse",
        ]
    }

    fn resource_limits(&self) -> ResourceLimits {
        ResourceLimits {
            max_input_size: 100 * 1024 * 1024,  // 100MB for large simulations
            max_output_size: 100 * 1024 * 1024, // 100MB
            max_memory_pages: 32768,            // 2GB
            timeout_ms: 30000,                  // 30s for complex physics
            max_fuel: 1_000_000_000_000,        // 1T instructions
        }
    }

    async fn execute(
        &self,
        action: &str,
        input: &[u8],
        params: &[u8],
    ) -> Result<Vec<u8>, ComputeError> {
        // Tier 2 Direct Execution
        if action == "execute" {
            // Should verify params IS a ScienceRequest?
            // Since we don't have nested execution fully impl, we might want to check
            // logic here. But let's route to dispatch for consistency.
            // Wait, dispatch expects library and method strings used to create request hash!
            // If action="execute", library SHOULD be "science".
            return self
                .dispatch("science", "execute", input, params)
                .await
                .map_err(ComputeError::ExecutionFailed);
        }

        // Tier 1 Legacy/Frontend Dispatch
        let parts: Vec<&str> = action.split(':').collect();
        if parts.len() < 2 {
            return Err(ComputeError::ExecutionFailed(
                "Invalid action format. Use library:method".into(),
            ));
        }

        let library = parts[0];
        let method = parts[1];

        // Directly call dispatch logic, bypassing redundant wrapping/unwrapping!
        self.dispatch(library, method, input, params)
            .await
            .map_err(ComputeError::ExecutionFailed)
    }
}

// ----------------------------------------------------------------------------
// WASM BINDINGS (Keep remaining methods for backward compatibility)
// ----------------------------------------------------------------------------

// Removed wasm_bindgen
impl ScienceModule {
    // Removed execute_coupled (Legacy JSON path). Use execute_raw (Cap'n Proto) for all requests.

    /// Enhanced validation with Proof-of-Simulation
    pub fn validate_result(
        &self,
        library: &str,
        method: &str,
        input: &[u8],
        params: &[u8], // Updated to bytes
        claimed_result_hash: &str,
        validation_method: &str, // "full", "spot", "cross_scale"
    ) -> Result<JsValue, String> {
        self.telemetry.borrow_mut().validation_requests += 1;

        let request_hash = self.compute_request_hash(library, method, input, params);

        log::info!("VALIDATE: {} claim={}", request_hash, claimed_result_hash);

        // Check cache first
        if let Some(entry) = self.cache.borrow_mut().get(&request_hash, None) {
            if entry.proof.result_hash == claimed_result_hash {
                return self.build_validation_result(true, "Cache match", &entry.proof);
            }
        }

        // Perform validation based on method
        let (is_valid, reason) = match validation_method {
            "spot" => self.validate_spot(library, method, input, params, claimed_result_hash),
            "cross_scale" => {
                self.validate_cross_scale(library, method, input, params, claimed_result_hash)
            }
            "full" => self.validate_full(library, method, input, params, claimed_result_hash),
            _ => self.validate_full(library, method, input, params, claimed_result_hash),
        }?;

        self.build_validation_result(is_valid, &reason, &ComputationProof::default())
    }

    /// Method discovery with scale capabilities
    pub fn methods_with_capabilities(&self) -> JsValue {
        let mut capabilities = HashMap::new();

        // Atomic methods with quantum fidelity
        capabilities.insert(
            "atomic".to_string(),
            vec![
                (
                    "select".to_string(),
                    SimulationScale {
                        spatial: 1e-10,
                        temporal: 1e-15,
                        energy: 1.6e-19,
                        fidelity: FidelityLevel::Research,
                    },
                ),
                (
                    "computeEnergy".to_string(),
                    SimulationScale {
                        spatial: 1e-10,
                        temporal: 1e-15,
                        energy: 1.6e-19,
                        fidelity: FidelityLevel::QuantumExact,
                    },
                ),
                (
                    "molecularDynamics".to_string(),
                    SimulationScale {
                        spatial: 1e-9,
                        temporal: 1e-12,
                        energy: 1.6e-19,
                        fidelity: FidelityLevel::Engineering,
                    },
                ),
            ],
        );

        // Continuum methods
        capabilities.insert(
            "continuum".to_string(),
            vec![
                (
                    "solveLinear".to_string(),
                    SimulationScale {
                        spatial: 1e-3,
                        temporal: 1e-3,
                        energy: 1.0,
                        fidelity: FidelityLevel::Engineering,
                    },
                ),
                (
                    "computeStress".to_string(),
                    SimulationScale {
                        spatial: 1e-3,
                        temporal: 1e-3,
                        energy: 1.0,
                        fidelity: FidelityLevel::Research,
                    },
                ),
            ],
        );

        // etc...

        JsValue::from_str(&serde_json::to_string(&capabilities).unwrap_or_default())
    }

    /// Get performance telemetry
    pub fn telemetry(&self) -> JsValue {
        let telemetry = self.telemetry.borrow();
        let data = serde_json::json!({
            "cache_hits": telemetry.cache_hits,
            "cache_misses": telemetry.cache_misses,
            "cache_hit_rate": if telemetry.cache_hits + telemetry.cache_misses > 0 {
                telemetry.cache_hits as f64 / (telemetry.cache_hits + telemetry.cache_misses) as f64
            } else { 0.0 },
            "compute_time_ms": telemetry.compute_time_ms,
            "cross_scale_calls": telemetry.cross_scale_calls,
            "validation_requests": telemetry.validation_requests,
            "cache_size": self.cache.borrow().len(),
        });
        JsValue::from_str(&serde_json::to_string(&data).unwrap_or_default())
    }

    /// Clear cache (useful for memory management)
    pub fn clear_cache(&self) {
        self.cache.borrow_mut().clear();
        log::info!("Cache cleared");
    }

    /// Get cache statistics
    pub fn cache_stats(&self) -> JsValue {
        let cache = self.cache.borrow();
        let stats = serde_json::json!({
            "entries": cache.len(),
            "is_empty": cache.is_empty(),
        });
        JsValue::from_str(&serde_json::to_string(&stats).unwrap_or_default())
    }
}

// ----------------------------------------------------------------------------
// PRIVATE HELPER METHODS
// ----------------------------------------------------------------------------

impl ScienceModule {
    fn compute_request_hash(
        &self,
        library: &str,
        method: &str,
        input: &[u8],
        params: &[u8],
    ) -> String {
        let mut hasher = blake3::Hasher::new();
        hasher.update(library.as_bytes());
        hasher.update(b":");
        hasher.update(method.as_bytes());
        hasher.update(b"|");
        hasher.update(input);
        hasher.update(b"|");
        hasher.update(params); // Already bytes
        hasher.update(b"|");
        hasher.update(&self.node_id.to_le_bytes()); // Node-specific but deterministic
        hasher.finalize().to_hex().to_string()
    }

    #[allow(dead_code)]
    fn compute_result_hash(&self, data: &[u8]) -> String {
        blake3::hash(data).to_hex().to_string()
    }

    #[allow(dead_code)]
    fn compute_method_hash(&self, library: &str, method: &str) -> String {
        let mut hasher = blake3::Hasher::new();
        hasher.update(library.as_bytes());
        hasher.update(b":");
        hasher.update(method.as_bytes());
        hasher.update(b"@v1.0"); // Version tag for deterministic updates
        hasher.finalize().to_hex().to_string()
    }

    #[allow(dead_code)]
    fn compute_hash(&self, data: &[u8]) -> String {
        blake3::hash(data).to_hex().to_string()
    }

    #[allow(dead_code)]
    fn scale_compatible(
        &self,
        cached_scale: &SimulationScale,
        requested_scale: &SimulationScale,
    ) -> bool {
        // Higher fidelity can satisfy lower fidelity requests
        // e.g., QuantumExact can satisfy Engineering request
        match (cached_scale.fidelity, requested_scale.fidelity) {
            (FidelityLevel::RealityProof, _) => true,
            (FidelityLevel::QuantumExact, f) if f <= FidelityLevel::QuantumExact => true,
            (FidelityLevel::Research, f) if f <= FidelityLevel::Research => true,
            (FidelityLevel::Engineering, f) if f <= FidelityLevel::Engineering => true,
            (FidelityLevel::Heuristic, FidelityLevel::Heuristic) => true,
            _ => false,
        }
    }

    #[allow(dead_code)]
    fn build_enhanced_result(
        &self,
        data: &[u8],
        proof: &ComputationProof,
        start_time: f64,
        from_cache: bool,
        scale: &SimulationScale,
    ) -> Result<JsValue, String> {
        let duration = web_sys::window()
            .and_then(|w| w.performance())
            .map_or(0.0, |p| p.now())
            - start_time;

        self.telemetry.borrow_mut().compute_time_ms += duration as u64;

        let result = serde_json::json!({
            "status": "success",
            "data": data,
            "proof": proof,
            "performance": {
                "duration_ms": duration,
                "from_cache": from_cache,
                "node_id": self.node_id,
                "scale": scale,
            },
            "cache_key": proof.input_hash,
            "result_hash": proof.result_hash,
            "timestamp": self.get_current_timestamp(),
        });

        Ok(JsValue::from_str(&serde_json::to_string(&result).map_err(
            |e| format!("Failed to serialize result: {}", e),
        )?))
    }

    fn build_validation_result(
        &self,
        is_valid: bool,
        reason: &str,
        proof: &ComputationProof,
    ) -> Result<JsValue, String> {
        let result = serde_json::json!({
            "is_valid": is_valid,
            "reason": reason,
            "proof": proof,
            "validator_node": self.node_id,
            "timestamp": self.get_current_timestamp(),
        });

        Ok(JsValue::from_str(&serde_json::to_string(&result).map_err(
            |e| format!("Failed to serialize validation result: {}", e),
        )?))
    }

    fn validate_spot(
        &self,
        library: &str,
        method: &str,
        input: &[u8],
        params: &[u8],
        claimed_hash: &str,
    ) -> Result<(bool, String), String> {
        let result = match library {
            "atomic" | "science" => self.atomic.borrow_mut().execute(method, input, params),
            "continuum" => self.continuum.borrow_mut().execute(method, input, params),
            "kinetic" => self.kinetic.borrow_mut().execute(method, input, params),
            "math" => self.math.borrow_mut().execute(method, input, params),
            _ => return Ok((false, format!("Unknown library: {}", library))),
        }
        .map_err(|e| format!("{:?}", e))?;

        // Use verification data generation (proof of sampling)
        let _verification_sample = self.generate_verification_data(&result);

        // Verify against expected spot
        let expected = self.compute_expected_spot(claimed_hash, &[]);
        let computed_hash = self.compute_result_hash(&result);

        if computed_hash == expected {
            Ok((true, "Computed hash matches claim".into()))
        } else {
            Ok((
                false,
                format!(
                    "Hash mismatch: claimed={} computed={}",
                    claimed_hash, computed_hash
                ),
            ))
        }
    }

    fn validate_cross_scale(
        &self,
        library: &str,
        method: &str,
        input: &[u8],
        params: &[u8],
        claimed_hash: &str,
    ) -> Result<(bool, String), String> {
        // Cross-scale validation stub
        // Ensure continuum consistency
        let expected = self.compute_expected_continuum_hash(claimed_hash);

        // Delegate to spot check but compare against continuum-adjusted expectation
        self.validate_spot(library, method, input, params, &expected)
    }

    fn validate_full(
        &self,
        library: &str,
        method: &str,
        input: &[u8],
        params: &[u8],
        claimed_hash: &str,
    ) -> Result<(bool, String), String> {
        self.validate_spot(library, method, input, params, claimed_hash)
    }

    fn generate_verification_data(&self, result: &[u8]) -> Vec<u8> {
        // Generate deterministic subset for verification (Proof of Sampling)
        // We take 3 chunks: Start, Middle, End
        let mut data = Vec::new();
        let chunk_size = 1024.min(result.len() / 3);

        if chunk_size > 0 {
            // Start chunk
            data.extend_from_slice(&result[..chunk_size]);

            // Middle chunk
            let mid = result.len() / 2;
            data.extend_from_slice(&result[mid - chunk_size / 2..mid + chunk_size / 2]);

            // End chunk
            data.extend_from_slice(&result[result.len() - chunk_size..]);
        } else {
            // If result is small, just return the whole thing
            data.extend_from_slice(result);
        }
        data
    }

    fn get_current_epoch(&self) -> u64 {
        (sdk::js_interop::get_now() / 1000) as u64 // Unix epoch in seconds as u64
    }

    fn get_current_timestamp(&self) -> u64 {
        self.get_current_epoch()
    }

    fn compute_expected_spot(&self, claimed_hash: &str, _spot_seed: &[u8]) -> String {
        // In a real implementation, we would perform a partial re-computation based on the seed.
        // For now, we assume the claimed hash is the "expected" one if we are just an observer,
        // BUT if we are the validator, we should have re-computed it.
        // This method seems to be a helper for the *Verifier* to check against the *Prover*.
        // Returning the claimed_hash is a placeholder for "I agree" or "I retrieved this from consensus".
        claimed_hash.to_string()
    }

    fn compute_expected_continuum_hash(&self, claimed_hash: &str) -> String {
        // Placeholder for continuum consistency check
        claimed_hash.to_string()
    }
    fn extract_scale_from_params(&self, params: &[u8]) -> Option<SimulationScale> {
        // Try to interpret params as JSON
        if let Ok(params_str) = std::str::from_utf8(params) {
            if let Ok(json) = serde_json::from_str::<serde_json::Value>(params_str) {
                // Check for "scale" or "scale_hint"
                if let Some(scale_val) = json.get("scale").or_else(|| json.get("scale_hint")) {
                    if let Ok(scale) = serde_json::from_value(scale_val.clone()) {
                        return Some(scale);
                    }
                }
            }
        }
        // Future: Binary format extraction
        None
    }
}

// ----------------------------------------------------------------------------
// DEFAULT IMPLEMENTATIONS
// ----------------------------------------------------------------------------
