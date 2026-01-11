/**
 * INOS Technical Codex — Chapter 7: The Moonshot (Cosmos)
 *
 * The grand finale. Demonstrating the planetary-scale computer.
 * Integrating N-Body physics, Robotics protocols, and the Infinite Canvas.
 */

import { useEffect, useRef, useMemo } from 'react';
import styled, { useTheme } from 'styled-components';
import * as d3 from 'd3';
import { Style as ManuscriptStyle } from '../styles/manuscript';
import ChapterNav from '../ui/ChapterNav';
import ScrollReveal from '../ui/ScrollReveal';
import GlobalDashboard from '../features/analytics/GlobalDashboard';
import { Canvas } from '@react-three/fiber';
import { OrbitControls, Float } from '@react-three/drei';
import { EffectComposer, Bloom, Noise } from '@react-three/postprocessing';
import { MorphicLattice } from '../features/robot/MorphicLattice';
import { useLatticeState } from '../features/robot/useLatticeState';
import { useGlobalAnalytics } from '../features/analytics/useGlobalAnalytics';
import RollingCounter from '../ui/RollingCounter';

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
    margin: ${p => p.theme.spacing[10]} 0;
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

  GalaxyContainer: styled.div`
    height: 600px;
    background: #ffffff;
    border-radius: 8px;
    position: relative;
    overflow: hidden;
    margin: ${p => p.theme.spacing[8]} 0;
    box-shadow: 0 20px 50px rgba(0, 0, 0, 0.5);

    &::after {
      content: '';
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      box-shadow: inset 0 0 100px rgba(0, 0, 0, 0.8);
      pointer-events: none;
    }
  `,

  StatBadge: styled.div`
    position: absolute;
    top: 20px;
    right: 20px;
    background: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(8px);
    border: 1px solid rgba(255, 255, 255, 0.2);
    padding: 12px 20px;
    border-radius: 4px;
    color: #fff;
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    z-index: 10;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    display: flex;
    flex-direction: column;
    gap: 4px;

    span {
      color: #8b5cf6;
      font-weight: 800;
    }
  `,
};

function SupercomputerDensityMap() {
  const svgRef = useRef<SVGSVGElement>(null);
  const theme = useTheme();

  useEffect(() => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const g = svg.append('g');

    let animId: number;
    let timer = 0;

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
  }, [theme]);

  return <svg ref={svgRef} viewBox="0 0 800 300" style={{ width: '100%', height: 'auto' }} />;
}

