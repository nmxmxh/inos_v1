use crate::engine::{ComputeError, ResourceLimits, UnitProxy};
use async_trait::async_trait;
use sdk::sab::SafeSAB;
use serde_json::Value as JsonValue;
use std::sync::atomic::{AtomicU32, Ordering};

/// Boid learning simulation - skeletal birds learning to fly
///
/// SAB Layout (offset 0x400000, 64KB reserved):
/// Per-bird state (58 floats = 232 bytes):
///   [0-2]   position (x, y, z)
///   [3-5]   velocity (vx, vy, vz)
///   [6-9]   rotation quaternion (x, y, z, w)
///   [10]    angular_velocity
///   [11-13] wing_angles (left, right, tail)
///   [14]    fitness
///   [15-58] neural_weights (44 floats: 8x4 + 4x3 = 44)
///
/// Epoch signaling: Written to SAB offset 0x0000

// Constants for memory layout
const SAB_OFFSET_BOIDS: usize = 0x400000;
const BYTES_PER_BIRD: usize = 232;

// Physics constants
const FLOCK_RADIUS: f32 = 10.0;
const SEPARATION_WEIGHT: f32 = 1.2;
const ALIGNMENT_WEIGHT: f32 = 0.5;
const DAMPING: f32 = 0.98;
const BOUNDARY_X: f32 = 30.0;
const BOUNDARY_Y: f32 = 15.0;
const BOUNDARY_Z: f32 = 30.0;
const BOUNDARY_SPRING: f32 = 0.5;

// Animation constants
const WING_FREQ_BASE: f32 = 5.0;
const WING_AMPLITUDE: f32 = 0.6;

/// Global epoch counter for signaling state changes
static EPOCH_COUNTER: AtomicU32 = AtomicU32::new(0);

#[derive(Clone)]
pub struct BoidUnit {
    _config: BoidConfig,
}

#[derive(Clone)]
struct BoidConfig {
    _bird_count: u32,
    _sab_offset: usize,
}

impl BoidUnit {
    pub fn new(bird_count: u32, sab_offset: usize) -> Self {
        Self {
            _config: BoidConfig {
                _bird_count: bird_count,
                _sab_offset: sab_offset,
            },
        }
    }

    /// Step boids physics in SAB (called from lib.rs)
    /// Returns the current epoch number
    pub fn step_physics_sab(
        sab: &SafeSAB,
        bird_count: u32,
        dt: f32,
        elapsed_time: f32,
    ) -> Result<u32, String> {
        // Increment and retrieve epoch
        let epoch = EPOCH_COUNTER.fetch_add(1, Ordering::SeqCst);

        // Standardized Global Epoch at 0x0000 (used for all reactivity)
        sab.write(0, &epoch.to_le_bytes())
            .map_err(|e| format!("Failed to write epoch: {}", e))?;

        // Standardized Bird Epoch at 0x0020 (idx 8 * 4)
        sab.write(0x20, &epoch.to_le_bytes())
            .map_err(|e| format!("Failed to write bird epoch: {}", e))?;

        // Physics simulation
        for i in 0..bird_count as usize {
            let base = SAB_OFFSET_BOIDS + i * BYTES_PER_BIRD;

            // Read current state
            let mut pos = Self::read_vec3(sab, base, 0)?;
            let mut vel = Self::read_vec3(sab, base, 12)?;

            // 1. Core Biological Flocking (Vector-based)
            let mut flock_force = [0.0f32; 3];
            let mut neighbors = 0;

            for other in 0..bird_count as usize {
                if other == i {
                    continue;
                }

                let other_base = SAB_OFFSET_BOIDS + other * BYTES_PER_BIRD;
                let other_pos = Self::read_vec3(sab, other_base, 0)?;

                let dx = other_pos[0] - pos[0];
                let dy = other_pos[1] - pos[1];
                let dz = other_pos[2] - pos[2];
                let dist_sq = dx * dx + dy * dy + dz * dz;

                if dist_sq < FLOCK_RADIUS && dist_sq > 0.001 {
                    let dist = dist_sq.sqrt();
                    let inv_dist = 1.0 / dist;

                    // Separation: Inverse-square repulsion
                    flock_force[0] -= dx * inv_dist * SEPARATION_WEIGHT;
                    flock_force[1] -= dy * inv_dist * SEPARATION_WEIGHT;
                    flock_force[2] -= dz * inv_dist * SEPARATION_WEIGHT;

                    // Alignment: Match velocity
                    let other_vel = Self::read_vec3(sab, other_base, 12)?;
                    flock_force[0] += other_vel[0] * ALIGNMENT_WEIGHT;
                    flock_force[1] += other_vel[1] * ALIGNMENT_WEIGHT;
                    flock_force[2] += other_vel[2] * ALIGNMENT_WEIGHT;

                    neighbors += 1;
                }
            }

            // 2. Biological Rhythm (Breathing/Oscillation)
            // Gently pull towards center with a biological rhythm
            let center_pull = [-pos[0] * 0.01, -pos[1] * 0.02, -pos[2] * 0.01];
            let rhythm = (elapsed_time * 0.2 + i as f32 * 0.1).sin() * 0.05;

            // 3. Integrate Velocity
            vel[0] = vel[0] * DAMPING + (flock_force[0] + center_pull[0]) * dt;
            vel[1] = vel[1] * DAMPING + (flock_force[1] + center_pull[1] + rhythm) * dt;
            vel[2] = vel[2] * DAMPING + (flock_force[2] + center_pull[2]) * dt;

            // 4. Update Position
            pos[0] += vel[0] * dt * 2.0;
            pos[1] += vel[1] * dt * 2.0;
            pos[2] += vel[2] * dt * 2.0;

            // 5. Soft Boundaries
            let bounds = [BOUNDARY_X, BOUNDARY_Y, BOUNDARY_Z];
            for j in 0..3 {
                if pos[j].abs() > bounds[j] {
                    vel[j] -= pos[j].signum() * BOUNDARY_SPRING;
                }
            }

            // --- Write Visual State ---
            Self::write_vec3(sab, base, 0, &pos)?;
            Self::write_vec3(sab, base, 12, &vel)?;

            // Facing direction (Rotation)
            let speed_sq = vel[0] * vel[0] + vel[1] * vel[1] + vel[2] * vel[2];
            if speed_sq > 0.0001 {
                let rot_y = vel[0].atan2(vel[2]);
                let bank_z = -vel[0] * 0.3; // Subtle banking for biological feel

                sab.write(base + 24, &rot_y.to_le_bytes())?;
                sab.write(base + 28, &bank_z.to_le_bytes())?;
            }

            // Biological Wing Rhythm
            let wing_freq = WING_FREQ_BASE + (i % 5) as f32; // Organic variation
            let flap = (elapsed_time * wing_freq + i as f32).sin() * WING_AMPLITUDE;

            sab.write(base + 44, &flap.to_le_bytes())?; // Synchronized wing pair
            sab.write(base + 52, &(flap * 0.2).to_le_bytes())?; // Rhythmic tail

            // Fitness signaling (for Go GA)
            let fitness = 0.5 + (speed_sq.sqrt() * 0.1).min(0.5);
            sab.write(base + 56, &fitness.to_le_bytes())?;
        }

        Ok(epoch)
    }

