pub mod engine;
pub mod executor;
pub mod units;

#[cfg(target_arch = "wasm32")]
getrandom::register_custom_getrandom!(sdk::js_interop::getrandom_custom);

#[cfg(test)]
pub mod benchmarks;

use engine::ComputeEngine;
use log::{info, warn};
use sdk::{Epoch, Reactor, IDX_SYSTEM_EPOCH};
use units::{
    AudioUnit, BoidUnit, CryptoUnit, DataUnit, GpuUnit, ImageUnit, MathUnit, PhysicsEngine,
    VideoUnit,
};

// --- PERSISTENT SAB CACHE ---
use std::sync::OnceLock;

// Use OnceLock for thread-safe one-time initialization without spin-waiting
static GLOBAL_SAB: OnceLock<sdk::sab::SafeSAB> = OnceLock::new();

pub(crate) fn set_cached_sab(sab: sdk::sab::SafeSAB) {
    let _ = GLOBAL_SAB.set(sab);
}

pub(crate) fn get_cached_sab() -> Option<sdk::sab::SafeSAB> {
    GLOBAL_SAB.get().cloned()
}

// Use OnceLock for engine to avoid lock overhead on every access
static COMPUTE_ENGINE: OnceLock<ComputeEngine> = OnceLock::new();

fn get_engine() -> &'static ComputeEngine {
    COMPUTE_ENGINE.get_or_init(|| initialize_engine())
}

fn initialize_engine() -> ComputeEngine {
    use std::sync::Arc;
    let mut engine = ComputeEngine::new();

    // Register Unit Proxies (Arc for thread-safety)
    engine.register(Arc::new(ImageUnit::new()));
    engine.register(Arc::new(CryptoUnit::new()));
    engine.register(Arc::new(DataUnit::new()));
    engine.register(Arc::new(AudioUnit::new()));
    engine.register(Arc::new(GpuUnit::new()));
    engine.register(Arc::new(PhysicsEngine::new()));
    engine.register(Arc::new(BoidUnit::new()));
    engine.register(Arc::new(VideoUnit::new()));
    // NOTE: ApiProxy is NOT registered here - it's handled separately
    // due to browser API constraints (HTTP/WebSocket use non-Send types)
    engine.register(Arc::new(MathUnit::new()));
    // NOTE: StorageUnit is NOT registered here - it's handled separately via
    // StorageSupervisor due to browser API constraints (IndexedDB/OPFS are non-Send)

    engine
}

/// Standardized Memory Allocator for WebAssembly
#[no_mangle]
pub extern "C" fn compute_alloc(size: usize) -> *mut u8 {
    let mut buf = Vec::with_capacity(size);
    let ptr = buf.as_mut_ptr();
    std::mem::forget(buf);
    ptr
}

/// Standardized Memory Deallocator for WebAssembly
#[no_mangle]
pub extern "C" fn compute_free(ptr: *mut u8, size: usize) {
    if !ptr.is_null() {
        unsafe {
            let _ = Vec::from_raw_parts(ptr, 0, size);
        }
    }
}

