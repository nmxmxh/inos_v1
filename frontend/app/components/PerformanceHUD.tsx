import { useEffect, useState } from 'react';
import { useSystemStore } from '../../src/store/system';

export default function PerformanceHUD() {
  const { units, stats } = useSystemStore();
  const [nodes, setNodes] = useState(1);

  // Calculate total capability power
  const totalCapabilities = Object.values(units).reduce(
    (acc, unit) => acc + unit.capabilities.length,
    0
  );
  const powerTFLOPS = (nodes * 12.5 + totalCapabilities * 0.5).toFixed(1);
  const meshOps = (nodes * 1.8 + totalCapabilities * 0.2).toFixed(1);

  useEffect(() => {
    setNodes(Object.keys(units).length || 1);
  }, [units]);

  // Try to use real memory usage if available (Chrome only)
  const realMemoryBytes = (performance as any).memory?.usedJSHeapSize;
  const realMemoryDisplay = realMemoryBytes
    ? (realMemoryBytes / 1024 / 1024 / 1024).toFixed(3)
    : (nodes * 0.512).toFixed(3); // 512MB per node base

  return (
    <div className="status-bar">
      <div className="status-item">
        <span className="status-label">ACTIVE NODES</span>
        <span className="status-value">{nodes}</span>
      </div>
      <div className="status-separator" />
      <div className="status-item">
        <span className="status-label">GLOBAL MEM</span>
        <span className="status-value">{realMemoryDisplay}</span>
        <span className="status-unit">GB</span>
      </div>
      <div className="status-separator" />
      <div className="status-item">
        <span className="status-label">LATENT COMPUTE</span>
        <span className="status-value">{powerTFLOPS}</span>
        <span className="status-unit">TFLOPS</span>
      </div>
      <div className="status-separator" />
      <div className="status-item">
        <span className="status-label">MESH OPS</span>
        <span className="status-value">{meshOps}</span>
        <span className="status-unit">M/s</span>
      </div>
      <div style={{ flex: 1 }} /> {/* Spacer */}
      <div className="status-item">
        <span className="status-label">NETWORK</span>
        <span className="status-value">P2P-SAB</span>
        <span className="status-unit">FAST</span>
      </div>
      <div className="status-separator" />
      <div className="status-item">
        <span className="status-label">LOCAL</span>
        <span className="status-value">{stats.fps.toFixed(0)}</span>
        <span className="status-unit">FPS</span>
      </div>
    </div>
  );
}
