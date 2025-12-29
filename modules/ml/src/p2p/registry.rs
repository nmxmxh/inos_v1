use super::{P2pError, Result};
use async_trait::async_trait;
use dashmap::DashMap;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;

/// Model type classification
#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
pub enum ModelType {
    Llm,
    Vision,
    Audio,
    Multimodal,
}

/// Model format
#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
pub enum ModelFormat {
    Safetensors,
    Gguf,
    Onnx,
}

/// Version-specific chunk information
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct VersionChunkInfo {
    pub total_chunks: usize,
    pub total_size: u64,
    pub chunk_ids: Vec<String>,
    pub chunk_sizes: Vec<u64>,
    pub chunk_hashes: Vec<String>,
    pub manifest_hash: String,
    pub created_at: u64,
}

/// Enhanced model metadata with versioning
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ModelMetadata {
    pub model_id: String,
    pub name: String,
    pub current_version: String,
    pub model_type: ModelType,
    pub format: ModelFormat,
    pub description: Option<String>,
    pub author: Option<String>,
    pub license: Option<String>,
    pub tags: Vec<String>,

    // Versioned chunk info
    pub versioned_chunks: HashMap<String, VersionChunkInfo>,

    // Metrics
    pub download_count: u64,
    pub inference_count: u64,
    pub avg_inference_time_ms: f32,
    pub created_at: u64,
    pub last_accessed: u64,
}

/// Peer location for chunk availability
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct PeerLocation {
    pub peer_id: String,
    pub latency_ms: Option<u32>,
    pub bandwidth_mbps: Option<f32>,
    pub reputation: f32,
    pub last_seen: u64,
}

/// Query parameters for model search
#[derive(Debug, Clone, Default)]
pub struct ModelQuery {
    pub name: Option<String>,
    pub model_type: Option<ModelType>,
    pub tags: Vec<String>,
    pub min_downloads: Option<u64>,
    pub created_after: Option<u64>,
    pub version: Option<String>,
    pub limit: Option<usize>,
    pub offset: Option<usize>,
}

/// Registry metrics
#[derive(Debug, Clone)]
pub struct RegistryMetrics {
    pub total_models: usize,
    pub total_versions: usize,
    pub total_chunks: usize,
    pub storage_size_bytes: u64,
    pub avg_model_size_bytes: f64,
    pub most_popular_model: Option<String>,
}

/// Enhanced trait for model registry operations
#[async_trait(?Send)]
pub trait ModelRegistry: Send + Sync {
    /// Register a new model
    async fn register_model(&self, metadata: ModelMetadata) -> Result<()>;

    /// Get model metadata
    async fn get_model(&self, model_id: &str) -> Result<Option<ModelMetadata>>;

    /// List all registered models
    async fn list_models(&self) -> Result<Vec<ModelMetadata>>;

    /// Update model metadata
    async fn update_model(&self, metadata: ModelMetadata) -> Result<()>;

    /// Remove a model from registry
    async fn remove_model(&self, model_id: &str) -> Result<()>;

    /// Check if model exists
    async fn has_model(&self, model_id: &str) -> Result<bool>;

    /// Query models with filters
    async fn query_models(&self, query: &ModelQuery) -> Result<Vec<ModelMetadata>>;

    /// Search models by name or description
    async fn search_models(&self, search_term: &str) -> Result<Vec<ModelMetadata>>;

    /// Get most popular models
    async fn get_popular_models(&self, limit: usize) -> Result<Vec<ModelMetadata>>;

    /// Get recently used models
    async fn get_recently_used(&self, limit: usize) -> Result<Vec<ModelMetadata>>;

    /// Update last accessed timestamp
    async fn touch(&self, model_id: &str) -> Result<()>;

    /// Get chunk availability percentage
    async fn chunk_availability(&self, model_id: &str) -> Result<f32>;

    /// Get registry metrics
    async fn metrics(&self) -> Result<RegistryMetrics>;
}

