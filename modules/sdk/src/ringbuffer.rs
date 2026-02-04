use crate::guard::{policy_for, GuardError, RegionGuard, RegionId, RegionOwner, RegionPolicy};
use crate::sab::SafeSAB;

/// Generic Ring Buffer backed by SharedArrayBuffer
/// Layout: [Head (4 bytes) | Tail (4 bytes) | Data (Capacity - 8 bytes)]
/// Thread-safe for Single Producer Single Consumer (SPSC)
pub struct RingBuffer {
    sab: SafeSAB,
    base_offset: u32,
    data_capacity: u32,
    guard_policy: Option<RegionPolicy>,
    guard_owner: Option<RegionOwner>,
    signal_epoch_index: Option<u32>,
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
            guard_policy: None,
            guard_owner: None,
            signal_epoch_index: None,
        }
    }

    pub fn new_guarded(
        sab: SafeSAB,
        base_offset: u32,
        total_size: u32,
        region_id: RegionId,
        owner: RegionOwner,
    ) -> Self {
        let policy = policy_for(region_id);
        let signal_epoch_index = policy.epoch_index;
        Self {
            sab,
            base_offset,
            data_capacity: total_size - Self::HEADER_SIZE,
            guard_policy: Some(policy),
            guard_owner: Some(owner),
            signal_epoch_index,
        }
    }

    /// Write a framed message [Length: u32][Data...]
    /// Multi-Producer Safe: Uses atomic reservation and commitment.
    pub fn write_message(&self, data: &[u8]) -> Result<bool, String> {
        let guard = self.acquire_write_guard()?;
        let msg_len = data.len() as u32;
        let total_len = 4 + msg_len;

        // 1. Reserve space atomically
        let start_tail = self.reserve_space(total_len)?;
        if start_tail == 0xFFFFFFFF {
            return Ok(false); // No space
        }

        // 2. Write Data first (skipping the 4-byte length header)
        let data_start = (start_tail + 4) % self.data_capacity;
        self.write_raw_at(data_start, data)?;

        // 3. Commit: Write Length Header LAST
        let len_bytes = msg_len.to_le_bytes();
        self.write_raw_at(start_tail, &len_bytes)?;

        if let Some(epoch_idx) = self.signal_epoch_index {
            crate::js_interop::signal_epoch(self.sab.barrier_view(), epoch_idx);
        }

        if let Some(guard) = guard {
            guard.ensure_epoch_advanced().map_err(to_string)?;
            guard.release().map_err(to_string)?;
        }

        Ok(true)
    }

    /// Read next framed message
    /// Multi-Producer Safe: Only reads if length header is non-zero (committed).
    pub fn read_message(&self) -> Result<Option<Vec<u8>>, String> {
        self.validate_read_guard()?;
        let head = self.load_head();
        let tail = self.load_tail();

        if head == tail {
            return Ok(None);
        }

        // Peek length (without moving head)
        let mut len_bytes = [0u8; 4];
        self.peek_raw_at(head, &mut len_bytes)?;
        let msg_len = u32::from_le_bytes(len_bytes);

        if msg_len == 0 {
            // Producer reserved space but hasn't committed length header yet.
            // Wait for producer to finish.
            return Ok(None);
        }

        // Consume Length + Data
        let mut msg_data = vec![0u8; msg_len as usize];
        let data_start = (head + 4) % self.data_capacity;
        self.read_raw_at(data_start, &mut msg_data)?;

        // CLEAR HEADER to 0 to prevent stale reads on wrap-around
        let zero_bytes = [0u8; 4];
        self.write_raw_at(head, &zero_bytes)?;

        // Advance Head
        self.store_head((head + 4 + msg_len) % self.data_capacity);

        Ok(Some(msg_data))
    }

    /// Read raw bytes (stream mode)
    /// Returns bytes read
    pub fn read(&self, buf: &mut [u8]) -> Result<usize, String> {
        self.validate_read_guard()?;
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
        self.read_raw_at(head, &mut buf[0..to_read])?;
        self.store_head((head + to_read as u32) % self.data_capacity);
        Ok(to_read)
    }

    pub fn read_raw(&self, buf: &mut [u8]) -> Result<(), String> {
        self.validate_read_guard()?;
        let head = self.load_head();
        self.read_raw_at(head, buf)?;
        self.store_head((head + buf.len() as u32) % self.data_capacity);
        Ok(())
    }

    pub fn peek_raw(&self, buf: &mut [u8]) -> Result<(), String> {
        self.validate_read_guard()?;
        let head = self.load_head();
        self.read_raw_at(head, buf)
    }

    pub fn skip_raw(&self, amount: u32) -> Result<(), String> {
        self.validate_read_guard()?;
        let head = self.load_head();
        self.store_head((head + amount) % self.data_capacity);
        Ok(())
    }

    fn acquire_write_guard(&self) -> Result<Option<RegionGuard>, String> {
        let Some(policy) = self.guard_policy else {
            return Ok(None);
        };
        let Some(owner) = self.guard_owner else {
            return Ok(None);
        };
        let guard = RegionGuard::acquire_write(self.sab.clone(), policy, owner)
            .map_err(to_string)?;
        Ok(Some(guard))
    }

    fn validate_read_guard(&self) -> Result<(), String> {
        let Some(policy) = self.guard_policy else {
            return Ok(());
        };
        let Some(owner) = self.guard_owner else {
            return Ok(());
        };
        RegionGuard::validate_read(&self.sab, policy, owner).map_err(to_string)
    }

    fn reserve_space(&self, amount: u32) -> Result<u32, String> {
        loop {
            let head = self.load_head();
            let tail = self.load_tail();

            let available = if tail >= head {
                self.data_capacity - (tail - head) - 1
            } else {
                head - tail - 1
            };

            if available < amount {
                return Ok(0xFFFFFFFF);
            }

            let new_tail = (tail + amount) % self.data_capacity;
            let (view_val, _) = self.get_sab_view();
            let idx = (self.base_offset + Self::TAIL_OFFSET) / 4;

            let actual_old = crate::js_interop::atomic_compare_exchange(
                &view_val,
                idx,
                tail as i32,
                new_tail as i32,
            );
            if actual_old as u32 == tail {
                return Ok(tail);
            }
            // Retry if tail moved
        }
    }

    fn write_raw_at(&self, offset: u32, data: &[u8]) -> Result<(), String> {
        let to_write = data.len();
        let write_idx = offset as usize;

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

        Ok(())
    }

    fn read_raw_at(&self, offset: u32, buf: &mut [u8]) -> Result<(), String> {
        let to_read = buf.len();
        let read_idx = offset as usize;

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

        Ok(())
    }

    fn peek_raw_at(&self, offset: u32, buf: &mut [u8]) -> Result<(), String> {
        self.read_raw_at(offset, buf) // Peek in ring buffer is just read without moving head
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

    fn get_sab_view(&self) -> (crate::js_interop::Int32Array, u32) {
        let buffer = self.sab.inner();
        let length = crate::js_interop::get_byte_length(buffer);
        let view = crate::js_interop::create_i32_view(buffer, 0, length / 4);
        (view, length / 4)
    }

    fn load_head(&self) -> u32 {
        let (view_val, _) = self.get_sab_view();
        let idx = (self.base_offset + Self::HEAD_OFFSET) / 4;
        let val = crate::js_interop::atomic_load(&view_val, idx);
        val as u32
    }

    fn store_head(&self, val: u32) {
        let (view_val, _) = self.get_sab_view();
        let idx = (self.base_offset + Self::HEAD_OFFSET) / 4;
        crate::js_interop::atomic_store(&view_val, idx, val as i32);
    }

    fn load_tail(&self) -> u32 {
        let (view_val, _) = self.get_sab_view();
        let idx = (self.base_offset + Self::TAIL_OFFSET) / 4;
        let val = crate::js_interop::atomic_load(&view_val, idx);
        val as u32
    }

    fn _store_tail(&self, val: u32) {
        let (view_val, _) = self.get_sab_view();
        let idx = (self.base_offset + Self::TAIL_OFFSET) / 4;
        crate::js_interop::atomic_store(&view_val, idx, val as i32);
    }
}

fn to_string(err: GuardError) -> String {
    match err {
        GuardError::Unauthorized(msg) => format!("Guard unauthorized: {msg}"),
        GuardError::Locked(msg) => format!("Guard locked: {msg}"),
        GuardError::OutOfRange(msg) => format!("Guard out of range: {msg}"),
    }
}
