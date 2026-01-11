import { useEffect, useState } from 'react';
import { OFFSET_MESH_METRICS, IDX_METRICS_EPOCH } from '../../../src/wasm/layout';

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

export function useMeshMetrics() {
  const [metrics, setMetrics] = useState<MeshMetrics | null>(null);

  useEffect(() => {
    const sab = (window as any).__INOS_SAB__;
    if (!sab) return;

    let lastEpoch = -1;
    let flagsView: Int32Array | null = null;
    let dataView: DataView | null = null;

    const interval = setInterval(() => {
      try {
        if (!flagsView || flagsView.buffer !== sab) {
          flagsView = new Int32Array(sab, 0, 32);
          dataView = new DataView(sab, OFFSET_MESH_METRICS, 256);
        }

        const currentEpoch = Atomics.load(flagsView, IDX_METRICS_EPOCH);

        if (Number(currentEpoch) === lastEpoch) return;
        lastEpoch = Number(currentEpoch);

        // Read metrics from SAB
        const view = dataView!;

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
      } catch (e) {
        // SAB not ready or out of bounds
      }
    }, 200);

    return () => clearInterval(interval);
  }, []);

  return metrics;
}
