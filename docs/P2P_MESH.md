# INOS P2P Mesh Architecture
## Adaptive Resource Allocation at Scale

## Overview

INOS implements an **adaptive P2P mesh** where replica count scales dynamically based on resource size, demand, and economic constraints. The system manages resources from **5-7 replicas** (small, low-demand) to **500-700 replicas** (large, high-demand) using intelligent chunking and work allocation.

## Core Principle: Adaptive Scaling

**Not all resources are equal.** The mesh adapts to:
- **Resource Size** - 1KB file vs 1GB file
- **Demand Level** - 1 request/hour vs 1000 requests/second
- **Credit Budget** - Free tier vs premium allocation
- **Network Topology** - Local cluster vs global distribution

---

## Scaling Strategy

### Small Resources (< 1MB)
```
Size: < 1MB
Replicas: 5-7 nodes
Strategy: Full replication
Use Case: State, metadata, small files
```

### Medium Resources (1MB - 100MB)
```
Size: 1MB - 100MB
Replicas: 10-50 nodes
Strategy: Chunked replication (1MB chunks)
Use Case: Images, documents, code repositories
```

### Large Resources (100MB - 10GB)
```
Size: 100MB - 10GB
Replicas: 50-500 nodes
Strategy: Distributed chunking + erasure coding
Use Case: Videos, datasets, databases
```

### Massive Resources (> 10GB)
```
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
```
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
```
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

```
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

```
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
|:---|:---|:---|:---|
| `0x01000000` | 256B | **Metadata** | Atomic Flags & Manifests (The "Switchboard") |
| `0x01050000` | 512KB | **Inbox** | RingBuffer: Kernel ➔ Module (JobRequest) |
| `0x010D0000` | 512KB | **Outbox** | RingBuffer: Module ➔ Kernel (JobResult) |
| `0x01150000` | 31MB+ | **Arena** | Dynamic Heap for payloads (Zero-Copy) |

#### 2. The Signaling Protocol (Mutate ➔ Signal ➔ React)
Instead of queues, we use atomic flags.

**Step A: Mutation (The Action)**
*   Rust decompresses a chunk directly into the **Arena** at `0xF000`.
*   Rust updates the **Manifest** at `0x0010`: `{ Type: PHYSICS_UPDATE, Ptr: 0xF000 }`.

**Step B: The Signal (The Switch)**
*   Rust performs a single Atomic Store: `Flags.KernelEvent = 1`.

**Step C: Reaction (The Context)**
*   Go (watching the atomic) sees `1`.
*   Go reads the Manifest at `0x0010`.
*   Go acts on the data at `0xF000` without moving a single byte.

#### 3. Flow Control (Backpressure)
To prevent the "Firehose Effect" (Network faster than Rust), we use atomic counters:
*   `InboxCount`: Incremented by JS (write), Decremented by Rust (read).
*   **Rule:** If `InboxCount > WarningThreshold`, JS pauses Network Reads (TCP/WebRTC backpressure).

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

## Summary

**INOS Adaptive Mesh:**
- ✅ **Scales from 5 to 700 replicas** based on size and demand
- ✅ **Conservative load targeting** - Never exceed 50% per node (sweet spot: 25-50%)
- ✅ **Sparse network handling** - Gracefully degrades when nodes are scarce
- ✅ **1MB chunking** for optimal P2P performance
- ✅ **Hierarchical distribution** for massive resources
- ✅ **Economic pricing** with diminishing returns
- ✅ **Erasure coding** for fault tolerance
- ✅ **Smart load balancing** with chunk awareness
- ✅ **SAB-based LRU cache** for zero-copy performance

**Key Insight:** The "Rule of 5-7" is just the **minimum baseline**. The mesh adapts to resource needs, scaling to hundreds of nodes for large, high-demand resources while maintaining efficiency for small resources **and never overloading nodes**.

This is a **true distributed CDN + compute platform** with intelligent caching, not just a P2P file sharing system.



## Overview

INOS implements a **demand-based P2P mesh** where nodes dynamically allocate resources (compute, storage, bandwidth) based on real-time needs. The **Rule of 5-7** applies to **any resource type**, not just particle physics.

## Core Principles

### 1. **Resource-Agnostic Replication**
Any resource can be replicated across 5-7 nodes:
- **Compute Jobs** (ML inference, physics simulation, rendering)
- **Storage Blocks** (files, databases, caches)
- **Sensor Streams** (camera, GPS, IMU data)
- **State Synchronization** (game state, collaborative editing)

### 2. **Demand-Based Allocation**
Resources are allocated based on:
- **Load** - Current CPU/GPU/memory usage
- **Latency** - Network RTT to requesting node
- **Cost** - Credit balance and pricing
- **Capability** - Node hardware (GPU, storage, sensors)
- **Reputation** - Historical reliability score

### 3. **Adaptive Topology**
The mesh topology changes based on demand:
- **Hot resources** → More replicas (up to 7)
- **Cold resources** → Fewer replicas (down to 3)
- **Geographic clustering** → Nodes group by latency
- **Capability clustering** → GPU nodes form sub-meshes

---

## Layer Responsibilities (System-Wide)

### Go WASM (Kernel - Orchestration)
**Role**: Resource allocation, peer coordination, economic routing

```go
// Kernel manages ALL resources, not just particles
type Resource interface {
    Type() ResourceType  // Compute, Storage, Stream, State
    Size() uint64        // Bytes or compute units
    Replicas() []NodeID  // Current replica set (5-7 nodes)
    Demand() float64     // Current demand score (0-1)
}

