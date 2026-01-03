pub mod engine;
pub mod executor;
mod units;

#[cfg(test)]
pub mod benchmarks;

use engine::ComputeEngine;
use log::{info, warn};
use sdk::{Epoch, Reactor, IDX_SYSTEM_EPOCH};
use units::{
    ApiProxy, AudioUnit, BoidUnit, CryptoUnit, DataUnit, GpuUnit, ImageUnit, PhysicsEngine,
    StorageUnit,
};

fn initialize_engine() -> ComputeEngine {
    let mut engine = ComputeEngine::new();

    // Register Unit Proxies
    engine.register(Box::new(ImageUnit::new()));
    engine.register(Box::new(CryptoUnit::new()));
    engine.register(Box::new(DataUnit::new()));
    engine.register(Box::new(AudioUnit::new()));
    engine.register(Box::new(GpuUnit::new()));
    engine.register(Box::new(
        StorageUnit::new().expect("Failed to initialize StorageUnit"),
    ));
    engine.register(Box::new(PhysicsEngine::new()));
    engine.register(Box::new(BoidUnit::new()));
    engine.register(Box::new(ApiProxy::new()));

    engine
}

/// Standardized Memory Allocator for WebAssembly
#[no_mangle]
pub extern "C" fn compute_alloc(size: usize) -> *mut u8 {
    let mut buf = Vec::with_capacity(size);
    let ptr = buf.as_mut_ptr();
    std::mem::forget(buf);
    ptr
}

/// Standardized Initialization with SharedArrayBuffer
#[no_mangle]
pub extern "C" fn compute_init_with_sab() -> i32 {
    // Use stable ABI to get global object
    let global = sdk::js_interop::get_global();
    sdk::js_interop::console_log("[compute] Init: Global object retrieved", 3);

    let sab_key = sdk::js_interop::create_string("__INOS_SAB__");
    let sab_val = sdk::js_interop::reflect_get(&global, &sab_key);
    sdk::js_interop::console_log("[compute] Init: SAB Value retrieved", 3);

    let offset_key = sdk::js_interop::create_string("__INOS_SAB_OFFSET__");
    let offset_val = sdk::js_interop::reflect_get(&global, &offset_key);

    let size_key = sdk::js_interop::create_string("__INOS_SAB_SIZE__");
    let size_val = sdk::js_interop::reflect_get(&global, &size_key);

    let id_key = sdk::js_interop::create_string("__INOS_MODULE_ID__");
    let id_val = sdk::js_interop::reflect_get(&global, &id_key);

    if let (Ok(val), Ok(off), Ok(sz), Ok(id)) = (sab_val, offset_val, size_val, id_val) {
        sdk::js_interop::console_log("[compute] Init: All values retrieved successfully", 3);
        if !val.is_undefined() && !val.is_null() {
            sdk::js_interop::console_log("[compute] Init: SAB is defined and not null", 3);
            let offset = sdk::js_interop::as_f64(&off).unwrap_or(0.0) as u32;
            let size = sdk::js_interop::as_f64(&sz).unwrap_or(0.0) as u32;
            let module_id = id.as_f64().unwrap_or(0.0) as u32;

            // Create TWO SafeSAB references:
            // 1. Scoped view for module data (offset-based)
            let _module_sab = sdk::sab::SafeSAB::new_shared_view(&val, offset, size);
            // 2. Global SAB for registry writes (full access)
            let global_sab = sdk::sab::SafeSAB::new(&val);

            // Set global identity context
            sdk::set_module_id(module_id);

            sdk::init_logging();
            info!("Compute module initialized (ID: {}) with synchronized SAB bridge (Offset: 0x{:x}, Size: {}MB)", 
                module_id, offset, size / 1024 / 1024);

            // Trigger registration of capabilities using GLOBAL SAB
            register_compute_capabilities(&global_sab);

            return 1; // success
        } else {
            sdk::js_interop::console_log("[compute] Init FAILED: SAB is undefined or null", 1);
        }
    } else {
        sdk::js_interop::console_log("[compute] Init FAILED: Could not retrieve global values", 1);
    }
    0 // failure
}

