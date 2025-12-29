use crate::matter::{ScienceError, ScienceProxy, ScienceResult};
use blake3;
use nalgebra::{Complex, DMatrix, DVector, SymmetricEigen, Vector3, SVD};
use serde::{Deserialize, Serialize};
use std::cell::RefCell;
use std::collections::HashMap;
use std::rc::Rc;

use crate::mesh::cache::ComputationCache;
use crate::types::{CacheEntry, Telemetry};

// ----------------------------------------------------------------------------
// QUANTUM-AWARE LINEAR ALGEBRA
// ----------------------------------------------------------------------------

#[derive(Serialize, Deserialize)]
pub enum MatrixPrecision {
    F32,        // For continuum/kinetic (fast)
    F64,        // For quantum chemistry (standard)
    ComplexF64, // For quantum mechanics (wavefunctions)
}

#[derive(Serialize, Deserialize)]
pub struct MatrixData {
    data: Vec<u8>,
    shape: (usize, usize),
    precision: MatrixPrecision,
    symmetry: Option<MatrixSymmetry>, // For optimization
    sparsity: f64,                    // 0.0 = dense, 1.0 = fully sparse
}

#[derive(Serialize, Deserialize)]
pub enum MatrixSymmetry {
    Symmetric,
    Hermitian,
    SkewSymmetric,
    PositiveDefinite,
}

#[derive(Serialize, Deserialize)]
pub struct QuantumState {
    // For quantum systems
    density_matrix: Option<MatrixData>,      // Mixed states
    wavefunction: Option<Vec<Complex<f64>>>, // Pure states
    subsystems: Vec<usize>,                  // Dimensions of each subsystem
}

// ----------------------------------------------------------------------------
// ENHANCED MATH PROXY
// ----------------------------------------------------------------------------

type MathMethod = fn(&mut MathProxy, &[u8], &[u8]) -> ScienceResult<Vec<u8>>;

pub struct MathProxy {
    cache: Rc<RefCell<ComputationCache>>,
    telemetry: Rc<RefCell<Telemetry>>,
    method_implementations: HashMap<String, MathMethod>,

    // For distributed mesh operations
    local_node_id: u64,
    shard_id: u32,
}

impl MathProxy {
    pub fn new(cache: Rc<RefCell<ComputationCache>>, telemetry: Rc<RefCell<Telemetry>>) -> Self {
        let mut proxy = Self {
            cache,
            telemetry,
            method_implementations: HashMap::new(),
            local_node_id: 0, // Would be set by mesh
            shard_id: 0,
        };

        // Register all method implementations
        proxy.register_methods();
        proxy
    }

    fn register_methods(&mut self) {
        self.method_implementations
            .insert("dot".to_string(), Self::execute_dot);
        self.method_implementations
            .insert("cross".to_string(), Self::execute_cross);
        self.method_implementations
            .insert("normalize".to_string(), Self::execute_normalize);
        self.method_implementations
            .insert("matrix_multiply".to_string(), Self::execute_matrix_multiply);
        self.method_implementations
            .insert("inverse".to_string(), Self::execute_inverse);
        self.method_implementations
            .insert("eigenvalues".to_string(), Self::execute_eigenvalues);
        self.method_implementations
            .insert("svd".to_string(), Self::execute_svd);
        self.method_implementations
            .insert("qr_decomposition".to_string(), Self::execute_qr);
        self.method_implementations
            .insert("cholesky".to_string(), Self::execute_cholesky);
        self.method_implementations
            .insert("solve_linear".to_string(), Self::execute_solve_linear);
        self.method_implementations
            .insert("tensor_product".to_string(), Self::execute_tensor_product);
        self.method_implementations
            .insert("partial_trace".to_string(), Self::execute_partial_trace);
        self.method_implementations.insert(
            "expectation_value".to_string(),
            Self::execute_expectation_value,
        );
        self.method_implementations
            .insert("commutator".to_string(), Self::execute_commutator);
        self.method_implementations
            .insert("matrix_exp".to_string(), Self::execute_matrix_exp);
        self.method_implementations
            .insert("matrix_log".to_string(), Self::execute_matrix_log);
        self.method_implementations
            .insert("determinant".to_string(), Self::execute_determinant);
        self.method_implementations
            .insert("trace".to_string(), Self::execute_trace);
        self.method_implementations.insert(
            "condition_number".to_string(),
            Self::execute_condition_number,
        );
        self.method_implementations
            .insert("pseudoinverse".to_string(), Self::execute_pseudoinverse);
        self.method_implementations
            .insert("schur_decomposition".to_string(), Self::execute_schur);
        self.method_implementations
            .insert("lyapunov_solve".to_string(), Self::execute_lyapunov);
        self.method_implementations
            .insert("riccati_solve".to_string(), Self::execute_riccati);
        self.method_implementations.insert(
            "distributed_matmul".to_string(),
            Self::execute_distributed_matmul,
        );
    }

