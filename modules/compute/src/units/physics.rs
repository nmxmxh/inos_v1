use crate::engine::{ComputeError, ResourceLimits, UnitProxy};
use async_trait::async_trait;
use serde_json::Value as JsonValue;

/// Physics simulation via Rapier3D library proxy
///
/// Architecture: Rust validates + prepares, delegates to Rapier3D library
/// - Lightweight: Validation and parameter marshalling in WASM
/// - Secure: Input validation and resource limits in Rust
/// - Fast: Full physics performance via native Rapier3D
/// - Clean: Separation of concerns (validation vs execution)
///
/// Rapier3D Methods Supported:
/// - Rigid Bodies: create, update, remove, get_state
/// - Colliders: create, attach, remove, set_properties
/// - Joints: create_fixed, create_revolute, create_prismatic, create_spherical
/// - Forces: apply_force, apply_impulse, apply_torque, set_velocity
/// - Queries: raycast, shape_cast, intersection_test, contact_query
/// - Simulation: step, set_gravity, get_contacts, get_islands
pub struct PhysicsEngine {
    config: PhysicsConfig,
}

#[derive(Clone)]
struct PhysicsConfig {
    max_raycast_distance: f32,
    max_simulation_steps: u32,
}

impl Default for PhysicsConfig {
    fn default() -> Self {
        Self {
            max_raycast_distance: 1000.0,
            max_simulation_steps: 100,
        }
    }
}

impl PhysicsEngine {
    pub fn new() -> Self {
        log::info!("Physics engine initialized (Rapier3D library proxy)");
        Self {
            config: PhysicsConfig::default(),
        }
    }

    /// Validate rigid body parameters
    fn validate_rigid_body_params(&self, params: &JsonValue) -> Result<(), ComputeError> {
        let body_type = params
            .get("body_type")
            .and_then(|v| v.as_str())
            .ok_or_else(|| ComputeError::InvalidParams("Missing body_type".to_string()))?;

        if !["dynamic", "static", "kinematic", "fixed"].contains(&body_type) {
            return Err(ComputeError::InvalidParams(format!(
                "Invalid body_type: {}. Must be dynamic, static, kinematic, or fixed",
                body_type
            )));
        }

        // Validate position if provided
        if let Some(pos) = params.get("position") {
            self.validate_vector3(pos, "position")?;
        }

        // Validate rotation if provided (quaternion)
        if let Some(rot) = params.get("rotation") {
            self.validate_quaternion(rot)?;
        }

        Ok(())
    }

    /// Validate collider parameters
    fn validate_collider_params(&self, params: &JsonValue) -> Result<(), ComputeError> {
        let shape = params
            .get("shape")
            .and_then(|v| v.as_str())
            .ok_or_else(|| ComputeError::InvalidParams("Missing shape".to_string()))?;

        match shape {
            "box" | "cuboid" => {
                params.get("half_extents").ok_or_else(|| {
                    ComputeError::InvalidParams("Box requires half_extents".to_string())
                })?;
            }
            "sphere" | "ball" => {
                params
                    .get("radius")
                    .and_then(|v| v.as_f64())
                    .ok_or_else(|| {
                        ComputeError::InvalidParams("Sphere requires radius (number)".to_string())
                    })?;
            }
            "capsule" => {
                params.get("half_height").ok_or_else(|| {
                    ComputeError::InvalidParams("Capsule requires half_height".to_string())
                })?;
                params.get("radius").ok_or_else(|| {
                    ComputeError::InvalidParams("Capsule requires radius".to_string())
                })?;
            }
            "cylinder" => {
                params.get("half_height").ok_or_else(|| {
                    ComputeError::InvalidParams("Cylinder requires half_height".to_string())
                })?;
                params.get("radius").ok_or_else(|| {
                    ComputeError::InvalidParams("Cylinder requires radius".to_string())
                })?;
            }
            "cone" => {
                params.get("half_height").ok_or_else(|| {
                    ComputeError::InvalidParams("Cone requires half_height".to_string())
                })?;
                params.get("radius").ok_or_else(|| {
                    ComputeError::InvalidParams("Cone requires radius".to_string())
                })?;
            }
            "trimesh" | "heightfield" | "convex_hull" => {
                // Complex shapes - just verify they have data
                params.get("vertices").ok_or_else(|| {
                    ComputeError::InvalidParams(format!("{} requires vertices", shape))
                })?;
            }
            _ => {
                return Err(ComputeError::InvalidParams(format!(
                    "Invalid shape: {}. Supported: box, sphere, capsule, cylinder, cone, trimesh, heightfield, convex_hull",
                    shape
                )));
            }
        }

        Ok(())
    }

