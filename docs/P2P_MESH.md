# Overview

INOS implements an **adaptive, mesh-native P2P architecture** where every node acts as both client and server. The steady state is decentralized: signaling, resource discovery, and state synchronization happen over resilient gossip and DHT primitives.

Bootstrap still requires a first-contact path (for example, a hosted seed domain or known peer list). This is a distributed-systems constraint, not central control.

Replica counts scale dynamically from **5-7 replicas** for standard objects to **500-700 replicas** for massive, high-demand datasets. This "serverless" architecture ensures the content of the mesh is the only consistent truth.

## Core Principle: Adaptive Scaling

**Not all resources are equal.** The mesh adapts to:

- **Resource Size** - 1KB file vs 1GB file
- **Demand Level** - 1 request/hour vs 1000 requests/second
- **Credit Budget** - Free tier vs premium allocation
- **Network Topology** - Local cluster vs global distribution

---

## Scaling Strategy

### Small Resources (< 1MB)

```text
Size: < 1MB
Replicas: 5-7 nodes
Strategy: Full replication
Use Case: State, metadata, small files
```

### Medium Resources (1MB - 100MB)

```text
Size: 1MB - 100MB
Replicas: 10-50 nodes
Strategy: Chunked replication (1MB chunks)
Use Case: Images, documents, code repositories
```

### Large Resources (100MB - 10GB)

```text
Size: 100MB - 10GB
Replicas: 50-500 nodes
Strategy: Distributed chunking + erasure coding
Use Case: Videos, datasets, databases
```

### Massive Resources (> 10GB)

```text
Size: > 10GB
Replicas: 500-700+ nodes
Strategy: Hierarchical chunking + CDN-style distribution
Use Case: ML models, video archives, scientific data
```

---

## Chunking Strategy (Rust Implementation)

### 1MB Chunk Size (Optimal for P2P)

```rust
// Rust handles chunking for all large resources
pub struct ChunkManager {
    chunk_size: usize,  // 1MB default
}

impl ChunkManager {
    const DEFAULT_CHUNK_SIZE: usize = 1024 * 1024;  // 1MB
    
    pub fn chunk_resource(&self, data: &[u8]) -> Vec<Chunk> {
        let mut chunks = Vec::new();
        let total_size = data.len();
        let num_chunks = (total_size + self.chunk_size - 1) / self.chunk_size;
        
        for i in 0..num_chunks {
            let start = i * self.chunk_size;
            let end = std::cmp::min(start + self.chunk_size, total_size);
            
            let chunk = Chunk {
                id: i,
                hash: self.hash_chunk(&data[start..end]),
                data: data[start..end].to_vec(),
                compressed: self.compress(&data[start..end]),  // LZ4
            };
            
            chunks.push(chunk);
        }
        
        chunks
    }
    
    // Erasure coding for fault tolerance (Reed-Solomon)
    pub fn add_parity_chunks(&self, chunks: &[Chunk]) -> Vec<Chunk> {
        // Add 20% parity chunks (e.g., 100 data chunks + 20 parity)
        let parity_count = (chunks.len() as f32 * 0.2) as usize;
        reed_solomon::encode(chunks, parity_count)
    }
}
```

**Why 1MB chunks?**

- ✅ Small enough for fast P2P transfer (< 1 second on 10Mbps)
- ✅ Large enough to minimize overhead
- ✅ Matches typical browser chunk size for streaming
- ✅ Optimal for compression (LZ4 works best on 1MB blocks)

---

## Adaptive Replica Allocation

### Go Kernel - Smart Allocator

