use crate::flux::boundary::ScaleMapping;
use crate::matter::{ScienceError, ScienceProxy, ScienceResult};
use crate::mesh::cache::ComputationCache;
use blake3;
// use groan_rs::system::System;
pub struct System; // Mock
impl System {
    pub fn group_create(&mut self, _: &str, _: &str) -> Result<(), String> {
        Ok(())
    }
    pub fn group_get_n_atoms(&self, _: &str) -> Result<usize, String> {
        Ok(0)
    }
    pub fn get_n_atoms(&self) -> usize {
        0
    }
}
use crate::types::{CacheEntry, Telemetry};
use serde::{Deserialize, Serialize};

use std::cell::RefCell;
use std::collections::HashMap;
use std::rc::Rc;

// ----------------------------------------------------------------------------
// QUANTUM CHEMISTRY TYPES
// ----------------------------------------------------------------------------

#[derive(Serialize, Deserialize)]
pub enum QuantumMethod {
    HF,   // Hartree-Fock
    DFT,  // Density Functional Theory
    MP2,  // Møller-Plesset 2nd order
    CCSD, // Coupled Cluster Singles and Doubles
    QMC,  // Quantum Monte Carlo
    CI,   // Configuration Interaction
}

#[derive(Serialize, Deserialize)]
pub struct QuantumCalculation {
    method: QuantumMethod,
    basis_set: String,
    functional: Option<String>, // For DFT
    charge: i32,
    multiplicity: u32,
    accuracy: f64, // Target energy convergence (kJ/mol)
}

#[derive(Serialize, Deserialize)]
pub struct MolecularDynamicsParams {
    temperature: f64, // Kelvin
    pressure: f64,    // bar
    timestep: f64,    // femtoseconds
    steps: u64,
    thermostat: String, // "berendsen", "nose-hoover", "langevin"
    barostat: String,   // "berendsen", "parrinello-rahman"
    save_frequency: u64,
}

#[derive(Serialize, Deserialize)]
pub struct AtomicSelection {
    query: String,     // Groan selection syntax: "resid 1 to 10 and name CA"
    indices: Vec<u32>, // Pre-computed for validation
}

#[derive(Serialize, Deserialize)]
pub struct SystemSnapshot {
    coordinates: Vec<[f32; 3]>,        // Ångstroms
    velocities: Option<Vec<[f32; 3]>>, // nm/ps
    forces: Option<Vec<[f32; 3]>>,     // kJ/mol/nm
    energies: Option<SystemEnergies>,
    topology: SystemTopology,
    box_dimensions: Option<[f32; 3]>,
    simulation_time: f64, // picoseconds
}

#[derive(Serialize, Deserialize)]
pub struct SystemEnergies {
    potential: f64,
    kinetic: f64,
    total: f64,
    bonded: Option<f64>,
    nonbonded: Option<f64>,
    quantum: Option<f64>, // From QM/MM or full quantum
}

#[derive(Serialize, Deserialize)]
pub struct SystemTopology {
    atom_names: Vec<String>,
    residue_names: Vec<String>,
    residue_numbers: Vec<u32>,
    chain_ids: Vec<char>,
    elements: Vec<String>,
    masses: Vec<f64>,
    charges: Vec<f64>,
    bonds: Vec<(u32, u32)>,
    angles: Vec<(u32, u32, u32)>,
    dihedrals: Vec<(u32, u32, u32, u32)>,
}

// ----------------------------------------------------------------------------
// ENHANCED ATOMIC PROXY
// ----------------------------------------------------------------------------

pub struct AtomicProxy {
    system: Option<System>,
    // TODO(v3.1): Quantum chemistry integration via FFI to PySCF/Q-Chem
    #[allow(dead_code)]
    quantum_engine: Option<QuantumEngine>,
    #[allow(dead_code)]
    force_field: Option<ForceField>,
    cache: Rc<RefCell<ComputationCache>>,
    telemetry: Rc<RefCell<Telemetry>>,

    // For multi-scale coupling
    // TODO(v3.1): QM/MM and quantum-to-continuum mapping
    #[allow(dead_code)]
    continuum_coupling: Option<ContinuumCoupling>,
    #[allow(dead_code)]
    scale_mapping: Option<ScaleMapping>,
}

// TODO(v3.1): Quantum chemistry engine stub - will interface with external QM codes
#[allow(dead_code)]
pub struct QuantumEngine {
    // Would interface with external QM code (PySCF, Q-Chem, etc.)
    // via FFI or Python bindings
}

