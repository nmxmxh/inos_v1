/**
 * INOS Technical Codex — Problem Page (Chapter 1)
 *
 * Framing the systemic villain: Centralization, Moore's Law death,
 * climate cost, and wasted utilities.
 */

import { useEffect, useRef, useState } from 'react';
import styled, { useTheme } from 'styled-components';
import { motion, AnimatePresence } from 'framer-motion';
import * as d3 from 'd3';
import { Style as ManuscriptStyle } from '../styles/manuscript';
import ChapterNav from '../ui/ChapterNav';
import ScrollReveal from '../ui/ScrollReveal';

const Style = {
  ...ManuscriptStyle,

  StatsGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: ${p => p.theme.spacing[6]};
    margin: ${p => p.theme.spacing[8]} 0;
  `,

  StatCard: styled.div`
    background: rgba(255, 255, 255, 0.8);
    backdrop-filter: blur(8px);
    padding: ${p => p.theme.spacing[5]};
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 4px;
    display: flex;
    flex-direction: column;
    gap: ${p => p.theme.spacing[2]};
  `,

  ComparisonGrid: styled.div`
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: ${p => p.theme.spacing[6]};
    margin: ${p => p.theme.spacing[8]} 0;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      grid-template-columns: 1fr;
    }
  `,

  ComparisonCard: styled.div<{ $variant: 'villain' | 'hero' }>`
    background: ${p =>
      p.$variant === 'villain' ? 'rgba(220, 38, 38, 0.06)' : 'rgba(22, 163, 74, 0.06)'};
    backdrop-filter: blur(12px);
    border: 1px solid
      ${p => (p.$variant === 'villain' ? 'rgba(220, 38, 38, 0.25)' : 'rgba(22, 163, 74, 0.25)')};
    border-radius: 6px;
    padding: ${p => p.theme.spacing[6]};
  `,

  ComparisonTitle: styled.h4<{ $variant: 'villain' | 'hero' }>`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.15em;
    color: ${p => (p.$variant === 'villain' ? '#dc2626' : '#16a34a')};
    margin: 0 0 ${p => p.theme.spacing[3]};
  `,

  ComparisonValue: styled.div`
    font-size: 2rem;
    font-weight: 700;
    color: ${p => p.theme.colors.inkDark};
    margin-bottom: ${p => p.theme.spacing[2]};
  `,

  ComparisonLabel: styled.p`
    font-size: ${p => p.theme.fontSizes.sm};
    color: ${p => p.theme.colors.inkMedium};
    margin: 0;
    line-height: 1.5;
  `,

  IllustrationContainer: styled.div`
    width: 100%;
    background: rgba(255, 255, 255, 0.6);
    backdrop-filter: blur(8px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 6px;
    margin: ${p => p.theme.spacing[6]} 0;
    overflow: hidden;
  `,

  IllustrationHeader: styled.div`
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: ${p => p.theme.spacing[3]} ${p => p.theme.spacing[4]};
    border-bottom: 1px solid ${p => p.theme.colors.borderSubtle};
  `,

  IllustrationTitle: styled.span`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    font-weight: 400;
    color: ${p => p.theme.colors.inkMedium};
    text-transform: uppercase;
    letter-spacing: 0.1em;
  `,

  ScenarioSelector: styled.div`
    display: flex;
    gap: ${p => p.theme.spacing[2]};
  `,

  ScenarioButton: styled.button<{ $active: boolean }>`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 9px;
    padding: 4px 10px;
    border-radius: 3px;
    border: 1px solid ${p => (p.$active ? p.theme.colors.accent : p.theme.colors.borderSubtle)};
    background: ${p => (p.$active ? p.theme.colors.accent : 'transparent')};
    color: ${p => (p.$active ? '#fff' : p.theme.colors.inkMedium)};
    cursor: pointer;
    transition: all 0.2s;
    text-transform: uppercase;
    letter-spacing: 0.05em;

    &:hover {
      border-color: ${p => p.theme.colors.accent};
    }
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
};

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: CENTRALIZATION TAX
// Shows hub-spoke model with distinctive entity shapes
// ────────────────────────────────────────────────────────────────────────────
function CentralizationDiagram() {
  const svgRef = useRef<SVGSVGElement>(null);
  const theme = useTheme();

  useEffect(() => {
    if (!svgRef.current) return;

    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const width = 600;
    const height = 320;
    const centerX = width / 2;
    const centerY = height / 2;

    const orbitRadius = 110;

    // Devices with distinctive shapes
    const devices = [
      { angle: 0, label: 'Mobile App', shape: 'phone', color: '#3b82f6' },
      { angle: 60, label: 'Desktop', shape: 'laptop', color: '#8b5cf6' },
      { angle: 120, label: 'Web Server', shape: 'server', color: '#f59e0b' },
      { angle: 180, label: 'IoT Device', shape: 'iot', color: '#10b981' },
      { angle: 240, label: 'API Gateway', shape: 'gateway', color: '#ec4899' },
      { angle: 300, label: 'Browser', shape: 'browser', color: '#06b6d4' },
    ];

    // Glow filter
    const defs = svg.append('defs');
    const filter = defs.append('filter').attr('id', 'glow-central');
    filter.append('feGaussianBlur').attr('stdDeviation', '3').attr('result', 'coloredBlur');
    const feMerge = filter.append('feMerge');
    feMerge.append('feMergeNode').attr('in', 'coloredBlur');
    feMerge.append('feMergeNode').attr('in', 'SourceGraphic');

    // Draw devices
    devices.forEach((device, i) => {
      const angle = (device.angle * Math.PI) / 180;
      const x = centerX + orbitRadius * Math.cos(angle);
      const y = centerY + orbitRadius * Math.sin(angle);

      // Line to center
      svg
        .append('line')
        .attr('x1', x)
        .attr('y1', y)
        .attr('x2', centerX)
        .attr('y2', centerY)
        .attr('stroke', '#dc2626')
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '3,3')
        .attr('opacity', 0.25);

      // Animated data packet
      const packet = svg
        .append('circle')
        .attr('r', 4)
        .attr('fill', '#dc2626')
        .attr('filter', 'url(#glow-central)')
        .attr('opacity', 0);

      function animatePacket() {
        packet
          .attr('cx', x)
          .attr('cy', y)
          .attr('opacity', 0.8)
          .transition()
          .delay(i * 250)
          .duration(1400)
          .ease(d3.easeCubicInOut)
          .attr('cx', centerX)
          .attr('cy', centerY)
          .attr('opacity', 0)
          .on('end', animatePacket);
      }
      animatePacket();

      // Device shape based on type
      const g = svg.append('g').attr('transform', `translate(${x}, ${y})`);

      if (device.shape === 'phone') {
        g.append('rect')
          .attr('x', -10)
          .attr('y', -16)
          .attr('width', 20)
          .attr('height', 32)
          .attr('rx', 3)
          .attr('fill', 'rgba(255,255,255,0.95)')
          .attr('stroke', device.color)
          .attr('stroke-width', 1.5);
        g.append('rect')
          .attr('x', -7)
          .attr('y', -12)
          .attr('width', 14)
          .attr('height', 20)
          .attr('fill', device.color)
          .attr('opacity', 0.15);
      } else if (device.shape === 'laptop') {
        g.append('rect')
          .attr('x', -18)
          .attr('y', -12)
          .attr('width', 36)
          .attr('height', 22)
          .attr('rx', 2)
          .attr('fill', 'rgba(255,255,255,0.95)')
          .attr('stroke', device.color)
          .attr('stroke-width', 1.5);
        g.append('path')
          .attr('d', 'M-22,10 L22,10 L18,14 L-18,14 Z')
          .attr('fill', device.color)
          .attr('opacity', 0.2);
      } else if (device.shape === 'server') {
        [-8, 0, 8].forEach(yOff => {
          g.append('rect')
            .attr('x', -16)
            .attr('y', yOff - 5)
            .attr('width', 32)
            .attr('height', 9)
            .attr('rx', 1)
            .attr('fill', 'rgba(255,255,255,0.95)')
            .attr('stroke', device.color)
            .attr('stroke-width', 1);
          g.append('circle')
            .attr('cx', 10)
            .attr('cy', yOff - 0.5)
            .attr('r', 1.5)
            .attr('fill', device.color);
        });
      } else if (device.shape === 'iot') {
        g.append('circle')
          .attr('r', 14)
          .attr('fill', 'rgba(255,255,255,0.95)')
          .attr('stroke', device.color)
          .attr('stroke-width', 1.5);
        g.append('circle').attr('r', 5).attr('fill', device.color).attr('opacity', 0.3);
        [0, 120, 240].forEach(a => {
          const rad = (a * Math.PI) / 180;
          g.append('line')
            .attr('x1', 7 * Math.cos(rad))
            .attr('y1', 7 * Math.sin(rad))
            .attr('x2', 12 * Math.cos(rad))
            .attr('y2', 12 * Math.sin(rad))
            .attr('stroke', device.color)
            .attr('stroke-width', 1.5);
        });
      } else if (device.shape === 'gateway') {
        g.append('polygon')
          .attr('points', '0,-16 16,0 0,16 -16,0')
          .attr('fill', 'rgba(255,255,255,0.95)')
          .attr('stroke', device.color)
          .attr('stroke-width', 1.5);
        g.append('circle').attr('r', 4).attr('fill', device.color).attr('opacity', 0.3);
      } else if (device.shape === 'browser') {
        g.append('rect')
          .attr('x', -16)
          .attr('y', -12)
          .attr('width', 32)
          .attr('height', 24)
          .attr('rx', 2)
          .attr('fill', 'rgba(255,255,255,0.95)')
          .attr('stroke', device.color)
          .attr('stroke-width', 1.5);
        g.append('line')
          .attr('x1', -16)
          .attr('y1', -5)
          .attr('x2', 16)
          .attr('y2', -5)
          .attr('stroke', device.color)
          .attr('stroke-width', 0.75);
        [-10, -5, 0].forEach(cx => {
          g.append('circle')
            .attr('cx', cx)
            .attr('cy', -8.5)
            .attr('r', 1.5)
            .attr('fill', device.color);
        });
      }

      // Label
      svg
        .append('text')
        .attr('x', x)
        .attr('y', y + 32)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkDark)
        .attr('font-family', "'Inter', sans-serif")
        .text(device.label);
    });

    // Central data center building
    const dcG = svg.append('g').attr('transform', `translate(${centerX}, ${centerY})`);

    // Building base
    dcG
      .append('rect')
      .attr('x', -35)
      .attr('y', -30)
      .attr('width', 70)
      .attr('height', 60)
      .attr('rx', 4)
      .attr('fill', 'rgba(220, 38, 38, 0.08)')
      .attr('stroke', '#dc2626')
      .attr('stroke-width', 2);

    // Server racks inside - properly centered 2x3 grid
    const rackSize = 14;
    const rackGap = 4;
    const gridWidth = 3 * rackSize + 2 * rackGap;
    const gridHeight = 2 * rackSize + rackGap;
    const gridStartX = -gridWidth / 2;
    const gridStartY = -gridHeight / 2;

    for (let row = 0; row < 2; row++) {
      for (let col = 0; col < 3; col++) {
        const rx = gridStartX + col * (rackSize + rackGap);
        const ry = gridStartY + row * (rackSize + rackGap);
        dcG
          .append('rect')
          .attr('x', rx)
          .attr('y', ry)
          .attr('width', rackSize)
          .attr('height', rackSize)
          .attr('rx', 2)
          .attr('fill', '#dc2626')
          .attr('opacity', 0.15);
      }
    }

    dcG
      .append('text')
      .attr('y', 48)
      .attr('text-anchor', 'middle')
      .attr('font-size', 11)
      .attr('font-weight', 700)
      .attr('fill', '#dc2626')
      .attr('font-family', "'Inter', sans-serif")
      .text('DATA CENTER');
  }, [theme]);

  return <svg ref={svgRef} viewBox="0 0 600 320" style={{ width: '100%', height: 'auto' }} />;
}

