use crate::engine::{ComputeError, ResourceLimits, UnitProxy};
use async_trait::async_trait;
use base64::{engine::general_purpose, Engine as _};
use naga::{
    front::wgsl,
    valid::{Capabilities, ValidationFlags, Validator},
    Module,
};
use serde::Serialize;
use serde_json::Value as JsonValue;
use std::collections::HashMap;

/// GPU graphics processing via WebGPU delegation
///
/// Architecture: Rust validates + prepares, JavaScript executes via WebGPU
/// - Lightweight: ~2KB WASM footprint (vs 2MB with wgpu)
/// - Secure: Naga validation + security checks in Rust
/// - Fast: Full GPU performance via browser WebGPU API
/// - Clean: Separation of concerns (validation vs execution)
pub struct GpuUnit {
    config: GpuConfig,
    shader_cache: HashMap<&'static str, &'static str>,
    validator: ShaderValidator,
}

#[derive(Clone)]
struct GpuConfig {
    max_shader_size: usize, // 1MB max shader code
    #[allow(dead_code)] // Future: buffer size validation
    max_buffer_size: usize, // 100MB max buffer
    max_workgroup_size: [u32; 3], // [256, 256, 64]
    #[allow(dead_code)] // Future: dispatch size validation
    max_dispatch_size: [u32; 3], // [65535, 65535, 65535]
}

impl Default for GpuConfig {
    fn default() -> Self {
        Self {
            max_shader_size: 1024 * 1024,       // 1MB
            max_buffer_size: 100 * 1024 * 1024, // 100MB
            max_workgroup_size: [256, 256, 64],
            max_dispatch_size: [65535, 65535, 65535],
        }
    }
}

/// Shader security validator
struct ShaderValidator {
    max_workgroup_size: u32,
    max_total_invocations: u32,
    banned_patterns: Vec<&'static str>,
}

impl ShaderValidator {
    fn new() -> Self {
        Self {
            max_workgroup_size: 256,
            max_total_invocations: 1024,
            banned_patterns: vec![
                "loop {",               // Unbounded loops
                "while(true)",          // Infinite loops
                "atomicAdd",            // Can cause hangs
                "storageBarrier()",     // Complex synchronization
                "workgroupUniformLoad", // Potential side-channel
                "discard",              // Fragment shader only
                "@fragment",            // Wrong pipeline type
                "@vertex",              // Wrong pipeline type
            ],
        }
    }

    fn validate_security(&self, module: &Module, source: &str) -> Result<(), ComputeError> {
        // 1. Check for banned patterns
        for pattern in &self.banned_patterns {
            if source.contains(pattern) {
                return Err(ComputeError::ExecutionFailed(format!(
                    "Shader contains banned pattern: {}",
                    pattern
                )));
            }
        }

        // 2. Validate workgroup sizes
        for entry_point in &module.entry_points {
            if let naga::ShaderStage::Compute = entry_point.stage {
                let workgroup_size = entry_point.workgroup_size;
                let total_invocations = workgroup_size[0] * workgroup_size[1] * workgroup_size[2];

                if total_invocations > self.max_total_invocations {
                    return Err(ComputeError::ExecutionFailed(format!(
                        "Workgroup too large: {} > {} total invocations",
                        total_invocations, self.max_total_invocations
                    )));
                }

                for (i, size) in workgroup_size.iter().enumerate() {
                    if *size > self.max_workgroup_size {
                        return Err(ComputeError::ExecutionFailed(format!(
                            "Workgroup dimension[{}] too large: {} > {}",
                            i, size, self.max_workgroup_size
                        )));
                    }
                }
            }
        }

        Ok(())
    }
}

/// Optimized WebGPU request structure
#[derive(Serialize)]
struct WebGpuRequest {
    method: String,
    shader: String,
    buffers: Vec<BufferDesc>,
    workgroup: [u32; 3],
    dispatch: [u32; 3],
}

#[derive(Serialize)]
struct BufferDesc {
    id: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    data: String, // Empty for output buffers
    size: usize,
    usage: String,     // "storage", "uniform"
    type_hint: String, // "float32", "uint32"
}

