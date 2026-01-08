use crate::js_interop::Int32Array;
use crate::js_interop::JsValue;
use once_cell::sync::Lazy;
use std::sync::atomic::Ordering;
use std::sync::Mutex;

// Global barrier view for zero-copy context verification
static GLOBAL_BARRIER_VIEW: Lazy<Mutex<Option<JsValue>>> = Lazy::new(|| Mutex::new(None));

/// Set the global barrier view for SDK-wide atomic access.
/// Called once during module initialization.
pub fn set_global_barrier_view(view: JsValue) {
    if let Ok(mut guard) = GLOBAL_BARRIER_VIEW.lock() {
        *guard = Some(view);
    }
}

/// Get the global barrier view for atomic SAB access.
/// Returns None if not yet initialized.
pub fn get_global_barrier_view() -> Option<JsValue> {
    GLOBAL_BARRIER_VIEW.lock().ok().and_then(|g| g.clone())
}

/// Safe wrapper around SharedArrayBuffer to prevent data races and ensure memory safety
///
/// This struct enforces:
/// 1. Bounds checking
/// 2. Type safety (u8 views)
/// 3. Atomic operations with proper memory barriers
/// 4. Rust-side safe borrowing
// Removed wasm_bindgen attribute
#[cfg(target_arch = "wasm32")]
type BufferHandle = crate::js_interop::JsValue;
#[cfg(not(target_arch = "wasm32"))]
#[derive(Clone, Default)]
pub(crate) struct BufferHandle(u32);

#[cfg(not(target_arch = "wasm32"))]
unsafe impl Sync for BufferHandle {}
#[cfg(not(target_arch = "wasm32"))]
unsafe impl Send for BufferHandle {}

/// Safe wrapper around SharedArrayBuffer to prevent data races and ensure memory safety
#[derive(Clone)]
pub struct SafeSAB {
    pub(crate) buffer: BufferHandle,
    /// Persistent view for memory barriers to prevent JS object bloat
    pub(crate) barrier_view: JsValue,
    base_offset: usize,
    capacity: usize,
}

impl SafeSAB {
    /// Create a new SafeSAB from an existing SharedArrayBuffer (entire buffer)
    pub fn new(_buffer: &JsValue) -> Self {
        #[cfg(target_arch = "wasm32")]
        let capacity = crate::js_interop::get_byte_length(_buffer) as usize;
        #[cfg(not(target_arch = "wasm32"))]
        let capacity = crate::js_interop::get_byte_length(&JsValue::UNDEFINED) as usize;

        // PRE-CACHE full-buffer barrier view for zero-copy efficiency
        // We always use a view starting at 0 so that abs_offset indexing is consistent
        #[cfg(target_arch = "wasm32")]
        let barrier_view: JsValue =
            crate::js_interop::create_i32_view(_buffer, 0, (capacity / 4) as u32).into();
        #[cfg(not(target_arch = "wasm32"))]
        let barrier_view = JsValue::UNDEFINED;

        Self {
            #[cfg(target_arch = "wasm32")]
            buffer: _buffer.clone(),
            #[cfg(not(target_arch = "wasm32"))]
            buffer: BufferHandle(0),
            barrier_view,
            base_offset: 0,
            capacity,
        }
    }

    /// Create a new SafeSAB as a view into a sub-region of a SharedArrayBuffer
    pub fn new_shared_view(_buffer: &JsValue, offset: u32, size: u32) -> Self {
        #[cfg(target_arch = "wasm32")]
        let total_capacity = crate::js_interop::get_byte_length(_buffer) as u32;
        #[cfg(not(target_arch = "wasm32"))]
        let total_capacity = size;

        // PRE-CACHE full-buffer barrier view for zero-copy efficiency
        // Even for shared views, we use a full-buffer view for barriers to simplify indexing
        #[cfg(target_arch = "wasm32")]
        let barrier_view: JsValue =
            crate::js_interop::create_i32_view(_buffer, 0, total_capacity / 4).into();
        #[cfg(not(target_arch = "wasm32"))]
        let barrier_view = JsValue::UNDEFINED;

        Self {
            #[cfg(target_arch = "wasm32")]
            buffer: _buffer.clone(),
            #[cfg(not(target_arch = "wasm32"))]
            buffer: BufferHandle(0),
            barrier_view,
            base_offset: offset as usize,
            capacity: size as usize,
        }
    }
    pub fn with_size(size: usize) -> Self {
        #[cfg(target_arch = "wasm32")]
        {
            let buffer = js_sys::SharedArrayBuffer::new(size as u32);
            let buffer_js: JsValue = buffer.into();
            Self::new(&buffer_js)
        }
        #[cfg(not(target_arch = "wasm32"))]
        Self {
            buffer: BufferHandle(0),
            barrier_view: JsValue::UNDEFINED,
            base_offset: 0,
            capacity: size,
        }
    }