/// Standardized Initialization with SharedArrayBuffer
#[no_mangle]
pub extern "C" fn compute_init_with_sab() -> i32 {
    // Use stable ABI to get global object
    let global = sdk::js_interop::get_global();
    sdk::js_interop::console_log("[compute] Init: Global object retrieved", 3);

    let sab_key = sdk::js_interop::create_string("__INOS_SAB__");
    let sab_val = sdk::js_interop::reflect_get(&global, &sab_key);
    sdk::js_interop::console_log("[compute] Init: SAB Value retrieved", 3);

    let offset_key = sdk::js_interop::create_string("__INOS_SAB_OFFSET__");
    let offset_val = sdk::js_interop::reflect_get(&global, &offset_key);

    let size_key = sdk::js_interop::create_string("__INOS_SAB_SIZE__");
    let size_val = sdk::js_interop::reflect_get(&global, &size_key);

    let id_key = sdk::js_interop::create_string("__INOS_MODULE_ID__");
    let id_val = sdk::js_interop::reflect_get(&global, &id_key);

    if let (Ok(val), Ok(off), Ok(sz), Ok(id)) = (sab_val, offset_val, size_val, id_val) {
        sdk::js_interop::console_log("[compute] Init: All values retrieved successfully", 3);
        if !val.is_undefined() && !val.is_null() {
            sdk::js_interop::console_log("[compute] Init: SAB is defined and not null", 3);
            let offset = sdk::js_interop::as_f64(&off).unwrap_or(0.0) as u32;
            let size = sdk::js_interop::as_f64(&sz).unwrap_or(0.0) as u32;
            let module_id = sdk::js_interop::as_f64(&id).unwrap_or(0.0) as u32;

            // Create TWO SafeSAB references:
            // 1. Scoped view for module data (offset-based)
            let _module_sab = sdk::sab::SafeSAB::new_shared_view(&val, offset, size);
            // 2. Global SAB for registry and buffer writes (uses absolute layout offsets)
            let global_sab = sdk::sab::SafeSAB::new(&val);

            // Set global identity context
            sdk::set_module_id(module_id);
            sdk::identity::init_identity_from_js();

            sdk::init_logging();

            // Set global barrier view for zero-copy context verification
            sdk::sab::set_global_barrier_view(global_sab.barrier_view().clone());

            // Capture the initial context ID to prevent zombie execution
            sdk::init_context();

            info!("Compute module initialized (ID: {}) with synchronized SAB bridge (Offset: 0x{:x}, Size: {}MB)", 
                module_id, offset, size / 1024 / 1024);

            // CACHE THE SAB for high-frequency access
            set_cached_sab(global_sab.clone());

            // Trigger registration of capabilities using GLOBAL SAB
            register_compute_capabilities(&global_sab);

            return 1; // success
        } else {
            sdk::js_interop::console_log("[compute] Init FAILED: SAB is undefined or null", 1);
        }
    } else {
        sdk::js_interop::console_log("[compute] Init FAILED: Could not retrieve global values", 1);
    }
    0 // failure
}

// --- GENERIC UNIT DISPATCHER ---
// This allows JS to call ANY registered unit method via a single entry point