```go
type AdaptiveAllocator struct {
    minReplicas int     // 5 (minimum for quorum)
    maxReplicas int     // 700 (maximum for massive resources)
    targetLoad  float64 // 0.375 (sweet spot: 37.5% = midpoint of 25-50%)
    maxLoad     float64 // 0.50 (never exceed 50%)
}

func (aa *AdaptiveAllocator) CalculateReplicas(r Resource) int {
    // 1. Base replicas from size
    sizeReplicas := aa.replicasFromSize(r.Size)
    
    // 2. Adjust for demand
    demandMultiplier := 1.0 + r.DemandScore  // 1.0-2.0
    
    // 3. Adjust for budget
    budgetMultiplier := aa.budgetMultiplier(r.CreditBudget)
    
    // 4. Calculate ideal replica count
    idealReplicas := int(float64(sizeReplicas) * demandMultiplier * budgetMultiplier)
    
    // 5. Adjust for available node capacity
    availableReplicas := aa.calculateAvailableCapacity(r, idealReplicas)
    
    // 6. Clamp to min/max
    return clamp(availableReplicas, aa.minReplicas, aa.maxReplicas)
}

// Calculate how many replicas we can actually allocate without overloading nodes
func (aa *AdaptiveAllocator) calculateAvailableCapacity(r Resource, idealReplicas int) int {
    availableNodes := aa.getNodesWithCapacity(r)
    
    // If we don't have enough nodes, scale down gracefully
    if len(availableNodes) < aa.minReplicas {
        // Sparse network: Accept higher load temporarily
        return min(len(availableNodes), idealReplicas)
    }
    
    // Calculate how many nodes we can use without exceeding maxLoad
    usableNodes := 0
    for _, node := range availableNodes {
        if node.Load < aa.maxLoad {
            usableNodes++
        }
    }
    
    // Return the lesser of ideal replicas or usable nodes
    return min(idealReplicas, usableNodes)
}

// Get nodes that have capacity for this resource
func (aa *AdaptiveAllocator) getNodesWithCapacity(r Resource) []Node {
    nodes := aa.getAllHealthyNodes()
    
    // Filter by capability (GPU, storage, etc.)
    capable := filterByCapability(nodes, r.Requirements)
    
    // Sort by available capacity (prefer nodes with most headroom)
    sort.Slice(capable, func(i, j int) bool {
        return capable[i].Load < capable[j].Load
    })
    
    return capable
}

func (aa *AdaptiveAllocator) replicasFromSize(size uint64) int {
    switch {
    case size < 1*MB:
        return 5  // Small: 5-7 replicas
    case size < 10*MB:
        return 10  // Medium-small: 10-20 replicas
    case size < 100*MB:
        return 30  // Medium: 30-50 replicas
    case size < 1*GB:
        return 100  // Large: 100-200 replicas
    case size < 10*GB:
        return 300  // Very large: 300-500 replicas
    default:
        return 500  // Massive: 500-700 replicas
    }
}

func (aa *AdaptiveAllocator) budgetMultiplier(budget uint64) float64 {
    // More credits = more replicas = better performance
    switch {
    case budget < 100:
        return 0.5  // Free tier: 50% of base replicas
    case budget < 1000:
        return 1.0  // Standard: 100% of base replicas
    case budget < 10000:
        return 1.3  // Premium: 130% of base replicas
    default:
        return 1.5  // Enterprise: 150% of base replicas
    }
}
```

**Key Features:**

- **Conservative Load Targeting**: Never exceed 50% load per node
- **Sweet Spot**: Target 25-50% load (37.5% average)
- **Sparse Network Handling**: Gracefully degrade when nodes are scarce
- **Capacity-Aware**: Prefer nodes with most available headroom
- **Adaptive Scaling**: Reduce replicas if network can't support ideal count

**Load Distribution Example:**

```text
Scenario: 1GB resource, ideal 100 replicas
Available nodes: 80 healthy nodes
Average load: 30%

Calculation:
- Nodes with < 50% load: 80 nodes
- Allocate: min(100, 80) = 80 replicas
- Expected load per node: 1GB / 80 = 12.5MB
- New average load: 30% + (12.5MB impact) ≈ 35-40%
- ✅ Within 25-50% sweet spot
```

---

---

## Hierarchical Distribution

### For Massive Resources (> 10GB)

```go
type HierarchicalMesh struct {
    // Layer 1: Seed nodes (authoritative)
    seeds []NodeID  // 5-7 nodes with full resource
    
    // Layer 2: Regional hubs (chunked)
    hubs map[Region][]NodeID  // 50-100 nodes per region
    
    // Layer 3: Edge nodes (cached chunks)
    edges map[Region][]NodeID  // 500-700 nodes total
}

func (hm *HierarchicalMesh) AllocateMassiveResource(r Resource) {
    // 1. Chunk resource into 1MB pieces
    chunks := chunkResource(r, 1*MB)
    
    // 2. Distribute chunks to seeds (full replication)
    for _, seed := range hm.seeds {
        seed.StoreAllChunks(chunks)
    }
    
    // 3. Distribute chunks to regional hubs (partial replication)
    for region, hubs := range hm.hubs {
        // Each hub stores 20% of chunks + parity
        chunkSubset := selectChunksForRegion(chunks, region)
        for _, hub := range hubs {
            hub.StoreChunks(chunkSubset)
        }
    }
    
    // 4. Edge nodes cache on-demand
    // (chunks are fetched from nearest hub when requested)
}
```

---

## Smart Work Allocation

### Load Balancer with Chunk Awareness

