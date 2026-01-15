/**
 * INOS Technical Codex — Deep Dive: P2P Mesh
 *
 * A comprehensive exploration of peer-to-peer networking, distributed storage,
 * and shared compute. Explains how INOS achieves decentralized resilience.
 *
 * Educational approach: Start with P2P basics (torrents), then INOS specifics.
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

  ExperimentalBanner: styled.div`
    background: linear-gradient(135deg, rgba(139, 92, 246, 0.1) 0%, rgba(59, 130, 246, 0.1) 100%);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(139, 92, 246, 0.25);
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[6]} 0;
    display: flex;
    align-items: flex-start;
    gap: ${p => p.theme.spacing[4]};

    .icon {
      font-size: 24px;
    }

    .content {
      flex: 1;

      h4 {
        margin: 0 0 ${p => p.theme.spacing[2]} 0;
        color: #8b5cf6;
        font-size: ${p => p.theme.fontSizes.base};
      }

      p {
        margin: 0;
        line-height: 1.6;
        color: ${p => p.theme.colors.inkMedium};
        font-size: ${p => p.theme.fontSizes.sm};
      }
    }
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

  ComparisonGrid: styled.div`
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[5]} 0;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      grid-template-columns: 1fr;
    }
  `,

  ComparisonCard: styled.div<{ $type: 'traditional' | 'p2p' }>`
    background: ${p =>
      p.$type === 'traditional' ? 'rgba(220, 38, 38, 0.06)' : 'rgba(22, 163, 74, 0.06)'};
    backdrop-filter: blur(12px);
    border: 1px solid
      ${p => (p.$type === 'traditional' ? 'rgba(220, 38, 38, 0.2)' : 'rgba(22, 163, 74, 0.2)')};
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};

    h4 {
      margin: 0 0 ${p => p.theme.spacing[3]} 0;
      color: ${p => (p.$type === 'traditional' ? '#dc2626' : '#16a34a')};
      font-size: ${p => p.theme.fontSizes.base};
    }

    ul {
      margin: 0;
      padding-left: ${p => p.theme.spacing[5]};
    }

    li {
      margin-bottom: ${p => p.theme.spacing[2]};
      line-height: 1.5;
      font-size: ${p => p.theme.fontSizes.sm};
    }
  `,

  ScalingTable: styled.table`
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

    td {
      color: ${p => p.theme.colors.inkMedium};
    }

    td:last-child {
      color: #16a34a;
      font-weight: 500;
    }
  `,
};

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: CLIENT-SERVER VS P2P (D3Container + interactive toggle)
// ────────────────────────────────────────────────────────────────────────────
function ClientServerVsP2PDiagram() {
  const theme = useTheme();
  const [activeMode, setActiveMode] = useState<'server' | 'p2p'>('server');

  const renderDiagram: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').remove();
      const scale = Math.min(1, width / 700);
      const centerX = 350;

      if (activeMode === 'server') {
        const serverY = 50;
        svg
          .append('rect')
          .attr('x', centerX - 40)
          .attr('y', serverY - 20)
          .attr('width', 80)
          .attr('height', 40)
          .attr('rx', 6)
          .attr('fill', '#dc2626')
          .attr('stroke', '#991b1b')
          .attr('stroke-width', 2);
        svg
          .append('text')
          .attr('x', centerX)
          .attr('y', serverY + 5)
          .attr('text-anchor', 'middle')
          .attr('font-size', 11)
          .attr('font-weight', 600)
          .attr('fill', 'white')
          .text('SERVER');
        svg
          .append('text')
          .attr('x', centerX)
          .attr('y', serverY - 30)
          .attr('text-anchor', 'middle')
          .attr('font-size', 10)
          .attr('fill', '#dc2626')
          .text('⚠️ Single Point of Failure');

        const clients = [
          { x: 100 * scale + 50 * (1 - scale), y: 160 },
          { x: 250 * scale + 100 * (1 - scale), y: 180 },
          { x: 400 * scale + 150 * (1 - scale), y: 180 },
          { x: 550 * scale + 100 * (1 - scale), y: 160 },
        ];
        clients.forEach(c => {
          svg
            .append('line')
            .attr('x1', c.x)
            .attr('y1', c.y - 15)
            .attr('x2', centerX)
            .attr('y2', 70)
            .attr('stroke', 'rgba(220, 38, 38, 0.4)')
            .attr('stroke-width', 2)
            .attr('stroke-dasharray', '4,2');
          svg
            .append('circle')
            .attr('cx', c.x)
            .attr('cy', c.y)
            .attr('r', 18 * scale + 12 * (1 - scale))
            .attr('fill', 'rgba(220, 38, 38, 0.1)')
            .attr('stroke', '#dc2626')
            .attr('stroke-width', 1.5);
          svg
            .append('text')
            .attr('x', c.x)
            .attr('y', c.y + 4)
            .attr('text-anchor', 'middle')
            .attr('font-size', 10)
            .text('📱');
        });
        svg
          .append('text')
          .attr('x', centerX)
          .attr('y', 210)
          .attr('text-anchor', 'middle')
          .attr('font-size', 10)
          .attr('fill', theme.colors.inkLight)
          .text('All bandwidth costs on server • Limited by server capacity');
      } else {
        const nodes = [
          { x: centerX, y: 50 },
          { x: 150 * scale + 80 * (1 - scale), y: 90 },
          { x: 550 * scale + 80 * (1 - scale), y: 90 },
          { x: 100 * scale + 60 * (1 - scale), y: 160 },
          { x: 300 * scale + 100 * (1 - scale), y: 170 },
          { x: 450 * scale + 100 * (1 - scale), y: 170 },
          { x: 600 * scale + 80 * (1 - scale), y: 160 },
        ];
        const connections = [
          [0, 1],
          [0, 2],
          [0, 4],
          [0, 5],
          [1, 3],
          [1, 4],
          [2, 5],
          [2, 6],
          [3, 4],
          [4, 5],
          [5, 6],
        ];
        connections.forEach(([i, j]) =>
          svg
            .append('line')
            .attr('x1', nodes[i].x)
            .attr('y1', nodes[i].y)
            .attr('x2', nodes[j].x)
            .attr('y2', nodes[j].y)
            .attr('stroke', 'rgba(22, 163, 74, 0.3)')
            .attr('stroke-width', 2)
        );
        nodes.forEach((n, i) => {
          svg
            .append('circle')
            .attr('cx', n.x)
            .attr('cy', n.y)
            .attr('r', 20 * scale + 14 * (1 - scale))
            .attr('fill', 'rgba(22, 163, 74, 0.15)')
            .attr('stroke', '#16a34a')
            .attr('stroke-width', 2);
          svg
            .append('text')
            .attr('x', n.x)
            .attr('y', n.y + 5)
            .attr('text-anchor', 'middle')
            .attr('font-size', 10)
            .attr('font-weight', 600)
            .attr('fill', '#16a34a')
            .text(i === 0 ? '⭐' : '🔗');
        });
        svg
          .append('text')
          .attr('x', centerX)
          .attr('y', 25)
          .attr('text-anchor', 'middle')
          .attr('font-size', 10)
          .attr('fill', '#16a34a')
          .text('✓ No single point of failure • Self-healing');
        svg
          .append('text')
          .attr('x', centerX)
          .attr('y', 210)
          .attr('text-anchor', 'middle')
          .attr('font-size', 10)
          .attr('fill', theme.colors.inkLight)
          .text('Bandwidth shared across all peers • Scales with network size');
      }
    },
    [theme, activeMode]
  );

  return (
    <div>
      <D3Container
        render={renderDiagram}
        dependencies={[renderDiagram]}
        viewBox="0 0 700 220"
        height={220}
      />
      <div
        style={{
          display: 'flex',
          gap: '8px',
          padding: '12px 16px',
          borderTop: `1px solid ${theme.colors.borderSubtle}`,
          background: 'rgba(0,0,0,0.02)',
        }}
      >
        <button
          onClick={() => setActiveMode('server')}
          style={{
            padding: '6px 12px',
            background: activeMode === 'server' ? '#dc2626' : 'white',
            color: activeMode === 'server' ? 'white' : '#6b7280',
            border: '1px solid',
            borderColor: activeMode === 'server' ? '#dc2626' : '#d1d5db',
            borderRadius: '4px',
            fontSize: '11px',
            cursor: 'pointer',
          }}
        >
          Client-Server
        </button>
        <button
          onClick={() => setActiveMode('p2p')}
          style={{
            padding: '6px 12px',
            background: activeMode === 'p2p' ? '#16a34a' : 'white',
            color: activeMode === 'p2p' ? 'white' : '#6b7280',
            border: '1px solid',
            borderColor: activeMode === 'p2p' ? '#16a34a' : '#d1d5db',
            borderRadius: '4px',
            fontSize: '11px',
            cursor: 'pointer',
          }}
        >
          Peer-to-Peer
        </button>
      </div>
    </div>
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: HIERARCHICAL MESH TOPOLOGY (D3Container + D3 Transitions)
// ────────────────────────────────────────────────────────────────────────────
function HierarchicalMeshDiagram() {
  const theme = useTheme();

  const renderDiagram: D3RenderFn = useCallback(
    svg => {
      svg.selectAll('*').remove();

      const centerX = 350;
      const seedY = 50,
        hubY = 130,
        edgeY = 210;

      const seeds = [
        { x: centerX - 100, y: seedY },
        { x: centerX, y: seedY },
        { x: centerX + 100, y: seedY },
      ];
      const hubs = [
        { x: centerX - 160, y: hubY },
        { x: centerX - 80, y: hubY },
        { x: centerX, y: hubY },
        { x: centerX + 80, y: hubY },
        { x: centerX + 160, y: hubY },
      ];
      const edges: { x: number; y: number }[] = [];
      for (let i = 0; i < 7; i++) edges.push({ x: centerX - 240 + i * 80, y: edgeY });

      // Static connections
      seeds.forEach(s =>
        hubs.forEach(h => {
          if (Math.abs(s.x - h.x) < 130)
            svg
              .append('line')
              .attr('x1', s.x)
              .attr('y1', s.y + 16)
              .attr('x2', h.x)
              .attr('y2', h.y - 12)
              .attr('stroke', 'rgba(139, 92, 246, 0.12)')
              .attr('stroke-width', 1);
        })
      );
      hubs.forEach(h =>
        edges.forEach(e => {
          if (Math.abs(h.x - e.x) < 100)
            svg
              .append('line')
              .attr('x1', h.x)
              .attr('y1', h.y + 12)
              .attr('x2', e.x)
              .attr('y2', e.y - 10)
              .attr('stroke', 'rgba(14, 165, 233, 0.1)')
              .attr('stroke-width', 1);
        })
      );

      // Labels
      [
        { y: seedY, color: '#8b5cf6', label: 'SEEDS' },
        { y: hubY, color: '#0ea5e9', label: 'HUBS' },
        { y: edgeY, color: '#16a34a', label: 'EDGES' },
      ].forEach(l =>
        svg
          .append('text')
          .attr('x', 25)
          .attr('y', l.y + 4)
          .attr('font-size', 8)
          .attr('font-weight', 600)
          .attr('fill', l.color)
          .text(l.label)
      );

      // Nodes
      seeds.forEach(s => {
        svg
          .append('circle')
          .attr('cx', s.x)
          .attr('cy', s.y)
          .attr('r', 16)
          .attr('fill', 'rgba(139, 92, 246, 0.12)')
          .attr('stroke', '#8b5cf6')
          .attr('stroke-width', 2);
        svg
          .append('text')
          .attr('x', s.x)
          .attr('y', s.y + 4)
          .attr('text-anchor', 'middle')
          .attr('font-size', 11)
          .text('🌱');
      });
      hubs.forEach(h => {
        svg
          .append('circle')
          .attr('cx', h.x)
          .attr('cy', h.y)
          .attr('r', 12)
          .attr('fill', 'rgba(14, 165, 233, 0.12)')
          .attr('stroke', '#0ea5e9')
          .attr('stroke-width', 1.5);
        svg
          .append('text')
          .attr('x', h.x)
          .attr('y', h.y + 3)
          .attr('text-anchor', 'middle')
          .attr('font-size', 8)
          .text('🔷');
      });
      edges.forEach(e =>
        svg
          .append('circle')
          .attr('cx', e.x)
          .attr('cy', e.y)
          .attr('r', 10)
          .attr('fill', 'rgba(22, 163, 74, 0.1)')
          .attr('stroke', '#16a34a')
          .attr('stroke-width', 1.5)
      );

      // Legend
      const legendY = 250;
      svg
        .append('text')
        .attr('x', 70)
        .attr('y', legendY - 15)
        .attr('font-size', 9)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkMedium)
        .text('↓ DATA DOWN');
      [
        { x: 70, label: '📦 Chunk', color: '#f59e0b' },
        { x: 170, label: '🧠 Model', color: '#8b5cf6' },
        { x: 270, label: '⚡ State', color: '#06b6d4' },
      ].forEach(i => {
        svg.append('circle').attr('cx', i.x).attr('cy', legendY).attr('r', 4).attr('fill', i.color);
        svg
          .append('text')
          .attr('x', i.x + 10)
          .attr('y', legendY + 4)
          .attr('font-size', 8)
          .attr('fill', theme.colors.inkMedium)
          .text(i.label);
      });
      svg
        .append('text')
        .attr('x', 400)
        .attr('y', legendY - 15)
        .attr('font-size', 9)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkMedium)
        .text('↑ REQUESTS UP');
      [
        { x: 400, label: '❓ Query', color: '#10b981' },
        { x: 490, label: '🔄 Sync', color: '#3b82f6' },
        { x: 580, label: '💰 Credit', color: '#eab308' },
      ].forEach(i => {
        svg.append('circle').attr('cx', i.x).attr('cy', legendY).attr('r', 4).attr('fill', i.color);
        svg
          .append('text')
          .attr('x', i.x + 10)
          .attr('y', legendY + 4)
          .attr('font-size', 8)
          .attr('fill', theme.colors.inkMedium)
          .text(i.label);
      });

      // Animated packets using D3 transitions (not React state)
      const flows = [
        {
          emoji: '📦',
          color: '#f59e0b',
          path: [
            { x: seeds[0].x, y: seeds[0].y + 16 },
            { x: hubs[1].x, y: hubs[1].y - 12 },
            { x: hubs[1].x, y: hubs[1].y + 12 },
            { x: edges[1].x, y: edges[1].y - 10 },
          ],
          delay: 0,
        },
        {
          emoji: '🧠',
          color: '#8b5cf6',
          path: [
            { x: seeds[1].x, y: seeds[1].y + 16 },
            { x: hubs[2].x, y: hubs[2].y - 12 },
            { x: hubs[2].x, y: hubs[2].y + 12 },
            { x: edges[3].x, y: edges[3].y - 10 },
          ],
          delay: 1000,
        },
        {
          emoji: '⚡',
          color: '#06b6d4',
          path: [
            { x: seeds[2].x, y: seeds[2].y + 16 },
            { x: hubs[3].x, y: hubs[3].y - 12 },
            { x: hubs[3].x, y: hubs[3].y + 12 },
            { x: edges[5].x, y: edges[5].y - 10 },
          ],
          delay: 2000,
        },
      ];

      const timeouts: number[] = [];

      flows.forEach(flow => {
        const packet = svg
          .append('circle')
          .attr('cx', flow.path[0].x)
          .attr('cy', flow.path[0].y)
          .attr('r', 5)
          .attr('fill', flow.color)
          .attr('stroke', 'white')
          .attr('stroke-width', 1)
          .style('opacity', 0);
        const label = svg
          .append('text')
          .attr('x', flow.path[0].x)
          .attr('y', flow.path[0].y + 3)
          .attr('text-anchor', 'middle')
          .attr('font-size', 6)
          .text(flow.emoji)
          .style('opacity', 0);

        function animateFlow() {
          packet
            .style('opacity', 1)
            .attr('cx', flow.path[0].x)
            .attr('cy', flow.path[0].y)
            .transition()
            .duration(1000)
            .attr('cx', flow.path[1].x)
            .attr('cy', flow.path[1].y)
            .transition()
            .duration(100)
            .attr('cx', flow.path[2].x)
            .attr('cy', flow.path[2].y)
            .transition()
            .duration(1000)
            .attr('cx', flow.path[3].x)
            .attr('cy', flow.path[3].y)
            .transition()
            .duration(500)
            .style('opacity', 0)
            .on('end', animateFlow);
          label
            .style('opacity', 1)
            .attr('x', flow.path[0].x)
            .attr('y', flow.path[0].y + 3)
            .transition()
            .duration(1000)
            .attr('x', flow.path[1].x)
            .attr('y', flow.path[1].y + 3)
            .transition()
            .duration(100)
            .attr('x', flow.path[2].x)
            .attr('y', flow.path[2].y + 3)
            .transition()
            .duration(1000)
            .attr('x', flow.path[3].x)
            .attr('y', flow.path[3].y + 3)
            .transition()
            .duration(500)
            .style('opacity', 0);
        }
        const tId = window.setTimeout(animateFlow, flow.delay);
        timeouts.push(tId);
      });

      // Request flows (going up)
      const requests = [
        { emoji: '❓', color: '#10b981', from: edges[0], to: hubs[0], delay: 500 },
        { emoji: '🔄', color: '#3b82f6', from: edges[4], to: hubs[3], delay: 1500 },
        { emoji: '💰', color: '#eab308', from: edges[6], to: hubs[4], delay: 2500 },
      ];

      requests.forEach(req => {
        const packet = svg
          .append('circle')
          .attr('cx', req.from.x)
          .attr('cy', req.from.y - 10)
          .attr('r', 4)
          .attr('fill', req.color)
          .attr('stroke', 'white')
          .attr('stroke-width', 1)
          .style('opacity', 0);
        const label = svg
          .append('text')
          .attr('x', req.from.x)
          .attr('y', req.from.y - 7)
          .attr('text-anchor', 'middle')
          .attr('font-size', 5)
          .text(req.emoji)
          .style('opacity', 0);

        function animateReq() {
          packet
            .style('opacity', 1)
            .attr('cx', req.from.x)
            .attr('cy', req.from.y - 10)
            .transition()
            .duration(1500)
            .attr('cx', req.to.x)
            .attr('cy', req.to.y + 12)
            .transition()
            .duration(500)
            .style('opacity', 0)
            .on('end', animateReq);
          label
            .style('opacity', 1)
            .attr('x', req.from.x)
            .attr('y', req.from.y - 7)
            .transition()
            .duration(1500)
            .attr('x', req.to.x)
            .attr('y', req.to.y + 15)
            .transition()
            .duration(500)
            .style('opacity', 0);
        }
        const tId = window.setTimeout(animateReq, req.delay);
        timeouts.push(tId);
      });

      return () => {
        timeouts.forEach(clearTimeout);
        svg.selectAll('*').interrupt();
      };
    },
    [theme]
  );

  return (
    <D3Container
      render={renderDiagram}
      dependencies={[renderDiagram]}
      viewBox="0 0 700 280"
      height={280}
    />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: GOSSIP PROTOCOL (Properly Spaced & Centered)
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: GOSSIP PROTOCOL (D3Container + D3 Transitions)
// ────────────────────────────────────────────────────────────────────────────
interface GossipNode {
  x: number;
  y: number;
  index: number;
  delay: number;
}

function GossipDiagram() {
  const theme = useTheme();

  const renderDiagram: D3RenderFn = useCallback(
    svg => {
      svg.selectAll('*').remove();

      const centerX = 350;
      const ringCenterY = 130;
      const nodeCount = 8;
      const rxRadius = 200;
      const ryRadius = 80;

      const nodes: GossipNode[] = [];
      const getInformedDelay = (i: number) => Math.min(i, nodeCount - i) * 800;

      for (let i = 0; i < nodeCount; i++) {
        const angle = (i / nodeCount) * Math.PI * 2 - Math.PI / 2;
        nodes.push({
          x: centerX + Math.cos(angle) * rxRadius,
          y: ringCenterY + Math.sin(angle) * ryRadius,
          index: i,
          delay: getInformedDelay(i),
        });
      }

      // 1. Static connections (background)
      for (let i = 0; i < nodeCount; i++) {
        for (let offset = 1; offset <= 2; offset++) {
          const next = (i + offset) % nodeCount;
          // ID needed for animation targeting
          const lineId = `line-${Math.min(i, next)}-${Math.max(i, next)}`;
          svg
            .append('line')
            .attr('id', lineId)
            .attr('x1', nodes[i].x)
            .attr('y1', nodes[i].y)
            .attr('x2', nodes[next].x)
            .attr('y2', nodes[next].y)
            .attr('stroke', 'rgba(139, 92, 246, 0.1)')
            .attr('stroke-width', 1);
        }
      }

      // 2. Nodes
      const nodeGroups = nodes.map(n => {
        const g = svg.append('g').style('opacity', 1);
        const circle = g
          .append('circle')
          .attr('cx', n.x)
          .attr('cy', n.y)
          .attr('r', 18)
          .attr('fill', 'rgba(200, 200, 200, 0.12)')
          .attr('stroke', '#9ca3af')
          .attr('stroke-width', 2);
        const label = g
          .append('text')
          .attr('x', n.x)
          .attr('y', n.y + 5)
          .attr('text-anchor', 'middle')
          .attr('font-size', 11)
          .attr('font-weight', 600)
          .attr('fill', '#9ca3af')
          .text(n.index);
        const glow = g
          .append('circle')
          .attr('cx', n.x)
          .attr('cy', n.y)
          .attr('r', 24)
          .attr('fill', 'none')
          .attr('stroke', 'rgba(249, 115, 22, 0.35)')
          .attr('stroke-width', 3)
          .style('opacity', 0);
        return { g, circle, label, glow };
      });

      // 3. Stats & Progress Bar
      const statsY = 260;
      const counterText = svg
        .append('text')
        .attr('x', centerX)
        .attr('y', statsY)
        .attr('text-anchor', 'middle')
        .attr('font-size', 11)
        .attr('font-weight', 600)
        .attr('fill', '#9ca3af')
        .text(`Nodes informed: 0 / ${nodeCount}`);
      const barWidth = 200,
        barX = centerX - barWidth / 2;
      svg
        .append('rect')
        .attr('x', barX)
        .attr('y', statsY + 10)
        .attr('width', barWidth)
        .attr('height', 10)
        .attr('rx', 5)
        .attr('fill', 'rgba(200, 200, 200, 0.25)');
      const progressBar = svg
        .append('rect')
        .attr('x', barX)
        .attr('y', statsY + 10)
        .attr('width', 0)
        .attr('height', 10)
        .attr('rx', 5)
        .attr('fill', '#16a34a');

      // 4. Legend
      const legendY = statsY + 45;
      const legendItems = [
        { label: '● Has chunk', color: '#16a34a', xOffset: -130 },
        { label: '● Waiting', color: '#9ca3af', xOffset: -10 },
        { label: '● Gossip packet', color: '#f97316', xOffset: 100 },
      ];
      legendItems.forEach(item => {
        svg
          .append('circle')
          .attr('cx', centerX + item.xOffset)
          .attr('cy', legendY)
          .attr('r', 5)
          .attr('fill', item.color);
        svg
          .append('text')
          .attr('x', centerX + item.xOffset + 10)
          .attr('y', legendY + 4)
          .attr('font-size', 9)
          .attr('fill', theme.colors.inkMedium)
          .text(item.label);
      });

      const timeouts: number[] = [];

      function startCycle() {
        // Reset state
        nodeGroups.forEach(ng => {
          ng.circle
            .transition()
            .duration(0)
            .attr('fill', 'rgba(200, 200, 200, 0.12)')
            .attr('stroke', '#9ca3af');
          ng.label.transition().duration(0).attr('fill', '#9ca3af');
          ng.glow.style('opacity', 0);
        });
        progressBar.transition().duration(0).attr('width', 0);
        counterText.text(`Nodes informed: 0 / ${nodeCount}`).attr('fill', '#9ca3af');

        // Animate infection
        let informedCount = 0;
        nodes.forEach((n, i) => {
          const tId = window.setTimeout(() => {
            nodeGroups[i].circle
              .transition()
              .duration(300)
              .attr('fill', 'rgba(22, 163, 74, 0.15)')
              .attr('stroke', '#16a34a');
            nodeGroups[i].label.transition().duration(300).attr('fill', '#16a34a');
            nodeGroups[i].glow.style('opacity', 1).transition().duration(600).style('opacity', 0);

            informedCount++;
            counterText
              .text(`Nodes informed: ${informedCount} / ${nodeCount}`)
              .attr('fill', '#16a34a');
            progressBar
              .transition()
              .duration(300)
              .attr('width', (informedCount / nodeCount) * barWidth);

            // Send packets to neighbors
            [1, -1].forEach(offset => {
              const targetIdx = (i + offset + nodeCount) % nodeCount;
              const target = nodes[targetIdx];
              if (target.delay > n.delay) {
                // Animate connection line
                const lineId = `line-${Math.min(i, targetIdx)}-${Math.max(i, targetIdx)}`;
                const line = svg.select(`#${lineId}`);
                if (!line.empty()) {
                  line
                    .transition()
                    .duration(100)
                    .attr('stroke', '#16a34a')
                    .attr('stroke-width', 2)
                    .style('opacity', 1)
                    .transition()
                    .duration(500)
                    .attr('stroke', 'rgba(139, 92, 246, 0.1)')
                    .attr('stroke-width', 1);
                }

                const packet = svg
                  .append('circle')
                  .attr('cx', n.x)
                  .attr('cy', n.y)
                  .attr('r', 7)
                  .attr('fill', '#f97316')
                  .attr('stroke', 'white')
                  .attr('stroke-width', 1.5)
                  .style('opacity', 0);
                const emoji = svg
                  .append('text')
                  .attr('x', n.x)
                  .attr('y', n.y + 4)
                  .attr('text-anchor', 'middle')
                  .attr('font-size', 8)
                  .text('📨')
                  .style('opacity', 0);

                packet
                  .transition()
                  .delay(100)
                  .duration(0)
                  .style('opacity', 1)
                  .transition()
                  .duration(600)
                  .attr('cx', target.x)
                  .attr('cy', target.y)
                  .transition()
                  .duration(100)
                  .style('opacity', 0)
                  .remove();

                emoji
                  .transition()
                  .delay(100)
                  .duration(0)
                  .style('opacity', 1)
                  .transition()
                  .duration(600)
                  .attr('x', target.x)
                  .attr('y', target.y + 4)
                  .transition()
                  .duration(100)
                  .style('opacity', 0)
                  .remove();
              }
            });
          }, n.delay);
          timeouts.push(tId);
        });

        // Loop cycle
        const cycleId = window.setTimeout(startCycle, 5000);
        timeouts.push(cycleId);
      }

      startCycle();

      return () => {
        timeouts.forEach(clearTimeout);
        svg.selectAll('*').interrupt();
      };
    },
    [theme]
  );

  return (
    <D3Container
      render={renderDiagram}
      dependencies={[renderDiagram]}
      viewBox="0 0 700 340"
      height={340}
    />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// MAIN COMPONENT
// ────────────────────────────────────────────────────────────────────────────
export function Mesh() {
  return (
    <Style.BlogContainer>
      <Style.SectionTitle>Deep Dive</Style.SectionTitle>
      <Style.PageTitle>P2P Mesh</Style.PageTitle>

      <Style.LeadParagraph>
        What if your data didn't live on a single server? What if it was everywhere—and nowhere—at
        once? This is the promise of <strong>peer-to-peer networking</strong>: a world without
        central authorities, where resilience emerges from redundancy.
      </Style.LeadParagraph>

      <Style.ExperimentalBanner>
        <span className="icon">🧪</span>
        <div className="content">
          <h4>Pioneering Technology</h4>
          <p>
            The P2P mesh is our most ambitious subsystem. While the core protocols are implemented
            and tested, real-world validation requires network scale. We're actively seeking early
            adopters and community partners to help stress-test these systems.{' '}
            <strong>This is where the adventure begins.</strong>
          </p>
        </div>
      </Style.ExperimentalBanner>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 1: WHAT IS P2P? */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 1: The Centralization Problem</h3>
          <p>
            When you visit a website, your browser sends a request to a <strong>server</strong>—a
            central computer that stores all the data. This model has dominated the internet for 30
            years. But it has fundamental flaws:
          </p>
          <ul>
            <li>
              <strong>Single point of failure:</strong> If Netflix's servers go down, 200 million
              users lose access.
            </li>
            <li>
              <strong>Bandwidth bottleneck:</strong> One server must handle all traffic. Viral
              content = crashed servers.
            </li>
            <li>
              <strong>Censorship vulnerability:</strong> One court order can take down a site
              globally.
            </li>
            <li>
              <strong>Privacy erosion:</strong> The server sees everything—who requests what, when,
              from where.
            </li>
          </ul>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Interactive: Compare Architectures</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <ClientServerVsP2PDiagram />
        <Style.IllustrationCaption>
          Click to toggle between client-server and peer-to-peer topologies
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.HistoryCard>
        <h4>🌊 The BitTorrent Revolution</h4>
        <p>
          In 2001, Bram Cohen invented BitTorrent—and changed networking forever. Instead of
          downloading a file from one server, you download <em>pieces</em> from multiple peers
          simultaneously. Each peer that finishes downloading becomes a new source.
        </p>
        <p>
          <strong>The result:</strong> The more popular a file, the{' '}
          <em>faster and more available</em> it becomes. This is the opposite of traditional
          servers, where popularity causes crashes. INOS builds on this foundation.
        </p>
      </Style.HistoryCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 2: INOS MESH ARCHITECTURE */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 2: The INOS Mesh</h3>
          <p>
            INOS implements an <strong>adaptive P2P mesh</strong> that goes beyond simple file
            sharing. Our mesh handles:
          </p>
          <ul>
            <li>
              <strong>Storage:</strong> Files, databases, encrypted vaults
            </li>
            <li>
              <strong>Compute:</strong> ML inference, physics simulations, rendering jobs
            </li>
            <li>
              <strong>State:</strong> Real-time collaborative editing, game state sync
            </li>
            <li>
              <strong>Credits:</strong> The economic backbone that incentivizes participation
            </li>
          </ul>
          <p style={{ marginBottom: 0 }}>
            The key innovation is <strong>adaptive replication</strong>: the mesh automatically
            adjusts how many copies of data exist based on demand, size, and economic constraints.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.DefinitionBox>
        <h4>Adaptive Replication</h4>
        <p>
          INOS uses a <code>Rule of 5-7</code> as the minimum baseline: every piece of data exists
          on at least 5-7 nodes. But for large, high-demand resources, replicas scale up to{' '}
          <strong>500-700 nodes</strong>. The system continuously balances availability, cost, and
          latency.
        </p>
      </Style.DefinitionBox>

      <Style.ContentCard>
        <h3>Scaling Strategy by Resource Size</h3>
        <Style.ScalingTable>
          <thead>
            <tr>
              <th>Size</th>
              <th>Replicas</th>
              <th>Strategy</th>
              <th>Use Case</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>&lt;1 MB</td>
              <td>5-7</td>
              <td>Full replication</td>
              <td>State, metadata, configs</td>
            </tr>
            <tr>
              <td>1-100 MB</td>
              <td>10-50</td>
              <td>1MB chunking</td>
              <td>Images, documents, code</td>
            </tr>
            <tr>
              <td>100MB-10GB</td>
              <td>50-500</td>
              <td>Erasure coding</td>
              <td>Videos, datasets</td>
            </tr>
            <tr>
              <td>&gt;10 GB</td>
              <td>500-700</td>
              <td>Hierarchical CDN</td>
              <td>ML models, archives</td>
            </tr>
          </tbody>
        </Style.ScalingTable>
      </Style.ContentCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 3: HIERARCHICAL TOPOLOGY */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 3: Seeds, Hubs, and Edges</h3>
          <p>
            For massive resources (10GB+), flat P2P doesn't scale. INOS uses a{' '}
            <strong>hierarchical topology</strong>:
          </p>
          <ul>
            <li>
              <strong>Seeds (5-7):</strong> Authoritative nodes with complete copies. The source of
              truth.
            </li>
            <li>
              <strong>Hubs (50-100 per region):</strong> Regional aggregators storing 20% of chunks
              + parity data.
            </li>
            <li>
              <strong>Edges (500-700 total):</strong> Leaf nodes that cache frequently-accessed
              chunks on-demand.
            </li>
          </ul>
          <p style={{ marginBottom: 0 }}>
            This creates a CDN-like structure—but owned by the network, not a corporation.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Hierarchical Mesh Topology</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <HierarchicalMeshDiagram />
        <Style.IllustrationCaption>
          Seeds provide authority, Hubs provide regional speed, Edges cache hot content
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 4: GOSSIP PROTOCOL */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 4: The Gossip Protocol</h3>
          <p>
            How do nodes discover each other and learn about available data? Through{' '}
            <strong>gossip</strong>—a protocol inspired by how rumors spread.
          </p>
          <p>Each node periodically tells its neighbors about:</p>
          <ul>
            <li>
              <strong>PeerList:</strong> "Here are the nodes I know about"
            </li>
            <li>
              <strong>ChunkAd:</strong> "I have chunk X available"
            </li>
            <li>
              <strong>ModelAd:</strong> "I can run ML model Y"
            </li>
            <li>
              <strong>LedgerSync:</strong> "My credit balance merkle root is Z"
            </li>
          </ul>
          <p style={{ marginBottom: 0 }}>
            Information spreads exponentially: each node tells 3 neighbors, who each tell 3 more.
            Within seconds, the entire network knows about new resources.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Animated: Gossip Propagation</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <GossipDiagram />
        <Style.IllustrationCaption>
          Watch information spread through the network via gossip
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.CodeBlock>
        {`// From gossip.capnp.go - Gossip payload types
type GossipPayload union {
    ledgerSync :LedgerSync;    # Credit balance synchronization
    peerList   :PeerList;      # Known peers advertisement
    chunkAd    :ChunkAdvertisement;  # "I have this chunk"
    modelAd    :ModelAdvertisement;  # "I can run this model"
}

type PeerInfo struct {
    id           :Text;        # Peer identifier
    address      :Text;        # Connection address
    capabilities :List(Text);  # ["gpu", "storage", "compute"]
    reputation   :Float32;     # Trust score (0-1)
    lastSeen     :Int64;       # Unix timestamp
}`}
      </Style.CodeBlock>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 5: MESH API */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 5: The Mesh API</h3>
          <p>
            The <code>P2PMesh</code> Cap'n Proto interface provides 17 methods for interacting with
            the mesh. Here are the key ones:
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.CodeBlock>
        {`// From mesh.capnp - Core P2P Mesh API
interface P2PMesh {
    # Discovery
    findPeersWithChunk    (hash :Text)           -> (peers :List(PeerInfo));
    findBestPeerForChunk  (hash :Text)           -> (peer :PeerInfo);
    
    # Storage
    registerChunk   (hash :Text, size :UInt64)   -> (success :Bool);
    unregisterChunk (hash :Text)                 -> (success :Bool);
    
    # Reputation
    reportPeerPerformance (peerId :Text, metrics :PerfMetrics);
    getPeerReputation     (peerId :Text)         -> (score :Float32);
    getTopPeers           (count :UInt32)        -> (peers :List(PeerInfo));
    
    # Compute
    registerModel (model :ModelInfo)             -> (modelId :Text);
    findModel     (modelId :Text)                -> (providers :List(PeerInfo));
    
    # Connection
    connectToPeer      (peerId :Text, address :Text) -> (success :Bool);
    disconnectFromPeer (peerId :Text)                -> (success :Bool);
}`}
      </Style.CodeBlock>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* LESSON 6: SHARED COMPUTE */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 6: Distributed Compute</h3>
          <p>
            The mesh isn't just for storage—it's a <strong>distributed supercomputer</strong>. Any
            node can offer compute capabilities (GPU, CPU, specialized hardware) and earn credits
            for executing jobs.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.ComparisonGrid>
        <Style.ComparisonCard $type="traditional">
          <h4>☁️ Traditional Cloud Compute</h4>
          <ul>
            <li>Pay per hour to AWS/GCP/Azure</li>
            <li>Data leaves your control</li>
            <li>Vendor lock-in</li>
            <li>Centralized capacity limits</li>
            <li>Geographic restrictions</li>
          </ul>
        </Style.ComparisonCard>
        <Style.ComparisonCard $type="p2p">
          <h4>🌐 INOS Mesh Compute</h4>
          <ul>
            <li>Pay with credits to the network</li>
            <li>Encrypted, verifiable execution</li>
            <li>No lock-in—any node can serve</li>
            <li>Scales with network growth</li>
            <li>Globally distributed by default</li>
          </ul>
        </Style.ComparisonCard>
      </Style.ComparisonGrid>

      <Style.HistoryCard>
        <h4>💡 The Vision: Your Phone as a Server</h4>
        <p>
          Imagine a world where your idle devices—phones, laptops, smart TVs—contribute compute
          power to the mesh and earn credits passively. When you need compute, you spend those
          credits. The network becomes self-sustaining.
        </p>
        <p>
          This is the endgame: a global, economically-incentivized substrate for computation and
          storage, owned by no one and available to everyone.
        </p>
      </Style.HistoryCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SUMMARY */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Summary: The Decentralized Future</h3>
          <ul>
            <li>
              <strong>P2P eliminates single points of failure</strong> — data exists everywhere
            </li>
            <li>
              <strong>Adaptive replication (5-700 nodes)</strong> — scales with demand
            </li>
            <li>
              <strong>Hierarchical topology</strong> — Seeds → Hubs → Edges for massive scale
            </li>
            <li>
              <strong>Gossip protocol</strong> — information spreads in seconds
            </li>
            <li>
              <strong>Distributed compute</strong> — any node can contribute GPU/CPU
            </li>
            <li>
              <strong>Credit economy</strong> — incentivizes participation
            </li>
          </ul>
          <p style={{ marginBottom: 0 }}>
            The INOS mesh is still experimental, but the foundations are solid. As the network
            grows, so does its resilience. <strong>Join the mesh. Be the mesh.</strong>
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <ChapterNav
        prev={{ to: '/deep-dives/signaling', title: 'Epoch Signaling' }}
        next={{ to: '/deep-dives/economy', title: 'Credits & Economy' }}
      />
    </Style.BlogContainer>
  );
}

export default Mesh;
