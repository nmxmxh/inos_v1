use crate::sab::SafeSAB;

/// Generic Ring Buffer backed by SharedArrayBuffer
/// Layout: [Head (4 bytes) | Tail (4 bytes) | Data (Capacity - 8 bytes)]
/// Thread-safe for Single Producer Single Consumer (SPSC)
pub struct RingBuffer {
    sab: SafeSAB,
    base_offset: u32,
    data_capacity: u32,
}

impl RingBuffer {
    const HEAD_OFFSET: u32 = 0;
    const TAIL_OFFSET: u32 = 4;
    const HEADER_SIZE: u32 = 8;

    pub fn new(sab: SafeSAB, base_offset: u32, total_size: u32) -> Self {
        assert!(total_size > Self::HEADER_SIZE, "RingBuffer too small");
        Self {
            sab,
            base_offset,
            data_capacity: total_size - Self::HEADER_SIZE,
        }
    }

    /// Write a framed message [Length: u32][Data...]
    /// Returns true if written, false if not enough space
    pub fn write_message(&self, data: &[u8]) -> Result<bool, String> {
        let msg_len = data.len() as u32;
        let total_len = 4 + msg_len; // 4 bytes for length header

        let head = self.load_head();
        let tail = self.load_tail();

        let available = if tail >= head {
            self.data_capacity - (tail - head) - 1
        } else {
            head - tail - 1
        };

        if available < total_len {
            return Ok(false);
        }

        // Write Length Header (Little Endian)
        let len_bytes = msg_len.to_le_bytes();
        self.write_raw(&len_bytes)?;

        // Write Data
        self.write_raw(data)?;

        Ok(true)
    }

    /// Read next framed message
    /// Returns Some(Vec<u8>) if message available, None if empty/partial
    pub fn read_message(&self) -> Result<Option<Vec<u8>>, String> {
        let head = self.load_head();
        let tail = self.load_tail();

        if head == tail {
            return Ok(None);
        }

        // Check if we have at least 4 bytes for length
        let available = if tail >= head {
            tail - head
        } else {
            self.data_capacity - (head - tail)
        };

        if available < 4 {
            // Should not happen if rights are atomic, but good for safety
            return Ok(None);
        }

        // Peek length (without moving head)
        let mut len_bytes = [0u8; 4];
        self.peek_raw(&mut len_bytes)?;
        let msg_len = u32::from_le_bytes(len_bytes);

        if available < 4 + msg_len {
            // Partial message written? Wait for rest.
            return Ok(None);
        }

        // Consume Length
        self.skip_raw(4)?;

        // Consume Data
        let mut msg_data = vec![0u8; msg_len as usize];
        self.read_raw(&mut msg_data)?;

        Ok(Some(msg_data))
    }

    // Internal raw write (wrapping)
    fn write_raw(&self, data: &[u8]) -> Result<(), String> {
        let tail = self.load_tail();
        let to_write = data.len();
        let write_idx = (tail as usize) % self.data_capacity as usize;

        let first_chunk = std::cmp::min(to_write, (self.data_capacity as usize) - write_idx);
        let second_chunk = to_write - first_chunk;

        self.sab.write(
            (self.base_offset + Self::HEADER_SIZE) as usize + write_idx,
            &data[0..first_chunk],
        )?;

        if second_chunk > 0 {
            self.sab.write(
                (self.base_offset + Self::HEADER_SIZE) as usize,
                &data[first_chunk..to_write],
            )?;
        }

        self.store_tail((tail + to_write as u32) % self.data_capacity);
        Ok(())
    }

    /// Read raw bytes (stream mode)
    /// Returns bytes read
    pub fn read(&self, buf: &mut [u8]) -> Result<usize, String> {
        let head = self.load_head();
        let tail = self.load_tail();

        if head == tail {
            return Ok(0);
        }

        let available = if tail >= head {
            tail - head
        } else {
            self.data_capacity - (head - tail)
        };

        let to_read = std::cmp::min(available as usize, buf.len());

        // Use read_raw internal logic but don't duplicate code?
        // Actually read_raw takes buf so it matches signature partially.
        // But read_raw returns Result<(), String> and assumes buf fits.
        // Let's implement read using read_raw logic but handling partial reads.

        let read_idx = (head as usize) % self.data_capacity as usize;
        let first_chunk = std::cmp::min(to_read, (self.data_capacity as usize) - read_idx);
        let second_chunk = to_read - first_chunk;

        let chunk1_data = self.sab.read(
            (self.base_offset + Self::HEADER_SIZE) as usize + read_idx,
            first_chunk,
        )?;
        buf[0..first_chunk].copy_from_slice(&chunk1_data);

        if second_chunk > 0 {
            let chunk2_data = self.sab.read(
                (self.base_offset + Self::HEADER_SIZE) as usize,
                second_chunk,
            )?;
            buf[first_chunk..to_read].copy_from_slice(&chunk2_data);
        }

        self.store_head((head + to_read as u32) % self.data_capacity);
        Ok(to_read)
    }

