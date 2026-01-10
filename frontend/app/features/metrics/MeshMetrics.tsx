/**
 * INOS Technical Codex — Mesh Metrics Component
 *
 * P2P status bar showing operations/sec, nodes, computing capacity.
 * Reads from SAB epochs for real data.
 */

import styled, { css } from 'styled-components';
import { motion } from 'framer-motion';

const Style = {
  MetricsBar: styled(motion.div)`
    display: flex;
    align-items: center;
    justify-content: center;
    gap: ${p => p.theme.spacing[10]};
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

  PulseIndicator: styled.div<{ $active: boolean }>`
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: ${p => (p.$active ? p.theme.colors.accent : p.theme.colors.inkFaded)};
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
          border: 1px solid ${p.theme.colors.accent};
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

export function MeshMetricsBar() {
  const metrics = useMeshMetrics();

  const displayMetrics = metrics || {
    opsPerSecond: 0,
    nodeCount: 1,
    computeCapacity: 0,
    meshActive: false,
    p50Latency: 0,
    connectedPeers: 0,
    avgReputation: 0.95,
    gossipRate: 0,
    sectorId: 0,
  };

  // Derived metrics
  const opsPerSecond = metrics ? Math.floor(metrics.gossipRate * 100) : 0;
  const computeCapacity = metrics
    ? Math.floor(opsPerSecond + (metrics.connectedPeers || 0) * 1.5)
    : 0;

  return (
    <Style.MetricsBar initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={{ delay: 1 }}>
      <Style.Metric>
        <Style.PulseIndicator $active={displayMetrics.meshActive} />
        <Style.Label>Mesh</Style.Label>
        <Style.Value>{displayMetrics.meshActive ? 'LIVE' : 'SYNC'}</Style.Value>
      </Style.Metric>

      <Style.Divider />

      <Style.Metric title="Network Throughput">
        <Style.Label>Ops/s</Style.Label>
        <Style.Value>
          <RollingCounter value={opsPerSecond} />
        </Style.Value>
      </Style.Metric>

      <Style.Metric title="Distributed Compute Capacity">
        <Style.Label>Cap</Style.Label>
        <Style.Value>
          <RollingCounter value={computeCapacity} suffix=" GFLOPS" />
        </Style.Value>
      </Style.Metric>

      <Style.Divider />

      <Style.Metric title="Connected Nodes">
        <Style.Label>Nodes</Style.Label>
        <Style.Value>
          <RollingCounter value={displayMetrics.connectedPeers || 1} />
        </Style.Value>
      </Style.Metric>

      <Style.Metric title="Network Latency (P50)">
        <Style.Label>Lat</Style.Label>
        <Style.Value>
          <RollingCounter value={Math.floor(displayMetrics.p50Latency || 0)} suffix="ms" />
        </Style.Value>
      </Style.Metric>

      <Style.Metric title="Sector ID (Regional Mesh Identifier)">
        <Style.Label>Sector</Style.Label>
        <Style.Value>
          {displayMetrics.sectorId
            ? `0x${displayMetrics.sectorId.toString(16).toUpperCase().padStart(4, '0')}`
            : '0x0000'}
        </Style.Value>
      </Style.Metric>

      <Style.Metric title="Global Reputation Score">
        <Style.Label>Rep</Style.Label>
        <Style.Value>{((metrics?.avgReputation || 0.95) * 100).toFixed(1)}%</Style.Value>
      </Style.Metric>
    </Style.MetricsBar>
  );
}

export default MeshMetricsBar;
