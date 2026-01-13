/**
 * INOS Technical Codex — Layout Component
 *
 * Fixed header/footer, minimal design, mesh metrics in footer.
 * Refactored to Style object pattern.
 */

import styled, { ThemeProvider } from 'styled-components';
import { MotionConfig } from 'framer-motion';
import { useOutlet } from 'react-router-dom';
import { theme } from '../styles/theme';
import { usePrefersReducedMotion } from '../hooks/useReducedMotion';
import Navigation from './Navigation';
import MeshMetricsBar from '../features/metrics/MeshMetrics';
import PageTransition from './PageTransition';
import MobileDock from './MobileDock';

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
    background: rgba(244, 241, 234, 0.2);
    backdrop-filter: blur(5px) saturate(20%);
    border-bottom: 1px solid ${p => p.theme.colors.borderSubtle};
    height: 64px;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      height: 56px;
      padding: ${p => p.theme.spacing[2]} ${p => p.theme.spacing[4]};
      /* Center logo on mobile if desired, or keep start */
      justify-content: center;
    }
  `,

  // Wrapper to hide Navigation on mobile
  DesktopNav: styled.div`
    display: flex;
    width: 100%;
    @media (max-width: ${p => p.theme.breakpoints.md}) {
      display: none;
    }
  `,

  // Mobile Title Only
  MobileTitle: styled.div`
    display: none;
    font-family: ${p => p.theme.fonts.typewriter};
    font-weight: 700;
    font-size: 14px;
    letter-spacing: 0.15em;
    color: ${p => p.theme.colors.inkDark};
    text-transform: uppercase;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      display: block;
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
      margin-bottom: 80px; /* More space for dock */
    }
  `,

  Footer: styled.footer`
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    z-index: ${p => p.theme.zIndex.fixed};
    background: rgba(244, 241, 234, 0.2);
    backdrop-filter: blur(5px) saturate(20%);
    border-top: 1px solid ${p => p.theme.colors.borderSubtle};
    height: 48px;
    display: flex;
    align-items: center;
    justify-content: center;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      display: none; /* Hidden on mobile, moved to dock */
    }
  `,

  MobileDockWrapper: styled.div`
    display: none;
    @media (max-width: ${p => p.theme.breakpoints.md}) {
      display: block;
    }
  `,
};

export function Layout() {
  const prefersReducedMotion = usePrefersReducedMotion();
  const outlet = useOutlet();

  return (
    <ThemeProvider theme={theme}>
      <MotionConfig reducedMotion={prefersReducedMotion ? 'always' : 'never'}>
        <Style.LayoutContainer>
          <Style.Header>
            <Style.DesktopNav>
              <Navigation />
            </Style.DesktopNav>
            <Style.MobileTitle>INOS CODEX</Style.MobileTitle>
          </Style.Header>

          <Style.Main>
            <PageTransition>{outlet}</PageTransition>
          </Style.Main>

          <Style.Footer>
            <MeshMetricsBar />
          </Style.Footer>

          <Style.MobileDockWrapper>
            <MobileDock />
          </Style.MobileDockWrapper>
        </Style.LayoutContainer>
      </MotionConfig>
    </ThemeProvider>
  );
}

export default Layout;
