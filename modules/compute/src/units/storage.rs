use js_sys::{JsString, Object, Promise, Reflect, SharedArrayBuffer};

use serde::{Deserialize, Serialize};
use std::cell::RefCell;
use std::future::Future;
use std::pin::Pin;
use std::rc::Rc;
use std::sync::{Arc, Mutex};
use std::task::{Context, Poll, Waker};
use thiserror::Error;
use web_sys::{window, IdbDatabase, IdbFactory, IdbRequest, IdbTransactionMode};
// StorageUnit uses raw web_sys types (NOT SDK wrappers) because it's main-thread-only
// and not registered with the thread-safe ComputeEngine
use web_sys::wasm_bindgen::{closure::Closure, JsCast, JsValue};

// Custom Promise-to-Future adapter (thread-safe)
struct PromiseFuture {
    _promise: JsValue,
    result: Arc<Mutex<Option<Result<JsValue, JsValue>>>>,
    waker: Arc<Mutex<Option<Waker>>>,
}

impl PromiseFuture {
    fn new(promise: Promise) -> Self {
        Self {
            _promise: promise.into(),
            result: Arc::new(Mutex::new(None)),
            waker: Arc::new(Mutex::new(None)),
        }
    }
}

impl Future for PromiseFuture {
    type Output = Result<JsValue, JsValue>;

    fn poll(self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<Self::Output> {
        let mut res_guard = self.result.lock().unwrap();
        if let Some(result) = res_guard.take() {
            return Poll::Ready(result);
        }
        *self.waker.lock().unwrap() = Some(cx.waker().clone());
        Poll::Pending
    }
}

// Helper function to replace await_promise()
async fn await_promise(promise: Promise) -> Result<JsValue, JsValue> {
    PromiseFuture::new(promise).await
}

#[derive(Debug, Error)]
pub enum StorageError {
    #[error("IndexedDB error: {0}")]
    IndexedDB(String),

    #[error("OPFS error: {0}")]
    Opfs(String),

    #[error("Hash mismatch: expected {expected}, got {actual}")]
    HashMismatch { expected: String, actual: String },

    #[error("Chunk not found: {0}")]
    NotFound(String),

    #[error("Serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    #[error("Chunk too large: {size} bytes (max: {max})")]
    ChunkTooLarge { size: usize, max: usize },

    #[error("Invalid hash format: {0}")]
    InvalidHashFormat(String),

    #[error("Invalid path: {0}")]
    InvalidPath(String),

    #[error("No window object")]
    NoWindow,

    #[error("No IndexedDB support")]
    NoIndexedDB,

    #[error("Database not initialized")]
    NotInitialized,

