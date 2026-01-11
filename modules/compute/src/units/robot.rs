use crate::engine::{ComputeError, ResourceLimits, UnitProxy};
use async_trait::async_trait;
use sdk::layout::*;
use std::sync::Mutex;

const NODE_COUNT: usize = 512;
const FILAMENT_COUNT: usize = 1024;
const DT: f32 = 0.016;
const DAMPING: f32 = 0.98;

struct LatticeState {
    pos: Vec<f32>,                       // 512 * 3
    prev_pos: Vec<f32>,                  // 512 * 3
    filaments: Vec<(usize, usize, f32)>, // (index1, index2, rest_length)
    initialized: bool,
    // Pre-allocated buffers to avoid allocation churn
    matrix_buf: Vec<u8>,
    filament_buf: Vec<u8>,
}

/// RobotUnit: Morphic Lattice simulation (Moonshot)
/// Implements Verlet integration and Syntropy logic
pub struct RobotUnit {
    state: Mutex<LatticeState>,
}

impl RobotUnit {
    pub fn new() -> Self {
        Self {
            state: Mutex::new(LatticeState {
                pos: vec![0.0; NODE_COUNT * 3],
                prev_pos: vec![0.0; NODE_COUNT * 3],
                filaments: Vec::with_capacity(FILAMENT_COUNT),
                initialized: false,
                matrix_buf: vec![0u8; NODE_COUNT * 64],
                filament_buf: vec![0u8; FILAMENT_COUNT * 8],
            }),
        }
    }

    fn init_lattice(&self, state: &mut LatticeState) {
        use std::f32::consts::PI;

        // 1. Initialize nodes in a Structured UV Sphere
        // 512 nodes = ~16 rings * 32 segments
        const RINGS: usize = 16;
        const SEGMENTS: usize = 32;

        for i in 0..NODE_COUNT {
            let ring = i / SEGMENTS;
            let segment = i % SEGMENTS;

            // Normalized coordinates (0..1)
            let u = segment as f32 / SEGMENTS as f32;
            let v = ring as f32 / RINGS as f32;

            let theta = u * 2.0 * PI;
            let phi = v * PI;

            let r = 5.0;
            let x = r * phi.sin() * theta.cos();
            let y = r * phi.cos();
            let z = r * phi.sin() * theta.sin();

            state.pos[i * 3] = x;
            state.pos[i * 3 + 1] = y;
            state.pos[i * 3 + 2] = z;
            state.prev_pos[i * 3] = x;
            state.prev_pos[i * 3 + 1] = y;
            state.prev_pos[i * 3 + 2] = z;
        }

        // 2. Initialize filaments (Nearest Neighbor Mesh)
        // Connect to: next in ring, next in segment
        state.filaments.clear();
        for i in 0..NODE_COUNT {
            let ring = i / SEGMENTS;
            let segment = i % SEGMENTS;

            // Connect Right (Horizontal)
            let right_neighbor = ring * SEGMENTS + ((segment + 1) % SEGMENTS);
            if right_neighbor < NODE_COUNT && right_neighbor != i {
                Self::add_filament(state, i, right_neighbor);
            }

            // Connect Down (Vertical)
            if ring < RINGS - 1 {
                let down_neighbor = (ring + 1) * SEGMENTS + segment;
                if down_neighbor < NODE_COUNT {
                    Self::add_filament(state, i, down_neighbor);
                }
            }

            // Connect Diagonal (for shear stability)
            if ring < RINGS - 1 {
                let diag_neighbor = (ring + 1) * SEGMENTS + ((segment + 1) % SEGMENTS);
                if diag_neighbor < NODE_COUNT {
                    Self::add_filament(state, i, diag_neighbor);
                }
            }
        }

        // Fill remaining filaments with random Cross-Links if any slots left (up to 1024)
        // ... (Optional)

        state.initialized = true;
        // info!(
        //     "[robot] Morphic Lattice initialized with {} nodes and {} filaments (Structured Sphere)",
        //     NODE_COUNT,
        //     state.filaments.len()
        // );
    }

