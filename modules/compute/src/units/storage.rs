use serde::{Deserialize, Serialize};
use thiserror::Error;

use sdk::syscalls::{HostPayload, HostResponse, SyscallClient};

#[derive(Debug, Error)]
pub enum StorageError {
    #[error("Host call failed: {0}")]
    Host(String),

    #[error("Serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    #[error("Chunk not found: {0}")]
    NotFound(String),

    #[error("No shared buffer")]
    NoSharedBuffer,
}

#[derive(Clone, Debug, Serialize, Deserialize, Default)]
pub struct StorageParams {
    pub content_hash: String,
    pub priority: Option<String>,
    pub chunk_index: Option<u64>,
    pub model_id: Option<String>,
}

#[derive(Clone, Debug, Serialize, Deserialize, Default)]
pub struct ChunkMetadata {
    pub hash: String,
    pub size: u64,
    pub priority: String,
    pub last_accessed: u64,
    pub access_count: u64,
    pub model_id: Option<String>,
}

pub struct StorageUnit;

impl StorageUnit {
    pub fn new() -> Result<Self, StorageError> {
        Ok(Self)
    }

    pub async fn execute(
        &self,
        method: &str,
        input: &[u8],
        params: &str,
    ) -> Result<Vec<u8>, StorageError> {
        let params: StorageParams = serde_json::from_str(params)?;
        match method {
            "store_chunk" | "write" => {
                self.store_chunk(&params.content_hash, input, params.priority.as_deref())
                    .await?;
                Ok(Vec::new())
            }
            "store_chunk_zero_copy" => {
                let offset = params.chunk_index.unwrap_or(0) as u32;
                let size = input.len() as u32;
                self.store_chunk_zero_copy(&params.content_hash, offset, size, params.priority.as_deref())
                    .await?;
                Ok(Vec::new())
            }
            "load_chunk" | "read" => self.load_chunk(&params.content_hash).await,
            "delete_chunk" | "delete" => {
                self.delete_chunk(&params.content_hash).await?;
                Ok(Vec::new())
            }
            "query_index" => self.query_index(&params).await,
            _ => Err(StorageError::Host(format!(
                "Unknown storage method: {}",
                method
            ))),
        }
    }

    async fn store_chunk(
        &self,
        hash: &str,
        data: &[u8],
        priority: Option<&str>,
    ) -> Result<(), StorageError> {
        let sab = crate::get_cached_sab().ok_or(StorageError::NoSharedBuffer)?;
        let meta = serde_json::json!({
            "hash": hash,
            "priority": priority.unwrap_or("medium"),
            "method": "store",
        });
        let custom = serde_json::to_vec(&meta)?;

        let response = SyscallClient::host_call(
            &sab,
            "storage.store_chunk",
            HostPayload::Inline(data),
            Some(&custom),
        )
        .await
        .map_err(StorageError::Host)?;

        match response {
            HostResponse::Inline { .. } | HostResponse::SabRef { .. } => Ok(()),
        }
    }

    async fn store_chunk_zero_copy(
        &self,
        hash: &str,
        offset: u32,
        size: u32,
        priority: Option<&str>,
    ) -> Result<(), StorageError> {
        let sab = crate::get_cached_sab().ok_or(StorageError::NoSharedBuffer)?;
        let meta = serde_json::json!({
            "hash": hash,
            "priority": priority.unwrap_or("medium"),
            "method": "store",
        });
        let custom = serde_json::to_vec(&meta)?;

        let response = SyscallClient::host_call(
            &sab,
            "storage.store_chunk",
            HostPayload::SabRef { offset, size },
            Some(&custom),
        )
        .await
        .map_err(StorageError::Host)?;

        match response {
            HostResponse::Inline { .. } | HostResponse::SabRef { .. } => Ok(()),
        }
    }

    async fn load_chunk(&self, hash: &str) -> Result<Vec<u8>, StorageError> {
        let sab = crate::get_cached_sab().ok_or(StorageError::NoSharedBuffer)?;
        let meta = serde_json::json!({
            "hash": hash,
            "method": "load",
        });
        let custom = serde_json::to_vec(&meta)?;

        match SyscallClient::host_call(
            &sab,
            "storage.load_chunk",
            HostPayload::Inline(&[]),
            Some(&custom),
        )
        .await
        .map_err(StorageError::Host)?
        {
            HostResponse::Inline { data, .. } => Ok(data),
            HostResponse::SabRef { offset, size, .. } => {
                let mut data = vec![0u8; size as usize];
                sab.read_raw(offset as usize, &mut data)
                    .map_err(StorageError::Host)?;
                Ok(data)
            }
        }
    }

    async fn delete_chunk(&self, hash: &str) -> Result<(), StorageError> {
        let sab = crate::get_cached_sab().ok_or(StorageError::NoSharedBuffer)?;
        let meta = serde_json::json!({
            "hash": hash,
            "method": "delete",
        });
        let custom = serde_json::to_vec(&meta)?;

        let response = SyscallClient::host_call(
            &sab,
            "storage.delete_chunk",
            HostPayload::Inline(&[]),
            Some(&custom),
        )
        .await
        .map_err(StorageError::Host)?;

        match response {
            HostResponse::Inline { .. } | HostResponse::SabRef { .. } => Ok(()),
        }
    }

    async fn query_index(&self, params: &StorageParams) -> Result<Vec<u8>, StorageError> {
        let sab = crate::get_cached_sab().ok_or(StorageError::NoSharedBuffer)?;
        let custom = serde_json::to_vec(params)?;

        match SyscallClient::host_call(
            &sab,
            "storage.query_index",
            HostPayload::Inline(&[]),
            Some(&custom),
        )
        .await
        .map_err(StorageError::Host)?
        {
            HostResponse::Inline { data, .. } => Ok(data),
            HostResponse::SabRef { offset, size, .. } => {
                let mut data = vec![0u8; size as usize];
                sab.read_raw(offset as usize, &mut data)
                    .map_err(StorageError::Host)?;
                Ok(data)
            }
        }
    }
}

unsafe impl Send for StorageUnit {}
unsafe impl Sync for StorageUnit {}
