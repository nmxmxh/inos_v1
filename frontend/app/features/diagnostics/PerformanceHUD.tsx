/**
 * INOS Technical Codex — Performance HUD
 *
 * Real-time metrics display reading from SharedArrayBuffer epochs.
 * Zero-polling architecture using direct SAB reads.
 */

import styled from 'styled-components';
import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import {
  IDX_BIRD_EPOCH,
  IDX_MATRIX_EPOCH,
  IDX_METRICS_EPOCH,
  OFFSET_BRIDGE_METRICS,
} from '../../../src/wasm/layout';

const HudContainer = styled(motion.div)`
  display: flex;
  align-items: center;
  gap: ${p => p.theme.spacing[6]};
  padding: ${p => p.theme.spacing[2]} ${p => p.theme.spacing[4]};
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(8px);
  border: 1px solid ${p => p.theme.colors.borderSubtle};
  font-family: ${p => p.theme.fonts.typewriter};
  font-size: ${p => p.theme.fontSizes.xs};
`;

const Metric = styled.div`
  display: flex;
  align-items: baseline;
  gap: ${p => p.theme.spacing[2]};
`;

const Label = styled.span`
  color: ${p => p.theme.colors.inkLight};
  text-transform: uppercase;
  letter-spacing: 0.05em;
`;

const Value = styled.span`
  color: ${p => p.theme.colors.accent};
  font-weight: ${p => p.theme.fontWeights.bold};
`;

const StatusDot = styled.span<{ $active: boolean }>`
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: ${p => (p.$active ? p.theme.colors.success : p.theme.colors.inkFaded)};
  transition: background 0.2s ease;
`;

interface Metrics {
  birdEpoch: number;
  matrixEpoch: number;
  metricsEpoch: number;
  active: boolean;
  bridge: {
    hits: number;
    misses: number;
    readNs: number;
    writeNs: number;
    health: number;
  };
}

interface PerformanceHUDProps {
  compact?: boolean;
}

export function PerformanceHUD({ compact = false }: PerformanceHUDProps) {
  const [metrics, setMetrics] = useState<Metrics>({
    birdEpoch: 0,
    matrixEpoch: 0,
    metricsEpoch: 0,
    active: false,
    bridge: { hits: 0, misses: 0, readNs: 0, writeNs: 0, health: 100 },
  });

  useEffect(() => {
    const sab = (window as any).__INOS_SAB__;
    if (!sab) return;

    let lastBirdEpoch = 0;

    const interval = setInterval(() => {
      try {
        const flags = new Int32Array(sab, 0, 32);
        const birdEpoch = Atomics.load(flags, IDX_BIRD_EPOCH);
        const matrixEpoch = Atomics.load(flags, IDX_MATRIX_EPOCH);
        const metricsEpoch = Atomics.load(flags, IDX_METRICS_EPOCH);

        // Read bridge metrics (32 bytes = 4 x BigUint64)
        const metricsView = new BigUint64Array(sab, OFFSET_BRIDGE_METRICS, 4);
        const hits = Number(Atomics.load(metricsView, 0));
        const misses = Number(Atomics.load(metricsView, 1));
        const readNs = Number(Atomics.load(metricsView, 2));
        const writeNs = Number(Atomics.load(metricsView, 3));

        const total = hits + misses;
        const health = total > 0 ? (hits / total) * 100 : 100;

        // Active if epoch changed since last check
        const active = birdEpoch !== lastBirdEpoch;
        lastBirdEpoch = birdEpoch;

        setMetrics({
          birdEpoch,
          matrixEpoch,
          metricsEpoch,
          active,
          bridge: { hits, misses, readNs, writeNs, health },
        });
      } catch (e) {
        // SAB not ready or invalid
      }
    }, 100);

    return () => clearInterval(interval);
  }, []);

  if (compact) {
    return (
      <HudContainer initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={{ delay: 0.5 }}>
        <StatusDot
          $active={metrics.active}
          aria-label={metrics.active ? 'System active' : 'System idle'}
        />
        <Value>{metrics.birdEpoch}</Value>
      </HudContainer>
    );
  }

  return (
    <HudContainer
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: 0.5 }}
    >
      <Metric>
        <StatusDot $active={metrics.active} aria-hidden="true" />
        <Label>Status</Label>
        <Value>{metrics.active ? 'Active' : 'Idle'}</Value>
      </Metric>

      <Metric>
        <Label>Bird Epoch</Label>
        <Value>{metrics.birdEpoch}</Value>
      </Metric>

      <Metric>
        <Label>Matrix Epoch</Label>
        <Value>{metrics.matrixEpoch}</Value>
      </Metric>

      <Metric title={`Bridge Health: ${metrics.bridge.health.toFixed(1)}%`}>
        <StatusDot $active={metrics.bridge.health > 90} />
        <Label>Bridge</Label>
        <Value>
          {metrics.bridge.hits}/{metrics.bridge.misses}
        </Value>
      </Metric>

      <Metric>
        <Label>I/O Latency</Label>
        <Value>{(metrics.bridge.readNs / 1000000).toFixed(2)}ms</Value>
      </Metric>
    </HudContainer>
  );
}

export default PerformanceHUD;