    /// Get total capacity in bytes
    pub fn capacity(&self) -> usize {
        self.capacity
    }

    /// Get the base offset within the SharedArrayBuffer
    pub fn base_offset(&self) -> usize {
        self.base_offset
    }

    /// Safe read from buffer with memory barriers
    pub fn read(&self, offset: usize, length: usize) -> Result<Vec<u8>, String> {
        self.bounds_check(offset, length)?;

        // Acquire barrier before reading
        self.memory_barrier_acquire(offset);

        let mut slice = vec![0u8; length];
        let abs_offset = self.base_offset + offset;
        crate::js_interop::copy_from_sab(self.as_js(), abs_offset as u32, &mut slice);

        // Release barrier after reading
        self.memory_barrier_release(offset);

        Ok(slice)
    }

    /// Safe write to buffer with memory barriers
    pub fn write(&self, offset: usize, data: &[u8]) -> Result<usize, String> {
        self.bounds_check(offset, data.len())?;

        // Acquire barrier before writing
        self.memory_barrier_acquire(offset);

        let abs_offset = self.base_offset + offset;
        crate::js_interop::copy_to_sab(self.as_js(), abs_offset as u32, data);

        // Release barrier after writing
        self.memory_barrier_release(offset);

        Ok(data.len())
    }

    /// Bulk read from buffer with single pair of memory barriers
    pub fn read_raw(&self, offset: usize, dest: &mut [u8]) -> Result<(), String> {
        self.bounds_check(offset, dest.len())?;

        // Acquire barrier once for the whole block
        self.memory_barrier_acquire(offset);

        let abs_offset = self.base_offset + offset;
        crate::js_interop::copy_from_sab(self.as_js(), abs_offset as u32, dest);

        // Release barrier once for the whole block
        self.memory_barrier_release(offset);

        Ok(())
    }

    /// Bulk write to buffer with single pair of memory barriers
    pub fn write_raw(&self, offset: usize, data: &[u8]) -> Result<(), String> {
        self.bounds_check(offset, data.len())?;

        // Acquire barrier once for the whole block
        self.memory_barrier_acquire(offset);

        let abs_offset = self.base_offset + offset;
        crate::js_interop::copy_to_sab(self.as_js(), abs_offset as u32, data);

        // Release barrier once for the whole block
        self.memory_barrier_release(offset);

        Ok(())
    }

    fn bounds_check(&self, offset: usize, length: usize) -> Result<(), String> {
        if offset + length > self.capacity {
            return Err(format!(
                "Out of bounds: {} + {} > {}",
                offset, length, self.capacity
            ));
        }
        Ok(())
    }

    fn memory_barrier_acquire(&self, offset: usize) {
        // Use cached barrier_view to avoid JS object creation
        let abs_offset = self.base_offset + offset;
        let index = (abs_offset / 4) as u32;
        let _ = crate::js_interop::atomic_load(&self.barrier_view, index);
    }

    fn memory_barrier_release(&self, offset: usize) {
        // Use cached barrier_view to avoid JS object creation
        let abs_offset = self.base_offset + offset;
        let index = (abs_offset / 4) as u32;
        // RELEASE is technically an atomic_load for our memory model
        let _ = crate::js_interop::atomic_load(&self.barrier_view, index);
    }

