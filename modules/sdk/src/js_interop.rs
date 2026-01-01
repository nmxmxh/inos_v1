use web_sys::wasm_bindgen::JsValue;

// Define stable imports that we implement manually in system.ts
#[link(wasm_import_module = "env")]
#[allow(improper_ctypes)]
extern "C" {
    // Stable name: inos_create_u8_array
    #[link_name = "inos_create_u8_array"]
    fn create_u8_array_raw(ptr: *const u8, len: u32) -> JsValue;

    // Stable name: inos_wrap_u8_array
    #[link_name = "inos_wrap_u8_array"]
    fn wrap_u8_array_raw(val: JsValue) -> JsValue;

    // Stable name: inos_create_u8_view
    #[link_name = "inos_create_u8_view"]
    fn create_u8_view_raw(buffer: JsValue, offset: u32, len: u32) -> JsValue;

    // Stable name: inos_create_i32_view
    #[link_name = "inos_create_i32_view"]
    fn create_i32_view_raw(buffer: JsValue, offset: u32, len: u32) -> JsValue;

    // Stable name: inos_get_global
    #[link_name = "inos_get_global"]
    fn get_global_raw() -> JsValue;

    // Stable name: inos_reflect_get
    #[link_name = "inos_reflect_get"]
    fn reflect_get_raw(target: JsValue, key: JsValue) -> JsValue;

    // Stable name: inos_as_f64
    #[link_name = "inos_as_f64"]
    fn as_f64_raw(val: JsValue) -> f64;

    // Stable name: inos_log
    #[link_name = "inos_log"]
    fn log_raw(ptr: *const u8, len: u32, level: u8);

    // Stable name: inos_create_string
    #[link_name = "inos_create_string"]
    fn create_string_raw(ptr: *const u8, len: u32) -> JsValue;

    // Stable name: inos_get_now
    #[link_name = "inos_get_now"]
    fn get_now_raw() -> f64;

    // Stable name: inos_get_performance_now (high-resolution timer)
    #[link_name = "inos_get_performance_now"]
    fn get_performance_now_raw() -> f64;

    // Stable name: inos_atomic_add
    #[link_name = "inos_atomic_add"]
    fn atomic_add_raw(typed_array: JsValue, index: u32, value: i32) -> i32;

    // Stable name: inos_atomic_load
    #[link_name = "inos_atomic_load"]
    fn atomic_load_raw(typed_array: JsValue, index: u32) -> i32;

    // Stable name: inos_atomic_store
    #[link_name = "inos_atomic_store"]
    fn atomic_store_raw(typed_array: JsValue, index: u32, value: i32) -> i32;

    // Stable name: inos_atomic_wait
    #[link_name = "inos_atomic_wait"]
    fn atomic_wait_raw(typed_array: JsValue, index: u32, value: i32, timeout: f64) -> i32;

    // Stable name: inos_atomic_compare_exchange
    #[link_name = "inos_atomic_compare_exchange"]
    fn atomic_compare_exchange_raw(
        typed_array: JsValue,
        index: u32,
        expected: i32,
        replacement: i32,
    ) -> i32;

    // Stable name: inos_math_random
    #[link_name = "inos_math_random"]
    fn math_random_raw() -> f64;

    // Stable name: inos_copy_to_sab
    #[link_name = "inos_copy_to_sab"]
    fn copy_to_sab_raw(target_buffer: JsValue, target_offset: u32, src_ptr: *const u8, len: u32);

    // Stable name: inos_copy_from_sab
    #[link_name = "inos_copy_from_sab"]
    fn copy_from_sab_raw(src_buffer: JsValue, src_offset: u32, dest_ptr: *mut u8, len: u32);

    // Stable name: inos_get_byte_length
    #[link_name = "inos_get_byte_length"]
    fn get_byte_length_raw(val: JsValue) -> u32;
}

/// Create a string on the JS heap using the stable ABI
pub fn create_string(s: &str) -> JsValue {
    unsafe { create_string_raw(s.as_ptr(), s.len() as u32) }
}

/// Create a Uint8Array from a Rust byte slice using the stable ABI
pub fn create_u8_array(data: &[u8]) -> js_sys::Uint8Array {
    unsafe {
        let val = create_u8_array_raw(data.as_ptr(), data.len() as u32);
        val.into()
    }
}

/// Wrap an existing JS object (ArrayBuffer, etc.) as a Uint8Array using stable ABI
pub fn wrap_u8_array(val: JsValue) -> js_sys::Uint8Array {
    unsafe {
        let val = wrap_u8_array_raw(val);
        val.into()
    }
}