/// External poll entry point for JavaScript
#[no_mangle]
pub extern "C" fn compute_poll() {
    // High-frequency reactor for Compute
}

/// N-body particle physics step
/// Reads particles from SAB at 0x200000, applies forces, writes back
#[no_mangle]
pub extern "C" fn compute_nbody_step(particle_count: u32, dt: f32) -> i32 {
    use sdk::js_interop;

    // Get SAB
    let global = js_interop::get_global();
    let sab_key = js_interop::create_string("__INOS_SAB__");
    let sab_val = match js_interop::reflect_get(&global, &sab_key) {
        Ok(val) if !val.is_undefined() && !val.is_null() => val,
        _ => {
            js_interop::console_log("[compute] compute_nbody_step: SAB not available", 1);
            return 0;
        }
    };

    let sab = sdk::sab::SafeSAB::new(&sab_val);

    const PARTICLE_BUFFER_OFFSET: usize = 0x200000;
    const PARTICLE_SIZE: usize = 32; // 8 floats per particle
    const G: f32 = 5.0;
    const SOFTENING: f32 = 15.0;
    const DAMPING: f32 = 1.0;

    // Read all particles
    let mut particles: Vec<[f32; 8]> = Vec::with_capacity(particle_count as usize);
    for i in 0..particle_count as usize {
        let offset = PARTICLE_BUFFER_OFFSET + i * PARTICLE_SIZE;
        let mut particle = [0.0f32; 8];
        for j in 0..8 {
            if let Ok(bytes) = sab.read(offset + j * 4, 4) {
                particle[j] = f32::from_le_bytes([bytes[0], bytes[1], bytes[2], bytes[3]]);
            }
        }
        particles.push(particle);
    }

    // Apply N-body forces
    for i in 0..particle_count as usize {
        let mut fx = 0.0f32;
        let mut fy = 0.0f32;

        let p1 = particles[i];
        let mass1 = p1[6];

        for j in 0..particle_count as usize {
            if i == j {
                continue;
            }

            let p2 = particles[j];
            let dx = p2[0] - p1[0];
            let dy = p2[1] - p1[1];
            let dist_sq = dx * dx + dy * dy + SOFTENING * SOFTENING;
            let inv_dist = 1.0 / dist_sq.sqrt();
            let inv_dist_cube = inv_dist * inv_dist * inv_dist;

            let force = G * mass1 * p2[6] * inv_dist_cube;
            fx += dx * force;
            fy += dy * force;
        }

        // Update velocity
        let ax = fx / mass1;
        let ay = fy / mass1;
        particles[i][3] += ax * dt;
        particles[i][4] += ay * dt;
        particles[i][3] *= DAMPING;
        particles[i][4] *= DAMPING;

        // Update position
        particles[i][0] += particles[i][3] * dt;
        particles[i][1] += particles[i][4] * dt;
    }

    // Write back to SAB
    for i in 0..particle_count as usize {
        let offset = PARTICLE_BUFFER_OFFSET + i * PARTICLE_SIZE;
        for j in 0..8 {
            let bytes = particles[i][j].to_le_bytes();
            let _ = sab.write(offset + j * 4, &bytes);
        }
    }

    // Increment system epoch
    let flags_offset = 0;
    let epoch_idx = 7; // IDX_SYSTEM_EPOCH
    if let Ok(bytes) = sab.read(flags_offset + epoch_idx * 4, 4) {
        let current = i32::from_le_bytes([bytes[0], bytes[1], bytes[2], bytes[3]]);
        let new_epoch = (current + 1).to_le_bytes();
        let _ = sab.write(flags_offset + epoch_idx * 4, &new_epoch);
    }

    1 // success
}

