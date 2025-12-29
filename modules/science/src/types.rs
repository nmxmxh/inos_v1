use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize, Clone)]
pub enum ScienceError {
    Internal(String),
    MethodNotFound(String),
    InvalidParams(String),
    InvalidLibrary(String),
}

pub type ScienceResult<T> = Result<T, ScienceError>;

#[derive(Serialize, Deserialize, Clone, Copy, PartialEq, Eq, PartialOrd, Debug, Default)]
pub enum FidelityLevel {
    #[default]
    Heuristic, // Game physics
    Engineering,  // Industry standard
    Research,     // Publication quality
    QuantumExact, // Full Schrödinger
    RealityProof, // With merkle proofs
}

#[derive(Serialize, Deserialize, Clone, Copy, Debug, Default)]
pub struct SimulationScale {
    pub spatial: f64,  // meters
    pub temporal: f64, // seconds
    pub energy: f64,   // joules
    pub fidelity: FidelityLevel,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ComputationProof {
    pub input_hash: String,
    pub method_hash: String,
    pub params_hash: String,
    pub result_hash: String,
    pub node_id: u64,
    pub shard_id: u32,
    pub epoch: u64,
    pub verification_data: Vec<u8>,
}

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct CacheEntry {
    pub data: Vec<u8>,
    pub result_hash: String,
    pub timestamp: u64,
    pub access_count: u32,
    pub scale: SimulationScale,
    pub proof: ComputationProof,
}
#[derive(Serialize, Deserialize, Clone, Debug, Default)]
pub struct Telemetry {
    pub computations: u64,
    pub cache_hits: u64,
    pub cache_misses: u64,
    pub validation_requests: u64,
    pub quantum_ops: u64,
    pub mesh_solves: u64,
    pub physics_steps: u64,
    pub compute_time_ms: u64,
    pub cross_scale_calls: u64,
}
