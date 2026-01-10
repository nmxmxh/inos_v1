/**
 * INOS Technical Codex — Deep Dive: The Persistence Paradox
 *
 * A deep exploration of the INOS storage backbone.
 * Focusing on 1MB Hashing, Double Compression, and Tiered Persistence.
 */

import { useEffect, useRef } from 'react';
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
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }

    p {
      margin: 0 0 ${p => p.theme.spacing[3]} 0;
      line-height: 1.6;
    }

    p:last-child {
      margin-bottom: 0;
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
    background: rgba(16, 185, 129, 0.08); /* Green tint for database */
    backdrop-filter: blur(12px);
    border: 1px solid rgba(16, 185, 129, 0.2);
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[6]} 0;

    h4 {
      margin: 0 0 ${p => p.theme.spacing[2]} 0;
      color: #059669;
      font-size: ${p => p.theme.fontSizes.lg};
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }

    p {
      margin: 0;
      line-height: 1.7;
    }

    code {
      background: rgba(16, 185, 129, 0.1);
      padding: 2px 6px;
      border-radius: 3px;
      font-family: 'JetBrains Mono', monospace;
      font-size: 0.9em;
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
};

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATIONS
// ────────────────────────────────────────────────────────────────────────────

function SabMemoryMap() {
  const svgRef = useRef<SVGSVGElement>(null);

  useEffect(() => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const width = 700;
    const height = 240;
    const margin = { top: 40, right: 40, bottom: 40, left: 40 };

    const g = svg.append('g');

    // Draw SAB Container
    const containerW = width - margin.left - margin.right;
    const containerH = height - margin.top - margin.bottom;

    g.append('rect')
      .attr('x', margin.left)
      .attr('y', margin.top)
      .attr('width', containerW)
      .attr('height', containerH)
      .attr('rx', 12)
      .attr('fill', '#f8fafc')
      .attr('stroke', '#cbd5e1')
      .attr('stroke-width', 2);

    g.append('text')
      .attr('x', width / 2)
      .attr('y', 25)
      .attr('text-anchor', 'middle')
      .attr('font-size', 10)
      .attr('font-weight', 800)
      .attr('fill', '#64748b')
      .text('SHARED ARRAY BUFFER (HOT CACHE — 1024 SLOTS)');

    // Draw grid of slots
    const rows = 4;
    const cols = 16;
    const slotW = 32;
    const slotH = 20;
    const gap = 6;

    // Calculate start positions to center the grid within the rect
    const gridW = cols * slotW + (cols - 1) * gap;
    const gridH = rows * slotH + (rows - 1) * gap;
    const startX = margin.left + (containerW - gridW) / 2;
    const startY = margin.top + (containerH - gridH) / 2;

    for (let r = 0; r < rows; r++) {
      for (let c = 0; c < cols; c++) {
        const x = startX + c * (slotW + gap);
        const y = startY + r * (slotH + gap);
        const isHot = Math.random() > 0.7;

        const slot = g
          .append('rect')
          .attr('x', x)
          .attr('y', y)
          .attr('width', slotW)
          .attr('height', slotH)
          .attr('rx', 2)
          .attr('fill', isHot ? '#ef4444' : '#e2e8f0')
          .attr('opacity', isHot ? 0.6 : 0.3);

        if (isHot) {
          slot
            .append('animate')
            .attr('attributeName', 'opacity')
            .attr('values', '0.6;1;0.6')
            .attr('dur', `${1 + Math.random() * 2}s`)
            .attr('repeatCount', 'indefinite');
        }
      }
    }

    // Legend
    svg.append('circle').attr('cx', 550).attr('cy', 215).attr('r', 4).attr('fill', '#ef4444');
    svg
      .append('text')
      .attr('x', 560)
      .attr('y', 219)
      .attr('font-size', 9)
      .attr('fill', '#64748b')
      .text('Active Pattern Slot');

    svg.append('circle').attr('cx', 430).attr('cy', 215).attr('r', 4).attr('fill', '#e2e8f0');
    svg
      .append('text')
      .attr('x', 440)
      .attr('y', 219)
      .attr('font-size', 9)
      .attr('fill', '#64748b')
      .text('Free/Stale Memory');
  }, []);

  return <svg ref={svgRef} viewBox="0 0 700 240" style={{ width: '100%', height: 'auto' }} />;
}

