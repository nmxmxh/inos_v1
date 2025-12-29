use crate::engine::{MLError, Result};
use serde::{Deserialize, Serialize};
use serde_json::Value as JsonValue;
use std::collections::HashMap;

/// INOS-native inference engine with P2P model distribution
///
/// Architecture: Following INOS principles
/// - P2P model distribution: 1MB chunks via storage mesh
/// - Zero-copy SAB: Direct SharedArrayBuffer access
/// - WASM sandboxing: Resource limits, fuel limits
/// - Progressive loading: Start inference while downloading chunks
/// - Local quantization: No model downloads, quantize on-device
pub struct InferenceJob {
    model_cache: ModelCache,
    chunk_manager: ChunkManager,
    quantizer: LocalQuantizer,
    config: InferenceConfig,
}

#[derive(Clone)]
struct InferenceConfig {
    max_cache_size_mb: usize,
    chunk_size: usize, // 1MB chunks (INOS standard)
    max_concurrent_chunks: usize,
    enable_progressive: bool,
}

impl Default for InferenceConfig {
    fn default() -> Self {
        Self {
            max_cache_size_mb: 512,  // 512MB model cache
            chunk_size: 1024 * 1024, // 1MB chunks (INOS standard)
            max_concurrent_chunks: 10,
            enable_progressive: true,
        }
    }
}

/// Model cache (LRU, size-limited)
struct ModelCache {
    models: HashMap<String, CachedModel>,
    total_size_bytes: usize,
    max_size_bytes: usize,
}

