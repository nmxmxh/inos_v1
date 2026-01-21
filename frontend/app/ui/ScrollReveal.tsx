/**
 * INOS Technical Codex — Scroll Reveal Component
 *
 * Reveals content when scrolled into view using Framer Motion's useInView.
 */

import { motion, useInView, Variants } from 'framer-motion';
import { useRef } from 'react';
import {
  SLIDE_UP_VARIANTS,
  FADE_VARIANTS,
  MANUSCRIPT_VARIANTS,
  REDUCED_MOTION_VARIANTS,
} from '../styles/motion';
import { usePrefersReducedMotion } from '../hooks/useReducedMotion';

type VariantType = 'slideUp' | 'fade' | 'manuscript';

const VARIANT_MAP: Record<VariantType, Variants> = {
  slideUp: SLIDE_UP_VARIANTS,
  fade: FADE_VARIANTS,
  manuscript: MANUSCRIPT_VARIANTS,
};

interface ScrollRevealProps {
  children: React.ReactNode;
  variant?: VariantType;
  /** Trigger once or every time element enters viewport */
  once?: boolean;
  /** Margin around the element for triggering (negative = trigger before visible) */
  /** Margin around the element for triggering (negative = trigger before visible). Use `0px 0px -100px 0px` format. */
  margin?: `${number}px ${number}px ${number}px ${number}px`;
  /** Delay before animation starts (in seconds) */
  delay?: number;
  /** Custom className */
  className?: string;
}

export function ScrollReveal({
  children,
  variant = 'slideUp',
  once = true,
  margin = '0px 0px -100px 0px',
  delay = 0,
  className,
}: ScrollRevealProps) {
  const ref = useRef<HTMLDivElement>(null);
  const isInView = useInView(ref, { once, margin });
  const prefersReducedMotion = usePrefersReducedMotion();

  const variants = prefersReducedMotion ? REDUCED_MOTION_VARIANTS : VARIANT_MAP[variant];

  return (
    <motion.div
      ref={ref}
      className={className}
      variants={variants}
      initial="initial"
      animate={isInView ? 'animate' : 'initial'}
      transition={{ delay }}
    >
      {children}
    </motion.div>
  );
}

export default ScrollReveal;
