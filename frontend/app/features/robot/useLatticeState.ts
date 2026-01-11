import { useEffect, useMemo, useState } from 'react';
import {
  OFFSET_ROBOT_STATE,
  OFFSET_ROBOT_NODES,
  OFFSET_ROBOT_FILAMENTS,
  IDX_ROBOT_EPOCH,
} from '../../../bridge/generated/protocols/schemas/system/v1/sab_layout.consts';

export interface LatticeState {
  phase: number;
  syntropy: number;
  timestamp: number;
}

/**
 * useLatticeState - Zero-Copy Morphic Lattice State Hook
 *
 * Provides reactive access to the topological manifold metrics and binary buffers.
 * Pulsed by IDX_ROBOT_EPOCH (60Hz) from the RobotSupervisor/RobotUnit.
 */
export function useLatticeState() {
  const [metrics, setMetrics] = useState<LatticeState | null>(null);

  const sab = (window as any).__INOS_SAB__;

  // Persistent views for zero-copy access
  const matrices = useMemo(() => {
    if (!sab) return null;
    return new Float32Array(sab, OFFSET_ROBOT_NODES, 512 * 16);
  }, [sab]);

  const filaments = useMemo(() => {
    if (!sab) return null;
    return new Uint32Array(sab, OFFSET_ROBOT_FILAMENTS, 1024 * 2);
  }, [sab]);

  useEffect(() => {
    if (!sab) return;

    let lastEpoch = -1;
    let frameId: number;

    // Create views ONCE to avoid GC pressure
    const flags = new Int32Array(sab, 0, 64);
    const view = new DataView(sab, OFFSET_ROBOT_STATE, 32);

    const poll = () => {
      try {
        const currentEpoch = Atomics.load(flags, IDX_ROBOT_EPOCH);

        if (currentEpoch !== lastEpoch) {
          lastEpoch = currentEpoch;

          setMetrics({
            phase: view.getUint32(8, true),
            syntropy: view.getFloat32(12, true),
            timestamp: Date.now(),
          });
        }
      } catch (e) {
        // SAB detached or out of bounds
      }
      frameId = requestAnimationFrame(poll);
    };

    frameId = requestAnimationFrame(poll);
    return () => cancelAnimationFrame(frameId);
  }, [sab]);

  return {
    metrics,
    matrices,
    filaments,
  };
}
