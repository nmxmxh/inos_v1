#[cfg(test)]
mod tests {
    use super::super::StorageEngine;

    // ========== STORAGE ENGINE TESTS ==========
    // These tests validate the actual StorageEngine implementation

    #[test]
    fn test_storage_engine_creation() {
        let key = [0u8; 32];
        let engine = StorageEngine::new(&key);
        assert!(
            engine.is_ok(),
            "StorageEngine should be created with valid 32-byte key"
        );
    }

    #[test]
    fn test_storage_engine_invalid_key() {
        let key = [0u8; 16]; // Wrong size
        let engine = StorageEngine::new(&key);
        assert!(
            engine.is_err(),
            "StorageEngine should reject invalid key size"
        );
        assert_eq!(engine.unwrap_err(), "Key must be 32 bytes");
    }

    #[test]
    fn test_store_retrieve_small_chunk() {
        let key = [1u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        let data = b"Hello, INOS!";
        let blob = engine.store_chunk(data).expect("Failed to store chunk");

        // Blob should be: [12 bytes nonce][encrypted data]
        assert!(
            blob.len() > 12,
            "Blob should contain nonce + encrypted data"
        );

        let retrieved = engine
            .retrieve_chunk(&blob)
            .expect("Failed to retrieve chunk");
        assert_eq!(retrieved, data, "Retrieved data should match original");
    }

    #[test]
    fn test_store_retrieve_large_chunk() {
        let key = [2u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        // 1MB chunk
        let data = vec![0xAB; 1024 * 1024];
        let blob = engine
            .store_chunk(&data)
            .expect("Failed to store large chunk");

        let retrieved = engine
            .retrieve_chunk(&blob)
            .expect("Failed to retrieve large chunk");
        assert_eq!(
            retrieved.len(),
            data.len(),
            "Retrieved data length should match"
        );
        assert_eq!(retrieved, data, "Retrieved data should match original");
    }

    #[test]
    fn test_compression_reduces_size() {
        let key = [3u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        // Highly compressible data
        let data = vec![0u8; 10000];
        let blob = engine
            .store_chunk(&data)
            .expect("Failed to store compressible data");

        // Compressed + encrypted should be smaller than original
        // (accounting for nonce + auth tag overhead)
        // Brotli should compress 10000 zeros to much less
        assert!(
            blob.len() < data.len(),
            "Compressed blob ({} bytes) should be smaller than original ({} bytes)",
            blob.len(),
            data.len()
        );
    }

    #[test]
    fn test_encryption_changes_data() {
        let key = [4u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        let data = b"Secret data";
        let blob = engine.store_chunk(data).expect("Failed to encrypt data");

        // Encrypted data should not contain plaintext
        let encrypted_part = &blob[12..]; // Skip nonce
        assert!(
            !encrypted_part.windows(data.len()).any(|w| w == data),
            "Encrypted data should not contain plaintext"
        );
    }

    #[test]
    fn test_different_keys_produce_different_blobs() {
        let data = b"Same data";

        let key1 = [5u8; 32];
        let engine1 = StorageEngine::new(&key1).expect("Failed to create engine1");
        let blob1 = engine1
            .store_chunk(data)
            .expect("Failed to encrypt with key1");

        let key2 = [6u8; 32];
        let engine2 = StorageEngine::new(&key2).expect("Failed to create engine2");
        let blob2 = engine2
            .store_chunk(data)
            .expect("Failed to encrypt with key2");

        // Different keys should produce different ciphertexts
        // (even though nonces might differ, the encrypted parts should differ)
        assert_ne!(
            &blob1[12..],
            &blob2[12..],
            "Different keys should produce different ciphertexts"
        );
    }

    #[test]
    fn test_retrieve_with_wrong_key_fails() {
        let data = b"Encrypted data";

        let key1 = [7u8; 32];
        let engine1 = StorageEngine::new(&key1).expect("Failed to create engine1");
        let blob = engine1.store_chunk(data).expect("Failed to encrypt");

        let key2 = [8u8; 32];
        let engine2 = StorageEngine::new(&key2).expect("Failed to create engine2");
        let result = engine2.retrieve_chunk(&blob);

        assert!(result.is_err(), "Decryption with wrong key should fail");
    }

    #[test]
    fn test_retrieve_corrupted_blob_fails() {
        let key = [9u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        let data = b"Original data";
        let mut blob = engine.store_chunk(data).expect("Failed to encrypt");

        // Corrupt the blob (flip a bit in the encrypted part)
        if blob.len() > 20 {
            blob[20] ^= 0xFF;
        }

        let result = engine.retrieve_chunk(&blob);
        assert!(result.is_err(), "Decryption of corrupted blob should fail");
    }

    #[test]
    fn test_retrieve_truncated_blob_fails() {
        let key = [10u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        let data = b"Data to encrypt";
        let blob = engine.store_chunk(data).expect("Failed to encrypt");

        // Truncate blob (too short for nonce)
        let truncated = &blob[..10];

        let result = engine.retrieve_chunk(truncated);
        assert!(result.is_err(), "Decryption of truncated blob should fail");
        assert_eq!(result.unwrap_err(), "Blob too short");
    }

    #[test]
    fn test_empty_data_roundtrip() {
        let key = [11u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        let data = b"";
        let blob = engine
            .store_chunk(data)
            .expect("Failed to encrypt empty data");

        let retrieved = engine
            .retrieve_chunk(&blob)
            .expect("Failed to decrypt empty data");
        assert_eq!(retrieved, data, "Empty data should roundtrip correctly");
    }

    #[test]
    fn test_nonce_uniqueness() {
        let key = [12u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        let data = b"Same data";

        // Store same data twice
        let blob1 = engine
            .store_chunk(data)
            .expect("Failed to encrypt first time");
        let blob2 = engine
            .store_chunk(data)
            .expect("Failed to encrypt second time");

        // Nonces should be different (first 12 bytes)
        assert_ne!(
            &blob1[..12],
            &blob2[..12],
            "Nonces should be unique for each encryption"
        );

        // But both should decrypt correctly
        let retrieved1 = engine
            .retrieve_chunk(&blob1)
            .expect("Failed to decrypt blob1");
        let retrieved2 = engine
            .retrieve_chunk(&blob2)
            .expect("Failed to decrypt blob2");
        assert_eq!(retrieved1, data);
        assert_eq!(retrieved2, data);
    }

    #[test]
    fn test_concurrent_store_retrieve() {
        use std::sync::Arc;
        use std::thread;

        let key = [13u8; 32];
        let engine = Arc::new(StorageEngine::new(&key).expect("Failed to create engine"));

        let mut handles = vec![];

        for i in 0..10 {
            let engine_clone = Arc::clone(&engine);
            let handle = thread::spawn(move || {
                let data = format!("Data {}", i).into_bytes();
                let blob = engine_clone
                    .store_chunk(&data)
                    .expect(&format!("Failed to store in thread {}", i));
                let retrieved = engine_clone
                    .retrieve_chunk(&blob)
                    .expect(&format!("Failed to retrieve in thread {}", i));
                assert_eq!(retrieved, data, "Data mismatch in thread {}", i);
            });
            handles.push(handle);
        }

        for handle in handles {
            handle.join().expect("Thread panicked");
        }
    }

    #[test]
    fn test_binary_data_roundtrip() {
        let key = [14u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        // Binary data with all byte values
        let data: Vec<u8> = (0..=255).collect();
        let blob = engine
            .store_chunk(&data)
            .expect("Failed to encrypt binary data");

        let retrieved = engine
            .retrieve_chunk(&blob)
            .expect("Failed to decrypt binary data");
        assert_eq!(retrieved, data, "Binary data should roundtrip correctly");
    }

    #[test]
    fn test_max_chunk_size() {
        let key = [15u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        // 10MB chunk (large but reasonable)
        let data = vec![0x42; 10 * 1024 * 1024];
        let blob = engine
            .store_chunk(&data)
            .expect("Failed to store large chunk");

        let retrieved = engine
            .retrieve_chunk(&blob)
            .expect("Failed to retrieve large chunk");
        assert_eq!(retrieved.len(), data.len(), "Large chunk size should match");
        // Don't compare full data to save time, just check length and a sample
        assert_eq!(
            &retrieved[..100],
            &data[..100],
            "Large chunk data should match"
        );
    }

    // ========== PERFORMANCE TESTS ==========

    #[test]
    #[ignore] // Run with --ignored flag
    fn bench_store_1mb() {
        let key = [16u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");
        let data = vec![0u8; 1024 * 1024];

        let start = std::time::Instant::now();
        for _ in 0..10 {
            let _ = engine.store_chunk(&data).expect("Failed to store");
        }
        let elapsed = start.elapsed();

        let throughput_mb_s = (10.0 * 1024.0 * 1024.0) / elapsed.as_secs_f64() / 1024.0 / 1024.0;
        println!("10x 1MB store: {:?} ({:.2} MB/s)", elapsed, throughput_mb_s);

        // Should be reasonably fast (at least 10 MB/s)
        assert!(
            throughput_mb_s > 10.0,
            "Throughput should be > 10 MB/s, got {:.2}",
            throughput_mb_s
        );
    }

    #[test]
    #[ignore]
    fn bench_retrieve_1mb() {
        let key = [17u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");
        let data = vec![0u8; 1024 * 1024];
        let blob = engine.store_chunk(&data).expect("Failed to store");

        let start = std::time::Instant::now();
        for _ in 0..10 {
            let _ = engine.retrieve_chunk(&blob).expect("Failed to retrieve");
        }
        let elapsed = start.elapsed();

        let throughput_mb_s = (10.0 * 1024.0 * 1024.0) / elapsed.as_secs_f64() / 1024.0 / 1024.0;
        println!(
            "10x 1MB retrieve: {:?} ({:.2} MB/s)",
            elapsed, throughput_mb_s
        );

        // Should be reasonably fast (at least 10 MB/s)
        assert!(
            throughput_mb_s > 10.0,
            "Throughput should be > 10 MB/s, got {:.2}",
            throughput_mb_s
        );
    }
}
