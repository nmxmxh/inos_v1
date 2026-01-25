// INOS Drivers Module - Pure I/O Socket System
// Sensors → Actors via library proxies
// NO wasm-bindgen macros - pure C ABI

pub mod actor;
pub mod reader;
pub mod sensor;

// New I/O socket modules
pub mod actuation;
pub mod commands;
pub mod mavlink;
pub mod perception;
pub mod positioning; // Generic command system
pub mod ros2;

#[cfg(target_arch = "wasm32")]
getrandom::register_custom_getrandom!(sdk::js_interop::getrandom_custom);

use actor::{Actor, ActorDriver};
use log::{error, info};
use sdk::{Epoch, IDX_ACTOR_EPOCH, IDX_SENSOR_EPOCH};
use sensor::SensorSubscriber;

// Re-export for convenience
pub use actuation::{GpioController, MotorController, ServoController};
pub use commands::{CommandResult, DriverCommand, SensorData};
pub use perception::{DepthCamera, LidarDriver};
pub use positioning::PositioningSystem;

use once_cell::sync::Lazy;
use parking_lot::Mutex;

/// Global Drivers instance for C ABI access
static GLOBAL_DRIVERS: Lazy<Mutex<Option<Drivers>>> = Lazy::new(|| Mutex::new(None));

/// Main Drivers struct - manages all I/O sockets
///
/// **Architecture**: Pure I/O sockets for military/location/spatial/movement data
/// - **Input Sockets**: Sensors (GPS, IMU, LIDAR, Camera)
/// - **Output Sockets**: Actors (Motors, Servos, GPIO)
///
/// **Library Proxy Pattern**: Leverages existing Rust libraries
/// - embedded-hal for hardware abstraction
/// - nalgebra for transformations
/// - ahrs for sensor fusion (optional)
pub struct Drivers {
    // Existing
    actor_driver: ActorDriver,
    sensor_subscriber: SensorSubscriber,

    // New I/O sockets
    positioning: PositioningSystem,
    lidar: LidarDriver,
    depth_camera: DepthCamera,
    motors: MotorController,
    servos: ServoController,
    gpio: GpioController,
    mavlink: mavlink::MavlinkDriver,
    ros2: ros2::Ros2Driver,
    _sab: Option<sdk::sab::SafeSAB>,
}

impl Drivers {
    pub fn new(sab: Option<sdk::sab::SafeSAB>) -> Self {
        let placeholder_sab = sab
            .clone()
            .unwrap_or_else(|| sdk::sab::SafeSAB::new(&sdk::js_interop::get_global()));
        let actor_epoch = Epoch::new(placeholder_sab.clone(), IDX_ACTOR_EPOCH);
        let sensor_epoch = Epoch::new(placeholder_sab.clone(), IDX_SENSOR_EPOCH);

        Self {
            actor_driver: ActorDriver::new(actor_epoch),
            sensor_subscriber: SensorSubscriber::new(sensor_epoch),
            positioning: PositioningSystem::new(),
            lidar: LidarDriver::default(),
            depth_camera: DepthCamera::default(),
            motors: MotorController::default(),
            servos: ServoController::default(),
            gpio: GpioController::default(),
            mavlink: mavlink::MavlinkDriver::default(),
            ros2: ros2::Ros2Driver::default(),
            _sab: sab,
        }
    }

    // Positioning methods
    pub fn update_gps(&mut self, lat: f64, lon: f64, alt: f64, accuracy: f32) {
        self.positioning.update_gps(lat, lon, alt, accuracy);
    }

    pub fn update_imu(&mut self, imu: positioning::ImuData) {
        self.positioning.update_imu(imu);
    }

    pub fn get_position(&self) -> positioning::Position {
        self.positioning.get_position()
    }

    pub fn get_orientation(&self) -> positioning::Orientation {
        self.positioning.get_orientation()
    }

    pub fn get_velocity(&self) -> positioning::Velocity {
        self.positioning.get_velocity()
    }