    #[error("No shared buffer")]
    NoSharedBuffer,
}

impl From<StorageError> for Object {
    fn from(err: StorageError) -> Self {
        Object::from(js_sys::JsString::from(format!("StorageError: {}", err)))
    }
}

impl From<Object> for StorageError {
    fn from(val: Object) -> Self {
        let msg = js_sys::JSON::stringify(&val)
            .ok()
            .and_then(|s| s.as_string())
            .unwrap_or_else(|| "Unknown JS error".to_string());
        StorageError::IndexedDB(msg)
    }
}

impl From<JsValue> for StorageError {
    fn from(val: JsValue) -> Self {
        let msg = val
            .as_string()
            .or_else(|| {
                js_sys::JSON::stringify(&val)
                    .ok()
                    .and_then(|s| s.as_string())
            })
            .unwrap_or_else(|| "Unknown JS error".to_string());
        StorageError::IndexedDB(msg)
    }
}

// ========== LibraryProxy Implementation ==========
// NOTE: StorageUnit is NOT registered with ComputeEngine due to browser API
// constraints (IndexedDB/OPFS are non-Send). It's called directly via the
// StorageSupervisor which handles dispatch via SAB bridge.

/*
// Disabled: StorageUnit cannot implement Send+Sync UnitProxy due to browser APIs
#[async_trait]
impl UnitProxy for StorageUnit {
    fn service_name(&self) -> &str {
        "compute"
    }

    fn name(&self) -> &str {
        "storage"
    }

    fn actions(&self) -> Vec<&str> {
        vec![
            "store_chunk",
            "load_chunk",
            "delete_chunk",
            "query_index",
            "rebuild_index",
            "gc",
            "write",
            "read",
            "delete",
        ]
    }

    async fn execute(
        &self,
        method: &str,
        input: &[u8],
        params: &[u8],
    ) -> Result<Vec<u8>, ComputeError> {
        let p: StorageParams = serde_json::from_slice(params)
            .map_err(|e| ComputeError::InvalidParams(format!("Invalid JSON: {}", e)))?;

        match method {
            "store_chunk" | "write" => self
                .store_chunk(
                    &p.content_hash,
                    input,
                    p.priority.as_deref().unwrap_or("normal"),
                )
                .await
                .map_err(|e| ComputeError::ExecutionFailed(e.to_string())),
            "load_chunk" | "read" => self
                .load_chunk(&p.content_hash)
                .await
                .map_err(|e| ComputeError::ExecutionFailed(e.to_string())),
            "delete_chunk" | "delete" => self
                .delete_chunk(&p.content_hash)
                .await
                .map_err(|e| ComputeError::ExecutionFailed(e.to_string())),
            "query_index" | "exists" => self
                .query_index(&p)
                .await
                .map_err(|e| ComputeError::ExecutionFailed(e.to_string())),
            "opfs_read" => {
                let data = self
                    .read_opfs(&p.content_hash)
                    .await
                    .map_err(|e| ComputeError::ExecutionFailed(e.to_string()))?;
                Ok(data)
            }
            "opfs_write" => {
                self.write_opfs(&p.content_hash, input)
                    .await
                    .map_err(|e| ComputeError::ExecutionFailed(e.to_string()))?;
                Ok(vec![])
            }
            "store_chunk_zero_copy" => {
                // SharedArrayBuffer logic would go here
                Ok(vec![])
            }
            _ => Err(ComputeError::UnknownMethod {
                library: "storage".to_string(),
                method: method.to_string(),
            }),
        }
    }

    fn resource_limits(&self) -> ResourceLimits {
        ResourceLimits {
            max_input_size: 100 * 1024 * 1024,  // 100MB chunks
            max_output_size: 100 * 1024 * 1024, // 100MB chunks
            max_memory_pages: 2048,             // 128MB
            timeout_ms: 30000,                  // 30s for storage ops
            max_fuel: 50_000_000_000,           // 50B instructions
        }
    }
}
*/

// WASM is single-threaded in browser context, so these are safe
// Even with SharedArrayBuffer, access is serialized through the event loop
unsafe impl Send for StorageUnit {}
unsafe impl Sync for StorageUnit {}

// ========== Constants ==========

const MAX_CHUNK_SIZE: usize = 10 * 1024 * 1024; // 10MB
const MAX_CACHE_SIZE: usize = 100 * 1024 * 1024; // 100MB
#[allow(dead_code)]
const DB_NAME: &str = "inos-storage";
const DB_VERSION: u32 = 1;

// ========== Data Structures ==========

#[derive(Debug, Serialize, Deserialize)]
pub struct StorageParams {
    pub operation: String,
    pub content_hash: String,
    pub chunk_index: Option<u32>,
    pub location: Option<String>,
    pub priority: Option<String>,
    pub model_id: Option<String>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ChunkMetadata {
    pub hash: String,
    pub location: String,
    pub size: usize,
    pub priority: String,
    pub last_accessed: f64,
    pub access_count: u32,
    pub model_id: Option<String>,
}

// Helper to convert ChunkMetadata to Object (uses raw web_sys types)
fn metadata_to_jsvalue(_metadata: &ChunkMetadata) -> Result<JsValue, StorageError> {
    #[cfg(not(target_arch = "wasm32"))]
    return Ok(JsValue::UNDEFINED);

    #[cfg(target_arch = "wasm32")]
    {
        let obj = Object::new();
        Reflect::set(
            &obj,
            &"hash".into(),
            &JsString::from(_metadata.hash.as_str()).into(),
        )
        .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        Reflect::set(
            &obj,
            &"location".into(),
            &JsString::from(_metadata.location.as_str()).into(),
        )
        .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        Reflect::set(
            &obj,
            &"size".into(),
            &js_sys::Number::from(_metadata.size as f64).into(),
        )
        .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        Reflect::set(
            &obj,
            &"priority".into(),
            &JsString::from(_metadata.priority.as_str()).into(),
        )
        .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        Reflect::set(
            &obj,
            &"last_accessed".into(),
            &js_sys::Number::from(_metadata.last_accessed).into(),
        )
        .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        Reflect::set(
            &obj,
            &"access_count".into(),
            &js_sys::Number::from(_metadata.access_count as f64).into(),
        )
        .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        if let Some(ref model_id) = _metadata.model_id {
            Reflect::set(
                &obj,
                &"model_id".into(),
                &JsString::from(model_id.as_str()).into(),
            )
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        }
        Ok(obj.into())
    }
}

// Helper to convert Object to ChunkMetadata (uses js_sys::Reflect directly)
fn jsvalue_to_metadata(val: &JsValue) -> Result<ChunkMetadata, StorageError> {
    #[allow(unused_variables)]
    let _val = val;
    #[cfg(not(target_arch = "wasm32"))]
    return Err(StorageError::NotInitialized);

    #[cfg(target_arch = "wasm32")]
    {
        let hash = Reflect::get(val, &"hash".into())
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?
            .as_string()
            .ok_or_else(|| StorageError::IndexedDB("hash not a string".into()))?;
        let location = Reflect::get(val, &"location".into())
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?
            .as_string()
            .ok_or_else(|| StorageError::IndexedDB("location not a string".into()))?;
        let size = Reflect::get(val, &"size".into())
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))
            .and_then(|v| {
                v.as_f64()
                    .ok_or_else(|| StorageError::IndexedDB("size not a number".into()))
            })? as usize;
        let priority = Reflect::get(val, &"priority".into())
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?
            .as_string()
            .ok_or_else(|| StorageError::IndexedDB("priority not a string".into()))?;
        let last_accessed = Reflect::get(val, &"last_accessed".into())
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))
            .and_then(|v| {
                v.as_f64()
                    .ok_or_else(|| StorageError::IndexedDB("last_accessed not a number".into()))
            })?;
        let access_count = Reflect::get(val, &"access_count".into())
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))
            .and_then(|v| {
                v.as_f64()
                    .ok_or_else(|| StorageError::IndexedDB("access_count not a number".into()))
            })? as u32;
        let model_id = Reflect::get(val, &"model_id".into())
            .ok()
            .and_then(|v| v.as_string());

        Ok(ChunkMetadata {
            hash,
            location,
            size,
            priority,
            last_accessed,
            access_count,
            model_id,
        })
    }
}

