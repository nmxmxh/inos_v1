# Drivers Module: Pure I/O Socket System

**Date**: 2026-01-01  
**Philosophy**: Drivers = Universal I/O Sockets (Input: Sensors, Output: Actors)  
**Architecture**: Pure Rust, No wasm-bindgen, Cap'n Proto schemas

---

## Core Concept

**Drivers are I/O Sockets**, not device managers:

```
┌─────────────────────────────────────────────┐
│              INOS Kernel (Go)               │
│  ┌─────────────────────────────────────┐   │
│  │      Supervisor (Orchestrator)       │   │
│  └──────────────┬──────────────────────┘   │
│                 │ SAB (Zero-Copy)           │
└─────────────────┼─────────────────────────┘
                  │
┌─────────────────┼─────────────────────────┐
│                 ▼                          │
│         Drivers Module (Rust)             │
│  ┌──────────────────────────────────┐    │
│  │  INPUT SOCKETS (Sensors)         │    │
│  │  - GPS/INS/IMU (Position)        │    │
│  │  - LIDAR/Camera (Perception)     │    │
│  │  - MAVLink (Telemetry)           │    │
│  │  - ROS2 Topics (Sensor Data)     │    │
│  │  - MQTT (IoT Sensors)            │    │
│  │  - CAN Bus (Vehicle Data)        │    │
│  └──────────────────────────────────┘    │
│                                            │
│  ┌──────────────────────────────────┐    │
│  │  OUTPUT SOCKETS (Actors)         │    │
│  │  - Motor Control (Movement)      │    │
│  │  - Servo Commands (Actuation)    │    │
│  │  - MAVLink (Commands)            │    │
│  │  - ROS2 Actions (Robot Control)  │    │
│  │  - GPIO (Hardware Control)       │    │
│  │  - CAN Bus (Vehicle Control)     │    │
│  └──────────────────────────────────┘    │
└─────────────────────────────────────────┘
```

---

## Cap'n Proto Schema Integration

### Actor Schema (Output Socket)

From `protocols/schemas/io/v1/actor.capnp`:

```capnp
interface Actor {
  struct Command {
    id @0 :Text;
    targetId @1 :Text;
    timestampNs @2 :Int64;
    metadata @3 :Base.Base.Metadata;
    
    union {
      rawBytes @4 :Data;
      
      # Robotics / Motion (CRITICAL FOR MILITARY)
      moveTo @5 :Pose3D;        # Target position + orientation
      velocity @6 :Vector3;      # Linear velocity command
      torque @7 :List(Float32); # Joint torques (robot arms, legs)
      
      # Visual / Holographic
      displayFrame @8 :Data;
      setHologram @9 :HologramPatch;
      
      # Environment
      gpioSet @10 :List(GPIOState);
    }
  }
  
  struct Pose3D {
    position @0 :Vector3;    # X, Y, Z (meters)
    rotation @1 :Quaternion; # Orientation (quaternion for stability)
  }
}
```

**Usage**: Drivers translate high-level commands to hardware protocols

### Sensor Schema (Input Socket)

From `protocols/schemas/io/v1/sensor.capnp`:

```capnp
interface IO {
  struct SensorFrame {
    sourceId @0 :Text;
    metadata @1: Base.Base.Metadata;
    
    # Hard Real-Time Precision (CRITICAL FOR MILITARY)
    timestampNs @2 :Int64;      # UTC Nanoseconds (Absolute)
    monotonicNs @3 :Int64;      # System Monotonic (Intervals)
    
    union {
      rawBytes @4 :Data;
      
      # Low-Level Telemetry (SPATIAL AWARENESS)
      imu6Axis @5 :List(Float32);   # [ax,ay,az,gx,gy,gz]
      magnetometer @6 :List(Float32);
      gpsPosition @7 :GPS;
      
      # High-Bandwidth Perception
      depthMap @8 :Data;            # Compressed Depth
      lidarScan @9 :List(Point3D);  # Point Clouds
      holographicFrame @10 :Data;   # Volumetric Data
      
      # Audio / Video
      audioChunk @11 :Data;
      videoFrame @12 :Data;
      
      custom @13 :CustomData;
    }
  }
  
  struct GPS {
    latitude @0 :Float64;
    longitude @1 :Float64;
    altitude @2 :Float64;
    accuracy @3 :Float32;
  }
}
```