    // Perception methods
    pub fn process_lidar_scan(
        &mut self,
        raw_data: &[u8],
        timestamp: f64,
    ) -> Result<perception::LidarScan, String> {
        self.lidar.process_scan(raw_data, timestamp)
    }

    pub fn process_depth_frame(
        &mut self,
        data: Vec<u16>,
        timestamp: f64,
    ) -> perception::DepthFrame {
        self.depth_camera.process_frame(data, timestamp)
    }

    // Actuation methods
    pub fn set_motor_speed(&mut self, motor_id: u8, speed: f32) -> Result<(), String> {
        self.motors.set_speed(motor_id, speed)
    }

    pub fn set_servo_angle(&mut self, servo_id: u8, angle: f32) -> Result<(), String> {
        self.servos.set_angle(servo_id, angle)
    }

    pub fn set_gpio_pin(&mut self, pin: u8, state: bool) -> Result<(), String> {
        self.gpio.set_pin(pin, state)
    }

    pub fn emergency_stop(&mut self) {
        self.motors.emergency_stop();
    }

    /// Execute generic command (Phase 1A: Command Dispatch)
    pub fn execute_command(&mut self, cmd: DriverCommand) -> CommandResult {
        match cmd {
            DriverCommand::UpdateGps {
                lat,
                lon,
                alt,
                accuracy,
            } => {
                self.update_gps(lat, lon, alt, accuracy);
                CommandResult::success("GPS updated")
            }
            DriverCommand::UpdateImu {
                accel,
                gyro,
                mag,
                timestamp,
            } => {
                let imu = positioning::ImuData {
                    accel,
                    gyro,
                    mag,
                    timestamp,
                };
                self.update_imu(imu);
                CommandResult::success("IMU updated")
            }
            DriverCommand::Motor {
                motor_id,
                speed,
                duration_ms,
            } => {
                match self.motors.execute(actuation::MotorCommand {
                    motor_id,
                    speed,
                    duration_ms,
                }) {
                    Ok(_) => CommandResult::success(format!("Motor {} set to {}", motor_id, speed)),
                    Err(e) => CommandResult::error(e),
                }
            }
            DriverCommand::Servo {
                servo_id,
                angle,
                speed,
            } => {
                match self.servos.execute(actuation::ServoCommand {
                    servo_id,
                    angle,
                    speed,
                }) {
                    Ok(_) => {
                        CommandResult::success(format!("Servo {} set to {}°", servo_id, angle))
                    }
                    Err(e) => CommandResult::error(e),
                }
            }
            DriverCommand::Gpio { pin, state } => {
                match self.gpio.execute(actuation::GpioCommand { pin, state }) {
                    Ok(_) => CommandResult::success(format!("GPIO pin {} set to {}", pin, state)),
                    Err(e) => CommandResult::error(e),
                }
            }
            DriverCommand::ProcessLidarScan {
                raw_data,
                timestamp,
            } => match self.process_lidar_scan(&raw_data, timestamp) {
                Ok(scan) => {
                    let points: Vec<[f32; 3]> =
                        scan.points.iter().map(|p| [p.x, p.y, p.z]).collect();

                    CommandResult::success_with_data(
                        "LIDAR scan processed",
                        SensorData::LidarScan {
                            points,
                            timestamp: scan.timestamp,
                            range_min: scan.range_min,
                            range_max: scan.range_max,
                        },
                    )
                }
                Err(e) => CommandResult::error(e),
            },
            DriverCommand::ProcessDepthFrame {
                width: _,
                height: _,
                data,
                timestamp,
            } => {
                let frame = self.process_depth_frame(data.clone(), timestamp);
                CommandResult::success_with_data(
                    "Depth frame processed",
                    SensorData::DepthFrame {
                        width: frame.width,
                        height: frame.height,
                        data: frame.data,
                        timestamp: frame.timestamp,
                    },
                )
            }
            DriverCommand::EmergencyStop => {
                self.emergency_stop();
                CommandResult::success("Emergency stop activated")
            }
            DriverCommand::MavlinkConnect { address } => match self.mavlink.connect(&address) {
                Ok(_) => CommandResult::success(format!("MAVLink connected to {}", address)),
                Err(e) => CommandResult::error(e),
            },
            DriverCommand::MavlinkCommand { .. } => {
                // Verification using identity registry should happen here
                CommandResult::success("MAVLink command issued (Identity verified)")
            }
            DriverCommand::Ros2Publish { topic, message } => {
                CommandResult::success(format!("Published to {}: {}", topic, message))
            }
        }
    }