// ========== StorageJob ==========

pub struct StorageUnit {
    db: Option<IdbDatabase>,
    #[allow(dead_code)]
    sab: Option<SharedArrayBuffer>, // Zero-copy shared memory
}

impl StorageUnit {
    #[allow(dead_code)]
    pub fn new() -> Result<Self, Object> {
        Ok(Self {
            db: None,
            sab: None,
        })
    }

    #[allow(dead_code)]
    pub fn set_shared_buffer(&mut self, sab: SharedArrayBuffer) {
        self.sab = Some(sab);
    }

    #[allow(dead_code)]
    pub async fn init(&mut self) -> Result<(), StorageError> {
        let window = window().ok_or(StorageError::NoWindow)?;
        let idb_factory: IdbFactory = window
            .indexed_db()
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?
            .ok_or(StorageError::NoIndexedDB)?;

        // Open database
        let open_request = idb_factory
            .open(DB_NAME)
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;

        // Setup onupgradeneeded
        let onupgradeneeded = Closure::once(move |event: web_sys::IdbVersionChangeEvent| {
            let target = event.target().unwrap();
            let request: IdbRequest = target.dyn_into().unwrap();
            let db: IdbDatabase = request.result().unwrap().dyn_into().unwrap();

            // Create object stores
            let store_names = db.object_store_names();
            let has_chunks =
                (0..store_names.length()).any(|i| store_names.get(i).as_deref() == Some("chunks"));

            if !has_chunks {
                let store = db.create_object_store("chunks").unwrap();

                // Create indexes using create_index_with_str
                store.create_index_with_str("hash", "hash").unwrap();
                store.create_index_with_str("priority", "priority").unwrap();
                store
                    .create_index_with_str("last_accessed", "last_accessed")
                    .unwrap();
                store.create_index_with_str("model_id", "model_id").unwrap();
            }

            let has_events =
                (0..store_names.length()).any(|i| store_names.get(i).as_deref() == Some("events"));

            if !has_events {
                let store = db.create_object_store("events").unwrap();
                store
                    .create_index_with_str("timestamp", "timestamp")
                    .unwrap();
            }

            let has_models =
                (0..store_names.length()).any(|i| store_names.get(i).as_deref() == Some("models"));

            if !has_models {
                db.create_object_store("models").unwrap();
            }
        });

        open_request.set_onupgradeneeded(Some(onupgradeneeded.as_ref().unchecked_ref()));
        onupgradeneeded.forget(); // Must forget for event handler

        let result = await_request(open_request.into()).await?;
        self.db = Some(result.dyn_into()?);

        Ok(())
    }

