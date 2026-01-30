//! Drone physics unit for AI Grand Prix racing simulation
#![allow(dead_code)]
//!
//! Follows the same proven patterns as boids.rs:
//! - Ping-pong buffers for zero contention
//! - Epoch signaling for sync
//! - Batched updates for efficiency

use crate::engine::{ComputeError, ResourceLimits, UnitProxy};
use async_trait::async_trait;
use sdk::pingpong::PingPongBuffer;
use sdk::sab::SafeSAB;
use serde_json::Value as JsonValue;
use std::sync::atomic::{AtomicU32, Ordering};
use std::sync::Mutex;

// ============= CONSTANTS =============

#[allow(dead_code)]

/// Maximum drones supported
const MAX_DRONES: usize = 32;

/// Bytes per drone state (128 bytes, aligned)
const DRONE_STRIDE: usize = 128;

/// Epoch index for drone physics (from grandprix epochs 48-55)
const IDX_DRONE_PHYSICS: usize = 50;

// Memory offsets (Must match frontend/src/racing/layout.ts)
const OFFSET_DRONE_STATE_A: usize = 0x160200;
const OFFSET_DRONE_STATE_B: usize = 0x161200;
const SIZE_DRONE_BUFFER: usize = 4096; // 32 * 128 bytes

// Physics constants (Neros 8-inch profile)
const MASS: f32 = 1.2; // kg
const GRAVITY: f32 = 9.81; // m/s²
const MAX_THRUST: f32 = 40.0; // N (peak motor thrust)
const DRAG_COEFF: f32 = 0.42;
const ARM_LENGTH: f32 = 0.1; // m (200mm diagonal / 2)

// Inertia tensor (simplified)
const IXX: f32 = 0.0075; // kg·m²
const IYY: f32 = 0.0075;
const IZZ: f32 = 0.013;

/// Global epoch counter
static EPOCH_COUNTER: AtomicU32 = AtomicU32::new(0);

// ============= DRONE STATE LAYOUT =============
// 128 bytes per drone, aligned for SIMD

/// Drone state offsets (in f32 indices)
mod offsets {
    pub const POSITION: usize = 0; // [0-3] xyz + pad
    pub const VELOCITY: usize = 4; // [4-7] xyz + pad
    pub const ORIENTATION: usize = 8; // [8-11] quaternion wxyz
    pub const ANGULAR_VEL: usize = 12; // [12-15] xyz + pad
    pub const MOTOR_RPM: usize = 16; // [16-19] 4 motors
    pub const CONTROL: usize = 20; // [20-23] thr/pitch/roll/yaw
    pub const RACE_STATE: usize = 24; // [24-27] lap/gate/time/flags
    pub const RESERVED: usize = 28; // [28-31] future use
}

// ============= SCRATCH BUFFERS =============

#[derive(Default)]
struct DroneScratch {
    state_data: Vec<f32>,
    last_count: u32,
}

// ============= DRONE UNIT =============

pub struct DroneUnit {
    scratch: Mutex<DroneScratch>,
}

impl DroneUnit {
    pub fn new() -> Self {
        Self {
            scratch: Mutex::new(DroneScratch::default()),
        }
    }