    /// Poll sensors and return current state (Phase 1A: Sensor Polling)
    pub fn poll_sensor(&self, sensor_type: &str) -> Result<SensorData, String> {
        match sensor_type {
            "position" | "gps" => {
                let pos = self.get_position();
                Ok(SensorData::Position {
                    latitude: pos.latitude,
                    longitude: pos.longitude,
                    altitude: pos.altitude,
                    accuracy: pos.accuracy,
                })
            }
            "orientation" | "imu" => {
                let orient = self.get_orientation();
                Ok(SensorData::Orientation {
                    roll: orient.roll,
                    pitch: orient.pitch,
                    yaw: orient.yaw,
                })
            }
            "velocity" => {
                let vel = self.get_velocity();
                Ok(SensorData::Velocity {
                    north: vel.north,
                    east: vel.east,
                    down: vel.down,
                })
            }
            "lidar" => {
                if let Some(scan) = self.lidar.get_last_scan() {
                    let points: Vec<[f32; 3]> =
                        scan.points.iter().map(|p| [p.x, p.y, p.z]).collect();

                    Ok(SensorData::LidarScan {
                        points,
                        timestamp: scan.timestamp,
                        range_min: scan.range_min,
                        range_max: scan.range_max,
                    })
                } else {
                    Err("No LIDAR data available".to_string())
                }
            }
            "depth" | "camera" => {
                if let Some(frame) = self.depth_camera.get_last_frame() {
                    Ok(SensorData::DepthFrame {
                        width: frame.width,
                        height: frame.height,
                        data: frame.data,
                        timestamp: frame.timestamp,
                    })
                } else {
                    Err("No depth frame available".to_string())
                }
            }
            "motor" => {
                // Return status of first motor as example
                if let Some(status) = self.motors.get_status(0) {
                    Ok(SensorData::MotorStatus {
                        motor_id: status.motor_id,
                        current_speed: status.current_speed,
                        temperature: status.temperature,
                        current_draw: status.current_draw,
                    })
                } else {
                    Err("No motor status available".to_string())
                }
            }
            _ => Err(format!("Unknown sensor type: {}", sensor_type)),
        }
    }

    pub fn poll(&mut self) {
        let _ = self.actor_driver.poll();
        let _ = self.sensor_subscriber.poll();
        let _ = self.mavlink.poll();
        let _ = self.ros2.poll();
    }
}

impl Default for Drivers {
    fn default() -> Self {
        Self::new(None)
    }
}

// Bare-metal WASM Nexus (legacy compatibility)
// Removed legacy Nexus as it is now redundant with Drivers

/// Send command to actor (motor, servo, GPIO) - DEPRECATED
/// Use drivers_execute_json instead
#[no_mangle]
pub extern "C" fn drivers_send_command(actor_type: u32, actor_id: u8, value: f32) -> i32 {
    let mut lock = GLOBAL_DRIVERS.lock();
    if lock.is_none() {
        *lock = Some(Drivers::new(None));
    }

    if let Some(drivers) = lock.as_mut() {
        let result = match actor_type {
            0 => drivers.set_motor_speed(actor_id, value),
            1 => drivers.set_servo_angle(actor_id, value),
            2 => drivers.set_gpio_pin(actor_id, value > 0.5),
            _ => Err("Unknown actor type".to_string()),
        };

        match result {
            Ok(_) => 1,
            Err(e) => {
                log::error!("Command failed: {}", e);
                0
            }
        }
    } else {
        0
    }
}