/// Initialize Boids population in SAB
#[no_mangle]
pub extern "C" fn compute_boids_init(bird_count: u32) -> i32 {
    use sdk::js_interop;

    let global = js_interop::get_global();
    let sab_key = js_interop::create_string("__INOS_SAB__");
    let sab_val = match js_interop::reflect_get(&global, &sab_key) {
        Ok(val) if !val.is_undefined() && !val.is_null() => val,
        _ => return 0,
    };

    let sab = sdk::sab::SafeSAB::new(&sab_val);
    match units::boids::BoidUnit::init_population_sab(&sab, bird_count) {
        Ok(_) => 1,
        Err(_) => 0,
    }
}

/// Step Boids physics in SAB
/// Returns the current epoch number, or 0 on error
#[no_mangle]
pub extern "C" fn compute_boids_step(bird_count: u32, dt: f32, elapsed_time: f32) -> u32 {
    use sdk::js_interop;

    let global = js_interop::get_global();
    let sab_key = js_interop::create_string("__INOS_SAB__");
    let sab_val = match js_interop::reflect_get(&global, &sab_key) {
        Ok(val) if !val.is_undefined() && !val.is_null() => val,
        _ => return 0,
    };

    let sab = sdk::sab::SafeSAB::new(&sab_val);
    match units::boids::BoidUnit::step_physics_sab(&sab, bird_count, dt, elapsed_time) {
        Ok(epoch) => epoch,
        Err(_) => 0,
    }
}

/// Initialize enhanced N-body simulation with particle types and parameters
/// Particle types: 0=normal, 1=star, 2=black hole, 3=dark matter
#[no_mangle]
pub extern "C" fn compute_init_nbody_enhanced(
    particle_count: u32,
    force_law: u32,
    enable_collisions: u32,
) -> i32 {
    use sdk::js_interop;

    let global = js_interop::get_global();
    let sab_key = js_interop::create_string("__INOS_SAB__");
    let sab_val = match js_interop::reflect_get(&global, &sab_key) {
        Ok(val) if !val.is_undefined() && !val.is_null() => val,
        _ => {
            js_interop::console_log("[compute] init_nbody_enhanced: SAB not available", 1);
            return 0;
        }
    };

    let sab = sdk::sab::SafeSAB::new(&sab_val);

    const PARAMS_OFFSET: usize = 0x300000;

    js_interop::console_log(
        &format!(
            "[compute] Initializing enhanced N-body: {} particles, force_law={}, collisions={}",
            particle_count, force_law, enable_collisions
        ),
        3,
    );

    // Initialize simulation parameters at 0x300000
    let params = [
        5.0f32, // G
        0.016,  // dt
        particle_count as f32,
        15.0, // softening
        force_law as f32,
        0.5, // dark_matter_factor
        0.0, // cosmic_expansion
        enable_collisions as f32,
        1.2,    // merge_threshold
        0.3,    // restitution
        1.0,    // tidal_forces
        0.01,   // drag_coefficient
        0.1,    // turbulence_strength
        0.05,   // turbulence_scale
        0.05,   // magnetic_strength
        0.01,   // radiation_pressure
        1000.0, // universe_radius
        0.1,    // background_density
        0.0,    // time (will be updated each frame)
    ];

    for (i, &param) in params.iter().enumerate() {
        let bytes = param.to_le_bytes();
        let _ = sab.write(PARAMS_OFFSET + i * 4, &bytes);
    }

    js_interop::console_log("[compute] Enhanced N-body initialized successfully", 3);
    1
}

