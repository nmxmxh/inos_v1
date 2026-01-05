use crate::engine::{ComputeError, ResourceLimits, UnitProxy};
use async_trait::async_trait;
use sdk::sab::SafeSAB;
use serde_json::Value as JsonValue;
use std::sync::atomic::{AtomicU32, Ordering};

/// Boid learning simulation - skeletal birds learning to fly
///
/// SAB Layout (offset 0x400000, 64KB reserved):
/// Per-bird state (59 floats = 236 bytes):
///   [0-2]   position (x, y, z)
///   [3-5]   velocity (vx, vy, vz)
///   [6-9]   rotation quaternion (x, y, z, w)
///   [10]    angular_velocity
///   [11-13] wing_angles (left, right, tail)
///   [14]    fitness
///   [15-58] neural_weights (44 floats: 8x4 + 4x3 = 44)
///
/// Epoch signaling: Written to SAB offset 0x0000

/// Global epoch counter for signaling state changes
static EPOCH_COUNTER: AtomicU32 = AtomicU32::new(0);

#[derive(Clone)]
pub struct BoidUnit {
    _config: BoidConfig,
}

#[derive(Clone)]
struct BoidConfig {
    _max_birds: u32,
    _bird_count: u32,
    _learning_rate: f32,
    _mutation_rate: f32,
    _bird_offset: usize,
}

impl Default for BoidConfig {
    fn default() -> Self {
        Self {
            _max_birds: 10000,
            _bird_count: 1000,
            _learning_rate: 0.01,
            _mutation_rate: 0.1,
            _bird_offset: 0x400000, // 4MB offset
        }
    }
}

impl BoidUnit {
    pub fn new() -> Self {
        Self {
            _config: BoidConfig::default(),
        }
    }

