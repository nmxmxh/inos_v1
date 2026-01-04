use crate::js_interop;
use once_cell::sync::Lazy;
use std::sync::Mutex;

/// The context ID that was active when this module was initialized.
static INITIAL_CONTEXT_ID: Lazy<Mutex<Option<String>>> = Lazy::new(|| Mutex::new(None));

/// Captures the current `window.__INOS_CONTEXT_ID__` as the "Initial" context.
/// This should be called once during `init_with_sab`.
pub fn init_context() {
    if let Some(current_id) = get_current_context_id() {
        if let Ok(mut initial) = INITIAL_CONTEXT_ID.lock() {
            if initial.is_none() {
                js_interop::console_log(&format!("[SDK] Context Initialized: {}", current_id), 3);
                *initial = Some(current_id);
            }
        }
    }
}

/// Checks if the current global context ID matches the one captured at initialization.
/// Returns `true` if the context is still valid, `false` if the module is now a "Zombie".
pub fn is_valid() -> bool {
    let current = get_current_context_id();
    let initial = INITIAL_CONTEXT_ID
        .lock()
        .ok()
        .and_then(|guard| guard.clone());

    match (current, initial) {
        (Some(curr), Some(init)) => curr == init,
        // If we haven't initialized the context check yet, assume valid for backward compatibility
        (_, None) => true,
        // If we can't get the current context, something is wrong, but we don't necessarily kill it
        (None, _) => true,
    }
}

/// Helper to read the current context ID from the JS global scope.
fn get_current_context_id() -> Option<String> {
    let global = js_interop::get_global();
    if global.is_undefined() || global.is_null() {
        return None;
    }

    let key = js_interop::create_string("__INOS_CONTEXT_ID__");
    if let Ok(val) = js_interop::reflect_get(&global, &key) {
        if val.is_string() {
            // We need a way to convert JsValue (string) to Rust String in js_interop
            // For now, we'll assume js_interop provides this or add it.
            return js_interop::js_to_string(&val);
        }
    }
    None
}