/// Create a Uint8Array view on an existing buffer using stable ABI
pub fn create_u8_view(buffer: JsValue, offset: u32, len: u32) -> js_sys::Uint8Array {
    unsafe {
        let val = create_u8_view_raw(buffer, offset, len);
        val.into()
    }
}

/// Create a Int32Array view on an existing buffer using stable ABI
pub fn create_i32_view(buffer: JsValue, offset: u32, len: u32) -> js_sys::Int32Array {
    unsafe {
        let val = create_i32_view_raw(buffer, offset, len);
        val.into()
    }
}

/// Log a message using the stable ABI
/// level: 0=error, 1=warn, 2=info, 3=debug, 4=trace
pub fn console_log(msg: &str, level: u8) {
    unsafe {
        log_raw(msg.as_ptr(), msg.len() as u32, level);
    }
}

/// Log an error message using the stable ABI (convenience wrapper)
pub fn console_error(msg: &str) {
    console_log(msg, 0);
}

/// Get the global object (window/self/global) using stable ABI
pub fn get_global() -> JsValue {
    unsafe { get_global_raw() }
}

/// Get a property from a JS object using stable ABI (Reflect.get)
pub fn reflect_get(target: &JsValue, key: &JsValue) -> Result<JsValue, JsValue> {
    Ok(unsafe { reflect_get_raw(target.clone(), key.clone()) })
}

/// Convert JsValue to f64 using stable ABI
pub fn as_f64(val: &JsValue) -> Option<f64> {
    let res = unsafe { as_f64_raw(val.clone()) };
    if res.is_nan() {
        if val.is_undefined() || val.is_null() {
            None
        } else {
            Some(res)
        }
    } else {
        Some(res)
    }
}

/// Get current time in milliseconds since epoch using stable ABI
pub fn get_now() -> u64 {
    unsafe { get_now_raw() as u64 }
}

/// Get high-resolution time in fractional milliseconds (microsecond precision)
/// Uses performance.now() which is monotonic and high-resolution
pub fn get_performance_now() -> f64 {
    unsafe { get_performance_now_raw() }
}

/// Atomic add using stable ABI
pub fn atomic_add(typed_array: &JsValue, index: u32, value: i32) -> i32 {
    unsafe { atomic_add_raw(typed_array.clone(), index, value) }
}

/// Atomic load using stable ABI
pub fn atomic_load(typed_array: &JsValue, index: u32) -> i32 {
    unsafe { atomic_load_raw(typed_array.clone(), index) }
}

/// Atomic store using stable ABI
pub fn atomic_store(typed_array: &JsValue, index: u32, value: i32) -> i32 {
    unsafe { atomic_store_raw(typed_array.clone(), index, value) }
}

/// Atomic wait using stable ABI
/// returns: 0=ok, 1=not-equal, 2=timed-out
pub fn atomic_wait(typed_array: &JsValue, index: u32, value: i32, timeout_ms: f64) -> i32 {
    unsafe { atomic_wait_raw(typed_array.clone(), index, value, timeout_ms) }
}

/// Atomic compare-exchange using stable ABI
/// returns: old value
pub fn atomic_compare_exchange(
    typed_array: &JsValue,
    index: u32,
    expected: i32,
    replacement: i32,
) -> i32 {
    unsafe { atomic_compare_exchange_raw(typed_array.clone(), index, expected, replacement) }
}

/// Math.random() using stable ABI
pub fn math_random() -> f64 {
    unsafe { math_random_raw() }
}

/// Copy data from WASM memory to an external SAB using stable ABI
pub fn copy_to_sab(target_buffer: &JsValue, target_offset: u32, data: &[u8]) {
    unsafe {
        copy_to_sab_raw(
            target_buffer.clone(),
            target_offset,
            data.as_ptr(),
            data.len() as u32,
        )
    }
}

/// Copy data from an external SAB to WASM memory using stable ABI
pub fn copy_from_sab(src_buffer: &JsValue, src_offset: u32, dest: &mut [u8]) {
    unsafe {
        copy_from_sab_raw(
            src_buffer.clone(),
            src_offset,
            dest.as_mut_ptr(),
            dest.len() as u32,
        )
    }
}

/// Get byte length of a JS object (ArrayBuffer, etc.) using stable ABI
pub fn get_byte_length(val: &JsValue) -> u32 {
    unsafe { get_byte_length_raw(val.clone()) }
}