**Usage**: Drivers normalize hardware data to Cap'n Proto format

---

## Military/Spatial/Movement Requirements

### 1. Positioning & Navigation (GPS/INS/IMU)

**Requirement**: Centimeter-level accuracy, GPS-denied operation

**Sensor Fusion Stack**:
```
GPS (Absolute Position) ──┐
                          ├──> Kalman Filter ──> Fused Position
IMU/INS (Relative Motion) ┘
```

**Implementation**:
```rust
// modules/drivers/src/positioning/mod.rs
pub struct PositioningSystem {
    gps: GpsReceiver,
    imu: ImuSensor,
    ins: InertialNavSystem,
    fusion: KalmanFilter,
}

impl PositioningSystem {
    pub fn get_position(&mut self) -> SensorFrame {
        // Read GPS
        let gps_data = self.gps.read();
        
        // Read IMU
        let imu_data = self.imu.read();
        
        // Sensor fusion
        let fused = self.fusion.update(gps_data, imu_data);
        
        // Convert to Cap'n Proto
        SensorFrame {
            source_id: "positioning_system",
            timestamp_ns: get_utc_ns(),
            monotonic_ns: get_monotonic_ns(),
            data: SensorData::GpsPosition(GPS {
                latitude: fused.lat,
                longitude: fused.lon,
                altitude: fused.alt,
                accuracy: fused.accuracy,
            }),
        }
    }
}
```

**Protocols**:
- ✅ NMEA (GPS standard)
- ✅ UBX (u-blox binary)
- ✅ MAVLink (drone telemetry)

### 2. Spatial Awareness (LIDAR/Depth/Vision)

**Requirement**: Real-time obstacle detection, 3D mapping

**Sensors**:
- LIDAR (point clouds)
- Depth cameras (RGB-D)
- Stereo vision
- Radar

**Implementation**:
```rust
// modules/drivers/src/perception/lidar.rs
pub struct LidarDriver {
    device: LidarDevice,
    point_cloud_buffer: Vec<Point3D>,
}

impl LidarDriver {
    pub fn scan(&mut self) -> SensorFrame {
        // Read LIDAR scan
        let points = self.device.read_scan();
        
        // Convert to Cap'n Proto Point3D
        let point_cloud: Vec<Point3D> = points.iter().map(|p| Point3D {
            x: p.x,
            y: p.y,
            z: p.z,
            intensity: p.intensity,
        }).collect();
        
        SensorFrame {
            source_id: "lidar_front",
            timestamp_ns: get_utc_ns(),
            monotonic_ns: get_monotonic_ns(),
            data: SensorData::LidarScan(point_cloud),
        }
    }
}
```

**Protocols**:
- ✅ Velodyne (LIDAR standard)
- ✅ ROS2 PointCloud2
- ✅ Custom binary formats

### 3. Movement Control (Motors/Servos/Actuators)

**Requirement**: Precise, low-latency motor control

**Actors**:
- DC motors (wheels, propellers)
- Servo motors (joints, gimbals)
- Stepper motors (precision positioning)
- Hydraulic actuators

**Implementation**:
```rust
// modules/drivers/src/actuation/motor.rs
pub struct MotorController {
    motors: Vec<Motor>,
    can_bus: CanBus,
}

impl MotorController {
    pub fn execute_command(&mut self, cmd: ActorCommand) {
        match cmd.data {
            CommandData::MoveTo(pose) => {
                // Inverse kinematics
                let joint_angles = self.inverse_kinematics(pose);
                
                // Send to motors via CAN bus
                for (motor_id, angle) in joint_angles.iter().enumerate() {
                    self.can_bus.send_position_command(motor_id, *angle);
                }
            }
            CommandData::Velocity(vel) => {
                // Direct velocity control
                self.set_wheel_velocities(vel);
            }
            CommandData::Torque(torques) => {
                // Direct torque control (force feedback)
                for (motor_id, torque) in torques.iter().enumerate() {
                    self.can_bus.send_torque_command(motor_id, *torque);
                }
            }
            _ => {}
        }
    }
}
```