// ────────────────────────────────────────────────────────────────────────────
// COPY TAX SCENARIOS
// ────────────────────────────────────────────────────────────────────────────
const COPY_TAX_SCENARIOS = [
  {
    id: 'web',
    label: 'Web App',
    stages: [
      { label: 'React\nFrontend', color: '#61dafb', lang: 'JS' },
      { label: 'JSON\nSerialize', color: '#dc2626', isCopy: true },
      { label: 'HTTP\nRequest', color: '#6b7280' },
      { label: 'JSON\nParse', color: '#dc2626', isCopy: true },
      { label: 'Express\nBackend', color: '#68a063', lang: 'JS' },
      { label: 'SQL\nQuery', color: '#336791' },
    ],
  },
  {
    id: 'wasm',
    label: 'Go ↔ JS',
    stages: [
      { label: 'Go\nKernel', color: '#00add8', lang: 'Go' },
      { label: 'Marshal\nto bytes', color: '#dc2626', isCopy: true },
      { label: 'WASM\nBridge', color: '#654ff0' },
      { label: 'Copy to\nJS heap', color: '#dc2626', isCopy: true },
      { label: 'TypeScript\nLayer', color: '#3178c6', lang: 'TS' },
      { label: 'Render\nCanvas', color: '#f59e0b' },
    ],
  },
  {
    id: 'ml',
    label: 'Rust ↔ Python',
    stages: [
      { label: 'PyTorch\nModel', color: '#ee4c2c', lang: 'Py' },
      { label: 'NumPy\nto bytes', color: '#dc2626', isCopy: true },
      { label: 'FFI\nBridge', color: '#6b7280' },
      { label: 'Rust\nVec copy', color: '#dc2626', isCopy: true },
      { label: 'Rust\nEngine', color: '#dea584', lang: 'Rs' },
      { label: 'SIMD\nCompute', color: '#10b981' },
    ],
  },
  {
    id: 'worker',
    label: 'JS Workers',
    stages: [
      { label: 'Main\nThread', color: '#f59e0b', lang: 'JS' },
      { label: 'Structured\nClone', color: '#dc2626', isCopy: true },
      { label: 'Message\nChannel', color: '#6b7280' },
      { label: 'Deserialize\nin Worker', color: '#dc2626', isCopy: true },
      { label: 'Web\nWorker', color: '#f59e0b', lang: 'JS' },
      { label: 'Process\nData', color: '#10b981' },
    ],
  },
];