    // ------------------------------------------------------------------------
    // CORE LINEAR ALGEBRA IMPLEMENTATIONS
    // ------------------------------------------------------------------------

    fn execute_dot(&mut self, input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        let (a, b) = self.parse_two_vectors(input)?;

        // Check dimensions
        if a.len() != b.len() {
            return Err(ScienceError::InvalidParams(format!(
                "Vectors must have same length: {} != {}",
                a.len(),
                b.len()
            )));
        }

        let result = a.dot(&b);

        // Serialize result as f64
        Ok(result.to_le_bytes().to_vec())
    }

    fn execute_cross(&mut self, input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        let (a, b) = self.parse_two_vectors(input)?;

        // Check that they're 3D vectors
        if a.len() != 3 || b.len() != 3 {
            return Err(ScienceError::InvalidParams(
                "Cross product requires 3D vectors".to_string(),
            ));
        }

        let a_vec = Vector3::<f64>::from_vec(a.as_slice().to_vec());
        let b_vec = Vector3::<f64>::from_vec(b.as_slice().to_vec());
        let result = a_vec.cross(&b_vec);

        // Return as [x, y, z] f64 array
        Ok(result.iter().flat_map(|&x| x.to_le_bytes()).collect())
    }

    fn execute_matrix_multiply(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        // Parse matrices from input
        // Input format: [matrix A data][matrix B data]
        let rows_a = params_json["rows_a"]
            .as_u64()
            .ok_or_else(|| ScienceError::InvalidParams("Missing rows_a".to_string()))?
            as usize;
        let cols_a = params_json["cols_a"]
            .as_u64()
            .ok_or_else(|| ScienceError::InvalidParams("Missing cols_a".to_string()))?
            as usize;
        let rows_b = params_json["rows_b"]
            .as_u64()
            .ok_or_else(|| ScienceError::InvalidParams("Missing rows_b".to_string()))?
            as usize;
        let cols_b = params_json["cols_b"]
            .as_u64()
            .ok_or_else(|| ScienceError::InvalidParams("Missing cols_b".to_string()))?
            as usize;

        // Check compatibility
        if cols_a != rows_b {
            return Err(ScienceError::InvalidParams(format!(
                "Matrix dimensions incompatible: {}x{} * {}x{}",
                rows_a, cols_a, rows_b, cols_b
            )));
        }

        // Parse matrices
        let a = self.parse_matrix(input, 0, rows_a, cols_a)?;
        // Offset for B is size of A (rows * cols * 8 bytes)
        let b = self.parse_matrix(input, rows_a * cols_a * 8, rows_b, cols_b)?;

        // Perform multiplication
        let result = &a * &b;

        // Serialize result
        self.serialize_matrix(&result)
    }

    fn execute_inverse(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let matrix = self.parse_single_matrix(input, params)?;

        // Check if square
        if matrix.nrows() != matrix.ncols() {
            return Err(ScienceError::InvalidParams(
                "Matrix must be square".to_string(),
            ));
        }

        // Compute inverse
        let inverse = matrix
            .try_inverse()
            .ok_or_else(|| ScienceError::Internal("Matrix is singular".to_string()))?;

        self.serialize_matrix(&inverse)
    }

    fn execute_eigenvalues(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let matrix = self.parse_single_matrix(input, params)?;

        // Check if square
        if matrix.nrows() != matrix.ncols() {
            return Err(ScienceError::InvalidParams(
                "Matrix must be square".to_string(),
            ));
        }

        let params_str = std::str::from_utf8(params).unwrap_or("{}");
        // Parse params to determine method
        let params_json: serde_json::Value =
            serde_json::from_str(params_str).unwrap_or(serde_json::json!({}));

        let eigenvalues: Vec<f64> = if params_json
            .get("symmetric")
            .and_then(|v| v.as_bool())
            .unwrap_or(false)
        {
            // More efficient for symmetric matrices
            let eigen = SymmetricEigen::new(matrix);
            eigen.eigenvalues.as_slice().to_vec()
        } else {
            // General eigenvalues (complex)
            let eigen = matrix.complex_eigenvalues();
            eigen.iter().map(|c| c.re).collect() // Return real parts for now
        };

        // Serialize eigenvalues
        let mut result = Vec::new();
        for val in eigenvalues {
            result.extend_from_slice(&val.to_le_bytes());
        }
        Ok(result)
    }

