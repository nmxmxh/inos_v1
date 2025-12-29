use bytes::Bytes;
use capnp::{message, serialize_packed};
use serde::{Deserialize, Serialize};
use std::collections::HashSet;
use std::sync::{Arc, Mutex};
use tokio::sync::broadcast;

use sdk::sab::SafeSAB;
use sdk::signal::{Epoch, IDX_STORAGE_EPOCH};

use crate::ml::adaptive_allocator::AdaptiveAllocator;
use crate::mosaic::dispatch::VoxelID;
use crate::science_capnp::science::mosaic_message;
use sdk::syscalls::SyscallClient; // Added

// ----------------------------------------------------------------------------
// P2P TYPES & PROTOCOL (Delegated to Kernel)
// ----------------------------------------------------------------------------

// PeerID is just a wrapper around 32 bytes
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct PeerID(pub [u8; 32]);

impl PeerID {
    pub fn new(id: &[u8]) -> Self {
        let mut fixed = [0u8; 32];
        if id.len() >= 32 {
            fixed.copy_from_slice(&id[0..32]);
        }
        PeerID(fixed)
    }
}

// ChunkID mapping to/from bytes
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct ChunkID(pub [u8; 32]);

/// Message types derived from science.capnp
#[derive(Debug, Clone)]
pub enum BridgeMessage {
    AllocationQuery {
        voxel_range: VoxelRange,
        strategy: String,
    },
    AllocationResponse {
        recommended_peers: Vec<PeerID>,
        confidence: f32,
    },
    ChunkResponse {
        chunk_id: ChunkID,
        data: Bytes,
    },
    // We explicitly DO NOT handle P2P routing here anymore.
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct VoxelRange {
    pub min: VoxelID,
    pub max: VoxelID,
}

// ----------------------------------------------------------------------------
// P2P BRIDGE IMPLEMENTATION
// ----------------------------------------------------------------------------

pub struct P2PBridge {
    pub local_peer: Arc<PeerNode>,
    // Removed peers, chunk_index, routing_table
    pub allocator: Arc<dyn AdaptiveAllocator + Send + Sync>,
    pub config: BridgeConfig,

    // SAB Signaling
    pub epoch: Mutex<Epoch>,
    pub sab: Arc<SafeSAB>,

    // Internal Routing
    pub message_tx: broadcast::Sender<BridgeMessage>,
}

unsafe impl Send for P2PBridge {}
unsafe impl Sync for P2PBridge {}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChunkMetadata {
    pub chunk_id: ChunkID,
    pub size: u64,
    pub version: u64,
    pub holders: HashSet<PeerID>,
    pub last_accessed: u64,
    pub access_count: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BridgeConfig {
    pub chunk_size: usize,
    pub redundancy_factor: u8,
    pub ml_integration: bool,
}

impl P2PBridge {
    pub fn new(
        local_peer_id: &[u8], // Just ID, Kernel handles the rest
        allocator: Arc<dyn AdaptiveAllocator + Send + Sync>,
        config: BridgeConfig,
        sab: Arc<SafeSAB>,
    ) -> Result<Arc<Self>, String> {
        let (message_tx, _) = broadcast::channel(1024);

        let bridge = Arc::new(Self {
            local_peer: Arc::new(PeerNode {
                id: PeerID::new(local_peer_id),
            }), // Simplified
            allocator,
            config,
            epoch: Mutex::new(Epoch::new(sab.inner(), IDX_STORAGE_EPOCH)),
            sab,
            message_tx,
        });

        Ok(bridge)
    }

    /// Poll for new messages from the Go Kernel via SAB
    pub fn poll(&self) -> Result<Vec<BridgeMessage>, String> {
        let mut epoch = self.epoch.lock().unwrap();
        if !epoch.has_changed() {
            return Ok(Vec::new());
        }

        // Logic to read from SAB 0x010000 (Inbox)
        let data = self.sab.read(0x010000, 0x040000)?; // 256KB Inbox

        // Deserialize using Cap'n Proto
        let reader = serialize_packed::read_message(&mut &data[..], message::ReaderOptions::new())
            .map_err(|e| format!("Capnp decode failed: {}", e))?;

        let root = reader
            .get_root::<mosaic_message::Reader>()
            .map_err(|e| format!("Get root failed: {}", e))?;

        let mut messages = Vec::new();

        match root.which().map_err(|_| "Unknown variant".to_string())? {
            mosaic_message::AllocationQuery(query) => {
                let query = query.map_err(|e| e.to_string())?;
                // Handle Query - simplify for now
                let strategy = query
                    .get_strategy()
                    .map_err(|e| e.to_string())?
                    .to_str()
                    .map_err(|e| e.to_string())?
                    .to_string();
                // Extract voxel range...
                messages.push(BridgeMessage::AllocationQuery {
                    voxel_range: VoxelRange {
                        min: VoxelID([0, 0, 0]),
                        max: VoxelID([0, 0, 0]),
                    }, // stub
                    strategy,
                });
            }
            mosaic_message::AllocationResponse(resp) => {
                let resp = resp.map_err(|e| e.to_string())?;
                messages.push(BridgeMessage::AllocationResponse {
                    recommended_peers: vec![],
                    confidence: resp.get_confidence(),
                });
            }
            mosaic_message::ChunkResponse(chunk) => {
                let chunk = chunk.map_err(|e| e.to_string())?;
                messages.push(BridgeMessage::ChunkResponse {
                    chunk_id: ChunkID(
                        chunk
                            .get_id()
                            .map_err(|e| e.to_string())?
                            .try_into()
                            .map_err(|_| "Bad ID")?,
                    ),
                    data: Bytes::copy_from_slice(chunk.get_data().map_err(|e| e.to_string())?),
                });
            }
        }

        Ok(messages)
    }

