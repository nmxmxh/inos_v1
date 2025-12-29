use serde::{Deserialize, Serialize};
use std::time::Duration;
use thiserror::Error;

/// Additional context for errors to aid debugging and telemetry
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ErrorContext {
    pub peer_id: Option<String>,
    pub chunk_id: Option<String>,
    pub model_id: Option<String>,
    pub operation: String,
    pub timestamp: u64,
    pub retry_count: u32,
}

impl Default for ErrorContext {
    fn default() -> Self {
        Self {
            peer_id: None,
            chunk_id: None,
            model_id: None,
            operation: String::new(),
            timestamp: current_timestamp(),
            retry_count: 0,
        }
    }
}

impl ErrorContext {
    pub fn new(operation: &str) -> Self {
        Self {
            operation: operation.to_string(),
            ..Default::default()
        }
    }

    pub fn with_peer_id(mut self, peer_id: &str) -> Self {
        self.peer_id = Some(peer_id.to_string());
        self
    }

    pub fn with_chunk_id(mut self, chunk_id: &str) -> Self {
        self.chunk_id = Some(chunk_id.to_string());
        self
    }

    pub fn with_model_id(mut self, model_id: &str) -> Self {
        self.model_id = Some(model_id.to_string());
        self
    }
}

/// Get current timestamp (WASM-compatible)
fn current_timestamp() -> u64 {
    #[cfg(target_arch = "wasm32")]
    {
        (js_sys::Date::now() / 1000.0) as u64
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs()
    }
}

/// Unified error type for P2P operations with structured context
#[derive(Debug, Error)]
pub enum P2pError {
    #[error("Chunk not found: {chunk_id}")]
    ChunkNotFound {
        chunk_id: String,
        context: ErrorContext,
    },

    #[error("Hash mismatch: expected {expected}, got {actual} (chunk: {chunk_id})")]
    HashMismatch {
        expected: String,
        actual: String,
        chunk_id: String,
    },

    #[error("Storage error: {message}")]
    Storage {
        message: String,
        context: ErrorContext,
    },

    #[error("Network error: {message}")]
    Network {
        message: String,
        peer_id: Option<String>,
        context: ErrorContext,
    },

    #[error("Peer timeout after {duration:?}: {peer_id}")]
    PeerTimeout {
        peer_id: String,
        duration: Duration,
        context: ErrorContext,
    },

    #[error("Assembly failed for model {model_id}: {reason}")]
    AssemblyFailed {
        model_id: String,
        reason: String,
        missing_chunks: Vec<String>,
    },

    #[error("Rate limit exceeded for peer {peer_id}: retry after {retry_after:?}")]
    RateLimitExceeded {
        peer_id: String,
        retry_after: Duration,
        context: ErrorContext,
    },

    #[error("Invalid chunk data for {chunk_id}: {reason}")]
    InvalidChunkData { chunk_id: String, reason: String },

    #[error("Serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    #[error("Serialization error")]
    SerializationError {
        context: ErrorContext,
        source: Option<Box<dyn std::error::Error + Send + Sync>>,
    },

    #[error("Deserialization error")]
    DeserializationError {
        context: ErrorContext,
        source: Option<Box<dyn std::error::Error + Send + Sync>>,
    },

    #[error("Model not found: {model_id}")]
    ModelNotFound {
        model_id: String,
        available_models: Vec<String>,
        context: ErrorContext,
    },

    #[error("Peer {peer_id} not available: {reason}")]
    PeerNotAvailable {
        peer_id: String,
        reason: String,
        reputation: f32,
        context: ErrorContext,
    },

    #[error("Verification failed for {chunk_id}: {reason}")]
    VerificationFailed {
        chunk_id: String,
        reason: String,
        proof_type: String,
    },

    #[error("Insufficient resources: {resource} (required: {required}, available: {available})")]
    InsufficientResources {
        resource: String,
        required: u64,
        available: u64,
        context: ErrorContext,
    },