```go
type ChunkAwareLoadBalancer struct {
    chunkLocations map[ChunkID][]NodeID  // Which nodes have which chunks
    nodeLoad       map[NodeID]float64     // Current load per node
}

func (lb *ChunkAwareLoadBalancer) RouteChunkRequest(chunkID ChunkID) NodeID {
    // 1. Find all nodes with this chunk
    candidates := lb.chunkLocations[chunkID]
    
    // 2. Filter by health and load
    available := filterHealthy(candidates)
    available = filterByLoad(available, maxLoad=0.8)
    
    // 3. Select by latency (fastest)
    return selectLowestLatency(available)
}

func (lb *ChunkAwareLoadBalancer) DistributeWork(job ComputeJob) []NodeID {
    // For large jobs, distribute across multiple nodes
    requiredNodes := lb.calculateRequiredNodes(job)
    
    // Select least-loaded nodes with required capabilities
    return lb.selectLeastLoaded(requiredNodes, job.Requirements)
}

func (lb *ChunkAwareLoadBalancer) calculateRequiredNodes(job ComputeJob) int {
    // Parallelize large jobs across multiple nodes
    switch {
    case job.Size < 10*MB:
        return 1  // Small job: single node
    case job.Size < 100*MB:
        return 5  // Medium job: 5 nodes
    case job.Size < 1*GB:
        return 20  // Large job: 20 nodes
    default:
        return 50  // Massive job: 50+ nodes (MapReduce style)
    }
}
```

---

## Economic Model (Credit-Based)

### Pricing Scales with Replicas

```go
type AdaptivePricing struct {
    baseRate uint64  // Credits per GB-hour
}

func (ap *AdaptivePricing) CalculateCost(size uint64, replicas int) uint64 {
    // Base cost
    baseCost := (size / GB) * ap.baseRate
    
    // Replication cost (diminishing returns)
    replicationFactor := 1.0
    if replicas > 7 {
        // First 7 replicas: 1.0x
        // Next 43 replicas (8-50): 0.5x each
        // Next 450 replicas (51-500): 0.2x each
        // Next 200 replicas (501-700): 0.1x each
        replicationFactor = 7.0 +
            0.5*float64(min(replicas-7, 43)) +
            0.2*float64(min(max(replicas-50, 0), 450)) +
            0.1*float64(max(replicas-500, 0))
    } else {
        replicationFactor = float64(replicas)
    }
    
    return uint64(float64(baseCost) * replicationFactor)
}
```

**Example Costs:**

```text
1GB file, 5 replicas:   100 credits/hour
1GB file, 50 replicas:  350 credits/hour (not 500!)
1GB file, 500 replicas: 1,200 credits/hour (not 5,000!)
```

---

## Fault Tolerance at Scale

### Erasure Coding for Large Resources

```rust
// Reed-Solomon erasure coding
// Example: 100 data chunks + 20 parity chunks
// Can recover from loss of ANY 20 chunks

pub fn encode_with_parity(chunks: &[Chunk]) -> Vec<Chunk> {
    let data_count = chunks.len();
    let parity_count = (data_count as f32 * 0.2) as usize;
    
    // Generate parity chunks
    let parity = reed_solomon::encode(chunks, parity_count);
    
    // Total: 120 chunks (100 data + 20 parity)
    // Can lose up to 20 chunks and still recover
    [chunks, &parity].concat()
}

pub fn recover_missing_chunks(
    available: &[Chunk],
    missing: &[ChunkID],
) -> Result<Vec<Chunk>> {
    // Recover missing chunks from available + parity
    reed_solomon::decode(available, missing)
}
```

---

## Performance Characteristics

### Bandwidth Scaling

```text
Resource Size | Replicas | Bandwidth/Node | Total Bandwidth
--------------|----------|----------------|----------------
1 MB          | 5        | 10 KB/s        | 50 KB/s
10 MB         | 10       | 20 KB/s        | 200 KB/s
100 MB        | 30       | 50 KB/s        | 1.5 MB/s
1 GB          | 100      | 100 KB/s       | 10 MB/s
10 GB         | 300      | 200 KB/s       | 60 MB/s
100 GB        | 500      | 300 KB/s       | 150 MB/s
```

### Latency Targets

```text
Operation           | Small (5-7)  | Large (500-700)
--------------------|--------------|------------------
First Byte          | 20-50ms      | 10-20ms (cached)
Full Download       | 100-500ms    | 1-5s (parallel)
Chunk Fetch         | 50-100ms     | 20-50ms (nearest)
```

---

### Zero-Copy Reactive Transport (The "Metal" Pipeline)

We move beyond "Message Passing" to **"Reactive Mutation"**. Components do not send messages; they mutate shared state and signal intent.

**The Pipeline:** `Network (WebRTC)` ➔ `SAB (Memory)` ➔ `Rust (Compute)` ➔ `Go (Logic)` ➔ `JS (Render)`

#### 1. The Shared State (SAB Layout)

All components share a single 48MB Linear Memory region (SAB). The first 16MB are reserved for the Go Kernel.

