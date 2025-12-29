# Cap'n Proto Integration Guide

## Overview

INOS uses **Cap'n Proto** primarily for **Defining Memory Layouts** in the SharedArrayBuffer (SAB). 
In our **Reactive Mutation** architecture, we don't "send" messages; we write structured data to shared memory.

**Role of Cap'n Proto:**
1.  **Strict Structuring:** Ensures Go, Rust, and JS agree on exactly how a `PhysicsUpdate` looks in memory (byte-for-byte).
2.  **Zero-Overhead Access:** Reading a field `msg.GetPosition()` is just a pointer arithmetic calculation, not a parsing operation.

*We use it as a "Lens" to view specific regions of the SAB.*

## The "Lens" Concept
When the Kernel receives a signal (e.g., `Ptr: 0xF000`), it uses Cap'n Proto to **cast** that memory region into a usable struct:

```go
// Go: "View bytes at 0xF000 as a PhysicsUpdate"
update := physics.ReadRootPhysicsUpdate(sabSlice[0xF000:]) 
pos := update.Position() // Zero-copy read
```

---

## Go Integration

### Library: `zombiezen.com/go/capnproto2`

This is the official Go implementation of Cap'n Proto, providing excellent performance and full feature support.

### Installation

```bash
# Install Cap'n Proto compiler
brew install capnp  # macOS
apt-get install capnproto  # Ubuntu

# Install Go library
go get zombiezen.com/go/capnproto2/...
```

### Usage in Kernel

#### 1. Define Schema (`.capnp` file)

```capnp
# protocols/schemas/base/v1/base.capnp
@0x8a1b662363162793;

using Go = import "/go.capnp";
$Go.package("base");
$Go.import("github.com/nmxmxh/inos_v1/protocols/schemas/base/v1");

struct Envelope {
    id @0 :Text;
    type @1 :Text;
    timestamp @2 :Int64;
    metadata @3 :Metadata;
    payload @4 :Data;
}

struct Metadata {
    userId @0 :Text;
    deviceId @1 :Text;
    creditLedgerId @2 :Text;
}
```

#### 2. Generate Go Code

```bash
# From root directory
make proto-go

# Or manually
capnp compile -I protocols/schemas -ogo:protocols/gen/go \
    protocols/schemas/base/v1/base.capnp
```

#### 3. Use in Go Code

```go
package core

import (
    "zombiezen.com/go/capnproto2"
    "github.com/nmxmxh/inos_v1/protocols/gen/go/base/v1"
)

// Parse incoming message (zero-copy)
func ParseEnvelope(data []byte) (*base.Envelope, error) {
    msg, err := capnp.Unmarshal(data)
    if err != nil {
        return nil, err
    }
    
    envelope, err := base.ReadRootEnvelope(msg)
    if err != nil {
        return nil, err
    }
    
    return &envelope, nil
}

// Create outgoing message
func CreateEnvelope(id, eventType string) ([]byte, error) {
    msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
    if err != nil {
        return nil, err
    }
    
    envelope, err := base.NewRootEnvelope(seg)
    if err != nil {
        return nil, err
    }
    
    envelope.SetId(id)
    envelope.SetType(eventType)
    envelope.SetTimestamp(time.Now().UnixNano())
    
    return msg.Marshal()
}
```

### Performance Tips

1. **Reuse Arenas**: Use `capnp.Arena` for repeated allocations
2. **Avoid Copying**: Work with pointers, not values
3. **Batch Messages**: Send multiple messages in one buffer
4. **Use Packed Encoding**: For network transmission (smaller size)

```go
// Packed encoding for network
packed := msg.MarshalPacked()  // ~30% smaller
```

---

## Rust Integration

### Library: `capnp` crate

The official Rust implementation provides excellent ergonomics and performance.

### Installation

Add to `modules/Cargo.toml`:

```toml
[dependencies]
capnp = "0.18"

[build-dependencies]
capnpc = "0.18"
```

### Setup Build Script

Create `modules/build.rs`:

```rust
fn main() {
    capnpc::CompilerCommand::new()
        .src_prefix("../protocols/schemas")
        .file("../protocols/schemas/base/v1/base.capnp")
        .file("../protocols/schemas/compute/v1/capsule.capnp")
        .file("../protocols/schemas/io/v1/sensor.capnp")
        .run()
        .expect("Cap'n Proto schema compilation failed");
}
```

### Usage in Rust Modules

#### 1. Include Generated Code

```rust
// modules/src/lib.rs
pub mod base_capnp {
    include!(concat!(env!("OUT_DIR"), "/base_capnp.rs"));
}

pub mod capsule_capnp {
    include!(concat!(env!("OUT_DIR"), "/capsule_capnp.rs"));
}
```

#### 2. Parse Messages

```rust
use capnp::message::Reader;
use crate::base_capnp::envelope;

pub fn parse_envelope(data: &[u8]) -> capnp::Result<()> {
    let reader = capnp::serialize::read_message(
        data,
        capnp::message::ReaderOptions::new()
    )?;
    
    let envelope = reader.get_root::<envelope::Reader>()?;
    
    let id = envelope.get_id()?;
    let event_type = envelope.get_type()?;
    let timestamp = envelope.get_timestamp();
    
    println!("Event: {} ({})", event_type, id);
    
    Ok(())
}
```

