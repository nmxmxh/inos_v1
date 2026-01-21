/**
 * INOS Technical Codex — Deep Dive: Graphics Pipeline
 *
 * A stunning exploration of zero-copy SAB architecture and WebGPU-capable
 * rendering. Integrates a procedural Terrain background with Architectural Boids
 * to demonstrate high-performance orchestration.
 *
 * Architecture: physics.rs → math.rs → gpu.rs → Three.js
 */

import { useState, useEffect, useCallback } from 'react';
import styled, { useTheme } from 'styled-components';
import * as d3 from 'd3';
import { Style as ManuscriptStyle } from '../../styles/manuscript';
import ChapterNav from '../../ui/ChapterNav';
import ScrollReveal from '../../ui/ScrollReveal';
import D3Container, { D3RenderFn } from '../../ui/D3Container';

// Import Three.js scenes
// import { TerrainScene } from '../../features/scenes';

const Style = {
  ...ManuscriptStyle,

  ContentCard: styled.div`
    background: rgba(255, 255, 255, 0.95);
    backdrop-filter: blur(16px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    box-shadow:
      0 4px 20px rgba(0, 0, 0, 0.05),
      0 1px 2px rgba(0, 0, 0, 0.02);
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
    background: rgba(255, 255, 255, 0.98);
    backdrop-filter: blur(16px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.03);
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

  PipelineStep: styled.div<{ $active?: boolean }>`
    display: flex;
    align-items: center;
    gap: ${p => p.theme.spacing[3]};
    padding: ${p => p.theme.spacing[3]} ${p => p.theme.spacing[4]};
    background: ${p => (p.$active ? 'rgba(59, 130, 246, 0.1)' : 'rgba(0, 0, 0, 0.02)')};
    border: 1px solid ${p => (p.$active ? 'rgba(59, 130, 246, 0.3)' : p.theme.colors.borderSubtle)};
    border-radius: 6px;
    margin-bottom: ${p => p.theme.spacing[2]};
    transition: all 0.2s ease;

    .step-number {
      width: 28px;
      height: 28px;
      background: ${p => (p.$active ? '#3b82f6' : '#e5e7eb')};
      color: ${p => (p.$active ? 'white' : '#6b7280')};
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 12px;
      font-weight: 600;
    }

    .step-content {
      flex: 1;

      .title {
        font-weight: 600;
        font-size: ${p => p.theme.fontSizes.sm};
        color: ${p => (p.$active ? '#1d4ed8' : p.theme.colors.inkDark)};
      }

      .desc {
        font-size: 11px;
        color: ${p => p.theme.colors.inkMedium};
      }
    }

    .timing {
      font-family: ${p => p.theme.fonts.typewriter};
      font-size: 11px;
      color: ${p => (p.$active ? '#3b82f6' : p.theme.colors.inkLight)};
    }
  `,

  MetricCard: styled.div`
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: ${p => p.theme.spacing[4]};
    background: rgba(255, 255, 255, 0.9);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 6px;
    margin-bottom: ${p => p.theme.spacing[3]};

    .label {
      font-size: ${p => p.theme.fontSizes.sm};
      color: ${p => p.theme.colors.inkMedium};
    }

    .value {
      font-family: ${p => p.theme.fonts.typewriter};
      font-size: ${p => p.theme.fontSizes.lg};
      font-weight: 700;
      color: ${p => p.theme.colors.inkDark};

      &.good {
        color: #16a34a;
      }
      &.warning {
        color: #f59e0b;
      }
      &.bad {
        color: #dc2626;
      }
    }
  `,

  PerformanceTable: styled.table`
    width: 100%;
    border-collapse: collapse;
    margin: ${p => p.theme.spacing[4]} 0;
    font-size: ${p => p.theme.fontSizes.sm};

    th,
    td {
      padding: ${p => p.theme.spacing[3]};
      text-align: left;
      border-bottom: 1px solid ${p => p.theme.colors.borderSubtle};
    }

    th {
      background: rgba(0, 0, 0, 0.02);
      font-weight: 600;
      color: ${p => p.theme.colors.inkDark};
    }

    .highlight {
      background: rgba(22, 163, 74, 0.08);
      color: #16a34a;
      font-weight: 600;
    }
  `,

  BufferIndicator: styled.div<{ $active: 'A' | 'B' }>`
    display: inline-flex;
    gap: ${p => p.theme.spacing[2]};
    padding: ${p => p.theme.spacing[2]} ${p => p.theme.spacing[3]};
    background: rgba(0, 0, 0, 0.05);
    border-radius: 4px;

    .buffer {
      padding: ${p => p.theme.spacing[1]} ${p => p.theme.spacing[2]};
      border-radius: 3px;
      font-family: ${p => p.theme.fonts.typewriter};
      font-size: 11px;
      font-weight: 600;

      &.A {
        background: ${p => (p.$active === 'A' ? '#3b82f6' : 'transparent')};
        color: ${p => (p.$active === 'A' ? 'white' : p.theme.colors.inkLight)};
        border: 1px solid ${p => (p.$active === 'A' ? '#3b82f6' : p.theme.colors.borderSubtle)};
      }

      &.B {
        background: ${p => (p.$active === 'B' ? '#8b5cf6' : 'transparent')};
        color: ${p => (p.$active === 'B' ? 'white' : p.theme.colors.inkLight)};
        border: 1px solid ${p => (p.$active === 'B' ? '#8b5cf6' : p.theme.colors.borderSubtle)};
      }
    }
  `,

  SceneContainer: styled.div`
    width: 100%;
    height: 400px;
    background: linear-gradient(135deg, #0f172a 0%, #1e293b 100%);
    border-radius: 8px;
    overflow: hidden;
    position: relative;

    .controls {
      position: absolute;
      top: ${p => p.theme.spacing[4]};
      right: ${p => p.theme.spacing[4]};
      display: flex;
      flex-direction: column;
      gap: ${p => p.theme.spacing[2]};
      z-index: 10;
    }

    .fallback {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: 100%;
      color: rgba(255, 255, 255, 0.7);
      text-align: center;
      padding: ${p => p.theme.spacing[6]};

      .icon {
        font-size: 48px;
        margin-bottom: ${p => p.theme.spacing[4]};
      }

      p {
        margin: 0;
        font-size: ${p => p.theme.fontSizes.sm};
      }
    }
  `,

  EffectToggle: styled.button<{ $active?: boolean }>`
    padding: ${p => p.theme.spacing[2]} ${p => p.theme.spacing[3]};
    background: ${p => (p.$active ? 'rgba(139, 92, 246, 0.9)' : 'rgba(255, 255, 255, 0.1)')};
    border: 1px solid ${p => (p.$active ? '#8b5cf6' : 'rgba(255, 255, 255, 0.2)')};
    border-radius: 4px;
    color: white;
    font-size: 11px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s ease;

    &:hover {
      background: ${p => (p.$active ? 'rgba(139, 92, 246, 1)' : 'rgba(255, 255, 255, 0.2)')};
    }
  `,

  ShaderGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: ${p => p.theme.spacing[3]};
    margin: ${p => p.theme.spacing[5]} 0;
  `,

  ShaderCard: styled.div<{ $category: string }>`
    background: rgba(255, 255, 255, 0.9);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 6px;
    padding: ${p => p.theme.spacing[4]};

    .category {
      font-size: 10px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: ${p => {
        switch (p.$category) {
          case 'rendering':
            return '#3b82f6';
          case 'particles':
            return '#f59e0b';
          case 'postproc':
            return '#8b5cf6';
          case 'procedural':
            return '#10b981';
          case 'physics':
            return '#ef4444';
          case 'shaders':
            return '#6366f1';
          default:
            return p.theme.colors.inkMedium;
        }
      }};
      margin-bottom: ${p => p.theme.spacing[2]};
    }

    .count {
      font-size: ${p => p.theme.fontSizes['2xl']};
      font-weight: 700;
      color: ${p => p.theme.colors.inkDark};
    }

    .label {
      font-size: ${p => p.theme.fontSizes.sm};
      color: ${p => p.theme.colors.inkMedium};
    }
  `,
};

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: ZERO-COPY PIPELINE
// ────────────────────────────────────────────────────────────────────────────
interface PipelineProps {
  activeBuffer: 'A' | 'B';
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: ZERO-COPY DATA FLOW (FULL PIPELINE)
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: ZERO-COPY DATA FLOW (FULL PIPELINE) (D3Container)
// ────────────────────────────────────────────────────────────────────────────
function ZeroCopyPipelineDiagram({ activeBuffer }: PipelineProps) {
  const theme = useTheme();

  const renderDiagram: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').remove();
      const isA = activeBuffer === 'A';
      const designWidth = 700;
      const scale = Math.min(1, width / designWidth);
      const sabX = 220 * scale + 50 * (1 - scale);
      const sabY = 40,
        sabW = 260 * scale,
        sabH = 180;

      // 1. SAB Container
      svg
        .append('rect')
        .attr('x', sabX)
        .attr('y', sabY)
        .attr('width', sabW)
        .attr('height', sabH)
        .attr('rx', 12)
        .attr('fill', 'rgba(0,0,0,0.02)')
        .attr('stroke', theme.colors.borderSubtle)
        .attr('stroke-dasharray', '4,2');
      svg
        .append('text')
        .attr('x', sabX + sabW / 2)
        .attr('y', sabY - 10)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 800)
        .attr('fill', theme.colors.inkMedium)
        .text('SHARED ARRAY CHANNEL (SAB)');

      // 2. Stages
      const drawStage = (name: string, sub: string, x: number, y: number, color: string) => {
        const sx = x * scale;
        const g = svg.append('g');
        g.append('rect')
          .attr('x', sx)
          .attr('y', y)
          .attr('width', 140 * scale)
          .attr('height', 45)
          .attr('rx', 6)
          .attr('fill', 'white')
          .attr('stroke', color)
          .attr('stroke-width', 2);
        g.append('text')
          .attr('x', sx + 10)
          .attr('y', y + 18)
          .attr('font-size', 10 * scale + 2 * (1 - scale))
          .attr('font-weight', 800)
          .attr('fill', color)
          .text(name);
        g.append('text')
          .attr('x', sx + 10)
          .attr('y', y + 33)
          .attr('font-size', 8)
          .attr('fill', theme.colors.inkLight)
          .text(sub);
        return g;
      };

      drawStage('GO SUPERVISOR', 'boids_supervisor.go', 30, 20, '#ef4444');
      drawStage('RUST COMPUTE', 'boids.rs (Physics)', 30, 85, '#f59e0b');
      drawStage('RUST MATH', 'math.rs (Matrices)', 30, 150, '#8b5cf6');
      drawStage('GPU RENDER', 'Instanced Mesh', 530 + 30 * (1 - scale), 85, '#10b981');

      // 3. SAB Slots
      const drawSlot = (y: number, name: string, active: boolean, type: 'boid' | 'matrix') => {
        const color = type === 'boid' ? '#3b82f6' : '#8b5cf6';
        svg
          .append('rect')
          .attr('x', sabX + 20)
          .attr('y', y)
          .attr('width', sabW - 40)
          .attr('height', 20)
          .attr('rx', 4)
          .attr('fill', active ? `${color}15` : 'white')
          .attr('stroke', active ? color : '#eee')
          .attr('stroke-width', active ? 2 : 1);
        svg
          .append('text')
          .attr('x', sabX + sabW / 2)
          .attr('y', y + 13)
          .attr('text-anchor', 'middle')
          .attr('font-size', 8 * scale + 2 * (1 - scale))
          .attr('font-weight', 800)
          .attr('fill', active ? color : '#999')
          .text(`${name} (${active ? 'ACTIVE' : 'STAGING'})`);
      };

      drawSlot(sabY + 15, 'Boid Buffer A', isA, 'boid');
      drawSlot(sabY + 40, 'Boid Buffer B', !isA, 'boid');
      drawSlot(sabY + 80, 'Matrix Buffer A', isA, 'matrix');
      drawSlot(sabY + 105, 'Matrix Buffer B', !isA, 'matrix');
      drawSlot(sabY + 145, 'Atomic Flags / Epochs', true, 'boid');

      // 4. Paths & Animations
      const lineGen = d3.line().curve(d3.curveMonotoneX);
      const animatePulse = (path: [number, number][], color: string, dur: string, r = 3) => {
        svg
          .append('circle')
          .attr('r', r)
          .attr('fill', color)
          .append('animateMotion')
          .attr('path', lineGen(path)!)
          .attr('dur', dur)
          .attr('repeatCount', 'indefinite');
      };

      const goPath: [number, number][] = [
        [170 * scale, 42],
        [200 * scale, 42],
        [200 * scale, sabY + 25],
        [sabX + 20, sabY + 25],
      ];
      const physPath: [number, number][] = [
        [170 * scale, 107],
        [210 * scale, 107],
        [210 * scale, sabY + 50],
        [sabX + 20, sabY + 50],
      ];
      const mathPath: [number, number][] = [
        [170 * scale, 172],
        [210 * scale, 172],
        [210 * scale, sabY + 115],
        [sabX + 20, sabY + 115],
      ];
      const gpuPath: [number, number][] = [
        [sabX + sabW - 20, sabY + 90],
        [510 * scale + 200 * (1 - scale), sabY + 90],
        [510 * scale + 200 * (1 - scale), 107],
        [530 * scale + 170 * (1 - scale), 107],
      ];

      svg
        .append('path')
        .attr('d', lineGen(goPath)!)
        .attr('fill', 'none')
        .attr('stroke', '#ef4444')
        .attr('stroke-width', 2)
        .attr('stroke-dasharray', '4,2');
      svg
        .append('path')
        .attr('d', lineGen(physPath)!)
        .attr('fill', 'none')
        .attr('stroke', '#f59e0b')
        .attr('stroke-width', 2);
      svg
        .append('path')
        .attr('d', lineGen(mathPath)!)
        .attr('fill', 'none')
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 2);
      svg
        .append('path')
        .attr('d', lineGen(gpuPath)!)
        .attr('fill', 'none')
        .attr('stroke', '#10b981')
        .attr('stroke-width', 2);

      animatePulse(goPath, '#ef4444', '4s');
      animatePulse(physPath, '#f59e0b', '1.5s');
      animatePulse(mathPath, '#8b5cf6', '1.5s');
      animatePulse(gpuPath, '#10b981', '0.8s');

      const goReadSignal: [number, number][] = [
        [sabX + 20, sabY + 155],
        [10, sabY + 155],
        [10, 42],
        [30 * scale, 42],
      ];
      const physSignal: [number, number][] = [
        [170 * scale, 115],
        [215 * scale, 115],
        [215 * scale, sabY + 155],
        [sabX + 20, sabY + 155],
      ];
      const mathSignal: [number, number][] = [
        [170 * scale, 180],
        [215 * scale, 180],
        [215 * scale, sabY + 155],
        [sabX + 20, sabY + 155],
      ];
      const flagsToMath: [number, number][] = [
        [sabX + 20, sabY + 155],
        [10, sabY + 155],
        [10, 172],
        [30 * scale, 172],
      ];

      svg
        .append('path')
        .attr('d', lineGen(goReadSignal)!)
        .attr('fill', 'none')
        .attr('stroke', '#ef4444')
        .attr('stroke-width', 1.5)
        .attr('stroke-dasharray', '2,2')
        .attr('opacity', 0.5);
      svg
        .append('path')
        .attr('d', lineGen(physSignal)!)
        .attr('fill', 'none')
        .attr('stroke', '#f59e0b')
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '2,2')
        .attr('opacity', 0.6);
      svg
        .append('path')
        .attr('d', lineGen(mathSignal)!)
        .attr('fill', 'none')
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '2,2')
        .attr('opacity', 0.6);
      svg
        .append('path')
        .attr('d', lineGen(flagsToMath)!)
        .attr('fill', 'none')
        .attr('stroke', theme.colors.inkLight)
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '1,1');

      animatePulse(goReadSignal, '#ef4444', '4s', 2);
      animatePulse(physSignal, '#f59e0b', '1.5s', 2);
      animatePulse(mathSignal, '#8b5cf6', '1.5s', 2);
      animatePulse(flagsToMath, theme.colors.inkMedium, '1.5s', 1.5);
    },
    [theme, activeBuffer]
  );

  return (
    <D3Container
      render={renderDiagram}
      dependencies={[renderDiagram]}
      viewBox="0 0 700 240"
      height={240}
    />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: PING-PONG BUFFERS (A/B SWAP) (D3Container)
// ────────────────────────────────────────────────────────────────────────────
function PingPongBufferDiagram({ activeBuffer }: PipelineProps) {
  const theme = useTheme();

  const renderDiagram: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').remove();
      const isA = activeBuffer === 'A';
      const designWidth = 700;
      const scale = Math.min(1, width / designWidth);
      const centerX = designWidth / 2;
      const mainColor = '#3b82f6',
        backColor = '#8b5cf6';

      // 1. Buffer Slots
      const drawBuffer = (name: string, y: number, color: string, active: boolean) => {
        const g = svg.append('g');
        g.append('rect')
          .attr('x', centerX - 130 * scale)
          .attr('y', y)
          .attr('width', 260 * scale)
          .attr('height', 40)
          .attr('rx', 6)
          .attr('fill', active ? `${color}15` : 'white')
          .attr('stroke', active ? color : '#e5e7eb')
          .attr('stroke-width', active ? 2 : 1);
        g.append('text')
          .attr('x', centerX - 110 * scale)
          .attr('y', y + 25)
          .attr('font-size', 12 * scale + 2 * (1 - scale))
          .attr('font-weight', 800)
          .attr('fill', active ? color : '#ccc')
          .text(`BUFFER ${name}`);
        g.append('text')
          .attr('x', centerX + 115 * scale)
          .attr('y', y + 25)
          .attr('text-anchor', 'end')
          .attr('font-size', 10 * scale + 2 * (1 - scale))
          .attr('font-weight', 600)
          .attr('fill', active ? color : '#999')
          .text(active ? 'READ ONLY' : 'WRITE ONLY');
      };

      drawBuffer('A', 40, mainColor, isA);
      drawBuffer('B', 100, backColor, !isA);

      // 2. Actors
      const drawConnector = (color: string, targetY: number, label: string, isLeft: boolean) => {
        const sx = isLeft ? 100 * scale : designWidth - 100 * scale;
        const arrowX1 = isLeft ? sx + 25 : sx - 25;
        const arrowX2 = isLeft ? centerX - 135 * scale : centerX + 135 * scale;

        svg
          .append('circle')
          .attr('cx', sx)
          .attr('cy', 90)
          .attr('r', 25 * scale + 5 * (1 - scale))
          .attr('fill', 'white')
          .attr('stroke', color)
          .attr('stroke-width', 2);
        svg
          .append('text')
          .attr('x', sx)
          .attr('y', 94)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9 * scale + 2 * (1 - scale))
          .attr('font-weight', 800)
          .attr('fill', color)
          .text(label);

        const path = isLeft
          ? `M${arrowX1},90 C${(arrowX1 + arrowX2) / 2},90 ${(arrowX1 + arrowX2) / 2},${targetY + 20} ${arrowX2},${targetY + 20}`
          : `M${arrowX1},90 C${(arrowX1 + arrowX2) / 2},90 ${(arrowX1 + arrowX2) / 2},${targetY + 20} ${arrowX2},${targetY + 20}`;

        svg
          .append('path')
          .attr('d', path)
          .attr('fill', 'none')
          .attr('stroke', color)
          .attr('stroke-width', 2)
          .attr('stroke-dasharray', '4,2')
          .style('transition', 'all 0.4s ease');
      };

      drawConnector('#f59e0b', isA ? 100 : 40, 'RUST', true);
      drawConnector('#10b981', isA ? 40 : 100, 'GPU', false);
    },
    [theme, activeBuffer]
  );

  return (
    <D3Container
      render={renderDiagram}
      dependencies={[renderDiagram]}
      viewBox="0 0 700 180"
      height={180}
    />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: GPU SHADER CATEGORIES
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: GPU SHADER CATEGORIES (D3Container)
// ────────────────────────────────────────────────────────────────────────────
function ShaderCategoriesDiagram() {
  const theme = useTheme();

  const renderDiagram: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').remove();
      const designWidth = 700;
      const scale = Math.min(1, width / designWidth);

      const categories = [
        { name: 'Rendering', count: 12, color: '#3b82f6', y: 30 },
        { name: 'Particles', count: 9, color: '#f59e0b', y: 70 },
        { name: 'Post-Processing', count: 15, color: '#8b5cf6', y: 110 },
        { name: 'Procedural', count: 10, color: '#10b981', y: 150 },
        { name: 'Physics Sim', count: 8, color: '#ef4444', y: 190 },
        { name: 'Shader Library', count: 11, color: '#6366f1', y: 230 },
      ];

      const maxCount = 15;
      const barWidth = 400 * scale;
      const startX = 150 * scale + 50 * (1 - scale);

      categories.forEach(cat => {
        svg
          .append('rect')
          .attr('x', startX)
          .attr('y', cat.y)
          .attr('width', barWidth)
          .attr('height', 24)
          .attr('rx', 4)
          .attr('fill', 'rgba(0, 0, 0, 0.05)');
        svg
          .append('rect')
          .attr('x', startX)
          .attr('y', cat.y)
          .attr('width', (cat.count / maxCount) * barWidth)
          .attr('height', 24)
          .attr('rx', 4)
          .attr('fill', cat.color)
          .attr('opacity', 0.8);
        svg
          .append('text')
          .attr('x', startX - 10)
          .attr('y', cat.y + 16)
          .attr('text-anchor', 'end')
          .attr('font-size', 12 * scale + 2 * (1 - scale))
          .attr('fill', theme.colors.inkDark)
          .text(cat.name);
        svg
          .append('text')
          .attr('x', startX + (cat.count / maxCount) * barWidth + 10)
          .attr('y', cat.y + 16)
          .attr('font-size', 12)
          .attr('font-weight', 600)
          .attr('fill', cat.color)
          .text(cat.count);
      });

      const total = categories.reduce((sum, c) => sum + c.count, 0);
      svg
        .append('text')
        .attr('x', 350)
        .attr('y', 280)
        .attr('text-anchor', 'middle')
        .attr('font-size', 14)
        .attr('font-weight', 700)
        .attr('fill', theme.colors.inkDark)
        .text(`${total} GPU Shaders Available`);
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

// Scenes are imported from features/scenes/

// ────────────────────────────────────────────────────────────────────────────
// MAIN COMPONENT
// ────────────────────────────────────────────────────────────────────────────
export default function Graphics() {
  const [activeBuffer, setActiveBuffer] = useState<'A' | 'B'>('A');

  useEffect(() => {
    const interval = setInterval(() => {
      setActiveBuffer(prev => (prev === 'A' ? 'B' : 'A'));
    }, 1000);
    return () => clearInterval(interval);
  }, []);

  return (
    <Style.BlogContainer>
      <Style.SectionTitle>Chapter 08 // Graphics Pipeline</Style.SectionTitle>
      <Style.PageTitle>The Light Path: From Compute to Retina</Style.PageTitle>

      <Style.LeadParagraph>
        Human vision is instantaneous perception, not a series of buffered images. When zero-copy
        memory meets GPU rendering, you get the <strong>Light Path</strong>: industrial-grade
        performance with 10,000+ entities @ 60fps. No frame drops. No GC pressure. Just pure,
        unencumbered reality.
      </Style.LeadParagraph>

      <Style.SectionDivider />

      {/* ─────────────────────────────────────────────────────────────────── */}
      {/* LESSON 1: ZERO-COPY PIPELINE */}
      {/* ─────────────────────────────────────────────────────────────────── */}
      <ScrollReveal>
        <Style.SectionTitle id="zero-copy">Lesson 1: The Zero-Copy Pipeline</Style.SectionTitle>

        <Style.ContentCard>
          <h3>The Problem: The Copy Tax</h3>
          <p>
            Traditional web graphics burn CPU cycles shuffling data between layers. Every frame,
            physics results are copied to JavaScript, transformed, then copied again to the GPU.
            This "copy tax" limits performance to hundreds of entities when you want thousands.
          </p>
        </Style.ContentCard>

        <Style.DefinitionBox>
          <h4>Zero-Copy Architecture</h4>
          <p>
            In INOS, data exists in a <code>SharedArrayBuffer</code> at fixed byte offsets. The
            Terrain background you see leverages two separate dispatch chains:
            <code>gpu.rs:perlin_noise</code> for the mountain base, and{' '}
            <code>gpu.rs:worley_noise</code>
            for rocky detail. The Cloud layer dispatches <code>fractal_noise</code> in real-time.
            <strong> Zero-copy between CPU stages; one final GPU upload.</strong>
          </p>
        </Style.DefinitionBox>

        <Style.IllustrationContainer>
          <Style.IllustrationHeader>
            <Style.IllustrationTitle>Animated: Zero-Copy Data Flow</Style.IllustrationTitle>
          </Style.IllustrationHeader>
          <ZeroCopyPipelineDiagram activeBuffer={activeBuffer} />
          <Style.IllustrationCaption>
            Data flows through SAB → Rust Physics → Rust Math → GPU with no inter-stage copies (one
            unavoidable GPU upload)
          </Style.IllustrationCaption>
        </Style.IllustrationContainer>

        <Style.ContentCard>
          <h3>The Performance Impact</h3>
          <Style.PerformanceTable>
            <thead>
              <tr>
                <th>Stage</th>
                <th>Traditional (1k entities)</th>
                <th>INOS Zero-Copy</th>
                <th>Improvement</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>Physics Compute</td>
                <td>~3ms</td>
                <td className="highlight">~1.5ms</td>
                <td>2×</td>
              </tr>
              <tr>
                <td>Matrix Generation</td>
                <td>~2ms</td>
                <td className="highlight">~0.8ms</td>
                <td>2.5×</td>
              </tr>
              <tr>
                <td>GPU Upload</td>
                <td>~1ms</td>
                <td className="highlight">~0.3ms</td>
                <td>3×</td>
              </tr>
              <tr style={{ fontWeight: 600 }}>
                <td>Total Frame Budget</td>
                <td>~6ms</td>
                <td className="highlight">~2.6ms</td>
                <td>2.3×</td>
              </tr>
            </tbody>
          </Style.PerformanceTable>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.SectionDivider />

      {/* ─────────────────────────────────────────────────────────────────── */}
      {/* LESSON 2: PING-PONG BUFFERS */}
      {/* ─────────────────────────────────────────────────────────────────── */}
      <ScrollReveal>
        <Style.SectionTitle id="ping-pong">Lesson 2: Ping-Pong Buffers</Style.SectionTitle>

        <Style.ContentCard>
          <h3>The Problem: Read/Write Conflicts</h3>
          <p>
            When physics writes to a buffer while the GPU reads it, you get "torn reads" — half the
            data is from frame N, half from frame N+1. The result: visual glitches, jitter, and
            undefined behavior.
          </p>
        </Style.ContentCard>

        <Style.DefinitionBox>
          <h4>Ping-Pong Double Buffering</h4>
          <p>
            INOS uses dual buffers. Physics writes to Buffer A while rendering reads Buffer B. On
            frame complete, an epoch counter increments, and buffers swap roles:{' '}
            <code>isBufferA = (epoch % 2 === 0)</code>.
            <strong> Lock-free. No mutexes. No torn reads.</strong>
          </p>
        </Style.DefinitionBox>

        <Style.IllustrationContainer>
          <Style.IllustrationHeader>
            <Style.IllustrationTitle>Animated: Ping-Pong Buffer Swap</Style.IllustrationTitle>
          </Style.IllustrationHeader>
          <PingPongBufferDiagram activeBuffer={activeBuffer} />
          <Style.IllustrationCaption>
            Epoch increments trigger automatic buffer role swap — write becomes read, read becomes
            write
          </Style.IllustrationCaption>
        </Style.IllustrationContainer>

        <Style.CodeBlock>
          {`// Rust: Write to inactive buffer
let buffer_info = matrix_ping_pong.write_buffer_info();
sab.write_raw(buffer_info.offset, &matrices);

// Flip epoch to signal completion
let new_epoch = matrix_ping_pong.flip();

// JavaScript: Read from active buffer
const epoch = Atomics.load(sabView, IDX_MATRIX_EPOCH);
const isBufferA = epoch % 2 === 0;
const offset = isBufferA ? OFFSET_MATRIX_A : OFFSET_MATRIX_B;
instanceMesh.instanceMatrix.array.set(
  new Float32Array(sab, offset, count * 16)
);`}
        </Style.CodeBlock>

        {/* Frame Timeline */}
        <Style.ContentCard>
          <h3>Frame Timeline (Felt Time)</h3>
          <Style.CodeBlock>
            {`T0:      Physics writes → epoch++
T0+2µs:  Math wakes → writes matrices → epoch++  
T0+5µs:  Renderer wakes → reads matrices
T0+6ms:  GPU completes draw call`}
          </Style.CodeBlock>
          <p style={{ marginTop: '1rem' }}>
            This is not a simulation—it's the actual measured timeline on an M1 MacBook Pro.
            Microsecond-level coordination with zero polling.
          </p>
        </Style.ContentCard>

        {/* R3F Contrast */}
        <Style.ContentCard>
          <h3>Contrast: Traditional React Three Fiber</h3>
          <p>
            In a traditional React Three Fiber setup, physics, transforms, and rendering all run
            inside JavaScript every frame. Physics libraries like Cannon.js or Rapier allocate
            objects. Transform calculations create temporary matrices. The GC wakes up. Frame times
            spike.
          </p>
          <p>
            <strong>INOS inverts this entirely:</strong> JavaScript's only job is to swap pointers
            and set <code>needsUpdate = true</code>. Physics runs in Rust workers. Matrix generation
            runs in Rust. The render loop contains zero computation—just pointer arithmetic.
          </p>
        </Style.ContentCard>

        {/* Failure Mode Resilience */}
        <Style.DefinitionBox>
          <h4>Graceful Degradation</h4>
          <p>
            If physics stalls, rendering simply reuses the last valid buffer—no partial frames, no
            jitter. The renderer checks <code>matrixEpoch</code> before dispatching; if unchanged,
            it skips the dispatch entirely and displays the last complete frame. This is automatic
            resilience, not explicit error handling.
          </p>
        </Style.DefinitionBox>
      </ScrollReveal>

      <Style.SectionDivider />

      {/* ─────────────────────────────────────────────────────────────────── */}
      {/* LESSON 3: SIGNAL-BASED ARCHITECTURE */}
      {/* ─────────────────────────────────────────────────────────────────── */}
      <ScrollReveal>
        <Style.SectionTitle id="signal-based">
          Lesson 3: Signal-Based Architecture
        </Style.SectionTitle>

        <Style.ContentCard>
          <h3>Zero-CPU Idle Time</h3>
          <p>
            Traditional animation loops poll for changes: "Is physics done? Is physics done?" This
            burns CPU cycles even when nothing happens. INOS uses
            <code> Atomics.wait()</code> — threads sleep until signaled, consuming 0% CPU when idle
            and waking instantly when data is ready.
          </p>
          <p style={{ fontSize: '0.875rem', color: '#6b7280', fontStyle: 'italic' }}>
            <strong>Note:</strong> <code>Atomics.wait()</code> requires cross-origin isolated
            contexts (COOP/COEP headers). Where unavailable, INOS degrades gracefully to timed
            waits.
          </p>
        </Style.ContentCard>

        <Style.ContentCard>
          <h3>The Epoch Signal Flow</h3>
          <ol>
            <li>
              <strong>Rust completes physics:</strong> Increments <code>IDX_BIRD_EPOCH</code>
            </li>
            <li>
              <strong>Math unit wakes:</strong> <code>Atomics.notify()</code> triggers matrix
              generation
            </li>
            <li>
              <strong>Math completes:</strong> Increments <code>IDX_MATRIX_EPOCH</code>
            </li>
            <li>
              <strong>Renderer wakes:</strong> Reads matrices, updates GPU
            </li>
          </ol>
          <p>
            <strong>Result:</strong> A dependency chain with zero polling. Each stage sleeps until
            its input is ready, then immediately processes.
          </p>
        </Style.ContentCard>

        <Style.CodeBlock>
          {`// Signal-based discovery loop in Go
for {
    // BLOCK until epoch changes (0% CPU while waiting)
    oldEpoch := Atomics.Load(sab, IDX_REGISTRY_EPOCH)
    Atomics.Wait(sab, IDX_REGISTRY_EPOCH, oldEpoch, -1)  // -1 = infinite
    
    // INSTANT wake when signaled
    newEpoch := Atomics.Load(sab, IDX_REGISTRY_EPOCH)
    if newEpoch > oldEpoch {
        processNewModules()  // React immediately
    }
}`}
        </Style.CodeBlock>
      </ScrollReveal>

      <Style.SectionDivider />

      {/* ─────────────────────────────────────────────────────────────────── */}
      {/* LESSON 4: INSTANCED RENDERING */}
      {/* ─────────────────────────────────────────────────────────────────── */}
      <ScrollReveal>
        <Style.SectionTitle id="instanced">Lesson 4: Instanced Rendering</Style.SectionTitle>

        <Style.ContentCard>
          <h3>One Draw Call, 10,000 Entities</h3>
          <p>
            Without instancing, 10,000 birds = 10,000 draw calls = 10,000 GPU context switches =
            slideshow. With instanced rendering, we send geometry
            <strong> once</strong> and inject 10,000 transformation matrices from SAB. One draw
            call. Full GPU performance.
          </p>
        </Style.ContentCard>

        <Style.DefinitionBox>
          <h4>Matrix Generation in Rust</h4>
          <p>
            Each bird has 8 parts (body, head, beak, wings, tail). The MathUnit generates 8 × 4×4
            matrices per bird in Rust using nalgebra, writing directly to SAB. JavaScript just
            copies the result to the GPU — <strong>zero computation in the render loop</strong>.
          </p>
        </Style.DefinitionBox>

        <Style.CodeBlock>
          {`// JavaScript render loop (zero math!)
useFrame((state, delta) => {
  // 1. Dispatch physics to Rust
  dispatch.execute('boids', 'step_physics', { count: 1000, dt: delta });
  
  // 2. Dispatch matrix generation to Rust  
  dispatch.execute('math', 'compute_instance_matrices', { count: 1000 });
  
  // 3. Just copy — no computation
  const matrixBase = (matrixEpoch % 2 === 0) 
    ? CONSTS.OFFSET_MATRIX_BUFFER_A 
    : CONSTS.OFFSET_MATRIX_BUFFER_B;
  
  const sabView = new Float32Array(sab, matrixBase, 1000 * 16);
  bodiesRef.current.instanceMatrix.array.set(sabView);
  bodiesRef.current.instanceMatrix.needsUpdate = true;
});`}
        </Style.CodeBlock>
      </ScrollReveal>

      <Style.SectionDivider />

      {/* ─────────────────────────────────────────────────────────────────── */}
      {/* LESSON 5: WEBGPU & POST-PROCESSING */}
      {/* ─────────────────────────────────────────────────────────────────── */}
      <ScrollReveal>
        <Style.SectionTitle id="webgpu">Lesson 5: WebGPU & Post-Processing</Style.SectionTitle>

        <Style.ContentCard>
          <h3>66 GPU Shaders, Ready to Use</h3>
          <p>
            The INOS GPU unit provides 66 validated WGSL compute shaders across 6 categories. Each
            shader is validated by Naga before execution, checked for security (workgroup limits,
            banned patterns), and cached for instant reuse.
          </p>
        </Style.ContentCard>

        <Style.IllustrationContainer>
          <Style.IllustrationHeader>
            <Style.IllustrationTitle>GPU Shader Categories</Style.IllustrationTitle>
          </Style.IllustrationHeader>
          <ShaderCategoriesDiagram />
          <Style.IllustrationCaption>
            gpu.rs provides shaders for rendering, particles, post-processing, procedural
            generation, physics simulation, and materials
          </Style.IllustrationCaption>
        </Style.IllustrationContainer>

        <Style.ShaderGrid>
          <Style.ShaderCard $category="rendering">
            <div className="category">Rendering Pipeline</div>
            <div className="count">12</div>
            <div className="label">PBR, ray tracing, deferred shading</div>
          </Style.ShaderCard>
          <Style.ShaderCard $category="particles">
            <div className="category">Particle Systems</div>
            <div className="count">9</div>
            <div className="label">N-body, forces, collisions, trails</div>
          </Style.ShaderCard>
          <Style.ShaderCard $category="postproc">
            <div className="category">Post-Processing</div>
            <div className="count">15</div>
            <div className="label">Bloom, DoF, motion blur, SSAO</div>
          </Style.ShaderCard>
          <Style.ShaderCard $category="procedural">
            <div className="category">Procedural Gen</div>
            <div className="count">10</div>
            <div className="label">Noise, heightmaps, erosion, textures</div>
          </Style.ShaderCard>
          <Style.ShaderCard $category="physics">
            <div className="category">Physics Simulation</div>
            <div className="count">8</div>
            <div className="label">Fluid, cloth, smoke, SPH</div>
          </Style.ShaderCard>
          <Style.ShaderCard $category="shaders">
            <div className="category">Shader Library</div>
            <div className="count">11</div>
            <div className="label">Glass, metal, fabric, water, SSS</div>
          </Style.ShaderCard>
        </Style.ShaderGrid>

        <Style.CodeBlock>
          {`// WGSL Shader Validation in Rust (gpu.rs)
pub fn validate_shader(&self, shader_code: &str) -> Result<ShaderAnalysis> {
    // 1. Quick hash check (cache hit?)
    let hash = blake3::hash(shader_code.as_bytes());
    if let Some(cached) = self.validation_cache.get(&hash) {
        return Ok(cached.clone());
    }
    
    // 2. Parse WGSL with Naga
    let module = wgsl::parse_str(shader_code)?;
    
    // 3. Validate module structure
    Validator::new(ValidationFlags::all(), Capabilities::all())
        .validate(&module)?;
    
    // 4. Security checks
    self.validator.validate_security(&module, shader_code)?;
    
    // 5. Analyze bindings for auto-bind
    let analysis = self.validator.analyze_shader(&module, shader_code)?;
    
    // 6. Cache for future use
    self.validation_cache.insert(hash, analysis.clone());
    Ok(analysis)
}`}
        </Style.CodeBlock>
      </ScrollReveal>

      <Style.SectionDivider />

      {/* ─────────────────────────────────────────────────────────────────── */}
      {/* CONCLUSION */}
      {/* ─────────────────────────────────────────────────────────────────── */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Graphics Without Compromise</h3>
          <p>
            The INOS graphics pipeline eliminates the false choice between performance and features.
            Zero-copy SAB, ping-pong buffers, signal-based architecture, instanced rendering, and 66
            GPU shaders combine into a system that can render 10,000+ animated entities at 60fps —
            in a web browser.
          </p>
          <p>
            <strong>The future is clear:</strong> WebGPU compute shaders will move even matrix
            generation to the GPU. The same SAB architecture that powers today's performance will
            enable tomorrow's innovations.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <ChapterNav
        prev={{ to: '/deep-dives/threads', title: 'Supervisor Threads' }}
        next={{ to: '/deep-dives/database', title: 'Database & Storage' }}
      />
    </Style.BlogContainer>
  );
}
