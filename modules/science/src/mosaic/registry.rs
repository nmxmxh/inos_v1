use dashmap::DashMap;
use lazy_static::lazy_static;
use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, HashMap, HashSet};
use std::hash::{Hash, Hasher};
use std::sync::{Arc, RwLock};
use thiserror::Error;

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct SubstanceDNA {
    pub id: String,
    pub name: String,
    pub mass_density: f32,            // kg/m^3
    pub young_modulus: f32,           // Pa
    pub thermal_conductivity: f32,    // W/(m*K)
    pub bond_energy_limit: f32,       // eV
    pub melting_point: f32,           // K
    pub boiling_point: f32,           // K
    pub specific_heat: f32,           // J/(kg*K)
    pub electrical_conductivity: f32, // S/m
    pub magnetic_permeability: f32,   // H/m
    pub optical_properties: OpticalProperties,
    pub phase_transitions: Vec<PhaseTransition>,
    pub reactivity_profiles: Vec<ReactivityProfile>,
    pub tags: HashSet<String>,
    pub metadata: HashMap<String, String>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct OpticalProperties {
    pub refractive_index: f32,
    pub absorption_coefficient: f32,
    pub reflectance: f32,
    pub transparency: f32,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct PhaseTransition {
    pub transition_type: TransitionType,
    pub temperature: f32,   // K
    pub latent_heat: f32,   // J/kg
    pub volume_change: f32, // Fraction
}

#[derive(Debug, Serialize, Deserialize, Clone, PartialEq)]
pub enum TransitionType {
    SolidToLiquid,
    LiquidToGas,
    SolidToGas,
    PhaseChange, // For allotropic changes
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ReactivityProfile {
    pub with_substance: String, // Substance ID
    pub reaction_type: ReactionType,
    pub activation_energy: f32, // eV
    pub reaction_rate: f32,     // mol/(L*s)
    pub products: Vec<ReactionProduct>,
}

#[derive(Debug, Serialize, Deserialize, Clone, PartialEq)]
pub enum ReactionType {
    Exothermic,
    Endothermic,
    Catalytic,
    Redox,
    AcidBase,
    Precipitation,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ReactionProduct {
    pub substance_id: String,
    pub stoichiometry: f32,
    pub phase: Phase,
}

#[derive(Debug, Serialize, Deserialize, Clone, Copy, PartialEq, Eq)]
pub enum Phase {
    Solid,
    Liquid,
    Gas,
    Plasma,
    BoseEinsteinCondensate,
    Superfluid,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct CompositeMaterial {
    pub id: String,
    pub name: String,
    pub components: Vec<MaterialComponent>,
    pub effective_properties: SubstanceDNA,
    pub microstructure: Microstructure,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct MaterialComponent {
    pub substance_id: String,
    pub volume_fraction: f32,
    pub orientation: Option<[f32; 3]>, // For anisotropic materials
    pub interface_energy: f32,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Microstructure {
    pub grain_size: f32,     // meters
    pub porosity: f32,       // 0-1
    pub anisotropy: f32,     // 0-1
    pub defect_density: f32, // defects per m^3
}

/// Registry entry with versioning and dependencies
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct RegistryEntry<T> {
    pub data: T,
    pub version: u32,
    pub dependencies: Vec<String>,
    pub checksum: u64,
    pub last_modified: u64,
    pub creator: String,
}

#[derive(Error, Debug)]
pub enum RegistryError {
    #[error("Entry not found: {0}")]
    NotFound(String),

    #[error("Version conflict: {0}")]
    VersionConflict(String),

    #[error("Circular dependency detected: {0}")]
    CircularDependency(String),

    #[error("Invalid dependency: {0}")]
    InvalidDependency(String),

    #[error("Serialization error: {0}")]
    SerializationError(#[from] serde_json::Error),

    #[error("IO error: {0}")]
    IoError(#[from] std::io::Error),

    #[error("Lock poisoned")]
    LockPoisoned,
}

/// Main registry for all simulation components
pub struct SimulationRegistry {
    substances: RwLock<HashMap<String, RegistryEntry<SubstanceDNA>>>,
    composites: RwLock<HashMap<String, RegistryEntry<CompositeMaterial>>>,

    // Fast lookup indices
    substance_by_property: RwLock<BTreeMap<PropertyRange, Vec<String>>>,
    tag_index: RwLock<HashMap<String, HashSet<String>>>,

    // Dependency graph for version management
    dependencies: RwLock<HashMap<String, HashSet<String>>>,

    // Cache for expensive computations
    property_cache: DashMap<String, ComputedProperties>,

    // Event handlers for registry changes
    event_handlers: RwLock<Vec<Box<dyn RegistryEventHandler + Send + Sync>>>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct PropertyRange {
    pub property: PropertyType,
    pub min: f32,
    pub max: f32,
}

impl Eq for PropertyRange {}

impl PartialOrd for PropertyRange {
    fn partial_cmp(&self, other: &Self) -> Option<std::cmp::Ordering> {
        Some(self.cmp(other))
    }
}

impl Ord for PropertyRange {
    fn cmp(&self, other: &Self) -> std::cmp::Ordering {
        self.property
            .cmp(&other.property)
            .then_with(|| self.min.total_cmp(&other.min))
            .then_with(|| self.max.total_cmp(&other.max))
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub enum PropertyType {
    MassDensity,
    YoungModulus,
    ThermalConductivity,
    BondEnergyLimit,
    MeltingPoint,
    BoilingPoint,
    SpecificHeat,
    ElectricalConductivity,
    RefractiveIndex,
}

#[derive(Debug, Clone)]
pub struct ComputedProperties {
    pub acoustic_impedance: f32,
    pub thermal_diffusivity: f32,
    pub prandtl_number: f32,
    pub mach_number_limit: f32,
    pub rayleigh_number: f32,
    pub computed_at: std::time::Instant,
}

pub trait RegistryEventHandler: Send + Sync {
    fn on_entry_added(&self, entry_type: &str, id: &str);
    fn on_entry_updated(&self, entry_type: &str, id: &str, old_version: u32, new_version: u32);
    fn on_entry_removed(&self, entry_type: &str, id: &str);
}

// Thread-safe global registry instance
lazy_static! {
    pub static ref GLOBAL_REGISTRY: Arc<SimulationRegistry> = Arc::new(SimulationRegistry::new());
}

impl Default for SimulationRegistry {
    fn default() -> Self {
        Self::new()
    }
}

impl SimulationRegistry {
    pub fn new() -> Self {
        Self {
            substances: RwLock::new(HashMap::new()),
            composites: RwLock::new(HashMap::new()),
            substance_by_property: RwLock::new(BTreeMap::new()),
            tag_index: RwLock::new(HashMap::new()),
            dependencies: RwLock::new(HashMap::new()),
            property_cache: DashMap::new(),
            event_handlers: RwLock::new(Vec::new()),
        }
    }

    /// Register a new substance
    pub fn register_substance(
        &self,
        substance: SubstanceDNA,
        creator: &str,
    ) -> Result<(), RegistryError> {
        let mut substances = self
            .substances
            .write()
            .map_err(|_| RegistryError::LockPoisoned)?;

        if substances.contains_key(&substance.id) {
            return Err(RegistryError::VersionConflict(format!(
                "Substance {} already exists",
                substance.id
            )));
        }

        // Check dependencies
        self.validate_dependencies(&substance.id, &substance.tags)?;

        // Create registry entry
        let entry = RegistryEntry {
            data: substance.clone(),
            version: 1,
            dependencies: Vec::new(),
            checksum: Self::calculate_checksum(&substance),
            last_modified: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_secs(),
            creator: creator.to_string(),
        };

        substances.insert(substance.id.clone(), entry);

        // Update indices
        self.update_indices(&substance)?;

        // Notify handlers
        self.notify_handlers("substance", &substance.id, None, Some(1))?;

        Ok(())
    }

    /// Update an existing substance
    pub fn update_substance(
        &self,
        id: &str,
        updater: impl FnOnce(&mut SubstanceDNA) -> Result<(), String>,
        updater_name: &str,
    ) -> Result<SubstanceDNA, RegistryError> {
        let mut substances = self
            .substances
            .write()
            .map_err(|_| RegistryError::LockPoisoned)?;

        let entry = substances
            .get_mut(id)
            .ok_or_else(|| RegistryError::NotFound(id.to_string()))?;

        let old_version = entry.version;
        let mut substance = entry.data.clone();

        // Apply update
        updater(&mut substance).map_err(RegistryError::VersionConflict)?;

        // Update entry
        entry.data = substance.clone();
        entry.version += 1;
        entry.checksum = Self::calculate_checksum(&substance);
        entry.last_modified = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_secs();
        entry.creator = updater_name.to_string();

        // Clear cache for this substance
        self.property_cache.remove(id);

        // Update indices
        let new_version = entry.version;
        let _ = entry; // Release borrow on substances
        self.update_indices(&substance)?;

        // Notify handlers
        self.notify_handlers("substance", id, Some(old_version), Some(new_version))?;

        Ok(substance)
    }

    /// Get a substance by ID
    pub fn get_substance(&self, id: &str) -> Result<SubstanceDNA, RegistryError> {
        let substances = self
            .substances
            .read()
            .map_err(|_| RegistryError::LockPoisoned)?;

        substances
            .get(id)
            .map(|entry| entry.data.clone())
            .ok_or_else(|| RegistryError::NotFound(id.to_string()))
    }

    /// Find substances by property range
    pub fn find_substances_by_property(
        &self,
        property: PropertyType,
        min: f32,
        max: f32,
    ) -> Result<Vec<SubstanceDNA>, RegistryError> {
        let index = self
            .substance_by_property
            .read()
            .map_err(|_| RegistryError::LockPoisoned)?;

        let target_range = PropertyRange { property, min, max };

        // Find overlapping ranges
        let mut results = Vec::new();
        let substances = self
            .substances
            .read()
            .map_err(|_| RegistryError::LockPoisoned)?;

        for (range, ids) in index.range(..=target_range.clone()) {
            if ranges_overlap(range, &target_range) {
                for id in ids {
                    if let Some(entry) = substances.get(id) {
                        results.push(entry.data.clone());
                    }
                }
            }
        }

        Ok(results)
    }

    /// Find substances by tags
    pub fn find_substances_by_tags(
        &self,
        tags: &[String],
        mode: TagMatchMode,
    ) -> Result<Vec<SubstanceDNA>, RegistryError> {
        let tag_index = self
            .tag_index
            .read()
            .map_err(|_| RegistryError::LockPoisoned)?;
        let substances = self
            .substances
            .read()
            .map_err(|_| RegistryError::LockPoisoned)?;

        match mode {
            TagMatchMode::Any => {
                let mut ids = HashSet::new();
                for tag in tags {
                    if let Some(tagged_ids) = tag_index.get(tag) {
                        ids.extend(tagged_ids.iter().cloned());
                    }
                }
                Ok(ids
                    .iter()
                    .filter_map(|id| substances.get(id).map(|e| e.data.clone()))
                    .collect())
            }
            TagMatchMode::All => {
                if tags.is_empty() {
                    return Ok(Vec::new());
                }

                // Start with first tag's substances
                let first_tag = &tags[0];
                let mut result_ids: Option<HashSet<String>> = tag_index.get(first_tag).cloned();

                // Intersect with remaining tags
                for tag in &tags[1..] {
                    if let Some(current_ids) = result_ids.as_mut() {
                        if let Some(tag_ids) = tag_index.get(tag) {
                            current_ids.retain(|id| tag_ids.contains(id));
                        } else {
                            result_ids = None;
                            break;
                        }
                    }
                }

                Ok(result_ids
                    .unwrap_or_default()
                    .iter()
                    .filter_map(|id| substances.get(id).map(|e| e.data.clone()))
                    .collect())
            }
            TagMatchMode::None => {
                let excluded_ids: HashSet<String> = tags
                    .iter()
                    .filter_map(|tag| tag_index.get(tag))
                    .flat_map(|ids| ids.iter())
                    .cloned()
                    .collect();

                Ok(substances
                    .values()
                    .filter(|entry| !excluded_ids.contains(&entry.data.id))
                    .map(|entry| entry.data.clone())
                    .collect())
            }
        }
    }

    /// Compute derived properties for a substance
    pub fn compute_properties(
        &self,
        substance_id: &str,
    ) -> Result<ComputedProperties, RegistryError> {
        // Check cache first
        if let Some(cached) = self.property_cache.get(substance_id) {
            // Refresh if stale (older than 1 minute)
            if cached.computed_at.elapsed() < std::time::Duration::from_secs(60) {
                return Ok(cached.clone());
            }
        }

        let substance = self.get_substance(substance_id)?;

        // Compute derived properties
        let computed = ComputedProperties {
            acoustic_impedance: substance.mass_density
                * (substance.young_modulus / substance.mass_density).sqrt(),
            thermal_diffusivity: substance.thermal_conductivity
                / (substance.mass_density * substance.specific_heat),
            prandtl_number: 0.7, // Simplified, should be computed from viscosity
            mach_number_limit: (2.0 * substance.bond_energy_limit
                / (substance.mass_density * 1e-3))
                .sqrt(),
            rayleigh_number: 9.8
                * 1e-3
                * substance.thermal_conductivity
                * (substance.boiling_point - substance.melting_point)
                * substance.mass_density
                / (1e-6 * 1e-3), // Simplified
            computed_at: std::time::Instant::now(),
        };

        self.property_cache
            .insert(substance_id.to_string(), computed.clone());
        Ok(computed)
    }

    /// Create composite material from components
    pub fn create_composite(
        &self,
        id: String,
        name: String,
        components: Vec<MaterialComponent>,
        creator: &str,
    ) -> Result<CompositeMaterial, RegistryError> {
        // Validate all component substances exist
        for component in &components {
            self.get_substance(&component.substance_id)?;
        }

        // Compute effective properties using mixture rules
        let effective_properties = self.compute_composite_properties(&components)?;

        let composite = CompositeMaterial {
            id: id.clone(),
            name,
            components,
            effective_properties,
            microstructure: Microstructure {
                grain_size: 1e-6,
                porosity: 0.05,
                anisotropy: 0.3,
                defect_density: 1e12,
            },
        };

        // Register composite
        let mut composites = self
            .composites
            .write()
            .map_err(|_| RegistryError::LockPoisoned)?;

        let entry = RegistryEntry {
            data: composite.clone(),
            version: 1,
            dependencies: Vec::new(),
            checksum: Self::calculate_checksum(&composite),
            last_modified: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_secs(),
            creator: creator.to_string(),
        };

        composites.insert(id, entry);

        Ok(composite)
    }

    /// Compute effective properties for composite
    fn compute_composite_properties(
        &self,
        components: &[MaterialComponent],
    ) -> Result<SubstanceDNA, RegistryError> {
        let mut total_volume = 0.0;
        let mut weighted_properties = SubstanceDNA {
            id: "composite".to_string(),
            name: "Composite".to_string(),
            mass_density: 0.0,
            young_modulus: 0.0,
            thermal_conductivity: 0.0,
            bond_energy_limit: 0.0,
            melting_point: 3000.0, // Start high for min()
            boiling_point: 5000.0,
            specific_heat: 0.0,
            electrical_conductivity: 0.0,
            magnetic_permeability: 1.0,
            optical_properties: OpticalProperties {
                refractive_index: 1.0,
                absorption_coefficient: 0.0,
                reflectance: 0.0,
                transparency: 1.0,
            },
            phase_transitions: Vec::new(),
            reactivity_profiles: Vec::new(),
            tags: HashSet::new(),
            metadata: HashMap::new(),
        };

        for component in components {
            let substance = self.get_substance(&component.substance_id)?;
            let weight = component.volume_fraction;
            total_volume += weight;

            // Voigt-Reuss averaging for Young's modulus
            weighted_properties.young_modulus += weight * substance.young_modulus;

            // Linear averaging for most properties
            weighted_properties.mass_density += weight * substance.mass_density;
            weighted_properties.thermal_conductivity += weight * substance.thermal_conductivity;
            weighted_properties.bond_energy_limit += weight * substance.bond_energy_limit;
            weighted_properties.specific_heat += weight * substance.specific_heat;
            weighted_properties.electrical_conductivity +=
                weight * substance.electrical_conductivity;

            // Use lowest phase transition temperatures
            weighted_properties.melting_point = weighted_properties
                .melting_point
                .min(substance.melting_point);
            weighted_properties.boiling_point = weighted_properties
                .boiling_point
                .min(substance.boiling_point);
        }

        // Normalize by total volume
        if total_volume > 0.0 {
            weighted_properties.mass_density /= total_volume;
            weighted_properties.thermal_conductivity /= total_volume;
            weighted_properties.bond_energy_limit /= total_volume;
            weighted_properties.specific_heat /= total_volume;
            weighted_properties.electrical_conductivity /= total_volume;
        }

        Ok(weighted_properties)
    }

    /// Add event handler
    pub fn add_event_handler(
        &self,
        handler: Box<dyn RegistryEventHandler + Send + Sync>,
    ) -> Result<(), RegistryError> {
        let mut handlers = self
            .event_handlers
            .write()
            .map_err(|_| RegistryError::LockPoisoned)?;
        handlers.push(handler);
        Ok(())
    }

    /// Update indices when substance is added or modified
    fn update_indices(&self, substance: &SubstanceDNA) -> Result<(), RegistryError> {
        let mut property_index = self
            .substance_by_property
            .write()
            .map_err(|_| RegistryError::LockPoisoned)?;
        let mut tag_index = self
            .tag_index
            .write()
            .map_err(|_| RegistryError::LockPoisoned)?;

        // Remove old entries
        self.remove_from_indices_locked(&substance.id, &mut property_index, &mut tag_index)?;

        // Add to property index
        let properties = vec![
            (PropertyType::MassDensity, substance.mass_density),
            (PropertyType::YoungModulus, substance.young_modulus),
            (
                PropertyType::ThermalConductivity,
                substance.thermal_conductivity,
            ),
            (PropertyType::BondEnergyLimit, substance.bond_energy_limit),
            (PropertyType::MeltingPoint, substance.melting_point),
            (PropertyType::BoilingPoint, substance.boiling_point),
            (PropertyType::SpecificHeat, substance.specific_heat),
            (
                PropertyType::ElectricalConductivity,
                substance.electrical_conductivity,
            ),
            (
                PropertyType::RefractiveIndex,
                substance.optical_properties.refractive_index,
            ),
        ];

        for (prop_type, value) in properties {
            let range = Self::bin_value(value, prop_type);
            property_index
                .entry(range)
                .or_insert_with(Vec::new)
                .push(substance.id.clone());
        }

        // Add to tag index
        for tag in &substance.tags {
            tag_index
                .entry(tag.clone())
                .or_insert_with(HashSet::new)
                .insert(substance.id.clone());
        }

        Ok(())
    }

    fn remove_from_indices_locked(
        &self,
        id: &str,
        property_index: &mut BTreeMap<PropertyRange, Vec<String>>,
        tag_index: &mut HashMap<String, HashSet<String>>,
    ) -> Result<(), RegistryError> {
        // Remove from property index
        for ids in property_index.values_mut() {
            ids.retain(|existing_id| existing_id != id);
        }

        // Remove empty ranges
        property_index.retain(|_, ids| !ids.is_empty());

        // Remove from tag index
        for ids in tag_index.values_mut() {
            ids.remove(id);
        }

        // Remove empty tags
        tag_index.retain(|_, ids| !ids.is_empty());

        Ok(())
    }

    fn bin_value(value: f32, prop_type: PropertyType) -> PropertyRange {
        // Create logarithmic bins for large ranges
        let (min, max) = match prop_type {
            PropertyType::MassDensity => (value * 0.9, value * 1.1),
            PropertyType::YoungModulus => (value * 0.95, value * 1.05),
            _ => (value * 0.99, value * 1.01),
        };

        PropertyRange {
            property: prop_type,
            min,
            max,
        }
    }

    fn calculate_checksum<T: Serialize>(data: &T) -> u64 {
        use std::collections::hash_map::DefaultHasher;
        use std::hash::Hash;

        let serialized = serde_json::to_string(data).unwrap_or_default();
        let mut hasher = DefaultHasher::new();
        serialized.hash(&mut hasher);
        hasher.finish()
    }

    fn validate_dependencies(
        &self,
        id: &str,
        _tags: &HashSet<String>,
    ) -> Result<(), RegistryError> {
        let deps = self
            .dependencies
            .read()
            .map_err(|_| RegistryError::LockPoisoned)?;

        // Check for circular dependencies
        let mut visited = HashSet::new();
        let mut stack = Vec::new();

        stack.push(id.to_string());
        while let Some(current) = stack.pop() {
            if visited.contains(&current) {
                return Err(RegistryError::CircularDependency(format!(
                    "Circular dependency involving {}",
                    id
                )));
            }
            visited.insert(current.clone());

            if let Some(deps_set) = deps.get(&current) {
                for dep in deps_set {
                    stack.push(dep.clone());
                }
            }
        }

        Ok(())
    }

    fn notify_handlers(
        &self,
        entry_type: &str,
        id: &str,
        old_version: Option<u32>,
        new_version: Option<u32>,
    ) -> Result<(), RegistryError> {
        let handlers = self
            .event_handlers
            .read()
            .map_err(|_| RegistryError::LockPoisoned)?;

        match (old_version, new_version) {
            (None, Some(_)) => {
                for handler in handlers.iter() {
                    handler.on_entry_added(entry_type, id);
                }
            }
            (Some(old), Some(new)) if old < new => {
                for handler in handlers.iter() {
                    handler.on_entry_updated(entry_type, id, old, new);
                }
            }
            (Some(_), None) => {
                for handler in handlers.iter() {
                    handler.on_entry_removed(entry_type, id);
                }
            }
            _ => {}
        }

        Ok(())
    }

    /// Export registry to JSON
    pub fn export(&self, path: &str) -> Result<(), RegistryError> {
        let export_data = RegistryExport {
            substances: self
                .substances
                .read()
                .map_err(|_| RegistryError::LockPoisoned)?
                .clone(),
            composites: self
                .composites
                .read()
                .map_err(|_| RegistryError::LockPoisoned)?
                .clone(),
            export_time: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_secs(),
        };

        let json = serde_json::to_string_pretty(&export_data)?;
        std::fs::write(path, json)?;

        Ok(())
    }

    /// Import registry from JSON
    pub fn import(&self, path: &str) -> Result<(), RegistryError> {
        let json = std::fs::read_to_string(path)?;
        let import_data: RegistryExport = serde_json::from_str(&json)?;

        let mut substances = self
            .substances
            .write()
            .map_err(|_| RegistryError::LockPoisoned)?;
        let mut composites = self
            .composites
            .write()
            .map_err(|_| RegistryError::LockPoisoned)?;

        // Merge imported data
        for (id, entry) in import_data.substances {
            substances.insert(id, entry);
        }

        for (id, entry) in import_data.composites {
            composites.insert(id, entry);
        }

        // Rebuild indices
        let all_data: Vec<SubstanceDNA> = substances.values().map(|e| e.data.clone()).collect();
        for data in all_data {
            self.update_indices(&data)?;
        }

        Ok(())
    }
}

#[derive(Debug, Serialize, Deserialize)]
struct RegistryExport {
    substances: HashMap<String, RegistryEntry<SubstanceDNA>>,
    composites: HashMap<String, RegistryEntry<CompositeMaterial>>,
    export_time: u64,
}

#[derive(Debug, Clone, Copy)]
pub enum TagMatchMode {
    Any,  // Match any of the tags
    All,  // Match all tags
    None, // Match none of the tags
}

fn ranges_overlap(a: &PropertyRange, b: &PropertyRange) -> bool {
    if a.property != b.property {
        return false;
    }
    a.min <= b.max && b.min <= a.max
}

/// DNALibrary with registry integration
pub struct DNALibrary {
    registry: Arc<SimulationRegistry>,
}

impl DNALibrary {
    pub fn new(registry: Arc<SimulationRegistry>) -> Self {
        Self { registry }
    }

    pub fn lookup(&self, id: &str) -> Option<SubstanceDNA> {
        self.registry.get_substance(id).ok()
    }

    pub fn find_by_property(
        &self,
        property: PropertyType,
        min: f32,
        max: f32,
    ) -> Result<Vec<SubstanceDNA>, RegistryError> {
        self.registry
            .find_substances_by_property(property, min, max)
    }

    pub fn create_mixture(
        &self,
        components: &[(String, f32)], // (substance_id, volume_fraction)
        name: &str,
    ) -> Result<SubstanceDNA, RegistryError> {
        let material_components: Vec<MaterialComponent> = components
            .iter()
            .map(|(id, fraction)| MaterialComponent {
                substance_id: id.clone(),
                volume_fraction: *fraction,
                orientation: None,
                interface_energy: 0.0,
            })
            .collect();

        let composite = self.registry.create_composite(
            format!("mixture_{}", name.to_lowercase().replace(" ", "_")),
            name.to_string(),
            material_components,
            "DNALibrary",
        )?;

        Ok(composite.effective_properties)
    }
}

// Default implementations for common materials
impl SimulationRegistry {
    pub fn register_default_materials(&self) -> Result<(), RegistryError> {
        // Water
        self.register_substance(
            SubstanceDNA {
                id: "water".to_string(),
                name: "Water".to_string(),
                mass_density: 997.0,
                young_modulus: 2.2e9,
                thermal_conductivity: 0.6,
                bond_energy_limit: 0.5,
                melting_point: 273.15,
                boiling_point: 373.15,
                specific_heat: 4182.0,
                electrical_conductivity: 0.0005,
                magnetic_permeability: 1.2566e-6,
                optical_properties: OpticalProperties {
                    refractive_index: 1.333,
                    absorption_coefficient: 0.01,
                    reflectance: 0.02,
                    transparency: 0.98,
                },
                phase_transitions: vec![
                    PhaseTransition {
                        transition_type: TransitionType::SolidToLiquid,
                        temperature: 273.15,
                        latent_heat: 334000.0,
                        volume_change: -0.09,
                    },
                    PhaseTransition {
                        transition_type: TransitionType::LiquidToGas,
                        temperature: 373.15,
                        latent_heat: 2257000.0,
                        volume_change: 1600.0,
                    },
                ],
                reactivity_profiles: vec![ReactivityProfile {
                    with_substance: "sodium".to_string(),
                    reaction_type: ReactionType::Exothermic,
                    activation_energy: 0.1,
                    reaction_rate: 1000.0,
                    products: vec![
                        ReactionProduct {
                            substance_id: "sodium_hydroxide".to_string(),
                            stoichiometry: 1.0,
                            phase: Phase::Liquid,
                        },
                        ReactionProduct {
                            substance_id: "hydrogen".to_string(),
                            stoichiometry: 0.5,
                            phase: Phase::Gas,
                        },
                    ],
                }],
                tags: vec![
                    "liquid".to_string(),
                    "polar".to_string(),
                    "solvent".to_string(),
                ]
                .into_iter()
                .collect(),
                metadata: HashMap::from([
                    ("description".to_string(), "Universal solvent".to_string()),
                    ("phase".to_string(), "liquid".to_string()),
                ]),
            },
            "system",
        )?;

        // Iron
        self.register_substance(
            SubstanceDNA {
                id: "iron".to_string(),
                name: "Iron".to_string(),
                mass_density: 7874.0,
                young_modulus: 211e9,
                thermal_conductivity: 80.2,
                bond_energy_limit: 4.3,
                melting_point: 1811.0,
                boiling_point: 3134.0,
                specific_heat: 449.0,
                electrical_conductivity: 1.0e7,
                magnetic_permeability: 6.3e-3,
                optical_properties: OpticalProperties {
                    refractive_index: 2.91,
                    absorption_coefficient: 3.84,
                    reflectance: 0.65,
                    transparency: 0.0,
                },
                phase_transitions: vec![
                    PhaseTransition {
                        transition_type: TransitionType::SolidToLiquid,
                        temperature: 1811.0,
                        latent_heat: 247000.0,
                        volume_change: 0.04,
                    },
                    PhaseTransition {
                        transition_type: TransitionType::LiquidToGas,
                        temperature: 3134.0,
                        latent_heat: 6340000.0,
                        volume_change: 1200.0,
                    },
                ],
                reactivity_profiles: vec![ReactivityProfile {
                    with_substance: "oxygen".to_string(),
                    reaction_type: ReactionType::Exothermic,
                    activation_energy: 0.5,
                    reaction_rate: 10.0,
                    products: vec![ReactionProduct {
                        substance_id: "iron_oxide".to_string(),
                        stoichiometry: 1.0,
                        phase: Phase::Solid,
                    }],
                }],
                tags: vec![
                    "metal".to_string(),
                    "ferromagnetic".to_string(),
                    "solid".to_string(),
                ]
                .into_iter()
                .collect(),
                metadata: HashMap::from([
                    (
                        "description".to_string(),
                        "Common metal, ferromagnetic".to_string(),
                    ),
                    ("crystal_structure".to_string(), "bcc".to_string()),
                ]),
            },
            "system",
        )?;

        Ok(())
    }
}

// Convenience functions for global registry
pub fn register_substance_global(
    substance: SubstanceDNA,
    creator: &str,
) -> Result<(), RegistryError> {
    GLOBAL_REGISTRY.register_substance(substance, creator)
}

pub fn get_substance_global(id: &str) -> Result<SubstanceDNA, RegistryError> {
    GLOBAL_REGISTRY.get_substance(id)
}

pub fn create_composite_global(
    id: String,
    name: String,
    components: Vec<MaterialComponent>,
    creator: &str,
) -> Result<CompositeMaterial, RegistryError> {
    GLOBAL_REGISTRY.create_composite(id, name, components, creator)
}
