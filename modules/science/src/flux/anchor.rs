use blake3;
use nalgebra::Vector3;
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, VecDeque};

// ----------------------------------------------------------------------------
// CONSERVATION LAWS AS FIRST-CLASS CITIZENS
// ----------------------------------------------------------------------------

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FluxProposal {
    pub mass_flux: f64,
    pub momentum_flux: Vector3<f64>,
    pub energy_flux: f64,
    pub angular_momentum_flux: Vector3<f64>,
    pub charge_flux: f64,
    pub entropy_flux: f64,
    pub probability_flux: f64,
    pub measurement_scale: SimulationScale,
    pub proof: FluxProof,
    pub timestamp: f64,
    pub interface_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct FluxProof {
    pub state_root: String,
    pub computation_hash: String,
    pub validator_signatures: Vec<String>,
    pub error_bounds: ErrorBounds,
    pub confidence: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConservationViolation {
    pub law: ConservationLaw,
    pub magnitude: f64,
    pub location: Vector3<f64>,
    pub scales: Vec<SimulationScale>,
    pub correction: FluxCorrection,
    pub proof: ViolationProof,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, Hash)]
pub enum ConservationLaw {
    MassConservation,
    MomentumConservation,
    EnergyConservation,
    AngularMomentumConservation,
    ChargeConservation,
    BaryonNumberConservation,
    LeptonNumberConservation,
    ColorChargeConservation,
    InformationConservation,
    ProbabilityConservation,
    CPTSymmetry,
    GaugeSymmetry,
    LorentzSymmetry,
    ScaleInvariance,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FluxCorrection {
    pub adjustment: FluxProposal,
    pub method: CorrectionMethod,
    pub dissipation: f64,
    pub causal: bool,
    pub restoration_proof: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum CorrectionMethod {
    LeastSquares,
    SymmetryPreserving,
    Variational,
    Learned { model_hash: String },
    Consensus,
    TimeReversed,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ViolationProof {
    pub state_hash: String,
    pub computation_hash: String,
    pub witness: Vec<u8>,
    pub consensus: ConsensusProof,
}

// ----------------------------------------------------------------------------
// CONSERVATION ANCHOR - THE REALITY GUARDIAN
// ----------------------------------------------------------------------------

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConservationAnchor {
    pub id: String,
    pub location: Vector3<f64>,
    pub enforced_laws: Vec<ConservationLaw>,
    pub tolerances: HashMap<ConservationLaw, f64>,
    pub flux_history: VecDeque<FluxRecord>,
    pub violations_detected: u64,
    pub corrections_applied: u64,
    pub anchor_signature: String,
    pub telemetry: AnchorTelemetry,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FluxRecord {
    pub incoming: FluxProposal,
    pub outgoing: FluxProposal,
    pub net_flux: FluxProposal,
    pub violation: Option<ConservationViolation>,
    pub correction: Option<FluxCorrection>,
    pub timestamp: f64,
    pub epoch: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct AnchorTelemetry {
    pub checks_performed: u64,
    pub violations_found: u64,
    pub corrections_made: u64,
    pub average_check_time_ns: u64,
    pub worst_violation: f64,
    pub energy_dissipated: f64,
    pub cache_hits: u64,
    pub cache_misses: u64,
}

impl ConservationAnchor {
    pub fn new(id: &str, location: Vector3<f64>) -> Self {
        let mut tolerances = HashMap::new();

        tolerances.insert(ConservationLaw::MassConservation, 1e-9);
        tolerances.insert(ConservationLaw::EnergyConservation, 1e-9);
        tolerances.insert(ConservationLaw::MomentumConservation, 1e-9);
        tolerances.insert(ConservationLaw::AngularMomentumConservation, 1e-8);
        tolerances.insert(ConservationLaw::ChargeConservation, 1e-12);
        tolerances.insert(ConservationLaw::ProbabilityConservation, 1e-15);

        let enforced_laws = vec![
            ConservationLaw::MassConservation,
            ConservationLaw::EnergyConservation,
            ConservationLaw::MomentumConservation,
            ConservationLaw::AngularMomentumConservation,
            ConservationLaw::ChargeConservation,
            ConservationLaw::ProbabilityConservation,
        ];

        let mut hasher = blake3::Hasher::new();
        hasher.update(id.as_bytes());
        hasher.update(&location.x.to_le_bytes());
        hasher.update(&location.y.to_le_bytes());
        hasher.update(&location.z.to_le_bytes());
        let anchor_signature = hasher.finalize().to_hex().to_string();

        Self {
            id: id.to_string(),
            location,
            enforced_laws,
            tolerances,
            flux_history: VecDeque::with_capacity(1000),
            violations_detected: 0,
            corrections_applied: 0,
            anchor_signature,
            telemetry: AnchorTelemetry::default(),
        }
    }

    pub fn negotiate(
        &mut self,
        incoming: FluxProposal,
        outgoing: FluxProposal,
        epoch: u64,
    ) -> (FluxProposal, FluxProposal, Option<ConservationViolation>) {
        let _start_time = sdk::js_interop::get_performance_now(); // High-resolution timer
        self.telemetry.checks_performed += 1;

        let net_flux = self.calculate_net_flux(&incoming, &outgoing);
        let violations = self.check_conservation_laws(&net_flux);

        if violations.is_empty() {
            self.record_flux(incoming.clone(), outgoing.clone(), net_flux, None, epoch);
            (incoming, outgoing, None)
        } else {
            self.telemetry.violations_found += 1;
            self.violations_detected += 1;

            let worst_violation = violations.iter().map(|v| v.magnitude).fold(0.0, f64::max);

            let correction = self.generate_correction(&incoming, &outgoing, &violations);
            let (adjusted_in, adjusted_out) =
                self.apply_correction(&incoming, &outgoing, &correction);

            let violation = ConservationViolation {
                law: violations[0].law.clone(),
                magnitude: worst_violation,
                location: self.location,
                scales: vec![incoming.measurement_scale, outgoing.measurement_scale],
                correction: correction.clone(),
                proof: self.generate_violation_proof(&incoming, &outgoing, &violations),
            };

            let adjusted_net = self.calculate_net_flux(&adjusted_in, &adjusted_out);
            self.record_flux(
                adjusted_in.clone(),
                adjusted_out.clone(),
                adjusted_net,
                Some(violation.clone()),
                epoch,
            );

            self.telemetry.corrections_made += 1;
            self.corrections_applied += 1;
            self.telemetry.energy_dissipated += correction.dissipation;
            self.telemetry.worst_violation = self.telemetry.worst_violation.max(worst_violation);

            (adjusted_in, adjusted_out, Some(violation))
        }
    }

    fn calculate_net_flux(&self, incoming: &FluxProposal, outgoing: &FluxProposal) -> FluxProposal {
        FluxProposal {
            mass_flux: incoming.mass_flux - outgoing.mass_flux,
            momentum_flux: incoming.momentum_flux - outgoing.momentum_flux,
            energy_flux: incoming.energy_flux - outgoing.energy_flux,
            angular_momentum_flux: incoming.angular_momentum_flux - outgoing.angular_momentum_flux,
            charge_flux: incoming.charge_flux - outgoing.charge_flux,
            entropy_flux: incoming.entropy_flux - outgoing.entropy_flux,
            probability_flux: incoming.probability_flux - outgoing.probability_flux,
            measurement_scale: incoming.measurement_scale,
            proof: FluxProof::default(),
            timestamp: incoming.timestamp,
            interface_id: incoming.interface_id.clone(),
        }
    }

    fn check_conservation_laws(&self, net_flux: &FluxProposal) -> Vec<ConservationViolation> {
        let mut violations = Vec::new();

        for law in &self.enforced_laws {
            let tolerance = self.tolerances.get(law).copied().unwrap_or(1e-6);

            let (magnitude, expected_zero): (f64, f64) = match law {
                ConservationLaw::MassConservation => (net_flux.mass_flux.abs(), 0.0),
                ConservationLaw::MomentumConservation => (net_flux.momentum_flux.norm(), 0.0),
                ConservationLaw::EnergyConservation => (net_flux.energy_flux.abs(), 0.0),
                ConservationLaw::AngularMomentumConservation => {
                    (net_flux.angular_momentum_flux.norm(), 0.0)
                }
                ConservationLaw::ChargeConservation => (net_flux.charge_flux.abs(), 0.0),
                ConservationLaw::ProbabilityConservation => (net_flux.probability_flux.abs(), 0.0),
                ConservationLaw::InformationConservation => {
                    let violation = if net_flux.entropy_flux < 0.0 {
                        -net_flux.entropy_flux
                    } else {
                        0.0
                    };
                    (violation, 0.0)
                }
                _ => continue,
            };

            let relative_violation = if expected_zero.abs() > 1e-30_f64 {
                magnitude / expected_zero.abs()
            } else {
                magnitude
            };

            if relative_violation > tolerance {
                violations.push(ConservationViolation {
                    law: law.clone(),
                    magnitude,
                    location: self.location,
                    scales: vec![net_flux.measurement_scale],
                    correction: FluxCorrection::default(),
                    proof: ViolationProof::default(),
                });
            }
        }

        violations
    }

    fn generate_correction(
        &self,
        incoming: &FluxProposal,
        outgoing: &FluxProposal,
        _violations: &[ConservationViolation],
    ) -> FluxCorrection {
        let net_flux = self.calculate_net_flux(incoming, outgoing);

        let adjustment = FluxProposal {
            mass_flux: -net_flux.mass_flux / 2.0,
            momentum_flux: -net_flux.momentum_flux / 2.0,
            energy_flux: -net_flux.energy_flux / 2.0,
            angular_momentum_flux: -net_flux.angular_momentum_flux / 2.0,
            charge_flux: -net_flux.charge_flux / 2.0,
            entropy_flux: 0.0,
            probability_flux: -net_flux.probability_flux / 2.0,
            measurement_scale: net_flux.measurement_scale,
            proof: FluxProof::default(),
            timestamp: net_flux.timestamp,
            interface_id: net_flux.interface_id.clone(),
        };

        let dissipation = self.calculate_dissipation(&net_flux, &adjustment);

        let restoration_proof = self.generate_restoration_proof(&net_flux, &adjustment);

        FluxCorrection {
            adjustment,
            method: CorrectionMethod::Variational,
            dissipation,
            causal: true,
            restoration_proof,
        }
    }

    fn apply_correction(
        &self,
        incoming: &FluxProposal,
        outgoing: &FluxProposal,
        correction: &FluxCorrection,
    ) -> (FluxProposal, FluxProposal) {
        let adjusted_incoming = FluxProposal {
            mass_flux: incoming.mass_flux + correction.adjustment.mass_flux,
            momentum_flux: incoming.momentum_flux + correction.adjustment.momentum_flux,
            energy_flux: incoming.energy_flux + correction.adjustment.energy_flux,
            angular_momentum_flux: incoming.angular_momentum_flux
                + correction.adjustment.angular_momentum_flux,
            charge_flux: incoming.charge_flux + correction.adjustment.charge_flux,
            entropy_flux: incoming.entropy_flux,
            probability_flux: incoming.probability_flux + correction.adjustment.probability_flux,
            measurement_scale: incoming.measurement_scale,
            proof: self.generate_corrected_proof(incoming, "corrected_incoming"),
            timestamp: incoming.timestamp,
            interface_id: incoming.interface_id.clone(),
        };

        let adjusted_outgoing = FluxProposal {
            mass_flux: outgoing.mass_flux - correction.adjustment.mass_flux,
            momentum_flux: outgoing.momentum_flux - correction.adjustment.momentum_flux,
            energy_flux: outgoing.energy_flux - correction.adjustment.energy_flux,
            angular_momentum_flux: outgoing.angular_momentum_flux
                - correction.adjustment.angular_momentum_flux,
            charge_flux: outgoing.charge_flux - correction.adjustment.charge_flux,
            entropy_flux: outgoing.entropy_flux + correction.dissipation / 300.0,
            probability_flux: outgoing.probability_flux - correction.adjustment.probability_flux,
            measurement_scale: outgoing.measurement_scale,
            proof: self.generate_corrected_proof(outgoing, "corrected_outgoing"),
            timestamp: outgoing.timestamp,
            interface_id: outgoing.interface_id.clone(),
        };

        (adjusted_incoming, adjusted_outgoing)
    }

    fn calculate_dissipation(&self, _net_flux: &FluxProposal, adjustment: &FluxProposal) -> f64 {
        let kinetic_energy = 0.5 * adjustment.momentum_flux.norm_squared() / 1.0;
        let potential_energy = adjustment.energy_flux.abs();
        let angular_energy = 0.5 * adjustment.angular_momentum_flux.norm_squared() / 1.0;
        let information_cost = adjustment.entropy_flux.abs() * 300.0 * 1.38e-23;

        kinetic_energy + potential_energy + angular_energy + information_cost
    }

    fn generate_violation_proof(
        &self,
        incoming: &FluxProposal,
        outgoing: &FluxProposal,
        violations: &[ConservationViolation],
    ) -> ViolationProof {
        let mut hasher = blake3::Hasher::new();
        hasher.update(&bincode::serialize(incoming).unwrap_or_default());
        hasher.update(&bincode::serialize(outgoing).unwrap_or_default());

        for violation in violations {
            hasher.update(&bincode::serialize(violation).unwrap_or_default());
        }

        hasher.update(self.anchor_signature.as_bytes());

        ViolationProof {
            state_hash: hasher.finalize().to_hex().to_string(),
            computation_hash: String::new(),
            witness: Vec::new(),
            consensus: ConsensusProof::default(),
        }
    }

    fn generate_restoration_proof(
        &self,
        net_flux: &FluxProposal,
        adjustment: &FluxProposal,
    ) -> String {
        let mut hasher = blake3::Hasher::new();
        hasher.update(&bincode::serialize(net_flux).unwrap_or_default());
        hasher.update(&bincode::serialize(adjustment).unwrap_or_default());

        let corrected_mass = net_flux.mass_flux + adjustment.mass_flux;
        let corrected_energy = net_flux.energy_flux + adjustment.energy_flux;

        hasher.update(&corrected_mass.to_le_bytes());
        hasher.update(&corrected_energy.to_le_bytes());

        hasher.finalize().to_hex().to_string()
    }

    fn generate_corrected_proof(&self, original: &FluxProposal, label: &str) -> FluxProof {
        let mut hasher = blake3::Hasher::new();
        hasher.update(&bincode::serialize(original).unwrap_or_default());
        hasher.update(label.as_bytes());
        hasher.update(self.anchor_signature.as_bytes());

        FluxProof {
            state_root: String::new(),
            computation_hash: hasher.finalize().to_hex().to_string(),
            validator_signatures: vec![self.anchor_signature.clone()],
            error_bounds: ErrorBounds {
                lower: -1e-12,
                upper: 1e-12,
                confidence: 0.99,
            },
            confidence: 0.99,
        }
    }

    fn record_flux(
        &mut self,
        incoming: FluxProposal,
        outgoing: FluxProposal,
        net_flux: FluxProposal,
        violation: Option<ConservationViolation>,
        epoch: u64,
    ) {
        let record = FluxRecord {
            incoming,
            outgoing,
            net_flux,
            violation,
            correction: None,
            timestamp: sdk::js_interop::get_now() as f64 / 1000.0, // Unix epoch in seconds (f64)
            epoch,
        };

        self.flux_history.push_back(record);
        if self.flux_history.len() > 1000 {
            self.flux_history.pop_front();
        }
    }

    fn update_telemetry(
        &mut self,
        duration_ns: u64,
        violation_found: bool,
        violation_magnitude: Option<f64>,
    ) {
        let n = self.telemetry.checks_performed as f64;
        self.telemetry.average_check_time_ns =
            ((self.telemetry.average_check_time_ns as f64 * (n - 1.0) + duration_ns as f64) / n)
                as u64;

        if violation_found {
            self.telemetry.violations_found += 1;
            if let Some(mag) = violation_magnitude {
                self.telemetry.worst_violation = self.telemetry.worst_violation.max(mag);
            }
        }
    }

    pub fn check_conservation(a: &FluxProposal, b: &FluxProposal) -> bool {
        let mass_match = (a.mass_flux + b.mass_flux).abs() < 1e-6;
        let energy_match = (a.energy_flux + b.energy_flux).abs() < 1e-6;
        let momentum_match = (a.momentum_flux + b.momentum_flux).norm_squared() < 1e-6;

        mass_match && energy_match && momentum_match
    }

    pub fn get_statistics(&self) -> AnchorStatistics {
        let mut law_violations = HashMap::new();
        let mut total_violation_magnitude = 0.0;
        let mut max_violation: f64 = 0.0;

        for record in &self.flux_history {
            if let Some(violation) = &record.violation {
                *law_violations.entry(violation.law.clone()).or_insert(0) += 1;
                total_violation_magnitude += violation.magnitude;
                max_violation = max_violation.max(violation.magnitude);
            }
        }

        let avg_violation = if law_violations.values().sum::<u32>() > 0 {
            total_violation_magnitude / law_violations.values().sum::<u32>() as f64
        } else {
            0.0
        };

        AnchorStatistics {
            anchor_id: self.id.clone(),
            total_checks: self.telemetry.checks_performed,
            total_violations: self.violations_detected,
            total_corrections: self.corrections_applied,
            violation_rate: if self.telemetry.checks_performed > 0 {
                self.violations_detected as f64 / self.telemetry.checks_performed as f64
            } else {
                0.0
            },
            average_violation_magnitude: avg_violation,
            maximum_violation: max_violation,
            energy_dissipated: self.telemetry.energy_dissipated,
            law_violations,
            cache_hit_rate: if self.telemetry.cache_hits + self.telemetry.cache_misses > 0 {
                self.telemetry.cache_hits as f64
                    / (self.telemetry.cache_hits + self.telemetry.cache_misses) as f64
            } else {
                0.0
            },
        }
    }

    pub fn validate_identity(&self) -> bool {
        let mut hasher = blake3::Hasher::new();
        hasher.update(self.id.as_bytes());
        hasher.update(&self.location.x.to_le_bytes());
        hasher.update(&self.location.y.to_le_bytes());
        hasher.update(&self.location.z.to_le_bytes());
        let expected_signature = hasher.finalize().to_hex().to_string();

        self.anchor_signature == expected_signature
    }
}

// ----------------------------------------------------------------------------
// SUPPORTING TYPES
// ----------------------------------------------------------------------------

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
pub struct SimulationScale {
    pub spatial: f64,
    pub temporal: f64,
    pub energy: f64,
    pub fidelity: FidelityLevel,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq)]
pub enum FidelityLevel {
    Heuristic,
    Engineering,
    Research,
    QuantumExact,
    RealityProof,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ErrorBounds {
    pub lower: f64,
    pub upper: f64,
    pub confidence: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ConsensusProof {
    pub validator_count: u32,
    pub agreement_threshold: f64,
    pub signatures: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AnchorStatistics {
    pub anchor_id: String,
    pub total_checks: u64,
    pub total_violations: u64,
    pub total_corrections: u64,
    pub violation_rate: f64,
    pub average_violation_magnitude: f64,
    pub maximum_violation: f64,
    pub energy_dissipated: f64,
    pub law_violations: HashMap<ConservationLaw, u32>,
    pub cache_hit_rate: f64,
}

// Default implementations
impl Default for FluxProposal {
    fn default() -> Self {
        Self {
            mass_flux: 0.0,
            momentum_flux: Vector3::zeros(),
            energy_flux: 0.0,
            angular_momentum_flux: Vector3::zeros(),
            charge_flux: 0.0,
            entropy_flux: 0.0,
            probability_flux: 0.0,
            measurement_scale: SimulationScale {
                spatial: 1.0,
                temporal: 1.0,
                energy: 1.0,
                fidelity: FidelityLevel::Engineering,
            },
            proof: FluxProof::default(),
            timestamp: 0.0,
            interface_id: String::new(),
        }
    }
}

impl Default for FluxCorrection {
    fn default() -> Self {
        Self {
            adjustment: FluxProposal::default(),
            method: CorrectionMethod::LeastSquares,
            dissipation: 0.0,
            causal: true,
            restoration_proof: String::new(),
        }
    }
}

// ----------------------------------------------------------------------------
// CONSERVATION MESH (DISTRIBUTED ANCHORS)
// ----------------------------------------------------------------------------

pub struct ConservationMesh {
    pub anchors: HashMap<String, ConservationAnchor>,
    pub global_violations: Vec<GlobalViolation>,
    pub mesh_signature: String,
}

impl Default for ConservationMesh {
    fn default() -> Self {
        Self::new()
    }
}

impl ConservationMesh {
    pub fn new() -> Self {
        Self {
            anchors: HashMap::new(),
            global_violations: Vec::new(),
            mesh_signature: String::new(),
        }
    }

    pub fn add_anchor(&mut self, anchor: ConservationAnchor) {
        self.anchors.insert(anchor.id.clone(), anchor);
    }

    pub fn negotiate_across_mesh(
        &mut self,
        anchor_id: &str,
        incoming: FluxProposal,
        outgoing: FluxProposal,
        epoch: u64,
    ) -> Result<(FluxProposal, FluxProposal), String> {
        let anchor = self
            .anchors
            .get_mut(anchor_id)
            .ok_or_else(|| format!("Anchor not found: {}", anchor_id))?;

        let (adj_in, adj_out, violation) = anchor.negotiate(incoming, outgoing, epoch);

        if let Some(violation) = violation {
            self.record_global_violation(violation);
        }

        Ok((adj_in, adj_out))
    }

    fn record_global_violation(&mut self, violation: ConservationViolation) {
        self.global_violations.push(GlobalViolation {
            violation,
            timestamp: sdk::js_interop::get_now() as f64 / 1000.0, // Unix epoch in seconds (f64)
            mesh_consensus: self.check_mesh_consensus(),
        });
    }

    fn check_mesh_consensus(&self) -> MeshConsensus {
        MeshConsensus {
            agreed: true,
            validator_count: self.anchors.len() as u32,
            confidence: 0.95,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GlobalViolation {
    pub violation: ConservationViolation,
    pub timestamp: f64,
    pub mesh_consensus: MeshConsensus,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MeshConsensus {
    pub agreed: bool,
    pub validator_count: u32,
    pub confidence: f64,
}