    fn execute_svd(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let matrix = self.parse_single_matrix(input, params)?;

        // Compute SVD
        let svd = SVD::new(matrix, true, true);

        // Return U, S, V^T
        let mut result = Vec::new();

        // U matrix
        if let Some(u) = svd.u {
            result.extend_from_slice(&(u.nrows() as u32).to_le_bytes());
            result.extend_from_slice(&(u.ncols() as u32).to_le_bytes());
            result.extend_from_slice(
                u.as_slice()
                    .iter()
                    .flat_map(|&x| x.to_le_bytes())
                    .collect::<Vec<u8>>()
                    .as_slice(),
            );
        }

        // Singular values
        result.extend_from_slice(&(svd.singular_values.len() as u32).to_le_bytes());
        for &val in svd.singular_values.iter() {
            result.extend_from_slice(&val.to_le_bytes());
        }

        // V^T matrix
        if let Some(v_t) = svd.v_t {
            result.extend_from_slice(&(v_t.nrows() as u32).to_le_bytes());
            result.extend_from_slice(&(v_t.ncols() as u32).to_le_bytes());
            result.extend_from_slice(
                v_t.as_slice()
                    .iter()
                    .flat_map(|&x| x.to_le_bytes())
                    .collect::<Vec<u8>>()
                    .as_slice(),
            );
        }

        Ok(result)
    }

    fn execute_solve_linear(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        // Parse A and b from input
        // Format: [A data][b data]
        let rows = params_json["rows"].as_u64().unwrap_or(0) as usize;
        let cols = params_json["cols"].as_u64().unwrap_or(0) as usize;

        let a = self.parse_matrix(input, 0, rows, cols)?;
        let b = self.parse_vector(input, rows * cols * 8, rows)?;

        // Choose solver based on params
        let solver = params_json
            .get("solver")
            .and_then(|v| v.as_str())
            .unwrap_or("lu");

        let solution = match solver {
            "lu" => a.lu().solve(&b),
            "qr" => a.qr().solve(&b),
            "svd" => a.svd(true, true).solve(&b, 1e-12).ok(),
            "cholesky" => a.cholesky().map(|c| c.solve(&b)),
            _ => {
                return Err(ScienceError::InvalidParams(format!(
                    "Unknown solver: {}",
                    solver
                )))
            }
        }
        .ok_or_else(|| ScienceError::Internal("System has no solution".to_string()))?;

        // Serialize solution vector
        self.serialize_vector(&solution)
    }

    // ------------------------------------------------------------------------
    // QUANTUM-AWARE OPERATIONS
    // ------------------------------------------------------------------------

