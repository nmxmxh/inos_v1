pub mod cache;
pub mod chunks;
pub mod config;
pub mod distributed;
pub mod error;
pub mod registry;
pub mod verification;

pub use cache::SmartCache;
pub use chunks::{Chunk, ChunkInfo, ChunkLoader, StorageChunkLoader};
pub use config::P2pConfig;
pub use distributed::{DistributedInference, PeerCapability, SimpleDistributedInference};
pub use error::{ErrorContext, P2pError, Result, ResultExt};
pub use registry::{InMemoryModelRegistry, ModelMetadata, ModelRegistry};
pub use verification::PorVerifier;
