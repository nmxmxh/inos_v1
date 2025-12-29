use blake3;
use nalgebra::{Matrix3, Vector3};
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, VecDeque};

// ----------------------------------------------------------------------------
// CROSS-SCALE REALITY NEGOTIATION
// ----------------------------------------------------------------------------

#[derive(Debug, Serialize, Deserialize, Clone)]
pub enum ScaleEvent {
    /// Quantum events that require continuum response
    BondBreak {
        pos: Vector3<f64>,
        energy: f64,
        atoms: Vec<[usize; 2]>,
        time: f64,
        quantum_state_hash: String,
    },

    ElectronicTransition {
        pos: Vector3<f64>,
        old_density: f64,
        new_density: f64,
        orbital_change: Option<String>,
        spin: Option<f64>,
    },

    /// Continuum events requiring kinetic response
    StressPeak {
        pos: Vector3<f64>,
        magnitude: f64,
        stress_tensor: Matrix3<f64>,
        element_id: usize,
        yield_criterion: YieldCriterion,
        plastic_strain: f64,
    },

    CrackInitiation {
        pos: Vector3<f64>,
        direction: Vector3<f64>,
        stress_intensity: f64,
        material_hash: String,
    },

    /// Kinetic events feeding back to continuum
    CollisionEvent {
        body_a: u32,
        body_b: u32,
        impact_point: Vector3<f64>,
        impulse: f64,
        normal: Vector3<f64>,
        energy_dissipated: f64,
        continuum_node: Option<u32>,
    },

    ScaleMismatch {
        pos: Vector3<f64>,
        scale_from: SimulationScale,
        scale_to: SimulationScale,
        discrepancy: f64,
        field: String,
    },
}

#[derive(Debug, Serialize, Deserialize, Clone, Copy)]
pub enum YieldCriterion {
    VonMises(f64),
    Tresca(f64),
    MohrCoulomb(f64, f64),
    DruckerPrager(f64, f64),
    JohnsonCook(f64, f64, f64),
}

#[derive(Debug, Serialize, Deserialize, Clone, Copy)]
pub struct SimulationScale {
    pub spatial: f64,
    pub temporal: f64,
    pub energy: f64,
    pub fidelity: FidelityLevel,
}

#[derive(Debug, Serialize, Deserialize, Clone, Copy, PartialEq)]
pub enum FidelityLevel {
    Heuristic,
    Engineering,
    Research,
    QuantumExact,
    RealityProof,
}

// ----------------------------------------------------------------------------
// ADVANCED SCALE MAPPING
// ----------------------------------------------------------------------------

#[derive(Serialize, Deserialize, Clone)]
pub struct ScaleMapping {
    pub id: String,
    pub source_scale: SimulationScale,
    pub target_scale: SimulationScale,
    pub method: MappingMethod,
    pub parameters: MappingParameters,
    pub validation: MappingValidation,
}

#[derive(Serialize, Deserialize, Clone)]
pub enum MappingMethod {
    LinearInterpolation {
        kernel_radius: f64,
        shape_function: ShapeFunction,
    },

    ConservativeMapping {
        conserved_quantity: ConservedQuantity,
        tolerance: f64,
        iterative_refinement: bool,
    },

    LearnedMapping {
        model_hash: String,
        training_error: f64,
        inference_cost: f64,
    },

    BridgingDomain {
        overlap_width: f64,
        blending_function: BlendingFunction,
        handshake_region: Vec<[f64; 3]>,
    },

    Arlequin {
        coupling_energy_weight: f64,
        lagrange_multipliers: bool,
        consistency_tolerance: f64,
    },

    Quasicontinuum {
        representative_atoms: Vec<usize>,
        sampling_region: Vec<[f64; 3]>,
        force_correction: bool,
    },

    CauchyBorn {
        crystal_symmetry: String,
        lattice_parameters: [f64; 6],
        unit_cell_hash: String,
    },
}

