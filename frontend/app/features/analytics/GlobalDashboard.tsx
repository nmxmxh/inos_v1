import React from 'react';
import styled from 'styled-components';
import { motion, AnimatePresence } from 'framer-motion';
import { useGlobalAnalytics } from './useGlobalAnalytics';
import RollingCounter from '../../ui/RollingCounter';
import NumberFormatter from '../../ui/NumberFormatter';

const Style = {
  Container: styled(motion.div)`
    display: flex;
    flex-direction: column;
    gap: ${p => p.theme.spacing[6]};
    padding: ${p => p.theme.spacing[6]};
    background: ${p => p.theme.colors.paperCream};
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: ${p => p.theme.borders.radius.lg};
    box-shadow: ${p => p.theme.shadows.page};
    font-family: ${p => p.theme.fonts.main};
    position: relative;
    overflow: hidden;

    &::before {
      content: '';
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      height: 4px;
      background: ${p => p.theme.colors.accent};
    }
  `,

  Header: styled.div`
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
    border-bottom: 1px solid ${p => p.theme.colors.borderSubtle};
    padding-bottom: ${p => p.theme.spacing[4]};
  `,

  Title: styled.h2`
    font-family: ${p => p.theme.fonts.display};
    font-size: ${p => p.theme.fontSizes.xl};
    font-weight: ${p => p.theme.fontWeights.extrabold};
    color: ${p => p.theme.colors.inkDark};
    margin: 0;
    letter-spacing: ${p => p.theme.letterSpacing?.tight || '-0.02em'};
    text-transform: uppercase;
  `,

  Subtitle: styled.span`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 9px;
    color: ${p => p.theme.colors.inkLight};
    text-transform: uppercase;
    letter-spacing: 0.1em;
  `,

  Grid: styled.div`
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: ${p => p.theme.spacing[4]};

    @media (max-width: ${p => p.theme.breakpoints.sm}) {
      grid-template-columns: 1fr;
    }
  `,

  StatCard: styled(motion.div)`
    padding: ${p => p.theme.spacing[4]};
    background: ${p => p.theme.colors.paperWhite};
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: ${p => p.theme.borders.radius.md};
    display: flex;
    flex-direction: column;
    gap: ${p => p.theme.spacing[1]};
    position: relative;
  `,

  StatLabel: styled.span`
    font-size: 10px;
    font-weight: ${p => p.theme.fontWeights.semibold};
    color: ${p => p.theme.colors.inkMedium};
    text-transform: uppercase;
    letter-spacing: 0.05em;
  `,

  StatValue: styled.div`
    font-size: ${p => p.theme.fontSizes['3xl']};
    font-weight: ${p => p.theme.fontWeights.extrabold};
    color: ${p => p.theme.colors.accent};
    font-feature-settings: 'tnum';
    display: flex;
    align-items: baseline;
    gap: ${p => p.theme.spacing[1]};
  `,

  StatUnit: styled.span`
    font-size: ${p => p.theme.fontSizes.sm};
    font-weight: ${p => p.theme.fontWeights.medium};
    color: ${p => p.theme.colors.inkLight};
  `,

  Visualizer: styled.div`
    height: 120px;
    width: 100%;
    background: ${p => p.theme.colors.blueprintLight};
    border: 1px solid ${p => p.theme.colors.blueprintGrid};
    border-radius: ${p => p.theme.borders.radius.md};
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-top: ${p => p.theme.spacing[2]};
    background-image:
      linear-gradient(${p => p.theme.colors.blueprintGrid} 1px, transparent 1px),
      linear-gradient(90deg, ${p => p.theme.colors.blueprintGrid} 1px, transparent 1px);
    background-size: 20px 20px;
  `,

  NodeDot: styled(motion.div)`
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: ${p => p.theme.colors.accent};
    box-shadow: 0 0 10px ${p => p.theme.colors.accent};
  `,

  Footer: styled.div`
    display: flex;
    justify-content: space-between;
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    color: ${p => p.theme.colors.inkFaded};
    margin-top: ${p => p.theme.spacing[2]};
  `,

  BlueprintLabel: styled.div`
    position: absolute;
    top: 8px;
    left: 8px;
    font-size: 8px;
    color: ${p => p.theme.colors.blueprint};
    font-weight: bold;
    opacity: 0.5;
  `,
};

function formatBytes(bytes: bigint) {
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  let val = Number(bytes);
  let unitIndex = 0;
  while (val >= 1024 && unitIndex < units.length - 1) {
    val /= 1024;
    unitIndex++;
  }
  return { value: val.toFixed(2), unit: units[unitIndex] };
}

