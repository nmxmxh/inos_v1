import { useEffect, useRef } from 'react';
import * as d3 from 'd3';
import styled from 'styled-components';

const Container = styled.div`
  width: 220px;
  background: rgba(15, 23, 42, 0.9);
  backdrop-filter: blur(8px);
  border: 1px solid rgba(139, 92, 246, 0.3);
  border-radius: 8px;
  padding: 12px;
  color: #fff;
  font-family: 'JetBrains Mono', monospace;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
  pointer-events: none;
  overflow: hidden;

  h4 {
    margin: 0 0 8px 0;
    font-size: 10px;
    text-transform: uppercase;
    color: #8b5cf6;
    letter-spacing: 0.1em;
    display: flex;
    justify-content: space-between;
  }
`;

const StatRow = styled.div`
  display: flex;
  justify-content: space-between;
  font-size: 9px;
  margin-bottom: 4px;
  color: #94a3b8;

  span:last-child {
    color: #fff;
    font-weight: 600;
  }
`;

interface BoidHUDProps {
  data: {
    sep: number;
    ali: number;
    coh: number;
    trick: number;
    energy: number;
    fitness: number;
  };
}

export default function BoidHUD({ data }: BoidHUDProps) {
  const svgRef = useRef<SVGSVGElement>(null);

  useEffect(() => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    // const width = 196;
    // const height = 100;
    // const padding = 20;

    // --- RADAR CHART for Neural Weights ---
    const radarCenter = { x: 50, y: 50 };
    const radius = 35;
    const axes = [
      { id: 'sep', label: 'SEP', value: data.sep, angle: 0 },
      { id: 'ali', label: 'ALI', value: data.ali, angle: Math.PI / 2 },
      { id: 'coh', label: 'COH', value: data.coh, angle: Math.PI },
      { id: 'trk', label: 'TRK', value: data.trick, angle: (3 * Math.PI) / 2 },
    ];

    const g = svg.append('g');

    // Radar Background
    [0.25, 0.5, 0.75, 1.0].forEach(r => {
      g.append('circle')
        .attr('cx', radarCenter.x)
        .attr('cy', radarCenter.y)
        .attr('r', radius * r)
        .attr('fill', 'none')
        .attr('stroke', '#334155')
        .attr('stroke-width', 1);
    });

    // Axes
    axes.forEach(axis => {
      const x = radarCenter.x + Math.cos(axis.angle) * radius;
      const y = radarCenter.y + Math.sin(axis.angle) * radius;
      g.append('line')
        .attr('x1', radarCenter.x)
        .attr('y1', radarCenter.y)
        .attr('x2', x)
        .attr('y2', y)
        .attr('stroke', '#334155')
        .attr('stroke-width', 1);

      // Label
      g.append('text')
        .attr('x', radarCenter.x + Math.cos(axis.angle) * (radius + 10))
        .attr('y', radarCenter.y + Math.sin(axis.angle) * (radius + 10))
        .attr('text-anchor', 'middle')
        .attr('dominant-baseline', 'middle')
        .attr('fill', '#94a3b8')
        .attr('font-size', '8px')
        .text(axis.label);
    });

    // Data Shape
    const lineGenerator = d3
      .line<{ x: number; y: number }>()
      .x(d => d.x)
      .y(d => d.y)
      .curve(d3.curveLinearClosed);

    // Normalize weights (-1 to 1 range typically in boids) to 0-1 for chart
    // But boids weights here seem to be multipliers, maybe -1 to 5 range?
    // Let's assume reasonable bounds for visualization
    const normalize = (v: number) => Math.max(0.1, Math.min(1.0, (v + 1.0) / 4.0));

    const points = axes.map(axis => {
      const r = normalize(axis.value) * radius;
      return {
        x: radarCenter.x + Math.cos(axis.angle) * r,
        y: radarCenter.y + Math.sin(axis.angle) * r,
      };
    });

    g.append('path')
      .datum(points)
      .attr('d', lineGenerator)
      .attr('fill', 'rgba(139, 92, 246, 0.3)')
      .attr('stroke', '#8b5cf6')
      .attr('stroke-width', 2);

    // --- ENERGY BAR ---
    // Right side gauges
    const rightX = 110;
    const gaugeWidth = 80;

    // Energy Header
    g.append('text')
      .attr('x', rightX)
      .attr('y', 20)
      .attr('fill', '#94a3b8')
      .attr('font-size', '8px')
      .text('ENERGY');

    // Energy Bar bg
    g.append('rect')
      .attr('x', rightX)
      .attr('y', 25)
      .attr('width', gaugeWidth)
      .attr('height', 6)
      .attr('fill', '#1e293b')
      .attr('rx', 2);

    // Energy Bar fill
    g.append('rect')
      .attr('x', rightX)
      .attr('y', 25)
      .attr('width', gaugeWidth * Math.max(0, Math.min(1, data.energy)))
      .attr('height', 6)
      .attr('fill', data.energy < 0.3 ? '#ef4444' : '#10b981')
      .attr('rx', 2);

    // Fitness Header
    g.append('text')
      .attr('x', rightX)
      .attr('y', 55)
      .attr('fill', '#94a3b8')
      .attr('font-size', '8px')
      .text('FITNESS SCORE');

    // Fitness Value
    g.append('text')
      .attr('x', rightX)
      .attr('y', 75)
      .attr('fill', '#fff')
      .attr('font-size', '16px')
      .attr('font-weight', 'bold')
      .text(data.fitness.toFixed(3));
  }, [data]);

  return (
    <Container>
      <h4>
        Leader Unit <span>ID: 001</span>
      </h4>
      <svg ref={svgRef} width="100%" height="100" viewBox="0 0 196 100" />
      <div style={{ marginTop: '8px', borderTop: '1px solid #334155', paddingTop: '8px' }}>
        <StatRow>
          <span>Genome ID</span>
          <span>#8F3A2C</span>
        </StatRow>
        <StatRow>
          <span>Generation</span>
          <span>124</span>
        </StatRow>
        <StatRow>
          <span>State</span>
          <span style={{ color: '#10b981' }}>OPTIMAL</span>
        </StatRow>
      </div>
    </Container>
  );
}
