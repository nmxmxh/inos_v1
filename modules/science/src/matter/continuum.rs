use crate::flux::boundary::ScaleMapping;
use crate::matter::{ScienceError, ScienceProxy, ScienceResult};
use crate::mesh::cache::ComputationCache;
use crate::types::{CacheEntry, Telemetry};
use blake3;
use nalgebra::{DMatrix, DVector, Matrix3};
use serde::{Deserialize, Serialize};
use std::cell::RefCell;
use std::collections::HashMap;
use std::rc::Rc;

// ----------------------------------------------------------------------------
// CONTINUUM PHYSICS TYPES
// ----------------------------------------------------------------------------

#[derive(Serialize, Deserialize, Clone)]
pub enum MaterialModel {
    LinearElastic {
        youngs_modulus: f64,
        poissons_ratio: f64,
        density: f64,
    },
    NeoHookean {
        shear_modulus: f64,
        bulk_modulus: f64,
    },
    Plasticity(Box<PlasticityData>),
    Viscoelastic {
        instantaneous_modulus: f64,
        longterm_modulus: f64,
        relaxation_time: f64,
    },
    QuantumInformed {
        atomic_system_hash: String,
        mapping_parameters: Box<ScaleMapping>,
    },
}

#[derive(Serialize, Deserialize, Clone)]
pub struct PlasticityData {
    pub youngs_modulus: f64,
    pub poissons_ratio: f64,
    pub yield_stress: f64,
    pub hardening_modulus: f64,
}

#[derive(Serialize, Deserialize)]
pub struct MeshData {
    pub vertices: Vec<[f64; 3]>,
    pub elements: Vec<Vec<usize>>,
    pub element_type: ElementType,
    pub boundary_groups: HashMap<String, Vec<usize>>,
    pub material_regions: HashMap<String, Vec<usize>>,
}

#[derive(Serialize, Deserialize, Clone, Copy)]
pub enum ElementType {
    Tetrahedron4,
    Tetrahedron10,
    Hexahedron8,
    Hexahedron20,
    Triangle3,
    Triangle6,
    Quad4,
    Quad8,
}

#[derive(Serialize, Deserialize)]
pub struct BoundaryCondition {
    pub nodes: Vec<usize>,
    pub dofs: Vec<usize>,
    pub values: Vec<f64>,
    pub bc_type: BCType,
}

#[derive(Serialize, Deserialize)]
pub enum BCType {
    Dirichlet,
    Neumann,
    Periodic,
    Symmetry,
    Contact,
    QuantumCoupled,
}

#[derive(Serialize, Deserialize)]
pub struct LoadCase {
    pub forces: Vec<[f64; 3]>,
    pub pressures: Vec<PressureLoad>,
    pub body_forces: [f64; 3],
    pub thermal_loads: Option<Vec<f64>>,
    pub quantum_forces: Option<Vec<[f64; 3]>>,
}

#[derive(Serialize, Deserialize)]
pub struct PressureLoad {
    pub face: Vec<usize>,
    pub magnitude: f64,
    pub direction: [f64; 3],
}

