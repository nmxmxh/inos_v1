import { useRef, useCallback } from 'react';
import styled from 'styled-components';
import * as d3 from 'd3';
import D3Container from '../ui/D3Container';

const Container = styled.div`
  width: 100%;
  height: 400px;
  position: relative;
`;

const Tooltip = styled.div`
  position: absolute;
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.85);
  color: white;
  border-radius: 4px;
  font-size: 12px;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.2s;
  z-index: 10;
  max-width: 200px;
  backdrop-filter: blur(4px);
  border: 1px solid rgba(255, 255, 255, 0.1);
`;

export default function HistoryTimeline() {
  const tooltipRef = useRef<HTMLDivElement>(null);

  const renderViz = useCallback(
    (svg: d3.Selection<SVGSVGElement, unknown, null, undefined>, width: number, height: number) => {
      svg.selectAll('*').remove();

      const padding = 60;

      // Data - The Fork
      const years = [
        { year: 1990, label: 'WWW Initialized', detail: 'HTTP/1.0 created for static documents.' },
        {
          year: 1996,
          label: 'The Fork',
          detail: 'The vision of Shared Memory (Plan 9) is traded for Message Passing (HTTP).',
        },
        {
          year: 2005,
          label: 'The Cloud',
          detail: 'Centralization scales, but Serialization Tax grows with microservices.',
        },
        {
          year: 2015,
          label: 'WASM Launch',
          detail: 'Near-native speed arrives, but still trapped in single-threaded silos.',
        },
        {
          year: 2026,
          label: 'INOS',
          detail: 'The Correction. Distributed Shared Memory (SAB) unites all layers.',
        },
      ];

      const xScale = d3
        .scaleLinear()
        .domain([1990, 2028])
        .range([padding, width - padding]);

      // Background Grid
      svg
        .append('g')
        .selectAll('line')
        .data(d3.range(1990, 2030, 5))
        .enter()
        .append('line')
        .attr('x1', d => xScale(d))
        .attr('y1', 0)
        .attr('x2', d => xScale(d))
        .attr('y2', height)
        .attr('stroke', '#f1f5f9')
        .attr('stroke-width', 1);

      // Main Timeline Axis
      svg
        .append('line')
        .attr('x1', padding)
        .attr('y1', height / 2)
        .attr('x2', width - padding)
        .attr('y2', height / 2)
        .attr('stroke', '#cbd5e1')
        .attr('stroke-width', 2);

      // Path definitions
      const messagePath: [number, number][] = [
        [1996, height / 2],
        [2002, height / 2 - 50],
        [2008, height / 2 - 90],
        [2018, height / 2 - 110],
        [2028, height / 2 - 120],
      ];

      const memoryPath: [number, number][] = [
        [1996, height / 2],
        [2005, height / 2 + 50],
        [2018, height / 2 + 90],
        [2026, height / 2],
      ];

      const lineGen = d3
        .line<[number, number]>()
        .x(d => xScale(d[0]))
        .y(d => d[1])
        .curve(d3.curveBasis);

      // Helper for animated path
      const animatePath = (data: any, color: string, width: number, dashed = false) => {
        const path = svg
          .append('path')
          .datum(data)
          .attr('d', lineGen)
          .attr('fill', 'none')
          .attr('stroke', color)
          .attr('stroke-width', width)
          .attr('opacity', 0.8);

        if (dashed) {
          path.attr('stroke-dasharray', '5,5');
        }

        const totalLength = path.node()?.getTotalLength() || 0;

        path
          .attr('stroke-dasharray', `${totalLength} ${totalLength}`)
          .attr('stroke-dashoffset', totalLength)
          .transition()
          .duration(2000)
          .ease(d3.easeCubicInOut)
          .attr('stroke-dashoffset', dashed ? 5 : 0);

        return path;
      };

      // Draw paths with animation
      animatePath(messagePath, '#ef4444', 3, true);
      animatePath(memoryPath, '#6366f1', 4);

      // Add labels
      svg
        .append('text')
        .attr('x', width - 220)
        .attr('y', height / 2 - 135)
        .attr('fill', '#ef4444')
        .attr('font-size', '11px')
        .attr('font-weight', '800')
        .text('HTTP / JSON / MESSAGE PASSING');

      svg
        .append('text')
        .attr('x', width - 220)
        .attr('y', height / 2 + 30)
        .attr('fill', '#6366f1')
        .attr('font-size', '11px')
        .attr('font-weight', '800')
        .text('INOS / SHARED MEMORY (SAB)');

      // Animated "Copy Tax" markers along red path
      const markers = [
        { text: '60% COPY TAX', year: 2005, yOffset: -85 },
        { text: 'SERIALIZATION WALL', year: 2012, yOffset: -105 },
        { text: 'ZERO COPY', year: 2024, yOffset: 65, color: '#6366f1' },
      ];

      svg
        .selectAll('.marker')
        .data(markers)
        .enter()
        .append('text')
        .attr('x', d => xScale(d.year))
        .attr('y', d => height / 2 + d.yOffset)
        .attr('fill', d => d.color || '#ef4444')
        .attr('font-size', '9px')
        .attr('font-weight', 'bold')
        .attr('text-anchor', 'middle')
        .attr('opacity', 0)
        .transition()
        .delay((_d, i) => 1500 + i * 500)
        .duration(1000)
        .attr('opacity', 0.7);

      // Milestone Circles
      const gMilestones = svg
        .selectAll('.milestone')
        .data(years)
        .enter()
        .append('g')
        .attr('class', 'milestone')
        .attr('transform', d => `translate(${xScale(d.year)}, ${height / 2})`);

      gMilestones
        .append('circle')
        .attr('r', 0)
        .attr('fill', 'white')
        .attr('stroke', '#1e40af')
        .attr('stroke-width', 2)
        .transition()
        .delay((_d, i) => i * 400)
        .duration(800)
        .attr('r', 6);

      // Interaction layer
      gMilestones
        .append('circle')
        .attr('r', 15)
        .attr('fill', 'transparent')
        .style('cursor', 'pointer')
        .on('mouseenter', function (event, d) {
          d3.select(this.parentNode as any)
            .select('circle')
            .transition()
            .attr('r', 10)
            .attr('fill', '#1e40af');
          const tooltip = d3.select(tooltipRef.current);
          tooltip
            .style('opacity', 1)
            .html(
              `<strong>${d.year}</strong><br/>${d.label}<br/><br/><div style="color: #cbd5e1; font-size: 11px;">${d.detail}</div>`
            )
            .style('left', event.offsetX + 20 + 'px')
            .style('top', event.offsetY - 20 + 'px');
        })
        .on('mouseleave', function () {
          d3.select(this.parentNode as any)
            .select('circle')
            .transition()
            .attr('r', 6)
            .attr('fill', 'white');
          d3.select(tooltipRef.current).style('opacity', 0);
        });

      gMilestones
        .append('text')
        .attr('y', 25)
        .attr('text-anchor', 'middle')
        .attr('font-size', '10px')
        .attr('font-weight', '600')
        .attr('fill', '#94a3b8')
        .text(d => d.year.toString());

      return () => {
        svg.selectAll('*').interrupt();
      };
    },
    []
  );

  return (
    <Container>
      <Tooltip ref={tooltipRef} />
      <D3Container
        render={renderViz}
        dependencies={[renderViz]}
        viewBox="0 0 800 400"
        height="100%"
      />
    </Container>
  );
}
