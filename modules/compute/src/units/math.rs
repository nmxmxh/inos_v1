use crate::engine::{ComputeError, ResourceLimits, UnitProxy};
use async_trait::async_trait;
use sdk::pingpong::PingPongBuffer;
use serde_json::Value as JsonValue;

/// Math unit providing linear algebra operations via nalgebra library proxy
///
/// Architecture: Rust validates + prepares, computes via nalgebra
/// - Full-featured: Complete linear algebra (matrices, vectors, quaternions)
/// - GPU-ready: Can output matrices directly to SAB for GPU consumption
/// - Generic: Works with any entity count or transformation requirements
///
/// Operations Supported:
/// - Matrices: create, multiply, invert, transpose, decompose
/// - Vectors: normalize, dot, cross, length, lerp
/// - Quaternions: from_euler, to_euler, slerp, multiply
/// - Transforms: compose, decompose, apply_to_points
/// - Batch: batch_transform, compute_instance_matrices
#[derive(Default)]
struct PersistentScratch {
    input_data: Vec<u8>,
    output_data: Vec<u8>,
}

pub struct MathUnit {
    config: MathConfig,
    scratch: std::sync::Mutex<PersistentScratch>,
}

#[derive(Clone)]
struct MathConfig {
    max_batch_size: usize,
    #[allow(dead_code)]
    max_matrix_count: usize,
}

impl Default for MathConfig {
    fn default() -> Self {
        Self {
            max_batch_size: 100_000,     // 100k entities
            max_matrix_count: 1_000_000, // 1M matrices
        }
    }
}

impl MathUnit {
    pub fn new() -> Self {
        log::info!("Math unit initialized (nalgebra library)");
        Self {
            config: MathConfig::default(),
            scratch: std::sync::Mutex::new(PersistentScratch::default()),
        }
    }

    /// Validate Matrix4x4 structure
    fn validate_matrix4(&self, mat: &JsonValue, name: &str) -> Result<(), ComputeError> {
        if let Some(arr) = mat.as_array() {
            if arr.len() != 16 {
                return Err(ComputeError::InvalidParams(format!(
                    "{} must have 16 elements (4x4 matrix), got {}",
                    name,
                    arr.len()
                )));
            }
            for (i, v) in arr.iter().enumerate() {
                if !v.is_number() {
                    return Err(ComputeError::InvalidParams(format!(
                        "{}[{}] must be a number",
                        name, i
                    )));
                }
            }
            Ok(())
        } else {
            Err(ComputeError::InvalidParams(format!(
                "{} must be an array of 16 numbers",
                name
            )))
        }
    }

    /// Validate Vector3 structure
    fn validate_vector3(&self, vec: &JsonValue, name: &str) -> Result<(), ComputeError> {
        if let Some(obj) = vec.as_object() {
            for axis in ["x", "y", "z"] {
                obj.get(axis).and_then(|v| v.as_f64()).ok_or_else(|| {
                    ComputeError::InvalidParams(format!("{}.{} must be a number", name, axis))
                })?;
            }
            Ok(())
        } else if let Some(arr) = vec.as_array() {
            if arr.len() != 3 {
                return Err(ComputeError::InvalidParams(format!(
                    "{} array must have 3 elements",
                    name
                )));
            }
            Ok(())
        } else {
            Err(ComputeError::InvalidParams(format!(
                "{} must be {{x, y, z}} object or [x, y, z] array",
                name
            )))
        }
    }

    /// Validate Quaternion structure
    #[allow(dead_code)]
    fn validate_quaternion(&self, quat: &JsonValue, name: &str) -> Result<(), ComputeError> {
        if let Some(obj) = quat.as_object() {
            for comp in ["x", "y", "z", "w"] {
                obj.get(comp).and_then(|v| v.as_f64()).ok_or_else(|| {
                    ComputeError::InvalidParams(format!("{}.{} must be a number", name, comp))
                })?;
            }
            Ok(())
        } else if let Some(arr) = quat.as_array() {
            if arr.len() != 4 {
                return Err(ComputeError::InvalidParams(format!(
                    "{} array must have 4 elements",
                    name
                )));
            }
            Ok(())
        } else {
            Err(ComputeError::InvalidParams(format!(
                "{} must be {{x, y, z, w}} object or [x, y, z, w] array",
                name
            )))
        }
    }