/// In-memory model registry with enhanced features
pub struct InMemoryModelRegistry {
    models: Arc<DashMap<String, ModelMetadata>>,
    chunk_locations: Arc<DashMap<String, Vec<PeerLocation>>>,
}

impl InMemoryModelRegistry {
    pub fn new() -> Self {
        Self {
            models: Arc::new(DashMap::new()),
            chunk_locations: Arc::new(DashMap::new()),
        }
    }

    /// Report chunk availability from a peer
    pub async fn report_chunk_availability(
        &self,
        peer_id: &str,
        chunk_ids: Vec<String>,
        reputation: f32,
    ) -> Result<()> {
        let location = PeerLocation {
            peer_id: peer_id.to_string(),
            latency_ms: None,
            bandwidth_mbps: None,
            reputation,
            last_seen: current_timestamp(),
        };

        for chunk_id in chunk_ids {
            self.chunk_locations
                .entry(chunk_id)
                .or_default()
                .push(location.clone());
        }

        Ok(())
    }

    /// Find best peers for a chunk
    pub async fn find_chunk_sources(
        &self,
        chunk_id: &str,
        min_reputation: f32,
    ) -> Result<Vec<PeerLocation>> {
        let locations = self
            .chunk_locations
            .get(chunk_id)
            .map(|entry| {
                entry
                    .iter()
                    .filter(|loc| loc.reputation >= min_reputation)
                    .cloned()
                    .collect()
            })
            .unwrap_or_default();

        Ok(locations)
    }

    /// Cleanup old models
    pub async fn cleanup_old_models(&self, max_age_days: u64) -> Result<u64> {
        let threshold = current_timestamp() - (max_age_days * 24 * 60 * 60);
        let mut removed = 0;

        let to_remove: Vec<String> = self
            .models
            .iter()
            .filter(|entry| entry.last_accessed < threshold)
            .map(|entry| entry.model_id.clone())
            .collect();

        for model_id in to_remove {
            self.models.remove(&model_id);
            removed += 1;
        }

        Ok(removed)
    }
}

impl Default for InMemoryModelRegistry {
    fn default() -> Self {
        Self::new()
    }
}

#[async_trait(?Send)]
impl ModelRegistry for InMemoryModelRegistry {
    async fn register_model(&self, metadata: ModelMetadata) -> Result<()> {
        if metadata.versioned_chunks.is_empty() {
            return Err(P2pError::InvalidChunkData {
                chunk_id: metadata.model_id.clone(),
                reason: "No chunks specified".to_string(),
            });
        }

        self.models.insert(metadata.model_id.clone(), metadata);
        Ok(())
    }

    async fn get_model(&self, model_id: &str) -> Result<Option<ModelMetadata>> {
        Ok(self.models.get(model_id).map(|entry| entry.clone()))
    }

    async fn list_models(&self) -> Result<Vec<ModelMetadata>> {
        Ok(self
            .models
            .iter()
            .map(|entry| entry.value().clone())
            .collect())
    }

    async fn update_model(&self, metadata: ModelMetadata) -> Result<()> {
        self.models.insert(metadata.model_id.clone(), metadata);
        Ok(())
    }

    async fn remove_model(&self, model_id: &str) -> Result<()> {
        self.models.remove(model_id);
        Ok(())
    }

    async fn has_model(&self, model_id: &str) -> Result<bool> {
        Ok(self.models.contains_key(model_id))
    }

