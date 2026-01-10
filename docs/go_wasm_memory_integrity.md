# INOS Architecture: Go WASM Memory Integrity via Explicit Bridging

## 1. The Architectural Challenge: Split Memory Model

In the INOS v1 architecture, we use a hybrid approach combining a **Go Kernel** (Supervisor/Orchestration) and **Rust Modules** (Compute/Physics).

### The Principles
-   **Zero-Copy**: Data should ideally reside in a single `SharedArrayBuffer` (SAB) accessed by all units.
-   **Unified Memory**: All WASM instances should share the SAB as their linear memory.

### The Reality (Go WASM Constraints)
The standard Go WASM runtime (`wasm_exec.js` + `GOOS=js GOARCH=wasm`) assumes it owns its linear memory. When the Go WASM module is instantiated:
1.  The Go runtime typically requests a new, growable `WebAssembly.Memory`.
2.  It does **not** natively support importing an existing `SharedArrayBuffer` as its primary linear memory without significant hacking (patching `wasm_exec.js` and ensuring the SAB meets Go's exact layout requirements).

This creates a **Split Memory Architecture**:
-   **Rust Modules**: Instantiated with the shared SAB. Writes to `0x1000050` go directly to the shared memory.
-   **Go Kernel**: Instantiated with its own private linear memory. Reads from `0x1000050` access its private memory (which is empty/zero).

This was the root cause of the "Timeout waiting for bird count" issue: Go was reading from the wrong memory space.

## 2. The Solution: Synchronized Memory Twin Architecture

To adhere to data integrity while operating within Go's constraints, we implemented a **Synchronized Memory Twin** pattern via an **Explicit Bridge**.

### Mechanism
The Kernel maintains a **Local Replica** (Twin) of the shared state. This replica is explicitly synchronized with the Global SAB via the bridge.

```go
// Synchronization: Global SAB -> Local Twin
js.CopyBytesToGo(localTwin, view.Call("subarray", offset, offset+size))
```

### Architectural Trade-off & Benefits
While this deviates from strict "Zero-Copy" for the Supervisor, it introduces a critical stability feature: **Snapshot Isolation**.

-   **Snapshot Consistency**: The Supervisor operates on a stable snapshot of the state. It is immune to "tearing reads" or mid-calculation updates from high-frequency Rust compute threads (running at 60Hz+).
-   **Isolation**: The Kernel's decision logic is protected from memory corruption in the hot shared path.
-   **Explicit Synchronization**: The "Cost" of copying is the "Price" of consistency. By batching reads (via `ReadAt`), we pay this price efficiently.

## 3. Optimizing the Twin (`ReadAt`)

To align with INOS performance principles, we implemented **Zero-Allocation Synchronization**.

### The `ReadAt` Pattern: Ephemeral Fixed Buffer
We refactored the bridge to implement `ReadAt(offset, dest []byte)`. This allows the caller loop to recycle the "Twin Buffer":
1.  **Allocate**: Create the **Fixed Buffer** (`populationBuf`) once.
2.  **Sync**: Copy the global SAB state into this buffer (Bulk Copy).
3.  **Process**: Analyze the **Ephemeral Snapshot**.
4.  **Repeat**: Overwrite the buffer in the next Epoch.

This creates an **Ephemeral Fixed Buffer Twin**. The data in Go is valid *only* for the duration of the current processing cycle, ensuring the Kernel always acts on the latest "Scene" without allocating new memory.

```go
// Zero-Allocation Ephemeral Sync
for {
    // 1. Wait for Signal (Zero Latency)
    <-s.bridge.WaitForEpochAsync(idx, current)

    // 2. Sync Ephemeral Twin (Bulk Copy)
    s.bridge.ReadAt(offset, s.populationBuf)
    
    // 3. Process Stable Snapshot
    // ...
}
```

This effectively creates a **Double-Buffered State System**:
-   **Front Buffer (SAB)**: Hot, mutated by Rust/JS.
-   **Back Buffer (Go Twin)**: Ephemeral, stable snapshot for Kernel logic.