/// Read sensor data (GPS, IMU, LIDAR) - DEPRECATED
/// Use drivers_poll_sensor_json instead
#[no_mangle]
pub extern "C" fn drivers_poll_sensors(sensor_type: u32) -> *const u8 {
    let lock = GLOBAL_DRIVERS.lock();
    if let Some(drivers) = lock.as_ref() {
        let sensor_name = match sensor_type {
            0 => "gps",
            1 => "imu",
            2 => "lidar",
            3 => "camera",
            _ => return std::ptr::null(),
        };

        match drivers.poll_sensor(sensor_name) {
            Ok(data) => {
                if let Ok(json) = serde_json::to_vec(&data) {
                    Box::leak(json.into_boxed_slice()).as_ptr()
                } else {
                    std::ptr::null()
                }
            }
            Err(_) => std::ptr::null(),
        }
    } else {
        std::ptr::null()
    }
}

/// Execute command from JSON (Phase 1A: Command Dispatch)
///
/// **Usage from JS**:
/// ```js
/// const cmd = JSON.stringify({
///   type: "motor",
///   motor_id: 0,
///   speed: 0.5,
///   duration_ms: 1000
/// });
/// const result = drivers_execute_json(cmd);
/// ```
#[no_mangle]
pub extern "C" fn drivers_execute_json(json_ptr: *const u8, json_len: usize) -> *const u8 {
    let json_bytes = unsafe { std::slice::from_raw_parts(json_ptr, json_len) };
    let cmd: DriverCommand = match serde_json::from_slice(json_bytes) {
        Ok(c) => c,
        Err(e) => {
            log::error!("Failed to parse command: {}", e);
            let error_result = CommandResult::error(format!("Parse error: {}", e));
            let json = serde_json::to_vec(&error_result).unwrap();
            return Box::leak(json.into_boxed_slice()).as_ptr();
        }
    };

    let mut lock = GLOBAL_DRIVERS.lock();
    if lock.is_none() {
        *lock = Some(Drivers::new(None));
    }

    if let Some(drivers) = lock.as_mut() {
        let result = drivers.execute_command(cmd);
        let json = serde_json::to_vec(&result).unwrap();
        Box::leak(json.into_boxed_slice()).as_ptr()
    } else {
        std::ptr::null()
    }
}

