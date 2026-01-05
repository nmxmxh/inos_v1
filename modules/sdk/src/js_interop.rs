#[cfg(target_arch = "wasm32")]
mod wasm_send_safe {
    use js_sys::{Int32Array as RawInt32Array, Uint8Array as RawUint8Array};
    use std::ops::{Deref, DerefMut};
    use web_sys::wasm_bindgen::JsValue as RawJsValue;

    /// A transparent wrapper that implements Send and Sync for WASM interop types.
    /// This is safe in our architecture as long as we don't access these objects
    /// from multiple threads simultaneously without external synchronization.
    #[repr(transparent)]
    #[derive(Clone, Debug, PartialEq)]
    pub struct SendWrap<T>(pub T);

    unsafe impl<T> Send for SendWrap<T> {}
    unsafe impl<T> Sync for SendWrap<T> {}

    impl<T> Deref for SendWrap<T> {
        type Target = T;
        fn deref(&self) -> &Self::Target {
            &self.0
        }
    }

    impl<T> DerefMut for SendWrap<T> {
        fn deref_mut(&mut self) -> &mut Self::Target {
            &mut self.0
        }
    }

    impl<T> From<T> for SendWrap<T> {
        fn from(val: T) -> Self {
            SendWrap(val)
        }
    }

    pub type JsValue = SendWrap<RawJsValue>;
    pub type Uint8Array = SendWrap<RawUint8Array>;
    pub type Int32Array = SendWrap<RawInt32Array>;

    // Identity conversions for JsValue, Uint8Array, and Int32Array are handled by the blanket
    // From<T> for SendWrap<T> implementation in line 30.

    impl From<JsValue> for RawJsValue {
        fn from(val: JsValue) -> Self {
            val.0
        }
    }

    // Specialized "downcasting" conversions for bridge return types.
    // Since our bridge functions return RawJsValue, we need these to cast into our wrapped aliases.
    impl From<RawJsValue> for Uint8Array {
        fn from(val: RawJsValue) -> Self {
            use web_sys::wasm_bindgen::JsCast;
            SendWrap(val.unchecked_into())
        }
    }

    impl From<RawJsValue> for Int32Array {
        fn from(val: RawJsValue) -> Self {
            use web_sys::wasm_bindgen::JsCast;
            SendWrap(val.unchecked_into())
        }
    }

    // Cross-type conversions (Raw -> Wrapped)
    impl From<RawUint8Array> for JsValue {
        fn from(val: RawUint8Array) -> Self {
            SendWrap(val.into())
        }
    }

    impl From<RawInt32Array> for JsValue {
        fn from(val: RawInt32Array) -> Self {
            SendWrap(val.into())
        }
    }

    // Wrapped Cross-type conversions (Wrapped -> Wrapped)
    impl From<Uint8Array> for JsValue {
        fn from(val: Uint8Array) -> Self {
            SendWrap(val.0.clone().into())
        }
    }

    impl From<Int32Array> for JsValue {
        fn from(val: Int32Array) -> Self {
            SendWrap(val.0.clone().into())
        }
    }

    impl From<js_sys::SharedArrayBuffer> for JsValue {
        fn from(val: js_sys::SharedArrayBuffer) -> Self {
            SendWrap(val.into())
        }
    }

    impl From<js_sys::JsString> for JsValue {
        fn from(val: js_sys::JsString) -> Self {
            SendWrap(val.into())
        }
    }

    impl From<js_sys::Object> for JsValue {
        fn from(val: js_sys::Object) -> Self {
            SendWrap(val.into())
        }
    }

    impl From<js_sys::Promise> for JsValue {
        fn from(val: js_sys::Promise) -> Self {
            SendWrap(val.into())
        }
    }
}

#[cfg(target_arch = "wasm32")]
pub use wasm_send_safe::{Int32Array, JsValue, SendWrap, Uint8Array};

#[cfg(not(target_arch = "wasm32"))]
mod native_types {
    #[derive(Debug, Clone, PartialEq, Default)]
    pub struct JsValue(pub u32);

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

    impl From<&str> for JsValue {
        fn from(_s: &str) -> Self {
            JsValue::UNDEFINED
        }
    }

