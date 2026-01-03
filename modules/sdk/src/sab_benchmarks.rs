use crate::sab::SafeSAB;
use crate::signal::Epoch;
use std::time::Instant;
use web_sys::wasm_bindgen::JsValue;

/// Architectural Benchmarks for zero-copy SAB and Epoch signaling
#[cfg(test)]
mod benchmarks {
    use super::*;

    #[test]
    fn benchmark_sab_throughput() {
        let sab = JsValue::UNDEFINED;
        let safe_sab = SafeSAB::new(&sab);
        let data = vec![0u8; 1024 * 1024]; // 1MB chunk
        let iterations = 1000;

        println!("\n--- SAB Throughput Benchmark ---");
        let start = Instant::now();
        for _ in 0..iterations {
            safe_sab.write(0, &data).unwrap();
        }
        let duration = start.elapsed();
        let total_gb = (iterations as f64 * data.len() as f64) / (1024.0 * 1024.0 * 1024.0);
        let gb_per_sec = total_gb / duration.as_secs_f64();

        println!("Write Speed: {:.2} GB/s", gb_per_sec);

        let start = Instant::now();
        for _ in 0..iterations {
            let _ = safe_sab.read(0, data.len()).unwrap();
        }
        let duration = start.elapsed();
        let gb_per_sec = total_gb / duration.as_secs_f64();
        println!("Read Speed:  {:.2} GB/s", gb_per_sec);
    }

    #[test]
    fn benchmark_epoch_latency() {
        let sab = JsValue::UNDEFINED;
        let mut epoch = Epoch::new(&sab, 0);
        let iterations = 1_000_000;

        println!("\n--- Epoch Signaling Latency ---");
        let start = Instant::now();
        for _ in 0..iterations {
            epoch.increment();
        }
        let duration = start.elapsed();
        let ns_per_signal = duration.as_nanos() as f64 / iterations as f64;
        println!("Increment Latency: {:.2} ns/op", ns_per_signal);

        let start = Instant::now();
        for _ in 0..iterations {
            let _ = epoch.has_changed();
        }
        let duration = start.elapsed();
        let ns_per_signal = duration.as_nanos() as f64 / iterations as f64;
        println!("Check Latency:     {:.2} ns/op", ns_per_signal);
    }

    #[test]
    fn benchmark_ringbuffer_throughput() {
        use crate::ringbuffer::RingBuffer;
        let sab = JsValue::UNDEFINED;
        let safe_sab = SafeSAB::new(&sab);
        let rb = RingBuffer::new(safe_sab, 1024, 64 * 1024);
        let msg = vec![0u8; 256];
        let iterations = 100_000;

        println!("\n--- RingBuffer Throughput ---");
        let start = Instant::now();
        for _ in 0..iterations {
            rb.write_message(&msg).unwrap();
            let _ = rb.read_message().unwrap();
        }
        let duration = start.elapsed();
        let ops_per_sec = iterations as f64 / duration.as_secs_f64();
        println!("Write/Read Cycle: {:.2} ops/sec", ops_per_sec);
    }
}