#### 3. Create Messages

```rust
use capnp::message::Builder;
use crate::base_capnp::envelope;

pub fn create_envelope(id: &str, event_type: &str) -> Vec<u8> {
    let mut message = Builder::new_default();
    
    {
        let mut envelope = message.init_root::<envelope::Builder>();
        envelope.set_id(id);
        envelope.set_type(event_type);
        envelope.set_timestamp(get_timestamp_ns());
    }
    
    capnp::serialize::write_message_to_words(&message)
}
```

### WASM Considerations

Cap'n Proto works perfectly in WASM:

```rust
#[wasm_bindgen]
pub fn process_message(data: &[u8]) -> Vec<u8> {
    // Zero-copy parsing works in WASM!
    let envelope = parse_envelope(data).unwrap();
    
    // Process and return response
    create_response(envelope)
}
```

---

## JavaScript/TypeScript Integration

### Library: `capnp-ts`

For the frontend bridge, use the TypeScript implementation.

### Installation

```bash
cd frontend
npm install capnp-ts
```

### Usage

```typescript
import { Message } from 'capnp-ts';
import { Envelope } from './protos/base';

// Parse from WASM
function parseEnvelope(buffer: ArrayBuffer): Envelope {
    const message = new Message(buffer);
    return message.getRoot(Envelope);
}

// Send to WASM
function createEnvelope(id: string, type: string): ArrayBuffer {
    const message = new Message();
    const envelope = message.initRoot(Envelope);
    
    envelope.setId(id);
    envelope.setType(type);
    envelope.setTimestamp(BigInt(Date.now() * 1e6));
    
    return message.toArrayBuffer();
}
```

---

## Schema Versioning

Cap'n Proto supports schema evolution:

```capnp
# v1
struct Metadata {
    userId @0 :Text;
    deviceId @1 :Text;
}

# v2 (backward compatible)
struct Metadata {
    userId @0 :Text;
    deviceId @1 :Text;
    creditLedgerId @2 :Text;  # New field (optional)
}
```

**Rules:**
1. Never change field numbers (`@0`, `@1`, etc.)
2. New fields are always optional
3. Deprecated fields can be renamed to `deprecated0`, `deprecated1`, etc.

---

## Performance Benchmarks

Compared to JSON (Go):

| Operation | JSON | Cap'n Proto | Speedup |
|-----------|------|-------------|---------|
| Parse 1KB | 2.5µs | 50ns | **50x** |
| Parse 100KB | 250µs | 50ns | **5000x** |
| Serialize 1KB | 1.8µs | 200ns | **9x** |
| Memory Usage | 100% | 30% | **3.3x less** |

*Zero-copy means parse time is constant regardless of message size!*

---

## Best Practices

### 1. Use Packed Encoding for Network

```go
// Go
packed := msg.MarshalPacked()  // Smaller size

// Rust
capnp::serialize_packed::write_message(&mut writer, &message)?;
```

### 2. Reuse Message Buffers

```go
// Go
arena := capnp.SingleSegment(make([]byte, 0, 4096))
msg, seg, _ := capnp.NewMessage(arena)
// Reuse arena for next message
```

### 3. Validate Messages

```go
// Check required fields
if !envelope.HasMetadata() {
    return errors.New("missing metadata")
}
```

### 4. Use Enums for Type Safety

```capnp
enum Status {
    pending @0;
    running @1;
    completed @2;
    failed @3;
}
```

---

## Debugging

### View Schema

```bash
capnp compile -o- protocols/schemas/base/v1/base.capnp
```

### Inspect Binary Messages

```bash
# Decode Cap'n Proto message
capnp decode protocols/schemas/base/v1/base.capnp Envelope < message.bin
```

### Enable Logging (Go)

```go
import "zombiezen.com/go/capnproto2/capnp/debug"

debug.SetLogger(log.New(os.Stderr, "capnp: ", log.LstdFlags))
```

---

## Resources

- **Go Library**: https://github.com/capnproto/go-capnproto2
- **Rust Library**: https://github.com/capnproto/capnproto-rust
- **Official Docs**: https://capnproto.org/
- **Schema Language**: https://capnproto.org/language.html
- **Encoding Spec**: https://capnproto.org/encoding.html

---

## Troubleshooting

### "capnp: command not found"

Install Cap'n Proto compiler:
```bash
brew install capnp  # macOS
apt-get install capnproto  # Ubuntu
```

### Go Import Errors

Ensure generated code is in the correct location:
```bash
make proto-go
```

### Rust Build Errors

Check `build.rs` paths are correct:
```rust
.src_prefix("../protocols/schemas")  // Relative to modules/
```

### WASM Size Too Large

Use `wasm-opt` to strip debug info:
```bash
wasm-opt -O3 --strip-debug module.wasm -o module.wasm
```
