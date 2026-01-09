/**
 * INOS Technical Codex — Deep Dive: P2P Mesh
 *
 * A comprehensive exploration of peer-to-peer networking, distributed storage,
 * and shared compute. Explains how INOS achieves decentralized resilience.
 *
 * Educational approach: Start with P2P basics (torrents), then INOS specifics.
 */

import { useEffect, useRef, useState } from 'react';
import styled, { useTheme } from 'styled-components';
import * as d3 from 'd3';
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
// D3 ILLUSTRATION: CLIENT-SERVER VS P2P
// ────────────────────────────────────────────────────────────────────────────
function ClientServerVsP2PDiagram() {
  const svgRef = useRef<SVGSVGElement>(null);
  const theme = useTheme();
  const [activeMode, setActiveMode] = useState<'server' | 'p2p'>('server');

  useEffect(() => {
    if (!svgRef.current) return;

    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const width = 700;
    const height = 220;

    if (activeMode === 'server') {
      // Client-Server: Central server, all clients connect to it
      const serverX = width / 2;
      const serverY = 50;

      // Server (big)
      svg
        .append('rect')
        .attr('x', serverX - 40)
        .attr('y', serverY - 20)
        .attr('width', 80)
        .attr('height', 40)
        .attr('rx', 6)
        .attr('fill', '#dc2626')
        .attr('stroke', '#991b1b')
        .attr('stroke-width', 2);

      svg
        .append('text')
        .attr('x', serverX)
        .attr('y', serverY + 5)
        .attr('text-anchor', 'middle')
        .attr('font-size', 11)
        .attr('font-weight', 600)
        .attr('fill', 'white')
        .text('SERVER');

      // Single point of failure indicator
      svg
        .append('text')
        .attr('x', serverX)
        .attr('y', serverY - 30)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('fill', '#dc2626')
        .text('⚠️ Single Point of Failure');

      // Clients
      const clients = [
        { x: 100, y: 160 },
        { x: 250, y: 180 },
        { x: 400, y: 180 },
        { x: 550, y: 160 },
      ];

      clients.forEach(client => {
        // Connection line
        svg
          .append('line')
          .attr('x1', client.x)
          .attr('y1', client.y - 15)
          .attr('x2', serverX)
          .attr('y2', serverY + 20)
          .attr('stroke', 'rgba(220, 38, 38, 0.4)')
          .attr('stroke-width', 2)
          .attr('stroke-dasharray', '4,2');

        // Client circle
        svg
          .append('circle')
          .attr('cx', client.x)
          .attr('cy', client.y)
          .attr('r', 18)
          .attr('fill', 'rgba(220, 38, 38, 0.1)')
          .attr('stroke', '#dc2626')
          .attr('stroke-width', 1.5);

        svg
          .append('text')
          .attr('x', client.x)
          .attr('y', client.y + 4)
          .attr('text-anchor', 'middle')
          .attr('font-size', 10)
          .attr('fill', '#dc2626')
          .text('📱');
      });

      // Cost indicator
      svg
        .append('text')
        .attr('x', width / 2)
        .attr('y', height - 10)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('fill', theme.colors.inkLight)
        .text('All bandwidth costs on server • Limited by server capacity');
    } else {
      // P2P: Mesh network, everyone connected to everyone
      const nodes = [
        { x: width / 2, y: 50 },
        { x: 150, y: 90 },
        { x: 550, y: 90 },
        { x: 100, y: 160 },
        { x: 300, y: 170 },
        { x: 450, y: 170 },
        { x: 600, y: 160 },
      ];

      // Draw connections between nearby nodes
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

      connections.forEach(([i, j]) => {
        svg
          .append('line')
          .attr('x1', nodes[i].x)
          .attr('y1', nodes[i].y)
          .attr('x2', nodes[j].x)
          .attr('y2', nodes[j].y)
          .attr('stroke', 'rgba(22, 163, 74, 0.3)')
          .attr('stroke-width', 2);
      });

      // Draw nodes
      nodes.forEach((node, i) => {
        svg
          .append('circle')
          .attr('cx', node.x)
          .attr('cy', node.y)
          .attr('r', 20)
          .attr('fill', 'rgba(22, 163, 74, 0.15)')
          .attr('stroke', '#16a34a')
          .attr('stroke-width', 2);

        svg
          .append('text')
          .attr('x', node.x)
          .attr('y', node.y + 5)
          .attr('text-anchor', 'middle')
          .attr('font-size', 10)
          .attr('font-weight', 600)
          .attr('fill', '#16a34a')
          .text(i === 0 ? '⭐' : '🔗');
      });

      // Resilience indicator
      svg
        .append('text')
        .attr('x', width / 2)
        .attr('y', 25)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('fill', '#16a34a')
        .text('✓ No single point of failure • Self-healing');

      // Cost indicator
      svg
        .append('text')
        .attr('x', width / 2)
        .attr('y', height - 10)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('fill', theme.colors.inkLight)
        .text('Bandwidth shared across all peers • Scales with network size');
    }
  }, [theme, activeMode]);

  return (
    <div>
      <svg ref={svgRef} viewBox="0 0 700 220" style={{ width: '100%', height: 'auto' }} />
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
// D3 ILLUSTRATION: HIERARCHICAL MESH TOPOLOGY (Multi-Flow Animation)
// ────────────────────────────────────────────────────────────────────────────
function HierarchicalMeshDiagram() {
  const svgRef = useRef<SVGSVGElement>(null);
  const theme = useTheme();
  const [frame, setFrame] = useState(0);
  const animationRef = useRef<number | null>(null);

  useEffect(() => {
    const animate = () => {
      setFrame(f => (f + 1) % 360);
      animationRef.current = requestAnimationFrame(animate);
    };
    animationRef.current = requestAnimationFrame(animate);
    return () => {
      if (animationRef.current) cancelAnimationFrame(animationRef.current);
    };
  }, []);

  useEffect(() => {
    if (!svgRef.current) return;

    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const width = 700;
    const centerX = width / 2;

    // Layer definitions with MORE spacing to prevent clashing
    const seedY = 50;
    const hubY = 130;
    const edgeY = 210;

    // Node positions - centered
    const seeds = [
      { x: centerX - 100, y: seedY, id: 0 },
      { x: centerX, y: seedY, id: 1 },
      { x: centerX + 100, y: seedY, id: 2 },
    ];

    const hubs = [
      { x: centerX - 160, y: hubY, id: 0 },
      { x: centerX - 80, y: hubY, id: 1 },
      { x: centerX, y: hubY, id: 2 },
      { x: centerX + 80, y: hubY, id: 3 },
      { x: centerX + 160, y: hubY, id: 4 },
    ];

    const edges: { x: number; y: number; id: number }[] = [];
    const edgeCount = 7;
    const edgeSpacing = 80;
    const edgeStartX = centerX - ((edgeCount - 1) * edgeSpacing) / 2;
    for (let i = 0; i < edgeCount; i++) {
      edges.push({ x: edgeStartX + i * edgeSpacing, y: edgeY, id: i });
    }

    // Draw static connections
    seeds.forEach(seed => {
      hubs.forEach(hub => {
        if (Math.abs(seed.x - hub.x) < 130) {
          svg
            .append('line')
            .attr('x1', seed.x)
            .attr('y1', seed.y + 16)
            .attr('x2', hub.x)
            .attr('y2', hub.y - 12)
            .attr('stroke', 'rgba(139, 92, 246, 0.12)')
            .attr('stroke-width', 1);
        }
      });
    });

    hubs.forEach(hub => {
      edges.forEach(edge => {
        if (Math.abs(hub.x - edge.x) < 100) {
          svg
            .append('line')
            .attr('x1', hub.x)
            .attr('y1', hub.y + 12)
            .attr('x2', edge.x)
            .attr('y2', edge.y - 10)
            .attr('stroke', 'rgba(14, 165, 233, 0.1)')
            .attr('stroke-width', 1);
        }
      });
    });

    // Layer labels (left side) - compact
    [
      { y: seedY, color: '#8b5cf6', label: 'SEEDS' },
      { y: hubY, color: '#0ea5e9', label: 'HUBS' },
      { y: edgeY, color: '#16a34a', label: 'EDGES' },
    ].forEach(layer => {
      svg
        .append('text')
        .attr('x', 25)
        .attr('y', layer.y + 4)
        .attr('font-size', 8)
        .attr('font-weight', 600)
        .attr('fill', layer.color)
        .text(layer.label);
    });

    // Draw nodes
    seeds.forEach(seed => {
      svg
        .append('circle')
        .attr('cx', seed.x)
        .attr('cy', seed.y)
        .attr('r', 16)
        .attr('fill', 'rgba(139, 92, 246, 0.12)')
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 2);
      svg
        .append('text')
        .attr('x', seed.x)
        .attr('y', seed.y + 4)
        .attr('text-anchor', 'middle')
        .attr('font-size', 11)
        .text('🌱');
    });

    hubs.forEach(hub => {
      svg
        .append('circle')
        .attr('cx', hub.x)
        .attr('cy', hub.y)
        .attr('r', 12)
        .attr('fill', 'rgba(14, 165, 233, 0.12)')
        .attr('stroke', '#0ea5e9')
        .attr('stroke-width', 1.5);
      svg
        .append('text')
        .attr('x', hub.x)
        .attr('y', hub.y + 3)
        .attr('text-anchor', 'middle')
        .attr('font-size', 8)
        .text('🔷');
    });

    edges.forEach(edge => {
      svg
        .append('circle')
        .attr('cx', edge.x)
        .attr('cy', edge.y)
        .attr('r', 10)
        .attr('fill', 'rgba(22, 163, 74, 0.1)')
        .attr('stroke', '#16a34a')
        .attr('stroke-width', 1.5);
    });

    // Multiple simultaneous flows with different colors and types
    const flows = [
      // DOWN flows (data packets)
      {
        type: 'chunk',
        emoji: '📦',
        color: '#f59e0b',
        from: seeds[0],
        toHub: hubs[1],
        toEdge: edges[1],
        offset: 0,
      },
      {
        type: 'model',
        emoji: '🧠',
        color: '#8b5cf6',
        from: seeds[1],
        toHub: hubs[2],
        toEdge: edges[3],
        offset: 60,
      },
      {
        type: 'state',
        emoji: '⚡',
        color: '#06b6d4',
        from: seeds[2],
        toHub: hubs[3],
        toEdge: edges[5],
        offset: 120,
      },
    ];

    // UP flows (requests)
    const requests = [
      {
        type: 'query',
        emoji: '❓',
        color: '#10b981',
        fromEdge: edges[0],
        toHub: hubs[0],
        offset: 30,
      },
      {
        type: 'sync',
        emoji: '🔄',
        color: '#3b82f6',
        fromEdge: edges[4],
        toHub: hubs[3],
        offset: 90,
      },
      {
        type: 'credit',
        emoji: '💰',
        color: '#eab308',
        fromEdge: edges[6],
        toHub: hubs[4],
        offset: 150,
      },
    ];

    // Helper to draw a packet
    const drawPacket = (x: number, y: number, emoji: string, color: string, size: number = 5) => {
      svg
        .append('circle')
        .attr('cx', x)
        .attr('cy', y)
        .attr('r', size)
        .attr('fill', color)
        .attr('stroke', 'white')
        .attr('stroke-width', 1);
      svg
        .append('text')
        .attr('x', x)
        .attr('y', y + 3)
        .attr('text-anchor', 'middle')
        .attr('font-size', 6)
        .text(emoji);
    };

    // Animate DOWN flows
    flows.forEach(flow => {
      const cycleFrame = (frame + flow.offset) % 180;
      const phase = cycleFrame / 60; // 0-3

      if (phase < 1) {
        // Seed → Hub
        const t = phase;
        const x = flow.from.x + (flow.toHub.x - flow.from.x) * t;
        const y = flow.from.y + 16 + (flow.toHub.y - 12 - (flow.from.y + 16)) * t;
        drawPacket(x, y, flow.emoji, flow.color);
      } else if (phase < 2) {
        // Hub → Edge
        const t = phase - 1;
        const x = flow.toHub.x + (flow.toEdge.x - flow.toHub.x) * t;
        const y = flow.toHub.y + 12 + (flow.toEdge.y - 10 - (flow.toHub.y + 12)) * t;
        drawPacket(x, y, flow.emoji, flow.color);
      }
    });

    // Animate UP flows (requests going up)
    requests.forEach(req => {
      const cycleFrame = (frame + req.offset) % 180;
      const phase = cycleFrame / 90; // 0-2

      if (phase < 1) {
        // Edge → Hub
        const t = phase;
        const x = req.fromEdge.x + (req.toHub.x - req.fromEdge.x) * t;
        const y = req.fromEdge.y - 10 + (req.toHub.y + 12 - (req.fromEdge.y - 10)) * t;
        drawPacket(x, y, req.emoji, req.color, 4);
      }
    });

    // Legend at bottom - two clear rows
    const legendY = 250;

    // DOWN section (left half)
    svg
      .append('text')
      .attr('x', 70)
      .attr('y', legendY - 15)
      .attr('font-size', 9)
      .attr('font-weight', 600)
      .attr('fill', theme.colors.inkMedium)
      .text('↓ DATA DOWN');

    const downItems = [
      { x: 70, label: '📦 Chunk', color: '#f59e0b' },
      { x: 170, label: '🧠 Model', color: '#8b5cf6' },
      { x: 270, label: '⚡ State', color: '#06b6d4' },
    ];
    downItems.forEach(item => {
      svg
        .append('circle')
        .attr('cx', item.x)
        .attr('cy', legendY)
        .attr('r', 4)
        .attr('fill', item.color);
      svg
        .append('text')
        .attr('x', item.x + 10)
        .attr('y', legendY + 4)
        .attr('font-size', 8)
        .attr('fill', theme.colors.inkMedium)
        .text(item.label);
    });

    // UP section (right half)
    svg
      .append('text')
      .attr('x', 400)
      .attr('y', legendY - 15)
      .attr('font-size', 9)
      .attr('font-weight', 600)
      .attr('fill', theme.colors.inkMedium)
      .text('↑ REQUESTS UP');

    const upItems = [
      { x: 400, label: '❓ Query', color: '#10b981' },
      { x: 490, label: '🔄 Sync', color: '#3b82f6' },
      { x: 580, label: '💰 Credit', color: '#eab308' },
    ];
    upItems.forEach(item => {
      svg
        .append('circle')
        .attr('cx', item.x)
        .attr('cy', legendY)
        .attr('r', 4)
        .attr('fill', item.color);
      svg
        .append('text')
        .attr('x', item.x + 10)
        .attr('y', legendY + 4)
        .attr('font-size', 8)
        .attr('fill', theme.colors.inkMedium)
        .text(item.label);
    });
  }, [theme, frame]);

  return <svg ref={svgRef} viewBox="0 0 700 280" style={{ width: '100%', height: 'auto' }} />;
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: GOSSIP PROTOCOL (Properly Spaced & Centered)
// ────────────────────────────────────────────────────────────────────────────
function GossipDiagram() {
  const svgRef = useRef<SVGSVGElement>(null);
  const theme = useTheme();
  const [frame, setFrame] = useState(0);
  const animationRef = useRef<number | null>(null);

  useEffect(() => {
    const animate = () => {
      setFrame(f => (f + 1) % 300);
      animationRef.current = requestAnimationFrame(animate);
    };
    animationRef.current = requestAnimationFrame(animate);
    return () => {
      if (animationRef.current) cancelAnimationFrame(animationRef.current);
    };
  }, []);

  useEffect(() => {
    if (!svgRef.current) return;

    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const width = 700;
    const centerX = width / 2;
    const ringCenterY = 130; // Pushed down for more space

    // Nodes - 8 nodes in a wider ellipse for better spacing
    const nodeCount = 8;
    const rxRadius = 200; // horizontal - wider
    const ryRadius = 80; // vertical - taller
    const nodes: { x: number; y: number; index: number; informed: boolean; informedAt: number }[] =
      [];

    const getInformedFrame = (nodeIndex: number): number => {
      if (nodeIndex === 0) return 0;
      const distance = Math.min(nodeIndex, nodeCount - nodeIndex);
      return distance * 40;
    };

    for (let i = 0; i < nodeCount; i++) {
      const angle = (i / nodeCount) * Math.PI * 2 - Math.PI / 2;
      const informedAt = getInformedFrame(i);
      nodes.push({
        x: centerX + Math.cos(angle) * rxRadius,
        y: ringCenterY + Math.sin(angle) * ryRadius,
        index: i,
        informed: frame >= informedAt,
        informedAt,
      });
    }

    // Draw connection lines (mesh) - connect each to 2 neighbors
    for (let i = 0; i < nodeCount; i++) {
      for (let offset = 1; offset <= 2; offset++) {
        const next = (i + offset) % nodeCount;
        const isActive =
          (nodes[i].informed &&
            !nodes[next].informed &&
            frame >= nodes[i].informedAt &&
            frame < nodes[i].informedAt + 40) ||
          (nodes[next].informed &&
            !nodes[i].informed &&
            frame >= nodes[next].informedAt &&
            frame < nodes[next].informedAt + 40);

        svg
          .append('line')
          .attr('x1', nodes[i].x)
          .attr('y1', nodes[i].y)
          .attr('x2', nodes[next].x)
          .attr('y2', nodes[next].y)
          .attr('stroke', isActive ? 'rgba(249, 115, 22, 0.5)' : 'rgba(139, 92, 246, 0.1)')
          .attr('stroke-width', isActive ? 2 : 1);
      }
    }

    // Draw nodes
    nodes.forEach(node => {
      const recentlyInformed = node.informed && frame - node.informedAt < 30;

      // Glow effect
      if (recentlyInformed) {
        svg
          .append('circle')
          .attr('cx', node.x)
          .attr('cy', node.y)
          .attr('r', 24)
          .attr('fill', 'none')
          .attr('stroke', 'rgba(249, 115, 22, 0.35)')
          .attr('stroke-width', 3);
      }

      // Node circle
      svg
        .append('circle')
        .attr('cx', node.x)
        .attr('cy', node.y)
        .attr('r', 18)
        .attr('fill', node.informed ? 'rgba(22, 163, 74, 0.15)' : 'rgba(200, 200, 200, 0.12)')
        .attr('stroke', node.informed ? '#16a34a' : '#9ca3af')
        .attr('stroke-width', 2);

      // Node number
      svg
        .append('text')
        .attr('x', node.x)
        .attr('y', node.y + 5)
        .attr('text-anchor', 'middle')
        .attr('font-size', 11)
        .attr('font-weight', 600)
        .attr('fill', node.informed ? '#16a34a' : '#9ca3af')
        .text(node.index);
    });

    // Animated gossip packets
    nodes.forEach(source => {
      if (!source.informed) return;

      [1, -1].forEach(offset => {
        const targetIndex = (source.index + offset + nodeCount) % nodeCount;
        const target = nodes[targetIndex];
        const travelStart = source.informedAt + 8;
        const travelEnd = target.informedAt;

        if (frame >= travelStart && frame < travelEnd && !target.informed) {
          const progress = (frame - travelStart) / (travelEnd - travelStart);
          if (progress >= 0 && progress <= 1) {
            const px = source.x + (target.x - source.x) * progress;
            const py = source.y + (target.y - source.y) * progress;

            svg
              .append('circle')
              .attr('cx', px)
              .attr('cy', py)
              .attr('r', 7)
              .attr('fill', '#f97316')
              .attr('stroke', 'white')
              .attr('stroke-width', 1.5);
            svg
              .append('text')
              .attr('x', px)
              .attr('y', py + 4)
              .attr('text-anchor', 'middle')
              .attr('font-size', 8)
              .text('📨');
          }
        }
      });
    });

    // Stats section - CENTERED below ring with MUCH more space
    const statsY = 260; // Pushed way down to avoid ring
    const informedCount = nodes.filter(n => n.informed).length;

    // Counter text - centered
    svg
      .append('text')
      .attr('x', centerX)
      .attr('y', statsY)
      .attr('text-anchor', 'middle')
      .attr('font-size', 11)
      .attr('font-weight', 600)
      .attr('fill', '#16a34a')
      .text(`Nodes informed: ${informedCount} / ${nodeCount}`);

    // Progress bar - centered
    const barWidth = 200;
    const barX = centerX - barWidth / 2;
    svg
      .append('rect')
      .attr('x', barX)
      .attr('y', statsY + 10)
      .attr('width', barWidth)
      .attr('height', 10)
      .attr('rx', 5)
      .attr('fill', 'rgba(200, 200, 200, 0.25)');
    svg
      .append('rect')
      .attr('x', barX)
      .attr('y', statsY + 10)
      .attr('width', (informedCount / nodeCount) * barWidth)
      .attr('height', 10)
      .attr('rx', 5)
      .attr('fill', '#16a34a');

    // Legend - centered at bottom with more space
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
  }, [theme, frame]);

  return <svg ref={svgRef} viewBox="0 0 700 340" style={{ width: '100%', height: 'auto' }} />;
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
