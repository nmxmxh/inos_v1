use blake3::Hasher;
use dashmap::DashMap;
use futures::stream::{Stream, StreamExt};
use serde::{Deserialize, Serialize};
use std::sync::Arc;

// Error Types
#[derive(Debug, thiserror::Error)]
pub enum HashingError {
    #[error("Hash verification failed: expected {expected}, got {actual}")]
    VerificationFailed { expected: String, actual: String },

    #[error("Data too large: {size} bytes exceeds limit {limit}")]
    DataTooLarge { size: usize, limit: usize },

    #[error("Invalid hash format: {0}")]
    InvalidHashFormat(String),

    #[error("IO error: {0}")]
    IoError(#[from] std::io::Error),

    #[error("Async hashing error: {0}")]
    AsyncError(String),
}

/// Generate BLAKE3 hash of data (Simple API)
pub fn hash_data(data: &[u8]) -> String {
    let mut hasher = Hasher::new();
    hasher.update(data);
    hasher.finalize().to_hex().to_string()
}

/// Incremental hasher for streaming large data
pub struct StreamingHasher {
    hasher: Hasher,
    total_bytes: u64,
}

impl Default for StreamingHasher {
    fn default() -> Self {
        Self::new()
    }
}

impl StreamingHasher {
    pub fn new() -> Self {
        Self {
            hasher: Hasher::new(),
            total_bytes: 0,
        }
    }

    /// Update hash with more data
    pub fn update(&mut self, data: &[u8]) {
        self.hasher.update(data);
        self.total_bytes += data.len() as u64;
    }

    /// Finalize and get hash
    pub fn finalize(&self) -> String {
        self.hasher.finalize().to_hex().to_string()
    }

    /// Get total bytes processed
    pub fn total_bytes(&self) -> u64 {
        self.total_bytes
    }

    /// Hash a stream asynchronously
    pub async fn hash_stream<S>(mut stream: S) -> Result<String, String>
    where
        S: Stream<Item = Result<Vec<u8>, std::io::Error>> + Unpin,
    {
        let mut hasher = Hasher::new();
        let mut total_bytes = 0;

        while let Some(chunk) = stream.next().await {
            let data = chunk.map_err(|e| format!("Stream error: {}", e))?;
            hasher.update(&data);
            total_bytes += data.len();
        }

        // Use log crate only if available/configured, otherwise print or ignore for generic consistency
        log::debug!("Hashed {} bytes from stream", total_bytes);
        Ok(hasher.finalize().to_hex().to_string())
    }
}

/// Salted hashing for preventing rainbow table attacks
pub fn hash_with_salt(data: &[u8], salt: &[u8]) -> String {
    let mut hasher = Hasher::new();
    hasher.update(salt);
    hasher.update(data);
    hasher.finalize().to_hex().to_string()
}

/// Keyed hashing for authenticated data (HMAC equivalent with BLAKE3)
pub fn hash_with_key(data: &[u8], key: &[u8; 32]) -> String {
    let mut hasher = Hasher::new_keyed(key);
    hasher.update(data);
    hasher.finalize().to_hex().to_string()
}

/// Merkle Proof Structure
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MerkleProof {
    pub leaf_index: usize,
    pub leaf_hash: String,
    pub sibling_hashes: Vec<String>,
    pub root_hash: String,
}

/// Tree Hasher for Large Files (Merkle Tree)
pub struct TreeHasher {
    #[allow(dead_code)]
    chunk_size: usize,
    hashes: Vec<String>,
}

impl TreeHasher {
    pub fn new(chunk_size: usize) -> Self {
        Self {
            chunk_size,
            hashes: Vec::new(),
        }
    }

    /// Add a chunk to the tree
    pub fn add_chunk(&mut self, data: &[u8]) {
        let hash = hash_data(data);
        self.hashes.push(hash);
    }

    /// Build merkle tree and return root hash
    pub fn build_tree(&self) -> String {
        if self.hashes.is_empty() {
            return hash_data(&[]);
        }

        let mut current_level = self.hashes.clone();

        while current_level.len() > 1 {
            let mut next_level = Vec::new();

            for pair in current_level.chunks(2) {
                let combined = if pair.len() == 2 {
                    format!("{}{}", pair[0], pair[1])
                } else {
                    // Odd number, duplicate the last one
                    format!("{}{}", pair[0], pair[0])
                };

                let hash = hash_data(combined.as_bytes());
                next_level.push(hash);
            }

            current_level = next_level;
        }

        current_level[0].clone()
    }

