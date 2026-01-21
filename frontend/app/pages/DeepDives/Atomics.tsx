/**
 * INOS Technical Codex — Deep Dive: Atomic Operations (Chapter 05)
 *
 * A comprehensive exploration of atomic operations, lock-free concurrency,
 * memory ordering, and how INOS leverages hardware guarantees for
 * sub-microsecond synchronization.
 *
 * Educational approach: Teach the fundamentals before showing implementation.
 */

import { useCallback, useState, useEffect } from 'react';
import styled, { useTheme } from 'styled-components';
import D3Container, { D3RenderFn } from '../../ui/D3Container';
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
      margin: 0 0 ${p => p.theme.spacing[2]} 0;
      line-height: 1.6;
      font-size: ${p => p.theme.fontSizes.sm};
    }

    p:last-child {
      margin-bottom: 0;
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

  MetricRow: styled.div`
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: ${p => p.theme.spacing[3]} 0;
    border-bottom: 1px dashed ${p => p.theme.colors.borderSubtle};

    &:last-child {
      border-bottom: none;
    }
  `,

  MetricLabel: styled.span`
    font-size: ${p => p.theme.fontSizes.sm};
    color: ${p => p.theme.colors.inkMedium};
  `,

  MetricValue: styled.span<{ $highlight?: boolean }>`
    font-family: 'JetBrains Mono', monospace;
    font-size: ${p => p.theme.fontSizes.sm};
    font-weight: 600;
    color: ${p => (p.$highlight ? '#16a34a' : p.theme.colors.inkDark)};
  `,

  PerformanceMeter: styled.div`
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 8px;
    padding: ${p => p.theme.spacing[4]};
    background: rgba(0, 0, 0, 0.03);
    margin: ${p => p.theme.spacing[6]} 0;
    font-family: ${p => p.theme.fonts.typewriter};
  `,

  MeterRow: styled.div`
    display: flex;
    justify-content: space-between;
    margin-bottom: ${p => p.theme.spacing[2]};
    font-size: 11px;
    color: ${p => p.theme.colors.inkMedium};
  `,

  MeterBar: styled.div.attrs<{ $percent: number; $color?: string }>(props => ({
    style: {
      '--meter-width': `${props.$percent}%`,
      '--meter-color': props.$color || '#8b5cf6',
    } as React.CSSProperties & Record<string, string>,
  }))<{ $percent: number; $color?: string }>`
    height: 4px;
    background: rgba(0, 0, 0, 0.1);
    border-radius: 2px;
    overflow: hidden;
    position: relative;

    &::after {
      content: '';
      position: absolute;
      left: 0;
      top: 0;
      height: 100%;
      width: var(--meter-width);
      background: var(--meter-color);
      transition: width 0.3s ease;
    }
  `,

  OrderingGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: ${p => p.theme.spacing[3]};
    margin: ${p => p.theme.spacing[5]} 0;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      grid-template-columns: repeat(2, 1fr);
    }
  `,

  OrderingCard: styled.div<{ $color: string }>`
    background: ${p => `${p.$color}10`};
    border: 1px solid ${p => `${p.$color}30`};
    border-top: 3px solid ${p => p.$color};
    border-radius: 6px;
    padding: ${p => p.theme.spacing[4]};
    text-align: center;

    h5 {
      margin: 0 0 ${p => p.theme.spacing[2]} 0;
      color: ${p => p.$color};
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }

    p {
      margin: 0;
      font-size: 11px;
      color: ${p => p.theme.colors.inkMedium};
      line-height: 1.5;
    }
  `,
};

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: LOCK CONTENTION VS LOCK-FREE
// ────────────────────────────────────────────────────────────────────────────
function LockContentionDiagram() {
  const theme = useTheme();

  const renderDiagram: D3RenderFn = useCallback(
    svg => {
      svg.selectAll('*').remove();

      const designWidth = 700;
      const mutexY = 80;
      const atomicY = 200;
      const threadColors = ['#dc2626', '#f59e0b', '#8b5cf6', '#0ea5e9'];

      // Title - Mutex approach
      svg
        .append('text')
        .attr('x', 50)
        .attr('y', 30)
        .attr('font-size', 11)
        .attr('font-weight', 700)
        .attr('fill', '#dc2626')
        .text('❌ Mutex: Threads Block Each Other');

      // Mutex lock illustration
      const lockX = designWidth / 2;
      svg
        .append('rect')
        .attr('x', lockX - 30)
        .attr('y', mutexY - 15)
        .attr('width', 60)
        .attr('height', 30)
        .attr('rx', 4)
        .attr('fill', 'rgba(220, 38, 38, 0.1)')
        .attr('stroke', '#dc2626')
        .attr('stroke-width', 2);

      svg
        .append('text')
        .attr('x', lockX)
        .attr('y', mutexY + 5)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 600)
        .attr('fill', '#dc2626')
        .text('🔒 LOCK');

      // Threads trying to access (mutex)
      const threadPositions = [100, 220, 480, 600];
      threadPositions.forEach((x, i) => {
        const isBlocked = i > 0;
        svg
          .append('circle')
          .attr('cx', x)
          .attr('cy', mutexY)
          .attr('r', 20)
          .attr('fill', isBlocked ? 'rgba(156, 163, 175, 0.2)' : `${threadColors[i]}20`)
          .attr('stroke', isBlocked ? '#9ca3af' : threadColors[i])
          .attr('stroke-width', 2)
          .attr('stroke-dasharray', isBlocked ? '4,2' : 'none');

        svg
          .append('text')
          .attr('x', x)
          .attr('y', mutexY + 5)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9)
          .attr('font-weight', 600)
          .attr('fill', isBlocked ? '#9ca3af' : threadColors[i])
          .text(`T${i + 1}`);

        svg
          .append('text')
          .attr('x', x)
          .attr('y', mutexY + 38)
          .attr('text-anchor', 'middle')
          .attr('font-size', 8)
          .attr('fill', isBlocked ? '#9ca3af' : theme.colors.inkMedium)
          .text(isBlocked ? 'BLOCKED' : 'RUNNING');

        // Arrow to lock
        if (i === 0) {
          svg
            .append('line')
            .attr('x1', x + 25)
            .attr('y1', mutexY)
            .attr('x2', lockX - 35)
            .attr('y2', mutexY)
            .attr('stroke', threadColors[i])
            .attr('stroke-width', 2)
            .attr('marker-end', 'url(#arrow-red)');
        }
      });

      // Title - Atomic approach
      svg
        .append('text')
        .attr('x', 50)
        .attr('y', 150)
        .attr('font-size', 11)
        .attr('font-weight', 700)
        .attr('fill', '#16a34a')
        .text('✓ Atomics: All Threads Progress');

      // Shared memory cell
      svg
        .append('rect')
        .attr('x', lockX - 40)
        .attr('y', atomicY - 15)
        .attr('width', 80)
        .attr('height', 30)
        .attr('rx', 4)
        .attr('fill', 'rgba(22, 163, 74, 0.1)')
        .attr('stroke', '#16a34a')
        .attr('stroke-width', 2);

      svg
        .append('text')
        .attr('x', lockX)
        .attr('y', atomicY + 5)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 600)
        .attr('fill', '#16a34a')
        .text('⚡ ATOMIC');

      // Threads accessing simultaneously
      threadPositions.forEach((x, i) => {
        svg
          .append('circle')
          .attr('cx', x)
          .attr('cy', atomicY)
          .attr('r', 20)
          .attr('fill', `${threadColors[i]}20`)
          .attr('stroke', threadColors[i])
          .attr('stroke-width', 2);

        svg
          .append('text')
          .attr('x', x)
          .attr('y', atomicY + 5)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9)
          .attr('font-weight', 600)
          .attr('fill', threadColors[i])
          .text(`T${i + 1}`);

        svg
          .append('text')
          .attr('x', x)
          .attr('y', atomicY + 38)
          .attr('text-anchor', 'middle')
          .attr('font-size', 8)
          .attr('fill', '#16a34a')
          .text('RUNNING');

        // All arrows converge (dashed for simultaneous)
        const targetX = x < lockX ? lockX - 45 : lockX + 45;
        svg
          .append('line')
          .attr('x1', x + (x < lockX ? 25 : -25))
          .attr('y1', atomicY)
          .attr('x2', targetX)
          .attr('y2', atomicY)
          .attr('stroke', threadColors[i])
          .attr('stroke-width', 1.5)
          .attr('stroke-dasharray', '4,2');
      });

      // Arrow marker definition
      svg
        .append('defs')
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
        .attr('fill', '#dc2626');
    },
    [theme]
  );

  return (
    <D3Container
      render={renderDiagram}
      dependencies={[renderDiagram]}
      viewBox="0 0 700 270"
      height={270}
    />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: COMPARE-AND-SWAP ANIMATION
// ────────────────────────────────────────────────────────────────────────────
function AtomicCASDiagram() {
  const theme = useTheme();

  const renderViz: D3RenderFn = useCallback(
    svg => {
      svg.selectAll('*').remove();

      const designWidth = 700;
      const centerX = designWidth / 2;
      const centerY = 120;

      // Title
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', 25)
        .attr('text-anchor', 'middle')
        .attr('font-size', 12)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkDark)
        .text('Compare-And-Swap (CAS) Operation');

      // Memory cell
      const cellWidth = 100;
      const cellHeight = 60;
      const cell = svg
        .append('g')
        .attr('transform', `translate(${centerX - cellWidth / 2}, ${centerY - cellHeight / 2})`);

      cell
        .append('rect')
        .attr('width', cellWidth)
        .attr('height', cellHeight)
        .attr('rx', 6)
        .attr('fill', 'rgba(139, 92, 246, 0.1)')
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 2);

      cell
        .append('text')
        .attr('x', cellWidth / 2)
        .attr('y', 20)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('fill', theme.colors.inkLight)
        .text('MEMORY');

      const valText = cell
        .append('text')
        .attr('x', cellWidth / 2)
        .attr('y', 45)
        .attr('text-anchor', 'middle')
        .attr('font-size', 24)
        .attr('font-weight', 700)
        .attr('fill', '#8b5cf6')
        .attr('font-family', "'JetBrains Mono', monospace")
        .text('42');

      // Expected value (left)
      const expectedX = centerX - 200;
      svg
        .append('rect')
        .attr('x', expectedX - 40)
        .attr('y', centerY - 30)
        .attr('width', 80)
        .attr('height', 60)
        .attr('rx', 6)
        .attr('fill', 'rgba(100, 116, 139, 0.1)')
        .attr('stroke', '#64748b')
        .attr('stroke-width', 1);

      svg
        .append('text')
        .attr('x', expectedX)
        .attr('y', centerY - 10)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('fill', theme.colors.inkLight)
        .text('EXPECTED');

      svg
        .append('text')
        .attr('x', expectedX)
        .attr('y', centerY + 20)
        .attr('text-anchor', 'middle')
        .attr('font-size', 20)
        .attr('font-weight', 700)
        .attr('fill', '#64748b')
        .attr('font-family', "'JetBrains Mono', monospace")
        .text('42');

      // Desired value (right)
      const desiredX = centerX + 200;
      svg
        .append('rect')
        .attr('x', desiredX - 40)
        .attr('y', centerY - 30)
        .attr('width', 80)
        .attr('height', 60)
        .attr('rx', 6)
        .attr('fill', 'rgba(22, 163, 74, 0.1)')
        .attr('stroke', '#16a34a')
        .attr('stroke-width', 1);

      svg
        .append('text')
        .attr('x', desiredX)
        .attr('y', centerY - 10)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('fill', theme.colors.inkLight)
        .text('DESIRED');

      svg
        .append('text')
        .attr('x', desiredX)
        .attr('y', centerY + 20)
        .attr('text-anchor', 'middle')
        .attr('font-size', 20)
        .attr('font-weight', 700)
        .attr('fill', '#16a34a')
        .attr('font-family', "'JetBrains Mono', monospace")
        .text('43');

      // Step labels
      const stepsY = 210;
      const steps = [
        { x: expectedX, label: '1. Compare', sublabel: 'Expected == Current?' },
        { x: centerX, label: '2. Match?', sublabel: 'If yes → proceed' },
        { x: desiredX, label: '3. Swap', sublabel: 'Write new value' },
      ];

      steps.forEach(step => {
        svg
          .append('text')
          .attr('x', step.x)
          .attr('y', stepsY)
          .attr('text-anchor', 'middle')
          .attr('font-size', 10)
          .attr('font-weight', 600)
          .attr('fill', theme.colors.inkDark)
          .text(step.label);

        svg
          .append('text')
          .attr('x', step.x)
          .attr('y', stepsY + 15)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9)
          .attr('fill', theme.colors.inkLight)
          .text(step.sublabel);
      });

      // Animation
      let timerId: number;

      function animate() {
        valText.text('42').style('fill', '#8b5cf6');

        // Compare arrow
        const compareArrow = svg
          .append('line')
          .attr('x1', expectedX + 45)
          .attr('y1', centerY)
          .attr('x2', centerX - cellWidth / 2 - 5)
          .attr('y2', centerY)
          .attr('stroke', '#64748b')
          .attr('stroke-width', 2)
          .attr('stroke-dasharray', '8,4')
          .style('opacity', 0);

        compareArrow
          .transition()
          .duration(500)
          .style('opacity', 1)
          .transition()
          .delay(800)
          .duration(300)
          .style('opacity', 0)
          .remove();

        // Check mark appears
        const checkMark = svg
          .append('text')
          .attr('x', centerX)
          .attr('y', centerY + 55)
          .attr('text-anchor', 'middle')
          .attr('font-size', 16)
          .style('opacity', 0)
          .text('✓ Match!');

        checkMark
          .transition()
          .delay(1000)
          .duration(300)
          .style('opacity', 1)
          .attr('fill', '#16a34a')
          .transition()
          .delay(1500)
          .duration(300)
          .style('opacity', 0)
          .remove();

        // Swap arrow
        const swapArrow = svg
          .append('line')
          .attr('x1', desiredX - 45)
          .attr('y1', centerY)
          .attr('x2', centerX + cellWidth / 2 + 5)
          .attr('y2', centerY)
          .attr('stroke', '#16a34a')
          .attr('stroke-width', 2)
          .style('opacity', 0);

        swapArrow
          .transition()
          .delay(1500)
          .duration(500)
          .style('opacity', 1)
          .transition()
          .delay(500)
          .duration(300)
          .style('opacity', 0)
          .remove();

        // Update value
        valText
          .transition()
          .delay(2000)
          .duration(200)
          .style('opacity', 0)
          .on('end', () => {
            valText.text('43').style('fill', '#16a34a');
            valText.transition().duration(200).style('opacity', 1);

            // Success burst
            cell
              .append('rect')
              .attr('width', cellWidth)
              .attr('height', cellHeight)
              .attr('rx', 6)
              .attr('fill', 'none')
              .attr('stroke', '#16a34a')
              .attr('stroke-width', 4)
              .transition()
              .duration(500)
              .attr('transform', 'scale(1.3) translate(-15, -9)')
              .style('opacity', 0)
              .remove();

            timerId = window.setTimeout(animate, 2500);
          });
      }

      animate();

      return () => {
        if (timerId) clearTimeout(timerId);
        svg.selectAll('*').interrupt();
      };
    },
    [theme]
  );

  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 700 250" height={250} />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: MEMORY ORDERING
// ────────────────────────────────────────────────────────────────────────────
function MemoryOrderingDiagram() {
  const theme = useTheme();

  const renderDiagram: D3RenderFn = useCallback(
    svg => {
      svg.selectAll('*').remove();

      const designWidth = 700;
      const centerX = designWidth / 2;
      const timelineY = 100;

      // Title
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', 25)
        .attr('text-anchor', 'middle')
        .attr('font-size', 12)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkDark)
        .text('Memory Barriers Ensure Visibility');

      // Timeline
      svg
        .append('line')
        .attr('x1', 50)
        .attr('y1', timelineY)
        .attr('x2', designWidth - 50)
        .attr('y2', timelineY)
        .attr('stroke', theme.colors.borderMedium)
        .attr('stroke-width', 2);

      svg
        .append('text')
        .attr('x', 50)
        .attr('y', timelineY + 25)
        .attr('font-size', 9)
        .attr('fill', theme.colors.inkLight)
        .text('Time →');

      // CPU 1 (Writer)
      const cpu1Y = timelineY - 50;
      svg
        .append('rect')
        .attr('x', 80)
        .attr('y', cpu1Y - 15)
        .attr('width', 80)
        .attr('height', 30)
        .attr('rx', 4)
        .attr('fill', 'rgba(139, 92, 246, 0.15)')
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 1.5);

      svg
        .append('text')
        .attr('x', 120)
        .attr('y', cpu1Y + 5)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 600)
        .attr('fill', '#8b5cf6')
        .text('CPU 1');

      // Write operation
      svg
        .append('text')
        .attr('x', 200)
        .attr('y', cpu1Y + 5)
        .attr('font-size', 10)
        .attr('fill', theme.colors.inkDark)
        .text('data = 42');

      // Memory barrier
      const barrierX = 350;
      svg
        .append('line')
        .attr('x1', barrierX)
        .attr('y1', timelineY - 70)
        .attr('x2', barrierX)
        .attr('y2', timelineY + 70)
        .attr('stroke', '#f59e0b')
        .attr('stroke-width', 3)
        .attr('stroke-dasharray', '6,3');

      svg
        .append('rect')
        .attr('x', barrierX - 45)
        .attr('y', timelineY - 12)
        .attr('width', 90)
        .attr('height', 24)
        .attr('rx', 4)
        .attr('fill', '#f59e0b');

      svg
        .append('text')
        .attr('x', barrierX)
        .attr('y', timelineY + 5)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('font-weight', 600)
        .attr('fill', 'white')
        .text('⚡ BARRIER');

      // Epoch signal
      svg
        .append('text')
        .attr('x', 440)
        .attr('y', cpu1Y + 5)
        .attr('font-size', 10)
        .attr('fill', '#16a34a')
        .attr('font-weight', 600)
        .text('Atomics.store(epoch++)');

      // CPU 2 (Reader)
      const cpu2Y = timelineY + 50;
      svg
        .append('rect')
        .attr('x', 80)
        .attr('y', cpu2Y - 15)
        .attr('width', 80)
        .attr('height', 30)
        .attr('rx', 4)
        .attr('fill', 'rgba(14, 165, 233, 0.15)')
        .attr('stroke', '#0ea5e9')
        .attr('stroke-width', 1.5);

      svg
        .append('text')
        .attr('x', 120)
        .attr('y', cpu2Y + 5)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 600)
        .attr('fill', '#0ea5e9')
        .text('CPU 2');

      // Wait + Read
      svg
        .append('text')
        .attr('x', 200)
        .attr('y', cpu2Y + 5)
        .attr('font-size', 10)
        .attr('fill', '#9ca3af')
        .text('... waiting ...');

      svg
        .append('text')
        .attr('x', 440)
        .attr('y', cpu2Y + 5)
        .attr('font-size', 10)
        .attr('fill', '#16a34a')
        .attr('font-weight', 600)
        .text('Atomics.wait() → wakes!');

      svg
        .append('text')
        .attr('x', 580)
        .attr('y', cpu2Y + 5)
        .attr('font-size', 10)
        .attr('fill', theme.colors.inkDark)
        .text('read data = 42 ✓');

      // Explanation
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', 200)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('fill', theme.colors.inkMedium)
        .text('The barrier guarantees CPU 2 sees all writes from CPU 1 before the epoch signal.');
    },
    [theme]
  );

  return (
    <D3Container
      render={renderDiagram}
      dependencies={[renderDiagram]}
      viewBox="0 0 700 220"
      height={220}
    />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// LIVE TELEMETRY COMPONENT (Layman-Friendly)
// ────────────────────────────────────────────────────────────────────────────
function AtomicsTelemetry() {
  const [metrics, setMetrics] = useState({
    heartbeat: 0,
    messagesDelivered: 0,
    balanceUpdates: 0,
    decisionsResolved: 0,
  });

  useEffect(() => {
    let heartbeat = 0;
    let messages = 0;
    let balances = 0;
    let decisions = 0;

    const timer = setInterval(() => {
      heartbeat += 1;
      messages += Math.floor(Math.random() * 200) + 50;
      balances += Math.floor(Math.random() * 100) + 20;
      decisions += Math.floor(Math.random() * 50) + 10;

      setMetrics({
        heartbeat,
        messagesDelivered: messages,
        balanceUpdates: balances,
        decisionsResolved: decisions,
      });
    }, 500);
    return () => clearInterval(timer);
  }, []);

  return (
    <Style.PerformanceMeter>
      <Style.IllustrationTitle style={{ marginBottom: '16px', display: 'block' }}>
        ⚡ Live System Activity
      </Style.IllustrationTitle>

      <Style.MeterRow>
        <span>System Heartbeat</span>
        <span style={{ fontWeight: 700, color: '#8b5cf6' }}>{metrics.heartbeat} pulses</span>
      </Style.MeterRow>
      <Style.MeterBar $percent={metrics.heartbeat % 100} $color="#8b5cf6" />

      <Style.MeterRow style={{ marginTop: '12px' }}>
        <span>Messages Delivered</span>
        <span>{metrics.messagesDelivered.toLocaleString()}</span>
      </Style.MeterRow>
      <Style.MeterBar
        $percent={Math.min((metrics.messagesDelivered / 10000) * 100, 100)}
        $color="#0ea5e9"
      />

      <Style.MeterRow style={{ marginTop: '12px' }}>
        <span>Balance Updates</span>
        <span>{metrics.balanceUpdates.toLocaleString()}</span>
      </Style.MeterRow>
      <Style.MeterBar
        $percent={Math.min((metrics.balanceUpdates / 5000) * 100, 100)}
        $color="#16a34a"
      />

      <Style.MeterRow style={{ marginTop: '12px' }}>
        <span>Decisions Resolved</span>
        <span>{metrics.decisionsResolved.toLocaleString()}</span>
      </Style.MeterRow>
      <Style.MeterBar
        $percent={Math.min((metrics.decisionsResolved / 2500) * 100, 100)}
        $color="#f59e0b"
      />

      <p style={{ fontSize: '10px', marginTop: '12px', color: '#888', textAlign: 'center' }}>
        All happening instantly. No waiting. No delays. That's the power of atomic operations.
      </p>
    </Style.PerformanceMeter>
  );
}

// ────────────────────────────────────────────────────────────────────────────
// MAIN COMPONENT
// ────────────────────────────────────────────────────────────────────────────
export default function AtomicsPage() {
  return (
    <Style.BlogContainer>
      <Style.SectionTitle>Chapter 05 // Atomics & Shared Memory</Style.SectionTitle>
      <Style.PageTitle>Coordination Without Contention</Style.PageTitle>

      <Style.LeadParagraph>
        Imagine a busy kitchen where multiple chefs need to update the same recipe card at once.
        Traditional systems use a "one chef at a time" rule, creating lines.
        <strong>Atomic operations</strong> are like magic: every chef can work simultaneously, and
        nothing ever gets lost or mixed up.
      </Style.LeadParagraph>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 1: THE SYNCHRONIZATION PROBLEM */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 1: The Shared Counter Problem</h3>
          <p>
            Imagine two bank tellers both trying to update the same account balance at exactly the
            same time. Without coordination, disaster strikes: both read $100, both add $50, and
            both write $150—losing $50 in the process.
          </p>
          <p>
            This is called a <strong>race condition</strong>—when two things happen at once and step
            on each other's toes. The classic solution? Take turns: lock the door, do your work,
            unlock. Safe, but slow.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.HistoryCard>
        <h4>📚 A Brief History</h4>
        <p>
          The "take turns" approach was invented in 1965 by computer scientist Edsger Dijkstra. It
          works, but over the years, programmers discovered it causes traffic jams in software—
          everyone waiting in line while one person finishes.
        </p>
      </Style.HistoryCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 2: LOCKS VS LOCK-FREE */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 2: The Traffic Jam Problem</h3>
          <p>
            Think of a traditional lock like a single-lane bridge. When Person A is crossing,
            Persons B, C, and D must wait. This creates two problems:
          </p>
          <ul>
            <li>
              <strong>Waiting time adds up:</strong> Each person waiting burns energy just standing
              there. In computers, this wastes precious processing power.
            </li>
            <li>
              <strong>Important people get stuck:</strong> An ambulance (high-priority task) can get
              stuck behind a regular car (low-priority task).
            </li>
          </ul>
          <p>
            <strong>Atomic operations</strong> are like having a magical bridge where everyone can
            cross at once without colliding. No waiting. No traffic jams.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Animated: Locks vs Lock-Free</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <LockContentionDiagram />
        <Style.IllustrationCaption>
          With mutexes, threads block each other. With atomics, all threads make progress
          simultaneously.
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.ComparisonGrid>
        <Style.ComparisonCard $type="bad">
          <h4>🔴 Traditional (Lock-Based)</h4>
          <p>
            <strong>Speed:</strong> Slow (people wait in line)
          </p>
          <p>
            <strong>Efficiency:</strong> Lots of waiting around
          </p>
          <p>
            <strong>Risk:</strong> Traffic jams happen
          </p>
        </Style.ComparisonCard>

        <Style.ComparisonCard $type="good">
          <h4>🟢 INOS (Atomic Operations)</h4>
          <p>
            <strong>Speed:</strong> Instant (no waiting)
          </p>
          <p>
            <strong>Efficiency:</strong> Everyone works at once
          </p>
          <p>
            <strong>Risk:</strong> None (magic bridge!)
          </p>
        </Style.ComparisonCard>
      </Style.ComparisonGrid>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 3: COMPARE-AND-SWAP */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 3: The Magic of Compare-And-Swap</h3>
          <p>
            The workhorse of lock-free programming is <strong>Compare-And-Swap (CAS)</strong>. It's
            a single hardware instruction that atomically:
          </p>
          <ol>
            <li>Reads the current value at a memory address</li>
            <li>Compares it to an expected value</li>
            <li>If they match, writes a new value</li>
            <li>Returns whether the swap succeeded</li>
          </ol>
          <p>
            All of this happens in <strong>one indivisible operation</strong>. No other thread can
            see an intermediate state. It either succeeds completely or fails completely.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.DefinitionBox>
        <h4>Atomics.compareExchange()</h4>
        <p>
          JavaScript's{' '}
          <code>Atomics.compareExchange(typedArray, index, expected, replacement)</code> maps
          directly to the CPU's <code>LOCK CMPXCHG</code> instruction. It returns the original
          value, allowing you to detect success or retry.
        </p>
      </Style.DefinitionBox>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Animated: CAS Operation</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <AtomicCASDiagram />
        <Style.IllustrationCaption>
          CAS checks if memory matches expected, then atomically swaps to desired value
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.CodeBlock>
        {`// Lock-free counter increment using CAS
