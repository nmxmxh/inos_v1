@0xf1a2b3c4d5e6f789;

using Base = import "/base/v1/base.capnp";

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
}
