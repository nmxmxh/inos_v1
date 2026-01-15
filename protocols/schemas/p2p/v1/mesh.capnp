@0xf434190284712409;

using Base = import "/base/v1/base.capnp";

interface P2PMesh {

  # =================================================================
  # 1. Chunk Discovery (The "Search")
  # =================================================================
  
  findPeersWithChunk @0 (chunkHash :Text) -> (peers :List(PeerCapability));
  findBestPeerForChunk @1 (chunkHash :Text) -> (peer :PeerCapability);
  
  registerChunk @2 (chunkHash :Text, size :UInt64, priority :ChunkPriority = medium) -> (success :Bool);
  unregisterChunk @3 (chunkHash :Text) -> (success :Bool);
  
  scheduleChunkPrefetch @4 (chunkHashes :List(Text), priority :PrefetchPriority = normal) -> (success :Bool);

  # =================================================================
  # 2. Reputation System (The "Trust")
  # =================================================================
  
  reportPeerPerformance @5 (peerId :Text, success :Bool, latencyMs :Float32, operation :Text = "chunk_fetch");
  getPeerReputation @6 (peerId :Text) -> (score :Float32, confidence :Float32);
  getTopPeers @7 (limit :UInt16 = 10) -> (peers :List(PeerCapability));

  # =================================================================
  # 3. Model Registry (The "Library")
  # =================================================================
  
  registerModel @8 (metadata :ModelMetadata) -> (success :Bool, modelId :Text);
  findModel @9 (modelId :Text) -> (metadata :ModelMetadata, availablePeers :List(Text));
  listModels @10 (query :ModelQuery) -> (models :List(ModelMetadata));

  # =================================================================
  # 4. Mesh Management (The "Connection")
  # =================================================================
  
  getMeshMetrics @11 () -> (metrics :MeshMetrics);
  connectToPeer @12 (peerId :Text, address :Text) -> (success :Bool, connectionId :Text);
  disconnectFromPeer @13 (peerId :Text) -> (success :Bool);
  allocateSharedBuffer @14 (size :UInt64) -> (bufferId :Text, offset :UInt64);
  
  # =================================================================
  # 5. Compute Delegation (The "Brain")
  # =================================================================

  delegateCompute @17 (envelope :Base.Base.Envelope) -> (response :Base.Base.Envelope);

  # =================================================================
  # 6. Reactive Events (Envelope Compatible)
  # =================================================================
  # We use a callback interface for push events to support streaming.
  
  subscribeToEvents @15 (topics :List(Text), listener :EventListener) -> (subscriptionId :Text);
  unsubscribeFromEvents @16 (subscriptionId :Text) -> (success :Bool);
}

interface EventListener {
  # Callback for receiving events wrapped in Base.Envelope
  onEvent @0 (envelope :Base.Base.Envelope) -> ();
}

# =================================================================
# Data Structures
# =================================================================

struct PeerCapability {
  peerId @0 :Text;
  # removed unused metadata field to fix ordinal gap
  
  availableChunks @1 :List(Text);
  bandwidthKbps @2 :Float32;
  latencyMs @3 :Float32;
  reputation @4 :Float32;
  capabilities @5 :List(Text); 
  
  lastSeen @6 :Int64; 
  connectionState @7 :ConnectionState;
  
  region @8 :Text;
  coordinates @9 :GeoCoordinates;
}

struct GeoCoordinates {
  latitude @0 :Float32;
  longitude @1 :Float32;
}

enum ConnectionState {
  disconnected @0;
  connecting @1;
  connected @2;
  degraded @3; 
  failed @4;
}

struct ModelMetadata {
  modelId @0 :Text;
  name @1 :Text;
  version @2 :Text;
  
  totalChunks @3 :UInt32;
  totalSize @4 :UInt64;
  chunkIds @5 :List(Text);
  
  createdAt @6 :UInt64;
  lastAccessed @7 :UInt64;
  
  # Rich Metadata
  modelType @8 :ModelType;
  format @9 :ModelFormat;
  tags @10 :List(Text);
  
  # Advanced Specs
  architecture @11 :Text; 
  parameterCount @12 :UInt64;
  quantization @13 :QuantizationLevel;
  license @14 :Text;
  author @15 :Text;
  description @16 :Text;
  
  # Resource Requirements
  estimatedInferenceTimeMs @17 :Float32;
  memoryRequiredMb @18 :UInt32;
  gpuRequired @19 :Bool;
  
  # Partitioning Support
  layerChunks @20 :List(LayerChunkMapping);
}

struct LayerChunkMapping {
  layerIndex @0 :UInt32;
  chunkIndices @1 :List(UInt32);
}

# ... Enums (ModelType, ModelFormat, QuantizationLevel) same as before ...
enum ModelType {
  llm @0;
  vision @1;
  audio @2;
  multimodal @3;
  embedding @4;
  diffusion @5;
}

enum ModelFormat {
  safetensors @0;
  gguf @1;
  onnx @2;
  pytorch @3;
  tensorflow @4;
}

enum QuantizationLevel {
  fp32 @0;
  fp16 @1;
  int8 @2;
  int4 @3;
}

struct ModelQuery {
  modelType @0 :ModelType;
  tags @1 :List(Text);
  maxSize @2 :UInt64;
  minReputation @3 :Float32;
  createdAfter @4 :UInt64;
  limit @5 :UInt16;
  offset @6 :UInt16;
  searchTerm @7 :Text;
}

struct MeshMetrics {
  totalPeers @0 :UInt32;
  connectedPeers @1 :UInt32;
  dhtEntries @2 :UInt32;
  gossipRatePerSec @3 :Float32;
  avgReputation @4 :Float32;
  bytesSent @5 :UInt64;
  bytesReceived @6 :UInt64;
  p50LatencyMs @7 :Float32;
  p95LatencyMs @8 :Float32;
  connectionSuccessRate @9 :Float32;
  chunkFetchSuccessRate @10 :Float32;
  localChunks @11 :UInt32;
  totalChunksAvailable @12 :UInt32;
}

enum ChunkPriority {
  low @0;
  medium @1;
  high @2;
  critical @3;
}

enum PrefetchPriority {
  background @0;
  normal @1;
  aggressive @2;
}

# =================================================================
# Event Payloads (To be wrapped in Base.Envelope)
# =================================================================

struct MeshEvent {
  union {
    peerUpdate @0 :PeerCapability;
    chunkDiscovered @1 :ChunkDiscovery;
    modelRegistered @2 :ModelMetadata;
    reputationChange @3 :ReputationUpdate;
  }
}

struct ChunkDiscovery {
  chunkHash @0 :Text;
  peerId @1 :Text;
  priority @2 :ChunkPriority;
}

struct ReputationUpdate {
  peerId @0 :Text;
  newScore @1 :Float32;
  reason @2 :Text;
}

struct DHTEntry {
  key @0 :Text;
  value @1 :List(Text);
  timestamp @2 :Int64;
  ttl @3 :Int64;
}
