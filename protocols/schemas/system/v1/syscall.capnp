@0xde4f1a7b7c4a2b19;

using Base = import "/base/v1/base.capnp";
using Resource = import "/system/v1/resource.capnp";

# INOS Kernel Syscall Interface
# Defines the ABI for Modules to request Kernel services via SAB.

interface Syscall {

  # =================================================================
  # The Syscall Envelope (Placed in SAB Outbox)
  # =================================================================
  
  struct Message {
    header @0 :Header;
    body @1 :Body;
  }

  struct Header {
    magic @0 :UInt32;           # 0x53424142 ("SABS")
    version @1 :UInt8;
    flags @2 :UInt8;
    
    # Routing
    sourceModuleId @3 :UInt32;
    callId @4 :UInt64;          # Monotonic request ID
    
    # Operation Code (Redundant with Body union, but good for fast peek)
    opcode @5 :Opcode;
    
    metadata @6 :Base.Base.Metadata;
  }
  
  struct Body {
    union {
      noop @0 :Void;
      fetchChunk @1 :FetchChunkRequest;
      storeChunk @2 :StoreChunkRequest;
      spawnThread @3 :Void; # Placeholder
      killThread @4 :Void;  # Placeholder
      sendMessage @5 :SendMessageRequest;
      hostCall @6 :HostCallRequest;
    }
  }

  enum Opcode {
    noop @0;
    fetchChunk @1;
    storeChunk @2;
    spawnThread @3;
    killThread @4;
    sendMessage @5;
    hostCall @6;
  }

  # =================================================================
  # Request Payloads
  # =================================================================

  struct FetchChunkRequest {
    hash @0 :Data;              # Content Address
    priority @1 :UInt8;         # 0-255
    
    # Zero-Copy Destination
    destinationOffset @2 :UInt64; 
    destinationSize @3 :UInt32;
  }

  struct StoreChunkRequest {
    hash @0 :Data;
    sourceOffset @1 :UInt64;
    size @2 :UInt32;
    ttl @3 :UInt32;
  }

  struct SendMessageRequest {
    targetId @0 :Text;
    payload @1 :Data;
    priority @2 :UInt8;
  }

  struct HostCallRequest {
    service @0 :Text;           # e.g. "storage.put", "api.request"
    payload @1 :Resource.Resource;
  }

  # =================================================================
  # Response (Placed in SAB Inbox)
  # =================================================================

  struct Response {
    callId @0 :UInt64;
    status @1 :Status;
    
    # Result Union
    result @2 :Result;
    
    # Extended Error Info
    error @3 :Base.Base.Error;
  }

  enum Status {
    success @0;
    pending @1;
    invalidRequest @2;
    timeout @3;
    internalError @4;
  }

  struct Result {
    union {
      fetchChunk @0 :FetchChunkResult;
      storeChunk @1 :StoreChunkResult;
      sendMessage @2 :SendMessageResult;
      generic @3 :Void;
      hostCall @4 :HostCallResult;
    }
  }

  struct SendMessageResult {
    delivered @0 :Bool;
    latencyMs @1 :UInt16;
  }

  struct FetchChunkResult {
    bytesTransferred @0 :UInt32;
    fromPeerId @1 :UInt64;
    hashVerified @2 :Bool;
    timeToFirstByteNs @3 :UInt32;
  }

  struct StoreChunkResult {
    replicas @0 :UInt16;
  }

  struct HostCallResult {
    payload @0 :Resource.Resource;
  }
}
