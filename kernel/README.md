# Layer 2: The Kernel (Network Orchestrator)

**Philosophy**: "Knows where to send work, does not do the work."
**Technology**: Go (WASM).

This directory contains the brain of the INOS node. It does not crunch numbers; it manages the graph.

## Role
- **Organic Scheduler**: Dispatches jobs ("Compute Capsules") to nodes based on:
    - **Capability Profiles**: (Has GPU? Is realtime? GeoHash?)
    - **Credit Economy**: (Is it profitable to mine? To store?)
- **Supervisor**: The `threads/` directory contains intelligent supervisors that watch the pipeline.
- **Governance**: Enforces the Metadata DNA policy.

## 3. Orchestration Protocol (`system/v1/orchestration.capnp`)

The Kernel is not just a router; it is a **Supervisor**. It uses the `LifecycleCmd` struct to manage the "living" state of the node.
- **Spawn/Kill**: Starts or stops Rust/WASM capsules based on demand or credit balance.
- **ResizeMem**: Dynamically adjusts the `SharedArrayBuffer` allocation for a capsule (e.g., if a Lidar sensor needs a larger buffer).
- **Heartbeat**: Monitors `isCongested` flags from capsules to trigger backpressure in the Transport layer.

## Structure
- **`threads/`**:
    - **`matchmakers/`**: Match jobs to nodes.
    - **`watchers/`**: Monitor side effects and pipeline health.
    - **`adjusters/`**: Throttle traffic and manage sensor fidelity.
    *   **Contribution**: Allows Nodes to form a mesh and exchange data directly, bypassing the Orchestrator for heavy traffic.

## 🏗 Architecture & Capabilities

The Kernel runs in a loop, processing events and managing the "Parasitic Economy".

### The Organic Scheduler (Pseudocode)

```go
type NodeProfile struct {
    HasGPU     bool
    GeoHash    string
    LatencyMs  int
    CreditCost float64
}

// threads/matchmakers/scheduler.go
func (s *Scheduler) Dispatch(job Job) {
    // 1. Identify Requirements
    requiredCaps := job.Manifest.Capabilities // e.g., ["gpu", "simd"]
    
    // 2. Scan the Mesh (DHT / Gossip)
    candidates := s.Network.FindNodes(requiredCaps)
    
    // 3. Score Candidates (Organic Selection)
    bestNode := candidates.SelectBest(func(n Node) float64 {
        // Prefer local nodes (latency) + low cost
        return (1000 / n.LatencyMs) * (1 / n.CreditCost)
    })
    
    // 4. Send Capsule via WebTransport
    s.Transport.SendCapsule(bestNode.ID, job.CapsuleBytes)
}
```

### Intelligent Supervision (Pseudocode)

```go
// threads/watchers/pipeline.go
func (w *Watcher) MonitorMining() {
    for {
        // Check Battery / User Activity
        if w.Host.IsUserActive() || w.Host.BatteryLevel() < 0.2 {
             // Throttling: Stop Mining
             w.Kernel.Broadcast("mining:stop")
        } else {
             // Resume: Convert electricity to Credits
             w.Kernel.Broadcast("mining:start")
        }
        time.Sleep(1 * time.Second)
    }
}
```

## 🧪 Test Requirements

1.  **Scheduler Logic**:
    *   Simulate a mesh of 1000 nodes with varying latencies and capability profiles.
    *   Verify that `Dispatch()` consistently chooses the optimal node.

2.  **Concurrency**:
    *   Test the `Watcher` routines under high event load (10k events/sec). Ensure Go channels do not block.

3.  **Governance**:
    *   Verify that jobs without valid `CreditLedgerId` are rejected immediately.
