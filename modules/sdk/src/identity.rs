use std::sync::atomic::{AtomicU32, Ordering};

/// Global atomic to store the module ID assigned by the kernel
static MODULE_ID: AtomicU32 = AtomicU32::new(0);

// Removed wasm_bindgen attribute
pub struct IdentityContext {
    node_id: String,
    module_id: u32,
}

impl IdentityContext {
    pub fn new(node_id: String, module_id: u32) -> Self {
        Self { node_id, module_id }
    }

    pub fn node_id(&self) -> &str {
        &self.node_id
    }

    pub fn module_id(&self) -> u32 {
        self.module_id
    }
}

/// Set the global module ID for this instance
pub fn set_module_id(id: u32) {
    MODULE_ID.store(id, Ordering::SeqCst);
}

/// Get the global module ID for this instance
pub fn get_module_id() -> u32 {
    MODULE_ID.load(Ordering::SeqCst)
}