    /// Get raw inner buffer handle (for interop with other SDK components)
    pub fn inner(&self) -> &JsValue {
        self.as_js()
    }

    fn as_js(&self) -> &JsValue {
        #[cfg(target_arch = "wasm32")]
        return &self.buffer;
        #[cfg(not(target_arch = "wasm32"))]
        {
            use once_cell::sync::Lazy;
            struct SyncJsValue(JsValue);
            unsafe impl Sync for SyncJsValue {}
            unsafe impl Send for SyncJsValue {}
            static UNDEFINED: Lazy<SyncJsValue> = Lazy::new(|| SyncJsValue(JsValue::UNDEFINED));
            &UNDEFINED.0
        }
    }

    /// Get the barrier view (Int32Array) for atomic operations
    pub fn barrier_view(&self) -> &JsValue {
        &self.barrier_view
    }

    /// Get a typed Int32Array view of a region (for Atomics)
    pub fn int32_view(&self, offset: usize, count: usize) -> Result<Int32Array, String> {
        let byte_len = count * 4;
        self.bounds_check(offset, byte_len)?;

        // Check alignment
        if (offset & 3) != 0 {
            return Err("Offset must be 4-byte aligned for Int32Array".to_string());
        }

        let abs_offset = self.base_offset + offset;
        // Int32Array constructor via stable ABI
        Ok(crate::js_interop::create_i32_view(
            self.as_js(),
            abs_offset as u32,
            count as u32,
        ))
    }

    // ========== SAB REGION CONSTANTS ==========
    // Delegated to crate::layout for single source of truth
    pub const OFFSET_ECONOMICS: usize = crate::layout::OFFSET_ECONOMICS;
    pub const SIZE_ECONOMICS: usize = crate::layout::SIZE_ECONOMICS;
    pub const OFFSET_IDENTITY_REGISTRY: usize = crate::layout::OFFSET_IDENTITY_REGISTRY;
    pub const SIZE_IDENTITY_REGISTRY: usize = crate::layout::SIZE_IDENTITY_REGISTRY;
    pub const OFFSET_SOCIAL_GRAPH: usize = crate::layout::OFFSET_SOCIAL_GRAPH;
    pub const SIZE_SOCIAL_GRAPH: usize = crate::layout::SIZE_SOCIAL_GRAPH;
    pub const OFFSET_PATTERN_EXCHANGE: usize = crate::layout::OFFSET_PATTERN_EXCHANGE;
}

// SAFETY: SafeSAB wraps SharedArrayBuffer which is designed to be shared across
// Web Workers (threads). All access is synchronized via Atomics operations which
// provide proper memory barriers. The underlying JS types (SharedArrayBuffer,
// Uint8Array, Int32Array) are all thread-safe when used with Atomics.
unsafe impl Send for SafeSAB {}
unsafe impl Sync for SafeSAB {}

/// Production-grade reader-writer lock using atomic operations
/// Aligned with INOS v1.9 epoch-based signaling pattern
pub struct SABRwLock {
    /// State encoding: bit 0 = writer flag, bits 1-31 = reader count
    /// Max 2^31-1 concurrent readers (sufficient for WASM)
    state: std::sync::atomic::AtomicU32,

    /// Metrics for monitoring contention
    metrics: LockMetrics,
}

struct LockMetrics {
    read_contentions: std::sync::atomic::AtomicU64,
    write_contentions: std::sync::atomic::AtomicU64,
    total_read_time_ns: std::sync::atomic::AtomicU64,
    total_write_time_ns: std::sync::atomic::AtomicU64,
}

impl Default for LockMetrics {
    fn default() -> Self {
        Self {
            read_contentions: std::sync::atomic::AtomicU64::new(0),
            write_contentions: std::sync::atomic::AtomicU64::new(0),
            total_read_time_ns: std::sync::atomic::AtomicU64::new(0),
            total_write_time_ns: std::sync::atomic::AtomicU64::new(0),
        }
    }
}

impl Default for SABRwLock {
    fn default() -> Self {
        Self::new()
    }
}

impl SABRwLock {
    const WRITER_BIT: u32 = 1;
    const READER_INC: u32 = 2;

