#[cfg(target_arch = "wasm32")]
pub use web_sys::wasm_bindgen::JsValue;

#[cfg(not(target_arch = "wasm32"))]
#[derive(Debug, Clone, PartialEq, Default)]
pub struct JsValue(u32);

#[cfg(not(target_arch = "wasm32"))]
impl JsValue {
    pub const NULL: JsValue = JsValue(0);
    pub const UNDEFINED: JsValue = JsValue(1);
    pub fn is_null(&self) -> bool {
        self.0 == 0
    }
    pub fn is_undefined(&self) -> bool {
        self.0 == 1
    }
    pub fn is_string(&self) -> bool {
        false
    }
    pub fn as_f64(&self) -> Option<f64> {
        None
    }
    pub fn as_string(&self) -> Option<String> {
        None
    }
}

#[cfg(not(target_arch = "wasm32"))]
impl From<&str> for JsValue {
    fn from(_s: &str) -> Self {
        JsValue::UNDEFINED
    }
}

#[cfg(not(target_arch = "wasm32"))]
impl From<String> for JsValue {
    fn from(_s: String) -> Self {
        JsValue::UNDEFINED
    }
}

#[cfg(target_arch = "wasm32")]
pub type Uint8Array = js_sys::Uint8Array;
#[cfg(not(target_arch = "wasm32"))]
pub type Uint8Array = JsValue;

#[cfg(target_arch = "wasm32")]
pub type Int32Array = js_sys::Int32Array;
#[cfg(not(target_arch = "wasm32"))]
pub type Int32Array = JsValue;

#[cfg(target_arch = "wasm32")]
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

    // Stable name: inos_js_to_string
    #[link_name = "inos_js_to_string"]
    fn js_to_string_raw(val: JsValue, ptr: *mut u8, max_len: u32) -> u32;
}

#[cfg(not(target_arch = "wasm32"))]
mod native_mock {
    use super::*;
    use once_cell::sync::Lazy;
    use std::sync::Mutex;

    // A mock registry of buffers. JsValue::from(u32) will be treated as an index.
    static BUFFERS: Lazy<Mutex<Vec<Vec<u8>>>> = Lazy::new(|| Mutex::new(vec![]));

    #[allow(dead_code)]
    pub fn register_buffer(data: Vec<u8>) -> u32 {
        let mut buffers = BUFFERS.lock().unwrap();
        buffers.push(data);
        (buffers.len() - 1) as u32
    }

    fn get_buffer_index(_val: &JsValue) -> usize {
        // In native tests, JSValue is just a wrapper for a pointer or hash
        // For now, we'll just return 0 for simplicity as most tests use a single SAB.
        0
    }

    fn ensure_buffer_initialized() {
        let mut buffers = BUFFERS.lock().unwrap();
        if buffers.is_empty() {
            buffers.push(vec![0u8; 16 * 1024 * 1024]);
        }
    }

    pub fn create_u8_array(_data: &[u8]) -> Uint8Array {
        JsValue::UNDEFINED
    }
    pub fn wrap_u8_array(_val: &JsValue) -> Uint8Array {
        JsValue::UNDEFINED
    }
    pub fn create_u8_view(_buffer: &JsValue, _offset: u32, _len: u32) -> Uint8Array {
        JsValue::UNDEFINED
    }
    pub fn create_i32_view(_buffer: &JsValue, _offset: u32, _len: u32) -> Int32Array {
        JsValue::UNDEFINED
    }
    pub fn log(msg: &str, level: u8) {
        println!("[LOG {}] {}", level, msg);
    }
    pub fn create_string(_s: &str) -> JsValue {
        JsValue::UNDEFINED
    }
    pub fn get_now() -> u64 {
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_millis() as u64
    }
    pub fn get_performance_now() -> f64 {
        0.0
    }

    pub fn get_global() -> JsValue {
        JsValue::UNDEFINED
    }

    pub fn reflect_get(_target: &JsValue, _key: &JsValue) -> Result<JsValue, JsValue> {
        Ok(JsValue::UNDEFINED)
    }

    pub fn js_to_string(_val: &JsValue) -> Option<String> {
        None
    }

    pub fn get_byte_length(_val: &JsValue) -> u32 {
        ensure_buffer_initialized();
        let buffers = BUFFERS.lock().unwrap();
        buffers[0].len() as u32
    }

