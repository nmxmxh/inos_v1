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

## Environment & Visual Pipeline (GPU-Aware)

Grand Prix now uses the GPU unit for production-grade procedural visuals while keeping zero-copy rules intact.

```
GPU NOISE PATH (visuals only)
GPU Unit (gpu.rs) -> WebGpuRequest -> WebGpuExecutor (WebGPU) -> DataTexture -> Three.js
```

Principles applied (from graphics.md):
- Cached TypedArray views (no per-frame allocation)
- Ping-pong epoch for drone state reads
- Instanced meshes for all drone parts
- GPU-driven noise updated on a fixed cadence (not every frame)

Textures are updated periodically and reused across frames:
- Ground albedo + roughness variation
- Cloud shadow layer (alpha map)

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
    pub reserved: [f32; 4],      // battery, motor_temp, ... (16B)
}
```

---

## Physics (Production 6DOF)

No Rapier overhead. Full quad dynamics with:
- Motor lag + torque (C_T / C_Q)
- Battery sag + thermal drift
- Anisotropic drag in body axes
- Wind + turbulence + ground effect
- Moment-based mixer (X-frame)

```rust
pub fn step_all_drones(...) {
    // Control → moments → motor thrusts → forces/torques
    // Wind + turbulence + ground effect + anisotropic drag
    // Integrate linear + angular dynamics, clamp rates for chaos resistance
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

## Race-Winning Control Algorithm (Production-Grade)

The physics model reflects real-world quad behavior (motor torque, RPM lag, drag anisotropy,
gusts, ground effect, battery sag). A winning controller must push performance while remaining
stable in turbulence and close-proximity flow.

### Strategy

1. **Lookahead Pathing**
   - Fit a spline through the next 2–3 gates.
   - Track the tangent direction at a speed‑scaled lookahead distance.

2. **Velocity Scheduling**
   - `v_max = sqrt(a_lat_max / curvature)`
   - Reduce `v_max` as battery state drops.
   - Reserve vertical margin near ground or heavy gusts.

3. **Rate Control (Primary)**
   - Rate commands: pitch/roll/yaw rates (not absolute angles).
   - Feed‑forward yaw rate from curvature.
   - Bank into turns: roll rate proportional to lateral error.

4. **Wind Compensation**
   - Estimate wind from observed drift.
   - Bias yaw and roll into wind to keep the line.

5. **Chaos Resistance**
   - Clamp jerk (rate change per tick).
   - Recovery mode when angular rates exceed thresholds.
   - Avoid throttle cut near ground (ground effect + downwash).

### Pseudo‑Code (External Controller)

```text
path = spline(gates[current..current+3])
lookahead = clamp(k * speed, 4m, 14m)

heading_err = wrap(desired_heading - current_heading)
lat_err = cross_track_error(path)
curvature = path.curvature(lookahead)

v_max = sqrt(a_lat_max / max(curvature, eps))
v_cmd = clamp(v_nominal, v_min, v_max)

yaw_rate = heading_err * k_yaw + curvature * v_cmd
roll_rate = lat_err * k_roll + yaw_rate * k_bank
pitch_rate = (v_cmd - forward_speed) * k_pitch

throttle = hover + k_alt * alt_err + k_ff * v_cmd^2
throttle = clamp_rate(throttle, prev_throttle, max_delta)

send [throttle, pitch_rate, roll_rate, yaw_rate]
```

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
