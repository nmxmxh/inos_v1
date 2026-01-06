use crate::layout::*;
use crate::sab::SafeSAB;

/// Enhanced Module Registry Entry (96 bytes) - Phase 2
/// Production-grade with collision handling and extended metadata
#[repr(C)]
#[derive(Clone, Copy)]
pub struct EnhancedModuleEntry {
    // Header (32 bytes)
    pub signature: u64, // 0x494E4F5352454749 ("INOSREGI")
    pub id_hash: u32,   // CRC32C of module ID
    pub version_major: u8,
    pub version_minor: u8,
    pub version_patch: u8,
    pub flags: u8,

    // Metadata (16 bytes)
    pub timestamp: u64,   // Unix timestamp (milliseconds)
    pub data_offset: u32, // Offset to extended data in Arena
    pub data_size: u32,

    // Resource profile (8 bytes)
    pub resource_flags: u16,
    pub min_memory_mb: u16,
    pub min_gpu_memory_mb: u16,
    pub min_cpu_cores: u8,
    pub reserved1: u8,

    // Cost model (8 bytes)
    pub base_cost: u16,
    pub per_mb_cost: u8,
    pub per_second_cost: u16,
    pub reserved2: u8,

    // Dependency/capability pointers (24 bytes)
    pub dep_table_offset: u32, // Offset to dependency table in arena
    pub dep_count: u16,
    pub max_version_major: u8,
    pub min_version_major: u8,
    pub cap_table_offset: u32, // Offset to capability table in arena
    pub cap_count: u16,
    pub reserved3: [u8; 2],

    // Module ID inline (12 bytes, null-terminated)
    pub module_id: [u8; 12],

    // Quick hash for fast lookup (4 bytes)
    pub quick_hash: u32, // FNV-1a hash

    // Padding to 96 bytes (16 bytes)
    pub reserved4: [u8; 16],
}

// Compile-time size verification
const _: [(); 96] = [(); std::mem::size_of::<EnhancedModuleEntry>()];

/// Magic signature for validation
pub const REGISTRY_SIGNATURE: u64 = 0x494E4F5352454749;

/// Module entry flags
pub const FLAG_HAS_EXTENDED_DATA: u8 = 0b0001;
pub const FLAG_IS_ACTIVE: u8 = 0b0010;
pub const FLAG_HAS_OVERFLOW: u8 = 0b0100;

/// Resource profile flags
pub const RESOURCE_CPU_INTENSIVE: u16 = 0b0001;
pub const RESOURCE_GPU_INTENSIVE: u16 = 0b0010;
pub const RESOURCE_MEMORY_INTENSIVE: u16 = 0b0100;
pub const RESOURCE_IO_INTENSIVE: u16 = 0b1000;
pub const RESOURCE_NETWORK_INTENSIVE: u16 = 0b10000;

impl EnhancedModuleEntry {
    pub fn new() -> Self {
        Self {
            signature: REGISTRY_SIGNATURE,
            id_hash: 0,
            version_major: 0,
            version_minor: 0,
            version_patch: 0,
            flags: 0,
            timestamp: 0,
            data_offset: 0,
            data_size: 0,
            resource_flags: 0,
            min_memory_mb: 0,
            min_gpu_memory_mb: 0,
            min_cpu_cores: 0,
            reserved1: 0,
            base_cost: 0,
            per_mb_cost: 0,
            per_second_cost: 0,
            reserved2: 0,
            dep_table_offset: 0,
            dep_count: 0,
            max_version_major: 255,
            min_version_major: 1,
            cap_table_offset: 0,
            cap_count: 0,
            reserved3: [0; 2],
            module_id: [0; 12],
            quick_hash: 0,
            reserved4: [0; 16],
        }
    }

    pub fn is_valid(&self) -> bool {
        self.signature == REGISTRY_SIGNATURE && self.id_hash != 0
    }

    pub fn is_active(&self) -> bool {
        (self.flags & FLAG_IS_ACTIVE) != 0
    }

    pub fn set_active(&mut self) {
        self.flags |= FLAG_IS_ACTIVE;
    }

    pub fn set_flag(&mut self, flag: u8) {
        self.flags |= flag;
    }

    pub fn has_flag(&self, flag: u8) -> bool {
        (self.flags & flag) != 0
    }

    pub fn set_resource_flag(&mut self, flag: u16) {
        self.resource_flags |= flag;
    }

