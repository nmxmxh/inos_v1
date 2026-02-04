use crate::layout::{
    OFFSET_REGION_GUARDS, REGION_GUARD_COUNT, REGION_GUARD_ENTRY_SIZE, SIZE_REGION_GUARDS,
};
use crate::sab::SafeSAB;

/// Region Owner bitmask (shared across Go/Rust/JS)
#[repr(u32)]
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum RegionOwner {
    Kernel = 1 << 0,
    Module = 1 << 1,
    Host = 1 << 2,
    System = 1 << 3,
}

/// Access mode for a region
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum AccessMode {
    ReadOnly,
    SingleWriter,
    MultiWriter,
}

/// Region policy: who can read/write and how access is enforced
#[derive(Clone, Copy, Debug)]
pub struct RegionPolicy {
    pub region_id: u32,
    pub access: AccessMode,
    pub writer_mask: u32,
    pub reader_mask: u32,
    pub epoch_index: Option<u32>,
}

/// Known regions for guard enforcement
#[repr(u32)]
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum RegionId {
    Inbox = 0,
    OutboxHost = 1,
    OutboxKernel = 2,
    MeshEventQueue = 3,
    ArenaRequestQueue = 4,
    ArenaResponseQueue = 5,
    Arena = 6,
}

pub fn policy_for(region: RegionId) -> RegionPolicy {
    match region {
        RegionId::Inbox => RegionPolicy {
            region_id: region as u32,
            access: AccessMode::SingleWriter,
            writer_mask: RegionOwner::Kernel as u32,
            reader_mask: RegionOwner::Module as u32 | RegionOwner::Host as u32,
            epoch_index: Some(crate::layout::IDX_INBOX_DIRTY),
        },
        RegionId::OutboxHost => RegionPolicy {
            region_id: region as u32,
            access: AccessMode::SingleWriter,
            writer_mask: RegionOwner::Kernel as u32,
            reader_mask: RegionOwner::Host as u32,
            epoch_index: Some(crate::layout::IDX_OUTBOX_HOST_DIRTY),
        },
        RegionId::OutboxKernel => RegionPolicy {
            region_id: region as u32,
            access: AccessMode::MultiWriter,
            writer_mask: RegionOwner::Module as u32,
            reader_mask: RegionOwner::Kernel as u32,
            epoch_index: Some(crate::layout::IDX_OUTBOX_KERNEL_DIRTY),
        },
        RegionId::MeshEventQueue => RegionPolicy {
            region_id: region as u32,
            access: AccessMode::SingleWriter,
            writer_mask: RegionOwner::Kernel as u32,
            reader_mask: RegionOwner::Host as u32,
            epoch_index: Some(crate::layout::IDX_MESH_EVENT_EPOCH),
        },
        RegionId::ArenaRequestQueue => RegionPolicy {
            region_id: region as u32,
            access: AccessMode::SingleWriter,
            writer_mask: RegionOwner::Module as u32,
            reader_mask: RegionOwner::Kernel as u32,
            epoch_index: Some(crate::layout::IDX_ARENA_ALLOCATOR),
        },
        RegionId::ArenaResponseQueue => RegionPolicy {
            region_id: region as u32,
            access: AccessMode::SingleWriter,
            writer_mask: RegionOwner::Kernel as u32,
            reader_mask: RegionOwner::Module as u32,
            epoch_index: None,
        },
        RegionId::Arena => RegionPolicy {
            region_id: region as u32,
            access: AccessMode::MultiWriter,
            writer_mask: RegionOwner::Kernel as u32 | RegionOwner::Module as u32,
            reader_mask: RegionOwner::Kernel as u32
                | RegionOwner::Module as u32
                | RegionOwner::Host as u32,
            epoch_index: None,
        },
    }
}

