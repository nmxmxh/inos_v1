import { useState } from 'react';
import styled from 'styled-components';
import { motion, AnimatePresence, Variants } from 'framer-motion';
import { NavLink } from 'react-router-dom';
import { NAV_ITEMS } from './Navigation';
import MeshMetricsBar from '../features/metrics/MeshMetrics';
import { useSystemStore } from '../../src/store/system';

// ========== ICONS ==========

const Icons = {
  Menu: (props: any) => (
    <svg
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      {...props}
    >
      <path d="M3 12h18M3 6h18M3 18h18" />
    </svg>
  ),
  Close: (props: any) => (
    <svg
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      {...props}
    >
      <path d="M18 6L6 18M6 6l12 12" />
    </svg>
  ),
  Problem: (props: any) => (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      {...props}
    >
      <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" />
      <path d="M3.3 7l8.7 5 8.7-5" />
      <path d="M12 22v-9" />
    </svg>
  ),
  Insight: (props: any) => (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      {...props}
    >
      <path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  ),
  Architecture: (props: any) => (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      {...props}
    >
      <path d="M12 2L2 7l10 5 10-5-10-5Z" />
      <path d="M2 17l10 5 10-5" />
      <path d="M2 12l10 5 10-5" />
    </svg>
  ),
  Cosmos: (props: any) => (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      {...props}
    >
      <circle cx="12" cy="12" r="10" />
      <path d="M2 12h20" />
      <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
    </svg>
  ),
};

const getIconForRoute = (route: string) => {
  if (route.includes('problem')) return Icons.Problem;
  if (route.includes('insight')) return Icons.Insight;
  if (route.includes('architecture')) return Icons.Architecture;
  if (route.includes('cosmos')) return Icons.Cosmos;
  return Icons.Cosmos; // Fallback
};

// ========== ANIMATION CONFIG ==========

const dockVariants: Variants = {
  closed: {
    width: 280, // Wider closed state
    height: 64, // Taller header
    borderRadius: 32,
    transition: { type: 'spring', stiffness: 250, damping: 25 },
  },
  open: {
    width: 'min(92vw, 380px)',
    height: 'auto',
    borderRadius: 24,
    transition: { type: 'spring', stiffness: 250, damping: 25 },
  },
};

const contentVariants: Variants = {
  closed: { opacity: 0, height: 0, transition: { duration: 0.1 } },
  open: { opacity: 1, height: 'auto', transition: { delay: 0.15, duration: 0.3 } },
};

// ========== STYLES ==========

const Wrapper = styled.div`
  position: fixed;
  bottom: 32px; /* Slightly higher up */
  left: 0;
  width: 100%;
  pointer-events: none;
  display: flex;
  justify-content: center;
  z-index: 9999;
`;

const Dock = styled(motion.div)`
  pointer-events: auto;
  background: rgba(15, 15, 15, 0.92);
  backdrop-filter: blur(24px) saturate(180%);
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.5);
  overflow: hidden;
  display: flex;
  flex-direction: column;
`;

const HeaderRow = styled.div`
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  cursor: pointer;
  flex-shrink: 0;
  width: 100%;
  box-sizing: border-box;
  min-width: 280px;
`;

const HeaderLeft = styled.div`
  display: flex;
  align-items: center;
  gap: 12px;
  color: #fff;
`;

const MenuLabel = styled.span`
  font-family: ${p => p.theme.fonts.typewriter};
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.15em;
  text-transform: uppercase;
`;

const StatusBadge = styled.div<{ $active: boolean }>`
  display: flex;
  align-items: center;
  gap: 10px;
  background: rgba(255, 255, 255, 0.05);
  padding: 6px 10px;
  border-radius: 99px;
  border: 1px solid rgba(255, 255, 255, 0.05);

  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: ${p => (p.$active ? '#10b981' : '#666')};
    box-shadow: ${p => (p.$active ? '0 0 8px #10b981' : 'none')};
  }

  .text {
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    color: rgba(255, 255, 255, 0.7);
    letter-spacing: 0.05em;
    font-weight: 600;
  }
`;

const ContentArea = styled(motion.div)`
  padding: 8px 16px 24px 16px;
  display: flex;
  flex-direction: column;
  gap: 24px;
  width: 100%;
  box-sizing: border-box;
`;

const NavList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-top: 8px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
`;

const NavItem = styled(NavLink)`
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 20px;
  color: rgba(255, 255, 255, 0.5);
  text-decoration: none;
  transition: all 0.2s;
  border-radius: 12px;
  background: transparent;

  .icon-box {
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.7;
    transition: opacity 0.2s;
  }

  .label {
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 14px;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    font-weight: 500;
  }

  &:hover {
    background: rgba(255, 255, 255, 0.05);
    color: white;
    .icon-box {
      opacity: 1;
    }
  }

  &.active {
    color: white;
    background: rgba(255, 255, 255, 0.08);
    .label {
      font-weight: 700;
    }
    .icon-box {
      opacity: 1;
      color: ${p => p.theme.colors.accent || '#fff'};
    }
  }
`;

const MetricsContainer = styled.div`
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  padding-top: 20px;
  padding-left: 8px;
  padding-right: 8px;

  /* Metrics styling overrides for dark menu */
  * {
    color: rgba(255, 255, 255, 0.8) !important;
  }
`;

// ========== COMPONENT ==========

export function MobileDock() {
  const [isOpen, setIsOpen] = useState(false);
  const status = useSystemStore(s => s.status);
  const isReady = status === 'ready';

  const toggle = () => setIsOpen(p => !p);
  const close = () => setIsOpen(false);

  const ToggleIcon = isOpen ? Icons.Close : Icons.Menu;

  return (
    <Wrapper>
      <Dock initial="closed" animate={isOpen ? 'open' : 'closed'} variants={dockVariants}>
        <HeaderRow onClick={toggle}>
          <HeaderLeft>
            <ToggleIcon />
            <MenuLabel>MENU</MenuLabel>
          </HeaderLeft>

          <StatusBadge $active={isReady}>
            <div className="dot" />
            <div className="text">{isReady ? 'ONLINE' : 'SYNCING'}</div>
          </StatusBadge>
        </HeaderRow>

        <AnimatePresence>
          {isOpen && (
            <ContentArea
              key="content"
              initial="closed"
              animate="open"
              exit="closed"
              variants={contentVariants}
            >
              <NavList>
                {NAV_ITEMS.map(item => {
                  const Icon = getIconForRoute(item.to);
                  return (
                    <NavItem key={item.to} to={item.to} onClick={close}>
                      <span className="icon-box">
                        <Icon />
                      </span>
                      <span className="label">{item.label}</span>
                    </NavItem>
                  );
                })}
              </NavList>

              <MetricsContainer>
                <MeshMetricsBar />
              </MetricsContainer>
            </ContentArea>
          )}
        </AnimatePresence>
      </Dock>
    </Wrapper>
  );
}

export default MobileDock;
