import { SAB_OFFSETS, EPOCH_INDICES, DRONE_CONSTANTS } from './layout';

export class ControlBridge {
  private ws: WebSocket | null = null;
  private controlView: Float32Array;
  private flags: Int32Array;

  constructor(sab: SharedArrayBuffer) {
    this.controlView = new Float32Array(
      sab,
      SAB_OFFSETS.DRONE_CONTROL,
      DRONE_CONSTANTS.MAX_DRONES * 4
    );
    // Flags are at the beginning of SAB (offset 0)
    this.flags = new Int32Array(sab, 0, 1024);
  }

  public connect(url: string) {
    this.ws = new WebSocket(url);
    this.ws.binaryType = 'arraybuffer';

    this.ws.onmessage = event => {
      this.handleMessage(event.data);
    };

    this.ws.onopen = () => {
      console.log('[ControlBridge] Connected to external controller');
    };

    this.ws.onerror = err => {
      console.error('[ControlBridge] WebSocket error:', err);
    };
  }

  private handleMessage(data: ArrayBuffer) {
    // Binary protocol: [droneId: u8, throttle: f32, pitch: f32, roll: f32, yaw: f32]
    // Total 17 bytes -> padded to 20 bytes? Or packed?
    // Python struct.pack('<Bffff', ...) creates 17 bytes.

    // Simplest approach: Use DataView to parse mixed types
    const view = new DataView(data);
    if (data.byteLength < 17) return;

    const droneId = view.getUint8(0);
    if (droneId >= DRONE_CONSTANTS.MAX_DRONES) return;

    const throttle = view.getFloat32(1, true); // little-endian
    const pitch = view.getFloat32(5, true);
    const roll = view.getFloat32(9, true);
    const yaw = view.getFloat32(13, true);

    // Write to SAB
    // Each drone has 4 floats (16 bytes) in control view
    const baseIndex = droneId * 4;
    this.controlView[baseIndex] = throttle;
    this.controlView[baseIndex + 1] = pitch;
    this.controlView[baseIndex + 2] = roll;
    this.controlView[baseIndex + 3] = yaw;

    // Signal epoch to notify Rust (if it was waiting - though Rust physics loop usually polls or waits on its own timer/vsync)
    // Actually, Rust physics runs in compute worker.
    // If Rust is waiting for control updates, we signal.
    // But typically physics runs on a timer (250Hz) and just reads the latest control.
    // However, signaling is good for event-driven architectures.
    Atomics.add(this.flags, EPOCH_INDICES.DRONE_CONTROL, 1);
  }

  public disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}
