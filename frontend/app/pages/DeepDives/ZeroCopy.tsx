/**
 * INOS Technical Codex — Deep Dive: Zero-Copy I/O
 *
 * A comprehensive exploration of SharedArrayBuffer, the zero-copy paradigm,
 * and how INOS eliminates data copying across WASM module boundaries.
 *
 * Educational approach: Teach the fundamentals before showing the implementation.
 */

import { useCallback } from 'react';
import styled, { useTheme } from 'styled-components';
import * as d3 from 'd3';
import D3Container from '../../ui/D3Container';
import { Style as ManuscriptStyle } from '../../styles/manuscript';
import ChapterNav from '../../ui/ChapterNav';
import ScrollReveal from '../../ui/ScrollReveal';

const Style = {
  ...ManuscriptStyle,

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

  IllustrationCaption: styled.p`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    color: ${p => p.theme.colors.inkMedium};
    text-align: center;
    padding: ${p => p.theme.spacing[3]};
    margin: 0;
    border-top: 1px solid ${p => p.theme.colors.borderSubtle};
  `,

  HistoryCard: styled.div`
    background: rgba(139, 92, 246, 0.08);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(139, 92, 246, 0.2);
    border-left: 3px solid #8b5cf6;
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[6]} 0;

    h4 {
      margin: 0 0 ${p => p.theme.spacing[3]} 0;
      color: #8b5cf6;
    }

    p {
      margin: 0 0 ${p => p.theme.spacing[3]} 0;
      line-height: 1.6;
    }

    p:last-child {
      margin-bottom: 0;
    }
  `,

  WarningCard: styled.div`
    background: rgba(234, 179, 8, 0.08);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(234, 179, 8, 0.2);
    border-left: 3px solid #eab308;
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[6]} 0;

    h4 {
      margin: 0 0 ${p => p.theme.spacing[3]} 0;
      color: #ca8a04;
    }

    p {
      margin: 0;
      line-height: 1.6;
    }
  `,

  CodeBlock: styled.pre`
    background: #1a1a2e;
    color: #e2e8f0;
    padding: ${p => p.theme.spacing[5]};
    border-radius: 6px;
    overflow-x: auto;
    font-family: 'JetBrains Mono', 'Fira Code', monospace;
    font-size: 13px;
    line-height: 1.6;
    margin: ${p => p.theme.spacing[4]} 0;

    .keyword {
      color: #c792ea;
    }
    .function {
      color: #82aaff;
    }
    .string {
      color: #c3e88d;
    }
    .comment {
      color: #546e7a;
    }
    .number {
      color: #f78c6c;
    }
    .type {
      color: #ffcb6b;
    }
  `,

  Timeline: styled.div`
    display: flex;
    flex-direction: column;
    gap: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[6]} 0;
    padding: ${p => p.theme.spacing[5]};
    padding-left: ${p => p.theme.spacing[8]};
    background: rgba(255, 255, 255, 0.85);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 8px;
    border-left: 3px solid ${p => p.theme.colors.accent};
  `,

  TimelineItem: styled.div`
    position: relative;
    line-height: 1.6;
    color: ${p => p.theme.colors.inkDark};

    &::before {
      content: '';
      position: absolute;
      left: -${p => p.theme.spacing[6]};
      top: 8px;
      width: 12px;
      height: 12px;
      border-radius: 50%;
      background: ${p => p.theme.colors.accent};
      border: 2px solid white;
      box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
    }
  `,

  TimelineYear: styled.span`
    display: inline-block;
    font-weight: 700;
    font-size: ${p => p.theme.fontSizes.lg};
    color: ${p => p.theme.colors.accent};
    margin-right: ${p => p.theme.spacing[3]};
    min-width: 50px;
  `,

  ComparisonGrid: styled.div`
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[5]} 0;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      grid-template-columns: 1fr;
    }
  `,

  ComparisonCard: styled.div<{ $type: 'bad' | 'good' }>`
    background: ${p => (p.$type === 'bad' ? 'rgba(220, 38, 38, 0.06)' : 'rgba(22, 163, 74, 0.06)')};
    backdrop-filter: blur(12px);
    border: 1px solid
      ${p => (p.$type === 'bad' ? 'rgba(220, 38, 38, 0.2)' : 'rgba(22, 163, 74, 0.2)')};
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};

    h4 {
      margin: 0 0 ${p => p.theme.spacing[3]} 0;
      color: ${p => (p.$type === 'bad' ? '#dc2626' : '#16a34a')};
    }

    p {
      margin: 0;
      line-height: 1.6;
      font-size: ${p => p.theme.fontSizes.sm};
    }
  `,

  DefinitionBox: styled.div`
    background: rgba(59, 130, 246, 0.08);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(59, 130, 246, 0.2);
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[6]} 0;

    h4 {
      margin: 0 0 ${p => p.theme.spacing[2]} 0;
      color: #2563eb;
      font-size: ${p => p.theme.fontSizes.lg};
    }

    p {
      margin: 0;
      line-height: 1.7;
    }

    code {
      background: rgba(59, 130, 246, 0.1);
      padding: 2px 6px;
      border-radius: 3px;
      font-family: 'JetBrains Mono', monospace;
      font-size: 0.9em;
    }
  `,
};

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: COPY TAX COMPARISON
// ────────────────────────────────────────────────────────────────────────────
function CopyTaxDiagram() {
  const theme = useTheme();

  const renderViz = useCallback(
    (
      svg: d3.Selection<SVGSVGElement, unknown, null, undefined>,
      _width: number,
      _height: number
    ) => {
      svg.selectAll('*').remove();

      // Traditional approach (top half)
      const tradY = 50;
      const stages = [
        { x: 60, label: 'Module A', sublabel: 'Object in Memory' },
        { x: 180, label: 'Serialize', sublabel: 'JSON.stringify' },
        { x: 300, label: 'Copy', sublabel: 'postMessage' },
        { x: 420, label: 'Deserialize', sublabel: 'JSON.parse' },
        { x: 540, label: 'Module B', sublabel: 'New Object' },
      ];

      // Title
      svg
        .append('text')
        .attr('x', 30)
        .attr('y', tradY - 15)
        .attr('font-size', 11)
        .attr('font-weight', 700)
        .attr('fill', '#dc2626')
        .attr('font-family', "'Inter', sans-serif")
        .text('Traditional: The Copy Tax');

      // Draw traditional stages
      stages.forEach((stage, i) => {
        // Box
        svg
          .append('rect')
          .attr('x', stage.x)
          .attr('y', tradY)
          .attr('width', 100)
          .attr('height', 50)
          .attr('rx', 4)
          .attr('fill', i === 0 || i === 4 ? 'rgba(220, 38, 38, 0.1)' : 'rgba(220, 38, 38, 0.05)')
          .attr('stroke', '#dc2626')
          .attr('stroke-width', 1);

        // Label
        svg
          .append('text')
          .attr('x', stage.x + 50)
          .attr('y', tradY + 22)
          .attr('text-anchor', 'middle')
          .attr('font-size', 10)
          .attr('font-weight', 600)
          .attr('fill', '#dc2626')
          .attr('font-family', "'Inter', sans-serif")
          .text(stage.label);

        // Sublabel
        svg
          .append('text')
          .attr('x', stage.x + 50)
          .attr('y', tradY + 38)
          .attr('text-anchor', 'middle')
          .attr('font-size', 8)
          .attr('fill', theme.colors.inkLight)
          .attr('font-family', "'Inter', sans-serif")
          .text(stage.sublabel);

        // Arrow
        if (i < stages.length - 1) {
          const nextX = stages[i + 1].x;
          svg
            .append('line')
            .attr('x1', stage.x + 100)
            .attr('y1', tradY + 25)
            .attr('x2', nextX)
            .attr('y2', tradY + 25)
            .attr('stroke', '#dc2626')
            .attr('stroke-width', 1.5);

          svg
            .append('polygon')
            .attr(
              'points',
              `${nextX},${tradY + 25} ${nextX - 6},${tradY + 21} ${nextX - 6},${tradY + 29}`
            )
            .attr('fill', '#dc2626');
        }
      });

      // INOS approach (bottom half)
      const inosY = 160;

      // Title
      svg
        .append('text')
        .attr('x', 30)
        .attr('y', inosY - 15)
        .attr('font-size', 11)
        .attr('font-weight', 700)
        .attr('fill', '#16a34a')
        .attr('font-family', "'Inter', sans-serif")
        .text('INOS: Zero-Copy via SAB');

      const inosStages = [
        { x: 120, label: 'Module A', sublabel: 'Writes to SAB' },
        { x: 300, label: 'SharedArrayBuffer', sublabel: 'Same Memory' },
        { x: 480, label: 'Module B', sublabel: 'Reads from SAB' },
      ];

      inosStages.forEach((stage, i) => {
        // Box
        const isMiddle = i === 1;
        svg
          .append('rect')
          .attr('x', stage.x)
          .attr('y', inosY)
          .attr('width', isMiddle ? 140 : 100)
          .attr('height', 50)
          .attr('rx', 4)
          .attr('fill', isMiddle ? 'rgba(139, 92, 246, 0.1)' : 'rgba(22, 163, 74, 0.1)')
          .attr('stroke', isMiddle ? '#8b5cf6' : '#16a34a')
          .attr('stroke-width', isMiddle ? 2 : 1);

        // Label
        svg
          .append('text')
          .attr('x', stage.x + (isMiddle ? 70 : 50))
          .attr('y', inosY + 22)
          .attr('text-anchor', 'middle')
          .attr('font-size', 10)
          .attr('font-weight', 600)
          .attr('fill', isMiddle ? '#8b5cf6' : '#16a34a')
          .attr('font-family', "'Inter', sans-serif")
          .text(stage.label);

        // Sublabel
        svg
          .append('text')
          .attr('x', stage.x + (isMiddle ? 70 : 50))
          .attr('y', inosY + 38)
          .attr('text-anchor', 'middle')
          .attr('font-size', 8)
          .attr('fill', theme.colors.inkLight)
          .attr('font-family', "'Inter', sans-serif")
          .text(stage.sublabel);
      });

      // Arrows for INOS
      svg
        .append('line')
        .attr('x1', 220)
        .attr('y1', inosY + 25)
        .attr('x2', 300)
        .attr('y2', inosY + 25)
        .attr('stroke', '#16a34a')
        .attr('stroke-width', 1.5);

      svg
        .append('polygon')
        .attr('points', `300,${inosY + 25} 294,${inosY + 21} 294,${inosY + 29}`)
        .attr('fill', '#16a34a');

      svg
        .append('line')
        .attr('x1', 440)
        .attr('y1', inosY + 25)
        .attr('x2', 480)
        .attr('y2', inosY + 25)
        .attr('stroke', '#16a34a')
        .attr('stroke-width', 1.5);

      svg
        .append('polygon')
        .attr('points', `480,${inosY + 25} 474,${inosY + 21} 474,${inosY + 29}`)
        .attr('fill', '#16a34a');

      // Cost comparison
      svg
        .append('text')
        .attr('x', 650)
        .attr('y', tradY + 30)
        .attr('text-anchor', 'end')
        .attr('font-size', 9)
        .attr('fill', '#dc2626')
        .attr('font-family', "'Inter', sans-serif")
        .text('~4 copies');

      svg
        .append('text')
        .attr('x', 650)
        .attr('y', inosY + 30)
        .attr('text-anchor', 'end')
        .attr('font-size', 9)
        .attr('font-weight', 600)
        .attr('fill', '#16a34a')
        .attr('font-family', "'Inter', sans-serif")
        .text('0 copies');
    },
    [theme]
  );

  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 700 240" height={240} />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: SAB MEMORY REGIONS
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: SAB MEMORY REGIONS
// ────────────────────────────────────────────────────────────────────────────
function SABRegionsDiagram() {
  const theme = useTheme();

  const renderViz = useCallback(
    (
      svg: d3.Selection<SVGSVGElement, unknown, null, undefined>,
      _width: number,
      _height: number
    ) => {
      svg.selectAll('*').remove();

      const startX = 80;
      const barWidth = 500;

      // Memory regions from sab_layout.capnp - adjusted for better visibility
      const regions = [
        { offset: '0x000000', label: 'Atomic Flags', size: '128B', color: '#dc2626', height: 24 },
        {
          offset: '0x000080',
          label: 'Supervisor Alloc',
          size: '176B',
          color: '#f59e0b',
          height: 24,
        },
        { offset: '0x000140', label: 'Module Registry', size: '6KB', color: '#8b5cf6', height: 32 },
        {
          offset: '0x002000',
          label: 'Supervisor Headers',
          size: '4KB',
          color: '#00add8',
          height: 28,
        },
        { offset: '0x004000', label: 'Economics', size: '16KB', color: '#16a34a', height: 36 },
        {
          offset: '0x008000',
          label: 'Identity Registry',
          size: '16KB',
          color: '#ec4899',
          height: 36,
        },
        {
          offset: '0x010000',
          label: 'Pattern Exchange',
          size: '64KB',
          color: '#6366f1',
          height: 44,
        },
        { offset: '0x100000', label: 'Job Queues', size: '1MB', color: '#0ea5e9', height: 52 },
        { offset: '0x200000', label: 'Dynamic Arena', size: '~30MB', color: '#84cc16', height: 70 },
      ];

      // Title
      svg
        .append('text')
        .attr('x', startX + barWidth / 2)
        .attr('y', 22)
        .attr('text-anchor', 'middle')
        .attr('font-size', 12)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkDark)
        .attr('font-family', "'Inter', sans-serif")
        .text('SharedArrayBuffer Memory Layout (32MB)');

      let currentY = 45;
      regions.forEach(region => {
        // Region bar
        svg
          .append('rect')
          .attr('x', startX)
          .attr('y', currentY)
          .attr('width', barWidth)
          .attr('height', region.height)
          .attr('rx', 3)
          .attr('fill', `${region.color}15`)
          .attr('stroke', region.color)
          .attr('stroke-width', 1);

        // Offset (left)
        svg
          .append('text')
          .attr('x', startX - 8)
          .attr('y', currentY + region.height / 2 + 4)
          .attr('text-anchor', 'end')
          .attr('font-size', 8)
          .attr('font-family', "'JetBrains Mono', monospace")
          .attr('fill', theme.colors.inkLight)
          .text(region.offset);

        // Label
        svg
          .append('text')
          .attr('x', startX + 12)
          .attr('y', currentY + region.height / 2 + 4)
          .attr('font-size', 10)
          .attr('font-weight', 600)
          .attr('fill', region.color)
          .attr('font-family', "'Inter', sans-serif")
          .text(region.label);

        // Size (right)
        svg
          .append('text')
          .attr('x', startX + barWidth - 12)
          .attr('y', currentY + region.height / 2 + 4)
          .attr('text-anchor', 'end')
          .attr('font-size', 9)
          .attr('font-weight', 500)
          .attr('fill', region.color)
          .attr('font-family', "'Inter', sans-serif")
          .text(region.size);

        currentY += region.height + 6;
      });
    },
    [theme]
  );

  // viewBox 660x520 as per original
  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 660 520" height={520} />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: RING BUFFER
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: RING BUFFER
// ────────────────────────────────────────────────────────────────────────────
function RingBufferDiagram() {
  const theme = useTheme();

  const renderViz = useCallback(
    (
      svg: d3.Selection<SVGSVGElement, unknown, null, undefined>,
      _width: number,
      _height: number
    ) => {
      svg.selectAll('*').remove();

      const centerX = 200;
      const centerY = 130;
      const outerRadius = 90;
      const innerRadius = 50;
      const slotCount = 8;

      // Title
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', 25)
        .attr('text-anchor', 'middle')
        .attr('font-size', 12)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkDark)
        .attr('font-family', "'Inter', sans-serif")
        .text('Zero-Copy Ring Buffer');

      // Draw ring segments
      const arc = d3.arc();

      for (let i = 0; i < slotCount; i++) {
        const startAngle = (i / slotCount) * 2 * Math.PI - Math.PI / 2;
        const endAngle = ((i + 1) / slotCount) * 2 * Math.PI - Math.PI / 2;

        // Determine state (filled, empty, head, tail)
        let fillColor = 'rgba(229, 231, 235, 0.5)';
        let strokeColor: string = theme.colors.borderSubtle;
        if (i >= 1 && i <= 4) {
          fillColor = 'rgba(139, 92, 246, 0.2)';
          strokeColor = '#8b5cf6';
        }
        if (i === 1) {
          fillColor = 'rgba(22, 163, 74, 0.3)';
          strokeColor = '#16a34a';
        }
        if (i === 4) {
          fillColor = 'rgba(234, 179, 8, 0.3)';
          strokeColor = '#eab308';
        }

        svg
          .append('path')
          .attr(
            'd',
            arc({
              innerRadius,
              outerRadius,
              startAngle,
              endAngle,
              padAngle: 0.02,
            }) || ''
          )
          .attr('transform', `translate(${centerX}, ${centerY})`)
          .attr('fill', fillColor)
          .attr('stroke', strokeColor)
          .attr('stroke-width', 1.5);
      }

      // Center label
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', centerY)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkMedium)
        .attr('font-family', "'Inter', sans-serif")
        .text('Message');

      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', centerY + 14)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkMedium)
        .attr('font-family', "'Inter', sans-serif")
        .text('Queue');

      // Legend
      const legendY = 260;
      const legendItems = [
        { color: '#16a34a', label: 'Head (read)' },
        { color: '#eab308', label: 'Tail (write)' },
        { color: '#8b5cf6', label: 'Filled slots' },
      ];

      legendItems.forEach((item, i) => {
        const x = 80 + i * 120;
        svg
          .append('rect')
          .attr('x', x)
          .attr('y', legendY)
          .attr('width', 12)
          .attr('height', 12)
          .attr('rx', 2)
          .attr('fill', item.color);

        svg
          .append('text')
          .attr('x', x + 18)
          .attr('y', legendY + 10)
          .attr('font-size', 10)
          .attr('fill', theme.colors.inkMedium)
          .attr('font-family', "'Inter', sans-serif")
          .text(item.label);
      });

      // Key operations
      const opsX = 430;
      svg
        .append('text')
        .attr('x', opsX)
        .attr('y', 60)
        .attr('font-size', 11)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkDark)
        .attr('font-family', "'Inter', sans-serif")
        .text('Zero-Copy Operations');

      const ops = [
        { label: 'EnqueueZeroCopy()', desc: 'Returns SAB offset for direct write' },
        { label: 'DequeueZeroCopy()', desc: 'Returns SAB offset for direct read' },
        { label: 'No memcpy()', desc: 'Data never leaves SAB' },
      ];

      ops.forEach((op, i) => {
        const y = 90 + i * 55;
        svg
          .append('text')
          .attr('x', opsX)
          .attr('y', y)
          .attr('font-size', 10)
          .attr('font-weight', 600)
          .attr('fill', '#8b5cf6')
          .attr('font-family', "'JetBrains Mono', monospace")
          .text(op.label);

        svg
          .append('text')
          .attr('x', opsX)
          .attr('y', y + 16)
          .attr('font-size', 9)
          .attr('fill', theme.colors.inkMedium)
          .attr('font-family', "'Inter', sans-serif")
          .text(op.desc);
      });
    },
    [theme]
  );

  // viewBox 660x290 as per original
  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 660 290" height={290} />
  );
}