    async fn query_models(&self, query: &ModelQuery) -> Result<Vec<ModelMetadata>> {
        let mut results: Vec<ModelMetadata> = self
            .models
            .iter()
            .map(|entry| entry.value().clone())
            .filter(|model| {
                // Filter by name
                if let Some(ref name) = query.name {
                    if !model.name.contains(name) {
                        return false;
                    }
                }

                // Filter by type
                if let Some(ref model_type) = query.model_type {
                    if &model.model_type != model_type {
                        return false;
                    }
                }

                // Filter by tags
                if !query.tags.is_empty() && !query.tags.iter().any(|tag| model.tags.contains(tag))
                {
                    return false;
                }

                // Filter by downloads
                if let Some(min_downloads) = query.min_downloads {
                    if model.download_count < min_downloads {
                        return false;
                    }
                }

                // Filter by creation date
                if let Some(created_after) = query.created_after {
                    if model.created_at < created_after {
                        return false;
                    }
                }

                true
            })
            .collect();

        // Apply offset and limit
        if let Some(offset) = query.offset {
            results = results.into_iter().skip(offset).collect();
        }

        if let Some(limit) = query.limit {
            results.truncate(limit);
        }

        Ok(results)
    }

    async fn search_models(&self, search_term: &str) -> Result<Vec<ModelMetadata>> {
        let search_lower = search_term.to_lowercase();

        Ok(self
            .models
            .iter()
            .filter(|entry| {
                let model = entry.value();
                model.name.to_lowercase().contains(&search_lower)
                    || model
                        .description
                        .as_ref()
                        .map(|d| d.to_lowercase().contains(&search_lower))
                        .unwrap_or(false)
                    || model
                        .tags
                        .iter()
                        .any(|tag| tag.to_lowercase().contains(&search_lower))
            })
            .map(|entry| entry.value().clone())
            .collect())
    }

    async fn get_popular_models(&self, limit: usize) -> Result<Vec<ModelMetadata>> {
        let mut models: Vec<ModelMetadata> = self
            .models
            .iter()
            .map(|entry| entry.value().clone())
            .collect();

        models.sort_by(|a, b| b.download_count.cmp(&a.download_count));
        models.truncate(limit);

        Ok(models)
    }

    async fn get_recently_used(&self, limit: usize) -> Result<Vec<ModelMetadata>> {
        let mut models: Vec<ModelMetadata> = self
            .models
            .iter()
            .map(|entry| entry.value().clone())
            .collect();

        models.sort_by(|a, b| b.last_accessed.cmp(&a.last_accessed));
        models.truncate(limit);

        Ok(models)
    }

    async fn touch(&self, model_id: &str) -> Result<()> {
        if let Some(mut entry) = self.models.get_mut(model_id) {
            entry.last_accessed = current_timestamp();
            entry.inference_count += 1;
        }
        Ok(())
    }

    async fn chunk_availability(&self, model_id: &str) -> Result<f32> {
        let model = self
            .get_model(model_id)
            .await?
            .ok_or_else(|| P2pError::ModelNotFound {
                model_id: model_id.to_string(),
                available_models: vec![],
                context: super::ErrorContext::default(),
            })?;

        if let Some(chunk_info) = model.versioned_chunks.get(&model.current_version) {
            let total = chunk_info.total_chunks;
            if total == 0 {
                return Ok(0.0);
            }

            let available = chunk_info
                .chunk_ids
                .iter()
                .filter(|chunk_id| self.chunk_locations.contains_key(*chunk_id))
                .count();

            Ok((available as f32 / total as f32) * 100.0)
        } else {
            Ok(0.0)
        }
    }

