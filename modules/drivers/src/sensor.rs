use crate::reader::RingBufferReader;
use capnp::serialize;
use sdk::sensor_capnp;
use sdk::Epoch;

pub trait Sensor: Send {
    fn id(&self) -> &str;
    fn on_frame(&mut self, frame: &[u8]) -> Result<(), String>;
}

pub struct SensorSubscriber {
    sensors: Vec<Box<dyn Sensor>>,
    epoch: Epoch,
    ring_buffer: Option<sdk::ringbuffer::RingBuffer>,
}

impl SensorSubscriber {
    pub fn new(epoch: Epoch) -> Self {
        Self {
            sensors: Vec::new(),
            epoch,
            ring_buffer: None, // Initialized later or passed in
        }
    }

    pub fn set_ring_buffer(&mut self, rb: sdk::ringbuffer::RingBuffer) {
        self.ring_buffer = Some(rb);
    }

    pub fn register_sensor(&mut self, sensor: Box<dyn Sensor>) {
        self.sensors.push(sensor);
    }

    pub fn poll(&mut self) -> Result<(), String> {
        if self.epoch.has_changed() {
            // Read from OffsetInbox (Host -> Drivers)
            if let Some(rb) = &self.ring_buffer {
                let mut reader = RingBufferReader::new(rb);

                // Keep reading messages as long as data is available
                while rb.available() > 0 {
                    // deserialize_from_read reads a message from the stream
                    // It expects standard Cap'n Proto framing (segment table)
                    match serialize::read_message(&mut reader, capnp::message::ReaderOptions::new())
                    {
                        Ok(message_reader) => {
                            if let Ok(root) =
                                message_reader.get_root::<sensor_capnp::i_o::sensor_frame::Reader>()
                            {
                                // Extract data based on variant
                                let data = match root.which() {
                                    Ok(sensor_capnp::i_o::sensor_frame::Which::RawBytes(Ok(
                                        data,
                                    ))) => Some(data),
                                    Ok(sensor_capnp::i_o::sensor_frame::Which::DepthMap(Ok(
                                        data,
                                    ))) => Some(data),
                                    Ok(sensor_capnp::i_o::sensor_frame::Which::AudioChunk(Ok(
                                        data,
                                    ))) => Some(data),
                                    Ok(sensor_capnp::i_o::sensor_frame::Which::VideoFrame(Ok(
                                        data,
                                    ))) => Some(data),
                                    _ => None, // Structured data (IMU, etc.) handling not yet implemented via raw interface
                                };

                                if let Some(bytes) = data {
                                    for sensor in &mut self.sensors {
                                        if let Ok(source_id) = root.get_source_id() {
                                            if sensor.id() == source_id {
                                                let _ = sensor.on_frame(bytes);
                                            }
                                        }
                                    }
                                }
                            }
                        }
                        Err(_) => break, // Stop on error or incomplete message
                    }
                }
            }
        }
        Ok(())
    }
}