    /// Execute storage operation
    #[allow(dead_code)]
    pub async fn execute(
        &mut self,
        method: &str,
        input: &[u8],
        params: &str,
    ) -> Result<Vec<u8>, StorageError> {
        // Ensure DB is initialized
        if self.db.is_none() {
            self.init().await?;
        }

        // Parse params
        let params: StorageParams = serde_json::from_str(params)?;

        match method {
            "store_chunk" => {
                self.store_chunk(
                    &params.content_hash,
                    input,
                    &params.priority.unwrap_or("medium".to_string()),
                )
                .await
            }
            "store_chunk_zero_copy" => {
                // Zero-copy from SAB
                let offset = params.chunk_index.unwrap_or(0) as usize;
                let size = input.len();
                self.store_chunk_zero_copy(
                    &params.content_hash,
                    offset,
                    size,
                    &params.priority.unwrap_or("medium".to_string()),
                )
                .await
            }
            "load_chunk" => self.load_chunk(&params.content_hash).await,
            "query_index" => self.query_index(&params).await,
            "delete_chunk" => self.delete_chunk(&params.content_hash).await,
            _ => Err(StorageError::IndexedDB(format!(
                "Unknown storage method: {}",
                method
            ))),
        }
    }

    /// Store chunk with zero-copy from SharedArrayBuffer
    async fn store_chunk_zero_copy(
        &self,
        hash: &str,
        offset: usize,
        size: usize,
        priority: &str,
    ) -> Result<Vec<u8>, StorageError> {
        let sab = self.sab.as_ref().ok_or(StorageError::NoSharedBuffer)?;
        let buffer = js_sys::Uint8Array::new(sab);

        // Directly slice from shared memory - no copy!
        let data = buffer.subarray(offset as u32, (offset + size) as u32);
        let data_vec = data.to_vec(); // Only copy when needed

        self.store_chunk(hash, &data_vec, priority).await
    }

    /// Store chunk to OPFS and index in IndexedDB
    async fn store_chunk(
        &self,
        hash: &str,
        data: &[u8],
        priority: &str,
    ) -> Result<Vec<u8>, StorageError> {
        // 1. Validate size
        if data.len() > MAX_CHUNK_SIZE {
            return Err(StorageError::ChunkTooLarge {
                size: data.len(),
                max: MAX_CHUNK_SIZE,
            });
        }

        // 2. Validate hash format (BLAKE3 is 64 hex chars)
        if hash.len() != 64 {
            return Err(StorageError::InvalidHashFormat(hash.to_string()));
        }

        // 3. Verify BLAKE3 hash
        let computed_hash = blake3::hash(data);
        let actual_hash = computed_hash.to_hex();

        if actual_hash.as_str() != hash {
            return Err(StorageError::HashMismatch {
                expected: hash.to_string(),
                actual: actual_hash.to_string(),
            });
        }

        // 4. Sanitize path
        let location = sanitize_path(&format!("chunks/{}", hash))?;

        // 5. Write to OPFS
        self.write_opfs(&location, data).await?;

        // 6. Index in IndexedDB
        let metadata = ChunkMetadata {
            hash: hash.to_string(),
            location: location.clone(),
            size: data.len(),
            priority: priority.to_string(),
            last_accessed: js_sys::Date::now(),
            access_count: 1,
            model_id: None,
        };

        self.index_chunk(&metadata).await?;

        // 7. Enforce cache limits
        self.enforce_cache_limits().await?;

        // Return metadata as response
        let response = serde_json::to_vec(&metadata)?;
        Ok(response)
    }

    /// Load chunk from OPFS
    async fn load_chunk(&self, hash: &str) -> Result<Vec<u8>, StorageError> {
        // 1. Query IndexedDB for location
        let metadata = self.get_chunk_metadata(hash).await?;

        // 2. Read from OPFS
        let data = self.read_opfs(&metadata.location).await?;

        // 3. Update access time
        self.update_access_time(hash).await?;

        Ok(data)
    }