#[derive(Clone)]
struct CachedModel {
    id: String,
    chunks: Vec<ModelChunk>,
    metadata: ModelMetadata,
    size_bytes: usize,
    last_used: std::time::Instant,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct ModelChunk {
    pub index: usize,
    pub hash: String, // BLAKE3 hash (INOS standard)
    pub data: Vec<u8>,
    pub compressed: bool,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct ModelMetadata {
    pub model_id: String,
    pub total_chunks: usize,
    pub total_size: usize,
    pub dtype: String,
    pub architecture: String,
}

/// Chunk manager for P2P distribution
struct ChunkManager {
    chunk_size: usize,
}

impl ChunkManager {
    fn new(chunk_size: usize) -> Self {
        Self { chunk_size }
    }

    /// Split model into 1MB chunks (INOS standard)
    fn chunk_model(&self, model_data: &[u8]) -> Vec<ModelChunk> {
        let num_chunks = model_data.len().div_ceil(self.chunk_size);
        let mut chunks = Vec::with_capacity(num_chunks);

        for i in 0..num_chunks {
            let start = i * self.chunk_size;
            let end = (start + self.chunk_size).min(model_data.len());
            let chunk_data = &model_data[start..end];

            chunks.push(ModelChunk {
                index: i,
                hash: self.hash_chunk(chunk_data),
                data: chunk_data.to_vec(),
                compressed: false,
            });
        }

        chunks
    }

    /// BLAKE3 hash (INOS standard) with streaming for large chunks
    fn hash_chunk(&self, data: &[u8]) -> String {
        const CHUNK_SIZE: usize = 1024 * 1024; // 1MB chunks

        if data.len() > CHUNK_SIZE {
            // Stream large chunks to avoid memory spikes
            let mut hasher = blake3::Hasher::new();
            for chunk in data.chunks(CHUNK_SIZE) {
                hasher.update(chunk);
            }
            hasher.finalize().to_hex().to_string()
        } else {
            // Single-shot for small chunks (faster)
            blake3::hash(data).to_hex().to_string()
        }
    }

    /// Reconstruct model from chunks
    fn reconstruct_model(&self, chunks: &[ModelChunk]) -> Result<Vec<u8>> {
        let total_size: usize = chunks.iter().map(|c| c.data.len()).sum();
        let mut model_data = Vec::with_capacity(total_size);

        for chunk in chunks {
            model_data.extend_from_slice(&chunk.data);
        }

        Ok(model_data)
    }
}

/// Local quantization engine (no downloads)
struct LocalQuantizer {
    #[allow(dead_code)]
    quantization_cache: HashMap<String, QuantizedModel>,
}

#[derive(Clone)]
struct QuantizedModel {
    data: Vec<i8>,
    scale: f32,
    zero_point: i32,
    #[allow(dead_code)]
    original_dtype: String,
}

impl LocalQuantizer {
    fn new() -> Self {
        Self {
            quantization_cache: HashMap::new(),
        }
    }

    /// Int8 quantization (local, no downloads)
    fn quantize_int8(&self, input: &[f32]) -> Result<QuantizedModel> {
        // Min-max quantization
        let min = input.iter().fold(f32::INFINITY, |a, &b| a.min(b));
        let max = input.iter().fold(f32::NEG_INFINITY, |a, &b| a.max(b));

        let scale = (max - min) / 255.0;
        let zero_point = (-min / scale).round() as i32;

        let quantized: Vec<i8> = input
            .iter()
            .map(|&x| {
                let q = ((x - min) / scale).round() as i32 + zero_point;
                q.clamp(0, 255) as i8
            })
            .collect();

        Ok(QuantizedModel {
            data: quantized,
            scale,
            zero_point,
            original_dtype: "float32".to_string(),
        })
    }

    /// Int4 quantization (local, no downloads)
    fn quantize_int4(&self, input: &[f32]) -> Result<Vec<u8>> {
        // Pack two 4-bit values per byte
        let quantized_i8 = self.quantize_int8(input)?;

        let mut packed = Vec::with_capacity(quantized_i8.data.len().div_ceil(2));
        for chunk in quantized_i8.data.chunks(2) {
            let high = (chunk[0] >> 4) & 0x0F;
            let low = if chunk.len() > 1 {
                (chunk[1] >> 4) & 0x0F
            } else {
                0
            };
            packed.push(((high << 4) | low) as u8);
        }

        Ok(packed)
    }

    /// Dequantize back to f32
    fn dequantize(&self, quantized: &QuantizedModel) -> Result<Vec<f32>> {
        let dequantized: Vec<f32> = quantized
            .data
            .iter()
            .map(|&q| {
                let q_int = q as i32 - quantized.zero_point;
                q_int as f32 * quantized.scale
            })
            .collect();

        Ok(dequantized)
    }
}

impl ModelCache {
    fn new(max_size_mb: usize) -> Self {
        Self {
            models: HashMap::new(),
            total_size_bytes: 0,
            max_size_bytes: max_size_mb * 1024 * 1024,
        }
    }

    fn get(&mut self, model_id: &str) -> Option<&CachedModel> {
        if let Some(model) = self.models.get_mut(model_id) {
            model.last_used = std::time::Instant::now();
            Some(model)
        } else {
            None
        }
    }

    fn insert(&mut self, model: CachedModel) -> Result<()> {
        // Evict old models if cache is full
        while self.total_size_bytes + model.size_bytes > self.max_size_bytes {
            self.evict_lru()?;
        }

        self.total_size_bytes += model.size_bytes;
        self.models.insert(model.id.clone(), model);

        Ok(())
    }

    fn evict_lru(&mut self) -> Result<()> {
        if let Some((id, model)) = self
            .models
            .iter()
            .min_by_key(|(_, m)| m.last_used)
            .map(|(id, m)| (id.clone(), m.clone()))
        {
            self.total_size_bytes -= model.size_bytes;
            self.models.remove(&id);
        }

        Ok(())
    }
}

impl InferenceJob {
    pub fn new() -> Result<Self> {
        let config = InferenceConfig::default();

        Ok(Self {
            model_cache: ModelCache::new(config.max_cache_size_mb),
            chunk_manager: ChunkManager::new(config.chunk_size),
            quantizer: LocalQuantizer::new(),
            config,
        })
    }

    // ===== MODEL LOADING (P2P MESH) =====

    /// Load model from P2P mesh (stub - delegates to kernel)
    pub fn load_model_p2p(&mut self, model_id: &str, _params: &JsonValue) -> Result<String> {
        // Check cache first
        if self.model_cache.get(model_id).is_some() {
            return Ok(format!("Model {} loaded from cache", model_id));
        }

        // Create request for kernel to fetch via P2P mesh
        let request = P2PModelRequest {
            model_id: model_id.to_string(),
            chunk_size: self.config.chunk_size,
            max_concurrent: self.config.max_concurrent_chunks,
            progressive: self.config.enable_progressive,
        };

        // Serialize request for kernel
        let request_json =
            serde_json::to_string(&request).map_err(|e| MLError::InferenceError(e.to_string()))?;

        // Return request for kernel to handle
        Ok(format!("P2P request: {}", request_json))
    }

    /// Store model chunks (called by kernel after P2P fetch)
    pub fn store_model_chunks(
        &mut self,
        model_id: &str,
        chunks: Vec<ModelChunk>,
        metadata: ModelMetadata,
    ) -> Result<()> {
        let size_bytes: usize = chunks.iter().map(|c| c.data.len()).sum();

        let cached_model = CachedModel {
            id: model_id.to_string(),
            chunks,
            metadata,
            size_bytes,
            last_used: std::time::Instant::now(),
        };

        self.model_cache.insert(cached_model)
    }

    // ===== INFERENCE (STUBS - DELEGATE TO KERNEL) =====

    /// LLM inference (stub)
    pub fn run_llm(&self, prompt: &str, params: &JsonValue) -> Result<String> {
        let model_id = params
            .get("model_id")
            .and_then(|v| v.as_str())
            .unwrap_or("llama-7b-q4");

        // Check if model is cached
        if self.model_cache.models.contains_key(model_id) {
            Ok(format!("LLM inference with model {}: {}", model_id, prompt))
        } else {
            Err(MLError::InferenceError(format!(
                "Model {} not loaded. Use load_model_p2p first.",
                model_id
            )))
        }
    }

    /// Vision inference (stub)
    pub fn run_vision(&self, _image: &[u8], params: &JsonValue) -> Result<Vec<(String, f32)>> {
        let model_id = params
            .get("model_id")
            .and_then(|v| v.as_str())
            .unwrap_or("clip-vit-base");

        if self.model_cache.models.contains_key(model_id) {
            Ok(vec![
                ("cat".to_string(), 0.95),
                ("dog".to_string(), 0.03),
                ("bird".to_string(), 0.02),
            ])
        } else {
            Err(MLError::InferenceError(format!(
                "Model {} not loaded",
                model_id
            )))
        }
    }

    /// Embedding inference (stub)
    pub fn run_embedding(&self, _text: &str, params: &JsonValue) -> Result<Vec<f32>> {
        let model_id = params
            .get("model_id")
            .and_then(|v| v.as_str())
            .unwrap_or("all-MiniLM-L6-v2");

        if self.model_cache.models.contains_key(model_id) {
            // Return dummy embedding
            Ok(vec![0.1; 384])
        } else {
            Err(MLError::InferenceError(format!(
                "Model {} not loaded",
                model_id
            )))
        }
    }

    /// Audio transcription (stub)
    pub fn run_audio(&self, _audio: &[f32], params: &JsonValue) -> Result<String> {
        let model_id = params
            .get("model_id")
            .and_then(|v| v.as_str())
            .unwrap_or("whisper-base");

        if self.model_cache.models.contains_key(model_id) {
            Ok("Transcribed text from audio".to_string())
        } else {
            Err(MLError::InferenceError(format!(
                "Model {} not loaded",
                model_id
            )))
        }
    }

    // ===== QUANTIZATION (LOCAL) =====

    /// Quantize to int8 (local, no downloads)
    pub fn quantize_int8(&self, input: &[f32]) -> Result<Vec<i8>> {
        let quantized = self.quantizer.quantize_int8(input)?;
        Ok(quantized.data)
    }

    /// Quantize to int4 (local, no downloads)
    pub fn quantize_int4(&self, input: &[f32]) -> Result<Vec<u8>> {
        self.quantizer.quantize_int4(input)
    }

    /// Dequantize (local)
    pub fn dequantize(&self, input: &[i8], scale: f32, zero_point: i32) -> Result<Vec<f32>> {
        let quantized = QuantizedModel {
            data: input.to_vec(),
            scale,
            zero_point,
            original_dtype: "float32".to_string(),
        };

        self.quantizer.dequantize(&quantized)
    }

    pub fn execute(&mut self, method: &str, input: &[u8], params_str: &str) -> Result<Vec<u8>> {
        let params: JsonValue =
            serde_json::from_str(params_str).map_err(|e| MLError::InvalidParams(e.to_string()))?;

        match method {
            "load_model_p2p" => {
                let model_id = params
                    .get("model_id")
                    .and_then(|v| v.as_str())
                    .ok_or_else(|| MLError::InvalidParams("Missing model_id".to_string()))?;
                let result = self.load_model_p2p(model_id, &params)?;
                Ok(result.into_bytes())
            }
            "run_llm" => {
                let prompt = std::str::from_utf8(input)
                    .map_err(|e| MLError::InvalidParams(format!("Invalid UTF-8: {}", e)))?;
                let result = self.run_llm(prompt, &params)?;
                Ok(result.into_bytes())
            }
            "run_vision" => {
                let result = self.run_vision(input, &params)?;
                Ok(serde_json::to_vec(&result)?)
            }
            "run_embedding" => {
                let text = std::str::from_utf8(input)
                    .map_err(|e| MLError::InvalidParams(format!("Invalid UTF-8: {}", e)))?;
                let result = self.run_embedding(text, &params)?;
                Ok(serde_json::to_vec(&result)?)
            }
            "run_audio" => {
                // Convert input to f32 slice
                let audio: Vec<f32> = input
                    .chunks_exact(4)
                    .map(|c| f32::from_le_bytes([c[0], c[1], c[2], c[3]]))
                    .collect();
                let result = self.run_audio(&audio, &params)?;
                Ok(result.into_bytes())
            }
            "quantize_int8" => {
                let input_f32: Vec<f32> = input
                    .chunks_exact(4)
                    .map(|c| f32::from_le_bytes([c[0], c[1], c[2], c[3]]))
                    .collect();
                let result = self.quantize_int8(&input_f32)?;
                // Convert i8 to u8 for transmission
                Ok(result.iter().map(|&x| x as u8).collect())
            }
            "quantize_int4" => {
                let input_f32: Vec<f32> = input
                    .chunks_exact(4)
                    .map(|c| f32::from_le_bytes([c[0], c[1], c[2], c[3]]))
                    .collect();
                let result = self.quantize_int4(&input_f32)?;
                Ok(result)
            }
            "dequantize" => {
                let scale = params
                    .get("scale")
                    .and_then(|v| v.as_f64())
                    .ok_or_else(|| MLError::InvalidParams("Missing scale".to_string()))?
                    as f32;
                let zero_point = params
                    .get("zero_point")
                    .and_then(|v| v.as_i64())
                    .ok_or_else(|| MLError::InvalidParams("Missing zero_point".to_string()))?
                    as i32;
                // input is i8 data
                let i8_data: Vec<i8> = input.iter().map(|&x| x as i8).collect();
                let result = self.dequantize(&i8_data, scale, zero_point)?;
                Ok(result.iter().flat_map(|&f| f.to_le_bytes()).collect())
            }
            "shard_model" => {
                let chunks = self.chunk_manager.chunk_model(input);
                Ok(serde_json::to_vec(&chunks)?)
            }
            "export_model" => {
                let model_id = params
                    .get("model_id")
                    .and_then(|v| v.as_str())
                    .ok_or_else(|| MLError::InvalidParams("Missing model_id".to_string()))?;

                let model = self.model_cache.get(model_id).ok_or_else(|| {
                    MLError::InferenceError(format!("Model {} not found", model_id))
                })?;

                let bytes = self
                    .chunk_manager
                    .reconstruct_model(&model.chunks)
                    .map_err(|e| MLError::InferenceError(e.to_string()))?;
                Ok(bytes)
            }
            "get_model_metadata" => {
                let model_id = params
                    .get("model_id")
                    .and_then(|v| v.as_str())
                    .ok_or_else(|| MLError::InvalidParams("Missing model_id".to_string()))?;

                let model = self.model_cache.get(model_id).ok_or_else(|| {
                    MLError::InferenceError(format!("Model {} not found", model_id))
                })?;

                Ok(serde_json::to_vec(&model.metadata)?)
            }
            _ => Err(MLError::UnknownMethod {
                library: "inference".to_string(),
                method: method.to_string(),
            }),
        }
    }

    pub fn methods(&self) -> Vec<&'static str> {
        vec![
            // Model loading (2)
            "load_model_p2p",
            "store_model_chunks",
            "shard_model",
            "export_model",
            "get_model_metadata",
            // Inference (4)
            "run_llm",
            "run_vision",
            "run_embedding",
            "run_audio",
            // Quantization (3)
            "quantize_int8",
            "quantize_int4",
            "dequantize",
        ]
    }
}

/// P2P model request (sent to kernel)
#[derive(Serialize, Deserialize)]
struct P2PModelRequest {
    model_id: String,
    chunk_size: usize,
    max_concurrent: usize,
    progressive: bool,
}