function CopyTaxDiagram({ scenario }: { scenario: (typeof COPY_TAX_SCENARIOS)[0] }) {
  const svgRef = useRef<SVGSVGElement>(null);
  const theme = useTheme();

  useEffect(() => {
    if (!svgRef.current) return;

    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const width = 650;
    const height = 380;
    const barHeight = 44;
    const barGap = 8;
    const startY = 20;
    const leftMargin = 120;
    const maxBarWidth = 420;

    // Data sizes growing at each step (representing overhead)
    const stages = scenario.stages;
    const sizes = stages.map((s, i) => {
      // Calculate cumulative overhead
      let overhead = 1.0;
      for (let j = 0; j <= i; j++) {
        if (stages[j].isCopy) overhead *= 1.35;
      }
      return { ...s, size: overhead, y: startY + i * (barHeight + barGap) };
    });

    const maxSize = Math.max(...sizes.map(s => s.size));

    // Draw bars
    sizes.forEach((stage, i) => {
      const barWidth = (stage.size / maxSize) * maxBarWidth;
      const y = stage.y;

      // Bar background
      svg
        .append('rect')
        .attr('x', leftMargin)
        .attr('y', y)
        .attr('width', barWidth)
        .attr('height', barHeight)
        .attr('rx', 3)
        .attr('fill', stage.isCopy ? 'rgba(220, 38, 38, 0.15)' : stage.color)
        .attr('opacity', stage.isCopy ? 1 : 0.2);

      // Bar border
      svg
        .append('rect')
        .attr('x', leftMargin)
        .attr('y', y)
        .attr('width', barWidth)
        .attr('height', barHeight)
        .attr('rx', 3)
        .attr('fill', 'none')
        .attr('stroke', stage.isCopy ? '#dc2626' : stage.color)
        .attr('stroke-width', stage.isCopy ? 1.5 : 1);

      // Stage label (left side)
      const labelLines = stage.label.split('\n');
      labelLines.forEach((line, li) => {
        svg
          .append('text')
          .attr('x', leftMargin - 10)
          .attr('y', y + barHeight / 2 + (li - (labelLines.length - 1) / 2) * 12 + 4)
          .attr('text-anchor', 'end')
          .attr('font-size', 10)
          .attr('font-weight', 500)
          .attr('fill', stage.isCopy ? '#dc2626' : theme.colors.inkDark)
          .attr('font-family', "'Inter', sans-serif")
          .text(line);
      });

      // Size indicator (inside bar)
      if (stage.isCopy) {
        svg
          .append('text')
          .attr('x', leftMargin + barWidth - 8)
          .attr('y', y + barHeight / 2 + 4)
          .attr('text-anchor', 'end')
          .attr('font-size', 10)
          .attr('font-weight', 700)
          .attr('fill', '#dc2626')
          .attr('font-family', "'Inter', sans-serif")
          .text(`+${Math.round((stage.size - 1) * 100)}%`);
      }

      // Draw arrow connector to next
      if (i < sizes.length - 1) {
        const nextY = sizes[i + 1].y;
        const arrowX = leftMargin + barWidth / 2;
        svg
          .append('line')
          .attr('x1', arrowX)
          .attr('y1', y + barHeight)
          .attr('x2', arrowX)
          .attr('y2', nextY)
          .attr('stroke', sizes[i + 1].isCopy ? '#dc2626' : '#d1d5db')
          .attr('stroke-width', 1)
          .attr('stroke-dasharray', sizes[i + 1].isCopy ? '3,2' : 'none')
          .attr('opacity', 0.5);
      }
    });

    // Legend
    svg
      .append('text')
      .attr('x', width / 2)
      .attr('y', height - 10)
      .attr('text-anchor', 'middle')
      .attr('font-size', 10)
      .attr('fill', theme.colors.inkLight)
      .attr('font-family', "'Inter', sans-serif")
      .text('Bar width = memory footprint (JSON +30-50% vs Protobuf)');
  }, [scenario, theme]);

  return <svg ref={svgRef} viewBox="0 0 650 380" style={{ width: '100%', height: 'auto' }} />;
}

