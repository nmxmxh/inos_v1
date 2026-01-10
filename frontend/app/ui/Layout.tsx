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
    background: rgba(244, 241, 234, 0.4);
    backdrop-filter: blur(20px) saturate(180%);
    border-bottom: 1px solid ${p => p.theme.colors.borderSubtle};
    height: 64px;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      height: 56px;
      padding: ${p => p.theme.spacing[2]} ${p => p.theme.spacing[4]};
    }
  `,

  Main: styled.main`
    flex: 1;
    position: relative;
    margin-top: 64px;
    margin-bottom: 48px;
    z-index: ${p => p.theme.zIndex.content};

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      margin-top: 56px;
      margin-bottom: 40px;
    }
  `,

  Footer: styled.footer`
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    z-index: ${p => p.theme.zIndex.fixed};
    background: rgba(244, 241, 234, 0.4);
    backdrop-filter: blur(20px) saturate(180%);
    border-top: 1px solid ${p => p.theme.colors.borderSubtle};
    height: 48px;
    display: flex;
    align-items: center;
    justify-content: center;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      height: 40px;
    }
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