    fn execute_tensor_product(&mut self, input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        // Parse multiple matrices for tensor (Kronecker) product
        let matrices: Vec<MatrixData> =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        // Start with identity
        let mut result = DMatrix::<f64>::identity(1, 1);

        for mat_data in matrices {
            let matrix = self.deserialize_matrix(&mat_data)?;
            result = result.kronecker(&matrix);
        }

        // Return as MatrixData
        let output = MatrixData {
            data: result
                .as_slice()
                .iter()
                .flat_map(|&x| x.to_le_bytes())
                .collect::<Vec<u8>>(),
            shape: (result.nrows(), result.ncols()),
            precision: MatrixPrecision::F64,
            symmetry: None,
            sparsity: 0.0,
        };

        bincode::serialize(&output).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    fn execute_partial_trace(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        // Partial trace over quantum subsystems
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let subsystems_to_trace: Vec<usize> = params_json
            .get("trace_subsystems")
            .and_then(|v| v.as_array())
            .map(|arr| {
                arr.iter()
                    .filter_map(|v| v.as_u64().map(|n| n as usize))
                    .collect()
            })
            .unwrap_or_default();

        // Parse quantum state (density matrix)
        let quantum_state: QuantumState =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        if let Some(density_matrix) = quantum_state.density_matrix {
            let matrix = self.deserialize_matrix(&density_matrix)?;

            // Validate density matrix properties
            if (matrix.nrows() != matrix.ncols()) || (matrix.trace() - 1.0).abs() > 1e-10 {
                return Err(ScienceError::InvalidParams(
                    "Invalid density matrix".to_string(),
                ));
            }

            // Get subsystem dimensions
            // let total_dim = matrix.nrows();
            let subsystem_dims = quantum_state.subsystems;

            // Compute partial trace (simplified example)
            // In reality, this requires proper tensor product structure
            let remaining_dims: usize = subsystem_dims
                .iter()
                .enumerate()
                .filter(|(i, _)| !subsystems_to_trace.contains(i))
                .map(|(_, &dim)| dim)
                .product();

            let mut reduced = DMatrix::zeros(remaining_dims, remaining_dims);

            // Simplified partial trace for 2 qubits
            if subsystem_dims.len() == 2 && subsystems_to_trace.len() == 1 {
                let dim_a = subsystem_dims[0];
                let dim_b = subsystem_dims[1];

                for i in 0..dim_a {
                    for j in 0..dim_a {
                        let mut sum = 0.0;
                        for k in 0..dim_b {
                            let idx_i = i * dim_b + k;
                            let idx_j = j * dim_b + k;
                            sum += matrix[(idx_i, idx_j)];
                        }
                        reduced[(i, j)] = sum;
                    }
                }
            }

            // Return reduced density matrix
            let output = MatrixData {
                data: reduced
                    .as_slice()
                    .iter()
                    .flat_map(|&x| x.to_le_bytes())
                    .collect::<Vec<u8>>(),
                shape: (reduced.nrows(), reduced.ncols()),
                precision: MatrixPrecision::F64,
                symmetry: Some(MatrixSymmetry::Hermitian),
                sparsity: 0.0,
            };

            bincode::serialize(&output).map_err(|e| ScienceError::Internal(e.to_string()))
        } else {
            Err(ScienceError::InvalidParams(
                "No density matrix provided".to_string(),
            ))
        }
    }

    fn execute_expectation_value(
        &mut self,
        input: &[u8],
        _params: &[u8],
    ) -> ScienceResult<Vec<u8>> {
        // Compute <ψ|O|ψ> or Tr(ρO) for observable O
        let (state_data, observable_data): (QuantumState, MatrixData) =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let observable = self.deserialize_matrix(&observable_data)?;

        let expectation = if let Some(density_matrix) = state_data.density_matrix {
            // Mixed state: Tr(ρO)
            let rho = self.deserialize_matrix(&density_matrix)?;
            (rho * &observable).trace()
        } else if let Some(wavefunction) = state_data.wavefunction {
            // Pure state: <ψ|O|ψ>
            // Convert wavefunction to vector
            let psi = DVector::from_iterator(
                wavefunction.len(),
                wavefunction.iter().map(|c| c.re), // Real part for now
            );
            let o_psi = &observable * &psi;
            psi.dot(&o_psi)
        } else {
            return Err(ScienceError::InvalidParams(
                "No quantum state provided".to_string(),
            ));
        };

        // Return expectation value as f64
        Ok(expectation.to_le_bytes().to_vec())
    }

    fn execute_commutator(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        // Compute [A, B] = AB - BA
        let params_str = std::str::from_utf8(params).unwrap_or("{}");
        let params_json: serde_json::Value =
            serde_json::from_str(params_str).unwrap_or(serde_json::json!({}));

        let compute_anti = params_json
            .get("anti_commutator")
            .and_then(|v| v.as_bool())
            .unwrap_or(false);

        let (a_data, b_data): (MatrixData, MatrixData) =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let a = self.deserialize_matrix(&a_data)?;
        let b = self.deserialize_matrix(&b_data)?;

        // Check dimensions
        if a.nrows() != b.nrows() || a.ncols() != b.ncols() {
            return Err(ScienceError::InvalidParams(
                "Matrices must have same dimensions for commutator".to_string(),
            ));
        }

        let result = if compute_anti {
            // Anti-commutator: {A, B} = AB + BA
            &a * &b + &b * &a
        } else {
            // Commutator: [A, B] = AB - BA
            &a * &b - &b * &a
        };

        // Return result as MatrixData
        let output = MatrixData {
            data: result
                .as_slice()
                .iter()
                .flat_map(|&x| x.to_le_bytes())
                .collect::<Vec<u8>>(),
            shape: (result.nrows(), result.ncols()),
            precision: MatrixPrecision::F64,
            symmetry: None,
            sparsity: 0.0,
        };

        bincode::serialize(&output).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    // ------------------------------------------------------------------------
    // CONTROL THEORY & DYNAMICAL SYSTEMS
    // ------------------------------------------------------------------------

    fn execute_lyapunov(&mut self, input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        // Solve AX + XA^T + Q = 0 (continuous Lyapunov equation)
        let (a_data, q_data): (MatrixData, MatrixData) =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let a = self.deserialize_matrix(&a_data)?;
        let q = self.deserialize_matrix(&q_data)?;

        // Check dimensions
        if a.nrows() != a.ncols() || q.nrows() != q.ncols() || a.nrows() != q.nrows() {
            return Err(ScienceError::InvalidParams(
                "A must be square, Q must be square with same dimension as A".to_string(),
            ));
        }

        // Bartels-Stewart algorithm using Schur decomposition

        // Compute Schur decomposition of A: A = U * T * U^T
        // unpack() consumes schur, so we call it once
        let (u, t) = a.clone().schur().unpack();

        // Transform Q: Q' = U^T * Q * U
        let q_prime = u.transpose() * &q * &u;

        // Solve triangular Sylvester equation: T*X' + X'*T^T + Q' = 0
        let x_prime = self.solve_triangular_sylvester(&t, &q_prime)?;

        // Transform back: X = U * X' * U^T
        let x = &u * x_prime * u.transpose();

        let output = MatrixData {
            data: x
                .as_slice()
                .iter()
                .flat_map(|&v| v.to_le_bytes())
                .collect::<Vec<u8>>(),
            shape: (x.nrows(), x.ncols()),
            precision: MatrixPrecision::F64,
            symmetry: Some(MatrixSymmetry::Symmetric),
            sparsity: 0.0,
        };

        bincode::serialize(&output).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    /// Solve triangular Sylvester equation: T*X + X*T^T + Q = 0
    /// where T is upper quasi-triangular (from Schur decomposition)
    fn solve_triangular_sylvester(
        &self,
        t: &DMatrix<f64>,
        q: &DMatrix<f64>,
    ) -> ScienceResult<DMatrix<f64>> {
        let n = t.nrows();
        let mut x = DMatrix::zeros(n, n);

        // Back-substitution for upper triangular system
        for i in (0..n).rev() {
            for j in (0..n).rev() {
                let mut sum = -q[(i, j)];

                // Subtract contributions from already computed elements
                for k in (i + 1)..n {
                    sum -= t[(i, k)] * x[(k, j)];
                }
                for k in (j + 1)..n {
                    sum -= x[(i, k)] * t[(k, j)];
                }

                // Solve for x[i,j]
                let denom = t[(i, i)] + t[(j, j)];
                if denom.abs() < 1e-14 {
                    return Err(ScienceError::Internal(
                        "Singular system in Lyapunov solver".to_string(),
                    ));
                }
                x[(i, j)] = sum / denom;
            }
        }

        Ok(x)
    }

    // ------------------------------------------------------------------------
    // HELPER METHODS
    // ------------------------------------------------------------------------

    fn parse_two_vectors(&self, input: &[u8]) -> ScienceResult<(DVector<f64>, DVector<f64>)> {
        // Input format: [len_a as u32][a_data][len_b as u32][b_data]
        if input.len() < 8 {
            return Err(ScienceError::InvalidParams("Input too short".to_string()));
        }

        let len_a = u32::from_le_bytes(input[0..4].try_into().unwrap()) as usize;
        let offset_a = 4;
        let offset_b = offset_a + len_a * 8;

        if input.len() < offset_b + 4 {
            return Err(ScienceError::InvalidParams("Input truncated".to_string()));
        }

        let len_b = u32::from_le_bytes(input[offset_b..offset_b + 4].try_into().unwrap()) as usize;

        if input.len() < offset_b + 4 + len_b * 8 {
            return Err(ScienceError::InvalidParams("Input truncated".to_string()));
        }

        // Parse vectors
        let mut a = DVector::zeros(len_a);
        for i in 0..len_a {
            let start = offset_a + i * 8;
            let bytes = &input[start..start + 8];
            a[i] = f64::from_le_bytes(bytes.try_into().unwrap());
        }

        let mut b = DVector::zeros(len_b);
        for i in 0..len_b {
            let start = offset_b + 4 + i * 8;
            let bytes = &input[start..start + 8];
            b[i] = f64::from_le_bytes(bytes.try_into().unwrap());
        }

        Ok((a, b))
    }

    fn parse_matrix(
        &self,
        input: &[u8],
        offset: usize,
        rows: usize,
        cols: usize,
    ) -> ScienceResult<DMatrix<f64>> {
        let required_len = offset + rows * cols * 8;
        if input.len() < required_len {
            return Err(ScienceError::InvalidParams(format!(
                "Input buffer too small: need {} bytes, have {}",
                required_len,
                input.len()
            )));
        }

        let mut matrix = DMatrix::zeros(rows, cols);

        for i in 0..rows {
            for j in 0..cols {
                let start = offset + (i * cols + j) * 8;
                let bytes = &input[start..start + 8];
                matrix[(i, j)] = f64::from_le_bytes(bytes.try_into().unwrap());
            }
        }

        Ok(matrix)
    }

    fn parse_single_matrix(&self, input: &[u8], params: &[u8]) -> ScienceResult<DMatrix<f64>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let rows = params_json["rows"]
            .as_u64()
            .ok_or_else(|| ScienceError::InvalidParams("Missing rows".to_string()))?
            as usize;
        let cols = params_json["cols"]
            .as_u64()
            .ok_or_else(|| ScienceError::InvalidParams("Missing cols".to_string()))?
            as usize;

        self.parse_matrix(input, 0, rows, cols)
    }

    fn parse_vector(&self, input: &[u8], offset: usize, len: usize) -> ScienceResult<DVector<f64>> {
        let required_len = offset + len * 8;
        if input.len() < required_len {
            return Err(ScienceError::InvalidParams(
                "Input buffer too small".to_string(),
            ));
        }

        let mut vec = DVector::zeros(len);
        for i in 0..len {
            let start = offset + i * 8;
            let bytes = &input[start..start + 8];
            vec[i] = f64::from_le_bytes(bytes.try_into().unwrap());
        }

        Ok(vec)
    }

    fn serialize_matrix(&self, matrix: &DMatrix<f64>) -> ScienceResult<Vec<u8>> {
        let mut result = Vec::with_capacity(matrix.nrows() * matrix.ncols() * 8);

        // Write dimensions first
        result.extend_from_slice(&(matrix.nrows() as u32).to_le_bytes());
        result.extend_from_slice(&(matrix.ncols() as u32).to_le_bytes());

        // Write data in row-major order
        for i in 0..matrix.nrows() {
            for j in 0..matrix.ncols() {
                result.extend_from_slice(&matrix[(i, j)].to_le_bytes());
            }
        }

        Ok(result)
    }

    fn serialize_vector(&self, vector: &DVector<f64>) -> ScienceResult<Vec<u8>> {
        let mut result = Vec::with_capacity(vector.len() * 8);

        // Write length first
        result.extend_from_slice(&(vector.len() as u32).to_le_bytes());

        // Write data
        for i in 0..vector.len() {
            result.extend_from_slice(&vector[i].to_le_bytes());
        }

        Ok(result)
    }

    fn deserialize_matrix(&self, mat_data: &MatrixData) -> ScienceResult<DMatrix<f64>> {
        let (rows, cols) = mat_data.shape;

        match mat_data.precision {
            MatrixPrecision::F64 => {
                let mut matrix = DMatrix::zeros(rows, cols);

                for i in 0..rows {
                    for j in 0..cols {
                        let start = (i * cols + j) * 8;
                        let bytes = &mat_data.data[start..start + 8];
                        matrix[(i, j)] = f64::from_le_bytes(bytes.try_into().unwrap());
                    }
                }

                Ok(matrix)
            }
            _ => Err(ScienceError::InvalidParams(
                "Unsupported matrix precision".to_string(),
            )),
        }
    }

    // ------------------------------------------------------------------------
    // SPOT VALIDATION FOR PROOF-OF-SIMULATION
    // ------------------------------------------------------------------------

    fn validate_spot_matrix_multiply(
        &mut self,
        input: &[u8],
        params: &[u8],
        spot_seed: &[u8],
    ) -> ScienceResult<Vec<u8>> {
        // Deterministically select a single element (i,j) to compute
        let mut hasher = blake3::Hasher::new();
        hasher.update(spot_seed);
        let hash = hasher.finalize();
        let hash_bytes = hash.as_bytes();

        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;
        let rows_a = params_json["rows_a"].as_u64().unwrap_or(0) as usize;
        // let cols_a = params_json["cols_a"].as_u64().unwrap_or(0) as usize;
        let cols_b = params_json["cols_b"].as_u64().unwrap_or(0) as usize;
        let cols_a = params_json["cols_a"].as_u64().unwrap_or(0) as usize;

        // Use hash to select element
        let i = (u64::from_le_bytes(hash_bytes[0..8].try_into().unwrap()) as usize) % rows_a;
        let j = (u64::from_le_bytes(hash_bytes[8..16].try_into().unwrap()) as usize) % cols_b;

        // Parse matrices
        let a = self.parse_matrix(input, 0, rows_a, cols_a)?;
        let b = self.parse_matrix(input, rows_a * cols_a * 8, cols_a, cols_b)?;

        // Compute only element (i,j) of A * B
        let mut element = 0.0;
        for k in 0..cols_a {
            element += a[(i, k)] * b[(k, j)];
        }

        // Return the element and its indices for verification
        let mut result = Vec::new();
        result.extend_from_slice(&(i as u32).to_le_bytes());
        result.extend_from_slice(&(j as u32).to_le_bytes());
        result.extend_from_slice(&element.to_le_bytes());

        Ok(result)
    }
}

impl Default for MathProxy {
    fn default() -> Self {
        Self::new(
            Rc::new(RefCell::new(ComputationCache::new())),
            Rc::new(RefCell::new(Telemetry::default())),
        )
    }
}

impl ScienceProxy for MathProxy {
    fn name(&self) -> &'static str {
        "math"
    }

    fn methods(&self) -> Vec<&'static str> {
        vec![
            "dot",
            "cross",
            "normalize",
            "matrix_multiply",
            "inverse",
            "eigenvalues",
            "svd",
            "qr_decomposition",
            "cholesky",
            "solve_linear",
            "tensor_product",
            "partial_trace",
            "expectation_value",
            "commutator",
            "matrix_exp",
            "matrix_log",
            "determinant",
            "trace",
            "condition_number",
            "pseudoinverse",
            "schur_decomposition",
            "lyapunov_solve",
            "riccati_solve",
        ]
    }

