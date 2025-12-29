use js_sys::{JsString, Object, Promise};
// Note: JsCast trait needed for unchecked_into method
use web_sys::wasm_bindgen::JsCast;

use crate::p2p::{ErrorContext, P2pConfig, P2pError, Result};
use async_trait::async_trait;
use blake3::Hasher;
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::collections::HashMap;

/// Chunk data structure with verification methods
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Chunk {
    pub id: String,
    pub model_id: String,
    pub index: usize,
    pub data: Vec<u8>,
    pub hash: String,
    pub size: usize,
}

impl Chunk {
    /// Compute BLAKE3 hash of chunk data
    pub fn compute_hash(&self) -> String {
        let mut hasher = Hasher::new();
        hasher.update(&self.data);
        hasher.finalize().to_hex().to_string()
    }

    /// Verify chunk integrity
    pub fn is_valid(&self) -> bool {
        self.compute_hash() == self.hash
    }
}

/// Chunk metadata from storage (matches StorageUnit response)
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ChunkMetadata {
    pub hash: String,
    pub location: String,
    pub size: usize,
    pub priority: String,
    pub last_accessed: f64,
    pub access_count: u32,
    pub model_id: Option<String>,
}

/// Chunk metadata for tracking
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ChunkInfo {
    pub id: String,
    pub index: usize,
    pub hash: String,
    pub size: usize,
    pub available_locally: bool,
}

/// Trait for loading model chunks
#[async_trait(?Send)]
pub trait ChunkLoader: Send + Sync {
    /// Load all chunks for a model
    async fn load_model_chunks(&self, model_id: &str) -> Result<Vec<Chunk>>;

    /// Assemble chunks into model weights
    async fn assemble_weights(&self, chunks: Vec<Chunk>) -> Result<HashMap<String, Vec<f32>>>;

    /// Verify chunk integrity using BLAKE3
    fn verify_chunk(&self, chunk: &Chunk) -> Result<()>;

    /// Load a single chunk
    async fn load_chunk(&self, chunk_id: &str) -> Result<Chunk>;
}

/// Storage-based chunk loader implementation
pub struct StorageChunkLoader {
    config: P2pConfig,
}

// WASM bridge to compute engine - returns a Promise, not async
#[allow(improper_ctypes)]
extern "C" {
    fn compute_execute(library: &str, action: &str, input: Object, params: Object) -> Promise;
}

impl StorageChunkLoader {
    pub fn new(config: P2pConfig) -> Self {
        Self { config }
    }

    /// Query storage for chunk IDs
    async fn query_chunk_ids(&self, model_id: &str) -> Result<Vec<String>> {
        log::info!("Querying chunks for model: {}", model_id);

        // Call compute engine's storage unit
        let params = json!({
            "operation": "query_index",
            "model_id": model_id,
        });

        let promise = unsafe {
            compute_execute(
                "storage",
                "query_index",
                Object::new(),
                JsString::from(serde_json::to_string(&params).unwrap()).into(),
            )
        };

        let result =
            crate::await_promise(promise)
                .await
                .map_err(|e| P2pError::SerializationError {
                    context: ErrorContext::new("compute_execute_failed"),
                    source: Some(Box::new(std::io::Error::other(format!("{:?}", e)))),
                })?;

        // Parse result
        let chunks: Vec<ChunkMetadata> =
            serde_json::from_str(result.as_string().unwrap_or_default().as_str()).map_err(|e| {
                P2pError::DeserializationError {
                    context: ErrorContext::new("query_result"),
                    source: Some(Box::new(e)),
                }
            })?;

        Ok(chunks.into_iter().map(|c| c.hash).collect())
    }

    /// Fetch a single chunk with retry logic
    async fn fetch_chunk_with_retry(&self, chunk_id: String) -> Result<Chunk> {
        for attempt in 0..self.config.retry_attempts {
            match self.fetch_chunk(&chunk_id).await {
                Ok(chunk) => return Ok(chunk),
                Err(e) if attempt < self.config.retry_attempts - 1 => {
                    log::warn!("Chunk fetch failed (attempt {}): {}", attempt + 1, e);
                    // Exponential backoff
                    let delay_ms = 100 * 2_u64.pow(attempt);
                    sleep_async(delay_ms).await;
                    continue;
                }
                Err(e) => return Err(e),
            }
        }
        Err(P2pError::ChunkNotFound {
            chunk_id,
            context: ErrorContext::default(),
        })
    }

