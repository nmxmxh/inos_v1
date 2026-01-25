use crate::layout;

#[cfg(target_arch = "wasm32")]
#[repr(transparent)]
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct JsValue(pub u32);

#[cfg(target_arch = "wasm32")]
impl JsValue {
    pub const UNDEFINED: JsValue = JsValue(0);
    pub const NULL: JsValue = JsValue(1);

    pub fn is_null(&self) -> bool {
        self.0 == Self::NULL.0
    }

    pub fn is_undefined(&self) -> bool {
        self.0 == Self::UNDEFINED.0
    }
}

#[cfg(target_arch = "wasm32")]
pub type Uint8Array = JsValue;
#[cfg(target_arch = "wasm32")]
pub type Int32Array = JsValue;

#[cfg(not(target_arch = "wasm32"))]
#[derive(Debug, Clone, PartialEq, Default)]
pub struct JsValue(pub u32);

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
pub type Uint8Array = JsValue;
#[cfg(not(target_arch = "wasm32"))]
pub type Int32Array = JsValue;

#[cfg(target_arch = "wasm32")]
#[link(wasm_import_module = "env")]
extern "C" {
    fn inos_create_u8_array(ptr: *const u8, len: u32) -> u32;
    fn inos_wrap_u8_array(val: u32) -> u32;
    fn inos_create_u8_view(buffer: u32, offset: u32, len: u32) -> u32;
    fn inos_create_i32_view(buffer: u32, offset: u32, len: u32) -> u32;
    fn inos_create_string(ptr: *const u8, len: u32) -> u32;
    fn inos_get_global() -> u32;
    fn inos_reflect_get(target: u32, key: u32) -> u32;
    fn inos_as_f64(val: u32) -> f64;
    fn inos_log(ptr: *const u8, len: u32, level: u8);
    fn inos_get_now() -> f64;
    fn inos_get_performance_now() -> f64;
    fn inos_math_random() -> f64;
    fn inos_atomic_add(typed_array: u32, index: u32, value: i32) -> i32;
    fn inos_atomic_load(typed_array: u32, index: u32) -> i32;
    fn inos_atomic_store(typed_array: u32, index: u32, value: i32) -> i32;
    fn inos_atomic_wait(typed_array: u32, index: u32, value: i32, timeout: f64) -> i32;
    fn inos_atomic_notify(typed_array: u32, index: u32, count: i32) -> i32;
    fn inos_atomic_compare_exchange(
        typed_array: u32,
        index: u32,
        expected: i32,
        replacement: i32,
    ) -> i32;
    fn inos_copy_to_sab(target_buffer: u32, target_offset: u32, src_ptr: *const u8, len: u32);
    fn inos_copy_from_sab(src_buffer: u32, src_offset: u32, dest_ptr: *mut u8, len: u32);
    fn inos_get_byte_length(val: u32) -> u32;
    fn inos_js_to_string(val: u32, ptr: *mut u8, max_len: u32) -> u32;
    fn inos_create_sab(len: u32) -> u32;
    fn inos_fill_random(ptr: *mut u8, len: u32);
}

#[cfg(not(target_arch = "wasm32"))]
pub(crate) mod native_mock {
    use super::JsValue;
    use std::collections::HashMap;
    use std::sync::Mutex;

    use once_cell::sync::Lazy;

    static BUFFERS: Lazy<Mutex<HashMap<u32, Vec<u8>>>> = Lazy::new(|| Mutex::new(HashMap::new()));

    pub fn register_buffer(data: Vec<u8>) -> u32 {
        let mut guard = BUFFERS.lock().unwrap();
        let next = guard.len() as u32 + 2;
        guard.insert(next, data);
        next
    }

    pub fn get_byte_length(val: &JsValue) -> u32 {
        let guard = BUFFERS.lock().unwrap();
        if let Some(buf) = guard.get(&val.0) {
            return buf.len() as u32;
        }
        0
    }

    pub fn copy_to_sab(val: &JsValue, target_offset: u32, src: &[u8]) {
        let mut guard = BUFFERS.lock().unwrap();
        if let Some(buf) = guard.get_mut(&val.0) {
            let start = target_offset as usize;
            let end = start + src.len();
            if end <= buf.len() {
                buf[start..end].copy_from_slice(src);
            }
        }
    }

    pub fn copy_from_sab(val: &JsValue, src_offset: u32, dest: &mut [u8]) {
        let guard = BUFFERS.lock().unwrap();
        if let Some(buf) = guard.get(&val.0) {
            let start = src_offset as usize;
            let end = start + dest.len();
            if end <= buf.len() {
                dest.copy_from_slice(&buf[start..end]);
            }
        }
    }
}

pub fn console_log(_msg: &str, _level: u8) {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        inos_log(_msg.as_ptr(), _msg.len() as u32, _level);
    }
}

pub fn create_string(s: &str) -> JsValue {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return JsValue(inos_create_string(s.as_ptr(), s.len() as u32));
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = s;
        JsValue::UNDEFINED
    }
}

pub fn create_u8_array(data: &[u8]) -> Uint8Array {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return JsValue(inos_create_u8_array(data.as_ptr(), data.len() as u32));
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = data;
        JsValue::UNDEFINED
    }
}

pub fn wrap_u8_array(val: &JsValue) -> Uint8Array {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return JsValue(inos_wrap_u8_array(val.0));
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = val;
        JsValue::UNDEFINED
    }
}

pub fn create_u8_view(buffer: &JsValue, offset: u32, len: u32) -> Uint8Array {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return JsValue(inos_create_u8_view(buffer.0, offset, len));
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = (buffer, offset, len);
        JsValue::UNDEFINED
    }
}