function Blake3HashingDiagram() {
  const svgRef = useRef<SVGSVGElement>(null);
  const theme = useTheme();

  useEffect(() => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const width = 700;

    // Draw Merkle Tree for BLAKE3
    const levels = 3;
    const nodesAtLevel = [4, 2, 1];
    const nodeRadius = 16;
    const startY = 240;
    const gapY = 70;

    const treeG = svg.append('g').attr('transform', 'translate(0, 20)');

    // Nodes and Lines
    const nodePositions: { x: number; y: number }[][] = [];

    for (let l = 0; l < levels; l++) {
      const levelNodes: { x: number; y: number }[] = [];
      const count = nodesAtLevel[l];
      const gapX = width / (count + 1);

      for (let i = 0; i < count; i++) {
        const x = gapX * (i + 1);
        const y = startY - l * gapY;
        levelNodes.push({ x, y });

        if (l > 0) {
          const child1 = nodePositions[l - 1][i * 2];
          const child2 = nodePositions[l - 1][i * 2 + 1];

          treeG
            .append('line')
            .attr('x1', x)
            .attr('y1', y)
            .attr('x2', child1.x)
            .attr('y2', child1.y)
            .attr('stroke', theme.colors.borderSubtle)
            .attr('stroke-width', 2);

          treeG
            .append('line')
            .attr('x1', x)
            .attr('y1', y)
            .attr('x2', child2.x)
            .attr('y2', child2.y)
            .attr('stroke', theme.colors.borderSubtle)
            .attr('stroke-width', 2);
        }

        const color = l === 2 ? '#10b981' : l === 1 ? '#3b82f6' : theme.colors.borderSubtle;
        treeG
          .append('circle')
          .attr('cx', x)
          .attr('cy', y)
          .attr('r', nodeRadius)
          .attr('fill', 'white')
          .attr('stroke', color)
          .attr('stroke-width', 2);

        treeG
          .append('text')
          .attr('x', x)
          .attr('y', y + 4)
          .attr('text-anchor', 'middle')
          .attr('font-size', 8)
          .attr('font-family', 'JetBrains Mono')
          .attr('font-weight', 800)
          .attr('fill', color)
          .text(l === 2 ? 'ROOT' : 'HASH');
      }
      nodePositions.push(levelNodes);
    }

    // Input Chunks (1MB Pulses)
    const dataChunks = ['Pulse A', 'Pulse B', 'Pulse C', 'Pulse D'];
    nodePositions[0].forEach((pos, i) => {
      const g = treeG.append('g');
      g.append('rect')
        .attr('x', pos.x - 25)
        .attr('y', pos.y + 30)
        .attr('width', 50)
        .attr('height', 20)
        .attr('rx', 4)
        .attr('fill', '#f1f5f9')
        .attr('stroke', theme.colors.borderSubtle);

      g.append('text')
        .attr('x', pos.x)
        .attr('y', pos.y + 43)
        .attr('text-anchor', 'middle')
        .attr('font-size', 8)
        .attr('fill', theme.colors.inkMedium)
        .text(dataChunks[i]);

      // Pulse animation
      g.append('circle')
        .attr('cx', pos.x)
        .attr('cy', pos.y + 40)
        .attr('r', 2)
        .attr('fill', '#ef4444')
        .attr('opacity', 0)
        .append('animate')
        .attr('attributeName', 'opacity')
        .attr('values', '0;1;0')
        .attr('dur', '2s')
        .attr('begin', `${i * 0.5}s`)
        .attr('repeatCount', 'indefinite');
    });

    svg
      .append('text')
      .attr('x', width / 2)
      .attr('y', 20)
      .attr('text-anchor', 'middle')
      .attr('font-size', 10)
      .attr('font-weight', 800)
      .attr('fill', theme.colors.inkMedium)
      .text('1MB HASHING HEARTBEAT (BLAKE3 MERKLE TREE)');
  }, [theme]);

  return <svg ref={svgRef} viewBox="0 0 700 320" style={{ width: '100%', height: 'auto' }} />;
}

