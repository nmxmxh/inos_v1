/**
 * INOS Technical Codex — Architecture Page (Chapter 3)
 *
 * Comprehensive technical overview of the 3-layer architecture,
 * SAB memory layout, module system, and library proxy pattern.
 * This page is intentionally more technical.
 */

import { useCallback } from 'react';
import styled, { useTheme } from 'styled-components';
import * as d3 from 'd3';
import { NavLink } from 'react-router-dom';
import D3Container from '../ui/D3Container';
import { Style as ManuscriptStyle } from '../styles/manuscript';
import ChapterNav from '../ui/ChapterNav';
import ScrollReveal from '../ui/ScrollReveal';

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
    text-transform: uppercase;
    letter-spacing: 0.08em;
  `,

  LayerCard: styled.div<{ $color: string }>`
    background: rgba(255, 255, 255, 0.95);
    backdrop-filter: blur(8px);
    border: 1px solid ${p => p.$color}30;
    border-left: 4px solid ${p => p.$color};
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[4]} 0;
  `,

  LayerHeader: styled.div<{ $color: string }>`
    display: flex;
    align-items: center;
    gap: ${p => p.theme.spacing[3]};
    margin-bottom: ${p => p.theme.spacing[4]};
  `,

  LayerNumber: styled.span<{ $color: string }>`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    font-weight: 700;
    color: ${p => p.$color};
    text-transform: uppercase;
    letter-spacing: 0.1em;
  `,

  LayerTitle: styled.span<{ $color: string }>`
    font-size: 1.15rem;
    font-weight: 700;
    color: ${p => p.$color};
  `,

  CodeNote: styled.div`
    background: rgba(255, 255, 255, 0.92);
    backdrop-filter: blur(8px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-left: 3px solid ${p => p.theme.colors.accent};
    border-radius: 6px;
    padding: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[5]} 0;
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 11px;
    color: ${p => p.theme.colors.inkDark};
    line-height: 1.7;

    code {
      background: rgba(0, 0, 0, 0.06);
      padding: 2px 6px;
      border-radius: 3px;
      font-size: 10px;
    }

    ul {
      margin: ${p => p.theme.spacing[3]} 0 0 0;
      padding-left: ${p => p.theme.spacing[5]};
    }

    li {
      margin-bottom: ${p => p.theme.spacing[2]};
    }
  `,

  ModuleGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[5]} 0;
  `,

  ModuleCard: styled.div<{ $color: string }>`
    background: rgba(255, 255, 255, 0.85);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.$color}35;
    border-left: 3px solid ${p => p.$color};
    border-radius: 8px;
    padding: ${p => p.theme.spacing[4]};
  `,

  ModuleName: styled.div<{ $color: string }>`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 12px;
    font-weight: 700;
    color: ${p => p.$color};
    margin-bottom: ${p => p.theme.spacing[2]};
    text-transform: uppercase;
    letter-spacing: 0.05em;
  `,

  ModuleDesc: styled.div`
    font-size: ${p => p.theme.fontSizes.sm};
    color: ${p => p.theme.colors.inkMedium};
    line-height: 1.5;
  `,

  TierTable: styled.table`
    width: 100%;
    border-collapse: collapse;
    margin: ${p => p.theme.spacing[5]} 0;
    font-size: ${p => p.theme.fontSizes.sm};
    background: rgba(255, 255, 255, 0.88);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 8px;
    overflow: hidden;

    th,
    td {
      padding: ${p => p.theme.spacing[3]} ${p => p.theme.spacing[4]};
      text-align: left;
      border-bottom: 1px solid ${p => p.theme.colors.borderSubtle};
    }

    th {
      font-weight: 600;
      color: ${p => p.theme.colors.inkDark};
      background: rgba(0, 0, 0, 0.04);
    }

    td {
      color: ${p => p.theme.colors.inkDark};
    }

    tr:last-child td {
      border-bottom: none;
    }
  `,

  ProxySection: styled.div`
    background: rgba(139, 92, 246, 0.04);
    border: 1px solid rgba(139, 92, 246, 0.15);
    border-radius: 8px;
    padding: ${p => p.theme.spacing[6]};
    margin: ${p => p.theme.spacing[6]} 0;

    h3 {
      margin-top: 0;
      color: #8b5cf6;
    }

    ul {
      margin: ${p => p.theme.spacing[4]} 0;
      padding-left: ${p => p.theme.spacing[6]};
    }

    li {
      margin-bottom: ${p => p.theme.spacing[3]};
      line-height: 1.6;
    }
  `,

  Quote: styled.blockquote`
    font-family: ${p => p.theme.fonts.main};
    font-size: ${p => p.theme.fontSizes.lg};
    font-weight: ${p => p.theme.fontWeights.medium};
    color: ${p => p.theme.colors.accent};
    font-style: italic;
    margin: ${p => p.theme.spacing[8]} 0;
    padding: ${p => p.theme.spacing[5]} ${p => p.theme.spacing[6]};
    background: rgba(255, 255, 255, 0.85);
    backdrop-filter: blur(8px);
    border-left: 4px solid ${p => p.theme.colors.accent};
    border-radius: 0 8px 8px 0;
  `,

  StaticHostNote: styled.div`
    background: rgba(22, 163, 74, 0.1);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(22, 163, 74, 0.3);
    border-radius: 8px;
    padding: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[6]} 0;

    h4 {
      margin: 0 0 ${p => p.theme.spacing[3]} 0;
      color: #16a34a;
    }

    p {
      margin: 0;
      font-weight: 500;
      color: #1a1a1a;
      line-height: 1.6;
    }
  `,

  BuildGraphContainer: styled.div`
    background: rgba(0, 0, 0, 0.02);
    border: 1px dashed ${p => p.theme.colors.borderSubtle};
    border-radius: 8px;
    padding: ${p => p.theme.spacing[6]};
    margin: ${p => p.theme.spacing[6]} 0;
  `,

  DeepDiveGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[5]} 0;
  `,

  DeepDiveLink: styled(NavLink)<{ $color: string }>`
    display: block;
    background: rgba(255, 255, 255, 0.9);
    border: 1px solid ${p => p.$color}30;
    border-left: 3px solid ${p => p.$color};
    border-radius: 6px;
    padding: ${p => p.theme.spacing[4]};
    text-decoration: none;
    transition:
      transform 0.2s,
      box-shadow 0.2s;

    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
    }
  `,

  DeepDiveTitle: styled.div<{ $color: string }>`
    font-weight: 600;
    color: ${p => p.$color};
    margin-bottom: ${p => p.theme.spacing[1]};
  `,

  DeepDiveDesc: styled.div`
    font-size: ${p => p.theme.fontSizes.sm};
    color: ${p => p.theme.colors.inkMedium};
  `,
};

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: THREE-LAYER ARCHITECTURE (IMPROVED)
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: THREE-LAYER ARCHITECTURE (IMPROVED)
// ────────────────────────────────────────────────────────────────────────────
function ThreeLayerDiagram() {
  const theme = useTheme();

  const renderViz = useCallback(
    (
      svg: d3.Selection<SVGSVGElement, unknown, null, undefined>,
      _width: number,
      _height: number
    ) => {
      svg.selectAll('*').remove();

      // ViewBox: 650x360
      const layerWidth = 440;
      const layerX = 30;

      const layers = [
        {
          y: 35,
          label: 'LAYER 3: MODULES',
          sublabel: 'Rust WASM Compute Units',
          color: '#dea584',
          items: ['compute', 'storage', 'drivers', 'diagnostics'],
        },
        {
          y: 135,
          label: 'LAYER 2: KERNEL',
          sublabel: 'Go WASM Orchestration',
          color: '#00add8',
          items: ['Supervisors', 'Scheduler', 'Mesh', 'Economics'],
        },
        {
          y: 235,
          label: 'LAYER 1: HOST',
          sublabel: 'Browser / Static Server',
          color: '#f59e0b',
          items: ['React UI', 'Web APIs', 'WebRTC', 'IndexedDB'],
        },
      ];

      // Draw layers
      layers.forEach(layer => {
        // Layer background
        svg
          .append('rect')
          .attr('x', layerX)
          .attr('y', layer.y)
          .attr('width', layerWidth)
          .attr('height', 80)
          .attr('rx', 6)
          .attr('fill', `${layer.color}12`)
          .attr('stroke', layer.color)
          .attr('stroke-width', 1.5);

        // Layer label (centered)
        svg
          .append('text')
          .attr('x', layerX + layerWidth / 2)
          .attr('y', layer.y + 22)
          .attr('text-anchor', 'middle')
          .attr('font-size', 12)
          .attr('font-weight', 700)
          .attr('fill', layer.color)
          .attr('font-family', "'Inter', sans-serif")
          .text(layer.label);

        // Sublabel (centered)
        svg
          .append('text')
          .attr('x', layerX + layerWidth / 2)
          .attr('y', layer.y + 38)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9)
          .attr('fill', theme.colors.inkLight)
          .attr('font-family', "'Inter', sans-serif")
          .text(layer.sublabel);

        // Items in a row (centered)
        const itemWidth = 95;
        const itemGap = 12;
        const totalItemsWidth = layer.items.length * itemWidth + (layer.items.length - 1) * itemGap;
        const itemStartX = layerX + (layerWidth - totalItemsWidth) / 2;

        layer.items.forEach((item, i) => {
          const x = itemStartX + i * (itemWidth + itemGap);
          svg
            .append('rect')
            .attr('x', x)
            .attr('y', layer.y + 50)
            .attr('width', itemWidth)
            .attr('height', 22)
            .attr('rx', 3)
            .attr('fill', 'rgba(255,255,255,0.95)')
            .attr('stroke', `${layer.color}50`)
            .attr('stroke-width', 1);

          svg
            .append('text')
            .attr('x', x + itemWidth / 2)
            .attr('y', layer.y + 65)
            .attr('text-anchor', 'middle')
            .attr('font-size', 9)
            .attr('font-weight', 500)
            .attr('fill', theme.colors.inkDark)
            .attr('font-family', "'Inter', sans-serif")
            .text(item);
        });
      });

      // SAB connector on the right side
      const sabX = 500;
      const sabY = 70;
      const sabHeight = 210;

      svg
        .append('rect')
        .attr('x', sabX)
        .attr('y', sabY)
        .attr('width', 110)
        .attr('height', sabHeight)
        .attr('rx', 6)
        .attr('fill', 'rgba(139, 92, 246, 0.08)')
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 2);

      svg
        .append('text')
        .attr('x', sabX + 55)
        .attr('y', sabY + 30)
        .attr('text-anchor', 'middle')
        .attr('font-size', 12)
        .attr('font-weight', 700)
        .attr('fill', '#8b5cf6')
        .attr('font-family', "'Inter', sans-serif")
        .text('SAB');

      svg
        .append('text')
        .attr('x', sabX + 55)
        .attr('y', sabY + 50)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('fill', theme.colors.inkMedium)
        .attr('font-family', "'Inter', sans-serif")
        .text('Shared');

      svg
        .append('text')
        .attr('x', sabX + 55)
        .attr('y', sabY + 64)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('fill', theme.colors.inkMedium)
        .attr('font-family', "'Inter', sans-serif")
        .text('Memory');

      // Arrows from layers to SAB
      const arrowData = [
        { y: layers[0].y + 40, color: '#dea584' },
        { y: layers[1].y + 40, color: '#00add8' },
        { y: layers[2].y + 40, color: '#f59e0b' },
      ];

      arrowData.forEach(arrow => {
        svg
          .append('line')
          .attr('x1', layerX + layerWidth)
          .attr('y1', arrow.y)
          .attr('x2', sabX)
          .attr('y2', arrow.y)
          .attr('stroke', arrow.color)
          .attr('stroke-width', 1.5)
          .attr('stroke-dasharray', '5,3');

        // Arrow head
        svg
          .append('polygon')
          .attr(
            'points',
            `${sabX},${arrow.y} ${sabX - 7},${arrow.y - 4} ${sabX - 7},${arrow.y + 4}`
          )
          .attr('fill', arrow.color);
      });

      // Zero Copies label at bottom of SAB
      svg
        .append('text')
        .attr('x', sabX + 55)
        .attr('y', sabY + sabHeight - 45)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('font-weight', 600)
        .attr('fill', '#8b5cf6')
        .attr('font-family', "'Inter', sans-serif")
        .text('Zero');
      svg
        .append('text')
        .attr('x', sabX + 55)
        .attr('y', sabY + sabHeight - 32)
        .attr('text-anchor', 'middle')
        .attr('font-size', 9)
        .attr('font-weight', 600)
        .attr('fill', '#8b5cf6')
        .attr('font-family', "'Inter', sans-serif")
        .text('Copies');
    },
    [theme]
  );

  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 650 360" height={360} />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: SAB MEMORY MAP (UPDATED FOR NEW LAYOUT)
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: SAB MEMORY MAP (UPDATED FOR NEW LAYOUT)
// ────────────────────────────────────────────────────────────────────────────
function SABMemoryMapDiagram() {
  const theme = useTheme();

  const renderViz = useCallback(
    (
      svg: d3.Selection<SVGSVGElement, unknown, null, undefined>,
      _width: number,
      _height: number
    ) => {
      svg.selectAll('*').remove();

      const startX = 120;
      const barWidth = 380;

      // Updated memory regions from sab_layout.capnp (starts at 0x000000)
      const regions = [
        {
          offset: '0x000000',
          label: 'Atomic Flags',
          sublabel: 'Epochs, Mutexes',
          size: '128B',
          color: '#dc2626',
          height: 22,
        },
        {
          offset: '0x000140',
          label: 'Module Registry',
          sublabel: '64 modules',
          size: '6KB',
          color: '#8b5cf6',
          height: 28,
        },
        {
          offset: '0x002000',
          label: 'Supervisor Headers',
          sublabel: '32 supervisors',
          size: '4KB',
          color: '#16a34a',
          height: 26,
        },
        {
          offset: '0x004000',
          label: 'Economics + Identity',
          sublabel: 'Credits, DIDs',
          size: '32KB',
          color: '#00add8',
          height: 30,
        },
        {
          offset: '0x010000',
          label: 'Pattern Exchange',
          sublabel: 'Learned patterns',
          size: '64KB',
          color: '#f59e0b',
          height: 30,
        },
        {
          offset: '0x050000',
          label: 'Inbox / Outbox',
          sublabel: 'Ring buffers',
          size: '1MB',
          color: '#ec4899',
          height: 38,
        },
        {
          offset: '0x150000',
          label: 'Arena (Dynamic)',
          sublabel: 'Boids, Matrices, Overflow',
          size: '~30MB',
          color: '#6366f1',
          height: 60,
        },
      ];

      // Title
      svg
        .append('text')
        .attr('x', startX + barWidth / 2)
        .attr('y', 28)
        .attr('text-anchor', 'middle')
        .attr('font-size', 11)
        .attr('font-weight', 600)
        .attr('fill', theme.colors.inkDark)
        .attr('font-family', "'Inter', sans-serif")
        .text('SAB Memory Map (32MB Default)');

      // Draw regions
      let currentY = 50;
      regions.forEach(region => {
        // Region bar
        svg
          .append('rect')
          .attr('x', startX)
          .attr('y', currentY)
          .attr('width', barWidth)
          .attr('height', region.height)
          .attr('rx', 4)
          .attr('fill', `${region.color}15`)
          .attr('stroke', region.color)
          .attr('stroke-width', 1);

        // Offset label (left)
        svg
          .append('text')
          .attr('x', startX - 10)
          .attr('y', currentY + region.height / 2 + 4)
          .attr('text-anchor', 'end')
          .attr('font-size', 9)
          .attr('fill', theme.colors.inkLight)
          .attr('font-family', "'Inter', sans-serif")
          .text(region.offset);

        // Region label
        svg
          .append('text')
          .attr('x', startX + 12)
          .attr('y', currentY + region.height / 2 - 2)
          .attr('font-size', 10)
          .attr('font-weight', 600)
          .attr('fill', region.color)
          .attr('font-family', "'Inter', sans-serif")
          .text(region.label);

        // Sublabel
        if (region.height > 24) {
          svg
            .append('text')
            .attr('x', startX + 12)
            .attr('y', currentY + region.height / 2 + 12)
            .attr('font-size', 8)
            .attr('fill', theme.colors.inkMedium)
            .attr('font-family', "'Inter', sans-serif")
            .text(region.sublabel);
        }

        // Size (right)
        svg
          .append('text')
          .attr('x', startX + barWidth - 12)
          .attr('y', currentY + region.height / 2 + 4)
          .attr('text-anchor', 'end')
          .attr('font-size', 9)
          .attr('font-weight', 500)
          .attr('fill', region.color)
          .attr('font-family', "'Inter', sans-serif")
          .text(region.size);

        currentY += region.height + 6;
      });
    },
    [theme]
  );

  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 620 340" height={340} />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: LIBRARY PROXY PATTERN (IMPROVED)
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: LIBRARY PROXY PATTERN (IMPROVED)
// ────────────────────────────────────────────────────────────────────────────
function LibraryProxyDiagram() {
  const theme = useTheme();

  const renderViz = useCallback(
    (
      svg: d3.Selection<SVGSVGElement, unknown, null, undefined>,
      _width: number,
      _height: number
    ) => {
      svg.selectAll('*').remove();

      // ViewBox: 620x240 with centered layout
      const centerX = 310;
      const proxyWidth = 340;
      const proxyY = 155;
      const unitY = 35;
      const unitHeight = 50;
      const unitWidth = 110;

      // Central proxy box
      svg
        .append('rect')
        .attr('x', centerX - proxyWidth / 2)
        .attr('y', proxyY)
        .attr('width', proxyWidth)
        .attr('height', 55)
        .attr('rx', 6)
        .attr('fill', 'rgba(139, 92, 246, 0.1)')
        .attr('stroke', '#8b5cf6')
        .attr('stroke-width', 2);

      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', proxyY + 24)
        .attr('text-anchor', 'middle')
        .attr('font-size', 13)
        .attr('font-weight', 700)
        .attr('fill', '#8b5cf6')
        .attr('font-family', "'Inter', sans-serif")
        .text('UnitProxy Trait');

      svg
        .append('text')
        .attr('x', centerX)
        .attr('y', proxyY + 44)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('fill', theme.colors.inkMedium)
        .attr('font-family', "'Inter', sans-serif")
        .text('service_name() | actions() | execute()');

      // Units above - centered
      const units = [
        { label: 'Driver', color: '#f59e0b' },
        { label: 'Compute', color: '#dea584' },
        { label: 'Storage', color: '#16a34a' },
        { label: 'Diagnostics', color: '#ec4899' },
      ];

      const totalUnitsWidth = units.length * unitWidth + (units.length - 1) * 15;
      const unitsStartX = centerX - totalUnitsWidth / 2;

      units.forEach((unit, i) => {
        const x = unitsStartX + i * (unitWidth + 15);
        const unitCenterX = x + unitWidth / 2;

        // Unit box
        svg
          .append('rect')
          .attr('x', x)
          .attr('y', unitY)
          .attr('width', unitWidth)
          .attr('height', unitHeight)
          .attr('rx', 5)
          .attr('fill', 'rgba(255,255,255,0.95)')
          .attr('stroke', unit.color)
          .attr('stroke-width', 1.5);

        svg
          .append('text')
          .attr('x', unitCenterX)
          .attr('y', unitY + 22)
          .attr('text-anchor', 'middle')
          .attr('font-size', 11)
          .attr('font-weight', 600)
          .attr('fill', unit.color)
          .attr('font-family', "'Inter', sans-serif")
          .text(unit.label);

        svg
          .append('text')
          .attr('x', unitCenterX)
          .attr('y', unitY + 38)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9)
          .attr('fill', theme.colors.inkLight)
          .attr('font-family', "'Inter', sans-serif")
          .text('Unit');

        // Connecting line from unit to proxy
        svg
          .append('line')
          .attr('x1', unitCenterX)
          .attr('y1', unitY + unitHeight)
          .attr('x2', unitCenterX)
          .attr('y2', proxyY)
          .attr('stroke', '#8b5cf6')
          .attr('stroke-width', 1.5)
          .attr('stroke-dasharray', '5,3');

        // Arrow pointing down to proxy
        svg
          .append('polygon')
          .attr(
            'points',
            `${unitCenterX},${proxyY} ${unitCenterX - 5},${proxyY - 8} ${unitCenterX + 5},${proxyY - 8}`
          )
          .attr('fill', '#8b5cf6');
      });

      // "implements" label on the side
      svg
        .append('text')
        .attr('x', centerX + proxyWidth / 2 + 20)
        .attr('y', 115)
        .attr('font-size', 10)
        .attr('fill', '#8b5cf6')
        .attr('font-style', 'italic')
        .attr('font-family', "'Inter', sans-serif")
        .text('implements');
    },
    [theme]
  );

  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 620 240" height={240} />
  );
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: BUILD PIPELINE (FIXED)
// ────────────────────────────────────────────────────────────────────────────
// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: BUILD PIPELINE (FIXED)
// ────────────────────────────────────────────────────────────────────────────
function BuildPipelineDiagram() {
  const theme = useTheme();

  const renderViz = useCallback(
    (
      svg: d3.Selection<SVGSVGElement, unknown, null, undefined>,
      _width: number,
      _height: number
    ) => {
      svg.selectAll('*').remove();

      const stages = [
        {
          id: 'schema',
          label: "Cap'n Proto Schema",
          color: '#8b5cf6',
          details: 'Binary definition',
        },
        {
          id: 'gen',
          label: 'Code Generation',
          color: '#16a34a',
          details: 'Go / Rust / TS bindings',
        },
        { id: 'comp', label: 'WASM Compilation', color: '#00add8', details: 'Go / Rust LLVM' },
        { id: 'link', label: 'Runtime Link', color: '#f59e0b', details: 'SAB Memory Bind' },
      ];

      const stageWidth = 140;
      const startX = 40;
      const centerY = 100;

      stages.forEach((s, i) => {
        const x = startX + i * stageWidth;
        const g = svg.append('g').attr('transform', `translate(${x}, ${centerY})`);

        // Box
        g.append('rect')
          .attr('x', 0)
          .attr('y', -30)
          .attr('width', 120)
          .attr('height', 60)
          .attr('rx', 4)
          .attr('fill', `${s.color}10`)
          .attr('stroke', s.color)
          .attr('stroke-width', 1.5);

        // Label
        g.append('text')
          .attr('x', 60)
          .attr('y', 0)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9)
          .attr('font-weight', 700)
          .attr('fill', s.color)
          .text(s.label);

        // Details
        g.append('text')
          .attr('x', 60)
          .attr('y', 15)
          .attr('text-anchor', 'middle')
          .attr('font-size', 7)
          .attr('fill', theme.colors.inkMedium)
          .text(s.details);

        // Arrow to next
        if (i < stages.length - 1) {
          svg
            .append('line')
            .attr('x1', x + 120)
            .attr('y1', 0)
            .attr('x2', x + stageWidth)
            .attr('y2', 0)
            .attr('transform', `translate(0, ${centerY})`)
            .attr('stroke', theme.colors.borderSubtle)
            .attr('stroke-width', 1)
            .attr('stroke-dasharray', '3,2');
        }
      });
    },
    [theme]
  );

  return (
    <D3Container render={renderViz} dependencies={[renderViz]} viewBox="0 0 600 200" height={200} />
  );
}

export function Architecture() {
  return (
    <Style.BlogContainer>
      <Style.SectionTitle>Chapter 03</Style.SectionTitle>
      <Style.PageTitle>The Architecture</Style.PageTitle>

      <Style.LeadParagraph>
        Building a distributed compute platform inside a browser required solving problems that
        existing frameworks were never designed to handle. This chapter explains the four
        innovations that make INOS possible.
      </Style.LeadParagraph>

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION 1: THE CHALLENGE */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>The Challenge We Faced</h3>
        <p>
          Traditional web applications treat the browser as a thin client. Compute happens on
          servers. Data flows over networks. Every interaction pays a latency tax.
        </p>
        <p>
          But WebAssembly changed everything. For the first time, we could run Rust and Go at
          near-native speed in the browser. The question became: how do we make multiple WASM
          modules cooperate without the copy overhead that plagued server architectures?
        </p>
        <p style={{ marginBottom: 0 }}>
          The answer required rethinking how software components communicate. We needed innovations
          in <strong>memory sharing</strong>, <strong>layer separation</strong>,{' '}
          <strong>signaling</strong>, and <strong>interface design</strong>.
        </p>
      </Style.ContentCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION 2: INNOVATION #1 - ZERO-COPY */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>Innovation #1: Zero-Copy Memory</h3>
        <p>
          In traditional systems, when Module A wants to share data with Module B, it serializes the
          data, copies it, and Module B deserializes it. For a physics simulation running at 60 FPS
          with 10,000 entities, this copying alone could consume the entire frame budget.
        </p>
        <p>
          INOS eliminates this entirely. All layers share a single{' '}
          <strong>SharedArrayBuffer</strong> (SAB). When Rust writes bird positions to memory,
          JavaScript reads them directly. No copying. No serialization. Just pointer arithmetic.
        </p>
        <p style={{ marginBottom: 0 }}>
          The SAB is divided into fixed regions: atomic flags for signaling, registries for
          discovery, ring buffers for job queues, and a dynamic arena for compute buffers. Every
          offset is defined in a Cap'n Proto schema, ensuring all languages agree on the layout.
        </p>
      </Style.ContentCard>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>SAB Memory Map</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <SABMemoryMapDiagram />
        <Style.IllustrationCaption>
          All offsets defined in protocols/schemas/system/v1/sab_layout.capnp
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.CodeNote>
        <strong>Cap'n Proto as a Lens:</strong> We don't serialize Cap'n Proto messages. We use
        Cap'n Proto to <em>view</em> raw bytes as structured data. Reading a 64-bit float from a
        bird's position field is a single pointer offset, completed in nanoseconds regardless of how
        many birds exist.
      </Style.CodeNote>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION 3: INNOVATION #2 - THREE LAYERS */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>Innovation #2: The Three-Layer Architecture</h3>
        <p>
          A single language cannot excel at everything. JavaScript excels at UI rendering and
          browser integration. Go excels at concurrency and coordination. Rust excels at
          memory-safe, SIMD-optimized computation.
        </p>
        <p style={{ marginBottom: 0 }}>
          Rather than force a compromise, INOS lets each language do what it does best. The three
          layers communicate through the shared SAB, never calling functions across boundaries.
        </p>
      </Style.ContentCard>

      <ScrollReveal>
        <Style.IllustrationContainer>
          <Style.IllustrationHeader>
            <Style.IllustrationTitle>Three-Layer Architecture</Style.IllustrationTitle>
          </Style.IllustrationHeader>
          <ThreeLayerDiagram />
          <Style.IllustrationCaption>
            Each layer operates independently, synchronized through shared memory
          </Style.IllustrationCaption>
        </Style.IllustrationContainer>
      </ScrollReveal>

      <Style.LayerCard $color="#f59e0b">
        <Style.LayerHeader $color="#f59e0b">
          <Style.LayerNumber $color="#f59e0b">Layer 1</Style.LayerNumber>
          <Style.LayerTitle $color="#f59e0b">The Host (JavaScript)</Style.LayerTitle>
        </Style.LayerHeader>
        <p>
          The Host layer handles everything the browser provides: DOM rendering via React, camera
          and sensor access via Web APIs, network transport via WebRTC. It reads computed state from
          the SAB and renders it at 60 FPS.
        </p>
        <p style={{ marginBottom: 0 }}>
          <strong>Why JavaScript:</strong> Direct access to browser APIs. Mature ecosystem.
          Excellent developer tooling. The Host never performs heavy computation.
        </p>
      </Style.LayerCard>

      <Style.LayerCard $color="#00add8">
        <Style.LayerHeader $color="#00add8">
          <Style.LayerNumber $color="#00add8">Layer 2</Style.LayerNumber>
          <Style.LayerTitle $color="#00add8">The Kernel (Go)</Style.LayerTitle>
        </Style.LayerHeader>
        <p>
          The Kernel layer orchestrates everything. Supervisors manage module lifecycles. Genetic
          algorithms optimize parameters. A mesh coordinator handles P2P discovery. An economic
          engine tracks credits.
        </p>
        <p style={{ marginBottom: 0 }}>
          <strong>Why Go:</strong> Goroutines for concurrent supervision without callback hell. The
          Go runtime's garbage collector is isolated to its own memory, never pausing the Rust
          modules or JavaScript rendering.
        </p>
      </Style.LayerCard>

      <Style.LayerCard $color="#dea584">
        <Style.LayerHeader $color="#dea584">
          <Style.LayerNumber $color="#dea584">Layer 3</Style.LayerNumber>
          <Style.LayerTitle $color="#dea584">The Modules (Rust)</Style.LayerTitle>
        </Style.LayerHeader>
        <p>
          Rust modules are the compute workhorses. Physics simulations, cryptographic operations,
          image processing, audio encoding. They write results directly to SAB buffers, achieving
          near-native performance with compile-time memory safety.
        </p>
        <p style={{ marginBottom: 0 }}>
          <strong>Why Rust:</strong> SIMD vectorization, no garbage collection pauses, and the
          ability to operate directly on SAB memory views. A ping-pong buffer pattern eliminates
          read-write contention.
        </p>
      </Style.LayerCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION 4: INNOVATION #3 - SIGNALING */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>Innovation #3: Epoch-Based Signaling</h3>
        <p>
          With shared memory, the next problem is coordination. How does JavaScript know when Rust
          has finished writing new bird positions? Polling wastes CPU. Callbacks require function
          calls across layers.
        </p>
        <p>
          INOS uses <strong>epoch counters</strong>. Each major data region has an atomic integer.
          When a producer finishes writing, it increments the epoch. Consumers compare their
          last-seen epoch to the current value. Changed? Read the new data. Same? Skip the cycle.
        </p>
        <ol>
          <li>
            <strong>Mutate:</strong> Rust computes new positions and writes them to SAB Buffer B
          </li>
          <li>
            <strong>Signal:</strong> Rust atomically increments the Bird Epoch counter
          </li>
          <li>
            <strong>React:</strong> JavaScript sees the epoch changed and reads from Buffer B
          </li>
        </ol>
        <p style={{ marginBottom: 0 }}>
          This pattern enables natural debouncing, efficient idle detection via{' '}
          <code>Atomics.wait()</code>, and the ability to replay state changes for debugging.
        </p>
      </Style.ContentCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION 5: INNOVATION #4 - LIBRARY PROXY */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>Innovation #4: The Library Proxy Pattern</h3>
        <p>
          With multiple Rust modules (compute, storage, drivers, diagnostics), the Kernel needs a
          way to discover and invoke them uniformly. Rather than hardcoded integrations, every
          module implements a single trait: <code>UnitProxy</code>.
        </p>
        <p style={{ marginBottom: 0 }}>
          This pattern means new modules can be added without changing the Kernel. The module
          registry in the SAB lists all available units, their capabilities, and their cost models.
          The Kernel reads this registry and dispatches work accordingly.
        </p>
      </Style.ContentCard>

      <Style.IllustrationContainer>
        <Style.IllustrationHeader>
          <Style.IllustrationTitle>Library Proxy Pattern</Style.IllustrationTitle>
        </Style.IllustrationHeader>
        <LibraryProxyDiagram />
        <Style.IllustrationCaption>
          Every unit implements the same trait, enabling dynamic discovery
        </Style.IllustrationCaption>
      </Style.IllustrationContainer>

      <Style.ModuleGrid>
        <Style.ModuleCard $color="#dea584">
          <Style.ModuleName $color="#dea584">Compute</Style.ModuleName>
          <Style.ModuleDesc>
            Audio encoding, cryptography, data compression, GPU shaders, image processing, physics
            simulation, external API calls.
          </Style.ModuleDesc>
        </Style.ModuleCard>
        <Style.ModuleCard $color="#16a34a">
          <Style.ModuleName $color="#16a34a">Storage</Style.ModuleName>
          <Style.ModuleDesc>
            ChaCha20 encryption, Brotli compression, BLAKE3 content hashing. Secure blob management
            for the P2P mesh.
          </Style.ModuleDesc>
        </Style.ModuleCard>
        <Style.ModuleCard $color="#f59e0b">
          <Style.ModuleName $color="#f59e0b">Drivers</Style.ModuleName>
          <Style.ModuleDesc>
            I/O abstractions for browser sensors (camera, GPS, accelerometer) and actors (audio
            output, haptics, notifications).
          </Style.ModuleDesc>
        </Style.ModuleCard>
        <Style.ModuleCard $color="#ec4899">
          <Style.ModuleName $color="#ec4899">Diagnostics</Style.ModuleName>
          <Style.ModuleDesc>
            Health metrics, performance counters, heartbeat monitoring. Writes to SAB for real-time
            dashboards.
          </Style.ModuleDesc>
        </Style.ModuleCard>
      </Style.ModuleGrid>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION 6: THE RESULT */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>What This Enables</h3>
        <p>
          These four innovations combine to create something that was previously impossible: a
          fully-featured distributed compute platform running entirely in the browser.
        </p>
        <ul>
          <li>
            <strong>No backend required.</strong> INOS compiles to static files. Deploy to any CDN.
            The P2P mesh handles coordination between browsers.
          </li>
          <li>
            <strong>Near-native performance.</strong> SIMD physics, GPU compute, and zero-copy
            rendering achieve frame rates that rival native applications.
          </li>
          <li>
            <strong>Adaptive resource usage.</strong> Mobile devices run lighter workloads.
            Workstations contribute more. Everyone participates in proportion to their capability.
          </li>
        </ul>
      </Style.ContentCard>

      <Style.StaticHostNote>
        <h4>🌐 No Servers Required</h4>
        <p>
          The entire INOS stack compiles to HTML, JavaScript, and WebAssembly. Upload to GitHub
          Pages, Vercel, or Cloudflare Pages. No Node.js process. No Docker container. No AWS bill.
          Your CDN serves the static files, and the P2P mesh handles the rest.
        </p>
      </Style.StaticHostNote>

      <Style.TierTable>
        <thead>
          <tr>
            <th>Device Tier</th>
            <th>Example</th>
            <th>SAB Size</th>
            <th>P2P Role</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>Light</td>
            <td>Mobile phone, IoT device</td>
            <td>32MB</td>
            <td>Heartbeat only</td>
          </tr>
          <tr>
            <td>Moderate</td>
            <td>Laptop, tablet</td>
            <td>64MB</td>
            <td>Gossip participant</td>
          </tr>
          <tr>
            <td>Heavy</td>
            <td>Desktop workstation</td>
            <td>128MB</td>
            <td>Full DHT node</td>
          </tr>
          <tr>
            <td>Dedicated</td>
            <td>Server, dedicated node</td>
            <td>256MB+</td>
            <td>Relay and seed</td>
          </tr>
        </tbody>
      </Style.TierTable>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION 6: THE IMPLEMENTATION DETAIL */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>The Implementation Detail</h3>
        <p>
          Architecture remains a theory until it is built. In INOS, the implementation is governed
          by <strong>Schema-First Development</strong>. We use Cap'n Proto to define the geometry of
          our shared reality (the SAB layout) before a single line of application logic is written.
        </p>

        <Style.IllustrationContainer>
          <Style.IllustrationHeader>
            <Style.IllustrationTitle>Fig_04 // The Build Pipeline</Style.IllustrationTitle>
          </Style.IllustrationHeader>
          <BuildPipelineDiagram />
          <Style.IllustrationCaption>
            Automated synchronization between schema definitions and multi-language binaries.
          </Style.IllustrationCaption>
        </Style.IllustrationContainer>

        <p>
          This pipeline ensures that when the Rust <code>muscle</code> writes a matrix to the
          SharedArrayBuffer, the TypeScript <code>sensory</code> layer knows exactly where to read
          it, down to the byte.
        </p>

        <Style.CodeNote>
          <strong>Implementation Checklist:</strong>
          <ul>
            <li>
              <strong>Uniformity</strong>: Every module exposes the <code>UnitProxy</code> trait.
            </li>
            <li>
              <strong>Hot-Reloading</strong>: Modules are compiled to independent WASM files.
            </li>
            <li>
              <strong>Deterministic Linking</strong>: The Go Kernel maps memory regions at boot.
            </li>
          </ul>
        </Style.CodeNote>
      </Style.ContentCard>

      <Style.SectionDivider />

      {/* ══════════════════════════════════════════════════════════════════════ */}
      {/* SECTION 7: DEEP DIVES */}
      {/* ══════════════════════════════════════════════════════════════════════ */}
      <Style.ContentCard>
        <h3>Continue the Journey</h3>
        <p>Each innovation has its own deep-dive page with interactive visualizations:</p>
        <Style.DeepDiveGrid>
          <Style.DeepDiveLink to="/deep-dives/zero-copy" $color="#8b5cf6">
            <Style.DeepDiveTitle $color="#8b5cf6">Zero-Copy I/O</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>SAB pointers vs data copying</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
          <Style.DeepDiveLink to="/deep-dives/signaling" $color="#dc2626">
            <Style.DeepDiveTitle $color="#dc2626">Epoch Signaling</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>Reactive mutation patterns</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
          <Style.DeepDiveLink to="/deep-dives/mesh" $color="#16a34a">
            <Style.DeepDiveTitle $color="#16a34a">P2P Mesh</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>Gossip + DHT + Reputation</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
          <Style.DeepDiveLink to="/deep-dives/economy" $color="#f59e0b">
            <Style.DeepDiveTitle $color="#f59e0b">Economic Storage</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>Credits and storage tiers</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
          <Style.DeepDiveLink to="/deep-dives/threads" $color="#00add8">
            <Style.DeepDiveTitle $color="#00add8">Supervisor Threads</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>Genetic algorithms + coordination</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
          <Style.DeepDiveLink to="/deep-dives/graphics" $color="#ec4899">
            <Style.DeepDiveTitle $color="#ec4899">Graphics Pipeline</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>WebGPU + instanced rendering</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
          <Style.DeepDiveLink to="/deep-dives/database" $color="#10b981">
            <Style.DeepDiveTitle $color="#10b981">Database & Storage</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>OPFS + BLAKE3 content addressing</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
        </Style.DeepDiveGrid>
      </Style.ContentCard>

      <ChapterNav
        prev={{ to: '/insight', title: '02. The Insight' }}
        next={{ to: '/genesis', title: '04. Genesis' }}
      />
    </Style.BlogContainer>
  );
}

export default Architecture;