#[derive(Debug)]
pub enum GuardError {
    Unauthorized(&'static str),
    Locked(&'static str),
    OutOfRange(&'static str),
}

/// Guard entry layout (4 x i32)
const GUARD_LOCK: u32 = 0; // Active writer lock (owner id)
const GUARD_LAST_EPOCH: u32 = 1;
const GUARD_VIOLATIONS: u32 = 2;
const GUARD_LAST_OWNER: u32 = 3;

fn guard_index(region_id: u32, field: u32) -> u32 {
    let entry_words = (REGION_GUARD_ENTRY_SIZE / 4) as u32;
    region_id * entry_words + field
}

fn increment_violation(sab: &SafeSAB, region_id: u32) {
    if let Ok(flags) = sab.int32_view(OFFSET_REGION_GUARDS, SIZE_REGION_GUARDS / 4) {
        let idx = guard_index(region_id, GUARD_VIOLATIONS);
        let _ = crate::js_interop::atomic_add(&flags, idx, 1);
    }
}

fn read_guard_word(sab: &SafeSAB, region_id: u32, field: u32) -> Option<u32> {
    sab.int32_view(OFFSET_REGION_GUARDS, SIZE_REGION_GUARDS / 4)
        .ok()
        .map(|flags| crate::js_interop::atomic_load(&flags, guard_index(region_id, field)) as u32)
}

fn write_guard_word(sab: &SafeSAB, region_id: u32, field: u32, value: u32) {
    if let Ok(flags) = sab.int32_view(OFFSET_REGION_GUARDS, SIZE_REGION_GUARDS / 4) {
        crate::js_interop::atomic_store(&flags, guard_index(region_id, field), value as i32);
    }
}

fn cas_guard_word(sab: &SafeSAB, region_id: u32, field: u32, expected: u32, value: u32) -> bool {
    if let Ok(flags) = sab.int32_view(OFFSET_REGION_GUARDS, SIZE_REGION_GUARDS / 4) {
        let actual =
            crate::js_interop::atomic_compare_exchange(&flags, guard_index(region_id, field), expected as i32, value as i32);
        return actual as u32 == expected;
    }
    false
}

pub struct RegionGuard {
    sab: SafeSAB,
    policy: RegionPolicy,
    owner: RegionOwner,
    start_epoch: Option<u32>,
    released: bool,
}

impl RegionGuard {
    pub fn acquire_write(sab: SafeSAB, policy: RegionPolicy, owner: RegionOwner) -> Result<Self, GuardError> {
        if (owner as u32) & policy.writer_mask == 0 {
            increment_violation(&sab, policy.region_id);
            return Err(GuardError::Unauthorized("writer not allowed for region"));
        }

        if policy.region_id >= REGION_GUARD_COUNT as u32 {
            return Err(GuardError::OutOfRange("region id out of range"));
        }

        match policy.access {
            AccessMode::ReadOnly => {
                increment_violation(&sab, policy.region_id);
                return Err(GuardError::Unauthorized("region is read-only"));
            }
            AccessMode::SingleWriter => {
                if !cas_guard_word(&sab, policy.region_id, GUARD_LOCK, 0, owner as u32) {
                    increment_violation(&sab, policy.region_id);
                    return Err(GuardError::Locked("region already locked"));
                }
            }
            AccessMode::MultiWriter => {
                // No lock, but record last owner for debugging
                write_guard_word(&sab, policy.region_id, GUARD_LAST_OWNER, owner as u32);
            }
        }

        let start_epoch = policy
            .epoch_index
            .map(|idx| crate::js_interop::atomic_load(sab.barrier_view(), idx) as u32);

        Ok(Self {
            sab,
            policy,
            owner,
            start_epoch,
            released: false,
        })
    }

    pub fn validate_read(sab: &SafeSAB, policy: RegionPolicy, owner: RegionOwner) -> Result<(), GuardError> {
        if (owner as u32) & policy.reader_mask == 0 {
            increment_violation(sab, policy.region_id);
            return Err(GuardError::Unauthorized("reader not allowed for region"));
        }
        Ok(())
    }

    /// Verify epoch advanced after a write (if configured).
    pub fn ensure_epoch_advanced(&self) -> Result<(), GuardError> {
        let Some(idx) = self.policy.epoch_index else {
            return Ok(());
        };
        let Some(start) = self.start_epoch else {
            return Ok(());
        };
        let current = crate::js_interop::atomic_load(self.sab.barrier_view(), idx) as u32;
        if current <= start {
            increment_violation(&self.sab, self.policy.region_id);
            return Err(GuardError::Unauthorized("epoch not advanced after write"));
        }
        write_guard_word(&self.sab, self.policy.region_id, GUARD_LAST_EPOCH, current);
        Ok(())
    }

    pub fn release(mut self) -> Result<(), GuardError> {
        if self.released {
            return Ok(());
        }

        if self.policy.access == AccessMode::SingleWriter {
            if !cas_guard_word(&self.sab, self.policy.region_id, GUARD_LOCK, self.owner as u32, 0) {
                increment_violation(&self.sab, self.policy.region_id);
                return Err(GuardError::Locked("release failed: lock owner mismatch"));
            }
        }

        self.released = true;
        Ok(())
    }
}

impl Drop for RegionGuard {
    fn drop(&mut self) {
        if self.released {
            return;
        }
        if self.policy.access == AccessMode::SingleWriter {
            let _ = cas_guard_word(&self.sab, self.policy.region_id, GUARD_LOCK, self.owner as u32, 0);
        }
    }
}
