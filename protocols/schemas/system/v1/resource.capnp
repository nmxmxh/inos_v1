@0xc4d5e6f7a8b9c0d1;

# Universal Mesh Resource Protocol
# Standardizes how data shards are moved, cached, and validated across the mesh.

struct Resource {
  id @0 :Text;              # Unique resource identifier
  digest @1 :Data;          # Content-addressable Hash (BLAKE3 32-byte)
  
  # Sizes
  rawSize @2 :UInt32;       # Original uncompressed size
  wireSize @3 :UInt32;      # Size after compression/encryption
  
  # Transport Layer
  compression @4 :Compression;
  enum Compression { none@0; brotli@1; snappy@2; lz4@3; }
  
  encryption @5 :Encryption;
  enum Encryption { none@0; chaCha20@1; aesGcm@2; }
  
  # Shared Memory / Storage Context
  allocation @6 :Allocation;
  struct Allocation {
    type @0 :Type;
    enum Type { 
      sab @0;             # SharedArrayBuffer (Zero-Copy)
      heap @1;            # WASM Linear Memory
      persistent @2;      # Content-Addressable Storage (CAS)
    }
    
    offset @1 :UInt32;      # Absolute offset if type == sab
    lifetime @2 :Lifetime;
    enum Lifetime { 
      ephemeral @0;       # Single job
      epoch @1;           # Current system epoch
      stable @2;          # Until explicit free
    }
  }

  # Temporal & Flow Control (The "Fluid" Layer)
  timestamp @10 :UInt64;    # Source Nanoseconds (Sensor Fusion / Frame Time)
  sequence @11 :UInt64;     # Continuous sequence for streaming flows
  priority @12 :UInt8;      # 0-255: VR/AR = 255 (Highest), Logs = 10 (Lowest)
  deadline @13 :UInt64;     # Unix Nanoseconds: drop if past this
  
  # Fragments & Progressive Streaming (Water Flowing)
  fragmentIndex @14 :UInt32;
  isLast @15 :Bool;
  
  # Data access
  union {
    inline @7 :Data;        # Small payload (<64KB)
    shards @8 :List(Data);  # List of hashes for large resources (sharding)
    sabRef @16 :SABRef;     # Direct zero-copy SAB slice
  }
  
  struct SABRef {
    offset @0 :UInt32;      # Absolute offset in SAB
    size @1 :UInt32;        # Slice size
    stride @2 :UInt16;      # Optional: spacing for interleaved data (Vertex/Boid)
  }
  
  metadata @9 :ResourceMetadata;
}

struct ResourceMetadata {
  contentType @0 :Text;     # e.g., "application/x-boids", "sensor/pose", "mesh/gltf"
  spatialContext @1 :Data;  # Compact spatial hashing / origin bits
  custom @2 :Data;          # JSON/Binary app-specific blob
}