    pub fn atomic_add(val: &JsValue, index: u32, value: i32) -> i32 {
        ensure_buffer_initialized();
        let mut buffers = BUFFERS.lock().unwrap();
        let idx = get_buffer_index(val);
        if idx >= buffers.len() {
            return 0;
        }
        let buf = &mut buffers[idx];
        let byte_idx = (index * 4) as usize;
        if byte_idx + 4 > buf.len() {
            return 0;
        }

        let old = i32::from_le_bytes(buf[byte_idx..byte_idx + 4].try_into().unwrap());
        let new = old + value;
        buf[byte_idx..byte_idx + 4].copy_from_slice(&new.to_le_bytes());
        old
    }

    pub fn atomic_load(val: &JsValue, index: u32) -> i32 {
        ensure_buffer_initialized();
        let buffers = BUFFERS.lock().unwrap();
        let idx = get_buffer_index(val);
        if idx >= buffers.len() {
            return 0;
        }
        let buf = &buffers[idx];
        let byte_idx = (index * 4) as usize;
        if byte_idx + 4 > buf.len() {
            return 0;
        }
        i32::from_le_bytes(buf[byte_idx..byte_idx + 4].try_into().unwrap())
    }

    pub fn atomic_store(val: &JsValue, index: u32, value: i32) -> i32 {
        ensure_buffer_initialized();
        let mut buffers = BUFFERS.lock().unwrap();
        let idx = get_buffer_index(val);
        if idx >= buffers.len() {
            return 0;
        }
        let buf = &mut buffers[idx];
        let byte_idx = (index * 4) as usize;
        if byte_idx + 4 > buf.len() {
            return 0;
        }

        buf[byte_idx..byte_idx + 4].copy_from_slice(&value.to_le_bytes());
        value
    }

    pub fn atomic_wait(_typed_array: &JsValue, _index: u32, _value: i32, _timeout_ms: f64) -> i32 {
        0
    }

    pub fn atomic_compare_exchange(
        val: &JsValue,
        index: u32,
        expected: i32,
        replacement: i32,
    ) -> i32 {
        ensure_buffer_initialized();
        let mut buffers = BUFFERS.lock().unwrap();
        let idx = get_buffer_index(val);
        if idx >= buffers.len() {
            return 0;
        }
        let buf = &mut buffers[idx];
        let byte_idx = (index * 4) as usize;
        if byte_idx + 4 > buf.len() {
            return 0;
        }

        let old = i32::from_le_bytes(buf[byte_idx..byte_idx + 4].try_into().unwrap());
        if old == expected {
            buf[byte_idx..byte_idx + 4].copy_from_slice(&replacement.to_le_bytes());
        }
        old
    }

    pub fn math_random() -> f64 {
        0.5
    }

    pub fn copy_to_sab(val: &JsValue, target_offset: u32, src: &[u8]) {
        ensure_buffer_initialized();
        let mut buffers = BUFFERS.lock().unwrap();
        let _idx = get_buffer_index(val);
        let buf = &mut buffers[0];
        let offset = target_offset as usize;
        if offset + src.len() <= buf.len() {
            buf[offset..offset + src.len()].copy_from_slice(src);
        }
    }

    pub fn copy_from_sab(val: &JsValue, src_offset: u32, dest: &mut [u8]) {
        ensure_buffer_initialized();
        let buffers = BUFFERS.lock().unwrap();
        let _idx = get_buffer_index(val);
        let buf = &buffers[0];
        let offset = src_offset as usize;
        if offset + dest.len() <= buf.len() {
            dest.copy_from_slice(&buf[offset..offset + dest.len()]);
        }
    }
}

pub fn create_string(s: &str) -> JsValue {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        create_string_raw(s.as_ptr(), s.len() as u32)
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::create_string(s)
}

pub fn create_u8_array(data: &[u8]) -> Uint8Array {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        let val = create_u8_array_raw(data.as_ptr(), data.len() as u32);
        val.into()
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::create_u8_array(data)
}

pub fn wrap_u8_array(val: &JsValue) -> Uint8Array {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        let val_owned = wrap_u8_array_raw(val.clone());
        val_owned.into()
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::wrap_u8_array(val)
}

pub fn create_u8_view(buffer: &JsValue, offset: u32, len: u32) -> Uint8Array {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        let val = create_u8_view_raw(buffer.clone(), offset, len);
        val.into()
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::create_u8_view(buffer, offset, len)
}

pub fn create_i32_view(buffer: &JsValue, offset: u32, len: u32) -> Int32Array {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        let val = create_i32_view_raw(buffer.clone(), offset, len);
        val.into()
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::create_i32_view(buffer, offset, len)
}

