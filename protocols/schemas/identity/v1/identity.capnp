@0xf58a7b3c2e1d0942;


# INOS Identity Schema v1.0
# Defines the distributed trust anchor (DID) and device binding.

struct IdentityRegistry {
  dids @0 :List(IdentityEntry);
}

struct IdentityEntry {
  did @0 :Text;
  publicKey @1 :Data;
  status @2 :IdentityStatus;
  
  # Device Graph
  devices @3 :List(DeviceEntry);
  
  # TSS Metadata
  threshold @4 :UInt8;
  totalShares @5 :UInt8;
  
  # Recovery
  guardianDids @6 :List(Text);
  recoveryThreshold @7 :UInt8;
}

struct DeviceEntry {
  deviceId @0 :Text;            # device:<blake3(fingerprint)>
  nodeId @1 :Text;              # node:<blake3(device_id)>
  name @2 :Text;                # User-friendly name
  addedAt @3 :Int64;
  lastSeenAt @4 :Int64;
  fingerprint @5 :DeviceFingerprint;
  
  # Resource Allocation Tier
  tier @6 :ResourceTier;
  
  # Detailed Profile & Capabilities
  profile @7 :ResourceProfile;
  capabilities @8 :DeviceCapability;
}

struct DeviceFingerprint {
  # High-entropy fingerprint data
  webAuthn @0 :Data;
  canvas @1 :Data;
  audio @2 :Data;
  fonts @3 :Data;
  hardware @4 :Text;            # e.g., "Apple M2"
}

enum IdentityStatus {
  active @0;
  underRecovery @1;
  revoked @2;
  systemWallet @3;
}

struct ResourceProfile {
  memoryLimitMb @0 :UInt32;     # Max SAB size in MB
  storageLimitGb @1 :UInt32;    # [DEPRECATED] Total storage in GB
  cpuCores @2 :UInt8;           # Available cores
  p2pPriority @3 :Float32;       # Weight for DHT/Gossip (0.0 - 1.0)
  
  # Granular Storage (Phase 10)
  idbLimitMb @4 :UInt32;        # Index/Metadata limit (IndexedDB)
  opfsLimitGb @5 :UInt32;       # Bulk storage limit (File System)
  p2pQuotaGb @6 :UInt32;        # Space contributed to the mesh
}

struct DeviceCapability {
  hasGpu @0 :Bool;
  hasWebGpu @1 :Bool;
  canMine @2 :Bool;
  canInference @3 :Bool;
  maxOpsPerSec @4 :UInt64;
}

enum ResourceTier {
  light @0;      # 32MB SAB, 5GB Storage (Mobile/IoT)
  moderate @1;   # 64MB SAB, 20GB Storage (Laptop)
  heavy @2;      # 128MB SAB, 100GB Storage (Workstation)
  dedicated @3;  # 256MB+ SAB, 500GB+ Storage (Dedicated Node)
}

struct IdentityAction {
  union {
    addDevice @0 :DeviceEntry;
    removeDevice @1 :Text;
    initiateRecovery @2 :Void;
    submitRecoverySignature @3 :Data;
    upgradeToFullIdentity @4 :Text; # Target DID
  }
  
  signature @5 :Data;
  timestamp @6 :Int64;
}
