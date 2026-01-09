/**
 * INOS Technical Codex — Cosmos Page (Chapter 4)
 *
 * The aspirational "One More Thing".
 * Refactored to Style object pattern.
 */

import styled from 'styled-components';
import { Style as ManuscriptStyle } from '../styles/manuscript';
import ChapterNav from '../ui/ChapterNav';
import ScrollReveal from '../ui/ScrollReveal';

const Style = {
  ...ManuscriptStyle,

  AweSection: styled.section`
    margin: ${p => p.theme.spacing[12]} 0;
    text-align: center;
  `,

  FinalQuote: styled.p`
    font-family: ${p => p.theme.fonts.main};
    font-size: ${p => p.theme.fontSizes.xl};
    font-weight: ${p => p.theme.fontWeights.normal};
    color: ${p => p.theme.colors.inkDark};
    letter-spacing: 0.05em;
    line-height: 1.6;
    max-width: 600px;
    margin: 0 auto;
  `,
};

export function Cosmos() {
  return (
    <Style.BlogContainer>
      <Style.SectionTitle>Chapter 04</Style.SectionTitle>
      <Style.PageTitle>The Cosmos</Style.PageTitle>

      <Style.LeadParagraph>
        <strong>One more thing.</strong> We didn't build INOS just to fix memory copies. We built it
        to simulate the universe.
      </Style.LeadParagraph>

      <Style.BlogSection>
        <p>
          By removing the "Copy Tax", we unlock a level of performance that allows for massive,
          real-time N-body simulations running directly in a decentralized mesh.
        </p>
      </Style.BlogSection>

      <ScrollReveal>
        <Style.AweSection>
          <Style.FinalQuote>
            "If you wish to make an apple pie from scratch, you must first invent the universe."
          </Style.FinalQuote>
          <p
            style={{
              marginTop: '2rem',
              opacity: 0.6,
              fontSize: '0.8rem',
              textTransform: 'uppercase',
              letterSpacing: '0.1em',
            }}
          >
            — Carl Sagan
          </p>
        </Style.AweSection>
      </ScrollReveal>

      <Style.BlogSection>
        <p>
          The boids you saw at the beginning are the first step. The next is a billions-of-stars
          cosmological simulation, where every node on the internet contributes a few cycles of
          gravity.
        </p>
        <p>
          <strong>Welcome to INOS. Let's build the future, one byte at a time.</strong>
        </p>
      </Style.BlogSection>

      <ChapterNav prev={{ to: '/architecture', title: 'Architecture' }} />
    </Style.BlogContainer>
  );
}

export default Cosmos;