// Dynamic allocation based on demand
func (k *Kernel) AllocateResource(r Resource) {
    // 1. Score all available nodes
    scores := k.ScoreNodes(r)
    
    // 2. Select optimal replica set (5-7 nodes)
    replicas := k.SelectReplicas(scores, r.Demand())
    
    // 3. Assign primary (lowest latency + highest capability)
    primary := replicas[0]
    
    // 4. Coordinate replication
    k.CoordinateReplication(primary, replicas[1:], r)
}
```

**Why Go?**
- Goroutines for managing thousands of resources concurrently
- Channels for coordinating replica groups
- Built-in networking for DHT and peer discovery
- Low GC overhead for routing decisions

### Rust WASM (Modules - Execution)
**Role**: Execute compute jobs, compress data, manage storage

```rust
// Rust executes ANY compute job, not just physics
pub trait ComputeJob {
    fn execute(&self, input: &[u8]) -> Result<Vec<u8>>;
    fn compress(&self, output: &[u8]) -> Vec<u8>;  // LZ4/Brotli
    fn write_sab(&self, data: &[u8], offset: usize);  // Zero-copy
}

// Examples:
// - ML inference (TensorFlow Lite)
// - Image processing (resize, filter)
// - Video encoding (H.264)
// - Physics simulation
// - Cryptographic operations
```

**Why Rust?**
- Zero-cost abstractions for any compute workload
- SIMD for parallel processing
- No GC pauses during critical operations
- Direct SAB access for zero-copy I/O

### JavaScript (Host - Coordination)
**Role**: WebRTC connections, UI, sensor access

```typescript
// JS coordinates resources across the mesh
class ResourceMesh {
  // Manage ANY resource type
  async allocate(resource: Resource): Promise<ReplicaGroup> {
    // 1. Query kernel for optimal allocation
    const allocation = await this.kernel.allocate(resource);
    
    // 2. Establish WebRTC connections to replicas
    const connections = await this.connectToReplicas(allocation.replicas);
    
    // 3. Monitor demand and adjust
    this.monitorDemand(resource, connections);
    
    return { primary: allocation.primary, replicas: connections };
  }
}
```

---

## Resource Types and Allocation Strategies

### 1. **Compute Resources**

```typescript
interface ComputeJob {
  type: 'ml_inference' | 'physics' | 'rendering' | 'encoding';
  input: Uint8Array;
  requirements: {
    gpu: boolean;
    memory: number;  // MB
    duration: number;  // estimated ms
  };
}

// Allocation strategy: Capability + Load
function allocateCompute(job: ComputeJob): ReplicaGroup {
  // Filter nodes by capability
  const capable = nodes.filter(n => 
    (!job.requirements.gpu || n.hasGPU) &&
    n.availableMemory >= job.requirements.memory
  );
  
  // Score by load (prefer idle nodes)
  const scored = capable.map(n => ({
    node: n,
    score: 1.0 - n.cpuUsage  // Higher score = less loaded
  }));
  
  // Select top 5-7 nodes
  return selectTopN(scored, 5 + Math.floor(job.demand * 2));
}
```

### 2. **Storage Resources**

```typescript
interface StorageBlock {
  hash: string;  // Content-addressed
  size: number;  // Bytes
  temperature: 'hot' | 'warm' | 'cold';  // Access frequency
}

// Allocation strategy: Temperature + Geography
function allocateStorage(block: StorageBlock): ReplicaGroup {
  const replicaCount = {
    hot: 7,   // Frequently accessed → more replicas
    warm: 5,  // Occasionally accessed
    cold: 3   // Rarely accessed → fewer replicas
  }[block.temperature];
  
  // Geographic distribution for fault tolerance
  return selectGeographicallyDistributed(nodes, replicaCount);
}
```

### 3. **Sensor Streams**

```typescript
interface SensorStream {
  type: 'camera' | 'gps' | 'imu' | 'microphone';
  fps: number;
  bandwidth: number;  // KB/s
}

