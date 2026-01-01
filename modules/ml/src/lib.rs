use js_sys::{JsString, Object, Promise};
use std::cell::RefCell;
use std::future::Future;
use std::pin::Pin;
use std::rc::Rc;
use std::task::{Context, Poll, Waker};

use log::{error, info};

// Custom Promise-to-Future adapter (no wasm-bindgen dependency)
// Note: This is a simplified version that doesn't use callbacks
// For production, we'd need a proper callback mechanism
struct PromiseFuture {
    _promise: Promise,
    result: Rc<RefCell<Option<Result<Object, Object>>>>,
    waker: Rc<RefCell<Option<Waker>>>,
}

impl PromiseFuture {
    fn new(promise: Promise) -> Self {
        Self {
            _promise: promise,
            result: Rc::new(RefCell::new(None)),
            waker: Rc::new(RefCell::new(None)),
        }
    }
}

impl Future for PromiseFuture {
    type Output = Result<Object, Object>;

    fn poll(self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<Self::Output> {
        // Check if we have a result
        if let Some(result) = self.result.borrow_mut().take() {
            return Poll::Ready(result);
        }

        // Store waker for later
        *self.waker.borrow_mut() = Some(cx.waker().clone());

        // For now, just return Pending
        // In a full implementation, we'd register callbacks with the promise
        // But that requires wasm-bindgen or a custom callback system
        Poll::Pending
    }
}

// Helper function to replace JsFuture::from()
pub async fn await_promise(promise: Promise) -> Result<Object, Object> {
    PromiseFuture::new(promise).await
}

pub mod brain;
pub mod engine;
pub mod model_capnp {
    #![allow(dead_code, unused_imports, unused_parens)]
    include!(concat!(env!("OUT_DIR"), "/ml/v1/model_capnp.rs"));
}
pub mod jobs;
pub mod models;
pub mod p2p; // New Brain module

pub use brain::CyberneticBrain;
pub use engine::MLEngine;
pub use jobs::inference::InferenceJob as MLJob;

/// Initialize ML module
pub fn init() {
    #[cfg(feature = "console_error_panic_hook")]
    console_error_panic_hook::set_once();

    info!("ML module initialized (with Cybernetic Brain)");
}

/// Standardized Memory Allocator for WebAssembly
#[no_mangle]
pub extern "C" fn ml_alloc(size: usize) -> *mut u8 {
    let mut buf = Vec::with_capacity(size);
    let ptr = buf.as_mut_ptr();
    std::mem::forget(buf);
    ptr
}

/// Standardized Initialization with SharedArrayBuffer
#[no_mangle]
pub extern "C" fn ml_init_with_sab() -> i32 {
    // Use stable ABI to get global object
    let global = sdk::js_interop::get_global();
    sdk::js_interop::console_log("[ml] Init: Global object retrieved", 3);
    let sab_key = sdk::js_interop::create_string("__INOS_SAB__");
    let sab_val = sdk::js_interop::reflect_get(&global, &sab_key);
    sdk::js_interop::console_log("[ml] Init: SAB Value retrieved", 3);

    let offset_key = sdk::js_interop::create_string("__INOS_SAB_OFFSET__");
    let offset_val = sdk::js_interop::reflect_get(&global, &offset_key);

    let size_key = sdk::js_interop::create_string("__INOS_SAB_SIZE__");
    let size_val = sdk::js_interop::reflect_get(&global, &size_key);

    let id_key = sdk::js_interop::create_string("__INOS_MODULE_ID__");
    let id_val = sdk::js_interop::reflect_get(&global, &id_key);

    if let (Ok(val), Ok(off), Ok(sz)) = (sab_val, offset_val, size_val) {
        sdk::js_interop::console_log("[ml] Init: All values retrieved successfully", 3);
        if !val.is_undefined() && !val.is_null() {
            sdk::js_interop::console_log("[ml] Init: SAB is defined and not null", 3);
            let offset = sdk::js_interop::as_f64(&off).unwrap_or(0.0) as u32;
            let size = sdk::js_interop::as_f64(&sz).unwrap_or(0.0) as u32;
            let module_id = id_val.ok().and_then(|v| v.as_f64()).unwrap_or(0.0) as u32;

            // Create TWO SafeSAB references:
            // 1. Scoped view for module data
            let module_sab = sdk::sab::SafeSAB::new_shared_view(val.clone(), offset, size);
            // 2. Global SAB for registry writes
            let global_sab = sdk::sab::SafeSAB::new(val.clone());

            // Set global identity context
            sdk::set_module_id(module_id);

            sdk::init_logging();
            sdk::js_interop::console_log("[ml] DEBUG: Logging initialized", 3);
            sdk::js_interop::console_log(
                &format!(
                    "[ml] ML module init: ID={}, Offset=0x{:x}",
                    module_id, offset
                ),
                1,
            );
            // info!("ML module initialized (ID: {}) with synchronized SAB bridge (Offset: 0x{:x}, Size: {}MB)",
            //     module_id, offset, size / 1024 / 1024);

            // Helper to register simple modules
            let register_ml = |sab: &sdk::sab::SafeSAB| {
                use sdk::registry::*;
                let id = "ml";
                let mut builder = ModuleEntryBuilder::new(id).version(1, 0, 0);
                builder = builder.capability("inference", true, 2048);
                builder = builder.capability("training", true, 4096);
                builder = builder.capability("tensor", true, 1024);
                builder = builder.capability("layers", true, 1024);
                builder = builder.capability("brain", true, 8192);

                match builder.build() {
                    Ok((mut entry, _, caps)) => {
                        if let Ok(offset) = write_capability_table(sab, &caps) {
                            entry.cap_table_offset = offset;
                        }
                        if let Ok((slot, _)) = find_slot_double_hashing(sab, id) {
                            match write_enhanced_entry(sab, slot, &entry) {
                                Ok(_) => info!("Registered module {} at slot {}", id, slot),
                                Err(e) => {
                                    error!("Failed to write registry entry for {}: {:?}", id, e)
                                }
                            }
                        } else {
                            error!("Could not find available slot for module {}", id);
                        }
                    }
                    Err(e) => error!("Failed to build module entry for {}: {:?}", id, e),
                }
            };

            register_ml(&global_sab);

            // Success: Bridge found and identity set
            return 1;
        } else {
            sdk::js_interop::console_log("[ml] Init FAILED: SAB is undefined or null", 1);
        }
    } else {
        sdk::js_interop::console_log("[ml] Init FAILED: Could not retrieve global values", 1);
    }
    0
}

/// External poll entry point for JavaScript
#[no_mangle]
pub extern "C" fn ml_poll() {
    // High-frequency reactor for ML
}

/// WASM exports for ML operations
// Removed wasm_bindgen
pub struct MLModule {
    engine: MLEngine,
    brain: CyberneticBrain,
}

// Removed wasm_bindgen
impl MLModule {
    pub fn new() -> Result<MLModule, Object> {
        let engine = MLEngine::new().map_err(|e| {
            Object::from(JsString::from(format!(
                "Failed to initialize ML engine: {}",
                e
            )))
        })?;

        let brain = CyberneticBrain::new();

        Ok(MLModule { engine, brain })
    }