/// Enhanced N-body step with full particle structure (64 bytes per particle)
/// Layout: position(12) + velocity(12) + acceleration(12) + mass(4) + radius(4) +
///         color(16) + temperature(4) + luminosity(4) + type(4) + lifetime(4) + angular_vel(12)
#[no_mangle]
pub extern "C" fn compute_nbody_step_enhanced(particle_count: u32, dt: f32) -> i32 {
    use sdk::js_interop;

    let global = js_interop::get_global();
    let sab_key = js_interop::create_string("__INOS_SAB__");
    let sab_val = match js_interop::reflect_get(&global, &sab_key) {
        Ok(val) if !val.is_undefined() && !val.is_null() => val,
        _ => return 0,
    };

    let sab = sdk::sab::SafeSAB::new(&sab_val);

    const PARTICLE_BUFFER_OFFSET: usize = 0x200000;
    const PARTICLE_SIZE: usize = 88;
    const G: f32 = 5.0;
    const SOFTENING: f32 = 15.0;
    const DAMPING: f32 = 1.0;

    // Read all particles (simplified structure for CPU fallback)
    let mut particles: Vec<[f32; 22]> = Vec::with_capacity(particle_count as usize);
    for i in 0..particle_count as usize {
        let offset = PARTICLE_BUFFER_OFFSET + i * PARTICLE_SIZE;
        let mut particle = [0.0f32; 22];
        for j in 0..22 {
            if let Ok(bytes) = sab.read(offset + j * 4, 4) {
                particle[j] = f32::from_le_bytes([bytes[0], bytes[1], bytes[2], bytes[3]]);
            }
        }
        particles.push(particle);
    }

    // Apply N-body forces (full 3D)
    for i in 0..particle_count as usize {
        let mut fx = 0.0f32;
        let mut fy = 0.0f32;
        let mut fz = 0.0f32;

        let p1 = particles[i];
        let mass1 = p1[9]; // mass at index 9

        for j in 0..particle_count as usize {
            if i == j {
                continue;
            }

            let p2 = particles[j];
            let dx = p2[0] - p1[0]; // position x
            let dy = p2[1] - p1[1]; // position y
            let dz = p2[2] - p1[2]; // position z

            let dist_sq = dx * dx + dy * dy + dz * dz + SOFTENING * SOFTENING;
            let inv_dist = 1.0 / dist_sq.sqrt();
            let inv_dist_cube = inv_dist * inv_dist * inv_dist;

            let force = G * mass1 * p2[9] * inv_dist_cube;
            fx += dx * force;
            fy += dy * force;
            fz += dz * force;
        }

        // Update velocity
        let ax = fx / mass1;
        let ay = fy / mass1;
        let az = fz / mass1;
        particles[i][3] += ax * dt; // velocity x
        particles[i][4] += ay * dt; // velocity y
        particles[i][5] += az * dt; // velocity z

        particles[i][3] *= DAMPING;
        particles[i][4] *= DAMPING;
        particles[i][5] *= DAMPING;

        // Update position
        particles[i][0] += particles[i][3] * dt;
        particles[i][1] += particles[i][4] * dt;
        particles[i][2] += particles[i][5] * dt;

        // Update temperature from velocity (collisional heating)
        let speed_sq = particles[i][3] * particles[i][3]
            + particles[i][4] * particles[i][4]
            + particles[i][5] * particles[i][5];
        particles[i][14] = particles[i][14] * 0.9 + speed_sq * 0.01 * 0.1; // temperature at index 14
    }

    // Write back to SAB
    for i in 0..particle_count as usize {
        let offset = PARTICLE_BUFFER_OFFSET + i * PARTICLE_SIZE;
        for j in 0..22 {
            let bytes = particles[i][j].to_le_bytes();
            let _ = sab.write(offset + j * 4, &bytes);
        }
    }

    // Increment system epoch
    let flags_offset = 0;
    let epoch_idx = 7;
    if let Ok(bytes) = sab.read(flags_offset + epoch_idx * 4, 4) {
        let current = i32::from_le_bytes([bytes[0], bytes[1], bytes[2], bytes[3]]);
        let new_epoch = (current + 1).to_le_bytes();
        let _ = sab.write(flags_offset + epoch_idx * 4, &new_epoch);
    }

    1
}