    /// Send a message to the Go Kernel via SAB Outbox
    pub async fn send_to_peer(
        &self,
        target_peer_id: &str,
        msg: BridgeMessage,
    ) -> Result<(), String> {
        let mut message = message::Builder::new_default();
        {
            let mosaic = message.init_root::<mosaic_message::Builder>();
            match msg {
                BridgeMessage::AllocationQuery {
                    voxel_range,
                    strategy,
                } => {
                    let mut query = mosaic.init_allocation_query();
                    let mut range = query.reborrow().init_voxel_range();
                    let mut min = range.reborrow().init_min(3);
                    min.set(0, voxel_range.min.0[0]);
                    min.set(1, voxel_range.min.0[1]);
                    min.set(2, voxel_range.min.0[2]);
                    let mut max = range.init_max(3);
                    max.set(0, voxel_range.max.0[0]);
                    max.set(1, voxel_range.max.0[1]);
                    max.set(2, voxel_range.max.0[2]);
                    query.set_strategy(&strategy);
                }
                BridgeMessage::AllocationResponse {
                    recommended_peers,
                    confidence,
                } => {
                    let mut resp = mosaic.init_allocation_response();
                    let mut peers = resp
                        .reborrow()
                        .init_recommended_peers(recommended_peers.len() as u32);
                    for (i, peer) in recommended_peers.iter().enumerate() {
                        peers.set(i as u32, &peer.0);
                    }
                    resp.set_confidence(confidence);
                }
                BridgeMessage::ChunkResponse { chunk_id, data } => {
                    let mut resp = mosaic.init_chunk_response();
                    resp.set_id(&chunk_id.0);
                    resp.set_data(&data[..]);
                    // voxel_range, version, etc. stubs for now
                }
            }
        }

        let mut payload = Vec::new();
        serialize_packed::write_message(&mut payload, &message).map_err(|e| e.to_string())?;

        SyscallClient::send_message(&self.sab, target_peer_id, &payload).await?;

        Ok(())
    }

    // Helper for init (temporary)
    pub async fn request_chunk(&self, chunk_id: ChunkID, _priority: u8) -> Result<Vec<u8>, String> {
        // Convert ChunkID to hex string for the syscall
        let hash = hex::encode(chunk_id.0);

        // We need an offset in the Arena to store the fetched data.
        // In a real implementation, we would allocate from self.allocator or similar.
        // For this v1 compatibility, we'll use a reserved scratch space at 0x150000 (Start of Arena)
        // or just pass 0 if the Kernel handles allocation (it doesn't, it handles transfer).
        // Let's assume we read into 0x150000.
        let dest_offset = 0x150000;
        let dest_size = self.config.chunk_size as u32;

        let _response_bytes =
            SyscallClient::fetch_chunk(&self.sab, &hash, dest_offset, dest_size).await?;

        // Parse the response to verify success?
        // SyscallClient::fetch_chunk already returns the raw response bytes from the inbox.
        // Ideally we check the status in the response, but SyscallClient logic might have already checked generic status.
        // If successful, the data is now at dest_offset.

        // We read the data from SAB to return it (Rust-side copy for now)
        let data = self.sab.read(dest_offset as usize, dest_size as usize)?;

        Ok(data)
    }

    pub async fn request_execution(
        &self,
        id: String,
        library: String,
        method: String,
        params: String,
        _scale: crate::types::SimulationScale,
    ) -> Result<(), String> {
        // Map execution request to store_chunk (storing a Job Description)
        // This is a "Poor Man's RPC" via Storage.
        let job = serde_json::json!({
            "id": id,
            "library": library,
            "method": method,
            "params": params
        });
        let job_data = serde_json::to_vec(&job).map_err(|e| e.to_string())?;

        // Hash it to get an ID
        let hash = blake3::hash(&job_data).to_hex().to_string();

        // Store it (Source offset?)
        // Write to scratch space first
        let src_offset = 0x150000;
        self.sab.write(src_offset, &job_data)?;

        // Syscall: Store
        let _replicas =
            SyscallClient::store_chunk(&self.sab, &hash, src_offset as u64, job_data.len() as u32)
                .await?;

        log::info!("Job {} stored as chunk {}", id, hash);
        Ok(())
    }
}

// Temporary PeerNode struct until we fully remove usage in lib.rs
#[derive(Debug, Clone)]
pub struct PeerNode {
    pub id: PeerID,
}