// Allocation strategy: Latency + Bandwidth
function allocateSensorStream(stream: SensorStream): ReplicaGroup {
  // Prioritize low-latency nodes
  const lowLatency = nodes.filter(n => n.latency < 50);  // < 50ms
  
  // Filter by available bandwidth
  const capable = lowLatency.filter(n => 
    n.availableBandwidth >= stream.bandwidth
  );
  
  // Select 5-7 nearest nodes
  return selectByLatency(capable, 5);
}
```

### 4. **State Synchronization**

```typescript
interface SharedState {
  type: 'game_state' | 'document' | 'whiteboard';
  updateFrequency: number;  // Updates per second
  conflictResolution: 'lww' | 'crdt' | 'ot';  // Last-write-wins, CRDT, OT
}

// Allocation strategy: Consistency + Latency
function allocateState(state: SharedState): ReplicaGroup {
  // For high-frequency updates, prefer nearby nodes
  if (state.updateFrequency > 30) {
    return selectByLatency(nodes, 5);
  }
  
  // For low-frequency, prioritize geographic distribution
  return selectGeographicallyDistributed(nodes, 7);
}
```

---

## Demand-Based Scaling

### Dynamic Replica Adjustment

```go
type DemandMonitor struct {
    resources map[ResourceID]*Resource
    metrics   *MetricsCollector
}

func (dm *DemandMonitor) AdjustReplicas() {
    for _, resource := range dm.resources {
        demand := dm.CalculateDemand(resource)
        
        // Scale up if demand is high
        if demand > 0.8 && len(resource.Replicas) < 7 {
            newReplica := dm.RecruitReplica(resource)
            resource.Replicas = append(resource.Replicas, newReplica)
        }
        
        // Scale down if demand is low
        if demand < 0.3 && len(resource.Replicas) > 3 {
            resource.Replicas = resource.Replicas[:len(resource.Replicas)-1]
        }
    }
}

func (dm *DemandMonitor) CalculateDemand(r *Resource) float64 {
    // Weighted demand score
    return 0.4 * r.AccessFrequency +   // How often accessed
           0.3 * r.NetworkUtilization + // Bandwidth usage
           0.2 * r.ComputeLoad +        // CPU/GPU usage
           0.1 * r.CreditFlow           // Economic activity
}
```

### Load Balancing

```go
type LoadBalancer struct {
    replicaGroups map[ResourceID]*ReplicaGroup
}

func (lb *LoadBalancer) RouteRequest(req Request) NodeID {
    group := lb.replicaGroups[req.ResourceID]
    
    // 1. Check if primary is available
    if group.Primary.IsHealthy() && group.Primary.Load < 0.8 {
        return group.Primary.ID
    }
    
    // 2. Find least-loaded replica
    leastLoaded := group.Replicas[0]
    for _, replica := range group.Replicas[1:] {
        if replica.Load < leastLoaded.Load {
            leastLoaded = replica
        }
    }
    
    return leastLoaded.ID
}
```

---

## Economic Routing

### Credit-Based Allocation

```go
type EconomicRouter struct {
    creditLedger *CreditLedger
    pricing      *PricingModel
}

func (er *EconomicRouter) AllocateWithBudget(
    resource Resource,
    budget uint64,  // Credits available
) (*ReplicaGroup, error) {
    // 1. Calculate cost for different replica counts
    costs := make(map[int]uint64)
    for n := 3; n <= 7; n++ {
        costs[n] = er.pricing.CalculateCost(resource, n)
    }
    
    // 2. Find maximum replicas within budget
    replicaCount := 3
    for n := 7; n >= 3; n-- {
        if costs[n] <= budget {
            replicaCount = n
            break
        }
    }
    
    // 3. Allocate with optimal replica count
    return er.AllocateReplicas(resource, replicaCount)
}
```

### Pricing Model

```go
type PricingModel struct {
    computeRate  uint64  // Credits per CPU-second
    storageRate  uint64  // Credits per GB-hour
    bandwidthRate uint64  // Credits per GB transferred
}

