#[cfg(test)]
mod cas_tests {
    use super::super::StorageEngine;

    // ========== CAS (Content-Addressable Storage) TESTS ==========

    #[test]
    fn test_cas_basic_roundtrip() {
        let key = [1u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        let data = b"Hello, CAS!";
        let (hash, blob) = engine
            .store_cas_chunk(data)
            .expect("Failed to store CAS chunk");

        // Hash should be deterministic (BLAKE3)
        assert_eq!(
            hash.len(),
            64,
            "BLAKE3 hash should be 32 bytes (64 hex chars)"
        );

        let retrieved = engine
            .retrieve_cas_chunk(&blob, &hash)
            .expect("Failed to retrieve CAS chunk");
        assert_eq!(retrieved, data, "Retrieved data should match original");
    }

    #[test]
    fn test_cas_deduplication() {
        let key = [2u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        let data = b"Duplicate data";

        // Store same data twice
        let (hash1, _blob1) = engine
            .store_cas_chunk(data)
            .expect("Failed to store first chunk");
        let (hash2, _blob2) = engine
            .store_cas_chunk(data)
            .expect("Failed to store second chunk");

        // Hashes should be identical (deduplication)
        assert_eq!(
            hash1, hash2,
            "Identical data should produce identical hashes"
        );
    }

    #[test]
    fn test_cas_different_data_different_hash() {
        let key = [3u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        let data1 = b"Data A";
        let data2 = b"Data B";

        let (hash1, _) = engine
            .store_cas_chunk(data1)
            .expect("Failed to store chunk 1");
        let (hash2, _) = engine
            .store_cas_chunk(data2)
            .expect("Failed to store chunk 2");

        assert_ne!(
            hash1, hash2,
            "Different data should produce different hashes"
        );
    }

    #[test]
    fn test_cas_hash_verification_fails_on_mismatch() {
        let key = [4u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        let data = b"Original data";
        let (_hash, blob) = engine
            .store_cas_chunk(data)
            .expect("Failed to store CAS chunk");

        // Try to retrieve with wrong hash
        let wrong_hash = "0".repeat(64);
        let result = engine.retrieve_cas_chunk(&blob, &wrong_hash);

        assert!(result.is_err(), "Should fail with wrong hash");
        assert!(
            result.unwrap_err().contains("Hash mismatch"),
            "Error should mention hash mismatch"
        );
    }

    #[test]
    fn test_cas_large_chunk() {
        let key = [5u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        // 1MB chunk
        let data = vec![0xAB; 1024 * 1024];
        let (hash, blob) = engine
            .store_cas_chunk(&data)
            .expect("Failed to store large CAS chunk");

        let retrieved = engine
            .retrieve_cas_chunk(&blob, &hash)
            .expect("Failed to retrieve large CAS chunk");

        assert_eq!(retrieved.len(), data.len(), "Size should match");
        assert_eq!(retrieved, data, "Data should match");
    }

    #[test]
    fn test_cas_empty_data() {
        let key = [6u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        let data = b"";
        let (hash, blob) = engine
            .store_cas_chunk(data)
            .expect("Failed to store empty data");

        // Empty data should still produce a valid hash
        assert_eq!(hash.len(), 64);

        let retrieved = engine
            .retrieve_cas_chunk(&blob, &hash)
            .expect("Failed to retrieve empty data");
        assert_eq!(retrieved, data);
    }

    #[test]
    fn test_cas_hash_collision_resistant() {
        let key = [7u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        // Test similar but different data
        let data1 = b"Data1";
        let data2 = b"Data2"; // Only 1 char different

        let (hash1, _) = engine.store_cas_chunk(data1).expect("Failed to store 1");
        let (hash2, _) = engine.store_cas_chunk(data2).expect("Failed to store 2");

        assert_ne!(
            hash1, hash2,
            "Similar data should still produce different hashes (no collision)"
        );
    }

    #[test]
    fn test_cas_retrieval_with_corrupted_blob() {
        let key = [8u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        let data = b"Important data";
        let (hash, mut blob) = engine
            .store_cas_chunk(data)
            .expect("Failed to store CAS chunk");

        // Corrupt the blob
        if blob.len() > 20 {
            blob[20] ^= 0xFF;
        }

        // Hash verification should fail because decryption will fail
        let result = engine.retrieve_cas_chunk(&blob, &hash);
        assert!(result.is_err(), "Should fail with corrupted blob");
    }

    #[test]
    fn test_cas_concurrent_storage() {
        use std::sync::Arc;
        use std::thread;

        let key = [9u8; 32];
        let engine = Arc::new(StorageEngine::new(&key).expect("Failed to create engine"));

        let mut handles = vec![];

        for i in 0..10 {
            let engine_clone = Arc::clone(&engine);
            let handle = thread::spawn(move || {
                let data = format!("Data {}", i).into_bytes();
                let (hash, blob) = engine_clone
                    .store_cas_chunk(&data)
                    .expect(&format!("Failed to store in thread {}", i));
                let retrieved = engine_clone
                    .retrieve_cas_chunk(&blob, &hash)
                    .expect(&format!("Failed to retrieve in thread {}", i));
                assert_eq!(retrieved, data, "Data mismatch in thread {}", i);
                hash
            });
            handles.push(handle);
        }

        let mut hashes = vec![];
        for handle in handles {
            let hash = handle.join().expect("Thread panicked");
            hashes.push(hash);
        }

        // All hashes should be unique (different data)
        let unique_hashes: std::collections::HashSet<_> = hashes.iter().collect();
        assert_eq!(
            unique_hashes.len(),
            10,
            "All 10 different chunks should have unique hashes"
        );
    }

    #[test]
    fn test_cas_binary_data() {
        let key = [10u8; 32];
        let engine = StorageEngine::new(&key).expect("Failed to create engine");

        // All byte values
        let data: Vec<u8> = (0..=255).collect();
        let (hash, blob) = engine
            .store_cas_chunk(&data)
            .expect("Failed to store binary data");

        let retrieved = engine
            .retrieve_cas_chunk(&blob, &hash)
            .expect("Failed to retrieve binary data");
        assert_eq!(retrieved, data, "Binary data should roundtrip correctly");
    }
}
