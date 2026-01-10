/**
 * INOS Technical Codex — Mesh Metrics Component
 *
 * P2P status bar showing operations/sec, nodes, computing capacity.
 * Reads from SAB epochs for real data.
 */

import styled, { css } from 'styled-components';
import { motion } from 'framer-motion';
import { useEffect, useState } from 'react';
import { IDX_BIRD_EPOCH, IDX_MATRIX_EPOCH } from '../../../src/wasm/layout';

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

export function MeshMetricsBar() {
  const [metrics, setMetrics] = useState({
    opsPerSecond: 0,
    nodeCount: 1,
    computeCapacity: 0,
    meshActive: false,
    epochRate: 0,
  });

  useEffect(() => {
    const sab = (window as any).__INOS_SAB__;
    if (!sab) return;

    let lastEpoch = 0;

    const interval = setInterval(() => {
      try {
        const flags = new Int32Array(sab, 0, 32);
        const birdEpoch = Atomics.load(flags, IDX_BIRD_EPOCH);
        const matrixEpoch = Atomics.load(flags, IDX_MATRIX_EPOCH);

        const epochDelta = birdEpoch - lastEpoch;
        lastEpoch = birdEpoch;

        const opsPerSecond = epochDelta * 10 * 10;

        setMetrics({
          opsPerSecond,
          nodeCount: 1,
          computeCapacity: Math.floor((opsPerSecond + matrixEpoch * 0.001) / 1000),
          meshActive: epochDelta > 0,
          epochRate: epochDelta * 10,
        });
      } catch {
        // SAB not ready
      }
    }, 100);

    return () => clearInterval(interval);
  }, []);

  return (
    <Style.MetricsBar initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={{ delay: 1 }}>
      <Style.Metric>
        <Style.PulseIndicator $active={metrics.meshActive} />
        <Style.Label>Mesh</Style.Label>
        <Style.Value>{metrics.meshActive ? 'LIVE' : 'SYNC'}</Style.Value>
      </Style.Metric>

      <Style.Divider />

      <Style.Metric>
        <Style.Label>Ops/s</Style.Label>
        <Style.Value>
          <RollingCounter value={metrics.opsPerSecond} />
        </Style.Value>
      </Style.Metric>

      <Style.Metric>
        <Style.Label>Capacity</Style.Label>
        <Style.Value>
          <RollingCounter value={metrics.computeCapacity} suffix=" GFLOPS" />
        </Style.Value>
      </Style.Metric>

      <Style.Divider />

      <Style.Metric>
        <Style.Label>Nodes</Style.Label>
        <Style.Value>
          <RollingCounter value={metrics.nodeCount} />
        </Style.Value>
      </Style.Metric>

      <Style.Metric>
        <Style.Label>Temporal Sync</Style.Label>
        <Style.Value>
          <RollingCounter value={metrics.epochRate} suffix=" Hz" />
        </Style.Value>
      </Style.Metric>
    </Style.MetricsBar>
  );
}

export default MeshMetricsBar;