    /// Fetch a single chunk from storage
    async fn fetch_chunk(&self, chunk_id: &str) -> Result<Chunk> {
        log::info!("Fetching chunk: {}", chunk_id);

        // Call compute engine's storage unit
        let params = json!({
            "operation": "load_chunk",
            "content_hash": chunk_id,
        });

        let promise = unsafe {
            compute_execute(
                "storage",
                "load_chunk",
                Object::new(),
                JsString::from(serde_json::to_string(&params).unwrap()).into(),
            )
        };

        let result =
            crate::await_promise(promise)
                .await
                .map_err(|e| P2pError::SerializationError {
                    context: ErrorContext::new("compute_execute_failed"),
                    source: Some(Box::new(std::io::Error::other(format!("{:?}", e)))),
                })?;

        // Check for errors
        if result.is_null() || result.is_undefined() {
            return Err(P2pError::ChunkNotFound {
                chunk_id: chunk_id.to_string(),
                context: ErrorContext::new("storage_returned_null"),
            });
        }

        // Convert to bytes using stable ABI to avoid hashed imports
        let data = sdk::js_interop::wrap_u8_array(result.clone().into()).to_vec();

        // Verify hash
        let computed_hash = {
            let mut hasher = Hasher::new();
            hasher.update(&data);
            hasher.finalize().to_hex().to_string()
        };

        if computed_hash != chunk_id {
            return Err(P2pError::HashMismatch {
                expected: chunk_id.to_string(),
                actual: computed_hash,
                chunk_id: chunk_id.to_string(),
            });
        }

        // Create chunk
        let size = data.len();
        Ok(Chunk {
            id: chunk_id.to_string(),
            model_id: String::new(), // Will be filled by caller
            index: 0,                // Will be filled by caller
            data,
            hash: chunk_id.to_string(),
            size,
        })
    }

    /// Deserialize chunk data into weights (safetensors format)
    fn deserialize_chunk(&self, chunk: &Chunk) -> Result<HashMap<String, Vec<f32>>> {
        log::info!("Deserializing chunk {} ({} bytes)", chunk.id, chunk.size);

        if chunk.data.is_empty() {
            return Ok(HashMap::new());
        }

        // Parse safetensors
        let tensors = safetensors::SafeTensors::deserialize(&chunk.data).map_err(|e| {
            P2pError::DeserializationError {
                context: ErrorContext::new("safetensors_deserialize")
                    .with_chunk_id(&chunk.id)
                    .with_model_id(&chunk.model_id),
                source: Some(Box::new(e)),
            }
        })?;

        let mut weights = HashMap::new();

        // Iterate over tensors and convert key-value pairs
        for (name, view) in tensors.tensors() {
            let dtype = view.dtype();
            let data = view.data();

            // We currently only support F32 for the internal weight representation
            match dtype {
                safetensors::Dtype::F32 => {
                    // Safety: data is guaranteed to be aligned for u8, but we need to cast to f32.
                    // SafeTensors ensures data length is correct for the shape and type.
                    // We must handle endianness if necessary, but safetensors is usually little-endian.
                    // For WASM/internal use, standard casting usually suffices if arch matches.
                    // However, robust way is using bytemuck or manual reconstruction.

                    let count = data.len() / 4;
                    let mut float_data = Vec::with_capacity(count);

                    // Manual copy to ensure alignment safety and endianness handling (little endian standard)
                    for chunk in data.chunks_exact(4) {
                        let val = f32::from_le_bytes(chunk.try_into().unwrap());
                        float_data.push(val);
                    }

                    weights.insert(name.to_string(), float_data);
                }
                _ => {
                    log::warn!(
                        "Unsupported dtype {:?} for tensor '{}' in chunk {} (skipping)",
                        dtype,
                        name,
                        chunk.id
                    );
                    // P2P/Mesh robustness: skip unsupported types rather than failing entire chunk
                }
            }
        }

        Ok(weights)
    }
}

