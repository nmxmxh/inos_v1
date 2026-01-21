use chacha20poly1305::{
    aead::{Aead, KeyInit},
    ChaCha20Poly1305, Key, Nonce,
};
use rand::{rngs::StdRng, RngCore, SeedableRng};

use log::{error, info};

// Storage module bare-metal (no wasm-bindgen macros)
#[derive(Debug)]
pub struct StorageEngine {
    encryption_key: Key,
}

/// Standardized Memory Allocator for WebAssembly
#[no_mangle]
pub extern "C" fn vault_alloc(size: usize) -> *mut u8 {
    let mut buf = Vec::with_capacity(size);
    let ptr = buf.as_mut_ptr();
    std::mem::forget(buf);
    ptr
}

/// Standardized Initialization with SharedArrayBuffer
#[no_mangle]
pub extern "C" fn vault_init_with_sab() -> i32 {
    sdk::js_interop::console_log("[vault] DEBUG: Init function called", 3);
    // Use stable ABI to get global object
    let global = sdk::js_interop::get_global();
    let sab_key = sdk::js_interop::create_string("__INOS_SAB__");
    let sab_val = sdk::js_interop::reflect_get(&global, &sab_key);

    let offset_key = sdk::js_interop::create_string("__INOS_SAB_OFFSET__");
    let offset_val = sdk::js_interop::reflect_get(&global, &offset_key);

    let size_key = sdk::js_interop::create_string("__INOS_SAB_SIZE__");
    let size_val = sdk::js_interop::reflect_get(&global, &size_key);

    let id_key = sdk::js_interop::create_string("__INOS_MODULE_ID__");
    let id_val = sdk::js_interop::reflect_get(&global, &id_key);

    if let (Ok(val), Ok(off), Ok(sz)) = (sab_val, offset_val, size_val) {
        if !val.is_undefined() && !val.is_null() {
            let offset = sdk::js_interop::as_f64(&off).unwrap_or(0.0) as u32;
            let size = sdk::js_interop::as_f64(&sz).unwrap_or(0.0) as u32;
            let module_id = id_val.ok().and_then(|v| v.as_f64()).unwrap_or(0.0) as u32;

            // Create TWO SafeSAB references:
            // 1. Scoped view for module data
            let _module_sab = sdk::sab::SafeSAB::new_shared_view(&val, offset, size);
            // 2. Global SAB for registry and buffer writes (uses absolute layout offsets)
            let global_sab = sdk::sab::SafeSAB::new(&val);

            sdk::set_module_id(module_id);
            sdk::identity::init_identity_from_js();
            sdk::init_logging();
            info!("Vault module initialized with synchronized SAB bridge (Offset: 0x{:x}, Size: {}MB)", 
                offset, size / 1024 / 1024);

            // Helper to register simple modules
            let register_storage = |sab: &sdk::sab::SafeSAB| {
                use sdk::registry::*;
                let id = "vault";
                let mut builder = ModuleEntryBuilder::new(id).version(1, 0, 0);
                builder = builder.capability("storage", false, 256);
                builder = builder.capability("encryption", false, 256);
                builder = builder.capability("compression", false, 256);
                builder = builder.capability("ledger", false, 512);

                match builder.build() {
                    Ok((mut entry, _, caps)) => {
                        if let Ok(offset) = write_capability_table(sab, &caps) {
                            entry.cap_table_offset = offset;
                        }
                        if let Ok((slot, _)) = find_slot_double_hashing(sab, id) {
                            match write_enhanced_entry(sab, slot, &entry) {
                                Ok(_) => info!("Registered module {} at slot {}", id, slot),
                                Err(e) => {
                                    error!("Failed to write registry entry for {}: {:?}", id, e)
                                }
                            }
                        } else {
                            error!("Could not find available slot for module {}", id);
                        }
                    }
                    Err(e) => error!("Failed to build module entry for {}: {:?}", id, e),
                }
            };

            register_storage(&global_sab);
            // Signal registry change to wake Go discovery loop
            sdk::registry::signal_registry_change(&global_sab);

            return 1;
        }
    }
    0
}