function RoboticProtocolBridge() {
  const svgRef = useRef<SVGSVGElement>(null);
  const theme = useTheme();

  useEffect(() => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();
    const timeouts: any[] = [];

    const g = svg.append('g').attr('transform', 'translate(50, 40)');

    // Vertical pipeline: Hardware -> SDK -> SAB -> Mesh -> Coordination
    const stages = [
      { id: 'hw', label: 'HARDWARE (SENSORS/ACTUATORS)', color: '#64748b' },
      { id: 'proto', label: 'PROTOCOL (MAVLINK/ROS2)', color: '#64748b' },
      { id: 'sab', label: 'SAB ZERO-COPY HUB', color: '#8b5cf6', highlight: true },
      { id: 'mesh', label: 'GLOBAL MESH GOSSIP', color: '#10b981' },
      { id: 'coord', label: 'SWARM COORDINATION (P2P)', color: '#f59e0b' },
    ];

    stages.forEach((s, i) => {
      const y = i * 60;

      // Box
      g.append('rect')
        .attr('x', 150)
        .attr('y', y)
        .attr('width', 300)
        .attr('height', 40)
        .attr('rx', 4)
        .attr('fill', s.highlight ? `${s.color}15` : '#f8fafc')
        .attr('stroke', s.color)
        .attr('stroke-width', s.highlight ? 2 : 1);

      g.append('text')
        .attr('x', 300)
        .attr('y', y + 25)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('font-weight', 800)
        .attr('fill', s.color)
        .text(s.label);

      // Connectors
      if (i < stages.length - 1) {
        g.append('line')
          .attr('x1', 300)
          .attr('y1', y + 40)
          .attr('x2', 300)
          .attr('y2', i === 1 ? y + 60 : y + 60)
          .attr('stroke', theme.colors.borderSubtle)
          .attr('stroke-width', 1);
      }
    });

    // Swarm Visualization (Right side)
    const vehicles = d3.range(4).map(i => ({
      id: i,
      x: 550,
      y: 50 + i * 60,
    }));

    vehicles.forEach(v => {
      const vehicleGroup = g.append('g').attr('transform', `translate(${v.x}, ${v.y})`);

      // Drone/Car Icon
      vehicleGroup.append('path').attr('d', 'M -15 -8 L 15 -8 L 0 15 Z').attr('fill', '#1e293b');

      // Sync lines to "Coordination" box
      g.append('path')
        .attr('d', `M ${v.x - 20} ${v.y} Q ${v.x - 50} ${v.y} 450 ${260}`)
        .attr('fill', 'none')
        .attr('stroke', '#f59e0b')
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '4,2')
        .style('opacity', 0.4);

      // Data pulse from Hardware
      const pulse = g.append('circle').attr('r', 3).attr('fill', '#8b5cf6').style('opacity', 0);

      function runPulse() {
        pulse
          .style('opacity', 1)
          .attr('cx', 150)
          .attr('cy', 20)
          .transition()
          .duration(2000)
          .attr('cy', 140) // SAB
          .transition()
          .duration(1000)
          .attr('cy', 200) // Mesh
          .transition()
          .duration(1000)
          .attr('cx', 300)
          .attr('cy', 260) // Coord
          .transition()
          .duration(1500)
          .attr('cx', v.x)
          .attr('cy', v.y)
          .on('end', runPulse);
      }
      const timeoutId = setTimeout(runPulse, v.id * 1000);
      timeouts.push(timeoutId);
    });

    g.append('text')
      .attr('x', 550)
      .attr('y', 20)
      .attr('text-anchor', 'middle')
      .attr('font-size', 8)
      .attr('font-weight', 600)
      .attr('fill', theme.colors.inkMedium)
      .text('DISTRIBUTED SWARM');

    return () => {
      timeouts.forEach(clearTimeout);
      svg.selectAll('*').interrupt().remove();
    };
  }, [theme]);

  return <svg ref={svgRef} viewBox="0 0 750 350" style={{ width: '100%', height: 'auto' }} />;
}

function RoadmapTimeline() {
  const svgRef = useRef<SVGSVGElement>(null);
  const theme = useTheme();

  useEffect(() => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const milestones = [
      { label: 'SAB Signaling', status: 'done', color: '#16a34a' },
      { label: 'Storage Mesh', status: 'done', color: '#16a34a' },
      { label: 'Economic Layer', status: 'done', color: '#16a34a' },
      { label: 'PoUW Consensus', status: 'next', color: '#8b5cf6' },
      { label: 'Planetary Sim', status: 'next', color: '#8b5cf6' },
    ];

    const width = 600;
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
  }, [theme]);

  return <svg ref={svgRef} viewBox="0 0 600 150" style={{ width: '100%', height: 'auto' }} />;
}

const LatticeTelemetry = () => {
  const { metrics } = useLatticeState();
  const globalAnalytics = useGlobalAnalytics();
  const activeNodes = globalAnalytics?.activeNodeCount ?? 1;

  const phaseName = useMemo(() => {
    switch (metrics?.phase) {
      case 0:
        return 'ENTROPIC (INIT)';
      case 1:
        return 'EMERGENT (SEEKING)';
      case 2:
        return 'MORPHIC (LOCKED)';
      default:
        return 'CONNECTING...';
    }
  }, [metrics?.phase]);

  return (
    <Style.StatBadge>
      SIMULATION: <span>{phaseName}</span>
      {activeNodes === 1 ? 'NODE' : 'NODES'}:{' '}
      <span>
        <RollingCounter value={activeNodes} decimals={0} />
      </span>
      SYNTROPY:{' '}
      <span>
        <RollingCounter value={metrics ? metrics.syntropy : 0} decimals={4} />
      </span>
    </Style.StatBadge>
  );
};