    /// Step all drones in a single batched update
    pub fn step_all_drones(&self, sab: &SafeSAB, count: u32, dt: f32) -> Result<u32, String> {
        sdk::init_logging();

        let mut scratch = self.scratch.lock().map_err(|_| "Failed to lock scratch")?;

        let n = (count as usize).min(MAX_DRONES);
        let total_floats = n * (DRONE_STRIDE / 4);

        // Resize scratch if needed
        if scratch.state_data.len() < total_floats {
            scratch.state_data.resize(total_floats, 0.0);
            scratch.last_count = count;
        }

        // Create ping-pong accessor (use custom offsets to avoid bird buffer collision)
        let ping_pong = PingPongBuffer::custom(
            sab.clone(),
            OFFSET_DRONE_STATE_A,
            OFFSET_DRONE_STATE_B,
            SIZE_DRONE_BUFFER,
            DRONE_STRIDE,
            IDX_DRONE_PHYSICS as u32,
        );
        let read_info = ping_pong.read_buffer_info();
        let write_info = ping_pong.write_buffer_info();

        let epoch = EPOCH_COUNTER.fetch_add(1, Ordering::SeqCst);

        // Read from SAB
        let byte_slice = unsafe {
            std::slice::from_raw_parts_mut(
                scratch.state_data.as_mut_ptr() as *mut u8,
                total_floats * 4,
            )
        };
        sab.read_raw(read_info.offset, byte_slice)?;

        // Process each drone
        for i in 0..n {
            let base = i * (DRONE_STRIDE / 4);

            // Read state
            let mut pos = [
                scratch.state_data[base + offsets::POSITION],
                scratch.state_data[base + offsets::POSITION + 1],
                scratch.state_data[base + offsets::POSITION + 2],
            ];
            let mut vel = [
                scratch.state_data[base + offsets::VELOCITY],
                scratch.state_data[base + offsets::VELOCITY + 1],
                scratch.state_data[base + offsets::VELOCITY + 2],
            ];
            let mut quat = [
                scratch.state_data[base + offsets::ORIENTATION], // w
                scratch.state_data[base + offsets::ORIENTATION + 1], // x
                scratch.state_data[base + offsets::ORIENTATION + 2], // y
                scratch.state_data[base + offsets::ORIENTATION + 3], // z
            ];
            let mut ang_vel = [
                scratch.state_data[base + offsets::ANGULAR_VEL],
                scratch.state_data[base + offsets::ANGULAR_VEL + 1],
                scratch.state_data[base + offsets::ANGULAR_VEL + 2],
            ];

            // Read control input
            let throttle = scratch.state_data[base + offsets::CONTROL];
            let pitch_cmd = scratch.state_data[base + offsets::CONTROL + 1];
            let roll_cmd = scratch.state_data[base + offsets::CONTROL + 2];
            let yaw_cmd = scratch.state_data[base + offsets::CONTROL + 3];

            // Motor model: throttle → thrust
            let thrust = throttle * MAX_THRUST;

            // Rotate thrust to world frame
            let thrust_body = [0.0, thrust, 0.0];
            let thrust_world = quat_rotate(&quat, &thrust_body);

            // Forces
            let gravity_force = [0.0, -GRAVITY * MASS, 0.0];
            let drag = [
                -DRAG_COEFF * vel[0] * vel[0].abs(),
                -DRAG_COEFF * vel[1] * vel[1].abs(),
                -DRAG_COEFF * vel[2] * vel[2].abs(),
            ];

            // Acceleration
            let accel = [
                (thrust_world[0] + gravity_force[0] + drag[0]) / MASS,
                (thrust_world[1] + gravity_force[1] + drag[1]) / MASS,
                (thrust_world[2] + gravity_force[2] + drag[2]) / MASS,
            ];

            // Integrate velocity
            vel[0] += accel[0] * dt;
            vel[1] += accel[1] * dt;
            vel[2] += accel[2] * dt;

            // Integrate position
            pos[0] += vel[0] * dt;
            pos[1] += vel[1] * dt;
            pos[2] += vel[2] * dt;

            // Ground collision
            if pos[1] < 0.0 {
                pos[1] = 0.0;
                vel[1] = 0.0;
            }

            // Angular dynamics (simplified rate control)
            let target_rates = [
                roll_cmd * 5.0, // rad/s
                pitch_cmd * 5.0,
                yaw_cmd * 2.0,
            ];

            // PD control to target rates
            ang_vel[0] += (target_rates[0] - ang_vel[0]) * dt * 10.0;
            ang_vel[1] += (target_rates[1] - ang_vel[1]) * dt * 10.0;
            ang_vel[2] += (target_rates[2] - ang_vel[2]) * dt * 10.0;

            // Integrate quaternion
            integrate_quaternion(&mut quat, &ang_vel, dt);

            // Write back to scratch
            scratch.state_data[base + offsets::POSITION] = pos[0];
            scratch.state_data[base + offsets::POSITION + 1] = pos[1];
            scratch.state_data[base + offsets::POSITION + 2] = pos[2];

            scratch.state_data[base + offsets::VELOCITY] = vel[0];
            scratch.state_data[base + offsets::VELOCITY + 1] = vel[1];
            scratch.state_data[base + offsets::VELOCITY + 2] = vel[2];

            scratch.state_data[base + offsets::ORIENTATION] = quat[0];
            scratch.state_data[base + offsets::ORIENTATION + 1] = quat[1];
            scratch.state_data[base + offsets::ORIENTATION + 2] = quat[2];
            scratch.state_data[base + offsets::ORIENTATION + 3] = quat[3];

            scratch.state_data[base + offsets::ANGULAR_VEL] = ang_vel[0];
            scratch.state_data[base + offsets::ANGULAR_VEL + 1] = ang_vel[1];
            scratch.state_data[base + offsets::ANGULAR_VEL + 2] = ang_vel[2];
        }

        // Write to SAB
        let out_bytes = unsafe {
            std::slice::from_raw_parts(scratch.state_data.as_ptr() as *const u8, total_floats * 4)
        };
        sab.write_raw(write_info.offset, out_bytes)?;

        // Flip buffers and signal epoch
        let new_epoch = ping_pong.flip();

        // Log periodically
        if epoch % 250 == 0 {
            log::info!("[Drone] Step {} | Count: {} | DT: {:.4}", epoch, count, dt);
        }

        Ok(new_epoch as u32)
    }