    fn read_raw(&self, buf: &mut [u8]) -> Result<(), String> {
        let head = self.load_head();
        let to_read = buf.len();
        let read_idx = (head as usize) % self.data_capacity as usize;

        let first_chunk = std::cmp::min(to_read, (self.data_capacity as usize) - read_idx);
        let second_chunk = to_read - first_chunk;

        let chunk1 = self.sab.read(
            (self.base_offset + Self::HEADER_SIZE) as usize + read_idx,
            first_chunk,
        )?;
        buf[0..first_chunk].copy_from_slice(&chunk1);

        if second_chunk > 0 {
            let chunk2 = self.sab.read(
                (self.base_offset + Self::HEADER_SIZE) as usize,
                second_chunk,
            )?;
            buf[first_chunk..to_read].copy_from_slice(&chunk2);
        }

        self.store_head((head + to_read as u32) % self.data_capacity);
        Ok(())
    }

    fn peek_raw(&self, buf: &mut [u8]) -> Result<(), String> {
        let head = self.load_head();
        let to_read = buf.len();
        let read_idx = (head as usize) % self.data_capacity as usize;

        let first_chunk = std::cmp::min(to_read, (self.data_capacity as usize) - read_idx);
        let second_chunk = to_read - first_chunk;

        let chunk1_data = self.sab.read(
            (self.base_offset + Self::HEADER_SIZE) as usize + read_idx,
            first_chunk,
        )?;
        buf[0..first_chunk].copy_from_slice(&chunk1_data);

        if second_chunk > 0 {
            let chunk2_data = self.sab.read(
                (self.base_offset + Self::HEADER_SIZE) as usize,
                second_chunk,
            )?;
            buf[first_chunk..to_read].copy_from_slice(&chunk2_data);
        }
        Ok(())
    }

    fn skip_raw(&self, amount: u32) -> Result<(), String> {
        let head = self.load_head();
        self.store_head((head + amount) % self.data_capacity);
        Ok(())
    }

    /// Available bytes to read
    pub fn available(&self) -> u32 {
        let head = self.load_head();
        let tail = self.load_tail();

        if tail >= head {
            tail - head
        } else {
            self.data_capacity - (head - tail)
        }
    }

    fn load_head(&self) -> u32 {
        let buffer = &self.sab.buffer;
        let byte_len = crate::js_interop::get_byte_length(buffer);
        let view = crate::js_interop::create_i32_view(buffer.clone(), 0, byte_len / 4);
        let idx = (self.base_offset + Self::HEAD_OFFSET) / 4;
        let view_val: web_sys::wasm_bindgen::JsValue = view.into();
        let val = crate::js_interop::atomic_load(&view_val, idx);
        val as u32
    }

    fn store_head(&self, val: u32) {
        let buffer = &self.sab.buffer;
        let byte_len = crate::js_interop::get_byte_length(buffer);
        let view = crate::js_interop::create_i32_view(buffer.clone(), 0, byte_len / 4);
        let idx = (self.base_offset + Self::HEAD_OFFSET) / 4;
        let view_val: web_sys::wasm_bindgen::JsValue = view.into();
        crate::js_interop::atomic_store(&view_val, idx, val as i32);
    }

    fn load_tail(&self) -> u32 {
        let buffer = &self.sab.buffer;
        let byte_len = crate::js_interop::get_byte_length(buffer);
        let view = crate::js_interop::create_i32_view(buffer.clone(), 0, byte_len / 4);
        let idx = (self.base_offset + Self::TAIL_OFFSET) / 4;
        let view_val: web_sys::wasm_bindgen::JsValue = view.into();
        let val = crate::js_interop::atomic_load(&view_val, idx);
        val as u32
    }

    fn store_tail(&self, val: u32) {
        let buffer = &self.sab.buffer;
        let byte_len = crate::js_interop::get_byte_length(buffer);
        let view = crate::js_interop::create_i32_view(buffer.clone(), 0, byte_len / 4);
        let idx = (self.base_offset + Self::TAIL_OFFSET) / 4;
        let view_val: web_sys::wasm_bindgen::JsValue = view.into();
        crate::js_interop::atomic_store(&view_val, idx, val as i32);
    }
}