    fn add_filament(state: &mut LatticeState, idx1: usize, idx2: usize) {
        if state.filaments.len() >= FILAMENT_COUNT {
            return;
        }

        let dx = state.pos[idx1 * 3] - state.pos[idx2 * 3];
        let dy = state.pos[idx1 * 3 + 1] - state.pos[idx2 * 3 + 1];
        let dz = state.pos[idx1 * 3 + 2] - state.pos[idx2 * 3 + 2];
        let dist = (dx * dx + dy * dy + dz * dz).sqrt();
        state.filaments.push((idx1, idx2, dist));
    }

    fn step_physics(&self) -> Result<(), ComputeError> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| ComputeError::ExecutionFailed("Mutex poisoned".into()))?;
        if !state.initialized {
            self.init_lattice(&mut state);
        }

        let sab = crate::get_cached_sab()
            .ok_or_else(|| ComputeError::ExecutionFailed("SAB not available".into()))?;

        // 1. Read Syntropy Score from SAB
        let mut state_buf = [0u8; 32];
        sab.read_raw(OFFSET_ROBOT_STATE, &mut state_buf)
            .map_err(|e| ComputeError::ExecutionFailed(e))?;
        let phase = u32::from_le_bytes([state_buf[8], state_buf[9], state_buf[10], state_buf[11]]);
        let syntropy =
            f32::from_le_bytes([state_buf[12], state_buf[13], state_buf[14], state_buf[15]]);
        let model_accuracy =
            f32::from_le_bytes([state_buf[16], state_buf[17], state_buf[18], state_buf[19]]);

        // 2. Verlet Integration
        for i in 0..NODE_COUNT {
            let x = state.pos[i * 3];
            let y = state.pos[i * 3 + 1];
            let z = state.pos[i * 3 + 2];

            let px = state.prev_pos[i * 3];
            let py = state.prev_pos[i * 3 + 1];
            let pz = state.prev_pos[i * 3 + 2];

            // Velocity + Damping
            let vx = (x - px) * DAMPING;
            let vy = (y - py) * DAMPING;
            let vz = (z - pz) * DAMPING;

            // Syntropy Force (Attractive force toward Axis of Motion)
            // Axis of Motion: A rotating double helix or torus
            // ML Integration: Model Accuracy tightens the precision of the manifold
            let precision = if model_accuracy > 0.0 {
                model_accuracy
            } else {
                0.8
            };

            let t = (phase as f32 * 0.1) + (i as f32 * 0.02);
            let target_x = 3.0 * t.cos() * syntropy * precision;
            let target_y = 3.0 * t.sin() * syntropy * precision;
            let target_z = 2.0 * (t * 2.0).cos() * syntropy * precision;

            let ax = (target_x - x) * 0.1 * syntropy;
            let ay = (target_y - y) * 0.1 * syntropy;
            let az = (target_z - z) * 0.1 * syntropy;

            state.prev_pos[i * 3] = x;
            state.prev_pos[i * 3 + 1] = y;
            state.prev_pos[i * 3 + 2] = z;

            state.pos[i * 3] = x + vx + ax * DT * DT;
            state.pos[i * 3 + 1] = y + vy + ay * DT * DT;
            state.pos[i * 3 + 2] = z + vz + az * DT * DT;
        }

        // 3. Constrain Filaments (Springs)
        for _ in 0..2 {
            // 2 iterations for stability
            for i in 0..state.filaments.len() {
                let (idx1, idx2, rest_len) = state.filaments[i];
                let dx = state.pos[idx1 * 3] - state.pos[idx2 * 3];
                let dy = state.pos[idx1 * 3 + 1] - state.pos[idx2 * 3 + 1];
                let dz = state.pos[idx1 * 3 + 2] - state.pos[idx2 * 3 + 2];
                let dist = (dx * dx + dy * dy + dz * dz).sqrt();
                if dist < 0.0001 {
                    continue;
                }

                let diff = (rest_len - dist) / dist;
                let offset_x = dx * 0.5 * diff;
                let offset_y = dy * 0.5 * diff;
                let offset_z = dz * 0.5 * diff;

                state.pos[idx1 * 3] += offset_x;
                state.pos[idx1 * 3 + 1] += offset_y;
                state.pos[idx1 * 3 + 2] += offset_z;

                state.pos[idx2 * 3] -= offset_x;
                state.pos[idx2 * 3 + 1] -= offset_y;
                state.pos[idx2 * 3 + 2] -= offset_z;
            }
        }

        // 4. Write Matrices to SAB (Zero-Copy Visualization)
        // Stride: 64 bytes (16 floats)
        // Use reuse buffer with mem::take to avoid borrow conflicts
        let mut matrix_buf = std::mem::take(&mut state.matrix_buf);

        for i in 0..NODE_COUNT {
            let x = state.pos[i * 3];
            let y = state.pos[i * 3 + 1];
            let z = state.pos[i * 3 + 2];

            // Identity matrix with position
            let mut matrix = [0.0f32; 16];
            matrix[0] = 1.0;
            matrix[5] = 1.0;
            matrix[10] = 1.0;
            matrix[15] = 1.0;
            matrix[12] = x;
            matrix[13] = y;
            matrix[14] = z;

            // Simple scale reduction as nodes organize
            let scale = 1.0 - (syntropy * 0.5);
            matrix[0] *= scale;
            matrix[5] *= scale;
            matrix[10] *= scale;

            for j in 0..16 {
                let bytes = matrix[j].to_le_bytes();
                matrix_buf[(i * 64) + (j * 4)..(i * 64) + (j * 4) + 4].copy_from_slice(&bytes);
            }
        }
        sab.write_raw(OFFSET_ROBOT_NODES, &matrix_buf)
            .map_err(|e| ComputeError::ExecutionFailed(e))?;

        // Restore buffer
        state.matrix_buf = matrix_buf;

        // 5. Write Filaments to SAB
        // Use reuse buffer with mem::take
        let mut filament_buf = std::mem::take(&mut state.filament_buf);

        // Only update if filaments changed (Currently they don't after init, but good practice)
        // Optimization: Check if we need to rewrite? For now, just write.
        for (i, &(idx1, idx2, _)) in state.filaments.iter().enumerate() {
            filament_buf[i * 8..i * 8 + 4].copy_from_slice(&(idx1 as u32).to_le_bytes());
            filament_buf[i * 8 + 4..i * 8 + 8].copy_from_slice(&(idx2 as u32).to_le_bytes());
        }
        sab.write_raw(OFFSET_ROBOT_FILAMENTS, &filament_buf)
            .map_err(|e| ComputeError::ExecutionFailed(e))?;

        // Restore buffer
        state.filament_buf = filament_buf;

        // 6. Signal Epoch (Notify Frontend)
        // We use the Atomic Flags region for wait-free signaling
        let epoch_offset = OFFSET_ATOMIC_FLAGS + (IDX_ROBOT_EPOCH as usize * 4);
        let mut epoch_buf = [0u8; 4];
        sab.read_raw(epoch_offset, &mut epoch_buf).ok(); // Best effort read
        let current_epoch = u32::from_le_bytes(epoch_buf);
        let new_epoch = current_epoch.wrapping_add(1);
        sab.write_raw(epoch_offset, &new_epoch.to_le_bytes())
            .map_err(|e| ComputeError::ExecutionFailed(e))?;

        Ok(())
    }
}

#[async_trait]
impl UnitProxy for RobotUnit {
    fn service_name(&self) -> &str {
        "robot"
    }

    async fn execute(
        &self,
        method: &str,
        _input: &[u8],
        _params: &[u8],
    ) -> Result<Vec<u8>, ComputeError> {
        match method {
            "step_physics" => {
                self.step_physics()?;
                Ok(vec![1]) // success signal
            }
            "get_telemetry" => {
                // The frontend reads directly from SAB, but we return status
                Ok(vec![1])
            }
            _ => Err(ComputeError::UnknownMethod {
                library: "robot".to_string(),
                method: method.to_string(),
            }),
        }
    }

    fn actions(&self) -> Vec<&str> {
        vec!["step_physics", "get_telemetry"]
    }

    fn resource_limits(&self) -> ResourceLimits {
        ResourceLimits {
            max_input_size: 1024,
            max_output_size: 65536,
            max_memory_pages: 512,
            timeout_ms: 100,
            max_fuel: 50_000_000,
        }
    }
}
