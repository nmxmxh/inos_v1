/**
 * INOS Technical Codex — Landing Page
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
      'Global data centers consume 200+ TWh annually. Bitcoin mining alone drains 150 TWh for 7 transactions per second. We have accepted the Copy Tax as the cost of doing business. But what if 99% of that energy is pure waste?',
  },
  {
    number: '02',
    title: 'The Insight',
    path: '/insight',
    description:
      'What if data could move between threads without copying? What if every browser became a compute node? WebAssembly, WebGPU, WebRTC, SharedArrayBuffer. The pieces exist. Someone just had to wire them together.',
  },
  {
    number: '03',
    title: 'The Architecture',
    path: '/architecture',
    description:
      'A tri-layer stack where Go orchestrates, Rust executes, and JavaScript renders. Built on a zero-copy build pipeline synchronized by binary schemas. Pure shared reality.',
  },
  {
    number: '04',
    title: 'Genesis',
    path: '/genesis',
    description:
      'The 30-year legacy of message-passing has cost us 60% of CPU cycles in translation alone. INOS is a technical correction—a return to Distributed Shared Memory.',
  },
  {
    number: '05',
    title: 'The Cosmos',
    path: '/cosmos',
    description:
      'The moonshot: a planetary-scale supercomputer. Our roadmap leads to a million browsers simulating galaxies in real-time. Fidelity scales with the network.',
  },
];

function useLiveStats() {
  const [stats, setStats] = useState({
    opsPerSecond: 0,
    birdCount: 1000,
    epoch: 0,
  });

  useEffect(() => {
    const sab = (window as any).__INOS_SAB__;
    if (!sab) return;

    let lastEpoch = 0;
    const interval = setInterval(() => {
      try {
        const flags = new Int32Array(sab, 0, 32);
        const epoch = Atomics.load(flags, IDX_BIRD_EPOCH);
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
        <Style.Subtitle>The Internet-Native Operating System</Style.Subtitle>
        <Style.PageTitle>The Technical Codex</Style.PageTitle>

        <Style.LeadParagraph>
          The <strong>Architecture of Waste</strong> has defined the last decade of systems. We have
          accepted high-latency serialization, expensive memory copies, and centralized computation
          as the "cost of doing business." INOS is the <strong>rejection</strong> of that tax. A
          distributed runtime where computing becomes a <strong>living system</strong>.
        </Style.LeadParagraph>

        <ScrollReveal variant="fade">
          <p style={{ fontWeight: 500, color: '#2d2d2d' }}>
            Right now, in your browser,{' '}
            <strong>{stats.birdCount.toLocaleString()} autonomous agents</strong> are performing a
            collective ballet within a <strong>Single Unified Memory Space</strong>. No
            serialization. No copies. Not a simulation. A <strong>Biological Runtime</strong>.
          </p>
        </ScrollReveal>

        <Style.LiveStatsGrid>
          <Style.StatBox>
            <Style.MetricLabel>Circulatory Throughput</Style.MetricLabel>
            <Style.MetricValue>
              <RollingCounter value={stats.opsPerSecond} />
            </Style.MetricValue>
            <Style.MetricUnit>Ops/s</Style.MetricUnit>
          </Style.StatBox>
          <Style.StatBox>
            <Style.MetricLabel>Active Entity Fidelity</Style.MetricLabel>
            <Style.MetricValue>
              <RollingCounter value={stats.birdCount} />
            </Style.MetricValue>
            <Style.MetricUnit>Boids</Style.MetricUnit>
          </Style.StatBox>
          <Style.StatBox>
            <Style.MetricLabel>Temporal Synchronization</Style.MetricLabel>
            <Style.MetricValue>
              <RollingCounter value={stats.epoch} />
            </Style.MetricValue>
            <Style.MetricUnit>Epochs</Style.MetricUnit>
          </Style.StatBox>
        </Style.LiveStatsGrid>

        <ScrollReveal variant="manuscript">
          <h3>The Dimostrazione</h3>
          <p>
            Leonardo Da Vinci believed in <em>Sapere Vedere</em>—"Knowing How to See." The boids
            moving in the background are live evidence: a system that rejects data redundancy in
            favor of <strong>shared presence</strong>.
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>Live_Ref_01 // The Circulatory Mesh</Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <DimostrazioneStory />
            <Style.IllustrationCaption>
              Live visualization of the SharedArrayBuffer memory pool. Pointers are swapped between
              Go, Rust, and TS governance layers with zero serialization overhead.
            </Style.IllustrationCaption>
          </Style.IllustrationContainer>

          <p style={{ marginTop: '2rem', fontWeight: 500, color: '#2d2d2d' }}>
            The boids are proof of concept. The full stack goes deeper: a{' '}
            <strong>Go Orchestrator</strong> for policy and evolutionary learning, a{' '}
            <strong>Rust Engine</strong> for SIMD-accelerated physics, and a{' '}
            <strong>TypeScript Sensory Layer</strong> for concurrent rendering. All three run in
            WebAssembly, wired through the same shared buffer, synchronized by atomic epochs.
          </p>

          <p style={{ marginTop: '1.5rem' }}>
            The Technical Codex documents this journey of building a system that treats{' '}
            <strong>computing as a living organism</strong>.
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
      </Style.TOCSection>

      <ChapterNav next={{ to: '/problem', title: '01. The Problem' }} />
    </Style.BlogContainer>
  );
}

export default Landing;
