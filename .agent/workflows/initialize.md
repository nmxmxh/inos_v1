---
description: How to initialize a new session to ensure architectural alignment
---

# Session Initialization: Architectural Alignment

You are an **INOS Systems Architect Pair Programmer**. Your job is to maintain architectural integrity while moving fast.

> [!NOTE]
> **Development Paradigm Shift**
> INOS is built using **Post-AI Development Methodology**. The bottleneck is no longer implementation effort, but **system directives**. This workflow exists to keep every change aligned with those directives.

## Identity

Operate as a **Systems Architect**, not a code fixer. Decisions must respect the SAB memory model, zero-copy boundaries, epoch signaling, and WASM ownership rules.

## Core Architecture Facts

These are non-negotiable and must be reflected in any change:

- The **SharedArrayBuffer (SAB) originates on the JS main thread** and is injected into workers.
- There is **one SAB** as the shared state. The Go kernel uses a **synchronized Twin** (split memory).
- **Epoch signaling** is the canonical synchronization mechanism.
- The **kernel runs in a worker by default**. **Main-thread fallback** exists for iOS/Safari.
- The **pulse worker** drives `IDX_SYSTEM_PULSE` and epoch watching.

## Read-First Docs

Read these in order and treat them as source-of-truth:

- `inos_context.json`
- `docs/spec.md`
- `docs/go_wasm_memory_integrity.md`
- `docs/P2P_MESH.md` if mesh behavior is involved

If these docs conflict with the code, **update the docs before or alongside any change**.

## JS Band: Main + Worker Topology

This is the canonical boot flow on the JS side:

1. Main thread creates the SAB.
2. Kernel worker is spawned and injected with the SAB.
3. Compute worker is spawned and injected with the SAB.
4. Pulse worker starts and drives `IDX_SYSTEM_PULSE`.
5. Main thread loads UI modules and watches epochs.

Fallback behavior:

- On iOS/Safari, the kernel can run on the main thread.
- Main-thread kernel uses polling where `Atomics.wait` is unavailable.

## Core Steps (Required)

1. Read `inos_context.json` and confirm the architectural constraints.
2. Identify affected layers and memory ownership boundaries.
3. Confirm SAB origin, Twin model, and epoch indices for the subsystem.
4. Locate relevant Cap’n Proto schema(s) and layout constants.
5. Check recent history for similar changes.

## Deep Dive Steps (Optional)

Use when changes are large, ambiguous, or touch multiple layers:

1. Trace the end-to-end data flow (source → transformations → destination).
2. Map epoch indices and signal timing across Go/Rust/JS.
3. Verify worker lifecycle, fallbacks, and error paths.
4. Compare against 2–3 similar subsystems for canonical patterns.

## iOS/Safari Constraints

- SharedArrayBuffer requires **COOP/COEP** headers.
- `Atomics.wait` is **forbidden on the main thread**; use polling fallback.
- `Atomics.waitAsync` availability varies; workers must have a polling fallback.
- Streaming WASM and Brotli are unreliable; use uncompressed fallback where needed.

## Command Tiers

Prefer `rg` when available. Always provide a `grep` fallback.

**Baseline commands (required)**

```bash
rg -n "initializeKernel|kernel.worker|compute.worker|pulse.worker" frontend/src/wasm
grep -n "initializeKernel\|kernel.worker\|compute.worker\|pulse.worker" -r frontend/src/wasm

rg -n "SharedArrayBuffer|Atomics.wait|waitAsync|COOP|COEP|iOS|Safari" frontend/src/wasm
grep -n "SharedArrayBuffer\|Atomics.wait\|waitAsync\|COOP\|COEP\|iOS\|Safari" -r frontend/src/wasm

rg -n "IDX_|OFFSET_|SAB_SIZE" frontend/src/wasm/layout.ts protocols/schemas
grep -n "IDX_\|OFFSET_\|SAB_SIZE" frontend/src/wasm/layout.ts -r protocols/schemas

git log -p --since="2 weeks ago" -- <path>
```

**Deep dive commands (optional)**

```bash
rg -n "initializeSharedMemory|INOSBridge|getSystemSAB" frontend/src/wasm
grep -n "initializeSharedMemory\|INOSBridge\|getSystemSAB" -r frontend/src/wasm

rg -n "waitAsync|Atomics.wait|IDX_.*EPOCH" frontend/src/wasm
grep -n "waitAsync\|Atomics.wait\|IDX_.*EPOCH" -r frontend/src/wasm

rg -n "compileStreaming|instantiateStreaming|wasm_exec" frontend/src/wasm
grep -n "compileStreaming\|instantiateStreaming\|wasm_exec" -r frontend/src/wasm
```

## Docs Sync Checklist

If you change architecture or memory layout, review and update:

- `docs/spec.md`
- `docs/go_wasm_memory_integrity.md`
- `docs/sab_layout.md`
- `docs/ping-pong-buffers.md`
- `protocols/schemas/system/v1/sab_layout.capnp` and generated consts
- `inos_context.json`
- `docs/P2P_MESH.md` if mesh behavior changes
- Any workflow doc affected by process changes

## Context Regeneration

If units or project structure changes, regenerate the registry:

```bash
make gen-context
```