    /// Validate joint parameters
    fn validate_joint_params(&self, params: &JsonValue) -> Result<(), ComputeError> {
        params
            .get("body1")
            .ok_or_else(|| ComputeError::InvalidParams("Missing body1".to_string()))?;
        params
            .get("body2")
            .ok_or_else(|| ComputeError::InvalidParams("Missing body2".to_string()))?;

        Ok(())
    }

    /// Validate raycast parameters
    fn validate_raycast_params(&self, params: &JsonValue) -> Result<(), ComputeError> {
        let origin = params
            .get("origin")
            .ok_or_else(|| ComputeError::InvalidParams("Missing origin".to_string()))?;

        let direction = params
            .get("direction")
            .ok_or_else(|| ComputeError::InvalidParams("Missing direction".to_string()))?;

        self.validate_vector3(origin, "origin")?;
        self.validate_vector3(direction, "direction")?;

        // Validate max_distance
        if let Some(max_dist) = params.get("max_distance").and_then(|v| v.as_f64()) {
            if max_dist > self.config.max_raycast_distance as f64 {
                return Err(ComputeError::InvalidParams(format!(
                    "max_distance {} exceeds limit {}",
                    max_dist, self.config.max_raycast_distance
                )));
            }
            if max_dist <= 0.0 {
                return Err(ComputeError::InvalidParams(
                    "max_distance must be positive".to_string(),
                ));
            }
        }

        Ok(())
    }

    /// Validate force/impulse parameters
    fn validate_force_params(&self, params: &JsonValue) -> Result<(), ComputeError> {
        params
            .get("body_id")
            .ok_or_else(|| ComputeError::InvalidParams("Missing body_id".to_string()))?;

        let force = params
            .get("force")
            .ok_or_else(|| ComputeError::InvalidParams("Missing force vector".to_string()))?;

        self.validate_vector3(force, "force")?;

        Ok(())
    }

    /// Validate Vector3 structure
    fn validate_vector3(&self, vec: &JsonValue, name: &str) -> Result<(), ComputeError> {
        if !vec.is_object() {
            return Err(ComputeError::InvalidParams(format!(
                "{} must be an object with x, y, z fields",
                name
            )));
        }

        for axis in ["x", "y", "z"] {
            vec.get(axis).and_then(|v| v.as_f64()).ok_or_else(|| {
                ComputeError::InvalidParams(format!("{}.{} must be a number", name, axis))
            })?;
        }

        Ok(())
    }

    /// Validate Quaternion structure
    fn validate_quaternion(&self, quat: &JsonValue) -> Result<(), ComputeError> {
        if !quat.is_object() {
            return Err(ComputeError::InvalidParams(
                "rotation must be an object with x, y, z, w fields".to_string(),
            ));
        }

        for component in ["x", "y", "z", "w"] {
            quat.get(component)
                .and_then(|v| v.as_f64())
                .ok_or_else(|| {
                    ComputeError::InvalidParams(format!("rotation.{} must be a number", component))
                })?;
        }

        Ok(())
    }

    /// Create library proxy response
    fn proxy_response(&self, method: &str, params: JsonValue) -> Result<Vec<u8>, ComputeError> {
        let response = serde_json::json!({
            "library": "rapier3d",
            "method": method,
            "params": params
        });

        serde_json::to_vec(&response).map_err(|e| ComputeError::ExecutionFailed(e.to_string()))
    }
}

impl Default for PhysicsEngine {
    fn default() -> Self {
        Self::new()
    }
}

#[async_trait(?Send)]
impl UnitProxy for PhysicsEngine {
    fn service_name(&self) -> &str {
        "physics"
    }

    fn actions(&self) -> Vec<&str> {
        vec![
            // Rigid Body Management
            "create_rigid_body",
            "remove_rigid_body",
            "get_rigid_body_state",
            "set_rigid_body_position",
            "set_rigid_body_rotation",
            "set_rigid_body_velocity",
            "set_rigid_body_angular_velocity",
            // Collider Management
            "create_collider",
            "attach_collider",
            "remove_collider",
            "set_collider_friction",
            "set_collider_restitution",
            "set_collider_density",
            // Joint Management
            "create_fixed_joint",
            "create_revolute_joint",
            "create_prismatic_joint",
            "create_spherical_joint",
            "remove_joint",
            "set_joint_limits",
            // Forces and Impulses
            "apply_force",
            "apply_impulse",
            "apply_torque",
            "apply_force_at_point",
            "apply_impulse_at_point",
            // Queries
            "raycast",
            "raycast_all",
            "shape_cast",
            "intersection_test",
            "contact_query",
            "proximity_query",
            // Simulation Control
            "step_simulation",
            "set_gravity",
            "get_contacts",
            "get_collision_events",
            "get_islands",
            "reset_simulation",
        ]
    }