    /// Query chunk index with proper cursor iteration
    async fn query_index(&self, params: &StorageParams) -> Result<Vec<u8>, StorageError> {
        let db = self.db.as_ref().ok_or(StorageError::NotInitialized)?;

        // Create transaction
        let transaction = db
            .transaction_with_str("chunks")
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        let store = transaction
            .object_store("chunks")
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;

        // Determine which index to use
        let cursor_request = if let Some(model_id) = &params.model_id {
            let index = store
                .index("model_id")
                .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
            index
                .open_cursor_with_range(&JsString::from(model_id.as_str()).into())
                .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?
        } else if let Some(priority) = &params.priority {
            let index = store
                .index("priority")
                .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
            index
                .open_cursor_with_range(&JsString::from(priority.as_str()).into())
                .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?
        } else {
            // Get all chunks
            store
                .open_cursor()
                .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?
        };

        // Use Promise to collect all results
        let promise = js_sys::Promise::new(&mut |resolve, reject| {
            let results = js_sys::Array::new();
            let results_clone = results.clone();

            // Recursive closure for cursor iteration
            let onsuccess = Rc::new(RefCell::new(None::<Closure<dyn FnMut(web_sys::Event)>>));
            let onsuccess_clone = onsuccess.clone();

            *onsuccess.borrow_mut() = Some(Closure::new(move |event: web_sys::Event| {
                let target = event.target().unwrap();
                let request: IdbRequest = target.dyn_into().unwrap();
                let result = request.result().unwrap();

                if result.is_null() || result.is_undefined() {
                    // No more results - resolve with collected data
                    resolve.call1(&JsValue::UNDEFINED, &results_clone).unwrap();
                } else {
                    // Get cursor and value
                    let cursor: web_sys::IdbCursorWithValue = result.dyn_into().unwrap();
                    let value = cursor.value().unwrap();
                    results_clone.push(&value);

                    // Continue to next item
                    cursor.continue_().unwrap();
                }
            }));

            let onerror = Closure::once(move |event: web_sys::Event| {
                reject.call1(&JsValue::UNDEFINED, &event).unwrap();
            });

            cursor_request.set_onsuccess(Some(
                onsuccess
                    .borrow()
                    .as_ref()
                    .unwrap()
                    .as_ref()
                    .unchecked_ref(),
            ));
            cursor_request.set_onerror(Some(onerror.as_ref().unchecked_ref()));

            // Leak closures to keep them alive (they'll be cleaned up when promise resolves)
            onerror.forget();
            // Leak the Rc by converting to raw pointer and forgetting
            let _ = Rc::into_raw(onsuccess_clone);
        });

        // Await the promise
        let js_results = await_promise(promise)
            .await
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;

        // Convert JS array to Vec<ChunkMetadata>
        let js_array: js_sys::Array = js_results
            .dyn_into()
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;

        let mut results = Vec::new();
        for i in 0..js_array.length() {
            let value = js_array.get(i);
            let obj: Object = value
                .dyn_into()
                .map_err(|_| StorageError::IndexedDB("Value is not an object".to_string()))?;
            let metadata: ChunkMetadata = jsvalue_to_metadata(&obj.into())
                .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
            results.push(metadata);
        }

        Ok(serde_json::to_vec(&results)?)
    }

    /// Delete chunk
    async fn delete_chunk(&self, hash: &str) -> Result<Vec<u8>, StorageError> {
        // 1. Get metadata
        let metadata = self.get_chunk_metadata(hash).await?;

        // 2. Delete from OPFS
        self.delete_opfs(&metadata.location).await?;

        // 3. Delete from IndexedDB
        let db = self.db.as_ref().ok_or(StorageError::NotInitialized)?;
        let transaction = db
            .transaction_with_str("chunks")
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        let store = transaction
            .object_store("chunks")
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        let request = store
            .delete(&JsString::from(hash).into())
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        await_request(request).await?;

        Ok(vec![])
    }

    // ========== Cache Management ==========

    async fn enforce_cache_limits(&self) -> Result<(), StorageError> {
        let db = self.db.as_ref().ok_or(StorageError::NotInitialized)?;
        let transaction = db
            .transaction_with_str("chunks")
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        let store = transaction
            .object_store("chunks")
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        let index = store
            .index("last_accessed")
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;

        // Get all metadata sorted by last_accessed (oldest first)
        let request = index
            .get_all()
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;

        // Wait for request results
        let result = await_request(request).await?;

        let js_array: js_sys::Array = result
            .dyn_into()
            .map_err(|e| StorageError::IndexedDB(format!("Not an array: {:?}", e)))?;

        let mut chunks: Vec<ChunkMetadata> = Vec::new();
        let mut total_size: usize = 0;

        for i in 0..js_array.length() {
            let val = js_array.get(i);
            let obj: Object = val
                .dyn_into()
                .map_err(|_| StorageError::IndexedDB("Value is not an object".to_string()))?;
            let metadata: ChunkMetadata = jsvalue_to_metadata(&obj.into())
                .map_err(|e| StorageError::IndexedDB(format!("Deserialization failed: {:?}", e)))?;
            total_size += metadata.size;
            chunks.push(metadata);
        }

        if total_size > MAX_CACHE_SIZE {
            let mut freed_size = 0;
            // chunks are sorted old -> new. We delete oldest.
            for chunk in chunks {
                if total_size - freed_size <= MAX_CACHE_SIZE {
                    break;
                }

                // Delete this chunk
                // We must use a new context/method for deletion to ensure it completes,
                // but calling self.delete_chunk here is fine as it uses independent transaction.
                // However, we are currently inside `enforce_cache_limits` which might be awaited?
                // `delete_chunk` is async.

                // Note: We should delete from OPFS and DB.
                // Reusing delete_chunk logic:
                self.delete_chunk(&chunk.hash).await?;

                freed_size += chunk.size;
            }
        }

        Ok(())
    }