    #[error("Configuration error: {field} = {value} is invalid (expected: {expected})")]
    ConfigurationError {
        field: String,
        value: String,
        expected: String,
    },

    #[error("Operation cancelled: {operation} - {reason}")]
    Cancelled {
        operation: String,
        reason: String,
        context: ErrorContext,
    },

    #[error("Peer reputation too low: {peer_id} (score: {score}, threshold: {threshold})")]
    LowPeerReputation {
        peer_id: String,
        score: f32,
        threshold: f32,
        context: ErrorContext,
    },

    #[error("Invalid state: expected {expected}, got {actual}")]
    InvalidState {
        expected: String,
        actual: String,
        context: ErrorContext,
    },

    #[error("Recoverable error: {message} (retry: {retry_count}/{max_retries})")]
    Recoverable {
        message: String,
        retry_count: u32,
        max_retries: u32,
    },

    #[error("Fatal error: {message}")]
    Fatal { message: String },
}

impl P2pError {
    /// Check if the error is recoverable (can be retried)
    pub fn is_recoverable(&self) -> bool {
        matches!(
            self,
            P2pError::Network { .. }
                | P2pError::PeerTimeout { .. }
                | P2pError::RateLimitExceeded { .. }
                | P2pError::PeerNotAvailable { .. }
                | P2pError::Recoverable { .. }
        )
    }

    /// Check if the error is fatal (should stop operations)
    pub fn is_fatal(&self) -> bool {
        matches!(self, P2pError::Fatal { .. })
    }

    /// Get retry delay for rate-limited errors
    pub fn retry_after(&self) -> Option<Duration> {
        match self {
            P2pError::RateLimitExceeded { retry_after, .. } => Some(*retry_after),
            P2pError::PeerTimeout { duration, .. } => Some(*duration),
            _ => None,
        }
    }

    /// Get error severity level for logging
    pub fn severity(&self) -> &'static str {
        match self {
            P2pError::Fatal { .. } => "error",
            P2pError::HashMismatch { .. }
            | P2pError::InvalidChunkData { .. }
            | P2pError::VerificationFailed { .. } => "warn",
            P2pError::Recoverable { .. }
            | P2pError::RateLimitExceeded { .. }
            | P2pError::PeerTimeout { .. } => "info",
            P2pError::SerializationError { .. } | P2pError::DeserializationError { .. } => "error",
            _ => "debug",
        }
    }

    /// Get the operation name from context
    pub fn operation(&self) -> Option<&str> {
        match self {
            P2pError::ChunkNotFound { context, .. }
            | P2pError::Storage { context, .. }
            | P2pError::Network { context, .. }
            | P2pError::PeerTimeout { context, .. }
            | P2pError::RateLimitExceeded { context, .. }
            | P2pError::ModelNotFound { context, .. }
            | P2pError::PeerNotAvailable { context, .. }
            | P2pError::InsufficientResources { context, .. }
            | P2pError::Cancelled { context, .. }
            | P2pError::LowPeerReputation { context, .. }
            | P2pError::InvalidState { context, .. }
            | P2pError::SerializationError { context, .. }
            | P2pError::DeserializationError { context, .. } => Some(&context.operation),
            _ => None,
        }
    }

    /// Add context to an error
    pub fn with_context(self, context: ErrorContext) -> Self {
        match self {
            P2pError::ChunkNotFound { chunk_id, .. } => {
                P2pError::ChunkNotFound { chunk_id, context }
            }
            P2pError::Storage { message, .. } => P2pError::Storage { message, context },
            P2pError::Network {
                message, peer_id, ..
            } => P2pError::Network {
                message,
                peer_id,
                context,
            },
            _ => self,
        }
    }

    /// Convert error to metrics tags for observability
    pub fn to_metrics_tags(&self) -> Vec<(&'static str, String)> {
        let mut tags = Vec::new();

        match self {
            P2pError::ChunkNotFound { chunk_id, context } => {
                tags.push(("error_type", "chunk_not_found".to_string()));
                tags.push(("chunk_id", chunk_id.clone()));
                tags.push(("operation", context.operation.clone()));
            }
            P2pError::HashMismatch { chunk_id, .. } => {
                tags.push(("error_type", "hash_mismatch".to_string()));
                tags.push(("chunk_id", chunk_id.clone()));
            }
            P2pError::Network {
                peer_id, context, ..
            } => {
                tags.push(("error_type", "network".to_string()));
                if let Some(pid) = peer_id {
                    tags.push(("peer_id", pid.clone()));
                }
                tags.push(("operation", context.operation.clone()));
            }
            P2pError::SerializationError { context, .. }
            | P2pError::DeserializationError { context, .. } => {
                tags.push(("error_type", "serialization".to_string()));
                tags.push(("operation", context.operation.clone()));
            }
            _ => {
                tags.push(("error_type", "other".to_string()));
            }
        }

        tags
    }
}

