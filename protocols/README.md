# Protocols (The Nervous System)

**Role**: The connective tissue ensuring zero-copy, low-latency communication.

## Standards
- **Transport**: **WebTransport (QUIC)**. Essential for traversing the "Void" (Internet) with low latency.
- **Serialization**: **Cap'n Proto**. Chosen for zero-copy deserialization, vital for the Robotics/Sensor feedback loop.

## Structure
- **`schemas/`**: Versioned Cap'n Proto definitions.
    - **`base/v1/`**: Core Envelopes.
    - **`economy/v1/`**: Ledger & Wallets.
    - **`io/v1/`**: Sensor definitions.

## 🧬 Metatada DNA (The Schema)

Every event in the system must traverse the nervous system wrapped in this envelope.

```protobuf
syntax = "proto3";

package inos.v2;

// The Standard Envelope for all INOS messages
message Envelope {
    // 1. Routing
    string id = 1;              // UUID
    string event_type = 2;      // "sensor:lidar:v1:stream"
    int64 timestamp = 3;        // Unix nanoseconds

    // 2. The DNA (Context)
    Metadata metadata = 4;

    // 3. The Payload (Cap'n Proto bytes)
    bytes payload = 5;
}

message Metadata {
    // Identity
    string user_id = 1;
    string device_id = 2;       // The Node ID
    
    // Context
    map<string, string> trace = 3; 
    string security_token = 4;
    
    // Economics (New in v2.1)
    string credit_ledger_id = 5; // Who pays for this computation?
}

// protocols/schemas/ledger.capnp
message Ledger {
    struct Transaction {
        id @0 :Text;
        from @1 :Text; // Wallet ID
        to @2 :Text;   // Wallet ID
        amount @3 :UInt64;
        signature @4 :Data;
    }
    
    struct Wallet {
        publicKey @0 :Data;
        balance @1 :UInt64;
    }
}
```

## 🏗 Architecture & Bindings

The nervous system binds the **Environment Machine (JS)** to the **Kernel (Go)** and **Capsules (Rust)**.

### Communication Flow (Pseudocode)

```typescript
// Frontend (JS) -> Sends an Event
const envelope = new Envelope({
    type: "compute:physics:v1:request",
    metadata: { credit_ledger_id: "wallet_123" },
    payload: capnp.serialize(physicsRequest)
});

// The Bridge routes it via WebTransport or Local Memory
Bridge.dispatch(envelope);
```

```go
// Kernel (Go) -> Receives & Routes
func (k *Kernel) OnEvent(env Envelope) {
    if env.Type == "compute:physics:v1:request" {
        // Check Credits
        if !k.Economy.HasCredits(env.Metadata.CreditLedgerId) {
            return Error("Insufficient Funds")
        }
        
        // Dispatch to a Rust Capsule
        nodeID := k.Scheduler.FindOptimalNode(Requirements{GPU: true})
        k.Transport.Send(nodeID, env)
    }
}
```

## 🧪 Test Requirements

1.  **Schema Validation**:
    *   Verify that all `proto` files compile for Go, Rust, and TS.
    *   Test that an event serialized in TS can be deserialized in Rust (and vice versa) without data loss.

2.  **Zero-Copy Verification**:
    *   Benchmark the overhead of `Envelope` wrapping. It must be `< 5ns` for local transfers.

3.  **Backwards Compatibility**:
    *   Ensure v2.0 events are accepted by v2.1 parsers (using standard Protobuf rules).