    /// Validate Euler angles structure
    fn validate_euler(&self, euler: &JsonValue, name: &str) -> Result<(), ComputeError> {
        if let Some(obj) = euler.as_object() {
            // Euler can be {x, y, z} (radians) or {pitch, yaw, roll}
            let has_xyz = obj.contains_key("x") && obj.contains_key("y") && obj.contains_key("z");
            let has_pyr =
                obj.contains_key("pitch") && obj.contains_key("yaw") && obj.contains_key("roll");

            if !has_xyz && !has_pyr {
                return Err(ComputeError::InvalidParams(format!(
                    "{} must have either {{x, y, z}} or {{pitch, yaw, roll}}",
                    name
                )));
            }
            Ok(())
        } else {
            Err(ComputeError::InvalidParams(format!(
                "{} must be an object with angle components",
                name
            )))
        }
    }

    /// Validate batch operation parameters
    fn validate_batch_params(&self, params: &JsonValue) -> Result<(), ComputeError> {
        let count = params
            .get("count")
            .and_then(|v| v.as_u64())
            .ok_or_else(|| ComputeError::InvalidParams("Missing count".to_string()))?;

        if count as usize > self.config.max_batch_size {
            return Err(ComputeError::InvalidParams(format!(
                "count {} exceeds max_batch_size {}",
                count, self.config.max_batch_size
            )));
        }

        Ok(())
    }

    /// Create library proxy response for nalgebra
    fn proxy_response(&self, method: &str, params: JsonValue) -> Result<Vec<u8>, ComputeError> {
        let response = serde_json::json!({
            "library": "nalgebra",
            "method": method,
            "params": params
        });

        serde_json::to_vec(&response).map_err(|e| ComputeError::ExecutionFailed(e.to_string()))
    }

    /// Execute a computation directly (for methods we can compute in WASM)
    fn compute_result(&self, result: JsonValue) -> Result<Vec<u8>, ComputeError> {
        serde_json::to_vec(&result).map_err(|e| ComputeError::ExecutionFailed(e.to_string()))
    }
}

impl Default for MathUnit {
    fn default() -> Self {
        Self::new()
    }
}

#[async_trait]
impl UnitProxy for MathUnit {
    fn service_name(&self) -> &str {
        "math"
    }

    fn actions(&self) -> Vec<&str> {
        vec![
            // Matrix Operations
            "matrix_identity",
            "matrix_from_translation",
            "matrix_from_rotation",
            "matrix_from_scale",
            "matrix_from_euler",
            "matrix_from_quaternion",
            "matrix_compose", // translation + rotation + scale
            "matrix_decompose",
            "matrix_multiply",
            "matrix_invert",
            "matrix_transpose",
            "matrix_determinant",
            // Vector Operations
            "vector_normalize",
            "vector_length",
            "vector_dot",
            "vector_cross",
            "vector_lerp",
            "vector_slerp",
            "vector_add",
            "vector_subtract",
            "vector_scale",
            "vector_distance",
            // Quaternion Operations
            "quaternion_identity",
            "quaternion_from_euler",
            "quaternion_from_axis_angle",
            "quaternion_to_euler",
            "quaternion_multiply",
            "quaternion_slerp",
            "quaternion_conjugate",
            "quaternion_invert",
            // Transform Operations
            "transform_point",
            "transform_direction",
            "transform_normal",
            "look_at",
            "perspective",
            "orthographic",
            // Batch Operations (GPU-ready)
            "batch_compose_matrices",
            "batch_transform_points",
            "compute_instance_matrices",
        ]
    }

    fn resource_limits(&self) -> ResourceLimits {
        ResourceLimits {
            max_input_size: 100 * 1024 * 1024,  // 100MB for large point clouds
            max_output_size: 100 * 1024 * 1024, // 100MB for batch matrices
            max_memory_pages: 4096,             // 256MB
            timeout_ms: 5000,                   // 5s
            max_fuel: 10_000_000_000,           // 10B instructions
        }
    }

    async fn execute(
        &self,
        method: &str,
        _input: &[u8],
        params: &[u8],
    ) -> Result<Vec<u8>, ComputeError> {
        let params: JsonValue = serde_json::from_slice(params)
            .map_err(|e| ComputeError::InvalidParams(format!("Invalid JSON: {}", e)))?;

        match method {
            // Matrix Identity - can compute directly
            "matrix_identity" => {
                use nalgebra::Matrix4;
                let m: Matrix4<f64> = Matrix4::identity();
                let data: Vec<f64> = m.iter().cloned().collect();
                self.compute_result(serde_json::json!({ "matrix": data }))
            }

            // Matrix from Translation
            "matrix_from_translation" => {
                let translation = params.get("translation").ok_or_else(|| {
                    ComputeError::InvalidParams("Missing translation".to_string())
                })?;
                self.validate_vector3(translation, "translation")?;

                use nalgebra::{Matrix4, Vector3};
                let x = translation.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0);
                let y = translation.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0);
                let z = translation.get("z").and_then(|v| v.as_f64()).unwrap_or(0.0);

                let m = Matrix4::new_translation(&Vector3::new(x, y, z));
                let data: Vec<f64> = m.iter().cloned().collect();
                self.compute_result(serde_json::json!({ "matrix": data }))
            }