#[derive(Serialize, Deserialize, Clone)]
pub enum ShapeFunction {
    Linear,
    Quadratic,
    Cubic,
    Gaussian { sigma: f64 },
    Wendland { radius: f64 },
}

#[derive(Serialize, Deserialize, Clone)]
pub enum ConservedQuantity {
    Energy,
    Momentum,
    Mass,
    Charge,
    AngularMomentum,
    All,
}

#[derive(Serialize, Deserialize, Clone)]
pub enum BlendingFunction {
    Linear,
    Exponential { alpha: f64 },
    Polynomial { degree: u32 },
    Sigmoid,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct MappingParameters {
    pub cutoff_radius: f64,
    pub min_sampling: f64,
    pub preserve_symmetry: bool,
    pub max_error: f64,
    pub budget: f64,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct MappingValidation {
    pub dataset_hash: String,
    pub rms_error: f64,
    pub max_error: f64,
    pub conservation_error: f64,
    pub proof: MappingProof,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct MappingProof {
    pub merkle_root: String,
    pub validator_nodes: Vec<u64>,
    pub consensus_reached: bool,
    pub error_bounds: ErrorBounds,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct ErrorBounds {
    pub lower: f64,
    pub upper: f64,
    pub confidence: f64,
}

// ----------------------------------------------------------------------------
// COUPLING CONFIGURATION
// ----------------------------------------------------------------------------

#[derive(Serialize, Deserialize, Clone)]
pub struct ScaleCoupling {
    pub id: String,
    pub from_scale: SimulationScale,
    pub to_scale: SimulationScale,
    pub coupling_type: CouplingType,
    pub mapping: ScaleMapping,
    pub triggers: Vec<TriggerCondition>,
    pub feedback: FeedbackType,
    pub update_frequency: u32,
    pub convergence: ConvergenceCriteria,
}

#[derive(Serialize, Deserialize, Clone)]
pub enum CouplingType {
    QuantumToContinuum,
    ContinuumToKinetic,
    KineticToContinuum,
    AllToAll,
    Adaptive,
    Hierarchical { levels: Vec<SimulationScale> },
}

#[derive(Serialize, Deserialize, Clone)]
pub enum TriggerCondition {
    SpatialRegion { bounds: [[f64; 3]; 2] },
    TimeInterval { start: f64, end: f64 },
    OnEvent(Box<ScaleEvent>),
    ErrorThreshold { field: String, threshold: f64 },
    EnergyDensity { threshold: f64 },
    GradientThreshold { field: String, threshold: f64 },
}

#[derive(Serialize, Deserialize, Clone)]
pub enum FeedbackType {
    Unidirectional,
    Bidirectional { damping: f64 },
    Conservative,
    Delayed { delay_time: f64 },
    Adaptive,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct ConvergenceCriteria {
    pub max_iterations: u32,
    pub field_tolerance: f64,
    pub energy_tolerance: f64,
    pub relaxation_factor: f64,
    pub aitken_acceleration: bool,
}

// ----------------------------------------------------------------------------
// NEGOTIATION RESULTS
// ----------------------------------------------------------------------------

#[derive(Serialize, Deserialize, Clone)]
pub struct NegotiationResult {
    pub success: bool,
    pub mapped_data: Vec<u8>,
    pub energy_discrepancy: f64,
    pub momentum_discrepancy: [f64; 3],
    pub consistency_proof: ConsistencyProof,
    pub performance: NegotiationPerformance,
}

#[derive(Serialize, Deserialize, Clone, Default)]
pub struct ConsistencyProof {
    pub source_hash: String,
    pub mapping_hash: String,
    pub result_hash: String,
    pub merkle_proof: String,
    pub validator_signatures: Vec<String>,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct ScaleBuffer {
    pub pending_events: VecDeque<ScaleEvent>,
    pub active_events: Vec<ScaleEvent>,
    pub completed_events: VecDeque<(ScaleEvent, NegotiationResult)>,
    pub priorities: HashMap<String, u32>,
}

// ----------------------------------------------------------------------------
// TRANS-SCALE NEGOTIATOR
// ----------------------------------------------------------------------------

pub struct TransScaleNegotiator {
    pub buffers: HashMap<ScaleTransition, ScaleBuffer>,
    pub mapping_library: HashMap<String, ScaleMapping>,
    pub active_couplings: Vec<ScaleCoupling>,
    pub telemetry: NegotiationTelemetry,
    pub rng_seed: u64,
    pub negotiation_cache: HashMap<String, NegotiationResult>,
}

#[derive(Serialize, Deserialize, Clone, Copy, PartialEq, Eq, Hash, Debug)]
pub enum ScaleTransition {
    AtomicToContinuum,
    ContinuumToAtomic,
    ContinuumToKinetic,
    KineticToContinuum,
    AtomicToKinetic,
    KineticToAtomic,
}

#[derive(Serialize, Deserialize, Default, Clone)]
pub struct NegotiationTelemetry {
    pub negotiations_performed: u64,
    pub events_processed: u64,
    pub average_time_ms: f64,
    pub cache_hits: u64,
    pub cache_misses: u64,
    pub energy_conservation_violations: u64,
    pub convergence_failures: u64,
}

impl Default for TransScaleNegotiator {
    fn default() -> Self {
        Self::new()
    }
}

impl TransScaleNegotiator {
    pub fn new() -> Self {
        let mut buffers = HashMap::new();

        for transition in [
            ScaleTransition::AtomicToContinuum,
            ScaleTransition::ContinuumToAtomic,
            ScaleTransition::ContinuumToKinetic,
            ScaleTransition::KineticToContinuum,
            ScaleTransition::AtomicToKinetic,
            ScaleTransition::KineticToAtomic,
        ] {
            buffers.insert(
                transition,
                ScaleBuffer {
                    pending_events: VecDeque::new(),
                    active_events: Vec::new(),
                    completed_events: VecDeque::new(),
                    priorities: HashMap::new(),
                },
            );
        }

        Self {
            buffers,
            mapping_library: Self::default_mappings(),
            active_couplings: Vec::new(),
            telemetry: NegotiationTelemetry::default(),
            rng_seed: 123456789,
            negotiation_cache: HashMap::new(),
        }
    }

    fn default_mappings() -> HashMap<String, ScaleMapping> {
        let mut library = HashMap::new();

        library.insert(
            "quantum_to_continuum_v1".to_string(),
            ScaleMapping {
                id: "quantum_to_continuum_v1".to_string(),
                source_scale: SimulationScale {
                    spatial: 1e-10,
                    temporal: 1e-15,
                    energy: 1.6e-19,
                    fidelity: FidelityLevel::QuantumExact,
                },
                target_scale: SimulationScale {
                    spatial: 1e-8,
                    temporal: 1e-12,
                    energy: 1.6e-18,
                    fidelity: FidelityLevel::Research,
                },
                method: MappingMethod::ConservativeMapping {
                    conserved_quantity: ConservedQuantity::Energy,
                    tolerance: 1e-6,
                    iterative_refinement: true,
                },
                parameters: MappingParameters {
                    cutoff_radius: 5e-10,
                    min_sampling: 1e27,
                    preserve_symmetry: true,
                    max_error: 0.01,
                    budget: 1.0,
                },
                validation: MappingValidation {
                    dataset_hash: "default_validation_set_v1".to_string(),
                    rms_error: 0.005,
                    max_error: 0.02,
                    conservation_error: 1e-8,
                    proof: MappingProof {
                        merkle_root: "default_proof_root".to_string(),
                        validator_nodes: vec![],
                        consensus_reached: true,
                        error_bounds: ErrorBounds {
                            lower: 0.0,
                            upper: 0.03,
                            confidence: 0.95,
                        },
                    },
                },
            },
        );

        library.insert(
            "continuum_to_kinetic_v1".to_string(),
            ScaleMapping {
                id: "continuum_to_kinetic_v1".to_string(),
                source_scale: SimulationScale {
                    spatial: 1e-3,
                    temporal: 1e-3,
                    energy: 1.0,
                    fidelity: FidelityLevel::Engineering,
                },
                target_scale: SimulationScale {
                    spatial: 1e-2,
                    temporal: 1e-3,
                    energy: 1.0,
                    fidelity: FidelityLevel::Engineering,
                },
                method: MappingMethod::LinearInterpolation {
                    kernel_radius: 1e-3,
                    shape_function: ShapeFunction::Gaussian { sigma: 5e-4 },
                },
                parameters: MappingParameters {
                    cutoff_radius: 5e-3,
                    min_sampling: 1e6,
                    preserve_symmetry: false,
                    max_error: 0.05,
                    budget: 0.1,
                },
                validation: MappingValidation {
                    dataset_hash: "default_validation_set_v2".to_string(),
                    rms_error: 0.03,
                    max_error: 0.08,
                    conservation_error: 0.01,
                    proof: MappingProof {
                        merkle_root: "default_proof_root_v2".to_string(),
                        validator_nodes: vec![],
                        consensus_reached: true,
                        error_bounds: ErrorBounds {
                            lower: 0.0,
                            upper: 0.1,
                            confidence: 0.90,
                        },
                    },
                },
            },
        );

        library
    }

    pub fn queue_event(&mut self, event: ScaleEvent, priority: u32) -> String {
        let transition = self.event_to_transition(&event);
        let event_id = self.generate_event_id(&event);

        let buffer = self.buffers.get_mut(&transition).unwrap();
        buffer.pending_events.push_back(event);
        buffer.priorities.insert(event_id.clone(), priority);

        self.telemetry.events_processed += 1;
        event_id
    }

    fn event_to_transition(&self, event: &ScaleEvent) -> ScaleTransition {
        match event {
            ScaleEvent::BondBreak { .. } | ScaleEvent::ElectronicTransition { .. } => {
                ScaleTransition::AtomicToContinuum
            }
            ScaleEvent::StressPeak { .. } | ScaleEvent::CrackInitiation { .. } => {
                ScaleTransition::ContinuumToKinetic
            }
            ScaleEvent::CollisionEvent { .. } => ScaleTransition::KineticToContinuum,
            ScaleEvent::ScaleMismatch {
                scale_from,
                scale_to,
                ..
            } => {
                if scale_from.spatial < scale_to.spatial {
                    ScaleTransition::AtomicToContinuum
                } else {
                    ScaleTransition::ContinuumToAtomic
                }
            }
        }
    }

    fn generate_event_id(&self, event: &ScaleEvent) -> String {
        let mut hasher = blake3::Hasher::new();
        let event_bytes = bincode::serialize(event).unwrap_or_default();
        hasher.update(&event_bytes);
        hasher.update(&self.rng_seed.to_le_bytes());
        format!("event_{}", &hasher.finalize().to_hex()[..16])
    }

    pub fn negotiate_all(&mut self) -> Vec<NegotiationResult> {
        let mut results = Vec::new();
        let mut events_to_process: Vec<(ScaleEvent, ScaleTransition)> = Vec::new();
        // Phase 1: Collect all pending events
        for (transition, buffer) in &mut self.buffers {
            while let Some(event) = buffer.pending_events.pop_front() {
                events_to_process.push((event, *transition));
            }
        }
        // Phase 2: Process events
        for (event, transition) in events_to_process {
            let result = self.negotiate_event(&event, transition);
            results.push(result.clone());

            if let Some(buffer) = self.buffers.get_mut(&transition) {
                buffer.completed_events.push_back((event, result));
            }
        }
        results
    }

    pub fn negotiate_event(
        &mut self,
        event: &ScaleEvent,
        transition: ScaleTransition,
    ) -> NegotiationResult {
        let cache_key = self.generate_cache_key(event, transition);
        if let Some(cached) = self.negotiation_cache.get(&cache_key) {
            self.telemetry.cache_hits += 1;
            return cached.clone();
        }

        self.telemetry.cache_misses += 1;
        self.telemetry.negotiations_performed += 1;

        let mapping = self.select_mapping(event, transition);

        let result = match transition {
            ScaleTransition::AtomicToContinuum => self.atomic_to_continuum(event, &mapping),
            ScaleTransition::ContinuumToAtomic => self.continuum_to_atomic(event, &mapping),
            ScaleTransition::ContinuumToKinetic => self.continuum_to_kinetic(event, &mapping),
            ScaleTransition::KineticToContinuum => self.kinetic_to_continuum(event, &mapping),
            ScaleTransition::AtomicToKinetic => self.atomic_to_kinetic(event, &mapping),
            ScaleTransition::KineticToAtomic => self.kinetic_to_atomic(event, &mapping),
        };

        self.negotiation_cache.insert(cache_key, result.clone());
        result
    }

    pub fn atomic_to_continuum(
        &self,
        event: &ScaleEvent,
        mapping: &ScaleMapping,
    ) -> NegotiationResult {
        match event {
            ScaleEvent::BondBreak {
                pos, energy, atoms, ..
            } => {
                let continuum_forces =
                    self.map_atomic_to_continuum_forces(*pos, *energy, atoms, mapping);
                let damage_data =
                    self.create_continuum_damage_field(*pos, *energy, &continuum_forces);

                NegotiationResult {
                    success: true,
                    mapped_data: damage_data.clone(),
                    energy_discrepancy: self.calculate_energy_discrepancy(*energy, &damage_data),
                    momentum_discrepancy: [0.0; 3],
                    consistency_proof: self.generate_consistency_proof(
                        event,
                        mapping,
                        &damage_data,
                    ),
                    performance: NegotiationPerformance::default(),
                }
            }

            ScaleEvent::ElectronicTransition {
                pos,
                old_density,
                new_density,
                ..
            } => {
                let delta_density = new_density - old_density;
                let stress_change = self.density_to_stress(delta_density, *pos);
                let stress_data = bincode::serialize(&stress_change).unwrap_or_default();

                NegotiationResult {
                    success: true,
                    mapped_data: stress_data.clone(),
                    energy_discrepancy: 0.0,
                    momentum_discrepancy: [0.0; 3],
                    consistency_proof: self.generate_consistency_proof(
                        event,
                        mapping,
                        &stress_data,
                    ),
                    performance: NegotiationPerformance::default(),
                }
            }

            _ => NegotiationResult::failure(),
        }
    }

    pub fn continuum_to_atomic(
        &self,
        event: &ScaleEvent,
        mapping: &ScaleMapping,
    ) -> NegotiationResult {
        match event {
            ScaleEvent::StressPeak {
                pos, stress_tensor, ..
            } => {
                let atomic_forces = self.map_stress_to_atomic_forces(*pos, *stress_tensor, mapping);
                let force_data = bincode::serialize(&atomic_forces).unwrap_or_default();

                NegotiationResult {
                    success: true,
                    mapped_data: force_data.clone(),
                    energy_discrepancy: self
                        .calculate_stress_energy_discrepancy(*stress_tensor, &atomic_forces),
                    momentum_discrepancy: [0.0; 3],
                    consistency_proof: self.generate_consistency_proof(event, mapping, &force_data),
                    performance: NegotiationPerformance::default(),
                }
            }
            _ => NegotiationResult::failure(),
        }
    }

    pub fn continuum_to_kinetic(
        &self,
        event: &ScaleEvent,
        mapping: &ScaleMapping,
    ) -> NegotiationResult {
        match event {
            ScaleEvent::StressPeak {
                pos,
                stress_tensor,
                element_id,
                ..
            } => {
                let rigid_body_forces = self.map_stress_to_rigid_body_forces(
                    *pos,
                    *stress_tensor,
                    *element_id,
                    mapping,
                );
                let force_data = bincode::serialize(&rigid_body_forces).unwrap_or_default();

                NegotiationResult {
                    success: true,
                    mapped_data: force_data.clone(),
                    energy_discrepancy: 0.0,
                    momentum_discrepancy: [0.0; 3],
                    consistency_proof: self.generate_consistency_proof(event, mapping, &force_data),
                    performance: NegotiationPerformance::default(),
                }
            }

            ScaleEvent::CrackInitiation {
                pos,
                direction,
                stress_intensity,
                ..
            } => {
                let fragment_forces =
                    self.map_crack_to_fragment_forces(*pos, *direction, *stress_intensity, mapping);
                let force_data = bincode::serialize(&fragment_forces).unwrap_or_default();

                NegotiationResult {
                    success: true,
                    mapped_data: force_data,
                    energy_discrepancy: *stress_intensity * 0.1,
                    momentum_discrepancy: [0.0; 3],
                    consistency_proof: ConsistencyProof::default(),
                    performance: NegotiationPerformance::default(),
                }
            }

            _ => NegotiationResult::failure(),
        }
    }

    pub fn kinetic_to_continuum(
        &self,
        event: &ScaleEvent,
        mapping: &ScaleMapping,
    ) -> NegotiationResult {
        match event {
            ScaleEvent::CollisionEvent {
                impact_point,
                impulse,
                normal,
                energy_dissipated,
                continuum_node,
                ..
            } => {
                let stress_wave = self.map_impulse_to_stress_wave(
                    *impact_point,
                    *impulse,
                    *normal,
                    *energy_dissipated,
                    *continuum_node,
                    mapping,
                );
                let wave_data = bincode::serialize(&stress_wave).unwrap_or_default();

                NegotiationResult {
                    success: true,
                    mapped_data: wave_data,
                    energy_discrepancy: *energy_dissipated,
                    momentum_discrepancy: [
                        impulse * normal.x,
                        impulse * normal.y,
                        impulse * normal.z,
                    ],
                    consistency_proof: ConsistencyProof::default(),
                    performance: NegotiationPerformance::default(),
                }
            }
            _ => NegotiationResult::failure(),
        }
    }

    pub fn atomic_to_kinetic(
        &self,
        _event: &ScaleEvent,
        _mapping: &ScaleMapping,
    ) -> NegotiationResult {
        NegotiationResult::failure()
    }

    pub fn kinetic_to_atomic(
        &self,
        _event: &ScaleEvent,
        _mapping: &ScaleMapping,
    ) -> NegotiationResult {
        NegotiationResult::failure()
    }

    // Mapping implementations
    fn map_atomic_to_continuum_forces(
        &self,
        _pos: Vector3<f64>,
        energy: f64,
        atoms: &[[usize; 2]],
        _mapping: &ScaleMapping,
    ) -> Vec<[f64; 3]> {
        let mut forces = Vec::new();
        for &[_i, _j] in atoms {
            forces.push([energy, 0.0, 0.0]);
            forces.push([-energy, 0.0, 0.0]);
        }
        forces
    }

    fn create_continuum_damage_field(
        &self,
        _pos: Vector3<f64>,
        energy: f64,
        _forces: &[[f64; 3]],
    ) -> Vec<u8> {
        let damage_tensor = Matrix3::new(
            energy,
            0.0,
            0.0,
            0.0,
            energy * 0.5,
            0.0,
            0.0,
            0.0,
            energy * 0.5,
        );
        bincode::serialize(&damage_tensor).unwrap_or_default()
    }

    fn density_to_stress(&self, delta_density: f64, _pos: Vector3<f64>) -> Matrix3<f64> {
        let pressure = delta_density;
        Matrix3::new(pressure, 0.0, 0.0, 0.0, pressure, 0.0, 0.0, 0.0, pressure)
    }

    fn map_stress_to_atomic_forces(
        &self,
        _pos: Vector3<f64>,
        stress: Matrix3<f64>,
        _mapping: &ScaleMapping,
    ) -> Vec<[f64; 3]> {
        vec![[stress.m11, stress.m12, stress.m13]]
    }

    fn map_stress_to_rigid_body_forces(
        &self,
        pos: Vector3<f64>,
        stress: Matrix3<f64>,
        element_id: usize,
        _mapping: &ScaleMapping,
    ) -> Vec<ExternalForce> {
        vec![ExternalForce {
            body_handle: element_id as u32,
            force: [stress.m11 as f32, stress.m12 as f32, stress.m13 as f32],
            torque: [0.0; 3],
            application_point: Some([pos.x as f32, pos.y as f32, pos.z as f32]),
        }]
    }

    fn map_crack_to_fragment_forces(
        &self,
        pos: Vector3<f64>,
        direction: Vector3<f64>,
        stress_intensity: f64,
        _mapping: &ScaleMapping,
    ) -> Vec<ExternalForce> {
        vec![ExternalForce {
            body_handle: 0,
            force: [
                (stress_intensity * direction.x) as f32,
                (stress_intensity * direction.y) as f32,
                (stress_intensity * direction.z) as f32,
            ],
            torque: [0.0; 3],
            application_point: Some([pos.x as f32, pos.y as f32, pos.z as f32]),
        }]
    }

    fn map_impulse_to_stress_wave(
        &self,
        impact_point: Vector3<f64>,
        impulse: f64,
        normal: Vector3<f64>,
        energy_dissipated: f64,
        _continuum_node: Option<u32>,
        _mapping: &ScaleMapping,
    ) -> StressWave {
        StressWave {
            origin: [impact_point.x, impact_point.y, impact_point.z],
            amplitude: impulse,
            direction: [normal.x, normal.y, normal.z],
            wave_type: WaveType::Compressional,
            speed: 5000.0,
            attenuation: energy_dissipated,
        }
    }

    fn select_mapping(&self, _event: &ScaleEvent, transition: ScaleTransition) -> ScaleMapping {
        match transition {
            ScaleTransition::AtomicToContinuum => self
                .mapping_library
                .get("quantum_to_continuum_v1")
                .cloned()
                .unwrap_or_else(|| self.mapping_library.values().next().unwrap().clone()),
            ScaleTransition::ContinuumToKinetic => self
                .mapping_library
                .get("continuum_to_kinetic_v1")
                .cloned()
                .unwrap_or_else(|| self.mapping_library.values().next().unwrap().clone()),
            _ => self.mapping_library.values().next().unwrap().clone(),
        }
    }

    fn generate_cache_key(&self, event: &ScaleEvent, transition: ScaleTransition) -> String {
        let mut hasher = blake3::Hasher::new();
        hasher.update(&bincode::serialize(event).unwrap_or_default());
        hasher.update(format!("{:?}", transition).as_bytes());
        hasher.update(&self.rng_seed.to_le_bytes());
        hasher.finalize().to_hex().to_string()
    }

    fn calculate_energy_discrepancy(&self, quantum_energy: f64, _continuum_data: &[u8]) -> f64 {
        quantum_energy * 0.1
    }

    fn calculate_stress_energy_discrepancy(
        &self,
        stress: Matrix3<f64>,
        _atomic_forces: &[[f64; 3]],
    ) -> f64 {
        0.5 * stress.trace() * stress.trace()
    }

    fn generate_consistency_proof(
        &self,
        event: &ScaleEvent,
        mapping: &ScaleMapping,
        result_data: &[u8],
    ) -> ConsistencyProof {
        let mut hasher = blake3::Hasher::new();
        hasher.update(&bincode::serialize(event).unwrap_or_default());
        let source_hash = hasher.finalize().to_hex().to_string();

        let mut hasher = blake3::Hasher::new();
        hasher.update(&bincode::serialize(mapping).unwrap_or_default());
        let mapping_hash = hasher.finalize().to_hex().to_string();

        let mut hasher = blake3::Hasher::new();
        hasher.update(result_data);
        let result_hash = hasher.finalize().to_hex().to_string();

        ConsistencyProof {
            source_hash: source_hash.clone(),
            mapping_hash: mapping_hash.clone(),
            result_hash: result_hash.clone(),
            merkle_proof: format!("{}_{}_{}", source_hash, mapping_hash, result_hash),
            validator_signatures: Vec::new(),
        }
    }

    pub fn map_scale(
        &self,
        input_data: &[u8],
        _mapping: &ScaleMapping,
        coupling_type: &CouplingType,
    ) -> Result<Vec<u8>, String> {
        match coupling_type {
            CouplingType::QuantumToContinuum
            | CouplingType::ContinuumToKinetic
            | CouplingType::KineticToContinuum
            | CouplingType::AllToAll
            | CouplingType::Adaptive
            | CouplingType::Hierarchical { .. } => Ok(input_data.to_vec()),
        }
    }
}

// Supporting types
#[derive(Serialize, Deserialize, Clone)]
pub struct ExternalForce {
    pub body_handle: u32,
    pub force: [f32; 3],
    pub torque: [f32; 3],
    pub application_point: Option<[f32; 3]>,
}

#[derive(Serialize, Deserialize)]
pub struct StressWave {
    pub origin: [f64; 3],
    pub amplitude: f64,
    pub direction: [f64; 3],
    pub wave_type: WaveType,
    pub speed: f64,
    pub attenuation: f64,
}

#[derive(Serialize, Deserialize)]
pub enum WaveType {
    Compressional,
    Shear,
    Rayleigh,
    Love,
}

// Removed manual Default for ConsistencyProof as it is now derived

#[derive(Serialize, Deserialize, Clone, Default)]
pub struct NegotiationPerformance {
    pub time_taken_ms: f64,
    pub memory_used_bytes: u64,
    pub iterations: u32,
    pub cache_hit: bool,
}

impl NegotiationResult {
    fn failure() -> Self {
        Self {
            success: false,
            mapped_data: Vec::new(),
            energy_discrepancy: 0.0,
            momentum_discrepancy: [0.0; 3],
            consistency_proof: ConsistencyProof::default(),
            performance: NegotiationPerformance::default(),
        }
    }
}

impl Default for ScaleMapping {
    fn default() -> Self {
        Self {
            id: "default".to_string(),
            source_scale: SimulationScale {
                spatial: 1e-10,
                temporal: 1e-15,
                energy: 1.6e-19,
                fidelity: FidelityLevel::Research,
            },
            target_scale: SimulationScale {
                spatial: 1e-8,
                temporal: 1e-12,
                energy: 1.6e-18,
                fidelity: FidelityLevel::Research,
            },
            method: MappingMethod::LinearInterpolation {
                kernel_radius: 1e-9,
                shape_function: ShapeFunction::Linear,
            },
            parameters: MappingParameters {
                cutoff_radius: 5e-10,
                min_sampling: 1e27,
                preserve_symmetry: true,
                max_error: 0.01,
                budget: 1.0,
            },
            validation: MappingValidation {
                dataset_hash: "default".to_string(),
                rms_error: 0.0,
                max_error: 0.0,
                conservation_error: 0.0,
                proof: MappingProof {
                    merkle_root: "default".to_string(),
                    validator_nodes: vec![],
                    consensus_reached: false,
                    error_bounds: ErrorBounds {
                        lower: 0.0,
                        upper: 0.0,
                        confidence: 0.0,
                    },
                },
            },
        }
    }
}
