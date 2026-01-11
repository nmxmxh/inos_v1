import D3Container from '../../ui/D3Container';
import { useCallback, useState } from 'react';
import styled, { useTheme } from 'styled-components';
import * as d3 from 'd3';

const Style = {
  StoryContainer: styled.div`
    width: 100%;
    height: 550px;
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
    (svg: d3.Selection<SVGSVGElement, unknown, null, undefined>, width: number, height: number) => {
      svg.selectAll('*').remove();

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
          y: height / 2 - 150,
          type: 'brain',
          desc: 'Policy & Evolutionary Control',
        },
        {
          id: 'RUST',
          label: 'RUST ENGINE',
          x: width / 2 + 250,
          y: height / 2 - 150,
          type: 'muscle',
          desc: 'WASM Physics & SIMD Writes',
        },
        {
          id: 'TS',
          label: 'TS SENSORY LAYER',
          x: width / 2,
          y: height / 2 + 160,
          type: 'vision',
          desc: 'Concurrent Read-Only Rendering',
        },
      ];

      // Glow filters
      const defs = svg.append('defs');
      const filter = defs.append('filter').attr('id', 'glow');
      filter.append('feGaussianBlur').attr('stdDeviation', '3.5').attr('result', 'coloredBlur');
      const feMerge = filter.append('feMerge');
      feMerge.append('feMergeNode').attr('in', 'coloredBlur');
      feMerge.append('feMergeNode').attr('in', 'SourceGraphic');

      // Links (Data Pipelines)
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
        .attr('stroke-dasharray', '4,6')
        .attr('opacity', 0.15);

      // Build Memory Lattice (The Substrate)
      const gridSize = 10;
      const spacing = 12;
      const hubSize = gridSize * spacing + 12;
      const latticeContainer = svg
        .append('g')
        .attr(
          'transform',
          `translate(${width / 2 - (gridSize * spacing) / 2}, ${height / 2 - (gridSize * spacing) / 2})`
        );

      // Hub Outer Container
      latticeContainer
        .append('rect')
        .attr('width', hubSize)
        .attr('height', hubSize)
        .attr('x', -6)
        .attr('y', -6)
        .attr('rx', 12)
        .attr('fill', 'rgba(109, 40, 217, 0.02)')
        .attr('stroke', '#6d28d9')
        .attr('stroke-width', 0.5)
        .attr('stroke-dasharray', '2,2')
        .attr('opacity', 0.4);

      const latticePoints = [];
      for (let i = 0; i < gridSize; i++) {
        for (let j = 0; j < gridSize; j++) {
          latticePoints.push({
            id: `p-${i}-${j}`,
            x: i * spacing,
            y: j * spacing,
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
        .attr('r', 1)
        .attr('fill', '#6d28d9')
        .attr('opacity', 0.2);

      // Buffer Bank Visuals (A and B zones)
      const bankA = latticeContainer
        .append('rect')
        .attr('width', (gridSize * spacing) / 2 - 4)
        .attr('height', gridSize * spacing - 4)
        .attr('x', 2)
        .attr('y', 2)
        .attr('rx', 6)
        .attr('fill', 'rgba(109, 40, 217, 0.05)')
        .attr('stroke', '#6d28d9')
        .attr('stroke-width', 2)
        .attr('stroke-dasharray', '4,2')
        .attr('opacity', 1);

      const bankB = latticeContainer
        .append('rect')
        .attr('width', (gridSize * spacing) / 2 - 4)
        .attr('height', gridSize * spacing - 4)
        .attr('x', (gridSize * spacing) / 2 + 2)
        .attr('y', 2)
        .attr('rx', 6)
        .attr('fill', 'none')
        .attr('stroke', '#ec4899')
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '4,2')
        .attr('opacity', 0.3);

      // Data pulses (Directional Flow)
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
          svg
            .append('circle')
            .attr('r', size)
            .attr('fill', color)
            .attr('filter', 'url(#glow)')
            .attr('cx', source.x)
            .attr('cy', source.y)
            .attr('opacity', 0)
            .transition()
            .delay(delay + streamDelay)
            .duration(100)
            .attr('opacity', 1)
            .transition()
            .duration(1200)
            .ease(d3.easeCubicInOut)
            .attr('cx', target.x)
            .attr('cy', target.y)
            .on('start', function () {
              if (toId === 'SAB') {
                createRipple(width / 2, height / 2, color, true);
              } else if (fromId === 'SAB') {
                createRipple(width / 2, height / 2, color, false);
              }
            })
            .attr('opacity', 0.2)
            .transition()
            .duration(100)
            .attr('opacity', 0)
            .remove();
        });
      };

      const createRipple = (x: number, y: number, color: string, isWrite: boolean) => {
        latticeContainer
          .append('circle')
          .attr('cx', x - (width / 2 - (gridSize * spacing) / 2))
          .attr('cy', y - (height / 2 - (gridSize * spacing) / 2))
          .attr('r', 0)
          .attr('fill', 'none')
          .attr('stroke', color)
          .attr('stroke-width', 1.5)
          .attr('opacity', 0.8)
          .transition()
          .duration(800)
          .attr('r', 70)
          .attr('opacity', 0)
          .remove();

        // Targeted point effect
        points
          .transition()
          .duration(300)
          .attr('r', d => {
            const cx = isWrite ? gridSize * 0.75 : gridSize * 0.25;
            const dist = Math.sqrt(Math.pow(d.row - cx, 2) + Math.pow(d.col - gridSize / 2, 2));
            return dist < 2.5 ? 2.5 : 1;
          })
          .attr('opacity', d => {
            const cx = isWrite ? gridSize * 0.75 : gridSize * 0.25;
            const dist = Math.sqrt(Math.pow(d.row - cx, 2) + Math.pow(d.col - gridSize / 2, 2));
            return dist < 2.5 ? 0.8 : 0.2;
          })
          .transition()
          .duration(500)
          .attr('r', 1)
          .attr('opacity', 0.2);
      };

      // Flip interval (The Heartbeat)
      const flipTimer = d3.interval(() => {
        setActiveBuffer(prev => {
          const next = prev === 'A' ? 'B' : 'A';

          bankA
            .transition()
            .duration(800)
            .ease(d3.easeElasticOut.amplitude(1).period(0.4))
            .attr('opacity', next === 'A' ? 1 : 0.3)
            .attr('stroke-width', next === 'A' ? 2 : 1)
            .attr('fill', next === 'A' ? 'rgba(109, 40, 217, 0.05)' : 'none');

          bankB
            .transition()
            .duration(800)
            .ease(d3.easeElasticOut.amplitude(1).period(0.4))
            .attr('opacity', next === 'B' ? 1 : 0.3)
            .attr('stroke-width', next === 'B' ? 2 : 1)
            .attr('fill', next === 'B' ? 'rgba(236, 72, 153, 0.05)' : 'none');

          points
            .transition()
            .duration(300)
            .attr('opacity', 0.5)
            .attr('r', 1.2)
            .transition()
            .duration(300)
            .attr('r', 1)
            .attr('opacity', 0.2);

          return next;
        });

        pulse('RUST', 'SAB', '#8b5cf6', 0, 2.5);
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
          g.append('text')
            .attr('text-anchor', 'middle')
            .attr('dy', '85px')
            .attr('font-family', theme.fonts.typewriter)
            .attr('font-size', '11px')
            .attr('font-weight', '700')
            .attr('fill', '#1a1a1a')
            .text(d.label);

          g.append('text')
            .attr('text-anchor', 'middle')
            .attr('dy', '100px')
            .attr('font-family', theme.fonts.main)
            .attr('font-size', '10px')
            .attr('font-weight', '500')
            .attr('fill', '#737373')
            .text(d.desc);
        } else {
          g.append('rect')
            .attr('width', 160)
            .attr('height', 50)
            .attr('x', -80)
            .attr('y', -25)
            .attr('rx', 8)
            .attr('fill', '#ffffff')
            .attr('stroke', d.id === 'RUST' ? '#a855f7' : d.id === 'GO' ? '#f472b6' : '#22c55e')
            .attr('stroke-width', 1)
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
            .attr('dy', '15px')
            .attr('font-family', theme.fonts.main)
            .attr('font-size', '10px')
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
        viewBox="0 0 800 600"
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