export default function Cosmos() {
  const theme = useTheme();

  return (
    <Style.Container>
      <ScrollReveal>
        <Style.HeroSection>
          <Style.BigTitle>THE MOONSHOT</Style.BigTitle>
          <p style={{ maxWidth: '600px', margin: '20px auto', color: theme.colors.inkMedium }}>
            "One more thing. We've bridged the Persistence Paradox. We've made the web fast. We've
            made it permanent. But why? To compute the impossible."
          </p>
        </Style.HeroSection>
      </ScrollReveal>
      <Style.SectionDivider />
      <ScrollReveal>
        <Style.ContentCard>
          <h3>The Morphic Lattice</h3>
          <p>
            We stopped trying to build a robot that looks like a human. That was thinking too small.
          </p>
          <p>
            The <strong>Morphic Lattice</strong> is the shape of this planetary computer. It
            connects thousands of isolated devices into a single, breathing organism. No wires. Just
            pure math holding them together in a living geometry.
          </p>

          <Style.GalaxyContainer>
            <LatticeTelemetry />
            <Canvas camera={{ position: [0, 8, 12], fov: 60 }} dpr={[1, 2]}>
              <color attach="background" args={['#ffffff']} />
              <fog attach="fog" args={['#ffffff', 10, 30]} />
              <ambientLight intensity={0.8} />
              <OrbitControls
                makeDefault
                enableZoom={true}
                enablePan={false}
                autoRotate
                autoRotateSpeed={0.5}
                minDistance={5}
                maxDistance={18}
                enableDamping
                dampingFactor={0.05}
                zoomSpeed={0.4}
              />
              <Float speed={2} rotationIntensity={0.5} floatIntensity={0.5}>
                <MorphicLattice />
              </Float>
              <EffectComposer enableNormalPass={false}>
                <Bloom luminanceThreshold={0.5} mipmapBlur intensity={0.8} radius={0.4} />
                <Noise opacity={0.02} />
              </EffectComposer>
            </Canvas>
          </Style.GalaxyContainer>

          <Style.DefinitionBox>
            <h4>Syntropy (The Heartbeat)</h4>
            <p>
              Think of Syntropy as the network's metabolism. When it's low, devices are
              wandering—searching for peers in the dark (<strong>Red Pulse</strong>).
            </p>
            <p>
              When it's high, they lock into place (<strong>Violet Pulse</strong>). They stop
              competing and start computing together. The chaos organizes into a crystal.
            </p>
          </Style.DefinitionBox>
        </Style.ContentCard>
      </ScrollReveal>
      <Style.SectionDivider />
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Chapter 7: The Grand Finale</h3>
          <p>
            For decades, the peak of human computing has lived in massive, centralized data centers.
            Supercomputers like Frontier or Fugaku represent the pinnacle of this era—brute force
            efficiency locked behind high walls and cooling towers.
          </p>
          <p>
            INOS suggests a different peak. Not a monolith, but a <strong>Cosmos</strong>.
          </p>

          <Style.DefinitionBox>
            <h4>The Planetary Computer</h4>
            <p>
              A distributed system where a million browser nodes act as a single, unified
              supercomputer. No single point of failure. No centralized bill. Just pure, emergent
              compute.
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
          <h3>Lesson 1: The Scale Gap</h3>
          <p>
            Traditional supercomputers are limited by the physical space they occupy. INOS is
            limited only by the number of connected humans. By bridging the latency gap with
            Zero-Copy memory and the persistence gap with our Storage Mesh, we've created the
            infrastructure for "Planetary Scale."
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>Scale Comparison: Monolith vs Swarm</Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <SupercomputerDensityMap />
          </Style.IllustrationContainer>
          <p>
            The INOS mesh isn't just a network; it's a <strong>Spider's Mesh</strong>—a living,
            continuously expanding illusion where{' '}
            <strong>every edge is essentially a center</strong>. In nature, a web is an extension of
            the spider's nervous system. In INOS, the P2P Expansion Architecture functions as a
            global intelligence net. Information entering the mesh doesn't queue; it{' '}
            <em>diffuses</em>. Every node that joins becomes an active hub, spawning its own
            clusters and circulating data with the fidelity of a unified organism.
          </p>
        </Style.ContentCard>
      </ScrollReveal>
      <Style.SectionDivider />
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 2: The Computation (N-Body)</h3>
          <p>
            To prove this, we look to the stars. The N-Body problem—simulating the gravitational
            interaction of thousands of celestial bodies—is one of the most computationally
            expensive problems in physics.
          </p>
          <p>
            In INOS, this isn't just a demo. It's a test of the <strong>nbody.wgsl</strong> engine
            running across the mesh. Each node computes a sector, gossips the results, and reacts in
            real-time.
          </p>
        </Style.ContentCard>
      </ScrollReveal>
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 3: The Real-World Link</h3>
          <p>
            This isn't just about pixels. It's about the physical world. By integrating
            <strong>MAVLink</strong> and <strong>ROS2</strong> protocols directly into our Rust
            Muscles, INOS becomes the operating system for decentralized robotics.
          </p>
          <p>
            The expansive architecture allows for <strong>Swarm Synchronization</strong>. Instead of
            checking in with a central tower (monolith), autonomous vehicles and drones synchronize
            directly through the mesh. The flow is instantaneous: from local hardware to the SAB
            Hub, and out to the global coordination mesh. The swarm becomes a single, coordinated
            organism.
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>
                Robotics Swarm Coordination Pipeline
              </Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <RoboticProtocolBridge />
          </Style.IllustrationContainer>
        </Style.ContentCard>
      </ScrollReveal>
      <Style.SectionDivider />
      <ScrollReveal>
        <Style.ContentCard>
          <h3>The Roadmap: To Universum and Beyond</h3>
          <p>
            The Moonshot is not a single event, but a series of technical reconciliations. We have
            crossed the threshold of Zero-Copy and Mesh Resilience. The path ahead is where the
            network graduates into a living computer.
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>Fig_07 // The Living Roadmap</Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <RoadmapTimeline />
            <Style.IllustrationCaption>
              Current progress and future milestones on the path to the Planetary Computer.
            </Style.IllustrationCaption>
          </Style.IllustrationContainer>

          <p>
            Our current focus is <strong>Proof-of-Useful-Work (PoUW)</strong>. Instead of burning
            electricity to solve arbitrary hashes, INOS nodes solve real physics problems—like the
            N-Body simulation below—to verify their contribution to the mesh and earn yield.
          </p>
        </Style.ContentCard>
      </ScrollReveal>
      <Style.SectionDivider />
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Final Lesson: The Grand Demonstration</h3>
          <p>
            The simulation below is currently <strong>Offline / Research Phase</strong>.
          </p>
          <p>
            To achieve a true "Moonshot"—simulating a galaxy of 10 million stars in real-time—we
            need more than code. We need a collective. Every browser that joins the mesh adds
            another TFLOP of compute power.
          </p>

          <Style.DefinitionBox>
            <h4>Active Manifold</h4>
            <p>
              The Morphic Lattice above is a live demonstration of Go-orchestrated, Rust-computed,
              and Zero-Copy-visualized topological manifold physics. It is the first stage of the
              Planetary Computer's physical manifestation.
            </p>
          </Style.DefinitionBox>
        </Style.ContentCard>
      </ScrollReveal>
      <Style.SectionDivider />
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Conclusion: The Graduation</h3>
          <p>
            We began with a simple problem: the web was too slow and too fragile. We solved it with
            blood (Zero-Copy), a nervous system (Epoch Signaling), and a memory (Global Mesh).
          </p>
          <p>
            But the potential of INOS isn't just "faster data." It is the ability to interrogate
            reality itself. If we can distribute the computation of 10 million stars, we aren't just
            building a browser; we are building a <strong>Universal Simulation Engine</strong>.
          </p>
          <Style.DefinitionBox
            style={{
              background: 'rgba(30, 41, 59, 0.05)',
              border: '1px solid rgba(30, 41, 59, 0.1)',
            }}
          >
            <h4 style={{ color: '#1e293b' }}>The Simulation Theory</h4>
            <p>
              By decentralizing compute, we move closer to "Universal Fidelity." If the universe
              itself is a computation, then INOS is our attempt to peer under the hood. We aren't
              just simulating the cosmos; we are building the substrate for the next 10,000
              realities.
            </p>
          </Style.DefinitionBox>
          <p
            style={{
              textAlign: 'center',
              marginTop: '40px',
              fontSize: '24px',
              fontStyle: 'italic',
              color: theme.colors.inkMedium,
            }}
          >
            "Knowing is not enough; we must apply." — Da Vinci
          </p>
          <p
            style={{
              textAlign: 'center',
              fontSize: '10px',
              marginTop: '20px',
              color: theme.colors.inkLight,
              fontFamily: theme.fonts.typewriter,
            }}
          >
            CODEX VOL. 1 COMPLETE // UNIVERSUM INITIATED
          </p>
        </Style.ContentCard>
      </ScrollReveal>
      <ChapterNav prev={{ title: '04. Genesis', to: '/genesis' }} />
    </Style.Container>
  );
}