pub fn console_log(msg: &str, level: u8) {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        log_raw(msg.as_ptr(), msg.len() as u32, level);
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::log(msg, level)
}

pub fn console_error(msg: &str) {
    console_log(msg, 0);
}

pub fn get_global() -> JsValue {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        get_global_raw()
    }
    #[cfg(not(target_arch = "wasm32"))]
    JsValue::UNDEFINED
}

pub fn reflect_get(_target: &JsValue, _key: &JsValue) -> Result<JsValue, JsValue> {
    #[cfg(target_arch = "wasm32")]
    return Ok(unsafe { reflect_get_raw(_target.clone(), _key.clone()) });
    #[cfg(not(target_arch = "wasm32"))]
    Ok(JsValue::UNDEFINED)
}

pub fn as_f64(_val: &JsValue) -> Option<f64> {
    #[cfg(target_arch = "wasm32")]
    {
        let res = unsafe { as_f64_raw(_val.clone()) };
        if res.is_nan() {
            if _val.is_undefined() || _val.is_null() {
                None
            } else {
                Some(res)
            }
        } else {
            Some(res)
        }
    }
    #[cfg(not(target_arch = "wasm32"))]
    None
}

pub fn get_now() -> u64 {
    #[cfg(target_arch = "wasm32")]
    return unsafe { get_now_raw() as u64 };
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::get_now()
}

pub fn get_performance_now() -> f64 {
    #[cfg(target_arch = "wasm32")]
    return unsafe { get_performance_now_raw() };
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::get_performance_now()
}

pub fn atomic_add(typed_array: &JsValue, index: u32, value: i32) -> i32 {
    #[cfg(target_arch = "wasm32")]
    return unsafe { atomic_add_raw(typed_array.clone(), index, value) };
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::atomic_add(typed_array, index, value)
}

pub fn atomic_load(typed_array: &JsValue, index: u32) -> i32 {
    #[cfg(target_arch = "wasm32")]
    return unsafe { atomic_load_raw(typed_array.clone(), index) };
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::atomic_load(typed_array, index)
}

pub fn atomic_store(typed_array: &JsValue, index: u32, value: i32) -> i32 {
    #[cfg(target_arch = "wasm32")]
    return unsafe { atomic_store_raw(typed_array.clone(), index, value) };
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::atomic_store(typed_array, index, value)
}

pub fn atomic_wait(typed_array: &JsValue, index: u32, value: i32, timeout_ms: f64) -> i32 {
    #[cfg(target_arch = "wasm32")]
    return unsafe { atomic_wait_raw(typed_array.clone(), index, value, timeout_ms) };
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::atomic_wait(typed_array, index, value, timeout_ms)
}

pub fn atomic_compare_exchange(
    typed_array: &JsValue,
    index: u32,
    expected: i32,
    replacement: i32,
) -> i32 {
    #[cfg(target_arch = "wasm32")]
    return unsafe {
        atomic_compare_exchange_raw(typed_array.clone(), index, expected, replacement)
    };
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::atomic_compare_exchange(typed_array, index, expected, replacement)
}

pub fn math_random() -> f64 {
    #[cfg(target_arch = "wasm32")]
    return unsafe { math_random_raw() };
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::math_random()
}

pub fn copy_to_sab(target_buffer: &JsValue, target_offset: u32, data: &[u8]) {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        copy_to_sab_raw(
            target_buffer.clone(),
            target_offset,
            data.as_ptr(),
            data.len() as u32,
        )
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::copy_to_sab(target_buffer, target_offset, data)
}

pub fn copy_from_sab(src_buffer: &JsValue, src_offset: u32, dest: &mut [u8]) {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        copy_from_sab_raw(
            src_buffer.clone(),
            src_offset,
            dest.as_mut_ptr(),
            dest.len() as u32,
        )
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::copy_from_sab(src_buffer, src_offset, dest)
}

pub fn get_byte_length(_val: &JsValue) -> u32 {
    #[cfg(target_arch = "wasm32")]
    return unsafe { get_byte_length_raw(_val.clone()) };
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::get_byte_length(_val)
}

pub fn js_to_string(_val: &JsValue) -> Option<String> {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        let mut buf = [0u8; 128]; // Context IDs are short
        let len = js_to_string_raw(_val.clone(), buf.as_mut_ptr(), 128);
        if len == 0 {
            return None;
        }
        Some(String::from_utf8_lossy(&buf[..len as usize]).to_string())
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::js_to_string(_val)
}
