# AI Grand Prix Drone Racing

Real-time autonomous drone racing using INOS zero-copy architecture.

---

## Fast-Path Architecture

Drone racing uses the **same patterns as boids**, proven at 10,000+ entities @ 60fps.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ GRAND PRIX FAST PATH                                                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  External Control              Compute Worker              Three.js        │
│  (WebSocket)                   (Direct SAB)                (Main Thread)   │
│  ┌──────────┐                  ┌───────────────┐           ┌─────────────┐ │
│  │  Python  │ Binary           │  drone.rs     │ Epoch     │  Instanced  │ │
│  │  DCL     │ ─────────►       │  step_all()   │ ───────►  │  Mesh       │ │
│  │  Client  │                  │               │           │  Render     │ │
│  └──────────┘                  └───────────────┘           └─────────────┘ │
│       │                              │                           │         │
│       │                              │                           │         │
│       ▼                              ▼                           ▼         │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    SharedArrayBuffer                                │   │
│  │  ┌───────────┐  ┌────────────┐  ┌────────────┐  ┌───────────────┐  │   │
│  │  │  Control  │  │  State A   │  │  State B   │  │   Matrices    │  │   │
│  │  │  16B/drone│  │  (write)   │  │  (read)    │  │   16×f32/drn  │  │   │
│  │  └───────────┘  └────────────┘  └────────────┘  └───────────────┘  │   │
│  │                           Ping-Pong Flip                            │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Key Patterns (Proven)

| Pattern | Source | Grand Prix Use |
|:--------|:-------|:---------------|
| Compute Worker | `compute.worker.ts` | `drone.rs` physics loop |
| Ping-Pong | `pingpong.rs` | State A/B flip on epoch |
| Epoch Signal | `sab.rs` | `IDX_DRONE_PHYSICS` (50) |
| Batched Update | `boids.rs` | `step_all_drones()` |
| Instanced Mesh | `ArchitecturalBoids.tsx` | 8-part drone model |

---

## SAB Layout (Drone Region)

```
0x160000  Control Buffer    32 × 16B = 512B      [throttle, pitch, roll, yaw]
0x160200  State Buffer A    32 × 128B = 4KB      [pos, vel, quat, ...]
0x161200  State Buffer B    32 × 128B = 4KB      [pos, vel, quat, ...]
0x162200  Matrix Buffer A   32 × 64B = 2KB       [4×4 transform]
0x162A00  Matrix Buffer B   32 × 64B = 2KB       [4×4 transform]
```

---

## Drone State (128 bytes per drone)

```rust
#[repr(C)]
pub struct DroneState {
    pub position: [f32; 4],      // XYZ + pad (16B)
    pub velocity: [f32; 4],      // XYZ + pad (16B)
    pub orientation: [f32; 4],   // Quaternion WXYZ (16B)
    pub angular_vel: [f32; 4],   // XYZ + pad (16B)
    pub motor_rpm: [f32; 4],     // 4 motors (16B)
    pub control: [f32; 4],       // thr/pitch/roll/yaw (16B)
    pub race_state: [u32; 4],    // lap/gate/time/flags (16B)
    pub reserved: [f32; 4],      // Future use (16B)
}
```

---

## Physics (Simple 6DOF)

No Rapier overhead. Direct quadcopter dynamics:

```rust
pub fn step_all_drones(sab: &mut [u8], count: u32, dt: f32) {
    let states = DronePool::from_sab_mut(sab);
    
    for i in 0..count {
        let ctrl = states.control(i);
        
        // Motor model
        let thrust = motor_thrust(ctrl.throttle);
        
        // Forces
        let gravity = [0.0, -9.81 * MASS, 0.0, 0.0];
        let thrust_world = quat_rotate(states.orientation(i), [0.0, thrust, 0.0, 0.0]);
        
        // Integration
        let accel = vec4_add(gravity, thrust_world);
        states.set_velocity(i, vec4_add(states.velocity(i), vec4_scale(accel, dt)));
        states.set_position(i, vec4_add(states.position(i), vec4_scale(states.velocity(i), dt)));
        
        // Quaternion integration
        integrate_quat(states.orientation_mut(i), states.angular_vel(i), dt);
    }
    
    signal_epoch(IDX_DRONE_PHYSICS);
}
```

---

## External Control (WebSocket)

```python
# Python client (simple_controller.py)
import websocket
import struct

ws = websocket.WebSocket()
ws.connect("ws://localhost:5173/ws/control")

while True:
    # Read sensor state (future: via WebSocket response)
    # Compute control
    throttle, pitch, roll, yaw = compute_control(state)
    
    # Send binary command
    ws.send(struct.pack('<Bffff', drone_id, throttle, pitch, roll, yaw))
```

---

## Epochs

| Index | Name | Writer | Reader |
|:------|:-----|:-------|:-------|
| 48 | `IDX_DRONE_SENSOR` | Rust | Python |
| 49 | `IDX_DRONE_CONTROL` | Python | Rust |
| 50 | `IDX_DRONE_PHYSICS` | Rust | Three.js |
| 51 | `IDX_RACE_STATE` | Rust | UI |

---

## Files

| File | Purpose |
|:-----|:--------|
| `modules/compute/src/units/drone.rs` | 6DOF batched physics |
| `frontend/src/racing/control-bridge.ts` | WebSocket ↔ SAB |
| `frontend/app/pages/GrandPrix.tsx` | Three.js simulation |
| `examples/python/racing_client.py` | Algorithm template |

---

## Performance

| Stage | Time | Method |
|:------|:-----|:-------|
| Control → SAB | 0.1ms | WebSocket binary |
| Physics (32) | 0.8ms | Batched, no Rapier |
| Matrix gen | 0.2ms | Inline |
| SAB → GPU | 0.3ms | Ping-pong |
| **Total** | **1.4ms** | 60fps headroom |

---

## Quick Start

```bash
# Build
make modules-build

# Run
cd frontend && yarn dev

# Open
open http://localhost:5173/grandprix
```