    async fn metrics(&self) -> Result<RegistryMetrics> {
        let models: Vec<ModelMetadata> = self
            .models
            .iter()
            .map(|entry| entry.value().clone())
            .collect();

        let total_models = models.len();
        let total_versions: usize = models.iter().map(|m| m.versioned_chunks.len()).sum();
        let total_chunks: usize = models
            .iter()
            .flat_map(|m| m.versioned_chunks.values())
            .map(|v| v.total_chunks)
            .sum();
        let storage_size_bytes: u64 = models
            .iter()
            .flat_map(|m| m.versioned_chunks.values())
            .map(|v| v.total_size)
            .sum();

        let avg_model_size_bytes = if total_models > 0 {
            storage_size_bytes as f64 / total_models as f64
        } else {
            0.0
        };

        let most_popular_model = models
            .iter()
            .max_by_key(|m| m.download_count)
            .map(|m| m.model_id.clone());

        Ok(RegistryMetrics {
            total_models,
            total_versions,
            total_chunks,
            storage_size_bytes,
            avg_model_size_bytes,
            most_popular_model,
        })
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

    fn create_test_metadata() -> ModelMetadata {
        let mut versioned_chunks = HashMap::new();
        versioned_chunks.insert(
            "1.0".to_string(),
            VersionChunkInfo {
                total_chunks: 2,
                total_size: 1024 * 1024,
                chunk_ids: vec!["chunk-1".to_string(), "chunk-2".to_string()],
                chunk_sizes: vec![512 * 1024, 512 * 1024],
                chunk_hashes: vec!["hash1".to_string(), "hash2".to_string()],
                manifest_hash: "manifest".to_string(),
                created_at: 0,
            },
        );

        ModelMetadata {
            model_id: "test-model".to_string(),
            name: "Test Model".to_string(),
            current_version: "1.0".to_string(),
            model_type: ModelType::Llm,
            format: ModelFormat::Safetensors,
            description: Some("A test model".to_string()),
            author: Some("Test Author".to_string()),
            license: Some("MIT".to_string()),
            tags: vec!["test".to_string(), "demo".to_string()],
            versioned_chunks,
            download_count: 10,
            inference_count: 100,
            avg_inference_time_ms: 50.0,
            created_at: 0,
            last_accessed: 0,
        }
    }

    #[tokio::test]
    async fn test_registry_operations() {
        let registry = InMemoryModelRegistry::new();
        let metadata = create_test_metadata();

        // Register
        registry.register_model(metadata.clone()).await.unwrap();

        // Get
        let retrieved = registry.get_model("test-model").await.unwrap();
        assert!(retrieved.is_some());
        assert_eq!(retrieved.unwrap().name, "Test Model");

        // Has
        assert!(registry.has_model("test-model").await.unwrap());

        // List
        let models = registry.list_models().await.unwrap();
        assert_eq!(models.len(), 1);

        // Touch
        registry.touch("test-model").await.unwrap();
        let updated = registry.get_model("test-model").await.unwrap().unwrap();
        assert_eq!(updated.inference_count, 101);

        // Remove
        registry.remove_model("test-model").await.unwrap();
        assert!(!registry.has_model("test-model").await.unwrap());
    }

    #[tokio::test]
    async fn test_query_models() {
        let registry = InMemoryModelRegistry::new();
        registry
            .register_model(create_test_metadata())
            .await
            .unwrap();

        // Query by type
        let query = ModelQuery {
            model_type: Some(ModelType::Llm),
            ..Default::default()
        };
        let results = registry.query_models(&query).await.unwrap();
        assert_eq!(results.len(), 1);

        // Query by tags
        let query = ModelQuery {
            tags: vec!["test".to_string()],
            ..Default::default()
        };
        let results = registry.query_models(&query).await.unwrap();
        assert_eq!(results.len(), 1);
    }

    #[tokio::test]
    async fn test_search_models() {
        let registry = InMemoryModelRegistry::new();
        registry
            .register_model(create_test_metadata())
            .await
            .unwrap();

        let results = registry.search_models("Test").await.unwrap();
        assert_eq!(results.len(), 1);

        let results = registry.search_models("demo").await.unwrap();
        assert_eq!(results.len(), 1);
    }

    #[tokio::test]
    async fn test_metrics() {
        let registry = InMemoryModelRegistry::new();
        registry
            .register_model(create_test_metadata())
            .await
            .unwrap();

        let metrics = registry.metrics().await.unwrap();
        assert_eq!(metrics.total_models, 1);
        assert_eq!(metrics.total_versions, 1);
        assert_eq!(metrics.total_chunks, 2);
        assert_eq!(metrics.storage_size_bytes, 1024 * 1024);
    }
}
