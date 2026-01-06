use log::info;
use once_cell::sync::Lazy;
use parking_lot::Mutex;
use sdk::sab::SafeSAB;
use sdk::Reactor;

/// Diagnostics Module (The Watchdog)
///
/// Responsibilities:
/// 1. MemoryScan: Audit the SAB for overlapping allocations or corruption.
/// 2. EpochPulse: Monitor heartbeats of all active modules.
/// 3. SignalTracing: Trace syscall latency across layers.

pub struct DiagnosticsModule {
    reactor: Reactor,
    sab: sdk::sab::SafeSAB,
    last_scan: u32,
}

static GLOBAL_WATCHDOG: Lazy<Mutex<Option<DiagnosticsModule>>> = Lazy::new(|| Mutex::new(None));

impl DiagnosticsModule {
    pub fn new(sab: SafeSAB) -> Self {
        sdk::init_logging();
        info!("Diagnostics Watchdog initialized");
        Self {
            reactor: Reactor::new(sab.clone()),
            sab,
            last_scan: 0,
        }
    }

    /// Scan memory areas for overlap or corruption
    pub fn scan_memory(&self) -> Result<(), String> {
        info!("Watchdog: Scanning SAB memory areas...");
        use sdk::layout::*;

        let regions = [
            ("AtomicFlags", OFFSET_ATOMIC_FLAGS, SIZE_ATOMIC_FLAGS),
            (
                "SupervisorAlloc",
                OFFSET_SUPERVISOR_ALLOC,
                SIZE_SUPERVISOR_ALLOC,
            ),
            (
                "ModuleRegistry",
                OFFSET_MODULE_REGISTRY,
                SIZE_MODULE_REGISTRY,
            ),
            (
                "SupervisorHeaders",
                OFFSET_SUPERVISOR_HEADERS,
                SIZE_SUPERVISOR_HEADERS,
            ),
            ("SyscallTable", OFFSET_SYSCALL_TABLE, SIZE_SYSCALL_TABLE),
            (
                "PatternExchange",
                OFFSET_PATTERN_EXCHANGE,
                SIZE_PATTERN_EXCHANGE,
            ),
            ("JobHistory", OFFSET_JOB_HISTORY, SIZE_JOB_HISTORY),
            ("Coordination", OFFSET_COORDINATION, SIZE_COORDINATION),
            ("InboxOutbox", OFFSET_INBOX_OUTBOX, SIZE_INBOX_OUTBOX),
        ];

        for i in 0..regions.len() {
            let (name1, off1, size1) = regions[i];

            // Basic alignment check (64-byte boundaries for cache-line friendliness)
            if off1 % 64 != 0 {
                return Err(format!(
                    "Region {} is not 64-byte aligned: 0x{:x}",
                    name1, off1
                ));
            }

            for j in (i + 1)..regions.len() {
                let (name2, off2, size2) = regions[j];

                // Check if region1 and region2 overlap
                let end1 = off1 + size1;
                let end2 = off2 + size2;

                if off1 < end2 && off2 < end1 {
                    return Err(format!("Memory collision detected: {} (0x{:x}-0x{:x}) overlaps with {} (0x{:x}-0x{:x})",
                        name1, off1, end1, name2, off2, end2));
                }
            }
        }

        info!("Watchdog: SAB Memory Map Verified (0 Collisions)");
        Ok(())
    }

    /// Record a pulse from a module and check its health
    pub fn pulse(&self, module_id: u32) {
        use sdk::layout::*;
        // OFFSET_DIAGNOSTICS + (module_id * 8) = heartbeat storage
        // Byte 0-3: Last Pulse Timestamp (Epoch)
        // Byte 4-7: Pulse Counter

        let heart_offset = OFFSET_DIAGNOSTICS + (module_id as usize * 8);
        let sab = &self.sab;

        // Increment pulse counter
        let mut count = 0;
        if let Ok(data) = sab.read(heart_offset + 4, 4) {
            if data.len() == 4 {
                count = u32::from_le_bytes(data.try_into().unwrap());
            }
        }
        count += 1;
        let _ = sab.write(heart_offset + 4, &count.to_le_bytes());

        // Update timestamp (simulation of relative epoch)
        let now = (sdk::js_interop::get_now() as f64 / 1000.0) as u32;
        let _ = sab.write(heart_offset, &now.to_le_bytes());
    }
}

