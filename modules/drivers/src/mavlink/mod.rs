// MAVLink Driver for Drone Telemetry & Control
// Part of Phase 17 Robotics Extensions

#[cfg(feature = "mavlink")]
use mavlink::ardupilotmega::MavMessage;
#[cfg(feature = "mavlink")]
use mavlink::{MavConnection, MavHeader};

pub struct MavlinkDriver {
    #[cfg(feature = "mavlink")]
    connection: Option<Box<dyn MavConnection<MavMessage> + Send>>,
}

impl MavlinkDriver {
    pub fn new() -> Self {
        Self {
            #[cfg(feature = "mavlink")]
            connection: None,
        }
    }

    #[cfg(feature = "mavlink")]
    pub fn connect(&mut self, address: &str) -> Result<(), String> {
        let conn = mavlink::connect(address).map_err(|e| e.to_string())?;
        self.connection = Some(conn);
        Ok(())
    }

    #[cfg(feature = "mavlink")]
    pub fn send_message(&mut self, msg: MavMessage) -> Result<(), String> {
        if let Some(conn) = &mut self.connection {
            conn.send_default(&msg).map_err(|e| e.to_string())?;
            Ok(())
        } else {
            Err("Not connected".to_string())
        }
    }

    #[cfg(not(feature = "mavlink"))]
    pub fn connect(&mut self, _address: &str) -> Result<(), String> {
        Err("MAVLink feature not enabled".to_string())
    }

    pub fn poll(&mut self) -> Result<(), String> {
        Ok(())
    }
}

impl Default for MavlinkDriver {
    fn default() -> Self {
        Self::new()
    }
}
