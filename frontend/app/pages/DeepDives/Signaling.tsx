/**
 * INOS Technical Codex — Deep Dive: Epoch Signaling
 *
 * A comprehensive exploration of reactivity paradigms: polling, callbacks,
 * atomics, and epoch-based signaling. Explains why INOS achieves <10µs latency.
 *
 * Educational approach: Compare paradigms visually with animated diagrams.
 */

import { useState, useCallback } from 'react';
import styled, { useTheme } from 'styled-components';
import * as d3 from 'd3';
import { Style as ManuscriptStyle } from '../../styles/manuscript';
import ChapterNav from '../../ui/ChapterNav';
import ScrollReveal from '../../ui/ScrollReveal';
import D3Container, { D3RenderFn } from '../../ui/D3Container';

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

  ComparisonGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[5]} 0;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      grid-template-columns: 1fr;
    }
  `,

  ParadigmCard: styled.div<{ $type: 'bad' | 'medium' | 'good' }>`
    background: ${p =>
      p.$type === 'bad'
        ? 'rgba(220, 38, 38, 0.06)'
        : p.$type === 'medium'
          ? 'rgba(234, 179, 8, 0.06)'
          : 'rgba(22, 163, 74, 0.06)'};
    backdrop-filter: blur(12px);
    border: 1px solid
      ${p =>
        p.$type === 'bad'
          ? 'rgba(220, 38, 38, 0.2)'
          : p.$type === 'medium'
            ? 'rgba(234, 179, 8, 0.2)'
            : 'rgba(22, 163, 74, 0.2)'};
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};

    h4 {
      margin: 0 0 ${p => p.theme.spacing[3]} 0;
      color: ${p => (p.$type === 'bad' ? '#dc2626' : p.$type === 'medium' ? '#ca8a04' : '#16a34a')};
      font-size: ${p => p.theme.fontSizes.base};
    }

    p {
      margin: 0 0 ${p => p.theme.spacing[2]} 0;
      line-height: 1.5;
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

  AnimationControls: styled.div`
    display: flex;
    gap: ${p => p.theme.spacing[2]};
    padding: ${p => p.theme.spacing[3]} ${p => p.theme.spacing[4]};
    border-top: 1px solid ${p => p.theme.colors.borderSubtle};
    background: rgba(0, 0, 0, 0.02);
  `,

  ControlButton: styled.button<{ $active?: boolean }>`
    padding: ${p => p.theme.spacing[2]} ${p => p.theme.spacing[3]};
    background: ${p => (p.$active ? p.theme.colors.accent : 'rgba(255, 255, 255, 0.9)')};
    border: 1px solid ${p => (p.$active ? p.theme.colors.accent : p.theme.colors.borderSubtle)};
    border-radius: 4px;
    font-size: 11px;
    font-family: ${p => p.theme.fonts.typewriter};
    color: ${p => (p.$active ? 'white' : p.theme.colors.inkMedium)};
    cursor: pointer;
    text-transform: uppercase;
    letter-spacing: 0.05em;

    &:hover {
      border-color: ${p => p.theme.colors.accent};
    }
  `,
};

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: PARADIGM COMPARISON (Truly Animated)
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: PARADIGM COMPARISON (D3Container + Transitions)
// ────────────────────────────────────────────────────────────────────────────
function ParadigmComparisonDiagram() {
  const theme = useTheme();
  const [activeParadigm, setActiveParadigm] = useState<'polling' | 'callback' | 'epoch'>('polling');
  const [isPlaying, setIsPlaying] = useState(true);

  const renderDiagram: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').interrupt();
      svg.selectAll('*').remove();

      const scale = Math.min(1, width / 700);
      const timelineY = 120;
      const cpuY = 45;
      const colors = { polling: '#dc2626', callback: '#f59e0b', epoch: '#16a34a' };

      // 1. Static Layout
      svg
        .append('line')
        .attr('x1', 50 * scale)
        .attr('y1', timelineY)
        .attr('x2', width - 50 * scale)
        .attr('y2', timelineY)
        .attr('stroke', theme.colors.borderSubtle)
        .attr('stroke-width', 2);
      svg
        .append('text')
        .attr('x', 50 * scale)
        .attr('y', timelineY + 20)
        .attr('font-size', 9)
        .attr('fill', theme.colors.inkLight)
        .text('0ms');
      svg
        .append('text')
        .attr('x', width - 50 * scale)
        .attr('y', timelineY + 20)
        .attr('text-anchor', 'end')
        .attr('font-size', 9)
        .attr('fill', theme.colors.inkLight)
        .text('100ms');

      const titles = {
        polling: 'Polling: Constant CPU checks (wastes cycles)',
        callback: 'Callbacks: Queue → Process → Deliver (delayed)',
        epoch: 'Epochs: Sleep → Instant wake (efficient)',
      };
      svg
        .append('text')
        .attr('x', width / 2)
        .attr('y', 20)
        .attr('text-anchor', 'middle')
        .attr('font-size', 12 * scale + 2 * (1 - scale))
        .attr('font-weight', 600)
        .attr('fill', colors[activeParadigm])
        .text(titles[activeParadigm]);

      svg
        .append('rect')
        .attr('x', 50 * scale)
        .attr('y', cpuY - 12)
        .attr('width', 100 * scale)
        .attr('height', 20)
        .attr('rx', 3)
        .attr('fill', 'rgba(0,0,0,0.05)')
        .attr('stroke', theme.colors.borderSubtle);
      svg
        .append('text')
        .attr('x', 100 * scale)
        .attr('y', cpuY - 18)
        .attr('text-anchor', 'middle')
        .attr('font-size', 8)
        .attr('fill', theme.colors.inkLight)
        .text('CPU');

      const cpuBar = svg
        .append('rect')
        .attr('x', 52 * scale)
        .attr('y', cpuY - 10)
        .attr('width', 0)
        .attr('height', 16)
        .attr('rx', 2)
        .attr('fill', colors[activeParadigm]);
      const statusText = svg
        .append('text')
        .attr('x', 165 * scale)
        .attr('y', cpuY + 3)
        .attr('font-size', 10 * scale + 2 * (1 - scale))
        .attr('font-weight', 500)
        .attr('fill', colors[activeParadigm])
        .attr('font-family', "'JetBrains Mono', monospace");

      const dataChangeX = 50 * scale + (width - 100 * scale) * 0.45;
      const dataMarker = svg.append('g').attr('opacity', 0);
      dataMarker
        .append('line')
        .attr('x1', dataChangeX)
        .attr('y1', timelineY - 35)
        .attr('x2', dataChangeX)
        .attr('y2', timelineY + 5)
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 2)
        .attr('stroke-dasharray', '3,2');
      dataMarker
        .append('text')
        .attr('x', dataChangeX)
        .attr('y', timelineY - 40)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('fill', '#8b5cf6')
        .text('⚡ DATA CHANGED');

      const progressDot = svg
        .append('circle')
        .attr('cx', 50 * scale)
        .attr('cy', timelineY)
        .attr('r', 5)
        .attr('fill', colors[activeParadigm]);

      // 2. Animation Sequences
      if (!isPlaying) return;

      function loop() {
        progressDot
          .attr('cx', 50 * scale)
          .transition()
          .duration(5000)
          .ease(d3.easeLinear)
          .attr('cx', width - 50 * scale)
          .on('end', loop);

        dataMarker.style('opacity', 0).transition().delay(2250).duration(0).style('opacity', 1);

        if (activeParadigm === 'polling') {
          cpuBar
            .transition()
            .duration(5000)
            .ease(d3.easeLinear)
            .attrTween('width', () => t => String((70 + Math.sin(t * 30) * 20) * scale));
          statusText
            .text('Checking...')
            .transition()
            .delay(2250)
            .duration(0)
            .text('HIT! (5ms delay)');

          for (let i = 0; i < 11; i++) {
            const pollX = 50 * scale + (width - 100 * scale) * (i / 10);
            svg
              .append('circle')
              .attr('cx', pollX)
              .attr('cy', timelineY - 25)
              .attr('r', 8 * scale)
              .attr('fill', 'rgba(220, 38, 38, 0.3)')
              .attr('stroke', colors.polling)
              .attr('stroke-width', 2)
              .style('opacity', 0)
              .transition()
              .delay(i * 500)
              .duration(200)
              .style('opacity', 1)
              .attr('fill', i >= 5 ? colors.polling : 'rgba(220, 38, 38, 0.3)');
          }
        } else if (activeParadigm === 'callback') {
          cpuBar
            .attr('width', 5 * scale)
            .transition()
            .delay(2250)
            .duration(200)
            .attr('width', 40 * scale)
            .transition()
            .delay(400)
            .duration(200)
            .attr('width', 60 * scale)
            .transition()
            .delay(500)
            .duration(200)
            .attr('width', 20 * scale);
          statusText
            .text('Idle...')
            .transition()
            .delay(2250)
            .duration(0)
            .text('Queueing...')
            .transition()
            .delay(600)
            .duration(0)
            .text('Processing...')
            .transition()
            .delay(900)
            .duration(0)
            .text('Callback fired! (8ms)');

          const packet = svg
            .append('rect')
            .attr('x', dataChangeX)
            .attr('y', timelineY - 35)
            .attr('width', 25 * scale)
            .attr('height', 20)
            .attr('rx', 3)
            .attr('fill', colors.callback)
            .style('opacity', 0);
          packet
            .transition()
            .delay(2250)
            .duration(200)
            .style('opacity', 1)
            .transition()
            .duration(600)
            .attr('x', 320 * scale)
            .transition()
            .duration(500)
            .attr('x', 455 * scale)
            .style('opacity', 0);

          svg
            .append('circle')
            .attr('cx', 550 * scale)
            .attr('cy', timelineY - 30)
            .attr('r', 12 * scale)
            .attr('fill', 'rgba(234, 179, 8, 0.3)')
            .attr('stroke', colors.callback)
            .attr('stroke-width', 2)
            .style('opacity', 0)
            .transition()
            .delay(3750)
            .duration(300)
            .style('opacity', 1);
        } else if (activeParadigm === 'epoch') {
          cpuBar
            .attr('width', 2 * scale)
            .transition()
            .delay(2250)
            .duration(100)
            .attr('width', 80 * scale)
            .transition()
            .duration(100)
            .attr('width', 2 * scale);
          statusText
            .text('Sleeping (0% CPU)...')
            .transition()
            .delay(2250)
            .duration(0)
            .text('INSTANT WAKE! (<10µs)');

          const thread = svg
            .append('text')
            .attr('x', width / 2)
            .attr('y', timelineY - 25)
            .attr('text-anchor', 'middle')
            .attr('font-size', 16 * scale)
            .text('💤');
          thread.transition().delay(2250).duration(0).text('⚡');

          svg
            .append('circle')
            .attr('cx', width / 2)
            .attr('cy', timelineY - 30)
            .attr('r', 0)
            .attr('fill', 'none')
            .attr('stroke', colors.epoch)
            .attr('stroke-width', 3)
            .style('opacity', 0)
            .transition()
            .delay(2250)
            .duration(500)
            .attr('r', 60 * scale)
            .style('opacity', 0);
        }
      }

      loop();
    },
    [theme, activeParadigm, isPlaying]
  );

  return (
    <div>
      <D3Container
        render={renderDiagram}
        dependencies={[renderDiagram]}
        viewBox="0 0 700 200"
        height={200}
      />
      <Style.AnimationControls>
        <Style.ControlButton $active={isPlaying} onClick={() => setIsPlaying(!isPlaying)}>
          {isPlaying ? '⏸ Pause' : '▶ Play'}
        </Style.ControlButton>
        <Style.ControlButton
          $active={activeParadigm === 'polling'}
          onClick={() => setActiveParadigm('polling')}
        >
          Polling
        </Style.ControlButton>
        <Style.ControlButton
          $active={activeParadigm === 'callback'}
          onClick={() => setActiveParadigm('callback')}
        >
          Callbacks
        </Style.ControlButton>
        <Style.ControlButton
          $active={activeParadigm === 'epoch'}
          onClick={() => setActiveParadigm('epoch')}
        >
          Epochs
        </Style.ControlButton>
      </Style.AnimationControls>
    </div>
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: CPU USAGE COMPARISON
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: CPU USAGE COMPARISON (D3Container)
// ────────────────────────────────────────────────────────────────────────────
function CpuUsageDiagram() {
  const theme = useTheme();

  const renderDiagram: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').remove();
      const scale = Math.min(1, width / 660);
      const barHeight = 35;
      const startX = 150 * scale + 50 * (1 - scale);
      const barWidth = 400 * scale;

      const data = [
        { label: 'Polling (10ms)', value: 85, color: '#dc2626', cpu: '85% CPU' },
        { label: 'Callbacks/Events', value: 25, color: '#f59e0b', cpu: '25% CPU' },
        { label: 'Epoch Signaling', value: 2, color: '#16a34a', cpu: '2% CPU' },
      ];

      svg
        .append('text')
        .attr('x', width / 2)
        .attr('y', 25)
        .attr('text-anchor', 'middle')
        .attr('font-size', 12 * scale + 2 * (1 - scale))
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkDark)
        .text('CPU Usage: Waiting for Data Change');

      data.forEach((d, i) => {
        const y = 50 + i * (barHeight + 15);
        svg
          .append('text')
          .attr('x', startX - 10)
          .attr('y', y + barHeight / 2 + 4)
          .attr('text-anchor', 'end')
          .attr('font-size', 11 * scale + 1 * (1 - scale))
          .attr('fill', theme.colors.inkDark)
          .text(d.label);
        svg
          .append('rect')
          .attr('x', startX)
          .attr('y', y)
          .attr('width', barWidth)
          .attr('height', barHeight)
          .attr('rx', 4)
          .attr('fill', 'rgba(0, 0, 0, 0.05)');
        svg
          .append('rect')
          .attr('x', startX)
          .attr('y', y)
          .attr('width', (barWidth * d.value) / 100)
          .attr('height', barHeight)
          .attr('rx', 4)
          .attr('fill', d.color);
        svg
          .append('text')
          .attr('x', startX + barWidth + 10)
          .attr('y', y + barHeight / 2 + 4)
          .attr('font-size', 11 * scale)
          .attr('font-weight', 600)
          .attr('fill', d.color)
          .attr('font-family', "'JetBrains Mono', monospace")
          .text(d.cpu);
      });

      svg
        .append('text')
        .attr('x', width / 2)
        .attr('y', 210)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('fill', theme.colors.inkLight)
        .text('Lower is better. Epoch signaling sleeps until data changes.');
    },
    [theme]
  );

  return (
    <D3Container
      render={renderDiagram}
      dependencies={[renderDiagram]}
      viewBox="0 0 660 235"
      height={235}
    />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: EPOCH FLOW
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: EPOCH FLOW (D3Container)
// ────────────────────────────────────────────────────────────────────────────
function EpochFlowDiagram() {
  const theme = useTheme();

  const renderDiagram: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').remove();
      const scale = Math.min(1, width / 660);
      const centerX = width / 2;
      const spacing = 180 * scale;

      svg
        .append('text')
        .attr('x', width / 2)
        .attr('y', 25)
        .attr('text-anchor', 'middle')
        .attr('font-size', 12 * scale + 2 * (1 - scale))
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkDark)
        .text('Epoch Signaling: Mutate → Signal → React');

      const stages = [
        {
          x: centerX - spacing,
          label: 'MUTATE',
          sublabel: 'Write data to SAB',
          color: '#8b5cf6',
          icon: '✎',
        },
        {
          x: centerX,
          label: 'SIGNAL',
          sublabel: 'Atomics.store(epoch++)',
          color: '#16a34a',
          icon: '⚡',
        },
        {
          x: centerX + spacing,
          label: 'REACT',
          sublabel: 'Waiters wake instantly',
          color: '#0ea5e9',
          icon: '↻',
        },
      ];

      stages.forEach((stage, i) => {
        svg
          .append('circle')
          .attr('cx', stage.x)
          .attr('cy', 80)
          .attr('r', 35 * scale)
          .attr('fill', `${stage.color}15`)
          .attr('stroke', stage.color)
          .attr('stroke-width', 2);
        svg
          .append('text')
          .attr('x', stage.x)
          .attr('y', 88 * scale + (1 - scale) * 85)
          .attr('text-anchor', 'middle')
          .attr('font-size', 24 * scale)
          .attr('fill', stage.color)
          .text(stage.icon);
        svg
          .append('text')
          .attr('x', stage.x)
          .attr('y', 135)
          .attr('text-anchor', 'middle')
          .attr('font-size', 11 * scale + 1 * (1 - scale))
          .attr('font-weight', 600)
          .attr('fill', stage.color)
          .text(stage.label);
        svg
          .append('text')
          .attr('x', stage.x)
          .attr('y', 150)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9 * scale)
          .attr('fill', theme.colors.inkLight)
          .attr('font-family', "'JetBrains Mono', monospace")
          .text(stage.sublabel);

        if (i < stages.length - 1) {
          const nextX = stages[i + 1].x;
          svg
            .append('line')
            .attr('x1', stage.x + 40 * scale)
            .attr('y1', 80)
            .attr('x2', nextX - 40 * scale)
            .attr('y2', 80)
            .attr('stroke', theme.colors.inkLight)
            .attr('stroke-width', 2)
            .attr('stroke-dasharray', '4,4');
          svg
            .append('polygon')
            .attr(
              'points',
              `${nextX - 40 * scale},80 ${nextX - 48 * scale},75 ${nextX - 48 * scale},85`
            )
            .attr('fill', theme.colors.inkLight);
        }
      });

      svg
        .append('rect')
        .attr('x', centerX - 200 * scale)
        .attr('y', 175)
        .attr('width', 400 * scale)
        .attr('height', 30)
        .attr('rx', 4)
        .attr('fill', 'rgba(22, 163, 74, 0.1)')
        .attr('stroke', 'rgba(22, 163, 74, 0.3)');
      svg
        .append('text')
        .attr('x', width / 2)
        .attr('y', 195)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10 * scale)
        .attr('font-weight', 500)
        .attr('fill', '#16a34a')
        .text('Total latency: <10µs (100x faster than callbacks)');
    },
    [theme]
  );

  return (
    <D3Container
      render={renderDiagram}
      dependencies={[renderDiagram]}
      viewBox="0 0 660 220"
      height={220}
    />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: ANIMATED LOOP COMPARISON
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: ANIMATED LOOP COMPARISON (D3Container)
// ────────────────────────────────────────────────────────────────────────────
function AnimatedLoopDiagram() {
  const theme = useTheme();
  const [isRunning, setIsRunning] = useState(true);

  const renderDiagram: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').interrupt();
      svg.selectAll('*').remove();
      const scale = Math.min(1, width / 660);

      svg
        .append('text')
        .attr('x', width / 2)
        .attr('y', 20)
        .attr('text-anchor', 'middle')
        .attr('font-size', 11 * scale + 1 * (1 - scale))
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkDark)
        .text('Live Animation: Polling vs Sleeping');

      // Polling side
      const pollingX = 165 * scale;
      const pollingY = 100;
      svg
        .append('text')
        .attr('x', pollingX)
        .attr('y', 45)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 600)
        .attr('fill', '#dc2626')
        .text('Polling Loop');
      svg
        .append('rect')
        .attr('x', pollingX - 60 * scale)
        .attr('y', 55)
        .attr('width', 120 * scale)
        .attr('height', 20)
        .attr('rx', 3)
        .attr('fill', 'rgba(220, 38, 38, 0.1)')
        .attr('stroke', 'rgba(220, 38, 38, 0.3)');
      const pollingCpuBar = svg
        .append('rect')
        .attr('x', pollingX - 58 * scale)
        .attr('y', 57)
        .attr('width', 0)
        .attr('height', 16)
        .attr('rx', 2)
        .attr('fill', '#dc2626');
      svg
        .append('text')
        .attr('x', pollingX)
        .attr('y', 69)
        .attr('text-anchor', 'middle')
        .attr('font-size', 8)
        .attr('fill', '#dc2626')
        .attr('font-family', "'JetBrains Mono', monospace")
        .text('CPU: always busy');
      const pollingCircle = svg
        .append('circle')
        .attr('cx', pollingX)
        .attr('cy', pollingY)
        .attr('r', 25 * scale)
        .attr('fill', 'rgba(220, 38, 38, 0.2)')
        .attr('stroke', '#dc2626')
        .attr('stroke-width', 2);
      const pollingText = svg
        .append('text')
        .attr('x', pollingX)
        .attr('y', pollingY + 5)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 600)
        .attr('fill', '#dc2626')
        .attr('font-family', "'JetBrains Mono', monospace")
        .text('CHECK');

      // Epoch side
      const epochX = width - 165 * scale;
      const epochY = 100;
      svg
        .append('text')
        .attr('x', epochX)
        .attr('y', 45)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 600)
        .attr('fill', '#16a34a')
        .text('Epoch Wait');
      svg
        .append('rect')
        .attr('x', epochX - 60 * scale)
        .attr('y', 55)
        .attr('width', 120 * scale)
        .attr('height', 20)
        .attr('rx', 3)
        .attr('fill', 'rgba(22, 163, 74, 0.1)')
        .attr('stroke', 'rgba(22, 163, 74, 0.3)');
      const epochCpuBar = svg
        .append('rect')
        .attr('x', epochX - 58 * scale)
        .attr('y', 57)
        .attr('width', 0)
        .attr('height', 16)
        .attr('rx', 2)
        .attr('fill', '#16a34a');
      svg
        .append('text')
        .attr('x', epochX)
        .attr('y', 69)
        .attr('text-anchor', 'middle')
        .attr('font-size', 8)
        .attr('fill', '#16a34a')
        .attr('font-family', "'JetBrains Mono', monospace")
        .text('CPU: sleeping');
      const epochCircle = svg
        .append('circle')
        .attr('cx', epochX)
        .attr('cy', epochY)
        .attr('r', 25 * scale)
        .attr('fill', 'rgba(22, 163, 74, 0.2)')
        .attr('stroke', '#16a34a')
        .attr('stroke-width', 2);
      const epochText = svg
        .append('text')
        .attr('x', epochX)
        .attr('y', epochY + 5)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 600)
        .attr('fill', '#16a34a')
        .attr('font-family', "'JetBrains Mono', monospace")
        .text('SLEEP');

      if (!isRunning) return;

      function animate() {
        // Polling loop
        pollingCpuBar
          .attr('width', 0)
          .transition()
          .duration(200)
          .attr('width', 116 * scale)
          .transition()
          .duration(200)
          .attr('width', 60 * scale)
          .transition()
          .duration(200)
          .attr('width', 100 * scale)
          .on('end', animate);
        pollingCircle
          .transition()
          .duration(200)
          .attr('r', 30 * scale)
          .transition()
          .duration(200)
          .attr('r', 25 * scale);

        const pollingStates = ['CHECK', 'WAIT', 'CHECK', 'LOOP'];
        let stateIdx = 0;
        const updateText = () => {
          pollingText.text(pollingStates[stateIdx]);
          stateIdx = (stateIdx + 1) % pollingStates.length;
          pollingText.transition().duration(500).on('end', updateText);
        };
        updateText();

        // Epoch loop
        function epochLoop() {
          epochCpuBar
            .attr('width', 3 * scale)
            .transition()
            .delay(2000)
            .duration(100)
            .attr('width', 116 * scale)
            .transition()
            .duration(100)
            .attr('width', 3 * scale)
            .on('end', epochLoop);
          epochCircle
            .transition()
            .delay(2000)
            .duration(100)
            .attr('fill', 'rgba(22, 163, 74, 0.6)')
            .transition()
            .duration(100)
            .attr('fill', 'rgba(22, 163, 74, 0.2)');
          epochText
            .text('SLEEP')
            .transition()
            .delay(2000)
            .duration(0)
            .text('WAKE!')
            .transition()
            .delay(200)
            .duration(0)
            .text('SLEEP');
        }
        epochLoop();
      }

      animate();
    },
    [theme, isRunning]
  );

  return (
    <div>
      <D3Container
        render={renderDiagram}
        dependencies={[isRunning]}
        viewBox="0 0 660 160"
        height={160}
      />
      <Style.AnimationControls>
        <Style.ControlButton $active={isRunning} onClick={() => setIsRunning(!isRunning)}>
          {isRunning ? '⏸ Pause' : '▶ Play'}
        </Style.ControlButton>
      </Style.AnimationControls>
    </div>
  );
}

