//! Ping-Pong Buffer Architecture for Zero-Allocation Data Exchange
//!
//! This module implements a dual-buffer system for Go↔Rust↔JS communication:
//! - Read from Buffer A while writing to Buffer B
//! - At frame boundary, epoch increments and buffers flip
//! - Provides lock-free concurrent access via SAB
//!
//! **Memory Flow:**
//! ```text
//! Go Kernel ──► SAB Buffer A ◄── Rust Module (read)
//! Go Kernel ◄── SAB Buffer B ──► Rust Module (write)
//!                    ╱
//!             Epoch Flip
//! ```

use crate::js_interop;
use crate::layout::{
    BIRD_STRIDE, IDX_BIRD_EPOCH, IDX_MATRIX_EPOCH, IDX_PINGPONG_ACTIVE, MATRIX_STRIDE,
    OFFSET_BIRD_BUFFER_A, OFFSET_BIRD_BUFFER_B, OFFSET_MATRIX_BUFFER_A, OFFSET_MATRIX_BUFFER_B,
    SIZE_BIRD_BUFFER, SIZE_MATRIX_BUFFER,
};
use crate::sab::SafeSAB;

/// Ping-pong buffer for zero-allocation data exchange
#[derive(Clone)]
pub struct PingPongBuffer {
    sab: SafeSAB,
    buffer_a_offset: usize,
    buffer_b_offset: usize,
    buffer_size: usize,
    item_stride: usize,
    epoch_index: u32,
}

/// Result of a buffer operation
#[derive(Debug, Clone)]
pub struct BufferInfo {
    pub offset: usize,
    pub size: usize,
    pub epoch: i32,
    pub is_buffer_a: bool,
}

impl PingPongBuffer {
    /// Create a ping-pong buffer for bird state data
    pub fn bird_buffer(sab: SafeSAB) -> Self {
        Self {
            sab,
            buffer_a_offset: OFFSET_BIRD_BUFFER_A,
            buffer_b_offset: OFFSET_BIRD_BUFFER_B,
            buffer_size: SIZE_BIRD_BUFFER,
            item_stride: BIRD_STRIDE,
            epoch_index: IDX_BIRD_EPOCH,
        }
    }

    /// Create a ping-pong buffer for matrix output data
    pub fn matrix_buffer(sab: SafeSAB) -> Self {
        Self {
            sab,
            buffer_a_offset: OFFSET_MATRIX_BUFFER_A,
            buffer_b_offset: OFFSET_MATRIX_BUFFER_B,
            buffer_size: SIZE_MATRIX_BUFFER,
            item_stride: MATRIX_STRIDE,
            epoch_index: IDX_MATRIX_EPOCH,
        }
    }

    /// Create a custom ping-pong buffer with specified parameters
    pub fn custom(
        sab: SafeSAB,
        buffer_a_offset: usize,
        buffer_b_offset: usize,
        buffer_size: usize,
        item_stride: usize,
        epoch_index: u32,
    ) -> Self {
        Self {
            sab,
            buffer_a_offset,
            buffer_b_offset,
            buffer_size,
            item_stride,
            epoch_index,
        }
    }

    /// Get the current epoch value
    #[inline]
    pub fn current_epoch(&self) -> i32 {
        js_interop::atomic_load(self.sab.barrier_view(), self.epoch_index)
    }

    /// Check if buffer A is currently the read buffer (even epoch)
    #[inline]
    pub fn is_buffer_a_active(&self) -> bool {
        self.current_epoch() % 2 == 0
    }

    /// Get info about the current read buffer (consumers read from here)
    pub fn read_buffer_info(&self) -> BufferInfo {
        let epoch = self.current_epoch();
        let is_a = epoch % 2 == 0;
        BufferInfo {
            offset: if is_a {
                self.buffer_a_offset
            } else {
                self.buffer_b_offset
            },
            size: self.buffer_size,
            epoch,
            is_buffer_a: is_a,
        }
    }

    /// Get info about the current write buffer (producers write here)
    pub fn write_buffer_info(&self) -> BufferInfo {
        let epoch = self.current_epoch();
        let is_a = epoch % 2 == 0;
        // Write to opposite of read buffer
        BufferInfo {
            offset: if is_a {
                self.buffer_b_offset
            } else {
                self.buffer_a_offset
            },
            size: self.buffer_size,
            epoch,
            is_buffer_a: !is_a,
        }
    }

