use crate::matter::{ScienceError, ScienceProxy, ScienceResult};
use crate::mesh::cache::ComputationCache;
use crate::types::{CacheEntry, Telemetry};
use blake3;
use nalgebra::{Quaternion, UnitQuaternion, Vector3};
use rapier3d::prelude::*;
use serde::{Deserialize, Serialize};

use std::cell::RefCell;
use std::collections::{HashMap, VecDeque};
use std::rc::Rc;

// ----------------------------------------------------------------------------
// KINETIC PHYSICS TYPES
// ----------------------------------------------------------------------------

#[derive(Serialize, Deserialize, Clone)]
pub enum BodyType {
    Static,
    Dynamic,
    Kinematic,
    Sensor,
    QuantumCoupled,
    ContinuumCoupled,
}

#[derive(Serialize, Deserialize, Clone)]
pub enum ColliderShape {
    Sphere { radius: f32 },
    Box { half_extents: [f32; 3] },
    Capsule { half_height: f32, radius: f32 },
    Cylinder { half_height: f32, radius: f32 },
}

#[derive(Serialize, Deserialize)]
pub struct RigidBodyDefinition {
    pub body_type: BodyType,
    pub position: [f32; 3],
    pub rotation: [f32; 4],
    pub linear_velocity: [f32; 3],
    pub angular_velocity: [f32; 3],
    pub mass: f32,
    pub linear_damping: f32,
    pub angular_damping: f32,
    pub gravity_scale: f32,
}

#[derive(Serialize, Deserialize)]
pub struct ColliderDefinition {
    pub shape: ColliderShape,
    pub translation: [f32; 3],
    pub rotation: [f32; 4],
    pub material: ColliderMaterial,
    pub sensor: bool,
}