            // Matrix from Euler angles
            "matrix_from_euler" => {
                let euler = params
                    .get("euler")
                    .ok_or_else(|| ComputeError::InvalidParams("Missing euler".to_string()))?;
                self.validate_euler(euler, "euler")?;

                use nalgebra::Rotation3;
                let x = euler
                    .get("x")
                    .or_else(|| euler.get("pitch"))
                    .and_then(|v| v.as_f64())
                    .unwrap_or(0.0);
                let y = euler
                    .get("y")
                    .or_else(|| euler.get("yaw"))
                    .and_then(|v| v.as_f64())
                    .unwrap_or(0.0);
                let z = euler
                    .get("z")
                    .or_else(|| euler.get("roll"))
                    .and_then(|v| v.as_f64())
                    .unwrap_or(0.0);

                let rotation = Rotation3::from_euler_angles(x, y, z);
                let m = rotation.to_homogeneous();
                let data: Vec<f64> = m.iter().cloned().collect();
                self.compute_result(serde_json::json!({ "matrix": data }))
            }

            // Matrix from Scale
            "matrix_from_scale" => {
                let scale = params
                    .get("scale")
                    .ok_or_else(|| ComputeError::InvalidParams("Missing scale".to_string()))?;

                use nalgebra::Matrix4;
                let (sx, sy, sz) = if let Some(n) = scale.as_f64() {
                    (n, n, n) // Uniform scale
                } else {
                    self.validate_vector3(scale, "scale")?;
                    let x = scale.get("x").and_then(|v| v.as_f64()).unwrap_or(1.0);
                    let y = scale.get("y").and_then(|v| v.as_f64()).unwrap_or(1.0);
                    let z = scale.get("z").and_then(|v| v.as_f64()).unwrap_or(1.0);
                    (x, y, z)
                };

                let m = Matrix4::new_nonuniform_scaling(&nalgebra::Vector3::new(sx, sy, sz));
                let data: Vec<f64> = m.iter().cloned().collect();
                self.compute_result(serde_json::json!({ "matrix": data }))
            }

            // Matrix Multiply
            "matrix_multiply" => {
                let a = params
                    .get("a")
                    .ok_or_else(|| ComputeError::InvalidParams("Missing matrix a".to_string()))?;
                let b = params
                    .get("b")
                    .ok_or_else(|| ComputeError::InvalidParams("Missing matrix b".to_string()))?;

                self.validate_matrix4(a, "a")?;
                self.validate_matrix4(b, "b")?;

                use nalgebra::Matrix4;
                let a_arr: Vec<f64> = a
                    .as_array()
                    .unwrap()
                    .iter()
                    .map(|v| v.as_f64().unwrap())
                    .collect();
                let b_arr: Vec<f64> = b
                    .as_array()
                    .unwrap()
                    .iter()
                    .map(|v| v.as_f64().unwrap())
                    .collect();

                let ma = Matrix4::from_column_slice(&a_arr);
                let mb = Matrix4::from_column_slice(&b_arr);
                let result = ma * mb;

                let data: Vec<f64> = result.iter().cloned().collect();
                self.compute_result(serde_json::json!({ "matrix": data }))
            }

            // Matrix Invert
            "matrix_invert" => {
                let m = params
                    .get("matrix")
                    .ok_or_else(|| ComputeError::InvalidParams("Missing matrix".to_string()))?;
                self.validate_matrix4(m, "matrix")?;

                use nalgebra::Matrix4;
                let arr: Vec<f64> = m
                    .as_array()
                    .unwrap()
                    .iter()
                    .map(|v| v.as_f64().unwrap())
                    .collect();
                let mat = Matrix4::from_column_slice(&arr);

                if let Some(inv) = mat.try_inverse() {
                    let data: Vec<f64> = inv.iter().cloned().collect();
                    self.compute_result(serde_json::json!({ "matrix": data, "invertible": true }))
                } else {
                    self.compute_result(serde_json::json!({ "matrix": null, "invertible": false }))
                }
            }

