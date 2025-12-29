# **INOS v2.0: The Distributed Runtime**

> **Source of Truth:** This document (`spec.md`) is the definitive architectural directive for INOS v2.0. All implementations (Kernel, SDK, Modules) MUST adhere to these requirements.

> **Version:** 2.0 (Stabilized Core) | **Status:** 🚀 [PRODUCTION-READY] | **Philosophy:** "Universal Context & Economic Storage"

> [!NOTE]
> **Development Paradigm Shift**
> INOS is built using **Post-AI Development Methodology**—where the bottleneck has shifted from "implementation effort" to "system directives." This is an **Intentional Architecture** manifested through amplified human intelligence, not an accidental architecture grown organically. The complexity is managed through AI-augmented reasoning, enabling what would traditionally require large teams to be orchestrated by focused architectural vision.

---

## **1. Architecture Overview**

INOS is a hybrid distributed runtime that unifies high-performance native ingress, a polyglot WASM core, and a global, economically-incentivized storage mesh.

```text
┌────────────────────────────────────────────────────────┐
│  Layer 3: The Modules (WASM)                           │
│  [Rust Compute/Storage] [React+Vite UI]                │
├────────────────────────────────────────────────────────┤
│  Layer 2: The Kernel (WASM)                            │
│  [Go Orchestration & Currency]                         │
├────────────────────────────────────────────────────────┤
│  Layer 1: The Hybrid Host (Native)                     │
│  [Nginx + Brotli] [JS Web API Bridge]                  │
└────────────────────────────────────────────────────────┘
```

---

## **2. Layer 1: The Hybrid Host (The Body)**

*   **Ingress (Nginx + Brotli):**
    *   **Role:** High-speed network termination and compression.
    *   **Technology:** Nginx + **Brotli** (Rust-based) for maximum efficiency.
    *   **Function:** Sanitizes and routes packets to the Kernel.
*   **I/O Bridge (JavaScript/TS):**
    *   **Role:** Hardware & DOM Access.
    *   **Technology:** Modern JS runtime.
    *   **Function:** Exposes sensors (Bluetooth, Camera, GPS) and renders the UI.

---

## **3. Layer 2: The Kernel (The Brain)**

*   **Implementation:** **Go** (WASM) - *The Threaded Kernel*
*   **Role:** The Operating System Logic, Supervisor & Side-Effect Manager.
*   **Status:** ✅ **FULLY IMPLEMENTED** (Core scheduler, SAB bridge, Mesh Coordinator active)
*   **Responsibilities:**
    *   **Orchestration:** Managing Module lifecycles and Storage Policies.
    *   **Governance:** Policy enforcement using the Metadata DNA.
    *   **Mesh Coordination:** Global state synchronization via Gossip & DHT.

---

## **3.5. Layer 2.5: The Reactive Mutation Layer**

We replace traditional "Message Passing" (Queues) with **"Reactive Mutation"** (Shared State + Signals).

### **3.5.1 The Paradigm: Mutate ➔ Signal ➔ React**
In this architecture, components do not "talk" to each other; they update the shared reality and signal others to notice.

*   **Mutate:** A component (e.g., Rust) writes data directly to the SharedArrayBuffer (SAB). Usage of the `Arena` at `0x0D0000` (updated for v2.0 layout).
*   **Signal:** The component increments an Atomic Epoch Counter (e.g., `Epoch += 1`).
*   **React:** The Kernel (watching epochs via `sdk::signal`) detects `Epoch > LastSeenEpoch`, reads the new state, and acts.

> [!IMPORTANT]
> **Implemented v2.0: Epoch-Based Signaling**
> We have fully transitioned from binary flags to epoch counters. This enables:
> - **Debouncing**: Multiple mutations can be batched into a single epoch.
> - **Replay**: Historical epochs can be reconstructed for debugging.
> - **Consistency**: All watchers see the same sequence of state transitions.
> - **Atomic Wait**: Using `Atomics.wait()` for zero-CPU idling.

### **3.5.2 Zero-Copy Pipelining**
No data is ever copied between languages. We pass **Pointers**, not Payloads.

`Network (WebRTC) ➔ SAB (Inbox) ➔ Rust (Decompress) ➔ SAB (Arena) ➔ JS (Render)`

1.  **Network ➔ SAB:** Browser writes packet to `Inbox`.
2.  **SAB ➔ Rust:** Rust reads `Inbox`, decompresses to `Arena`.
3.  **Rust ➔ SAB:** Rust updates Manifest in `Metadata`. Signals Kernel.
4.  **SAB ➔ JS:** JS reads `Arena` via pointer for rendering.

---

## **4. Layer 3: The Modules (The Work)**

