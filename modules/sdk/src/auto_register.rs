use crate::registry::*;
use crate::sab::SafeSAB;


/// Module metadata for auto-registration
pub struct ModuleMetadata {
    pub id: &'static str,
    pub version: (u8, u8, u8),
    pub dependencies: &'static [(&'static str, (u8, u8, u8), bool)], // (id, min_version, optional)
    pub resource_flags: u16,
    pub min_memory_mb: u16,
    pub min_gpu_memory_mb: u16,
    pub min_cpu_cores: u8,
    pub base_cost: u16,
    pub per_mb_cost: u8,
    pub per_second_cost: u16,
}

/// Central registry of all INOS modules
pub const ALL_MODULES: &[ModuleMetadata] = &[
    // GPU module (no dependencies)
    ModuleMetadata {
        id: "gpu",
        version: (1, 0, 0),
        dependencies: &[],
        resource_flags: RESOURCE_GPU_INTENSIVE,
        min_memory_mb: 512,
        min_gpu_memory_mb: 1024,
        min_cpu_cores: 1,
        base_cost: 500,
        per_mb_cost: 50,
        per_second_cost: 5000,
    },
    // Storage module (no dependencies)
    ModuleMetadata {
        id: "storage",
        version: (1, 0, 0),
        dependencies: &[],
        resource_flags: RESOURCE_IO_INTENSIVE | RESOURCE_MEMORY_INTENSIVE,
        min_memory_mb: 256,
        min_gpu_memory_mb: 0,
        min_cpu_cores: 1,
        base_cost: 200,
        per_mb_cost: 10,
        per_second_cost: 1000,
    },
    // Crypto module (no dependencies)
    ModuleMetadata {
        id: "crypto",
        version: (1, 0, 0),
        dependencies: &[],
        resource_flags: RESOURCE_CPU_INTENSIVE,
        min_memory_mb: 128,
        min_gpu_memory_mb: 0,
        min_cpu_cores: 2,
        base_cost: 100,
        per_mb_cost: 5,
        per_second_cost: 500,
    },
    // ML module (depends on gpu, storage)
    ModuleMetadata {
        id: "ml",
        version: (1, 0, 0),
        dependencies: &[("gpu", (1, 0, 0), false), ("storage", (1, 0, 0), false)],
        resource_flags: RESOURCE_GPU_INTENSIVE | RESOURCE_MEMORY_INTENSIVE,
        min_memory_mb: 2048,
        min_gpu_memory_mb: 4096,
        min_cpu_cores: 4,
        base_cost: 1000,
        per_mb_cost: 100,
        per_second_cost: 10000,
    },
    // Image module (CPU intensive)
    ModuleMetadata {
        id: "image",
        version: (1, 0, 0),
        dependencies: &[],
        resource_flags: RESOURCE_CPU_INTENSIVE | RESOURCE_MEMORY_INTENSIVE,
        min_memory_mb: 512,
        min_gpu_memory_mb: 0,
        min_cpu_cores: 2,
        base_cost: 300,
        per_mb_cost: 20,
        per_second_cost: 2000,
    },
    // Audio module (CPU intensive)
    ModuleMetadata {
        id: "audio",
        version: (1, 0, 0),
        dependencies: &[],
        resource_flags: RESOURCE_CPU_INTENSIVE,
        min_memory_mb: 256,
        min_gpu_memory_mb: 0,
        min_cpu_cores: 2,
        base_cost: 250,
        per_mb_cost: 15,
        per_second_cost: 1500,
    },
    // Data module (IO intensive)
    ModuleMetadata {
        id: "data",
        version: (1, 0, 0),
        dependencies: &[],
        resource_flags: RESOURCE_IO_INTENSIVE | RESOURCE_MEMORY_INTENSIVE,
        min_memory_mb: 1024,
        min_gpu_memory_mb: 0,
        min_cpu_cores: 2,
        base_cost: 400,
        per_mb_cost: 30,
        per_second_cost: 3000,
    },
    // Mining module (depends on crypto, gpu)
    ModuleMetadata {
        id: "mining",
        version: (1, 0, 0),
        dependencies: &[("crypto", (1, 0, 0), false), ("gpu", (1, 0, 0), false)],
        resource_flags: RESOURCE_GPU_INTENSIVE | RESOURCE_CPU_INTENSIVE,
        min_memory_mb: 1024,
        min_gpu_memory_mb: 2048,
        min_cpu_cores: 4,
        base_cost: 800,
        per_mb_cost: 80,
        per_second_cost: 8000,
    },
    // Physics module (depends on gpu, storage)
    ModuleMetadata {
        id: "physics",
        version: (1, 0, 0),
        dependencies: &[("gpu", (1, 0, 0), false), ("storage", (1, 0, 0), false)],
        resource_flags: RESOURCE_GPU_INTENSIVE | RESOURCE_MEMORY_INTENSIVE,
        min_memory_mb: 1536,
        min_gpu_memory_mb: 3072,
        min_cpu_cores: 4,
        base_cost: 900,
        per_mb_cost: 90,
        per_second_cost: 9000,
    },
    // Science module (depends on physics, storage, data)
    ModuleMetadata {
        id: "science",
        version: (1, 0, 0),
        dependencies: &[
            ("physics", (1, 0, 0), false),
            ("storage", (1, 0, 0), false),
            ("data", (1, 0, 0), false),
        ],
        resource_flags: RESOURCE_GPU_INTENSIVE | RESOURCE_MEMORY_INTENSIVE | RESOURCE_CPU_INTENSIVE,
        min_memory_mb: 4096,
        min_gpu_memory_mb: 8192,
        min_cpu_cores: 8,
        base_cost: 1500,
        per_mb_cost: 150,
        per_second_cost: 15000,
    },
];

