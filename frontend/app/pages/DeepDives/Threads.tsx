/**
 * INOS Technical Codex — Deep Dive: Supervisor Threads
 *
 * A comprehensive exploration of the actor model, supervisor hierarchy,
 * and intelligent thread orchestration. Explains how INOS achieves
 * concurrent execution with learning, optimization, and self-healing.
 */

import { useState, useCallback } from 'react';
import styled, { useTheme } from 'styled-components';
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

  DefinitionBox: styled.div`
    background: rgba(139, 92, 246, 0.08);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(139, 92, 246, 0.2);
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[6]} 0;

    h4 {
      margin: 0 0 ${p => p.theme.spacing[2]} 0;
      color: #8b5cf6;
      font-size: ${p => p.theme.fontSizes.lg};
    }

    p {
      margin: 0;
      line-height: 1.7;
    }

    code {
      background: rgba(139, 92, 246, 0.1);
      padding: 2px 6px;
      border-radius: 3px;
      font-family: 'JetBrains Mono', monospace;
      font-size: 0.9em;
    }
  `,

  ResponsibilityGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[6]} 0;
  `,

  ResponsibilityCard: styled.div<{ $color: string }>`
    background: ${p => `${p.$color}10`};
    backdrop-filter: blur(12px);
    border: 1px solid ${p => `${p.$color}30`};
    border-radius: 8px;
    padding: ${p => p.theme.spacing[4]};
    text-align: center;

    .icon {
      font-size: 1.5rem;
      margin-bottom: ${p => p.theme.spacing[2]};
    }
    .name {
      font-size: ${p => p.theme.fontSizes.sm};
      font-weight: 600;
      color: ${p => p.$color};
      margin-bottom: ${p => p.theme.spacing[1]};
    }
    .desc {
      font-size: 11px;
      color: ${p => p.theme.colors.inkLight};
    }
  `,

  LoopCard: styled.div<{ $color: string }>`
    background: ${p => `${p.$color}08`};
    backdrop-filter: blur(12px);
    border: 1px solid ${p => `${p.$color}20`};
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};

    h4 {
      margin: 0 0 ${p => p.theme.spacing[2]} 0;
      color: ${p => p.$color};
      font-size: ${p => p.theme.fontSizes.base};
    }

    p {
      margin: 0;
      font-size: ${p => p.theme.fontSizes.sm};
      line-height: 1.6;
      color: ${p => p.theme.colors.inkMedium};
    }

    code {
      background: ${p => `${p.$color}15`};
      color: ${p => p.$color};
      padding: 2px 6px;
      border-radius: 3px;
      font-family: 'JetBrains Mono', monospace;
      font-size: 0.85em;
    }
  `,

  LoopGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[6]} 0;
  `,
};

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: SUPERVISOR HIERARCHY
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: SUPERVISOR HIERARCHY (D3Container)
// ────────────────────────────────────────────────────────────────────────────
function SupervisorHierarchyDiagram() {
  const theme = useTheme();

  const renderDiagram: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').remove();
      const designWidth = 700;
      const scale = Math.min(1, width / designWidth);
      const centerX = designWidth / 2;

      // Root supervisor
      svg
        .append('rect')
        .attr('x', centerX - 80 * scale)
        .attr('y', 30)
        .attr('width', 160 * scale)
        .attr('height', 50)
        .attr('rx', 8)
        .attr('fill', 'rgba(139, 92, 246, 0.15)')
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 3);
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', 55)
        .attr('text-anchor', 'middle')
        .attr('font-size', 13 * scale + 1 * (1 - scale))
        .attr('font-weight', 700)
        .attr('fill', '#8b5cf6')
        .text('RootSupervisor');
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', 72)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9 * scale)
        .attr('fill', theme.colors.inkLight)
        .text('Global coordination');

      // Unit supervisors
      const units = [
        { name: 'AudioSupervisor', x: centerX - 250 * scale, color: '#10b981' },
        { name: 'CryptoSupervisor', x: centerX - 80 * scale, color: '#3b82f6' },
        { name: 'GPUSupervisor', x: centerX + 80 * scale, color: '#f59e0b' },
        { name: 'StorageSupervisor', x: centerX + 250 * scale, color: '#ef4444' },
      ];

      // Lines from root to units
      units.forEach(unit => {
        svg
          .append('path')
          .attr('d', `M${centerX},80 L${unit.x},130`)
          .attr('stroke', '#d1d5db')
          .attr('stroke-width', 2)
          .attr('fill', 'none');
      });

      // Unit supervisor boxes
      units.forEach(unit => {
        svg
          .append('rect')
          .attr('x', unit.x - 70 * scale)
          .attr('y', 130)
          .attr('width', 140 * scale)
          .attr('height', 45)
          .attr('rx', 6)
          .attr('fill', `${unit.color}15`)
          .attr('stroke', unit.color)
          .attr('stroke-width', 2);
        svg
          .append('text')
          .attr('x', unit.x)
          .attr('y', 152)
          .attr('text-anchor', 'middle')
          .attr('font-size', 11 * scale)
          .attr('font-weight', 600)
          .attr('fill', unit.color)
          .text(unit.name);
        svg
          .append('text')
          .attr('x', unit.x)
          .attr('y', 167)
          .attr('text-anchor', 'middle')
          .attr('font-size', 8 * scale)
          .attr('fill', theme.colors.inkLight)
          .text('Unit Supervisor');
      });

      // Workflow supervisors (bottom)
      const workflows = [
        {
          name: 'ImagePipeline',
          x: centerX - 165 * scale,
          parent1: centerX - 250 * scale,
          parent2: centerX - 80 * scale,
        },
        {
          name: 'MLInference',
          x: centerX + 165 * scale,
          parent1: centerX + 80 * scale,
          parent2: centerX + 250 * scale,
        },
      ];

      workflows.forEach(wf => {
        svg
          .append('path')
          .attr('d', `M${wf.parent1},175 L${wf.x},210`)
          .attr('stroke', '#d1d5db')
          .attr('stroke-width', 1.5)
          .attr('stroke-dasharray', '4,4')
          .attr('fill', 'none');
        svg
          .append('path')
          .attr('d', `M${wf.parent2},175 L${wf.x},210`)
          .attr('stroke', '#d1d5db')
          .attr('stroke-width', 1.5)
          .attr('stroke-dasharray', '4,4')
          .attr('fill', 'none');

        svg
          .append('rect')
          .attr('x', wf.x - 60 * scale)
          .attr('y', 210)
          .attr('width', 120 * scale)
          .attr('height', 40)
          .attr('rx', 6)
          .attr('fill', 'rgba(59, 130, 246, 0.08)')
          .attr('stroke', '#3b82f6')
          .attr('stroke-width', 1.5)
          .attr('stroke-dasharray', '5,3');
        svg
          .append('text')
          .attr('x', wf.x)
          .attr('y', 232)
          .attr('text-anchor', 'middle')
          .attr('font-size', 10 * scale)
          .attr('font-weight', 600)
          .attr('fill', '#3b82f6')
          .text(wf.name);
        svg
          .append('text')
          .attr('x', wf.x)
          .attr('y', 245)
          .attr('text-anchor', 'middle')
          .attr('font-size', 8 * scale)
          .attr('fill', theme.colors.inkLight)
          .text('Workflow Supervisor');
      });

      // Legend
      svg
        .append('text')
        .attr('x', 30)
        .attr('y', 280)
        .attr('font-size', 9 * scale)
        .attr('fill', theme.colors.inkLight)
        .text('━━ Parent-Child │ ╌╌╌ Composition');
    },
    [theme]
  );

  return (
    <D3Container
      render={renderDiagram}
      dependencies={[renderDiagram]}
      viewBox="0 0 700 300"
      height={300}
    />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: UNIFIED SUPERVISOR INTERNALS
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: UNIFIED SUPERVISOR INTERNALS (D3Container)
// ────────────────────────────────────────────────────────────────────────────
function UnifiedSupervisorDiagram() {
  const theme = useTheme();
  const [activeLoop, setActiveLoop] = useState<string | null>(null);

  const renderDiagram: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').remove();
      const designWidth = 700;
      const scale = Math.min(1, width / designWidth);
      const centerX = designWidth / 2;

      // Main supervisor box
      svg
        .append('rect')
        .attr('x', 80 * scale)
        .attr('y', 20)
        .attr('width', 540 * scale)
        .attr('height', 280)
        .attr('rx', 12)
        .attr('fill', 'rgba(139, 92, 246, 0.05)')
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 2);
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', 50)
        .attr('text-anchor', 'middle')
        .attr('font-size', 14 * scale + 2 * (1 - scale))
        .attr('font-weight', 700)
        .attr('fill', '#8b5cf6')
        .text('UnifiedSupervisor');

      // Intelligence engines (left side)
      const engines = [
        { name: 'Learning', icon: '🧠', y: 90, color: '#10b981' },
        { name: 'Optimizer', icon: '⚡', y: 130, color: '#f59e0b' },
        { name: 'Scheduler', icon: '📋', y: 170, color: '#3b82f6' },
        { name: 'Security', icon: '🔒', y: 210, color: '#ef4444' },
        { name: 'Health', icon: '💚', y: 250, color: '#22c55e' },
      ];

      svg
        .append('text')
        .attr('x', 160 * scale)
        .attr('y', 75)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10 * scale)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkMedium)
        .text('Intelligence Engines');

      engines.forEach(eng => {
        svg
          .append('rect')
          .attr('x', 100 * scale)
          .attr('y', eng.y)
          .attr('width', 120 * scale)
          .attr('height', 28)
          .attr('rx', 4)
          .attr('fill', `${eng.color}15`)
          .attr('stroke', eng.color)
          .attr('stroke-width', 1.5);
        svg
          .append('text')
          .attr('x', 115 * scale)
          .attr('y', eng.y + 18)
          .attr('font-size', 12)
          .text(eng.icon);
        svg
          .append('text')
          .attr('x', 135 * scale)
          .attr('y', eng.y + 18)
          .attr('font-size', 11 * scale)
          .attr('font-weight', 500)
          .attr('fill', eng.color)
          .text(eng.name);
      });

      // Goroutine loops (right side)
      const loops = [
        { name: 'monitorLoop()', interval: '1s', y: 100, color: '#8b5cf6' },
        { name: 'scheduleLoop()', interval: '∞', y: 150, color: '#3b82f6' },
        { name: 'learningLoop()', interval: '1m', y: 200, color: '#10b981' },
        { name: 'healthLoop()', interval: '30s', y: 250, color: '#22c55e' },
      ];

      svg
        .append('text')
        .attr('x', designWidth - 190 * scale)
        .attr('y', 75)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10 * scale)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkMedium)
        .text('Goroutine Loops');

      loops.forEach(loop => {
        const isActive = activeLoop === loop.name;
        svg
          .append('rect')
          .attr('x', designWidth - 260 * scale)
          .attr('y', loop.y)
          .attr('width', 140 * scale)
          .attr('height', 35)
          .attr('rx', 4)
          .attr('fill', isActive ? `${loop.color}25` : `${loop.color}10`)
          .attr('stroke', loop.color)
          .attr('stroke-width', isActive ? 2 : 1.5);
        svg
          .append('text')
          .attr('x', designWidth - 190 * scale)
          .attr('y', loop.y + 16)
          .attr('text-anchor', 'middle')
          .attr('font-size', 10 * scale)
          .attr('font-weight', 600)
          .attr('fill', loop.color)
          .text(loop.name);
        svg
          .append('text')
          .attr('x', designWidth - 190 * scale)
          .attr('y', loop.y + 28)
          .attr('text-anchor', 'middle')
          .attr('font-size', 8 * scale)
          .attr('fill', theme.colors.inkLight)
          .text(`interval: ${loop.interval}`);
      });

      // Central job queue
      svg
        .append('rect')
        .attr('x', centerX - 50 * scale)
        .attr('y', 130)
        .attr('width', 100 * scale)
        .attr('height', 60)
        .attr('rx', 6)
        .attr('fill', 'rgba(59, 130, 246, 0.1)')
        .attr('stroke', '#3b82f6')
        .attr('stroke-width', 2);
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', 155)
        .attr('text-anchor', 'middle')
        .attr('font-size', 11 * scale)
        .attr('font-weight', 600)
        .attr('fill', '#3b82f6')
        .text('JobQueue');
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', 175)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9 * scale)
        .attr('fill', theme.colors.inkLight)
        .text('channels.Jobs');

      // Arrows
      svg
        .append('path')
        .attr('d', `M${220 * scale},160 L${centerX - 50 * scale},160`)
        .attr('stroke', '#d1d5db')
        .attr('stroke-width', 2)
        .attr('marker-end', 'url(#arrowhead)');
      svg
        .append('path')
        .attr('d', `M${centerX + 50 * scale},160 L${designWidth - 260 * scale},160`)
        .attr('stroke', '#d1d5db')
        .attr('stroke-width', 2)
        .attr('marker-end', 'url(#arrowhead)');

      svg
        .append('defs')
        .append('marker')
        .attr('id', 'arrowhead')
        .attr('viewBox', '0 0 10 10')
        .attr('refX', 8)
        .attr('refY', 5)
        .attr('markerWidth', 6)
        .attr('markerHeight', 6)
        .attr('orient', 'auto-start-reverse')
        .append('path')
        .attr('d', 'M 0 0 L 10 5 L 0 10 z')
        .attr('fill', '#d1d5db');
    },
    [theme, activeLoop]
  );

  return (
    <div>
      <D3Container
        render={renderDiagram}
        dependencies={[activeLoop]}
        viewBox="0 0 700 320"
        height={320}
      />
      <div
        style={{
          display: 'flex',
          gap: '8px',
          justifyContent: 'center',
          marginTop: '12px',
          flexWrap: 'wrap',
        }}
      >
        {['monitorLoop()', 'scheduleLoop()', 'learningLoop()', 'healthLoop()'].map(loop => (
          <button
            key={loop}
            onClick={() => setActiveLoop(activeLoop === loop ? null : loop)}
            style={{
              padding: '6px 12px',
              background: activeLoop === loop ? '#8b5cf6' : 'white',
              color: activeLoop === loop ? 'white' : '#6b7280',
              border: '1px solid',
              borderColor: activeLoop === loop ? '#8b5cf6' : '#d1d5db',
              borderRadius: '4px',
              fontSize: '11px',
              cursor: 'pointer',
            }}
          >
            {loop}
          </button>
        ))}
      </div>
    </div>
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: EPOCH COMMUNICATION
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: EPOCH COMMUNICATION (D3Container)
// ────────────────────────────────────────────────────────────────────────────
function EpochCommunicationDiagram() {
  const theme = useTheme();

  const renderDiagram: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').interrupt();
      svg.selectAll('*').remove();
      const designWidth = 700;
      const scale = Math.min(1, width / designWidth);

      svg
        .append('text')
        .attr('x', designWidth / 2)
        .attr('y', 30)
        .attr('text-anchor', 'middle')
        .attr('font-size', 12 * scale + 2 * (1 - scale))
        .attr('font-weight', 600)
        .attr('fill', '#3b82f6')
        .text('SharedArrayBuffer — Epoch Flags (Atomic)');

      svg
        .append('rect')
        .attr('x', 50 * scale)
        .attr('y', 50)
        .attr('width', 600 * scale)
        .attr('height', 70)
        .attr('rx', 6)
        .attr('fill', 'rgba(59, 130, 246, 0.1)')
        .attr('stroke', '#3b82f6')
        .attr('stroke-width', 2);

      const epochs = [
        { name: 'IDX_KERNEL', x: 100 * scale, base: 1 },
        { name: 'IDX_ML', x: 220 * scale, base: 42 },
        { name: 'IDX_GPU', x: 340 * scale, base: 18 },
        { name: 'IDX_STORAGE', x: 460 * scale, base: 7 },
        { name: 'PATTERN', x: 580 * scale, base: 3 },
      ];

      epochs.forEach(ep => {
        const text = svg
          .append('text')
          .attr('x', ep.x)
          .attr('y', 82)
          .attr('text-anchor', 'middle')
          .attr('font-size', 16 * scale)
          .attr('font-weight', 700)
          .attr('fill', '#3b82f6')
          .text(ep.base);
        svg
          .append('text')
          .attr('x', ep.x)
          .attr('y', 102)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9 * scale)
          .attr('fill', theme.colors.inkMedium)
          .text(ep.name);

        function tick() {
          text
            .transition()
            .duration(500)
            .delay(Math.random() * 2000)
            .text(() => {
              const current = parseInt(text.text());
              return current + 1;
            })
            .on('end', tick);
        }
        tick();
      });

      const supervisors = [
        { name: 'MLSupervisor', x: 150 * scale, color: '#10b981' },
        { name: 'GPUSupervisor', x: 350 * scale, color: '#f59e0b' },
        { name: 'StorageSupervisor', x: 550 * scale, color: '#ef4444' },
      ];

      supervisors.forEach(sup => {
        const g = svg.append('g');
        g.append('rect')
          .attr('x', sup.x - 60 * scale)
          .attr('y', 155)
          .attr('width', 120 * scale)
          .attr('height', 55)
          .attr('rx', 6)
          .attr('fill', `${sup.color}15`)
          .attr('stroke', sup.color)
          .attr('stroke-width', 2);
        g.append('text')
          .attr('x', sup.x)
          .attr('y', 178)
          .attr('text-anchor', 'middle')
          .attr('font-size', 11 * scale)
          .attr('font-weight', 600)
          .attr('fill', sup.color)
          .text(sup.name);
        const eye = g
          .append('text')
          .attr('x', sup.x)
          .attr('y', 198)
          .attr('text-anchor', 'middle')
          .attr('font-size', 18 * scale)
          .text('👁️');

        svg
          .append('path')
          .attr('d', `M${sup.x},155 L${sup.x},125`)
          .attr('stroke', sup.color)
          .attr('stroke-width', 1.5)
          .attr('stroke-dasharray', '4,4');

        function blink() {
          eye
            .transition()
            .duration(200)
            .delay(1000 + Math.random() * 3000)
            .text('😌')
            .transition()
            .duration(200)
            .delay(200)
            .text('👁️')
            .on('end', blink);
        }
        blink();
      });

      svg
        .append('text')
        .attr('x', designWidth / 2)
        .attr('y', 245)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10 * scale)
        .attr('fill', theme.colors.inkLight)
        .text('epoch.hasChanged() → Reactive trigger (no polling!)');
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
// D3 ILLUSTRATION: JOB EXECUTION FLOW
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: JOB EXECUTION FLOW (D3Container)
// ────────────────────────────────────────────────────────────────────────────
function JobExecutionFlowDiagram() {
  const theme = useTheme();

  const renderDiagram: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').remove();
      const scale = Math.min(1, width / 740);

      const stages = [
        { name: 'Submit', icon: '📥', x: 70 * scale, color: '#6b7280' },
        { name: 'Validate', icon: '🔒', x: 190 * scale, color: '#ef4444' },
        { name: 'Schedule', icon: '📋', x: 310 * scale, color: '#3b82f6' },
        { name: 'Execute', icon: '⚡', x: 430 * scale, color: '#f59e0b' },
        { name: 'Learn', icon: '🧠', x: 550 * scale, color: '#10b981' },
        { name: 'Result', icon: '✅', x: 670 * scale, color: '#22c55e' },
      ];

      // Flow line
      svg
        .append('line')
        .attr('x1', 90 * scale)
        .attr('y1', 80)
        .attr('x2', 650 * scale)
        .attr('y2', 80)
        .attr('stroke', '#e5e7eb')
        .attr('stroke-width', 3);

      stages.forEach((stage, i) => {
        svg
          .append('circle')
          .attr('cx', stage.x)
          .attr('cy', 80)
          .attr('r', 28 * scale)
          .attr('fill', `${stage.color}15`)
          .attr('stroke', stage.color)
          .attr('stroke-width', 2);
        svg
          .append('text')
          .attr('x', stage.x)
          .attr('y', 85)
          .attr('text-anchor', 'middle')
          .attr('font-size', 18 * scale)
          .text(stage.icon);
        svg
          .append('text')
          .attr('x', stage.x)
          .attr('y', 130)
          .attr('text-anchor', 'middle')
          .attr('font-size', 11 * scale)
          .attr('font-weight', 600)
          .attr('fill', stage.color)
          .text(stage.name);

        if (i < stages.length - 1) {
          svg
            .append('path')
            .attr('d', `M${stage.x + 35 * scale}, 80 L${stages[i + 1].x - 35 * scale}, 80`)
            .attr('stroke', '#d1d5db')
            .attr('stroke-width', 2)
            .attr('marker-end', 'url(#flow-arrow)');
        }
      });

      svg
        .append('defs')
        .append('marker')
        .attr('id', 'flow-arrow')
        .attr('viewBox', '0 0 10 10')
        .attr('refX', 8)
        .attr('refY', 5)
        .attr('markerWidth', 6)
        .attr('markerHeight', 6)
        .attr('orient', 'auto')
        .append('path')
        .attr('d', 'M 0 0 L 10 5 L 0 10 z')
        .attr('fill', '#d1d5db');
    },
    [theme]
  );

  return (
    <D3Container
      render={renderDiagram}
      dependencies={[renderDiagram]}
      viewBox="0 0 740 160"
      height={160}
    />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// MAIN COMPONENT
// ────────────────────────────────────────────────────────────────────────────
export function Threads() {
  return (
    <Style.BlogContainer>
      <Style.SectionTitle>Deep Dive</Style.SectionTitle>
      <Style.PageTitle>Supervisor Threads</Style.PageTitle>

      <Style.LeadParagraph>
        How do you orchestrate thousands of concurrent operations without race conditions, without
        deadlocks, without chaos? INOS answers with <strong>intelligent supervisors</strong>—actors
        that don't just manage threads, they <strong>learn</strong>, <strong>optimize</strong>, and{' '}
        <strong>heal themselves</strong>.
      </Style.LeadParagraph>

      <Style.SectionDivider />

      {/* LESSON 1: THE ACTOR MODEL */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 1: The Actor Model</h3>
          <p>
            Traditional threading is dangerous. Shared mutable state leads to race conditions, locks
            lead to deadlocks, and debugging concurrent bugs is a nightmare. The{' '}
            <strong>Actor Model</strong> solves this by treating each concurrent unit as an
            independent entity with:
          </p>
          <ul>
            <li>
              <strong>Private state</strong> — No shared memory between actors
            </li>
            <li>
              <strong>Message passing</strong> — Communication via immutable messages
            </li>
            <li>
              <strong>Supervision</strong> — Parent actors monitor and restart children
            </li>
          </ul>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.DefinitionBox>
        <h4>INOS Supervisors: Actors on Steroids</h4>
        <p>
          INOS extends the actor model with <strong>intelligence engines</strong>. Each supervisor
          isn't just a passive message router—it's an intelligent manager that learns from job
          patterns, optimizes parameters, enforces security, and monitors its own health. All
          communication happens via <code>SAB + Epochs</code>—zero function calls, zero copies.
        </p>
      </Style.DefinitionBox>

      <Style.SectionDivider />

      {/* LESSON 2: SUPERVISOR HIERARCHY */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 2: The Supervisor Hierarchy</h3>
          <p>
            INOS organizes supervisors in a <strong>three-level hierarchy</strong>. The
            RootSupervisor coordinates all unit supervisors, which in turn can compose into workflow
            supervisors for multi-unit operations.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Supervisor Hierarchy</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <SupervisorHierarchyDiagram />
        <Style.IllustrationCaption>
          Root → Unit → Workflow: Three levels of intelligent orchestration
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.ContentCard>
        <h3>Hierarchy Roles</h3>
        <ul>
          <li>
            <strong>RootSupervisor</strong> — Global coordination, spawns/kills unit supervisors,
            routes jobs, aggregates metrics, manages mesh integration
          </li>
          <li>
            <strong>UnitSupervisor</strong> — Executes jobs for specific unit (audio, crypto, GPU,
            storage), learns unit-specific patterns, reports to parent via Epochs
          </li>
          <li>
            <strong>WorkflowSupervisor</strong> — Manages multi-unit pipelines (e.g., ML inference),
            coordinates execution across units, optimizes data flow with zero-copy SAB transfers
          </li>
        </ul>
      </Style.ContentCard>

      <Style.SectionDivider />

      {/* LESSON 3: THE SIX RESPONSIBILITIES */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 3: The Six Responsibilities</h3>
          <p>
            Every supervisor has <strong>six core responsibilities</strong>. These aren't optional
            features—they're the foundation of what makes INOS supervisors intelligent rather than
            mere job routers.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.ResponsibilityGrid>
        <Style.ResponsibilityCard $color="#10b981">
          <div className="icon">📊</div>
          <div className="name">Manager</div>
          <div className="desc">Resource allocation, load balancing</div>
        </Style.ResponsibilityCard>
        <Style.ResponsibilityCard $color="#3b82f6">
          <div className="icon">🧠</div>
          <div className="name">Learner</div>
          <div className="desc">Pattern recognition, prediction</div>
        </Style.ResponsibilityCard>
        <Style.ResponsibilityCard $color="#f59e0b">
          <div className="icon">⚡</div>
          <div className="name">Optimizer</div>
          <div className="desc">Parameter tuning, algorithm selection</div>
        </Style.ResponsibilityCard>
        <Style.ResponsibilityCard $color="#8b5cf6">
          <div className="icon">📋</div>
          <div className="name">Scheduler</div>
          <div className="desc">Queue management, deadline-aware</div>
        </Style.ResponsibilityCard>
        <Style.ResponsibilityCard $color="#ef4444">
          <div className="icon">🔒</div>
          <div className="name">Security</div>
          <div className="desc">Input validation, threat detection</div>
        </Style.ResponsibilityCard>
        <Style.ResponsibilityCard $color="#22c55e">
          <div className="icon">💚</div>
          <div className="name">Health</div>
          <div className="desc">Metrics, anomalies, self-healing</div>
        </Style.ResponsibilityCard>
      </Style.ResponsibilityGrid>

      <Style.SectionDivider />

      {/* LESSON 4: UNIFIED SUPERVISOR INTERNALS */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 4: UnifiedSupervisor Internals</h3>
          <p>
            The <code>UnifiedSupervisor</code> is the base implementation. It contains five
            intelligence engines and runs four goroutine loops concurrently. Jobs flow through
            channels, and each loop handles specific aspects of supervision.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Interactive: Supervisor Anatomy</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <UnifiedSupervisorDiagram />
        <Style.IllustrationCaption>
          Five intelligence engines + four goroutine loops = one intelligent supervisor
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.LoopGrid>
        <Style.LoopCard $color="#8b5cf6">
          <h4>🔄 monitorLoop()</h4>
          <p>
            Runs every <code>1 second</code>. Collects health metrics, detects degradation, triggers
            alerts. Calls <code>Monitor(ctx)</code> which analyzes CPU, memory, latency, error
            rates.
          </p>
        </Style.LoopCard>
        <Style.LoopCard $color="#3b82f6">
          <h4>📥 scheduleLoop()</h4>
          <p>
            Runs <code>continuously</code>. Listens on <code>channels.Jobs</code>, validates
            security policies, calls <code>ExecuteJob()</code>, records latency, sends results.
          </p>
        </Style.LoopCard>
        <Style.LoopCard $color="#10b981">
          <h4>🧠 learningLoop()</h4>
          <p>
            Runs every <code>1 minute</code>. Scans for new patterns from SAB, updates ML models,
            shares learned patterns with peer supervisors via Epoch signaling.
          </p>
        </Style.LoopCard>
        <Style.LoopCard $color="#22c55e">
          <h4>💚 healthLoop()</h4>
          <p>
            Runs every <code>30 seconds</code>. Deep health analysis, anomaly detection, triggers
            self-healing actions like queue reordering or resource reallocation.
          </p>
        </Style.LoopCard>
      </Style.LoopGrid>

      <Style.CodeBlock>
        {`// From unified.go - UnifiedSupervisor structure
type UnifiedSupervisor struct {
  name         string
  capabilities []string

  // Intelligence engines
  learning  *learning.EnhancedLearningEngine
  optimizer *optimization.OptimizationEngine
  scheduler *scheduling.SchedulingEngine
  security  *security.SecurityEngine
  healthMon *health.HealthMonitor

  // Channels & state
  channels *ChannelSet
  jobQueue *JobQueue
  running  atomic.Bool
}`}
      </Style.CodeBlock>

      <Style.SectionDivider />

      {/* LESSON 5: EPOCH-BASED COMMUNICATION */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 5: Supervisor Communication via Epochs</h3>
          <p>
            Supervisors don't call each other's functions. They communicate by{' '}
            <strong>incrementing Epoch counters</strong> in SharedArrayBuffer. Other supervisors
            watch these counters and react when they change—a fully reactive, lock-free pattern.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Animated: Epoch Communication</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <EpochCommunicationDiagram />
        <Style.IllustrationCaption>
          Supervisors watch Epoch flags atomically—when values change, they react
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.CodeBlock>
        {`// From protocol.go - Epoch-based signaling
func (sp *SupervisorProtocol) SignalChange() {
  sp.epoch.Increment()  // Atomic increment in SAB
}

// Production implementation uses WaitForChange() which blocks
// via futex-style atomic waits—no polling required.
func (sp *SupervisorProtocol) WatchSupervisor(targetID string) <-chan struct{} {
  targetEpoch := sp.getEpochForSupervisor(targetID)
  ch := make(chan struct{})
  
  go func() {
    for {
      targetEpoch.WaitForChange() // Blocks until epoch changes (no polling)
      ch <- struct{}{}            // Reactive notification
    }
  }()
  
  return ch
}`}
      </Style.CodeBlock>

      <Style.SectionDivider />

      {/* LESSON 6: JOB EXECUTION FLOW */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 6: Job Execution Flow</h3>
          <p>
            Every job follows a consistent path through the supervisor. Security validation happens
            before scheduling, learning happens after execution, and results flow back through the
            same channel that submitted the job.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Job Lifecycle</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <JobExecutionFlowDiagram />
        <Style.IllustrationCaption>
          Submit → Validate → Schedule → Execute → Learn → Result
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.CodeBlock>
        {`// From unified.go - Job processing
func (us *UnifiedSupervisor) processJob(job *foundation.Job) {
  startTime := time.Now()

  // 1. Security check
  if !us.validateJob(job) {
    us.jobsFailed.Add(1)
    job.ResultChan <- &foundation.Result{
      JobID:   job.ID,
      Success: false,
      Error:   "Security validation failed",
    }
    return
  }

  // 2. Execute job
  result := us.ExecuteJob(job)

  // 3. Record metrics
  latency := time.Since(startTime)
  us.recordLatency(latency)

  // 4. Update counters
  if result.Success {
    us.jobsCompleted.Add(1)
  } else {
    us.jobsFailed.Add(1)
  }

  // 5. Send result back through channel
  job.ResultChan <- result
}`}
      </Style.CodeBlock>

      <Style.SectionDivider />

      <Style.ContentCard>
        <h3>The Vision: Self-Organizing Intelligence</h3>
        <p>
          INOS supervisors aren't just managing threads—they're{' '}
          <strong>learning from every job</strong>,{' '}
          <strong>optimizing parameters continuously</strong>, and{' '}
          <strong>healing themselves when things go wrong</strong>. The more the system runs, the
          smarter it gets.
        </p>
        <p style={{ marginBottom: 0 }}>
          This is concurrent programming evolved:{' '}
          <strong>actors that think, learn, and adapt</strong>.
        </p>
      </Style.ContentCard>

      <ChapterNav
        prev={{ to: '/deep-dives/economy', title: 'Credits & Economy' }}
        next={{ to: '/deep-dives/graphics', title: 'WebGPU Pipeline' }}
      />
    </Style.BlogContainer>
  );
}

export default Threads;