    /// Initialize drone pool
    pub fn init_drones(sab: &SafeSAB, count: u32) -> Result<(), String> {
        // sdk::init_logging(); // Avoid potential re-init locks
        sdk::js_interop::console_log("[Drone] init_drones start", 1);

        let ping_pong = PingPongBuffer::custom(
            sab.clone(),
            OFFSET_DRONE_STATE_A,
            OFFSET_DRONE_STATE_B,
            SIZE_DRONE_BUFFER,
            DRONE_STRIDE,
            IDX_DRONE_PHYSICS as u32,
        );
        let read_info = ping_pong.read_buffer_info();
        let write_info = ping_pong.write_buffer_info();

        sdk::js_interop::console_log(
            &format!(
                "[Drone] Buffers: Read=0x{:x}, Write=0x{:x}",
                read_info.offset, write_info.offset
            ),
            1,
        );

        let n = (count as usize).min(MAX_DRONES);
        let total_bytes = n * DRONE_STRIDE;
        let mut data = vec![0u8; total_bytes];

        sdk::js_interop::console_log("[Drone] Generating initial state...", 1);

        for i in 0..n {
            let base = i * DRONE_STRIDE;

            // Position: staggered start grid
            let row = i / 4;
            let col = i % 4;
            let pos = [
                (col as f32 - 1.5) * 3.0,  // X spacing
                1.0,                       // Y height
                -(row as f32) * 5.0 - 5.0, // Z spacing
            ];

            // Velocity: zero
            let vel = [0.0f32; 3];

            // Orientation: identity quaternion (w=1)
            let quat = [1.0f32, 0.0, 0.0, 0.0];

            // Write position
            for (j, &v) in pos.iter().enumerate() {
                let offset = base + j * 4;
                data[offset..offset + 4].copy_from_slice(&v.to_le_bytes());
            }

            // Write velocity
            for (j, &v) in vel.iter().enumerate() {
                let offset = base + 12 + j * 4;
                data[offset..offset + 4].copy_from_slice(&v.to_le_bytes());
            }

            // Write orientation
            for (j, &v) in quat.iter().enumerate() {
                let offset = base + 32 + j * 4;
                data[offset..offset + 4].copy_from_slice(&v.to_le_bytes());
            }
        }

        sdk::js_interop::console_log("[Drone] Writing to SAB...", 1);

        // Write to both buffers
        if let Err(e) = sab.write_raw(read_info.offset, &data) {
            let msg = format!("[Drone] Write read_buffer failed: {}", e);
            sdk::js_interop::console_log(&msg, 1);
            return Err(e);
        }

        if let Err(e) = sab.write_raw(write_info.offset, &data) {
            let msg = format!("[Drone] Write write_buffer failed: {}", e);
            sdk::js_interop::console_log(&msg, 1);
        }

        let msg = format!("[Drone] Initialized {} drones", n);
        sdk::js_interop::console_log(&msg, 1);
        // log::info!("{}", msg);
        Ok(())
    }

    fn step_physics_impl(&self, params: &JsonValue) -> Result<JsonValue, ComputeError> {
        let count = params.get("count").and_then(|v| v.as_u64()).unwrap_or(1) as u32;
        let dt = params.get("dt").and_then(|v| v.as_f64()).unwrap_or(0.004) as f32; // 250 Hz default

        let sab = crate::get_cached_sab()
            .ok_or_else(|| ComputeError::ExecutionFailed("SAB not available".to_string()))?;

        let epoch = self
            .step_all_drones(&sab, count, dt)
            .map_err(|e| ComputeError::ExecutionFailed(e))?;

        Ok(serde_json::json!({
            "action": "step_physics",
            "count": count,
            "epoch": epoch
        }))
    }

