/**
 * INOS Technical Codex — Insight Page (Chapter 2)
 *
 * Framing Unified Memory as the "Circulatory System".
 * Explains how WebAssembly enables the zero-copy architecture.
 * Shows the boids pipeline: Rust Compute → Go Supervisor → JS Render.
 */

import { useEffect, useRef } from 'react';
import styled, { useTheme } from 'styled-components';
import * as d3 from 'd3';
import { Style as ManuscriptStyle } from '../styles/manuscript';
import ChapterNav from '../ui/ChapterNav';
import ScrollReveal from '../ui/ScrollReveal';

const Style = {
  ...ManuscriptStyle,

  Quote: styled.blockquote`
    font-family: ${p => p.theme.fonts.main};
    font-size: ${p => p.theme.fontSizes.xl};
    font-weight: ${p => p.theme.fontWeights.medium};
    color: ${p => p.theme.colors.accent};
    font-style: italic;
    margin: ${p => p.theme.spacing[8]} 0;
    padding-left: ${p => p.theme.spacing[6]};
    border-left: 4px solid ${p => p.theme.colors.accent};
    background: rgba(255, 255, 255, 0.8);
    backdrop-filter: blur(8px);
    padding: ${p => p.theme.spacing[4]} ${p => p.theme.spacing[6]};
    border-radius: 0 6px 6px 0;
  `,

  IllustrationContainer: styled.div`
    width: 100%;
    background: rgba(255, 255, 255, 0.75);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 6px;
    margin: ${p => p.theme.spacing[6]} 0;
    overflow: hidden;
  `,

  IllustrationHeader: styled.div`
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: ${p => p.theme.spacing[3]} ${p => p.theme.spacing[4]};
    border-bottom: 1px solid ${p => p.theme.colors.borderSubtle};
  `,

  IllustrationTitle: styled.span`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    font-weight: 500;
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
    letter-spacing: 0.1em;
  `,

  ContentCard: styled.div`
    background: rgba(255, 255, 255, 0.85);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 6px;
    padding: ${p => p.theme.spacing[5]};
    margin: ${p => p.theme.spacing[5]} 0;

    h3 {
      margin-top: 0;
    }

    p {
      line-height: 1.7;
    }

    ul {
      margin: ${p => p.theme.spacing[4]} 0;
    }

    li {
      margin-bottom: ${p => p.theme.spacing[3]};
    }
  `,

  CodeNote: styled.div`
    background: rgba(255, 255, 255, 0.9);
    backdrop-filter: blur(8px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-left: 3px solid ${p => p.theme.colors.accent};
    border-radius: 4px;
    padding: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[4]} 0;
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 11px;
    color: ${p => p.theme.colors.inkDark};
    line-height: 1.6;

    code {
      background: rgba(0, 0, 0, 0.06);
      padding: 2px 6px;
      border-radius: 3px;
      font-size: 10px;
    }
  `,

  SpinningBirdNote: styled.div`
    background: rgba(251, 191, 36, 0.15);
    backdrop-filter: blur(8px);
    border: 1px dashed rgba(251, 191, 36, 0.6);
    border-radius: 6px;
    padding: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[6]} 0;
    font-size: ${p => p.theme.fontSizes.sm};
    font-weight: 500;
    color: ${p => p.theme.colors.inkDark};
  `,

  RuleCard: styled.div`
    background: rgba(139, 92, 246, 0.08);
    backdrop-filter: blur(8px);
    border: 1px solid rgba(139, 92, 246, 0.2);
    border-radius: 6px;
    padding: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[3]} 0;
    display: flex;
    gap: ${p => p.theme.spacing[4]};
    align-items: flex-start;
  `,

  RuleNumber: styled.span`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 20px;
    font-weight: 700;
    color: #8b5cf6;
    min-width: 32px;
  `,

  RuleContent: styled.div`
    flex: 1;

    strong {
      color: #8b5cf6;
    }
  `,

  StepGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[5]} 0;
  `,

  StepCard: styled.div<{ $color: string }>`
    background: rgba(255, 255, 255, 0.9);
    backdrop-filter: blur(8px);
    border: 1px solid ${p => p.$color}40;
    border-left: 4px solid ${p => p.$color};
    border-radius: 6px;
    padding: ${p => p.theme.spacing[4]};
  `,

  StepLabel: styled.div<{ $color: string }>`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 9px;
    font-weight: 600;
    color: ${p => p.$color};
    text-transform: uppercase;
    letter-spacing: 0.1em;
    margin-bottom: ${p => p.theme.spacing[2]};
  `,

  StepTitle: styled.div`
    font-weight: 600;
    font-size: ${p => p.theme.fontSizes.base};
    color: ${p => p.theme.colors.inkDark};
    margin-bottom: ${p => p.theme.spacing[2]};
  `,

  StepDesc: styled.div`
    font-size: ${p => p.theme.fontSizes.sm};
    color: ${p => p.theme.colors.inkMedium};
    line-height: 1.5;
  `,
};

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: WASM vs TRADITIONAL
// ────────────────────────────────────────────────────────────────────────────
function WasmComparisonDiagram() {
  const svgRef = useRef<SVGSVGElement>(null);
  const theme = useTheme();

  useEffect(() => {
    if (!svgRef.current) return;

    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const width = 700;
    const height = 320;
    const midX = width / 2;

    const leftX = 160;
    const rightX = 540;

    // Draw divider
    svg
      .append('line')
      .attr('x1', midX)
      .attr('y1', 50)
      .attr('x2', midX)
      .attr('y2', height - 30)
      .attr('stroke', '#d1d5db')
      .attr('stroke-width', 1)
      .attr('stroke-dasharray', '6,4');

    // Headers
    svg
      .append('text')
      .attr('x', leftX)
      .attr('y', 35)
      .attr('text-anchor', 'middle')
      .attr('font-size', 11)
      .attr('font-weight', 700)
      .attr('fill', '#dc2626')
      .attr('font-family', "'Inter', sans-serif")
      .text('TRADITIONAL BROWSER');

    svg
      .append('text')
      .attr('x', rightX)
      .attr('y', 35)
      .attr('text-anchor', 'middle')
      .attr('font-size', 11)
      .attr('font-weight', 700)
      .attr('fill', '#16a34a')
      .attr('font-family', "'Inter', sans-serif")
      .text('INOS + WEBASSEMBLY');

    // Traditional: Separate memory spaces with copies
    const tradLayers = [
      { y: 65, label: 'Backend (Node.js)', sublabel: 'Own memory space', color: '#68a063' },
      {
        y: 115,
        label: 'JSON.stringify()',
        sublabel: '+40% overhead',
        color: '#dc2626',
        isCopy: true,
      },
      { y: 165, label: 'Frontend (React)', sublabel: 'Own memory space', color: '#61dafb' },
      {
        y: 215,
        label: 'postMessage()',
        sublabel: 'Structured clone',
        color: '#dc2626',
        isCopy: true,
      },
      { y: 265, label: 'Web Worker', sublabel: 'Own memory space', color: '#f59e0b' },
    ];

    tradLayers.forEach((layer, i) => {
      svg
        .append('rect')
        .attr('x', leftX - 80)
        .attr('y', layer.y)
        .attr('width', 160)
        .attr('height', 38)
        .attr('rx', 4)
        .attr('fill', layer.isCopy ? 'rgba(220, 38, 38, 0.1)' : 'rgba(255,255,255,0.95)')
        .attr('stroke', layer.color)
        .attr('stroke-width', layer.isCopy ? 1.5 : 1);

      svg
        .append('text')
        .attr('x', leftX)
        .attr('y', layer.y + 16)
        .attr('text-anchor', 'middle')
        .attr('font-size', 10)
        .attr('font-weight', layer.isCopy ? 600 : 500)
        .attr('fill', layer.isCopy ? '#dc2626' : theme.colors.inkDark)
        .attr('font-family', "'Inter', sans-serif")
        .text(layer.label);

      svg
        .append('text')
        .attr('x', leftX)
        .attr('y', layer.y + 30)
        .attr('text-anchor', 'middle')
        .attr('font-size', 8)
        .attr('fill', layer.isCopy ? '#dc2626' : theme.colors.inkLight)
        .attr('font-family', "'Inter', sans-serif")
        .text(layer.sublabel);

      if (i < tradLayers.length - 1) {
        svg
          .append('line')
          .attr('x1', leftX)
          .attr('y1', layer.y + 38)
          .attr('x2', leftX)
          .attr('y2', tradLayers[i + 1].y)
          .attr('stroke', tradLayers[i + 1].isCopy ? '#dc2626' : '#d1d5db')
          .attr('stroke-width', tradLayers[i + 1].isCopy ? 1.5 : 1)
          .attr('stroke-dasharray', tradLayers[i + 1].isCopy ? '4,2' : 'none');
      }
    });

    // WASM side: Single shared memory
    const sabY = 140;
    const sabHeight = 110;

    // Central SAB rectangle
    svg
      .append('rect')
      .attr('x', rightX - 100)
      .attr('y', sabY)
      .attr('width', 200)
      .attr('height', sabHeight)
      .attr('rx', 6)
      .attr('fill', 'rgba(22, 163, 74, 0.08)')
      .attr('stroke', '#16a34a')
      .attr('stroke-width', 2);

    svg
      .append('text')
      .attr('x', rightX)
      .attr('y', sabY + 24)
      .attr('text-anchor', 'middle')
      .attr('font-size', 12)
      .attr('font-weight', 700)
      .attr('fill', '#16a34a')
      .attr('font-family', "'Inter', sans-serif")
      .text('SharedArrayBuffer');

    svg
      .append('text')
      .attr('x', rightX)
      .attr('y', sabY + 42)
      .attr('text-anchor', 'middle')
      .attr('font-size', 9)
      .attr('fill', theme.colors.inkMedium)
      .attr('font-family', "'Inter', sans-serif")
      .text('One Memory. All Threads.');

    svg
      .append('text')
      .attr('x', rightX)
      .attr('y', sabY + 58)
      .attr('text-anchor', 'middle')
      .attr('font-size', 8)
      .attr('fill', theme.colors.inkLight)
      .attr('font-family', "'Inter', sans-serif")
      .text('Same physical addresses');

    // Modules pointing to SAB
    const wasmModules = [
      { x: rightX - 65, label: 'Go\nKernel', color: '#00add8' },
      { x: rightX, label: 'Rust\nCompute', color: '#dea584' },
      { x: rightX + 65, label: 'JS\nRender', color: '#f59e0b' },
    ];

    wasmModules.forEach(mod => {
      const y = sabY + 75;
      svg
        .append('rect')
        .attr('x', mod.x - 28)
        .attr('y', y)
        .attr('width', 56)
        .attr('height', 28)
        .attr('rx', 3)
        .attr('fill', 'rgba(255,255,255,0.95)')
        .attr('stroke', mod.color)
        .attr('stroke-width', 1.5);

      const lines = mod.label.split('\n');
      lines.forEach((line, i) => {
        svg
          .append('text')
          .attr('x', mod.x)
          .attr('y', y + 12 + i * 10)
          .attr('text-anchor', 'middle')
          .attr('font-size', 9)
          .attr('font-weight', 600)
          .attr('fill', mod.color)
          .attr('font-family', "'Inter', sans-serif")
          .text(line);
      });
    });
  }, [theme]);

  return <svg ref={svgRef} viewBox="0 0 700 320" style={{ width: '100%', height: 'auto' }} />;
}

// ────────────────────────────────────────────────────────────────────────────
// D3 ILLUSTRATION: BOIDS DATA FLOW PIPELINE
// ────────────────────────────────────────────────────────────────────────────
function BoidsDataFlowDiagram() {
  const svgRef = useRef<SVGSVGElement>(null);
  const theme = useTheme();

  useEffect(() => {
    if (!svgRef.current) return;

    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const width = 700;
    const height = 420;
    const centerX = width / 2;

    // Title
    svg
      .append('text')
      .attr('x', centerX)
      .attr('y', 25)
      .attr('text-anchor', 'middle')
      .attr('font-size', 11)
      .attr('font-weight', 600)
      .attr('fill', theme.colors.inkDark)
      .attr('font-family', "'Inter', sans-serif")
      .text('One Frame: 16ms Budget');

    // Pipeline stages (vertical flow)
    const pipeline = [
      {
        y: 55,
        label: 'RUST BOIDS UNIT',
        file: 'boids.rs',
        color: '#dea584',
        ops: [
          'Read positions from Buffer A',
          'Spatial hash (O(n) neighbors)',
          'Apply 3 flocking rules',
          'Write to Buffer B',
          'Flip active buffer',
        ],
      },
      {
        y: 175,
        label: 'GO SUPERVISOR',
        file: 'boids_supervisor.go',
        color: '#00add8',
        ops: [
          'Read population genes',
          'Calculate fitness: survival + cohesion',
          'Tournament selection (k=5)',
          'Crossover + mutation',
          'Write evolved genes back',
        ],
      },
      {
        y: 295,
        label: 'JS RENDERER',
        file: 'ArchitecturalBoids.tsx',
        color: '#f59e0b',
        ops: [
          'Check epoch flag (Atomics)',
          'Read matrix buffer',
          'Update instanceMatrix',
          'Three.js instanced draw',
          '1000 birds in 1 draw call',
        ],
      },
    ];

    pipeline.forEach((stage, idx) => {
      // Stage box
      svg
        .append('rect')
        .attr('x', 60)
        .attr('y', stage.y)
        .attr('width', 580)
        .attr('height', 100)
        .attr('rx', 6)
        .attr('fill', 'rgba(255,255,255,0.95)')
        .attr('stroke', stage.color)
        .attr('stroke-width', 1.5);

      // Label
      svg
        .append('text')
        .attr('x', 80)
        .attr('y', stage.y + 22)
        .attr('font-size', 11)
        .attr('font-weight', 700)
        .attr('fill', stage.color)
        .attr('font-family', "'Inter', sans-serif")
        .text(stage.label);

      // File
      svg
        .append('text')
        .attr('x', 80)
        .attr('y', stage.y + 36)
        .attr('font-size', 9)
        .attr('fill', theme.colors.inkLight)
        .attr('font-family', "'Inter', sans-serif")
        .text(stage.file);

      // Operations as horizontal flow
      const opStartX = 90;
      const opSpacing = 110;
      stage.ops.forEach((op, i) => {
        const x = opStartX + i * opSpacing;
        svg
          .append('rect')
          .attr('x', x)
          .attr('y', stage.y + 50)
          .attr('width', 100)
          .attr('height', 36)
          .attr('rx', 3)
          .attr('fill', `${stage.color}15`)
          .attr('stroke', `${stage.color}40`)
          .attr('stroke-width', 1);

        // Wrap text
        const words = op.split(' ');
        let line1 = '';
        let line2 = '';
        words.forEach(w => {
          if (line1.length + w.length < 14) {
            line1 += (line1 ? ' ' : '') + w;
          } else {
            line2 += (line2 ? ' ' : '') + w;
          }
        });

        svg
          .append('text')
          .attr('x', x + 50)
          .attr('y', stage.y + 65 + (line2 ? 0 : 5))
          .attr('text-anchor', 'middle')
          .attr('font-size', 8)
          .attr('fill', theme.colors.inkDark)
          .attr('font-family', "'Inter', sans-serif")
          .text(line1);

        if (line2) {
          svg
            .append('text')
            .attr('x', x + 50)
            .attr('y', stage.y + 77)
            .attr('text-anchor', 'middle')
            .attr('font-size', 8)
            .attr('fill', theme.colors.inkDark)
            .attr('font-family', "'Inter', sans-serif")
            .text(line2);
        }

        // Arrow to next op
        if (i < stage.ops.length - 1) {
          svg
            .append('text')
            .attr('x', x + 104)
            .attr('y', stage.y + 70)
            .attr('font-size', 10)
            .attr('fill', stage.color)
            .text('→');
        }
      });

      // Arrow to next stage
      if (idx < pipeline.length - 1) {
        svg
          .append('line')
          .attr('x1', centerX)
          .attr('y1', stage.y + 100)
          .attr('x2', centerX)
          .attr('y2', pipeline[idx + 1].y)
          .attr('stroke', '#8b5cf6')
          .attr('stroke-width', 2)
          .attr('marker-end', 'url(#arrow)');

        svg
          .append('text')
          .attr('x', centerX + 15)
          .attr('y', stage.y + 115)
          .attr('font-size', 8)
          .attr('fill', '#8b5cf6')
          .attr('font-family', "'Inter', sans-serif")
          .text('via SAB');
      }
    });

    // Bottom label
    svg
      .append('text')
      .attr('x', centerX)
      .attr('y', height - 10)
      .attr('text-anchor', 'middle')
      .attr('font-size', 9)
      .attr('fill', theme.colors.inkMedium)
      .attr('font-family', "'Inter', sans-serif")
      .text('All communication via SharedArrayBuffer. Zero copies. Zero serialization.');
  }, [theme]);

  return <svg ref={svgRef} viewBox="0 0 700 420" style={{ width: '100%', height: 'auto' }} />;
}

export function Insight() {
  return (
    <Style.BlogContainer>
      <Style.SectionTitle>Chapter 02</Style.SectionTitle>
      <Style.PageTitle>The Circulatory System</Style.PageTitle>

      <Style.LeadParagraph>
        <strong>What if data didn't move at all?</strong> What if the CPU, the GPU, and the Network
        all looked at the same exact physical memory addresses?
      </Style.LeadParagraph>

      <Style.Quote>
        "Data should flow through a system like blood through a body—without the heart needing to
        stop and explain the blood to the lungs."
      </Style.Quote>

      <Style.ContentCard>
        <p style={{ fontWeight: 600, fontSize: '1.1rem', color: '#1a1a1a', marginBottom: '1rem' }}>
          This is not a metaphor. This is what WebAssembly + SharedArrayBuffer enables.
        </p>
        <p>
          For the first time in browser history, multiple threads can share the same physical memory
          addresses. Rust writes to offset <code>0x1000</code>. JavaScript reads it. No copies. No
          JSON. No <code>postMessage</code>. Static compilation to near-native speed.
        </p>
      </Style.ContentCard>

      {/* WASM vs Traditional Comparison */}
      <ScrollReveal>
        <Style.IllustrationContainer>
          <Style.IllustrationHeader>
            <Style.IllustrationTitle>
              Traditional Browser vs INOS Architecture
            </Style.IllustrationTitle>
          </Style.IllustrationHeader>
          <WasmComparisonDiagram />
          <Style.IllustrationCaption>
            WebAssembly + SAB: One memory, three languages, zero copies
          </Style.IllustrationCaption>
        </Style.IllustrationContainer>
      </ScrollReveal>

      {/* Boids Algorithm Introduction */}
      <Style.ContentCard>
        <h3>The Boids Algorithm: Nature in Code</h3>
        <p>
          Look at the birds flying behind this text. They are not animated sprites. They are
          simulating <strong>emergent behavior</strong>. Each bird follows three simple rules, and
          from these rules, complex flocking emerges.
        </p>
        <p>
          Craig Reynolds invented this algorithm in 1986. Traditionally, it requires{' '}
          <strong>O(n²)</strong> calculations per frame: every bird must check every other bird.
          With 1,000 birds at 60 FPS, that is <strong>60 million calculations per second</strong>.
        </p>
      </Style.ContentCard>

      {/* The Three Rules */}
      <Style.RuleCard>
        <Style.RuleNumber>1</Style.RuleNumber>
        <Style.RuleContent>
          <strong>Separation.</strong> Steer to avoid crowding neighbors. Each bird calculates a
          repulsion vector from nearby birds within 3 body lengths.
        </Style.RuleContent>
      </Style.RuleCard>

      <Style.RuleCard>
        <Style.RuleNumber>2</Style.RuleNumber>
        <Style.RuleContent>
          <strong>Alignment.</strong> Steer toward the average heading of neighbors. Birds within
          the perception radius (10 units) contribute to a heading consensus.
        </Style.RuleContent>
      </Style.RuleCard>

      <Style.RuleCard>
        <Style.RuleNumber>3</Style.RuleNumber>
        <Style.RuleContent>
          <strong>Cohesion.</strong> Steer toward the average position of neighbors. This creates
          the flocking behavior: birds are pulled toward the group center.
        </Style.RuleContent>
      </Style.RuleCard>

      <Style.ContentCard>
        <p>
          In a traditional browser, you would run this in JavaScript. But JavaScript is
          single-threaded. At 1,000 birds, your frame rate drops. At 10,000, your browser freezes.
        </p>
        <p style={{ fontWeight: 600, color: '#1a1a1a' }}>
          INOS runs the same algorithm in compiled Rust, using spatial hashing to reduce complexity
          from O(n²) to O(n), and communicates with the renderer through shared memory. The result:
          10,000 birds at 60 FPS.
        </p>
      </Style.ContentCard>

      {/* The Full Pipeline */}
      <ScrollReveal>
        <Style.IllustrationContainer>
          <Style.IllustrationHeader>
            <Style.IllustrationTitle>
              Boids Pipeline: Compute → Learn → Render
            </Style.IllustrationTitle>
          </Style.IllustrationHeader>
          <BoidsDataFlowDiagram />
          <Style.IllustrationCaption>
            Each frame: Rust physics, Go evolution, JS rendering. Same memory.
          </Style.IllustrationCaption>
        </Style.IllustrationContainer>
      </ScrollReveal>

      {/* Detailed Layer Explanation */}
      <Style.ContentCard>
        <h3>The Three Layers in Detail</h3>
      </Style.ContentCard>

      <Style.StepGrid>
        <Style.StepCard $color="#dea584">
          <Style.StepLabel $color="#dea584">Layer 1: Compute</Style.StepLabel>
          <Style.StepTitle>Rust Boids Unit</Style.StepTitle>
          <Style.StepDesc>
            Every frame, reads bird positions from Buffer A, calculates flocking forces using SIMD,
            writes to Buffer B, then flips. Ping-pong buffering prevents read/write conflicts.
          </Style.StepDesc>
        </Style.StepCard>

        <Style.StepCard $color="#00add8">
          <Style.StepLabel $color="#00add8">Layer 2: Intelligence</Style.StepLabel>
          <Style.StepTitle>Go Supervisor</Style.StepTitle>
          <Style.StepDesc>
            Every 3 seconds, runs a genetic algorithm. Reads all bird genes, calculates fitness
            (survival + cohesion), selects parents via tournament, applies crossover and mutation.
          </Style.StepDesc>
        </Style.StepCard>

        <Style.StepCard $color="#f59e0b">
          <Style.StepLabel $color="#f59e0b">Layer 3: Perception</Style.StepLabel>
          <Style.StepTitle>JS Renderer</Style.StepTitle>
          <Style.StepDesc>
            Polls the epoch flag via Atomics. When signaled, reads transformation matrices directly
            from SAB into Three.js instanceMatrix. 1,000 birds in a single GPU draw call.
          </Style.StepDesc>
        </Style.StepCard>
      </Style.StepGrid>

      <Style.SpinningBirdNote>
        🐦 <strong>The Maverick Bird:</strong> You may notice an occasional bird doing barrel rolls.
        This is a quaternion interpolation quirk we have not yet fixed. We call it the "Maverick
        Bird." Watch for it. It is weirdly endearing.
      </Style.SpinningBirdNote>

      {/* Go Intelligence Deep Dive */}
      <Style.ContentCard>
        <h3>Go: The Intelligent Supervisor</h3>
        <p>
          The Go kernel does not just coordinate. It <strong>learns</strong>. Every 3 seconds, it
          reads the entire bird population from SAB, evaluates fitness, and evolves the next
          generation.
        </p>
        <p>The genetic algorithm uses these operations:</p>
        <ul>
          <li>
            <strong>Tournament Selection (k=5):</strong> Pick 5 random birds, choose the fittest as
            a parent. This balances exploration and exploitation.
          </li>
          <li>
            <strong>Uniform Crossover:</strong> For each gene, randomly pick from parent A or B.
            This recombines successful traits.
          </li>
          <li>
            <strong>Gaussian Mutation:</strong> Add noise to genes with probability 0.1. This
            introduces new variations.
          </li>
          <li>
            <strong>Elitism:</strong> The top 10% of birds are preserved. This prevents losing
            successful solutions.
          </li>
        </ul>
        <p>
          After evolution, Go calls <code>SignalEpoch()</code>. This increments an atomic counter in
          SAB. All listeners wake up. Zero latency. Zero polling.
        </p>
      </Style.ContentCard>

      <Style.CodeNote>
        <strong>Technical Note:</strong> Go WASM cannot directly share memory with SAB due to
        runtime constraints. INOS uses a <strong>Memory Twin</strong> architecture: Go maintains a
        synchronized snapshot via <code>js.CopyBytesToGo</code>. This provides snapshot isolation.
        The Supervisor operates on a stable view, immune to high-frequency Rust updates. See{' '}
        <code>misc/go_wasm_memory_integrity.md</code>.
      </Style.CodeNote>

      <ScrollReveal>
        <Style.BlueprintSection>
          <Style.JotterHeader>
            <Style.JotterNumber>Protocol 042</Style.JotterNumber>
            <Style.JotterHeading>Zero-Copy Substrate</Style.JotterHeading>
          </Style.JotterHeader>
          <p>
            By sharing the RAW pointer to the underlying buffer, we achieve O(1) data sharing. The
            size of the data—bytes or gigabytes—no longer affects the cost of moving it.
          </p>
        </Style.BlueprintSection>
      </ScrollReveal>

      <Style.ContentCard>
        <h3>The Implications</h3>
        <p style={{ fontWeight: 600, color: '#1a1a1a', marginBottom: '1rem' }}>
          This is not just optimization. This is a new programming model.
        </p>
        <p>
          <strong>Compute</strong> lives in Rust. Unsafe, SIMD-optimized, memory-safe at compile
          time. <strong>Intelligence</strong> lives in Go. Goroutines, genetic algorithms, strategic
          decisions. <strong>Perception</strong> lives in JavaScript. React, Three.js, the DOM.
        </p>
        <p>
          All three run concurrently, communicating through a shared circulatory system of bytes.
          The browser becomes a distributed operating system.
        </p>
      </Style.ContentCard>

      <Style.CodeNote>
        <strong>Further Reading:</strong> The thread architecture, supervisor hierarchy, and
        reactive mutation model are documented in <code>docs/threads.md</code>. It explains how
        units become supervisors, how patterns propagate, and how the P2P mesh will scale this to
        millions of nodes.
      </Style.CodeNote>

      <ChapterNav
        prev={{ to: '/problem', title: 'The Problem' }}
        next={{ to: '/architecture', title: 'Architecture' }}
      />
    </Style.BlogContainer>
  );
}

export default Insight;