    fn resource_limits(&self) -> ResourceLimits {
        ResourceLimits {
            max_input_size: 10 * 1024 * 1024,  // 10MB for mesh data
            max_output_size: 50 * 1024 * 1024, // 50MB for simulation state
            max_memory_pages: 2048,            // 128MB
            timeout_ms: 10000,                 // 10s for complex simulations
            max_fuel: 50_000_000_000,          // 50B instructions
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
            // Rigid Body Methods
            "create_rigid_body" => {
                self.validate_rigid_body_params(&params)?;
                self.proxy_response(method, params)
            }

            "set_rigid_body_position" | "set_rigid_body_rotation" => {
                params
                    .get("body_id")
                    .ok_or_else(|| ComputeError::InvalidParams("Missing body_id".to_string()))?;
                self.proxy_response(method, params)
            }

            "set_rigid_body_velocity" | "set_rigid_body_angular_velocity" => {
                params
                    .get("body_id")
                    .ok_or_else(|| ComputeError::InvalidParams("Missing body_id".to_string()))?;
                self.proxy_response(method, params)
            }

            // Collider Methods
            "create_collider" | "attach_collider" => {
                self.validate_collider_params(&params)?;
                self.proxy_response(method, params)
            }

            "set_collider_friction" | "set_collider_restitution" | "set_collider_density" => {
                params.get("collider_id").ok_or_else(|| {
                    ComputeError::InvalidParams("Missing collider_id".to_string())
                })?;
                self.proxy_response(method, params)
            }

            // Joint Methods
            "create_fixed_joint"
            | "create_revolute_joint"
            | "create_prismatic_joint"
            | "create_spherical_joint" => {
                self.validate_joint_params(&params)?;
                self.proxy_response(method, params)
            }

            // Force Methods
            "apply_force"
            | "apply_impulse"
            | "apply_torque"
            | "apply_force_at_point"
            | "apply_impulse_at_point" => {
                self.validate_force_params(&params)?;
                self.proxy_response(method, params)
            }

            // Query Methods
            "raycast" | "raycast_all" => {
                self.validate_raycast_params(&params)?;
                self.proxy_response(method, params)
            }

            "shape_cast" | "intersection_test" | "contact_query" | "proximity_query" => {
                // Basic validation - these have complex params
                if !params.is_object() {
                    return Err(ComputeError::InvalidParams(
                        "Params must be an object".to_string(),
                    ));
                }
                self.proxy_response(method, params)
            }

            // Simulation Methods
            "step_simulation" => {
                if let Some(steps) = params.get("steps").and_then(|v| v.as_u64()) {
                    if steps > self.config.max_simulation_steps as u64 {
                        return Err(ComputeError::InvalidParams(format!(
                            "steps {} exceeds limit {}",
                            steps, self.config.max_simulation_steps
                        )));
                    }
                }
                self.proxy_response(method, params)
            }

            "set_gravity" => {
                let gravity = params.get("gravity").ok_or_else(|| {
                    ComputeError::InvalidParams("Missing gravity vector".to_string())
                })?;
                self.validate_vector3(gravity, "gravity")?;
                self.proxy_response(method, params)
            }

            "get_contacts"
            | "get_collision_events"
            | "get_islands"
            | "reset_simulation"
            | "remove_rigid_body"
            | "remove_collider"
            | "remove_joint"
            | "get_rigid_body_state"
            | "set_joint_limits" => self.proxy_response(method, params),

            _ => Err(ComputeError::UnknownMethod {
                library: "physics".to_string(),
                method: method.to_string(),
            }),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_create_rigid_body() {
        let engine = PhysicsEngine::new();
        let params = serde_json::json!({
            "body_type": "dynamic",
            "position": {"x": 0.0, "y": 5.0, "z": 0.0}
        });

        let result = engine
            .execute(
                "create_rigid_body",
                &[],
                serde_json::to_vec(&params).unwrap().as_slice(),
            )
            .await;

        assert!(result.is_ok());
        let response: JsonValue = serde_json::from_slice(&result.unwrap()).unwrap();
        assert_eq!(response["library"], "rapier3d");
        assert_eq!(response["method"], "create_rigid_body");
    }

    #[tokio::test]
    async fn test_create_collider_box() {
        let engine = PhysicsEngine::new();
        let params = serde_json::json!({
            "shape": "box",
            "half_extents": {"x": 1.0, "y": 1.0, "z": 1.0},
            "friction": 0.5,
            "restitution": 0.3
        });

        let result = engine
            .execute(
                "create_collider",
                &[],
                serde_json::to_vec(&params).unwrap().as_slice(),
            )
            .await;

        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn test_create_collider_sphere() {
        let engine = PhysicsEngine::new();
        let params = serde_json::json!({
            "shape": "sphere",
            "radius": 1.5
        });

        let result = engine
            .execute(
                "create_collider",
                &[],
                serde_json::to_vec(&params).unwrap().as_slice(),
            )
            .await;

        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn test_raycast() {
        let engine = PhysicsEngine::new();
        let params = serde_json::json!({
            "origin": {"x": 0.0, "y": 0.0, "z": 0.0},
            "direction": {"x": 1.0, "y": 0.0, "z": 0.0},
            "max_distance": 100.0
        });

        let result = engine
            .execute(
                "raycast",
                &[],
                serde_json::to_vec(&params).unwrap().as_slice(),
            )
            .await;

        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn test_apply_force() {
        let engine = PhysicsEngine::new();
        let params = serde_json::json!({
            "body_id": 1,
            "force": {"x": 10.0, "y": 0.0, "z": 0.0}
        });

        let result = engine
            .execute(
                "apply_force",
                &[],
                serde_json::to_vec(&params).unwrap().as_slice(),
            )
            .await;

        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn test_create_joint() {
        let engine = PhysicsEngine::new();
        let params = serde_json::json!({
            "body1": 1,
            "body2": 2,
            "anchor": {"x": 0.0, "y": 0.0, "z": 0.0}
        });

        let result = engine
            .execute(
                "create_fixed_joint",
                &[],
                serde_json::to_vec(&params).unwrap().as_slice(),
            )
            .await;

        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn test_step_simulation() {
        let engine = PhysicsEngine::new();
        let params = serde_json::json!({
            "steps": 10,
            "dt": 0.016
        });

        let result = engine
            .execute(
                "step_simulation",
                &[],
                serde_json::to_vec(&params).unwrap().as_slice(),
            )
            .await;

        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn test_set_gravity() {
        let engine = PhysicsEngine::new();
        let params = serde_json::json!({
            "gravity": {"x": 0.0, "y": -9.81, "z": 0.0}
        });

        let result = engine
            .execute(
                "set_gravity",
                &[],
                serde_json::to_vec(&params).unwrap().as_slice(),
            )
            .await;

        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn test_invalid_body_type() {
        let engine = PhysicsEngine::new();
        let params = serde_json::json!({
            "body_type": "invalid",
            "position": {"x": 0.0, "y": 0.0, "z": 0.0}
        });

        let result = engine
            .execute(
                "create_rigid_body",
                &[],
                serde_json::to_vec(&params).unwrap().as_slice(),
            )
            .await;

        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_invalid_shape() {
        let engine = PhysicsEngine::new();
        let params = serde_json::json!({
            "shape": "triangle"
        });

        let result = engine
            .execute(
                "create_collider",
                &[],
                serde_json::to_vec(&params).unwrap().as_slice(),
            )
            .await;

        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_raycast_distance_limit() {
        let engine = PhysicsEngine::new();
        let params = serde_json::json!({
            "origin": {"x": 0.0, "y": 0.0, "z": 0.0},
            "direction": {"x": 1.0, "y": 0.0, "z": 0.0},
            "max_distance": 2000.0  // Exceeds limit of 1000.0
        });

        let result = engine
            .execute(
                "raycast",
                &[],
                serde_json::to_vec(&params).unwrap().as_slice(),
            )
            .await;

        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_actions_list() {
        let engine = PhysicsEngine::new();
        let actions = engine.actions();

        assert!(actions.contains(&"create_rigid_body"));
        assert!(actions.contains(&"create_collider"));
        assert!(actions.contains(&"raycast"));
        assert!(actions.contains(&"apply_force"));
        assert!(actions.contains(&"step_simulation"));
        assert!(actions.len() > 30); // Should have many methods
    }
}