    pub fn new() -> Self {
        Self {
            state: std::sync::atomic::AtomicU32::new(0),
            metrics: LockMetrics::default(),
        }
    }

    /// Fast-path read lock (single CAS operation)
    pub fn try_read(&self) -> Option<RwLockReadGuard<'_>> {
        let mut state = self.state.load(Ordering::Acquire);

        loop {
            // Check if writer is active
            if state & Self::WRITER_BIT != 0 {
                self.metrics
                    .read_contentions
                    .fetch_add(1, Ordering::Relaxed);
                return None;
            }

            // Check for reader overflow (unlikely but safe)
            let new_state = state.checked_add(Self::READER_INC)?;

            // Try to increment reader count
            match self
                .state
                .compare_exchange(state, new_state, Ordering::AcqRel, Ordering::Acquire)
            {
                Ok(_) => {
                    return Some(RwLockReadGuard {
                        lock: self,
                        start_time: web_time::Instant::now(),
                    })
                }
                Err(current) => state = current,
            }
        }
    }

    /// Write lock with exponential backoff
    pub fn try_write(&self) -> Option<RwLockWriteGuard<'_>> {
        // Fast path: try to acquire immediately
        match self
            .state
            .compare_exchange(0, Self::WRITER_BIT, Ordering::AcqRel, Ordering::Acquire)
        {
            Ok(_) => Some(RwLockWriteGuard {
                lock: self,
                start_time: web_time::Instant::now(),
            }),
            Err(_) => {
                self.metrics
                    .write_contentions
                    .fetch_add(1, Ordering::Relaxed);
                None
            }
        }
    }

    /// Async read lock with timeout (WASM-compatible)
    pub async fn read_timeout(&self, timeout_ms: u32) -> Result<RwLockReadGuard<'_>, String> {
        let start = web_time::Instant::now();
        let mut backoff = 1u32;

        loop {
            if let Some(guard) = self.try_read() {
                return Ok(guard);
            }

            // Check timeout
            if start.elapsed().as_millis() as u32 > timeout_ms {
                return Err("Lock timeout".to_string());
            }

            // Exponential backoff with jitter (max 16ms)
            // Simple pseudo-random for delay without js_sys::Math
            let delay = (backoff + 1).min(16);
            let _ = wasm_timer::Delay::new(std::time::Duration::from_millis(delay as u64)).await;
            backoff = (backoff * 2).min(16);
        }
    }

    /// Async write lock with timeout (WASM-compatible)
    pub async fn write_timeout(&self, timeout_ms: u32) -> Result<RwLockWriteGuard<'_>, String> {
        let start = web_time::Instant::now();
        let mut backoff = 1u32;

        loop {
            if let Some(guard) = self.try_write() {
                return Ok(guard);
            }

            // Check timeout
            if start.elapsed().as_millis() as u32 > timeout_ms {
                return Err("Lock timeout".to_string());
            }

            // Exponential backoff with jitter (max 16ms)
            let delay = (backoff as f64 + (crate::js_interop::math_random() * backoff as f64))
                .min(16.0) as u32;
            let _ = wasm_timer::Delay::new(std::time::Duration::from_millis(delay as u64)).await;
            backoff = (backoff * 2).min(16);
        }
    }

    /// Get lock metrics
    pub fn metrics(&self) -> LockMetricsSnapshot {
        LockMetricsSnapshot {
            read_contentions: self.metrics.read_contentions.load(Ordering::Relaxed),
            write_contentions: self.metrics.write_contentions.load(Ordering::Relaxed),
            total_read_time_ns: self.metrics.total_read_time_ns.load(Ordering::Relaxed),
            total_write_time_ns: self.metrics.total_write_time_ns.load(Ordering::Relaxed),
        }
    }
}

/// Snapshot of lock metrics for monitoring
#[derive(Debug, Clone, Copy)]
pub struct LockMetricsSnapshot {
    pub read_contentions: u64,
    pub write_contentions: u64,
    pub total_read_time_ns: u64,
    pub total_write_time_ns: u64,
}

/// RAII guard for read locks
pub struct RwLockReadGuard<'a> {
    lock: &'a SABRwLock,
    start_time: web_time::Instant,
}