**Protocols**:
- ✅ CAN bus (automotive/robotics standard)
- ✅ PWM (servo control)
- ✅ MAVLink (drone motors)
- ✅ ROS2 JointTrajectory

---

## Pure Rust Implementation (No wasm-bindgen)

### Current Issue

The existing code uses `wasm-bindgen` and `JsValue`:

```rust
// ❌ REMOVE THIS
use sdk::JsValue;

impl Nexus {
    pub fn new(sab: &sdk::JsValue) -> Self { ... }
    pub fn poll(&mut self) -> Result<(), JsValue> { ... }
}
```

### New Approach: Pure Rust with C ABI

```rust
// ✅ NEW APPROACH
// modules/drivers/src/lib.rs

// NO wasm-bindgen imports!
use sdk::sab::SafeSAB;

/// Memory allocator (C ABI)
#[no_mangle]
pub extern "C" fn drivers_alloc(size: usize) -> *mut u8 {
    let mut buf = Vec::with_capacity(size);
    let ptr = buf.as_mut_ptr();
    std::mem::forget(buf);
    ptr
}

/// Initialize with SAB (C ABI)
#[no_mangle]
pub extern "C" fn drivers_init_with_sab() -> i32 {
    // Get SAB from global without wasm-bindgen
    let global = sdk::js_interop::get_global();
    let sab_key = sdk::js_interop::create_string("__INOS_SAB__");
    let sab_val = sdk::js_interop::reflect_get(&global, &sab_key);
    
    if let Ok(val) = sab_val {
        if !val.is_undefined() && !val.is_null() {
            // Create SafeSAB (pure Rust)
            let sab = SafeSAB::new(val);
            
            // Initialize drivers
            DRIVERS.lock().unwrap().replace(Drivers::new(sab));
            
            return 1;
        }
    }
    0
}

/// Poll sensors (C ABI)
#[no_mangle]
pub extern "C" fn drivers_poll_sensors(buffer: *mut u8, buffer_len: usize) -> i32 {
    let drivers = DRIVERS.lock().unwrap();
    if let Some(ref drivers) = *drivers {
        // Read all sensors
        let frames = drivers.read_sensors();
        
        // Serialize to Cap'n Proto
        let serialized = capnp::serialize_packed::write_message_to_words(&frames);
        
        // Copy to buffer
        let len = serialized.len().min(buffer_len);
        unsafe {
            std::ptr::copy_nonoverlapping(serialized.as_ptr(), buffer, len);
        }
        
        return len as i32;
    }
    0
}

/// Send actor command (C ABI)
#[no_mangle]
pub extern "C" fn drivers_send_command(buffer: *const u8, buffer_len: usize) -> i32 {
    let drivers = DRIVERS.lock().unwrap();
    if let Some(ref mut drivers) = *drivers {
        // Deserialize Cap'n Proto command
        let slice = unsafe { std::slice::from_raw_parts(buffer, buffer_len) };
        let reader = capnp::serialize_packed::read_message(
            slice,
            capnp::message::ReaderOptions::new()
        ).ok()?;
        
        let cmd: ActorCommand = reader.get_root().ok()?;
        
        // Execute command
        drivers.execute_command(cmd);
        
        return 1;
    }
    0
}
```

---

## Architecture

### Module Structure

```
modules/drivers/
├── Cargo.toml
├── build.rs                    # Cap'n Proto code generation
├── src/
│   ├── lib.rs                  # C ABI exports (no wasm-bindgen)
│   ├── drivers.rs              # Main Drivers struct
│   │
│   ├── positioning/            # GPS/INS/IMU
│   │   ├── mod.rs
│   │   ├── gps.rs              # GPS receiver
│   │   ├── imu.rs              # IMU sensor
│   │   ├── ins.rs              # Inertial navigation
│   │   └── fusion.rs           # Kalman filter
│   │
│   ├── perception/             # LIDAR/Vision
│   │   ├── mod.rs
│   │   ├── lidar.rs            # LIDAR driver
│   │   ├── camera.rs           # Camera driver
│   │   └── depth.rs            # Depth sensor
│   │
│   ├── actuation/              # Motors/Servos
│   │   ├── mod.rs
│   │   ├── motor.rs            # Motor controller
│   │   ├── servo.rs            # Servo controller
│   │   └── kinematics.rs       # IK/FK
│   │
│   ├── protocols/              # Communication protocols
│   │   ├── mod.rs
│   │   ├── mavlink.rs          # MAVLink (drones)
│   │   ├── ros2.rs             # ROS2/DDS (robots)
│   │   ├── mqtt.rs             # MQTT (IoT)
│   │   ├── can.rs              # CAN bus (automotive)
│   │   └── nmea.rs             # NMEA (GPS)
│   │
│   ├── sockets/                # I/O socket abstraction
│   │   ├── mod.rs
│   │   ├── input.rs            # Sensor input sockets
│   │   └── output.rs           # Actor output sockets
│   │
│   └── capnp_generated/        # Generated Cap'n Proto code
│       ├── actor_capnp.rs
│       └── sensor_capnp.rs
```

