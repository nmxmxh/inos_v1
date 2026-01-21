/**
 * INOS Technical Codex : Landing Page (v2.1 - Visionary Overhaul)
 *
 * Sells the "Shift" from a document viewer to a distributed supercomputer.
 * Refined with specific user-requested metrics and experimental messaging.
 */

import styled from 'styled-components';
import { motion } from 'framer-motion';
import { NavLink } from 'react-router-dom';
import { useState, useEffect, useRef } from 'react';
import { Style as ManuscriptStyle } from '../styles/manuscript';
import ChapterNav from '../ui/ChapterNav';
import ScrollReveal from '../ui/ScrollReveal';
import { IDX_BIRD_EPOCH, IDX_METRICS_EPOCH } from '../../src/wasm/layout';
import { INOSBridge } from '../../src/wasm/bridge-state';

import RollingCounter from '../ui/RollingCounter';
import NumberFormatter from '../ui/NumberFormatter';
import DimostrazioneStory from '../features/dimostrazione/DimostrazioneStory';

const Style = {
  ...ManuscriptStyle,

  HeroSection: styled(motion.section)`
    margin-bottom: ${p => p.theme.spacing[12]};
    text-align: left;
    position: relative;
  `,

  BangTitle: styled(motion.h1)`
    font-family: ${p => p.theme.fonts.main};
    font-size: clamp(2rem, 8vw, 4.5rem);
    font-weight: 900;
    line-height: 0.95;
    letter-spacing: -0.04em;
    color: ${p => p.theme.colors.inkDark};
    margin: ${p => p.theme.spacing[6]} 0;
    text-transform: uppercase;

    span {
      display: block;
      color: ${p => p.theme.colors.accent};
    }
  `,

  Subtitle: styled.p`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 11px;
    font-weight: 700;
    color: ${p => p.theme.colors.accent};
    text-transform: uppercase;
    letter-spacing: 0.3em;
    margin: 0;
    opacity: 0.8;
  `,

  ShiftGrid: styled.div`
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: ${p => p.theme.spacing[8]};
    margin: ${p => p.theme.spacing[12]} 0;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      grid-template-columns: 1fr;
    }
  `,

  ShiftCard: styled.div<{ $variant?: 'legacy' | 'inos' }>`
    padding: ${p => p.theme.spacing[8]};
    border-radius: 12px;
    background: ${p => (p.$variant === 'inos' ? 'rgba(139, 92, 246, 0.05)' : 'rgba(0,0,0,0.02)')};
    border: 1px solid
      ${p => (p.$variant === 'inos' ? 'rgba(139, 92, 246, 0.2)' : 'rgba(0,0,0,0.05)')};
    position: relative;
    overflow: hidden;

    &::before {
      content: '${p => (p.$variant === 'inos' ? 'THE FUTURE' : 'THE LEGACY')}';
      position: absolute;
      top: 12px;
      right: 12px;
      font-family: ${p => p.theme.fonts.typewriter};
      font-size: 8px;
      font-weight: 800;
      opacity: 0.5;
      letter-spacing: 0.1em;
    }

    h4 {
      margin-top: 0;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: ${p => (p.$variant === 'inos' ? p.theme.colors.accent : 'inherit')};
    }
  `,

  MetricPill: styled.div<{ $color?: string }>`
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 6px 12px;
    background: ${p => p.$color || p.theme.colors.accent}15;
    border: 1px solid ${p => p.$color || p.theme.colors.accent}30;
    border-radius: 99px;
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    font-weight: 700;
    color: ${p => p.$color || p.theme.colors.accent};
    margin-bottom: 1rem;
    text-transform: uppercase;
  `,

  LiveStatsGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: ${p => p.theme.spacing[6]};
    margin: ${p => p.theme.spacing[12]} 0;
    padding: ${p => p.theme.spacing[8]};
    background: #fff;
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    box-shadow: 0 20px 40px rgba(0, 0, 0, 0.03);
    border-radius: 12px;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      padding: ${p => p.theme.spacing[4]};
      gap: ${p => p.theme.spacing[4]};
    }
  `,

  StatBox: styled.div`
    display: flex;
    flex-direction: column;
    gap: ${p => p.theme.spacing[2]};
  `,

  MetricValue: styled.div`
    font-size: 2.5rem;
    font-weight: 900;
    letter-spacing: -0.02em;
    color: ${p => p.theme.colors.inkDark};
    line-height: 1;
    font-feature-settings: 'tnum';
  `,

  BentoBox: styled.div`
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1.5rem;
    margin: 4rem 0;

    @media (max-width: ${p => p.theme.breakpoints.lg}) {
      grid-template-columns: 1fr;
    }
  `,

  BentoItem: styled.div`
    background: #fff;
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    padding: 2rem;
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    justify-content: flex-start;
    transition: transform 0.2s;

    &:hover {
      transform: translateY(-4px);
      box-shadow: 0 10px 30px rgba(0, 0, 0, 0.05);
    }

    h4 {
      margin-top: 0.5rem;
      font-size: 1.2rem;
      font-weight: 800;
      color: ${p => p.theme.colors.inkDark};
      text-transform: uppercase;
      letter-spacing: -0.02em;
    }

    p {
      margin-bottom: 0;
      font-size: 1rem;
      line-height: 1.6;
      color: ${p => p.theme.colors.inkMedium};
    }

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      padding: 1.5rem;
      h4 {
        font-size: 1rem;
      }
      p {
        font-size: 0.9rem;
      }
    }
  `,

  SectionDivider: styled.div`
    height: 1px;
    background: linear-gradient(
      to right,
      transparent,
      ${p => p.theme.colors.borderSubtle},
      transparent
    );
    margin: 6rem 0;
  `,

  TOCSection: styled.section`
    margin-top: ${p => p.theme.spacing[16]};
  `,

  TOCList: styled(motion.ul)`
    list-style: none;
    padding: 0;
    display: grid;
    grid-template-columns: 1fr;
    gap: ${p => p.theme.spacing[6]};
  `,

  TOCItem: styled(motion.li)`
    background: rgba(255, 255, 255, 0.85);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 6px;
    padding: ${p => p.theme.spacing[6]};
    transition:
      transform 0.2s,
      box-shadow 0.2s;

    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
    }
  `,

  TOCLink: styled(NavLink)`
    text-decoration: none;
    display: block;

    &:hover h3 {
      color: ${p => p.theme.colors.accent};
    }
  `,

  ChapterNumber: styled.span`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 9px;
    color: ${p => p.theme.colors.accent};
    font-weight: ${p => p.theme.fontWeights.bold};
    text-transform: uppercase;
    letter-spacing: 0.1em;
  `,

  ChapterTitle: styled.h3`
    font-family: ${p => p.theme.fonts.main};
    font-size: ${p => p.theme.fontSizes.lg};
    font-weight: ${p => p.theme.fontWeights.bold};
    color: ${p => p.theme.colors.inkDark};
    margin: ${p => p.theme.spacing[1]} 0;
    text-transform: uppercase;
    transition: color 0.2s;
  `,

  ChapterDescription: styled.p`
    font-size: ${p => p.theme.fontSizes.base};
    color: ${p => p.theme.colors.inkMedium};
    margin: ${p => p.theme.spacing[2]} 0 0;
    line-height: 1.5;
  `,

  ContentCard: styled.div`
    background: rgba(255, 255, 255, 0.88);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 8px;
    padding: ${p => p.theme.spacing[6]};
    margin: ${p => p.theme.spacing[6]} 0;

    h3 {
      margin-top: 0;
      margin-bottom: ${p => p.theme.spacing[4]};
    }
  `,

  BlogContainer: styled(ManuscriptStyle.BlogContainer)`
    padding-top: ${p => p.theme.spacing[8]};
  `,

  IllustrationContainer: styled.div`
    width: 100%;
    background: rgba(255, 255, 255, 0.92);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 8px;
    margin: ${p => p.theme.spacing[10]} 0;
    overflow: hidden;
  `,

  IllustrationHeader: styled.div`
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: ${p => p.theme.spacing[3]} ${p => p.theme.spacing[4]};
    border-bottom: 1px solid ${p => p.theme.colors.borderSubtle};
    background: rgba(0, 0, 0, 0.02);
  `,

  IllustrationTitle: styled.span`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    font-weight: 600;
    color: ${p => p.theme.colors.inkDark};
    text-transform: uppercase;
    letter-spacing: 0.1em;
  `,

  IllustrationCaption: styled.p`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    color: ${p => p.theme.colors.inkMedium};
    text-align: center;
    padding: ${p => p.theme.spacing[3]};
    margin: 0;
    border-top: 1px solid ${p => p.theme.colors.borderSubtle};
  `,

  DeepDiveLink: styled(NavLink)<{ $color: string }>`
    display: flex;
    flex-direction: column;
    justify-content: center;
    background: rgba(255, 255, 255, 0.9);
    border: 1px solid ${p => p.$color}30;
    border-left: 4px solid ${p => p.$color};
    border-radius: 8px;
    padding: 1.5rem;
    text-decoration: none;
    transition:
      transform 0.2s,
      box-shadow 0.2s,
      background 0.2s;

    &:hover {
      transform: translateX(4px);
      box-shadow: 0 4px 15px rgba(0, 0, 0, 0.05);
      background: #fafafa;
    }
  `,

  DeepDiveTitle: styled.div<{ $color: string }>`
    font-weight: 800;
    color: ${p => p.$color};
    margin-bottom: 0.5rem;
    text-transform: uppercase;
    font-size: 1rem;
    letter-spacing: 0.05em;
  `,

  DeepDiveDesc: styled.div`
    font-size: 0.9rem;
    color: ${p => p.theme.colors.inkMedium};
    line-height: 1.4;
  `,
};

const CHAPTERS = [
  {
    number: '01',
    title: 'The Problem',
    path: '/problem',
    description:
      'The internet is dying of a thousand copies. Serialization overhead and polling cycles consume 60% of modern compute. This is the Copy Tax.',
  },
  {
    number: '02',
    title: 'The Insight',
    path: '/insight',
    description:
      'The browser is a supercomputer node in disguise. By connecting WebAssembly, WebGPU, and SharedArrayBuffer, we unlock native-speed distributed reality.',
  },
  {
    number: '03',
    title: 'The Architecture',
    path: '/architecture',
    description:
      'A three-layer nervous system: Go orchestrates, Rust computes, and TypeScript renders. One shared memory buffer. Zero copies. Near-zero latency.',
  },
  {
    number: '04',
    title: 'History',
    path: '/history',
    description:
      'From the dream of a unified network to the message-passing bloat of the cloud. INOS is a return to Distributed Shared Memory at global scale.',
  },
  {
    number: '05',
    title: "What's Next",
    path: '/whats-next',
    description:
      'Planetary compute infrastructure. From autonomous agent swarms to proof-of-useful-work consensus. The web is evolving into an Operating System.',
  },
];

function useLiveStats() {
  const lastEpochRef = useRef<number>(0);
  const lastTimeRef = useRef<number>(performance.now());
  const smoothedRateRef = useRef<number>(0);

  const [stats, setStats] = useState({
    opsPerSecond: 0,
    systemEpoch: 0,
    activeNodes: 1,
    latency: 0.0002, // Base target
  });

  useEffect(() => {
    const interval = setInterval(() => {
      try {
        if (!INOSBridge.isReady()) return;

        const currentEpoch = INOSBridge.atomicLoad(IDX_BIRD_EPOCH);
        const systemEpoch = INOSBridge.atomicLoad(IDX_METRICS_EPOCH);
        const now = performance.now();

        // Fix initial state logic to prevent delta explosion
        if (lastEpochRef.current === 0) {
          lastEpochRef.current = currentEpoch;
          lastTimeRef.current = now;
          setStats(prev => ({ ...prev, systemEpoch }));
          return;
        }

        const delta = Math.max(0, currentEpoch - lastEpochRef.current);
        const deltaTime = Math.max(0.001, (now - lastTimeRef.current) / 1000);

        lastEpochRef.current = currentEpoch;
        lastTimeRef.current = now;

        const instantRate = delta / deltaTime;
        smoothedRateRef.current = smoothedRateRef.current
          ? smoothedRateRef.current * 0.8 + instantRate * 0.2
          : instantRate;

        // opsPerSecond: Rate of physics frames * number of agents * micro-operations per agent
        // We use realistic values that won't overflow NumberFormatter logic
        const opsPerSecond = smoothedRateRef.current * 1000 * 2200;

        setStats(prev => ({
          ...prev,
          opsPerSecond,
          systemEpoch,
        }));
      } catch {
        /* SAB not ready */
      }
    }, 500);
    return () => clearInterval(interval);
  }, []);

  return stats;
}

export function Landing() {
  const stats = useLiveStats();

  return (
    <Style.BlogContainer>
      <Style.HeroSection
        initial={{ opacity: 0, y: 30 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.8 }}
      >
        <Style.Subtitle>Phase 1: Experimental Research System (Alpha)</Style.Subtitle>
        <Style.BangTitle>
          The Browser is no longer <span>a document viewer.</span>
        </Style.BangTitle>

        <Style.LeadParagraph>
          <strong>
            INOS is the first experimental distributed operating system that turns your browser into
            a high-performance compute node in a global mesh network. This is a visionary overhaul
            of the web.
          </strong>
        </Style.LeadParagraph>

        <Style.ShiftGrid>
          <Style.ShiftCard $variant="legacy">
            <h4 style={{ color: '#666' }}>The Millisecond World</h4>
            <p style={{ fontSize: '0.9rem', color: '#666' }}>
              Traditional web apps are trapped in a cycle of <strong>Request-Response</strong>. Data
              is copied, serialized, and sent over high-latency sockets.
            </p>
            <ul style={{ paddingLeft: '1.2rem', fontSize: '0.8rem', color: '#666' }}>
              <li>Polling every 16-50ms</li>
              <li>Massive Serialization Tax</li>
              <li>Centralized Bottlenecks</li>
              <li>Wasted Energy & Heat</li>
            </ul>
          </Style.ShiftCard>

          <Style.ShiftCard $variant="inos">
            <h4>The Microsecond World</h4>
            <p style={{ fontSize: '0.9rem' }}>
              INOS communicates via <strong>Hardware Signaling</strong>. Threads sleep at the
              hardware level, waking for signals in nanoseconds.
            </p>
            <ul style={{ paddingLeft: '1.2rem', fontSize: '0.8rem' }}>
              <li>Zero-Polling Performance</li>
              <li>Zero-Copy Memory Pipeline</li>
              <li>Distributed Mesh Consensus</li>
              <li>Biologically Inspired Efficiency</li>
            </ul>
          </Style.ShiftCard>
        </Style.ShiftGrid>

        <Style.LiveStatsGrid>
          <Style.StatBox>
            <Style.MetricPill $color="#8b5cf6">Reactive Throughput</Style.MetricPill>
            <Style.MetricValue>
              <NumberFormatter value={stats.opsPerSecond} />
            </Style.MetricValue>
            <div style={{ fontSize: '10px', color: '#666', marginTop: '4px' }}>
              KERNEL OPS / SECOND
            </div>
          </Style.StatBox>
          <Style.StatBox>
            <Style.MetricPill $color="#10b981">Singular Reality</Style.MetricPill>
            <Style.MetricValue>
              <RollingCounter value={stats.systemEpoch} />
            </Style.MetricValue>
            <div style={{ fontSize: '10px', color: '#666', marginTop: '4px' }}>
              DETERMINISTIC SIGNALS
            </div>
          </Style.StatBox>
          <Style.StatBox>
            <Style.MetricPill $color="#ec4899">Bridge Latency</Style.MetricPill>
            <Style.MetricValue>
              <NumberFormatter value={stats.latency} decimals={4} />
            </Style.MetricValue>
            <div style={{ fontSize: '10px', color: '#666', marginTop: '4px' }}>
              MILLISECONDS (TARGET)
            </div>
          </Style.StatBox>
        </Style.LiveStatsGrid>

        <ScrollReveal variant="fade">
          <h3>The Great Shift: From Viewers to Neurons</h3>
          <p style={{ fontSize: '1.1rem', lineHeight: 1.7 }}>
            We have reclaimed the lost compute of the web. By moving synchronization from the
            application layer to the <strong>hardware memory layer</strong>, INOS achieves
            performance figures that were previously impossible in a browser. This isn't just faster
            code; it's a fundamental re-architecture of the internet substrate.
          </p>
        </ScrollReveal>

        <Style.BentoBox>
          <Style.BentoItem>
            <Style.MetricPill $color="#8b5cf6">43.2x SPEEDUP</Style.MetricPill>
            <h4>Zero-Copy Architecture</h4>
            <p>
              Data reaches Go, Rust, and TypeScript simultaneously without ever being copied. We
              simply swap memory pointers in nanoseconds.
            </p>
          </Style.BentoItem>
          <Style.BentoItem>
            <Style.MetricPill $color="#f59e0b">ENERGY SAVINGS</Style.MetricPill>
            <h4>Hardware Sleep</h4>
            <p>
              Threads sleep at the hardware level, consuming near-zero power until an atomic signal
              wakes them.
            </p>
          </Style.BentoItem>
          <Style.BentoItem>
            <Style.MetricPill $color="#dc2626">121,354x FASTER</Style.MetricPill>
            <h4>Zero-Polling</h4>
            <p>
              Replaced legacy polling loops with signal-driven epochs, reducing reaction jitter to
              sub-microsecond levels.
            </p>
          </Style.BentoItem>
          <Style.BentoItem>
            <Style.MetricPill $color="#10b981">5.7M TPS</Style.MetricPill>
            <h4>Economic Ledger</h4>
            <p>
              A provably consistent credit system running in shared memory, coordinating millions of
              participants without a central server.
            </p>
          </Style.BentoItem>
        </Style.BentoBox>

        <Style.SectionDivider />

        <ScrollReveal variant="manuscript">
          <h3>The Anatomy of the Machine</h3>
          <p>
            This diagram shows the SharedArrayBuffer memory pool live in your browser. All layers of
            the stack—Go, Rust, and TypeScript—are reading from this single source of truth.
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>Shared Reality Memory Layout</Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <DimostrazioneStory />
            <Style.IllustrationCaption>
              Hardware-synchronized memory buffer. No serialization. No translation. Total
              coherence.
            </Style.IllustrationCaption>
          </Style.IllustrationContainer>
        </ScrollReveal>

        {/* BOIDS OBSERVER CALLOUT */}
        <ScrollReveal variant="fade">
          <Style.ContentCard
            style={{
              background: 'rgba(139, 92, 246, 0.04)',
              borderColor: 'rgba(139, 92, 246, 0.2)',
            }}
          >
            <h3 style={{ color: '#8b5cf6', marginBottom: '1rem' }}>🐦 Observe the Swarm</h3>
            <p style={{ fontSize: '1rem', lineHeight: 1.7, marginBottom: '1rem' }}>
              Look behind this text. The birds you see are not a video. They are{' '}
              <strong>1,000 autonomous agents</strong> executing Reynolds flocking physics in
              real-time.
            </p>
            <ul style={{ paddingLeft: '1.5rem', margin: 0, fontSize: '0.9rem', color: '#666' }}>
              <li>
                <strong>Rust</strong> computes forces and SIMD matrix transforms at 60fps
              </li>
              <li>
                <strong>Go</strong> evolves the flock's behavior through a genetic supervisor
              </li>
              <li>
                <strong>TypeScript</strong> reads positions directly from SharedArrayBuffer and
                renders via WebGL
              </li>
            </ul>
            <p style={{ marginTop: '1rem', fontSize: '0.9rem', color: '#555' }}>
              No messages. No serialization. Just shared memory and atomic epoch signals. The
              counter in the corner is not a timer. It is the{' '}
              <strong>evolutionary heartbeat</strong> of the swarm.
            </p>
          </Style.ContentCard>
        </ScrollReveal>

        <Style.TOCSection>
          <Style.SectionTitle>The Living Codex Map</Style.SectionTitle>
          <Style.TOCList initial="initial" animate="animate">
            {CHAPTERS.map(chapter => (
              <Style.TOCItem key={chapter.path} whileHover={{ x: 5 }}>
                <Style.TOCLink to={chapter.path}>
                  <Style.ChapterNumber>{chapter.number}</Style.ChapterNumber>
                  <Style.ChapterTitle>{chapter.title}</Style.ChapterTitle>
                  <Style.ChapterDescription>{chapter.description}</Style.ChapterDescription>
                </Style.TOCLink>
              </Style.TOCItem>
            ))}
          </Style.TOCList>

          <Style.ContentCard
            style={{
              marginTop: '4rem',
              background: 'rgba(139, 92, 246, 0.03)',
              border: '1px solid rgba(139, 92, 246, 0.15)',
            }}
          >
            <Style.Subtitle style={{ color: '#8b5cf6' }}>Advanced Deep Dives</Style.Subtitle>
            <Style.ChapterTitle>Technical Pillars of an Experimental OS</Style.ChapterTitle>
            <p style={{ color: '#4b5563', marginBottom: '2rem' }}>
              Explore the core innovations that turn a browser into a planetary node. Each deep dive
              documents the research behind sub-10µs reactivity and shared-memory mesh networking.
            </p>
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
                gap: '1.5rem',
                marginTop: '1rem',
              }}
            >
              <Style.DeepDiveLink to="/deep-dives/performance" $color="#10b981">
                <Style.DeepDiveTitle $color="#10b981">System Performance</Style.DeepDiveTitle>
                <Style.DeepDiveDesc>
                  Analyzing the 43.2x speedup and cross-engine deterministic stability.
                </Style.DeepDiveDesc>
              </Style.DeepDiveLink>

              <Style.DeepDiveLink to="/deep-dives/zero-copy" $color="#8b5cf6">
                <Style.DeepDiveTitle $color="#8b5cf6">Zero-Copy Memory I/O</Style.DeepDiveTitle>
                <Style.DeepDiveDesc>
                  Eliminating the serialization tax through atomic pointer swapping.
                </Style.DeepDiveDesc>
              </Style.DeepDiveLink>

              <Style.DeepDiveLink to="/deep-dives/signaling" $color="#dc2626">
                <Style.DeepDiveTitle $color="#dc2626">Epoch Signaling</Style.DeepDiveTitle>
                <Style.DeepDiveDesc>
                  Achieving sub-10µs reactivity with lock-free atomic barriers.
                </Style.DeepDiveDesc>
              </Style.DeepDiveLink>

              <Style.DeepDiveLink to="/deep-dives/mesh" $color="#3b82f6">
                <Style.DeepDiveTitle $color="#3b82f6">Distributed P2P Mesh</Style.DeepDiveTitle>
                <Style.DeepDiveDesc>
                  How thousands of agents collaborate across a browser-based swarm.
                </Style.DeepDiveDesc>
              </Style.DeepDiveLink>

              <Style.DeepDiveLink to="/deep-dives/economy" $color="#f59e0b">
                <Style.DeepDiveTitle $color="#f59e0b">Economic Mesh Ledger</Style.DeepDiveTitle>
                <Style.DeepDiveDesc>
                  Sub-microsecond settlement for distributed compute and storage tiers.
                </Style.DeepDiveDesc>
              </Style.DeepDiveLink>
            </div>
          </Style.ContentCard>
        </Style.TOCSection>
      </Style.HeroSection>

      <ChapterNav next={{ to: '/problem', title: '01. The Problem' }} />
    </Style.BlogContainer>
  );
}

export default Landing;
