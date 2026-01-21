// use crate::js_interop::JsValue;
pub use crate::layout::{
    IDX_ACTOR_EPOCH, IDX_INBOX_DIRTY, IDX_KERNEL_READY, IDX_OUTBOX_HOST_DIRTY,
    IDX_OUTBOX_KERNEL_DIRTY, IDX_PANIC_STATE, IDX_SENSOR_EPOCH, IDX_STORAGE_EPOCH,
    IDX_SYSTEM_EPOCH, OFFSET_SAB_INBOX, OFFSET_SAB_OUTBOX, SIZE_INBOX, SIZE_OUTBOX,
};

use crate::ringbuffer::RingBuffer;
use crate::sab::SafeSAB;

pub struct Reactor {
    flags: SafeSAB,
    pub inbox: RingBuffer,
    pub outbox: RingBuffer,
}

impl Reactor {
    pub fn new(sab: SafeSAB) -> Self {
        // Flags (AtomicFlags) are at OFFSET_ATOMIC_FLAGS (0x00) within the System SAB
        // We use a shared view of the first 1024 bytes of the provided SafeSAB (which is already offset-scoped)
        let flags = SafeSAB::new_shared_view(sab.inner(), sab.base_offset() as u32, 1024);

        let inbox = RingBuffer::new(sab.clone(), OFFSET_SAB_INBOX as u32, SIZE_INBOX as u32);

        let outbox = RingBuffer::new(sab.clone(), OFFSET_SAB_OUTBOX as u32, SIZE_OUTBOX as u32);

        Self {
            flags,
            inbox,
            outbox,
        }
    }

    pub fn check_inbox(&self) -> bool {
        crate::js_interop::atomic_load(self.flags.barrier_view(), IDX_INBOX_DIRTY) == 1
    }

    pub fn ack_inbox(&self) {
        crate::js_interop::atomic_store(self.flags.barrier_view(), IDX_INBOX_DIRTY, 0);
    }

    pub fn raise_outbox(&self) {
        crate::js_interop::atomic_add(self.flags.barrier_view(), IDX_OUTBOX_KERNEL_DIRTY, 1);
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
    flags: SafeSAB,
    index: u32,
    last_seen: i32,
}

impl Epoch {
    pub fn new(sab: SafeSAB, index: u32) -> Self {
        // Flags are at the start of the scoped SAB
        let flags = SafeSAB::new_shared_view(sab.inner(), sab.base_offset() as u32, 1024);
        let current = crate::js_interop::atomic_load(flags.barrier_view(), index);
        Self {
            flags,
            index,
            last_seen: current,
        }
    }

    /// Check if the reality has been mutated (Epoch incremented)
    pub fn has_changed(&mut self) -> bool {
        let current = crate::js_interop::atomic_load(self.flags.barrier_view(), self.index);
        if current > self.last_seen {
            self.last_seen = current;
            true
        } else {
            false
        }
    }

    /// Signal a mutation (Increment Epoch)
    pub fn increment(&mut self) -> i32 {
        crate::js_interop::atomic_add(self.flags.barrier_view(), self.index, 1) + 1
    }

    pub fn current(&self) -> i32 {
        crate::js_interop::atomic_load(self.flags.barrier_view(), self.index)
    }
}
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_epoch_logic() {
        let sab = SafeSAB::with_size(1024);
        let mut epoch = Epoch::new(sab, IDX_SYSTEM_EPOCH);

        assert_eq!(epoch.current(), 0);
        assert!(!epoch.has_changed());

        epoch.increment();
        assert_eq!(epoch.current(), 1);
        assert!(epoch.has_changed());
        assert!(!epoch.has_changed()); // Second check should be false
    }

    #[test]
    fn test_reactor_signals() {
        let sab = SafeSAB::with_size(16 * 1024 * 1024);
        let reactor = Reactor::new(sab.clone());

        assert!(!reactor.check_inbox());

        // Mock signal via SafeSAB/barrier_view which handles the underlying rust-side mock
        crate::js_interop::atomic_store(sab.barrier_view(), IDX_INBOX_DIRTY, 1);
        assert!(reactor.check_inbox());

        reactor.ack_inbox();
        assert!(!reactor.check_inbox());

        let start_epoch =
            crate::js_interop::atomic_load(sab.barrier_view(), IDX_OUTBOX_KERNEL_DIRTY);
        reactor.raise_outbox();
        assert_eq!(
            crate::js_interop::atomic_load(sab.barrier_view(), IDX_OUTBOX_KERNEL_DIRTY),
            start_epoch + 1
        );
    }
}
