pub use crate::layout::{
    IDX_ACTOR_EPOCH, IDX_INBOX_DIRTY, IDX_KERNEL_READY, IDX_OUTBOX_DIRTY, IDX_PANIC_STATE,
    IDX_SENSOR_EPOCH, IDX_STORAGE_EPOCH, IDX_SYSTEM_EPOCH, OFFSET_INBOX_OUTBOX, SIZE_INBOX_OUTBOX,
};
use web_sys::wasm_bindgen::JsValue;

use crate::ringbuffer::RingBuffer;
use crate::sab::SafeSAB;

pub struct Reactor {
    flags: JsValue,
    pub inbox: RingBuffer,
    pub outbox: RingBuffer,
}

impl Reactor {
    pub fn new(sab: &JsValue) -> Self {
        // Flags region is 256 words (1024 bytes) to cover all epoch indices (0-255)
        // Use stable ABI to avoid hashed imports
        let flags = crate::js_interop::create_i32_view(sab.clone(), 0, 256);
        let flags_val: JsValue = flags.into();
        let safe_sab = SafeSAB::new(sab.clone());

        let inbox = RingBuffer::new(
            safe_sab.clone(),
            OFFSET_INBOX_OUTBOX as u32,
            (SIZE_INBOX_OUTBOX / 2) as u32,
        );

        let outbox = RingBuffer::new(
            safe_sab.clone(),
            (OFFSET_INBOX_OUTBOX + (SIZE_INBOX_OUTBOX / 2)) as u32,
            (SIZE_INBOX_OUTBOX / 2) as u32,
        );

        Self {
            flags: flags_val,
            inbox,
            outbox,
        }
    }

    pub fn check_inbox(&self) -> bool {
        crate::js_interop::atomic_load(&self.flags, IDX_INBOX_DIRTY) == 1
    }

    pub fn ack_inbox(&self) {
        crate::js_interop::atomic_store(&self.flags, IDX_INBOX_DIRTY, 0);
    }

    pub fn raise_outbox(&self) {
        // Increment sequence counter (0-255 loop or uint32 wrap)
        // Corresponds to IDX_OUTBOX_DIRTY (Index 2)
        crate::js_interop::atomic_add(&self.flags, IDX_OUTBOX_DIRTY, 1);
    }

    /// Read next message from Inbox (Ring Buffer)
    pub fn read_request(&self) -> Option<Vec<u8>> {
        self.inbox.read_message().unwrap_or(None)
    }

    /// Write message to Outbox (Ring Buffer)
    pub fn write_result(&self, data: &[u8]) -> bool {
        self.outbox.write_message(data).unwrap_or(false)
    }
}

/// Generic Epoch Counter for "Reactive Mutation"
pub struct Epoch {
    flags: JsValue,
    index: u32,
    last_seen: i32,
}

impl Epoch {
    pub fn new(sab: &JsValue, index: u32) -> Self {
        // Use stable ABI to avoid hashed imports
        let flags = crate::js_interop::create_i32_view(sab.clone(), 0, 256);
        let flags_val: JsValue = flags.into();
        let current = crate::js_interop::atomic_load(&flags_val, index);
        Self {
            flags: flags_val,
            index,
            last_seen: current,
        }
    }

    /// Check if the reality has been mutated (Epoch incremented)
    pub fn has_changed(&mut self) -> bool {
        let current = crate::js_interop::atomic_load(&self.flags, self.index);
        if current > self.last_seen {
            self.last_seen = current;
            true
        } else {
            false
        }
    }

    /// Signal a mutation (Increment Epoch)
    pub fn increment(&mut self) -> i32 {
        let next = crate::js_interop::atomic_add(&self.flags, self.index, 1) + 1;
        self.last_seen = next;
        next
    }

    pub fn current(&self) -> i32 {
        crate::js_interop::atomic_load(&self.flags, self.index)
    }
}
