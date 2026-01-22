@0xf1a2b3c4d5e6f789;

using Base = import "/base/v1/base.capnp";
using Runtime = import "/system/v1/runtime.capnp";

# P2P Gossip Protocol for state synchronization
# Wire Format: Base.Envelope
# Payload: GossipPayload (serialized as Data in Envelope.payload)

interface Gossip {
    # The payload structure that goes inside Base.Envelope.payload.data
    struct GossipPayload {
        metadata @4 :Base.Base.Metadata;
        union {
            ledgerSync @0 :LedgerSync;
            peerList @1 :PeerList;
            chunkAd @2 :ChunkAdvertisement;
            modelAd @3 :ModelAdvertisement;
            custom @5 :Data;
            sdpNotify @6 :SDPNotify;      # WebRTC SDP ready notification
            sdpRelay @7 :SDPRelay;        # WebRTC SDP relay message
            iceRelay @8 :ICERelay;        # ICE candidate relay
        }
    }
    
    struct LedgerSync {
        peerId @0 :Text;
        merkleRoot @1 :Data;   # 32 bytes (BLAKE3)
        signature @2 :Data;
        logSize @3 :UInt64;
    }
    
    struct PeerList {
        peers @0 :List(PeerInfo);
    }
    
    struct PeerInfo {
        id @0 :Text;
        address @1 :Text;      # WebRTC address
        capabilities @2 :List(Text);
        reputation @3 :Float32;
        lastSeen @4 :Int64;
        
        # Adaptive Mesh
        role @5 :Runtime.Runtime.RuntimeRole;
        runtimeCaps @6 :Runtime.Runtime.RuntimeCapabilities;
    }
    
    struct ChunkAdvertisement {
        hash @0 :Text;
        size @1 :UInt64;
        contentType @2 :Text;
    }
    
    struct ModelAdvertisement {
        modelId @0 :Text;
        chunks @1 :List(Text); # Available chunks
        layers @2 :List(UInt32); # Cached layers
    }
    
    # ========== SDP Relay Types for Decentralized WebRTC Signaling ==========
    
    # Lightweight notification that SDP is available in DHT
    struct SDPNotify {
        originatorId @0 :Text;    # Who created the offer
        targetId @1 :Text;        # Who should receive it
        sessionId @2 :Text;       # Unique session identifier
        timestamp @3 :Int64;      # For deduplication and TTL
        nonce @4 :Data;           # Replay prevention (8 bytes)
    }
    
    # Full SDP relay (for direct peer forwarding when DHT unavailable)
    struct SDPRelay {
        originatorId @0 :Text;
        targetId @1 :Text;
        sessionId @2 :Text;
        sdp @3 :Data;             # Encrypted SDP payload
        hopCount @4 :UInt8;
        maxHops @5 :UInt8;
        timestamp @6 :Int64;
        signature @7 :Data;       # Ed25519 signature from originator
    }
    
    # ICE candidate relay
    struct ICERelay {
        originatorId @0 :Text;
        targetId @1 :Text;
        sessionId @2 :Text;
        candidate @3 :Text;       # ICE candidate string
        sdpMLineIndex @4 :UInt16;
        timestamp @5 :Int64;
    }
}