    fn compute_parent_level(&self, level: &[String]) -> Vec<String> {
        let mut next = Vec::new();
        for pair in level.chunks(2) {
            let combined = if pair.len() == 2 {
                format!("{}{}", pair[0], pair[1])
            } else {
                format!("{}{}", pair[0], pair[0])
            };
            next.push(hash_data(combined.as_bytes()));
        }
        next
    }

    /// Generate inclusion proof for a specific chunk
    pub fn generate_proof(&self, chunk_index: usize) -> Option<MerkleProof> {
        if chunk_index >= self.hashes.len() {
            return None;
        }

        let mut proof = Vec::new();
        let mut current_index = chunk_index;
        let mut current_level = self.hashes.clone();

        while current_level.len() > 1 {
            let sibling_index = if current_index.is_multiple_of(2) {
                // Right sibling
                if current_index + 1 < current_level.len() {
                    Some(current_index + 1)
                } else {
                    None
                }
            } else {
                // Left sibling
                Some(current_index - 1)
            };

            if let Some(sib_idx) = sibling_index {
                proof.push(current_level[sib_idx].clone());
            }

            // Move to parent level
            current_index /= 2;
            current_level = self.compute_parent_level(&current_level);
        }

        let root = if current_level.is_empty() {
            // Should not happen if loop logic is correct
            String::new()
        } else {
            current_level[0].clone()
        };

        Some(MerkleProof {
            leaf_index: chunk_index,
            leaf_hash: self.hashes[chunk_index].clone(),
            sibling_hashes: proof,
            root_hash: root,
        })
    }
}

/// Production Hasher with Strategy Selection
#[derive(Clone, Debug)]
pub struct HashingConfig {
    pub chunk_size: usize,
    pub use_parallel: bool,
    pub enable_progress: bool,
    pub hash_cache_size: usize,
    pub verify_immediately: bool,
}

pub struct ProductionHasher {
    pub config: HashingConfig,
    streaming_cache: Arc<DashMap<String, StreamingHasher>>,
}

impl ProductionHasher {
    pub fn new(config: HashingConfig) -> Self {
        Self {
            config,
            streaming_cache: Arc::new(DashMap::new()),
        }
    }

    /// Hash data with automatic strategy selection
    pub fn hash_auto(&self, data: &[u8], _context: &str) -> String {
        // Simple synchronous hash for now.
        // Can add parallel logic here when rayon/wasm-bindgen-rayon is fully setup.
        hash_data(data)
    }

    /// Start streaming hash with ID
    pub fn start_streaming_hash(&self, stream_id: &str) {
        let hasher = StreamingHasher::new();
        self.streaming_cache.insert(stream_id.to_string(), hasher);
    }

    /// Update streaming hash
    pub fn update_streaming_hash(&self, stream_id: &str, data: &[u8]) -> Result<(), HashingError> {
        let mut entry = self
            .streaming_cache
            .get_mut(stream_id)
            .ok_or_else(|| HashingError::AsyncError("Stream not found".to_string()))?;

        entry.update(data);
        Ok(())
    }

    /// Finalize streaming hash
    pub fn finalize_streaming_hash(&self, stream_id: &str) -> Result<String, HashingError> {
        let entry = self
            .streaming_cache
            .remove(stream_id)
            .ok_or_else(|| HashingError::AsyncError("Stream not found".to_string()))?;

        Ok(entry.1.finalize()) // Remove returns (Key, Value)
    }

    /// Verify with detailed error
    pub fn verify(&self, data: &[u8], expected_hash: &str) -> Result<(), HashingError> {
        if expected_hash.len() != 64 {
            return Err(HashingError::InvalidHashFormat(format!(
                "Expected 64 characters, got {}",
                expected_hash.len()
            )));
        }

        let actual = self.hash_auto(data, "verify");
        if actual == expected_hash {
            Ok(())
        } else {
            Err(HashingError::VerificationFailed {
                expected: expected_hash.to_string(),
                actual,
            })
        }
    }
}