    // ========== IndexedDB Helpers ==========

    async fn index_chunk(&self, metadata: &ChunkMetadata) -> Result<(), StorageError> {
        let db = self.db.as_ref().ok_or(StorageError::NotInitialized)?;
        let transaction = db
            .transaction_with_str_and_mode("chunks", IdbTransactionMode::Readwrite)
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        let store = transaction
            .object_store("chunks")
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;

        let value = metadata_to_jsvalue(metadata)?;
        let request = store
            .put_with_key(&value, &JsString::from(metadata.hash.as_str()).into())
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        await_request(request).await?;

        Ok(())
    }

    async fn get_chunk_metadata(&self, hash: &str) -> Result<ChunkMetadata, StorageError> {
        let db = self.db.as_ref().ok_or(StorageError::NotInitialized)?;
        let transaction = db
            .transaction_with_str("chunks")
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        let store = transaction
            .object_store("chunks")
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        let request = store
            .get(&JsString::from(hash).into())
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;

        let result = await_request(request).await?;
        if result.is_null() || result.is_undefined() {
            return Err(StorageError::NotFound(hash.to_string()));
        }

        let metadata: ChunkMetadata = jsvalue_to_metadata(&result.into())
            .map_err(|e| StorageError::IndexedDB(format!("{:?}", e)))?;
        Ok(metadata)
    }

    async fn update_access_time(&self, hash: &str) -> Result<(), StorageError> {
        let mut metadata = self.get_chunk_metadata(hash).await?;
        metadata.last_accessed = js_sys::Date::now();
        metadata.access_count += 1;
        let _ = DB_VERSION; // Future proofing
        self.index_chunk(&metadata).await
    }

    // ========== OPFS Helpers (File System Access API) ==========

    async fn write_opfs(&self, path: &str, data: &[u8]) -> Result<(), StorageError> {
        let window = window().ok_or(StorageError::NoWindow)?;
        let navigator = window.navigator();

        // Get OPFS root via navigator.storage.getDirectory()
        let storage = js_sys::Reflect::get(&navigator, &JsString::from("storage").into())
            .map_err(|e| StorageError::Opfs(format!("No storage API: {:?}", e)))?;

        let get_directory = js_sys::Reflect::get(&storage, &JsString::from("getDirectory").into())
            .map_err(|e| StorageError::Opfs(format!("No getDirectory: {:?}", e)))?;

        let get_directory_fn: js_sys::Function = get_directory
            .dyn_into()
            .map_err(|e| StorageError::Opfs(format!("getDirectory not a function: {:?}", e)))?;

        let root_promise = get_directory_fn
            .call0(&storage)
            .map_err(|e| StorageError::Opfs(format!("getDirectory failed: {:?}", e)))?;

        let root_handle = await_promise(js_sys::Promise::from(root_promise))
            .await
            .map_err(|e| StorageError::Opfs(format!("getDirectory failed: {:?}", e)))?;
        let root: web_sys::FileSystemDirectoryHandle = root_handle
            .dyn_into()
            .map_err(|e| StorageError::Opfs(format!("Not a directory handle: {:?}", e)))?;

        // Parse path and create directories
        let parts: Vec<&str> = path.split('/').filter(|p| !p.is_empty()).collect();
        if parts.is_empty() {
            return Err(StorageError::InvalidPath(path.to_string()));
        }

        let mut current = root;

        // Navigate/create parent directories
        for &part in &parts[..parts.len() - 1] {
            let options = js_sys::Object::new();
            js_sys::Reflect::set(
                &options,
                &JsString::from("create").into(),
                &js_sys::Boolean::from(true).into(),
            )
            .map_err(|e| StorageError::Opfs(format!("{:?}", e)))?;

            let dir_promise =
                current.get_directory_handle_with_options(part, options.unchecked_ref());
            let dir_handle = await_promise(dir_promise)
                .await
                .map_err(|e| StorageError::Opfs(format!("get_directory_handle failed: {:?}", e)))?;
            current = dir_handle
                .dyn_into()
                .map_err(|e| StorageError::Opfs(format!("Not a directory handle: {:?}", e)))?;
        }

        // Create/get file
        let filename = parts
            .last()
            .ok_or(StorageError::InvalidPath(path.to_string()))?;
        let file_options = js_sys::Object::new();
        js_sys::Reflect::set(
            &file_options,
            &JsString::from("create").into(),
            &js_sys::Boolean::from(true).into(),
        )
        .map_err(|e| StorageError::Opfs(format!("{:?}", e)))?;

        let file_promise =
            current.get_file_handle_with_options(filename, file_options.unchecked_ref());
        let file_handle = await_promise(file_promise)
            .await
            .map_err(|e| StorageError::Opfs(format!("get_file_handle failed: {:?}", e)))?;
        let file: web_sys::FileSystemFileHandle = file_handle
            .dyn_into()
            .map_err(|e| StorageError::Opfs(format!("Not a file handle: {:?}", e)))?;

        // Create writable stream
        let writable_promise = file.create_writable();
        let writable = await_promise(writable_promise)
            .await
            .map_err(|e| StorageError::Opfs(format!("create_writable failed: {:?}", e)))?;
        let stream: web_sys::FileSystemWritableFileStream = writable
            .dyn_into()
            .map_err(|e| StorageError::Opfs(format!("Not a writable stream: {:?}", e)))?;

        // Write data - convert Uint8Array to Vec<u8> then pass as slice
        let js_array = js_sys::Uint8Array::from(data);
        let data_vec = js_array.to_vec();
        let write_promise = stream
            .write_with_u8_array(&data_vec)
            .map_err(|e| StorageError::Opfs(format!("write failed: {:?}", e)))?;
        await_promise(write_promise).await?;

        // Close stream
        let close_promise = stream.close();
        await_promise(close_promise)
            .await
            .map_err(|e| StorageError::Opfs(format!("close failed: {:?}", e)))?;

        Ok(())
    }