    /// Step boids physics in SAB (called from lib.rs)
    /// Returns the current epoch number
    /// Step boids physics in SAB (called from lib.rs)
    /// Returns the current epoch number
    pub fn step_physics_sab(sab: &SafeSAB, bird_count: u32, dt: f32) -> Result<u32, String> {
        const OFFSET: usize = 0x400000;
        const STRIDE: usize = 236; // 59 floats (15 base + 44 weights)

        // Ensure logging is initialized (idempotent)
        sdk::init_logging();

        // Increment and retrieve epoch
        let epoch = EPOCH_COUNTER.fetch_add(1, Ordering::SeqCst);

        // Standardized Global Epoch at 0x0000
        sab.write(0, &epoch.to_le_bytes())
            .map_err(|e| format!("Failed to write epoch: {}", e))?;

        // Standardized Bird Epoch at 0x0020 (idx 8 * 4)
        sab.write(0x20, &epoch.to_le_bytes())
            .map_err(|e| format!("Failed to write bird epoch: {}", e))?;

        static mut GLOBAL_TIME: f32 = 0.0;
        unsafe {
            GLOBAL_TIME += dt;
        }
        let time = unsafe { GLOBAL_TIME };

        // Diagnostic log every 100 steps
        if epoch % 100 == 0 {
            log::info!(
                "[Boids] Step {} | Count: {} | DT: {:.4}",
                epoch,
                bird_count,
                dt
            );
        }

        // --- BULK IO OPTIMIZATION ---
        // Read the entire population block at once
        let total_bytes = bird_count as usize * STRIDE;
        let mut population_data = vec![0u8; total_bytes];
        sab.read_raw(OFFSET, &mut population_data)?;

        for i in 0..bird_count as usize {
            let base = i * STRIDE;

            // Read position [0-2] and velocity [3-5] from local buffer
            let mut pos = [0.0f32; 3];
            let mut vel = [0.0f32; 3];
            for j in 0..3 {
                let p_idx = base + j * 4;
                let v_idx = base + 12 + j * 4;
                pos[j] = f32::from_le_bytes([
                    population_data[p_idx],
                    population_data[p_idx + 1],
                    population_data[p_idx + 2],
                    population_data[p_idx + 3],
                ]);
                vel[j] = f32::from_le_bytes([
                    population_data[v_idx],
                    population_data[v_idx + 1],
                    population_data[v_idx + 2],
                    population_data[v_idx + 3],
                ]);
            }

            // --- Technical Painting Motion ---
            // Combine flocking logic with some rhythmic, swirling motion
            let noise_x = (time * 0.5 + i as f32 * 0.1).sin() * 0.2;
            let noise_y = (time * 0.8 + i as f32 * 0.1).cos() * 0.1;
            let noise_z = (time * 0.3 + i as f32 * 0.2).sin() * 0.2;

            // Swirl center
            let tx = -pos[2] * 0.3;
            let tz = pos[0] * 0.3;

            // Apply forces
            vel[0] = vel[0] * 0.97 + (tx + noise_x) * 0.03;
            vel[1] = vel[1] * 0.95 + noise_y * 0.05;
            vel[2] = vel[2] * 0.97 + (tz + noise_z) * 0.03;

            // Vertical oscillation for "technical sketch" rhythm
            vel[1] += (time * 1.2 + i as f32 * 0.5).sin() * 0.05;

            // Update Position
            pos[0] += vel[0] * dt;
            pos[1] += vel[1] * dt;
            pos[2] += vel[2] * dt;

            // Soft boundaries (smooth return)
            if pos[0].abs() > 25.0 {
                vel[0] -= pos[0] * 0.01;
            }
            if pos[1].abs() > 12.0 {
                vel[1] -= pos[1] * 0.02;
            }
            if pos[2].abs() > 20.0 {
                vel[2] -= pos[2] * 0.01;
            }

            // Write back Pos and Vel to local buffer
            for j in 0..3 {
                population_data[base + j * 4..base + j * 4 + 4]
                    .copy_from_slice(&pos[j].to_le_bytes());
                population_data[base + 12 + j * 4..base + 16 + j * 4]
                    .copy_from_slice(&vel[j].to_le_bytes());
            }

            // --- Pose Updates ---
            let speed = (vel[0] * vel[0] + vel[1] * vel[1] + vel[2] * vel[2]).sqrt();
            if speed > 0.005 {
                let rot_y = vel[0].atan2(vel[2]);
                let bank_z = -vel[0] * 0.4;
                population_data[base + 24..base + 28].copy_from_slice(&rot_y.to_le_bytes());
                population_data[base + 28..base + 32].copy_from_slice(&bank_z.to_le_bytes());
            }

            // Wing Flapping
            let phase = (i as f32) * 2.1;
            let base_flap = 6.0 + (i % 8) as f32;
            let flap = (time * base_flap + phase).sin() * 0.7;

            population_data[base + 44..base + 48].copy_from_slice(&(-flap).to_le_bytes()); // wing_left
            population_data[base + 48..base + 52].copy_from_slice(&flap.to_le_bytes()); // wing_right
            let tail_angle = (time * 3.0 + phase).cos() * 0.15;
            population_data[base + 52..base + 56].copy_from_slice(&tail_angle.to_le_bytes()); // tail_angle

            // Fitness signaling (for Go GA)
            let fitness = 0.5 + (speed * 0.1).min(0.5);
            population_data[base + 56..base + 60].copy_from_slice(&fitness.to_le_bytes());
        }

        // Write the entire population block back in one go
        sab.write_raw(OFFSET, &population_data)?;

        Ok(epoch)
    }