    fn execute(&mut self, method: &str, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        // Check cache first
        let request_hash = self.compute_request_hash(method, input, params);
        if let Some(entry) = self.cache.borrow_mut().get(&request_hash, None) {
            self.telemetry.borrow_mut().cache_hits += 1;
            return Ok(entry.data);
        }

        self.telemetry.borrow_mut().cache_misses += 1;
        self.telemetry.borrow_mut().computations += 1;

        // Execute method
        let executor = self
            .method_implementations
            .get(method)
            .ok_or_else(|| ScienceError::MethodNotFound(method.to_string()))?;

        let result = executor(self, input, params)?;

        // Cache result
        let entry = CacheEntry {
            data: result.clone(),
            result_hash: request_hash.clone(), // Use request hash as placeholder for result hash if not computed separately
            timestamp: 0,                      // Placeholder
            access_count: 1,
            scale: Default::default(),
            proof: Default::default(),
        };
        self.cache.borrow_mut().put(request_hash, entry);

        Ok(result)
    }

    fn validate_spot(
        &mut self,
        method: &str,
        input: &[u8],
        params: &[u8],
        spot_seed: &[u8],
    ) -> ScienceResult<Vec<u8>> {
        // Spot validation for Proof-of-Simulation
        match method {
            "matrix_multiply" => self.validate_spot_matrix_multiply(input, params, spot_seed),
            "dot" => {
                // Validate dot product by computing a subset of terms
                let (a, b) = self.parse_two_vectors(input)?;
                let n = a.len().min(b.len());

                // Deterministically select stride and offset
                let mut hasher = blake3::Hasher::new();
                hasher.update(spot_seed);
                let hash = hasher.finalize();
                let hash_bytes = hash.as_bytes();

                let stride =
                    (u64::from_le_bytes(hash_bytes[0..8].try_into().unwrap()) as usize % 10) + 1;
                let offset =
                    (u64::from_le_bytes(hash_bytes[8..16].try_into().unwrap()) as usize) % stride;

                // Compute partial dot product
                let mut partial = 0.0;
                let mut count = 0;
                for i in (offset..n).step_by(stride) {
                    partial += a[i] * b[i];
                    count += 1;
                }

                // Return partial result and validation metadata
                let mut result = Vec::new();
                result.extend_from_slice(&(count as u32).to_le_bytes());
                result.extend_from_slice(&partial.to_le_bytes());
                result.extend_from_slice(&(stride as u32).to_le_bytes());
                result.extend_from_slice(&(offset as u32).to_le_bytes());

                Ok(result)
            }
            _ => {
                // For other methods, compute full result (expensive but accurate)
                // In production, each method should have its own spot validation
                self.execute(method, input, params)
            }
        }
    }
}