*   **Rust Modules:**
    *   **Role:** Heavy Lifting & Storage Logic.
    *   **Tasks:** Splitting files into chunks (CAS), Physics, GPU Kernels.
*   **JavaScript Modules (React + Vite):**
    *   **Role:** User Interface.
    *   **Tasks:** Rendering the visual dashboard.

---

## **5. The Data Backbone (Memory & Filesystem)**

We split data into **Structured** (IndexedDB), **Bulk** (OPFS), and **Distributed** (Mesh).

### **5.1 Browser-Native Storage**
*   **IndexedDB**: Structured data (identity, events, ledger, chunk index).
*   **OPFS**: Bulk storage (event logs, content chunks, model layers).
*   **P2P Mesh**: Distributed content and redundancy.

### **5.3 Distributed Object Store (The Storage Mesh)**
*   **Philosophy:** "The Content *is* the Address."
*   **Implementation:** `MeshCoordinator` (in Go) orchestrates DHT/Gossip.
*   **Mechanism (Merkle DAGs):**
    1.  **Chunking:** Files are split into 1MB chunks by Rust Modules (implementing `ChunkLoader`).
    2.  **Layered Compression (The Integrity Chain):**
        *   **Pass 1 (Ingress):** Nginx applies Brotli-Fast for network transmission optimization.
        *   **Pass 2 (Storage):** Rust modules apply Brotli-Max for storage density optimization.
        *   **Stability Anchor:** `Hash = BLAKE3(Compressed-1)` ensures global deduplication regardless of storage-level compression variations.
        *   **Result:** Network efficiency + storage efficiency + deterministic content addressing.
    3.  **Hashing (BLAKE3):** Unique IDs are derived from the *ingress-compressed* content for verification.
    4.  **Distribution:** Chunks are scattered based on the **Storage Policy**.
    5.  **Discovery:** Kademlia-based DHT for O(log n) lookup (see `kernel/core/mesh/dht.go`).

#### **5.3.1 Storage Policy & Redundancy**
*   **Replication Factor (RF):**
    *   **Default:** `RF=3` (3 distinct nodes).
    *   **Configurable:** Users can pledge credits for `RF=10+` for critical data.
    *   **Dynamic Scaling:** If a chunk is hot (viral), the mesh automatically increases its RF to meet demand.
*   **Self-Healing:** The Kernel monitors chunk availability. If a node fails, the system re-replicates missing chunks to maintain the target RF.

#### **5.3.2 Storage Tiers (The Market)**
The mesh recognizes two distinct classes of storage providers, incentivizing both speed and capacity:

1.  **Hot Tier (The Edge / CDN)**
    *   **Profile:** High Bandwidth, Low Latency, Limited Capacity.
    *   **Example:** 5G Drones, Edge Servers.
    *   **Role:** Serving active/viral content instantly.
    *   **Reward:** Earns credits for **Data Retrieval** (Bandwidth).
2.  **Cold Tier (The Vault)**
    *   **Profile:** High Latency, Massive Capacity.
    *   **Example:** Home NAS, Data Centers.
    *   **Role:** Long-term archival of high-fidelity assets (Games, Raw Video).
    *   **Reward:** Earns credits for **Data Retention** (Proof of Spacetime).

---

## **6. Communication Standards (The Nervous System)**

### **6.1 The Event Standard**
All events MUST use this format:

`{service}:{action}:v{version}:{state}`

*   **`service`**: Normalized service name (e.g., `storage`, `vision`).
*   **`action`**: Snake_case method (e.g., `fetch_chunk`, `detect_face`).
*   **`version`**: API version (e.g., `v1`).
*   **`state`**: Lifecycle position (`requested`, `processing`, `completed`).

### **6.2 The Event Envelope (With DNA)**
Metadata is a **standard field** included in every event.

```protobuf
message Envelope {
    // 1. Core Routing
    string id = 1;
    string event_type = 2; // "{service}:{action}..."
    int64 timestamp = 3;

    // 2. The DNA (Context)
    Metadata metadata = 4;

    // 3. The Payload
    bytes payload = 5;
}

message Metadata {
    // Identity
    string user_id = 1;
    string device_id = 2;
    
    // Context
    map<string, string> trace_context = 3; 
    string security_token = 4;
}
```

---

## **7. Summary**

| Layer | Config | Role |
| :--- | :--- | :--- |
| **Ingress** | Nginx + Brotli | Speed |
| **UI** | React + Vite | Interaction |
| **Kernel** | Go (WASM) | Logic & Orchestration |
| **Compute** | Rust (WASM) | Heavy Lifting |
| **Storage** | Rust (WASM) | Persistence & Integrity |
| **Metadata** | Metadata DNA | Global Context |