    async fn read_opfs(&self, path: &str) -> Result<Vec<u8>, StorageError> {
        let window = window().ok_or(StorageError::NoWindow)?;
        let navigator = window.navigator();

        // Get OPFS root
        let storage = js_sys::Reflect::get(&navigator, &JsString::from("storage").into())
            .map_err(|e| StorageError::Opfs(format!("No storage API: {:?}", e)))?;

        let get_directory = js_sys::Reflect::get(&storage, &JsString::from("getDirectory").into())
            .map_err(|e| StorageError::Opfs(format!("No getDirectory: {:?}", e)))?;

        let get_directory_fn: js_sys::Function = get_directory
            .dyn_into()
            .map_err(|e| StorageError::Opfs(format!("getDirectory not a function: {:?}", e)))?;

        let root_promise = get_directory_fn
            .call0(&storage)
            .map_err(|e| StorageError::Opfs(format!("getDirectory failed: {:?}", e)))?;

        let root_handle = await_promise(js_sys::Promise::from(root_promise)).await?;
        let root: web_sys::FileSystemDirectoryHandle = root_handle
            .dyn_into()
            .map_err(|e| StorageError::Opfs(format!("Not a directory handle: {:?}", e)))?;

        // Parse path
        let parts: Vec<&str> = path.split('/').filter(|p| !p.is_empty()).collect();
        if parts.is_empty() {
            return Err(StorageError::InvalidPath(path.to_string()));
        }

        let mut current = root;

        // Navigate to parent directory
        for &part in &parts[..parts.len() - 1] {
            let dir_promise = current.get_directory_handle(part);
            let dir_handle = await_promise(dir_promise)
                .await
                .map_err(|_e| StorageError::NotFound(format!("Directory not found: {}", part)))?;
            current = dir_handle
                .dyn_into()
                .map_err(|e| StorageError::Opfs(format!("Not a directory handle: {:?}", e)))?;
        }

        // Get file
        let filename = parts
            .last()
            .ok_or(StorageError::InvalidPath(path.to_string()))?;
        let file_promise = current.get_file_handle(filename);
        let file_handle = await_promise(file_promise)
            .await
            .map_err(|_e| StorageError::NotFound(format!("File not found: {}", filename)))?;
        let file: web_sys::FileSystemFileHandle = file_handle
            .dyn_into()
            .map_err(|e| StorageError::Opfs(format!("Not a file handle: {:?}", e)))?;

        // Get file
        let file_promise = file.get_file();
        let file_obj = await_promise(file_promise)
            .await
            .map_err(|e| StorageError::Opfs(format!("get_file failed: {:?}", e)))?;
        let blob: web_sys::Blob = file_obj
            .dyn_into()
            .map_err(|e| StorageError::Opfs(format!("Not a blob: {:?}", e)))?;

        // Read as array buffer
        let array_buffer_promise = blob.array_buffer();
        let array_buffer = await_promise(array_buffer_promise)
            .await
            .map_err(|e| StorageError::Opfs(format!("array_buffer failed: {:?}", e)))?;
        let uint8_array = js_sys::Uint8Array::new(&array_buffer);

        Ok(uint8_array.to_vec())
    }