/// Auto-register all modules to SAB
/// This is the ONLY function that needs to be called from JavaScript
// Removed wasm_bindgen attribute
pub fn register_all_modules(sab: &js_sys::SharedArrayBuffer) -> Result<String, String> {
    let safe_sab = SafeSAB::new(sab.clone());
    let mut registered = Vec::new();
    let mut failed = Vec::new();

    for module_meta in ALL_MODULES {
        match register_module(&safe_sab, module_meta) {
            Ok(slot) => {
                registered.push(format!(
                    "{}@{}.{}.{} -> slot {}",
                    module_meta.id,
                    module_meta.version.0,
                    module_meta.version.1,
                    module_meta.version.2,
                    slot
                ));
            }
            Err(e) => {
                failed.push(format!("{}: {}", module_meta.id, e));
            }
        }
    }

    // Return summary as JSON string
    let registered_str = registered
        .iter()
        .map(|s| format!("\"{}\"", s))
        .collect::<Vec<_>>()
        .join(",");
    let failed_str = failed
        .iter()
        .map(|s| format!("\"{}\"", s))
        .collect::<Vec<_>>()
        .join(",");

    let summary = format!(
        "{{\"registered\":[{}],\"failed\":[{}],\"total\":{},\"success_count\":{},\"failure_count\":{}}}",
        registered_str,
        failed_str,
        ALL_MODULES.len(),
        registered.len(),
        failed.len()
    );

    Ok(String::from_str(&summary))
}

/// Standardized module initialization function
/// This should be re-exported by each module with // Removed wasm_bindgen attribute
///
/// # Example
/// ```rust
/// // Removed wasm_bindgen attribute
/// pub fn compute_init_with_sab() -> Result<(), String> {
///     sdk::auto_register::init_with_sab()
/// }
/// ```
pub fn init_with_sab() -> Result<(), String> {
    use JsCast;
    use web_sys::console;

    // FIRST THING: Log that we're here
    console::log_1(&"[Rust] init_with_sab called!".into());

    // Get the module's linear memory (which IS the SharedArrayBuffer)
    console::log_1(&"[Rust] Getting memory...".into());
    let memory_js = memory();

    console::log_1(&"[Rust] Converting to WebAssembly.Memory...".into());
    let memory: js_sys::WebAssembly::Memory = memory_js.dyn_into()?;

    console::log_1(&"[Rust] Getting buffer...".into());
    let buffer = memory.buffer();

    // DEBUG: Check if buffer is actually a SharedArrayBuffer
    console::log_1(&"[Rust] Checking if SharedArrayBuffer...".into());
    let is_shared = buffer.is_instance_of::<js_sys::SharedArrayBuffer>();
    console::log_1(&format!("[Rust] Memory buffer is SharedArrayBuffer: {}", is_shared).into());

    if !is_shared {
        // Buffer is an ArrayBuffer, not SharedArrayBuffer
        // This means wasm-bindgen created its own memory and ignored our provided SharedArrayBuffer
        let buffer_obj: js_sys::Object = buffer.clone().into();
        console::log_1(
            &format!("[Rust] Buffer type: {:?}", buffer_obj.constructor().name()).into(),
        );
        return Err(String::from_str(
            "Module memory is not a SharedArrayBuffer! wasm-bindgen created its own memory.",
        ));
    }

    // Convert to SharedArrayBuffer
    console::log_1(&"[Rust] Converting to SharedArrayBuffer...".into());
    let sab = js_sys::SharedArrayBuffer::from(buffer);

    // Call register_all_modules
    console::log_1(&"[Rust] Calling register_all_modules...".into());
    register_all_modules(&sab)?;

    console::log_1(&"[Rust] Registration complete!".into());
    Ok(())
}