            // Quaternion from Euler
            "quaternion_from_euler" => {
                let euler = params
                    .get("euler")
                    .ok_or_else(|| ComputeError::InvalidParams("Missing euler".to_string()))?;
                self.validate_euler(euler, "euler")?;

                use nalgebra::UnitQuaternion;
                let x = euler
                    .get("x")
                    .or_else(|| euler.get("pitch"))
                    .and_then(|v| v.as_f64())
                    .unwrap_or(0.0);
                let y = euler
                    .get("y")
                    .or_else(|| euler.get("yaw"))
                    .and_then(|v| v.as_f64())
                    .unwrap_or(0.0);
                let z = euler
                    .get("z")
                    .or_else(|| euler.get("roll"))
                    .and_then(|v| v.as_f64())
                    .unwrap_or(0.0);

                let q = UnitQuaternion::from_euler_angles(x, y, z);
                self.compute_result(serde_json::json!({
                    "quaternion": {
                        "x": q.i,
                        "y": q.j,
                        "z": q.k,
                        "w": q.w
                    }
                }))
            }

            // Vector Normalize
            "vector_normalize" => {
                let v = params
                    .get("vector")
                    .ok_or_else(|| ComputeError::InvalidParams("Missing vector".to_string()))?;
                self.validate_vector3(v, "vector")?;

                use nalgebra::Vector3;
                let x = v.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0);
                let y = v.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0);
                let z = v.get("z").and_then(|v| v.as_f64()).unwrap_or(0.0);

                let vec = Vector3::new(x, y, z);
                let normalized = vec.normalize();

                self.compute_result(serde_json::json!({
                    "vector": { "x": normalized.x, "y": normalized.y, "z": normalized.z }
                }))
            }

            // Vector Cross Product
            "vector_cross" => {
                let a = params
                    .get("a")
                    .ok_or_else(|| ComputeError::InvalidParams("Missing vector a".to_string()))?;
                let b = params
                    .get("b")
                    .ok_or_else(|| ComputeError::InvalidParams("Missing vector b".to_string()))?;

                self.validate_vector3(a, "a")?;
                self.validate_vector3(b, "b")?;

                use nalgebra::Vector3;
                let va = Vector3::new(
                    a.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0),
                    a.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0),
                    a.get("z").and_then(|v| v.as_f64()).unwrap_or(0.0),
                );
                let vb = Vector3::new(
                    b.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0),
                    b.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0),
                    b.get("z").and_then(|v| v.as_f64()).unwrap_or(0.0),
                );

                let cross = va.cross(&vb);
                self.compute_result(serde_json::json!({
                    "vector": { "x": cross.x, "y": cross.y, "z": cross.z }
                }))
            }

            // Batch Compose Matrices (GPU-ready)
            "batch_compose_matrices" => {
                self.validate_batch_params(&params)?;

                let _positions = params.get("positions").ok_or_else(|| {
                    ComputeError::InvalidParams("Missing positions array".to_string())
                })?;

                // For batch operations, we validate structure and proxy to nalgebra
                // In production, this would compute directly and write to SAB
                self.proxy_response(method, params)
            }

            // Compute Instance Matrices (for instanced rendering) using Ping-Pong Buffers
            "compute_instance_matrices" => {
                self.validate_batch_params(&params)?;

                let count = params["count"].as_u64().unwrap_or(0) as usize;

                // Diagnostic: Track call frequency
                static CALL_COUNT: std::sync::atomic::AtomicU32 =
                    std::sync::atomic::AtomicU32::new(0);
                let call_num = CALL_COUNT.fetch_add(1, std::sync::atomic::Ordering::Relaxed);

                // If we have SAB access, we can do zero-copy compute
                if let Some(sab) = crate::get_cached_sab() {
                    use nalgebra::{Matrix4, Rotation3, Vector3};
                    use sdk::layout::BIRD_STRIDE;

                    // Use Ping-Pong buffer accessors
                    let bird_ping_pong = PingPongBuffer::bird_buffer(sab.clone());
                    let matrix_ping_pong = PingPongBuffer::matrix_buffer(sab.clone());

                    // IMPORTANT: Math unit reads from the bird buffer that was just written (ACTIVE BIRD BUFFER)
                    // and writes to the inactive matrix buffer.
                    let bird_info = bird_ping_pong.read_buffer_info();
                    let matrix_info = matrix_ping_pong.write_buffer_info();

                    if call_num % 100 == 0 {
                        log::info!(
                            "[Math] compute_instance_matrices #{} | count={} | Bird Epoch={} @ 0x{:X} | Matrix Epoch={} @ 0x{:X}",
                            call_num,
                            count,
                            bird_info.epoch,
                            bird_info.offset,
                            matrix_info.epoch,
                            matrix_info.offset
                        );
                    }

                    // ========== BULK I/O OPTIMIZATION ==========
                    let mut scratch = self
                        .scratch
                        .lock()
                        .map_err(|_| ComputeError::ExecutionFailed("Mutex lock failed".into()))?;

                    // Resize buffers if needed
                    let input_size = count * BIRD_STRIDE;
                    if scratch.input_data.len() < input_size {
                        scratch.input_data.resize(input_size, 0);
                    }
                    // Pre-allocate output buffer for 8 parts × count birds × 64 bytes per matrix
                    const PARTS: usize = 8;
                    let output_size = PARTS * count * 64;
                    if scratch.output_data.len() < output_size {
                        scratch.output_data.resize(output_size, 0);
                    }

                    // Read ALL bird base data in one call
                    sab.read_raw(bird_info.offset, &mut scratch.input_data[..input_size])
                        .map_err(|e| ComputeError::ExecutionFailed(e))?;

                    for i in 0..count {
                        let bird_base = i * BIRD_STRIDE;

                        // Read floats from local buffer
                        let read_f32 = |idx: usize| -> f32 {
                            let offset = bird_base + idx * 4;
                            f32::from_le_bytes([
                                scratch.input_data[offset],
                                scratch.input_data[offset + 1],
                                scratch.input_data[offset + 2],
                                scratch.input_data[offset + 3],
                            ])
                        };

                        let pos = Vector3::new(
                            read_f32(0) as f64,
                            read_f32(1) as f64,
                            read_f32(2) as f64,
                        );
                        let heading = read_f32(6) as f64;
                        let bank = read_f32(7) as f64;
                        let flap = read_f32(11) as f64;
                        let tail_yaw = read_f32(13) as f64;

                        let bird_matrix = Matrix4::new_translation(&pos)
                            * Rotation3::from_euler_angles(0.0, heading, bank).to_homogeneous();

                        let scratch_view = &mut scratch.output_data;

                        // Write matrix to local buffer
                        let mut write_mat = |mat: &Matrix4<f64>, part_idx: usize| {
                            let write_off = (part_idx * count * 64) + (i * 64);
                            for (m_idx, &val) in mat.iter().enumerate() {
                                let bytes = (val as f32).to_le_bytes();
                                let dest = write_off + m_idx * 4;
                                scratch_view[dest..dest + 4].copy_from_slice(&bytes);
                            }
                        };

                        // 0. Body
                        let body_mat = bird_matrix
                            * Rotation3::from_euler_angles(std::f64::consts::FRAC_PI_2, 0.0, 0.0)
                                .to_homogeneous();
                        write_mat(&body_mat, 0);

                        // 1. Head
                        let head_mat =
                            bird_matrix * Matrix4::new_translation(&Vector3::new(0.0, 0.0, 0.18));
                        write_mat(&head_mat, 1);

                        // 2. Beak
                        let beak_mat = bird_matrix
                            * Matrix4::new_translation(&Vector3::new(0.0, 0.0, 0.26))
                            * Rotation3::from_euler_angles(std::f64::consts::FRAC_PI_2, 0.0, 0.0)
                                .to_homogeneous();
                        write_mat(&beak_mat, 2);

                        // 3. Left Wing
                        let lw_p1 = Matrix4::new_translation(&Vector3::new(-0.04, 0.0, 0.05))
                            * Rotation3::from_euler_angles(0.0, 0.0, flap).to_homogeneous();
                        let lw_p2 = Matrix4::new_translation(&Vector3::new(-0.15, 0.0, 0.0));
                        let lw_m2 = bird_matrix * lw_p1;
                        let lw_mat = lw_m2 * lw_p2;
                        write_mat(&lw_mat, 3);

                        // 4. Left Wing Tip
                        let lwt_p3 = Matrix4::new_translation(&Vector3::new(-0.3, 0.0, 0.0))
                            * Rotation3::from_euler_angles(0.0, 0.0, flap * 0.5).to_homogeneous();
                        let lwt_p4 = Matrix4::new_translation(&Vector3::new(-0.12, 0.0, -0.05));
                        let lwt_mat = lw_m2 * lwt_p3 * lwt_p4;
                        write_mat(&lwt_mat, 4);

                        // 5. Right Wing
                        let rw_p1 = Matrix4::new_translation(&Vector3::new(0.04, 0.0, 0.05))
                            * Rotation3::from_euler_angles(0.0, 0.0, -flap).to_homogeneous();
                        let rw_p2 = Matrix4::new_translation(&Vector3::new(0.15, 0.0, 0.0));
                        let rw_m2 = bird_matrix * rw_p1;
                        let rw_mat = rw_m2 * rw_p2;
                        write_mat(&rw_mat, 5);

                        // 6. Right Wing Tip
                        let rwt_p3 = Matrix4::new_translation(&Vector3::new(0.3, 0.0, 0.0))
                            * Rotation3::from_euler_angles(0.0, 0.0, -flap * 0.5).to_homogeneous();
                        let rwt_p4 = Matrix4::new_translation(&Vector3::new(0.12, 0.0, -0.05));
                        let rwt_mat = rw_m2 * rwt_p3 * rwt_p4;
                        write_mat(&rwt_mat, 6);

                        // 7. Tail
                        let tail_p1 = Matrix4::new_translation(&Vector3::new(0.0, 0.0, -0.15))
                            * Rotation3::from_euler_angles(0.0, tail_yaw, 0.0).to_homogeneous();
                        let tail_p2 = Matrix4::new_translation(&Vector3::new(0.0, 0.0, -0.1));
                        let tail_mat = bird_matrix * tail_p1 * tail_p2;
                        write_mat(&tail_mat, 7);
                    }

                    // Write ALL matrices in one call to the INACTIVE matrix buffer
                    sab.write_raw(matrix_info.offset, &scratch.output_data[..output_size])
                        .map_err(|e| ComputeError::ExecutionFailed(e))?;

                    // FLIP MATRIX EPOCH to signal JS that new matrices are ready
                    let new_matrix_epoch = matrix_ping_pong.flip();

                    Ok(serde_json::to_vec(&serde_json::json!({
                        "status": "matrices_updated",
                        "count": count,
                        "epoch": new_matrix_epoch
                    }))
                    .unwrap())
                } else {
                    self.proxy_response(method, params)
                }
            }

            // Proxy other methods for future implementation
            "matrix_from_rotation"
            | "matrix_from_quaternion"
            | "matrix_compose"
            | "matrix_decompose"
            | "matrix_transpose"
            | "matrix_determinant"
            | "vector_length"
            | "vector_dot"
            | "vector_lerp"
            | "vector_slerp"
            | "vector_add"
            | "vector_subtract"
            | "vector_scale"
            | "vector_distance"
            | "quaternion_identity"
            | "quaternion_from_axis_angle"
            | "quaternion_to_euler"
            | "quaternion_multiply"
            | "quaternion_slerp"
            | "quaternion_conjugate"
            | "quaternion_invert"
            | "transform_point"
            | "transform_direction"
            | "transform_normal"
            | "look_at"
            | "perspective"
            | "orthographic"
            | "batch_transform_points" => self.proxy_response(method, params),

            _ => Err(ComputeError::UnknownMethod {
                library: "math".to_string(),
                method: method.to_string(),
            }),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_matrix_identity() {
        let unit = MathUnit::new();
        let result = unit.execute("matrix_identity", &[], b"{}").await.unwrap();

        let response: JsonValue = serde_json::from_slice(&result).unwrap();
        let matrix = response["matrix"].as_array().unwrap();

        assert_eq!(matrix.len(), 16);
        assert_eq!(matrix[0].as_f64().unwrap(), 1.0); // m[0][0]
        assert_eq!(matrix[5].as_f64().unwrap(), 1.0); // m[1][1]
        assert_eq!(matrix[10].as_f64().unwrap(), 1.0); // m[2][2]
        assert_eq!(matrix[15].as_f64().unwrap(), 1.0); // m[3][3]
    }

    #[tokio::test]
    async fn test_matrix_from_translation() {
        let unit = MathUnit::new();
        let params = serde_json::to_vec(&serde_json::json!({
            "translation": {"x": 10.0, "y": 20.0, "z": 30.0}
        }))
        .unwrap();

        let result = unit
            .execute("matrix_from_translation", &[], &params)
            .await
            .unwrap();

        let response: JsonValue = serde_json::from_slice(&result).unwrap();
        let matrix = response["matrix"].as_array().unwrap();

        // In column-major, translation is at [12, 13, 14]
        assert_eq!(matrix[12].as_f64().unwrap(), 10.0);
        assert_eq!(matrix[13].as_f64().unwrap(), 20.0);
        assert_eq!(matrix[14].as_f64().unwrap(), 30.0);
    }

    #[tokio::test]
    async fn test_matrix_from_euler() {
        let unit = MathUnit::new();
        let params = serde_json::to_vec(&serde_json::json!({
            "euler": {"x": 0.0, "y": std::f64::consts::FRAC_PI_2, "z": 0.0}
        }))
        .unwrap();

        let result = unit
            .execute("matrix_from_euler", &[], &params)
            .await
            .unwrap();

        let response: JsonValue = serde_json::from_slice(&result).unwrap();
        let matrix = response["matrix"].as_array().unwrap();

        // 90° Y rotation should have specific structure
        assert!(matrix.len() == 16);
    }

    #[tokio::test]
    async fn test_matrix_multiply() {
        let unit = MathUnit::new();

        // Identity * Translation = Translation
        let params = serde_json::to_vec(&serde_json::json!({
            "a": [1.0,0.0,0.0,0.0, 0.0,1.0,0.0,0.0, 0.0,0.0,1.0,0.0, 5.0,0.0,0.0,1.0],
            "b": [1.0,0.0,0.0,0.0, 0.0,1.0,0.0,0.0, 0.0,0.0,1.0,0.0, 0.0,10.0,0.0,1.0]
        }))
        .unwrap();

        let result = unit.execute("matrix_multiply", &[], &params).await.unwrap();

        let response: JsonValue = serde_json::from_slice(&result).unwrap();
        let matrix = response["matrix"].as_array().unwrap();

        // Combined translation
        assert_eq!(matrix[12].as_f64().unwrap(), 5.0);
        assert_eq!(matrix[13].as_f64().unwrap(), 10.0);
    }

    #[tokio::test]
    async fn test_matrix_invert() {
        let unit = MathUnit::new();

        // Translation matrix
        let params = serde_json::to_vec(&serde_json::json!({
            "matrix": [1.0,0.0,0.0,0.0, 0.0,1.0,0.0,0.0, 0.0,0.0,1.0,0.0, 5.0,10.0,15.0,1.0]
        }))
        .unwrap();

        let result = unit.execute("matrix_invert", &[], &params).await.unwrap();

        let response: JsonValue = serde_json::from_slice(&result).unwrap();
        assert!(response["invertible"].as_bool().unwrap());

        let matrix = response["matrix"].as_array().unwrap();
        // Inverse translation should be negative
        assert_eq!(matrix[12].as_f64().unwrap(), -5.0);
        assert_eq!(matrix[13].as_f64().unwrap(), -10.0);
        assert_eq!(matrix[14].as_f64().unwrap(), -15.0);
    }

    #[tokio::test]
    async fn test_quaternion_from_euler() {
        let unit = MathUnit::new();
        let params = serde_json::to_vec(&serde_json::json!({
            "euler": {"pitch": 0.0, "yaw": 0.0, "roll": 0.0}
        }))
        .unwrap();

        let result = unit
            .execute("quaternion_from_euler", &[], &params)
            .await
            .unwrap();

        let response: JsonValue = serde_json::from_slice(&result).unwrap();
        let q = &response["quaternion"];

        // Identity quaternion
        assert!((q["w"].as_f64().unwrap() - 1.0).abs() < 1e-6);
        assert!((q["x"].as_f64().unwrap()).abs() < 1e-6);
        assert!((q["y"].as_f64().unwrap()).abs() < 1e-6);
        assert!((q["z"].as_f64().unwrap()).abs() < 1e-6);
    }

    #[tokio::test]
    async fn test_vector_normalize() {
        let unit = MathUnit::new();
        let params = serde_json::to_vec(&serde_json::json!({
            "vector": {"x": 3.0, "y": 0.0, "z": 4.0}
        }))
        .unwrap();

        let result = unit
            .execute("vector_normalize", &[], &params)
            .await
            .unwrap();

        let response: JsonValue = serde_json::from_slice(&result).unwrap();
        let v = &response["vector"];

        // 3-4-5 triangle, normalized to unit length
        assert!((v["x"].as_f64().unwrap() - 0.6).abs() < 1e-6);
        assert!((v["y"].as_f64().unwrap()).abs() < 1e-6);
        assert!((v["z"].as_f64().unwrap() - 0.8).abs() < 1e-6);
    }

    #[tokio::test]
    async fn test_vector_cross() {
        let unit = MathUnit::new();
        let params = serde_json::to_vec(&serde_json::json!({
            "a": {"x": 1.0, "y": 0.0, "z": 0.0},
            "b": {"x": 0.0, "y": 1.0, "z": 0.0}
        }))
        .unwrap();

        let result = unit.execute("vector_cross", &[], &params).await.unwrap();

        let response: JsonValue = serde_json::from_slice(&result).unwrap();
        let v = &response["vector"];

        // X cross Y = Z
        assert!((v["x"].as_f64().unwrap()).abs() < 1e-6);
        assert!((v["y"].as_f64().unwrap()).abs() < 1e-6);
        assert!((v["z"].as_f64().unwrap() - 1.0).abs() < 1e-6);
    }

    #[tokio::test]
    async fn test_actions_list() {
        let unit = MathUnit::new();
        let actions = unit.actions();

        assert!(actions.contains(&"matrix_identity"));
        assert!(actions.contains(&"matrix_multiply"));
        assert!(actions.contains(&"quaternion_from_euler"));
        assert!(actions.contains(&"vector_normalize"));
        assert!(actions.contains(&"compute_instance_matrices"));
        assert!(actions.len() >= 35);
    }

    #[tokio::test]
    async fn test_service_name() {
        let unit = MathUnit::new();
        assert_eq!(unit.service_name(), "math");
    }

    #[tokio::test]
    async fn test_invalid_matrix() {
        let unit = MathUnit::new();
        let params = serde_json::to_vec(&serde_json::json!({
            "matrix": [1.0, 2.0, 3.0]  // Only 3 elements, need 16
        }))
        .unwrap();

        let result = unit.execute("matrix_invert", &[], &params).await;

        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_compute_instance_matrices() {
        let unit = MathUnit::new();
        let sab_size = 10 * 1024 * 1024; // 10MB
        let sab_inner = sdk::sab::SafeSAB::with_size(sab_size);
        crate::set_cached_sab(sab_inner.clone());

        const BYTES_PER_BIRD: usize = 236; // MUST match sdk::layout::BIRD_STRIDE
        let count = 2;
        let source_offset = 0x400000;
        let target_offset = 0x500000;

        // 1. Setup bird data in mocked SAB
        // Bird 0: At origin, facing forward
        // View: [x,y,z, vx,vy,vz, yaw,pitch, ...]
        let mut bird0 = [0.0f32; 14];
        bird0[0] = 0.0;
        bird0[1] = 0.0;
        bird0[2] = 0.0; // pos
        bird0[6] = 0.0;
        bird0[7] = 0.0; // rot
        bird0[11] = 0.5; // flap
        bird0[13] = 0.2; // tail_yaw

        for j in 0..14 {
            let _ = sab_inner.write(source_offset + j * 4, &bird0[j].to_le_bytes());
        }

        // 2. Call compute_instance_matrices
        let params = serde_json::to_vec(&serde_json::json!({
            "count": count,
            "source_offset": source_offset,
            "target_offset": target_offset,
            "pivots": [] // Hardcoded in Rust for boids
        }))
        .unwrap();

        let result = unit
            .execute("compute_instance_matrices", &[], &params)
            .await;
        assert!(result.is_ok(), "Compute matrices should succeed");

        // 3. Verify Body Matrix (Part 0, Bird 0)
        // Body is Base * Rotation(PI/2, 0, 0)
        // Since base is Identity, Body should be Rotation(PI/2, 0, 0)
        let mut mat_bytes = [0u8; 64];
        let bytes = sab_inner.read(target_offset, 64).unwrap();
        mat_bytes.copy_from_slice(&bytes);

        let mut mat = [0.0f32; 16];
        for i in 0..16 {
            mat[i] = f32::from_le_bytes([
                mat_bytes[i * 4],
                mat_bytes[i * 4 + 1],
                mat_bytes[i * 4 + 2],
                mat_bytes[i * 4 + 3],
            ]);
        }

        // Element (1,1) of matrix rotated PI/2 around X should be cos(PI/2) = 0
        assert!(mat[5].abs() < 1e-6);
        // Element (2,1) should be sin(PI/2) = 1 (index 6 is row 2, col 1)
        assert!((mat[6] - 1.0).abs() < 1e-6);
        // Element (1,2) should be -sin(PI/2) = -1 (index 9 is row 1, col 2)
        assert!((mat[9] - (-1.0)).abs() < 1e-6);

        // 4. Verify Head Matrix (Part 1, Bird 0)
        // Head is Base * Translation(0, 0, 0.18)
        let bytes = sab_inner
            .read(target_offset + (1 * count * 64), 64)
            .unwrap();
        for i in 0..16 {
            mat[i] = f32::from_le_bytes([
                bytes[i * 4],
                bytes[i * 4 + 1],
                bytes[i * 4 + 2],
                bytes[i * 4 + 3],
            ]);
        }
        // Translation should be in the last column (index 12, 13, 14, 15 is 4x4 layout in nalgebra)
        // nalgebra is column-major? Wait.
        // Matrix4 in nalgebra: 0,1,2,3 is first column. 12,13,14,15 is last column.
        assert!((mat[14] - 0.18).abs() < 1e-6);
    }

    #[tokio::test]
    async fn test_unknown_method() {
        let unit = MathUnit::new();
        let result = unit.execute("unknown_method", &[], b"{}").await;

        assert!(result.is_err());
        match result.unwrap_err() {
            ComputeError::UnknownMethod { library, method } => {
                assert_eq!(library, "math");
                assert_eq!(method, "unknown_method");
            }
            _ => panic!("Expected UnknownMethod error"),
        }
    }
}
