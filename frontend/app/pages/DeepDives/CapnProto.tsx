/**
 * INOS Technical Codex — Deep Dive: Cap'n Proto Schema DNA (Chapter 03)
 *
 * An in-depth exploration of the serialization-free paradigm,
 * the "tax" of traditional parsing, and how INOS maps schemas
 * directly to shared memory for absolute performance.
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

    ul {
      margin: ${p => p.theme.spacing[4]} 0;
      padding-left: ${p => p.theme.spacing[6]};
    }

    li {
      margin-bottom: ${p => p.theme.spacing[3]};
      line-height: 1.6;
    }
  `,

  ComparisonGrid: styled.div`
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[5]} 0;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      grid-template-columns: 1fr;
    }
  `,

  ComparisonCard: styled.div<{ $type: 'taxed' | 'fast' | 'hero' }>`
    background: ${p =>
      p.$type === 'taxed'
        ? 'rgba(239, 68, 68, 0.05)'
        : p.$type === 'fast'
          ? 'rgba(59, 130, 246, 0.05)'
          : 'rgba(16, 185, 129, 0.08)'};
    backdrop-filter: blur(12px);
    border: 1px solid
      ${p =>
        p.$type === 'taxed'
          ? 'rgba(239, 68, 68, 0.2)'
          : p.$type === 'fast'
            ? 'rgba(59, 130, 246, 0.2)'
            : 'rgba(16, 185, 129, 0.3)'};
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};

    h4 {
      margin: 0 0 ${p => p.theme.spacing[2]} 0;
      color: ${p => (p.$type === 'taxed' ? '#ef4444' : p.$type === 'fast' ? '#3b82f6' : '#059669')};
      font-family: ${p => p.theme.fonts.typewriter};
      text-transform: uppercase;
      font-size: 11px;
      letter-spacing: 0.05em;
    }

    .desc {
      font-size: 13px;
      line-height: 1.5;
      color: ${p => p.theme.colors.inkMedium};
    }
  `,

  DefinitionBox: styled.div`
    background: rgba(16, 185, 129, 0.08);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(16, 185, 129, 0.2);
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[6]} 0;

    h4 {
      margin: 0 0 ${p => p.theme.spacing[2]} 0;
      color: #059669;
      font-size: ${p => p.theme.fontSizes.lg};
    }

    p {
      margin: 0;
      line-height: 1.7;
    }

    code {
      background: rgba(16, 185, 129, 0.1);
      padding: 2px 4px;
      border-radius: 3px;
      font-family: inherit;
    }
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
};

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: THE SERIALIZATION TAX
// ────────────────────────────────────────────────────────────────────────────
function SerializationTaxDiagram() {
  const theme = useTheme();

  const renderViz = useCallback(
    (svg: d3.Selection<SVGSVGElement, unknown, null, undefined>, width: number) => {
      svg.selectAll('*').remove();
      const centerX = width / 2;
      const boxW = 140;
      const boxH = 60;

      // Traditional Path
      const taxedY = 70;
      svg
        .append('text')
        .attr('x', 50)
        .attr('y', taxedY - 40)
        .attr('font-size', 11)
        .attr('font-weight', 800)
        .attr('fill', '#ef4444')
        .attr('letter-spacing', '0.05em')
        .text('TRADITIONAL: THE SERIALIZATION TAX');

      // BOX 1: Native Memory
      svg
        .append('rect')
        .attr('x', centerX - 280)
        .attr('y', taxedY)
        .attr('width', boxW)
        .attr('height', boxH)
        .attr('rx', 4)
        .attr('fill', 'rgba(239, 68, 68, 0.05)')
        .attr('stroke', '#ef4444');
      svg
        .append('text')
        .attr('x', centerX - 210)
        .attr('y', taxedY + 35)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('font-family', theme.fonts.typewriter)
        .text('Kernel Memory');

      // Arrow 1: Encode
      svg
        .append('line')
        .attr('x1', centerX - 135)
        .attr('y1', taxedY + 30)
        .attr('x2', centerX - 75)
        .attr('y2', taxedY + 30)
        .attr('stroke', '#ef4444')
        .attr('marker-end', 'url(#arrow-red)');
      svg
        .append('text')
        .attr('x', centerX - 105)
        .attr('y', taxedY + 25)
        .attr('text-anchor', 'middle')
        .attr('font-size', 8)
        .attr('font-weight', 600)
        .text('ENCODE');

      // BOX 2: Buffer (Intermediate)
      svg
        .append('rect')
        .attr('x', centerX - 70)
        .attr('y', taxedY)
        .attr('width', boxW)
        .attr('height', boxH)
        .attr('rx', 4)
        .attr('fill', '#fff')
        .attr('stroke', '#ef4444')
        .attr('stroke-dasharray', '4,2');
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', taxedY + 35)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .text('JSON / Proto');

      // Arrow 2: Decode
      svg
        .append('line')
        .attr('x1', centerX + 75)
        .attr('y1', taxedY + 30)
        .attr('x2', centerX + 135)
        .attr('y2', taxedY + 30)
        .attr('stroke', '#ef4444')
        .attr('marker-end', 'url(#arrow-red)');
      svg
        .append('text')
        .attr('x', centerX + 105)
        .attr('y', taxedY + 25)
        .attr('text-anchor', 'middle')
        .attr('font-size', 8)
        .attr('font-weight', 600)
        .text('DECODE');

      // BOX 3: Native Memory
      svg
        .append('rect')
        .attr('x', centerX + 140)
        .attr('y', taxedY)
        .attr('width', boxW)
        .attr('height', boxH)
        .attr('rx', 4)
        .attr('fill', 'rgba(239, 68, 68, 0.05)')
        .attr('stroke', '#ef4444');
      svg
        .append('text')
        .attr('x', centerX + 210)
        .attr('y', taxedY + 35)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('font-family', theme.fonts.typewriter)
        .text('User Memory');

      // Cap'n Proto Path (Zero-Copy)
      const capY = 220; // Increased spacing
      svg
        .append('text')
        .attr('x', 50)
        .attr('y', capY - 50) // More space under title as requested
        .attr('font-size', 11)
        .attr('font-weight', 800)
        .attr('fill', '#059669')
        .attr('letter-spacing', '0.05em')
        .text('INOS: ZERO-COPY SCHEMA');

      // Shared Memory
      svg
        .append('rect')
        .attr('x', centerX - 280)
        .attr('y', capY)
        .attr('width', 560)
        .attr('height', boxH)
        .attr('rx', 4)
        .attr('fill', 'rgba(16, 185, 129, 0.08)')
        .attr('stroke', '#10b981')
        .attr('stroke-width', 2);
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', capY + 35)
        .attr('text-anchor', 'middle')
        .attr('font-size', 11)
        .attr('font-weight', 800)
        .attr('fill', '#059669')
        .text('SHAREDARRAYBUFFER (THE TRUTH)');

      // Accessors
      [
        [centerX - 180, 'Go'],
        [centerX, 'Rust'],
        [centerX + 180, 'JS'],
      ].forEach(([x, label]) => {
        svg
          .append('line')
          .attr('x1', x as number)
          .attr('y1', capY - 35)
          .attr('y2', capY - 5)
          .attr('x2', x as number)
          .attr('stroke', '#10b981')
          .attr('stroke-width', 1.5)
          .attr('marker-end', 'url(#arrow-green)');
        svg
          .append('text')
          .attr('x', x as number)
          .attr('y', capY - 40)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9)
          .attr('font-weight', 700)
          .attr('fill', '#059669')
          .text(label as string);
      });

      // Markers
      const defs = svg.append('defs');
      defs
        .append('marker')
        .attr('id', 'arrow-red')
        .attr('viewBox', '0 0 10 10')
        .attr('refX', 9)
        .attr('refY', 5)
        .attr('markerWidth', 6)
        .attr('markerHeight', 6)
        .attr('orient', 'auto')
        .append('path')
        .attr('d', 'M 0 0 L 10 5 L 0 10 z')
        .attr('fill', '#ef4444');
      defs
        .append('marker')
        .attr('id', 'arrow-green')
        .attr('viewBox', '0 0 10 10')
        .attr('refX', 9)
        .attr('refY', 5)
        .attr('markerWidth', 6)
        .attr('markerHeight', 6)
        .attr('orient', 'auto')
        .append('path')
        .attr('d', 'M 0 0 L 10 5 L 0 10 z')
        .attr('fill', '#10b981');
    },
    [theme]
  );

  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 700 320" height={320} />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: FORMAT COMPARISON
// ────────────────────────────────────────────────────────────────────────────
function FormatComparisonDiagram() {
  const theme = useTheme();

  const renderViz = useCallback(
    (svg: d3.Selection<SVGSVGElement, unknown, null, undefined>, _width: number) => {
      svg.selectAll('*').remove();

      // JSON Layout
      const jsonY = 40;
      svg
        .append('text')
        .attr('x', 20)
        .attr('y', jsonY)
        .attr('font-size', 10)
        .attr('font-weight', 800)
        .attr('fill', theme.colors.inkMedium)
        .text('JSON (TEXT-BASED)');
      const jsonStr = '{"id": 123, "val" : 45.67}';
      const jsonGroup = svg.append('g').attr('transform', `translate(20, ${jsonY + 10})`);
      jsonGroup
        .append('rect')
        .attr('width', 300)
        .attr('height', 30)
        .attr('rx', 4)
        .attr('fill', '#f1f5f9')
        .attr('stroke', '#cbd5e1');
      jsonStr.split('').forEach((char, i) => {
        jsonGroup
          .append('text')
          .attr('x', 10 + i * 11)
          .attr('y', 20)
          .attr('font-size', 11)
          .attr('font-family', 'JetBrains Mono')
          .attr('fill', '#475569')
          .text(char);
      });
      svg
        .append('text')
        .attr('x', 330)
        .attr('y', jsonY + 30)
        .attr('font-size', 9)
        .attr('fill', '#ef4444')
        .text('← MUST PARSE ENTIRE STRING');

      // Protobuf Layout
      const protoY = 130;
      svg
        .append('text')
        .attr('x', 20)
        .attr('y', protoY)
        .attr('font-size', 10)
        .attr('font-weight', 800)
        .attr('fill', theme.colors.inkMedium)
        .text('PROTOBUF (PACKED BINARY)');
      const protoGroup = svg.append('g').attr('transform', `translate(20, ${protoY + 10})`);
      const protoBytes = [
        ['0x08', '#3b82f6', 'TAG'],
        ['0x7b', '#60a5fa', '123'],
        ['0x11', '#3b82f6', 'TAG'],
        ['0xcd', '#60a5fa', '45.67...'],
      ];
      protoBytes.forEach((byte, i) => {
        const x = i * 60;
        protoGroup
          .append('rect')
          .attr('x', x)
          .attr('width', 55)
          .attr('height', 30)
          .attr('rx', 4)
          .attr('fill', (byte[1] as string) + '15')
          .attr('stroke', byte[1] as string);
        protoGroup
          .append('text')
          .attr('x', x + 27)
          .attr('y', 20)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9)
          .attr('font-family', 'JetBrains Mono')
          .text(byte[0]);
        protoGroup
          .append('text')
          .attr('x', x + 27)
          .attr('y', 45)
          .attr('text-anchor', 'middle')
          .attr('font-size', 8)
          .attr('fill', theme.colors.inkMedium)
          .text(byte[2]);
      });
      svg
        .append('text')
        .attr('x', 270)
        .attr('y', protoY + 30)
        .attr('font-size', 9)
        .attr('fill', '#f59e0b')
        .text('← SEQUENTIAL UNPACKING REQUIRED');

      // Cap\'n Proto Layout
      const capY = 220;
      svg
        .append('text')
        .attr('x', 20)
        .attr('y', capY)
        .attr('font-size', 10)
        .attr('font-weight', 800)
        .attr('fill', '#059669')
        .text("CAP'N PROTO (MEMORY MAP)");
      const capGroup = svg.append('g').attr('transform', `translate(20, ${capY + 10})`);
      const capLayout = [
        { x: 0, w: 100, label: 'Pointers', color: '#10b981' },
        { x: 105, w: 60, label: 'Int: 123', color: '#059669' },
        { x: 170, w: 120, label: 'Float: 45.67', color: '#059669' },
        { x: 295, w: 80, label: 'Padding', color: '#64748b' },
      ];
      capLayout.forEach(sect => {
        capGroup
          .append('rect')
          .attr('x', sect.x)
          .attr('width', sect.w)
          .attr('height', 30)
          .attr('rx', 4)
          .attr('fill', sect.color + '15')
          .attr('stroke', sect.color);
        capGroup
          .append('text')
          .attr('x', sect.x + sect.w / 2)
          .attr('y', 20)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9)
          .attr('font-weight', 600)
          .text(sect.label);
      });

      // Pointer Arrow
      capGroup
        .append('path')
        .attr('d', 'M 50,0 Q 50,-20 135,15')
        .attr('fill', 'none')
        .attr('stroke', '#10b981')
        .attr('stroke-width', 1.5)
        .attr('marker-end', 'url(#arrow-green)');

      svg
        .append('text')
        .attr('x', 400)
        .attr('y', capY + 30)
        .attr('font-size', 10)
        .attr('font-weight', 700)
        .attr('fill', '#059669')
        .text('DIRECT O(1) JUMP');
    },
    [theme]
  );

  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 700 300" height={300} />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: THE MEMORY TWIN
// ────────────────────────────────────────────────────────────────────────────
function MemoryTwinDiagram() {
  const theme = useTheme();

  const renderViz = useCallback(
    (svg: d3.Selection<SVGSVGElement, unknown, null, undefined>, width: number) => {
      svg.selectAll('*').remove();
      const centerX = width / 2;
      const boxW = 120;
      const boxH = 100;

      // Go Private Memory
      svg
        .append('rect')
        .attr('x', centerX - 250)
        .attr('y', 80)
        .attr('width', boxW)
        .attr('height', boxH)
        .attr('rx', 8)
        .attr('fill', 'rgba(0, 173, 216, 0.05)')
        .attr('stroke', '#00add8');
      svg
        .append('text')
        .attr('x', centerX - 250 + boxW / 2)
        .attr('y', 70)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('font-weight', 600)
        .text('GO MEMORY (TWIN A)');

      // Rust Private Memory
      svg
        .append('rect')
        .attr('x', centerX + 130)
        .attr('y', 80)
        .attr('width', boxW)
        .attr('height', boxH)
        .attr('rx', 8)
        .attr('fill', 'rgba(234, 76, 33, 0.05)')
        .attr('stroke', '#ea4c21');
      svg
        .append('text')
        .attr('x', centerX + 130 + boxW / 2)
        .attr('y', 70)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('font-weight', 600)
        .text('RUST MEMORY (TWIN B)');

      // Shared Reality
      svg
        .append('rect')
        .attr('x', centerX - 100)
        .attr('y', 60)
        .attr('width', 200)
        .attr('height', 140)
        .attr('rx', 4)
        .attr('fill', 'rgba(139, 92, 246, 0.05)')
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 2);
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', 50)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 700)
        .attr('fill', '#8b5cf6')
        .text('SHAREDARRAYBUFFER');

      // Schema Lines
      [80, 110, 140, 170].forEach(y => {
        svg
          .append('line')
          .attr('x1', centerX - 90)
          .attr('y1', y)
          .attr('x2', centerX + 90)
          .attr('y2', y)
          .attr('stroke', 'rgba(139, 92, 246, 0.2)');
      });

      // Pointers
      svg
        .append('line')
        .attr('x1', centerX - 135)
        .attr('y1', 130)
        .attr('x2', centerX - 105)
        .attr('y2', 130)
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 2)
        .attr('marker-end', 'url(#arrow-purple)');
      svg
        .append('line')
        .attr('x1', centerX + 135)
        .attr('y1', 130)
        .attr('x2', centerX + 105)
        .attr('y2', 130)
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 2)
        .attr('marker-end', 'url(#arrow-purple)');

      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', 135)
        .attr('text-anchor', 'middle')
        .attr('font-size', 8)
        .attr('fill', '#8b5cf6')
        .attr('font-family', theme.fonts.typewriter)
        .text('OFFSET 0x4200');

      // Marker
      svg
        .append('defs')
        .append('marker')
        .attr('id', 'arrow-purple')
        .attr('viewBox', '0 0 10 10')
        .attr('refX', 9)
        .attr('refY', 5)
        .attr('markerWidth', 6)
        .attr('markerHeight', 6)
        .attr('orient', 'auto')
        .append('path')
        .attr('d', 'M 0 0 L 10 5 L 0 10 z')
        .attr('fill', '#8b5cf6');
    },
    [theme]
  );

  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 700 240" height={240} />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// MAIN COMPONENT
// ────────────────────────────────────────────────────────────────────────────
export default function CapnProto() {
  return (
    <Style.BlogContainer>
      <ScrollReveal>
        <Style.SectionTitle>Chapter 03 // Cap'n Proto Schema DNA</Style.SectionTitle>
        <Style.PageTitle>The Serialization-Free Paradigm</Style.PageTitle>
        <Style.LeadParagraph>
          Legacy systems waste 40% of their CPU cycles simply "talking" to each other. INOS uses{' '}
          <strong>Cap'n Proto</strong> to map binary schemas directly to memory, eliminating the
          translation cost entirely.
        </Style.LeadParagraph>
      </ScrollReveal>

      <Style.SectionDivider />

      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 1: The Serialization Tax</h3>
          <p>
            When two modules communicate—say, a Go Kernel and a Rust Physics engine—they typically
            translate their internal state into an intermediate format like JSON or Protobuf. This
            is like two people who only speak different languages hiring a translator to write a
            transcript of their conversation.
          </p>
          <p>
            The <strong>Serialization Tax</strong> consists of:
          </p>
          <ul>
            <li>
              <strong>Encoding:</strong> Translating native objects into bytes.
            </li>
            <li>
              <strong>Decoding:</strong> Parsing those bytes back into new objects.
            </li>
            <li>
              <strong>Copying:</strong> Moving the bytes across the memory bus.
            </li>
            <li>
              <strong>Allocation:</strong> Creating thousands of temporary objects for the GC to
              clean up.
            </li>
          </ul>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Comparison: Copy/Parse vs Direct Access</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <SerializationTaxDiagram />
        <Style.IllustrationCaption>
          Traditional architectures burn CPU cycles on encoding/decoding intermediate formats. INOS
          uses a direct shared memory map for zero-allocation communication.
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.SectionDivider />

      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 2: Pointer Arithmetic vs. Recursive Parsing</h3>
          <p>
            Cap'n Proto is "serialization-free" because the bytes on the wire are identical to the
            bytes in memory. Accessing a nested field doesn't require walking a tree; it requires
            <strong> constant-time pointer arithmetic</strong>.
          </p>
          <p>
            In JSON, to find the 1,000th element in an array, you must parse the previous 999. In
            Protobuf, you must skip over field tags. In Cap'n Proto, you simply jump:
            <code>base_address + (index * stride)</code>.
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>Comparison: Parsing Latency Tiers</Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <FormatComparisonDiagram />
            <Style.IllustrationCaption>
              Text and packed formats require sequential processing. Cap'n Proto enables true random
              access.
            </Style.IllustrationCaption>
          </Style.IllustrationContainer>
          <Style.ComparisonGrid>
            <Style.ComparisonCard $type="taxed">
              <h4>JSON</h4>
              <div className="desc">
                Text-based. Must read every character to find the data you need. O(N) access time.
              </div>
            </Style.ComparisonCard>
            <Style.ComparisonCard $type="fast">
              <h4>Protobuf</h4>
              <div className="desc">
                Binary, but fields are still packed. Requires field-by-field unpacking into native
                types.
              </div>
            </Style.ComparisonCard>
            <Style.ComparisonCard $type="hero">
              <h4>Cap'n Proto</h4>
              <div className="desc">
                Zero parsing. Instant random access to any field via pointer arithmetic. O(1)
                access.
              </div>
            </Style.ComparisonCard>
          </Style.ComparisonGrid>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.CodeBlock>
        <span className="comment">// Protobuf approach (Taxed)</span>
        <br />
        <span className="keyword">const</span> <span className="function">msg</span> ={' '}
        <span className="type">BoidsMessage</span>.<span className="function">decode</span>(buffer);
        <br />
        <span className="comment">// ^ Decodes 10,000 objects into memory. CPU spikes.</span>
        <br />
        <br />
        <span className="comment">// Cap'n Proto approach (Heroic)</span>
        <br />
        <span className="keyword">const</span> <span className="function">reader</span> ={' '}
        <span className="type">msg</span>.<span className="function">getReader</span>();
        <br />
        <span className="keyword">const</span> <span className="function">boid</span> ={' '}
        <span className="type">reader</span>.<span className="function">getBoids</span>().
        <span className="function">get</span>(<span className="number">999</span>);
        <br />
        <span className="comment">// ^ Direct jump to byte 124,800. No allocation. Instant.</span>
      </Style.CodeBlock>

      <Style.SectionDivider />

      <ScrollReveal>
        <Style.ContentCard>
          <h3>The INOS Hack: The Memory Twin</h3>
          <p>
            We don't just use Cap'n Proto for messages. We use it to define the
            <strong>Shared Memory Twin</strong>.
          </p>
          <p>
            Because WebAssembly modules (Rust) and the WebWorker Kernel (Go) run in different
            sandboxes, they cannot share their private memories. However, they <em>can</em> both
            point to the same <strong>SharedArrayBuffer</strong>.
          </p>
          <p>
            The <code>.capnp</code> schema acts as the <strong>Universal Blueprint</strong>. When Go
            writes a physics matrix to <code>Offset 0x4200</code>, Rust reads it from{' '}
            <code>Offset 0x4200</code>. They aren't "passing messages"; they are looking at the same
            reality through different windows.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Concept: The Synchronized Memory Twin</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <MemoryTwinDiagram />
        <Style.IllustrationCaption>
          One buffer, two perspectives. Synchronized by the Cap'n Proto schema.
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.DefinitionBox>
        <h4>Why "Memory Twin"?</h4>
        <p>
          Just as biological twins share the same DNA but operate as separate entities, our Go and
          Rust modules share the same <strong>Memory Layout DNA</strong>
          while maintaining independent execution states. They are twins synchronized by a shared
          binary reality.
        </p>
      </Style.DefinitionBox>

      <Style.SectionDivider />

      <ChapterNav
        prev={{ title: 'Zero-Copy Memory I/O', to: '/deep-dives/zero-copy' }}
        next={{ title: 'Epoch Signaling', to: '/deep-dives/signaling' }}
      />
    </Style.BlogContainer>
  );
}