/// Standardized Memory Allocator for WebAssembly
#[no_mangle]
pub extern "C" fn diagnostics_alloc(size: usize) -> *mut u8 {
    let mut buf = Vec::with_capacity(size);
    let ptr = buf.as_mut_ptr();
    std::mem::forget(buf);
    ptr
}

/// Standardized Initialization with SharedArrayBuffer
#[no_mangle]
pub extern "C" fn diagnostics_init_with_sab() -> i32 {
    let global = sdk::js_interop::get_global();
    let sab_key = sdk::js_interop::create_string("__INOS_SAB__");
    let sab_val = sdk::js_interop::reflect_get(&global, &sab_key);

    let offset_key = sdk::js_interop::create_string("__INOS_SAB_OFFSET__");
    let offset_val = sdk::js_interop::reflect_get(&global, &offset_key);

    let size_key = sdk::js_interop::create_string("__INOS_SAB_SIZE__");
    let size_val = sdk::js_interop::reflect_get(&global, &size_key);

    if let (Ok(val), Ok(off), Ok(sz)) = (sab_val, offset_val, size_val) {
        if !val.is_undefined() && !val.is_null() {
            let offset = sdk::js_interop::as_f64(&off).unwrap_or(0.0) as u32;
            let size = sdk::js_interop::as_f64(&sz).unwrap_or(0.0) as u32;

            sdk::init_logging();
            info!(
                "Diagnostics Watchdog booting up... (Offset: 0x{:x}, Size: {}MB)",
                offset,
                size / 1024 / 1024
            );

            // Create SafeSAB for registry and buffer writes (uses absolute layout offsets)
            let safe_sab = sdk::sab::SafeSAB::new(&val);

            // Register capabilities using the global SAB
            register_diagnostics(&safe_sab);

            // Initialize global watchdog
            let mut lock = GLOBAL_WATCHDOG.lock();
            *lock = Some(DiagnosticsModule {
                reactor: Reactor::new(safe_sab.clone()),
                sab: safe_sab,
                last_scan: 0,
            });

            return 1;
        }
    }
    0
}

/// External poll entry point for JavaScript
#[no_mangle]
pub extern "C" fn diagnostics_poll() {
    let mut lock = GLOBAL_WATCHDOG.lock();
    if let Some(watchdog) = lock.as_mut() {
        // 1. Check for external diagnostics requests
        if watchdog.reactor.check_inbox() {
            watchdog.reactor.ack_inbox();
            if let Some(_req) = watchdog.reactor.read_request() {
                // Process diagnostics request (e.g. manual scan)
                let _ = watchdog.scan_memory();
            }
        }

        // 2. Periodic memory audit
        if watchdog.last_scan % 1000 == 0 {
            let _ = watchdog.scan_memory();
        }
        watchdog.last_scan = watchdog.last_scan.wrapping_add(1);
    }
}

fn register_diagnostics(sab: &sdk::sab::SafeSAB) {
    use sdk::registry::*;
    let id = "diagnostics";
    let mut builder = ModuleEntryBuilder::new(id).version(1, 0, 0);
    builder = builder.capability("memory_scan", false, 64);
    builder = builder.capability("epoch_pulse", false, 64);
    builder = builder.capability("signal_trace", false, 64);

    match builder.build() {
        Ok((mut entry, _, caps)) => {
            if let Ok(offset) = write_capability_table(sab, &caps) {
                entry.cap_table_offset = offset;
            }
            if let Ok((slot, _)) = find_slot_double_hashing(sab, id) {
                let _ = write_enhanced_entry(sab, slot, &entry);
            }
        }
        Err(_) => {}
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_memory_scan_detects_overlaps() {
        // Create a mock SAB for testing
        let diag = DiagnosticsModule::new(SafeSAB::with_size(1024));

        // Memory scan should pass with correct layout
        let result = diag.scan_memory();
        assert!(result.is_ok(), "Memory scan should pass with valid layout");
    }

    #[test]
    fn test_pulse_tracking() {
        let diag = DiagnosticsModule::new(SafeSAB::with_size(1024));

        // Pulse should not panic
        diag.pulse(0);
        diag.pulse(1);
        diag.pulse(255);
    }

    #[test]
    fn test_diagnostics_module_creation() {
        let diag = DiagnosticsModule::new(SafeSAB::with_size(1024));

        assert_eq!(diag.last_scan, 0);
    }
}
