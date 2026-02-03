# AI Grand Prix: Pragmatic Implementation

Racing simulation built on INOS's proven fast paths: zero-copy SAB, ping-pong buffers, epoch signaling.

---

## Brutal Reality Acknowledgment

DeepSeek correctly identified risks. Our response uses **existing INOS patterns that already work**:

| Risk | Already Solved By |
|:-----|:------------------|
| 3-layer latency | Compute Worker bypasses Go for physics |
| JS GC pauses | Persistent scratch buffers (zero allocation) |
| Per-entity overhead | Batched ping-pong buffer updates |
| Physics scaling | SIMD-vectorized `step_physics` in Rust |

---

## Fast Path Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│ GRAND PRIX FAST PATH (Proven Pattern from Boids)               │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│                     ┌─────────────────────┐                     │
│                     │  External Control   │                     │
│                     │  (WebSocket/Fetch)  │                     │
│                     └──────────┬──────────┘                     │
│                                │ Control Buffer (write-only)   │
│                                ▼                                │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │              SharedArrayBuffer (SAB)                        ││
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────────────────┐  ││
│  │  │ Epoch   │ │ Control │ │ State A │ │     State B      │  ││
│  │  │ Signals │ │ Buffer  │ │ (write) │ │     (read)       │  ││
│  │  │ [48-55] │ │ 4×f32/  │ │         │ │                  │  ││
│  │  │         │ │ drone   │ │ Drone[] │ │     Drone[]      │  ││
│  │  └────┬────┘ └────┬────┘ └────┬────┘ └────────┬─────────┘  ││
│  │       │           │           │               │             ││
│  └───────┼───────────┼───────────┼───────────────┼─────────────┘│
│          │           │           │               │              │
│          │           │      ┌────┴────┐          │              │
│          │           │      ▼         │          │              │
│          │           │  ┌────────────────────────┼──────────┐   │
│          │           │  │   Compute Worker       │          │   │
│          │           │  │   (Direct SAB Access)  │          │   │
│          │           │  │                        │          │   │
│          │           │  │   drone.rs             │          │   │
│          │           └─►│   ├── read control buf │          │   │
│          │              │   ├── step_all_drones  │◄─────────┘   │
│          │              │   ├── write state A    │  (flip)      │
│          │              │   └── signal epoch     │              │
│          │              └───────────┬────────────┘              │
│          │                          │                           │
│          │ Atomics.waitAsync        │ Atomics.notify            │
│          ▼                          ▼                           │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Three.js (Main Thread)                                  │   │
│  │  ├── useFrame: read State B (opposite of physics write)  │   │
│  │  ├── InstancedMesh.instanceMatrix.array.set(...)         │   │
│  │  └── Zero computation, just copy                         │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Key Design Decisions

### 1. Skip Go Kernel for Physics

Physics runs in **Compute Worker** with direct SAB access. No inbox/outbox hop.

```typescript
// compute.worker.ts - Direct dispatch (existing pattern)
computeExports.step_all_drones(drone_count, dt);
// Rust writes to ping-pong buffer, signals epoch
```

### 2. Batched Drone Physics (250 Hz)

Don't fight for 1000 Hz. Use 250 Hz physics with interpolation.

```rust
// drone.rs - One call updates all drones
pub fn step_all_drones(sab: &[u8], count: u32, dt: f32) {
    let drones = DronePool::from_sab(sab);
    
    for i in 0..count {
        // Read control input
        let ctrl = drones.control(i);
        
        // Motor model → thrust
        let thrust = motor_model(ctrl.throttle, drones.rpm(i));
        
        // 6DOF integration (simple, no Rapier)
        let accel = (thrust / MASS) - GRAVITY;
        drones.set_velocity(i, drones.velocity(i) + accel * dt);
        drones.set_position(i, drones.position(i) + drones.velocity(i) * dt);
        
        // Quaternion integration for orientation
        integrate_quaternion(drones.orientation_mut(i), drones.angular_vel(i), dt);
    }
    
    // Single epoch signal for all drones
    signal_epoch(IDX_DRONE_PHYSICS);
}
```