/// Generic compute execution dispatcher
/// Allows JavaScript to call any registered unit via:
///   compute_execute("math", "matrix_multiply", input_ptr, input_len, params_ptr, params_len)
///
/// Returns: pointer to result buffer (first 4 bytes = length, rest = data), or 0 on error
///
/// Example usage from JS:
///   const result = compute_execute("math", "matrix_identity", 0, 0, paramsPtr, paramsLen);
#[no_mangle]
pub extern "C" fn compute_execute(
    service_ptr: *const u8,
    service_len: usize,
    action_ptr: *const u8,
    action_len: usize,
    input_ptr: *const u8,
    input_len: usize,
    params_ptr: *const u8,
    params_len: usize,
) -> *mut u8 {
    // 0. Context Validation
    if !sdk::is_context_valid() {
        sdk::js_interop::console_log(
            "[compute_execute] FAILED: Context is invalid (Zombie Module)",
            2,
        );
        return std::ptr::null_mut();
    }

    // 1. Marshall service name
    let service = unsafe {
        if service_ptr.is_null() || service_len == 0 {
            sdk::js_interop::console_log(
                "[compute_execute] FAILED: service_ptr is null or len=0",
                1,
            );
            return std::ptr::null_mut();
        }
        let bytes = std::slice::from_raw_parts(service_ptr, service_len);
        match std::str::from_utf8(bytes) {
            Ok(s) => s,
            Err(_) => {
                let bytes_hex: Vec<String> =
                    bytes.iter().take(8).map(|b| format!("{:02x}", b)).collect();
                let msg = format!("[compute_execute] FAILED: service name not valid UTF-8. Ptr: {:p}, Len: {}, Bytes(8): {:?}", 
                    service_ptr, service_len, bytes_hex);
                sdk::js_interop::console_log(&msg, 1);
                return std::ptr::null_mut();
            }
        }
    };

    // 2. Marshall action name
    let action = unsafe {
        if action_ptr.is_null() || action_len == 0 {
            sdk::js_interop::console_log(
                "[compute_execute] FAILED: action_ptr is null or len=0",
                1,
            );
            return std::ptr::null_mut();
        }
        match std::str::from_utf8(std::slice::from_raw_parts(action_ptr, action_len)) {
            Ok(s) => s,
            Err(_) => {
                sdk::js_interop::console_log(
                    "[compute_execute] FAILED: action name is not valid UTF-8",
                    1,
                );
                return std::ptr::null_mut();
            }
        }
    };

    let input = unsafe {
        if input_ptr.is_null() || input_len == 0 {
            &[]
        } else {
            std::slice::from_raw_parts(input_ptr, input_len)
        }
    };

    let params = unsafe {
        if params_ptr.is_null() || params_len == 0 {
            b"{}"
        } else {
            std::slice::from_raw_parts(params_ptr, params_len)
        }
    };

    // Run the async execute in a synchronous context by polling it once
    // We CANNOT use block_on because it uses Atomics.wait which is forbidden on main thread

    // Initialize engine if needed (thread-safe spinlock)
    let engine = get_engine();
    let result = match poll_sync(engine.execute(service, action, input, params)) {
        Ok(res) => res,
        Err(e) => {
            let msg = format!("[compute_execute] Sync Execution Failed: {}", e);
            sdk::js_interop::console_log(&msg, 1);
            return std::ptr::null_mut();
        }
    };

    // Debug logging removed - was running 120+ times/sec at 60 FPS

    match result {
        Ok(output) => {
            let output: Vec<u8> = output; // Explicit type annotation
                                          // Allocate result buffer: 4 bytes for length + output data
            let total_len = 4 + output.len();
            let mut buffer = Vec::with_capacity(total_len);
            buffer.extend_from_slice(&(output.len() as u32).to_le_bytes());
            buffer.extend_from_slice(&output);

            let ptr = buffer.as_mut_ptr();
            std::mem::forget(buffer);
            ptr
        }
        Err(e) => {
            // Log error and return null
            let msg = format!("[compute_execute] Logic Error: {}", e);
            sdk::js_interop::console_log(&msg, 1);
            std::ptr::null_mut()
        }
    }
}

/// Zero-Copy Protocol Dispatcher
/// Standardizes communication with Cap'n Proto JobRequest capsule
/// Standard usage: compute_dispatch(request_ptr, request_len)
#[no_mangle]
pub extern "C" fn compute_dispatch(request_ptr: *const u8, request_len: usize) -> *mut u8 {
    if !sdk::is_context_valid() || request_ptr.is_null() || request_len == 0 {
        return std::ptr::null_mut();
    }

    let request_bytes = unsafe { std::slice::from_raw_parts(request_ptr, request_len) };
    let mut reader = std::io::Cursor::new(request_bytes);

    // Read message from slice (zero-copy from WASM heap perspective)
    let message_reader =
        match capnp::serialize::read_message(&mut reader, capnp::message::ReaderOptions::new()) {
            Ok(r) => r,
            Err(_) => return std::ptr::null_mut(),
        };

    let job =
        match message_reader.get_root::<sdk::protocols::compute::compute::job_request::Reader>() {
            Ok(j) => j,
            Err(_) => return std::ptr::null_mut(),
        };

    // Extract fields using zero-copy lenses
    let service = job
        .get_library() // Field remains 'library' in schema for compatibility
        .unwrap_or(capnp::text::Reader::from(""))
        .to_str()
        .unwrap_or("");
    let action = job
        .get_method() // Field remains 'method' in schema for compatibility
        .unwrap_or(capnp::text::Reader::from(""))
        .to_str()
        .unwrap_or("");
    let input = job.get_input().unwrap_or(&[]);

    let params_reader = job.get_params().unwrap();
    let params = match params_reader.which().unwrap() {
        sdk::protocols::compute::compute::job_params::Which::Binary(data) => data.unwrap_or(&[]),
        _ => &[], // Structured params handled inside specialized units if needed
    };

    let engine = get_engine();
    let result = match poll_sync(engine.execute(service, action, input, params)) {
        Ok(res) => res,
        Err(_) => return std::ptr::null_mut(),
    };

    match result {
        Ok(output) => {
            // Standardize output wrapping: [len:u32][data...]
            let total_len = 4 + output.len();
            let mut buffer = Vec::with_capacity(total_len);
            buffer.extend_from_slice(&(output.len() as u32).to_le_bytes());
            buffer.extend_from_slice(&output);

            let ptr = buffer.as_mut_ptr();
            std::mem::forget(buffer);
            ptr
        }
        Err(_) => std::ptr::null_mut(),
    }
}

