use serde::{Deserialize, Serialize};
use std::io::{Read, Write};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum CompressionError {
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
    #[error("Brotli error: {0}")]
    Brotli(String),
    #[error("Snappy error: {0}")]
    Snappy(String),
    #[error("LZ4 error: {0}")]
    Lz4(String),
    #[error("Unsupported algorithm")]
    Unsupported,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[repr(u8)]
pub enum CompressionAlgorithm {
    None = 0,
    Brotli = 1,
    Snappy = 2,
    Lz4 = 3,
}

impl CompressionAlgorithm {
    pub fn compress(&self, data: &[u8]) -> Result<Vec<u8>, CompressionError> {
        match self {
            CompressionAlgorithm::None => Ok(data.to_vec()),
            CompressionAlgorithm::Brotli => compress_brotli(data),
            CompressionAlgorithm::Snappy => compress_snappy(data),
            CompressionAlgorithm::Lz4 => compress_lz4(data),
        }
    }

    pub fn decompress(&self, data: &[u8]) -> Result<Vec<u8>, CompressionError> {
        match self {
            CompressionAlgorithm::None => Ok(data.to_vec()),
            CompressionAlgorithm::Brotli => decompress_brotli(data),
            CompressionAlgorithm::Snappy => decompress_snappy(data),
            CompressionAlgorithm::Lz4 => decompress_lz4(data),
        }
    }
}

fn compress_brotli(data: &[u8]) -> Result<Vec<u8>, CompressionError> {
    let mut compressor = brotli::CompressorReader::new(data, 4096, 6, 20); // Q=6, lgwin=20
    let mut compressed = Vec::new();
    compressor
        .read_to_end(&mut compressed)
        .map_err(|e| CompressionError::Brotli(e.to_string()))?;
    Ok(compressed)
}

fn decompress_brotli(data: &[u8]) -> Result<Vec<u8>, CompressionError> {
    let mut decompressor = brotli::Decompressor::new(data, 4096);
    let mut decompressed = Vec::new();
    decompressor
        .read_to_end(&mut decompressed)
        .map_err(|e| CompressionError::Brotli(e.to_string()))?;
    Ok(decompressed)
}

fn compress_snappy(data: &[u8]) -> Result<Vec<u8>, CompressionError> {
    let mut encoder = snap::write::FrameEncoder::new(Vec::new());
    encoder.write_all(data)?;
    encoder
        .into_inner()
        .map_err(|e| CompressionError::Snappy(e.error().to_string()))
}

fn decompress_snappy(data: &[u8]) -> Result<Vec<u8>, CompressionError> {
    let mut decoder = snap::read::FrameDecoder::new(data);
    let mut decompressed = Vec::new();
    decoder
        .read_to_end(&mut decompressed)
        .map_err(|e| CompressionError::Snappy(e.to_string()))?;
    Ok(decompressed)
}

fn compress_lz4(data: &[u8]) -> Result<Vec<u8>, CompressionError> {
    Ok(lz4_flex::compress_prepend_size(data))
}

fn decompress_lz4(data: &[u8]) -> Result<Vec<u8>, CompressionError> {
    lz4_flex::decompress_size_prepended(data).map_err(|e| CompressionError::Lz4(e.to_string()))
}

/// Computes BLAKE3 hash for content-addressable storage
/// Returns 32-byte hash suitable for deduplication and integrity verification
pub fn hash_blake3(data: &[u8]) -> [u8; 32] {
    use blake3::Hasher;
    let mut hasher = Hasher::new();
    hasher.update(data);
    let hash = hasher.finalize();
    *hash.as_bytes()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_hash_blake3_deterministic() {
        let data = b"test data";
        let hash1 = hash_blake3(data);
        let hash2 = hash_blake3(data);
        assert_eq!(hash1, hash2, "Hash should be deterministic");
    }

    #[test]
    fn test_hash_blake3_different_data() {
        let hash1 = hash_blake3(b"data1");
        let hash2 = hash_blake3(b"data2");
        assert_ne!(
            hash1, hash2,
            "Different data should produce different hashes"
        );
    }

    #[test]
    fn test_hash_blake3_empty() {
        let hash = hash_blake3(b"");
        assert_eq!(hash.len(), 32, "Should return 32-byte hash");
    }
}