| Offset (Abs) | Size | Name | Purpose |
| :--- | :--- | :--- | :--- |
| `0x01000000` | 256B | **Metadata** | Atomic Flags & Manifests (The "Switchboard") |
| `0x01050000` | 512KB | **Inbox** | RingBuffer: Kernel ➔ Module (JobRequest) |
| `0x010D0000` | 512KB | **Outbox** | RingBuffer: Module ➔ Kernel (JobResult) |
| `0x01150000` | 31MB+ | **Arena** | Dynamic Heap for payloads (Zero-Copy) |

#### 2. The Signaling Protocol (Mutate ➔ Signal ➔ React)

Instead of queues, we use atomic flags.

##### Step A: Mutation (The Action)

- Rust decompresses a chunk directly into the **Arena** at `0xF000`.
- Rust updates the **Manifest** at `0x0010`: `{ Type: PHYSICS_UPDATE, Ptr: 0xF000 }`.

##### Step B: The Signal (The Switch)

- Rust performs a single Atomic Store: `Flags.KernelEvent = 1`.

##### Step C: Reaction (The Context)

- Go (watching the atomic) sees `1`.
- Go reads the Manifest at `0x0010`.
- Go acts on the data at `0xF000` without moving a single byte.

#### 3. Flow Control (Backpressure)

To prevent the "Firehose Effect" (Network faster than Rust), we use atomic counters:

- `InboxCount`: Incremented by JS (write), Decremented by Rust (read).
- **Rule:** If `InboxCount > WarningThreshold`, JS pauses Network Reads (TCP/WebRTC backpressure).

### Cache Metrics

```go
type CacheMetrics struct {
    Hits       uint64
    Misses     uint64
    Evictions  uint64
    HitRate    float64  // Hits / (Hits + Misses)
    AvgLatency time.Duration
}

func (c *SABCache) GetMetrics() CacheMetrics {
    total := c.Hits + c.Misses
    hitRate := 0.0
    if total > 0 {
        hitRate = float64(c.Hits) / float64(total)
    }
    
    return CacheMetrics{
        Hits:      c.Hits,
        Misses:    c.Misses,
        Evictions: c.Evictions,
        HitRate:   hitRate,
    }
}
```

---

## The Grounding Agent: Consistent Truth & Mesh Synchronization

### 1. Consistent Truth (Merkle State Sync)

In a world without a central database, "truth" is determined by cryptographic consensus.

- **The Merkle Root**: Every node maintains a Merkle tree representing the state of its assigned sector or the common mesh metadata.
- **Anti-Entropy**: Nodes periodically swap Merkle roots via the Gossip protocol. If the roots match, the truth is consistent. If they differ, the nodes perform a "Merkle walk" to identify and reconcile only the specific chunks that are out of sync.
- **Content-Addressing**: Since all data is identified by its hash (BLAKE3/SHA256), the *content* of the mesh **is** the truth of the mesh. If the hash matches, the data is verified.

### 2. Mesh Time (Logical Synchronization)

Synchronizing physical clocks (NTP) in a global mesh is unreliable. INOS uses **Logical Time** to maintain causality across distributed compute jobs.

- **Lamport Timestamps**: We use monotonically increasing counters to ensure that an effect always follows its cause.
- **No Global Clock**: The mesh does not wait for a central "tick." Instead, the "truth" propagates as quickly as the network allows.
- **Epoch Ticking**: The local Kernel uses an `EpochTicker` to trigger periodic optimizations, but these are local events that eventually converge globally.

### 3. Decentralized Signaling (Mesh-Native After Bootstrap)

INOS avoids permanent signaling centralization. Any node can relay signaling once connected to the mesh.

- **Gossip Relay**: WebRTC Session Descriptions (SDP) and ICE candidates are propagated over the mesh under the `webrtc.signaling` topic.
- **First Contact**: New nodes use at least one known bootstrap address (seed node/domain) to enter the network.
- **Resilience**: After initial contact, signaling and discovery fan out across peers; no permanent single signaling server is required.

---

## Summary: A Serverless Distributed OS

**INOS P2P Mesh is:**

- ✅ **Decentralized Signaling** - No permanent central signaling dependency after first contact.
- ✅ **Consistent Truth** - Merkle-tree based anti-entropy ensures all nodes converge on the same state.
- ✅ **Resource-Agnostic** - Works for compute, storage, streams, and state.
- ✅ **Zero-Copy Pipeline** - Uses SharedArrayBuffer (SAB) to move data between Go, Rust, and JS without overhead.
- ✅ **Adaptive Scaling** - Replicas scale from 5 to 700+ based on real-time demand.

**This is a true distributed operating system**, where the network itself provides the compute and storage primitives required for the next generation of AI-driven applications.