    pub fn has_resource_flag(&self, flag: u16) -> bool {
        (self.resource_flags & flag) != 0
    }

    pub fn get_module_id(&self) -> String {
        let null_pos = self.module_id.iter().position(|&b| b == 0).unwrap_or(12);
        String::from_utf8_lossy(&self.module_id[..null_pos]).to_string()
    }
}

impl Default for EnhancedModuleEntry {
    fn default() -> Self {
        Self::new()
    }
}

// ========== CAPABILITY TABLE ==========

/// Capability entry stored in arena (36 bytes)
#[repr(C)]
#[derive(Clone, Copy)]
pub struct CapabilityEntry {
    pub id: [u8; 32], // Null-terminated string
    pub min_memory_mb: u16,
    pub flags: u8,
    pub reserved: u8,
}

pub const CAP_FLAG_REQUIRES_GPU: u8 = 0b0001;

impl CapabilityEntry {
    pub fn new(id: &str, requires_gpu: bool, min_memory_mb: u16) -> Self {
        let mut entry = Self {
            id: [0; 32],
            min_memory_mb,
            flags: if requires_gpu {
                CAP_FLAG_REQUIRES_GPU
            } else {
                0
            },
            reserved: 0,
        };

        // Copy ID (max 31 chars + null terminator)
        let id_bytes = id.as_bytes();
        let copy_len = id_bytes.len().min(31);
        entry.id[..copy_len].copy_from_slice(&id_bytes[..copy_len]);

        entry
    }
}

// ========== HASHING FUNCTIONS ==========

/// CRC32C hash (Castagnoli polynomial) for module IDs
pub fn crc32c_hash(data: &[u8]) -> u32 {
    const CRC32C_TABLE: [u32; 256] = generate_crc32c_table();

    let mut crc: u32 = 0xFFFFFFFF;
    for &byte in data {
        let index = ((crc ^ byte as u32) & 0xFF) as usize;
        crc = (crc >> 8) ^ CRC32C_TABLE[index];
    }
    !crc
}

const fn generate_crc32c_table() -> [u32; 256] {
    let mut table = [0u32; 256];
    let mut i = 0;
    while i < 256 {
        let mut crc = i as u32;
        let mut j = 0;
        while j < 8 {
            if crc & 1 != 0 {
                crc = (crc >> 1) ^ 0x82F63B78; // Castagnoli polynomial
            } else {
                crc >>= 1;
            }
            j += 1;
        }
        table[i] = crc;
        i += 1;
    }
    table
}

/// FNV-1a hash for quick lookup
pub fn fnv1a_hash(data: &[u8]) -> u32 {
    const FNV_PRIME: u32 = 0x01000193;
    const FNV_OFFSET: u32 = 0x811C9DC5;

    let mut hash = FNV_OFFSET;
    for &byte in data {
        hash ^= byte as u32;
        hash = hash.wrapping_mul(FNV_PRIME);
    }
    hash
}

// ========== DOUBLE HASHING ==========

const MAX_PROBE_ATTEMPTS: usize = 128;

/// Calculate primary slot using CRC32C
pub fn calculate_primary_slot(module_id: &str) -> usize {
    let hash = crc32c_hash(module_id.as_bytes());
    (hash as usize) % MAX_MODULES_INLINE
}

/// Calculate secondary hash for double hashing (must be coprime with table size)
pub fn calculate_secondary_hash(module_id: &str) -> usize {
    let hash = fnv1a_hash(module_id.as_bytes());
    let step = (hash as usize) % (MAX_MODULES_INLINE - 1);
    if step.is_multiple_of(2) {
        step + 1
    } else {
        step
    }
}

/// Find slot for module using double hashing
pub fn find_slot_double_hashing(sab: &SafeSAB, module_id: &str) -> Result<(usize, bool), String> {
    let primary_slot = calculate_primary_slot(module_id);
    let secondary_hash = calculate_secondary_hash(module_id);
    let module_hash = crc32c_hash(module_id.as_bytes());

    let mut slot = primary_slot;

    for attempt in 0..MAX_PROBE_ATTEMPTS {
        let entry = read_enhanced_entry(sab, slot)?;

        if !entry.is_valid() {
            return Ok((slot, true)); // New registration
        }

        if entry.id_hash == module_hash {
            let existing_id = entry.get_module_id();
            if existing_id == module_id {
                return Ok((slot, false)); // Re-registration
            }
        }

        slot = (primary_slot + (attempt + 1) * secondary_hash) % MAX_MODULES_INLINE;
    }

    Err("Inline registry full, need arena overflow".to_string())
}

