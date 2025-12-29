@0xa3b4c5d6e7f89012;

# ML Model Distribution Protocol
interface Model {
    struct ModelManifest {
        modelId @0 :Text;
        architecture @1 :Text;
        version @2 :Text;          # "v4.1"
        dependsOn @3 :Text;        # Base model (e.g., "llama-7b-q4")
        totalChunks @4 :UInt32;
        chunkSize @5 :UInt32;
        chunks @6 :List(Text);     # BLAKE3 hashes
        layerMapping @7 :List(LayerChunk);
        signature @8 :Data;        # Ed25519 signature
        providers @9 :List(Text);  # Peer IDs
        merkleRoot @10 :Data;      # Merkle root of all chunks
    }
    
    struct LayerChunk {
        layerId @0 :UInt32;
        chunkRange @1 :ChunkRange;
        size @2 :UInt64;
    }
    
    struct ChunkRange {
        start @0 :UInt32;
        end @1 :UInt32;
    }
    
    # Proof-of-Retrievability
    struct ChunkChallenge {
        chunkHash @0 :Text;
        challenge @1 :Data;        # Random 32-byte nonce
        timestamp @2 :Int64;
    }
    
    struct ChunkProof {
        chunkHash @0 :Text;
        proof @1 :Data;            # HMAC(chunk_data, challenge)
        timestamp @2 :Int64;
    }
    
    # Distributed Inference
    struct InferenceRequest {
        modelId @0 :Text;
        prompt @1 :Text;
        config @2 :GenerationConfig;
        partitions @3 :List(LayerPartition);
        encrypted @4 :Bool;        # Is partitions encrypted?
        coordinator @5 :Text;      # Trusted coordinator (Kernel)
    }
    
    struct LayerPartition {
        peerId @0 :Text;
        layerRange @1 :ChunkRange;
        inputOffset @2 :UInt64;    # SAB offset
        outputOffset @3 :UInt64;
    }
    
    struct GenerationConfig {
        maxTokens @0 :UInt32;
        temperature @1 :Float32;
        topP @2 :Float32;
        topK @3 :UInt32;
        stream @4 :Bool;
    }
    
    struct InferenceResult {
        tokens @0 :List(Text);
        logprobs @1 :List(Float32);
        finishReason @2 :FinishReason;
        usage @3 :UsageStats;
    }
    
    enum FinishReason {
        stopToken @0;
        maxTokens @1;
        endOfText @2;
    }
    
    struct UsageStats {
        promptTokens @0 :UInt32;
        completionTokens @1 :UInt32;
        totalTokens @2 :UInt32;
        timeMs @3 :Float32;
    }

    # ---------------------------------------------------------------------------
    # Intelligence / Brain Protocol (Zero-Copy)
    # ---------------------------------------------------------------------------

    struct BrainRequest {
        op @0 :BrainOp;
        features @1 :List(FeatureEntry);
        context @2 :Text;
    }

    enum BrainOp {
        predict @0;
        learn @1;
        correlate @2;
    }

    struct FeatureEntry {
        key @0 :Text;
        value @1 :Float32;
    }

    struct BrainResult {
        decision @0 :Text;
        confidence @1 :Float32;
        latencyMs @2 :UInt64;
        explanation @3 :List(Text);
    }
}
