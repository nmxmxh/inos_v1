pub mod actor;
pub mod reader;
pub mod sensor;

use actor::{Actor, ActorDriver};
use log::{error, info};
use sdk::{Epoch, IDX_ACTOR_EPOCH, IDX_SENSOR_EPOCH};
use sensor::SensorSubscriber;

use sdk::JsValue;

// Bare-metal WASM Nexus (no wasm-bindgen macros)
pub struct Nexus {
    actor_driver: ActorDriver,
    sensor_subscriber: SensorSubscriber,
}

/// Standardized Memory Allocator for WebAssembly
#[no_mangle]
pub extern "C" fn drivers_alloc(size: usize) -> *mut u8 {
    let mut buf = Vec::with_capacity(size);
    let ptr = buf.as_mut_ptr();
    std::mem::forget(buf);
    ptr
}

/// Standardized Initialization with SharedArrayBuffer
#[no_mangle]
pub extern "C" fn drivers_init_with_sab() -> i32 {
    sdk::js_interop::console_log("[drivers] DEBUG: Init function called", 3);
    // Use stable ABI to get global object
    let global = sdk::js_interop::get_global();
    let sab_key = sdk::js_interop::create_string("__INOS_SAB__");
    let sab_val = sdk::js_interop::reflect_get(&global, &sab_key);

    let offset_key = sdk::js_interop::create_string("__INOS_SAB_OFFSET__");
    let offset_val = sdk::js_interop::reflect_get(&global, &offset_key);

    let size_key = sdk::js_interop::create_string("__INOS_SAB_SIZE__");
    let size_val = sdk::js_interop::reflect_get(&global, &size_key);

    let _id_key = sdk::js_interop::create_string("__INOS_MODULE_ID__");
    let _id_val = sdk::js_interop::reflect_get(&global, &_id_key);

    if let (Ok(val), Ok(off), Ok(sz)) = (sab_val, offset_val, size_val) {
        if !val.is_undefined() && !val.is_null() {
            let offset = sdk::js_interop::as_f64(&off).unwrap_or(0.0) as u32;
            let size = sdk::js_interop::as_f64(&sz).unwrap_or(0.0) as u32;

            // Create TWO SafeSAB references:
            // 1. Scoped view for module data
            let module_sab = sdk::sab::SafeSAB::new_shared_view(val.clone(), offset, size);
            // 2. Global SAB for registry writes
            let global_sab = sdk::sab::SafeSAB::new(val.clone());

            sdk::init_logging();
            info!("Drivers module initialized with synchronized SAB bridge (Offset: 0x{:x}, Size: {}MB)", 
                offset, size / 1024 / 1024);

            // Helper to register simple modules
            let register_drivers = |sab: &sdk::sab::SafeSAB| {
                use sdk::registry::*;
                let id = "drivers";
                let mut builder = ModuleEntryBuilder::new(id).version(1, 0, 0);
                builder = builder.capability("usb", false, 64);
                builder = builder.capability("bluetooth", false, 64);
                builder = builder.capability("sensor", false, 64);
                builder = builder.capability("actor", false, 64);
                builder = builder.capability("nexus", false, 128);

                match builder.build() {
                    Ok((mut entry, _, caps)) => {
                        if let Ok(offset) = write_capability_table(sab, &caps) {
                            entry.cap_table_offset = offset;
                        }
                        if let Ok((slot, _)) = find_slot_double_hashing(sab, id) {
                            match write_enhanced_entry(sab, slot, &entry) {
                                Ok(_) => info!("Registered module {} at slot {}", id, slot),
                                Err(e) => {
                                    error!("Failed to write registry entry for {}: {:?}", id, e)
                                }
                            }
                        } else {
                            error!("Could not find available slot for module {}", id);
                        }
                    }
                    Err(e) => error!("Failed to build module entry for {}: {:?}", id, e),
                }
            };

            register_drivers(&global_sab);

            return 1;
        }
    }
    0
}

/// External poll entry point for JavaScript
#[no_mangle]
pub extern "C" fn drivers_poll() {
    // High-frequency reactor for Drivers
}

impl Nexus {
    pub fn new(sab: &sdk::JsValue) -> Self {
        let actor_epoch = Epoch::new(sab, IDX_ACTOR_EPOCH);
        let sensor_epoch = Epoch::new(sab, IDX_SENSOR_EPOCH);

        Self {
            actor_driver: ActorDriver::new(actor_epoch),
            sensor_subscriber: SensorSubscriber::new(sensor_epoch),
        }
    }

    pub fn poll(&mut self) -> Result<(), JsValue> {
        self.actor_driver.poll().map_err(|e| JsValue::from(e))?;
        self.sensor_subscriber
            .poll()
            .map_err(|e| JsValue::from(e))?;
        Ok(())
    }

    // Additional methods to add actors/sensors can be added here
    // or through specific registration WASM exports
}

// Example Hardware Driver for a Robot Leg (Direct Implementation)
pub struct RobotLegActor {
    id: String,
}

impl Actor for RobotLegActor {
    fn id(&self) -> &str {
        &self.id
    }

    fn on_command(&mut self, _cmd: &actor::ActorCommand) -> Result<(), String> {
        // Here we would interact with the specific hardware or memory region
        // if this was a combined driver.
        Ok(())
    }
}
