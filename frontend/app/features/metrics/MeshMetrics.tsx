/**
 * INOS Technical Codex — Mesh Metrics Component
 *
 * P2P status bar showing operations/sec, nodes, computing capacity.
 * Reads from SAB epochs for real data.
 */

import styled, { css } from 'styled-components';
import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { Link } from 'react-router-dom';
import { IDX_BIRD_EPOCH } from '../../../src/wasm/layout';
import { INOSBridge } from '../../../src/wasm/bridge-state';

const Style = {
  MetricsBar: styled(motion.div)`
    display: flex;
    align-items: center;
    justify-content: center;
    flex-wrap: wrap;
    gap: ${p => p.theme.spacing[10]};
    row-gap: ${p => p.theme.spacing[3]};
    padding: 0 ${p => p.theme.spacing[4]};
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 9px;
    color: ${p => p.theme.colors.inkMedium};
    letter-spacing: 0.08em;
    width: 100%;
    max-width: ${p => p.theme.layout.maxWidth};

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      gap: ${p => p.theme.spacing[4]};
      font-size: 8px;
    }
  `,

  Metric: styled.div`
    display: flex;
    align-items: center;
    gap: ${p => p.theme.spacing[2]};
  `,

  Label: styled.span`
    text-transform: uppercase;
    font-weight: ${p => p.theme.fontWeights.semibold};
    opacity: 0.85;
    font-size: 8px;
    margin-top: 1px;
  `,

  Value: styled.span`
    font-weight: ${p => p.theme.fontWeights.bold};
    color: ${p => p.theme.colors.inkDark};
    font-feature-settings: 'tnum';
    font-size: 10px;
    line-height: 1;
  `,

  PulseIndicator: styled.div<{ $active: boolean; $health?: 'good' | 'fair' | 'poor' }>`
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: ${p => {
      if (!p.$active) return p.theme.colors.inkFaded;
      if (p.$health === 'good') return p.theme.colors.success;
      if (p.$health === 'fair') return p.theme.colors.warning;
      if (p.$health === 'poor') return p.theme.colors.error;
      return p.theme.colors.accent;
    }};
    position: relative;

    ${p =>
      p.$active &&
      css`
        &::after {
          content: '';
          position: absolute;
          top: -2px;
          left: -2px;
          right: -2px;
          bottom: -2px;
          border-radius: 50%;
          border: 1px solid
            ${p.$health === 'good'
              ? p.theme.colors.success
              : p.$health === 'fair'
                ? p.theme.colors.warning
                : p.$health === 'poor'
                  ? p.theme.colors.error
                  : p.theme.colors.accent};
          animation: pulse 2s infinite;
        }
      `}

    @keyframes pulse {
      0% {
        transform: scale(1);
        opacity: 0.8;
      }
      100% {
        transform: scale(2.5);
        opacity: 0;
      }
    }
  `,

  Divider: styled.div`
    width: 1px;
    height: 12px;
    background: ${p => p.theme.colors.borderSubtle};
    @media (max-width: ${p => p.theme.breakpoints.md}) {
      display: none;
    }
  `,
};

import RollingCounter from '../../ui/RollingCounter';
import { useMeshMetrics } from './useMeshMetrics';
import { useGlobalAnalytics } from '../analytics/useGlobalAnalytics';
import NumberFormatter from '../../ui/NumberFormatter';

const OPS_PER_ENTITY = 2200;
const ENTITY_COUNT = 1000;
const EMA_ALPHA = 0.2;

function useLocalKernelStats() {
  const [stats, setStats] = useState({
    opsPerSecond: 0,
    epochRate: 0,
  });

  useEffect(() => {
    let lastEpoch = 0;
    let lastTime = performance.now();
    let smoothedRate = 0;

    const interval = setInterval(() => {
      try {
        if (!INOSBridge.isReady()) return;

        const epoch = INOSBridge.atomicLoad(IDX_BIRD_EPOCH);
        const now = performance.now();
        const deltaEpoch = Math.max(0, epoch - lastEpoch);
        const deltaTime = Math.max(0.001, (now - lastTime) / 1000);
        lastEpoch = epoch;
        lastTime = now;

        const instantRate = deltaEpoch / deltaTime;
        smoothedRate = smoothedRate
          ? smoothedRate * (1 - EMA_ALPHA) + instantRate * EMA_ALPHA
          : instantRate;

        setStats({
          epochRate: smoothedRate,
          opsPerSecond: smoothedRate * ENTITY_COUNT * OPS_PER_ENTITY,
        });
      } catch {
        // SAB not ready
      }
    }, 250);

    return () => clearInterval(interval);
  }, []);

  return stats;
}