pub fn create_i32_view(buffer: &JsValue, offset: u32, len: u32) -> Int32Array {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return JsValue(inos_create_i32_view(buffer.0, offset, len));
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = (buffer, offset, len);
        JsValue::UNDEFINED
    }
}

pub fn get_global() -> JsValue {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return JsValue(inos_get_global());
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        JsValue::UNDEFINED
    }
}

pub fn reflect_get(target: &JsValue, key: &JsValue) -> Result<JsValue, JsValue> {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return Ok(JsValue(inos_reflect_get(target.0, key.0)));
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = (target, key);
        Ok(JsValue::UNDEFINED)
    }
}

pub fn as_f64(val: &JsValue) -> Option<f64> {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        if val.is_null() || val.is_undefined() {
            return None;
        }
        return Some(inos_as_f64(val.0));
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = val;
        None
    }
}

pub fn js_to_string(val: &JsValue) -> Option<String> {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        let mut buf = vec![0u8; 1024];
        let len = inos_js_to_string(val.0, buf.as_mut_ptr(), buf.len() as u32) as usize;
        if len == 0 {
            return None;
        }
        buf.truncate(len);
        return String::from_utf8(buf).ok();
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = val;
        None
    }
}

pub fn get_global_string(key: &str) -> Option<String> {
    let global = get_global();
    let key_val = create_string(key);
    let value = reflect_get(&global, &key_val).ok()?;
    js_to_string(&value)
}

pub fn get_now() -> f64 {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return inos_get_now();
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        0.0
    }
}

pub fn get_performance_now() -> f64 {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return inos_get_performance_now();
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        0.0
    }
}

#[cfg(target_arch = "wasm32")]
#[no_mangle]
pub extern "Rust" fn __getrandom_v03_custom(
    dest: *mut u8,
    len: usize,
) -> Result<(), getrandom03::Error> {
    if dest.is_null() || len == 0 {
        return Ok(());
    }

    unsafe {
        let slice = core::slice::from_raw_parts_mut(dest, len);
        fill_random(slice);
    }
    Ok(())
}

pub fn ensure_getrandom() {
    #[cfg(target_arch = "wasm32")]
    {
        // getrandom 0.2 custom backend is registered at the module crate root.
    }
}

#[cfg(target_arch = "wasm32")]
pub fn getrandom_custom(dest: &mut [u8]) -> Result<(), getrandom::Error> {
    fill_random(dest);
    Ok(())
}

pub fn fill_random(_dest: &mut [u8]) {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        inos_fill_random(_dest.as_mut_ptr(), _dest.len() as u32);
    }
}

pub fn math_random() -> f64 {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return inos_math_random();
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        0.0
    }
}

pub fn atomic_add(typed_array: &JsValue, index: u32, value: i32) -> i32 {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return inos_atomic_add(typed_array.0, index, value);
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = (typed_array, index, value);
        0
    }
}

pub fn atomic_load(typed_array: &JsValue, index: u32) -> i32 {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return inos_atomic_load(typed_array.0, index);
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = (typed_array, index);
        0
    }
}

pub fn atomic_store(typed_array: &JsValue, index: u32, value: i32) -> i32 {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return inos_atomic_store(typed_array.0, index, value);
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = (typed_array, index, value);
        0
    }
}

pub fn atomic_wait(typed_array: &JsValue, index: u32, value: i32, timeout_ms: f64) -> i32 {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return inos_atomic_wait(typed_array.0, index, value, timeout_ms);
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = (typed_array, index, value, timeout_ms);
        2
    }
}

pub fn atomic_notify(typed_array: &JsValue, index: u32, count: i32) -> i32 {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return inos_atomic_notify(typed_array.0, index, count);
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = (typed_array, index, count);
        0
    }
}

pub fn atomic_compare_exchange(
    typed_array: &JsValue,
    index: u32,
    expected: i32,
    replacement: i32,
) -> i32 {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return inos_atomic_compare_exchange(typed_array.0, index, expected, replacement);
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let _ = (typed_array, index, expected, replacement);
        0
    }
}

pub fn copy_to_sab(target_buffer: &JsValue, target_offset: u32, data: &[u8]) {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        inos_copy_to_sab(
            target_buffer.0,
            target_offset,
            data.as_ptr(),
            data.len() as u32,
        );
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        native_mock::copy_to_sab(target_buffer, target_offset, data);
    }
}

pub fn copy_from_sab(src_buffer: &JsValue, src_offset: u32, dest: &mut [u8]) {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        inos_copy_from_sab(
            src_buffer.0,
            src_offset,
            dest.as_mut_ptr(),
            dest.len() as u32,
        );
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        native_mock::copy_from_sab(src_buffer, src_offset, dest);
    }
}

pub fn get_byte_length(val: &JsValue) -> u32 {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return inos_get_byte_length(val.0);
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        native_mock::get_byte_length(val)
    }
}

pub fn create_sab(len: u32) -> JsValue {
    #[cfg(target_arch = "wasm32")]
    unsafe {
        return JsValue(inos_create_sab(len));
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        let handle = native_mock::register_buffer(vec![0u8; len as usize]);
        JsValue(handle)
    }
}

pub fn console_log_buffered(msg: &str, level: u8) {
    console_log(msg, level);
}

pub fn maybe_bump_system_epoch(_typed_array: &JsValue, index: u32) {
    if index == layout::IDX_SYSTEM_EPOCH {
        let _ = atomic_add(_typed_array, index, 1);
        let _ = atomic_notify(_typed_array, index, i32::MAX);
    }
}