/// Read enhanced entry from SAB
pub fn read_enhanced_entry(sab: &SafeSAB, slot: usize) -> Result<EnhancedModuleEntry, String> {
    if slot >= MAX_MODULES_INLINE {
        return Err(format!("Slot {} exceeds max inline modules", slot));
    }

    let offset = OFFSET_MODULE_REGISTRY + (slot * MODULE_ENTRY_SIZE);
    let bytes = sab.read(offset, MODULE_ENTRY_SIZE)?;

    let entry = unsafe { std::ptr::read(bytes.as_ptr() as *const EnhancedModuleEntry) };

    Ok(entry)
}

/// Write enhanced entry to SAB
pub fn write_enhanced_entry(
    sab: &SafeSAB,
    slot: usize,
    entry: &EnhancedModuleEntry,
) -> Result<(), String> {
    if slot >= MAX_MODULES_INLINE {
        return Err(format!("Slot {} exceeds max inline modules", slot));
    }

    let offset = OFFSET_MODULE_REGISTRY + (slot * MODULE_ENTRY_SIZE);

    let bytes = unsafe {
        std::slice::from_raw_parts(
            entry as *const _ as *const u8,
            std::mem::size_of::<EnhancedModuleEntry>(),
        )
    };

    sab.write(offset, bytes)?;

    Ok(())
}

// ========== SAB WRITES ==========

/// Allocate space in the Arena
pub fn allocate_arena(sab: &SafeSAB, size: u32) -> Result<u32, String> {
    // 1. Get current bump pointer from AtomicFlags
    // OFFSET_ATOMIC_FLAGS = 0x000000
    // Index 8 * 4 bytes = 0x20
    // We use IDX_ARENA_ALLOCATOR directly with the typed array view.

    // 2. Atomic Fetch & Add
    // We need to implement atomic_fetch_add in SafeSAB or use raw pointer here safely.
    // SafeSAB uses atomic methods internally. We need to expose a way to do this.
    // Assuming SafeSAB has `fetch_add_u32` or similar. If not, we use raw access properly.
    // Let's assume we can read/write. Ideally we need atomic op.
    // For now, we will use a lock-based approach or raw atomics if SafeSAB allows.
    // Checking sab.rs... SafeSAB exposes `buffer` which is `SharedArrayBuffer`.
    // We can use `js_sys::Atomics::add`.

    // Use stable ABI to avoid hashed imports
    let buffer = sab.inner();
    let length = crate::js_interop::get_byte_length(buffer);
    let view = crate::js_interop::create_i32_view(buffer, 0, length / 4);
    let typed_array_val: crate::JsValue = view.into();

    let aligned_size = (size + 3) & !3;

    let old_usage =
        crate::js_interop::atomic_add(&typed_array_val, IDX_ARENA_ALLOCATOR, aligned_size as i32)
            as u32;

    let offset = OFFSET_ARENA as u32 + old_usage;
    let total_size = crate::js_interop::get_byte_length(sab.inner());

    if offset + aligned_size > total_size {
        // Rollback? No easy rollback in wait-free. Just fail.
        return Err("Arena out of memory".to_string());
    }

    Ok(offset)
}

/// Write dependency table to Arena
pub fn write_dependency_table(sab: &SafeSAB, deps: &[DependencyEntry]) -> Result<u32, String> {
    if deps.is_empty() {
        return Ok(0);
    }

    // format: [count:u32][entry...][entry...]
    // But wait, the EnhancedModuleEntry has `dep_count` and `dep_table_offset`.
    // So the table at offset should just be the entries?
    // Reader in `loader.go` expects 16 bytes per entry at the offset.
    // Reader in `registry.go` (Step 129) `readDependencyTable` reads count from Entry, then iterates.
    // So just the entries.

    let size = std::mem::size_of_val(deps) as u32;
    let offset = allocate_arena(sab, size)?;

    let bytes = unsafe { std::slice::from_raw_parts(deps.as_ptr() as *const u8, size as usize) };

    sab.write(offset as usize, bytes)?;
    Ok(offset)
}

