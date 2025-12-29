pub mod engine;
pub mod executor;
mod units;

use engine::ComputeEngine;
use log::info;
use sdk::{Epoch, Reactor, IDX_SYSTEM_EPOCH};
use units::{AudioUnit, CryptoUnit, DataUnit, GpuUnit, ImageUnit, MLUnit, StorageUnit};

fn initialize_engine() -> ComputeEngine {
    let mut engine = ComputeEngine::new();

    // Register Unit Proxies
    engine.register(Box::new(ImageUnit::new()));
    engine.register(Box::new(CryptoUnit::new()));
    engine.register(Box::new(DataUnit::new()));
    engine.register(Box::new(AudioUnit::new()));
    engine.register(Box::new(GpuUnit::new()));
    engine.register(Box::new(
        StorageUnit::new().expect("Failed to initialize StorageUnit"),
    ));
    engine.register(Box::new(
        MLUnit::new().expect("Failed to initialize MLUnit"),
    ));

    engine
}

/// Initialize compute module with SharedArrayBuffer from global scope
#[no_mangle]
pub extern "C" fn compute_init_with_sab() -> i32 {
    // Use stable ABI to get global object
    let global = sdk::js_interop::get_global();
    // sdk::js_interop::console_log("Init: Global object retrieved", 3);

    let sab_key = sdk::js_interop::create_string("__INOS_SAB__");
    let sab_val = sdk::js_interop::reflect_get(&global, &sab_key);
    // sdk::js_interop::console_log("Init: SAB Value retrieved", 3);

    let offset_key = sdk::js_interop::create_string("__INOS_SAB_OFFSET__");
    let offset_val = sdk::js_interop::reflect_get(&global, &offset_key);

    let size_key = sdk::js_interop::create_string("__INOS_SAB_SIZE__");
    let size_val = sdk::js_interop::reflect_get(&global, &size_key);

    let id_key = sdk::js_interop::create_string("__INOS_MODULE_ID__");
    let id_val = sdk::js_interop::reflect_get(&global, &id_key);

    if let (Ok(val), Ok(off), Ok(sz), Ok(id)) = (sab_val, offset_val, size_val, id_val) {
        if !val.is_undefined() && !val.is_null() {
            let offset = sdk::js_interop::as_f64(&off).unwrap_or(0.0) as u32;
            let size = sdk::js_interop::as_f64(&sz).unwrap_or(0.0) as u32;
            let module_id = id.as_f64().unwrap_or(0.0) as u32;

            let safe_sab = sdk::sab::SafeSAB::new_shared_view(val.clone(), offset, size);

            // Set global identity context
            sdk::set_module_id(module_id);

            sdk::init_logging();
            info!("Compute module initialized (ID: {}) with synchronized SAB bridge (Offset: 0x{:x}, Size: {}MB)", 
                module_id, offset, size / 1024 / 1024);

            // Trigger registration of capabilities
            // Note: In a real scenario, this might happen in a separate "start" phase,
            // but for this demo we register immediately upon init.
            register_compute_capabilities(&safe_sab);

            return 1; // success
        }
    }
    0 // failure
}

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
        let mut builder = ModuleEntryBuilder::new(id).version(1, 0, 0);
        builder = builder.capability("standard", gpu, mem);

        match builder.build() {
            Ok((mut entry, _, caps)) => {
                // No deps in simple mode
                if let Ok(offset) = write_capability_table(sab, &caps) {
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
    // Register core modules provided by this kernel
    register_simple("compute", 512, false); // Base compute

    // Note: specialized units (ml, storage/vault, etc.) register themselves via their own WASM binaries.
    // We do NOT register them here to avoid registry collisions.
}

impl ComputeKernel {
    pub fn new(
        sab: &web_sys::wasm_bindgen::JsValue,
        _offset: u32,
        _size: u32,
        node_id: String,
    ) -> Self {
        sdk::init_logging();
        info!("Compute Kernel initialized on node {}", node_id);

        let engine = initialize_engine();
        let reactor = Reactor::new(sab);

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
        let params = params_reader; // Already &[u8] due to schema change (Data)

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
