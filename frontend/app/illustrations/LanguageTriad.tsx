import { useRef, useEffect, useState } from 'react';
import styled from 'styled-components';
import * as d3 from 'd3';

const Container = styled.div`
  width: 100%;
  height: 500px;
  position: relative;
  background: radial-gradient(
    circle at center,
    rgba(248, 250, 252, 1) 0%,
    rgba(241, 245, 249, 0.5) 100%
  );
  border-radius: 12px;
  overflow: hidden;
`;

const DetailBox = styled.div`
  position: absolute;
  top: 20px;
  right: 20px;
  width: 240px;
  padding: 20px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(8px);
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1);
  font-size: 14px;
  z-index: 20;

  h4 {
    margin: 0 0 8px 0;
    font-size: 16px;
    font-weight: 800;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  p {
    margin: 0;
    color: #475569;
    line-height: 1.6;
  }

  ul {
    margin: 12px 0 0 0;
    padding: 0 0 0 16px;
    color: #64748b;
    font-size: 12px;
  }
`;

export default function LanguageTriad() {
  const svgRef = useRef<SVGSVGElement>(null);
  const [activeNode, setActiveNode] = useState<any>(null);

  useEffect(() => {
    if (!svgRef.current) return;

    const width = 800;
    const height = 500;
    const centerX = width / 2;
    const centerY = height / 2;
    const radius = 160;

    const svg = d3.select(svgRef.current).attr('viewBox', `0 0 ${width} ${height}`);

    svg.selectAll('*').remove();

    // Define Nodes
    const nodes = [
      {
        id: 'Go',
        label: 'THE BRAIN',
        color: '#00add8',
        icon: '🧠',
        x: centerX,
        y: centerY - radius,
        desc: 'The Orchestrator.',
        capabilities: [
          'Preemptive Scheduling',
          'Economic Policy Manager',
          'Supervisor coordination',
          'Epoch orchestration',
        ],
      },
      {
        id: 'Rust',
        label: 'THE MUSCLE',
        color: '#dea584',
        icon: '💪',
        x: centerX + radius * Math.cos(Math.PI / 6),
        y: centerY + radius * Math.sin(Math.PI / 6),
        desc: 'The Execution Layer.',
        capabilities: [
          'SIMD Physics compute',
          'Zero-copy memory access',
          'WebGPU pipeline prep',
          'BLAKE3 content hashing',
        ],
      },
      {
        id: 'JS',
        label: 'THE BODY',
        color: '#f7df1e',
        icon: '👁️',
        x: centerX - radius * Math.cos(Math.PI / 6),
        y: centerY + radius * Math.sin(Math.PI / 6),
        desc: 'The Perception Layer.',
        capabilities: [
          'User Ingress handling',
          'WebGPU rendering',
          'Sensor integration',
          'Atomic state reflection',
        ],
      },
    ];

    const links = [
      { source: nodes[0], target: nodes[1], label: 'Atomics' },
      { source: nodes[1], target: nodes[2], label: 'Shared Memory' },
      { source: nodes[2], target: nodes[0], label: 'Signals' },
    ];

    // Glow Filter
    const filter = svg.append('defs').append('filter').attr('id', 'glow');
    filter.append('feGaussianBlur').attr('stdDeviation', '4').attr('result', 'blur');
    const feMerge = filter.append('feMerge');
    feMerge.append('feMergeNode').attr('in', 'blur');
    feMerge.append('feMergeNode').attr('in', 'SourceGraphic');

    // Draw Links
    svg
      .selectAll('.link')
      .data(links)
      .enter()
      .append('path')
      .attr('class', 'link')
      .attr('d', d => `M${d.source.x},${d.source.y} L${d.target.x},${d.target.y}`)
      .attr('stroke', '#cbd5e1')
      .attr('stroke-width', 2)
      .attr('stroke-dasharray', '8,8')
      .attr('fill', 'none');

    // Animated Signal Pulses
    const drawPulse = (link: any, delay: number) => {
      const pulse = svg
        .append('circle')
        .attr('r', 4)
        .attr('fill', link.target.color)
        .style('filter', 'url(#glow)');

      const animate = () => {
        pulse
          .attr('transform', `translate(${link.source.x}, ${link.source.y})`)
          .style('opacity', 1)
          .transition()
          .delay(delay)
          .duration(3000)
          .ease(d3.easeLinear)
          .attrTween('transform', () => {
            return (t: number) => {
              const x = link.source.x + (link.target.x - link.source.x) * t;
              const y = link.source.y + (link.target.y - link.source.y) * t;
              return `translate(${x}, ${y})`;
            };
          })
          .transition()
          .duration(500)
          .style('opacity', 0)
          .on('end', animate);
      };

      animate();
    };

    links.forEach((link, i) => {
      drawPulse(link, i * 1000);
      drawPulse(link, i * 1000 + 1500); // Second pulse per link
    });

    // Draw Nodes
    const gNodes = svg
      .selectAll('.node-group')
      .data(nodes)
      .enter()
      .append('g')
      .attr('class', 'node-group')
      .attr('transform', d => `translate(${d.x}, ${d.y})`)
      .style('cursor', 'pointer')
      .on('mouseenter', (event, d) => {
        setActiveNode(d);
        d3.select(event.currentTarget).select('.node-circle').transition().attr('r', 55);
        d3.select(event.currentTarget).select('.node-bg').transition().attr('opacity', 1);
      })
      .on('mouseleave', event => {
        setActiveNode(null);
        d3.select(event.currentTarget).select('.node-circle').transition().attr('r', 45);
        d3.select(event.currentTarget).select('.node-bg').transition().attr('opacity', 0.2);
      });

    // Node Backdrop (Hollow ring)
    gNodes
      .append('circle')
      .attr('class', 'node-bg')
      .attr('r', 65)
      .attr('fill', 'none')
      .attr('stroke', d => d.color)
      .attr('stroke-width', 1)
      .attr('opacity', 0.2)
      .attr('stroke-dasharray', '2,2');

    // Main Node Circle
    gNodes
      .append('circle')
      .attr('class', 'node-circle')
      .attr('r', 45)
      .attr('fill', 'white')
      .attr('stroke', d => d.color)
      .attr('stroke-width', 4)
      .style('filter', 'url(#glow)');

    // Node Emoji/Icon
    gNodes
      .append('text')
      .attr('text-anchor', 'middle')
      .attr('dy', '.35em')
      .attr('font-size', '24px')
      .text(d => d.icon);

    // Node Label
    gNodes
      .append('text')
      .attr('text-anchor', 'middle')
      .attr('y', -65)
      .attr('font-weight', '800')
      .attr('font-size', '14px')
      .attr('fill', '#1e293b')
      .text(d => d.id);

    gNodes
      .append('text')
      .attr('text-anchor', 'middle')
      .attr('y', 75)
      .attr('font-weight', '700')
      .attr('font-size', '10px')
      .attr('fill', d => d.color)
      .attr('letter-spacing', '0.05em')
      .text(d => d.label);

    return () => {
      svg.selectAll('*').interrupt();
    };
  }, []);

  return (
    <Container>
      <svg ref={svgRef} style={{ width: '100%', height: '100%' }} />
      {activeNode && (
        <DetailBox>
          <h4 style={{ color: activeNode.color }}>
            {activeNode.id}: {activeNode.label}
          </h4>
          <p>{activeNode.desc}</p>
          <ul>
            {activeNode.capabilities.map((c: string, i: number) => (
              <li key={i}>{c}</li>
            ))}
          </ul>
        </DetailBox>
      )}
    </Container>
  );
}