/// Set simulation parameters at runtime
#[no_mangle]
pub extern "C" fn compute_set_sim_params(param_index: u32, value: f32) -> i32 {
    use sdk::js_interop;

    let global = js_interop::get_global();
    let sab_key = js_interop::create_string("__INOS_SAB__");
    let sab_val = match js_interop::reflect_get(&global, &sab_key) {
        Ok(val) if !val.is_undefined() && !val.is_null() => val,
        _ => return 0,
    };

    let sab = sdk::sab::SafeSAB::new(&sab_val);
    const PARAMS_OFFSET: usize = 0x300000;

    let offset = PARAMS_OFFSET + (param_index as usize) * 4;
    let bytes = value.to_le_bytes();
    match sab.write(offset, &bytes) {
        Ok(_) => 1,
        Err(_) => 0,
    }
}

pub struct ComputeKernel {
    reactor: Reactor,
    engine: ComputeEngine,
    epoch: Epoch,
}

// Helper to register capabilities (moved from ComputeKernel::new to be standalone)
fn register_compute_capabilities(sab: &sdk::sab::SafeSAB) {
    use sdk::registry::*;

    // Helper to register simple modules
    let register_simple = |id: &str, mem: u16, gpu: bool| {
        let mut builder = ModuleEntryBuilder::new(id).version(1, 4, 3);
        builder = builder.capability("image", gpu, mem);
        builder = builder.capability("video", gpu, mem);
        builder = builder.capability("audio", gpu, mem);
        builder = builder.capability("crypto", gpu, mem);
        builder = builder.capability("data", gpu, mem);
        builder = builder.capability("gpu_shader", gpu, 4096);

        match builder.build() {
            Ok((mut entry, _, caps)) => {
                // No deps in simple mode
                if let Ok(offset) = write_capability_table(sab, &caps) {
                    info!(
                        "[VERIFY] Cap table written to offset 0x{:x}, {} entries",
                        offset,
                        caps.len()
                    );

                    // Immediately verify the write by reading back
                    if let Ok(verify_data) = sab.read(offset as usize, 16) {
                        info!("[VERIFY] First 16 bytes after write: {:02x?}", verify_data);

                        // Check if first 4 bytes are the capability name or zeros
                        if verify_data[0] == 0
                            && verify_data[1] == 0
                            && verify_data[2] == 0
                            && verify_data[3] == 0
                        {
                            warn!("[VERIFY] ⚠️  CORRUPTION DETECTED: First 4 bytes are zeros immediately after write!");
                        } else {
                            info!("[VERIFY] ✓ Data intact: first 4 bytes = {:02x} {:02x} {:02x} {:02x}", 
                                verify_data[0], verify_data[1], verify_data[2], verify_data[3]);
                        }
                    }

                    entry.cap_table_offset = offset;
                }
                if let Ok((slot, _)) = find_slot_double_hashing(sab, id) {
                    if let Err(e) = write_enhanced_entry(sab, slot, &entry) {
                        info!("Failed to write module {}: {}", id, e);
                    } else {
                        info!("Registered module {} at slot {}", id, slot);
                    }
                }
            }
            Err(e) => info!("Failed to build module {}: {:?}", id, e),
        }
    };

    // Register core modules provided by this kernel
    register_simple("compute", 512, false); // Base compute
    register_simple("boids", 512, false); // Flocking simulation

    // Note: specialized units (ml, storage/vault, etc.) register themselves via their own WASM binaries.
    // We do NOT register them here to avoid registry collisions.
}

impl ComputeKernel {
    pub fn new(
        sab: &web_sys::wasm_bindgen::JsValue,
        _offset: u32,
        _size: u32,
        node_id: String,
    ) -> Self {
        sdk::init_logging();
        info!("Compute Kernel initialized on node {}", node_id);

        let engine = initialize_engine();
        let reactor = Reactor::new(sab);

        // Use standardized System Epoch index from SDK
        let epoch = Epoch::new(sab, IDX_SYSTEM_EPOCH);

        // No need to call register_compute_capabilities here anymore,
        // it's already done in compute_init_with_sab using the correct safe_sab.

        Self {
            reactor,
            engine,
            epoch,
        }
    }