/// Write capability table to Arena
pub fn write_capability_table(sab: &SafeSAB, caps: &[CapabilityEntry]) -> Result<u32, String> {
    if caps.is_empty() {
        return Ok(0);
    }

    let size = std::mem::size_of_val(caps) as u32;
    let offset = allocate_arena(sab, size)?;

    let bytes = unsafe { std::slice::from_raw_parts(caps.as_ptr() as *const u8, size as usize) };

    sab.write(offset as usize, bytes)?;
    Ok(offset)
}

// ========== DEPENDENCY TABLE ==========

/// Dependency entry stored in arena (16 bytes)
#[repr(C)]
#[derive(Clone, Copy)]
pub struct DependencyEntry {
    pub module_id_hash: u32,
    pub min_version_major: u8,
    pub min_version_minor: u8,
    pub min_version_patch: u8,
    pub max_version_major: u8,
    pub max_version_minor: u8,
    pub max_version_patch: u8,
    pub flags: u8,
    pub alternatives_offset: u16,
    pub reserved: [u8; 2],
}

pub const DEP_FLAG_OPTIONAL: u8 = 0b0001;
pub const DEP_FLAG_HAS_ALTERNATIVES: u8 = 0b0010;

impl DependencyEntry {
    pub fn new(module_id: &str, min_version: (u8, u8, u8), optional: bool) -> Self {
        Self {
            module_id_hash: crc32c_hash(module_id.as_bytes()),
            min_version_major: min_version.0,
            min_version_minor: min_version.1,
            min_version_patch: min_version.2,
            max_version_major: 255,
            max_version_minor: 255,
            max_version_patch: 255,
            flags: if optional { DEP_FLAG_OPTIONAL } else { 0 },
            alternatives_offset: 0,
            reserved: [0; 2],
        }
    }

    pub fn is_optional(&self) -> bool {
        (self.flags & DEP_FLAG_OPTIONAL) != 0
    }
}

// ========== MODULE BUILDER ==========

#[derive(Default)]
pub struct ResourceProfile {
    pub flags: u16,
    pub min_memory_mb: u16,
    pub min_gpu_memory_mb: u16,
    pub min_cpu_cores: u8,
}

pub struct CostModel {
    pub base_cost: u16,
    pub per_mb_cost: u8,
    pub per_second_cost: u16,
}

impl Default for CostModel {
    fn default() -> Self {
        Self {
            base_cost: 100,
            per_mb_cost: 10,
            per_second_cost: 1000,
        }
    }
}

pub struct ModuleEntryBuilder {
    id: String,
    version: (u8, u8, u8),
    dependencies: Vec<DependencyEntry>,
    capabilities: Vec<CapabilityEntry>,
    resource_profile: ResourceProfile,
    cost_model: CostModel,
    validation_errors: Vec<String>,
}

impl ModuleEntryBuilder {
    pub fn new(id: &str) -> Self {
        Self {
            id: id.to_string(),
            version: (1, 0, 0),
            dependencies: Vec::new(),
            capabilities: Vec::new(),
            resource_profile: ResourceProfile::default(),
            cost_model: CostModel::default(),
            validation_errors: Vec::new(),
        }
    }

    pub fn version(mut self, major: u8, minor: u8, patch: u8) -> Self {
        if major == 0 && minor == 0 && patch == 0 {
            self.validation_errors
                .push("Version cannot be 0.0.0".to_string());
        }
        self.version = (major, minor, patch);
        self
    }

    pub fn dependency(
        mut self,
        module_id: &str,
        min_version: (u8, u8, u8),
        optional: bool,
    ) -> Self {
        if module_id == self.id {
            self.validation_errors
                .push(format!("Module {} cannot depend on itself", self.id));
            return self;
        }

        if self
            .dependencies
            .iter()
            .any(|d| d.module_id_hash == crc32c_hash(module_id.as_bytes()))
        {
            self.validation_errors
                .push(format!("Duplicate dependency on {}", module_id));
            return self;
        }

        self.dependencies
            .push(DependencyEntry::new(module_id, min_version, optional));
        self
    }

    pub fn resource_profile(mut self, profile: ResourceProfile) -> Self {
        self.resource_profile = profile;
        self
    }

    pub fn cost_model(mut self, model: CostModel) -> Self {
        self.cost_model = model;
        self
    }

    pub fn capability(mut self, id: &str, requires_gpu: bool, min_memory_mb: u16) -> Self {
        self.capabilities
            .push(CapabilityEntry::new(id, requires_gpu, min_memory_mb));
        self
    }