### Main Drivers Struct

```rust
// modules/drivers/src/drivers.rs
use crate::positioning::PositioningSystem;
use crate::perception::{LidarDriver, CameraDriver};
use crate::actuation::MotorController;
use crate::protocols::{MavlinkClient, Ros2Client};

pub struct Drivers {
    // Positioning
    positioning: PositioningSystem,
    
    // Perception
    lidar: Vec<LidarDriver>,
    cameras: Vec<CameraDriver>,
    
    // Actuation
    motors: MotorController,
    
    // Protocols
    mavlink: Option<MavlinkClient>,
    ros2: Option<Ros2Client>,
    
    // SAB for zero-copy
    sab: SafeSAB,
}

impl Drivers {
    pub fn new(sab: SafeSAB) -> Self {
        Self {
            positioning: PositioningSystem::new(),
            lidar: Vec::new(),
            cameras: Vec::new(),
            motors: MotorController::new(),
            mavlink: None,
            ros2: None,
            sab,
        }
    }
    
    /// Read all sensors and return Cap'n Proto frames
    pub fn read_sensors(&self) -> Vec<SensorFrame> {
        let mut frames = Vec::new();
        
        // Positioning
        frames.push(self.positioning.get_position());
        
        // LIDAR
        for lidar in &self.lidar {
            frames.push(lidar.scan());
        }
        
        // Cameras
        for camera in &self.cameras {
            frames.push(camera.capture());
        }
        
        // MAVLink telemetry
        if let Some(ref mavlink) = self.mavlink {
            frames.extend(mavlink.read_telemetry());
        }
        
        // ROS2 topics
        if let Some(ref ros2) = self.ros2 {
            frames.extend(ros2.read_topics());
        }
        
        frames
    }
    
    /// Execute actor command
    pub fn execute_command(&mut self, cmd: ActorCommand) {
        match cmd.target_id.as_str() {
            "motors" => self.motors.execute_command(cmd),
            "mavlink" => {
                if let Some(ref mut mavlink) = self.mavlink {
                    mavlink.send_command(cmd);
                }
            }
            "ros2" => {
                if let Some(ref mut ros2) = self.ros2 {
                    ros2.send_action(cmd);
                }
            }
            _ => {
                log::warn!("Unknown actor target: {}", cmd.target_id);
            }
        }
    }
}
```

---

## Dependencies

```toml
[package]
name = "drivers"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["cdylib", "rlib"]

[dependencies]
sdk = { path = "../sdk" }

# Cap'n Proto (NO wasm-bindgen!)
capnp = "0.19"

# Positioning
nmea = "0.6"               # GPS NMEA parser
ublox = "0.4"              # u-blox GPS binary protocol
nalgebra = "0.32"          # Linear algebra (Kalman filter)

# Robotics protocols
mavlink = "0.13"           # Drone telemetry
ros2-client = "0.7"        # ROS2 (pure Rust!)
rustdds = "0.10"           # DDS middleware

# IoT protocols
rumqttc = "0.24"           # MQTT

# Automotive
socketcan = "3.3"          # CAN bus (Linux)
embedded-can = "0.4"       # CAN HAL (embedded)

# Utilities
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"
log = "0.4"
parking_lot = "0.12"       # Fast mutex

# NO wasm-bindgen!
# NO js-sys!
# NO web-sys!

[build-dependencies]
capnpc = "0.19"            # Cap'n Proto compiler
```

---

## Integration with Kernel

### Go Supervisor

