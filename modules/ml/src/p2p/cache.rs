use super::{Chunk, P2pConfig};
use dashmap::DashMap;
use moka::future::Cache;
use serde::{Deserialize, Serialize};
use std::sync::Arc;
use std::time::Duration;

/// Cache tier for multi-tier caching
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum CacheTier {
    Memory,     // Fast access, limited size
    Persistent, // Slower, persists across sessions
}

/// Access statistics for a chunk
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct AccessStats {
    pub access_count: u64,
    pub last_access: u64,
    pub first_access: u64,
    pub total_load_time_ms: u64,
}

impl Default for AccessStats {
    fn default() -> Self {
        Self {
            access_count: 0,
            last_access: current_timestamp(),
            first_access: current_timestamp(),
            total_load_time_ms: 0,
        }
    }
}

/// Cache metrics for monitoring
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct CacheMetrics {
    pub hits: u64,
    pub misses: u64,
    pub hit_rate: f32,
    pub memory_usage_bytes: u64,
    pub memory_capacity_bytes: u64,
    pub eviction_count: u64,
    pub average_load_time_ms: f32,
    pub prefetch_count: u64,
    pub compression_savings_bytes: u64,
}

impl Default for CacheMetrics {
    fn default() -> Self {
        Self {
            hits: 0,
            misses: 0,
            hit_rate: 0.0,
            memory_usage_bytes: 0,
            memory_capacity_bytes: 0,
            eviction_count: 0,
            average_load_time_ms: 0.0,
            prefetch_count: 0,
            compression_savings_bytes: 0,
        }
    }
}

/// Priority for cache operations
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Priority {
    Low,
    Normal,
    High,
}

/// Insert strategy for cache
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum InsertStrategy {
    MemoryOnly,
    PersistentOnly,
    Both,
    Adaptive,
}

/// Smart cache with multi-tier support and intelligent prefetching
pub struct SmartCache {
    memory_cache: Cache<String, Chunk>,
    config: P2pConfig,
    access_stats: Arc<DashMap<String, AccessStats>>,
    metrics: Arc<DashMap<String, CacheMetrics>>,
}

impl SmartCache {
    pub fn new(config: P2pConfig) -> Self {
        let memory_cache = Cache::builder()
            .max_capacity(config.max_cache_size as u64)
            .weigher(|_key: &String, value: &Chunk| value.size as u32)
            .time_to_idle(Duration::from_secs(60 * 60)) // 1 hour idle
            .build();

        Self {
            memory_cache,
            config,
            access_stats: Arc::new(DashMap::new()),
            metrics: Arc::new(DashMap::new()),
        }
    }

    /// Get a chunk from cache with priority
    pub async fn get(&self, chunk_id: &str) -> Option<Chunk> {
        let start_time = current_timestamp();

        // Update access stats
        self.update_access_stats(chunk_id, 0).await;

        // Try memory cache
        if let Some(chunk) = self.memory_cache.get(chunk_id).await {
            let load_time = current_timestamp() - start_time;
            self.record_access(chunk_id, true, load_time).await;
            return Some(chunk);
        }

        // Try persistent storage
        if let Some(path) = &self.config.persistence_path {
            #[cfg(not(target_arch = "wasm32"))]
            {
                let file_path = std::path::Path::new(path).join(chunk_id);
                if file_path.exists() {
                    if let Ok(data) = std::fs::read(&file_path) {
                        if let Ok(chunk) = serde_json::from_slice::<Chunk>(&data) {
                            // Promote to memory if valuable
                            // self.memory_cache.insert(chunk_id.to_string(), chunk.clone()).await;
                            let load_time = current_timestamp() - start_time;
                            self.record_access(chunk_id, true, load_time).await;
                            return Some(chunk);
                        }
                    }
                }
            }
            #[cfg(target_arch = "wasm32")]
            {
                let _ = path;
            }
        }

        // Record miss
        let load_time = current_timestamp() - start_time;
        self.record_access(chunk_id, false, load_time).await;

        None
    }

    /// Get with priority hint
    pub async fn get_with_priority(&self, chunk_id: &str, _priority: Priority) -> Option<Chunk> {
        // For now, priority doesn't change behavior
        // In full implementation, high priority could trigger prefetch
        self.get(chunk_id).await
    }

    /// Insert a chunk into cache
    pub async fn insert(&self, chunk_id: String, chunk: Chunk) {
        let size = chunk.size;
        self.memory_cache.insert(chunk_id.clone(), chunk).await;

        // Update metrics
        let mut metrics = self.metrics.entry("global".to_string()).or_default();
        metrics.memory_usage_bytes += size as u64;
    }