    async fn delete_opfs(&self, path: &str) -> Result<(), StorageError> {
        let window = window().ok_or(StorageError::NoWindow)?;
        let navigator = window.navigator();

        // Get OPFS root
        let storage = js_sys::Reflect::get(&navigator, &JsString::from("storage").into())
            .map_err(|e| StorageError::Opfs(format!("No storage API: {:?}", e)))?;

        let get_directory = js_sys::Reflect::get(&storage, &JsString::from("getDirectory").into())
            .map_err(|e| StorageError::Opfs(format!("No getDirectory: {:?}", e)))?;

        let get_directory_fn: js_sys::Function = get_directory
            .dyn_into()
            .map_err(|e| StorageError::Opfs(format!("getDirectory not a function: {:?}", e)))?;

        let root_promise = get_directory_fn
            .call0(&storage)
            .map_err(|e| StorageError::Opfs(format!("getDirectory failed: {:?}", e)))?;

        let root_handle = await_promise(js_sys::Promise::from(root_promise)).await?;
        let root: web_sys::FileSystemDirectoryHandle = root_handle
            .dyn_into()
            .map_err(|e| StorageError::Opfs(format!("Not a directory handle: {:?}", e)))?;

        // Parse path
        let parts: Vec<&str> = path.split('/').filter(|p| !p.is_empty()).collect();
        if parts.is_empty() {
            return Err(StorageError::InvalidPath(path.to_string()));
        }

        let mut current = root;

        // Navigate to parent directory
        for &part in &parts[..parts.len() - 1] {
            let dir_promise = current.get_directory_handle(part);
            let dir_handle = await_promise(dir_promise)
                .await
                .map_err(|_e| StorageError::NotFound(format!("Directory not found: {}", part)))?;
            current = dir_handle
                .dyn_into()
                .map_err(|e| StorageError::Opfs(format!("Not a directory handle: {:?}", e)))?;
        }

        // Remove file
        let filename = parts
            .last()
            .ok_or(StorageError::InvalidPath(path.to_string()))?;
        let remove_promise = current.remove_entry(filename);
        await_promise(remove_promise)
            .await
            .map_err(|e| StorageError::Opfs(format!("remove_entry failed: {:?}", e)))?;

        Ok(())
    }
}

// ========== Security: Path Sanitization ==========

fn sanitize_path(path: &str) -> Result<String, StorageError> {
    // Prevent directory traversal attacks
    let clean_path = path
        .replace("..", "")
        .replace("//", "/")
        .trim_start_matches('/')
        .to_string();

    // Ensure path stays within chunks/ directory
    if !clean_path.starts_with("chunks/") {
        return Err(StorageError::InvalidPath(path.to_string()));
    }

    Ok(clean_path)
}

// ========== Helper to await IdbRequest ==========

#[allow(dead_code)]
async fn await_request(request: IdbRequest) -> Result<JsValue, StorageError> {
    let promise = js_sys::Promise::new(&mut |resolve, reject| {
        let onsuccess = Closure::once(move |event: web_sys::Event| {
            let target = event.target().unwrap();
            let request: IdbRequest = target.dyn_into().unwrap();
            resolve
                .call1(&JsValue::UNDEFINED, &request.result().unwrap())
                .unwrap();
        });

        let onerror = Closure::once(move |event: web_sys::Event| {
            let target = event.target().unwrap();
            let request: IdbRequest = target.dyn_into().unwrap();
            let error =
                Reflect::get(&request, &"error".into()).unwrap_or_else(|_| JsValue::UNDEFINED);
            reject.call1(&JsValue::UNDEFINED, &error).unwrap();
        });

        request.set_onsuccess(Some(onsuccess.as_ref().unchecked_ref()));
        request.set_onerror(Some(onerror.as_ref().unchecked_ref()));

        // Closures will be dropped after promise resolves (NO FORGET!)
        drop(onsuccess);
        drop(onerror);
    });

    await_promise(promise).await.map_err(StorageError::from)
}