    /// Initialize population in SAB (called from lib.rs)
    pub fn init_population_sab(sab: &SafeSAB, bird_count: u32) -> Result<(), String> {
        const OFFSET: usize = 0x400000;
        const STRIDE: usize = 236; // 59 floats

        sdk::init_logging();
        log::info!(
            "[Boids] Initializing population: {} birds at 0x{:X}",
            bird_count,
            OFFSET
        );

        let total_bytes = bird_count as usize * STRIDE;
        let mut population_data = vec![0u8; total_bytes];

        for i in 0..bird_count as usize {
            let base = i * STRIDE;
            // Use golden ratio or spiral for initial distribution (painterly)
            let r = (i as f32).sqrt() * 1.5;
            let theta = i as f32 * 2.39996; // Golden angle

            let pos = [
                r * theta.cos(),
                (i as f32 * 0.1).sin() * 3.0,
                r * theta.sin(),
            ];
            let vel: [f32; 3] = [0.1, 0.0, 0.1];

            // Position
            for j in 0..3 {
                population_data[base + j * 4..base + j * 4 + 4]
                    .copy_from_slice(&pos[j].to_le_bytes());
            }
            // Velocity
            for j in 0..3 {
                population_data[base + 12 + j * 4..base + 12 + j * 4 + 4]
                    .copy_from_slice(&vel[j].to_le_bytes());
            }

            // Initialize neural weights (offsets 60-232)
            for w in 0..44 {
                let weight = ((i * 137 + w * 997) % 1000) as f32 * 0.002 - 0.001;
                population_data[base + 60 + w * 4..base + 60 + w * 4 + 4]
                    .copy_from_slice(&weight.to_le_bytes());
            }
        }

        // Write the entire population block back in one go
        sab.write_raw(OFFSET, &population_data)
            .map_err(|e| format!("SAB write failed: {}", e))?;

        log::info!("[Boids] Population initialization complete");
        Ok(())
    }

    fn init_population_impl(&self, params: &JsonValue) -> Result<JsonValue, ComputeError> {
        let bird_count = params
            .get("bird_count")
            .and_then(|v| v.as_u64())
            .unwrap_or(1000) as u32;

        let sab = crate::get_cached_sab().ok_or_else(|| {
            ComputeError::ExecutionFailed("SAB not available for initialization".to_string())
        })?;

        Self::init_population_sab(&sab, bird_count)
            .map_err(|e| ComputeError::ExecutionFailed(format!("Failed to init boids: {}", e)))?;

        Ok(serde_json::json!({
            "action": "init_population",
            "bird_count": bird_count,
            "status": "initialized"
        }))
    }

    fn step_physics_impl(&self, params: &JsonValue) -> Result<JsonValue, ComputeError> {
        let bird_count = params
            .get("bird_count")
            .and_then(|v| v.as_u64())
            .unwrap_or(1000) as u32;
        let dt = params.get("dt").and_then(|v| v.as_f64()).unwrap_or(0.016) as f32;

        let sab = crate::get_cached_sab().ok_or_else(|| {
            ComputeError::ExecutionFailed("SAB not available for physics step".to_string())
        })?;

        let epoch = Self::step_physics_sab(&sab, bird_count, dt)
            .map_err(|e| ComputeError::ExecutionFailed(format!("Boids step failed: {}", e)))?;

        Ok(serde_json::json!({
            "action": "step_physics",
            "epoch": epoch,
            "status": "success"
        }))
    }
}

#[async_trait]
impl UnitProxy for BoidUnit {
    fn service_name(&self) -> &str {
        "boids"
    }
    fn actions(&self) -> Vec<&str> {
        vec!["init_population", "step_physics"]
    }
    fn resource_limits(&self) -> ResourceLimits {
        ResourceLimits::default()
    }
    async fn execute(
        &self,
        method: &str,
        _input: &[u8],
        params: &[u8],
    ) -> Result<Vec<u8>, ComputeError> {
        let params: JsonValue = serde_json::from_slice(params).unwrap_or(JsonValue::Null);
        let res = match method {
            "init_population" => self.init_population_impl(&params)?,
            "step_physics" => self.step_physics_impl(&params)?,
            _ => {
                return Err(ComputeError::UnknownMethod {
                    library: "boids".to_string(),
                    method: method.to_string(),
                })
            }
        };
        Ok(serde_json::to_vec(&res).unwrap())
    }
}
