/**
 * INOS Technical Codex — Manuscript Styled Components
 *
 * Da Vinci-inspired with bold text, uppercase smaller titles, reduced gaps.
 * Refactored to Style object pattern.
 */

import styled, { keyframes } from 'styled-components';

// ═══════════════════════════════════════════════════════════════════
// KEYFRAMES
// ═══════════════════════════════════════════════════════════════════

export const inkReveal = keyframes`
  from {
    opacity: 0;
    filter: blur(4px);
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    filter: blur(0);
    transform: translateY(0);
  }
`;

// ═══════════════════════════════════════════════════════════════════
// STYLE OBJECT
// ═══════════════════════════════════════════════════════════════════

export const Style = {
  BoldText: styled.p`
    font-weight: ${p => p.theme.fontWeights.semibold};
    color: ${p => p.theme.colors.inkDark};
    line-height: ${p => p.theme.lineHeights.relaxed};
    margin: 0 0 ${p => p.theme.spacing[4]};
  `,

  ManuscriptSection: styled.section`
    background: rgba(255, 255, 255, 0.82);
    backdrop-filter: blur(12px);
    padding: ${p => p.theme.spacing[6]} ${p => p.theme.spacing[8]};
    border-left: 2px solid ${p => p.theme.colors.accent};
    position: relative;
    margin: ${p => p.theme.spacing[6]} 0;

    @media (prefers-reduced-motion: no-preference) {
      animation: ${inkReveal} 0.5s ease-out;
    }

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      padding: ${p => p.theme.spacing[4]};
      margin: ${p => p.theme.spacing[4]} 0;
    }

    p {
      font-weight: ${p => p.theme.fontWeights.medium};
      color: ${p => p.theme.colors.inkDark};
    }
  `,

  BlueprintSection: styled.section`
    background: ${p => p.theme.colors.blueprintLight};
    padding: ${p => p.theme.spacing[6]} ${p => p.theme.spacing[8]};
    position: relative;
    margin: ${p => p.theme.spacing[6]} 0;

    background-image:
      linear-gradient(${p => p.theme.colors.blueprintGrid} 1px, transparent 1px),
      linear-gradient(90deg, ${p => p.theme.colors.blueprintGrid} 1px, transparent 1px);
    background-size: 20px 20px;
    border: 1px solid ${p => p.theme.colors.blueprint};

    p {
      font-weight: ${p => p.theme.fontWeights.medium};
    }
  `,

  JotterSection: styled.section`
    background: rgba(255, 255, 255, 0.82);
    backdrop-filter: blur(12px);
    padding: ${p => p.theme.spacing[6]} ${p => p.theme.spacing[8]};
    border-left: 2px solid ${p => p.theme.colors.accent};
    position: relative;
    margin: ${p => p.theme.spacing[4]} 0;

    p {
      font-weight: ${p => p.theme.fontWeights.medium};
      color: ${p => p.theme.colors.inkDark};
      line-height: ${p => p.theme.lineHeights.relaxed};
      margin-bottom: ${p => p.theme.spacing[3]};
    }
  `,

  JotterHeader: styled.div`
    margin-bottom: ${p => p.theme.spacing[4]};
    display: flex;
    flex-direction: column;
    gap: ${p => p.theme.spacing[1]};
  `,

  JotterNumber: styled.span`
    font-family: ${p => p.theme.fonts.typewriter};
    color: ${p => p.theme.colors.accent};
    font-weight: ${p => p.theme.fontWeights.bold};
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.15em;
  `,

  JotterHeading: styled.h2`
    margin: 0;
    font-family: ${p => p.theme.fonts.main};
    font-weight: ${p => p.theme.fontWeights.bold};
    font-size: ${p => p.theme.fontSizes.lg};
    color: ${p => p.theme.colors.inkDark};
    text-transform: uppercase;
    letter-spacing: 0.05em;
  `,

  BlogContainer: styled.main`
    /* Glassmorphism page container */
    background: rgba(244, 241, 234, 0.5); /* Semi-transparent paper cream */
    backdrop-filter: blur(8px);
    border-radius: 8px;
    max-width: ${p => p.theme.layout.maxWidth};
    margin: 0 auto;
    padding: ${p => p.theme.spacing[16]} ${p => p.theme.spacing[6]};
    position: relative;
    z-index: ${p => p.theme.zIndex.content};

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      padding: ${p => p.theme.spacing[8]} ${p => p.theme.spacing[4]};
    }
  `,

  BlogSection: styled.section`
    margin-bottom: ${p => p.theme.spacing[8]};
    text-align: left;

    p {
      font-size: ${p => p.theme.fontSizes.base};
      font-weight: ${p => p.theme.fontWeights.medium};
      color: ${p => p.theme.colors.inkDark};
      margin-bottom: ${p => p.theme.spacing[3]};
      line-height: ${p => p.theme.lineHeights.relaxed};
    }
  `,

  LeadParagraph: styled.p`
    font-size: ${p => p.theme.fontSizes.lg};
    font-weight: ${p => p.theme.fontWeights.semibold};
    color: ${p => p.theme.colors.inkDark};
    line-height: ${p => p.theme.lineHeights.relaxed};
    margin-bottom: ${p => p.theme.spacing[6]};
    text-align: left;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      font-size: ${p => p.theme.fontSizes.base};
    }
  `,

  IllustrationContainer: styled.div`
    width: 100%;
    max-width: 100%;
    margin: ${p => p.theme.spacing[4]} 0;

    svg {
      width: 100%;
      height: auto;
    }
  `,

  PageTitle: styled.h1`
    font-family: ${p => p.theme.fonts.main};
    font-weight: ${p => p.theme.fontWeights.extrabold};
    font-size: ${p => p.theme.fontSizes['2xl']};
    color: ${p => p.theme.colors.inkDark};
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin: 0 0 ${p => p.theme.spacing[4]};
    text-align: left;

    @media (max-width: ${p => p.theme.breakpoints.md}) {
      font-size: ${p => p.theme.fontSizes.xl};
    }
  `,

  SectionTitle: styled.h2`
    font-family: ${p => p.theme.fonts.main};
    font-weight: ${p => p.theme.fontWeights.bold};
    font-size: ${p => p.theme.fontSizes.sm};
    color: ${p => p.theme.colors.inkLight};
    text-transform: uppercase;
    letter-spacing: 0.1em;
    margin: 0 0 ${p => p.theme.spacing[3]};
    text-align: left;
  `,

  MetricValue: styled.span`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: ${p => p.theme.fontSizes['3xl']};
    font-weight: ${p => p.theme.fontWeights.bold};
    color: ${p => p.theme.colors.inkDark};
    line-height: 1;
  `,

  MetricLabel: styled.span`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 9px;
    font-weight: ${p => p.theme.fontWeights.bold};
    color: ${p => p.theme.colors.accent};
    text-transform: uppercase;
    letter-spacing: 0.15em;
  `,

  MetricUnit: styled.span`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: ${p => p.theme.fontSizes.xs};
    color: ${p => p.theme.colors.inkLight};
    margin-left: ${p => p.theme.spacing[1]};
  `,
};
