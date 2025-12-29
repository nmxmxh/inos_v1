import { useEffect, useState } from 'react';
import { useSystemStore } from '../../src/store/system';

export default function PerformanceHUD() {
  const { units } = useSystemStore();
  const [fps, setFps] = useState(60);
  const [nodes, setNodes] = useState(1);

  // Latent variables based on nodes (1 Node = 4GB, 10 TFLOPS)
  // These are "Capacity" metrics unless we have real data
  const totalMemoryGB = nodes * 4;
  const totalTFLOPS = nodes * 10;
  const globalOps = nodes * 1_500_000;

  useEffect(() => {
    const interval = setInterval(() => {
      // Simulate slight FPS fluctuation
      setFps(prev => Math.max(55, Math.min(62, prev + (Math.random() - 0.5) * 2)));
    }, 1000);

    setNodes(Object.keys(units).length || 1);

    return () => clearInterval(interval);
  }, [units]);

  // Try to use real memory usage if available (Chrome only), otherwise fallback to latent
  const realMemoryBytes = (performance as any).memory?.usedJSHeapSize;
  const realMemoryDisplay = realMemoryBytes
    ? (realMemoryBytes / 1024 / 1024 / 1024).toFixed(3) // Shows e.g. 0.125 GB
    : null;

  return (
    <div className="status-bar">
      <div className="status-item">
        <span className="status-label">ACTIVE NODES</span>
        <span className="status-value">{nodes}</span>
      </div>
      <div className="status-separator" />
      <div className="status-item">
        <span className="status-label">GLOBAL MEM</span>
        <span className="status-value">{realMemoryDisplay || totalMemoryGB}</span>
        <span className="status-unit">GB</span>
      </div>
      <div className="status-separator" />
      <div className="status-item">
        <span className="status-label">LATENT COMPUTE</span>
        <span className="status-value">{totalTFLOPS}</span>
        <span className="status-unit">TFLOPS</span>
      </div>
      <div className="status-separator" />
      <div className="status-item">
        <span className="status-label">MESH OPS</span>
        <span className="status-value">{(globalOps / 1_000_000).toFixed(1)}</span>
        <span className="status-unit">M/s</span>
      </div>
      <div style={{ flex: 1 }} /> {/* Spacer */}
      <div className="status-item">
        <span className="status-label">LOCAL</span>
        <span className="status-value">{fps.toFixed(0)}</span>
        <span className="status-unit">FPS</span>
      </div>
    </div>
  );
}
