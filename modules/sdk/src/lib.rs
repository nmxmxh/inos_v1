// Suppress expected warning for WASM threading atomics (required for SharedArrayBuffer)
#![allow(unstable_features)]

pub mod credits;
pub mod identity;
mod logging;
pub mod signal;
pub mod social_graph;

pub mod arena;
pub mod compression;
pub mod context;
pub mod crdt;
pub mod hashing;
pub mod js_interop;
pub mod layout;
pub mod pingpong;
pub mod registry;
pub mod ringbuffer;
pub mod sab;
pub mod shader_registry;
pub mod syscalls;

#[cfg(test)]
pub mod sab_benchmarks;

#[cfg(test)]
pub mod benchmarks;

#[cfg(test)]
pub mod core_tests;

// Generated Cap'n Proto Modules (Must be at root for cross-references)
// We allow dead_code and unused_imports to silence standard capnpc warnings
#[allow(dead_code, unused_imports, unused_parens, clippy::match_single_binding)]
pub mod base_capnp {
    include!(concat!(env!("OUT_DIR"), "/base/v1/base_capnp.rs"));
}
#[allow(dead_code, unused_imports, unused_parens, clippy::match_single_binding)]
pub mod capsule_capnp {
    include!(concat!(env!("OUT_DIR"), "/compute/v1/capsule_capnp.rs"));
}
#[allow(dead_code, unused_imports, unused_parens, clippy::match_single_binding)]
pub mod orchestration_capnp {
    include!(concat!(
        env!("OUT_DIR"),
        "/system/v1/orchestration_capnp.rs"
    ));
}
#[allow(dead_code, unused_imports, unused_parens, clippy::match_single_binding)]
pub mod actor_capnp {
    include!(concat!(env!("OUT_DIR"), "/io/v1/actor_capnp.rs"));
}
#[allow(dead_code, unused_imports, unused_parens, clippy::match_single_binding)]
pub mod sensor_capnp {
    include!(concat!(env!("OUT_DIR"), "/io/v1/sensor_capnp.rs"));
}
#[allow(dead_code, unused_imports, unused_parens, clippy::match_single_binding)]
pub mod ledger_capnp {
    include!(concat!(env!("OUT_DIR"), "/economy/v1/ledger_capnp.rs"));
}
#[allow(dead_code, unused_imports, unused_parens, clippy::match_single_binding)]
pub mod syscall_capnp {
    include!(concat!(env!("OUT_DIR"), "/system/v1/syscall_capnp.rs"));
}
#[allow(dead_code, unused_imports, unused_parens, clippy::match_single_binding)]
pub mod sab_layout_capnp {
    include!(concat!(env!("OUT_DIR"), "/system/v1/sab_layout_capnp.rs"));
}
#[allow(dead_code, unused_imports, unused_parens, clippy::match_single_binding)]
pub mod identity_capnp {
    include!(concat!(env!("OUT_DIR"), "/identity/v1/identity_capnp.rs"));
}

pub mod protocols {
    pub use crate::actor_capnp as actor;
    pub use crate::base_capnp as base;
    pub use crate::capsule_capnp as compute;
    pub use crate::identity_capnp as identity;
    pub use crate::ledger_capnp as economy;
    pub use crate::orchestration_capnp as system;
    pub use crate::sab_layout_capnp as sab;
    pub use crate::sensor_capnp as io;
    pub use crate::syscall_capnp as syscall;
}

pub use context::{init_context, is_valid as is_context_valid};
pub use credits::{BudgetVerifier, CostTracker, ReplicationIncentive, ReplicationTier};
pub use identity::{
    get_module_id, set_module_id, IdentityContext, IdentityEntry, IdentityRegistry,
};
pub use logging::init_logging;
pub use shader_registry::{
    BindingProfile, GpuRequirements, ShaderManifest, ShaderMeta, ShaderRegistry, ValidationMetadata,
};
pub use signal::{
    Epoch, Reactor, IDX_ACTOR_EPOCH, IDX_INBOX_DIRTY, IDX_KERNEL_READY, IDX_OUTBOX_DIRTY,
    IDX_PANIC_STATE, IDX_SENSOR_EPOCH, IDX_STORAGE_EPOCH, IDX_SYSTEM_EPOCH,
};
pub use social_graph::{SocialEntry, SocialGraph};

// Re-export js-sys and JsValue for modules that need JavaScript interop
pub use crate::js_interop::JsValue;
pub use js_sys;