export function Problem() {
  const [copyTaxScenario, setCopyTaxScenario] = useState(COPY_TAX_SCENARIOS[0]);

  return (
    <Style.BlogContainer>
      <Style.SectionTitle>Chapter 01</Style.SectionTitle>
      <Style.PageTitle>The Architecture of Waste</Style.PageTitle>

      <Style.LeadParagraph>
        The future of computing was supposed to be faster every year. Moore's Law promised it.
        Instead, we hit a wall. Clock speeds stalled at 3-5 GHz. Single-threaded performance
        plateaued. And to compensate, we centralized. We built data centers the size of cities. We
        burned energy equivalent to small nations.
      </Style.LeadParagraph>

      {/* VILLAIN #1: THE CENTRALIZATION TAX */}
      <ScrollReveal>
        <Style.JotterSection>
          <Style.JotterHeader>
            <Style.JotterNumber>Villain #1</Style.JotterNumber>
            <Style.JotterHeading>The Centralization Tax</Style.JotterHeading>
          </Style.JotterHeader>
          <p>
            Global data centers consumed <strong>415 TWh in 2024</strong>. That is 1.5% of all
            electricity on Earth. By 2030, projections show <strong>945 TWh</strong>. For context:
            that is more than the entire nation of Japan uses today.
          </p>
          <p>
            We did not build a distributed internet. We built a handful of hyperscale silos. Amazon,
            Google, Microsoft. Your data, your compute, your future. All rented from three
            companies.
          </p>
        </Style.JotterSection>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>The Hub-Spoke Model</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <CentralizationDiagram />
        <Style.IllustrationCaption>
          All roads lead to the data center. All data flows to three companies.
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.ComparisonGrid>
        <Style.ComparisonCard $variant="villain">
          <Style.ComparisonTitle $variant="villain">Data Centers (2024)</Style.ComparisonTitle>
          <Style.ComparisonValue>415 TWh</Style.ComparisonValue>
          <Style.ComparisonLabel>
            1.5% of global electricity. Expected to double by 2030. 56% powered by fossil fuels in
            the US alone.
          </Style.ComparisonLabel>
        </Style.ComparisonCard>
        <Style.ComparisonCard $variant="villain">
          <Style.ComparisonTitle $variant="villain">Bitcoin Mining (2024)</Style.ComparisonTitle>
          <Style.ComparisonValue>150 TWh</Style.ComparisonValue>
          <Style.ComparisonLabel>
            For 7 transactions per second. Visa does 65,000 TPS. The promise of decentralization
            became an energy apocalypse.
          </Style.ComparisonLabel>
        </Style.ComparisonCard>
      </Style.ComparisonGrid>

      {/* VILLAIN #2: THE PHYSICS CEILING */}
      <ScrollReveal>
        <Style.JotterSection>
          <Style.JotterHeader>
            <Style.JotterNumber>Villain #2</Style.JotterNumber>
            <Style.JotterHeading>The Physics Ceiling</Style.JotterHeading>
          </Style.JotterHeader>
          <p>
            Moore's Law is not dead. Transistors still shrink. But <strong>Dennard Scaling</strong>{' '}
            ended in 2006. Smaller transistors no longer mean less heat. Power consumption rises
            quadratically with clock speed. At 10 GHz, a signal can only travel 2 centimeters in a
            single clock cycle.
          </p>
          <p>
            Single-threaded performance has barely doubled in the last decade. The industry shifted
            to multi-core, but most software cannot parallelize. We are running on borrowed time.
          </p>
        </Style.JotterSection>
      </ScrollReveal>

      <Style.StatsGrid>
        <Style.StatCard>
          <Style.MetricLabel>Clock Speed Ceiling</Style.MetricLabel>
          <Style.MetricValue>3-5 GHz</Style.MetricValue>
          <Style.MetricUnit>Heat barrier since 2005</Style.MetricUnit>
        </Style.StatCard>
        <Style.StatCard>
          <Style.MetricLabel>Single-Thread Gain</Style.MetricLabel>
          <Style.MetricValue>~2x</Style.MetricValue>
          <Style.MetricUnit>In the last 10 years</Style.MetricUnit>
        </Style.StatCard>
        <Style.StatCard>
          <Style.MetricLabel>Signal Limit @ 10GHz</Style.MetricLabel>
          <Style.MetricValue>2 cm</Style.MetricValue>
          <Style.MetricUnit>Speed of light constraint</Style.MetricUnit>
        </Style.StatCard>
      </Style.StatsGrid>

      {/* VILLAIN #3: THE COPY TAX */}
      <ScrollReveal>
        <Style.JotterSection>
          <Style.JotterHeader>
            <Style.JotterNumber>Villain #3</Style.JotterNumber>
            <Style.JotterHeading>The Copy Tax</Style.JotterHeading>
          </Style.JotterHeader>
          <p>
            Every time data crosses a boundary, it pays a toll. Frontend to backend? Serialize to
            JSON, send over HTTP, parse on the other side. Even JS to JS. Thread to worker? Copy the
            buffer. Systems spend up to <strong>40% of CPU cycles</strong> not computing, but
            shuffling bytes between walled gardens.
          </p>
          <p>
            Memory is abundant. Bandwidth is abundant. Yet we treat every component as an island,
            forcing data to be re-encoded at every handoff. Pure bureaucracy.
          </p>
        </Style.JotterSection>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>The Copy Tax Pipeline</Style.IllustrationTitle>
          <Style.ScenarioSelector>
            {COPY_TAX_SCENARIOS.map(s => (
              <Style.ScenarioButton
                key={s.id}
                $active={s.id === copyTaxScenario.id}
                onClick={() => setCopyTaxScenario(s)}
              >
                {s.label}
              </Style.ScenarioButton>
            ))}
          </Style.ScenarioSelector>
        </Style.IllustrationHeader>
        <AnimatePresence mode="wait">
          <motion.div
            key={copyTaxScenario.id}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ duration: 0.25, ease: 'easeInOut' }}
          >
            <CopyTaxDiagram scenario={copyTaxScenario} />
          </motion.div>
        </AnimatePresence>
        <Style.IllustrationCaption>
          40% of CPU time wasted on serialization and copying
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.ComparisonGrid>
        <Style.ComparisonCard $variant="villain">
          <Style.ComparisonTitle $variant="villain">Traditional Architecture</Style.ComparisonTitle>
          <Style.ComparisonValue>40%</Style.ComparisonValue>
          <Style.ComparisonLabel>
            CPU time spent on serialization, copying, and marshalling. Not computing. Just moving
            paperwork.
          </Style.ComparisonLabel>
        </Style.ComparisonCard>
        <Style.ComparisonCard $variant="hero">
          <Style.ComparisonTitle $variant="hero">Zero-Copy Architecture</Style.ComparisonTitle>
          <Style.ComparisonValue>0%</Style.ComparisonValue>
          <Style.ComparisonLabel>
            Shared memory. One buffer. Multiple readers. No copies. All that CPU freed for actual
            work.
          </Style.ComparisonLabel>
        </Style.ComparisonCard>
      </Style.ComparisonGrid>

      {/* THE QUESTION */}
      <Style.BlogSection>
        <ScrollReveal variant="fade">
          <h3>The Question</h3>
          <p>
            We have accepted all of this as the cost of doing business. Centralized compute. Stalled
            clock speeds. Wasted energy. Serialization overhead.
          </p>
          <p style={{ fontWeight: 600, color: '#1a1a1a', marginTop: '1.5rem' }}>
            But what if there is another way?
          </p>
          <p style={{ fontWeight: 500, color: '#1a1a1a' }}>
            What if every browser became a compute node? What if data could flow between threads
            without copying? What if we stopped building data centers and started building a mesh?
          </p>
          <p style={{ fontWeight: 600, color: '#1a1a1a', marginTop: '1.5rem' }}>
            That is the question INOS was built to answer.
          </p>
        </ScrollReveal>
      </Style.BlogSection>

      <ChapterNav
        prev={{ to: '/', title: 'Codex' }}
        next={{ to: '/insight', title: 'The Insight' }}
      />
    </Style.BlogContainer>
  );
}

export default Problem;