impl MathProxy {
    fn compute_request_hash(&self, method: &str, input: &[u8], params: &[u8]) -> String {
        let mut hasher = blake3::Hasher::new();
        hasher.update(method.as_bytes());
        hasher.update(b"@math_v1.0");
        hasher.update(input);
        hasher.update(params);
        hasher.update(&self.local_node_id.to_le_bytes());
        hasher.finalize().to_hex().to_string()
    }

    fn execute_normalize(&mut self, input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        let vec = self.parse_vector(input, 0, input.len() / 8)?;
        let normalized = vec.normalize();
        self.serialize_vector(&normalized)
    }

    fn execute_qr(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let matrix = self.parse_single_matrix(input, params)?;
        let qr = matrix.qr();
        let q = qr.q();
        let r = qr.r();
        let mut result = self.serialize_matrix(&q)?;
        result.extend(self.serialize_matrix(&r)?);
        Ok(result)
    }

    fn execute_cholesky(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let matrix = self.parse_single_matrix(input, params)?;
        let cholesky = matrix
            .cholesky()
            .ok_or_else(|| ScienceError::Internal("Not positive definite".into()))?;
        self.serialize_matrix(&cholesky.l())
    }

    fn execute_matrix_exp(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let matrix = self.parse_single_matrix(input, params)?;
        if matrix.nrows() != matrix.ncols() {
            return Err(ScienceError::InvalidParams("Not square".into()));
        }
        let n = matrix.nrows();
        let mut res = DMatrix::<f64>::identity(n, n);
        let mut term = DMatrix::<f64>::identity(n, n);
        for i in 1..15 {
            term = &term * &matrix / (i as f64);
            res += &term;
        }
        self.serialize_matrix(&res)
    }

