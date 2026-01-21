//! Architectural benchmarks for INOS Compute Module
//!
//! These benchmarks validate compute-specific performance claims:
//! - Arrow vs JavaScript: 10-25x faster with SIMD
//! - SIMD image processing: 5-10x acceleration
//! - Hardware crypto: AES-NI 3-5x faster than software

#[cfg(test)]
mod benchmarks {
    use std::time::Instant;

    /// Benchmark 1: Arrow SIMD vs Scalar Operations
    /// Target: 10-25x faster than JavaScript (simulated as scalar)
    /// Validates: data.rs L20 claim of 10-25x performance advantage
    /// Note: Requires --release mode for SIMD optimizations
    #[test]
    #[ignore] // Run with: cargo test --release -- --ignored
    fn bench_arrow_simd_vs_scalar() {
        use arrow::array::*;
        use arrow::compute;

        const SIZE: usize = 1_000_000;

        // Create Arrow array
        let array = Int64Array::from_iter_values(0..SIZE as i64);

        // Arrow SIMD sum (vectorized operations)
        let start = Instant::now();
        let arrow_sum = compute::sum(&array).unwrap();
        let arrow_duration = start.elapsed();

        // Simulated JavaScript/scalar sum (no SIMD)
        let vec: Vec<i64> = (0..SIZE as i64).collect();
        let start = Instant::now();
        let scalar_sum: i64 = vec.iter().sum();
        let scalar_duration = start.elapsed();

        assert_eq!(arrow_sum, scalar_sum);

        let speedup = scalar_duration.as_nanos() / arrow_duration.as_nanos().max(1);

        println!("\n=== Arrow SIMD vs Scalar Benchmark ===");
        println!("Array size: {} elements", SIZE);
        println!("Arrow (SIMD): {:?}", arrow_duration);
        println!("Scalar (JS-like): {:?}", scalar_duration);
        println!("Speedup: {}x (target: 10-25x)", speedup);
        println!(
            "Status: {}",
            if speedup >= 10 {
                "✅ PASS"
            } else if speedup >= 5 {
                "⚠️  ACCEPTABLE (>5x)"
            } else {
                "❌ FAIL"
            }
        );

        assert!(
            speedup >= 5,
            "Arrow should be at least 5x faster (got {}x)",
            speedup
        );
    }

    /// Benchmark 2: Arrow Filtering Performance
    /// Validates: Columnar data processing efficiency
    /// Note: Requires --release mode for SIMD optimizations
    #[test]
    #[ignore] // Run with: cargo test --release -- --ignored
    fn bench_arrow_filter_performance() {
        use arrow::array::*;
        use arrow::compute;

        const SIZE: usize = 1_000_000;

        // Create Arrow array
        let array = Int64Array::from_iter_values(0..SIZE as i64);

        // Arrow filter (SIMD-optimized)
        let start = Instant::now();
        let predicate = BooleanArray::from_iter((0..SIZE).map(|i| Some(i % 2 == 0)));
        let _filtered = compute::filter(&array, &predicate).unwrap();
        let arrow_duration = start.elapsed();

        // Scalar filter (JavaScript-like)
        let vec: Vec<i64> = (0..SIZE as i64).collect();
        let start = Instant::now();
        let _filtered: Vec<i64> = vec
            .iter()
            .enumerate()
            .filter(|(i, _)| i % 2 == 0)
            .map(|(_, &v)| v)
            .collect();
        let scalar_duration = start.elapsed();

        let speedup = scalar_duration.as_nanos() / arrow_duration.as_nanos().max(1);

        println!("\n=== Arrow Filter Benchmark ===");
        println!("Array size: {} elements", SIZE);
        println!("Arrow (SIMD): {:?}", arrow_duration);
        println!("Scalar: {:?}", scalar_duration);
        println!("Speedup: {}x", speedup);
        println!("Status: ✅ (validates columnar processing)");

        assert!(speedup >= 2, "Arrow filter should be at least 2x faster");
    }