#[derive(Serialize, Deserialize, Clone)]
pub struct FEMSolution {
    pub displacements: Vec<[f64; 3]>,
    pub strains: Vec<StrainTensor>,
    pub stresses: Vec<StressTensor>,
    pub reaction_forces: Vec<[f64; 3]>,
    pub energies: EnergyMeasures,
    pub convergence_data: ConvergenceData,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct StrainTensor {
    pub components: [f64; 6],
    pub principal: [f64; 3],
    pub von_mises: f64,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct StressTensor {
    pub components: [f64; 6],
    pub principal: [f64; 3],
    pub von_mises: f64,
    pub safety_factor: f64,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct EnergyMeasures {
    pub strain_energy: f64,
    pub external_work: f64,
    pub kinetic_energy: f64,
    pub dissipated_energy: f64,
    pub quantum_energy: f64,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct ConvergenceData {
    pub iterations: u32,
    pub residual_norm: f64,
    pub energy_error: f64,
    pub max_displacement: f64,
    pub convergence_criteria: Vec<f64>,
}

#[derive(Serialize, Deserialize)]
pub struct MultiScaleCoupling {
    pub atomic_regions: HashMap<String, Vec<usize>>,
    pub mapping_method: MappingMethod,
    pub update_frequency: u32,
    pub tolerance: f64,
}

#[derive(Serialize, Deserialize)]
pub enum MappingMethod {
    BridgingDomain,
    Arlequin,
    Quasicontinuum,
    CauchyBorn,
    DirectNodal,
}

#[derive(Serialize, Deserialize)]
struct Microstructure {
    // Placeholder for microstructure data
}

// ----------------------------------------------------------------------------
// ENHANCED CONTINUUM PROXY
// ----------------------------------------------------------------------------

pub struct ContinuumProxy {
    meshes: HashMap<String, MeshData>,
    materials: HashMap<String, MaterialModel>,
    solutions: HashMap<String, FEMSolution>,
    cache: Rc<RefCell<ComputationCache>>,
    telemetry: Rc<RefCell<Telemetry>>,

    // Multi-scale coupling state
    #[allow(dead_code)]
    coupled_atomic_systems: HashMap<String, String>,
    #[allow(dead_code)]
    coupling_mappings: HashMap<String, MultiScaleCoupling>,
}

impl Default for ContinuumProxy {
    fn default() -> Self {
        Self::new(
            Rc::new(RefCell::new(ComputationCache::new())),
            Rc::new(RefCell::new(Telemetry::default())),
        )
    }
}

impl ContinuumProxy {
    pub fn new(cache: Rc<RefCell<ComputationCache>>, telemetry: Rc<RefCell<Telemetry>>) -> Self {
        Self {
            meshes: HashMap::new(),
            materials: HashMap::new(),
            solutions: HashMap::new(),
            cache,
            telemetry,
            coupled_atomic_systems: HashMap::new(),
            coupling_mappings: HashMap::new(),
        }
    }

    // ------------------------------------------------------------------------
    // MESH GENERATION & MANIPULATION
    // ------------------------------------------------------------------------

    fn execute_generate_mesh(&mut self, _input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let mesh_name = params_json["name"]
            .as_str()
            .ok_or_else(|| ScienceError::InvalidParams("Missing mesh name".to_string()))?
            .to_string();

        let mesh_type = params_json["type"].as_str().unwrap_or("tetrahedral");
        let dimensions = params_json["dimensions"].as_f64().unwrap_or(1.0);

        let mesh_data = match mesh_type {
            "uniform_box" => self.generate_uniform_box_mesh(&params_json),
            _ => {
                return Err(ScienceError::InvalidParams(format!(
                    "Unknown mesh type: {}",
                    mesh_type
                )))
            }
        }?;

        self.meshes.insert(mesh_name.clone(), mesh_data);

        let metadata = serde_json::json!({
            "name": mesh_name,
            "nodes": self.meshes[&mesh_name].vertices.len(),
            "elements": self.meshes[&mesh_name].elements.len(),
            "dimensions": dimensions,
            "hash": self.compute_mesh_hash(&mesh_name),
        });

        serde_json::to_vec(&metadata).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    fn generate_uniform_box_mesh(&self, params: &serde_json::Value) -> ScienceResult<MeshData> {
        let size = params["size"].as_f64().unwrap_or(1.0);
        let divisions = params["divisions"].as_u64().unwrap_or(10) as usize;
        let element_type = params["element_type"].as_str().unwrap_or("Tetrahedron4");

        let mut vertices = Vec::new();
        let dx = size / divisions as f64;

        for i in 0..=divisions {
            for j in 0..=divisions {
                for k in 0..=divisions {
                    vertices.push([i as f64 * dx, j as f64 * dx, k as f64 * dx]);
                }
            }
        }

        let elements = match element_type {
            "Tetrahedron4" => self.generate_tetrahedral_connectivity(divisions),
            "Hexahedron8" => self.generate_hexahedral_connectivity(divisions),
            _ => {
                return Err(ScienceError::InvalidParams(format!(
                    "Unsupported element type: {}",
                    element_type
                )))
            }
        };

        let mut boundary_groups = HashMap::new();
        boundary_groups.insert(
            "bottom".to_string(),
            self.find_boundary_nodes(&vertices, 2, 0.0),
        );
        boundary_groups.insert(
            "top".to_string(),
            self.find_boundary_nodes(&vertices, 2, size),
        );
        boundary_groups.insert(
            "left".to_string(),
            self.find_boundary_nodes(&vertices, 0, 0.0),
        );
        boundary_groups.insert(
            "right".to_string(),
            self.find_boundary_nodes(&vertices, 0, size),
        );

        Ok(MeshData {
            vertices,
            elements,
            element_type: match element_type {
                "Tetrahedron4" => ElementType::Tetrahedron4,
                "Hexahedron8" => ElementType::Hexahedron8,
                _ => ElementType::Tetrahedron4,
            },
            boundary_groups,
            material_regions: HashMap::new(),
        })
    }

    fn generate_tetrahedral_connectivity(&self, divisions: usize) -> Vec<Vec<usize>> {
        let mut elements = Vec::new();
        let nodes_per_dim = divisions + 1;

        for i in 0..divisions {
            for j in 0..divisions {
                for k in 0..divisions {
                    let n0 = i * nodes_per_dim * nodes_per_dim + j * nodes_per_dim + k;
                    let n1 = n0 + 1;
                    let n2 = n0 + nodes_per_dim;
                    let n3 = n2 + 1;
                    let n4 = n0 + nodes_per_dim * nodes_per_dim;
                    let n5 = n4 + 1;
                    let n6 = n4 + nodes_per_dim;
                    let n7 = n6 + 1;

                    elements.push(vec![n0, n1, n2, n6]);
                    elements.push(vec![n0, n1, n5, n6]);
                    elements.push(vec![n1, n3, n7, n6]);
                    elements.push(vec![n1, n5, n7, n6]);
                    elements.push(vec![n0, n2, n4, n6]);
                }
            }
        }

        elements
    }

    fn generate_hexahedral_connectivity(&self, divisions: usize) -> Vec<Vec<usize>> {
        let mut elements = Vec::new();
        let nodes_per_dim = divisions + 1;

        for i in 0..divisions {
            for j in 0..divisions {
                for k in 0..divisions {
                    let n0 = i * nodes_per_dim * nodes_per_dim + j * nodes_per_dim + k;
                    let n1 = n0 + 1;
                    let n2 = n0 + nodes_per_dim;
                    let n3 = n2 + 1;
                    let n4 = n0 + nodes_per_dim * nodes_per_dim;
                    let n5 = n4 + 1;
                    let n6 = n4 + nodes_per_dim;
                    let n7 = n6 + 1;

                    elements.push(vec![n0, n1, n3, n2, n4, n5, n7, n6]);
                }
            }
        }

        elements
    }

    // ------------------------------------------------------------------------
    // FEM ASSEMBLY & SOLUTION
    // ------------------------------------------------------------------------

    fn execute_assemble_stiffness(
        &mut self,
        _input: &[u8],
        params: &[u8],
    ) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let mesh_name = params_json["mesh"]
            .as_str()
            .ok_or_else(|| ScienceError::InvalidParams("Missing mesh name".to_string()))?
            .to_string();

        let material_name = params_json["material"]
            .as_str()
            .ok_or_else(|| ScienceError::InvalidParams("Missing material name".to_string()))?
            .to_string();

        let cache_key = format!("stiffness:{mesh_name}:{material_name}");
        if let Some(entry) = self.cache.borrow_mut().get(&cache_key, None) {
            self.telemetry.borrow_mut().cache_hits += 1;
            return Ok(entry.data);
        }

        self.telemetry.borrow_mut().cache_misses += 1;
        self.telemetry.borrow_mut().computations += 1;

        let mesh = self
            .meshes
            .get(&mesh_name)
            .ok_or_else(|| ScienceError::Internal(format!("Mesh not found: {}", mesh_name)))?;

        let material = self.materials.get(&material_name).ok_or_else(|| {
            ScienceError::Internal(format!("Material not found: {}", material_name))
        })?;

        let stiffness_matrix = self.assemble_stiffness_matrix(mesh, material)?;

        let entry = CacheEntry {
            data: stiffness_matrix.clone(),
            result_hash: cache_key.clone(),
            timestamp: 0,
            access_count: 1,
            scale: Default::default(),
            proof: Default::default(),
        };
        self.cache.borrow_mut().put(cache_key, entry);

        Ok(stiffness_matrix)
    }

    fn assemble_stiffness_matrix(
        &self,
        mesh: &MeshData,
        _material: &MaterialModel,
    ) -> ScienceResult<Vec<u8>> {
        // STUB: Full fenris integration would go here
        let n_dofs = mesh.vertices.len() * 3;
        let mut result = Vec::new();

        result.extend_from_slice(&(n_dofs as u32).to_le_bytes());
        result.extend_from_slice(&(n_dofs as u32).to_le_bytes());

        // Placeholder: identity matrix
        // Placeholder: identity matrix
        for i in 0..n_dofs {
            for j in 0..n_dofs {
                let val: f64 = if i == j { 1.0 } else { 0.0 };
                result.extend_from_slice(&val.to_le_bytes());
            }
        }

        Ok(result)
    }

    fn execute_solve_linear(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let mesh_name = params_json["mesh"]
            .as_str()
            .ok_or_else(|| ScienceError::InvalidParams("Missing mesh name".to_string()))?
            .to_string();

        let load_case: LoadCase =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let mesh = self
            .meshes
            .get(&mesh_name)
            .ok_or_else(|| ScienceError::Internal(format!("Mesh not found: {}", mesh_name)))?;

        let mut forces = load_case.forces.clone();
        if let Some(quantum_forces) = &load_case.quantum_forces {
            let mapped_forces = self.map_quantum_forces(mesh, quantum_forces)?;

            for i in 0..forces.len().min(mapped_forces.len()) {
                forces[i][0] += mapped_forces[i][0];
                forces[i][1] += mapped_forces[i][1];
                forces[i][2] += mapped_forces[i][2];
            }
        }

        let solution = self.solve_linear_system(mesh, &forces, &params_json)?;

        let solution_name = format!("{}_solution", mesh_name);
        self.solutions.insert(solution_name, solution.clone());

        self.telemetry.borrow_mut().mesh_solves += 1;

        bincode::serialize(&solution).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    fn solve_linear_system(
        &self,
        mesh: &MeshData,
        forces: &[[f64; 3]],
        params: &serde_json::Value,
    ) -> ScienceResult<FEMSolution> {
        let material_name = params
            .get("material")
            .and_then(|v| v.as_str())
            .unwrap_or("default");

        let material = self
            .materials
            .get(material_name)
            .ok_or_else(|| ScienceError::Internal("Material not found".to_string()))?;

        let n_nodes = mesh.vertices.len();
        let mut force_vector = DVector::zeros(n_nodes * 3);

        for (i, force) in forces.iter().enumerate().take(n_nodes) {
            force_vector[i * 3] = force[0];
            force_vector[i * 3 + 1] = force[1];
            force_vector[i * 3 + 2] = force[2];
        }

        let constrained_dofs = self.extract_boundary_conditions(params);

        let stiffness = DMatrix::identity(n_nodes * 3, n_nodes * 3);
        let displacements =
            self.solve_constrained_system(&stiffness, &force_vector, &constrained_dofs)?;

        let (strains, stresses) = self.compute_strain_stress(mesh, &displacements, material)?;
        let reaction_forces =
            self.compute_reaction_forces(&stiffness, &displacements, &force_vector)?;
        let energies = self.compute_energies(&stiffness, &displacements, &force_vector)?;

        Ok(FEMSolution {
            displacements: self.vector_to_nodal_array(&displacements),
            strains,
            stresses,
            reaction_forces,
            energies,
            convergence_data: ConvergenceData {
                iterations: 1,
                residual_norm: 0.0,
                energy_error: 0.0,
                max_displacement: displacements.iter().fold(0.0, |max, &x| max.max(x.abs())),
                convergence_criteria: vec![1e-6],
            },
        })
    }

    // ------------------------------------------------------------------------
    // STRAIN & STRESS COMPUTATION
    // ------------------------------------------------------------------------

    fn execute_compute_strain(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let mesh_name = params_json["mesh"]
            .as_str()
            .ok_or_else(|| ScienceError::InvalidParams("Missing mesh name".to_string()))?
            .to_string();

        let displacements: Vec<[f64; 3]> =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let mesh = self
            .meshes
            .get(&mesh_name)
            .ok_or_else(|| ScienceError::Internal(format!("Mesh not found: {}", mesh_name)))?;

        let material_name = params_json
            .get("material")
            .and_then(|v| v.as_str())
            .unwrap_or("default");

        let material = self
            .materials
            .get(material_name)
            .ok_or_else(|| ScienceError::Internal("Material not found".to_string()))?;

        let disp_vector = self.nodal_array_to_vector(&displacements);
        let (strains, _) = self.compute_strain_stress(mesh, &disp_vector, material)?;

        bincode::serialize(&strains).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    fn execute_compute_stress(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let mesh_name = params_json["mesh"]
            .as_str()
            .ok_or_else(|| ScienceError::InvalidParams("Missing mesh name".to_string()))?
            .to_string();

        let displacements: Vec<[f64; 3]> =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let mesh = self
            .meshes
            .get(&mesh_name)
            .ok_or_else(|| ScienceError::Internal(format!("Mesh not found: {}", mesh_name)))?;

        let material_name = params_json
            .get("material")
            .and_then(|v| v.as_str())
            .unwrap_or("default");

        let material = self
            .materials
            .get(material_name)
            .ok_or_else(|| ScienceError::Internal("Material not found".to_string()))?;

        let disp_vector = self.nodal_array_to_vector(&displacements);
        let (_, stresses) = self.compute_strain_stress(mesh, &disp_vector, material)?;

        bincode::serialize(&stresses).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    // ------------------------------------------------------------------------
    // MULTI-SCALE COUPLING
    // ------------------------------------------------------------------------

    fn execute_apply_quantum_forces(
        &mut self,
        input: &[u8],
        params: &[u8],
    ) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let mesh_name = params_json["mesh"]
            .as_str()
            .ok_or_else(|| ScienceError::InvalidParams("Missing mesh name".to_string()))?
            .to_string();

        let quantum_forces: Vec<[f64; 3]> =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let mesh = self
            .meshes
            .get(&mesh_name)
            .ok_or_else(|| ScienceError::Internal(format!("Mesh not found: {}", mesh_name)))?;

        let continuum_forces = self.map_quantum_forces(mesh, &quantum_forces)?;

        bincode::serialize(&continuum_forces).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    fn execute_compute_homogenized_properties(
        &mut self,
        input: &[u8],
        _params: &[u8],
    ) -> ScienceResult<Vec<u8>> {
        let _microstructure: Microstructure =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        // STUB: Homogenization implementation
        let mut stiffness = [[0.0; 6]; 6];
        for (i, row) in stiffness.iter_mut().enumerate() {
            row[i] = 1.0;
        }

        bincode::serialize(&stiffness).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    // ------------------------------------------------------------------------
    // HELPER METHODS
    // ------------------------------------------------------------------------

    fn extract_boundary_conditions(&self, params: &serde_json::Value) -> Vec<(usize, f64)> {
        let mut constraints = Vec::new();

        if let Some(bcs) = params.get("boundary_conditions") {
            if let Some(bc_array) = bcs.as_array() {
                for bc in bc_array {
                    if let Some(nodes) = bc.get("nodes").and_then(|n| n.as_array()) {
                        if let Some(dofs) = bc.get("dofs").and_then(|d| d.as_array()) {
                            if let Some(value) = bc.get("value").and_then(|v| v.as_f64()) {
                                for node in nodes {
                                    if let Some(node_idx) = node.as_u64() {
                                        for dof in dofs {
                                            if let Some(dof_idx) = dof.as_u64() {
                                                let global_dof =
                                                    (node_idx as usize) * 3 + (dof_idx as usize);
                                                constraints.push((global_dof, value));
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        constraints
    }

    fn solve_constrained_system(
        &self,
        stiffness: &DMatrix<f64>,
        force: &DVector<f64>,
        constraints: &[(usize, f64)],
    ) -> ScienceResult<DVector<f64>> {
        let n = stiffness.nrows();
        let mut k_mod = stiffness.clone();
        let mut f_mod = force.clone();

        let penalty = 1e12;

        for &(dof, value) in constraints {
            if dof < n {
                k_mod[(dof, dof)] += penalty;
                f_mod[dof] = value * penalty;
            }
        }

        let lu = k_mod.lu();
        let solution = lu
            .solve(&f_mod)
            .ok_or_else(|| ScienceError::Internal("Linear solve failed".to_string()))?;

        Ok(solution)
    }

    fn compute_strain_stress(
        &self,
        mesh: &MeshData,
        displacements: &DVector<f64>,
        material: &MaterialModel,
    ) -> ScienceResult<(Vec<StrainTensor>, Vec<StressTensor>)> {
        let mut strains = Vec::new();
        let mut stresses = Vec::new();

        match mesh.element_type {
            ElementType::Tetrahedron4 => {
                for element in &mesh.elements {
                    if element.len() == 4 {
                        let mut coords = [[0.0; 3]; 4];
                        let mut disp = [[0.0; 3]; 4];

                        for (i, &node_idx) in element.iter().enumerate() {
                            if node_idx < mesh.vertices.len() {
                                coords[i] = mesh.vertices[node_idx];
                                disp[i] = [
                                    displacements[node_idx * 3],
                                    displacements[node_idx * 3 + 1],
                                    displacements[node_idx * 3 + 2],
                                ];
                            }
                        }

                        let strain = self.compute_tetrahedral_strain(&coords, &disp);
                        let stress = self.compute_stress_from_strain(&strain, material);

                        strains.push(strain);
                        stresses.push(stress);
                    }
                }
            }
            _ => {
                return Err(ScienceError::Internal(
                    "Strain computation only implemented for tetrahedra".to_string(),
                ))
            }
        }

        Ok((strains, stresses))
    }

    fn compute_tetrahedral_strain(
        &self,
        _coords: &[[f64; 3]; 4],
        disp: &[[f64; 3]; 4],
    ) -> StrainTensor {
        let mut strain = StrainTensor {
            components: [0.0; 6],
            principal: [0.0; 3],
            von_mises: 0.0,
        };

        // Simplified strain from average displacement gradient
        for (i, strain_comp) in strain.components.iter_mut().enumerate().take(3) {
            let mut sum = 0.0;
            for d in disp {
                sum += d[i];
            }
            *strain_comp = sum / 4.0;
        }

        strain
    }

    fn compute_stress_from_strain(
        &self,
        strain: &StrainTensor,
        material: &MaterialModel,
    ) -> StressTensor {
        match material {
            MaterialModel::LinearElastic {
                youngs_modulus: e,
                poissons_ratio: nu,
                ..
            } => {
                let lambda = e * nu / ((1.0 + nu) * (1.0 - 2.0 * nu));
                let mu = e / (2.0 * (1.0 + nu));

                let eps = &strain.components;
                let mut sigma = [0.0; 6];

                sigma[0] = lambda * (eps[0] + eps[1] + eps[2]) + 2.0 * mu * eps[0];
                sigma[1] = lambda * (eps[0] + eps[1] + eps[2]) + 2.0 * mu * eps[1];
                sigma[2] = lambda * (eps[0] + eps[1] + eps[2]) + 2.0 * mu * eps[2];
                sigma[3] = mu * eps[3];
                sigma[4] = mu * eps[4];
                sigma[5] = mu * eps[5];

                let von_mises = (0.5
                    * ((sigma[0] - sigma[1]).powi(2)
                        + (sigma[1] - sigma[2]).powi(2)
                        + (sigma[2] - sigma[0]).powi(2)
                        + 6.0 * (sigma[3].powi(2) + sigma[4].powi(2) + sigma[5].powi(2))))
                .sqrt();

                StressTensor {
                    components: sigma,
                    principal: self.compute_principal_stresses(&sigma),
                    von_mises,
                    safety_factor: 2.0,
                }
            }
            _ => StressTensor {
                components: [0.0; 6],
                principal: [0.0; 3],
                von_mises: 0.0,
                safety_factor: 0.0,
            },
        }
    }

    fn compute_principal_stresses(&self, stress: &[f64; 6]) -> [f64; 3] {
        let sigma = Matrix3::new(
            stress[0], stress[3], stress[4], stress[3], stress[1], stress[5], stress[4], stress[5],
            stress[2],
        );

        let eigen = sigma.symmetric_eigen();
        let mut eigenvalues = eigen.eigenvalues.as_slice().to_vec();
        eigenvalues.sort_by(|a, b| b.partial_cmp(a).unwrap());

        [eigenvalues[0], eigenvalues[1], eigenvalues[2]]
    }

    fn map_quantum_forces(
        &self,
        mesh: &MeshData,
        quantum_forces: &[[f64; 3]],
    ) -> ScienceResult<Vec<[f64; 3]>> {
        let mut continuum_forces = vec![[0.0; 3]; mesh.vertices.len()];

        for (i, &force) in quantum_forces.iter().enumerate() {
            let node_idx = i % mesh.vertices.len();

            continuum_forces[node_idx][0] += force[0];
            continuum_forces[node_idx][1] += force[1];
            continuum_forces[node_idx][2] += force[2];
        }

        Ok(continuum_forces)
    }

    fn vector_to_nodal_array(&self, vector: &DVector<f64>) -> Vec<[f64; 3]> {
        let mut result = Vec::new();

        for i in 0..vector.len() / 3 {
            result.push([vector[i * 3], vector[i * 3 + 1], vector[i * 3 + 2]]);
        }

        result
    }

    fn nodal_array_to_vector(&self, array: &[[f64; 3]]) -> DVector<f64> {
        let mut vector = DVector::zeros(array.len() * 3);

        for (i, node) in array.iter().enumerate() {
            vector[i * 3] = node[0];
            vector[i * 3 + 1] = node[1];
            vector[i * 3 + 2] = node[2];
        }

        vector
    }

    fn compute_reaction_forces(
        &self,
        stiffness: &DMatrix<f64>,
        displacements: &DVector<f64>,
        forces: &DVector<f64>,
    ) -> ScienceResult<Vec<[f64; 3]>> {
        let reactions = stiffness * displacements - forces;
        Ok(self.vector_to_nodal_array(&reactions))
    }

    fn compute_energies(
        &self,
        stiffness: &DMatrix<f64>,
        displacements: &DVector<f64>,
        forces: &DVector<f64>,
    ) -> ScienceResult<EnergyMeasures> {
        let strain_energy = 0.5 * displacements.dot(&(stiffness * displacements));
        let external_work = displacements.dot(forces);

        Ok(EnergyMeasures {
            strain_energy,
            external_work,
            kinetic_energy: 0.0,
            dissipated_energy: 0.0,
            quantum_energy: 0.0,
        })
    }

    fn compute_mesh_hash(&self, mesh_name: &str) -> String {
        if let Some(mesh) = self.meshes.get(mesh_name) {
            let mut hasher = blake3::Hasher::new();

            for vertex in &mesh.vertices {
                hasher.update(&vertex[0].to_le_bytes());
                hasher.update(&vertex[1].to_le_bytes());
                hasher.update(&vertex[2].to_le_bytes());
            }

            for element in &mesh.elements {
                for &node in element {
                    hasher.update(&(node as u64).to_le_bytes());
                }
            }

            hasher.finalize().to_hex().to_string()
        } else {
            String::new()
        }
    }

    fn find_boundary_nodes(
        &self,
        vertices: &[[f64; 3]],
        coord_index: usize,
        target_value: f64,
    ) -> Vec<usize> {
        let tolerance = 1e-9;
        vertices
            .iter()
            .enumerate()
            .filter(|(_, v)| (v[coord_index] - target_value).abs() < tolerance)
            .map(|(i, _)| i)
            .collect()
    }
}

impl ScienceProxy for ContinuumProxy {
    fn name(&self) -> &'static str {
        "continuum"
    }

    fn methods(&self) -> Vec<&'static str> {
        vec![
            "generate_mesh",
            "assemble_stiffness",
            "solve_linear",
            "compute_strain",
            "compute_stress",
            "apply_quantum_forces",
            "compute_homogenized_properties",
        ]
    }

    fn execute(&mut self, method: &str, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        self.telemetry.borrow_mut().computations += 1;

        match method {
            "generate_mesh" => self.execute_generate_mesh(input, params),
            "assemble_stiffness" => self.execute_assemble_stiffness(input, params),
            "solve_linear" => self.execute_solve_linear(input, params),
            "compute_strain" => self.execute_compute_strain(input, params),
            "compute_stress" => self.execute_compute_stress(input, params),
            "apply_quantum_forces" => self.execute_apply_quantum_forces(input, params),
            "compute_homogenized_properties" => {
                self.execute_compute_homogenized_properties(input, params)
            }
            _ => Err(ScienceError::MethodNotFound(method.to_string())),
        }
    }

    fn validate_spot(
        &mut self,
        method: &str,
        _input: &[u8],
        params: &[u8],
        spot_seed: &[u8],
    ) -> ScienceResult<Vec<u8>> {
        match method {
            "assemble_stiffness" => {
                let params_str = std::str::from_utf8(params)
                    .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
                let params_json: serde_json::Value = serde_json::from_str(params_str)
                    .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

                let mesh_name = params_json["mesh"].as_str().unwrap_or("");
                let mesh = self
                    .meshes
                    .get(mesh_name)
                    .ok_or_else(|| ScienceError::Internal("Mesh not found".to_string()))?;

                let mut hasher = blake3::Hasher::new();
                hasher.update(spot_seed);
                let hash = hasher.finalize();

                let element_idx = (u64::from_le_bytes(hash.as_bytes()[0..8].try_into().unwrap())
                    as usize)
                    % mesh.elements.len().max(1);

                // Return element index as validation
                Ok((element_idx as u64).to_le_bytes().to_vec())
            }
            _ => Ok(vec![]),
        }
    }
}