function atomicIncrement(buffer: Int32Array, index: number): number {
  while (true) {
    const current = Atomics.load(buffer, index);
    const desired = current + 1;
    
    // Try to swap: if current value is still what we read, update it
    const result = Atomics.compareExchange(buffer, index, current, desired);
    
    if (result === current) {
      return desired;  // Success! We won the race
    }
    // Another thread changed it first—retry with new value
  }
}`}
      </Style.CodeBlock>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 4: MEMORY ORDERING */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 4: Memory Ordering & Barriers</h3>
          <p>
            Modern CPUs reorder instructions for performance. Thread A might write{' '}
            <code>data = 42</code> then <code>ready = true</code>, but the CPU might flip the order.
            Thread B could see <code>ready = true</code> before <code>data = 42</code> is visible—a
            nightmare scenario.
          </p>
          <p>
            <strong>Memory barriers</strong> (also called fences) force ordering. They guarantee
            that all writes before the barrier are visible to all threads before any writes after
            the barrier.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Memory Barrier Flow</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <MemoryOrderingDiagram />
        <Style.IllustrationCaption>
          Barriers ensure writes are visible across CPUs before signaling
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.ContentCard>
        <h4>The Four Memory Orderings</h4>
        <Style.OrderingGrid>
          <Style.OrderingCard $color="#9ca3af">
            <h5>Relaxed</h5>
            <p>No ordering. Fast but dangerous. Use only for counters.</p>
          </Style.OrderingCard>

          <Style.OrderingCard $color="#0ea5e9">
            <h5>Acquire</h5>
            <p>Reads cannot move before. Used when "acquiring" a lock.</p>
          </Style.OrderingCard>

          <Style.OrderingCard $color="#8b5cf6">
            <h5>Release</h5>
            <p>Writes cannot move after. Used when "releasing" a lock.</p>
          </Style.OrderingCard>

          <Style.OrderingCard $color="#16a34a">
            <h5>SeqCst</h5>
            <p>Full ordering. Safest but slowest. JS Atomics default.</p>
          </Style.OrderingCard>
        </Style.OrderingGrid>
      </Style.ContentCard>

      <Style.WarningCard>
        <h4>💡 INOS Uses Sequential Consistency</h4>
        <p>
          JavaScript <code>Atomics</code> always use <strong>SeqCst</strong> (sequentially
          consistent) ordering—the strongest guarantee. This is intentional: browser vendors
          prioritized correctness over micro-optimization. INOS inherits this safety.
        </p>
      </Style.WarningCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 5: ATOMICS IN INOS */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 5: Atomics in INOS</h3>
          <p>
            INOS uses atomics as the foundation of its concurrency model. Every synchronization
            point—from epoch signaling to credit transfers—relies on lock-free operations in
            SharedArrayBuffer.
          </p>
          <ul>
            <li>
              <strong>Epoch Signaling:</strong> A single <code>Atomics.store()</code> wakes all
              waiting workers instantly.
            </li>
            <li>
              <strong>Credit Ledger:</strong> Balance updates use <code>Atomics.add()</code> for
              zero-contention accounting.
            </li>
            <li>
              <strong>Module Registry:</strong> CAS operations ensure only one thread initializes
              each module.
            </li>
            <li>
              <strong>Job Queues:</strong> Lock-free ring buffers using head/tail atomic pointers.
            </li>
          </ul>
        </Style.ContentCard>
      </ScrollReveal>

      <AtomicsTelemetry />

      <Style.ContentCard>
        <h3>Lesson 6: Performance Impact</h3>
        <p>
          The difference is not incremental—it's transformational. By eliminating locks, INOS
          achieves latencies that would be impossible with traditional synchronization.
        </p>

        <Style.MetricRow>
          <Style.MetricLabel>Mutex lock/unlock (typical)</Style.MetricLabel>
          <Style.MetricValue>~1,000 ns</Style.MetricValue>
        </Style.MetricRow>
        <Style.MetricRow>
          <Style.MetricLabel>Atomic CAS operation</Style.MetricLabel>
          <Style.MetricValue $highlight>~10 ns</Style.MetricValue>
        </Style.MetricRow>
        <Style.MetricRow>
          <Style.MetricLabel>Epoch signal latency (INOS)</Style.MetricLabel>
          <Style.MetricValue $highlight>&lt;10 µs</Style.MetricValue>
        </Style.MetricRow>
        <Style.MetricRow>
          <Style.MetricLabel>Credit transfer latency</Style.MetricLabel>
          <Style.MetricValue $highlight>&lt;1 µs</Style.MetricValue>
        </Style.MetricRow>
      </Style.ContentCard>

      <ChapterNav
        prev={{ title: 'Epoch Signaling', to: '/deep-dives/signaling' }}
        next={{ title: 'Distributed P2P Mesh', to: '/deep-dives/mesh' }}
      />
    </Style.BlogContainer>
  );
}