/// External poll entry point for JavaScript
#[no_mangle]
pub extern "C" fn vault_poll() {
    // High-frequency reactor for Vault
}

impl StorageEngine {
    pub fn new(key_bytes: &[u8]) -> Result<StorageEngine, String> {
        if key_bytes.len() != 32 {
            return Err("Key must be 32 bytes".to_string());
        }
        let key = Key::from_slice(key_bytes);
        Ok(StorageEngine {
            encryption_key: *key,
        })
    }

    /// Stores data with Brotli Compression -> ChaCha20 Encryption
    /// Returns: [Nonce (12B) | Encrypted Data]
    /// Returns: [Nonce (12B) | Encrypted Data]
    pub fn store_chunk(&self, data: &[u8]) -> Result<Vec<u8>, String> {
        // 1. Compress (Brotli)
        let compressed = sdk::compression::CompressionAlgorithm::Brotli
            .compress(data)
            .map_err(|e| e.to_string())?;

        // 2. Encrypt (ChaCha20-Poly1305)
        let cipher = ChaCha20Poly1305::new(&self.encryption_key);

        // Generate random nonce
        let mut nonce_bytes = [0u8; 12];
        let mut rng = StdRng::from_entropy(); // seeded from OS RNG (getrandom)
        rng.fill_bytes(&mut nonce_bytes);
        let nonce = Nonce::from_slice(&nonce_bytes);

        // Encrypt
        let ciphertext = cipher
            .encrypt(nonce, compressed.as_ref())
            .map_err(|e| e.to_string())?;

        // 3. Pack: [Nonce][Ciphertext]
        let mut result = Vec::with_capacity(12 + ciphertext.len());
        result.extend_from_slice(&nonce_bytes);
        result.extend_from_slice(&ciphertext);

        Ok(result)
    }

    /// Retrieves data: Decrypt ChaCha20 -> Decompress Brotli
    pub fn retrieve_chunk(&self, blob: &[u8]) -> Result<Vec<u8>, String> {
        if blob.len() < 12 {
            return Err("Blob too short".to_string());
        }

        // 1. Unpack
        let nonce_bytes = &blob[0..12];
        let ciphertext = &blob[12..];
        let nonce = Nonce::from_slice(nonce_bytes);

        // 2. Decrypt
        let cipher = ChaCha20Poly1305::new(&self.encryption_key);
        let compressed = cipher
            .decrypt(nonce, ciphertext)
            .map_err(|e| e.to_string())?;

        // 3. Decompress
        let decompressed = sdk::compression::CompressionAlgorithm::Brotli
            .decompress(&compressed)
            .map_err(|e| e.to_string())?;

        Ok(decompressed)
    }

    /// Stores data using Content-Addressable Storage (CAS)
    /// Returns: (BLAKE3 hash, encrypted blob)
    pub fn store_cas_chunk(&self, data: &[u8]) -> Result<(String, Vec<u8>), String> {
        // 1. Compute BLAKE3 hash for deduplication
        let hash = sdk::compression::hash_blake3(data);
        let hash_str = hex::encode(&hash);

        // 2. Store using standard encryption pipeline
        let blob = self.store_chunk(data)?;

        Ok((hash_str, blob))
    }

    /// Retrieves data from CAS by hash (for verification)
    /// Note: In production, hash would be used for DHT lookup to find nodes
    pub fn retrieve_cas_chunk(&self, blob: &[u8], expected_hash: &str) -> Result<Vec<u8>, String> {
        // 1. Retrieve and decrypt
        let data = self.retrieve_chunk(blob)?;

        // 2. Verify hash matches
        let actual_hash = sdk::compression::hash_blake3(&data);
        let actual_hash_str = hex::encode(&actual_hash);

        if actual_hash_str != expected_hash {
            return Err(format!(
                "Hash mismatch: expected {}, got {}",
                expected_hash, actual_hash_str
            ));
        }

        Ok(data)
    }
}

#[cfg(test)]
mod tests;

#[cfg(test)]
mod cas_tests;