    /// Poll for new compute segments using Reactive Mutation
    pub async fn poll(&mut self) -> bool {
        if !self.reactor.check_inbox() {
            return false;
        }

        self.reactor.ack_inbox();

        // 1. Get Inbox data and copy to buffer
        let data = match self.reactor.read_request() {
            Some(d) => d,
            None => return false,
        };

        // 2. Execute via Engine
        // Use proper Cap'n Proto processing
        let result = self.process_job(&data).await;

        match result {
            Ok(output) => {
                // Return success result
                if let Ok(serialized) = self.serialize_result(true, &output, "") {
                    if !self.reactor.write_result(&serialized) {
                        log::error!("Output too large for outbox: {} bytes", serialized.len());
                        // Write error result
                        if let Ok(err_bytes) = self.serialize_result(false, &[], "Output too large")
                        {
                            self.reactor.write_result(&err_bytes);
                        }
                    }
                }
            }
            Err(e) => {
                log::error!("Compute job failed: {}", e);
                // Write error result
                if let Ok(err_bytes) = self.serialize_result(false, &[], &e.to_string()) {
                    self.reactor.write_result(&err_bytes);
                }
            }
        }

        // 3. Signal completion via Epoch
        self.epoch.increment();

        true
    }

    /// Process job using Cap'n Proto "Lens"
    async fn process_job(&self, data: &[u8]) -> Result<Vec<u8>, engine::ComputeError> {
        let mut reader = std::io::Cursor::new(data);
        let message_reader =
            capnp::serialize::read_message(&mut reader, capnp::message::ReaderOptions::new())
                .map_err(|e| {
                    engine::ComputeError::ExecutionFailed(format!("Capnp read error: {}", e))
                })?;

        // Access the lens
        let job = message_reader
            .get_root::<sdk::protocols::compute::compute::job_request::Reader>()
            .map_err(|e| {
                engine::ComputeError::ExecutionFailed(format!("Capnp root error: {}", e))
            })?;

        // Zero-copy field access
        let library_reader = job
            .get_library()
            .map_err(|_| engine::ComputeError::ExecutionFailed("Invalid library field".into()))?;
        let library = library_reader
            .to_str()
            .map_err(|_| engine::ComputeError::ExecutionFailed("Library not valid UTF-8".into()))?;

        let method_reader = job
            .get_method()
            .map_err(|_| engine::ComputeError::ExecutionFailed("Invalid method field".into()))?;
        let method = method_reader
            .to_str()
            .map_err(|_| engine::ComputeError::ExecutionFailed("Method not valid UTF-8".into()))?;

        let params_reader = job
            .get_params()
            .map_err(|_| engine::ComputeError::ExecutionFailed("Invalid params field".into()))?;
        let params = params_reader; // Already &[u8] due to schema change (Data)

        let input = job
            .get_input()
            .map_err(|_| engine::ComputeError::ExecutionFailed("Invalid input field".into()))?;

        info!(
            "Engine execution (Capnp): unit={}, action={}, input_size={}",
            library,
            method,
            input.len()
        );

        self.engine.execute(library, method, input, params).await
    }

    /// Helper to serialize JobResult
    fn serialize_result(
        &self,
        success: bool,
        data: &[u8],
        error_msg: &str,
    ) -> Result<Vec<u8>, engine::ComputeError> {
        let mut message = capnp::message::Builder::new_default();
        let mut root = message.init_root::<sdk::protocols::compute::compute::job_result::Builder>();

        // Set status
        if success {
            root.set_status(sdk::protocols::compute::compute::Status::Success);
        } else {
            root.set_status(sdk::protocols::compute::compute::Status::Failed);
        }

        // Set output
        root.set_output(data);

        // Set error message
        root.set_error_message(error_msg);

        let mut output_bytes = Vec::new();
        capnp::serialize::write_message(&mut output_bytes, &message).map_err(|e| {
            engine::ComputeError::ExecutionFailed(format!("Serialize error: {}", e))
        })?;

        Ok(output_bytes)
    }
}