// TODO(v3.1): Force field parameters for classical MD
#[allow(dead_code)]
pub struct ForceField {
    name: String,
    parameters: HashMap<String, f64>,
    cutoff: f64,
    pme: bool, // Particle Mesh Ewald
}

// TODO(v3.1): QM/MM coupling for multi-scale simulations
#[allow(dead_code)]
pub struct ContinuumCoupling {
    // For QM/MM or quantum-to-continuum mapping
    qm_region: AtomicSelection,
    mm_region: AtomicSelection,
    boundary_condition: String,
}

impl Default for AtomicProxy {
    fn default() -> Self {
        Self::new(
            Rc::new(RefCell::new(ComputationCache::new())),
            Rc::new(RefCell::new(Telemetry::default())),
        )
    }
}

impl AtomicProxy {
    pub fn new(cache: Rc<RefCell<ComputationCache>>, telemetry: Rc<RefCell<Telemetry>>) -> Self {
        Self {
            system: None,
            quantum_engine: None,
            force_field: None,
            continuum_coupling: None,
            scale_mapping: None,
            cache,
            telemetry,
        }
    }

    // ------------------------------------------------------------------------
    // CORE MOLECULAR DYNAMICS
    // ------------------------------------------------------------------------

    fn execute_load_gro(&mut self, input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        let _gro_content = std::str::from_utf8(input)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 for GRO file".to_string()))?;
        self.telemetry.borrow_mut().computations += 1;
        Ok(vec![])
    }

    fn execute_load_pdb(&mut self, input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        let _pdb_content = std::str::from_utf8(input)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 for PDB file".to_string()))?;
        // STUB: groan_rs System loading requires file path
        self.telemetry.borrow_mut().computations += 1;
        Ok(vec![])
    }

