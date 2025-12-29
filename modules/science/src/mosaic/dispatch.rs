// use nalgebra::Vector3;
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet, VecDeque};

// ----------------------------------------------------------------------------
// VOXEL IDENTIFICATION
// ----------------------------------------------------------------------------

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct VoxelID(pub [i32; 3]);

impl VoxelID {
    pub fn neighbors(&self) -> Vec<VoxelID> {
        let [x, y, z] = self.0;
        let mut neighbors = Vec::with_capacity(26);

        for dx in -1..=1 {
            for dy in -1..=1 {
                for dz in -1..=1 {
                    if dx == 0 && dy == 0 && dz == 0 {
                        continue;
                    }
                    neighbors.push(VoxelID([x + dx, y + dy, z + dz]));
                }
            }
        }
        neighbors
    }

    pub fn distance(&self, other: &VoxelID) -> f32 {
        let [x1, y1, z1] = self.0;
        let [x2, y2, z2] = other.0;
        let dx = (x2 - x1) as f32;
        let dy = (y2 - y1) as f32;
        let dz = (z2 - z1) as f32;
        (dx * dx + dy * dy + dz * dz).sqrt()
    }
}

// ----------------------------------------------------------------------------
// SHARDING STRATEGIES
// ----------------------------------------------------------------------------

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub enum ShardingStrategy {
    ByScale(f32),
    ByDomain(String),
    Hybrid {
        base_scale: f32,
        domain_patterns: Vec<String>,
    },
    Adaptive {
        min_voxel_size: f32,
        max_voxel_size: f32,
        target_load: usize,
    },
}

// ----------------------------------------------------------------------------
// PROXY TYPES
// ----------------------------------------------------------------------------

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, Eq, Hash)]
pub enum ProxyType {
    Atomic,
    Continuum,
    Kinetic,
    Hybrid,
}

// ----------------------------------------------------------------------------
// COMPUTE NODE MANAGEMENT
// ----------------------------------------------------------------------------

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct ComputeNode {
    pub id: String,
    pub address: String,
    pub capabilities: Vec<ProxyType>,
    pub load: f32,
    pub capacity: usize,
    pub voxels: HashSet<VoxelID>,
}

// ----------------------------------------------------------------------------
// ELEMENT MIGRATION
// ----------------------------------------------------------------------------

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct ElementDescriptor {
    pub id: String,
    pub voxel_id: VoxelID,
    pub proxy_type: ProxyType,
    pub state_hash: u64,
    pub migration_cost: f32,
}

#[derive(Serialize, Deserialize, Clone, Debug)]
pub enum MigrationStatus {
    Pending,
    InFlight,
    Committed,
    RolledBack,
}

// ----------------------------------------------------------------------------
// ERROR HANDLING
// ----------------------------------------------------------------------------

#[derive(Debug)]
pub enum DispatchError {
    NoNodeAvailable(VoxelID),
    MigrationFailed(String, String),
    UnsupportedProxy(ProxyType, String),
    BoundaryViolation(String),
    FluxTimeout,
}

impl std::fmt::Display for DispatchError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            DispatchError::NoNodeAvailable(v) => {
                write!(f, "No compute node available for voxel {:?}", v)
            }
            DispatchError::MigrationFailed(e, r) => {
                write!(f, "Migration failed for element {}: {}", e, r)
            }
            DispatchError::UnsupportedProxy(p, n) => {
                write!(f, "Proxy type {:?} not supported on node {}", p, n)
            }
            DispatchError::BoundaryViolation(m) => write!(f, "Voxel boundary violation: {}", m),
            DispatchError::FluxTimeout => write!(f, "Flux coordination timeout"),
        }
    }
}

impl std::error::Error for DispatchError {}

// ----------------------------------------------------------------------------
// SPATIAL DISPATCHER
// ----------------------------------------------------------------------------

pub struct SpatialDispatcher {
    pub shard_registry: HashMap<VoxelID, String>,
    pub compute_nodes: HashMap<String, ComputeNode>,
    pub strategy: ShardingStrategy,
    pub voxel_size: f32,

