use crate::reader::RingBufferReader;
use capnp::serialize;
use sdk::actor_capnp;
use sdk::Epoch;

pub trait Actor: Send {
    fn id(&self) -> &str;
    fn on_command(&mut self, cmd: &ActorCommand) -> Result<(), String>;
}

pub struct ActorCommand {
    pub target_id: String,
    pub timestamp_ns: i64,
    pub payload: Vec<u8>, // Raw Cap'n Proto bytes for the specific command variant
}

pub struct ActorDriver {
    actors: Vec<Box<dyn Actor>>,
    epoch: Epoch,
    ring_buffer: Option<sdk::ringbuffer::RingBuffer>,
}

impl ActorDriver {
    pub fn new(epoch: Epoch) -> Self {
        Self {
            actors: Vec::new(),
            epoch,
            ring_buffer: None,
        }
    }

    pub fn register_actor(&mut self, actor: Box<dyn Actor>) {
        self.actors.push(actor);
    }

    pub fn set_ring_buffer(&mut self, rb: sdk::ringbuffer::RingBuffer) {
        self.ring_buffer = Some(rb);
    }

    pub fn poll(&mut self) -> Result<(), String> {
        if self.epoch.has_changed() {
            if let Some(rb) = &self.ring_buffer {
                let mut reader = RingBufferReader::new(rb);

                while rb.available() > 0 {
                    match serialize::read_message(&mut reader, capnp::message::ReaderOptions::new())
                    {
                        Ok(message_reader) => {
                            if let Ok(root) =
                                message_reader.get_root::<actor_capnp::actor::command::Reader>()
                            {
                                let target_id = root
                                    .get_target_id()
                                    .ok()
                                    .and_then(|t| t.to_str().ok())
                                    .unwrap_or("")
                                    .to_string();

                                let command = ActorCommand {
                                    target_id: target_id.clone(),
                                    timestamp_ns: root.get_timestamp_ns(),
                                    payload: Vec::new(), // TODO: Extract specific variant data
                                };

                                for actor in &mut self.actors {
                                    if actor.id() == target_id {
                                        let _ = actor.on_command(&command);
                                    }
                                }
                            }
                        }
                        Err(_) => break,
                    }
                }
            }
        }
        Ok(())
    }
}