    impl From<String> for JsValue {
        fn from(_s: String) -> Self {
            JsValue::UNDEFINED
        }
    }

    pub type Uint8Array = JsValue;
    pub type Int32Array = JsValue;
    pub type SendWrap<T> = T;
}

#[cfg(not(target_arch = "wasm32"))]
pub use native_types::{Int32Array, JsValue, SendWrap, Uint8Array};

#[cfg(target_arch = "wasm32")]
use web_sys::wasm_bindgen::JsValue as RawJsValue;

#[cfg(not(target_arch = "wasm32"))]
type RawJsValue = native_types::JsValue;

#[cfg(target_arch = "wasm32")]
#[link(wasm_import_module = "env")]
#[allow(improper_ctypes)]
extern "C" {
    // Stable name: inos_create_u8_array
    #[link_name = "inos_create_u8_array"]
    fn create_u8_array_raw(ptr: *const u8, len: u32) -> RawJsValue;

    // Stable name: inos_wrap_u8_array
    #[link_name = "inos_wrap_u8_array"]
    fn wrap_u8_array_raw(val: RawJsValue) -> RawJsValue;

    // Stable name: inos_create_u8_view
    #[link_name = "inos_create_u8_view"]
    fn create_u8_view_raw(buffer: RawJsValue, offset: u32, len: u32) -> RawJsValue;

    // Stable name: inos_create_i32_view
    #[link_name = "inos_create_i32_view"]
    fn create_i32_view_raw(buffer: RawJsValue, offset: u32, len: u32) -> RawJsValue;

    // Stable name: inos_get_global
    #[link_name = "inos_get_global"]
    fn get_global_raw() -> RawJsValue;

    // Stable name: inos_reflect_get
    #[link_name = "inos_reflect_get"]
    fn reflect_get_raw(target: RawJsValue, key: RawJsValue) -> RawJsValue;

    // Stable name: inos_as_f64
    #[link_name = "inos_as_f64"]
    fn as_f64_raw(val: RawJsValue) -> f64;

    // Stable name: inos_log
    #[link_name = "inos_log"]
    fn log_raw(ptr: *const u8, len: u32, level: u8);

    // Stable name: inos_create_string
    #[link_name = "inos_create_string"]
    fn create_string_raw(ptr: *const u8, len: u32) -> RawJsValue;

    // Stable name: inos_get_now
    #[link_name = "inos_get_now"]
    fn get_now_raw() -> f64;

    // Stable name: inos_get_performance_now (high-resolution timer)
    #[link_name = "inos_get_performance_now"]
    fn get_performance_now_raw() -> f64;

    // Stable name: inos_atomic_add
    #[link_name = "inos_atomic_add"]
    fn atomic_add_raw(typed_array: RawJsValue, index: u32, value: i32) -> i32;

    // Stable name: inos_atomic_load
    #[link_name = "inos_atomic_load"]
    fn atomic_load_raw(typed_array: RawJsValue, index: u32) -> i32;

    // Stable name: inos_atomic_store
    #[link_name = "inos_atomic_store"]
    fn atomic_store_raw(typed_array: RawJsValue, index: u32, value: i32) -> i32;

    // Stable name: inos_atomic_wait
    #[link_name = "inos_atomic_wait"]
    fn atomic_wait_raw(typed_array: RawJsValue, index: u32, value: i32, timeout: f64) -> i32;

    // Stable name: inos_atomic_compare_exchange
    #[link_name = "inos_atomic_compare_exchange"]
    fn atomic_compare_exchange_raw(
        typed_array: RawJsValue,
        index: u32,
        expected: i32,
        replacement: i32,
    ) -> i32;

    // Stable name: inos_math_random
    #[link_name = "inos_math_random"]
    fn math_random_raw() -> f64;

    // Stable name: inos_copy_to_sab
    #[link_name = "inos_copy_to_sab"]
    fn copy_to_sab_raw(target_buffer: RawJsValue, target_offset: u32, src_ptr: *const u8, len: u32);

    // Stable name: inos_copy_from_sab
    #[link_name = "inos_copy_from_sab"]
    fn copy_from_sab_raw(src_buffer: RawJsValue, src_offset: u32, dest_ptr: *mut u8, len: u32);

    // Stable name: inos_get_byte_length
    #[link_name = "inos_get_byte_length"]
    fn get_byte_length_raw(val: RawJsValue) -> u32;

    // Stable name: inos_js_to_string
    #[link_name = "inos_js_to_string"]
    fn js_to_string_raw(val: RawJsValue, ptr: *mut u8, max_len: u32) -> u32;
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

    fn get_buffer_index_raw(_val: &RawJsValue) -> usize {
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

    pub fn atomic_add(_val: &JsValue, index: u32, value: i32) -> i32 {
        ensure_buffer_initialized();
        let mut buffers = BUFFERS.lock().unwrap();
        let buf = &mut buffers[0];
        let byte_idx = (index * 4) as usize;
        if byte_idx + 4 > buf.len() {
            return 0;
        }

        let old = i32::from_le_bytes(buf[byte_idx..byte_idx + 4].try_into().unwrap());
        let new = old + value;
        buf[byte_idx..byte_idx + 4].copy_from_slice(&new.to_le_bytes());
        old
    }

    pub fn atomic_load(_val: &JsValue, index: u32) -> i32 {
        ensure_buffer_initialized();
        let buffers = BUFFERS.lock().unwrap();
        let buf = &buffers[0];
        let byte_idx = (index * 4) as usize;
        if byte_idx + 4 > buf.len() {
            return 0;
        }
        i32::from_le_bytes(buf[byte_idx..byte_idx + 4].try_into().unwrap())
    }

    pub fn atomic_store(_val: &JsValue, index: u32, value: i32) -> i32 {
        ensure_buffer_initialized();
        let mut buffers = BUFFERS.lock().unwrap();
        let buf = &mut buffers[0];
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
        _val: &JsValue,
        index: u32,
        expected: i32,
        replacement: i32,
    ) -> i32 {
        ensure_buffer_initialized();
        let mut buffers = BUFFERS.lock().unwrap();
        let buf = &mut buffers[0];
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

    pub fn copy_to_sab(_val: &JsValue, target_offset: u32, src: &[u8]) {
        ensure_buffer_initialized();
        let mut buffers = BUFFERS.lock().unwrap();
        let buf = &mut buffers[0];
        let offset = target_offset as usize;
        if offset + src.len() <= buf.len() {
            buf[offset..offset + src.len()].copy_from_slice(src);
        }
    }

    pub fn copy_from_sab(_val: &JsValue, src_offset: u32, dest: &mut [u8]) {
        ensure_buffer_initialized();
        let buffers = BUFFERS.lock().unwrap();
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
        create_string_raw(s.as_ptr(), s.len() as u32).into()
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::create_string(s)
}

pub fn create_u8_array(data: &[u8]) -> Uint8Array {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        create_u8_array_raw(data.as_ptr(), data.len() as u32).into()
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::create_u8_array(data)
}

pub fn wrap_u8_array(val: &JsValue) -> Uint8Array {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        wrap_u8_array_raw(val.0.clone()).into()
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::wrap_u8_array(val)
}

pub fn create_u8_view(buffer: &JsValue, offset: u32, len: u32) -> Uint8Array {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        create_u8_view_raw(buffer.0.clone(), offset, len).into()
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::create_u8_view(buffer, offset, len)
}

pub fn create_i32_view(buffer: &JsValue, offset: u32, len: u32) -> Int32Array {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        create_i32_view_raw(buffer.0.clone(), offset, len).into()
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
        get_global_raw().into()
    }
    #[cfg(not(target_arch = "wasm32"))]
    JsValue::UNDEFINED
}

pub fn reflect_get(_target: &JsValue, _key: &JsValue) -> Result<JsValue, JsValue> {
    #[cfg(target_arch = "wasm32")]
    return Ok(unsafe { reflect_get_raw(_target.0.clone(), _key.0.clone()).into() });
    #[cfg(not(target_arch = "wasm32"))]
    Ok(JsValue::UNDEFINED)
}

pub fn as_f64(_val: &JsValue) -> Option<f64> {
    #[cfg(target_arch = "wasm32")]
    {
        let res = unsafe { as_f64_raw(_val.0.clone()) };
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

#[cfg(target_arch = "wasm32")]
pub fn atomic_add<T>(typed_array: &SendWrap<T>, index: u32, value: i32) -> i32
where
    T: Clone + Into<web_sys::wasm_bindgen::JsValue>,
{
    unsafe { atomic_add_raw(typed_array.clone().0.into(), index, value) }
}

#[cfg(not(target_arch = "wasm32"))]
pub fn atomic_add(typed_array: &JsValue, index: u32, value: i32) -> i32 {
    native_mock::atomic_add(typed_array, index, value)
}

#[cfg(target_arch = "wasm32")]
pub fn atomic_load<T>(typed_array: &SendWrap<T>, index: u32) -> i32
where
    T: Clone + Into<web_sys::wasm_bindgen::JsValue>,
{
    unsafe { atomic_load_raw(typed_array.clone().0.into(), index) }
}

#[cfg(not(target_arch = "wasm32"))]
pub fn atomic_load(typed_array: &JsValue, index: u32) -> i32 {
    native_mock::atomic_load(typed_array, index)
}

#[cfg(target_arch = "wasm32")]
pub fn atomic_store<T>(typed_array: &SendWrap<T>, index: u32, value: i32) -> i32
where
    T: Clone + Into<web_sys::wasm_bindgen::JsValue>,
{
    unsafe { atomic_store_raw(typed_array.clone().0.into(), index, value) }
}

#[cfg(not(target_arch = "wasm32"))]
pub fn atomic_store(typed_array: &JsValue, index: u32, value: i32) -> i32 {
    native_mock::atomic_store(typed_array, index, value)
}

#[cfg(target_arch = "wasm32")]
pub fn atomic_wait<T>(typed_array: &SendWrap<T>, index: u32, value: i32, timeout_ms: f64) -> i32
where
    T: Clone + Into<web_sys::wasm_bindgen::JsValue>,
{
    unsafe { atomic_wait_raw(typed_array.0.clone().into(), index, value, timeout_ms) }
}

#[cfg(not(target_arch = "wasm32"))]
pub fn atomic_wait(typed_array: &JsValue, index: u32, value: i32, timeout_ms: f64) -> i32 {
    native_mock::atomic_wait(typed_array, index, value, timeout_ms)
}

#[cfg(target_arch = "wasm32")]
pub fn atomic_compare_exchange<T>(
    typed_array: &SendWrap<T>,
    index: u32,
    expected: i32,
    replacement: i32,
) -> i32
where
    T: Clone + Into<web_sys::wasm_bindgen::JsValue>,
{
    unsafe {
        atomic_compare_exchange_raw(typed_array.0.clone().into(), index, expected, replacement)
    }
}

#[cfg(not(target_arch = "wasm32"))]
pub fn atomic_compare_exchange(
    typed_array: &JsValue,
    index: u32,
    expected: i32,
    replacement: i32,
) -> i32 {
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
            target_buffer.0.clone(),
            target_offset,
            data.as_ptr(),
            data.len() as u32,
        );
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::copy_to_sab(target_buffer, target_offset, data);
}

pub fn copy_from_sab(src_buffer: &JsValue, src_offset: u32, dest: &mut [u8]) {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        copy_from_sab_raw(
            src_buffer.0.clone(),
            src_offset,
            dest.as_mut_ptr(),
            dest.len() as u32,
        );
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::copy_from_sab(src_buffer, src_offset, dest);
}

pub fn get_byte_length(_val: &JsValue) -> u32 {
    #[cfg(target_arch = "wasm32")]
    return unsafe { get_byte_length_raw(_val.0.clone()) };
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::get_byte_length(_val)
}

pub fn js_to_string(_val: &JsValue) -> Option<String> {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        let mut buf = [0u8; 128]; // Context IDs are short
        let len = js_to_string_raw(_val.0.clone(), buf.as_mut_ptr(), 128);
        if len == 0 {
            return None;
        }
        Some(String::from_utf8_lossy(&buf[..len as usize]).to_string())
    }
    #[cfg(not(target_arch = "wasm32"))]
    native_mock::js_to_string(_val)
}
