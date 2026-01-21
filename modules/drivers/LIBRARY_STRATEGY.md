# Drivers Module: Library Strategy

**Philosophy**: Proxy to comprehensive Rust libraries, don't reinvent the wheel

---

## Core Libraries (Production-Ready)

### Hardware Abstraction
```toml
embedded-hal = "1.0"           # Generic traits (I2C, SPI, GPIO)
embedded-hal-async = "1.0"     # Async traits
serialport = "5.0"             # Cross-platform serial
```

### Positioning & Navigation
```toml
imu-fusion-rs = "0.2"          # IMU sensor fusion (Kalman)
nmea = "0.6"                   # GPS NMEA parser
ublox = "0.4"                  # u-blox GPS binary
nalgebra = "0.32"              # Linear algebra
```

### Robotics Protocols
```toml
mavlink = "0.13"               # Drone telemetry (no_std)
ros2-client = "0.7"            # Pure Rust ROS2 (no C!)
rustdds = "0.10"               # DDS middleware
```

### IoT & Communication
```toml
rumqttc = "0.24"               # MQTT (async, robust)
socketcan = "3.3"              # CAN bus (Linux)
embedded-can = "0.4"           # CAN HAL (embedded)
btleplug = "0.11"              # Bluetooth LE (WASM support)
```

---

## Architecture

```
┌─────────────────────────────────────┐
│     Hardware (Sensors/Actuators)    │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│    Rust Libraries (embedded-hal)    │
│  - imu-fusion-rs (sensor fusion)    │
│  - mavlink (drone protocol)         │
│  - ros2-client (robotics)           │
│  - rumqttc (IoT)                    │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│      Drivers Module (Proxy)         │
│  - Normalize to Cap'n Proto         │
│  - Zero-copy via SAB                │
│  - C ABI exports (no wasm-bindgen)  │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│    Go Supervisor (Orchestration)    │
└─────────────────────────────────────┘
```

---

## Implementation Pattern

### Example: GPS/IMU Sensor Fusion

```rust
// Use existing library
use imu_fusion_rs::{FusionAhrs, FusionVector};
use nmea::Nmea;

pub struct PositioningSystem {
    fusion: FusionAhrs,
    gps: Nmea,
}

impl PositioningSystem {
    pub fn update(&mut self, imu_data: ImuData, gps_data: GpsData) -> SensorFrame {
        // Library does the heavy lifting
        self.fusion.update(
            FusionVector::new(imu_data.gyro[0], imu_data.gyro[1], imu_data.gyro[2]),
            FusionVector::new(imu_data.accel[0], imu_data.accel[1], imu_data.accel[2]),
            0.01, // 100Hz
        );
        
        let orientation = self.fusion.quaternion();
        
        // Convert to Cap'n Proto
        SensorFrame {
            source_id: "positioning",
            timestamp_ns: get_utc_ns(),
            data: SensorData::GpsPosition(GPS {
                latitude: gps_data.lat,
                longitude: gps_data.lon,
                altitude: gps_data.alt,
                accuracy: gps_data.accuracy,
            }),
        }
    }
}
```

**Effort**: 4 hours (vs 40 hours custom implementation)

---

## Benefits

✅ **Minimal Code**: Proxy pattern = thin wrapper  
✅ **Battle-Tested**: Use production-grade libraries  
✅ **Fast Development**: 3 weeks vs 15 weeks  
✅ **Easy Maintenance**: Upstream bug fixes  
✅ **Community Support**: Active ecosystems  

---

## Timeline

**Week 1**: Infrastructure
- Remove wasm-bindgen
- C ABI exports
- Cap'n Proto integration
- SAB zero-copy

**Week 2**: Positioning & Perception
- GPS/IMU fusion (imu-fusion-rs)
- LIDAR integration
- Sensor data normalization

**Week 3**: Protocols
- MAVLink (drones)
- ROS2 (robots)
- MQTT (IoT)
- CAN bus (vehicles)

**Total**: ~120 hours

---

## Success Metrics

- ✅ 100% pure Rust (no wasm-bindgen)
- ✅ <10cm positioning accuracy (RTK GPS)
- ✅ <1ms command latency
- ✅ MAVLink/ROS2 protocols working
- ✅ Zero-copy via SAB
- ✅ Cross-platform (Linux, embedded)