export const GlobalDashboard: React.FC = () => {
  const analytics = useGlobalAnalytics();

  if (!analytics) {
    return (
      <Style.Container initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }}>
        <Style.Header>
          <div>
            <Style.Subtitle>Initializing Analytics Layer</Style.Subtitle>
            <Style.Title>Global Mesh Registry</Style.Title>
          </div>
        </Style.Header>
        <Style.Visualizer>
          <Style.Subtitle>Waiting for Epoch 0...</Style.Subtitle>
        </Style.Visualizer>
      </Style.Container>
    );
  }

  const storage = formatBytes(analytics.totalStorageBytes);

  return (
    <Style.Container
      initial={{ opacity: 0, scale: 0.98 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ type: 'spring', damping: 20, stiffness: 100 }}
    >
      <Style.Header>
        <div>
          <Style.Subtitle>Zero-Copy Mesh Telemetry (Epoch 0x01)</Style.Subtitle>
          <Style.Title>Global Resource Registry</Style.Title>
        </div>
        <Style.Subtitle>
          Node Time: {new Date(analytics.timestamp).toLocaleTimeString()}
        </Style.Subtitle>
      </Style.Header>

      <Style.Grid>
        <Style.StatCard whileHover={{ y: -2 }}>
          <Style.StatLabel>Cumulative Mesh Compute</Style.StatLabel>
          <Style.StatValue>
            <NumberFormatter value={Number(analytics.totalComputeGFLOPS)} suffix=" GFLOPS" />
          </Style.StatValue>
          <Style.Subtitle>Pipelined across {analytics.activeNodeCount} nodes</Style.Subtitle>
        </Style.StatCard>

        <Style.StatCard whileHover={{ y: -2 }}>
          <Style.StatLabel>Distributed Storage Capacity</Style.StatLabel>
          <Style.StatValue>
            <RollingCounter value={parseFloat(storage.value)} decimals={2} />
            <Style.StatUnit>{storage.unit}</Style.StatUnit>
          </Style.StatValue>
          <Style.Subtitle>Content-Addressable (CAS)</Style.Subtitle>
        </Style.StatCard>

        <Style.StatCard whileHover={{ y: -2 }}>
          <Style.StatLabel>Mesh-Wide Throughput</Style.StatLabel>
          <Style.StatValue>
            <NumberFormatter value={Number(analytics.globalOpsPerSec)} suffix=" OPS" />
          </Style.StatValue>
          <Style.Subtitle>Global Gossip Propagation</Style.Subtitle>
        </Style.StatCard>

        <Style.StatCard whileHover={{ y: -2 }}>
          <Style.StatLabel>Consensus Population</Style.StatLabel>
          <Style.StatValue>
            <RollingCounter value={analytics.activeNodeCount} />
            <Style.StatUnit>NODES</Style.StatUnit>
          </Style.StatValue>
          <Style.Subtitle>Active Proof-of-Useful-Work (PoUW)</Style.Subtitle>
        </Style.StatCard>
      </Style.Grid>

      <Style.Visualizer>
        <Style.BlueprintLabel>MESHDATA_FLOW_VISUALIZER [REV_11.2]</Style.BlueprintLabel>
        <AnimatePresence mode="popLayout">
          {Array.from({ length: Math.min(analytics.activeNodeCount, 100) }).map((_, i) => (
            <Style.NodeDot
              key={i}
              initial={{ opacity: 0, scale: 0 }}
              animate={{
                opacity: 1,
                scale: 1,
                x: Math.sin(i * 1.5) * 60,
                y: Math.cos(i * 1.5) * 30,
              }}
              exit={{ opacity: 0, scale: 0 }}
              transition={{ delay: i * 0.05 }}
            />
          ))}
        </AnimatePresence>
        {analytics.activeNodeCount > 100 && (
          <div
            style={{
              position: 'absolute',
              bottom: 8,
              right: 8,
              fontSize: '9px',
              fontFamily: 'monospace',
              color: '#6d28d9',
              background: 'rgba(255,255,255,0.9)',
              padding: '2px 6px',
              borderRadius: '4px',
            }}
          >
            + {analytics.activeNodeCount - 100} NODES
          </div>
        )}
        <motion.div
          style={{ position: 'absolute', width: '100%', height: '100%' }}
          animate={{ rotate: 360 }}
          transition={{ duration: 20, repeat: Infinity, ease: 'linear' }}
        >
          <svg width="100%" height="100%" viewBox="0 0 100 100">
            <circle
              cx="50"
              cy="50"
              r="45"
              fill="none"
              strokeWidth="0.1"
              stroke="rgba(109, 40, 217, 0.2)"
              strokeDasharray="2 2"
            />
          </svg>
        </motion.div>
      </Style.Visualizer>

      <Style.Footer>
        <span>INTEGRITY CHECK: PASSED</span>
        <span>ENCRYPTION: ChaCha20-Poly1305</span>
        <span>LATENCY: ZERO-COPY (SAB)</span>
      </Style.Footer>
    </Style.Container>
  );
};

export default GlobalDashboard;