    /// execute_raw but for JSON payload (Direct Dispatch from JS)
    // Removed wasm_bindgen
    pub async fn execute_json(&self, val: Object) -> Result<Object, String> {
        #[derive(serde::Deserialize)]
        struct DirectRequest {
            library: String,
            method: String,
            #[serde(default)]
            input: Option<Vec<u8>>,
            #[serde(default)]
            params: Option<serde_json::Value>,
        }

        // Parse from JSON string instead of serde_wasm_bindgen
        let json_str = val.as_string().ok_or("Expected JSON string")?;
        let direct: DirectRequest = serde_json::from_str(&json_str).map_err(|e| e.to_string())?;

        let params_str = direct.params.map(|v| v.to_string()).unwrap_or_default();
        let input_bytes = direct.input.unwrap_or_default();

        let result = self
            .execute(&direct.library, &direct.method, &input_bytes, &params_str)
            .map_err(|e| format!("{:?}", e))?;

        Ok(js_sys::Uint8Array::from(&result[..]).into())
    }

    /// Execute ML operation
    pub fn execute(
        &self,
        library: &str,
        method: &str,
        input: &[u8],
        params: &str,
    ) -> Result<Vec<u8>, Object> {
        // Intercept "brain" calls
        if library == "brain" {
            return self.brain.process(method, input, params).map_err(|e| {
                Object::from(JsString::from(format!("Brain execution failed: {}", e)))
            });
        }

        self.engine
            .execute(library, method, input, params)
            .map_err(|e| Object::from(JsString::from(format!("ML execution failed: {}", e))))
    }

    /// Get list of supported methods
    pub fn methods(&self) -> Vec<String> {
        let mut m = self
            .engine
            .methods()
            .iter()
            .map(|s| s.to_string())
            .collect::<Vec<_>>();

        m.extend(vec![
            "brain:predict".to_string(),
            "brain:learn".to_string(),
            "brain:correlate".to_string(),
        ]);

        m
    }
}