function SyncAccessPipeline() {
  const svgRef = useRef<SVGSVGElement>(null);

  useEffect(() => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    // Main Thread (Blocked Area)
    svg
      .append('rect')
      .attr('x', 50)
      .attr('y', 50)
      .attr('width', 150)
      .attr('height', 160)
      .attr('rx', 8)
      .attr('fill', '#f1f5f9')
      .attr('stroke', '#cbd5e1');
    svg
      .append('text')
      .attr('x', 125)
      .attr('y', 40)
      .attr('text-anchor', 'middle')
      .attr('font-size', 10)
      .attr('font-weight', 800)
      .attr('fill', '#64748b')
      .text('MAIN THREAD (UI)');

    // Web Worker (Active Area)
    svg
      .append('rect')
      .attr('x', 250)
      .attr('y', 50)
      .attr('width', 200)
      .attr('height', 160)
      .attr('rx', 8)
      .attr('fill', '#ecfdf5')
      .attr('stroke', '#10b981');
    svg
      .append('text')
      .attr('x', 350)
      .attr('y', 40)
      .attr('text-anchor', 'middle')
      .attr('font-size', 10)
      .attr('font-weight', 800)
      .attr('fill', '#059669')
      .text('DEDICATED WORKER (STORAGE)');

    // OPFS (Storage Area)
    const opfsX = 520;
    svg
      .append('rect')
      .attr('x', opfsX)
      .attr('y', 50)
      .attr('width', 130)
      .attr('height', 160)
      .attr('rx', 8)
      .attr('fill', '#eff6ff')
      .attr('stroke', '#3b82f6');
    svg
      .append('text')
      .attr('x', opfsX + 65)
      .attr('y', 40)
      .attr('text-anchor', 'middle')
      .attr('font-size', 10)
      .attr('font-weight', 800)
      .attr('fill', '#1d4ed8')
      .text('OPFS (DISK)');

    // Worker Script
    svg
      .append('rect')
      .attr('x', 265)
      .attr('y', 80)
      .attr('width', 170)
      .attr('height', 100)
      .attr('rx', 4)
      .attr('fill', '#059669')
      .attr('opacity', 0.1);
    svg
      .append('text')
      .attr('x', 350)
      .attr('y', 110)
      .attr('text-anchor', 'middle')
      .attr('font-size', 12)
      .attr('font-weight', 700)
      .attr('fill', '#059669')
      .text('SQLite WASM');

    // Visualizing the "Blockage" (Main thread is busy with UI)
    const blockG = svg.append('g');
    for (let i = 0; i < 5; i++) {
      const line = blockG
        .append('rect')
        .attr('x', 70)
        .attr('y', 70 + i * 25)
        .attr('width', 110)
        .attr('height', 15)
        .attr('rx', 2)
        .attr('fill', '#e2e8f0');

      // Minor pulse to show main thread "jitter"
      line
        .append('animate')
        .attr('attributeName', 'opacity')
        .attr('values', '1;0.6;1')
        .attr('dur', `${1 + i * 0.2}s`)
        .attr('repeatCount', 'indefinite');
    }

    // Direct Sync Pipe
    svg
      .append('path')
      .attr('d', `M450,130 L520,130`)
      .attr('stroke', '#3b82f6')
      .attr('stroke-width', 4)
      .attr('stroke-dasharray', '8,4');

    svg
      .append('text')
      .attr('x', 485)
      .attr('y', 120)
      .attr('text-anchor', 'middle')
      .attr('font-size', 8)
      .attr('font-weight', 800)
      .attr('fill', '#3b82f6')
      .text('SYNC');

    // Animated Bytes - Continuous flow
    const packetCount = 4;
    for (let i = 0; i < packetCount; i++) {
      const circle = svg.append('circle').attr('r', 4).attr('fill', '#3b82f6');
      circle
        .append('animateMotion')
        .attr('path', `M450,130 L520,130`)
        .attr('dur', '1.2s')
        .attr('begin', `${i * 0.3}s`)
        .attr('repeatCount', 'indefinite');

      circle
        .append('animate')
        .attr('attributeName', 'opacity')
        .attr('values', '0;1;1;0')
        .attr('keyTimes', '0;0.1;0.9;1')
        .attr('dur', '1.2s')
        .attr('repeatCount', 'indefinite');
    }
  }, []);

  return <svg ref={svgRef} viewBox="0 0 700 240" style={{ width: '100%', height: 'auto' }} />;
}

