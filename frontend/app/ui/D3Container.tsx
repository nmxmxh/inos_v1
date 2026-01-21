import React, { useEffect, useRef, memo } from 'react';
import * as d3 from 'd3';
import styled from 'styled-components';

const Container = styled.div`
  width: 100%;
  position: relative;
  overflow: hidden;
`;

export type D3RenderFn = (
  svg: d3.Selection<SVGSVGElement, unknown, null, undefined>,
  width: number,
  height: number
) => void | (() => void);

interface D3ContainerProps {
  render: D3RenderFn;
  dependencies: React.DependencyList;
  height?: number | string;
  className?: string;
  viewBox?: string;
  preserveAspectRatio?: string;
}

/**
 * D3Container
 *
 * A high-performance wrapper for D3 visualizations that ensures:
 * 1. React.memo stability (prevents parent re-renders from trashing the DOM)
 * 2. Proper cleanup of D3 timers/transitions
 * 3. Responsive sizing
 *
 * @example
 * <D3Container
 *   height={400}
 *   dependencies={[theme.colors.accent, data]}
 *   render={(svg, width, height) => {
 *     svg.selectAll('*').remove();
 *     // ... d3 code
 *     return () => {
 *       // optional cleanup
 *     };
 *   }}
 * />
 */
const D3Container = memo(
  ({
    render,
    dependencies,
    height = 400,
    className,
    viewBox,
    preserveAspectRatio = 'xMidYMid meet',
  }: D3ContainerProps) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const svgRef = useRef<SVGSVGElement>(null);
    const cleanupRef = useRef<(() => void) | void>(undefined);

    // Track dimensions state to trigger re-renders on resize
    const [dimensions, setDimensions] = React.useState({ width: 0, height: 0 });

    useEffect(() => {
      // 1. Initial measurement
      if (containerRef.current) {
        const { width, height } = containerRef.current.getBoundingClientRect();
        setDimensions({ width, height });
      }

      // 2. Setup ResizeObserver
      const observer = new ResizeObserver(entries => {
        for (const entry of entries) {
          const { width, height } = entry.contentRect;
          if (width > 0 && height > 0) {
            setDimensions({ width, height });
          }
        }
      });

      if (containerRef.current) {
        observer.observe(containerRef.current);
      }

      return () => {
        observer.disconnect();
      };
    }, []);

    useEffect(() => {
      if (!containerRef.current || !svgRef.current || dimensions.width === 0) return;

      const svg = d3.select(svgRef.current);

      // Cleanup previous render if exists
      if (cleanupRef.current) {
        cleanupRef.current();
      }

      // Stop all active transitions
      svg.selectAll('*').interrupt();

      // Execute render with latest dimensions
      const cleanup = render(svg, dimensions.width, dimensions.height);
      cleanupRef.current = cleanup;

      return () => {
        if (cleanupRef.current) cleanupRef.current();
        svg.selectAll('*').interrupt();
      };
      // We explicitly merge external dependencies with our internal dimension state
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [...dependencies, dimensions.width, dimensions.height]);

    return (
      <Container ref={containerRef} className={className} style={{ height }}>
        <svg
          ref={svgRef}
          width="100%"
          height="100%"
          viewBox={viewBox}
          preserveAspectRatio={preserveAspectRatio}
          style={{ display: 'block', overflow: 'visible' }}
        />
      </Container>
    );
  }
);

D3Container.displayName = 'D3Container';

export default D3Container;
