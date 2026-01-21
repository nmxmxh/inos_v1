use crate::js_interop;
use std::sync::atomic::{AtomicI32, Ordering};

/// The context hash that was active when this module was initialized.
/// Stored as an atomic for lock-free access.
static INITIAL_CONTEXT_HASH: AtomicI32 = AtomicI32::new(0);

/// Read the current context hash from SAB using atomic load.
/// This is a single CPU instruction with zero JS allocations.
fn get_current_context_hash() -> i32 {
    crate::sab::with_global_barrier_view(|view| {
        crate::js_interop::atomic_load(view, crate::layout::IDX_CONTEXT_ID_HASH)
    })
    .unwrap_or(0)
}

/// Captures the current context hash from SAB.
/// This should be called once during `init_with_sab`.
pub fn init_context() {
    let current_hash = get_current_context_hash();
    if current_hash != 0 {
        let prev = INITIAL_CONTEXT_HASH.compare_exchange(
            0,
            current_hash,
            Ordering::SeqCst,
            Ordering::SeqCst,
        );
        if prev.is_ok() {
            js_interop::console_log(
                &format!("[SDK] Context Initialized (Hash: {})", current_hash),
                3,
            );
        }
    }
}

/// Checks if the current SAB context hash matches the one captured at initialization.
/// Returns `true` if the context is still valid, `false` if the module is now a "Zombie".
///
/// PERFORMANCE: This is a single atomic read from SAB - zero JS allocations.
pub fn is_valid() -> bool {
    let initial = INITIAL_CONTEXT_HASH.load(Ordering::Relaxed);
    if initial == 0 {
        return true; // Not initialized yet, assume valid
    }

    let current = get_current_context_hash();
    if current == 0 {
        return true; // SAB not available, assume valid
    }

    if current != initial {
        js_interop::console_log(
            &format!(
                "[SDK] 💀 Context Mismatch! Zombie Module. Current: {}, Initial: {}",
                current, initial
            ),
            1,
        );
        false
    } else {
        true
    }
}
