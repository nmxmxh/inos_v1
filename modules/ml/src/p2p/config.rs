use serde::{Deserialize, Serialize};

/// Configuration for P2P operations with validation and adaptive settings
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct P2pConfig {
    /// Chunk size in bytes (default: 1MB, range: 1KB-10MB)
    pub chunk_size: usize,

    /// Maximum cache size in bytes (default: 100MB)
    pub max_cache_size: usize,

    /// Request timeout in milliseconds (default: 5000ms)
    pub request_timeout_ms: u64,

    /// Number of retry attempts (default: 3)
    pub retry_attempts: u32,

    /// Enable prefetching (default: true)
    pub prefetch_enabled: bool,

    /// Maximum parallel chunk downloads (default: 4)
    pub parallel_chunk_limit: usize,

    /// Enable Brotli compression (default: true)
    pub compression_enabled: bool,

    /// Reputation decay factor (default: 0.95, range: 0-1)
    pub reputation_decay: f32,

    /// Minimum reputation score to trust peer (default: 0.5, range: 0-1)
    pub min_reputation: f32,

    /// Enable adaptive chunk size based on network conditions
    pub adaptive_chunk_size: bool,

    /// Minimum chunk size when adaptive is enabled (default: 256KB)
    pub min_chunk_size: usize,

    /// Maximum chunk size when adaptive is enabled (default: 5MB)
    pub max_chunk_size: usize,

    /// Number of pipeline stages for distributed inference (default: 4)
    pub pipeline_depth: usize,

    /// Path for persistent cache storage (optional)
    pub persistence_path: Option<String>,
}

impl Default for P2pConfig {
    fn default() -> Self {
        Self {
            chunk_size: 1024 * 1024,           // 1MB
            max_cache_size: 100 * 1024 * 1024, // 100MB
            request_timeout_ms: 5000,          // 5s
            retry_attempts: 3,
            prefetch_enabled: true,
            parallel_chunk_limit: 4,
            compression_enabled: true,
            reputation_decay: 0.95,
            min_reputation: 0.5,
            adaptive_chunk_size: false,
            min_chunk_size: 256 * 1024,      // 256KB
            max_chunk_size: 5 * 1024 * 1024, // 5MB
            pipeline_depth: 4,
            persistence_path: None,
        }
    }
}

impl P2pConfig {
    /// Validate configuration parameters
    pub fn validate(&self) -> Result<(), String> {
        if self.chunk_size == 0 {
            return Err("chunk_size cannot be 0".to_string());
        }
        if self.chunk_size < 1024 || self.chunk_size > 10 * 1024 * 1024 {
            return Err("chunk_size must be between 1KB and 10MB".to_string());
        }
        if self.min_reputation > 1.0 || self.min_reputation < 0.0 {
            return Err("min_reputation must be between 0 and 1".to_string());
        }
        if self.reputation_decay > 1.0 || self.reputation_decay < 0.0 {
            return Err("reputation_decay must be between 0 and 1".to_string());
        }
        if self.parallel_chunk_limit == 0 {
            return Err("parallel_chunk_limit cannot be 0".to_string());
        }
        if self.adaptive_chunk_size && self.min_chunk_size > self.max_chunk_size {
            return Err("min_chunk_size cannot be greater than max_chunk_size".to_string());
        }
        Ok(())
    }

    /// Create a config optimized for low bandwidth
    pub fn low_bandwidth() -> Self {
        Self {
            chunk_size: 512 * 1024, // 512KB
            parallel_chunk_limit: 2,
            compression_enabled: true,
            prefetch_enabled: false,
            request_timeout_ms: 10000, // 10s
            adaptive_chunk_size: true,
            ..Default::default()
        }
    }

    /// Create a config optimized for high performance
    pub fn high_performance() -> Self {
        Self {
            chunk_size: 2 * 1024 * 1024, // 2MB
            parallel_chunk_limit: 8,
            prefetch_enabled: true,
            max_cache_size: 500 * 1024 * 1024, // 500MB
            request_timeout_ms: 3000,          // 3s
            ..Default::default()
        }
    }

    /// Preset for mobile devices
    pub fn mobile() -> Self {
        Self {
            chunk_size: 512 * 1024,           // 512KB
            max_cache_size: 50 * 1024 * 1024, // 50MB
            parallel_chunk_limit: 2,
            request_timeout_ms: 10000, // 10s for mobile networks
            adaptive_chunk_size: true,
            min_chunk_size: 256 * 1024,
            max_chunk_size: 1024 * 1024,
            ..Default::default()
        }
    }

    /// Preset for desktop with good connectivity
    pub fn desktop() -> Self {
        Self {
            chunk_size: 2 * 1024 * 1024,        // 2MB
            max_cache_size: 1024 * 1024 * 1024, // 1GB
            parallel_chunk_limit: 6,
            request_timeout_ms: 3000, // 3s
            ..Default::default()
        }
    }

