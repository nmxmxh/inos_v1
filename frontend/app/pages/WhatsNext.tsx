/**
 * INOS Technical Codex — Chapter 7: The Moonshot (Cosmos)
 *
 * The grand finale. Demonstrating the planetary-scale computer.
 * Integrating N-Body physics and the Infinite Canvas.
 */

import { useCallback } from 'react';
import styled, { useTheme } from 'styled-components';
import * as d3 from 'd3';
import D3Container from '../ui/D3Container';
import { Style as ManuscriptStyle } from '../styles/manuscript';
import ChapterNav from '../ui/ChapterNav';
import ScrollReveal from '../ui/ScrollReveal';
import GlobalDashboard from '../features/analytics/GlobalDashboard';

const Style = {
  ...ManuscriptStyle,

  Container: styled.div`
    max-width: 900px;
    margin: 0 auto;
    padding: ${p => p.theme.spacing[10]} ${p => p.theme.spacing[6]};
    background: #f4f1ea;
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
      font-family: ${p => p.theme.fonts.main};
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }

    p {
      line-height: 1.75;
      margin-bottom: ${p => p.theme.spacing[4]};
    }

    p:last-child {
      margin-bottom: 0;
    }

    ul,
    ol {
      margin: ${p => p.theme.spacing[4]} 0;
      padding-left: ${p => p.theme.spacing[6]};
    }

    li {
      margin-bottom: ${p => p.theme.spacing[3]};
      line-height: 1.6;
    }
  `,

  HeroSection: styled.div`
    height: 60vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    padding: ${p => p.theme.spacing[10]};
    background: radial-gradient(circle at center, rgba(139, 92, 246, 0.05) 0%, transparent 70%);
  `,

  BigTitle: styled.h1`
    font-size: 64px;
    font-weight: 800;
    letter-spacing: -0.02em;
    margin: 0;
    background: linear-gradient(135deg, #1e293b 0%, #6d28d9 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  `,

  IllustrationContainer: styled.div`
    width: 100%;
    background: rgba(255, 255, 255, 0.92);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 8px;
    margin: ${p => p.theme.spacing[6]} 0;
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

  DefinitionBox: styled.div`
    background: rgba(139, 92, 246, 0.08);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(139, 92, 246, 0.2);
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[6]} 0;

    h4 {
      margin: 0 0 ${p => p.theme.spacing[2]} 0;
      color: #7c3aed;
      font-size: ${p => p.theme.fontSizes.lg};
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }

    p {
      margin: 0;
      line-height: 1.7;
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
    margin: ${p => p.theme.spacing[12]} 0;
  `,

  IllustrationCaption: styled.p`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    color: ${p => p.theme.colors.inkLight};
    text-align: center;
    padding: ${p => p.theme.spacing[3]};
    margin: 0;
    border-top: 1px solid ${p => p.theme.colors.borderSubtle};
    text-transform: uppercase;
    letter-spacing: 0.1em;
  `,

  ChapterNumber: styled.span`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    font-weight: 600;
    color: ${p => p.theme.colors.accent};
    text-transform: uppercase;
    letter-spacing: 0.15em;
  `,

  ChapterTitle: styled.h2`
    font-family: ${p => p.theme.fonts.main};
    font-size: 2rem;
    font-weight: 800;
    color: ${p => p.theme.colors.inkDark};
    text-transform: uppercase;
    letter-spacing: 0.05em;
  `,

  SectionTitle: styled.h3`
    font-family: ${p => p.theme.fonts.main};
    font-size: 12px;
    font-weight: 800;
    color: ${p => p.theme.colors.inkLight};
    text-transform: uppercase;
    letter-spacing: 0.15em;
    margin-bottom: ${p => p.theme.spacing[4]};
  `,
};

function SupercomputerDensityMap() {
  const theme = useTheme();

  const renderViz = useCallback(
    (
      svg: d3.Selection<SVGSVGElement, unknown, null, undefined>,
      _width: number,
      _height: number
    ) => {
      svg.selectAll('*').remove();

      const g = svg.append('g');

      // Static elements (Monolith)
      // --- LEFT SIDE: THE MONOLITH (CENTRALIZED) ---
      const monolithX = 120;
      const monolithY = 150;

      // Draw Rack
      g.append('rect')
        .attr('x', monolithX - 45)
        .attr('y', monolithY - 75)
        .attr('width', 90)
        .attr('height', 150)
        .attr('fill', '#1e293b')
        .attr('stroke', '#334155')
        .attr('stroke-width', 2)
        .attr('rx', 2);

      // Status lights
      for (let i = 0; i < 12; i++) {
        g.append('circle')
          .attr('cx', monolithX - 35)
          .attr('cy', monolithY - 66 + i * 12)
          .attr('r', 1.5)
          .attr('fill', '#ef4444')
          .append('animate')
          .attr('attributeName', 'opacity')
          .attr('values', '0.2;1;0.2')
          .attr('dur', `${0.5 + Math.random()}s`)
          .attr('repeatCount', 'indefinite');
      }

      g.append('text')
        .attr('x', monolithX)
        .attr('y', monolithY + 100)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('font-weight', 800)
        .attr('fill', '#ef4444')
        .text('CENTRALIZED LIMIT');

      // --- RIGHT SIDE: THE SPIDER'S MESH ---
      const meshCenterX = 600;
      const meshCenterY = 150;
      const meshGroup = g.append('g');
      const particleGroup = g.append('g');

      interface HubNode {
        x: number;
        y: number;
        r: number;
        isPrimary?: boolean;
      }

      const hubs: HubNode[] = [
        { x: meshCenterX, y: meshCenterY, r: 0, isPrimary: true },
        { x: meshCenterX - 60, y: meshCenterY - 60, r: 0 },
        { x: meshCenterX + 70, y: meshCenterY - 40, r: 0 },
        { x: meshCenterX - 50, y: meshCenterY + 70, r: 0 },
        { x: meshCenterX + 60, y: meshCenterY + 60, r: 0 },
      ];

      // Create persistent rings and spokes to avoid frame-by-frame DOM churn
      interface RingDataItem {
        hub: HubNode;
        ri: number;
      }
      const ringData: RingDataItem[] = hubs.flatMap((hub: HubNode) =>
        [0, 1, 2].map(ri => ({ hub, ri }))
      );

      const rings = meshGroup
        .selectAll<SVGCircleElement, RingDataItem>('.mesh-ring')
        .data(ringData)
        .enter()
        .append('circle')
        .attr('class', 'mesh-ring')
        .attr('cx', (d: RingDataItem) => d.hub.x)
        .attr('cy', (d: RingDataItem) => d.hub.y)
        .attr('fill', 'none')
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 0.5);

      interface SpokeDataItem {
        hub: HubNode;
        angleOffset: number;
      }
      const spokeData: SpokeDataItem[] = hubs.flatMap((hub: HubNode) =>
        [0, 1, 2, 3, 4, 5, 6, 7].map(si => ({
          hub,
          angleOffset: (si / 8) * Math.PI * 2,
        }))
      );

      const spokeLines = meshGroup
        .selectAll<SVGLineElement, SpokeDataItem>('.mesh-spoke')
        .data(spokeData)
        .enter()
        .append('line')
        .attr('class', 'mesh-spoke')
        .attr('x1', (d: SpokeDataItem) => d.hub.x)
        .attr('y1', (d: SpokeDataItem) => d.hub.y)
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 0.3)
        .style('opacity', 0.1);

      // Hub nodes
      meshGroup
        .selectAll<SVGCircleElement, HubNode>('.hub-node')
        .data(hubs)
        .enter()
        .append('circle')
        .attr('class', 'hub-node')
        .attr('cx', (d: HubNode) => d.x)
        .attr('cy', (d: HubNode) => d.y)
        .attr('r', 3)
        .attr('fill', '#8b5cf6')
        .style('opacity', 0.8);

      let animId: number;
      let timer = 0;

      function animateMesh() {
        timer += 0.01;

        rings
          .attr('r', (d: RingDataItem) => ((timer + d.ri * 0.5) % 1.5) * 60)
          .style('opacity', (d: RingDataItem) => (1 - ((timer + d.ri * 0.5) % 1.5) / 1.5) * 0.3);

        spokeLines
          .attr('x2', (d: SpokeDataItem) => d.hub.x + Math.cos(d.angleOffset + timer * 0.2) * 100)
          .attr('y2', (d: SpokeDataItem) => d.hub.y + Math.sin(d.angleOffset + timer * 0.2) * 100);

        if (Math.random() > 0.95) {
          const startHub = hubs[Math.floor(Math.random() * hubs.length)];
          const endHub = hubs[Math.floor(Math.random() * hubs.length)];
          if (startHub !== endHub) {
            const pulse = particleGroup
              .append('circle')
              .attr('r', 2)
              .attr('fill', '#8b5cf6')
              .attr('cx', startHub.x)
              .attr('cy', startHub.y);

            pulse
              .transition()
              .duration(1000)
              .attr('cx', endHub.x)
              .attr('cy', endHub.y)
              .style('opacity', 0)
              .on('end', () => pulse.remove());
          }
        }

        animId = requestAnimationFrame(animateMesh);
      }
      animateMesh();

      g.append('text')
        .attr('x', meshCenterX)
        .attr('y', meshCenterY + 125)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 800)
        .attr('fill', '#8b5cf6')
        .text("SPIDER'S MESH: EVERY EDGE IS A CENTER");

      g.append('text')
        .attr('x', meshCenterX)
        .attr('y', meshCenterY + 140)
        .attr('text-anchor', 'middle')
        .attr('font-size', 8)
        .attr('fill', theme.colors.inkMedium)
        .text('Continuous Expansion & Decentralized Search');

      // --- DIVIDER & CONTRAST FLOW ---
      const sourceX = 370;
      const sourceY = 150;

      const flowCount = 6;
      for (let i = 0; i < flowCount; i++) {
        // Monolith flow
        g.append('circle')
          .attr('r', 2)
          .attr('fill', '#ef4444')
          .append('animateMotion')
          .attr('path', `M ${sourceX + 30} ${sourceY} L ${monolithX + 45} ${sourceY}`)
          .attr('dur', '2.5s')
          .attr('begin', `${i * 1.5}s`)
          .attr('repeatCount', 'indefinite');

        // Mesh flow - floods all main hubs
        hubs.forEach((hub, hi) => {
          g.append('circle')
            .attr('r', 2)
            .attr('fill', '#8b5cf6')
            .append('animateMotion')
            .attr('path', `M ${sourceX + 30} ${sourceY} L ${hub.x} ${hub.y}`)
            .attr('dur', '0.8s')
            .attr('begin', `${i * 0.2 + hi * 0.1}s`)
            .attr('repeatCount', 'indefinite');
        });
      }

      svg
        .append('line')
        .attr('x1', 400)
        .attr('y1', 50)
        .attr('x2', 400)
        .attr('y2', 250)
        .attr('stroke', theme.colors.borderSubtle)
        .attr('stroke-dasharray', '4,4');

      return () => {
        cancelAnimationFrame(animId);
        svg.selectAll('*').interrupt().remove();
      };
    },
    [theme]
  );

  // Using key to force re-render when theme changes if needed, though dependency array handles it.
  // We use D3Container and pass the cleanup function returned by renderViz if any (but here we return the cleanup from useCallback itself)
  // Wait, D3Container expects render to return void or cleanup.

  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 800 300" height={300} />
  );
}

function RoadmapTimeline() {
  const theme = useTheme();

  const renderViz = useCallback(
    (
      svg: d3.Selection<SVGSVGElement, unknown, null, undefined>,
      width: number,
      _height: number
    ) => {
      svg.selectAll('*').remove();

      const milestones = [
        { label: 'SAB Signaling', status: 'done', color: '#16a34a' },
        { label: 'Storage Mesh', status: 'done', color: '#16a34a' },
        { label: 'Economic Layer', status: 'done', color: '#16a34a' },
        { label: 'PoUW Consensus', status: 'next', color: '#8b5cf6' },
        { label: 'Planetary Sim', status: 'next', color: '#8b5cf6' },
      ];

      const startX = 50;
      const step = (width - 100) / (milestones.length - 1);
      const y = 80;

      // Line
      svg
        .append('line')
        .attr('x1', startX)
        .attr('y1', y)
        .attr('x2', startX + (milestones.length - 1) * step)
        .attr('y2', y)
        .attr('stroke', theme.colors.borderSubtle)
        .attr('stroke-width', 2);

      milestones.forEach((m, i) => {
        const x = startX + i * step;

        // Node
        svg
          .append('circle')
          .attr('cx', x)
          .attr('cy', y)
          .attr('r', 6)
          .attr('fill', m.status === 'done' ? m.color : '#fff')
          .attr('stroke', m.color)
          .attr('stroke-width', 2);

        // Label
        svg
          .append('text')
          .attr('x', x)
          .attr('y', y + 25)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9)
          .attr('font-weight', 600)
          .attr('fill', theme.colors.inkDark)
          .text(m.label);

        // Status
        svg
          .append('text')
          .attr('x', x)
          .attr('y', y - 15)
          .attr('text-anchor', 'middle')
          .attr('font-size', 7)
          .attr('font-weight', 800)
          .attr('fill', m.color)
          .text(m.status === 'done' ? '✓ DONE' : '🚀 NEXT');
      });
    },
    [theme]
  );

  // viewBox 600 width is consistent with previous code
  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 600 150" height={150} />
  );
}

/**
 * Illustration: The Autonomous Stream
 * Visualizing the flow from Sensors -> SAB -> AI -> Actuators
 */
function TheAutonomousStream() {
  const theme = useTheme();

  const renderViz = useCallback(
    (svg: d3.Selection<SVGSVGElement, unknown, null, undefined>, width: number, height: number) => {
      svg.selectAll('*').remove();

      const centerY = height / 2;

      const g = svg.append('g');

      // 1. Sources (Left)
      const sources = [
        { id: 'lidar', label: 'LIDAR', y: centerY - 60 },
        { id: 'cam', label: 'VISION', y: centerY },
        { id: 'bio', label: 'BIO-SENSORS', y: centerY + 60 },
      ];

      // 2. The Core (Center)
      const core = { x: width / 2, y: centerY, label: 'SAB HIVE MIND' };

      // 3. The Outputs (Right)
      const outputs = [
        { id: 'vr', label: 'HOLOGRAMS', y: centerY - 60 },
        { id: 'drone', label: 'SWARMS', y: centerY },
        { id: 'car', label: 'AUTONOMY', y: centerY + 60 },
      ];

      // Draw lines first (back layer)
      sources.forEach(s => {
        g.append('path')
          .attr('d', `M 150 ${s.y} C 250 ${s.y}, 300 ${centerY}, ${width / 2 - 40} ${centerY}`)
          .attr('fill', 'none')
          .attr('stroke', '#e2e8f0')
          .attr('stroke-width', 1);
      });

      outputs.forEach(o => {
        g.append('path')
          .attr('d', `M ${width / 2 + 40} ${centerY} C 500 ${centerY}, 550 ${o.y}, 650 ${o.y}`)
          .attr('fill', 'none')
          .attr('stroke', '#e2e8f0')
          .attr('stroke-width', 1);
      });

      // Draw Source Nodes
      const sourceGroups = g
        .selectAll('.source')
        .data(sources)
        .enter()
        .append('g')
        .attr('transform', d => `translate(150, ${d.y})`);

      sourceGroups.append('circle').attr('r', 4).attr('fill', '#64748b');
      sourceGroups
        .append('text')
        .attr('x', -15)
        .attr('dy', 4)
        .attr('text-anchor', 'end')
        .attr('font-size', 9)
        .attr('font-weight', 700)
        .attr('fill', '#64748b')
        .text(d => d.label);

      // Draw Output Nodes
      const outputGroups = g
        .selectAll('.output')
        .data(outputs)
        .enter()
        .append('g')
        .attr('transform', d => `translate(650, ${d.y})`);

      outputGroups.append('circle').attr('r', 4).attr('fill', '#ec4899');
      outputGroups
        .append('text')
        .attr('x', 15)
        .attr('dy', 4)
        .attr('text-anchor', 'start')
        .attr('font-size', 9)
        .attr('font-weight', 700)
        .attr('fill', '#ec4899')
        .text(d => d.label);

      // Draw Core
      g.append('circle')
        .attr('cx', core.x)
        .attr('cy', core.y)
        .attr('r', 40)
        .attr('fill', '#fff')
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 2);

      g.append('text')
        .attr('x', core.x)
        .attr('y', core.y + 4)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 800)
        .attr('fill', '#8b5cf6')
        .text('INOS KERNEL');

      // Particles
      let timer: ReturnType<typeof setInterval>;

      function emitParticle() {
        const source = sources[Math.floor(Math.random() * sources.length)];
        const output = outputs[Math.floor(Math.random() * outputs.length)];

        const p = g.append('circle').attr('r', 2).attr('fill', '#8b5cf6').attr('opacity', 0);

        // Path 1: Source -> Core
        p.attr('transform', `translate(150, ${source.y})`)
          .transition()
          .duration(1000)
          .ease(d3.easeLinear)
          .attr('opacity', 1)
          .attrTween('transform', () => t => {
            const endX = width / 2 - 40;
            // simplify: linear move for now to ensure robustness without path ref
            const currX = 150 + (endX - 150) * t;
            const currY = source.y + (centerY - source.y) * t;
            // Add a slight curve
            const curveY = currY + Math.sin(t * Math.PI) * (centerY - source.y) * 0.5;
            return `translate(${currX}, ${curveY})`;
          })
          .on('end', () => {
            // Path 2: Core -> Output
            p.transition()
              .duration(1000)
              .ease(d3.easeQuadOut)
              .attrTween('transform', () => t => {
                const startX = width / 2 + 40;
                const endX = 650;
                const currX = startX + (endX - startX) * t;
                const currY = centerY + (output.y - centerY) * t;
                // Add a slight curve
                const curveY = currY + Math.sin(t * Math.PI) * (output.y - centerY) * 0.5;

                return `translate(${currX}, ${curveY})`;
              })
              .attr('fill', '#ec4899') // Change color to output
              .on('end', () => p.remove());
          });
      }

      timer = setInterval(emitParticle, 100);
      return () => clearInterval(timer);
    },
    [theme]
  );

  // viewBox 800 width consistent with original
  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 800 300" height={300} />
  );
}

export default function WhatsNext() {
  const theme = useTheme();

  return (
    <Style.Container>
      <ScrollReveal>
        <Style.HeroSection>
          <Style.BigTitle>WHAT'S NEXT</Style.BigTitle>
          <p style={{ maxWidth: '600px', margin: '20px auto', color: theme.colors.inkMedium }}>
            The architecture is proven. The primitives are live. What remains is the hardest part:
            turning theory into infrastructure that people depend on.
          </p>
        </Style.HeroSection>
      </ScrollReveal>
      <Style.SectionDivider />

      <ScrollReveal>
        <Style.ContentCard>
          <h3>Where We Stand</h3>
          <p>
            You've seen the primitives: Zero-Copy for speed, Epoch Signaling for coordination, and
            Mesh for scale. These aren't theoretical—they're running now. The boids you saw on the
            landing page? That's Go, Rust, and TypeScript reading the same memory, updating at 60fps
            without a single serialization call.
          </p>
          <p>
            But a prototype is not a product. The question is no longer "does this work?" but "what
            can we build with it?" The answer depends on who's asking.
          </p>

          <Style.DefinitionBox>
            <h4>For Builders</h4>
            <p>
              INOS gives you a shared-memory runtime that works like native code. Write once, run
              everywhere, scale automatically. No cloud vendor negotiation. No cold starts. No
              serialization overhead. Your compute is as close to the user as their browser.
            </p>
          </Style.DefinitionBox>
        </Style.ContentCard>
      </ScrollReveal>

      <ScrollReveal>
        <div style={{ margin: '40px 0' }}>
          <GlobalDashboard />
        </div>
      </ScrollReveal>

      <ScrollReveal>
        <Style.ContentCard>
          <h3>The Scale Gap</h3>
          <p>
            Traditional supercomputers are limited by the physical space they occupy. INOS is
            limited only by the number of connected nodes. We are not building a cloud: we are
            building a <strong>planetary-scale shared memory</strong>.
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>Scale Comparison: Monolith vs Swarm</Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <SupercomputerDensityMap />
          </Style.IllustrationContainer>

          <p>
            The INOS mesh is a living, continuously expanding environment where information diffuses
            rather than queues. We track three primary horizons of emergence:
          </p>
          <ul>
            <li>
              <strong>1,000 Nodes: The Regional Hive.</strong> Low-latency shared state for local
              AR, distributed robotics, and zero-trust neighborhood compute.
            </li>
            <li>
              <strong>100,000 Nodes: The Continental Mesh.</strong> A collective intelligence
              capable of real-time climate modeling, pandemic tracing, and large-scale economic
              simulations without a single central authority.
            </li>
            <li>
              <strong>1,000,000 Nodes: Planetary Fidelity.</strong> A world-substrate where the
              distinction between local memory and global reality dissolves entirely.
            </li>
          </ul>

          <p>
            For <strong>AI/ML Collectives</strong>, this means bypassing the cloud monopoly to
            access idle compute at a fraction of the cost. For <strong>Web3 Infrastructure</strong>,
            it is a runtime that behaves like local memory, removing the friction of a thousand
            bridges.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.SectionDivider />

      <ScrollReveal>
        <Style.ContentCard>
          <h3>The Roadmap: To Universum and Beyond</h3>
          <p>
            The path ahead graduates the network into a living computer. We are moving from
            <strong>collaborative simulations</strong> (10-100 nodes) to{' '}
            <strong>regional meshes</strong>
            (1k-10k) and finally to <strong>selective global workloads</strong>.
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>Fig_07 // The Living Roadmap</Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <RoadmapTimeline />
          </Style.IllustrationContainer>

          <p>
            Our current objective is <strong>Proof-of-Useful-Work (PoUW)</strong>. Instead of
            wasting cycles on arbitrary hashes, INOS nodes solve real physics and ML problems to
            verify their contribution and earn yield. This turns global compute into a market:
            efficient, verifiable, and accessible by every builder on the edge.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.SectionDivider />

      <ScrollReveal>
        <Style.ContentCard>
          <h3>The Bridge: Sensor to Action</h3>
          <p>
            The ultimate test of any architecture is not speed—it's agency. Can the system act on
            what it learns? INOS is designed for closed-loop systems: sensors write to SAB, AI
            processes in real-time, actuators respond instantly. No request-response cycle. No
            waiting for the cloud.
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>Fig_08 // The Autonomous Bridge</Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <TheAutonomousStream />
          </Style.IllustrationContainer>

          <p>
            This is the architecture for autonomous systems: drone swarms that think collectively,
            robotic arms that share muscle memory, AR layers that render reality in sync. The
            latency is measured in microseconds, not milliseconds.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      {/* WHY THIS IS NOW POSSIBLE */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Why This Is Now Possible</h3>
          <p>
            The technologies that make INOS feasible didn't exist a decade ago. Four convergent
            forces unlocked this architecture:
          </p>
          <ul>
            <li>
              <strong>SharedArrayBuffer Maturity:</strong> Cross-origin isolation made shared memory
              safe and stable in the modern browser runtime.
            </li>
            <li>
              <strong>WASM SIMD Stability:</strong> Browser VMs now support 128-bit vector
              operations, enabling near-native physics and ML execution.
            </li>
            <li>
              <strong>Thread Model Evolution:</strong> Web Workers and Worklets provide the
              necessary parallelism to drive complex kernels without UI lag.
            </li>
            <li>
              <strong>The Resource Surplus:</strong> Billions of devices sit idle. INOS is a
              correction that turns this surplus into a productive global computer.
            </li>
          </ul>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.SectionDivider />

      <ScrollReveal>
        <Style.ContentCard>
          <Style.ChapterNumber style={{ marginBottom: '1rem', display: 'block' }}>
            Conclusion
          </Style.ChapterNumber>
          <Style.ChapterTitle style={{ fontSize: '2rem', marginBottom: '1.5rem' }}>
            From Codex to Practice
          </Style.ChapterTitle>
          <p>
            This Codex documented the architecture. The primitives are proven: zero-copy, epoch
            signaling, distributed mesh. What remains is application. Building the tools that turn
            shared memory into shared capability.
          </p>
          <p>
            The next phase is not about faster benchmarks or bigger node counts. It's about the
            first application that couldn't exist without this architecture. The first AI collective
            that thinks across browser tabs. The first game world that persists without servers. The
            first sensor network that reasons in real-time.
          </p>
          <Style.DefinitionBox
            style={{
              background: 'rgba(30, 41, 59, 0.05)',
              border: '1px solid rgba(30, 41, 59, 0.1)',
              marginTop: '2rem',
            }}
          >
            <h4 style={{ color: '#1e293b' }}>The Invitation</h4>
            <p>
              If you've read this far, you understand the architecture. The question is: what will
              you build with it? The substrate is ready. The nervous system is live. The only
              missing piece is your application.
            </p>
          </Style.DefinitionBox>
          <p
            style={{
              textAlign: 'center',
              marginTop: '60px',
              fontSize: '24px',
              fontStyle: 'italic',
              color: theme.colors.inkMedium,
            }}
          >
            "Knowing is not enough; we must apply."
          </p>
          <p
            style={{
              textAlign: 'center',
              fontSize: '12px',
              fontWeight: 800,
              color: theme.colors.inkMedium,
              marginTop: '10px',
              letterSpacing: '0.2em',
            }}
          >
            — LEONARDO DA VINCI
          </p>
          <p
            style={{
              textAlign: 'center',
              fontSize: '10px',
              marginTop: '40px',
              color: theme.colors.inkLight,
              fontFamily: theme.fonts.typewriter,
            }}
          >
            CODEX VOL. 1 COMPLETE // BUILD BEGINS
          </p>
        </Style.ContentCard>
      </ScrollReveal>
      <ChapterNav prev={{ title: '04. History', to: '/history' }} />
    </Style.Container>
  );
}