func (pm *PricingModel) CalculateCost(r Resource, replicas int) uint64 {
    baseCost := pm.getBaseCost(r)
    
    // Replication overhead (diminishing returns)
    replicationMultiplier := 1.0 + 0.15*float64(replicas-1)
    
    // Geographic distribution discount (fault tolerance)
    geoDiscount := 0.9  // 10% discount for distributed replicas
    
    return uint64(float64(baseCost) * replicationMultiplier * geoDiscount)
}
```

---

## Fault Tolerance (System-Wide)

### Automatic Failover

```go
func (mesh *P2PMesh) HandleNodeFailure(failedNode NodeID) {
    // Find all resources affected by failure
    affectedResources := mesh.FindResourcesByNode(failedNode)
    
    for _, resource := range affectedResources {
        // 1. Remove failed node from replica group
        resource.RemoveReplica(failedNode)
        
        // 2. If primary failed, elect new primary
        if resource.Primary == failedNode {
            resource.Primary = mesh.ElectNewPrimary(resource.Replicas)
        }
        
        // 3. Recruit replacement replica if needed
        if len(resource.Replicas) < 3 {
            newReplica := mesh.RecruitReplica(resource)
            resource.AddReplica(newReplica)
        }
        
        // 4. Redistribute load from failed node
        mesh.RedistributeLoad(failedNode, resource)
    }
}
```

---

## Performance Characteristics

### Bandwidth Usage (Per Node)

```
Resource Type       | Replicas | Bandwidth/Node
--------------------|----------|----------------
Compute (ML)        | 5        | 50 KB/s
Storage (Hot)       | 7        | 200 KB/s
Sensor (Camera)     | 5        | 500 KB/s
State (Game)        | 5        | 100 KB/s
--------------------|----------|----------------
Total (Mixed Load)  | ~5-7     | ~850 KB/s
```

### Latency Targets

```
Operation           | Target    | With 5-7 Replicas
--------------------|-----------|-------------------
Compute Job         | < 100ms   | 50-80ms (fastest)
Storage Read        | < 50ms    | 20-40ms (cached)
Sensor Frame        | < 33ms    | 16-25ms (60fps)
State Update        | < 16ms    | 8-12ms (local)
```

---

## Summary

**INOS P2P Mesh is:**
- ✅ **Resource-agnostic** - Works for compute, storage, streams, state
- ✅ **Demand-based** - Scales replicas (3-7) based on load
- ✅ **Economically-aware** - Routes based on credit budgets
- ✅ **Fault-tolerant** - Automatic failover and redistribution
- ✅ **Adaptive** - Topology changes with demand patterns

**This is a distributed operating system**, not just a particle demo.


## Overview

INOS uses a **quorum-based P2P mesh** with 5-7 node replication per sector. This architecture provides fault tolerance, low latency, and efficient bandwidth usage while maintaining eventual consistency across the distributed system.

## Layer Responsibilities

### Go WASM (Kernel - Orchestration)
**Role**: Sector allocation, peer management, consensus

```go
// Kernel manages:
- Sector assignment (which node owns which particles)
- Replica group coordination (5-7 nodes per sector)
- Peer discovery via DHT
- Quorum consensus (3/5 or 4/7 votes)
- Failover and re-election
- Network topology optimization
```

**Why Go?**
- Excellent concurrency (goroutines for peer management)
- Mature networking libraries (WebRTC, DHT)
- Built-in channels for coordination
- Low GC overhead for routing logic

### Rust WASM (Modules - Compute)
**Role**: Physics computation, delta compression, serialization

```rust
// Rust modules handle:
- Particle physics (forces, collisions)
- Delta computation (what changed this frame)
- LZ4/Brotli compression of deltas
- Cap'n Proto serialization
- Direct SAB writes (zero-copy)
```

**Why Rust?**
- Zero-cost abstractions for compression
- SIMD for physics calculations
- No GC pauses during compression
- Direct memory access to SAB

### JavaScript (Host - Rendering)
**Role**: Three.js rendering, WebRTC connections, UI

```typescript
// JS handles:
- Three.js particle rendering
- WebRTC DataChannel management
- User input
- SAB reads (zero-copy from Rust)
```

## The Zero-Copy Pipeline

```
┌─────────────────────────────────────────────────────────────┐
│ Frame N                                                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ 1. Rust: Compute Physics (5ms)                              │
│    └─> Write to SAB (zero-copy)                             │
│                                                              │
│ 2. Rust: Compute Delta (1ms)                                │
│    └─> Only changed particles                               │
│                                                              │
│ 3. Rust: Compress Delta (2ms)                               │
│    └─> LZ4: ~70% size reduction                             │
│                                                              │
│ 4. Go: Route to Replicas (1ms)                              │
│    └─> Fastest-first broadcast                              │
│                                                              │
│ 5. Network: Send Compressed Delta (20-50ms)                 │
│    └─> 780 bytes @ 60fps = 46.8KB/s                         │
│                                                              │
│ 6. Peer Rust: Decompress (1ms)                              │
│    └─> Write to peer's SAB (zero-copy)                      │
│                                                              │
│ 7. JS: Render from SAB (8ms)                                │
│    └─> Read Float32Array (zero-copy)                        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
Total: ~38-68ms (including network)
```

## Performance Gains

### Without Zero-Copy (Traditional)
```
Physics (5ms)
  ↓ Copy to JS (2ms)
  ↓ JSON.stringify (5ms)
  ↓ Compress (3ms)
  ↓ Network (50ms)
  ↓ Decompress (3ms)
  ↓ JSON.parse (5ms)
  ↓ Copy to render buffer (2ms)
  ↓ Render (8ms)