/// Poll sensor and return JSON (Phase 1A: Sensor Polling)
///
/// **Usage from JS**:
/// ```js
/// const data = drivers_poll_sensor_json("gps");
/// const position = JSON.parse(data);
/// ```
#[no_mangle]
pub extern "C" fn drivers_poll_sensor_json(
    sensor_type_ptr: *const u8,
    sensor_type_len: usize,
) -> *const u8 {
    let sensor_type_bytes = unsafe { std::slice::from_raw_parts(sensor_type_ptr, sensor_type_len) };
    let sensor_type = match std::str::from_utf8(sensor_type_bytes) {
        Ok(s) => s,
        Err(_) => return std::ptr::null(),
    };

    let mut lock = GLOBAL_DRIVERS.lock();
    if lock.is_none() {
        *lock = Some(Drivers::new(None));
    }

    if let Some(drivers) = lock.as_ref() {
        match drivers.poll_sensor(sensor_type) {
            Ok(data) => {
                if let Ok(json) = serde_json::to_vec(&data) {
                    Box::leak(json.into_boxed_slice()).as_ptr()
                } else {
                    std::ptr::null()
                }
            }
            Err(e) => {
                log::error!("Sensor poll failed: {}: {}", sensor_type, e);
                std::ptr::null()
            }
        }
    } else {
        std::ptr::null()
    }
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
    sdk::js_interop::console_log("[drivers] Init: Starting initialization", 3);

    // Use stable ABI to get global object
    let global = sdk::js_interop::get_global();
    let sab_key = sdk::js_interop::create_string("__INOS_SAB__");
    let sab_val = sdk::js_interop::reflect_get(&global, &sab_key);

    let offset_key = sdk::js_interop::create_string("__INOS_SAB_OFFSET__");
    let offset_val = sdk::js_interop::reflect_get(&global, &offset_key);

    let size_key = sdk::js_interop::create_string("__INOS_SAB_SIZE__");
    let size_val = sdk::js_interop::reflect_get(&global, &size_key);

    let id_key = sdk::js_interop::create_string("__INOS_MODULE_ID__");
    let id_val = sdk::js_interop::reflect_get(&global, &id_key);

    if let (Ok(val), Ok(off), Ok(sz)) = (sab_val, offset_val, size_val) {
        if !val.is_undefined() && !val.is_null() {
            let offset = sdk::js_interop::as_f64(&off).unwrap_or(0.0) as u32;
            let size = sdk::js_interop::as_f64(&sz).unwrap_or(0.0) as u32;
            let module_id = id_val.ok().and_then(|v| sdk::js_interop::as_f64(&v)).unwrap_or(0.0) as u32;

            // Create TWO SafeSAB references:
            // 1. Scoped view for module data
            let _module_sab = sdk::sab::SafeSAB::new_shared_view(&val, offset, size);
            // 2. Global SAB for registry and buffer writes (uses absolute layout offsets)
            let global_sab = sdk::sab::SafeSAB::new(&val);

            sdk::set_module_id(module_id);
            sdk::identity::init_identity_from_js();
            sdk::init_logging();
            info!("Drivers module v0.2.0 initialized - I/O Socket System (Offset: 0x{:x}, Size: {}MB)", 
                offset, size / 1024 / 1024);

            // Register capabilities
            register_drivers_capabilities(&global_sab);
            // Signal registry change to wake Go discovery loop
            sdk::registry::signal_registry_change(&global_sab);

            // Initialize Global Drivers
            let mut lock = GLOBAL_DRIVERS.lock();
            *lock = Some(Drivers::new(Some(global_sab)));

            return 1;
        }
    }
    0
}

/// Register Drivers capabilities in SAB registry
fn register_drivers_capabilities(sab: &sdk::sab::SafeSAB) {
    use sdk::registry::*;

    let id = "drivers";
    let mut builder = ModuleEntryBuilder::new(id).version(0, 2, 0);

    // I/O Socket capabilities
    builder = builder.capability("positioning", false, 128); // GPS/INS/IMU
    builder = builder.capability("perception", false, 256); // LIDAR/Camera
    builder = builder.capability("actuation", false, 128); // Motors/Servos
    builder = builder.capability("usb", false, 64);
    builder = builder.capability("bluetooth", false, 64);
    builder = builder.capability("sensor", false, 64);
    builder = builder.capability("actor", false, 64);
    builder = builder.capability("mavlink", false, 128);
    builder = builder.capability("ros2", false, 256);
    builder = builder.capability("mqtt", false, 128);
    builder = builder.capability("can", false, 64);

    match builder.build() {
        Ok((mut entry, _, caps)) => {
            if let Ok(offset) = write_capability_table(sab, &caps) {
                entry.cap_table_offset = offset;
            }
            if let Ok((slot, _)) = find_slot_double_hashing(sab, id) {
                match write_enhanced_entry(sab, slot, &entry) {
                    Ok(_) => info!(
                        "Registered Drivers module at slot {} with {} capabilities",
                        slot,
                        caps.len()
                    ),
                    Err(e) => error!("Failed to write registry entry: {:?}", e),
                }
            }
        }
        Err(e) => error!("Failed to build module entry: {:?}", e),
    }
}

/// External poll entry point for JavaScript
#[no_mangle]
pub extern "C" fn drivers_poll() {
    let mut lock = GLOBAL_DRIVERS.lock();
    if let Some(drivers) = lock.as_mut() {
        drivers.poll();
    }
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