    fn execute_matrix_log(&mut self, _input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        Err(ScienceError::MethodNotFound(
            "Matrix log not implemented".into(),
        ))
    }

    fn execute_determinant(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let matrix = self.parse_single_matrix(input, params)?;
        let det = matrix.determinant();
        Ok(det.to_le_bytes().to_vec())
    }

    fn execute_trace(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let matrix = self.parse_single_matrix(input, params)?;
        let tr = matrix.trace();
        Ok(tr.to_le_bytes().to_vec())
    }

    fn execute_condition_number(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let matrix = self.parse_single_matrix(input, params)?;
        let svd = SVD::new(matrix, false, false);
        let max_s = svd.singular_values.max();
        let min_s = svd.singular_values.min();
        let cond = if min_s > 1e-12 {
            max_s / min_s
        } else {
            f64::INFINITY
        };
        Ok(cond.to_le_bytes().to_vec())
    }

    fn execute_pseudoinverse(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let matrix = self.parse_single_matrix(input, params)?;
        let svd = SVD::new(matrix, true, true);
        let pseudo = svd
            .pseudo_inverse(1e-12)
            .map_err(|e| ScienceError::Internal(e.to_string()))?;
        self.serialize_matrix(&pseudo)
    }

    fn execute_schur(&mut self, _input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        Err(ScienceError::MethodNotFound(
            "Schur decomposition not implemented".into(),
        ))
    }