function DhtMeshMap() {
  const svgRef = useRef<SVGSVGElement>(null);

  useEffect(() => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const width = 700;
    const height = 300;
    const centerX = width / 2;
    const centerY = height / 2;
    const radius = 100;

    // Draw Hash Space Circle
    svg
      .append('circle')
      .attr('cx', centerX)
      .attr('cy', centerY)
      .attr('r', radius)
      .attr('fill', 'none')
      .attr('stroke', '#cbd5e1')
      .attr('stroke-width', 1)
      .attr('stroke-dasharray', '4,4');

    // Draw Peers on the circle
    const peerCount = 8;
    const peers = d3.range(peerCount).map(i => {
      const angle = (i / peerCount) * 2 * Math.PI;
      return {
        x: centerX + radius * Math.cos(angle),
        y: centerY + radius * Math.sin(angle),
        id: Math.floor(Math.random() * 255),
      };
    });

    // Mesh lines
    peers.forEach((p1, i) => {
      peers.forEach((p2, j) => {
        if (i < j && (j === i + 1 || j === i + 2 || (i === 0 && j === peerCount - 1))) {
          svg
            .append('line')
            .attr('x1', p1.x)
            .attr('y1', p1.y)
            .attr('x2', p2.x)
            .attr('y2', p2.y)
            .attr('stroke', '#3b82f6')
            .attr('stroke-width', 1)
            .attr('opacity', 0.3);
        }
      });
    });

    peers.forEach(p => {
      const g = svg.append('g');
      g.append('circle').attr('cx', p.x).attr('cy', p.y).attr('r', 6).attr('fill', '#3b82f6');

      g.append('text')
        .attr('x', p.x)
        .attr('y', p.y - 12)
        .attr('text-anchor', 'middle')
        .attr('font-size', 8)
        .attr('font-family', 'JetBrains Mono')
        .attr('fill', '#1e40af')
        .text(`Node ${p.id}`);
    });

    // Animate Chunk Transfer
    const animateChunk = () => {
      const p1 = peers[Math.floor(Math.random() * peerCount)];
      const p2 = peers[Math.floor(Math.random() * peerCount)];
      if (p1 === p2) return animateChunk();

      const chunk = svg.append('circle').attr('r', 3).attr('fill', '#10b981').attr('opacity', 0);

      chunk
        .attr('cx', p1.x)
        .attr('cy', p1.y)
        .transition()
        .duration(1500)
        .attr('cx', p2.x)
        .attr('cy', p2.y)
        .style('opacity', 1)
        .transition()
        .duration(500)
        .style('opacity', 0)
        .on('end', function () {
          d3.select(this).remove();
        });
    };

    const interval = setInterval(animateChunk, 2000);
    return () => clearInterval(interval);
  }, []);

  return <svg ref={svgRef} viewBox="0 0 700 300" style={{ width: '100%', height: 'auto' }} />;
}

