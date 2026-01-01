import { useEffect, useState } from 'react';
import { useSystemStore } from '../../src/store/system';

interface KernelMetrics {
  fps: number;
  memory: number;
  nodes: number;
  epoch: number;
  meshNodes: number;
  sectorId: number;
  avgLatency: number;
  bytesTransferred: number;
}

export default function PerformanceHUD() {
  const { units, stats } = useSystemStore();
  const [metrics, setMetrics] = useState<KernelMetrics>({
    fps: 0,
    memory: 0,
    nodes: 0,
    epoch: 0,
    meshNodes: 0,
    sectorId: 0,
    avgLatency: 0,
    bytesTransferred: 0,
  });

  useEffect(() => {
    const interval = setInterval(() => {
      const sab = (window as any).__INOS_SAB__;
      const activeNodes = Object.values(units).filter(u => u.active).length;

      // Real memory if available (Chrome)
      const realMemoryBytes = (performance as any).memory?.usedJSHeapSize;
      const memoryMB = realMemoryBytes
        ? (realMemoryBytes / 1024 / 1024).toFixed(1)
        : (activeNodes * 16).toFixed(1);

      // Get kernel stats if available
      let kernelStats: any = {};
      try {
        const getKernelStats = (window as any).getKernelStats;
        if (typeof getKernelStats === 'function') {
          kernelStats = getKernelStats();
        }
      } catch (e) {
        // Kernel stats not available yet
      }

      // Read epoch from SAB (multiple epoch indices)
      let epoch = 0;
      let meshNodes = 0;
      let sectorId = 0;
      let avgLatency = 0;
      let bytesTransferred = 0;

      if (sab && sab instanceof SharedArrayBuffer) {
        try {
          const flags = new Int32Array(sab, 0, 256);

          // Read all epoch counters and find the highest
          const kernelEpoch = Atomics.load(flags, 0); // IDX_KERNEL_READY
          const sensorEpoch = Atomics.load(flags, 4); // IDX_SENSOR_EPOCH
          const actorEpoch = Atomics.load(flags, 5); // IDX_ACTOR_EPOCH
          const storageEpoch = Atomics.load(flags, 6); // IDX_STORAGE_EPOCH
          const systemEpoch = Atomics.load(flags, 7); // IDX_SYSTEM_EPOCH

          // Use the highest epoch as the current system epoch
          epoch = Math.max(kernelEpoch, sensorEpoch, actorEpoch, storageEpoch, systemEpoch);

          // Read mesh coordinator stats from SAB if available
          // These would be written by the Go kernel at a known offset
          // For now, use kernel stats if available
          if (kernelStats.meshNodes !== undefined) {
            meshNodes = kernelStats.meshNodes || 1;
            sectorId = kernelStats.sectorId || 0;
            avgLatency = kernelStats.avgLatency || 0;
            bytesTransferred = kernelStats.bytesTransferred || 0;
          }
        } catch (e) {
          console.warn('[PerformanceHUD] Failed to read from SAB:', e);
        }
      }

      setMetrics({
        fps: stats.fps || 60,
        memory: parseFloat(memoryMB),
        nodes: activeNodes,
        epoch,
        meshNodes,
        sectorId,
        avgLatency,
        bytesTransferred,
      });
    }, 500);

    return () => clearInterval(interval);
  }, [units, stats]);

  return (
    <div
      style={{
        position: 'fixed',
        bottom: '40px',
        left: '50%',
        transform: 'translateX(-50%)',
        display: 'inline-flex',
        flexDirection: 'column',
        gap: '8px',
        padding: '16px 24px',
        background: 'rgba(255,255,255,0.98)',
        backdropFilter: 'blur(10px)',
        border: '2px solid #000',
        borderRadius: '0',
        fontFamily: 'monospace',
        fontSize: '11px',
        boxShadow: '6px 6px 0 #000',
        zIndex: 100,
      }}
    >
      {/* Title */}
      <div
        style={{
          fontSize: '10px',
          fontWeight: 700,
          letterSpacing: '2px',
          color: '#000',
          borderBottom: '2px solid #000',
          paddingBottom: '8px',
          marginBottom: '4px',
        }}
      >
        KERNEL DIAGNOSTICS
      </div>

      {/* Main metrics row */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
        {/* FPS */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <div style={{ color: '#666', fontSize: '10px' }}>FPS</div>
          <div
            style={{
              color: metrics.fps > 55 ? '#10b981' : metrics.fps > 30 ? '#f59e0b' : '#ef4444',
              fontWeight: 700,
              fontSize: '14px',
            }}
          >
            {metrics.fps.toFixed(0)}
          </div>
        </div>

        <div style={{ width: '1px', height: '16px', background: '#ccc' }} />

        {/* Memory */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <div style={{ color: '#666', fontSize: '10px' }}>MEM</div>
          <div style={{ color: '#3b82f6', fontWeight: 700, fontSize: '13px' }}>
            {metrics.memory}
            <span style={{ fontSize: '9px', color: '#999', marginLeft: '2px' }}>MB</span>
          </div>
        </div>

        <div style={{ width: '1px', height: '16px', background: '#ccc' }} />

        {/* Nodes */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <div style={{ color: '#666', fontSize: '10px' }}>MODULES</div>
          <div style={{ color: '#8b5cf6', fontWeight: 700, fontSize: '13px' }}>{metrics.nodes}</div>
        </div>

        <div style={{ width: '1px', height: '16px', background: '#ccc' }} />

        {/* Epoch */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <div style={{ color: '#666', fontSize: '10px' }}>EPOCH</div>
          <div
            style={{
              color: metrics.epoch > 0 ? '#ec4899' : '#999',
              fontWeight: 700,
              fontSize: '13px',
            }}
          >
            {metrics.epoch > 0 ? metrics.epoch.toLocaleString() : '—'}
          </div>
        </div>

        <div style={{ width: '1px', height: '16px', background: '#ccc' }} />

        {/* Status */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <div
            style={{
              width: '8px',
              height: '8px',
              borderRadius: '50%',
              background: '#10b981',
              boxShadow: '0 0 8px rgba(16, 185, 129, 0.6)',
              animation: 'pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite',
            }}
          />
          <div
            style={{
              color: '#10b981',
              fontWeight: 700,
              fontSize: '11px',
              letterSpacing: '1.5px',
            }}
          >
            LIVE
          </div>
        </div>
      </div>

      {/* Mesh metrics row (if available) */}
      {metrics.meshNodes > 0 && (
        <>
          <div style={{ height: '1px', background: '#e5e5e5', margin: '4px 0' }} />
          <div style={{ display: 'flex', alignItems: 'center', gap: '16px', fontSize: '10px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
              <span style={{ color: '#666' }}>MESH:</span>
              <span style={{ color: '#000', fontWeight: 600 }}>{metrics.meshNodes} nodes</span>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
              <span style={{ color: '#666' }}>SECTOR:</span>
              <span style={{ color: '#000', fontWeight: 600 }}>#{metrics.sectorId}</span>
            </div>
            {metrics.avgLatency > 0 && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                <span style={{ color: '#666' }}>LATENCY:</span>
                <span style={{ color: '#000', fontWeight: 600 }}>
                  {metrics.avgLatency.toFixed(1)}ms
                </span>
              </div>
            )}
            {metrics.bytesTransferred > 0 && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                <span style={{ color: '#666' }}>TX:</span>
                <span style={{ color: '#000', fontWeight: 600 }}>
                  {(metrics.bytesTransferred / 1024).toFixed(1)}KB
                </span>
              </div>
            )}
          </div>
        </>
      )}

      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.5; }
        }
      `}</style>
    </div>
  );
}
