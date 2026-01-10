# IMPLEMENT: Synergetic Automaton (Moonshot Robot)

This document is the official technical manifest and implementation prompt for the INOS "Moonshot" robot.

## 1. Architectural Context
- **Infrastructure**: Zero-copy decentralized compute fabric using `SharedArrayBuffer` (SAB).
- **Control Plane**: Go Kernel (Supervisor) -> Rust Units (Physics/IK) -> JS/WebGPU (Rendering).
- **Synchronization**: Epoch-based lock-free signaling via absolute SAB offsets.

## 2. SAB Memory Layout (Zero-Copy)

| Offset | Structure | Description |
|:-------|:----------|:------------|
| **0x4000** | `SYSTEM_ROBOT_STATE` | Global simulation status (Phase, Syntropy Score) |
| **0x4100** | `NODE_MATRICES` | 32KB block for 512 node transformation matrices |
| **0xC100** | `NODE_STATE` | 16KB block for node-level physics (pos, vel, targets) |
| **0x10100** | `FILAMENT_DATA` | 12KB block for 1024 magnetic thread connections |

### Detailed Fields
- **SYSTEM_ROBOT_STATE**: `u64 epoch`, `u32 phase` (0=entropic, 1=emergent, 2=articulate), `f32[4] syntropy_score`.
- **NODE_STATE**: `f32[3] pos`, `f32[3] vel`, `f32[3] target`, `u16[8] neighbor_indices`.
- **FILAMENT_DATA**: `u16[2] node_indices`, `f32 current_len`, `f32 target_len`, `f32 energy_pulse`.

## 3. Component Responsibilities

### [A] Go Robot Supervisor
- **Role**: High-level intelligence and coordination.
- **Logic**: Implements Q-learning/Evolutionary algorithms to optimize the `syntropy_score`.
- **Signal**: Watches `IDX_ROBOT_EPOCH` and broadcasts commands via the state zone.

### [B] Rust RobotUnit (`modules/compute/src/units/robot.rs`)
- **Role**: High-frequency deterministic physics.
- **Logic**: Verlet integration for nodes + dynamic spring constraints for filaments.
- **Syntropy Algorithm**: Guides nodes from Brownian motion (chaos) to structural alignment (humanoid).

### [C] JS/WebGPU Frontend (`Cosmos.tsx`)
- **Role**: High-fidelity visualization and telemetry.
- **Rendering**: Three.js `InstancedMesh` (512 Icosahedrons) + `LineSegments` (1024 filaments).
- **Shader (`SyntropyShader`)**:
    - Vertex: Energy pulse propagation along filaments.
    - Fragment: Phase-based shift (Chaos/Red -> Order/Blue -> Articulation/Gold).

## 4. Animation Goals: "The Emergence of Order"
1. **Entropic Phase**: Nodes drift randomly; filaments are long and weak.
2. **Emergent Phase**: Nodes coalesce into a recognizable silhouette (Syntropy Optimization).
3. **Articulate Phase**: The "Codex Handshake"—a delicate, recursive gesture of protocol locking.

## 5. Performance Targets
- **Framerate**: 60 FPS constant.
- **Latency**: Sub-millisecond reaction to SAB epoch signals.
- **Zero-Copy**: 100% direct memory access in both Rust and JS; zero serialization.