export function MeshMetricsBar() {
  const metrics = useMeshMetrics();
  const global = useGlobalAnalytics();
  const local = useLocalKernelStats();

  const displayMetrics = metrics || {
    opsPerSecond: 0,
    nodeCount: 1,
    meshActive: false,
    p50Latency: 0,
    connectedPeers: 0,
    avgReputation: 0.95,
    gossipRate: 0,
    sectorId: 0,
    successRate: 1.0,
  };

  // Derived metrics (Prioritize Global if available, fallback to local estimation)
  const opsPerSecond = global?.globalOpsPerSec
    ? Number(global.globalOpsPerSec)
    : metrics
      ? Math.floor(metrics.gossipRate * 100)
      : 0;

  const activeNodes = global?.activeNodeCount || displayMetrics.connectedPeers || 1;
  const totalComputeGFLOPS = global?.totalComputeGFLOPS ?? 0;
  const avgCapability = global?.avgCapability ?? 0;
  const sectorId = displayMetrics.sectorId ?? 0;
  const avgReputation = displayMetrics.avgReputation ?? 0;

  // Mesh Health logic
  const successRate = displayMetrics.successRate || 1.0;
  const p50 = displayMetrics.p50Latency || 0;
  const healthStatus: 'good' | 'fair' | 'poor' =
    successRate > 0.98 && p50 < 100 ? 'good' : successRate > 0.9 ? 'fair' : 'poor';

  return (
    <Style.MetricsBar
      data-testid="mesh-metrics-bar"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ delay: 1 }}
    >
      <Style.Metric
        data-testid="metric-mesh"
        title="Mesh Connection Status — Green: Healthy, Yellow: Degraded, Red: Critical"
      >
        <Link
          to="/diagnostics"
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            textDecoration: 'none',
            color: 'inherit',
          }}
        >
          <Style.PulseIndicator
            $active={!!global || displayMetrics.meshActive}
            $health={healthStatus}
          />
          <Style.Label>Mesh</Style.Label>
          <Style.Value>{global || displayMetrics.meshActive ? 'LIVE' : 'SYNC'}</Style.Value>
        </Link>
      </Style.Metric>

      <Style.Divider />

      <Style.Metric
        data-testid="metric-node-ops"
        title="Local Throughput — Estimated kernel operations per second on this node (epoch rate × agents × micro-ops)"
      >
        <Style.Label>Node Ops/s</Style.Label>
        <NumberFormatter value={local.opsPerSecond} />
      </Style.Metric>

      <Style.Metric
        data-testid="metric-ops"
        title="Total Network Throughput — Aggregated operations per second across the entire mesh"
      >
        <Style.Label>Net Ops/s</Style.Label>
        <NumberFormatter value={opsPerSecond} />
      </Style.Metric>

      <Style.Metric
        data-testid="metric-cap"
        title="Total Compute Power — Combined compute capacity across active nodes"
      >
        <Style.Label>Cap</Style.Label>
        <NumberFormatter value={totalComputeGFLOPS} suffix="G" />
      </Style.Metric>

      <Style.Metric
        data-testid="metric-avg"
        title="Average Capability — Mean compute capacity per active node"
      >
        <Style.Label>Avg</Style.Label>
        <NumberFormatter value={avgCapability} suffix="G" />
      </Style.Metric>

      <Style.Metric
        data-testid="metric-nodes"
        title="Participating Nodes — Number of independent devices currently collaborating in your regional mesh"
      >
        <Style.Label>Nodes</Style.Label>
        <NumberFormatter value={activeNodes} decimals={0} />
      </Style.Metric>

      <Style.Metric data-testid="metric-sector" title="Sector ID — Your current mesh partition">
        <Style.Label>Sector</Style.Label>
        <NumberFormatter value={sectorId} decimals={0} />
      </Style.Metric>

      <Style.Metric data-testid="metric-rep" title="Global Trust — Aggregate reliability score">
        <Style.Label>Trust</Style.Label>
        <Style.Value>
          <RollingCounter value={avgReputation} decimals={2} />
        </Style.Value>
      </Style.Metric>

      <Style.Metric
        data-testid="metric-lat"
        title="Network Latency — Circular trip time for data packets. Lower is better (0-100ms is excellent)"
      >
        <Style.Label>Lat</Style.Label>
        <Style.Value>
          <RollingCounter value={Math.floor(displayMetrics.p50Latency || 0)} suffix="ms" />
        </Style.Value>
      </Style.Metric>
    </Style.MetricsBar>
  );
}

export default MeshMetricsBar;