    fn init_impl(&self, params: &JsonValue) -> Result<JsonValue, ComputeError> {
        sdk::js_interop::console_log("[Drone] init_impl called", 1);
        let count = params.get("count").and_then(|v| v.as_u64()).unwrap_or(8) as u32;

        let sab = crate::get_cached_sab()
            .ok_or_else(|| ComputeError::ExecutionFailed("SAB not available".to_string()))?;

        sdk::js_interop::console_log("[Drone] SAB retrieved", 1);

        Self::init_drones(&sab, count).map_err(|e| ComputeError::ExecutionFailed(e))?;

        Ok(serde_json::json!({
            "action": "init",
            "count": count,
            "status": "initialized"
        }))
    }
}

impl Default for DroneUnit {
    fn default() -> Self {
        Self::new()
    }
}

#[async_trait]
impl UnitProxy for DroneUnit {
    fn service_name(&self) -> &str {
        "drone"
    }

    fn actions(&self) -> Vec<&str> {
        vec!["init", "step_physics"]
    }

    fn resource_limits(&self) -> ResourceLimits {
        ResourceLimits::default()
    }

    async fn execute(
        &self,
        action: &str,
        _input: &[u8],
        params: &[u8],
    ) -> Result<Vec<u8>, ComputeError> {
        let params: JsonValue = if params.is_empty() {
            serde_json::json!({})
        } else {
            serde_json::from_slice(params)
                .map_err(|e| ComputeError::InvalidParams(e.to_string()))?
        };

        let result = match action {
            "init" => self.init_impl(&params)?,
            "step_physics" => self.step_physics_impl(&params)?,
            _ => {
                return Err(ComputeError::UnknownAction {
                    service: "drone".to_string(),
                    action: action.to_string(),
                })
            }
        };

        serde_json::to_vec(&result).map_err(|e| ComputeError::ExecutionFailed(e.to_string()))
    }
}

// ============= MATH HELPERS =============

/// Rotate vector by quaternion
#[inline]
fn quat_rotate(q: &[f32; 4], v: &[f32; 3]) -> [f32; 3] {
    let w = q[0];
    let x = q[1];
    let y = q[2];
    let z = q[3];

    // q * v * q^-1 (simplified for unit quaternion)
    let vx = v[0];
    let vy = v[1];
    let vz = v[2];

    let tx = 2.0 * (y * vz - z * vy);
    let ty = 2.0 * (z * vx - x * vz);
    let tz = 2.0 * (x * vy - y * vx);

    [
        vx + w * tx + y * tz - z * ty,
        vy + w * ty + z * tx - x * tz,
        vz + w * tz + x * ty - y * tx,
    ]
}

/// Integrate quaternion by angular velocity
#[inline]
fn integrate_quaternion(q: &mut [f32; 4], omega: &[f32; 3], dt: f32) {
    let w = q[0];
    let x = q[1];
    let y = q[2];
    let z = q[3];

    let ox = omega[0] * 0.5 * dt;
    let oy = omega[1] * 0.5 * dt;
    let oz = omega[2] * 0.5 * dt;

    // Quaternion derivative: q' = 0.5 * omega * q
    q[0] = w - x * ox - y * oy - z * oz;
    q[1] = x + w * ox + y * oz - z * oy;
    q[2] = y + w * oy + z * ox - x * oz;
    q[3] = z + w * oz + x * oy - y * ox;

    // Normalize
    let len = (q[0] * q[0] + q[1] * q[1] + q[2] * q[2] + q[3] * q[3]).sqrt();
    if len > 0.0001 {
        let inv = 1.0 / len;
        q[0] *= inv;
        q[1] *= inv;
        q[2] *= inv;
        q[3] *= inv;
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_quat_rotate() {
        // Identity quaternion should not change vector
        let q = [1.0, 0.0, 0.0, 0.0];
        let v = [1.0, 2.0, 3.0];
        let result = quat_rotate(&q, &v);
        assert!((result[0] - 1.0).abs() < 0.001);
        assert!((result[1] - 2.0).abs() < 0.001);
        assert!((result[2] - 3.0).abs() < 0.001);
    }

    #[test]
    fn test_integrate_quaternion() {
        let mut q = [1.0, 0.0, 0.0, 0.0];
        let omega = [0.0, 0.0, 1.0]; // Yaw rotation
        integrate_quaternion(&mut q, &omega, 0.1);

        // Quaternion should still be unit length
        let len = (q[0] * q[0] + q[1] * q[1] + q[2] * q[2] + q[3] * q[3]).sqrt();
        assert!((len - 1.0).abs() < 0.001);
    }
}