function TieredConvergenceMap() {
  const svgRef = useRef<SVGSVGElement>(null);
  const theme = useTheme();

  useEffect(() => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const tiers = [
      {
        id: 'sab',
        label: 'HOT: SAB',
        color: '#ef4444',
        x: 50,
        y: 120,
        w: 120,
        h: 60,
        desc: 'Pattern Cache',
      },
      {
        id: 'ram',
        label: 'WARM: ARENA',
        color: '#f59e0b',
        x: 220,
        y: 120,
        w: 120,
        h: 60,
        desc: 'LRU Heap',
      },
      {
        id: 'opfs',
        label: 'COLD: OPFS',
        color: '#3b82f6',
        x: 390,
        y: 120,
        w: 120,
        h: 60,
        desc: 'SQLite Blocks',
      },
      {
        id: 'mesh',
        label: 'ARCHIVE: MESH',
        color: '#10b981',
        x: 560,
        y: 120,
        w: 120,
        h: 60,
        desc: 'P2P Chunks',
      },
    ];

    svg
      .append('rect')
      .attr('x', 245)
      .attr('y', 20)
      .attr('width', 250)
      .attr('height', 50)
      .attr('rx', 25)
      .attr('fill', '#00add810')
      .attr('stroke', '#00add8')
      .attr('stroke-width', 2);

    svg
      .append('text')
      .attr('x', 370)
      .attr('y', 50)
      .attr('text-anchor', 'middle')
      .attr('font-size', 12)
      .attr('font-weight', 800)
      .attr('fill', '#00add8')
      .text('STORAGE SUPERVISOR (GO)');

    tiers.forEach(t => {
      const g = svg.append('g');
      g.append('rect')
        .attr('x', t.x)
        .attr('y', t.y)
        .attr('width', t.w)
        .attr('height', t.h)
        .attr('rx', 8)
        .attr('fill', t.color + '08')
        .attr('stroke', t.color)
        .attr('stroke-width', 2);
      g.append('text')
        .attr('x', t.x + t.w / 2)
        .attr('y', t.y + 25)
        .attr('text-anchor', 'middle')
        .attr('font-size', 11)
        .attr('font-weight', 800)
        .attr('fill', t.color)
        .text(t.label);
      g.append('text')
        .attr('x', t.x + t.w / 2)
        .attr('y', t.y + 45)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('fill', theme.colors.inkMedium)
        .text(t.desc);
      svg
        .append('path')
        .attr('d', `M${370},70 L${t.x + t.w / 2},${t.y}`)
        .attr('stroke', theme.colors.borderSubtle)
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '4,4');
    });

    svg
      .append('path')
      .attr('d', `M680,150 Q730,150 730,225 Q730,300 370,300 Q10,300 10,225 Q10,150 50,150`)
      .attr('fill', 'none')
      .attr('stroke', '#10b98120')
      .attr('stroke-width', 2)
      .attr('stroke-dasharray', '8,4');
  }, [theme]);

  return <svg ref={svgRef} viewBox="0 0 740 310" style={{ width: '100%', height: 'auto' }} />;
}

// ────────────────────────────────────────────────────────────────────────────
// MAIN PAGE COMPONENT
// ────────────────────────────────────────────────────────────────────────────

