import D3Container from '../../ui/D3Container';
import { useCallback, useState } from 'react';
import styled, { useTheme } from 'styled-components';
import * as d3 from 'd3';

const Style = {
  StoryContainer: styled.div`
    width: 100%;
    height: 640px;
    position: relative;
    overflow: hidden;
  `,
  Overlay: styled.div`
    position: absolute;
    bottom: 24px;
    left: 24px;
    right: 24px;
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 11px;
    color: ${p => p.theme.colors.inkMedium};
    line-height: 1.3;
    pointer-events: none;
    z-index: 10;
    display: flex;
    justify-content: space-between;
    align-items: flex-end;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      flex-direction: column;
      align-items: flex-start;
      gap: 12px;
    }
  `,
  Description: styled.div`
    max-width: 480px;
    font-weight: 500;
    color: ${p => p.theme.colors.inkDark};
    strong {
      font-weight: 700;
      color: ${p => p.theme.colors.inkDark};
    }
  `,
  Status: styled.div`
    text-align: right;
    font-weight: bold;
    color: ${p => p.theme.colors.accent};
    font-family: ${p => p.theme.fonts.typewriter};
    letter-spacing: 0.05em;
  `,
};

interface StoryNode {
  id: string;
  label: string;
  x: number;
  y: number;
  type: string;
  desc: string;
}

