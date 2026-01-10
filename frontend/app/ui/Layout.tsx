/**
 * INOS Technical Codex — Layout Component
 *
 * Fixed header/footer, minimal design, mesh metrics in footer.
 * Refactored to Style object pattern.
 */

import styled, { ThemeProvider } from 'styled-components';
import { MotionConfig } from 'framer-motion';
import { Outlet } from 'react-router-dom';
import { theme } from '../styles/theme';
import { usePrefersReducedMotion } from '../hooks/useReducedMotion';
import Navigation from './Navigation';
import MeshMetricsBar from '../features/metrics/MeshMetrics';
import PageTransition from './PageTransition';
import ArchitecturalBoids from '../features/boids/ArchitecturalBoids';

const Style = {
  LayoutContainer: styled.div`
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    background: transparent;
  `,

  Header: styled.header`
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    z-index: ${p => p.theme.zIndex.fixed};
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: ${p => p.theme.spacing[3]} ${p => p.theme.spacing[6]};
    background: rgba(244, 241, 234, 0.25);
    backdrop-filter: blur(12px);
    border-bottom: 1px solid ${p => p.theme.colors.borderSubtle};
    height: 48px;
  `,

  Main: styled.main`
    flex: 1;
    position: relative;
    margin-top: 48px;
    margin-bottom: 40px;
    z-index: ${p => p.theme.zIndex.content};
  `,

  Footer: styled.footer`
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    z-index: ${p => p.theme.zIndex.fixed};
    background: rgba(244, 241, 234, 0.25);
    backdrop-filter: blur(12px);
    border-top: 1px solid ${p => p.theme.colors.borderSubtle};
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
  `,
};

export function Layout() {
  const prefersReducedMotion = usePrefersReducedMotion();

  return (
    <ThemeProvider theme={theme}>
      <MotionConfig reducedMotion={prefersReducedMotion ? 'always' : 'never'}>
        <Style.LayoutContainer>
          <ArchitecturalBoids />

          <Style.Header>
            <Navigation />
          </Style.Header>

          <Style.Main>
            <PageTransition>
              <Outlet />
            </PageTransition>
          </Style.Main>

          <Style.Footer>
            <MeshMetricsBar />
          </Style.Footer>
        </Style.LayoutContainer>
      </MotionConfig>
    </ThemeProvider>
  );
}

export default Layout;