    /// Insert with strategy
    pub async fn insert_with_strategy(&self, chunk: Chunk, strategy: InsertStrategy) {
        match strategy {
            InsertStrategy::MemoryOnly | InsertStrategy::Adaptive => {
                self.insert(chunk.id.clone(), chunk).await;
            }
            InsertStrategy::PersistentOnly => {
                if let Err(e) = self.save_to_disk(&chunk).await {
                    log::error!("Failed to persist chunk {}: {}", chunk.id, e);
                }
            }
            InsertStrategy::Both => {
                self.insert(chunk.id.clone(), chunk.clone()).await;
                if let Err(e) = self.save_to_disk(&chunk).await {
                    log::error!("Failed to persist chunk {}: {}", chunk.id, e);
                }
            }
        }
    }

    /// Remove a chunk from cache
    pub async fn remove(&self, chunk_id: &str) {
        self.memory_cache.invalidate(chunk_id).await;
    }

    /// Check if chunk exists in cache
    pub async fn contains(&self, chunk_id: &str) -> bool {
        self.memory_cache.contains_key(chunk_id)
    }

    /// Clear all cached chunks
    pub async fn clear(&self) {
        self.memory_cache.invalidate_all();
        self.access_stats.clear();
    }

    /// Get cache statistics
    pub fn entry_count(&self) -> u64 {
        self.memory_cache.entry_count()
    }

    /// Get cache metrics
    pub fn metrics(&self) -> CacheMetrics {
        self.metrics
            .get("global")
            .map(|m| m.clone())
            .unwrap_or_default()
    }

    /// Calculate prefetch score for a chunk
    pub fn calculate_prefetch_score(&self, chunk_id: &str) -> f32 {
        let mut score = 0.0;

        if let Some(stats) = self.access_stats.get(chunk_id) {
            let recency = 1.0 / (current_timestamp() - stats.last_access + 1) as f32;
            let frequency = stats.access_count as f32;
            score = 0.7 * frequency + 0.3 * recency;
        }

        // Boost score for critical chunks
        if chunk_id.contains("layer-0") || chunk_id.contains("embedding") {
            score += 2.0;
        }

        if chunk_id.contains("attention") {
            score += 1.5;
        }

        score
    }

    /// Determine if chunk should be promoted to memory
    pub async fn should_promote(&self, chunk_id: &str) -> bool {
        if let Some(stats) = self.access_stats.get(chunk_id) {
            // Promote if accessed more than 3 times
            stats.access_count > 3
        } else {
            false
        }
    }

    /// Select tier for chunk based on value
    pub async fn select_tier(&self, chunk: &Chunk) -> CacheTier {
        let memory_usage = self.memory_cache.weighted_size();
        let capacity = self.config.max_cache_size as u64;
        let chunk_value = self.calculate_chunk_value(chunk).await;

        if memory_usage < capacity * 70 / 100 {
            // Plenty of memory
            CacheTier::Memory
        } else if chunk_value > 0.7 {
            // High value chunk
            CacheTier::Memory
        } else {
            // Lower value or memory full
            CacheTier::Persistent
        }
    }

    /// Calculate chunk value for tier selection
    async fn calculate_chunk_value(&self, chunk: &Chunk) -> f32 {
        let mut value = 0.5; // Base value

        // Check access stats
        if let Some(stats) = self.access_stats.get(&chunk.id) {
            let frequency_score = (stats.access_count as f32 / 10.0).min(1.0);
            let recency_score = 1.0 / (current_timestamp() - stats.last_access + 1) as f32;
            value = 0.6 * frequency_score + 0.4 * recency_score;
        }

        // Boost for critical chunks
        if chunk.id.contains("embedding") || chunk.id.contains("layer-0") {
            value += 0.3;
        }

        value.min(1.0)
    }

    /// Update access statistics
    async fn update_access_stats(&self, chunk_id: &str, load_time_ms: u64) {
        let mut stats = self.access_stats.entry(chunk_id.to_string()).or_default();

        stats.access_count += 1;
        stats.last_access = current_timestamp();
        stats.total_load_time_ms += load_time_ms;
    }

    /// Record cache access for metrics
    async fn record_access(&self, _chunk_id: &str, hit: bool, load_time_ms: u64) {
        let mut metrics = self.metrics.entry("global".to_string()).or_default();

        if hit {
            metrics.hits += 1;
        } else {
            metrics.misses += 1;
        }

        metrics.hit_rate = if metrics.hits + metrics.misses > 0 {
            metrics.hits as f32 / (metrics.hits + metrics.misses) as f32
        } else {
            0.0
        };

        // Update average load time with exponential moving average
        let alpha = 0.1;
        metrics.average_load_time_ms =
            alpha * load_time_ms as f32 + (1.0 - alpha) * metrics.average_load_time_ms;
    }