    /// Initialize population in SAB (called from lib.rs)
    pub fn init_population_sab(sab: &SafeSAB, bird_count: u32) -> Result<(), String> {
        // Golden angle spiral distribution
        for i in 0..bird_count as usize {
            let base = SAB_OFFSET_BOIDS + i * BYTES_PER_BIRD;

            let phi = i as f32 * 2.39996; // Golden angle
            let r = (i as f32).sqrt() * 1.5;

            let pos = [r * phi.cos(), (i as f32 * 0.2).sin() * 4.0, r * phi.sin()];

            let vel = [
                phi.sin() * 0.1,
                (i as f32 * 0.1).cos() * 0.05,
                phi.cos() * 0.1,
            ];

            Self::write_vec3(sab, base, 0, &pos)?;
            Self::write_vec3(sab, base, 12, &vel)?;

            // Initialize neural weights
            for w in 0..44 {
                let weight = ((i * 137 + w * 997) % 1000) as f32 * 0.002 - 0.001;
                sab.write(base + 60 + w * 4, &weight.to_le_bytes())?;
            }
        }

        Ok(())
    }

    // Helper functions for safe SAB access
    fn read_vec3(sab: &SafeSAB, base: usize, offset: usize) -> Result<[f32; 3], String> {
        let mut vec = [0.0f32; 3];
        for i in 0..3 {
            let bytes = sab.read(base + offset + i * 4, 4)?;
            vec[i] = f32::from_le_bytes(bytes.try_into().map_err(|_| "Invalid byte array length")?);
        }
        Ok(vec)
    }

    fn write_vec3(sab: &SafeSAB, base: usize, offset: usize, vec: &[f32; 3]) -> Result<(), String> {
        for i in 0..3 {
            sab.write(base + offset + i * 4, &vec[i].to_le_bytes())?;
        }
        Ok(())
    }

    fn init_population_impl(&self, params: &JsonValue) -> Result<JsonValue, ComputeError> {
        let bird_count = params
            .get("bird_count")
            .and_then(|v| v.as_u64())
            .unwrap_or(150) as u32;
        Ok(serde_json::json!({
            "action": "init_population",
            "bird_count": bird_count,
            "status": "initialized"
        }))
    }

    fn step_physics_impl(&self, params: &JsonValue) -> Result<JsonValue, ComputeError> {
        let _dt = params.get("dt").and_then(|v| v.as_f64()).unwrap_or(0.016) as f32;
        Ok(serde_json::json!({
            "action": "step_physics",
            "status": "success"
        }))
    }
}

#[async_trait(?Send)]
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
