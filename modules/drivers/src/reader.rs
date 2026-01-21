use sdk::ringbuffer::RingBuffer;
use std::io::{self, Read};

pub struct RingBufferReader<'a> {
    rb: &'a RingBuffer,
}

impl<'a> RingBufferReader<'a> {
    pub fn new(rb: &'a RingBuffer) -> Self {
        Self { rb }
    }
}

impl<'a> Read for RingBufferReader<'a> {
    fn read(&mut self, buf: &mut [u8]) -> io::Result<usize> {
        self.rb.read(buf).map_err(io::Error::other)
    }
}