    /// Read entire buffer contents from the read buffer
    pub fn read_all(&self, dest: &mut [u8]) -> Result<i32, String> {
        let info = self.read_buffer_info();
        if dest.len() < self.buffer_size {
            return Err(format!(
                "Destination buffer too small: {} < {}",
                dest.len(),
                self.buffer_size
            ));
        }
        self.sab.read_raw(info.offset, dest)?;
        Ok(info.epoch)
    }

    /// Write entire buffer contents to the write buffer
    pub fn write_all(&self, data: &[u8]) -> Result<i32, String> {
        let info = self.write_buffer_info();
        if data.len() > self.buffer_size {
            return Err(format!(
                "Data too large for buffer: {} > {}",
                data.len(),
                self.buffer_size
            ));
        }
        self.sab.write_raw(info.offset, data)?;
        Ok(info.epoch)
    }

    /// Flip buffers by incrementing epoch (call at frame boundary)
    /// Returns the new epoch value
    pub fn flip(&self) -> i32 {
        let new_epoch = js_interop::atomic_add(self.sab.barrier_view(), self.epoch_index, 1) + 1;

        // Update global active buffer indicator
        let active = if new_epoch % 2 == 0 { 0 } else { 1 };
        js_interop::atomic_store(self.sab.barrier_view(), IDX_PINGPONG_ACTIVE, active);

        // Notify any waiters (workers blocking on Atomics.wait)
        js_interop::atomic_notify(self.sab.barrier_view(), self.epoch_index, i32::MAX);

        new_epoch
    }

    /// Wait for epoch to change (blocking - use only in workers)
    /// Returns the new epoch value or -1 on timeout
    pub fn wait_for_flip(&self, expected_epoch: i32, timeout_ms: f64) -> i32 {
        let result = js_interop::atomic_wait(
            self.sab.barrier_view(),
            self.epoch_index,
            expected_epoch,
            timeout_ms,
        );
        match result {
            0 => self.current_epoch(), // "ok" - value changed
            1 => -1,                   // "timed-out"
            2 => self.current_epoch(), // "not-equal" - already changed
            _ => self.current_epoch(),
        }
    }

    /// Get the underlying SAB reference
    pub fn sab(&self) -> &SafeSAB {
        &self.sab
    }

    /// Get item stride in bytes
    pub fn item_stride(&self) -> usize {
        self.item_stride
    }

    /// Get maximum number of items that fit in the buffer
    pub fn max_items(&self) -> usize {
        self.buffer_size / self.item_stride
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_pingpong_buffer_creation() {
        let sab = SafeSAB::with_size(16 * 1024 * 1024);
        let bird_buffer = PingPongBuffer::bird_buffer(sab.clone());
        assert_eq!(bird_buffer.buffer_a_offset, OFFSET_BIRD_BUFFER_A);
        assert_eq!(bird_buffer.buffer_b_offset, OFFSET_BIRD_BUFFER_B);
        assert_eq!(bird_buffer.buffer_size, SIZE_BIRD_BUFFER);
        assert_eq!(bird_buffer.item_stride, BIRD_STRIDE);
    }

    #[test]
    fn test_buffer_flip() {
        let sab = SafeSAB::with_size(16 * 1024 * 1024);
        let buffer = PingPongBuffer::bird_buffer(sab);
        let initial_epoch = buffer.current_epoch();
        assert!(buffer.is_buffer_a_active());

        let new_epoch = buffer.flip();
        assert_eq!(new_epoch, initial_epoch + 1);
        assert!(!buffer.is_buffer_a_active());

        buffer.flip();
        assert!(buffer.is_buffer_a_active());
    }

    #[test]
    fn test_max_items() {
        let sab = SafeSAB::with_size(16 * 1024 * 1024);
        let bird_buffer = PingPongBuffer::bird_buffer(sab.clone());
        let matrix_buffer = PingPongBuffer::matrix_buffer(sab);
        assert_eq!(bird_buffer.max_items(), 10000);
        assert_eq!(matrix_buffer.max_items(), 10000);
    }
}
