use crate::engine::{MLError, Result};
use serde::{Deserialize, Serialize};

/// Production LLM model with P2P loading
///
/// Architecture: INOS-native
/// - P2P model distribution (1MB chunks via storage mesh)
/// - Streaming generation (token-by-token)
/// - KV cache with sliding window
/// - Multiple architectures (Llama, Mistral, GPT, Phi, Gemma)
/// - Quantization support (int8/int4, GGUF, GPTQ, AWQ)
pub struct LLMModel {
    model_id: Option<String>,
    architecture: ModelArchitecture,
    quantization: QuantizationType,
    kv_cache: Option<KVCache>,
    context_size: usize,
    config: LLMConfig,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub enum ModelArchitecture {
    Llama,
    Mistral,
    GPT2,
    Phi,
    Gemma,
    Qwen,
    Unknown(String),
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub enum QuantizationType {
    Float32,
    Float16,
    BFloat16,
    Int8,
    Int4,
    Q4_0, // GGUF quantization
    Q8_0, // GGUF quantization
    GPTQ,
    AWQ,
}

#[derive(Clone, Debug)]
pub struct KVCache {
    pub keys: Vec<Vec<f32>>,
    pub values: Vec<Vec<f32>>,
    pub max_seq_len: usize,
    pub current_len: usize,
    pub use_sliding_window: bool,
    pub window_size: usize,
}

impl KVCache {
    fn new(max_seq_len: usize, window_size: Option<usize>) -> Self {
        let use_sliding_window = window_size.is_some();
        Self {
            keys: Vec::new(),
            values: Vec::new(),
            max_seq_len,
            current_len: 0,
            use_sliding_window,
            window_size: window_size.unwrap_or(max_seq_len),
        }
    }

    #[allow(dead_code)]
    fn clear(&mut self) {
        self.keys.clear();
        self.values.clear();
        self.current_len = 0;
    }
}

#[derive(Clone, Debug)]
pub struct LLMConfig {
    pub rope_base: f32,
    pub rope_scaling: f32,
    pub use_flash_attention: bool,
}

impl Default for LLMConfig {
    fn default() -> Self {
        Self {
            rope_base: 10000.0,
            rope_scaling: 1.0,
            use_flash_attention: false,
        }
    }
}

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct GenerationConfig {
    pub max_tokens: usize,
    pub temperature: f32,
    pub top_p: f32,
    pub top_k: usize,
    pub repetition_penalty: f32,
    pub stop_tokens: Vec<String>,
    pub stream: bool,
}

impl Default for GenerationConfig {
    fn default() -> Self {
        Self {
            max_tokens: 512,
            temperature: 0.7,
            top_p: 0.9,
            top_k: 50,
            repetition_penalty: 1.1,
            stop_tokens: vec![],
            stream: false,
        }
    }
}

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct ChatMessage {
    pub role: ChatRole,
    pub content: String,
}

#[derive(Serialize, Deserialize, Clone, Debug)]
pub enum ChatRole {
    System,
    User,
    Assistant,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct GenerationResult {
    pub text: String,
    pub finish_reason: FinishReason,
    pub tokens_generated: usize,
    pub generation_time_ms: f32,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub enum FinishReason {
    StopToken,
    MaxTokens,
    EndOfText,
}

impl LLMModel {
    pub fn new() -> Result<Self> {
        Ok(Self {
            model_id: None,
            architecture: ModelArchitecture::Unknown("".to_string()),
            quantization: QuantizationType::Float32,
            kv_cache: None,
            context_size: 2048,
            config: LLMConfig::default(),
        })
    }

    /// Load model from P2P mesh (via chunks)
    pub fn load_from_chunks(&mut self, chunks: Vec<crate::p2p::Chunk>) -> Result<()> {
        if chunks.is_empty() {
            return Err(MLError::InferenceError("No chunks provided".to_string()));
        }

        // 1. Sort chunks by index
        let mut sorted_chunks = chunks;
        sorted_chunks.sort_by_key(|c| c.index);

        // 2. Verify completeness
        for (i, chunk) in sorted_chunks.iter().enumerate() {
            if chunk.index != i {
                return Err(MLError::InferenceError(format!(
                    "Missing chunk index {}",
                    i
                )));
            }
        }

        // 3. Concatenate data
        let total_size: usize = sorted_chunks.iter().map(|c| c.data.len()).sum();
        let mut model_data = Vec::with_capacity(total_size);
        for chunk in &sorted_chunks {
            model_data.extend_from_slice(&chunk.data);
        }

        // 4. Initialize from binary data
        let model_id = sorted_chunks[0].model_id.clone();
        self.initialize_from_data(&model_id, &model_data)?;

        self.model_id = Some(model_id);

        Ok(())
    }

    fn initialize_from_data(&mut self, model_id: &str, _data: &[u8]) -> Result<()> {
        // Parse metadata
        self.architecture = Self::parse_architecture(model_id);
        self.quantization = Self::parse_quantization(model_id);
        self.context_size = Self::extract_context_size(model_id);
        self.config = self.get_config_for_architecture();

        // Initialize KV cache
        let window_size = if model_id.contains("sliding") {
            Some(self.context_size / 2)
        } else {
            None
        };
        self.kv_cache = Some(KVCache::new(self.context_size, window_size));

        Ok(())
    }

    /// Load model from P2P mesh (legacy)
    pub fn load_model(&mut self, model_id: &str) -> Result<()> {
        self.initialize_from_data(model_id, &[])?;
        self.model_id = Some(model_id.to_string());
        Ok(())
    }

    fn parse_architecture(model_id: &str) -> ModelArchitecture {
        let id_lower = model_id.to_lowercase();
        if id_lower.contains("llama") {
            ModelArchitecture::Llama
        } else if id_lower.contains("mistral") {
            ModelArchitecture::Mistral
        } else if id_lower.contains("gpt") {
            ModelArchitecture::GPT2
        } else if id_lower.contains("phi") {
            ModelArchitecture::Phi
        } else if id_lower.contains("gemma") {
            ModelArchitecture::Gemma
        } else if id_lower.contains("qwen") {
            ModelArchitecture::Qwen
        } else {
            ModelArchitecture::Unknown(model_id.to_string())
        }
    }

    fn parse_quantization(model_id: &str) -> QuantizationType {
        let id_lower = model_id.to_lowercase();
        if id_lower.contains("q4_0") || id_lower.contains("q4-0") {
            QuantizationType::Q4_0
        } else if id_lower.contains("q8_0") || id_lower.contains("q8-0") {
            QuantizationType::Q8_0
        } else if id_lower.contains("int8") {
            QuantizationType::Int8
        } else if id_lower.contains("int4") {
            QuantizationType::Int4
        } else if id_lower.contains("gptq") {
            QuantizationType::GPTQ
        } else if id_lower.contains("awq") {
            QuantizationType::AWQ
        } else if id_lower.contains("bf16") {
            QuantizationType::BFloat16
        } else if id_lower.contains("f16") {
            QuantizationType::Float16
        } else {
            QuantizationType::Float32
        }
    }

    fn extract_context_size(model_id: &str) -> usize {
        let patterns = [
            ("64k", 65536),
            ("32k", 32768),
            ("16k", 16384),
            ("8k", 8192),
            ("4k", 4096),
            ("2k", 2048),
        ];

        for (pattern, size) in patterns.iter() {
            if model_id.contains(pattern) {
                return *size;
            }
        }
        2048 // Default
    }

    fn get_config_for_architecture(&self) -> LLMConfig {
        match self.architecture {
            ModelArchitecture::Llama => LLMConfig {
                rope_base: 10000.0,
                rope_scaling: 1.0,
                use_flash_attention: false,
            },
            ModelArchitecture::Mistral => LLMConfig {
                rope_base: 10000.0,
                rope_scaling: 8.0,
                use_flash_attention: true,
            },
            _ => LLMConfig::default(),
        }
    }

    /// Generate text (stub - delegates to InferenceJob)
    pub fn generate(
        &mut self,
        messages: &[ChatMessage],
        config: &GenerationConfig,
    ) -> Result<GenerationResult> {
        if self.model_id.is_none() {
            return Err(MLError::InferenceError("Model not loaded".to_string()));
        }

        // Format chat messages
        let _prompt = self.format_chat_prompt(messages);

        // Stub implementation
        Ok(GenerationResult {
            text: format!("Generated response ({} tokens)", config.max_tokens),
            finish_reason: FinishReason::MaxTokens,
            tokens_generated: config.max_tokens,
            generation_time_ms: 500.0,
        })
    }

    /// Stream tokens (stub - returns iterator)
    pub fn generate_stream(
        &mut self,
        _messages: &[ChatMessage],
        _config: &GenerationConfig,
    ) -> Result<Vec<String>> {
        if self.model_id.is_none() {
            return Err(MLError::InferenceError("Model not loaded".to_string()));
        }

        // Stub: return tokens as if streaming
        Ok(vec![
            "Hello".to_string(),
            " there".to_string(),
            "!".to_string(),
        ])
    }

    fn format_chat_prompt(&self, messages: &[ChatMessage]) -> String {
        let mut prompt = String::new();

        for message in messages {
            match message.role {
                ChatRole::System => {
                    prompt.push_str(&format!("System: {}\n", message.content));
                }
                ChatRole::User => {
                    prompt.push_str(&format!("User: {}\n", message.content));
                }
                ChatRole::Assistant => {
                    prompt.push_str(&format!("Assistant: {}\n", message.content));
                }
            }
        }

        prompt.push_str("Assistant: ");
        prompt
    }
}

/// Builder pattern for LLM configuration
pub struct LLMBuilder {
    model: LLMModel,
}

impl Default for LLMBuilder {
    fn default() -> Self {
        Self {
            model: LLMModel::new().unwrap(),
        }
    }
}

impl LLMBuilder {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn with_context_size(mut self, size: usize) -> Self {
        self.model.context_size = size;
        self
    }

    pub fn with_flash_attention(mut self, enable: bool) -> Self {
        self.model.config.use_flash_attention = enable;
        self
    }

    pub fn build(self) -> LLMModel {
        self.model
    }
}