/// WASM interop: Convert from JsValue
#[cfg(target_arch = "wasm32")]
impl From<js_sys::Object> for P2pError {
    fn from(js_val: js_sys::Object) -> Self {
        // Try to convert to string
        let message = js_sys::JSON::stringify(&js_val)
            .ok()
            .and_then(|s| s.as_string())
            .unwrap_or_else(|| "Unknown JS error".to_string());

        P2pError::Storage {
            message,
            context: ErrorContext::default(),
        }
    }
}

/// Result type for P2P operations
pub type Result<T> = std::result::Result<T, P2pError>;

/// Extension trait for adding context to Results
pub trait ResultExt<T> {
    fn with_context(self, operation: &str) -> Result<T>;
    fn with_peer_context(self, operation: &str, peer_id: &str) -> Result<T>;
    fn with_chunk_context(self, operation: &str, chunk_id: &str) -> Result<T>;
}

impl<T, E: Into<P2pError>> ResultExt<T> for std::result::Result<T, E> {
    fn with_context(self, operation: &str) -> Result<T> {
        self.map_err(|e| {
            let err: P2pError = e.into();
            err.with_context(ErrorContext {
                operation: operation.to_string(),
                ..ErrorContext::default()
            })
        })
    }

    fn with_peer_context(self, operation: &str, peer_id: &str) -> Result<T> {
        self.map_err(|e| {
            let err: P2pError = e.into();
            err.with_context(ErrorContext {
                operation: operation.to_string(),
                peer_id: Some(peer_id.to_string()),
                ..ErrorContext::default()
            })
        })
    }

    fn with_chunk_context(self, operation: &str, chunk_id: &str) -> Result<T> {
        self.map_err(|e| {
            let err: P2pError = e.into();
            err.with_context(ErrorContext {
                operation: operation.to_string(),
                chunk_id: Some(chunk_id.to_string()),
                ..ErrorContext::default()
            })
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_error_is_recoverable() {
        let err = P2pError::Network {
            message: "timeout".to_string(),
            peer_id: None,
            context: ErrorContext::default(),
        };
        assert!(err.is_recoverable());

        let fatal = P2pError::Fatal {
            message: "critical".to_string(),
        };
        assert!(!fatal.is_recoverable());
    }

    #[test]
    fn test_error_severity() {
        let fatal = P2pError::Fatal {
            message: "test".to_string(),
        };
        assert_eq!(fatal.severity(), "error");

        let recoverable = P2pError::Recoverable {
            message: "test".to_string(),
            retry_count: 1,
            max_retries: 3,
        };
        assert_eq!(recoverable.severity(), "info");
    }

    #[test]
    fn test_error_context() {
        let err = P2pError::ChunkNotFound {
            chunk_id: "test-chunk".to_string(),
            context: ErrorContext {
                operation: "load_chunk".to_string(),
                ..ErrorContext::default()
            },
        };
        assert_eq!(err.operation(), Some("load_chunk"));
    }
}
