# **INOS v1.8: The Distributed Runtime**

> **Version:** 1.8 (Storage Economy) | **Status:** Specification | **Philosophy:** "Universal Context & Economic Storage"

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
*   **Responsibilities:**
    *   **Intelligent Threads:**
        *   **Watchers:** View the entire pipeline (from JS Input to Data Persistence).
        *   **Adjusters:** detailed management of side-effects and performance tuning.
        *   **Supervisors:** Orchestrating concurrent Rust workers.
    *   **Orchestration:** Managing Module lifecycles and Storage Policies.
    *   **Governance:** Policy enforcement using the Metadata DNA.

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

We split data into **Episodic** (logs), **Semantic** (facts), and **Content** (files).

### **5.1 Episodic Memory (ClickHouse)**
*   **Content:** Telemetry, Logs, Event History.

### **5.2 Semantic Memory (CockroachDB)**
*   **Content:** Identity, Ledger, Permissions.
*   **CAS Switchboard:** Maps `FileHash -> [NodeID_A, NodeID_B]`.

### **5.3 Distributed Object Store (The Storage Mesh)**
*   **Philosophy:** "The Content *is* the Address."
*   **Mechanism (Merkle DAGs):**
    1.  **Chunking:** Files are split into 1MB chunks by Rust Modules.
    2.  **Compression:** Chunks are **Brotli-compressed** before hashing. *Store Compressed, Stream Compressed.*
    3.  **Hashing (BLAKE3):** Unique IDs are derived from the *compressed* content for verification.
    4.  **Distribution:** Chunks are scattered based on the **Storage Policy**.

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
| **Logic** | Go (WASM) | Order |
| **Compute** | Rust (WASM) | Power & Storage |
| **Hot Storage** | Edge Nodes | Speed (CDN) |
| **Cold Storage** | Vault Nodes | Capacity (Archive) |
| **Context** | Metadata DNA | Truth |