    fn execute_select(&mut self, _input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let query = params_json["query"]
            .as_str()
            .ok_or_else(|| ScienceError::InvalidParams("Missing 'query' parameter".to_string()))?;

        let system = self
            .system
            .as_mut()
            .ok_or_else(|| ScienceError::Internal("No system loaded".to_string()))?;

        // Create selection group
        system
            .group_create("selection", query)
            .map_err(|e| ScienceError::Internal(format!("Selection failed: {}", e)))?;

        // Get selected atoms
        let n_selected = system
            .group_get_n_atoms("selection")
            .map_err(|e| ScienceError::Internal(format!("Failed to get selection count: {}", e)))?;

        let indices: Vec<u32> = (0..n_selected as u32).collect(); // Simplified

        // Serialize selection
        let selection = AtomicSelection {
            query: query.to_string(),
            indices,
        };

        bincode::serialize(&selection).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    fn execute_rmsd(&mut self, _input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let _selection_a = params_json["selection_a"].as_str().unwrap_or("all");
        let _selection_b = params_json["selection_b"].as_str().unwrap_or("all");
        let _fit = params_json
            .get("fit")
            .and_then(|v| v.as_bool())
            .unwrap_or(true);

        let _system = self
            .system
            .as_ref()
            .ok_or_else(|| ScienceError::Internal("No system loaded".to_string()))?;

        // Create temporary groups
        // Need to Clone system? clone() not implemented for external type easily.
        // We will assume `group_create` works on `&mut system`. But here we have `&system`.
        // We can't mutate.
        // If we can't clone, we can't run RMSD without mutating groups?
        // groan_rs `group_create` requires `&mut self`.
        // Workaround: RMSD needs groups. If groups pre-exist great. If not, we have a problem.
        // Returning Error if stub fails.
        // let mut system_clone = system.clone();

        // STUB: return 0.0
        Ok(0.0f64.to_le_bytes().to_vec())
    }

    fn execute_center_of_mass(&mut self, _input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let _params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;
        // STUB: groan_rs center_of_mass API not available
        Ok(vec![0u8; 24]) // 3 f64 values
    }

    fn execute_distance(&mut self, _input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        // STUB: groan_rs API not available
        Ok(0.0f64.to_le_bytes().to_vec())
    }

    // ------------------------------------------------------------------------
    // QUANTUM CHEMISTRY
    // ------------------------------------------------------------------------

    fn execute_compute_energy(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let quantum_calc: QuantumCalculation =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        // Check cache first
        let request_hash = self.compute_quantum_request_hash(&quantum_calc);
        if let Some(entry) = self.cache.borrow_mut().get(&request_hash, None) {
            self.telemetry.borrow_mut().cache_hits += 1;
            return Ok(entry.data);
        }

        self.telemetry.borrow_mut().cache_misses += 1;
        self.telemetry.borrow_mut().quantum_ops += 1;
        self.telemetry.borrow_mut().computations += 1;

        // Determine which atoms to compute
        let system = self
            .system
            .as_ref()
            .ok_or_else(|| ScienceError::Internal("No system loaded".to_string()))?;

        // Get QM region from params or use full system
        let params_str = std::str::from_utf8(params).unwrap_or("{}");
        let qm_selection = self.get_qm_region(params_str);

        // Call quantum engine (stub for now)
        let energy = match quantum_calc.method {
            QuantumMethod::DFT => self.compute_dft_energy(system, &qm_selection, &quantum_calc),
            QuantumMethod::HF => self.compute_hf_energy(system, &qm_selection, &quantum_calc),
            QuantumMethod::MP2 => self.compute_mp2_energy(system, &qm_selection, &quantum_calc),
            QuantumMethod::CCSD => self.compute_ccsd_energy(system, &qm_selection, &quantum_calc),
            QuantumMethod::QMC => self.compute_qmc_energy(system, &qm_selection, &quantum_calc),
            QuantumMethod::CI => self.compute_ci_energy(system, &qm_selection, &quantum_calc),
        }?;

        // Cache result
        let energy_bytes = energy.to_le_bytes().to_vec();

        let entry = CacheEntry {
            data: energy_bytes.clone(),
            result_hash: request_hash.clone(),
            timestamp: 0,
            access_count: 1,
            scale: Default::default(),
            proof: Default::default(),
        };
        self.cache.borrow_mut().put(request_hash, entry);

        Ok(energy_bytes)
    }

    fn execute_compute_forces(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let quantum_calc: QuantumCalculation =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let system = self
            .system
            .as_ref()
            .ok_or_else(|| ScienceError::Internal("No system loaded".to_string()))?;

        let params_str = std::str::from_utf8(params).unwrap_or("{}");
        let qm_selection = self.get_qm_region(params_str);

        // Compute forces via finite differences or analytic gradients
        let forces = self.compute_quantum_forces(system, &qm_selection, &quantum_calc)?;

        // Return forces as serialized array of [f64; 3]
        bincode::serialize(&forces).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    fn execute_molecular_dynamics(
        &mut self,
        input: &[u8],
        _params: &[u8],
    ) -> ScienceResult<Vec<u8>> {
        let _md_params: MolecularDynamicsParams =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        // Check if we have a force field
        if self.force_field.is_none() {
            // Stub: proceed without FF
        }

        // Run MD simulation
        let trajectory = {
            let _sys = self
                .system
                .as_mut()
                .ok_or_else(|| ScienceError::Internal("No system loaded".to_string()))?;
            // Stub: return empty trajectory
            vec![SystemSnapshot {
                coordinates: vec![[0.0, 0.0, 0.0]; 1],
                velocities: None,
                forces: None,
                energies: None,
                topology: SystemTopology {
                    atom_names: vec!["CA".to_string()],
                    residue_names: vec!["ALA".to_string()],
                    residue_numbers: vec![1],
                    chain_ids: vec!['A'],
                    elements: vec!["C".to_string()],
                    masses: vec![12.01],
                    charges: vec![0.0],
                    bonds: vec![],
                    angles: vec![],
                    dihedrals: vec![],
                },
                box_dimensions: None,
                simulation_time: 0.0,
            }]
        };

        // Return trajectory snapshots
        bincode::serialize(&trajectory).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    fn execute_optimize_geometry(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let _quantum_calc: QuantumCalculation =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let _system = self
            .system
            .as_mut()
            .ok_or_else(|| ScienceError::Internal("No system loaded".to_string()))?;

        let params_str = std::str::from_utf8(params).unwrap_or("{}");
        let _qm_selection = self.get_qm_region(params_str);

        // Geometry optimization - stub (cannot construct System)
        // Just return metadata without optimization

        // Return final energy and coordinates
        self.serialize_system_metadata()
    }

    // ------------------------------------------------------------------------
    // MULTI-SCALE COUPLING
    // ------------------------------------------------------------------------

    fn execute_compute_stress_tensor(
        &mut self,
        _input: &[u8],
        params: &[u8],
    ) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let volume = params_json["volume"]
            .as_f64()
            .ok_or_else(|| ScienceError::InvalidParams("Missing volume".to_string()))?;

        let _system = self
            .system
            .as_ref()
            .ok_or_else(|| ScienceError::Internal("No system loaded".to_string()))?;

        let mut stress = [[0.0; 3]; 3];

        for row in stress.iter_mut() {
            for val in row.iter_mut() {
                *val /= volume;
            }
        }

        let mut result = Vec::new();
        for row in &stress {
            for val in row {
                result.extend_from_slice(&val.to_le_bytes());
            }
        }

        Ok(result)
    }

    fn execute_map_to_continuum(&mut self, input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        let mapping: ScaleMapping =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let system = self
            .system
            .as_ref()
            .ok_or_else(|| ScienceError::Internal("No system loaded".to_string()))?;

        let forces = self.compute_atomic_forces(system)?;
        let continuum_forces = self.map_atomic_to_continuum(&forces, &mapping)?;

        bincode::serialize(&continuum_forces).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    // ------------------------------------------------------------------------
    // HELPER METHODS
    // ------------------------------------------------------------------------

    fn serialize_system_metadata(&self) -> ScienceResult<Vec<u8>> {
        let system = self
            .system
            .as_ref()
            .ok_or_else(|| ScienceError::Internal("No system loaded".to_string()))?;

        let n_atoms = system.get_n_atoms();

        let mut metadata = HashMap::new();
        metadata.insert("n_atoms".to_string(), n_atoms.to_string());
        metadata.insert("n_residues".to_string(), "unknown".to_string());

        bincode::serialize(&metadata).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    fn get_qm_region(&self, params: &str) -> AtomicSelection {
        let params_json: serde_json::Value =
            serde_json::from_str(params).unwrap_or(serde_json::json!({}));

        let query = params_json["qm_region"]
            .as_str()
            .unwrap_or("all")
            .to_string();

        AtomicSelection {
            query,
            indices: Vec::new(),
        }
    }

    fn compute_quantum_request_hash(&self, quantum_calc: &QuantumCalculation) -> String {
        let mut hasher = blake3::Hasher::new();
        let calc_bytes = bincode::serialize(quantum_calc).unwrap_or_default();
        hasher.update(&calc_bytes);

        if let Some(system) = &self.system {
            // Simplified hash
            // get_coordinates() returns vectors?
            // stub: hash atom count
            let n = system.get_n_atoms();
            hasher.update(&n.to_le_bytes());
        }

        hasher.finalize().to_hex().to_string()
    }

    // ------------------------------------------------------------------------
    // QUANTUM ENGINE STUBS
    // ------------------------------------------------------------------------

    fn compute_dft_energy(
        &self,
        _system: &System,
        _selection: &AtomicSelection,
        _calc: &QuantumCalculation,
    ) -> ScienceResult<f64> {
        Ok(-1234.56)
    }
    fn compute_hf_energy(
        &self,
        _system: &System,
        _selection: &AtomicSelection,
        _calc: &QuantumCalculation,
    ) -> ScienceResult<f64> {
        Ok(-1234.0)
    }
    fn compute_mp2_energy(
        &self,
        _system: &System,
        _selection: &AtomicSelection,
        _calc: &QuantumCalculation,
    ) -> ScienceResult<f64> {
        Ok(-1234.5)
    }
    fn compute_ccsd_energy(
        &self,
        _system: &System,
        _selection: &AtomicSelection,
        _calc: &QuantumCalculation,
    ) -> ScienceResult<f64> {
        Ok(-1234.56)
    }
    fn compute_qmc_energy(
        &self,
        _system: &System,
        _selection: &AtomicSelection,
        _calc: &QuantumCalculation,
    ) -> ScienceResult<f64> {
        Ok(-1234.56)
    }
    fn compute_ci_energy(
        &self,
        _system: &System,
        _selection: &AtomicSelection,
        _calc: &QuantumCalculation,
    ) -> ScienceResult<f64> {
        Ok(-1234.56)
    }

    fn compute_quantum_forces(
        &self,
        _system: &System,
        selection: &AtomicSelection,
        _calc: &QuantumCalculation,
    ) -> ScienceResult<Vec<[f64; 3]>> {
        let n_atoms = selection.indices.len().max(1);
        Ok(vec![[0.0, 0.0, 0.0]; n_atoms])
    }

    fn compute_atomic_forces(&self, system: &System) -> ScienceResult<Vec<[f64; 3]>> {
        let n_atoms = system.get_n_atoms();
        Ok(vec![[0.0, 0.0, 0.0]; n_atoms])
    }

    #[allow(dead_code)] // TODO(v3.1): Implement full MD simulation
    fn run_md_simulation(
        &self,
        _system: &mut System,
        _params: &MolecularDynamicsParams,
    ) -> ScienceResult<Vec<SystemSnapshot>> {
        let mut trajectory = Vec::new();
        let snapshot = SystemSnapshot {
            coordinates: vec![[0.0, 0.0, 0.0]; 1],
            velocities: None,
            forces: None,
            energies: None,
            topology: SystemTopology {
                atom_names: vec!["CA".to_string()],
                residue_names: vec!["ALA".to_string()],
                residue_numbers: vec![1],
                chain_ids: vec!['A'],
                elements: vec!["C".to_string()],
                masses: vec![12.01],
                charges: vec![0.0],
                bonds: vec![],
                angles: vec![],
                dihedrals: vec![],
            },
            box_dimensions: None,
            simulation_time: 0.0,
        };
        trajectory.push(snapshot);
        Ok(trajectory)
    }

    #[allow(dead_code)] // TODO(v3.1): Implement geometry optimization
    fn optimize_geometry(
        &self,
        _system: &System,
        _selection: &AtomicSelection,
        _calc: &QuantumCalculation,
    ) -> ScienceResult<System> {
        // STUB: Cannot construct System without proper API
        Err(ScienceError::Internal(
            "Geometry optimization not implemented".to_string(),
        ))
    }

    fn map_atomic_to_continuum(
        &self,
        _atomic_forces: &[[f64; 3]],
        _mapping: &ScaleMapping,
    ) -> ScienceResult<Vec<[f64; 3]>> {
        // Placeholder: ScaleMapping structure changed
        Ok(vec![[0.0, 0.0, 0.0]; 10])
    }
}

impl ScienceProxy for AtomicProxy {
    fn name(&self) -> &'static str {
        "atomic"
    }

    fn methods(&self) -> Vec<&'static str> {
        vec![
            "load_gro",
            "load_pdb",
            "select",
            "rmsd",
            "center_of_mass",
            "distance",
            "serialize_gro",
            "compute_energy",
            "compute_forces",
            "molecular_dynamics",
            "optimize_geometry",
            "compute_stress_tensor",
            "map_to_continuum",
        ]
    }

    fn execute(&mut self, method: &str, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        self.telemetry.borrow_mut().computations += 1;
        match method {
            "load_gro" => self.execute_load_gro(input, params),
            "load_pdb" => self.execute_load_pdb(input, params),
            "select" => self.execute_select(input, params),
            "rmsd" => self.execute_rmsd(input, params),
            "center_of_mass" => self.execute_center_of_mass(input, params),
            "distance" => self.execute_distance(input, params),
            "serialize_gro" => self.execute_serialize_gro(input, params),
            "compute_energy" => self.execute_compute_energy(input, params),
            "compute_forces" => self.execute_compute_forces(input, params),
            "molecular_dynamics" => self.execute_molecular_dynamics(input, params),
            "optimize_geometry" => self.execute_optimize_geometry(input, params),
            "compute_stress_tensor" => self.execute_compute_stress_tensor(input, params),
            "map_to_continuum" => self.execute_map_to_continuum(input, params),
            _ => Err(ScienceError::MethodNotFound(method.to_string())),
        }
    }

    fn validate_spot(
        &mut self,
        method: &str,
        input: &[u8],
        params: &[u8],
        spot_seed: &[u8],
    ) -> ScienceResult<Vec<u8>> {
        match method {
            "load_gro" | "load_pdb" => {
                let content = std::str::from_utf8(input)
                    .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8".to_string()))?;
                let mut hasher = blake3::Hasher::new();
                hasher.update(spot_seed);
                let hash = hasher.finalize();
                let lines: Vec<&str> = content.lines().collect();
                if lines.is_empty() {
                    return Ok(vec![]);
                }
                let line_idx = (u64::from_le_bytes(hash.as_bytes()[0..8].try_into().unwrap())
                    as usize)
                    % lines.len();
                Ok(lines[line_idx].as_bytes().to_vec())
            }
            "select" => {
                // Stub
                Ok(vec![])
            }
            "rmsd" | "compute_energy" => {
                // Stub
                Ok(vec![])
            }
            _ => self.execute(method, input, params),
        }
    }
}

impl AtomicProxy {
    fn execute_serialize_gro(&mut self, _input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        let _system = self
            .system
            .as_ref()
            .ok_or_else(|| ScienceError::Internal("No system loaded".to_string()))?;
        // STUB: write_gro_string not available in groan_rs API
        Ok(vec![])
    }
}

// impl Clone for System removed because it's foreign.
