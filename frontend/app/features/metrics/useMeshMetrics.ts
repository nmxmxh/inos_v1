import { useEffect, useState } from 'react';
import { OFFSET_MESH_METRICS, IDX_METRICS_EPOCH } from '../../../src/wasm/layout';
import { INOSBridge } from '../../../src/wasm/bridge-state';

export interface MeshMetrics {
  totalPeers: number;
  connectedPeers: number;
  dhtEntries: number;
  gossipRate: number;
  avgReputation: number;
  bytesSent: bigint;
  bytesReceived: bigint;
  p50Latency: number;
  p95Latency: number;
  successRate: number;
  fetchSuccessRate: number;
  localChunks: number;
  totalChunks: number;
  sectorId: number;
  meshActive: boolean;
}

/**
 * useMeshMetrics - Zero-Allocation Mesh Telemetry Hook
 *
 * Uses INOSBridge cached views for zero-allocation SAB reads.
 * Reads mesh metrics directly from SAB without per-tick TypedArray creation.
 */
export function useMeshMetrics() {
  const [metrics, setMetrics] = useState<MeshMetrics | null>(null);

  useEffect(() => {
    let lastEpoch = -1;

    const interval = setInterval(() => {
      try {
        // Use INOSBridge for zero-allocation reads
        if (!INOSBridge.isReady()) return;

        const currentEpoch = INOSBridge.atomicLoad(IDX_METRICS_EPOCH);

        if (currentEpoch === lastEpoch) return;
        lastEpoch = currentEpoch;

        // Get cached DataView for mesh metrics region
        const view = INOSBridge.getRegionDataView(OFFSET_MESH_METRICS, 256);
        if (!view) return;

        setMetrics({
          totalPeers: view.getUint32(0, true),
          connectedPeers: view.getUint32(4, true),
          dhtEntries: view.getUint32(8, true),
          gossipRate: view.getFloat32(12, true),
          avgReputation: view.getFloat32(16, true),
          bytesSent: view.getBigUint64(24, true),
          bytesReceived: view.getBigUint64(32, true),
          p50Latency: view.getFloat32(40, true),
          p95Latency: view.getFloat32(44, true),
          successRate: view.getFloat32(48, true),
          fetchSuccessRate: view.getFloat32(52, true),
          localChunks: view.getUint32(56, true),
          totalChunks: view.getUint32(60, true),
          sectorId: view.getUint32(20, true),
          meshActive: true,
        });
      } catch {
        // SAB not ready or out of bounds
      }
    }, 200);

    return () => clearInterval(interval);
  }, []);

  return metrics;
}
