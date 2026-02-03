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
const OFFSET_DRONE_CONTROL: usize = 0x160000;
const OFFSET_DRONE_STATE_A: usize = 0x160200;
const OFFSET_DRONE_STATE_B: usize = 0x161200;
const SIZE_DRONE_BUFFER: usize = 4096; // 32 * 128 bytes
const CONTROL_STRIDE: usize = 16; // 4 * f32
const SIZE_CONTROL_BUFFER: usize = MAX_DRONES * CONTROL_STRIDE;

// Physics constants (Racing Quad 5" Profile)
const MASS: f32 = 0.65; // kg (typical 5" racing quad)
const GRAVITY: f32 = 9.81; // m/s²
const ARM_LENGTH: f32 = 0.1; // m (200mm diagonal / 2)

// Blade Element Theory Constants
const C_T: f32 = 0.11; // Thrust coefficient
const C_Q: f32 = 0.008; // Torque coefficient
const RHO: f32 = 1.225; // Air density kg/m³
const PROP_DIAMETER: f32 = 0.127; // 5" = 0.127m
const PROP_D4: f32 = 0.000260; // D^4 precomputed
const PROP_D5: f32 = 0.000033; // D^5 precomputed
const MAX_MOTOR_RPM: f32 = 35000.0; // Typical racing motor
const MOTOR_TAU: f32 = 0.04; // Motor time constant (seconds)
const ARM_COS_45: f32 = 0.70710678;
const MOTOR_DIR: [f32; 4] = [1.0, -1.0, 1.0, -1.0];

// Aerodynamic drag
const DRAG_AREA_X: f32 = 0.018; // Side area m²
const DRAG_AREA_Y: f32 = 0.012; // Top area m²
const DRAG_AREA_Z: f32 = 0.024; // Frontal area m²
const DRAG_COEFF_X: f32 = 1.05;
const DRAG_COEFF_Y: f32 = 0.9;
const DRAG_COEFF_Z: f32 = 1.15;

// Inertia tensor (simplified)
const IXX: f32 = 0.003; // kg·m² (lighter racing quad)
const IYY: f32 = 0.003;
const IZZ: f32 = 0.006;

// ============= GROUND EFFECT & FLOOR AWARENESS =============
// Ground effect: thrust increases ~30% when within 1.5× prop diameter
const GROUND_EFFECT_HEIGHT: f32 = 0.2; // ~1.5 × prop diameter (0.127 * 1.5)
const SAFE_ALTITUDE: f32 = 0.5; // Minimum safe hover altitude
const FLOOR_SPRING: f32 = 25.0; // Soft spring force for floor repulsion
const CEILING_HEIGHT: f32 = 30.0; // Maximum altitude

// ============= DRONE SEPARATION (COLLISION AVOIDANCE) =============
const DRONE_SEPARATION: f32 = 2.5; // Minimum distance between drones (~3× body)
const SEPARATION_FORCE: f32 = 12.0; // Repulsion strength

// ============= ORIENTATION SMOOTHING =============
// Reduced from 8.0 for smoother, more cinematic flight
const MAX_ROLL_RATE: f32 = 3.0; // rad/s (~170°/s)
const MAX_PITCH_RATE: f32 = 3.0;
const MAX_YAW_RATE: f32 = 2.0; // rad/s (~115°/s)
const RATE_P: f32 = 8.0; // P-gain (reduced from 15.0)
const RATE_D: f32 = 2.0; // D-gain for damping
const ANGULAR_DRAG: [f32; 3] = [0.015, 0.02, 0.018];

// ============= ENVIRONMENTAL FACTORS =============
const TURBULENCE_STRENGTH: f32 = 0.3; // m/s² peak turbulence
const TURBULENCE_SCALE: f32 = 0.15; // Spatial frequency
const BASE_WIND_SPEED: f32 = 2.5; // m/s
const GUST_STRENGTH: f32 = 1.8; // m/s
const AIR_TEMP_C: f32 = 24.0; // base ambient temperature
const AIR_TEMP_LAPSE: f32 = 0.0065; // °C per meter (ISA lapse rate)
const BATTERY_SAG: f32 = 0.15; // max sag at full throttle
const DISC_AREA: f32 = 0.0127; // pi * (0.127/2)^2
const MAX_DT: f32 = 0.02;

