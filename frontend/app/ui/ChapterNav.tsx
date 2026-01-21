/**
 * INOS Technical Codex — Chapter Navigation
 *
 * Prev/Next navigation with keyboard support.
 * Refactored to Style object pattern.
 */

import styled from 'styled-components';
import { motion } from 'framer-motion';
import { Link, useNavigate } from 'react-router-dom';
import { useEffect, useCallback } from 'react';
import { TRANSITIONS } from '../styles/motion';

const Style = {
  NavContainer: styled.nav`
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: ${p => p.theme.spacing[8]} 0;
    margin-top: ${p => p.theme.spacing[16]};
    border-top: 1px solid ${p => p.theme.colors.borderSubtle};
  `,

  NavButton: styled(motion.create(Link))`
    display: flex;
    align-items: center;
    gap: ${p => p.theme.spacing[3]};
    padding: ${p => p.theme.spacing[4]} ${p => p.theme.spacing[6]};
    background: transparent;
    border: 1px solid ${p => p.theme.colors.borderMedium};
    color: ${p => p.theme.colors.inkMedium};
    text-decoration: none;
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: ${p => p.theme.fontSizes.sm};
    transition: all 0.2s ease;

    &:hover {
      background: ${p => p.theme.colors.inkDark};
      color: white;
      border-color: ${p => p.theme.colors.inkDark};
      transform: translateY(-1px);
    }

    &:focus-visible {
      outline: 2px solid ${p => p.theme.colors.accent};
      outline-offset: 2px;
    }
  `,

  DisabledButton: styled.span`
    display: flex;
    align-items: center;
    gap: ${p => p.theme.spacing[3]};
    padding: ${p => p.theme.spacing[4]} ${p => p.theme.spacing[6]};
    color: ${p => p.theme.colors.inkFaded};
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: ${p => p.theme.fontSizes.sm};
    cursor: not-allowed;
  `,

  Arrow: styled.span`
    font-size: ${p => p.theme.fontSizes.lg};
  `,

  Label: styled.span`
    display: flex;
    flex-direction: column;
    text-align: inherit;
  `,

  Direction: styled.span`
    font-size: ${p => p.theme.fontSizes.xs};
    color: ${p => p.theme.colors.inkLight};
    text-transform: uppercase;
    letter-spacing: 0.1em;
  `,

  Title: styled.span`
    font-weight: ${p => p.theme.fontWeights.medium};
  `,
};

interface ChapterNavProps {
  prev?: { to: string; title: string } | null;
  next?: { to: string; title: string } | null;
}

export function ChapterNav({ prev, next }: ChapterNavProps) {
  const navigate = useNavigate();

  // Keyboard navigation
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      // Only handle if not in an input/textarea
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return;
      }

      if (e.key === 'ArrowLeft' && prev) {
        e.preventDefault();
        navigate(prev.to);
      } else if (e.key === 'ArrowRight' && next) {
        e.preventDefault();
        navigate(next.to);
      }
    },
    [prev, next, navigate]
  );

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);

  return (
    <Style.NavContainer aria-label="Chapter navigation">
      {prev ? (
        <Style.NavButton
          to={prev.to}
          whileHover={{ x: -2 }}
          whileTap={{ scale: 0.98 }}
          transition={TRANSITIONS.micro}
          aria-label={`Previous: ${prev.title}`}
        >
          <Style.Arrow>←</Style.Arrow>
          <Style.Label style={{ textAlign: 'left' }}>
            <Style.Direction>Previous</Style.Direction>
            <Style.Title>{prev.title}</Style.Title>
          </Style.Label>
        </Style.NavButton>
      ) : (
        <Style.DisabledButton aria-hidden="true">
          <Style.Arrow>←</Style.Arrow>
          <span>Previous</span>
        </Style.DisabledButton>
      )}

      {next ? (
        <Style.NavButton
          to={next.to}
          whileHover={{ x: 2 }}
          whileTap={{ scale: 0.98 }}
          transition={TRANSITIONS.micro}
          aria-label={`Next: ${next.title}`}
        >
          <Style.Label style={{ textAlign: 'right' }}>
            <Style.Direction>Next</Style.Direction>
            <Style.Title>{next.title}</Style.Title>
          </Style.Label>
          <Style.Arrow>→</Style.Arrow>
        </Style.NavButton>
      ) : (
        <Style.DisabledButton aria-hidden="true">
          <span>Next</span>
          <Style.Arrow>→</Style.Arrow>
        </Style.DisabledButton>
      )}
    </Style.NavContainer>
  );
}

export default ChapterNav;
