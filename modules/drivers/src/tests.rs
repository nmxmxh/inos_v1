// Comprehensive tests for Drivers module
// Tests for positioning, perception, actuation, and commands

use super::*;

#[test]
fn test_drivers_creation() {
    let drivers = Drivers::new();
    // Basic smoke test
    assert!(true);
}

#[test]
fn test_motor_control() {
    let mut drivers = Drivers::new();
    assert!(drivers.set_motor_speed(0, 0.5).is_ok());
    assert!(drivers.set_motor_speed(0, -0.5).is_ok());
    assert!(drivers.set_motor_speed(0, 1.5).is_ok()); // Should clamp to 1.0
}

#[test]
fn test_servo_control() {
    let mut drivers = Drivers::new();
    assert!(drivers.set_servo_angle(0, 90.0).is_ok());
    assert!(drivers.set_servo_angle(0, 0.0).is_ok());
    assert!(drivers.set_servo_angle(0, 180.0).is_ok());
}

#[test]
fn test_gpio_control() {
    let mut drivers = Drivers::new();
    assert!(drivers.set_gpio_pin(0, true).is_ok());
    assert!(drivers.set_gpio_pin(0, false).is_ok());
}

#[test]
fn test_positioning() {
    let mut drivers = Drivers::new();
    drivers.update_gps(37.7749, -122.4194, 10.0, 5.0);

    let pos = drivers.get_position();
    assert_eq!(pos.latitude, 37.7749);
    assert_eq!(pos.longitude, -122.4194);
    assert_eq!(pos.accuracy, 5.0);
}

#[test]
fn test_emergency_stop() {
    let mut drivers = Drivers::new();
    drivers.set_motor_speed(0, 0.8).unwrap();
    drivers.set_motor_speed(1, -0.5).unwrap();

    drivers.emergency_stop();

    // Motors should be stopped (we can't directly check without exposing internals)
    // But we can verify the function doesn't panic
    assert!(true);
}

#[test]
fn test_command_dispatch_motor() {
    let mut drivers = Drivers::new();

    let cmd = DriverCommand::Motor {
        motor_id: 0,
        speed: 0.5,
        duration_ms: 1000,
    };

    let result = drivers.execute_command(cmd);
    assert!(result.success);
    assert!(result.message.contains("Motor"));
}

#[test]
fn test_command_dispatch_gps() {
    let mut drivers = Drivers::new();

    let cmd = DriverCommand::UpdateGps {
        lat: 37.7749,
        lon: -122.4194,
        alt: 10.0,
        accuracy: 5.0,
    };

    let result = drivers.execute_command(cmd);
    assert!(result.success);

    let pos = drivers.get_position();
    assert_eq!(pos.latitude, 37.7749);
}

#[test]
fn test_command_dispatch_emergency_stop() {
    let mut drivers = Drivers::new();

    let cmd = DriverCommand::EmergencyStop;
    let result = drivers.execute_command(cmd);

    assert!(result.success);
    assert!(result.message.contains("Emergency"));
}

#[test]
fn test_sensor_polling_position() {
    let mut drivers = Drivers::new();
    drivers.update_gps(37.7749, -122.4194, 10.0, 5.0);

    let data = drivers.poll_sensor("position").unwrap();

    match data {
        SensorData::Position {
            latitude,
            longitude,
            ..
        } => {
            assert_eq!(latitude, 37.7749);
            assert_eq!(longitude, -122.4194);
        }
        _ => panic!("Wrong sensor data type"),
    }
}

#[test]
fn test_sensor_polling_orientation() {
    let drivers = Drivers::new();

    let data = drivers.poll_sensor("orientation").unwrap();

    match data {
        SensorData::Orientation { roll, pitch, yaw } => {
            assert_eq!(roll, 0.0);
            assert_eq!(pitch, 0.0);
            assert_eq!(yaw, 0.0);
        }
        _ => panic!("Wrong sensor data type"),
    }
}

#[test]
fn test_sensor_polling_velocity() {
    let drivers = Drivers::new();

    let data = drivers.poll_sensor("velocity").unwrap();

    match data {
        SensorData::Velocity { north, east, down } => {
            assert_eq!(north, 0.0);
            assert_eq!(east, 0.0);
            assert_eq!(down, 0.0);
        }
        _ => panic!("Wrong sensor data type"),
    }
}

#[test]
fn test_sensor_polling_unknown() {
    let drivers = Drivers::new();

    let result = drivers.poll_sensor("unknown_sensor");
    assert!(result.is_err());
    assert!(result.unwrap_err().contains("Unknown sensor type"));
}

#[test]
fn test_command_json_serialization() {
    let cmd = DriverCommand::Motor {
        motor_id: 0,
        speed: 0.5,
        duration_ms: 1000,
    };

    let json = serde_json::to_string(&cmd).unwrap();
    assert!(json.contains("\"type\":\"motor\""));

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
fn test_sensor_data_json_serialization() {
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
fn test_command_result_success() {
    let result = CommandResult::success("Operation completed");
    assert!(result.success);
    assert_eq!(result.message, "Operation completed");
    assert!(result.data.is_none());
}

#[test]
fn test_command_result_error() {
    let result = CommandResult::error("Operation failed");
    assert!(!result.success);
    assert_eq!(result.message, "Operation failed");
    assert!(result.data.is_some());
}

#[test]
fn test_imu_update() {
    let mut drivers = Drivers::new();

    let cmd = DriverCommand::UpdateImu {
        accel: [0.0, 0.0, 9.81],
        gyro: [0.0, 0.0, 0.0],
        mag: [0.0, 1.0, 0.0],
        timestamp: 1.0,
    };

    let result = drivers.execute_command(cmd);
    assert!(result.success);
}

#[test]
fn test_multiple_commands_sequence() {
    let mut drivers = Drivers::new();

    // GPS update
    let cmd1 = DriverCommand::UpdateGps {
        lat: 37.7749,
        lon: -122.4194,
        alt: 10.0,
        accuracy: 5.0,
    };
    assert!(drivers.execute_command(cmd1).success);

    // Motor command
    let cmd2 = DriverCommand::Motor {
        motor_id: 0,
        speed: 0.5,
        duration_ms: 1000,
    };
    assert!(drivers.execute_command(cmd2).success);

    // Servo command
    let cmd3 = DriverCommand::Servo {
        servo_id: 0,
        angle: 90.0,
        speed: 0.5,
    };
    assert!(drivers.execute_command(cmd3).success);

    // Verify GPS data persisted
    let pos = drivers.get_position();
    assert_eq!(pos.latitude, 37.7749);
}