    /// Dynamically adjust configuration based on network conditions
    pub fn adjust_for_conditions(&mut self, bandwidth_kbps: f32, latency_ms: f32) {
        if self.adaptive_chunk_size {
            // Adjust chunk size based on bandwidth (use 10% of bandwidth per chunk)
            let target_chunk_kb = (bandwidth_kbps * 0.1).clamp(50.0, 5000.0);
            self.chunk_size = (target_chunk_kb * 1024.0) as usize;
            self.chunk_size = self
                .chunk_size
                .clamp(self.min_chunk_size, self.max_chunk_size);
        }

        // Adjust parallelism based on bandwidth
        self.parallel_chunk_limit = if bandwidth_kbps < 1000.0 {
            2
        } else if bandwidth_kbps < 5000.0 {
            4
        } else {
            8
        };

        // Adjust timeout based on latency
        self.request_timeout_ms = (2000.0 + (latency_ms * 3.0)) as u64;
    }

    /// Calculate optimal batch size based on memory constraints
    pub fn batch_size_for_model(&self, model_size_bytes: usize) -> usize {
        let available_memory = self.max_cache_size as f32 * 0.7; // Use 70% of cache
        let chunks_needed = (model_size_bytes / self.chunk_size).max(1);

        // Don't exceed available memory or parallel limit
        let max_by_memory = (available_memory / self.chunk_size as f32) as usize;
        let max_by_parallel = self.parallel_chunk_limit;

        chunks_needed.min(max_by_memory).min(max_by_parallel)
    }

    /// Check if a peer is considered trusted
    pub fn is_peer_trusted(&self, reputation_score: f32) -> bool {
        reputation_score >= self.min_reputation
    }

    /// Apply reputation decay to a score
    pub fn apply_reputation_decay(&self, score: f32) -> f32 {
        score * self.reputation_decay
    }

    /// Load configuration from JSON
    pub fn from_json(json: &str) -> Result<Self, serde_json::Error> {
        serde_json::from_str(json)
    }

    /// Load from browser localStorage (WASM only)
    #[cfg(target_arch = "wasm32")]
    pub fn from_local_storage() -> Result<Self, String> {
        use web_sys::window;

        let window = window().ok_or("No window")?;
        let storage = window
            .local_storage()
            .map_err(|e| format!("{:?}", e))?
            .ok_or("No localStorage")?;

        let json = storage
            .get_item("p2p_config")
            .map_err(|e| format!("{:?}", e))?
            .unwrap_or_else(|| "{}".to_string());

        let config: Self = serde_json::from_str(&json).map_err(|e| e.to_string())?;

        Ok(config)
    }

    /// Save to browser localStorage (WASM only)
    #[cfg(target_arch = "wasm32")]
    pub fn save_to_local_storage(&self) -> Result<(), String> {
        use web_sys::window;

        let window = window().ok_or("No window")?;
        let storage = window
            .local_storage()
            .map_err(|e| format!("{:?}", e))?
            .ok_or("No localStorage")?;

        let json = serde_json::to_string(self).map_err(|e| e.to_string())?;

        storage
            .set_item("p2p_config", &json)
            .map_err(|e| format!("{:?}", e))?;

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_config_is_valid() {
        let config = P2pConfig::default();
        assert!(config.validate().is_ok());
    }

    #[test]
    fn test_validation_catches_invalid_chunk_size() {
        let config = P2pConfig {
            chunk_size: 0,
            ..P2pConfig::default()
        };
        assert!(config.validate().is_err());
    }

    #[test]
    fn test_validation_catches_invalid_reputation() {
        let config = P2pConfig {
            min_reputation: 1.5,
            ..P2pConfig::default()
        };
        assert!(config.validate().is_err());
    }

    #[test]
    fn test_adaptive_adjustment() {
        let mut config = P2pConfig {
            adaptive_chunk_size: true,
            ..P2pConfig::default()
        };

        // Simulate low bandwidth
        config.adjust_for_conditions(500.0, 100.0);
        assert_eq!(config.parallel_chunk_limit, 2);
        assert!(config.request_timeout_ms > 2000);

        // Simulate high bandwidth
        config.adjust_for_conditions(10000.0, 20.0);
        assert_eq!(config.parallel_chunk_limit, 8);
    }

    #[test]
    fn test_batch_size_calculation() {
        let config = P2pConfig::default();
        let model_size = 100 * 1024 * 1024; // 100MB
        let batch_size = config.batch_size_for_model(model_size);
        assert!(batch_size > 0);
        assert!(batch_size <= config.parallel_chunk_limit);
    }

    #[test]
    fn test_peer_trust() {
        let config = P2pConfig::default();
        assert!(config.is_peer_trusted(0.8));
        assert!(!config.is_peer_trusted(0.3));
    }

    #[test]
    fn test_reputation_decay() {
        let config = P2pConfig::default();
        let score = 1.0;
        let decayed = config.apply_reputation_decay(score);
        assert_eq!(decayed, 0.95);
    }
}