/// Helper to poll a future once synchronously
/// Panics or errors if the future yields (is not ready immediately)
fn poll_sync<T>(future: impl std::future::Future<Output = T>) -> Result<T, String> {
    use std::task::{Context, Poll, RawWaker, RawWakerVTable, Waker};

    unsafe fn clone(_: *const ()) -> RawWaker {
        RawWaker::new(std::ptr::null(), &VTABLE)
    }
    unsafe fn wake(_: *const ()) {}
    unsafe fn wake_by_ref(_: *const ()) {}
    unsafe fn drop(_: *const ()) {}

    static VTABLE: RawWakerVTable = RawWakerVTable::new(clone, wake, wake_by_ref, drop);

    // Safety: we are single-threaded on WASM main thread usually, or this is just a dummy waker
    let raw_waker = RawWaker::new(std::ptr::null(), &VTABLE);
    let waker = unsafe { Waker::from_raw(raw_waker) };
    let mut cx = Context::from_waker(&waker);

    let mut pinned = std::pin::pin!(future);
    match pinned.as_mut().poll(&mut cx) {
        Poll::Ready(val) => Ok(val),
        Poll::Pending => {
            Err("Future yielded! compute_execute requires synchronous completion.".to_string())
        }
    }
}
// ======================================================================
// LEGACY EXPORTS REMOVED
// ======================================================================
// All compute operations now use compute_execute() for consistency.
// The previous direct exports (compute_boids_step, compute_nbody_step, etc.)
// are now routed through the ComputeEngine via their respective units.
// ======================================================================

pub struct ComputeKernel {
    reactor: Reactor,
    engine: ComputeEngine,
    epoch: Epoch,
}