#[derive(Serialize, Deserialize)]
pub struct ColliderMaterial {
    pub restitution: f32,
    pub friction: f32,
    pub density: f32,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct ExternalForce {
    pub body_handle: u32,
    pub force: [f32; 3],
    pub torque: [f32; 3],
    pub application_point: Option<[f32; 3]>,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct SceneState {
    pub timestep: f32,
    pub global_time: f64,
    pub body_states: Vec<RigidBodyState>,
    pub energies: KineticEnergies,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct RigidBodyState {
    pub handle: u32,
    pub position: [f32; 3],
    pub rotation: [f32; 4],
    pub linear_velocity: [f32; 3],
    pub angular_velocity: [f32; 3],
    pub kinetic_energy: f32,
    pub potential_energy: f32,
    pub sleeping: bool,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct KineticEnergies {
    pub total_kinetic: f64,
    pub total_potential: f64,
    pub total_mechanical: f64,
    pub dissipated: f64,
    pub external_work: f64,
    pub quantum_work: f64,
    pub continuum_work: f64,
}

#[derive(Serialize, Deserialize)]
pub struct RayCastResult {
    pub hit: bool,
    pub body_handle: Option<u32>,
    pub point: [f32; 3],
    pub normal: [f32; 3],
    pub distance: f32,
}

// ----------------------------------------------------------------------------
// ENHANCED KINETIC PROXY
// ----------------------------------------------------------------------------

pub struct KineticProxy {
    rigid_body_set: RigidBodySet,
    collider_set: ColliderSet,
    impulse_joint_set: ImpulseJointSet,
    multibody_joint_set: MultibodyJointSet,

    physics_pipeline: PhysicsPipeline,
    query_pipeline: QueryPipeline,
    island_manager: IslandManager,
    broad_phase: BroadPhase,
    narrow_phase: NarrowPhase,
    ccd_solver: CCDSolver,

    integration_parameters: IntegrationParameters,
    gravity: Vector3<f32>,

    scene_state: SceneState,
    body_definition_map: HashMap<u32, RigidBodyDefinition>,

    cache: Rc<RefCell<ComputationCache>>,
    telemetry: Rc<RefCell<Telemetry>>,

    sab: Option<std::sync::Arc<sdk::sab::SafeSAB>>,

    state_history: VecDeque<SceneState>,
    steps_executed: u64,
    bodies_created: u32,
}

impl Default for KineticProxy {
    fn default() -> Self {
        Self::new(
            Rc::new(RefCell::new(ComputationCache::new())),
            Rc::new(RefCell::new(Telemetry::default())),
        )
    }
}

impl KineticProxy {
    pub fn new(cache: Rc<RefCell<ComputationCache>>, telemetry: Rc<RefCell<Telemetry>>) -> Self {
        let integration_params = IntegrationParameters {
            dt: 1.0 / 60.0,
            ..IntegrationParameters::default()
        };

        Self {
            rigid_body_set: RigidBodySet::new(),
            collider_set: ColliderSet::new(),
            impulse_joint_set: ImpulseJointSet::new(),
            multibody_joint_set: MultibodyJointSet::new(),

            physics_pipeline: PhysicsPipeline::new(),
            query_pipeline: QueryPipeline::new(),
            island_manager: IslandManager::new(),
            broad_phase: BroadPhase::new(),
            narrow_phase: NarrowPhase::new(),
            ccd_solver: CCDSolver::new(),

            integration_parameters: integration_params,
            gravity: Vector3::new(0.0, -9.81, 0.0),

            scene_state: SceneState {
                timestep: 1.0 / 60.0,
                global_time: 0.0,
                body_states: Vec::new(),
                energies: KineticEnergies {
                    total_kinetic: 0.0,
                    total_potential: 0.0,
                    total_mechanical: 0.0,
                    dissipated: 0.0,
                    external_work: 0.0,
                    quantum_work: 0.0,
                    continuum_work: 0.0,
                },
            },
            body_definition_map: HashMap::new(),
            cache,
            telemetry,
            sab: None,
            state_history: VecDeque::new(),
            steps_executed: 0,
            bodies_created: 0,
        }
    }

    pub fn set_sab(&mut self, sab: std::sync::Arc<sdk::sab::SafeSAB>) {
        self.sab = Some(sab);
    }

    fn execute_create_body(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let body_def: RigidBodyDefinition =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let params_str = std::str::from_utf8(params).unwrap_or("");
        let body_hash = self.compute_body_hash(&body_def);
        let cache_key = format!("body_create_{}", params_str);
        if let Some(entry) = self.cache.borrow_mut().get(&cache_key, None) {
            self.telemetry.borrow_mut().cache_hits += 1;
            return Ok(entry.data);
        }

        let mut rb_builder = match body_def.body_type {
            BodyType::Static => RigidBodyBuilder::fixed(),
            BodyType::Dynamic | BodyType::QuantumCoupled | BodyType::ContinuumCoupled => {
                RigidBodyBuilder::dynamic()
            }
            BodyType::Kinematic => RigidBodyBuilder::kinematic_position_based(),
            BodyType::Sensor => RigidBodyBuilder::dynamic(),
        };

        rb_builder = rb_builder.translation(vector![
            body_def.position[0],
            body_def.position[1],
            body_def.position[2]
        ]);

        // Set initial rotation via angular velocity (rapier API)
        // For initial orientation, we'll set it after creation
        let _rigid_body = rb_builder.build();

        rb_builder = rb_builder
            .linvel(vector![
                body_def.linear_velocity[0],
                body_def.linear_velocity[1],
                body_def.linear_velocity[2]
            ])
            .angvel(vector![
                body_def.angular_velocity[0],
                body_def.angular_velocity[1],
                body_def.angular_velocity[2]
            ]);

        if body_def.mass > 0.0 {
            rb_builder = rb_builder.additional_mass(body_def.mass);
        }

        rb_builder = rb_builder
            .linear_damping(body_def.linear_damping)
            .angular_damping(body_def.angular_damping)
            .gravity_scale(body_def.gravity_scale);

        let rigid_body = rb_builder.build();
        let body_handle = self.rigid_body_set.insert(rigid_body);

        let handle_idx = body_handle.into_raw_parts().0;
        self.body_definition_map.insert(handle_idx, body_def);
        self.bodies_created += 1;

        let result_tuple = (handle_idx, body_hash);
        let serialized_result =
            bincode::serialize(&result_tuple).map_err(|e| ScienceError::Internal(e.to_string()))?;

        let entry = CacheEntry {
            data: serialized_result.clone(),
            result_hash: cache_key.clone(),
            timestamp: 0,
            access_count: 1,
            scale: Default::default(),
            proof: Default::default(),
        };
        self.cache.borrow_mut().put(cache_key, entry);
        self.telemetry.borrow_mut().computations += 1;

        Ok(serialized_result)
    }

    fn execute_create_collider(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let collider_def: ColliderDefinition =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let body_handle_idx = params_json["body_handle"]
            .as_u64()
            .ok_or_else(|| ScienceError::InvalidParams("Missing body_handle".to_string()))?
            as u32;

        let body_handle = RigidBodyHandle::from_raw_parts(body_handle_idx, 0);

        let shape = match collider_def.shape {
            ColliderShape::Sphere { radius } => SharedShape::ball(radius),
            ColliderShape::Box { half_extents } => {
                SharedShape::cuboid(half_extents[0], half_extents[1], half_extents[2])
            }
            ColliderShape::Capsule {
                half_height,
                radius,
            } => SharedShape::capsule_y(half_height, radius),
            ColliderShape::Cylinder {
                half_height,
                radius,
            } => SharedShape::cylinder(half_height, radius),
        };

        let _quat = UnitQuaternion::from_quaternion(Quaternion::new(
            collider_def.rotation[3],
            collider_def.rotation[0],
            collider_def.rotation[1],
            collider_def.rotation[2],
        ));

        let collider = ColliderBuilder::new(shape)
            .translation(vector![
                collider_def.translation[0],
                collider_def.translation[1],
                collider_def.translation[2]
            ])
            .restitution(collider_def.material.restitution)
            .friction(collider_def.material.friction)
            .density(collider_def.material.density)
            .sensor(collider_def.sensor)
            .build();

        let collider_handle =
            self.collider_set
                .insert_with_parent(collider, body_handle, &mut self.rigid_body_set);

        let (idx, gen) = collider_handle.into_raw_parts();
        bincode::serialize(&(idx, gen)).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    fn execute_step(&mut self, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let dt = params_json["dt"].as_f64().unwrap_or(1.0 / 60.0) as f32;
        let substeps = params_json["substeps"].as_u64().unwrap_or(1) as usize;

        self.integration_parameters.dt = dt;

        let external_forces: Vec<ExternalForce> = bincode::deserialize(input).unwrap_or_default();

        self.apply_external_forces(&external_forces);

        for _ in 0..substeps {
            self.physics_pipeline.step(
                &self.gravity,
                &self.integration_parameters,
                &mut self.island_manager,
                &mut self.broad_phase,
                &mut self.narrow_phase,
                &mut self.rigid_body_set,
                &mut self.collider_set,
                &mut self.impulse_joint_set,
                &mut self.multibody_joint_set,
                &mut self.ccd_solver,
                Some(&mut self.query_pipeline),
                &(),
                &(),
            );
        }

        self.update_scene_state(dt * substeps as f32);
        self.steps_executed += 1;

        bincode::serialize(&self.scene_state).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    fn apply_external_forces(&mut self, forces: &[ExternalForce]) {
        for force in forces {
            let body_handle = RigidBodyHandle::from_raw_parts(force.body_handle, 0);

            if let Some(body) = self.rigid_body_set.get_mut(body_handle) {
                body.add_force(
                    vector![force.force[0], force.force[1], force.force[2]],
                    true,
                );
                body.add_torque(
                    vector![force.torque[0], force.torque[1], force.torque[2]],
                    true,
                );

                if let Some(point) = force.application_point {
                    let _world_point = body.position() * point![point[0], point[1], point[2]];
                    let force_vec = vector![force.force[0], force.force[1], force.force[2]];
                    // Apply force (rapier doesn't have apply_force_at_point in this version)
                    body.add_force(force_vec, true);
                }
            }
        }
    }

    fn execute_cast_ray(&mut self, _input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        let params_str = std::str::from_utf8(params)
            .map_err(|_| ScienceError::InvalidParams("Invalid UTF-8 params".to_string()))?;
        let params_json: serde_json::Value = serde_json::from_str(params_str)
            .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let origin = [
            params_json["origin_x"].as_f64().unwrap_or(0.0) as f32,
            params_json["origin_y"].as_f64().unwrap_or(0.0) as f32,
            params_json["origin_z"].as_f64().unwrap_or(0.0) as f32,
        ];

        let direction = [
            params_json["dir_x"].as_f64().unwrap_or(0.0) as f32,
            params_json["dir_y"].as_f64().unwrap_or(0.0) as f32,
            params_json["dir_z"].as_f64().unwrap_or(1.0) as f32,
        ];

        let max_toi = params_json["max_toi"].as_f64().unwrap_or(1000.0) as f32;

        let ray = Ray::new(
            point![origin[0], origin[1], origin[2]],
            vector![direction[0], direction[1], direction[2]].normalize(),
        );

        let hit = self.query_pipeline.cast_ray(
            &self.rigid_body_set,
            &self.collider_set,
            &ray,
            max_toi,
            true,
            QueryFilter::default(),
        );

        let result = match hit {
            Some((handle, toi)) => {
                let body_handle = self
                    .collider_set
                    .get(handle)
                    .and_then(|collider| collider.parent())
                    .map(|h| h.into_raw_parts().0);

                let point = ray.point_at(toi);

                RayCastResult {
                    hit: true,
                    body_handle,
                    point: [point.x, point.y, point.z],
                    normal: [0.0, 1.0, 0.0],
                    distance: toi,
                }
            }
            None => RayCastResult {
                hit: false,
                body_handle: None,
                point: [0.0; 3],
                normal: [0.0; 3],
                distance: 0.0,
            },
        };

        bincode::serialize(&result).map_err(|e| ScienceError::Internal(e.to_string()))
    }

    fn execute_add_force(&mut self, input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        let force: ExternalForce =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        self.apply_external_forces(std::slice::from_ref(&force));

        let body_handle = RigidBodyHandle::from_raw_parts(force.body_handle, 0);
        if let Some(body) = self.rigid_body_set.get(body_handle) {
            let state = self.body_to_state(body_handle, body);
            bincode::serialize(&state).map_err(|e| ScienceError::Internal(e.to_string()))
        } else {
            Err(ScienceError::Internal("Body not found".to_string()))
        }
    }

    fn execute_add_impulse(&mut self, input: &[u8], _params: &[u8]) -> ScienceResult<Vec<u8>> {
        let impulse: ExternalForce =
            bincode::deserialize(input).map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

        let body_handle = RigidBodyHandle::from_raw_parts(impulse.body_handle, 0);

        // Apply impulse
        if let Some(body) = self.rigid_body_set.get_mut(body_handle) {
            body.apply_impulse(
                vector![impulse.force[0], impulse.force[1], impulse.force[2]],
                true,
            );
            body.apply_torque_impulse(
                vector![impulse.torque[0], impulse.torque[1], impulse.torque[2]],
                true,
            );
        }

        // Get state after applying impulse (separate borrow)
        if let Some(body) = self.rigid_body_set.get(body_handle) {
            let state = self.body_to_state(body_handle, body);
            bincode::serialize(&state).map_err(|e| ScienceError::Internal(e.to_string()))
        } else {
            Err(ScienceError::Internal("Body not found".to_string()))
        }
    }

    fn execute_step_particles(&mut self, _input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        use crate::science_capnp::particle_reality_params;
        use capnp::message::ReaderOptions;
        use capnp::serialize;

        // 1. Decode Params
        let reader = serialize::read_message(&mut &params[..], ReaderOptions::new())
            .map_err(|e| ScienceError::InvalidParams(format!("Capnp decode failed: {}", e)))?;
        let params_reader = reader
            .get_root::<particle_reality_params::Reader>()
            .map_err(|e: capnp::Error| ScienceError::InvalidParams(e.to_string()))?;

        let offset = params_reader.get_sab_offset() as usize;
        let count = params_reader.get_particle_count() as usize;
        let dt = params_reader.get_dt();

        let sab = self
            .sab
            .as_ref()
            .ok_or_else(|| ScienceError::Internal("SAB not initialized".to_string()))?;

        // 2. Perform SAB-native update
        // Each particle: [x, y, z, vx, vy, vz, mass] = 7 * 4 = 28 bytes
        let stride = 7;
        let byte_len = count * stride * 4;
        let mut data = sab
            .read(offset, byte_len)
            .map_err(|e| ScienceError::Internal(e))?;

        // Convert to f32 slice
        let particles = unsafe {
            std::slice::from_raw_parts_mut(data.as_mut_ptr() as *mut f32, count * stride)
        };

        for i in 0..count {
            let idx = i * stride;

            // Gravity (Shared Reality)
            particles[idx + 4] += -9.81 * dt; // Vy

            // Euler Integration
            particles[idx + 0] += particles[idx + 3] * dt; // x
            particles[idx + 1] += particles[idx + 4] * dt; // y
            particles[idx + 2] += particles[idx + 5] * dt; // z

            // Simple damping
            particles[idx + 3] *= 0.99;
            particles[idx + 4] *= 0.99;
            particles[idx + 5] *= 0.99;

            // Boundary check (Big Bang floor)
            if particles[idx + 1] < -50.0 {
                particles[idx + 1] = -50.0;
                particles[idx + 4] *= -0.5; // Bounce
            }
        }

        // 3. Write back to SAB
        sab.write(offset, &data)
            .map_err(|e| ScienceError::Internal(e))?;

        Ok(vec![])
    }

    fn update_scene_state(&mut self, dt: f32) {
        self.scene_state.timestep = dt;
        self.scene_state.global_time += dt as f64;
        self.scene_state.body_states.clear();

        for (handle, body) in self.rigid_body_set.iter() {
            let state = self.body_to_state(handle, body);
            self.scene_state.body_states.push(state);
        }

        self.scene_state.energies = self.compute_all_energies();

        self.state_history.push_back(self.scene_state.clone());
        if self.state_history.len() > 1000 {
            self.state_history.pop_front();
        }
    }

    fn body_to_state(&self, handle: RigidBodyHandle, body: &RigidBody) -> RigidBodyState {
        let position = body.translation();
        let rotation = body.rotation();

        RigidBodyState {
            handle: handle.into_raw_parts().0,
            position: [position.x, position.y, position.z],
            rotation: [rotation.i, rotation.j, rotation.k, rotation.w],
            linear_velocity: [body.linvel().x, body.linvel().y, body.linvel().z],
            angular_velocity: [body.angvel().x, body.angvel().y, body.angvel().z],
            kinetic_energy: 0.5 * body.mass() * body.linvel().norm_squared(),
            potential_energy: body.mass() * (-self.gravity.y) * position.y,
            sleeping: body.is_sleeping(),
        }
    }

    fn compute_all_energies(&self) -> KineticEnergies {
        let mut energies = KineticEnergies {
            total_kinetic: 0.0,
            total_potential: 0.0,
            total_mechanical: 0.0,
            dissipated: 0.0,
            external_work: 0.0,
            quantum_work: 0.0,
            continuum_work: 0.0,
        };

        for (_, body) in self.rigid_body_set.iter() {
            let ke = 0.5 * body.mass() as f64 * body.linvel().norm_squared() as f64;
            let pe = body.mass() as f64 * (-self.gravity.y) as f64 * body.translation().y as f64;

            energies.total_kinetic += ke;
            energies.total_potential += pe;
        }

        energies.total_mechanical = energies.total_kinetic + energies.total_potential;
        energies
    }

    fn compute_body_hash(&self, body_def: &RigidBodyDefinition) -> String {
        let mut hasher = blake3::Hasher::new();
        let bytes = bincode::serialize(body_def).unwrap_or_default();
        hasher.update(&bytes);
        hasher.finalize().to_hex().to_string()
    }
}

impl ScienceProxy for KineticProxy {
    fn name(&self) -> &'static str {
        "kinetic"
    }

    fn methods(&self) -> Vec<&'static str> {
        vec![
            "create_body",
            "create_collider",
            "step",
            "cast_ray",
            "add_force",
            "add_impulse",
            "get_state",
        ]
    }

    fn execute(&mut self, method: &str, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>> {
        match method {
            "create_body" => self.execute_create_body(input, params),
            "create_collider" => self.execute_create_collider(input, params),
            "step" => self.execute_step(input, params),
            "step_particles" => self.execute_step_particles(input, params),
            "cast_ray" => self.execute_cast_ray(input, params),
            "add_force" => self.execute_add_force(input, params),
            "add_impulse" => self.execute_add_impulse(input, params),
            "get_state" => bincode::serialize(&self.scene_state)
                .map_err(|e| ScienceError::Internal(e.to_string())),
            _ => Err(ScienceError::MethodNotFound(method.to_string())),
        }
    }

    fn validate_spot(
        &mut self,
        method: &str,
        input: &[u8],
        params: &[u8],
        spot_seed: &[u8],
    ) -> ScienceResult<Vec<u8>> {
        match method {
            "step" => {
                let previous_energy = self.scene_state.energies.total_mechanical;
                let _result = self.execute_step(input, params)?;
                let energy_change =
                    (self.scene_state.energies.total_mechanical - previous_energy).abs();

                let validation_data = format!("energy_change={}", energy_change);
                Ok(validation_data.into_bytes())
            }

            "create_body" => {
                let body_def: RigidBodyDefinition = bincode::deserialize(input)
                    .map_err(|e| ScienceError::InvalidParams(e.to_string()))?;

                let mut hasher = blake3::Hasher::new();
                hasher.update(spot_seed);
                hasher.update(&bincode::serialize(&body_def).unwrap_or_default());
                let hash = hasher.finalize();

                let handle = u64::from_le_bytes(hash.as_bytes()[0..8].try_into().unwrap()) as u32;
                Ok(handle.to_le_bytes().to_vec())
            }

            _ => self.execute(method, input, params),
        }
    }
}