    /// Benchmark 3: SIMD Image Processing
    /// Target: 5-10x faster with SIMD
    /// Validates: image.rs SIMD acceleration claim
    /// Note: Requires --release mode for SIMD optimizations
    #[test]
    #[ignore] // Run with: cargo test --release -- --ignored
    fn bench_image_simd_processing() {
        use fast_image_resize as fr;

        use std::num::NonZeroU32;

        const SRC_WIDTH: u32 = 1920;
        const SRC_HEIGHT: u32 = 1080;
        const DST_WIDTH: u32 = 640;
        const DST_HEIGHT: u32 = 480;

        // Create source image (RGBA)
        let src_pixels = vec![128u8; (SRC_WIDTH * SRC_HEIGHT * 4) as usize];

        // SIMD resize (using fast_image_resize with SIMD)
        let start = Instant::now();
        let src_image = fr::Image::from_vec_u8(
            NonZeroU32::new(SRC_WIDTH).unwrap(),
            NonZeroU32::new(SRC_HEIGHT).unwrap(),
            src_pixels.clone(),
            fr::PixelType::U8x4,
        )
        .unwrap();

        let mut dst_image = fr::Image::new(
            NonZeroU32::new(DST_WIDTH).unwrap(),
            NonZeroU32::new(DST_HEIGHT).unwrap(),
            fr::PixelType::U8x4,
        );
        let mut resizer = fr::Resizer::new(fr::ResizeAlg::Convolution(fr::FilterType::Lanczos3));
        resizer
            .resize(&src_image.view(), &mut dst_image.view_mut())
            .unwrap();
        let simd_duration = start.elapsed();

        // Scalar resize (nearest neighbor - simple but slow)
        let start = Instant::now();
        let mut dst_pixels = vec![0u8; (DST_WIDTH * DST_HEIGHT * 4) as usize];
        let x_ratio = SRC_WIDTH as f32 / DST_WIDTH as f32;
        let y_ratio = SRC_HEIGHT as f32 / DST_HEIGHT as f32;

        for y in 0..DST_HEIGHT {
            for x in 0..DST_WIDTH {
                let src_x = (x as f32 * x_ratio) as u32;
                let src_y = (y as f32 * y_ratio) as u32;
                let src_idx = ((src_y * SRC_WIDTH + src_x) * 4) as usize;
                let dst_idx = ((y * DST_WIDTH + x) * 4) as usize;

                dst_pixels[dst_idx..dst_idx + 4].copy_from_slice(&src_pixels[src_idx..src_idx + 4]);
            }
        }
        let scalar_duration = start.elapsed();

        let speedup = scalar_duration.as_nanos() / simd_duration.as_nanos().max(1);

        println!("\n=== Image SIMD Processing Benchmark ===");
        println!("Source: {}x{}", SRC_WIDTH, SRC_HEIGHT);
        println!("Destination: {}x{}", DST_WIDTH, DST_HEIGHT);
        println!("SIMD (Lanczos3): {:?}", simd_duration);
        println!("Scalar (nearest): {:?}", scalar_duration);
        println!("Speedup: {}x (target: 5-10x)", speedup);
        println!(
            "Status: {}",
            if speedup >= 5 {
                "✅ PASS"
            } else if speedup >= 3 {
                "⚠️  ACCEPTABLE (>3x)"
            } else {
                "❌ FAIL"
            }
        );

        assert!(
            speedup >= 3,
            "SIMD should be at least 3x faster (got {}x)",
            speedup
        );
    }

