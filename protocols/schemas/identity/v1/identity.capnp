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

enum ResourceTier {
  light @0;      # 25% (1.0x baseline)
  moderate @1;   # 50% (1.5x)
  heavy @2;      # 75% (1.75x)
  dedicated @3;  # 100% (2.0x)
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
