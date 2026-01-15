/**
 * INOS Technical Codex : Landing Page
 *
 * Renaissance-inspired narrative, framed with hero/villain arc.
 * Refactored to Style object pattern.
 */

import styled from 'styled-components';
import { motion } from 'framer-motion';
import { NavLink } from 'react-router-dom';
import { useState, useEffect } from 'react';
import { Style as ManuscriptStyle } from '../styles/manuscript';
import ChapterNav from '../ui/ChapterNav';
import ScrollReveal from '../ui/ScrollReveal';
import { IDX_BIRD_EPOCH } from '../../src/wasm/layout';
import { INOSBridge } from '../../src/wasm/bridge-state';

import RollingCounter from '../ui/RollingCounter';
import DimostrazioneStory from '../features/dimostrazione/DimostrazioneStory';

const Style = {
  ...ManuscriptStyle,

  HeroSection: styled(motion.section)`
    margin-bottom: ${p => p.theme.spacing[12]};
    text-align: left;
  `,

  Subtitle: styled.p`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    font-weight: ${p => p.theme.fontWeights.bold};
    color: ${p => p.theme.colors.accent};
    text-transform: uppercase;
    letter-spacing: 0.2em;
    margin: 0 0 ${p => p.theme.spacing[2]};
  `,

  LiveStatsGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[8]} 0;
    padding: ${p => p.theme.spacing[6]};
    background: rgba(255, 255, 255, 0.75);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 6px;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      grid-template-columns: 1fr;
    }
  `,

  StatBox: styled.div`
    display: flex;
    flex-direction: column;
    gap: ${p => p.theme.spacing[1]};
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

  SectionDivider: styled.div`
    height: 1px;
    background: linear-gradient(
      to right,
      transparent,
      ${p => p.theme.colors.borderSubtle},
      transparent
    );
    margin: ${p => p.theme.spacing[10]} 0;
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

  BlogContainer: styled(ManuscriptStyle.BlogContainer)`
    padding-top: ${p => p.theme.spacing[8]}; // Reduced from default spacing[16]
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
    display: block;
    background: rgba(255, 255, 255, 0.9);
    border: 1px solid ${p => p.$color}30;
    border-left: 3px solid ${p => p.$color};
    border-radius: 6px;
    padding: ${p => p.theme.spacing[4]};
    text-decoration: none;
    transition:
      transform 0.2s,
      box-shadow 0.2s;

    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
    }
  `,

  DeepDiveTitle: styled.div<{ $color: string }>`
    font-weight: 700;
    color: ${p => p.$color};
    margin-bottom: ${p => p.theme.spacing[1]};
    text-transform: uppercase;
    font-size: 0.75rem;
    letter-spacing: 0.05em;
  `,

  DeepDiveDesc: styled.div`
    font-size: ${p => p.theme.fontSizes.sm};
    color: ${p => p.theme.colors.inkMedium};
  `,
};

const MANUSCRIPT_VARIANTS = {
  initial: { opacity: 0, y: 20 },
  animate: { opacity: 1, y: 0, transition: { duration: 0.8, ease: [0.22, 1, 0.36, 1] as const } },
};

const STAGGER_CONTAINER_VARIANTS = {
  initial: {},
  animate: {
    transition: { staggerChildren: 0.1 },
  },
};

const STAGGER_CHILD_VARIANTS = {
  initial: { opacity: 0, x: -10 },
  animate: { opacity: 1, x: 0 },
};

const CHAPTERS = [
  {
    number: '01',
    title: 'The Problem',
    path: '/problem',
    description:
      "Global data centers consume 200+ TWh annually, yet the majority of modern compute is wasted on coordination, translation, and data duplication: the 'Copy Tax'. We are building to reclaim the lost efficiency of the internet.",
  },
  {
    number: '02',
    title: 'The Insight',
    path: '/insight',
    description:
      'The pieces of the future are already here: WebAssembly, WebGPU, and SharedArrayBuffer. By wiring them into a zero-copy architecture, we turn every browser into an active participant in a global shared reality.',
  },
  {
    number: '03',
    title: 'The Architecture',
    path: '/architecture',
    description:
      'A tri-layer symphony where Go orchestrates policy, Rust executes SIMD-accelerated logic, and TypeScript renders at 60fps. All read from the same absolute memory buffer. No serialization. No latency.',
  },
  {
    number: '04',
    title: 'History',
    path: '/history',
    description:
      'The 30-year legacy of message-passing has cost us 60% of CPU cycles. INOS is a technical correction: a return to Distributed Shared Memory and the original promise of a unified network.',
  },
  {
    number: '05',
    title: "What's Next",
    path: '/whats-next',
    description:
      'The path forward: from experimental prototype to planetary infrastructure. Proof-of-Useful-Work consensus, autonomous sensor swarms, and a substrate where knowing is not enough—we must apply.',
  },
];

function useLiveStats() {
  const [stats, setStats] = useState({
    opsPerSecond: 0,
    birdCount: 1000,
    epoch: 0,
  });

  useEffect(() => {
    let lastEpoch = 0;

    const interval = setInterval(() => {
      try {
        // Use INOSBridge for zero-allocation reads
        if (!INOSBridge.isReady()) return;

        const epoch = INOSBridge.atomicLoad(IDX_BIRD_EPOCH);
        const delta = epoch - lastEpoch;
        lastEpoch = epoch;

        setStats(prev => ({
          ...prev,
          opsPerSecond: delta * 10 * 10,
          epoch,
        }));
      } catch {
        // SAB not ready
      }
    }, 100);

    return () => clearInterval(interval);
  }, []);

  return stats;
}

export function Landing() {
  const stats = useLiveStats();

  return (
    <Style.BlogContainer>
      <Style.HeroSection variants={MANUSCRIPT_VARIANTS} initial="initial" animate="animate">
        <Style.Subtitle>Phase 1: Experimental Research System</Style.Subtitle>
        <Style.PageTitle>The Technical Codex</Style.PageTitle>

        {/* ═══════════════════════════════════════════════════════════════════ */}
        {/* LAYER 1: WHAT IT IS (One sentence clarity) */}
        {/* ═══════════════════════════════════════════════════════════════════ */}
        <Style.LeadParagraph>
          <strong>
            INOS is an experimental shared-memory distributed runtime exploring post-HTTP
            architectures.
          </strong>
        </Style.LeadParagraph>

        {/* ═══════════════════════════════════════════════════════════════════ */}
        {/* LAYER 2: WHY IT MATTERS (Problem/Solution framing) */}
        {/* ═══════════════════════════════════════════════════════════════════ */}
        <ScrollReveal variant="fade">
          <p style={{ fontWeight: 500, color: '#2d2d2d', lineHeight: 1.7 }}>
            <strong>The Villain:</strong> Modern distributed systems spend over 60% of compute
            cycles on serialization and coordination. This is the <strong>Copy Tax</strong>, a
            silent drain on global energy. We spend more time translating data than processing it.
          </p>
          <p style={{ fontWeight: 500, color: '#2d2d2d', lineHeight: 1.7, marginTop: '1rem' }}>
            <strong>The Solution:</strong> INOS eliminates translation through a single shared
            buffer. Data flows like blood through a body, reaching Go, Rust, and TypeScript without
            the friction of copying. One reality, shared by all.
          </p>
          <p style={{ fontWeight: 500, color: '#2d2d2d', lineHeight: 1.7, marginTop: '1rem' }}>
            <strong>The Why:</strong> We seek the limit of connectivity. By removing the
            request-response cycle, we are creating a nervous system for a planetary-scale
            supercomputer. This is our research into a future where every device is a neuron in a
            global brain, synchronized at the speed of light.
          </p>
        </ScrollReveal>
        <br />

        {/* ═══════════════════════════════════════════════════════════════════ */}
        {/* LAYER 3: HOW IT WORKS (Live proof) */}
        {/* ═══════════════════════════════════════════════════════════════════ */}
        <ScrollReveal variant="fade">
          <p style={{ fontWeight: 500, color: '#2d2d2d' }}>
            Right now, in your browser,{' '}
            <strong>{stats.birdCount.toLocaleString()} autonomous agents</strong> are performing a
            collective ballet within a <strong>single shared buffer</strong>. Go orchestrates the
            policy. Rust executes the physics. JavaScript renders the reality. This is a nervous
            system for a new kind of machine.
          </p>
        </ScrollReveal>

        <Style.LiveStatsGrid>
          <Style.StatBox>
            <Style.MetricLabel>Throughput</Style.MetricLabel>
            <Style.MetricValue>
              <RollingCounter value={stats.opsPerSecond} />
            </Style.MetricValue>
            <Style.MetricUnit>Ops/s</Style.MetricUnit>
          </Style.StatBox>
          <Style.StatBox>
            <Style.MetricLabel>Active Entities</Style.MetricLabel>
            <Style.MetricValue>
              <RollingCounter value={stats.birdCount} />
            </Style.MetricValue>
            <Style.MetricUnit>Boids</Style.MetricUnit>
          </Style.StatBox>
          <Style.StatBox>
            <Style.MetricLabel>Sync Epoch</Style.MetricLabel>
            <Style.MetricValue>
              <RollingCounter value={stats.epoch} />
            </Style.MetricValue>
            <Style.MetricUnit>Ticks</Style.MetricUnit>
          </Style.StatBox>
        </Style.LiveStatsGrid>

        <ScrollReveal variant="manuscript">
          <h3>See It Working</h3>
          <p>
            The diagram below shows the actual memory layout. Go, Rust, and TypeScript all read from
            the same SharedArrayBuffer. When something changes, the system updates reality and rings
            a bell. Everyone listening hears the same bell at the same instant.
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>Live Memory Layout</Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <DimostrazioneStory />
            <Style.IllustrationCaption>
              SharedArrayBuffer memory pool. Pointers are swapped between Go, Rust, and TypeScript
              with zero serialization overhead.
            </Style.IllustrationCaption>
          </Style.IllustrationContainer>

          <p style={{ marginTop: '2rem', fontWeight: 500, color: '#2d2d2d' }}>
            The boids are proof of concept. The full stack includes:
          </p>
          <ul style={{ marginTop: '0.5rem', lineHeight: 1.8, color: '#2d2d2d' }}>
            <li>
              <strong>Go Kernel:</strong> Policy, scheduling, evolutionary learning
            </li>
            <li>
              <strong>Rust Modules:</strong> SIMD physics, cryptography, compression
            </li>
            <li>
              <strong>TypeScript Renderer:</strong> WebGPU, instanced rendering, 60fps
            </li>
          </ul>

          <p style={{ marginTop: '1.5rem' }}>
            All three share the same buffer. All three react to the same epoch signals. The
            Technical Codex documents how this works.
          </p>
        </ScrollReveal>
      </Style.HeroSection>

      <Style.TOCSection>
        <Style.SectionTitle>The Map of the Living Codex</Style.SectionTitle>
        <Style.TOCList variants={STAGGER_CONTAINER_VARIANTS} initial="initial" animate="animate">
          {CHAPTERS.map(chapter => (
            <Style.TOCItem key={chapter.path} variants={STAGGER_CHILD_VARIANTS}>
              <Style.TOCLink to={chapter.path}>
                <Style.ChapterNumber>{chapter.number}</Style.ChapterNumber>
                <Style.ChapterTitle>{chapter.title}</Style.ChapterTitle>
                <Style.ChapterDescription>{chapter.description}</Style.ChapterDescription>
              </Style.TOCLink>
            </Style.TOCItem>
          ))}
        </Style.TOCList>

        <ScrollReveal variant="fade">
          <Style.SectionDivider style={{ margin: '6rem 0 4rem' }} />
          <Style.ContentCard
            style={{
              background: 'rgba(139, 92, 246, 0.03)',
              border: '1px solid rgba(139, 92, 246, 0.15)',
            }}
          >
            <Style.Subtitle style={{ color: '#8b5cf6' }}>Advanced Technicals</Style.Subtitle>
            <Style.ChapterTitle>The Deep Dive Library</Style.ChapterTitle>
            <p style={{ color: '#4b5563', marginBottom: '2rem' }}>
              Explore the specific technical pillars that enable INOS to eliminate the 'Copy Tax'
              and achieve sub-10µs reactivity.
            </p>
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
                gap: '1rem',
              }}
            >
              <Style.DeepDiveLink to="/deep-dives/zero-copy" $color="#8b5cf6">
                <Style.DeepDiveTitle $color="#8b5cf6">Zero-Copy I/O</Style.DeepDiveTitle>
                <Style.DeepDiveDesc>Pointers over data copies</Style.DeepDiveDesc>
              </Style.DeepDiveLink>
              <Style.DeepDiveLink to="/deep-dives/signaling" $color="#dc2626">
                <Style.DeepDiveTitle $color="#dc2626">Epoch Signaling</Style.DeepDiveTitle>
                <Style.DeepDiveDesc>Hardware-latency reactivity</Style.DeepDiveDesc>
              </Style.DeepDiveLink>
              <Style.DeepDiveLink to="/deep-dives/atomics" $color="#8b5cf6">
                <Style.DeepDiveTitle $color="#8b5cf6">Atomics</Style.DeepDiveTitle>
                <Style.DeepDiveDesc>The indivisible units of logic</Style.DeepDiveDesc>
              </Style.DeepDiveLink>
              <Style.DeepDiveLink to="/deep-dives/mesh" $color="#16a34a">
                <Style.DeepDiveTitle $color="#16a34a">P2P Mesh</Style.DeepDiveTitle>
                <Style.DeepDiveDesc>Gossip + DHT + Reputation</Style.DeepDiveDesc>
              </Style.DeepDiveLink>
              <Style.DeepDiveLink to="/deep-dives/economy" $color="#f59e0b">
                <Style.DeepDiveTitle $color="#f59e0b">Economic Mesh</Style.DeepDiveTitle>
                <Style.DeepDiveDesc>Credits and storage tiers</Style.DeepDiveDesc>
              </Style.DeepDiveLink>
              <Style.DeepDiveLink to="/deep-dives/threads" $color="#00add8">
                <Style.DeepDiveTitle $color="#00add8">Supervisor Threads</Style.DeepDiveTitle>
                <Style.DeepDiveDesc>Genetic coordination</Style.DeepDiveDesc>
              </Style.DeepDiveLink>
              <Style.DeepDiveLink to="/deep-dives/graphics" $color="#ec4899">
                <Style.DeepDiveTitle $color="#ec4899">Graphics Pipeline</Style.DeepDiveTitle>
                <Style.DeepDiveDesc>WebGPU + instanced rendering</Style.DeepDiveDesc>
              </Style.DeepDiveLink>
              <Style.DeepDiveLink to="/deep-dives/database" $color="#10b981">
                <Style.DeepDiveTitle $color="#10b981">Storage & DB</Style.DeepDiveTitle>
                <Style.DeepDiveDesc>OPFS + BLAKE3 Addressing</Style.DeepDiveDesc>
              </Style.DeepDiveLink>
            </div>
          </Style.ContentCard>
        </ScrollReveal>
      </Style.TOCSection>

      <ChapterNav next={{ to: '/problem', title: '01. The Problem' }} />
    </Style.BlogContainer>
  );
}

export default Landing;