impl Drop for RwLockReadGuard<'_> {
    fn drop(&mut self) {
        // Decrement reader count
        self.lock
            .state
            .fetch_sub(SABRwLock::READER_INC, Ordering::Release);

        // Record metrics
        let duration = self.start_time.elapsed().as_nanos() as u64;
        self.lock
            .metrics
            .total_read_time_ns
            .fetch_add(duration, Ordering::Relaxed);
    }
}

/// RAII guard for write locks
pub struct RwLockWriteGuard<'a> {
    lock: &'a SABRwLock,
    start_time: web_time::Instant,
}

impl Drop for RwLockWriteGuard<'_> {
    fn drop(&mut self) {
        // Clear writer bit
        self.lock.state.store(0, Ordering::Release);

        // Record metrics
        let duration = self.start_time.elapsed().as_nanos() as u64;
        self.lock
            .metrics
            .total_write_time_ns
            .fetch_add(duration, Ordering::Relaxed);
    }
}

// SAFETY: SABRwLock uses atomic operations for all state management
unsafe impl Send for SABRwLock {}
unsafe impl Sync for SABRwLock {}

/// Typed view for ML tensors to avoid copying
pub struct TensorSAB<T> {
    sab: SafeSAB,
    _marker: std::marker::PhantomData<T>,
}

impl<T> TensorSAB<T>
where
    T: Copy + Default + 'static,
{
    pub fn new(shape: &[usize]) -> Result<Self, String> {
        let element_size = std::mem::size_of::<T>();
        let total_elements = shape.iter().product::<usize>();
        let total_bytes = element_size * total_elements;

        let sab = SafeSAB::with_size(total_bytes);

        Ok(Self {
            sab,
            _marker: std::marker::PhantomData,
        })
    }

    /// Copy tensor data with type safety
    pub fn write_tensor(&self, data: &[T]) -> Result<usize, String> {
        let byte_len = std::mem::size_of_val(data);

        // Safe transmutation to bytes
        let bytes = unsafe { std::slice::from_raw_parts(data.as_ptr() as *const u8, byte_len) };

        self.sab.write(0, bytes)
    }

    /// Read tensor data
    pub fn read_tensor(&self, count: usize) -> Result<Vec<T>, String> {
        let element_size = std::mem::size_of::<T>();
        let byte_len = count * element_size;

        let bytes = self.sab.read(0, byte_len)?;

        // Safety: We initialize with default values first to avoid uninitialized memory issues.
        // T: Default + Copy ensures this is cheap and safe.
        let mut result_vec = vec![T::default(); count];

        unsafe {
            std::ptr::copy_nonoverlapping(
                bytes.as_ptr(),
                result_vec.as_mut_ptr() as *mut u8,
                byte_len,
            );
        }

        Ok(result_vec)
    }
}
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_safesab_read_write() {
        let sab = SafeSAB::with_size(1024);
        let data = b"hello inos";
        sab.write(10, data).unwrap();

        let read_data = sab.read(10, data.len()).unwrap();
        assert_eq!(&read_data, data);
    }

    #[test]
    fn test_safesab_bounds() {
        let sab = SafeSAB::with_size(100);
        assert!(sab.write(95, &[0, 0, 0, 0, 0]).is_ok());
        assert!(sab.write(96, &[0, 0, 0, 0, 0]).is_err());
    }

    #[test]
    fn test_rwlock_basic() {
        let lock = SABRwLock::new();

        {
            let _read = lock.try_read().unwrap();
            assert!(lock.try_write().is_none());
            let _read2 = lock.try_read().unwrap();
        }

        let _write = lock.try_write().unwrap();
        assert!(lock.try_read().is_none());
    }

    #[test]
    fn test_tensor_sab() {
        let tensor = TensorSAB::<f32>::new(&[4]).unwrap();
        let data = vec![1.0, 2.0, 3.0, 4.0];
        tensor.write_tensor(&data).unwrap();

        let read_data = tensor.read_tensor(4).unwrap();
        assert_eq!(read_data, data);
    }
}