────────────────────
Total: 83ms (12 fps)
```

### With Zero-Copy (INOS)
```
Physics → SAB (5ms)
  ↓ Delta compute (1ms)
  ↓ Compress (2ms)
  ↓ Network (50ms)
  ↓ Decompress (1ms)
  ↓ SAB write (0ms - zero-copy)
  ↓ Render from SAB (8ms)
────────────────────
Total: 67ms (15 fps)

Savings: 16ms per frame (19% faster)
```

### With Fastest-First + Compression
```
Physics → SAB (5ms)
  ↓ Delta compute (1ms)
  ↓ Compress (2ms)
  ↓ Network (20ms - fastest replica)
  ↓ Decompress (1ms)
  ↓ SAB write (0ms)
  ↓ Render from SAB (8ms)
────────────────────
Total: 37ms (27 fps)

Savings: 46ms per frame (55% faster)
```

## Sector Replication (Rule of 5-7)

```
Sector 0 (Particles 0-199)
├── Primary: Node A (authoritative)
└── Replicas: [B, C, D, E, F]

Quorum: 3/5 or 4/7 nodes must agree
Latency: Wait for fastest replica only
Bandwidth: 46.8KB/s per connection
```

### Consensus Protocol

```go
type ReplicaGroup struct {
    Primary   NodeID
    Replicas  []NodeID
    VectorClock map[NodeID]uint64
}

func (rg *ReplicaGroup) Write(delta Delta) error {
    // 1. Primary writes immediately
    rg.Primary.WriteSAB(delta)
    
    // 2. Broadcast to replicas (non-blocking)
    promises := make([]Promise, len(rg.Replicas))
    for i, replica := range rg.Replicas {
        promises[i] = replica.WriteAsync(delta)
    }
    
    // 3. Wait for fastest replica (quorum = 1)
    return WaitForFastest(promises, timeout=50ms)
    
    // 4. Background reconciliation for stragglers
    go rg.ReconcileInBackground(promises)
}
```

## Bandwidth Analysis

```
Per Sector (200 particles):
- Changed per frame: ~50 particles (25%)
- Uncompressed: 50 × 26 bytes = 1.3KB
- Compressed (LZ4): 1.3KB × 0.3 = 390 bytes
- At 60fps: 390 × 60 = 23.4KB/s

Per Node (managing 1 sector):
- Outbound: 23.4KB/s × 6 replicas = 140KB/s
- Inbound: 23.4KB/s × 6 replicas = 140KB/s
- Total: 280KB/s (manageable on 4G)

Scaling to 1000 nodes:
- Each node: 280KB/s
- Global throughput: 280MB/s (distributed)
```

## Failover Strategy

```go
func (mesh *P2PMesh) HandleNodeFailure(failedNode NodeID) {
    // 1. Detect failure (no heartbeat for 5s)
    if !failedNode.IsAlive() {
        // 2. If failed node was primary, elect new primary
        if failedNode == sector.Primary {
            newPrimary := mesh.ElectPrimary(sector.Replicas)
            sector.Primary = newPrimary
        }
        
        // 3. Remove from replica group
        sector.Replicas = remove(sector.Replicas, failedNode)
        
        // 4. If replicas < 5, recruit new node
        if len(sector.Replicas) < 5 {
            newReplica := mesh.RecruitReplica(sector)
            sector.Replicas = append(sector.Replicas, newReplica)
        }
    }
}
```

## Summary

**Key Insights:**
1. **Go manages allocation** - Sector assignment, peer coordination, consensus
2. **Rust does heavy lifting** - Physics, compression, serialization
3. **Zero-copy throughout** - SAB eliminates 4 copy operations
4. **Fastest-first delivery** - Don't wait for slow replicas
5. **Compression** - 70% bandwidth reduction

**Performance:**
- **55% faster** than traditional approach
- **280KB/s** per node (scales to thousands)
- **27 fps** with network latency included

This architecture is production-ready for distributed physics simulation.