### 3. Ping-Pong State Buffers (Zero Contention)

Same pattern as boids. Physics writes A, renderer reads B. Flip on epoch.

```typescript
// GrandPrix.tsx
useFrame(() => {
  const epoch = Atomics.load(flags, IDX_DRONE_PHYSICS);
  const isBufferA = epoch % 2 === 0;
  const stateOffset = isBufferA ? DRONE_STATE_B : DRONE_STATE_A;
  
  // Direct SAB read → GPU
  const matrices = new Float32Array(sab, stateOffset, droneCount * 16);
  droneInstances.instanceMatrix.array.set(matrices);
  droneInstances.instanceMatrix.needsUpdate = true;
});
```

### 4. External Control via WebSocket (Realistic)

Python/DCL connects via WebSocket. Control commands are binary-packed.

```typescript
// frontend/src/racing/control-bridge.ts
class ControlBridge {
  private ws: WebSocket;
  private controlView: Float32Array;
  
  constructor(sab: SharedArrayBuffer) {
    this.controlView = new Float32Array(sab, CONTROL_OFFSET, MAX_DRONES * 4);
    this.ws = new WebSocket('ws://localhost:8080/control');
    
    this.ws.onmessage = (event) => {
      // Binary: [droneId:u8, throttle:f32, pitch:f32, roll:f32, yaw:f32]
      const data = new Float32Array(event.data);
      const droneId = data[0] | 0;
      this.controlView.set(data.subarray(1), droneId * 4);
      Atomics.add(flags, IDX_DRONE_CONTROL, 1);
    };
  }
}
```

### 4. GPU-Driven Environment (Visuals Only)

Grand Prix visuals now leverage the GPU unit for procedural noise to drive ground variation and cloud shadows:

```
gpu.rs (execute_wgsl) -> WebGpuRequest -> WebGpuExecutor -> DataTexture -> Three.js materials
```

This path respects graphics.md constraints:
- No per-frame allocations
- Cached TypedArray views
- GPU updates on a fixed cadence (no render-thread stalls)

---

## SAB Layout Extension

```
DRONE_CONTROL_OFFSET = 0x160000  (32 drones × 16 bytes = 512B)
DRONE_STATE_A        = 0x160200  (32 drones × 128 bytes = 4KB)
DRONE_STATE_B        = 0x161200  (32 drones × 128 bytes = 4KB)
DRONE_MATRIX_A       = 0x162200  (32 drones × 64 bytes = 2KB)
DRONE_MATRIX_B       = 0x162A00  (32 drones × 64 bytes = 2KB)
```

---

## Phases

### Phase 1: Single Drone (1 week)
- [ ] `drone.rs` with simple 6DOF
- [ ] Control buffer in SAB
- [ ] Three.js drone model + track
- [ ] Keyboard input

### Phase 2: Multi-Drone (1 week)
- [ ] Batched `step_all_drones`
- [ ] Ping-pong state buffers
- [ ] Instanced rendering (8 drones)
- [ ] Basic collision detection

### Phase 3: External Control (1 week)
- [ ] WebSocket control bridge
- [ ] Python client example
- [ ] Race logic (gates, laps)
- [ ] Telemetry HUD

---

## Files to Create

| File | Purpose |
|:-----|:--------|
| `modules/compute/src/units/drone.rs` | Batched 6DOF physics |
| `frontend/app/pages/GrandPrix.tsx` | Race simulation page |
| `frontend/src/racing/control-bridge.ts` | WebSocket ↔ SAB |
| `examples/python/racing_client.py` | External algorithm client |

---

## Performance Budget

| Stage | Budget | Achieved Via |
|:------|:-------|:-------------|
| Physics (32 drones) | <1ms | Batched loop, no Rapier |
| Matrix gen | <0.2ms | Inline in physics step |
| SAB → GPU | <0.3ms | Ping-pong, zero copy |
| **Total** | <1.5ms | Leaves headroom for 60fps |