// ============= PROXIMITY SPEED LIMITING =============
const MAX_SPEED: f32 = 12.0; // m/s
const MIN_SPEED: f32 = 3.0; // m/s (prevent stalling)
const APPROACH_SLOWDOWN_DIST: f32 = 8.0; // Start slowing at this distance from gate

// Gate positions for autonomous flight (matching RaceTrack.tsx - expanded track)
const GATE_POSITIONS: [[f32; 3]; 8] = [
    [0.0, 3.0, -20.0],
    [20.0, 4.0, -35.0],
    [40.0, 3.0, -20.0],
    [20.0, 4.0, 0.0],
    [0.0, 3.0, 20.0],
    [-20.0, 4.0, 35.0],
    [-40.0, 3.0, 20.0],
    [-20.0, 4.0, 0.0],
];

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
    control_data: Vec<f32>,
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
        let dt = dt.clamp(0.0005, MAX_DT);

        // Resize scratch if needed
        if scratch.state_data.len() < total_floats {
            scratch.state_data.resize(total_floats, 0.0);
            scratch.last_count = count;
        }
        if scratch.control_data.len() < n * 4 {
            scratch.control_data.resize(n * 4, 0.0);
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

        // Read from SAB (state)
        let byte_slice = unsafe {
            std::slice::from_raw_parts_mut(
                scratch.state_data.as_mut_ptr() as *mut u8,
                total_floats * 4,
            )
        };
        sab.read_raw(read_info.offset, byte_slice)?;

        // Read control buffer (external control overrides autonomous if present)
        let control_bytes = unsafe {
            std::slice::from_raw_parts_mut(
                scratch.control_data.as_mut_ptr() as *mut u8,
                n * CONTROL_STRIDE,
            )
        };
        sab.read_raw(OFFSET_DRONE_CONTROL, control_bytes)?;

        // Shared time step (avoid per-drone increment)
        static mut GLOBAL_TIME: f32 = 0.0;
        let time = unsafe {
            GLOBAL_TIME += dt;
            GLOBAL_TIME
        };

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
            let mut motor_rpm = [
                scratch.state_data[base + offsets::MOTOR_RPM],
                scratch.state_data[base + offsets::MOTOR_RPM + 1],
                scratch.state_data[base + offsets::MOTOR_RPM + 2],
                scratch.state_data[base + offsets::MOTOR_RPM + 3],
            ];
            let mut battery = scratch.state_data[base + offsets::RESERVED];
            let mut motor_temp = scratch.state_data[base + offsets::RESERVED + 1];

            if battery <= 0.0 {
                battery = 1.0;
            }

            // Read race state (gate index in slot 1)
            let current_gate = scratch.state_data[base + offsets::RACE_STATE + 1] as usize % 8;
            let target = GATE_POSITIONS[current_gate];

            // ============ CONTROL INPUT (EXTERNAL OR AUTONOMOUS) ============
            let ctrl_base = i * 4;
            let ext_throttle = scratch.control_data[ctrl_base].clamp(0.0, 1.0);
            let ext_pitch = scratch.control_data[ctrl_base + 1].clamp(-1.0, 1.0);
            let ext_roll = scratch.control_data[ctrl_base + 2].clamp(-1.0, 1.0);
            let ext_yaw = scratch.control_data[ctrl_base + 3].clamp(-1.0, 1.0);

            let external_active =
                ext_throttle > 0.01 || ext_pitch.abs() > 0.01 || ext_roll.abs() > 0.01 || ext_yaw.abs() > 0.01;

            let (throttle, pitch_cmd, roll_cmd, yaw_cmd) = if external_active {
                (ext_throttle, ext_pitch, ext_roll, ext_yaw)
            } else {
                let target = GATE_POSITIONS[current_gate];
                compute_autonomous_control(&pos, &vel, &quat, &target)
            };

            // Persist control for telemetry/visualization
            scratch.state_data[base + offsets::CONTROL] = throttle;
            scratch.state_data[base + offsets::CONTROL + 1] = pitch_cmd;
            scratch.state_data[base + offsets::CONTROL + 2] = roll_cmd;
            scratch.state_data[base + offsets::CONTROL + 3] = yaw_cmd;

            // Check if we passed the gate (within 3m)
            let dx = target[0] - pos[0];
            let dy = target[1] - pos[1];
            let dz = target[2] - pos[2];
            let dist = (dx * dx + dy * dy + dz * dz).sqrt();
            if dist < 3.0 {
                // Advance to next gate
                scratch.state_data[base + offsets::RACE_STATE + 1] =
                    ((current_gate + 1) % 8) as f32;
            }

            // ============ AIR DENSITY ============
            let air_temp = AIR_TEMP_C - pos[1] * AIR_TEMP_LAPSE;
            let rho = compute_air_density(pos[1], air_temp);

            // ============ RATE CONTROLLER (TARGET MOMENTS) ============
            let target_rates = [
                (roll_cmd * MAX_ROLL_RATE).clamp(-MAX_ROLL_RATE, MAX_ROLL_RATE),
                (pitch_cmd * MAX_PITCH_RATE).clamp(-MAX_PITCH_RATE, MAX_PITCH_RATE),
                (yaw_cmd * MAX_YAW_RATE).clamp(-MAX_YAW_RATE, MAX_YAW_RATE),
            ];

            let rate_error = [
                target_rates[0] - ang_vel[0],
                target_rates[1] - ang_vel[1],
                target_rates[2] - ang_vel[2],
            ];

            let desired_accel = [
                rate_error[0] * RATE_P - ang_vel[0] * RATE_D,
                rate_error[1] * RATE_P - ang_vel[1] * RATE_D,
                rate_error[2] * RATE_P - ang_vel[2] * RATE_D,
            ];

            let i_omega = [IXX * ang_vel[0], IYY * ang_vel[1], IZZ * ang_vel[2]];
            let omega_cross_i = [
                ang_vel[1] * i_omega[2] - ang_vel[2] * i_omega[1],
                ang_vel[2] * i_omega[0] - ang_vel[0] * i_omega[2],
                ang_vel[0] * i_omega[1] - ang_vel[1] * i_omega[0],
            ];

            let mut moment_cmd = [
                IXX * desired_accel[0] + omega_cross_i[0],
                IYY * desired_accel[1] + omega_cross_i[1],
                IZZ * desired_accel[2] + omega_cross_i[2],
            ];

            // ============ GROUND EFFECT ============
            // Thrust boost + leveling torque when near ground
            let (ge_multiplier, leveling_torque) = compute_ground_effect(pos[1], &quat);
            moment_cmd[0] += leveling_torque[0];
            moment_cmd[2] += leveling_torque[2];

            // ============ COLLECTIVE THRUST ============
            let sag = BATTERY_SAG * throttle + (1.0 - battery) * 0.1;
            let battery_factor = (1.0 - sag).clamp(0.65, 1.0);
            let max_rpm = MAX_MOTOR_RPM * battery_factor;
            let max_n = max_rpm / 60.0;
            let max_thrust_per_motor = C_T * rho * max_n * max_n * PROP_D4;
            let total_thrust_cmd = (throttle * 4.0 * max_thrust_per_motor).max(0.0);

            // ============ MIXER (X-FRAME) ============
            let mix = ARM_COS_45 * ARM_LENGTH;
            let roll_term = moment_cmd[0] / (4.0 * mix + 1e-5);
            let pitch_term = moment_cmd[1] / (4.0 * mix + 1e-5);

            let mut thrusts = [
                total_thrust_cmd * 0.25 - pitch_term + roll_term,
                total_thrust_cmd * 0.25 - pitch_term - roll_term,
                total_thrust_cmd * 0.25 + pitch_term - roll_term,
                total_thrust_cmd * 0.25 + pitch_term + roll_term,
            ];

            for m in 0..4 {
                thrusts[m] = thrusts[m].clamp(0.0, max_thrust_per_motor * 1.2);
            }

            // ============ MOTOR DYNAMICS ============
            let alpha = dt / (MOTOR_TAU + dt);
            let kq = C_Q * rho * PROP_D5;
            let kt = C_T * rho * PROP_D4;
            let mut total_thrust = 0.0;
            let mut yaw_torque = 0.0;

            for m in 0..4 {
                let target_n = (thrusts[m] / (kt + 1e-6)).sqrt();
                let target_rpm = (target_n * 60.0).min(max_rpm);

                motor_rpm[m] += (target_rpm - motor_rpm[m]) * alpha;
                motor_rpm[m] = motor_rpm[m].clamp(0.0, max_rpm);

                let n = motor_rpm[m] / 60.0;
                let thrust = kt * n * n;
                let torque = kq * n * n * MOTOR_DIR[m];
                total_thrust += thrust;
                yaw_torque += torque;
            }

            // Apply yaw correction (delta torque via RPM adjustment)
            let yaw_error = moment_cmd[2] - yaw_torque;
            let yaw_per_motor = yaw_error / 4.0;
            for m in 0..4 {
                let n = motor_rpm[m] / 60.0;
                let base_q = kq * n * n;
                let desired_q = (base_q + yaw_per_motor * MOTOR_DIR[m]).max(0.0);
                let desired_n = (desired_q / (kq + 1e-6)).sqrt();
                let desired_rpm = (desired_n * 60.0).min(max_rpm);
                motor_rpm[m] += (desired_rpm - motor_rpm[m]) * (alpha * 0.5);
            }

            // Recompute thrust and actual moments after yaw correction
            let mut motor_thrusts = [0.0f32; 4];
            total_thrust = 0.0;
            yaw_torque = 0.0;
            for m in 0..4 {
                let n = motor_rpm[m] / 60.0;
                let thrust = kt * n * n;
                motor_thrusts[m] = thrust;
                total_thrust += thrust;
                yaw_torque += kq * n * n * MOTOR_DIR[m];
            }

            let roll_term_actual =
                (motor_thrusts[0] - motor_thrusts[1] - motor_thrusts[2] + motor_thrusts[3]) * 0.25;
            let pitch_term_actual =
                (-motor_thrusts[0] - motor_thrusts[1] + motor_thrusts[2] + motor_thrusts[3]) * 0.25;
            let moment_actual = [
                roll_term_actual * 4.0 * mix,
                pitch_term_actual * 4.0 * mix,
                yaw_torque,
            ];

            let vel_body_for_inflow = quat_inv_rotate(&quat, &vel);
            let vi = (total_thrust / (2.0 * rho * DISC_AREA + 1e-5)).sqrt();
            let inflow_factor = (1.0 - vel_body_for_inflow[1] / (2.0 * vi + 0.5)).clamp(0.35, 1.25);
            total_thrust *= ge_multiplier * inflow_factor;

            // Rotate thrust to world frame (thrust is +Y in body)
            let thrust_body = [0.0, total_thrust, 0.0];
            let thrust_world = quat_rotate(&quat, &thrust_body);

            // ============ WIND FIELD ============
            let wind = compute_wind(&pos, time);
            let rel_vel = [vel[0] - wind[0], vel[1] - wind[1], vel[2] - wind[2]];

            // ============ AERODYNAMIC DRAG ============
            let vel_body = quat_inv_rotate(&quat, &rel_vel);
            let drag_body = [
                -0.5 * rho * DRAG_COEFF_X * DRAG_AREA_X * vel_body[0] * vel_body[0].abs(),
                -0.5 * rho * DRAG_COEFF_Y * DRAG_AREA_Y * vel_body[1] * vel_body[1].abs(),
                -0.5 * rho * DRAG_COEFF_Z * DRAG_AREA_Z * vel_body[2] * vel_body[2].abs(),
            ];
            let drag = quat_rotate(&quat, &drag_body);

            // ============ TURBULENCE ============
            // Position-based coherent disturbance
            let turbulence = compute_turbulence(&pos, time);

            // ============ FLOOR REPULSION ============
            // Soft spring force instead of hard clamp
            let floor_force = compute_floor_repulsion(pos[1], vel[1]);

            // ============ DRONE SEPARATION ============
            // Find nearest drone and compute separation force
            let mut separation = [0.0f32; 3];
            let mut nearest_dist = f32::MAX;
            for j in 0..n {
                if i == j {
                    continue;
                }
                let other_base = j * (DRONE_STRIDE / 4);
                let ox = scratch.state_data[other_base + offsets::POSITION];
                let oy = scratch.state_data[other_base + offsets::POSITION + 1];
                let oz = scratch.state_data[other_base + offsets::POSITION + 2];
                let dx = pos[0] - ox;
                let dy = pos[1] - oy;
                let dz = pos[2] - oz;
                let dist = (dx * dx + dy * dy + dz * dz).sqrt();
                if dist < nearest_dist {
                    nearest_dist = dist;
                }
                if dist < DRONE_SEPARATION && dist > 0.01 {
                    let strength = (DRONE_SEPARATION - dist) / dist * SEPARATION_FORCE;
                    separation[0] += dx * strength;
                    separation[1] += dy * strength;
                    separation[2] += dz * strength;
                }
            }

            // ============ LINEAR DYNAMICS ============
            let gravity_force = [0.0, -GRAVITY * MASS, 0.0];
            let accel = [
                (thrust_world[0]
                    + gravity_force[0]
                    + drag[0]
                    + turbulence[0]
                    + separation[0])
                    / MASS,
                (thrust_world[1]
                    + gravity_force[1]
                    + drag[1]
                    + turbulence[1]
                    + separation[1]
                    + floor_force)
                    / MASS,
                (thrust_world[2]
                    + gravity_force[2]
                    + drag[2]
                    + turbulence[2]
                    + separation[2])
                    / MASS,
            ];

            // Integrate velocity
            vel[0] += accel[0] * dt;
            vel[1] += accel[1] * dt;
            vel[2] += accel[2] * dt;

            // ============ SPEED LIMITING ============
            // Slow down near gates and other drones
            let speed_limit = compute_speed_limit(dist, nearest_dist);
            let speed = (vel[0] * vel[0] + vel[1] * vel[1] + vel[2] * vel[2]).sqrt();
            if speed > speed_limit {
                let scale = speed_limit / speed;
                vel[0] *= scale;
                vel[1] *= scale;
                vel[2] *= scale;
            }

            // Integrate position
            pos[0] += vel[0] * dt;
            pos[1] += vel[1] * dt;
            pos[2] += vel[2] * dt;

            // ============ SOFT FLOOR BOUNDARY ============
            // Allow soft spring to dominate, but hard clamp as safety
            if pos[1] < 0.1 {
                pos[1] = 0.1;
                vel[1] = vel[1].abs() * 0.2; // Gentle bounce
            }

            // Ceiling limit (soft)
            if pos[1] > CEILING_HEIGHT {
                pos[1] = CEILING_HEIGHT;
                vel[1] = -vel[1].abs() * 0.1;
            }

            // ============ ANGULAR DYNAMICS ============
            let damping = [
                -ang_vel[0] * ANGULAR_DRAG[0],
                -ang_vel[1] * ANGULAR_DRAG[1],
                -ang_vel[2] * ANGULAR_DRAG[2],
            ];

            let ang_accel = [
                (moment_actual[0] + damping[0] - omega_cross_i[0]) / IXX,
                (moment_actual[1] + damping[1] - omega_cross_i[1]) / IYY,
                (moment_actual[2] + damping[2] - omega_cross_i[2]) / IZZ,
            ];

            ang_vel[0] += ang_accel[0] * dt;
            ang_vel[1] += ang_accel[1] * dt;
            ang_vel[2] += ang_accel[2] * dt;

            ang_vel[0] = ang_vel[0].clamp(-MAX_ROLL_RATE * 2.5, MAX_ROLL_RATE * 2.5);
            ang_vel[1] = ang_vel[1].clamp(-MAX_PITCH_RATE * 2.5, MAX_PITCH_RATE * 2.5);
            ang_vel[2] = ang_vel[2].clamp(-MAX_YAW_RATE * 3.0, MAX_YAW_RATE * 3.0);

            // Integrate quaternion
            integrate_quaternion(&mut quat, &ang_vel, dt);

            // Battery + thermal model
            let load = (motor_rpm[0] + motor_rpm[1] + motor_rpm[2] + motor_rpm[3])
                / (4.0 * MAX_MOTOR_RPM);
            battery = (battery - (load * 0.0025 * dt)).clamp(0.15, 1.0);
            motor_temp = (motor_temp + (load * 0.4 - 0.15) * dt).clamp(0.0, 1.2);

            if !(pos[0].is_finite()
                && pos[1].is_finite()
                && pos[2].is_finite()
                && vel[0].is_finite()
                && vel[1].is_finite()
                && vel[2].is_finite()
                && ang_vel[0].is_finite()
                && ang_vel[1].is_finite()
                && ang_vel[2].is_finite()
                && quat[0].is_finite()
                && quat[1].is_finite()
                && quat[2].is_finite()
                && quat[3].is_finite())
            {
                pos = [0.0, 1.0, 0.0];
                vel = [0.0, 0.0, 0.0];
                ang_vel = [0.0, 0.0, 0.0];
                quat = [1.0, 0.0, 0.0, 0.0];
            }

            // ============ WRITE BACK ============
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

            // Store motor RPM for visualization
            scratch.state_data[base + offsets::MOTOR_RPM] = motor_rpm[0];
            scratch.state_data[base + offsets::MOTOR_RPM + 1] = motor_rpm[1];
            scratch.state_data[base + offsets::MOTOR_RPM + 2] = motor_rpm[2];
            scratch.state_data[base + offsets::MOTOR_RPM + 3] = motor_rpm[3];

            scratch.state_data[base + offsets::RESERVED] = battery;
            scratch.state_data[base + offsets::RESERVED + 1] = motor_temp;
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

/// Autonomous flight controller - P-loop targeting next gate
/// Returns (throttle, pitch, roll, yaw) commands in range [-1, 1]
#[inline]
fn compute_autonomous_control(
    pos: &[f32; 3],
    vel: &[f32; 3],
    quat: &[f32; 4],
    target: &[f32; 3],
) -> (f32, f32, f32, f32) {
    // Vector to target
    let dx = target[0] - pos[0];
    let dy = target[1] - pos[1];
    let dz = target[2] - pos[2];
    let dist_xz = (dx * dx + dz * dz).sqrt().max(0.1);

    // ========== ALTITUDE CONTROL ==========
    let alt_error = dy;
    let alt_rate = vel[1];
    // PD altitude controller
    let alt_p = 0.5;
    let alt_d = 0.3;
    let alt_cmd = alt_error * alt_p - alt_rate * alt_d;
    // Base hover throttle + altitude correction
    let hover_throttle = 0.55;
    let throttle = (hover_throttle + alt_cmd * 0.2).clamp(0.2, 0.95);

    // ========== YAW CONTROL ==========
    // Get current forward direction from quaternion
    let fwd = quat_rotate(quat, &[0.0, 0.0, -1.0]);

    // Desired heading
    let desired_yaw = (-dx).atan2(-dz);
    let current_yaw = fwd[0].atan2(-fwd[2]);
    let mut yaw_error = desired_yaw - current_yaw;

    // Wrap to [-pi, pi]
    while yaw_error > std::f32::consts::PI {
        yaw_error -= 2.0 * std::f32::consts::PI;
    }
    while yaw_error < -std::f32::consts::PI {
        yaw_error += 2.0 * std::f32::consts::PI;
    }

    let yaw_cmd = (yaw_error * 1.5).clamp(-1.0, 1.0);

    // ========== FORWARD VELOCITY / PITCH CONTROL ==========
    // Pitch forward to accelerate towards target
    let speed_xz = (vel[0] * vel[0] + vel[2] * vel[2]).sqrt();
    let target_speed = (dist_xz * 0.5).min(12.0); // Max 12 m/s
    let speed_error = target_speed - speed_xz;
    let pitch_cmd = (speed_error * 0.15).clamp(-0.5, 0.5);

    // ========== ROLL CONTROL ==========
    // Bank into turns proportional to yaw rate
    let roll_cmd = (-yaw_error * 0.3).clamp(-0.4, 0.4);

    (throttle, pitch_cmd, roll_cmd, yaw_cmd)
}

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

/// Rotate vector by inverse quaternion
#[inline]
fn quat_inv_rotate(q: &[f32; 4], v: &[f32; 3]) -> [f32; 3] {
    let conj = [q[0], -q[1], -q[2], -q[3]];
    quat_rotate(&conj, v)
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

/// Extract up-component from quaternion (how vertical the drone is)
/// Returns 1.0 when level, decreases when tilted
#[inline]
fn quat_up_component(q: &[f32; 4]) -> f32 {
    let _w = q[0];
    let x = q[1];
    let _y = q[2];
    let z = q[3];
    // Transform [0, 1, 0] by quaternion and extract y-component
    // Simplified: 1 - 2*(x² + z²)
    (1.0 - 2.0 * (x * x + z * z)).max(0.0)
}

/// Compute ground effect thrust multiplier and leveling torque
/// Returns (thrust_multiplier, leveling_torque)
#[inline]
fn compute_ground_effect(altitude: f32, quat: &[f32; 4]) -> (f32, [f32; 3]) {
    // Account for drone tilt in effective clearance
    let up_factor = quat_up_component(quat);
    let effective_clearance = altitude * up_factor;

    if effective_clearance > GROUND_EFFECT_HEIGHT {
        return (1.0, [0.0, 0.0, 0.0]);
    }

    let clearance = effective_clearance.max(0.02);
    let ratio = (PROP_DIAMETER / (4.0 * clearance)).clamp(0.0, 0.9);
    let thrust_multiplier = (1.0 / (1.0 - ratio * ratio)).clamp(1.0, 1.35);

    let height_ratio = (effective_clearance / GROUND_EFFECT_HEIGHT).clamp(0.0, 1.0);

    // Leveling torque: pushes drone level when near ground
    // Extract tilt from quaternion (simplified)
    let tilt_x = quat[1]; // Roll component
    let tilt_z = quat[3]; // Pitch component
    let leveling_strength = 5.0 * (1.0 - height_ratio);
    let leveling_torque = [
        -tilt_x * leveling_strength,
        0.0,
        -tilt_z * leveling_strength,
    ];

    (thrust_multiplier, leveling_torque)
}

/// Compute soft floor repulsion force
#[inline]
fn compute_floor_repulsion(altitude: f32, velocity_y: f32) -> f32 {
    if altitude >= SAFE_ALTITUDE {
        return 0.0;
    }

    let penetration = SAFE_ALTITUDE - altitude;
    // Spring force + damping to prevent oscillation
    let spring_force = penetration * FLOOR_SPRING;
    let damping = -velocity_y.min(0.0) * 5.0; // Only damp downward velocity
    spring_force + damping
}

/// Compute simple turbulence based on position and time
#[inline]
fn compute_turbulence(pos: &[f32; 3], time: f32) -> [f32; 3] {
    // Altitude-dependent (more turbulence higher up)
    let altitude_factor = (pos[1] / 18.0).clamp(0.0, 1.0);

    let p = [
        pos[0] * TURBULENCE_SCALE + time * 0.4,
        pos[1] * TURBULENCE_SCALE * 0.6 + time * 0.2,
        pos[2] * TURBULENCE_SCALE + time * 0.3,
    ];

    let n = fbm3(p, 3);
    let n2 = fbm3([p[0] + 19.7, p[1] - 7.3, p[2] + 2.1], 3);
    let n3 = fbm3([p[0] - 4.1, p[1] + 11.9, p[2] - 15.4], 3);

    [
        (n * 2.0 - 1.0) * TURBULENCE_STRENGTH * altitude_factor,
        (n2 * 2.0 - 1.0) * TURBULENCE_STRENGTH * 0.6 * altitude_factor,
        (n3 * 2.0 - 1.0) * TURBULENCE_STRENGTH * altitude_factor,
    ]
}

/// Compute wind vector using coherent noise + base flow
#[inline]
fn compute_wind(pos: &[f32; 3], time: f32) -> [f32; 3] {
    let base_dir = [
        (time * 0.05).cos(),
        0.0,
        (time * 0.05).sin(),
    ];

    let gust_pos = [
        pos[0] * 0.06 + time * 0.12,
        pos[1] * 0.03 + time * 0.05,
        pos[2] * 0.06 - time * 0.08,
    ];
    let gust = fbm3(gust_pos, 4);
    let gust_y = fbm3([gust_pos[0] + 5.2, gust_pos[1], gust_pos[2] - 9.1], 3);
    let gust_scale = (pos[1] / 20.0).clamp(0.2, 1.0);

    [
        base_dir[0] * BASE_WIND_SPEED + (gust * 2.0 - 1.0) * GUST_STRENGTH * gust_scale,
        (gust_y * 2.0 - 1.0) * 0.4 * GUST_STRENGTH * gust_scale,
        base_dir[2] * BASE_WIND_SPEED + (gust * 2.0 - 1.0) * GUST_STRENGTH * 0.6 * gust_scale,
    ]
}

/// Air density based on altitude and temperature (ISA approximation)
#[inline]
fn compute_air_density(altitude: f32, temp_c: f32) -> f32 {
    let rho = RHO * (-altitude / 8500.0).exp();
    let temp_factor = (273.15 / (273.15 + temp_c.max(-40.0))).clamp(0.7, 1.2);
    (rho * temp_factor).clamp(0.9, 1.3)
}

#[inline]
fn smoothstep(t: f32) -> f32 {
    t * t * (3.0 - 2.0 * t)
}

#[inline]
fn hash_u32(mut x: u32) -> u32 {
    x ^= x >> 16;
    x = x.wrapping_mul(0x7feb352d);
    x ^= x >> 15;
    x = x.wrapping_mul(0x846ca68b);
    x ^= x >> 16;
    x
}

#[inline]
fn hash3(ix: i32, iy: i32, iz: i32) -> f32 {
    let mut h = ix as u32;
    h = h.wrapping_mul(374761393) ^ (iy as u32).wrapping_mul(668265263);
    h ^= (iz as u32).wrapping_mul(362437);
    let v = hash_u32(h);
    (v as f32 / u32::MAX as f32).clamp(0.0, 1.0)
}

#[inline]
fn value_noise3(p: [f32; 3]) -> f32 {
    let x0 = p[0].floor() as i32;
    let y0 = p[1].floor() as i32;
    let z0 = p[2].floor() as i32;
    let xf = p[0] - x0 as f32;
    let yf = p[1] - y0 as f32;
    let zf = p[2] - z0 as f32;

    let u = smoothstep(xf);
    let v = smoothstep(yf);
    let w = smoothstep(zf);

    let c000 = hash3(x0, y0, z0);
    let c100 = hash3(x0 + 1, y0, z0);
    let c010 = hash3(x0, y0 + 1, z0);
    let c110 = hash3(x0 + 1, y0 + 1, z0);
    let c001 = hash3(x0, y0, z0 + 1);
    let c101 = hash3(x0 + 1, y0, z0 + 1);
    let c011 = hash3(x0, y0 + 1, z0 + 1);
    let c111 = hash3(x0 + 1, y0 + 1, z0 + 1);

    let x00 = c000 + (c100 - c000) * u;
    let x10 = c010 + (c110 - c010) * u;
    let x01 = c001 + (c101 - c001) * u;
    let x11 = c011 + (c111 - c011) * u;

    let y0 = x00 + (x10 - x00) * v;
    let y1 = x01 + (x11 - x01) * v;

    y0 + (y1 - y0) * w
}

#[inline]
fn fbm3(mut p: [f32; 3], octaves: usize) -> f32 {
    let mut value = 0.0;
    let mut amp = 0.5;
    let mut freq = 1.0;
    for _ in 0..octaves {
        value += value_noise3([p[0] * freq, p[1] * freq, p[2] * freq]) * amp;
        amp *= 0.5;
        freq *= 2.0;
        p[0] += 19.1;
        p[1] += 7.3;
        p[2] += 3.7;
    }
    value.clamp(0.0, 1.0)
}

/// Compute speed limit based on proximity to gates and other drones
#[inline]
fn compute_speed_limit(dist_to_gate: f32, dist_to_nearest_drone: f32) -> f32 {
    // Slow down when approaching gates
    let gate_factor = if dist_to_gate < APPROACH_SLOWDOWN_DIST {
        0.5 + 0.5 * (dist_to_gate / APPROACH_SLOWDOWN_DIST)
    } else {
        1.0
    };

    // Slow down when near other drones
    let collision_factor = if dist_to_nearest_drone < DRONE_SEPARATION * 2.0 {
        0.4 + 0.6 * (dist_to_nearest_drone / (DRONE_SEPARATION * 2.0))
    } else {
        1.0
    };

    (MAX_SPEED * gate_factor * collision_factor).max(MIN_SPEED)
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