    pending_migrations: VecDeque<String>,
    // TODO(v3.1): Flux counter for conservation tracking
    #[allow(dead_code)]
    flux_counter: u64,

    metrics: DispatchMetrics,
    voxel_cache: HashMap<[i32; 3], VoxelID>,
}

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct DispatchMetrics {
    pub total_migrations: usize,
    pub failed_migrations: usize,
    pub avg_migration_time_ms: f64,
    pub flux_coordinations: usize,
    pub cache_hit_rate: f32,
}

impl SpatialDispatcher {
    pub fn new(strategy: ShardingStrategy) -> Self {
        Self {
            shard_registry: HashMap::new(),
            compute_nodes: HashMap::new(),
            strategy,
            voxel_size: 1.0,
            pending_migrations: VecDeque::new(),
            flux_counter: 0,
            metrics: DispatchMetrics {
                total_migrations: 0,
                failed_migrations: 0,
                avg_migration_time_ms: 0.0,
                flux_coordinations: 0,
                cache_hit_rate: 0.0,
            },
            voxel_cache: HashMap::new(),
        }
    }

    pub fn register_node(&mut self, node: ComputeNode) {
        self.compute_nodes.insert(node.id.clone(), node);
    }

    pub fn deregister_node(&mut self, node_id: &str) -> Result<(), DispatchError> {
        if let Some(node) = self.compute_nodes.remove(node_id) {
            for voxel_id in node.voxels {
                self.reassign_voxel(&voxel_id)?;
            }
        }
        Ok(())
    }

    pub fn locate_shard(&mut self, pos: [f32; 3]) -> VoxelID {
        let voxel_coords = [
            (pos[0] / self.voxel_size).floor() as i32,
            (pos[1] / self.voxel_size).floor() as i32,
            (pos[2] / self.voxel_size).floor() as i32,
        ];

        if let Some(&voxel_id) = self.voxel_cache.get(&voxel_coords) {
            self.metrics.cache_hit_rate = (self.metrics.cache_hit_rate * 0.9) + 0.1;
            return voxel_id;
        }

        let voxel_size = match &self.strategy {
            ShardingStrategy::ByScale(scale) => self.voxel_size * scale,
            ShardingStrategy::Adaptive { min_voxel_size, .. } => *min_voxel_size,
            _ => self.voxel_size,
        };

        let voxel_id = VoxelID([
            (pos[0] / voxel_size).floor() as i32,
            (pos[1] / voxel_size).floor() as i32,
            (pos[2] / voxel_size).floor() as i32,
        ]);

        self.voxel_cache.insert(voxel_coords, voxel_id);
        voxel_id
    }

    pub fn get_node_for_voxel(&self, voxel_id: VoxelID) -> Result<&ComputeNode, DispatchError> {
        if let Some(node_id) = self.shard_registry.get(&voxel_id) {
            if let Some(node) = self.compute_nodes.get(node_id) {
                return Ok(node);
            }
        }

        self.find_best_node(voxel_id)
            .ok_or(DispatchError::NoNodeAvailable(voxel_id))
    }

    fn find_best_node(&self, voxel_id: VoxelID) -> Option<&ComputeNode> {
        self.compute_nodes
            .values()
            .filter(|node| node.load < 0.8)
            .min_by(|a, b| {
                a.load
                    .partial_cmp(&b.load)
                    .unwrap_or(std::cmp::Ordering::Equal)
                    .then_with(|| {
                        let a_dist = self.voxel_distance_to_node(voxel_id, a);
                        let b_dist = self.voxel_distance_to_node(voxel_id, b);
                        a_dist
                            .partial_cmp(&b_dist)
                            .unwrap_or(std::cmp::Ordering::Equal)
                    })
            })
    }

    fn voxel_distance_to_node(&self, voxel_id: VoxelID, node: &ComputeNode) -> f32 {
        if node.voxels.is_empty() {
            return f32::MAX;
        }

        node.voxels
            .iter()
            .map(|&node_voxel| voxel_id.distance(&node_voxel))
            .fold(f32::MAX, f32::min)
    }

