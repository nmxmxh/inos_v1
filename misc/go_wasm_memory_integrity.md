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

## 2. The Solution: Explicit Bridge (SAB Access via JS)

To adhere to data integrity while operating within Go's constraints, we implemented an **Explicit Bridge** in `sab_bridge.go`.

### Mechanism
Instead of relying on direct pointer arithmetic (which reads private memory), we use Go's `syscall/js` to access the global SAB object via the host JS environment.

```go
// Direct Copy from Shared SAB -> Go Private Memory
js.CopyBytesToGo(dest, view.Call("subarray", offset, offset+size))
```

### Architectural Trade-off
-   **Cons**: Reads are NOT Zero-Copy. Data is copied from the SAB to Go's heap.
-   **Pros**:
    -   **Correctness**: Guarantees Go sees the data Rust writes.
    -   **Stability**: Uses standard Go runtime features without fragile patches.
    -   **Simplicity**: Easy to reason about; explicit boundary crossing.

## 3. Optimizing the Bridge (`ReadAt`)

To align with INOS performance principles (even with the copy overhead), we implemented **Zero-Allocation Reads**.

### The `ReadAt` Pattern
We refactored the bridge to implement `ReadAt(offset, dest []byte)`. This allows the caller loop (e.g., `BoidsSupervisor`) to:
1.  Allocate a single buffer (`populationBuf`) once.
2.  Reuse this buffer every frame (100ms).
3.  Pass the buffer to the bridge.

This eliminates garbage collection pressure typically associated with "copying" data, making the bridge highly performant for high-frequency supervisor loops (60Hz+ equivalent).

```go
// Zero-Allocation Loop
for {
    // Reuses s.populationBuf
    s.bridge.ReadAt(offset, s.populationBuf)
    // Process data...
}
```

## 4. Future Alignment (Unified Memory)

Achieving "True Zero Copy" for Go (where Go's linear memory IS the SAB) remains a long-term goal. It requires:
1.  **Custom WASM Loader**: Replacing `wasm_exec.js`.
2.  **Go Runtime Patching**: Forcing Go to accept a fixed-size (or externally managed) memory buffer.
3.  **Layout Synchronization**: Ensuring Go's stack/heap doesn't overwrite system reserved areas.

Until then, the **Explicit Bridge with Zero-Allocation** provides the necessary robustness and performance for the Supervisor layer.
