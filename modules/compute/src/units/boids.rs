use crate::engine::{ComputeError, ResourceLimits, UnitProxy};
use async_trait::async_trait;
use sdk::pingpong::PingPongBuffer;
use sdk::sab::SafeSAB;
use serde_json::Value as JsonValue;
use std::collections::HashMap;
use std::sync::atomic::{AtomicU32, Ordering};
use std::sync::Mutex;

/// Boid learning simulation - skeletal birds with full flocking physics
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

// ============= FLOCKING PARAMETERS =============
// Based on actual model dimensions from ArchitecturalBoids.tsx:
// - Body length: ~0.45, Beak: ~0.22, Total length: ~0.67
// - Wingspan: ~0.9 (0.45 per wing)
// - Height: ~0.12

/// Desired separation distance (prevents visual overlap)
const DESIRED_SEPARATION: f32 = 3.0; // ~3x wingspan for comfortable spacing

/// Visual perception radius for flocking
const PERCEPTION_RADIUS: f32 = 10.0;

/// Spatial hash cell size (should be >= PERCEPTION_RADIUS)
const CELL_SIZE: f32 = 10.0;

/// Force weights for flocking behavior
/// NOTE: These are per-frame forces, NOT accelerations (no dt multiplication)
const SEPARATION_WEIGHT: f32 = 1.5; // Strong - prevent collisions
const ALIGNMENT_WEIGHT: f32 = 0.05; // Subtle heading matching
const COHESION_WEIGHT: f32 = 0.02; // Very weak - prevents tight clustering
const BOUNDARY_WEIGHT: f32 = 0.5; // Moderate boundary push

/// Speed limits
const MAX_SPEED: f32 = 6.0;
const MIN_SPEED: f32 = 2.0;

/// World boundaries
const BOUND_X: f32 = 25.0;
const BOUND_Y: f32 = 12.0;
const BOUND_Z: f32 = 20.0;

// ============= SPATIAL HASHING =============

/// Hash a 3D position to a cell key
#[inline]
fn spatial_hash(x: f32, y: f32, z: f32) -> (i32, i32, i32) {
    (
        (x / CELL_SIZE).floor() as i32,
        (y / CELL_SIZE).floor() as i32,
        (z / CELL_SIZE).floor() as i32,
    )
}

// ============= FLOCKING FORCES =============

/// Calculate boundary avoidance force - smooth return to bounds
fn boundary_force(pos: &[f32; 3]) -> [f32; 3] {
    let mut force = [0.0f32; 3];

    // X boundary
    if pos[0] > BOUND_X {
        force[0] = -(pos[0] - BOUND_X);
    } else if pos[0] < -BOUND_X {
        force[0] = -BOUND_X - pos[0];
    }

    // Y boundary
    if pos[1] > BOUND_Y {
        force[1] = -(pos[1] - BOUND_Y);
    } else if pos[1] < -BOUND_Y {
        force[1] = -BOUND_Y - pos[1];
    }

    // Z boundary
    if pos[2] > BOUND_Z {
        force[2] = -(pos[2] - BOUND_Z);
    } else if pos[2] < -BOUND_Z {
        force[2] = -BOUND_Z - pos[2];
    }

    force
}

/// Normalize a vector and return its length
#[inline]
fn normalize(v: &mut [f32; 3]) -> f32 {
    let len = (v[0] * v[0] + v[1] * v[1] + v[2] * v[2]).sqrt();
    if len > 0.0001 {
        let inv = 1.0 / len;
        v[0] *= inv;
        v[1] *= inv;
        v[2] *= inv;
    }
    len
}

#[derive(Default)]
struct PersistentScratch {
    population_data: Vec<u8>,
    positions: Vec<[f32; 3]>,
    velocities: Vec<[f32; 3]>,
    grid: HashMap<(i32, i32, i32), Vec<usize>>,
    neighbor_cache: Vec<usize>, // Reusable neighbor list
}

