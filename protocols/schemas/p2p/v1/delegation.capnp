@0xd8a9b7c6e5d4c3b2;

using Base = import "/base/v1/base.capnp";
using Resource = import "/system/v1/resource.capnp".Resource;

# INOS Mesh Delegation Protocol
# Standardizes how compute tasks are offloaded, verified, and settled.

struct DelegateRequest {
  id @0 :Text;                   # Unique request identifier
  resource @1 :Resource;        # Input resource (referenced by digest)
  operation @2 :Operation;      # What to do
  params @3 :Data;              # Operation-specific JSON parameters
  deadline @4 :UInt64;          # Unix Nanoseconds
  bid @5 :UInt32;               # Max credits offered for this job
  priority @6 :UInt8;           # 0-255
  
  struct Operation {
    union {
      hash @0 :HashParams;
      compress @1 :CompressParams;
      encrypt @2 :EncryptParams;
      custom @3 :Text;          # Custom method identifier
    }
  }
}

struct HashParams {
  algorithm @0 :Algorithm;
  enum Algorithm { blake3@0; sha256@1; }
}

struct CompressParams {
  algorithm @0 :Algorithm;
  enum Algorithm { brotli@0; snappy@1; lz4@2; }
  quality @1 :UInt8;            # 1-11 for Brotli
}

struct EncryptParams {
  algorithm @0 :Algorithm;
  enum Algorithm { chaCha20Poly1305@0; aes256Gcm@1; }
  keyId @1 :Text;               # Identifier for the key in keystore
}

struct DelegateResponse {
  requestId @0 :Text;           # Echo ID
  status @1 :Status;
  result @2 :Resource;          # Output resource
  metrics @3 :ExecutionMetrics;
  error @4 :Text;
  
  enum Status {
    success @0;
    inputMissing @1;            # Peer needs the input chunk
    capacityExceeded @2;        # Peer is too busy
    failed @3;
    timeout @4;
    verificationFailed @5;      # Internal self-check failed
  }
}

struct ExecutionMetrics {
  executionTimeNs @0 :UInt64;
  cpuCycles @1 :UInt64;
  peakMemoryBytes @2 :UInt32;
  energyMicroJoules @3 :UInt64; # For battery-aware delegation
}

# --- Capability Advertisement Extensions ---

struct CapabilityAdvertisement {
  id @0 :Text;                  # e.g., "compress.brotli.q6"
  score @1 :Float32;            # Normalized performance (0-1)
  cost @2 :UInt32;              # Microcredits per MB
  constraints @3 :Data;         # JSON metadata for limits
  hardware @4 :List(HardwareFeature);
}

enum HardwareFeature {
  avx512 @0;
  gpu @1;
  fpga @2;
  tpm @3;
  sgx @4;
  neon @5;
}