// Helper to register capabilities (moved from ComputeKernel::new to be standalone)
fn register_compute_capabilities(sab: &sdk::sab::SafeSAB) {
    use sdk::registry::*;

    // Helper to register simple modules
    let register_simple = |id: &str, mem: u16, gpu: bool| {
        let mut builder = ModuleEntryBuilder::new(id).version(1, 4, 3);
        builder = builder.capability("image", gpu, mem);
        builder = builder.capability("video", gpu, mem);
        builder = builder.capability("audio", gpu, mem);
        builder = builder.capability("crypto", gpu, mem);
        builder = builder.capability("data", gpu, mem);
        builder = builder.capability("gpu_shader", gpu, 4096);

        match builder.build() {
            Ok((mut entry, _, caps)) => {
                // No deps in simple mode
                if let Ok(offset) = write_capability_table(sab, &caps) {
                    info!(
                        "[VERIFY] Cap table written to offset 0x{:x}, {} entries",
                        offset,
                        caps.len()
                    );

                    // Immediately verify the write by reading back
                    if let Ok(verify_data) = sab.read(offset as usize, 16) {
                        info!("[VERIFY] First 16 bytes after write: {:02x?}", verify_data);

                        // Check if first 4 bytes are the capability name or zeros
                        if verify_data[0] == 0
                            && verify_data[1] == 0
                            && verify_data[2] == 0
                            && verify_data[3] == 0
                        {
                            warn!("[VERIFY] ⚠️  CORRUPTION DETECTED: First 4 bytes are zeros immediately after write!");
                        } else {
                            info!("[VERIFY] ✓ Data intact: first 4 bytes = {:02x} {:02x} {:02x} {:02x}", 
                                verify_data[0], verify_data[1], verify_data[2], verify_data[3]);
                        }
                    }

                    entry.cap_table_offset = offset;
                }
                if let Ok((slot, _)) = find_slot_double_hashing(sab, id) {
                    if let Err(e) = write_enhanced_entry(sab, slot, &entry) {
                        info!("Failed to write module {}: {}", id, e);
                    } else {
                        info!("Registered module {} at slot {}", id, slot);
                    }
                }
            }
            Err(e) => info!("Failed to build module {}: {:?}", id, e),
        }
    };

    // Register core modules provided by this kernel
    // Dynamically register all units from the engine
    let engine = get_engine();
    let capabilities = engine.generate_capability_registry();

    let mut builder = ModuleEntryBuilder::new("compute").version(1, 5, 0);
    // Add all discovered capabilities to the 'compute' module entry
    // In a production system, these might be separate modules, but for the monolithic kernel
    // we export them through the primary discovery unit.
    for cap_str in &capabilities {
        // cap_str is "service:action:v1"
        let parts: Vec<&str> = cap_str.split(':').collect();
        if parts.len() >= 2 {
            builder = builder.capability(parts[1], false, 512);
        }
    }

    match builder.build() {
        Ok((mut entry, _, caps)) => {
            if let Ok(offset) = write_capability_table(sab, &caps) {
                entry.cap_table_offset = offset;
            }
            if let Ok((slot, _)) = find_slot_double_hashing(sab, "compute") {
                let _ = write_enhanced_entry(sab, slot, &entry);
            }
        }
        Err(e) => info!("Failed to auto-register compute: {:?}", e),
    }

    // Register specialized 'boids' unit separately for legacy UI compatibility
    register_simple("boids", 512, false);

    // Signal registry change to wake Go discovery loop immediately
    sdk::registry::signal_registry_change(sab);

    // Note: specialized units (ml, storage/vault, etc.) register themselves via their own WASM binaries.
    // We do NOT register them here to avoid registry collisions.
}

impl ComputeKernel {
    pub fn new(sab: sdk::sab::SafeSAB, node_id: String) -> Self {
        sdk::init_logging();
        info!("Compute Kernel initialized on node {}", node_id);

        let engine = initialize_engine();
        let reactor = Reactor::new(sab.clone());

        // Use standardized System Epoch index from SDK
        let epoch = Epoch::new(sab, IDX_SYSTEM_EPOCH);

        // No need to call register_compute_capabilities here anymore,
        // it's already done in compute_init_with_sab using the correct safe_sab.

        Self {
            reactor,
            engine,
            epoch,
        }
    }

    /// Poll for new compute segments using Reactive Mutation
    pub async fn poll(&mut self) -> bool {
        if !self.reactor.check_inbox() {
            return false;
        }

        self.reactor.ack_inbox();

        // 1. Get Inbox data and copy to buffer
        let data = match self.reactor.read_request() {
            Some(d) => d,
            None => return false,
        };

        // 2. Execute via Engine
        // Use proper Cap'n Proto processing
        let result = self.process_job(&data).await;

        match result {
            Ok(output) => {
                // Return success result
                if let Ok(serialized) = self.serialize_result(true, &output, "") {
                    if !self.reactor.write_result(&serialized) {
                        log::error!("Output too large for outbox: {} bytes", serialized.len());
                        // Write error result
                        if let Ok(err_bytes) = self.serialize_result(false, &[], "Output too large")
                        {
                            self.reactor.write_result(&err_bytes);
                        }
                    }
                }
            }
            Err(e) => {
                log::error!("Compute job failed: {}", e);
                // Write error result
                if let Ok(err_bytes) = self.serialize_result(false, &[], &e.to_string()) {
                    self.reactor.write_result(&err_bytes);
                }
            }
        }

        // 3. Signal completion via Epoch
        self.epoch.increment();

        true
    }

