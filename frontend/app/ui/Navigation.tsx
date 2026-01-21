/**
 * INOS Technical Codex — Navigation Component
 *
 * Minimal navigation with subtle indicator.
 * Refactored to Style object pattern.
 */

import styled from 'styled-components';
import { motion } from 'framer-motion';
import { NavLink, useLocation } from 'react-router-dom';
import { SPRING } from '../styles/motion';

const Style = {
  Nav: styled.nav`
    display: flex;
    align-items: center;
    gap: ${p => p.theme.spacing[6]};
    width: 100%;
  `,

  Logo: styled(NavLink)`
    font-family: ${p => p.theme.fonts.typewriter};
    font-weight: ${p => p.theme.fontWeights.bold};
    font-size: 11px;
    color: ${p => p.theme.colors.inkDark};
    text-decoration: none;
    text-transform: uppercase;
    letter-spacing: 0.15em;

    &:hover {
      color: ${p => p.theme.colors.accent};
    }

    &:focus-visible {
      outline: 2px solid ${p => p.theme.colors.accent};
      outline-offset: 4px;
    }
  `,

  NavList: styled.ul`
    display: flex;
    align-items: center;
    gap: ${p => p.theme.spacing[4]};
    list-style: none;
    margin: 0;
    padding: 0;
    margin-left: auto;
  `,

  NavItem: styled.li`
    position: relative;
  `,

  StyledLink: styled(NavLink)`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    font-weight: ${p => p.theme.fontWeights.semibold};
    color: ${p => p.theme.colors.inkMedium};
    text-decoration: none;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    padding: ${p => p.theme.spacing[1]} 0;
    display: block;

    &:hover {
      color: ${p => p.theme.colors.inkDark};
    }

    &.active {
      font-weight: ${p => p.theme.fontWeights.bold};
      color: ${p => p.theme.colors.inkDark};
    }

    &:focus-visible {
      outline: 2px solid ${p => p.theme.colors.accent};
      outline-offset: 4px;
    }
  `,

  Indicator: styled(motion.div)`
    position: absolute;
    bottom: -4px;
    left: 0;
    right: 0;
    height: 1px;
    background: ${p => p.theme.colors.inkDark};
  `,
};

interface NavItemType {
  to: string;
  label: string;
}

export const NAV_ITEMS: NavItemType[] = [
  { to: '/problem', label: 'Problem' },
  { to: '/insight', label: 'Insight' },
  { to: '/architecture', label: 'Architecture' },
  { to: '/cosmos', label: 'Cosmos' },
];

export function Navigation() {
  const location = useLocation();

  return (
    <Style.Nav role="navigation" aria-label="Main navigation">
      <Style.Logo to="/" aria-label="INOS Home">
        INOS Codex
      </Style.Logo>

      <Style.NavList>
        {NAV_ITEMS.map(item => (
          <Style.NavItem key={item.to}>
            <Style.StyledLink to={item.to}>
              {item.label}
              {location.pathname === item.to && (
                <Style.Indicator layoutId="nav-indicator" transition={SPRING.default} />
              )}
            </Style.StyledLink>
          </Style.NavItem>
        ))}
      </Style.NavList>
    </Style.Nav>
  );
}

export default Navigation;