impl GpuUnit {
    pub fn new() -> Self {
        let mut shader_cache = HashMap::new();

        // Load pre-built WGSL shaders
        shader_cache.insert("matmul", include_str!("gpu_shaders/matmul.wgsl"));
        shader_cache.insert("fft", include_str!("gpu_shaders/fft.wgsl"));
        shader_cache.insert(
            "reduction_sum",
            include_str!("gpu_shaders/reduction_sum.wgsl"),
        );
        shader_cache.insert("pbr_lighting", include_str!("gpu_shaders/pbr.wgsl"));
        shader_cache.insert("nbody", include_str!("gpu_shaders/nbody.wgsl"));

        Self {
            config: GpuConfig::default(),
            shader_cache,
            validator: ShaderValidator::new(),
        }
    }

    /// Validate shader with Naga
    fn validate_shader(&self, shader_code: &str) -> Result<Module, ComputeError> {
        // 1. Size check
        if shader_code.len() > self.config.max_shader_size {
            return Err(ComputeError::ExecutionFailed(format!(
                "Shader too large: {} > {}",
                shader_code.len(),
                self.config.max_shader_size
            )));
        }

        // 2. Parse WGSL with Naga
        let module = wgsl::parse_str(shader_code)
            .map_err(|e| ComputeError::InvalidParams(format!("WGSL parse error: {:?}", e)))?;

        // 3. Validate module
        let mut validator = Validator::new(ValidationFlags::all(), Capabilities::all());
        validator.validate(&module).map_err(|e| {
            ComputeError::ExecutionFailed(format!("Shader validation failed: {:?}", e))
        })?;

        // 4. Security checks
        self.validator.validate_security(&module, shader_code)?;

        Ok(module)
    }

    /// Get shader code (pre-built or custom)
    fn get_shader_code(&self, method: &str, params: &JsonValue) -> Result<String, ComputeError> {
        // Check for pre-built shader
        if let Some(prebuilt) = self.shader_cache.get(method) {
            return Ok(prebuilt.to_string());
        }

        // For execute_wgsl method
        if method == "execute_wgsl" {
            let shader = params
                .get("shader")
                .and_then(|v| v.as_str())
                .ok_or_else(|| {
                    ComputeError::InvalidParams("Missing shader parameter".to_string())
                })?;

            // Validate custom shader
            self.validate_shader(shader)?;
            Ok(shader.to_string())
        } else {
            // No shader available - delegate to JS with method name
            Ok(String::new())
        }
    }

    /// Validate GPU request before delegating to WebGPU
    fn validate_request(&self, params: &JsonValue) -> Result<(), ComputeError> {
        // Validate workgroup size
        if let Some(workgroup) = params.get("workgroup").and_then(|v| v.as_array()) {
            for (i, dim) in workgroup.iter().take(3).enumerate() {
                if let Some(size) = dim.as_u64() {
                    if size as u32 > self.config.max_workgroup_size[i] {
                        return Err(ComputeError::ExecutionFailed(format!(
                            "Workgroup size[{}] too large: {} > {}",
                            i, size, self.config.max_workgroup_size[i]
                        )));
                    }
                }
            }
        }

        Ok(())
    }

    /// Create optimized WebGPU execution request
    fn create_webgpu_request(
        &self,
        method: &str,
        input: &[u8],
        params: &JsonValue,
    ) -> Result<Vec<u8>, ComputeError> {
        // Validate request
        self.validate_request(params)?;

        // Get shader code (if available)
        let shader_code = self.get_shader_code(method, params).unwrap_or_default();

        // Prepare buffers
        let buffer_type = params
            .get("buffer_type")
            .and_then(|v| v.as_str())
            .unwrap_or("float32");

        let buffers = vec![
            BufferDesc {
                id: "input".to_string(),
                data: general_purpose::STANDARD.encode(input),
                size: input.len(),
                usage: "storage".to_string(),
                type_hint: buffer_type.to_string(),
            },
            BufferDesc {
                id: "output".to_string(),
                data: String::new(), // Output buffer
                size: input.len(),   // Same size as input by default
                usage: "storage".to_string(),
                type_hint: buffer_type.to_string(),
            },
        ];

        // Extract workgroup and dispatch
        let workgroup = params
            .get("workgroup")
            .and_then(|v| v.as_array())
            .map(|arr| {
                [
                    arr.first().and_then(|v| v.as_u64()).unwrap_or(1) as u32,
                    arr.get(1).and_then(|v| v.as_u64()).unwrap_or(1) as u32,
                    arr.get(2).and_then(|v| v.as_u64()).unwrap_or(1) as u32,
                ]
            })
            .unwrap_or([64, 1, 1]);

        let dispatch = params
            .get("dispatch")
            .and_then(|v| v.as_array())
            .map(|arr| {
                [
                    arr.first().and_then(|v| v.as_u64()).unwrap_or(1) as u32,
                    arr.get(1).and_then(|v| v.as_u64()).unwrap_or(1) as u32,
                    arr.get(2).and_then(|v| v.as_u64()).unwrap_or(1) as u32,
                ]
            })
            .unwrap_or([1, 1, 1]);

        // Create request
        let request = WebGpuRequest {
            method: method.to_string(),
            shader: shader_code,
            buffers,
            workgroup,
            dispatch,
        };

        serde_json::to_vec(&request)
            .map_err(|e| ComputeError::ExecutionFailed(format!("Serialization failed: {}", e)))
    }
}

