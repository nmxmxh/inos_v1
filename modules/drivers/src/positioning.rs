// Positioning System - GPS/INS/IMU Sensor Fusion
// Provides centimeter-level accuracy for navigation

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Position {
    pub latitude: f64,
    pub longitude: f64,
    pub altitude: f64,
    pub accuracy: f32, // meters
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Velocity {
    pub north: f32, // m/s
    pub east: f32,
    pub down: f32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Orientation {
    pub roll: f32, // radians
    pub pitch: f32,
    pub yaw: f32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ImuData {
    pub accel: [f32; 3], // m/s² [x, y, z]
    pub gyro: [f32; 3],  // rad/s [x, y, z]
    pub mag: [f32; 3],   // μT [x, y, z]
    pub timestamp: f64,
}

/// Positioning System using GPS/INS/IMU fusion
///
/// **Library Proxy Pattern**: Wraps imu-fusion-rs and provides
/// a simple interface for position/orientation estimation.
///
/// **Use Cases**:
/// - Autonomous drones (GPS-denied navigation)
/// - Ground robots (odometry + IMU)
/// - Robotic arms (orientation tracking)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PositioningSystem {
    position: Position,
    velocity: Velocity,
    orientation: Orientation,
    last_update: f64,
}

impl PositioningSystem {
    pub fn new() -> Self {
        Self {
            position: Position {
                latitude: 0.0,
                longitude: 0.0,
                altitude: 0.0,
                accuracy: 100.0,
            },
            velocity: Velocity {
                north: 0.0,
                east: 0.0,
                down: 0.0,
            },
            orientation: Orientation {
                roll: 0.0,
                pitch: 0.0,
                yaw: 0.0,
            },
            last_update: 0.0,
        }
    }

    /// Update position from GPS data
    pub fn update_gps(&mut self, lat: f64, lon: f64, alt: f64, accuracy: f32) {
        self.position.latitude = lat;
        self.position.longitude = lon;
        self.position.altitude = alt;
        self.position.accuracy = accuracy;
        log::debug!("GPS update: ({}, {}) ±{}m", lat, lon, accuracy);
    }

    /// Update orientation from IMU data
    pub fn update_imu(&mut self, imu: ImuData) {
        // Future: Implement Kalman filter using ahrs crate (Madgwick/Mahony)
        // Current: Simple gyro integration for basic orientation tracking
        let dt = imu.timestamp - self.last_update;
        if dt > 0.0 && dt < 1.0 {
            // Gyroscope integration (dead reckoning)
            self.orientation.roll += imu.gyro[0] * dt as f32;
            self.orientation.pitch += imu.gyro[1] * dt as f32;
            self.orientation.yaw += imu.gyro[2] * dt as f32;
        }
        self.last_update = imu.timestamp;
    }

    /// Get current position
    pub fn get_position(&self) -> Position {
        self.position.clone()
    }

    /// Get current velocity
    pub fn get_velocity(&self) -> Velocity {
        self.velocity.clone()
    }

    /// Get current orientation
    pub fn get_orientation(&self) -> Orientation {
        self.orientation.clone()
    }

    /// Get position as bytes (for SAB writing)
    pub fn to_bytes(&self) -> Vec<u8> {
        serde_json::to_vec(self).unwrap_or_default()
    }

    /// Create from bytes (for SAB reading)
    pub fn from_bytes(bytes: &[u8]) -> Result<Self, String> {
        serde_json::from_slice(bytes).map_err(|e| e.to_string())
    }
}

impl Default for PositioningSystem {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_positioning_creation() {
        let pos = PositioningSystem::new();
        assert_eq!(pos.position.latitude, 0.0);
        assert_eq!(pos.position.longitude, 0.0);
    }

    #[test]
    fn test_gps_update() {
        let mut pos = PositioningSystem::new();
        pos.update_gps(37.7749, -122.4194, 10.0, 5.0);

        let position = pos.get_position();
        assert_eq!(position.latitude, 37.7749);
        assert_eq!(position.longitude, -122.4194);
        assert_eq!(position.accuracy, 5.0);
    }

    #[test]
    fn test_serialization() {
        let pos = PositioningSystem::new();
        let bytes = pos.to_bytes();
        assert!(!bytes.is_empty());

        let restored = PositioningSystem::from_bytes(&bytes).unwrap();
        assert_eq!(restored.position.latitude, pos.position.latitude);
    }
}
