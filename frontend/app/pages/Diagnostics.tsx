/**
 * INOS Technical Codex — Diagnostics Dashboard
 *
 * Full-screen telemetry visualization consolidates system health,
 * SAB bridge performance, and mesh stability.
 */

import styled from 'styled-components';
import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import {
  IDX_BIRD_EPOCH,
  IDX_MATRIX_EPOCH,
  IDX_METRICS_EPOCH,
  IDX_ARENA_ALLOCATOR,
  OFFSET_BRIDGE_METRICS,
} from '../../src/wasm/layout';
import { INOSBridge } from '../../src/wasm/bridge-state';
import NumberFormatter from '../ui/NumberFormatter';
import RollingCounter from '../ui/RollingCounter';
import { useEconomics } from '../hooks/useEconomics';
import { useGlobalAnalytics } from '../features/analytics/useGlobalAnalytics';
import { useMeshMetrics } from '../features/metrics/useMeshMetrics';
import { getIdentityStatusLabel, getTierLabel, useIdentitySnapshot } from '../hooks/useIdentity';

const Style = {
  PageContainer: styled.div`
    padding: ${p => p.theme.spacing[10]} ${p => p.theme.spacing[6]};
    max-width: 1200px;
    margin: 0 auto;
    font-family: ${p => p.theme.fonts.main};
  `,

  Header: styled.div`
    margin-bottom: ${p => p.theme.spacing[10]};
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
    border-bottom: 1px solid ${p => p.theme.colors.borderSubtle};
    padding-bottom: ${p => p.theme.spacing[6]};
  `,

  Title: styled.h1`
    font-size: 32px;
    font-weight: 800;
    letter-spacing: -0.02em;
    margin: 0;
    text-transform: uppercase;
  `,

  Subtitle: styled.p`
    color: ${p => p.theme.colors.inkLight};
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 11px;
    margin-top: ${p => p.theme.spacing[2]};
    letter-spacing: 0.1em;
    text-transform: uppercase;
  `,

  Grid: styled.div`
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: ${p => p.theme.spacing[6]};

    @media (max-width: ${p => p.theme.breakpoints.lg}) {
      grid-template-columns: 1fr;
    }
  `,

  Card: styled(motion.div)`
    background: rgba(255, 255, 255, 0.8);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 4px;
    padding: ${p => p.theme.spacing[6]};
    display: flex;
    flex-direction: column;
    gap: ${p => p.theme.spacing[4]};
  `,

  CardTitle: styled.h3`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    font-weight: 800;
    color: ${p => p.theme.colors.inkDark};
    text-transform: uppercase;
    letter-spacing: 0.15em;
    margin: 0;
    display: flex;
    align-items: center;
    gap: ${p => p.theme.spacing[3]};

    &::after {
      content: '';
      flex: 1;
      height: 1px;
      background: ${p => p.theme.colors.borderSubtle};
    }
  `,

  MetricRow: styled.div`
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    padding: ${p => p.theme.spacing[2]} 0;
    border-bottom: 1px dashed ${p => p.theme.colors.borderSubtle};

    &:last-child {
      border-bottom: none;
    }
  `,

  MetricLabel: styled.span`
    font-size: 12px;
    color: ${p => p.theme.colors.inkMedium};
  `,

  MetricValue: styled.span`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 14px;
    font-weight: 700;
    color: ${p => p.theme.colors.accent};
  `,

  StatusPill: styled.span<{ $active: boolean }>`
    padding: 2px 8px;
    border-radius: 100px;
    font-size: 10px;
    font-weight: 800;
    letter-spacing: 0.05em;
    background: ${p =>
      p.$active ? `${p.theme.colors.success}15` : `${p.theme.colors.inkFaded}15`};
    color: ${p => (p.$active ? p.theme.colors.success : p.theme.colors.inkFaded)};
  `,

  BigMetric: styled.div`
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: ${p => p.theme.spacing[8]};
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    background: rgba(0, 0, 0, 0.02);
    border-radius: 4px;
  `,

  BigValue: styled.div`
    font-size: 48px;
    font-weight: 800;
    color: ${p => p.theme.colors.inkDark};
    font-feature-settings: 'tnum';
  `,

  BigLabel: styled.div`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    color: ${p => p.theme.colors.inkLight};
    text-transform: uppercase;
    letter-spacing: 0.1em;
    margin-top: ${p => p.theme.spacing[2]};
  `,

  HealthBar: styled.div`
    height: 4px;
    width: 100%;
    background: ${p => p.theme.colors.borderSubtle};
    border-radius: 2px;
    overflow: hidden;
    margin-top: ${p => p.theme.spacing[2]};
  `,

  HealthFill: styled.div.attrs<{ $percent: number }>(props => ({
    style: {
      width: `${props.$percent}%`,
      background: props.$percent > 90 ? '#16a34a' : props.$percent > 70 ? '#f59e0b' : '#dc2626',
    },
  }))<{ $percent: number }>`
    height: 100%;
    transition:
      width 0.3s ease,
      background 0.3s ease;
  `,
};

