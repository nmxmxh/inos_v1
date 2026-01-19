import { useState } from 'react';
import styled from 'styled-components';
import { motion, AnimatePresence, Variants } from 'framer-motion';
import { NavLink, Link } from 'react-router-dom';
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
    width: '100%',
    height: 60,
    y: 0,
    borderRadius: 0,
    transition: { type: 'spring', stiffness: 300, damping: 30 },
  },
  open: {
    width: '100%',
    height: 'auto',
    y: 0,
    borderRadius: 0,
    transition: { type: 'spring', stiffness: 300, damping: 30 },
  },
};

const contentVariants: Variants = {
  closed: { opacity: 0, scaleY: 0.9, originY: 1, transition: { duration: 0.15 } },
  open: { opacity: 1, scaleY: 1, originY: 1, transition: { delay: 0.1, duration: 0.25 } },
};

// ========== STYLES ==========

const Wrapper = styled.div`
  position: fixed;
  bottom: 0;
  left: 0;
  width: 100%;
  pointer-events: none;
  display: flex;
  justify-content: center;
  z-index: 10000; /* Elevated above standard fixed headers */
`;

const Dock = styled(motion.div)`
  pointer-events: auto;
  width: 100%;
  background: #0a0a0a;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 -10px 30px rgba(0, 0, 0, 0.6);
  overflow: hidden;
  display: flex;
  flex-direction: column;
`;

const HeaderRow = styled.div`
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  flex-shrink: 0;
  width: 100%;
  box-sizing: border-box;
  background: rgba(0, 0, 0, 0.5);
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
`;

const HeaderLeft = styled.div`
  display: flex;
  align-items: center;
  gap: 14px;
  color: #fff;
  opacity: 0.8;
  cursor: pointer;
  height: 100%;

  &:hover {
    opacity: 1;
  }
`;

const MenuLabel = styled.span`
  font-family: ${p => p.theme.fonts.typewriter};
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  color: rgba(255, 255, 255, 0.6);
`;

const MeshButton = styled(Link)<{ $active: boolean }>`
  display: flex;
  align-items: center;
  gap: 12px;
  background: ${p => (p.$active ? 'rgba(0, 255, 127, 0.08)' : 'rgba(255, 255, 255, 0.03)')};
  padding: 8px 14px;
  border: 1px solid ${p => (p.$active ? 'rgba(0, 255, 127, 0.3)' : 'rgba(255, 255, 255, 0.1)')};
  box-shadow: ${p => (p.$active ? 'inset 0 0 10px rgba(0, 255, 127, 0.05)' : 'none')};
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  text-decoration: none;

  &:active {
    transform: scale(0.97);
    background: rgba(0, 255, 127, 0.15);
  }

  .dot {
    width: 4px;
    height: 4px;
    background: ${p => (p.$active ? '#00ff7f' : '#333')};
    box-shadow: ${p => (p.$active ? '0 0 8px #00ff7f' : 'none')};
  }

  .text {
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    color: ${p => (p.$active ? '#00ff7f' : 'rgba(255, 255, 255, 0.4)')};
    letter-spacing: 0.15em;
    font-weight: 700;
    text-transform: uppercase;
  }
`;

const ContentArea = styled(motion.div)`
  padding: 0;
  display: flex;
  flex-direction: column;
  width: 100%;
  box-sizing: border-box;
`;

const NavList = styled.div`
  display: flex;
  flex-direction: column;
  padding: 10px 0;
  background: #050505;
`;

const NavItem = styled(NavLink)`
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 24px;
  color: rgba(255, 255, 255, 0.4);
  text-decoration: none;
  transition: all 0.15s;
  border-left: 2px solid transparent;

  .item-left {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .icon-box {
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.5;
  }

  .label {
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.15em;
    font-weight: 600;
  }

  .ref {
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 9px;
    opacity: 0.2;
    letter-spacing: 0.05em;
  }

  &:hover {
    color: white;
    background: rgba(255, 255, 255, 0.02);
    .icon-box {
      opacity: 1;
    }
    .ref {
      opacity: 0.4;
    }
  }

  &.active {
    color: #fff;
    background: rgba(0, 255, 127, 0.03);
    border-left: 2px solid #00ff7f;
    .label {
      font-weight: 800;
      color: #fff;
    }
    .icon-box {
      opacity: 1;
      color: #00ff7f;
    }
  }
`;

const MetricsContainer = styled.div`
  border-top: 1px solid rgba(255, 255, 255, 0.05);
  background: #000;
  padding: 24px 20px;

  /* Metrics styling overrides */
  * {
    color: rgba(255, 255, 255, 0.6) !important;
  }
`;

// ========== COMPONENT ==========

export function MobileDock() {
  const [isOpen, setIsOpen] = useState(false);
  const status = useSystemStore(s => s.status);
  const isReady = status === 'ready';

  const toggle = () => {
    // If clicking the MeshButton specifically, we might want different behavior?
    // For now, toggle menu.
    setIsOpen(p => !p);
  };

  const close = () => setIsOpen(false);

  const ToggleIcon = isOpen ? Icons.Close : Icons.Menu;

  return (
    <Wrapper>
      <Dock initial="closed" animate={isOpen ? 'open' : 'closed'} variants={dockVariants}>
        <HeaderRow>
          <HeaderLeft onClick={toggle}>
            <ToggleIcon />
            <MenuLabel>SYSTEM</MenuLabel>
          </HeaderLeft>

          <MeshButton to="/diagnostics" onClick={close} $active={isReady}>
            <div className="dot" />
            <div className="text">{isReady ? 'ONLINE' : 'SYNC'}</div>
          </MeshButton>
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
                {NAV_ITEMS.map((item, idx) => {
                  const Icon = getIconForRoute(item.to);
                  const refId = `0X${(idx + 1).toString(16).toUpperCase().padStart(2, '0')}`;
                  return (
                    <NavItem key={item.to} to={item.to} onClick={close}>
                      <div className="item-left">
                        <span className="icon-box">
                          <Icon />
                        </span>
                        <span className="label">{item.label}</span>
                      </div>
                      <span className="ref">{refId}</span>
                    </NavItem>
                  );
                })}
              </NavList>

              <MetricsContainer>
                <MeshMetricsBar onClick={close} />
              </MetricsContainer>
            </ContentArea>
          )}
        </AnimatePresence>
      </Dock>
    </Wrapper>
  );
}

export default MobileDock;