export function ZeroCopy() {
  return (
    <Style.BlogContainer>
      <Style.SectionTitle>Deep Dive</Style.SectionTitle>
      <Style.PageTitle>Zero-Copy I/O</Style.PageTitle>

      <Style.LeadParagraph>
        Data copying is the silent killer of performance. Every time bytes move from one place to
        another, CPU cycles burn, caches invalidate, and garbage collectors wake up. This deep dive
        explains how INOS eliminates copying entirely using SharedArrayBuffer.
      </Style.LeadParagraph>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION: WHAT IS SHARED MEMORY? */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>Lesson 1: What is Shared Memory?</h3>
        <p>
          In computing, memory is typically <strong>private to each process</strong>. When your
          browser opens two tabs, each tab has its own memory space. This isolation is intentional:
          it prevents one program from corrupting another.
        </p>
        <p>
          But isolation creates a problem. When two parts of a program need to share data, they must
          <strong> copy it</strong>. The data leaves one memory space, travels through the operating
          system, and arrives in another. This copy takes time and uses resources.
        </p>
        <p style={{ marginBottom: 0 }}>
          <strong>Shared memory</strong> is a region that multiple threads or processes can access
          directly. Instead of copying data, they read and write to the same bytes. The challenge is
          coordination: without careful synchronization, two writers can corrupt the data.
        </p>
      </Style.ContentCard>

      <Style.DefinitionBox>
        <h4>SharedArrayBuffer</h4>
        <p>
          A <code>SharedArrayBuffer</code> is a JavaScript object representing a fixed-length region
          of raw binary data that can be shared between the main thread and Web Workers. Unlike
          regular
          <code>ArrayBuffer</code>, its contents are visible to multiple threads simultaneously.
        </p>
      </Style.DefinitionBox>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION: HISTORY */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>Lesson 2: The History of SharedArrayBuffer</h3>
        <p>
          SharedArrayBuffer has a fascinating and turbulent history in web development.
          Understanding this history explains why INOS's use of it is both powerful and carefully
          considered.
        </p>
      </Style.ContentCard>

      <Style.Timeline>
        <Style.TimelineItem>
          <Style.TimelineYear>2016</Style.TimelineYear>
          SharedArrayBuffer introduced in Chrome 60 and Firefox 55. Developers gain the ability to
          share memory between the main thread and Web Workers for the first time.
        </Style.TimelineItem>
        <Style.TimelineItem>
          <Style.TimelineYear>2018</Style.TimelineYear>
          <strong>Spectre vulnerability discovered.</strong> Attackers could use SharedArrayBuffer
          to create high-resolution timers, enabling side-channel attacks that read arbitrary
          memory. All browsers disable SharedArrayBuffer as an emergency measure.
        </Style.TimelineItem>
        <Style.TimelineItem>
          <Style.TimelineYear>2020</Style.TimelineYear>
          Browsers re-enable SharedArrayBuffer with strict requirements: sites must set
          Cross-Origin-Opener-Policy (COOP) and Cross-Origin-Embedder-Policy (COEP) headers. This
          "cross-origin isolation" prevents the timing attacks that Spectre exploited.
        </Style.TimelineItem>
        <Style.TimelineItem>
          <Style.TimelineYear>2024</Style.TimelineYear>
          SharedArrayBuffer is now stable and widely available. WebAssembly threads depend on it.
          Chrome, Firefox, Safari, and Edge all support cross-origin isolated contexts.
        </Style.TimelineItem>
      </Style.Timeline>

      <Style.WarningCard>
        <h4>⚠️ Required HTTP Headers</h4>
        <p>
          To use SharedArrayBuffer, your server must send these headers:
          <br />
          <code>Cross-Origin-Opener-Policy: same-origin</code>
          <br />
          <code>Cross-Origin-Embedder-Policy: require-corp</code>
          <br />
          Without these, <code>SharedArrayBuffer</code> will be <code>undefined</code>.
        </p>
      </Style.WarningCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION: THE COPY TAX */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>Lesson 3: The Copy Tax in Traditional Architectures</h3>
        <p>
          Consider a typical web application running a physics simulation. The simulation runs in a
          Web Worker to avoid blocking the main thread. Every frame, the worker calculates positions
          for 10,000 particles and sends them to the main thread for rendering.
        </p>
        <p>In a traditional architecture, this data flow involves multiple copies:</p>
        <ol>
          <li>
            <strong>Serialize:</strong> The worker converts the Float32Array to a transferable
            format
          </li>
          <li>
            <strong>Structured Clone:</strong> The browser copies the data into the message queue
          </li>
          <li>
            <strong>Deserialize:</strong> The main thread reconstructs the typed array
          </li>
          <li>
            <strong>GC Pressure:</strong> Old arrays are garbage collected, causing pauses
          </li>
        </ol>
        <p style={{ marginBottom: 0 }}>
          For 10,000 particles × 3 floats × 4 bytes × 60 FPS, that's <strong>7.2 MB/second</strong>{' '}
          of copying. In a multi-module system like INOS, this multiplies across every boundary.
        </p>
      </Style.ContentCard>

      <ScrollReveal>
        <Style.IllustrationContainer>
          <Style.IllustrationHeader>
            <Style.IllustrationTitle>Traditional vs Zero-Copy</Style.IllustrationTitle>
          </Style.IllustrationHeader>
          <CopyTaxDiagram />
          <Style.IllustrationCaption>
            Traditional IPC requires 4 copies; INOS requires 0
          </Style.IllustrationCaption>
        </Style.IllustrationContainer>
      </ScrollReveal>

      <Style.ComparisonGrid>
        <Style.ComparisonCard $type="bad">
          <h4>❌ Traditional (postMessage)</h4>
          <p>
            Each inter-module message triggers structured cloning. For real-time applications
            running at 60+ FPS, this creates constant GC pressure and unpredictable frame drops.
          </p>
        </Style.ComparisonCard>
        <Style.ComparisonCard $type="good">
          <h4>✓ INOS (SharedArrayBuffer)</h4>
          <p>
            Modules write directly to SAB memory. Readers access the same bytes. No copying, no
            serialization, no GC. Memory stays in place; only pointers move.
          </p>
        </Style.ComparisonCard>
      </Style.ComparisonGrid>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION: SAB LAYOUT */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>Lesson 4: INOS SAB Memory Layout</h3>
        <p>
          INOS allocates a single SharedArrayBuffer (32MB by default) at application startup. This
          buffer is divided into fixed regions, each serving a specific purpose. All offsets are
          defined in a Cap'n Proto schema to ensure consistency across Go, Rust, and JavaScript.
        </p>
        <p style={{ marginBottom: 0 }}>
          The layout is designed for <strong>cache efficiency</strong>. Frequently-accessed data
          (like epoch counters) lives in the first cache lines. Large, bulk data (like render
          buffers) lives in the dynamic arena at the end.
        </p>
      </Style.ContentCard>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>SAB Memory Regions</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <SABRegionsDiagram />
        <Style.IllustrationCaption>
          Offsets from protocols/schemas/system/v1/sab_layout.capnp
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.HistoryCard>
        <h4>📐 Cap'n Proto as a Lens</h4>
        <p>
          INOS uses Cap'n Proto not for serialization, but as a <strong>memory view</strong>.
          Reading a 64-bit float from a bird's position is a single pointer offset, completed in
          nanoseconds regardless of how many birds exist. No parsing. No allocation. Just
          arithmetic.
        </p>
        <p>
          This is fundamentally different from JSON or Protocol Buffers, which require decoding the
          entire message before accessing any field.
        </p>
      </Style.HistoryCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION: MESSAGE QUEUE */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>Lesson 5: Zero-Copy Message Queues</h3>
        <p>
          Even with shared memory, modules need a way to send messages to each other. INOS
          implements a <strong>zero-copy ring buffer</strong> pattern where the queue returns SAB
          offsets, not data copies.
        </p>
        <p style={{ marginBottom: 0 }}>
          When a producer enqueues a message, it receives an offset pointing to the message payload
          location in SAB. The producer writes directly to that offset. The consumer dequeues and
          receives the same offset, reading directly from SAB. The data never moves.
        </p>
      </Style.ContentCard>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Ring Buffer Pattern</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <RingBufferDiagram />
        <Style.IllustrationCaption>
          EnqueueZeroCopy and DequeueZeroCopy return offsets, not data
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.ContentCard>
        <h3>Implementation: Go SAB Bridge</h3>
        <p>
          The Go kernel uses <code>SABBridge</code> in{' '}
          <code>kernel/threads/supervisor/sab_bridge.go</code> to communicate with modules via SAB.
          Go cannot directly manipulate SAB memory, so it uses <code>js.CopyBytesToGo</code> for the
          bridging layer.
        </p>
      </Style.ContentCard>

      <Style.CodeBlock>
        {`// sab_bridge.go - ReadAt reads from SAB into Go memory
// Uses cached JS views to prevent memory leaks
func (sb *SABBridge) ReadAt(offset uint32, dest []byte) error {
    size := uint32(len(dest))
    if offset+size > sb.sabSize {
        return fmt.Errorf("out of bounds: off=%d len=%d cap=%d", 
            offset, size, sb.sabSize)
    }
    
    // Go's linear memory is distinct from SAB.
    // Use cached subarray view for efficiency.
    subView := sb.getCachedView(offset, size)
    if !subView.IsUndefined() {
        copied := js.CopyBytesToGo(dest, subView)
        if uint32(copied) != size {
            return fmt.Errorf("failed to copy all bytes")
        }
        return nil
    }
    return nil
}

// WriteRaw writes Go bytes directly to SAB
func (sb *SABBridge) WriteRaw(offset uint32, data []byte) error {
    subView := sb.getCachedView(offset, uint32(len(data)))
    copied := js.CopyBytesToJS(subView, data)
    if copied != len(data) {
        return fmt.Errorf("failed to copy all bytes to JS")
    }
    return nil
}`}
      </Style.CodeBlock>

      <Style.ContentCard>
        <h3>Implementation: Rust SafeSAB</h3>
        <p>
          Rust modules use <code>SafeSAB</code> from <code>modules/sdk/src/sab.rs</code> for safe,
          bounds-checked access. Memory barriers ensure visibility across threads.
        </p>
      </Style.ContentCard>

      <Style.CodeBlock>
        {`// sab.rs - Safe wrapper around SharedArrayBuffer
pub struct SafeSAB {
    buffer: BufferHandle,
    barrier_view: JsValue,  // Pre-cached Int32Array for Atomics
    base_offset: usize,
    capacity: usize,
}

impl SafeSAB {
    /// Safe write with memory barriers
    pub fn write(&self, offset: usize, data: &[u8]) -> Result<usize, String> {
        self.bounds_check(offset, data.len())?;
        
        // Acquire barrier before writing
        self.memory_barrier_acquire(offset);
        
        let abs_offset = self.base_offset + offset;
        crate::js_interop::copy_to_sab(self.as_js(), abs_offset as u32, data);
        
        // Release barrier after writing
        self.memory_barrier_release(offset);
        
        Ok(data.len())
    }
    
    /// Safe read with memory barriers
    pub fn read(&self, offset: usize, length: usize) -> Result<Vec<u8>, String> {
        self.bounds_check(offset, length)?;
        
        self.memory_barrier_acquire(offset);
        
        let mut slice = vec![0u8; length];
        let abs_offset = self.base_offset + offset;
        crate::js_interop::copy_from_sab(self.as_js(), abs_offset as u32, &mut slice);
        
        self.memory_barrier_release(offset);
        Ok(slice)
    }
}`}
      </Style.CodeBlock>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION: CONSTRAINTS */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>Lesson 6: Constraints and Trade-offs</h3>
        <p>
          Zero-copy is powerful, but it comes with constraints that every INOS developer must
          understand:
        </p>
        <ul>
          <li>
            <strong>Cross-Origin Isolation:</strong> The COOP/COEP headers prevent embedding
            third-party content without explicit permission. This breaks some common patterns like
            lazy-loading Google Fonts or embedding YouTube videos.
          </li>
          <li>
            <strong>Go WASM Limitation:</strong> Go's WASM runtime cannot directly access SAB
            memory. INOS uses a "Memory Twin" pattern where the kernel copies specific regions via
            <code>js.CopyBytesToGo</code> for decision-making, then writes results back.
          </li>
          <li>
            <strong>Alignment Requirements:</strong> Atomic operations require 4-byte alignment. The
            SAB layout is carefully designed so all atomic fields start at aligned offsets.
          </li>
          <li>
            <strong>Fixed Layout:</strong> Changing the SAB layout requires updating all three
            languages (Go, Rust, JS) simultaneously. The Cap'n Proto schema is the source of truth.
          </li>
        </ul>
      </Style.ContentCard>

      <Style.DefinitionBox>
        <h4>Memory Twin Pattern</h4>
        <p>
          Go cannot directly operate on SAB bytes due to WASM runtime constraints. Instead, the
          kernel
          <strong>bridges</strong> specific regions: it reads from SAB into Go memory for analysis,
          makes decisions using Go's concurrency primitives (goroutines, channels), then writes
          results back to SAB. This keeps Go's garbage collector isolated from the shared memory
          space.
        </p>
      </Style.DefinitionBox>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION: KEY TAKEAWAYS */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>Key Takeaways</h3>
        <ol>
          <li>
            <strong>SharedArrayBuffer enables true shared memory</strong> between JavaScript threads
            and WebAssembly modules. It was temporarily disabled due to Spectre but is now stable
            with cross-origin isolation.
          </li>
          <li>
            <strong>Zero-copy eliminates the serialization tax</strong> that cripples traditional
            web architectures. Data stays in place; only pointers move.
          </li>
          <li>
            <strong>INOS divides SAB into fixed regions</strong> defined by a Cap'n Proto schema.
            All three languages (Go, Rust, JS) agree on the exact layout.
          </li>
          <li>
            <strong>Zero-copy queues return offsets, not data.</strong> The MessageQueue's
            EnqueueZeroCopy/DequeueZeroCopy pattern is the foundation of all inter-module
            communication.
          </li>
          <li>
            <strong>Go uses a Memory Twin pattern</strong> to work around WASM limitations while
            keeping its garbage collector isolated from SAB.
          </li>
        </ol>
      </Style.ContentCard>

      <ChapterNav
        prev={{ to: '/architecture', title: 'Architecture' }}
        next={{ to: '/deep-dives/signaling', title: 'Epoch Signaling' }}
      />
    </Style.BlogContainer>
  );
}

export default ZeroCopy;