    pub fn migrate_element(
        &mut self,
        element: ElementDescriptor,
        from: VoxelID,
        to: VoxelID,
    ) -> Result<String, DispatchError> {
        if from == to {
            return Ok("Already in target voxel".to_string());
        }

        if !from.neighbors().contains(&to) {
            return Err(DispatchError::BoundaryViolation(format!(
                "Cannot migrate non-adjacent voxels: {:?} -> {:?}",
                from, to
            )));
        }

        let _source_node = self.get_node_for_voxel(from)?;
        let target_node = self.get_node_for_voxel(to)?;

        if !target_node.capabilities.contains(&element.proxy_type) {
            return Err(DispatchError::UnsupportedProxy(
                element.proxy_type,
                target_node.id.clone(),
            ));
        }

        let migration_id = format!("mig_{}_{}", element.id, self.metrics.total_migrations);
        self.pending_migrations.push_back(migration_id.clone());

        Ok(migration_id)
    }

    pub fn rebalance_shards(&mut self) -> Result<usize, DispatchError> {
        let mut reassignments = 0;

        let overloaded_nodes: Vec<String> = self
            .compute_nodes
            .values()
            .filter(|node| node.load > 0.75)
            .map(|node| node.id.clone())
            .collect();

        for node_id in overloaded_nodes {
            if let Some(node) = self.compute_nodes.get(&node_id) {
                let voxels_to_move: Vec<VoxelID> = node
                    .voxels
                    .iter()
                    .take((node.voxels.len() as f32 * 0.2) as usize)
                    .copied()
                    .collect();

                for voxel_id in voxels_to_move {
                    if self.reassign_voxel(&voxel_id).is_ok() {
                        reassignments += 1;
                    }
                }
            }
        }

        Ok(reassignments)
    }

    fn reassign_voxel(&mut self, voxel_id: &VoxelID) -> Result<(), DispatchError> {
        if let Some(new_node) = self.find_best_node(*voxel_id) {
            let new_node_id = new_node.id.clone();

            if let Some(old_node_id) = self.shard_registry.get(voxel_id) {
                if let Some(old_node) = self.compute_nodes.get_mut(old_node_id) {
                    old_node.voxels.remove(voxel_id);
                }
            }

            if let Some(new_node) = self.compute_nodes.get_mut(&new_node_id) {
                new_node.voxels.insert(*voxel_id);
                self.shard_registry.insert(*voxel_id, new_node_id);
            }
        }

        Ok(())
    }

    pub fn get_metrics(&self) -> &DispatchMetrics {
        &self.metrics
    }

    pub fn invalidate_cache_for_voxel(&mut self, voxel_id: VoxelID) {
        self.voxel_cache.retain(|_, v| *v != voxel_id);
    }
}

// ----------------------------------------------------------------------------
// ZERO-COPY MIGRATION
// ----------------------------------------------------------------------------

pub struct ZeroCopyContext {
    pub source_ptr: *const u8,
    pub dest_ptr: *mut u8,
    pub size: usize,
    pub alignment: usize,
}

impl ZeroCopyContext {
    /// # Safety
    ///
    /// This function performs raw pointer copies using `std::ptr::copy_nonoverlapping`.
    /// The caller must ensure that:
    /// 1. Both `source_ptr` and `dest_ptr` are valid for reads/writes of `size` bytes.
    /// 2. The memory regions do not overlap.
    pub unsafe fn migrate(&self) -> Result<(), String> {
        if self.source_ptr.is_null() || self.dest_ptr.is_null() {
            return Err("Null pointer in migration context".to_string());
        }

        std::ptr::copy_nonoverlapping(self.source_ptr, self.dest_ptr, self.size);

        Ok(())
    }
}

// ----------------------------------------------------------------------------
// PROXY DISPATCHER TRAIT
// ----------------------------------------------------------------------------

pub trait ProxyDispatcher {
    fn route_operation(
        &self,
        op_type: ProxyType,
        voxel_id: VoxelID,
        data: &[u8],
    ) -> Result<Vec<u8>, DispatchError>;

    fn can_handle(&self, proxy_type: ProxyType) -> bool;

    fn estimate_load(&self, voxel_id: VoxelID) -> f32;
}