// ────────────────────────────────────────────────────────────────────────────
// MAIN COMPONENT
// ────────────────────────────────────────────────────────────────────────────
export function Signaling() {
  return (
    <Style.BlogContainer>
      <Style.SectionTitle>Deep Dive</Style.SectionTitle>
      <Style.PageTitle>Epoch Signaling</Style.PageTitle>

      <Style.LeadParagraph>
        How does a thread know when data has changed? This question drives three decades of
        operating system design. Traditional approaches waste CPU cycles or add latency. INOS uses{' '}
        <strong>epoch signaling</strong>: atomic counters that achieve &lt;10µs notification
        latency.
      </Style.LeadParagraph>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 1: THE PROBLEM */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 1: The Notification Problem</h3>
          <p>
            Imagine you're writing a rendering loop. You need to draw frames whenever simulation
            data changes. But how do you know when data has changed?
          </p>
          <p>
            This is the <strong>notification problem</strong>, and it has plagued software
            engineering since the dawn of multi-threaded programming. Every solution involves a
            trade-off between <em>latency</em> (how fast you react) and <em>efficiency</em> (how
            much CPU you waste waiting).
          </p>
          <p style={{ marginBottom: 0 }}>
            Let's examine the three main paradigms and see why INOS chose a radically different
            approach.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.ComparisonGrid>
        <Style.ParadigmCard $type="bad">
          <h4>🔴 Polling</h4>
          <p>Check continuously in a loop.</p>
          <p>
            <strong>Latency:</strong> ~10ms typical
          </p>
          <p>
            <strong>CPU:</strong> 85%+ (always busy)
          </p>
          <p>
            <strong>Problem:</strong> Burns CPU even when nothing changes.
          </p>
        </Style.ParadigmCard>

        <Style.ParadigmCard $type="medium">
          <h4>🟡 Callbacks/Events</h4>
          <p>Register handlers, wait for events.</p>
          <p>
            <strong>Latency:</strong> 1-10ms
          </p>
          <p>
            <strong>CPU:</strong> ~25% (event queue overhead)
          </p>
          <p>
            <strong>Problem:</strong> Queue overhead adds latency.
          </p>
        </Style.ParadigmCard>

        <Style.ParadigmCard $type="good">
          <h4>🟢 Epoch Signaling</h4>
          <p>Atomic counter + sleep until change.</p>
          <p>
            <strong>Latency:</strong> &lt;10µs
          </p>
          <p>
            <strong>CPU:</strong> ~2% (sleeping)
          </p>
          <p>
            <strong>Advantage:</strong> 100x faster, near-zero CPU.
          </p>
        </Style.ParadigmCard>
      </Style.ComparisonGrid>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Interactive: Compare Paradigms</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <ParadigmComparisonDiagram />
        <Style.IllustrationCaption>
          Click buttons to see how each paradigm responds to data changes
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 2: DEEP DIVE ON POLLING */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 2: Why Polling Fails</h3>
          <p>
            Polling is the simplest approach: repeatedly check if data has changed. It's the first
            thing any programmer tries, and it works—poorly.
          </p>
          <p>Consider this naive render loop:</p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.CodeBlock>
        {`// ❌ Naive polling loop - DON'T DO THIS
function renderLoop() {
    while (true) {
        const data = readFromSAB(offset);  // Read shared buffer
        
        if (hasDataChanged(data)) {        // Compare to last known
            render(data);                   // Actually do work
        }
        
        // What now? Sleep? Busy loop?
        // Both options are bad.
    }
}`}
      </Style.CodeBlock>

      <Style.ContentCard>
        <p>The problems compound quickly:</p>
        <ul>
          <li>
            <strong>If you poll too fast (no sleep):</strong> You burn 100% CPU on one core doing
            nothing useful. Your laptop becomes a heater.
          </li>
          <li>
            <strong>If you poll too slow (10ms sleep):</strong> Your render loop can't respond
            faster than 10ms. At 60 FPS, that's a whole frame of latency.
          </li>
          <li>
            <strong>The Goldilocks problem:</strong> There's no "right" polling interval. Too fast
            wastes power; too slow adds latency. You can't win.
          </li>
        </ul>
      </Style.ContentCard>

      <Style.WarningCard>
        <h4>💡 The Deeper Problem</h4>
        <p>
          Polling fundamentally inverts the responsibility. The <em>consumer</em> (your render loop)
          is constantly asking "is there data yet?" But it should be the <em>producer</em> (the
          simulation) that says "data is ready—wake up!"
        </p>
      </Style.WarningCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 3: WHY CALLBACKS AREN'T ENOUGH */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 3: The Callback Overhead</h3>
          <p>
            Event systems and callbacks solve the busy-wait problem. The producer says "notify me
            when something happens," and the system delivers events to registered handlers. This is
            how most UI frameworks work.
          </p>
          <p>But there's hidden overhead:</p>
          <ul>
            <li>
              <strong>Event serialization:</strong> Events must be created as objects, often with
              JSON or similar encoding.
            </li>
            <li>
              <strong>Queue management:</strong> Events enter a queue, are sorted by priority, then
              dispatched.
            </li>
            <li>
              <strong>Context switching:</strong> The event loop must switch between producers and
              consumers.
            </li>
            <li>
              <strong>Garbage collection:</strong> In JavaScript, event objects create GC pressure.
            </li>
          </ul>
          <p style={{ marginBottom: 0 }}>
            For UI interactions (clicks, keypresses), this overhead is invisible—humans can't
            perceive 1ms. But for <strong>real-time systems</strong> processing 10,000+
            updates/second, it becomes a bottleneck.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>CPU Usage by Paradigm</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <CpuUsageDiagram />
        <Style.IllustrationCaption>
          Based on INOS benchmark: 10,000 operations/second, M1 MacBook Pro
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 4: EPOCH SIGNALING */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 4: The Epoch Solution</h3>
          <p>
            INOS takes a radically simpler approach. Instead of complex event systems, we use a
            single atomic integer called an <strong>epoch</strong>.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.DefinitionBox>
        <h4>Epoch</h4>
        <p>
          An <code>epoch</code> is a monotonically increasing counter stored in SharedArrayBuffer.
          Every time data changes, the producer increments the epoch. Consumers watch the epoch and
          wake only when it changes.
        </p>
      </Style.DefinitionBox>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Epoch Flow: Mutate → Signal → React</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <EpochFlowDiagram />
        <Style.IllustrationCaption>
          The epoch pattern eliminates event serialization and queue overhead
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.ContentCard>
        <p>The pattern is elegantly simple:</p>
        <ol>
          <li>
            <strong>MUTATE:</strong> The producer writes new data directly to SAB.
          </li>
          <li>
            <strong>SIGNAL:</strong> The producer atomically increments the epoch counter.
          </li>
          <li>
            <strong>REACT:</strong> Any consumer watching the epoch wakes instantly.
          </li>
        </ol>
        <p style={{ marginBottom: 0 }}>
          No event objects. No queue. No serialization. Just a single atomic operation that wakes
          all interested parties in <strong>&lt;10 microseconds</strong>.
        </p>
      </Style.ContentCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 5: IMPLEMENTATION */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 5: Implementation Details</h3>
          <p>
            INOS implements epochs in <code>kernel/threads/foundation/epoch.go</code> using a
            three-tier wait strategy optimized for near-instant response:
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.CodeBlock>
        {`// epoch.go - WaitForChange with <1µs latency
func (ee *EnhancedEpoch) WaitForChange(timeout time.Duration) (bool, error) {
    offset := OFFSET_ATOMIC_FLAGS + uint32(ee.index)*4
    
    // 1️⃣ FAST PATH: Check if already changed
    current := atomic.LoadUint32((*uint32)(unsafe.Add(ee.sabPtr, offset)))
    if current != ee.lastValue {
        ee.lastValue = current
        return true, nil  // Instant return!
    }
    
    // 2️⃣ SPIN-WAIT: Ultra-low latency for 1µs
    spinDeadline := time.Now().Add(time.Microsecond)
    for time.Now().Before(spinDeadline) {
        runtime.Gosched()  // Yield to scheduler
        current := atomic.LoadUint32(...)
        if current != ee.lastValue {
            ee.lastValue = current
            return true, nil
        }
    }
    
    // 3️⃣ SLEEP: Register for notification, sleep efficiently
    ch := make(chan struct{}, 1)
    ee.addWaiter(ch)
    defer ee.removeWaiter(ch)
    
    select {
    case <-ch:
        return true, nil  // Woken by producer
    case <-time.After(timeout):
        return false, nil  // Timed out
    }
}`}
      </Style.CodeBlock>

      <Style.ContentCard>
        <p>The three tiers ensure optimal behavior:</p>
        <ul>
          <li>
            <strong>Fast path:</strong> If data already changed, return immediately (0 latency).
          </li>
          <li>
            <strong>Spin-wait:</strong> For the first microsecond, busy-loop. This catches rapid
            updates with minimal latency.
          </li>
          <li>
            <strong>Sleep:</strong> After 1µs, go to sleep. The producer will wake us via channel.
          </li>
        </ul>
      </Style.ContentCard>

      <Style.HistoryCard>
        <h4>⚡ Why Spin-Wait for 1µs?</h4>
        <p>
          Context switching (sleeping and waking) takes ~1-10µs. If data changes within that window,
          spin-waiting is actually <em>faster</em> than sleeping. The 1µs spin threshold is
          optimized for typical real-time workloads.
        </p>
        <p>
          After 1µs, the probability of imminent change drops, so sleeping becomes more efficient.
          This hybrid approach achieves both low latency <em>and</em> low CPU usage.
        </p>
      </Style.HistoryCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 6: LIVE COMPARISON */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 6: See the Difference</h3>
          <p>
            The animated diagram below shows polling vs epoch signaling in real-time. Watch how
            polling constantly consumes CPU while epoch signaling sleeps peacefully until needed.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Live Animation: CPU Activity</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <AnimatedLoopDiagram />
        <Style.IllustrationCaption>
          Left: Polling constantly checks. Right: Epoch sleeps until signaled.
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 7: PERFORMANCE METRICS */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 7: Benchmark Results</h3>
          <p>
            From <code>integration/PERFORMANCE.md</code>, measured on MacBook Pro M1:
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.ContentCard>
        <Style.MetricRow>
          <Style.MetricLabel>Average Latency</Style.MetricLabel>
          <Style.MetricValue $highlight>&lt;10µs (vs 100-1000µs for events)</Style.MetricValue>
        </Style.MetricRow>
        <Style.MetricRow>
          <Style.MetricLabel>Maximum Latency</Style.MetricLabel>
          <Style.MetricValue $highlight>&lt;100µs (vs 1-10ms for events)</Style.MetricValue>
        </Style.MetricRow>
        <Style.MetricRow>
          <Style.MetricLabel>Operations/Second</Style.MetricLabel>
          <Style.MetricValue $highlight>&gt;100,000 (vs 1-10k for events)</Style.MetricValue>
        </Style.MetricRow>
        <Style.MetricRow>
          <Style.MetricLabel>Speedup vs Traditional</Style.MetricLabel>
          <Style.MetricValue $highlight>10-100x</Style.MetricValue>
        </Style.MetricRow>
      </Style.ContentCard>

      <Style.HistoryCard>
        <h4>🎯 Key Takeaways</h4>
        <p>
          <strong>1. Epochs are the ultimate notification mechanism.</strong> They combine the
          simplicity of polling with the efficiency of events—without the downsides of either.
        </p>
        <p>
          <strong>2. The magic is in SharedArrayBuffer.</strong> Epochs only work because SAB allows
          true shared memory with atomic operations across threads.
        </p>
        <p>
          <strong>3. INOS uses epochs everywhere.</strong> Module registration, render sync, mesh
          gossip—every subsystem signals via epochs.
        </p>
      </Style.HistoryCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* TAKEAWAYS */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Summary: The Epoch Revolution</h3>
          <ul>
            <li>
              <strong>Polling:</strong> Wastes CPU. Latency determined by poll interval.
            </li>
            <li>
              <strong>Callbacks:</strong> Better, but event queue adds 1-10ms overhead.
            </li>
            <li>
              <strong>Epochs:</strong> Atomic counter in SAB. &lt;10µs latency. Near-zero CPU when
              idle.
            </li>
          </ul>
          <p style={{ marginBottom: 0 }}>
            By embracing SharedArrayBuffer and atomic operations, INOS achieves{' '}
            <strong>10-100x lower latency</strong> than traditional event systems—while using less
            power.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <ChapterNav
        prev={{ to: '/deep-dives/zero-copy', title: 'Zero-Copy I/O' }}
        next={{ to: '/deep-dives/mesh', title: 'P2P Mesh' }}
      />
    </Style.BlogContainer>
  );
}

export default Signaling;