pub struct BoidUnit {
    _config: BoidConfig,
    scratch: Mutex<PersistentScratch>,
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
            scratch: Mutex::new(PersistentScratch::default()),
        }
    }

    /// Step boids physics in SAB with full flocking behavior using Ping-Pong Buffers
    /// Memory flow: Read from active buffer → compute in linear memory → write to inactive buffer → flip
    pub fn step_physics_sab(&self, sab: &SafeSAB, bird_count: u32, dt: f32) -> Result<u32, String> {
        use sdk::layout::{BIRD_STRIDE, IDX_BIRD_COUNT, IDX_BIRD_EPOCH, OFFSET_ATOMIC_FLAGS};

        // Ensure logging is initialized (idempotent)
        sdk::init_logging();

        // Lock scratch buffers
        let mut scratch_guard = self
            .scratch
            .lock()
            .map_err(|_| "Failed to lock scratch buffers")?;

        // Destructure to allow simultaneous field access
        let PersistentScratch {
            ref mut population_data,
            ref mut positions,
            ref mut velocities,
            ref mut grid,
            ref mut neighbor_cache,
        } = *scratch_guard;

        // Create ping-pong buffer accessor
        let ping_pong = PingPongBuffer::bird_buffer(sab.clone());
        let read_info = ping_pong.read_buffer_info();
        let write_info = ping_pong.write_buffer_info();

        // Increment local counter for diagnostics
        let epoch = EPOCH_COUNTER.fetch_add(1, Ordering::SeqCst);

        // Diagnostic log every 100 steps
        // if epoch % 100 == 0 {
        //     log::info!(
        //         "[Boids] Step {} | Count: {} | DT: {:.4} | Read: 0x{:X} | Write: 0x{:X}",
        //         epoch,
        //         bird_count,
        //         dt,
        //         read_info.offset,
        //         write_info.offset
        //     );
        // }

        static mut GLOBAL_TIME: f32 = 0.0;
        unsafe {
            GLOBAL_TIME += dt;
        }
        let time = unsafe { GLOBAL_TIME };

        // --- STEP 1: Resize buffers if needed ---
        let total_bytes = bird_count as usize * BIRD_STRIDE;
        if population_data.len() < total_bytes {
            population_data.resize(total_bytes, 0);
        }
        let n = bird_count as usize;
        if positions.len() < n {
            positions.resize(n, [0.0; 3]);
            velocities.resize(n, [0.0; 3]);
        }

        // --- STEP 2: Read from SAB ---
        sab.read_raw(read_info.offset, &mut population_data[..total_bytes])?;

        // Extract positions and velocities
        for i in 0..n {
            let base = i * BIRD_STRIDE;
            for j in 0..3 {
                positions[i][j] = f32::from_le_bytes([
                    population_data[base + j * 4],
                    population_data[base + j * 4 + 1],
                    population_data[base + j * 4 + 2],
                    population_data[base + j * 4 + 3],
                ]);
                velocities[i][j] = f32::from_le_bytes([
                    population_data[base + 12 + j * 4],
                    population_data[base + 12 + j * 4 + 1],
                    population_data[base + 12 + j * 4 + 2],
                    population_data[base + 12 + j * 4 + 3],
                ]);
            }
        }

        // --- STEP 3: Build spatial hash ---
        grid.clear();
        for (idx, pos) in positions[..n].iter().enumerate() {
            let cell = spatial_hash(pos[0], pos[1], pos[2]);
            grid.entry(cell)
                .or_insert_with(|| Vec::with_capacity(32))
                .push(idx);
        }

        // --- STEP 4: Process boids behavior ---
        for i in 0..n {
            let base = i * BIRD_STRIDE;
            let pos = positions[i];
            let mut vel = velocities[i];

            // Extract evolutionary weights (offsets 0, 1, 2) to modulate behavior
            // Weights start at base + 60 (byte offset) -> 15th float
            // We use the first 3 weights for force multipliers
            let w_sep = f32::from_le_bytes([
                population_data[base + 60],
                population_data[base + 61],
                population_data[base + 62],
                population_data[base + 63],
            ]);
            let w_ali = f32::from_le_bytes([
                population_data[base + 64],
                population_data[base + 65],
                population_data[base + 66],
                population_data[base + 67],
            ]);
            let w_coh = f32::from_le_bytes([
                population_data[base + 68],
                population_data[base + 69],
                population_data[base + 70],
                population_data[base + 71],
            ]);
            let w_trick = f32::from_le_bytes([
                population_data[base + 72],
                population_data[base + 73],
                population_data[base + 74],
                population_data[base + 75],
            ]);

            // Debug log for Bird 0 to verify writes (throttled)
            if i == 0 && epoch % 200 == 0 {
                log::info!(
                    "[Boids] Bird 0 Weights | Sep: {:.4} | Ali: {:.4} | Coh: {:.4} | Trick: {:.4}",
                    w_sep,
                    w_ali,
                    w_coh,
                    w_trick
                );
            }

            // Get neighbors via spatial hash (Inlined to avoid borrow issues)
            neighbor_cache.clear();
            let cell = spatial_hash(pos[0], pos[1], pos[2]);
            for dx in -1..=1 {
                for dy in -1..=1 {
                    for dz in -1..=1 {
                        let nc = (cell.0 + dx, cell.1 + dy, cell.2 + dz);
                        if let Some(indices) = grid.get(&nc) {
                            for &other_idx in indices {
                                if other_idx != i {
                                    let other_pos = &positions[other_idx];
                                    let dist_sq = (pos[0] - other_pos[0]).powi(2)
                                        + (pos[1] - other_pos[1]).powi(2)
                                        + (pos[2] - other_pos[2]).powi(2);
                                    if dist_sq < PERCEPTION_RADIUS * PERCEPTION_RADIUS {
                                        neighbor_cache.push(other_idx);
                                    }
                                }
                            }
                        }
                    }
                }
            }

            // ========== CLASSIC BOID FORCES ==========
            let mut sep = [0.0f32; 3];
            let mut ali = [0.0f32; 3];
            let mut coh = [0.0f32; 3];

            if !neighbor_cache.is_empty() {
                let mut avg_vel = [0.0f32; 3];
                let mut center = [0.0f32; 3];
                let inv_neighbors = 1.0 / neighbor_cache.len() as f32;

                for &ni in neighbor_cache.iter() {
                    let other_pos = &positions[ni];
                    let other_vel = &velocities[ni];

                    // Separation
                    let dx = pos[0] - other_pos[0];
                    let dy = pos[1] - other_pos[1];
                    let dz = pos[2] - other_pos[2];
                    let d_sq = (dx * dx + dy * dy + dz * dz).max(0.01);
                    let d = d_sq.sqrt();
                    if d < DESIRED_SEPARATION {
                        let strength = (DESIRED_SEPARATION - d) / d_sq;
                        sep[0] += dx * strength;
                        sep[1] += dy * strength;
                        sep[2] += dz * strength;
                    }

                    // Alignment & Cohesion accumulators
                    avg_vel[0] += other_vel[0];
                    avg_vel[1] += other_vel[1];
                    avg_vel[2] += other_vel[2];
                    center[0] += other_pos[0];
                    center[1] += other_pos[1];
                    center[2] += other_pos[2];
                }

                ali[0] = (avg_vel[0] * inv_neighbors) - vel[0];
                ali[1] = (avg_vel[1] * inv_neighbors) - vel[1];
                ali[2] = (avg_vel[2] * inv_neighbors) - vel[2];
                coh[0] = (center[0] * inv_neighbors) - pos[0];
                coh[1] = (center[1] * inv_neighbors) - pos[1];
                coh[2] = (center[2] * inv_neighbors) - pos[2];
            }

            let bnd = boundary_force(&pos);

            // ========== APPLY FORCES ==========
            // ========== APPLY FORCES with Evolutionary Multipliers ==========
            // Base weights * (1.0 + evolved_weight). Evolved weight range typically -5.0 to 5.0
            // We clamp multiplier to be non-negative to avoid reversed forces (unless desired?)
            let mod_sep = SEPARATION_WEIGHT * (1.0 + w_sep).max(0.0);
            let mod_ali = ALIGNMENT_WEIGHT * (1.0 + w_ali).max(0.0);
            let mod_coh = COHESION_WEIGHT * (1.0 + w_coh).max(0.0);
            let mod_bnd = BOUNDARY_WEIGHT; // Keep boundary constant for stability

            let accel_scale = dt * 60.0;
            vel[0] += (sep[0] * mod_sep + ali[0] * mod_ali + coh[0] * mod_coh + bnd[0] * mod_bnd)
                * accel_scale;
            vel[1] += (sep[1] * mod_sep + ali[1] * mod_ali + coh[1] * mod_coh + bnd[1] * mod_bnd)
                * accel_scale;
            vel[2] += (sep[2] * mod_sep + ali[2] * mod_ali + coh[2] * mod_coh + bnd[2] * mod_bnd)
                * accel_scale;

            // ========== ENERGY SYSTEM: Flap-Glide Cycles ==========
            // Energy stored at Offset 40 (idx 10)
            let mut energy = f32::from_le_bytes([
                population_data[base + 40],
                population_data[base + 41],
                population_data[base + 42],
                population_data[base + 43],
            ]);

            // Initialize energy if zero (first frame)
            if energy <= 0.0 || energy > 1.0 {
                energy = 0.85 + (i as f32 % 10.0) * 0.01; // Higher starting energy
            }

            // Energy dynamics: climbing ALWAYS costs energy (can't glide upward)
            let is_climbing = vel[1] > 0.3;
            let current_speed = (vel[0] * vel[0] + vel[1] * vel[1] + vel[2] * vel[2]).sqrt();

            if is_climbing {
                // Climbing always costs energy - birds can't glide upward
                energy -= dt * 0.12;
            } else if current_speed > 5.0 {
                // Very fast = some energy cost
                energy -= dt * 0.05;
            } else {
                // Cruising/gliding recovers energy fast
                energy += dt * 0.25;
            }
            energy = energy.clamp(0.4, 1.0); // Higher minimum = always active

            // ========== TRICK SYSTEM: STATE SMOOTHING & SCREW MOTION ==========
            // Read previous Trick Intensity from Offset 36 (idx 9)
            let mut trick_intensity = f32::from_le_bytes([
                population_data[base + 36],
                population_data[base + 37],
                population_data[base + 38],
                population_data[base + 39],
            ]);

            let is_barrel_roll = w_trick < -2.0;
            let target_intensity = if is_barrel_roll { 1.0 } else { 0.0 };

            // Lerp intensity for SMOOTH transitions (slower = more comfortable)
            let lerp_speed = 1.0; // Reduced from 3.0 for gradual easing
            trick_intensity += (target_intensity - trick_intensity) * dt * lerp_speed;

            // Hover Mode (> 2.0) - kept simple
            if w_trick > 2.0 {
                vel[0] *= 0.90;
                vel[1] *= 0.90;
                vel[2] *= 0.90;
                let t = (epoch as f32) * 0.1;
                vel[1] += t.sin() * 0.05;
            }

            // Screw Motion Physics (Barrel Roll)
            // Apply tangential force if intensity is significant
            if trick_intensity > 0.01 {
                // Gentle speed boost (reduced from 0.05 to 0.02)
                vel[0] *= 1.0 + (0.02 * trick_intensity);
                vel[2] *= 1.0 + (0.02 * trick_intensity);
                vel[1] += 0.02 * trick_intensity; // Gentle surge up

                // Tangential "Screw" Force: gentler spiral (0.08 vs 0.2)
                let screw_strength = 0.08 * trick_intensity;
                let old_x = vel[0];
                vel[0] += vel[2] * screw_strength;
                vel[2] -= old_x * screw_strength;
            }

            // Artistic Sweeps
            let swirl_strength = 3.0;
            vel[0] += -pos[2] * swirl_strength * dt;
            vel[2] += pos[0] * swirl_strength * dt;

            let phase = i as f32 * 0.73;
            let noise_scale = dt * 60.0;
            vel[0] += ((time * 0.5 + phase).sin() * 0.4) * noise_scale;
            vel[1] += ((time * 0.8 + phase * 1.5).cos() * 0.25) * noise_scale;
            vel[2] += ((time * 0.3 + phase * 0.9).sin() * 0.4) * noise_scale;

            let damping = 0.97_f32.powf(dt * 60.0);
            vel[0] *= damping;
            vel[1] *= damping;
            vel[2] *= damping;

            let speed = normalize(&mut vel);
            let clamped_speed = speed.clamp(MIN_SPEED, MAX_SPEED);
            vel[0] *= clamped_speed;
            vel[1] *= clamped_speed;
            vel[2] *= clamped_speed;

            let mut new_pos = pos;
            new_pos[0] += vel[0] * dt;
            new_pos[1] += vel[1] * dt;
            new_pos[2] += vel[2] * dt;

            // Encode results back to scratch
            for j in 0..3 {
                population_data[base + j * 4..base + j * 4 + 4]
                    .copy_from_slice(&new_pos[j].to_le_bytes());
                population_data[base + 12 + j * 4..base + 16 + j * 4]
                    .copy_from_slice(&vel[j].to_le_bytes());
            }

            // Save Trick Intensity state (Offset 36 = idx 9)
            population_data[base + 36..base + 40].copy_from_slice(&trick_intensity.to_le_bytes());

            if clamped_speed > 0.005 {
                let mut rot_y = vel[0].atan2(vel[2]);

                // HEADING TURN: Gentle direction change during trick
                // Reduced from 0.02 to 0.008 for comfortable turning
                if trick_intensity > 0.01 {
                    rot_y += trick_intensity * 0.008 * dt * 60.0;
                }

                // Base Physics Bank
                let physics_bank = (-vel[0] * 0.15).clamp(-0.25, 0.25);

                // Target Spiral Bank (Softer turn + Organic wobble)
                // 0.6 rad ~= 35 degrees (more natural than 70°)
                let wobble = (time * 3.0).sin() * 0.1;
                let spiral_bank = 0.6 + wobble;

                // VISUAL BLEND: Mix Physics Bank with Spiral Bank based on intensity
                // Smoothly transitions into the banking turn
                let bank_z = physics_bank * (1.0 - trick_intensity) + spiral_bank * trick_intensity;

                population_data[base + 24..base + 28].copy_from_slice(&rot_y.to_le_bytes());
                population_data[base + 28..base + 32].copy_from_slice(&bank_z.to_le_bytes());
            }

            // ========== ENERGY-BASED FLIGHT MODES ==========
            // Flight intensity depends on energy level
            // Low energy = Gliding (minimal flapping)
            // High energy = Active flapping

            let flight_intensity = if energy < 0.5 {
                // CRUISING MODE: Active flapping at lower energy
                0.6 + (energy - 0.4) * 0.4
            } else if energy < 0.75 {
                // NORMAL MODE: Good flapping
                0.7 + (energy - 0.5) * 0.6
            } else {
                // POWER MODE: Vigorous flapping
                0.85 + (energy - 0.75) * 0.6
            };

            // Base flap with per-bird phase variation
            let base_flap = 5.0 + (i % 8) as f32;
            let flap_amplitude = 0.6 * flight_intensity * (1.0 + trick_intensity * 0.3);
            let flap = (time * base_flap + i as f32 * 2.1).sin() * flap_amplitude;

            // Asymmetric wings during tricks (differential lift)
            let left_wing = -flap + trick_intensity * 0.4;
            let right_wing = flap - trick_intensity * 0.4;

            population_data[base + 44..base + 48].copy_from_slice(&left_wing.to_le_bytes());
            population_data[base + 48..base + 52].copy_from_slice(&right_wing.to_le_bytes());

            // Write tail_yaw at offset 52 (idx 13) - math.rs expects this here
            let tail_yaw = (-vel[0] * 0.1).clamp(-0.15, 0.15);
            population_data[base + 52..base + 56].copy_from_slice(&tail_yaw.to_le_bytes());

            // ========== COMPETITIVE FITNESS FUNCTION ==========
            // Creates REAL selection pressure with penalties

            let fitness_base = 0.1; // Lower base for more variance

            // NEIGHBOR SCORE: Optimal 3-7, penalize isolation AND overcrowding
            let neighbor_count = neighbor_cache.len();
            let neighbor_score = if neighbor_count == 0 {
                0.0 // Complete isolation = bad
            } else if neighbor_count <= 2 {
                0.1 // Very isolated
            } else if neighbor_count <= 7 {
                0.35 // Sweet spot
            } else if neighbor_count <= 15 {
                0.15 // Somewhat crowded
            } else {
                0.0 // Overcrowded = bad
            };

            // SPEED SCORE: Penalize extremes
            let speed_score = if clamped_speed < 1.5 {
                0.0 // Too slow
            } else if clamped_speed <= 4.0 {
                0.3 // Optimal range
            } else if clamped_speed <= 5.5 {
                0.15 // Somewhat fast
            } else {
                0.0 // Too fast
            };

            // ENERGY EFFICIENCY: Reward birds that conserve energy
            let energy_score = if energy > 0.5 { 0.25 } else { energy * 0.4 };

            let fitness = fitness_base + neighbor_score + speed_score + energy_score;
            population_data[base + 56..base + 60].copy_from_slice(&fitness.to_le_bytes());

            // Save energy state at offset 40
            population_data[base + 40..base + 44].copy_from_slice(&energy.to_le_bytes());
        }

        // --- STEP 5: Write Scratch → SAB ---
        sab.write_raw(write_info.offset, &population_data[..total_bytes])?;

        // Write bird count to Atomic Flags (Index 20 - IDX_BIRD_COUNT)
        // Use absolute offset (OFFSET_ATOMIC_FLAGS + Index * 4)
        sab.write(
            OFFSET_ATOMIC_FLAGS + IDX_BIRD_COUNT as usize * 4,
            &bird_count.to_le_bytes(),
        )
        .map_err(|e| format!("Failed to write bird count to SAB: {}", e))?;

        // --- STEP 6: Flip buffers ---
        let new_epoch = ping_pong.flip();

        sab.write(
            OFFSET_ATOMIC_FLAGS + IDX_BIRD_EPOCH as usize * 4,
            &(new_epoch as u32).to_le_bytes(),
        )
        .map_err(|e| format!("Epoch write failed: {}", e))?;

        Ok(new_epoch as u32)
    }

    /// Initialize population in SAB using Ping-Pong Buffer architecture
    /// Writes initial state to BOTH buffers A and B so first physics step works correctly
    pub fn init_population_sab(sab: &SafeSAB, bird_count: u32) -> Result<(), String> {
        use sdk::layout::{BIRD_STRIDE, IDX_BIRD_COUNT, OFFSET_ATOMIC_FLAGS};

        sdk::init_logging();

        // Create ping-pong buffer accessor
        let ping_pong = PingPongBuffer::bird_buffer(sab.clone());
        let read_info = ping_pong.read_buffer_info();
        let write_info = ping_pong.write_buffer_info();

        log::info!(
            "[Boids] Initializing population: {} birds | Buffer A: 0x{:X} | Buffer B: 0x{:X}",
            bird_count,
            read_info.offset,
            write_info.offset
        );

        // Write bird count to Atomic Flags (Index 20 - IDX_BIRD_COUNT)
        // Use absolute offset (OFFSET_ATOMIC_FLAGS + Index * 4)
        sab.write(
            OFFSET_ATOMIC_FLAGS + IDX_BIRD_COUNT as usize * 4,
            &bird_count.to_le_bytes(),
        )
        .map_err(|e| format!("Failed to write bird count to SAB: {}", e))?;

        let total_bytes = bird_count as usize * BIRD_STRIDE;
        let mut population_data = vec![0u8; total_bytes];

        for i in 0..bird_count as usize {
            let base = i * BIRD_STRIDE;
            // Use golden ratio spiral for initial distribution (painterly)
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

        // Write to BOTH buffers so first physics step can read from either
        sab.write_raw(read_info.offset, &population_data)
            .map_err(|e| format!("SAB write to buffer A failed: {}", e))?;
        sab.write_raw(write_info.offset, &population_data)
            .map_err(|e| format!("SAB write to buffer B failed: {}", e))?;

        log::info!("[Boids] Population initialization complete (both ping-pong buffers)");
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

        let epoch = self
            .step_physics_sab(&sab, bird_count, dt)
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
        vec!["init_population", "step_physics", "evolve_batch"]
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
        let res = match method {
            "init_population" => {
                let params: JsonValue = serde_json::from_slice(params).unwrap_or(JsonValue::Null);
                self.init_population_impl(&params)?
            }
            "step_physics" => {
                let params: JsonValue = serde_json::from_slice(params).unwrap_or(JsonValue::Null);
                self.step_physics_impl(&params)?
            }
            "evolve_batch" => return self.evolve_batch_impl(params),
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

impl BoidUnit {
    fn evolve_batch_impl(&self, resource_data: &[u8]) -> Result<Vec<u8>, ComputeError> {
        use sdk::protocols::resource::resource;

        // 1. Decode Resource Wrapper
        let mut reader = std::io::Cursor::new(resource_data);
        let message_reader =
            capnp::serialize::read_message(&mut reader, capnp::message::ReaderOptions::new())
                .map_err(|e| ComputeError::ExecutionFailed(format!("Capnp read error: {}", e)))?;

        let res_reader = message_reader
            .get_root::<resource::Reader>()
            .map_err(|e| ComputeError::ExecutionFailed(format!("Capnp root error: {}", e)))?;

        // 2. Extract Inline Data (Packed Parents)
        let inline_data = match res_reader.which() {
            Ok(resource::Which::Inline(data)) => {
                data.map_err(|_| ComputeError::ExecutionFailed("Invalid inline data".into()))?
            }
            _ => {
                return Err(ComputeError::ExecutionFailed(
                    "Expected inline resource data".into(),
                ))
            }
        };

        let parents = deserialize_genes_binary(inline_data);
        if parents.is_empty() {
            return Err(ComputeError::ExecutionFailed(
                "No parents provided for evolution".into(),
            ));
        }

        // 3. Simple Evolution Parameters (Hardcoded for now, can be in metadata/parameters)
        let offspring_count = parents.len(); // Default to 1:1 for simplicity in this shard

        let mut offspring = Vec::with_capacity(offspring_count);
        let mut rng = rand::thread_rng();

        for _i in 0..offspring_count {
            let p1 = tournament_select(&parents, &mut rng);
            let p2 = tournament_select(&parents, &mut rng);
            let mut child = crossover(&p1, &p2, &mut rng);
            mutate(&mut child, &mut rng);
            // BirdID will be reassigned by the supervisor in Go
            offspring.push(child);
        }

        // 4. Wrap Response in Resource
        let packed_offspring = serialize_genes_binary(&offspring);

        let mut out_message = capnp::message::Builder::new_default();
        {
            let mut out_res = out_message.init_root::<resource::Builder>();
            out_res.set_id("boids-evolve-res");
            out_res.set_inline(&packed_offspring);
        }

        let mut output_bytes = Vec::new();
        capnp::serialize::write_message(&mut output_bytes, &out_message)
            .map_err(|e| ComputeError::ExecutionFailed(format!("Serialize error: {}", e)))?;

        Ok(output_bytes)
    }
}

// ============= GENETIC ALGORITHM HELPERS (RUST SIDE) =============

#[derive(Clone, Debug)]
struct RustBirdGenes {
    bird_id: u32,
    fitness: f64,
    weights: [f32; 44],
}

fn tournament_select(population: &[RustBirdGenes], rng: &mut impl rand::Rng) -> RustBirdGenes {
    let mut best = &population[rng.gen_range(0..population.len())];
    for _ in 1..3 {
        let candidate = &population[rng.gen_range(0..population.len())];
        if candidate.fitness > best.fitness {
            best = candidate;
        }
    }
    best.clone()
}

fn crossover(p1: &RustBirdGenes, p2: &RustBirdGenes, rng: &mut impl rand::Rng) -> RustBirdGenes {
    let mut weights = [0.0f32; 44];
    for i in 0..44 {
        weights[i] = if rng.gen_bool(0.5) {
            p1.weights[i]
        } else {
            p2.weights[i]
        };
    }
    RustBirdGenes {
        bird_id: 0,
        fitness: 0.0,
        weights,
    }
}

fn mutate(genes: &mut RustBirdGenes, rng: &mut impl rand::Rng) {
    for i in 0..44 {
        if rng.gen_bool(0.1) {
            genes.weights[i] += rng.gen_range(-0.5..0.5);
            genes.weights[i] = genes.weights[i].clamp(-10.0, 10.0);
        }
    }
}

fn serialize_genes_binary(genes: &[RustBirdGenes]) -> Vec<u8> {
    const STRIDE: usize = 188;
    let mut buf = vec![0u8; genes.len() * STRIDE];
    for (i, g) in genes.iter().enumerate() {
        let offset = i * STRIDE;
        buf[offset..offset + 4].copy_from_slice(&g.bird_id.to_le_bytes());
        buf[offset + 4..offset + 12].copy_from_slice(&g.fitness.to_le_bytes());
        for w in 0..44 {
            let wo = offset + 12 + w * 4;
            buf[wo..wo + 4].copy_from_slice(&g.weights[w].to_le_bytes());
        }
    }
    buf
}

fn deserialize_genes_binary(data: &[u8]) -> Vec<RustBirdGenes> {
    const STRIDE: usize = 188;
    let count = data.len() / STRIDE;
    let mut genes = Vec::with_capacity(count);
    for i in 0..count {
        let offset = i * STRIDE;
        let bird_id = u32::from_le_bytes(data[offset..offset + 4].try_into().unwrap());
        let fitness = f64::from_le_bytes(data[offset + 4..offset + 12].try_into().unwrap());
        let mut weights = [0.0f32; 44];
        for w in 0..44 {
            let wo = offset + 12 + w * 4;
            weights[w] = f32::from_le_bytes(data[wo..wo + 4].try_into().unwrap());
        }
        genes.push(RustBirdGenes {
            bird_id,
            fitness,
            weights,
        });
    }
    genes
}