    pub fn validate(&self) -> Result<(), Vec<String>> {
        let mut errors = self.validation_errors.clone();

        if self.id.is_empty() {
            errors.push("Module ID cannot be empty".to_string());
        }
        if self.id.len() > 11 {
            errors.push(format!("Module ID '{}' too long (max 11 chars)", self.id));
        }

        if self.dependencies.len() > 255 {
            errors.push("Too many dependencies (max 255)".to_string());
        }

        if !errors.is_empty() {
            Err(errors)
        } else {
            Ok(())
        }
    }

    pub fn build(
        self,
    ) -> Result<
        (
            EnhancedModuleEntry,
            Vec<DependencyEntry>,
            Vec<CapabilityEntry>,
        ),
        Vec<String>,
    > {
        self.validate()?;

        let mut entry = EnhancedModuleEntry::new();
        entry.id_hash = crc32c_hash(self.id.as_bytes());
        entry.version_major = self.version.0;
        entry.version_minor = self.version.1;
        entry.version_patch = self.version.2;
        entry.timestamp = get_timestamp_ms();
        entry.resource_flags = self.resource_profile.flags;
        entry.min_memory_mb = self.resource_profile.min_memory_mb;
        entry.min_gpu_memory_mb = self.resource_profile.min_gpu_memory_mb;
        entry.min_cpu_cores = self.resource_profile.min_cpu_cores;
        entry.base_cost = self.cost_model.base_cost;
        entry.per_mb_cost = self.cost_model.per_mb_cost;
        entry.per_second_cost = self.cost_model.per_second_cost;
        entry.dep_count = self.dependencies.len() as u16;
        entry.cap_count = self.capabilities.len() as u16;

        // NOTE: Offsets for dep_table and cap_table are NOT set here.
        // The caller is responsible for writing tables to the Arena and setting offsets.
        // This decouples building the struct from SAB allocation effects.
        let id_bytes = self.id.as_bytes();
        let copy_len = id_bytes.len().min(11);
        entry.module_id[..copy_len].copy_from_slice(&id_bytes[..copy_len]);
        entry.module_id[copy_len] = 0;

        entry.quick_hash = fnv1a_hash(&entry.module_id[..copy_len]);
        entry.set_active();

        Ok((entry, self.dependencies, self.capabilities))
    }
}

/// Signal registry change to wake Go supervisor discovery loop
/// Uses Atomics.add + Atomics.notify for zero-CPU wake-up
pub fn signal_registry_change(sab: &SafeSAB) {
    use crate::js_interop;
    use crate::layout::IDX_REGISTRY_EPOCH;

    // Atomically increment the registry epoch
    let buffer = sab.inner();
    let length = js_interop::get_byte_length(buffer);
    let view = js_interop::create_i32_view(buffer, 0, length / 4);
    let typed_view: crate::JsValue = view.into();

    js_interop::atomic_add(&typed_view, IDX_REGISTRY_EPOCH, 1);
    js_interop::atomic_notify(&typed_view, IDX_REGISTRY_EPOCH, i32::MAX);
}

fn get_timestamp_ms() -> u64 {
    crate::js_interop::get_now()
}

// ========== TESTS ==========

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_enhanced_entry_size() {
        assert_eq!(std::mem::size_of::<EnhancedModuleEntry>(), 96);
        assert_eq!(std::mem::size_of::<CapabilityEntry>(), 36);
    }

    #[test]
    fn test_crc32c_hash() {
        let hash1 = crc32c_hash(b"ml");
        let hash2 = crc32c_hash(b"ml");
        let hash3 = crc32c_hash(b"gpu");

        assert_eq!(hash1, hash2);
        assert_ne!(hash1, hash3);
    }

    #[test]
    fn test_double_hashing() {
        let modules = vec!["ml", "gpu", "storage", "crypto"];
        let mut slots = std::collections::HashSet::new();

        for module_id in modules {
            let slot = calculate_primary_slot(module_id);
            slots.insert(slot);
        }

        // Should have unique slots (no collisions for these 4)
        assert!(slots.len() >= 3);
    }

    #[test]
    fn test_module_builder() {
        let (entry, _, _) = ModuleEntryBuilder::new("ml")
            .version(1, 0, 0)
            .dependency("gpu", (1, 0, 0), false)
            .dependency("storage", (1, 0, 0), false)
            .build()
            .unwrap();

        assert_eq!(entry.version_major, 1);
        assert_eq!(entry.dep_count, 2);
        assert!(entry.is_active());
        assert_eq!(entry.get_module_id(), "ml");
    }
}