export function Database() {
  return (
    <Style.BlogContainer>
      <Style.SectionTitle>Deep Dive</Style.SectionTitle>
      <Style.PageTitle>The Persistence Paradox</Style.PageTitle>
      <Style.LeadParagraph>
        Data is heavy, slow, and expensive to move. Yet, in the modern browser, we ask it to be
        instant, immutable, and globally available. This is the paradox INOS solves through tiered
        convergence.
      </Style.LeadParagraph>

      <Style.SectionDivider />

      {/* Prologue */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Prologue: A Decade of Ghost Writes</h3>
          <p>
            For ten years, the web was a graveyard of storage expectations. LocalStorage was too
            small (5MB). WebSQL was deprecated almost as soon as it arrived. IndexedDB, while
            powerful, became known for its "Promise Tax"—an asynchronous overhead that made
            nanosecond concurrency impossible.
          </p>

          <Style.HistoryCard>
            <h4>The Storage Lineage</h4>
            <p>
              Before the modern era, web apps relied on a chaotic mix of technologies.
              <strong> LocalStorage</strong> (2009) was first, but synchronized poorly.
              <strong> WebSQL</strong> (2010) was an attempt to put SQLite in the browser, but it
              was eventually abandoned. <strong> IndexedDB</strong> (2011) was the savior that
              turned into a labyrinth of callbacks.
            </p>
          </Style.HistoryCard>

          <p>
            INOS rejects the choice between "Synchronous but Volatile" and "Persistent but Slow." It
            builds a bridge using SharedArrayBuffer (SAB) for the hot tier and the Origin Private
            File System (OPFS) for the cold tier.
          </p>

          <Style.ComparisonGrid>
            <Style.ComparisonCard $type="bad">
              <h4>The Copy Tax</h4>
              <p>
                Traditional apps copy data from the DB to the Worker, then to the Main Thread. Each
                hop burns CPU and duplicates memory.
              </p>
            </Style.ComparisonCard>
            <Style.ComparisonCard $type="good">
              <h4>Zero-Copy Persistence</h4>
              <p>
                INOS writes once to a shared memory region. All modules read from the same bytes.
                Persistence is an asynchronous heartbeat, not a blocker.
              </p>
            </Style.ComparisonCard>
          </Style.ComparisonGrid>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.SectionDivider />

      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 1: Tier 1 - The Circulatory System</h3>
          <p>
            The fastest storage is the one that never waits. In INOS, the **Hot Tier** is a region
            of the SharedArrayBuffer called the <strong>Pattern Exchange</strong>.
          </p>
          <p>
            Whenever the Kernel (Go) or a Muscle (Rust) discovers a data pattern, it is stored
            directly in one of the 1024 shared slots. There are no locks, only atomic signals.
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>Internal Registry // Tier 1</Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <SabMemoryMap />
            <Style.IllustrationCaption>
              Nanosecond access to the shared pattern cache. Slots are updated via Atomics.store.
            </Style.IllustrationCaption>
          </Style.IllustrationContainer>

          <Style.DefinitionBox>
            <h4>HotPatternCache</h4>
            <p>
              Implemented in <code>storage.go</code>, this cache allows the system to recall
              frequently used data without ever touching the disk. It is the "circulatory system" of
              INOS, pumping bits at 100ns latency.
            </p>
          </Style.DefinitionBox>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.SectionDivider />

      {/* Lesson 2 */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 2: Tier 2 & 3 - Long-Term Memory</h3>
          <p>
            While SAB is the circulatory system, <strong>OPFS (Origin Private File System)</strong>
            is the system's long-term memory. Unlike IndexedDB, which is asynchronous and uses an
            object-store model, OPFS provides a real filesystem interface with
            <strong> Synchronous Access Handles</strong>.
          </p>
          <p>
            By running the storage engine in a <strong>Web Worker</strong>, we can perform
            synchronous reads and writes that bypass the main thread's Event Loop. This is where
            <strong> SQLite WASM</strong> lives, utilizing the OPFS VFS (Virtual File System) to
            achieve near-native I/O performance.
          </p>

          <Style.ComparisonGrid>
            <Style.ComparisonCard $type="bad">
              <h4>IndexedDB Latency</h4>
              <p>
                Every read/write requires a Promise. In a heavy compute loop, the 1-2ms delay per
                call adds up to an insurmountable bottleneck.
              </p>
            </Style.ComparisonCard>
            <Style.ComparisonCard $type="good">
              <h4>OPFS Sync Handles</h4>
              <p>
                A dedicated worker opens a direct binary stream to the disk. I/O is blocked for
                microseconds, not milliseconds. Native-grade persistence.
              </p>
            </Style.ComparisonCard>
          </Style.ComparisonGrid>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>
                Synchronous I/O Pipeline // Tier 2 & 3
              </Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <SyncAccessPipeline />
            <Style.IllustrationCaption>
              Direct, synchronous communication between the SQLite WASM worker and the OPFS disk.
            </Style.IllustrationCaption>
          </Style.IllustrationContainer>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.SectionDivider />

      {/* Lesson 3 */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 3: The 1MB Hashing Heartbeat</h3>
          <p>
            In a distributed world, filenames are irrelevant. INOS treats data as a stream of
            <strong> 1MB pulses</strong>. Each pulse is hashed using BLAKE3, creating a
            cryptographic fingerprint that serves as its unique address in the universe.
          </p>
          <p>
            This <strong>1MB Chunking</strong> is the heart of the system. It allows for perfect
            granularity: if 1 byte changes in a 1GB file, only one 1MB pulse needs to be re-verified
            and re-synchronized.
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>
                BLAKE3 Merkle Hashing // Content-Addressability
              </Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <Blake3HashingDiagram />
            <Style.IllustrationCaption>
              Data blocks are pulsed at 1MB intervals, hashed, and combined into a Merkle root.
            </Style.IllustrationCaption>
          </Style.IllustrationContainer>

          <Style.DefinitionBox>
            <h4>Global Deduplication</h4>
            <p>
              Because the hash is the address, if two users save the same 1MB block, the mesh
              identifies them as identical. INOS only stores it once, saving petabytes of redundant
              data across the network.
            </p>
          </Style.DefinitionBox>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.SectionDivider />

      {/* Lesson 4 */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 4: Double Compression Alchemy</h3>
          <p>
            Storage is a trade-off between speed and density. INOS employs a two-pass compression
            strategy known as **Double Compression**.
          </p>

          <Style.Timeline>
            <Style.TimelineItem>
              <Style.TimelineYear>Pass 1</Style.TimelineYear>
              <strong>Brotli-Fast (Q=6)</strong>: Optimized for Ingress. Data is compressed
              instantly as it enters the system to save bandwidth with minimal CPU hit.
            </Style.TimelineItem>
            <Style.TimelineItem>
              <Style.TimelineYear>Pass 2</Style.TimelineYear>
              <strong>Brotli-Max (Q=11)</strong>: Optimized for Storage. When data settles into the
              Cold Tier, it is re-compressed at maximum density, shrinking its footprint by an
              additional 20%.
            </Style.TimelineItem>
          </Style.Timeline>

          <p>
            This "Alchemy" ensures that the network is always fast, but the disk is always
            efficient.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.SectionDivider />

      {/* Lesson 5 */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 5: The Global Mesh Archive</h3>
          <p>
            When data is no longer needed on your local machine, it is archived into the
            <strong> Global Mesh</strong>. This is a Peer-to-Peer swarm where 256 sectors (0-255)
            collaborate to store the world's data.
          </p>
          <p>
            Availability is guaranteed by a **Dynamic Replication Factor (RF)**. Standard data is
            mirrored on 3 nodes (RF=3). Viral patterns or critical system assets scale automatically
            up to **RF=50**, creating a self-healing, high-bandwidth CDN distributed across the
            globe.
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>
                DHT Mesh Topology // 256 Hash Sectors
              </Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <DhtMeshMap />
            <Style.IllustrationCaption>
              Nodes assigned to sectors (0-255). Replication scaling from RF=3 to RF=50 based on
              demand.
            </Style.IllustrationCaption>
          </Style.IllustrationContainer>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.SectionDivider />

      {/* Conclusion */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Conclusion: The Converged Backbone</h3>
          <p>
            From the nanosecond pulses of Tier 1 Shared Memory to the deep-cold storage of the
            Global Mesh, INOS provides a singular interface for all data. We've bridged the
            "Persistence Paradox"—building a system that is as fast as RAM, as vast as the cloud,
            and as private as your own machine.
          </p>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>
                Converged Architecture // Final Topology
              </Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <TieredConvergenceMap />
            <Style.IllustrationCaption>
              A unified storage stack managed by the StorageSupervisor.
            </Style.IllustrationCaption>
          </Style.IllustrationContainer>

          <p>
            The web is no longer a graveyard of storage expectations. It is a world-scale database.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <ChapterNav
        prev={{ to: '/deep-dives/graphics', title: 'Graphics Pipeline' }}
        next={{ to: '/cosmos', title: '04. The Cosmos' }}
      />
    </Style.BlogContainer>
  );
}

export default Database;
