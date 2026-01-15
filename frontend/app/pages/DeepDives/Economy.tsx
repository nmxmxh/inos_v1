/**
 * INOS Technical Codex — Deep Dive: Credits & Economy
 *
 * A comprehensive exploration of the participation-first economy: UBI,
 * identity, social graphs, and gamification. Explains how INOS achieves
 * sustainable value creation and distribution.
 */

import { useState, useCallback } from 'react';
import styled, { useTheme } from 'styled-components';
import * as d3 from 'd3';
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
    background: rgba(16, 185, 129, 0.08);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(16, 185, 129, 0.2);
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[6]} 0;

    h4 {
      margin: 0 0 ${p => p.theme.spacing[2]} 0;
      color: #10b981;
      font-size: ${p => p.theme.fontSizes.lg};
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

  PillarGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
    gap: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[6]} 0;
  `,

  PillarCard: styled.div<{ $color: string }>`
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

  FeeTable: styled.table`
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

    tr:last-child td {
      border-bottom: none;
    }
  `,

  SocialCard: styled.div<{ $type: 'creator' | 'referrer' | 'close' }>`
    background: ${p =>
      p.$type === 'creator'
        ? 'rgba(245, 158, 11, 0.08)'
        : p.$type === 'referrer'
          ? 'rgba(59, 130, 246, 0.08)'
          : 'rgba(16, 185, 129, 0.08)'};
    backdrop-filter: blur(12px);
    border: 1px solid
      ${p =>
        p.$type === 'creator'
          ? 'rgba(245, 158, 11, 0.2)'
          : p.$type === 'referrer'
            ? 'rgba(59, 130, 246, 0.2)'
            : 'rgba(16, 185, 129, 0.2)'};
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};

    h4 {
      margin: 0 0 ${p => p.theme.spacing[2]} 0;
      color: ${p =>
        p.$type === 'creator' ? '#f59e0b' : p.$type === 'referrer' ? '#3b82f6' : '#10b981'};
      font-size: ${p => p.theme.fontSizes.base};
    }

    .yield {
      font-size: ${p => p.theme.fontSizes.xl};
      font-weight: 700;
      color: ${p =>
        p.$type === 'creator' ? '#f59e0b' : p.$type === 'referrer' ? '#3b82f6' : '#10b981'};
      margin-bottom: ${p => p.theme.spacing[2]};
    }

    p {
      margin: 0;
      font-size: ${p => p.theme.fontSizes.sm};
      line-height: 1.6;
      color: ${p => p.theme.colors.inkMedium};
    }
  `,

  SocialGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[6]} 0;
  `,

  MultiplierBox: styled.div`
    background: rgba(16, 185, 129, 0.08);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(16, 185, 129, 0.2);
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};
    text-align: center;
    font-family: 'JetBrains Mono', monospace;
    font-size: ${p => p.theme.fontSizes.lg};
    margin: ${p => p.theme.spacing[4]} 0;
  `,
};

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: ECONOMIC FLYWHEEL (D3Container + D3 transitions)
// ────────────────────────────────────────────────────────────────────────────
function EconomicFlywheelDiagram() {
  const theme = useTheme();

  const renderFlywheel: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').remove();

      // Responsive: scale based on container width, min 280px
      const scale = Math.min(1, width / 700);
      const centerX = 350;
      const centerY = 200;
      const radius = 100 * scale + 50 * (1 - scale); // Shrinks on mobile

      // Flywheel nodes
      const nodes = [
        { angle: -90, label: 'Worker', icon: '👷', color: '#10b981' },
        { angle: -30, label: 'Treasury', icon: '🏦', color: '#8b5cf6' },
        { angle: 30, label: 'UBI', icon: '💰', color: '#f59e0b' },
        { angle: 90, label: 'Consumer', icon: '🛒', color: '#3b82f6' },
        { angle: 150, label: 'Jobs', icon: '⚡', color: '#ef4444' },
        { angle: 210, label: 'Mesh', icon: '🌐', color: '#06b6d4' },
      ];

      // Draw connections
      nodes.forEach((node, i) => {
        const next = nodes[(i + 1) % nodes.length];
        const angle1 = (node.angle * Math.PI) / 180;
        const angle2 = (next.angle * Math.PI) / 180;
        svg
          .append('line')
          .attr('x1', centerX + Math.cos(angle1) * radius)
          .attr('y1', centerY + Math.sin(angle1) * radius)
          .attr('x2', centerX + Math.cos(angle2) * radius)
          .attr('y2', centerY + Math.sin(angle2) * radius)
          .attr('stroke', 'rgba(139, 92, 246, 0.2)')
          .attr('stroke-width', 2);
      });

      // Draw nodes
      const nodeRadius = 28 * scale + 14 * (1 - scale);
      nodes.forEach(node => {
        const angle = (node.angle * Math.PI) / 180;
        const x = centerX + Math.cos(angle) * radius;
        const y = centerY + Math.sin(angle) * radius;

        svg
          .append('circle')
          .attr('cx', x)
          .attr('cy', y)
          .attr('r', nodeRadius)
          .attr('fill', `${node.color}15`)
          .attr('stroke', node.color)
          .attr('stroke-width', 2);

        svg
          .append('text')
          .attr('x', x)
          .attr('y', y + 5)
          .attr('text-anchor', 'middle')
          .attr('font-size', 18 * scale + 10 * (1 - scale))
          .text(node.icon);

        const labelRadius = radius + 40 * scale + 25 * (1 - scale);
        const lx = centerX + Math.cos(angle) * labelRadius;
        const ly = centerY + Math.sin(angle) * labelRadius;
        svg
          .append('text')
          .attr('x', lx)
          .attr('y', ly + 4)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9)
          .attr('font-weight', 600)
          .attr('fill', node.color)
          .text(node.label);
      });

      // Center label
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', centerY - 5)
        .attr('text-anchor', 'middle')
        .attr('font-size', 11)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkDark)
        .text('INOS');
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', centerY + 10)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('fill', theme.colors.inkLight)
        .text('Economy');

      // Animated credit token
      const token = svg
        .append('circle')
        .attr('r', 10)
        .attr('fill', '#f59e0b')
        .attr('stroke', 'white')
        .attr('stroke-width', 2);
      const tokenText = svg
        .append('text')
        .attr('text-anchor', 'middle')
        .attr('font-size', 8)
        .attr('fill', 'white')
        .attr('dy', 4)
        .text('₵');

      function animateToken() {
        const startAngle = -90;
        const startRad = (startAngle * Math.PI) / 180;
        token
          .attr('cx', centerX + Math.cos(startRad) * radius)
          .attr('cy', centerY + Math.sin(startRad) * radius);
        tokenText
          .attr('x', centerX + Math.cos(startRad) * radius)
          .attr('y', centerY + Math.sin(startRad) * radius);

        token
          .transition()
          .duration(6000)
          .ease(d3.easeLinear)
          .attrTween(
            'cx',
            () => t => String(centerX + Math.cos(((startAngle + t * 360) * Math.PI) / 180) * radius)
          )
          .attrTween(
            'cy',
            () => t => String(centerY + Math.sin(((startAngle + t * 360) * Math.PI) / 180) * radius)
          )
          .on('end', animateToken);

        tokenText
          .transition()
          .duration(6000)
          .ease(d3.easeLinear)
          .attrTween(
            'x',
            () => t => String(centerX + Math.cos(((startAngle + t * 360) * Math.PI) / 180) * radius)
          )
          .attrTween(
            'y',
            () => t => String(centerY + Math.sin(((startAngle + t * 360) * Math.PI) / 180) * radius)
          );
      }
      animateToken();

      return () => {
        token.interrupt();
        tokenText.interrupt();
      };
    },
    [theme]
  );

  return (
    <D3Container
      render={renderFlywheel}
      dependencies={[renderFlywheel]}
      viewBox="0 0 700 400"
      height={400}
    />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: FEE DISTRIBUTION (D3Container + D3 transitions)
// ────────────────────────────────────────────────────────────────────────────
function FeeDistributionDiagram() {
  const theme = useTheme();

  const renderFee: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').remove();

      // Responsive scaling
      const scale = Math.min(1, width / 700);
      const offsetX = width < 500 ? -20 : 0; // Shift left on mobile

      // Source Job Payment
      svg
        .append('rect')
        .attr('x', 50 + offsetX)
        .attr('y', 60)
        .attr('width', 100 * scale + 50 * (1 - scale))
        .attr('height', 50)
        .attr('rx', 8)
        .attr('fill', '#3b82f620')
        .attr('stroke', '#3b82f6')
        .attr('stroke-width', 2);
      svg
        .append('text')
        .attr('x', 100 + offsetX)
        .attr('y', 82)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', 600)
        .attr('fill', '#3b82f6')
        .text('Job: 1000₵');
      svg
        .append('text')
        .attr('x', 100 + offsetX)
        .attr('y', 95)
        .attr('text-anchor', 'middle')
        .attr('font-size', 8)
        .attr('fill', theme.colors.inkLight)
        .text('+5% fee = 1050₵');

      // Distribution targets - responsive positions
      const baseTargets = [
        { label: 'Workers', amount: '950₵', pct: '95%', color: '#10b981', icon: '👷' },
        { label: 'Treasury', amount: '35₵', pct: '3.5%', color: '#8b5cf6', icon: '🏦' },
        { label: 'Creator', amount: '5₵', pct: '0.5%', color: '#f59e0b', icon: '👑' },
        { label: 'Referrer', amount: '5₵', pct: '0.5%', color: '#3b82f6', icon: '🔗' },
        { label: 'Close IDs', amount: '5₵', pct: '0.5%', color: '#06b6d4', icon: '👥' },
      ];

      // Calculate responsive x positions
      const startX = 150 + offsetX;
      const spacing = width < 500 ? 80 : 95;
      const targets = baseTargets.map((t, i) => ({ ...t, x: 220 + i * spacing + offsetX }));
      const startY = 85;

      // Draw lines and boxes
      targets.forEach((target, i) => {
        svg
          .append('line')
          .attr('x1', startX)
          .attr('y1', startY)
          .attr('x2', target.x - 25)
          .attr('y2', startY)
          .attr('stroke', `${target.color}40`)
          .attr('stroke-width', 2);

        const boxW = width < 500 ? 55 : 70;

        // Glow effect
        svg
          .append('filter')
          .attr('id', `glow-${i}`)
          .append('feGaussianBlur')
          .attr('stdDeviation', 2.5)
          .attr('result', 'coloredBlur');

        svg
          .append('rect')
          .attr('x', target.x - boxW / 2)
          .attr('y', 50)
          .attr('width', boxW)
          .attr('height', 70)
          .attr('rx', 8)
          .attr('fill', `${target.color}25`) // Increased opacity
          .attr('stroke', target.color)
          .attr('stroke-width', 1.5)
          .style('filter', `drop-shadow(0 0 4px ${target.color}60)`);

        svg
          .append('text')
          .attr('x', target.x)
          .attr('y', 70)
          .attr('text-anchor', 'middle')
          .attr('font-size', width < 500 ? 12 : 14)
          .text(target.icon);
        svg
          .append('text')
          .attr('x', target.x)
          .attr('y', 90)
          .attr('text-anchor', 'middle')
          .attr('font-size', width < 500 ? 7 : 8)
          .attr('font-weight', 600)
          .attr('fill', target.color)
          .text(target.label);
        svg
          .append('text')
          .attr('x', target.x)
          .attr('y', 103)
          .attr('text-anchor', 'middle')
          .attr('font-size', width < 500 ? 9 : 10)
          .attr('font-weight', 700)
          .attr('fill', theme.colors.inkDark)
          .text(target.amount);
        svg
          .append('text')
          .attr('x', target.x)
          .attr('y', 115)
          .attr('text-anchor', 'middle')
          .attr('font-size', 7)
          .attr('fill', theme.colors.inkLight)
          .text(target.pct);
      });

      // Animated token
      const token = svg
        .append('circle')
        .attr('r', 6)
        .attr('cx', startX)
        .attr('cy', startY)
        .attr('fill', targets[0].color);

      let currentIndex = 0;
      function animateToken() {
        const target = targets[currentIndex];
        token
          .attr('fill', target.color)
          .attr('cx', startX)
          .transition()
          .duration(1000)
          .ease(d3.easeQuadInOut)
          .attr('cx', target.x - 25)
          .on('end', () => {
            currentIndex = (currentIndex + 1) % targets.length;
            animateToken();
          });
      }
      animateToken();

      return () => token.interrupt();
    },
    [theme]
  );

  return (
    <D3Container render={renderFee} dependencies={[renderFee]} viewBox="0 0 700 150" height={150} />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: DEVICE GRAPH UBI (D3Container + interactive controls)
// ────────────────────────────────────────────────────────────────────────────
function DeviceGraphDiagram() {
  const theme = useTheme();
  const [deviceCount, setDeviceCount] = useState(3);

  const renderDevices: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').remove();

      const centerX = 350;
      const centerY = 120;
      const scale = Math.min(1, width / 700);
      const orbitRadius = 80 * scale + 50 * (1 - scale);

      // Central DID
      svg
        .append('circle')
        .attr('cx', centerX)
        .attr('cy', centerY)
        .attr('r', 30 * scale + 20 * (1 - scale))
        .attr('fill', 'rgba(139, 92, 246, 0.15)')
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 3);
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', centerY + 5)
        .attr('text-anchor', 'middle')
        .attr('font-size', 11)
        .attr('font-weight', 600)
        .attr('fill', '#8b5cf6')
        .text('Your DID');

      // Device icons
      const devices = ['💻', '📱', '🖥️', '📟', '⌚'];
      for (let i = 0; i < deviceCount; i++) {
        const angle = ((i / deviceCount) * 360 - 90) * (Math.PI / 180);
        const x = centerX + Math.cos(angle) * orbitRadius;
        const y = centerY + Math.sin(angle) * orbitRadius;

        svg
          .append('line')
          .attr('x1', centerX)
          .attr('y1', centerY)
          .attr('x2', x)
          .attr('y2', y)
          .attr('stroke', '#10b981')
          .attr('stroke-width', 2)
          .attr('stroke-dasharray', '4,4');
        svg
          .append('circle')
          .attr('cx', x)
          .attr('cy', y)
          .attr('r', 22 * scale + 14 * (1 - scale))
          .attr('fill', 'rgba(16, 185, 129, 0.1)')
          .attr('stroke', '#10b981')
          .attr('stroke-width', 2);
        svg
          .append('text')
          .attr('x', x)
          .attr('y', y + 5)
          .attr('text-anchor', 'middle')
          .attr('font-size', 16 * scale + 12 * (1 - scale))
          .text(devices[i % devices.length]);
      }

      // UBI Multiplier display
      const multiplier = 1.0 + deviceCount * 0.001;
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', 250)
        .attr('text-anchor', 'middle')
        .attr('font-size', 13)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkDark)
        .text(`UBI Multiplier: ${multiplier.toFixed(3)}x`);
      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', 270)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('fill', theme.colors.inkLight)
        .text('+0.1% per verified device (no cap)');
    },
    [theme, deviceCount]
  );

  return (
    <div>
      <D3Container
        render={renderDevices}
        dependencies={[renderDevices]}
        viewBox="0 0 700 300"
        height={300}
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
        {[1, 2, 3, 4, 5].map(n => (
          <button
            key={n}
            onClick={() => setDeviceCount(n)}
            style={{
              padding: '6px 12px',
              background: deviceCount === n ? '#10b981' : 'white',
              color: deviceCount === n ? 'white' : '#6b7280',
              border: '1px solid',
              borderColor: deviceCount === n ? '#10b981' : '#d1d5db',
              borderRadius: '4px',
              fontSize: '11px',
              cursor: 'pointer',
            }}
          >
            {n} device{n > 1 ? 's' : ''}
          </button>
        ))}
      </div>
    </div>
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: GAMIFICATION TIERS (D3Container)
// ────────────────────────────────────────────────────────────────────────────
function GamificationTiersDiagram() {
  const theme = useTheme();

  const renderTiers: D3RenderFn = useCallback(
    (svg, width) => {
      svg.selectAll('*').remove();

      const spacing = width < 500 ? 85 : 140;
      const startX = width < 500 ? 60 : 90;

      const tiers = [
        { name: 'Light', mult: '1.0x', sab: '32MB', storage: '5GB', color: '#9ca3af' },
        { name: 'Moderate', mult: '1.1x', sab: '64MB', storage: '20GB', color: '#3b82f6' },
        { name: 'Heavy', mult: '1.5x', sab: '128MB', storage: '100GB', color: '#8b5cf6' },
        { name: 'Dedicated', mult: '2.0x', sab: '256MB+', storage: '500GB+', color: '#f59e0b' },
      ].map((t, i) => ({ ...t, x: startX + i * spacing }));

      // Progress arrow
      svg
        .append('line')
        .attr('x1', 30)
        .attr('y1', 260)
        .attr('x2', width < 500 ? 370 : 640)
        .attr('y2', 260)
        .attr('stroke', '#e5e7eb')
        .attr('stroke-width', 3);
      const arrowX = width < 500 ? 370 : 640;
      svg
        .append('polygon')
        .attr('points', `${arrowX},260 ${arrowX - 10},255 ${arrowX - 10},265`)
        .attr('fill', '#e5e7eb');
      svg
        .append('text')
        .attr('x', 350)
        .attr('y', 285)
        .attr('text-anchor', 'middle')
        .attr('font-size', width < 500 ? 9 : 10)
        .attr('fill', theme.colors.inkLight)
        .text('More Resources → Higher Rewards');

      const barWidth = width < 500 ? 70 : 110;
      tiers.forEach((tier, i) => {
        const barHeight = 60 + i * 25;
        const barY = 230 - barHeight;

        svg
          .append('rect')
          .attr('x', tier.x - barWidth / 2)
          .attr('y', barY)
          .attr('width', barWidth)
          .attr('height', barHeight)
          .attr('rx', 8)
          .attr('fill', `${tier.color}20`)
          .attr('stroke', tier.color)
          .attr('stroke-width', 2);
        svg
          .append('text')
          .attr('x', tier.x)
          .attr('y', barY + 18)
          .attr('text-anchor', 'middle')
          .attr('font-size', width < 500 ? 9 : 11)
          .attr('font-weight', 600)
          .attr('fill', tier.color)
          .text(tier.name);
        svg
          .append('text')
          .attr('x', tier.x)
          .attr('y', barY + barHeight / 2 + 8)
          .attr('text-anchor', 'middle')
          .attr('font-size', width < 500 ? 18 : 22)
          .attr('font-weight', 700)
          .attr('fill', tier.color)
          .text(tier.mult);
        svg
          .append('text')
          .attr('x', tier.x)
          .attr('y', 245)
          .attr('text-anchor', 'middle')
          .attr('font-size', width < 500 ? 7 : 9)
          .attr('fill', theme.colors.inkLight)
          .text(`${tier.sab} | ${tier.storage}`);
      });
    },
    [theme]
  );

  return (
    <D3Container
      render={renderTiers}
      dependencies={[renderTiers]}
      viewBox="0 0 700 310"
      height={310}
    />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// MAIN COMPONENT
// ────────────────────────────────────────────────────────────────────────────
export function Economy() {
  return (
    <Style.BlogContainer>
      <Style.SectionTitle>Deep Dive</Style.SectionTitle>
      <Style.PageTitle>Credits &amp; Economy</Style.PageTitle>

      <Style.LeadParagraph>
        How do you build an economy without banks, without employers, without borders? INOS answers
        with a <strong>participation-first economy</strong>—where simply being present earns you
        income, and productive work multiplies your rewards.
      </Style.LeadParagraph>

      <Style.DefinitionBox>
        <h4>⚠️ Credits Are Not Cryptocurrency</h4>
        <p>
          Credits are <strong>scheduler accounting units</strong>, like CPU time in mainframe days.
          They measure resource contribution and consumption within the mesh. They are not tradeable
          tokens, not backed by fiat, and not subject to speculative markets. Credits are
          infrastructure, not investment.
        </p>
      </Style.DefinitionBox>

      <Style.SectionDivider />

      {/* LESSON 1: THE FIVE PILLARS */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 1: The Five Pillars</h3>
          <p>
            INOS economics rest on five foundational principles. Each addresses a specific failure
            mode of traditional economies while creating synergies with the others.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.PillarGrid>
        <Style.PillarCard $color="#10b981">
          <div className="icon">💰</div>
          <div className="name">Liveness (UBI)</div>
          <div className="desc">Baseline income for all</div>
        </Style.PillarCard>
        <Style.PillarCard $color="#3b82f6">
          <div className="icon">⚡</div>
          <div className="name">PoUW</div>
          <div className="desc">Proof of Useful Work</div>
        </Style.PillarCard>
        <Style.PillarCard $color="#8b5cf6">
          <div className="icon">🎮</div>
          <div className="name">Gaming</div>
          <div className="desc">Novelty rewards</div>
        </Style.PillarCard>
        <Style.PillarCard $color="#f59e0b">
          <div className="icon">🏦</div>
          <div className="name">Protocol Fee</div>
          <div className="desc">5% sustainability tax</div>
        </Style.PillarCard>
        <Style.PillarCard $color="#ef4444">
          <div className="icon">👑</div>
          <div className="name">Royalty</div>
          <div className="desc">Creator yield</div>
        </Style.PillarCard>
      </Style.PillarGrid>

      <Style.DefinitionBox>
        <h4>Universal Basic Income (UBI)</h4>
        <p>
          Every verified participant receives a continuous credit drip—ensuring a{' '}
          <strong>baseline of economic security</strong> regardless of current contribution. This
          isn't charity; it's recognition that network presence itself has value.
        </p>
      </Style.DefinitionBox>

      <Style.SectionDivider />

      {/* LESSON 2: THE ECONOMIC FLYWHEEL */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 2: The Economic Flywheel</h3>
          <p>
            The INOS economy is a <strong>closed-loop system</strong> where every action reinforces
            network health. Credits flow in a continuous cycle: work creates value, spending
            circulates it, and the protocol fee ensures sustainability.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Animated: Economic Flywheel</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <EconomicFlywheelDiagram />
        <Style.IllustrationCaption>
          Credits flow continuously through the economic flywheel, with each participant playing a
          vital role
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.SectionDivider />

      {/* LESSON 3: THE PROTOCOL FEE */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 3: The 5% Protocol Fee</h3>
          <p>
            Every transaction in INOS includes a <strong>5% protocol fee</strong>. This isn't
            extracted wealth—it's redistributed wealth that funds the network's key stakeholders:
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Animated: Fee Distribution</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <FeeDistributionDiagram />
        <Style.IllustrationCaption>
          A 1000₵ job becomes 1050₵ with fee, distributed across workers and the social graph
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.FeeTable>
        <thead>
          <tr>
            <th>Recipient</th>
            <th>Share</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>👷 Workers</td>
            <td>95%</td>
            <td>Direct compensation for productive work (multiplied by tier + devices)</td>
          </tr>
          <tr>
            <td>🏦 Treasury</td>
            <td>3.5%</td>
            <td>UBI reserve, growth pool, protocol surplus</td>
          </tr>
          <tr>
            <td>👑 Creator (nmxmxh)</td>
            <td>0.5%</td>
            <td>Architect royalty from all network activity</td>
          </tr>
          <tr>
            <td>🔗 Referrer</td>
            <td>0.5%</td>
            <td>Viral growth incentive for user acquisition</td>
          </tr>
          <tr>
            <td>👥 Close IDs</td>
            <td>0.5%</td>
            <td>Social proof and recovery network (shared)</td>
          </tr>
        </tbody>
      </Style.FeeTable>

      <Style.SectionDivider />

      {/* LESSON 4: SOCIAL ECONOMIC GRAPH */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 4: The Social-Economic Graph</h3>
          <p>
            Your economic success is tied to your <strong>social graph</strong>. INOS creates viral
            growth incentives AND identity security through three interconnected roles.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.SocialGrid>
        <Style.SocialCard $type="creator">
          <h4>👑 Creator</h4>
          <div className="yield">0.5%</div>
          <p>The architect (nmxmxh) earns from ALL network activity. Permanent.</p>
        </Style.SocialCard>
        <Style.SocialCard $type="referrer">
          <h4>🔗 Referrer</h4>
          <div className="yield">0.5%</div>
          <p>Your referrer earns from your activity. Locked after first job.</p>
        </Style.SocialCard>
        <Style.SocialCard $type="close">
          <h4>👥 Close IDs</h4>
          <div className="yield">0.5% shared</div>
          <p>
            Your recovery guardians share 0.5% of your activity. They help you recover your identity
            if devices are lost.
          </p>
        </Style.SocialCard>
      </Style.SocialGrid>

      <Style.DefinitionBox>
        <h4>🔐 Identity Recovery: No Seed Phrases Required</h4>
        <p>
          Lost your phone? Your Close IDs act as <strong>Guardian DIDs</strong>. Using threshold
          signatures (t-of-n), your remaining devices + guardian vouches can restore your identity
          to new hardware. No seed phrases, no central authority, no single point of failure. Your
          identity is distributed across your social graph—as long as you have <code>t</code>{' '}
          trusted votes, you're never locked out.
        </p>
      </Style.DefinitionBox>

      <Style.ContentCard>
        <h3>Close Identity Mechanics</h3>
        <p>
          Close IDs serve a <strong>dual purpose</strong>: they're your recovery guardians AND your
          social proof. The more reputation you build, the more close IDs you can have.
        </p>
        <Style.FeeTable>
          <thead>
            <tr>
              <th>Reputation</th>
              <th>Max Close IDs</th>
              <th>Each Earns</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>New user (0.0)</td>
              <td>5</td>
              <td>0.1% each</td>
            </tr>
            <tr>
              <td>Established (0.3)</td>
              <td>8</td>
              <td>0.0625% each</td>
            </tr>
            <tr>
              <td>Trusted (0.5)</td>
              <td>10</td>
              <td>0.05% each</td>
            </tr>
            <tr>
              <td>Veteran (1.0)</td>
              <td>15 (max)</td>
              <td>0.033% each</td>
            </tr>
          </tbody>
        </Style.FeeTable>
      </Style.ContentCard>

      <Style.SectionDivider />

      {/* LESSON 5: IDENTITY & DEVICE GRAPH */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 5: Identity &amp; The Device Graph</h3>
          <p>
            Your INOS identity isn't a username—it's a{' '}
            <strong>cryptographic distributed trust anchor</strong> (DID) spread across your
            devices. Adding more devices increases your UBI multiplier AND your resilience.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Interactive: Device Graph</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <DeviceGraphDiagram />
        <Style.IllustrationCaption>
          Each verified device adds +0.1% to your UBI multiplier—with no cap
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.CodeBlock>
        {`// From identity.capnp - Device binding
