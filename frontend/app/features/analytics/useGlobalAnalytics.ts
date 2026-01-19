import { useEffect, useState } from 'react';
import { OFFSET_GLOBAL_ANALYTICS, IDX_GLOBAL_METRICS_EPOCH } from '../../../src/wasm/layout';
import { INOSBridge } from '../../../src/wasm/bridge-state';

export interface GlobalAnalytics {
  totalStorageBytes: bigint;
  totalComputeGFLOPS: number;
  globalOpsPerSec: number;
  activeNodeCount: number;
  avgCapability: number; // Computed: totalCompute / nodes
  timestamp: number;
}

/**
 * useGlobalAnalytics - Zero-Allocation Mesh Telemetry Hook
 *
 * Uses INOSBridge cached views for zero-allocation SAB reads.
 * Pulsed by IDX_GLOBAL_METRICS_EPOCH from the AnalyticsSupervisor.
 */
export function useGlobalAnalytics() {
  const [data, setData] = useState<GlobalAnalytics | null>(null);

  useEffect(() => {
    let lastEpoch = -1;

    const poll = () => {
      try {
        // Use INOSBridge for zero-allocation reads
        if (!INOSBridge.isReady()) return;

        const currentEpoch = INOSBridge.atomicLoad(IDX_GLOBAL_METRICS_EPOCH);

        if (currentEpoch === lastEpoch) return;
        lastEpoch = currentEpoch;

        // Get cached DataView for global analytics region
        const view = INOSBridge.getRegionDataView(OFFSET_GLOBAL_ANALYTICS, 32);
        if (!view) return;

        const totalStorageBytes = view.getBigUint64(0, true);
        const totalComputeGFLOPS = Number(view.getBigUint64(8, true));
        const globalOpsPerSec = Number(view.getBigUint64(16, true));
        const activeNodeCount = view.getUint32(24, true);

        setData({
          totalStorageBytes,
          totalComputeGFLOPS,
          globalOpsPerSec,
          activeNodeCount,
          avgCapability: activeNodeCount > 0 ? totalComputeGFLOPS / activeNodeCount : 0,
          timestamp: Date.now(),
        });
      } catch {
        // SAB not ready or out of bounds
      }
    };

    const interval = setInterval(poll, 500); // 2Hz is plenty for global aggregation
    return () => clearInterval(interval);
  }, []);

  return data;
}