    fn execute_riccati(&mut self, _input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        Err(ScienceError::MethodNotFound(
            "Riccati solver not implemented".into(),
        ))
    }

    fn execute_distributed_matmul(
        &mut self,
        input: &[u8],
        params: &[u8],
    ) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let total_shards = params_json["total_shards"].as_u64().unwrap_or(1) as usize;
        let shard_id = params_json["shard_id"].as_u64().unwrap_or(0) as usize;
        let effective_shard = if params_json.get("shard_id").is_some() {
            shard_id
        } else {
            self.shard_id as usize
        };

        let rows_a = params_json["rows_a"]
            .as_u64()
            .ok_or(ScienceError::InvalidParams("Missing rows_a".into()))?
            as usize;
        let cols_a = params_json["cols_a"]
            .as_u64()
            .ok_or(ScienceError::InvalidParams("Missing cols_a".into()))?
            as usize;
        let rows_b = params_json["rows_b"]
            .as_u64()
            .ok_or(ScienceError::InvalidParams("Missing rows_b".into()))?
            as usize;
        let cols_b = params_json["cols_b"]
            .as_u64()
            .ok_or(ScienceError::InvalidParams("Missing cols_b".into()))?
            as usize;

        if cols_a != rows_b {
            return Err(ScienceError::InvalidParams("Dimension mismatch".into()));
        }

        let rows_per_shard = rows_a.div_ceil(total_shards);
        let start_row = effective_shard * rows_per_shard;
        let end_row = std::cmp::min(start_row + rows_per_shard, rows_a);
        let local_rows = end_row.saturating_sub(start_row);

        if local_rows == 0 {
            return Ok(vec![]);
        }

        let a = self.parse_matrix(input, 0, local_rows, cols_a)?;
        let offset_b = local_rows * cols_a * 8;
        let b = self.parse_matrix(input, offset_b, rows_b, cols_b)?;

        let c_local = &a * &b;
        self.serialize_matrix(&c_local)
    }
}
