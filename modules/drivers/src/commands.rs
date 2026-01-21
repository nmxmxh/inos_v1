// Generic Command System for Drivers Module
// Provides type-safe, JSON-based command dispatch

use serde::{Deserialize, Serialize};

/// Generic driver command enum
///
/// **Design**: Uses serde's tagged enum for type-safe JSON serialization
/// **Usage**: Commands can be sent from JS as JSON, deserialized, and dispatched
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum DriverCommand {
    // Positioning commands
    UpdateGps {
        lat: f64,
        lon: f64,
        alt: f64,
        accuracy: f32,
    },
    UpdateImu {
        accel: [f32; 3],
        gyro: [f32; 3],
        mag: [f32; 3],
        timestamp: f64,
    },

    // Actuation commands
    Motor {
        motor_id: u8,
        speed: f32,
        duration_ms: u32,
    },
    Servo {
        servo_id: u8,
        angle: f32,
        speed: f32,
    },
    Gpio {
        pin: u8,
        state: bool,
    },

    // Perception commands
    ProcessLidarScan {
        raw_data: Vec<u8>,
        timestamp: f64,
    },
    ProcessDepthFrame {
        width: u32,
        height: u32,
        data: Vec<u16>,
        timestamp: f64,
    },

    // Robotics commands
    MavlinkConnect {
        address: String,
    },
    MavlinkCommand {
        system_id: u8,
        component_id: u8,
        command_id: u16,
        params: [f32; 7],
    },
    Ros2Publish {
        topic: String,
        message: String,
    },

    // Control commands
    EmergencyStop,
}

/// Generic sensor data response
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum SensorData {
    Position {
        latitude: f64,
        longitude: f64,
        altitude: f64,
        accuracy: f32,
    },
    Orientation {
        roll: f32,
        pitch: f32,
        yaw: f32,
    },
    Velocity {
        north: f32,
        east: f32,
        down: f32,
    },
    LidarScan {
        points: Vec<[f32; 3]>, // [x, y, z]
        timestamp: f64,
        range_min: f32,
        range_max: f32,
    },
    DepthFrame {
        width: u32,
        height: u32,
        data: Vec<u16>,
        timestamp: f64,
    },
    MotorStatus {
        motor_id: u8,
        current_speed: f32,
        temperature: f32,
        current_draw: f32,
    },
    Error {
        message: String,
    },
}

/// Command execution result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CommandResult {
    pub success: bool,
    pub message: String,
    pub data: Option<SensorData>,
}

impl CommandResult {
    pub fn success(message: impl Into<String>) -> Self {
        Self {
            success: true,
            message: message.into(),
            data: None,
        }
    }

    pub fn success_with_data(message: impl Into<String>, data: SensorData) -> Self {
        Self {
            success: true,
            message: message.into(),
            data: Some(data),
        }
    }

    pub fn error(message: impl Into<String>) -> Self {
        let msg = message.into();
        Self {
            success: false,
            message: msg.clone(),
            data: Some(SensorData::Error { message: msg }),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_command_serialization() {
        let cmd = DriverCommand::Motor {
            motor_id: 0,
            speed: 0.5,
            duration_ms: 1000,
        };

        let json = serde_json::to_string(&cmd).unwrap();
        assert!(json.contains("\"type\":\"motor\""));
        assert!(json.contains("\"motor_id\":0"));

        let deserialized: DriverCommand = serde_json::from_str(&json).unwrap();
        match deserialized {
            DriverCommand::Motor {
                motor_id, speed, ..
            } => {
                assert_eq!(motor_id, 0);
                assert_eq!(speed, 0.5);
            }
            _ => panic!("Wrong command type"),
        }
    }

    #[test]
    fn test_sensor_data_serialization() {
        let data = SensorData::Position {
            latitude: 37.7749,
            longitude: -122.4194,
            altitude: 10.0,
            accuracy: 5.0,
        };

        let json = serde_json::to_string(&data).unwrap();
        assert!(json.contains("\"type\":\"position\""));

        let deserialized: SensorData = serde_json::from_str(&json).unwrap();
        match deserialized {
            SensorData::Position { latitude, .. } => {
                assert_eq!(latitude, 37.7749);
            }
            _ => panic!("Wrong data type"),
        }
    }

    #[test]
    fn test_command_result() {
        let result = CommandResult::success("Motor started");
        assert!(result.success);
        assert_eq!(result.message, "Motor started");

        let json = serde_json::to_string(&result).unwrap();
        let deserialized: CommandResult = serde_json::from_str(&json).unwrap();
        assert!(deserialized.success);
    }
}