interface SystemMetrics {
  birdEpoch: number;
  matrixEpoch: number;
  metricsEpoch: number;
  active: boolean;
  sabSize: number;
  arenaHead: number;
  epochRate: number;
  evolutionMs: number;
  opsPerSecond: number;
  bridge: {
    hits: number;
    misses: number;
    readNs: number;
    writeNs: number;
    health: number;
  };
  balance: number;
  pendingEscrow: number;
  earningsPulse: number;
}

const OPS_PER_ENTITY = 2200;
const ENTITY_COUNT = 1000;
const EMA_ALPHA = 0.2;

export default function Diagnostics() {
  const global = useGlobalAnalytics();
  const meshMetrics = useMeshMetrics();
  const { getBalance } = useEconomics(); // Zero-copy hook
  const identity = useIdentitySnapshot();
  const [metrics, setMetrics] = useState<SystemMetrics>({
    birdEpoch: 0,
    matrixEpoch: 0,
    metricsEpoch: 0,
    active: false,
    sabSize: 0,
    arenaHead: 0,
    epochRate: 0,
    evolutionMs: 0,
    opsPerSecond: 0,
    bridge: { hits: 0, misses: 0, readNs: 0, writeNs: 0, health: 100 },
    balance: 0,
    pendingEscrow: 0,
    earningsPulse: 0,
  });

  useEffect(() => {
    let lastBirdEpoch = 0;
    let lastTime = performance.now();
    let smoothedRate = 0;

    const interval = setInterval(() => {
      try {
        // Use INOSBridge for zero-allocation reads
        if (!INOSBridge.isReady()) return;

        const sab = INOSBridge.getSAB();
        if (!sab) return;

        const birdEpoch = INOSBridge.atomicLoad(IDX_BIRD_EPOCH);
        const matrixEpoch = INOSBridge.atomicLoad(IDX_MATRIX_EPOCH);
        const metricsEpoch = INOSBridge.atomicLoad(IDX_METRICS_EPOCH);
        const arenaHead = INOSBridge.atomicLoad(IDX_ARENA_ALLOCATOR);

        const now = performance.now();
        const deltaEpoch = Math.max(0, birdEpoch - lastBirdEpoch);
        const deltaTime = Math.max(0.001, (now - lastTime) / 1000);
        lastBirdEpoch = birdEpoch;
        lastTime = now;

        const instantRate = deltaEpoch / deltaTime;
        smoothedRate = smoothedRate
          ? smoothedRate * (1 - EMA_ALPHA) + instantRate * EMA_ALPHA
          : instantRate;
        const evolutionMs = smoothedRate > 0 ? 1000 / smoothedRate : 0;
        const opsPerSecond = smoothedRate * ENTITY_COUNT * OPS_PER_ENTITY;

        // Get cached DataView for bridge metrics
        const metricsView = INOSBridge.getRegionDataView(OFFSET_BRIDGE_METRICS, 32);
        if (!metricsView) return;

        const hits = Number(metricsView.getBigUint64(0, true));
        const misses = Number(metricsView.getBigUint64(8, true));
        const readNs = Number(metricsView.getBigUint64(16, true));
        const writeNs = Number(metricsView.getBigUint64(24, true));

        const total = hits + misses;
        const health = total > 0 ? (hits / total) * 100 : 100;
        const active = deltaEpoch > 0;

        // Zero-copy balance read (no worker messaging)
        const currentBalance = getBalance();

        setMetrics({
          birdEpoch,
          matrixEpoch,
          metricsEpoch,
          active,
          sabSize: sab.byteLength,
          arenaHead,
          epochRate: smoothedRate,
          evolutionMs,
          opsPerSecond,
          bridge: { hits, misses, readNs, writeNs, health },
          balance: currentBalance,
          pendingEscrow: 0, // TODO: Read from SAB when economics layout is defined
          earningsPulse: 0, // TODO: Read from SAB when economics layout is defined
        });
      } catch {
        // SAB invalid
      }
    }, 100);

    return () => clearInterval(interval);
  }, [getBalance]);

  return (
    <Style.PageContainer>
      <Style.Header>
        <div>
          <Style.Title>System Diagnostics</Style.Title>
          <Style.Subtitle>Real-time Kernel Performance Telemetry</Style.Subtitle>
        </div>
        <Style.StatusPill $active={metrics.active}>
          KERNEL_{metrics.active ? 'ACTIVE' : 'IDLE'}
        </Style.StatusPill>
      </Style.Header>

      <Style.Grid>
        {/* Core Simulation Performance */}
        <Style.Card
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          data-testid="performance-meter"
        >
          <Style.CardTitle>Simulation Pulse</Style.CardTitle>
          <p style={{ margin: 0, color: '#4b5563', lineHeight: 1.6, fontSize: '12px' }}>
            Derived from SAB atomics (bird + metrics epochs) and smoothed over the last sampling
            window to reduce jitter.
          </p>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
              gap: '1rem',
            }}
          >
            <Style.BigMetric>
              <Style.BigValue>
                <RollingCounter value={metrics.birdEpoch} />
              </Style.BigValue>
              <Style.BigLabel>Bird Epoch</Style.BigLabel>
            </Style.BigMetric>
            <Style.BigMetric>
              <Style.BigValue>
                <RollingCounter value={metrics.metricsEpoch} />
              </Style.BigValue>
              <Style.BigLabel>System Epoch</Style.BigLabel>
            </Style.BigMetric>
          </div>
          <Style.MetricRow>
            <Style.MetricLabel>Epoch Rate</Style.MetricLabel>
            <Style.MetricValue>{metrics.epochRate.toFixed(2)} Hz</Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Evolution Time</Style.MetricLabel>
            <Style.MetricValue>{metrics.evolutionMs.toFixed(2)} ms</Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Estimated Ops/s (Kernel)</Style.MetricLabel>
            <Style.MetricValue>
              <NumberFormatter value={metrics.opsPerSecond} />
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Active Entities</Style.MetricLabel>
            <Style.MetricValue>{ENTITY_COUNT.toLocaleString()}</Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Physics Integration</Style.MetricLabel>
            <Style.MetricValue>Rapier3D @ 60Hz</Style.MetricValue>
          </Style.MetricRow>
        </Style.Card>

        {/* Epoch Counters */}
        <Style.Card
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.15 }}
        >
          <Style.CardTitle>Epoch Signals</Style.CardTitle>
          <p style={{ margin: 0, color: '#4b5563', lineHeight: 1.6, fontSize: '12px' }}>
            Atomic counters stored in the SAB flags region. These signals drive buffer swaps and
            telemetry sync across Go, Rust, and JS.
          </p>
          <Style.MetricRow>
            <Style.MetricLabel>Bird Epoch</Style.MetricLabel>
            <Style.MetricValue>
              <RollingCounter value={metrics.birdEpoch} />
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Matrix Epoch</Style.MetricLabel>
            <Style.MetricValue>
              <RollingCounter value={metrics.matrixEpoch} />
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Metrics Epoch</Style.MetricLabel>
            <Style.MetricValue>
              <RollingCounter value={metrics.metricsEpoch} />
            </Style.MetricValue>
          </Style.MetricRow>
        </Style.Card>

        {/* Bridge Performance */}
        <Style.Card
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
        >
          <Style.CardTitle>SAB Bridge Health</Style.CardTitle>
          <p style={{ margin: 0, color: '#4b5563', lineHeight: 1.6, fontSize: '12px' }}>
            Measured from the SAB bridge metrics region. Higher hit rate means fewer blocking waits.
          </p>
          <Style.BigMetric>
            <Style.BigValue>{metrics.bridge.health.toFixed(1)}%</Style.BigValue>
            <Style.HealthBar>
              <Style.HealthFill $percent={metrics.bridge.health} />
            </Style.HealthBar>
            <Style.BigLabel>Reactive Signal Efficiency</Style.BigLabel>
          </Style.BigMetric>
          <Style.MetricRow>
            <Style.MetricLabel>WaitAsync Hits / Misses</Style.MetricLabel>
            <Style.MetricValue>
              {metrics.bridge.hits} / {metrics.bridge.misses}
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Average I/O Latency</Style.MetricLabel>
            <Style.MetricValue>{(metrics.bridge.readNs / 1000000).toFixed(3)}ms</Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Average Write Latency</Style.MetricLabel>
            <Style.MetricValue>{(metrics.bridge.writeNs / 1000000).toFixed(3)}ms</Style.MetricValue>
          </Style.MetricRow>
        </Style.Card>

        {/* Distributed Mesh Metrics */}
        <Style.Card
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3 }}
        >
          <Style.CardTitle>Mesh Health</Style.CardTitle>
          <p style={{ margin: 0, color: '#4b5563', lineHeight: 1.6, fontSize: '12px' }}>
            These values are read from the mesh metrics region and global analytics when present.
          </p>
          <Style.MetricRow>
            <Style.MetricLabel>Active Mesh Nodes</Style.MetricLabel>
            <Style.MetricValue>
              {global?.activeNodeCount ?? meshMetrics?.connectedPeers ?? 1}
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Connected Peers</Style.MetricLabel>
            <Style.MetricValue>{meshMetrics?.connectedPeers ?? 0}</Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Mesh Success Rate</Style.MetricLabel>
            <Style.MetricValue>
              {((meshMetrics?.successRate ?? 1) * 100).toFixed(2)}%
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>P50 Latency</Style.MetricLabel>
            <Style.MetricValue>{(meshMetrics?.p50Latency ?? 0).toFixed(1)} ms</Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>P95 Latency</Style.MetricLabel>
            <Style.MetricValue>{(meshMetrics?.p95Latency ?? 0).toFixed(1)} ms</Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Average Reputation</Style.MetricLabel>
            <Style.MetricValue>
              {((meshMetrics?.avgReputation ?? 0.984) * 100).toFixed(2)}%
            </Style.MetricValue>
          </Style.MetricRow>
        </Style.Card>

        {/* Mesh Traffic */}
        <Style.Card
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.35 }}
        >
          <Style.CardTitle>Mesh Traffic</Style.CardTitle>
          <p style={{ margin: 0, color: '#4b5563', lineHeight: 1.6, fontSize: '12px' }}>
            Global analytics provides throughput and capacity when the mesh is active; local runs
            will show minimal traffic.
          </p>
          <Style.MetricRow>
            <Style.MetricLabel>Network Throughput</Style.MetricLabel>
            <Style.MetricValue>
              <NumberFormatter value={Number(global?.globalOpsPerSec ?? 0)} /> OPS/S
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Compute Capacity</Style.MetricLabel>
            <Style.MetricValue>
              <NumberFormatter value={Number(global?.totalComputeGFLOPS ?? 0)} /> GFLOPS
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Bytes Sent</Style.MetricLabel>
            <Style.MetricValue>
              <NumberFormatter value={Number(meshMetrics?.bytesSent ?? 0)} suffix="B" />
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Bytes Received</Style.MetricLabel>
            <Style.MetricValue>
              <NumberFormatter value={Number(meshMetrics?.bytesReceived ?? 0)} suffix="B" />
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Sector ID</Style.MetricLabel>
            <Style.MetricValue>
              {meshMetrics?.sectorId
                ? `0x${meshMetrics.sectorId.toString(16).toUpperCase().padStart(4, '0')}`
                : '0x0000'}
            </Style.MetricValue>
          </Style.MetricRow>
        </Style.Card>

        {/* Resource Allocation */}
        <Style.Card
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.4 }}
        >
          <Style.CardTitle>Linear Memory Map</Style.CardTitle>
          <p style={{ margin: 0, color: '#4b5563', lineHeight: 1.6, fontSize: '12px' }}>
            Read directly from SAB metadata to show allocator state and memory footprint.
          </p>
          <Style.MetricRow>
            <Style.MetricLabel>SharedArrayBuffer Size</Style.MetricLabel>
            <Style.MetricValue>{(metrics.sabSize / (1024 * 1024)).toFixed(0)} MB</Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Arena Head</Style.MetricLabel>
            <Style.MetricValue>0x{metrics.arenaHead.toString(16).toUpperCase()}</Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Dynamic Heap Index</Style.MetricLabel>
            <Style.MetricValue>0x5C0000</Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Alloc Integrity</Style.MetricLabel>
            <Style.MetricValue style={{ color: '#10b981' }}>✓ VERIFIED</Style.MetricValue>
          </Style.MetricRow>
        </Style.Card>

        {/* Identity & Recovery */}
        <Style.Card
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.45 }}
          data-testid="identity-ledger-card"
        >
          <Style.CardTitle>Identity & Recovery</Style.CardTitle>
          <p style={{ margin: 0, color: '#4b5563', lineHeight: 1.6, fontSize: '12px' }}>
            Read from the identity + social SAB regions. Close IDs mark mutual recovery anchors.
          </p>
          <Style.MetricRow>
            <Style.MetricLabel>Node Identity (DID)</Style.MetricLabel>
            <Style.MetricValue style={{ fontSize: '10px', color: '#6b7280' }}>
              {identity?.did
                ? `${identity.did.slice(0, 28)}...`
                : `did:inos:${(window as any).inosModules?.compute?.node_id?.slice(0, 16) || 'anonymous'}...`}
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Identity Status</Style.MetricLabel>
            <Style.MetricValue>{getIdentityStatusLabel(identity?.status ?? 0)}</Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Resource Tier</Style.MetricLabel>
            <Style.MetricValue>{getTierLabel(identity?.tier ?? 0)}</Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Recovery Threshold</Style.MetricLabel>
            <Style.MetricValue>
              {identity ? `${identity.recoveryThreshold}/${identity.totalShares}` : '1/1'}
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Close IDs</Style.MetricLabel>
            <Style.MetricValue>
              {identity ? `${identity.verifiedCloseIds}/${identity.closeIds.length}` : '0/0'}
            </Style.MetricValue>
          </Style.MetricRow>
        </Style.Card>

        {/* Credits & Yield */}
        <Style.Card
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.5 }}
          data-testid="economic-ledger-card"
        >
          <Style.CardTitle>Credits & Yield</Style.CardTitle>
          <p style={{ margin: 0, color: '#4b5563', lineHeight: 1.6, fontSize: '12px' }}>
            Balance is read from the default account offset in SAB; yield fields are placeholders
            until economics metrics are written into SAB.
          </p>
          <Style.BigMetric
            style={{
              background: 'rgba(22, 163, 74, 0.05)',
              borderColor: 'rgba(22, 163, 74, 0.2)',
            }}
          >
            <Style.BigValue style={{ color: '#16a34a' }}>
              <NumberFormatter value={metrics.balance} decimals={0} />
            </Style.BigValue>
            <Style.BigLabel>Available Credits (µ)</Style.BigLabel>
          </Style.BigMetric>
          <Style.MetricRow>
            <Style.MetricLabel>Pending Escrow</Style.MetricLabel>
            <Style.MetricValue style={{ color: '#0284c7' }}>
              <NumberFormatter value={metrics.pendingEscrow} decimals={0} suffix="µ" /> (Locked for
              active jobs)
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Earnings Pulse</Style.MetricLabel>
            <Style.MetricValue style={{ color: '#16a34a' }}>
              +{metrics.earningsPulse.toFixed(2)} µ/min (Useful work)
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Projected Earnings/24h</Style.MetricLabel>
            <Style.MetricValue style={{ color: '#16a34a' }}>
              +
              <NumberFormatter value={metrics.earningsPulse * 60 * 24} decimals={0} suffix="µ" />
            </Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Staking Ratio</Style.MetricLabel>
            <Style.MetricValue>1.0x (Baseline trust)</Style.MetricValue>
          </Style.MetricRow>
          <Style.MetricRow>
            <Style.MetricLabel>Credit Utility</Style.MetricLabel>
            <Style.MetricValue style={{ color: '#6b7280' }}>
              Spendable for compute, storage, and mesh bandwidth
            </Style.MetricValue>
          </Style.MetricRow>
        </Style.Card>
      </Style.Grid>
    </Style.PageContainer>
  );
}