export function DimostrazioneStory() {
  const theme = useTheme();
  const [activeBuffer, setActiveBuffer] = useState<'A' | 'B'>('A');

  const renderViz = useCallback(
    (svg: d3.Selection<SVGSVGElement, unknown, null, undefined>, _domW: number, _domH: number) => {
      svg.selectAll('*').remove();

      // Fixed virtual dimensions
      const width = 800;
      const height = 700;

      // Final balanced sizing (Tiny bit smaller)
      const nodes: StoryNode[] = [
        {
          id: 'SAB',
          label: 'SAB HUB',
          x: width / 2,
          y: height / 2,
          type: 'core',
          desc: 'Absolute Memory Space',
        },
        {
          id: 'GO',
          label: 'GO ORCHESTRATOR',
          x: width / 2 - 250,
          y: height / 2 - 160,
          type: 'brain',
          desc: 'Policy Control',
        },
        {
          id: 'RUST',
          label: 'RUST ENGINE',
          x: width / 2 + 250,
          y: height / 2 - 160,
          type: 'muscle',
          desc: 'WASM Physics',
        },
        {
          id: 'TS',
          label: 'TS SENSORY LAYER',
          x: width / 2,
          y: height / 2 + 200,
          type: 'vision',
          desc: 'Concurrent Rendering',
        },
      ];

      // Filters
      const defs = svg.append('defs');
      const filter = defs.append('filter').attr('id', 'glow');
      filter.append('feGaussianBlur').attr('stdDeviation', '3').attr('result', 'coloredBlur'); // Reduced glow
      const feMerge = filter.append('feMerge');
      feMerge.append('feMergeNode').attr('in', 'coloredBlur');
      feMerge.append('feMergeNode').attr('in', 'SourceGraphic');

      // Links
      const links = [
        { source: 'SAB', target: 'GO' },
        { source: 'SAB', target: 'RUST' },
        { source: 'SAB', target: 'TS' },
      ];

      svg
        .append('g')
        .selectAll('line')
        .data(links)
        .enter()
        .append('line')
        .attr('x1', d => nodes.find(n => n.id === d.source)?.x ?? 0)
        .attr('y1', d => nodes.find(n => n.id === d.source)?.y ?? 0)
        .attr('x2', d => nodes.find(n => n.id === d.target)?.x ?? 0)
        .attr('y2', d => nodes.find(n => n.id === d.target)?.y ?? 0)
        .attr('stroke', '#6d28d9')
        .attr('stroke-width', 1.5)
        .attr('stroke-dasharray', '6,8')
        .attr('opacity', 0.15);

      // Build Memory Lattice (Compact Size)
      const gridSize = 10;
      const spacing = 15;
      const totalGridW = (gridSize - 1) * spacing;
      const padding = 18;
      const hubSize = totalGridW + padding * 2;

      const latticeContainer = svg
        .append('g')
        .attr('transform', `translate(${width / 2 - hubSize / 2}, ${height / 2 - hubSize / 2})`);

      // Hub Outer Container
      latticeContainer
        .append('rect')
        .attr('width', hubSize)
        .attr('height', hubSize)
        .attr('rx', 14)
        .attr('fill', 'rgba(109, 40, 217, 0.02)')
        .attr('stroke', '#6d28d9')
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '4,4')
        .attr('opacity', 0.5);

      const latticePoints = [];
      for (let i = 0; i < gridSize; i++) {
        for (let j = 0; j < gridSize; j++) {
          latticePoints.push({
            id: `p-${i}-${j}`,
            x: padding + i * spacing,
            y: padding + j * spacing,
            row: i,
            col: j,
          });
        }
      }

      const points = latticeContainer
        .selectAll('circle.lattice-point')
        .data(latticePoints)
        .enter()
        .append('circle')
        .attr('class', 'lattice-point')
        .attr('cx', d => d.x)
        .attr('cy', d => d.y)
        .attr('r', 1.0)
        .attr('fill', '#6d28d9')
        .attr('opacity', 0.15);

      // Buffer Banks
      const halfSize = (hubSize - padding * 3) / 2;

      const bankA = latticeContainer
        .append('rect')
        .attr('width', halfSize + padding)
        .attr('height', hubSize - padding)
        .attr('x', padding / 2)
        .attr('y', padding / 2)
        .attr('rx', 8)
        .attr('fill', 'rgba(109, 40, 217, 0.05)')
        .attr('stroke', '#6d28d9')
        .attr('stroke-width', 2)
        .attr('stroke-dasharray', '4,2')
        .attr('opacity', 1);

      const bankB = latticeContainer
        .append('rect')
        .attr('width', halfSize + padding)
        .attr('height', hubSize - padding)
        .attr('x', hubSize / 2 + 2)
        .attr('y', padding / 2)
        .attr('rx', 8)
        .attr('fill', 'none')
        .attr('stroke', '#ec4899')
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '4,2')
        .attr('opacity', 0.3);

      // Correct Bank Dimensions
      bankA
        .attr('width', hubSize / 2 - 4)
        .attr('height', hubSize - 8)
        .attr('x', 4)
        .attr('y', 4);

      bankB
        .attr('width', hubSize / 2 - 4)
        .attr('height', hubSize - 8)
        .attr('x', hubSize / 2)
        .attr('y', 4);

      // Data Pulses
      const pulse = (
        fromId: string,
        toId: string,
        color: string,
        delay: number = 0,
        size: number = 2
      ) => {
        const source = nodes.find(n => n.id === fromId);
        const target = nodes.find(n => n.id === toId);
        if (!source || !target) return;

        [0, 100, 200].forEach(streamDelay => {
          const p = svg
            .append('circle')
            .attr('r', size)
            .attr('fill', color)
            .attr('cx', source.x)
            .attr('cy', source.y)
            .attr('opacity', 0);

          p.transition()
            .delay(delay + streamDelay)
            .duration(100)
            .attr('opacity', 1)
            .on('start', function () {
              if (fromId === 'SAB') createRipple(color, false);
            })
            .transition()
            .duration(1200)
            .ease(d3.easeCubicInOut)
            .attr('cx', target.x)
            .attr('cy', target.y)
            .on('end', function () {
              if (toId === 'SAB') createRipple(color, true);
            })
            .transition()
            .duration(100)
            .attr('opacity', 0)
            .remove();
        });
      };

      const createRipple = (color: string, isWrite: boolean) => {
        // Ripple Circle
        latticeContainer
          .append('circle')
          .attr('cx', hubSize / 2)
          .attr('cy', hubSize / 2)
          .attr('r', 0)
          .attr('fill', 'none')
          .attr('stroke', color)
          .attr('stroke-width', 1.5)
          .attr('opacity', 0.8)
          .transition()
          .duration(800)
          .attr('r', 50) // Reduced radius
          .attr('opacity', 0)
          .remove();

        // Active Dots
        const cx = isWrite ? gridSize * 0.75 : gridSize * 0.25;
        const cy = gridSize / 2;

        points
          .filter(d => {
            const dist = Math.sqrt(Math.pow(d.row - cx, 2) + Math.pow(d.col - cy, 2));
            return dist < 2.5;
          })
          .transition()
          .duration(200)
          .attr('r', 1.8) // Reduced active size
          .attr('opacity', 0.9)
          .transition()
          .duration(500)
          .attr('r', 1.0)
          .attr('opacity', 0.15);
      };

      // Animation Loop
      const flipTimer = d3.interval(() => {
        setActiveBuffer(prev => {
          const next = prev === 'A' ? 'B' : 'A';

          bankA
            .transition()
            .duration(800)
            .ease(d3.easeElasticOut.amplitude(1).period(0.4))
            .attr('opacity', next === 'A' ? 1 : 0.3)
            .attr('stroke-width', next === 'A' ? 2 : 1)
            .attr('fill', next === 'A' ? 'rgba(109, 40, 217, 0.08)' : 'none');

          bankB
            .transition()
            .duration(800)
            .ease(d3.easeElasticOut.amplitude(1).period(0.4))
            .attr('opacity', next === 'B' ? 1 : 0.3)
            .attr('stroke-width', next === 'B' ? 2 : 1)
            .attr('fill', next === 'B' ? 'rgba(236, 72, 153, 0.08)' : 'none');

          points
            .transition()
            .duration(300)
            .attr('opacity', 0.6)
            .attr('r', 2)
            .transition()
            .duration(300)
            .attr('r', 1.0)
            .attr('opacity', 0.15);

          return next;
        });

        pulse('RUST', 'SAB', '#8b5cf6', 0, 2);
        pulse('GO', 'SAB', '#f472b6', 400, 2);
        pulse('SAB', 'TS', '#4ade80', 800, 2);
      }, 2400);

      // Draw Nodes
      const nodeGroups = svg
        .append('g')
        .selectAll('g')
        .data(nodes)
        .enter()
        .append('g')
        .attr('transform', d => `translate(${d.x}, ${d.y})`);

      nodeGroups.each(function (d) {
        const g = d3.select(this);

        if (d.id === 'SAB') {
          // Centered SAB Label
          g.append('text')
            .attr('text-anchor', 'middle')
            .attr('dy', `${hubSize / 2 + 20}px`)
            .attr('font-family', theme.fonts.typewriter)
            .attr('font-size', '11px')
            .attr('font-weight', '700')
            .attr('fill', '#1a1a1a')
            .text(d.label);

          g.append('text')
            .attr('text-anchor', 'middle')
            .attr('dy', `${hubSize / 2 + 35}px`)
            .attr('font-family', theme.fonts.main)
            .attr('font-size', '9px')
            .attr('font-weight', '500')
            .attr('fill', '#737373')
            .text(d.desc);
        } else {
          // External Nodes - Compact
          const w = 160;
          const h = 54;
          g.append('rect')
            .attr('width', w)
            .attr('height', h)
            .attr('x', -w / 2)
            .attr('y', -h / 2)
            .attr('rx', 8)
            .attr('fill', '#ffffff')
            .attr('stroke', d.id === 'RUST' ? '#a855f7' : d.id === 'GO' ? '#f472b6' : '#22c55e')
            .attr('stroke-width', 2)
            .attr('filter', 'url(#glow)');

          g.append('text')
            .attr('text-anchor', 'middle')
            .attr('dy', '-4px')
            .attr('font-family', theme.fonts.typewriter)
            .attr('font-size', '11px')
            .attr('font-weight', '700')
            .attr('fill', '#1a1a1a')
            .text(d.label);

          g.append('text')
            .attr('text-anchor', 'middle')
            .attr('dy', '12px')
            .attr('font-family', theme.fonts.main)
            .attr('font-size', '9px')
            .attr('font-weight', '500')
            .attr('fill', '#737373')
            .text(d.desc);
        }
      });

      return () => {
        flipTimer.stop();
      };
    },
    [theme]
  ); // Correct dependency on theme

  return (
    <Style.StoryContainer>
      <D3Container
        render={renderViz}
        dependencies={[renderViz]} // Stable dependnecy
        viewBox="0 0 800 700"
        height="100%"
        preserveAspectRatio="xMidYMid meet"
      />
      <Style.Overlay>
        <Style.Description>
          <strong style={{ display: 'block', marginBottom: '6px' }}>
            The Circulatory Pipeline:
          </strong>
          INOS eliminates message-passing latency by mapping all threads to the same absolute memory
          region. The <strong>Go Orchestrator</strong> evolutionary patches, the{' '}
          <strong>Rust Engine</strong> SIMD-writes physics, and the
          <strong> TS Sensory Layer</strong> reads the results concurrently for 120fps. Atomic{' '}
          <strong>Ping-Pong buffers</strong> ensure total isolation.
        </Style.Description>
        <Style.Status>MEMORY_EPOCH_SYNC // SAB_HUB_0x{activeBuffer}PONG</Style.Status>
      </Style.Overlay>
    </Style.StoryContainer>
  );
}

export default DimostrazioneStory;
