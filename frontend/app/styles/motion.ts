/**
 * INOS Technical Codex — Animation Timing System
 *
 * Based on Material Design 3 timing principles, optimized for INOS.
 * Supports accessibility via reduced motion.
 */

import { Transition, Variants } from 'framer-motion';

// ═══════════════════════════════════════════════════════════════════
// TIMING CONSTANTS (milliseconds)
// ═══════════════════════════════════════════════════════════════════

export const TIMING = {
  /** Micro: Button clicks, toggles, icon changes */
  MICRO: 180,
  /** Standard: Card reveals, dropdowns, modals */
  STANDARD: 250,
  /** Page: Full page transitions */
  PAGE: 350,
  /** Emphasis: Hero animations, attention-grabbing */
  EMPHASIS: 500,
  /** Stagger: Delay between staggered children */
  STAGGER: 50,
  /** Stagger fast: For lists with many items */
  STAGGER_FAST: 30,
} as const;

// ═══════════════════════════════════════════════════════════════════
// EASING CURVES (Material Design 3 inspired)
// ═══════════════════════════════════════════════════════════════════

export const EASING = {
  /** Standard: Most UI transitions */
  standard: [0.4, 0.0, 0.2, 1.0] as const,
  /** Emphasized: More dramatic, for hero elements */
  emphasized: [0.0, 0.0, 0.2, 1.0] as const,
  /** Decelerate: Elements entering (slowing down as they arrive) */
  decelerate: [0.0, 0.0, 0.0, 1.0] as const,
  /** Accelerate: Elements exiting (speeding up as they leave) */
  accelerate: [0.4, 0.0, 1.0, 1.0] as const,
} as const;

// ═══════════════════════════════════════════════════════════════════
// SPRING CONFIG (for interactive elements)
// ═══════════════════════════════════════════════════════════════════

export const SPRING = {
  /** Default spring for interactive elements */
  default: { type: 'spring', stiffness: 300, damping: 30 } as const,
  /** Gentle spring for subtle movements */
  gentle: { type: 'spring', stiffness: 200, damping: 25 } as const,
  /** Bouncy spring for playful interactions */
  bouncy: { type: 'spring', stiffness: 400, damping: 20 } as const,
} as const;

// ═══════════════════════════════════════════════════════════════════
// PRE-COMPOSED TRANSITIONS
// ═══════════════════════════════════════════════════════════════════

export const TRANSITIONS: Record<string, Transition> = {
  micro: { duration: TIMING.MICRO / 1000, ease: EASING.standard },
  standard: { duration: TIMING.STANDARD / 1000, ease: EASING.standard },
  page: { duration: TIMING.PAGE / 1000, ease: EASING.emphasized },
  emphasis: { duration: TIMING.EMPHASIS / 1000, ease: EASING.emphasized },
  enter: { duration: TIMING.STANDARD / 1000, ease: EASING.decelerate },
  exit: { duration: TIMING.MICRO / 1000, ease: EASING.accelerate },
  spring: SPRING.default,
};

// Stagger container transitions
export const STAGGER_TRANSITIONS = {
  container: {
    staggerChildren: TIMING.STAGGER / 1000,
    delayChildren: TIMING.MICRO / 1000,
  },
  fast: {
    staggerChildren: TIMING.STAGGER_FAST / 1000,
  },
} as const;

// ═══════════════════════════════════════════════════════════════════
// ANIMATION VARIANTS
// ═══════════════════════════════════════════════════════════════════

/** Simple fade in/out */
export const FADE_VARIANTS: Variants = {
  initial: { opacity: 0 },
  animate: { opacity: 1, transition: TRANSITIONS.standard },
  exit: { opacity: 0, transition: TRANSITIONS.micro },
};

/** Slide up with fade */
export const SLIDE_UP_VARIANTS: Variants = {
  initial: { y: 20, opacity: 0 },
  animate: { y: 0, opacity: 1, transition: TRANSITIONS.standard },
  exit: { y: -10, opacity: 0, transition: TRANSITIONS.exit },
};

/** Scale pop with fade */
export const POP_VARIANTS: Variants = {
  initial: { scale: 0.95, opacity: 0 },
  animate: { scale: 1, opacity: 1, transition: TRANSITIONS.standard },
  exit: { scale: 0.95, opacity: 0, transition: TRANSITIONS.micro },
};

/** Page transition - used by PageTransition component */
export const PAGE_VARIANTS: Variants = {
  initial: { opacity: 0 },
  animate: {
    opacity: 1,
    transition: TRANSITIONS.page,
  },
  exit: {
    opacity: 0,
    transition: { duration: 0.15, ease: EASING.accelerate },
  },
};

/** Manuscript ink reveal (Da Vinci style) */
export const MANUSCRIPT_VARIANTS: Variants = {
  initial: { opacity: 0, scale: 0.98, filter: 'blur(4px)' },
  animate: {
    opacity: 1,
    scale: 1,
    filter: 'blur(0px)',
    transition: { duration: 0.5, ease: EASING.decelerate },
  },
  exit: {
    opacity: 0,
    scale: 0.98,
    filter: 'blur(2px)',
    transition: { duration: 0.2 },
  },
};

/** Blueprint technical reveal */
export const BLUEPRINT_VARIANTS: Variants = {
  initial: { opacity: 0, clipPath: 'inset(0 100% 0 0)' },
  animate: {
    opacity: 1,
    clipPath: 'inset(0 0% 0 0)',
    transition: { duration: 0.6, ease: EASING.emphasized },
  },
  exit: { opacity: 0, transition: { duration: 0.15 } },
};

/** Stagger container - applies to parent */
export const STAGGER_CONTAINER_VARIANTS: Variants = {
  initial: {},
  animate: { transition: STAGGER_TRANSITIONS.container },
  exit: {},
};

/** Stagger child - used with stagger container */
export const STAGGER_CHILD_VARIANTS: Variants = {
  initial: { opacity: 0, y: 12 },
  animate: { opacity: 1, y: 0, transition: TRANSITIONS.standard },
  exit: { opacity: 0, y: -8, transition: TRANSITIONS.micro },
};

// ═══════════════════════════════════════════════════════════════════
// HOVER & TAP STATES
// ═══════════════════════════════════════════════════════════════════

export const INTERACTIVE_STATES = {
  hover: { scale: 1.02, transition: TRANSITIONS.micro },
  tap: { scale: 0.98, transition: { duration: 0.1 } },
  focus: { outline: '2px solid var(--sepia-accent)', outlineOffset: '2px' },
} as const;

// ═══════════════════════════════════════════════════════════════════
// REDUCED MOTION VARIANTS
// ═══════════════════════════════════════════════════════════════════

/** Minimal variants for users who prefer reduced motion */
export const REDUCED_MOTION_VARIANTS: Variants = {
  initial: { opacity: 0 },
  animate: { opacity: 1, transition: { duration: 0.01 } },
  exit: { opacity: 0, transition: { duration: 0.01 } },
};

/**
 * Get accessible variants based on reduced motion preference
 */
export function getAccessibleVariants(variants: Variants, prefersReducedMotion: boolean): Variants {
  return prefersReducedMotion ? REDUCED_MOTION_VARIANTS : variants;
}