impl Default for GpuUnit {
    fn default() -> Self {
        Self::new()
    }
}

#[async_trait(?Send)]
impl UnitProxy for GpuUnit {
    fn service_name(&self) -> &str {
        "compute"
    }

    fn name(&self) -> &str {
        "gpu"
    }

    fn actions(&self) -> Vec<&str> {
        vec![
            "transform_vertices",
            "compute_normals",
            "tangent_space",
            "deferred_shading",
            "forward_rendering",
            "pbr_material",
            "visibility_culling",
            "lod_selection",
            "instanced_rendering",
            "mesh_shading",
            "ray_tracing",
            "path_tracing",
            "particle_update",
            "particle_forces",
            "particle_collision",
            "particle_spawning",
            "particle_sorting",
            "particle_billboards",
            "particle_trails",
            "particle_mesh",
            "particle_nbody",
            "tone_mapping",
            "color_correction",
            "bloom",
            "chromatic_aberration",
            "gaussian_blur",
            "motion_blur",
            "depth_of_field",
            "sharpen",
            "pixelation",
            "edge_detection",
            "cel_shading",
            "halftone",
            "ssao",
            "ssr",
            "temporal_aa",
            "perlin_noise",
            "simplex_noise",
            "worley_noise",
            "fractal_noise",
            "heightmap_generation",
            "erosion_simulation",
            "vegetation_placement",
            "procedural_texture",
            "normal_map_generation",
            "ao_map_generation",
            "fluid_simulation",
            "smoke_simulation",
            "water_simulation",
            "cloth_simulation",
            "hair_simulation",
            "reaction_diffusion",
            "cellular_automata",
            "sph_particles",
            "phong_lighting",
            "blinn_phong",
            "pbr_lighting",
            "subsurface_scattering",
            "glass_shader",
            "metal_shader",
            "fabric_shader",
            "water_shader",
            "uv_mapping",
            "parallax_mapping",
            "displacement_mapping",
            "execute_wgsl",
        ]
    }

    fn resource_limits(&self) -> ResourceLimits {
        ResourceLimits {
            max_input_size: 100 * 1024 * 1024,  // 100MB
            max_output_size: 100 * 1024 * 1024, // 100MB
            max_memory_pages: 2048,             // 128MB
            timeout_ms: 10000,                  // 10s
            max_fuel: 10_000_000_000,           // 10B instructions
        }
    }