struct DeviceEntry {
  deviceId @0 :Text;            # device:<blake3(fingerprint)>
  nodeId @1 :Text;              # node:<blake3(device_id)>
  tier @6 :ResourceTier;        # light, moderate, heavy, dedicated
  profile @7 :ResourceProfile;  # SAB size, storage, CPU cores
  capabilities @8 :DeviceCapability;  # GPU, WebGPU, inference
}

enum ResourceTier {
  light @0;      # 32MB SAB, 5GB Storage
  moderate @1;   # 64MB SAB, 20GB Storage
  heavy @2;      # 128MB SAB, 100GB Storage
  dedicated @3;  # 256MB+ SAB, 500GB+ Storage
}`}
      </Style.CodeBlock>

      <Style.SectionDivider />

      {/* LESSON 6: GAMIFICATION */}
      <ScrollReveal>
        <Style.ContentCard>
          <h3>Lesson 6: Gamification &amp; Resource Tiers</h3>
          <p>
            INOS rewards <strong>good citizenship</strong> through a gamification layer. The more
            resources you contribute, the higher your UBI multiplier—turning participation into a
            game with real economic rewards.
          </p>
        </Style.ContentCard>
      </ScrollReveal>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Resource Tier Progression</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <GamificationTiersDiagram />
        <Style.IllustrationCaption>
          Resource tiers determine UBI multipliers—dedicated nodes earn 2x baseline
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.ContentCard>
        <h3>The Economic Engine: CreditSupervisor</h3>
        <p>
          Under the hood, the <strong>CreditSupervisor</strong> runs in the Go Kernel, managing all
          economic state directly in SharedArrayBuffer. Every epoch (60 seconds), it calculates
          earnings based on real resource contributions:
        </p>
      </Style.ContentCard>

      <Style.CodeBlock>
        {`// From credits.go - economic_tick calculation