    /// Process job using Cap'n Proto "Lens"
    async fn process_job(&self, data: &[u8]) -> Result<Vec<u8>, engine::ComputeError> {
        let mut reader = std::io::Cursor::new(data);
        let message_reader =
            capnp::serialize::read_message(&mut reader, capnp::message::ReaderOptions::new())
                .map_err(|e| {
                    engine::ComputeError::ExecutionFailed(format!("Capnp read error: {}", e))
                })?;

        // Access the lens
        let job = message_reader
            .get_root::<sdk::protocols::compute::compute::job_request::Reader>()
            .map_err(|e| {
                engine::ComputeError::ExecutionFailed(format!("Capnp root error: {}", e))
            })?;

        // Zero-copy field access
        let library_reader = job
            .get_library()
            .map_err(|_| engine::ComputeError::ExecutionFailed("Invalid library field".into()))?;
        let library = library_reader
            .to_str()
            .map_err(|_| engine::ComputeError::ExecutionFailed("Library not valid UTF-8".into()))?;

        let method_reader = job
            .get_method()
            .map_err(|_| engine::ComputeError::ExecutionFailed("Invalid method field".into()))?;
        let method = method_reader
            .to_str()
            .map_err(|_| engine::ComputeError::ExecutionFailed("Method not valid UTF-8".into()))?;

        let params_reader = job
            .get_params()
            .map_err(|_| engine::ComputeError::ExecutionFailed("Invalid params field".into()))?;

        let params_bytes = match params_reader.which() {
            Ok(sdk::protocols::compute::compute::job_params::Which::Binary(data)) => {
                data.map_err(|_| {
                    engine::ComputeError::ExecutionFailed("Invalid binary params".into())
                })?
            }
            Ok(sdk::protocols::compute::compute::job_params::Which::CustomParams(custom_res)) => {
                let custom = custom_res.map_err(|_| {
                    engine::ComputeError::ExecutionFailed("Invalid custom params".into())
                })?;
                custom
                    .get_shader_source()
                    .map_err(|_| {
                        engine::ComputeError::ExecutionFailed("Invalid shader field".into())
                    })?
                    .as_bytes()
            }
            _ => &[], // Other structured types fall back to empty bytes for generic engines
        };

        let params = params_bytes;

        let input = job
            .get_input()
            .map_err(|_| engine::ComputeError::ExecutionFailed("Invalid input field".into()))?;

        info!(
            "Engine execution (Capnp): unit={}, action={}, input_size={}",
            library,
            method,
            input.len()
        );

        self.engine.execute(library, method, input, params).await
    }

    /// Helper to serialize JobResult
    fn serialize_result(
        &self,
        success: bool,
        data: &[u8],
        error_msg: &str,
    ) -> Result<Vec<u8>, engine::ComputeError> {
        let mut message = capnp::message::Builder::new_default();
        let mut root = message.init_root::<sdk::protocols::compute::compute::job_result::Builder>();

        // Set status
        if success {
            root.set_status(sdk::protocols::compute::compute::Status::Success);
        } else {
            root.set_status(sdk::protocols::compute::compute::Status::Failed);
        }

        // Set output
        root.set_output(data);

        // Set error message
        root.set_error_message(error_msg);

        let mut output_bytes = Vec::new();
        capnp::serialize::write_message(&mut output_bytes, &message).map_err(|e| {
            engine::ComputeError::ExecutionFailed(format!("Serialize error: {}", e))
        })?;

        Ok(output_bytes)
    }
}