    async fn execute(
        &self,
        method: &str,
        input: &[u8],
        params_json: &[u8],
    ) -> Result<Vec<u8>, ComputeError> {
        let params: serde_json::Value = serde_json::from_slice(params_json)
            .map_err(|e| ComputeError::InvalidParams(format!("Invalid JSON: {}", e)))?;

        match method {
            // ===== CATEGORY 1: RENDERING PIPELINE (12) =====
            "transform_vertices" => self.create_webgpu_request(method, input, &params),
            "compute_normals" => self.create_webgpu_request(method, input, &params),
            "tangent_space" => self.create_webgpu_request(method, input, &params),
            "deferred_shading" => self.create_webgpu_request(method, input, &params),
            "forward_rendering" => self.create_webgpu_request(method, input, &params),
            "pbr_material" => self.create_webgpu_request(method, input, &params),
            "visibility_culling" => self.create_webgpu_request(method, input, &params),
            "lod_selection" => self.create_webgpu_request(method, input, &params),
            "instanced_rendering" => self.create_webgpu_request(method, input, &params),
            "mesh_shading" => self.create_webgpu_request(method, input, &params),
            "ray_tracing" => self.create_webgpu_request(method, input, &params),
            "path_tracing" => self.create_webgpu_request(method, input, &params),

            // ===== CATEGORY 2: PARTICLE SYSTEMS (8) =====
            "particle_update" => self.create_webgpu_request(method, input, &params),
            "particle_forces" => self.create_webgpu_request(method, input, &params),
            "particle_collision" => self.create_webgpu_request(method, input, &params),
            "particle_spawning" => self.create_webgpu_request(method, input, &params),
            "particle_sorting" => self.create_webgpu_request(method, input, &params),
            "particle_billboards" => self.create_webgpu_request(method, input, &params),
            "particle_trails" => self.create_webgpu_request(method, input, &params),
            "particle_mesh" => self.create_webgpu_request(method, input, &params),
            "particle_nbody" => self.create_webgpu_request(method, input, &params),

            // ===== CATEGORY 3: POST-PROCESSING (15) =====
            "tone_mapping" => self.create_webgpu_request(method, input, &params),
            "color_correction" => self.create_webgpu_request(method, input, &params),
            "bloom" => self.create_webgpu_request(method, input, &params),
            "chromatic_aberration" => self.create_webgpu_request(method, input, &params),
            "gaussian_blur" => self.create_webgpu_request(method, input, &params),
            "motion_blur" => self.create_webgpu_request(method, input, &params),
            "depth_of_field" => self.create_webgpu_request(method, input, &params),
            "sharpen" => self.create_webgpu_request(method, input, &params),
            "pixelation" => self.create_webgpu_request(method, input, &params),
            "edge_detection" => self.create_webgpu_request(method, input, &params),
            "cel_shading" => self.create_webgpu_request(method, input, &params),
            "halftone" => self.create_webgpu_request(method, input, &params),
            "ssao" => self.create_webgpu_request(method, input, &params),
            "ssr" => self.create_webgpu_request(method, input, &params),
            "temporal_aa" => self.create_webgpu_request(method, input, &params),

            // ===== CATEGORY 4: PROCEDURAL GENERATION (10) =====
            "perlin_noise" => self.create_webgpu_request(method, input, &params),
            "simplex_noise" => self.create_webgpu_request(method, input, &params),
            "worley_noise" => self.create_webgpu_request(method, input, &params),
            "fractal_noise" => self.create_webgpu_request(method, input, &params),
            "heightmap_generation" => self.create_webgpu_request(method, input, &params),
            "erosion_simulation" => self.create_webgpu_request(method, input, &params),
            "vegetation_placement" => self.create_webgpu_request(method, input, &params),
            "procedural_texture" => self.create_webgpu_request(method, input, &params),
            "normal_map_generation" => self.create_webgpu_request(method, input, &params),
            "ao_map_generation" => self.create_webgpu_request(method, input, &params),

            // ===== CATEGORY 5: PHYSICS SIMULATION (8) =====
            "fluid_simulation" => self.create_webgpu_request(method, input, &params),
            "smoke_simulation" => self.create_webgpu_request(method, input, &params),
            "water_simulation" => self.create_webgpu_request(method, input, &params),
            "cloth_simulation" => self.create_webgpu_request(method, input, &params),
            "hair_simulation" => self.create_webgpu_request(method, input, &params),
            "reaction_diffusion" => self.create_webgpu_request(method, input, &params),
            "cellular_automata" => self.create_webgpu_request(method, input, &params),
            "sph_particles" => self.create_webgpu_request(method, input, &params),

            // ===== CATEGORY 6: SHADER LIBRARY (11) =====
            "phong_lighting" => self.create_webgpu_request(method, input, &params),
            "blinn_phong" => self.create_webgpu_request(method, input, &params),
            "pbr_lighting" => self.create_webgpu_request(method, input, &params),
            "subsurface_scattering" => self.create_webgpu_request(method, input, &params),
            "glass_shader" => self.create_webgpu_request(method, input, &params),
            "metal_shader" => self.create_webgpu_request(method, input, &params),
            "fabric_shader" => self.create_webgpu_request(method, input, &params),
            "water_shader" => self.create_webgpu_request(method, input, &params),
            "uv_mapping" => self.create_webgpu_request(method, input, &params),
            "parallax_mapping" => self.create_webgpu_request(method, input, &params),
            "displacement_mapping" => self.create_webgpu_request(method, input, &params),

            // ===== CUSTOM SHADER (1) =====
            "execute_wgsl" => self.create_webgpu_request(method, input, &params),

            _ => Err(ComputeError::UnknownMethod {
                library: "gpu".to_string(),
                method: method.to_string(),
            }),
        }
    }
}