```go
// kernel/threads/supervisor/units/driver_supervisor.go
func (s *DriverSupervisor) Execute(job *Job) (*Result, error) {
    switch job.Method {
    case "sensor:read_all":
        return s.readAllSensors()
    case "actor:move_to":
        return s.sendMoveCommand(job.Params)
    case "actor:set_velocity":
        return s.sendVelocityCommand(job.Params)
    default:
        return nil, fmt.Errorf("unknown method: %s", job.Method)
    }
}

func (s *DriverSupervisor) readAllSensors() (*Result, error) {
    // Allocate buffer for Cap'n Proto data
    buffer := make([]byte, 1024*1024) // 1MB
    
    // Call Rust via C ABI
    length := drivers_poll_sensors(unsafe.Pointer(&buffer[0]), len(buffer))
    
    // Deserialize Cap'n Proto
    frames := deserializeSensorFrames(buffer[:length])
    
    // Process frames
    for _, frame := range frames {
        s.processSensorFrame(frame)
    }
    
    return &Result{Data: buffer[:length]}, nil
}
```

---

## Use Cases

### 1. Autonomous Drone

```rust
// Read GPS + IMU
let position = drivers.positioning.get_position();

// Read LIDAR for obstacle avoidance
let obstacles = drivers.lidar[0].scan();

// Send movement command
drivers.execute_command(ActorCommand {
    target_id: "mavlink",
    data: CommandData::MoveTo(Pose3D {
        position: Vector3 { x: 10.0, y: 20.0, z: 5.0 },
        rotation: Quaternion { x: 0.0, y: 0.0, z: 0.0, w: 1.0 },
    }),
});
```

### 2. Ground Robot

```rust
// Read wheel encoders + IMU
let odometry = drivers.positioning.get_odometry();

// Read LIDAR for mapping
let scan = drivers.lidar[0].scan();

// Send velocity command
drivers.execute_command(ActorCommand {
    target_id: "motors",
    data: CommandData::Velocity(Vector3 {
        x: 1.0,  // Forward 1 m/s
        y: 0.0,
        z: 0.0,
    }),
});
```

### 3. Robotic Arm

```rust
// Read joint positions
let joint_states = drivers.motors.get_joint_states();

// Send torque commands
drivers.execute_command(ActorCommand {
    target_id: "motors",
    data: CommandData::Torque(vec![0.5, 0.3, 0.2, 0.1, 0.0, 0.0]),
});
```

---

## Timeline

### Week 1: Core Infrastructure
- Day 1-2: Remove wasm-bindgen, implement C ABI
- Day 3-4: Cap'n Proto integration
- Day 5: SAB zero-copy I/O

### Week 2: Positioning & Perception
- Day 1-2: GPS/IMU drivers
- Day 3-4: Sensor fusion (Kalman filter)
- Day 5: LIDAR integration

### Week 3: Actuation & Protocols
- Day 1-2: Motor controllers
- Day 3-4: MAVLink/ROS2 integration
- Day 5: Testing & documentation

**Total**: 3 weeks, ~120 hours

---

## Success Criteria

- ✅ No wasm-bindgen usage
- ✅ Pure Rust with C ABI exports
- ✅ Cap'n Proto Actor/Sensor schemas
- ✅ GPS/INS/IMU sensor fusion
- ✅ LIDAR/camera integration
- ✅ Motor control working
- ✅ MAVLink/ROS2 protocols
- ✅ Zero-copy via SAB
- ✅ Military-grade positioning accuracy

---

## Conclusion

**Drivers = Universal I/O Sockets**

**Input Sockets** (Sensors):
- GPS/INS/IMU → Position
- LIDAR/Camera → Perception
- MAVLink/ROS2 → Telemetry

**Output Sockets** (Actors):
- Motors/Servos → Movement
- MAVLink/ROS2 → Commands
- GPIO/CAN → Control

**Architecture**:
- ✅ Pure Rust (no wasm-bindgen)
- ✅ C ABI exports
- ✅ Cap'n Proto schemas
- ✅ Zero-copy via SAB
- ✅ Military/spatial/movement focus

**Next Steps**:
1. Remove wasm-bindgen
2. Implement C ABI
3. Integrate Cap'n Proto
4. Add positioning systems
5. Add perception systems
6. Add actuation systems