func (cs *CreditSupervisor) economic_tick(metrics *ResourceMetrics, hours float64) int64 {
  // EARNED: Productive work rewards
  earned := (float64(metrics.ComputeCyclesUsed) * cs.rates.ComputeRate) +
    (float64(metrics.BytesServed) * cs.rates.BandwidthRate) +
    (float64(metrics.BytesStored) * cs.rates.StorageRate * hours) +
    (float64(metrics.UptimeSeconds) * cs.rates.UptimeRate) +
    (float64(metrics.LocalityScore) * cs.rates.LocalityBonus)
  
  // SPENT: Resource consumption costs
  spent := (float64(metrics.SyscallCount) * cs.rates.SyscallCost) *
    (1.0 + float64(metrics.MemoryPressure)) +
    (float64(metrics.ReplicationPriority) * cs.rates.ReplicationCost) +
    (float64(metrics.SchedulingBias) * cs.rates.SchedulingCost)
  
  return int64(earned - spent)  // Net delta per epoch
}`}
      </Style.CodeBlock>

      <Style.ContentCard>
        <h3>Multiplier Stacking</h3>
        <p>
          Multipliers are <strong>additive</strong>. A "Dedicated" user with 10 linked devices
          earns:
        </p>
        <Style.MultiplierBox>
          2.0x (tier) + (10 × 0.001) = <strong style={{ color: '#10b981' }}>2.01x</strong> UBI drip
        </Style.MultiplierBox>
        <p>
          The <strong>device multiplier</strong> (1.0 + devices × 0.001) is applied in the Kernel's
          UBI distribution loop. Add uptime bonuses, referral earnings, and royalty payouts, and a
          dedicated node operator can earn substantial passive income simply by being available.
        </p>
      </Style.ContentCard>

      <Style.CodeBlock>
        {`// From credits.go - ProcessUBIDrip applies multipliers
