import { useEffect, useState } from 'react';
import { OFFSET_GLOBAL_ANALYTICS, IDX_GLOBAL_METRICS_EPOCH } from '../../../src/wasm/layout';

export interface GlobalAnalytics {
  totalStorageBytes: bigint;
  totalComputeGFLOPS: bigint;
  globalOpsPerSec: bigint;
  activeNodeCount: number;
  timestamp: number;
}

/**
 * useGlobalAnalytics - Zero-Copy Mesh Telemetry Hook
 *
 * Pulsed by IDX_GLOBAL_METRICS_EPOCH from the AnalyticsSupervisor.
 * Reads aggregated cross-chain/cross-mesh metrics directly from the SAB
 * without intermediate JSON serialization.
 */
export function useGlobalAnalytics() {
  const [data, setData] = useState<GlobalAnalytics | null>(null);

  useEffect(() => {
    const sab = (window as any).__INOS_SAB__;
    if (!sab) return;

    let lastEpoch = -1;

    const poll = () => {
      try {
        const flags = new Int32Array(sab, 0, 64);
        const currentEpoch = Atomics.load(flags, IDX_GLOBAL_METRICS_EPOCH);

        if (currentEpoch === lastEpoch) return;
        lastEpoch = currentEpoch;

        // Structure: [TotalStorage(8), TotalCompute(8), GlobalOps(8), NodeCount(4)] (28 bytes)
        const view = new DataView(sab, OFFSET_GLOBAL_ANALYTICS, 32);

        setData({
          totalStorageBytes: view.getBigUint64(0, true),
          totalComputeGFLOPS: view.getBigUint64(8, true),
          globalOpsPerSec: view.getBigUint64(16, true),
          activeNodeCount: view.getUint32(24, true),
          timestamp: Date.now(),
        });
      } catch (e) {
        // SAB not ready or out of bounds
      }
    };

    const interval = setInterval(poll, 500); // 2Hz is plenty for global aggregation
    return () => clearInterval(interval);
  }, []);

  return data;
}