/// Register a single module standalone (for dynamic discovery)
// Removed wasm_bindgen attribute
pub fn register_standalone_module(
    sab: &js_sys::SharedArrayBuffer,
    id: &str,
    major: u8,
    minor: u8,
    patch: u8,
) -> Result<usize, String> {
    let safe_sab = SafeSAB::new(sab.clone());

    let (mut entry, dependencies, capabilities) = ModuleEntryBuilder::new(id)
        .version(major, minor, patch)
        .build()
        .map_err(|errors| errors.join(", "))?;

    // Find slot using double hashing
    let (slot, _is_new) = find_slot_double_hashing(&safe_sab, id)?;

    // Write dependencies to Arena
    if !dependencies.is_empty() {
        let offset = write_dependency_table(&safe_sab, &dependencies)
            .map_err(|e| format!("Failed to write dependencies: {}", e))?;
        entry.dep_table_offset = offset;
    }

    // Write capabilities to Arena
    if !capabilities.is_empty() {
        let offset = write_capability_table(&safe_sab, &capabilities)
            .map_err(|e| format!("Failed to write capabilities: {}", e))?;
        entry.cap_table_offset = offset;
    }

    // Write to SAB
    write_enhanced_entry(&safe_sab, slot, &entry)?;

    Ok(slot)
}

/// Register a single module with full metadata
pub fn register_module(sab: &SafeSAB, meta: &ModuleMetadata) -> Result<usize, String> {
    // Build module entry
    let mut builder = ModuleEntryBuilder::new(meta.id)
        .version(meta.version.0, meta.version.1, meta.version.2)
        .resource_profile(ResourceProfile {
            flags: meta.resource_flags,
            min_memory_mb: meta.min_memory_mb,
            min_gpu_memory_mb: meta.min_gpu_memory_mb,
            min_cpu_cores: meta.min_cpu_cores,
        })
        .cost_model(CostModel {
            base_cost: meta.base_cost,
            per_mb_cost: meta.per_mb_cost,
            per_second_cost: meta.per_second_cost,
        });

    // Add dependencies
    for (dep_id, min_version, optional) in meta.dependencies {
        builder = builder.dependency(dep_id, *min_version, *optional);
    }

    let (mut entry, dependencies, capabilities) =
        builder.build().map_err(|errors| errors.join(", "))?;

    // Find slot using double hashing
    let (slot, _is_new) = find_slot_double_hashing(sab, meta.id)?;

    // Write dependencies to Arena
    if !dependencies.is_empty() {
        let offset = write_dependency_table(sab, &dependencies)
            .map_err(|e| format!("Failed to write dependencies: {}", e))?;
        entry.dep_table_offset = offset;
    }

    // Write capabilities to Arena
    if !capabilities.is_empty() {
        let offset = write_capability_table(sab, &capabilities)
            .map_err(|e| format!("Failed to write capabilities: {}", e))?;
        entry.cap_table_offset = offset;
    }

    // Write to SAB
    write_enhanced_entry(sab, slot, &entry)?;

    Ok(slot)
}

/// Get module count
// Removed wasm_bindgen attribute
pub fn get_module_count() -> usize {
    ALL_MODULES.len()
}

/// Get module info by ID
// Removed wasm_bindgen attribute
pub fn get_module_info(module_id: &str) -> Result<String, String> {
    for module_meta in ALL_MODULES {
        if module_meta.id == module_id {
            // Build dependencies JSON array
            let deps = module_meta
                .dependencies
                .iter()
                .map(|(id, ver, opt)| {
                    format!(
                        "{{\"id\":\"{}\",\"min_version\":\"{}.{}.{}\",\"optional\":{}}}",
                        id, ver.0, ver.1, ver.2, opt
                    )
                })
                .collect::<Vec<_>>()
                .join(",");

            let info = format!(
                "{{\"id\":\"{}\",\"version\":\"{}.{}.{}\",\"dependencies\":[{}],\"resources\":{{\"memory_mb\":{},\"gpu_memory_mb\":{},\"cpu_cores\":{}}},\"cost\":{{\"base\":{},\"per_mb\":{},\"per_second\":{}}}}}",
                module_meta.id,
                module_meta.version.0, module_meta.version.1, module_meta.version.2,
                deps,
                module_meta.min_memory_mb,
                module_meta.min_gpu_memory_mb,
                module_meta.min_cpu_cores,
                module_meta.base_cost,
                module_meta.per_mb_cost,
                module_meta.per_second_cost
            );

            return Ok(String::from_str(&info));
        }
    }

    Err(String::from_str(&format!(
        "Module {} not found",
        module_id
    )))
}
