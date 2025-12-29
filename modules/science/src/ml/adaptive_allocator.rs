use crate::mosaic::bridge::PeerID;
use crate::mosaic::bridge::VoxelRange;
use async_trait::async_trait;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LoadPrediction {
    pub predicted_load: f32,
    pub confidence: f32,
    pub recommended_strategy: AllocationStrategy,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum AllocationStrategy {
    ReadOptimized,
    WriteOptimized,
    Balanced,
    Sequential,
    Random,
}

#[async_trait]
pub trait AdaptiveAllocator: Send + Sync {
    async fn predict_load(
        &self,
        voxel_range: &VoxelRange,
        strategy: AllocationStrategy,
    ) -> LoadPrediction;

    async fn allocate(
        &self,
        voxel_range: &VoxelRange,
        strategy: AllocationStrategy,
    ) -> Result<Vec<PeerID>, String>;
}
