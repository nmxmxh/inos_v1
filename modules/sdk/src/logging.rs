use log::{Level, LevelFilter, Metadata, Record};

struct WebLogger;

impl log::Log for WebLogger {
    fn enabled(&self, metadata: &Metadata) -> bool {
        metadata.level() <= Level::Info
    }

    fn log(&self, record: &Record) {
        if self.enabled(record.metadata()) {
            let msg = format!("[{}] {}", record.target(), record.args());
            let level = match record.level() {
                Level::Error => 0,
                Level::Warn => 1,
                Level::Info => 2,
                Level::Debug => 3,
                Level::Trace => 4,
            };
            // Use stable ABI for logging
            crate::js_interop::console_log(&msg, level);
        }
    }

    fn flush(&self) {}
}

static LOGGER: WebLogger = WebLogger;

pub fn init_logging() {
    // Idempotent: ignore error if logger is already set (common in multi-module WASM)
    let _ = log::set_logger(&LOGGER).map(|()| log::set_max_level(LevelFilter::Info));

    // Set panic hook to report errors to JS console via stable ABI
    std::panic::set_hook(Box::new(|info| {
        let payload = info.payload();
        let message = if let Some(s) = payload.downcast_ref::<&str>() {
            s.to_string()
        } else if let Some(s) = payload.downcast_ref::<String>() {
            s.clone()
        } else {
            "unspecified panic".to_string()
        };

        let location = info
            .location()
            .map(|loc| format!(" at {}:{}:{}", loc.file(), loc.line(), loc.column()))
            .unwrap_or_default();

        let full_msg = format!("| RUST PANIC | {}{}", message, location);
        crate::js_interop::console_log(&full_msg, 0); // 0 = Error level
    }));
}