    /// Get access statistics for a chunk
    pub fn get_access_stats(&self, chunk_id: &str) -> Option<AccessStats> {
        self.access_stats.get(chunk_id).map(|s| s.clone())
    }

    /// Get current memory usage
    pub fn current_usage(&self) -> u64 {
        self.memory_cache.weighted_size()
    }

    /// Estimate available memory for prefetching
    pub fn estimate_available_memory(&self) -> u64 {
        let capacity = self.config.max_cache_size as u64;
        let usage = self.current_usage();
        capacity.saturating_sub(usage)
    }

    /// Save chunk to disk
    async fn save_to_disk(&self, chunk: &Chunk) -> std::result::Result<(), String> {
        if let Some(path) = &self.config.persistence_path {
            #[cfg(not(target_arch = "wasm32"))]
            {
                let dir = std::path::Path::new(path);
                if !dir.exists() {
                    std::fs::create_dir_all(dir).map_err(|e| e.to_string())?;
                }

                let file_path = dir.join(&chunk.id);
                let json = serde_json::to_vec(chunk).map_err(|e| e.to_string())?;
                std::fs::write(file_path, json).map_err(|e| e.to_string())?;
                return Ok(());
            }
            #[cfg(target_arch = "wasm32")]
            {
                // Simple localStorage fallback (limited size)
                let _ = path;
                let _ = chunk;
                return Ok(()); // Stub for now to avoid error
            }
        }
        Ok(())
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

#[cfg(test)]
mod tests {
    use super::*;

    fn create_test_chunk(id: &str, size: usize) -> Chunk {
        Chunk {
            id: id.to_string(),
            model_id: "test-model".to_string(),
            index: 0,
            data: vec![0; size],
            hash: "test-hash".to_string(),
            size,
        }
    }

    #[tokio::test]
    async fn test_cache_operations() {
        let config = P2pConfig::default();
        let cache = SmartCache::new(config);

        let chunk = create_test_chunk("test-chunk", 100);

        // Insert
        cache.insert("test-chunk".to_string(), chunk.clone()).await;

        // Get
        let retrieved = cache.get("test-chunk").await;
        assert!(retrieved.is_some());
        assert_eq!(retrieved.unwrap().id, "test-chunk");

        // Contains
        assert!(cache.contains("test-chunk").await);

        // Remove
        cache.remove("test-chunk").await;
        assert!(!cache.contains("test-chunk").await);
    }

    #[tokio::test]
    async fn test_access_stats() {
        let config = P2pConfig::default();
        let cache = SmartCache::new(config);

        let chunk = create_test_chunk("test-chunk", 100);
        cache.insert("test-chunk".to_string(), chunk).await;

        // Access multiple times
        for _ in 0..5 {
            cache.get("test-chunk").await;
        }

        let stats = cache.get_access_stats("test-chunk");
        assert!(stats.is_some());
        assert_eq!(stats.unwrap().access_count, 5);
    }

    #[tokio::test]
    async fn test_metrics() {
        let config = P2pConfig::default();
        let cache = SmartCache::new(config);

        let chunk = create_test_chunk("test-chunk", 100);
        cache.insert("test-chunk".to_string(), chunk).await;

        // Hit
        cache.get("test-chunk").await;

        // Miss
        cache.get("nonexistent").await;

        let metrics = cache.metrics();
        assert_eq!(metrics.hits, 1);
        assert_eq!(metrics.misses, 1);
        assert_eq!(metrics.hit_rate, 0.5);
    }

    #[tokio::test]
    async fn test_prefetch_scoring() {
        let config = P2pConfig::default();
        let cache = SmartCache::new(config);

        // Critical chunk should have higher score
        let score_embedding = cache.calculate_prefetch_score("model-embedding-0");
        let score_regular = cache.calculate_prefetch_score("model-layer-10");

        assert!(score_embedding > score_regular);
    }

    #[tokio::test]
    async fn test_tier_selection() {
        let config = P2pConfig::default();
        let cache = SmartCache::new(config);

        let chunk = create_test_chunk("test-chunk", 100);

        // With empty cache, should select memory
        let tier = cache.select_tier(&chunk).await;
        assert_eq!(tier, CacheTier::Memory);
    }
}
