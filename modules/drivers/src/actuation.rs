// Actuation System - Motor Control, Servo, GPIO
// Provides precise control for movement and manipulation

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MotorCommand {
    pub motor_id: u8,
    pub speed: f32, // -1.0 to 1.0 (negative = reverse)
    pub duration_ms: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServoCommand {
    pub servo_id: u8,
    pub angle: f32, // degrees (0-180)
    pub speed: f32, // 0.0 to 1.0
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GpioCommand {
    pub pin: u8,
    pub state: bool, // true = HIGH, false = LOW
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MotorStatus {
    pub motor_id: u8,
    pub current_speed: f32,
    pub temperature: f32,  // Celsius
    pub current_draw: f32, // Amperes
}

/// Motor Controller
///
/// **Library Proxy Pattern**: Interfaces with motor drivers
/// (PWM, H-bridge, ESC) for precise speed control.
///
/// **Use Cases**:
/// - Drone motors (quadcopter, hexacopter)
/// - Wheeled robots (differential drive)
/// - Robotic arms (joint motors)
pub struct MotorController {
    motors: Vec<MotorStatus>,
}

impl MotorController {
    pub fn new(num_motors: usize) -> Self {
        let motors = (0..num_motors)
            .map(|i| MotorStatus {
                motor_id: i as u8,
                current_speed: 0.0,
                temperature: 25.0,
                current_draw: 0.0,
            })
            .collect();

        Self { motors }
    }

    /// Set motor speed
    pub fn set_speed(&mut self, motor_id: u8, speed: f32) -> Result<(), String> {
        let speed = speed.clamp(-1.0, 1.0);

        if let Some(motor) = self.motors.get_mut(motor_id as usize) {
            motor.current_speed = speed;
            log::debug!("Motor {} set to speed {}", motor_id, speed);
            Ok(())
        } else {
            Err(format!("Motor {} not found", motor_id))
        }
    }

    /// Execute motor command
    pub fn execute(&mut self, cmd: MotorCommand) -> Result<(), String> {
        self.set_speed(cmd.motor_id, cmd.speed)?;

        // Future: Implement duration-based control using std::time
        // For now, commands are instantaneous
        Ok(())
    }

    /// Get motor status
    pub fn get_status(&self, motor_id: u8) -> Option<MotorStatus> {
        self.motors.get(motor_id as usize).cloned()
    }

    /// Emergency stop all motors
    pub fn emergency_stop(&mut self) {
        for motor in &mut self.motors {
            motor.current_speed = 0.0;
        }
        log::warn!("Emergency stop activated - all motors stopped");
    }
}

/// Servo Controller
///
/// **Library Proxy Pattern**: Interfaces with servo motors
/// via PWM for precise angle control.
pub struct ServoController {
    servos: Vec<(u8, f32)>, // (servo_id, current_angle)
}

impl ServoController {
    pub fn new(num_servos: usize) -> Self {
        let servos = (0..num_servos)
            .map(|i| (i as u8, 90.0)) // Default to center position
            .collect();

        Self { servos }
    }

    /// Set servo angle
    pub fn set_angle(&mut self, servo_id: u8, angle: f32) -> Result<(), String> {
        let angle = angle.clamp(0.0, 180.0);

        if let Some(servo) = self.servos.get_mut(servo_id as usize) {
            servo.1 = angle;
            log::debug!("Servo {} set to angle {}", servo_id, angle);
            Ok(())
        } else {
            Err(format!("Servo {} not found", servo_id))
        }
    }

    /// Execute servo command
    pub fn execute(&mut self, cmd: ServoCommand) -> Result<(), String> {
        self.set_angle(cmd.servo_id, cmd.angle)?;
        // Future: Implement speed control with interpolation
        // For now, movements are instantaneous
        Ok(())
    }

    /// Get current angle
    pub fn get_angle(&self, servo_id: u8) -> Option<f32> {
        self.servos.get(servo_id as usize).map(|(_, angle)| *angle)
    }
}

/// GPIO Controller
///
/// **Library Proxy Pattern**: Interfaces with GPIO pins
/// for digital I/O control.
pub struct GpioController {
    pins: Vec<(u8, bool)>, // (pin_id, state)
}

impl GpioController {
    pub fn new(num_pins: usize) -> Self {
        let pins = (0..num_pins)
            .map(|i| (i as u8, false)) // Default to LOW
            .collect();

        Self { pins }
    }

    /// Set pin state
    pub fn set_pin(&mut self, pin: u8, state: bool) -> Result<(), String> {
        if let Some(p) = self.pins.get_mut(pin as usize) {
            p.1 = state;
            log::debug!(
                "GPIO pin {} set to {}",
                pin,
                if state { "HIGH" } else { "LOW" }
            );
            Ok(())
        } else {
            Err(format!("Pin {} not found", pin))
        }
    }

    /// Execute GPIO command
    pub fn execute(&mut self, cmd: GpioCommand) -> Result<(), String> {
        self.set_pin(cmd.pin, cmd.state)
    }

    /// Get pin state
    pub fn get_pin(&self, pin: u8) -> Option<bool> {
        self.pins.get(pin as usize).map(|(_, state)| *state)
    }
}

impl Default for MotorController {
    fn default() -> Self {
        Self::new(4) // 4 motors (quadcopter)
    }
}

impl Default for ServoController {
    fn default() -> Self {
        Self::new(8) // 8 servos
    }
}

impl Default for GpioController {
    fn default() -> Self {
        Self::new(16) // 16 GPIO pins
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_motor_controller() {
        let mut controller = MotorController::new(4);
        assert!(controller.set_speed(0, 0.5).is_ok());

        let status = controller.get_status(0).unwrap();
        assert_eq!(status.current_speed, 0.5);
    }

    #[test]
    fn test_servo_controller() {
        let mut controller = ServoController::new(4);
        assert!(controller.set_angle(0, 45.0).is_ok());

        let angle = controller.get_angle(0).unwrap();
        assert_eq!(angle, 45.0);
    }

    #[test]
    fn test_gpio_controller() {
        let mut controller = GpioController::new(8);
        assert!(controller.set_pin(0, true).is_ok());

        let state = controller.get_pin(0).unwrap();
        assert_eq!(state, true);
    }

    #[test]
    fn test_emergency_stop() {
        let mut controller = MotorController::new(4);
        controller.set_speed(0, 0.8).unwrap();
        controller.set_speed(1, -0.5).unwrap();

        controller.emergency_stop();

        assert_eq!(controller.get_status(0).unwrap().current_speed, 0.0);
        assert_eq!(controller.get_status(1).unwrap().current_speed, 0.0);
    }
}