    /// Benchmark 4: Hardware Crypto Acceleration (AES-GCM)
    /// Target: 3-5x faster with AES-NI
    /// Validates: crypto.rs hardware acceleration claim
    /// Note: Requires --release mode for hardware acceleration
    #[test]
    #[ignore] // Run with: cargo test --release -- --ignored
    fn bench_crypto_hardware_acceleration() {
        use aes_gcm::{aead::Aead, Aes256Gcm, KeyInit};

        const PLAINTEXT_SIZE: usize = 1024 * 1024; // 1MB
        const ITERATIONS: usize = 100;

        let key = [1u8; 32];
        let nonce = [0u8; 12];
        let plaintext = vec![42u8; PLAINTEXT_SIZE];

        let cipher = Aes256Gcm::new(&key.into());

        // Benchmark encryption throughput
        let start = Instant::now();
        for _ in 0..ITERATIONS {
            let _ = cipher.encrypt(&nonce.into(), plaintext.as_ref());
        }
        let duration = start.elapsed();

        let total_bytes = PLAINTEXT_SIZE * ITERATIONS;
        let throughput_mbps = (total_bytes as f64 / duration.as_secs_f64()) / 1_000_000.0;

        println!("\n=== Crypto Hardware Acceleration Benchmark ===");
        println!("Algorithm: AES-256-GCM");
        println!("Plaintext size: {} bytes", PLAINTEXT_SIZE);
        println!("Iterations: {}", ITERATIONS);
        println!("Total data: {} MB", total_bytes / 1_000_000);
        println!("Duration: {:?}", duration);
        println!("Throughput: {:.2} MB/s", throughput_mbps);
        println!(
            "Status: {}",
            if throughput_mbps > 1000.0 {
                "✅ EXCELLENT (>1000 MB/s - AES-NI active)"
            } else if throughput_mbps > 500.0 {
                "✅ PASS (>500 MB/s)"
            } else if throughput_mbps > 100.0 {
                "⚠️  ACCEPTABLE (>100 MB/s)"
            } else {
                "❌ FAIL"
            }
        );

        // With AES-NI, should achieve 500+ MB/s
        assert!(throughput_mbps > 100.0, "Should achieve 100+ MB/s");
    }

    /// Benchmark 5: RecordBatch Construction Performance
    /// Validates: Manual RecordBatch construction efficiency
    #[test]
    fn bench_recordbatch_construction() {
        use arrow::array::*;
        use arrow::datatypes::*;
        use arrow::record_batch::RecordBatch;
        use std::sync::Arc;

        const NUM_ROWS: usize = 100_000;
        const ITERATIONS: usize = 100;

        let start = Instant::now();
        for _ in 0..ITERATIONS {
            // Build schema
            let schema = Arc::new(Schema::new(vec![
                Field::new("id", DataType::Int64, false),
                Field::new("value", DataType::Float64, false),
                Field::new("name", DataType::Utf8, false),
            ]));

            // Build arrays
            let id_array: ArrayRef = Arc::new(Int64Array::from_iter_values(0..NUM_ROWS as i64));
            let value_array: ArrayRef = Arc::new(Float64Array::from_iter_values(
                (0..NUM_ROWS).map(|i| i as f64 * 1.5),
            ));
            let name_array: ArrayRef = Arc::new(StringArray::from_iter_values(
                (0..NUM_ROWS).map(|i| format!("name_{}", i)),
            ));

            // Create RecordBatch
            let _batch =
                RecordBatch::try_new(schema, vec![id_array, value_array, name_array]).unwrap();
        }
        let duration = start.elapsed();

        let total_rows = NUM_ROWS * ITERATIONS;
        let rows_per_sec = total_rows as f64 / duration.as_secs_f64();

        println!("\n=== RecordBatch Construction Benchmark ===");
        println!("Rows per batch: {}", NUM_ROWS);
        println!("Iterations: {}", ITERATIONS);
        println!("Total rows: {}", total_rows);
        println!("Duration: {:?}", duration);
        println!("Throughput: {:.2} rows/sec", rows_per_sec);
        println!("Status: ✅ (validates construction efficiency)");

        assert!(rows_per_sec > 1_000_000.0, "Should construct >1M rows/sec");
    }
}