func (cs *CreditSupervisor) ProcessUBIDrip(epoch uint64) {
  baselineDrip := int64(1)  // 1 credit per epoch
  
  for id, offset := range cs.accounts {
    acc := cs.readAccount(offset)
    
    // Device multiplier: 1.0 + (devices * 0.001)
    multiplier := 1.0 + (float64(acc.DeviceCount) * 0.001)
    drip := int64(float64(baselineDrip) * multiplier)
    
    acc.Balance += drip
    acc.EarnedTotal += uint64(drip)
    cs.writeAccount(offset, acc)
  }
}`}
      </Style.CodeBlock>

      <Style.SectionDivider />

      <Style.ContentCard>
        <h3>The Vision: Everyone Becomes a Stakeholder</h3>
        <p>
          In INOS, there are no passive users. Your phone isn't just a consumer device—it's a{' '}
          <strong>miniature data center</strong> earning credits while you sleep. Your referrals
          aren't just friends—they're <strong>your income stream</strong>. Your identity isn't just
          a login—it's a <strong>yielding asset</strong> tied to the network's success.
        </p>
        <p style={{ marginBottom: 0 }}>
          This is the participation-first economy:{' '}
          <strong>use it, earn from it, grow with it</strong>.
        </p>
      </Style.ContentCard>

      <ChapterNav
        prev={{ to: '/deep-dives/mesh', title: 'P2P Mesh' }}
        next={{ to: '/deep-dives/threads', title: 'Supervisor Threads' }}
      />
    </Style.BlogContainer>
  );
}

export default Economy;