#[async_trait(?Send)]
impl ChunkLoader for StorageChunkLoader {
    async fn load_model_chunks(&self, model_id: &str) -> Result<Vec<Chunk>> {
        // Query storage for chunk list
        let chunk_ids = self.query_chunk_ids(model_id).await?;

        if chunk_ids.is_empty() {
            return Err(P2pError::ModelNotFound {
                model_id: model_id.to_string(),
                available_models: vec![],
                context: ErrorContext::default(),
            });
        }

        // Parallel fetch with limit
        let mut chunks = Vec::new();
        let chunk_count = chunk_ids.len();
        let batch_size = self.config.parallel_chunk_limit;

        for batch_start in (0..chunk_count).step_by(batch_size) {
            let batch_end = (batch_start + batch_size).min(chunk_count);
            let batch = &chunk_ids[batch_start..batch_end];

            // Fetch batch in parallel
            let futures: Vec<_> = batch
                .iter()
                .map(|id| self.fetch_chunk_with_retry(id.clone()))
                .collect();

            let batch_chunks = futures::future::try_join_all(futures).await?;
            chunks.extend(batch_chunks);
        }

        Ok(chunks)
    }

    async fn assemble_weights(&self, chunks: Vec<Chunk>) -> Result<HashMap<String, Vec<f32>>> {
        // Streaming assembly to avoid OOM
        let mut weights = HashMap::new();

        for chunk in chunks {
            // Verify before deserializing
            self.verify_chunk(&chunk)?;

            // Deserialize and merge
            let partial_weights = self.deserialize_chunk(&chunk)?;
            weights.extend(partial_weights);
        }

        Ok(weights)
    }

    fn verify_chunk(&self, chunk: &Chunk) -> Result<()> {
        if !chunk.is_valid() {
            return Err(P2pError::HashMismatch {
                expected: chunk.hash.clone(),
                actual: chunk.compute_hash(),
                chunk_id: chunk.id.clone(),
            });
        }
        Ok(())
    }

    async fn load_chunk(&self, chunk_id: &str) -> Result<Chunk> {
        self.fetch_chunk_with_retry(chunk_id.to_string()).await
    }
}

/// Async sleep helper for WASM compatibility
async fn sleep_async(ms: u64) {
    #[cfg(target_arch = "wasm32")]
    {
        let _ms = ms;
        let promise = js_sys::Promise::new(&mut |resolve, _reject| {
            if let Some(window) = web_sys::window() {
                let _ = window.set_timeout_with_callback_and_timeout_and_arguments_0(
                    &resolve.unchecked_into::<js_sys::Function>(),
                    _ms as i32,
                );
            }
        });
        let _ = crate::await_promise(promise).await;
    }

    #[cfg(not(target_arch = "wasm32"))]
    {
        // For non-WASM targets, use tokio sleep if available
        // Otherwise this is a no-op
        #[cfg(feature = "tokio")]
        tokio::time::sleep(std::time::Duration::from_millis(ms)).await;
        #[cfg(not(feature = "tokio"))]
        let _ = ms;
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_chunk_verification() {
        let data = b"test data";
        let mut hasher = Hasher::new();
        hasher.update(data);
        let hash = hasher.finalize().to_hex().to_string();

        let chunk = Chunk {
            id: "test-chunk".to_string(),
            model_id: "test-model".to_string(),
            index: 0,
            data: data.to_vec(),
            hash: hash.clone(),
            size: data.len(),
        };

        let loader = StorageChunkLoader::new(P2pConfig::default());
        assert!(loader.verify_chunk(&chunk).is_ok());

        // Test hash mismatch
        let mut bad_chunk = chunk.clone();
        bad_chunk.hash = "invalid".to_string();
        assert!(loader.verify_chunk(&bad_chunk).is_err());
    }
}
