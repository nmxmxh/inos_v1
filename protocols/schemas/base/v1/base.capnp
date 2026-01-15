@0x8a1b662363162793;

# Standard Cap'n Proto schema for base types
# Go package will be generated in protocols/gen/go/base/v1

interface Base {
  
  # =================================================================
  # 1. The Wrapper (Envelope)
  # =================================================================
  # The standard wrapping for ALL messages in the Nervous System.
  struct Envelope {
    id @0 :Text;           # UUID
    type @1 :Text;         # "service:action:v1"
    timestamp @2 :Int64;   # Unix Nanoseconds (UTC)
    
    metadata @3 :Metadata;
    
    # Generic Payload Wrapper
    # Can carry ANY other protocol (SensorFrame, ActorCommand, etc.)
    # In Zero-Copy environments, this often points to a slice in SAB.
    payload @4 :Payload;
    
    version @5 :Text;      # Schema version (e.g., "v1.2")
  }

  # =================================================================
  # 2. The Shared Language (Common Types)
  # =================================================================
  
  # DNA of the request
  struct Metadata {
    moduleId @0 :UInt32;
    userId @1 :Text;
    deviceId @2 :Text;
    
    # Trace Context (OpenTelemetry W3C)
    traceParent @3 :Text;
    traceState @4 :Text;
    
    # Security
    authToken @5 :Text;
    
    # Economics
    creditLedgerId @6 :Text;
    
    # Metadata version for evolution
    version @7 :UInt32;
  }

  # Standard Error Type
  struct Error {
    code @0 :UInt32;      # HTTP-like status or internal code
    message @1 :Text;
    details @2 :Text;     # Stack trace or implementation specific
    temporary @3 :Bool;   # Can we retry?
    context @4 :Text;     # Structured error context (JSON)
  }
  
  # Generic Payload
  struct Payload {
     typeId @0 :Text;     # Mime-type or Schema ID
     data @1 :Data;       # The bytes
  }
}
