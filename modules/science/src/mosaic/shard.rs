use blake3;
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, VecDeque};
use std::sync::atomic::{AtomicU64, Ordering};

// ----------------------------------------------------------------------------
// VOXEL SHARDING CORE
// ----------------------------------------------------------------------------

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Voxel {
    pub id: String,
    pub bounds: [f64; 6],
    pub level: u8,
    pub shard_id: u32,
    pub material_hash: String,
    pub load_factor: f64,
    pub energy_density: f64,
    pub element_count: u64,
    pub last_update: u64,
    pub subdivision_state: SubdivisionState,
    pub neighbors: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum SubdivisionState {
    Coarse,
    Refining,
    Refined(u8),
    Merging,
    Critical,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Shard {
    pub id: u32,
    pub voxels: Vec<String>,
    pub node_ids: Vec<u64>,
    pub leader_node: u64,
    pub replicas: Vec<u64>,
    pub total_load: f64,
    pub capacity: f64,
    pub regions: Vec<Region>,
    pub telemetry: ShardTelemetry,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Region {
    pub name: String,
    pub node_count: u32,
    pub latency_map: HashMap<String, f32>,
    pub capabilities: Vec<Capability>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum Capability {
    QuantumCompute,
    GPUAccelerated,
    HighMemory,
    LowLatency,
    PersistentStorage,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ShardTelemetry {
    pub operations_per_second: f64,
    pub data_transferred_gb: f64,
    pub average_latency_ms: f64,
    pub cache_hit_rate: f64,
    pub voxel_updates: u64,
    pub cross_shard_syncs: u64,
    pub energy_consumption_kwh: f64,
    pub subdivision_events: u64,
    pub merge_events: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MosaicOctree {
    pub root: Voxel,
    pub voxels: HashMap<String, Voxel>,
    pub shards: HashMap<u32, Shard>,
    pub voxel_to_shard: HashMap<String, u32>,
    pub thresholds: Thresholds,
    pub history: VecDeque<TreeMutation>,
    pub seed: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Thresholds {
    pub subdivision_load: f64,
    pub subdivision_energy: f64,
    pub subdivision_elements: u64,
    pub merge_load: f64,
    pub max_level: u8,
    pub min_voxel_size: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TreeMutation {
    pub timestamp: u64,
    pub operation: MutationType,
    pub voxel_id: String,
    pub parent_voxel: Option<String>,
    pub child_voxels: Vec<String>,
    pub proof: MutationProof,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum MutationType {
    Subdivision,
    Merge,
    Migration,
    LoadUpdate,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MutationProof {
    pub pre_state_hash: String,
    pub post_state_hash: String,
    pub operation_hash: String,
    pub validator_signatures: Vec<String>,
}

// ----------------------------------------------------------------------------
// SHARD MANAGER
// ----------------------------------------------------------------------------

pub struct ShardManager {
    pub octree: MosaicOctree,
    pub distribution: DistributionStrategy,
    pub replication_factor: ReplicationConfig,
    pub balancer: LoadBalancer,
    pub telemetry: GlobalTelemetry,
    pub epoch: AtomicU64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum DistributionStrategy {
    SpatialLocality {
        radius: f64,
    },
    LoadBalanced,
    CapabilityAware,
    Hybrid {
        spatial_weight: f64,
        load_weight: f64,
        capability_weight: f64,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ReplicationConfig {
    pub min_replicas: u32,
    pub max_replicas: u32,
    pub strategy: ReplicationStrategy,
    pub consistency: ConsistencyModel,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum ReplicationStrategy {
    GeographicRedundancy,
    AccessPattern,
    CriticalityBased,
    Adaptive,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum ConsistencyModel {
    Strong,
    Eventual { sync_interval_ms: u64 },
    Causal,
    ReadYourWrites,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LoadBalancer {
    pub load_map: HashMap<u32, f64>,
    pub target_distribution: Vec<f64>,
    pub migration_queue: VecDeque<MigrationTask>,
    pub algorithm: BalancingAlgorithm,
    pub metrics: BalancingMetrics,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum BalancingAlgorithm {
    RoundRobin,
    LeastLoaded,
    MLPredictive { model_hash: String },
    GameTheoretic,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct BalancingMetrics {
    pub migrations_performed: u64,
    pub load_imbalance: f64,
    pub migration_overhead: f64,
    pub convergence_time_ms: f64,
    pub stability: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MigrationTask {
    pub voxel_id: String,
    pub from_shard: u32,
    pub to_shard: u32,
    pub priority: MigrationPriority,
    pub estimated_data_mb: f64,
    pub deadline: Option<u64>,
    pub dependencies: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum MigrationPriority {
    Low,
    Normal,
    High,
    Critical,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct GlobalTelemetry {
    pub total_voxels: u64,
    pub total_shards: u32,
    pub total_nodes: u64,
    pub average_load: f64,
    pub max_load: f64,
    pub min_load: f64,
    pub cross_shard_traffic_gb: f64,
    pub subdivision_rate: f64,
    pub merge_rate: f64,
    pub migration_rate: f64,
    pub energy_efficiency: f64,
}

impl ShardManager {
    pub fn new(total_nodes: u64, bounds: [f64; 6]) -> Self {
        let shard_count = (total_nodes / 1_000_000).max(1) as u32;

        let root_voxel = Voxel {
            id: Self::generate_voxel_id(&bounds, 0),
            bounds,
            level: 0,
            shard_id: 0,
            material_hash: String::new(),
            load_factor: 0.0,
            energy_density: 0.0,
            element_count: 0,
            last_update: 0,
            subdivision_state: SubdivisionState::Coarse,
            neighbors: Vec::new(),
        };

        let mut shards = HashMap::new();
        let mut voxel_to_shard = HashMap::new();

        for shard_id in 0..shard_count {
            let start_node = (shard_id as u64) * 1_000_000;
            let node_ids: Vec<u64> = (start_node..start_node + 1_000_000).collect();

            let shard = Shard {
                id: shard_id,
                voxels: Vec::new(),
                node_ids,
                leader_node: start_node,
                replicas: Vec::new(),
                total_load: 0.0,
                capacity: 1.0,
                regions: vec![Region {
                    name: "default".to_string(),
                    node_count: 1_000_000,
                    latency_map: HashMap::new(),
                    capabilities: vec![Capability::HighMemory],
                }],
                telemetry: ShardTelemetry::default(),
            };

            shards.insert(shard_id, shard);
        }

        voxel_to_shard.insert(root_voxel.id.clone(), 0);

        let octree = MosaicOctree {
            root: root_voxel.clone(),
            voxels: {
                let mut map = HashMap::new();
                map.insert(root_voxel.id.clone(), root_voxel);
                map
            },
            shards,
            voxel_to_shard,
            thresholds: Thresholds {
                subdivision_load: 0.8,
                subdivision_energy: 1e6,
                subdivision_elements: 1_000_000,
                merge_load: 0.2,
                max_level: 10,
                min_voxel_size: 1e-6,
            },
            history: VecDeque::with_capacity(1000),
            seed: 123456789,
        };

        Self {
            octree,
            distribution: DistributionStrategy::Hybrid {
                spatial_weight: 0.4,
                load_weight: 0.4,
                capability_weight: 0.2,
            },
            replication_factor: ReplicationConfig {
                min_replicas: 5,
                max_replicas: 700,
                strategy: ReplicationStrategy::Adaptive,
                consistency: ConsistencyModel::Eventual {
                    sync_interval_ms: 100,
                },
            },
            balancer: LoadBalancer {
                load_map: HashMap::new(),
                target_distribution: vec![1.0; shard_count as usize],
                migration_queue: VecDeque::new(),
                algorithm: BalancingAlgorithm::MLPredictive {
                    model_hash: "default_balancer_v1".to_string(),
                },
                metrics: BalancingMetrics::default(),
            },
            telemetry: GlobalTelemetry::default(),
            epoch: AtomicU64::new(0),
        }
    }

    pub fn locate_voxel(&self, point: [f64; 3]) -> Option<&Voxel> {
        let mut current_id = self.octree.root.id.clone();

        loop {
            let voxel = self.octree.voxels.get(&current_id)?;

            if Self::point_in_voxel(point, voxel.bounds) {
                if let SubdivisionState::Refined(_) = voxel.subdivision_state {
                    if let Some(child_id) = self.find_child_containing(voxel, point) {
                        current_id = child_id;
                        continue;
                    }
                }
                return Some(voxel);
            }

            return None;
        }
    }

    pub fn subdivide_voxel(&mut self, voxel_id: &str) -> Result<Vec<String>, String> {
        let epoch = self.epoch.fetch_add(1, Ordering::SeqCst);

        // Clone necessary data to avoid borrow checker issues
        let (
            voxel_bounds,
            voxel_level,
            voxel_shard_id,
            voxel_material_hash,
            voxel_load,
            voxel_energy,
            voxel_elements,
        ) = {
            let voxel = self
                .octree
                .voxels
                .get(voxel_id)
                .ok_or_else(|| format!("Voxel not found: {}", voxel_id))?;

            if let SubdivisionState::Refined(_) = voxel.subdivision_state {
                return Err("Voxel already subdivided".to_string());
            }

            if voxel.level >= self.octree.thresholds.max_level {
                return Err("Maximum subdivision level reached".to_string());
            }

            (
                voxel.bounds,
                voxel.level,
                voxel.shard_id,
                voxel.material_hash.clone(),
                voxel.load_factor,
                voxel.energy_density,
                voxel.element_count,
            )
        };

        let [min_x, min_y, min_z, max_x, max_y, max_z] = voxel_bounds;
        let mid_x = (min_x + max_x) / 2.0;
        let mid_y = (min_y + max_y) / 2.0;
        let mid_z = (min_z + max_z) / 2.0;

        let mut child_ids = Vec::new();
        for i in 0..8 {
            let child_bounds = match i {
                0 => [min_x, min_y, min_z, mid_x, mid_y, mid_z],
                1 => [mid_x, min_y, min_z, max_x, mid_y, mid_z],
                2 => [min_x, mid_y, min_z, mid_x, max_y, mid_z],
                3 => [mid_x, mid_y, min_z, max_x, max_y, mid_z],
                4 => [min_x, min_y, mid_z, mid_x, mid_y, max_z],
                5 => [mid_x, min_y, mid_z, max_x, mid_y, max_z],
                6 => [min_x, mid_y, mid_z, mid_x, max_y, max_z],
                7 => [mid_x, mid_y, mid_z, max_x, max_y, max_z],
                _ => unreachable!(),
            };

            let child_id = Self::generate_voxel_id(&child_bounds, voxel_level + 1);

            let child_voxel = Voxel {
                id: child_id.clone(),
                bounds: child_bounds,
                level: voxel_level + 1,
                shard_id: voxel_shard_id,
                material_hash: voxel_material_hash.clone(),
                load_factor: voxel_load / 8.0,
                energy_density: voxel_energy,
                element_count: voxel_elements / 8,
                last_update: epoch,
                subdivision_state: SubdivisionState::Coarse,
                neighbors: Vec::new(),
            };

            self.octree.voxels.insert(child_id.clone(), child_voxel);
            self.octree
                .voxel_to_shard
                .insert(child_id.clone(), voxel_shard_id);
            child_ids.push(child_id);
        }

        if let Some(parent) = self.octree.voxels.get_mut(voxel_id) {
            parent.subdivision_state = SubdivisionState::Refined(voxel_level + 1);
            parent.last_update = epoch;
        }

        if let Some(shard) = self.octree.shards.get_mut(&voxel_shard_id) {
            shard.voxels.retain(|id| id != voxel_id);
            shard.voxels.extend(child_ids.iter().cloned());
        }

        let mutation = TreeMutation {
            timestamp: epoch,
            operation: MutationType::Subdivision,
            voxel_id: voxel_id.to_string(),
            parent_voxel: None,
            child_voxels: child_ids.clone(),
            proof: self.generate_mutation_proof(voxel_id, &child_ids, "subdivide"),
        };

        self.octree.history.push_back(mutation);
        if self.octree.history.len() > 1000 {
            self.octree.history.pop_front();
        }

        self.telemetry.subdivision_rate += 1.0;
        self.telemetry.total_voxels += 7;

        Ok(child_ids)
    }

    pub fn check_hot_spots(&self) -> Vec<String> {
        let mut hot_voxels = Vec::new();
        let thresholds = &self.octree.thresholds;

        for (voxel_id, voxel) in &self.octree.voxels {
            let needs_subdivision = voxel.load_factor > thresholds.subdivision_load
                || voxel.energy_density > thresholds.subdivision_energy
                || voxel.element_count > thresholds.subdivision_elements;

            let can_subdivide = !matches!(voxel.subdivision_state, SubdivisionState::Refined(_));

            if needs_subdivision && can_subdivide && voxel.level < thresholds.max_level {
                hot_voxels.push(voxel_id.clone());
            }
        }

        hot_voxels.sort_by(|a, b| {
            let voxel_a = self.octree.voxels.get(a).unwrap();
            let voxel_b = self.octree.voxels.get(b).unwrap();
            let urgency_a = voxel_a.load_factor * voxel_a.energy_density;
            let urgency_b = voxel_b.load_factor * voxel_b.energy_density;
            urgency_b.partial_cmp(&urgency_a).unwrap()
        });

        hot_voxels
    }

    pub fn balance_load(&mut self) -> BalancingReport {
        let shard_loads: Vec<(u32, f64)> = self
            .octree
            .shards
            .iter()
            .map(|(id, shard)| (*id, shard.total_load))
            .collect();

        let total_load: f64 = shard_loads.iter().map(|(_, load)| load).sum();
        let avg_load = total_load / shard_loads.len() as f64;

        let mut overloaded = Vec::new();
        let mut underloaded = Vec::new();

        for (shard_id, load) in &shard_loads {
            let imbalance = (load - avg_load) / avg_load;

            if imbalance > 0.2 {
                overloaded.push((*shard_id, *load, imbalance));
            } else if imbalance < -0.2 {
                underloaded.push((*shard_id, *load, imbalance));
            }
        }

        overloaded.sort_by(|a, b| b.2.partial_cmp(&a.2).unwrap());
        underloaded.sort_by(|a, b| a.2.partial_cmp(&b.2).unwrap());

        let mut migrations_planned = 0;

        for (over_id, _over_load, _) in &overloaded {
            for (under_id, _under_load, _) in &underloaded {
                if let Some(shard) = self.octree.shards.get(over_id) {
                    let mut voxel_loads: Vec<(&String, f64)> = shard
                        .voxels
                        .iter()
                        .filter_map(|voxel_id| {
                            self.octree
                                .voxels
                                .get(voxel_id)
                                .map(|v| (voxel_id, v.load_factor))
                        })
                        .collect();

                    voxel_loads.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap());

                    for (voxel_id, _load) in voxel_loads.iter().take(1) {
                        let task = MigrationTask {
                            voxel_id: voxel_id.to_string(),
                            from_shard: *over_id,
                            to_shard: *under_id,
                            priority: MigrationPriority::Normal,
                            estimated_data_mb: 100.0,
                            deadline: None,
                            dependencies: Vec::new(),
                        };

                        self.balancer.migration_queue.push_back(task);
                        migrations_planned += 1;
                    }
                }
            }
        }

        let imbalance = shard_loads
            .iter()
            .map(|(_, load)| ((load - avg_load) / avg_load).abs())
            .sum::<f64>()
            / shard_loads.len() as f64;

        self.balancer.metrics.load_imbalance = imbalance;

        BalancingReport {
            overloaded_shards: overloaded.len(),
            underloaded_shards: underloaded.len(),
            planned_migrations: migrations_planned,
            current_imbalance: imbalance,
            target_imbalance: 0.1,
        }
    }

    fn generate_voxel_id(bounds: &[f64; 6], level: u8) -> String {
        let mut hasher = blake3::Hasher::new();
        for &b in bounds {
            hasher.update(&b.to_le_bytes());
        }
        hasher.update(&level.to_le_bytes());
        format!("voxel_{}", &hasher.finalize().to_hex()[..16])
    }

    fn point_in_voxel(point: [f64; 3], bounds: [f64; 6]) -> bool {
        let [min_x, min_y, min_z, max_x, max_y, max_z] = bounds;
        let [x, y, z] = point;

        x >= min_x && x <= max_x && y >= min_y && y <= max_y && z >= min_z && z <= max_z
    }

    fn find_child_containing(&self, _parent: &Voxel, _point: [f64; 3]) -> Option<String> {
        None
    }

    fn generate_mutation_proof(
        &self,
        voxel_id: &str,
        child_ids: &[String],
        operation: &str,
    ) -> MutationProof {
        let mut hasher = blake3::Hasher::new();
        hasher.update(voxel_id.as_bytes());
        for child_id in child_ids {
            hasher.update(child_id.as_bytes());
        }
        hasher.update(operation.as_bytes());
        hasher.update(&self.epoch.load(Ordering::SeqCst).to_le_bytes());

        MutationProof {
            pre_state_hash: String::new(),
            post_state_hash: String::new(),
            operation_hash: hasher.finalize().to_hex().to_string(),
            validator_signatures: Vec::new(),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BalancingReport {
    pub overloaded_shards: usize,
    pub underloaded_shards: usize,
    pub planned_migrations: usize,
    pub current_imbalance: f64,
    pub target_imbalance: f64,
}

// ----------------------------------------------------------------------------
// P2P DISTRIBUTION
// ----------------------------------------------------------------------------

pub struct P2PDistributor {
    pub voxel_replicas: HashMap<String, Vec<Replica>>,
    pub node_voxels: HashMap<u64, Vec<String>>,
    pub strategy: ReplicationStrategy,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Replica {
    pub node_id: u64,
    pub voxel_id: String,
    pub replica_type: ReplicaType,
    pub last_sync: u64,
    pub sync_status: SyncStatus,
    pub data_hash: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum ReplicaType {
    Primary,
    Secondary,
    Witness,
    Geographic,
    ColdStorage,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum SyncStatus {
    Synchronized,
    Synchronizing { progress: f64 },
    Behind { behind_by: u64 },
    Diverged { divergence_hash: String },
    Offline,
}

impl P2PDistributor {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn distribute_voxel(&mut self, voxel_id: &str, importance: f64, total_nodes: u64) {
        let replica_count = self.calculate_replica_count(importance);
        let replica_nodes = self.select_replica_nodes(voxel_id, replica_count, total_nodes);

        let mut replicas = Vec::new();
        for (i, node_id) in replica_nodes.iter().enumerate() {
            let replica_type = if i == 0 {
                ReplicaType::Primary
            } else if i < 5 {
                ReplicaType::Secondary
            } else {
                ReplicaType::Geographic
            };

            replicas.push(Replica {
                node_id: *node_id,
                voxel_id: voxel_id.to_string(),
                replica_type,
                last_sync: 0,
                sync_status: SyncStatus::Synchronized,
                data_hash: String::new(),
            });

            self.node_voxels
                .entry(*node_id)
                .or_default()
                .push(voxel_id.to_string());
        }

        self.voxel_replicas.insert(voxel_id.to_string(), replicas);
    }

    fn calculate_replica_count(&self, importance: f64) -> u32 {
        let min = 5;
        let max = 700;
        min + ((importance * (max - min) as f64) as u32)
    }

    fn select_replica_nodes(&self, voxel_id: &str, count: u32, total_nodes: u64) -> Vec<u64> {
        let mut nodes = Vec::new();
        let mut hasher = blake3::Hasher::new();
        hasher.update(voxel_id.as_bytes());

        for i in 0..count {
            let mut node_hasher = hasher.clone();
            node_hasher.update(&i.to_le_bytes());
            let hash = node_hasher.finalize();

            let node_id =
                u64::from_le_bytes(hash.as_bytes()[0..8].try_into().unwrap()) % total_nodes;
            nodes.push(node_id);
        }

        nodes
    }
}

impl Default for P2PDistributor {
    fn default() -> Self {
        Self {
            voxel_replicas: HashMap::new(),
            node_voxels: HashMap::new(),
            strategy: ReplicationStrategy::Adaptive,
        }
    }
}
