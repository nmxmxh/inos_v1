/**
 * INOS Technical Codex — Page Transition Component
 *
 * AnimatePresence wrapper for smooth page transitions.
 * Respects reduced motion preferences.
 */

import { motion, AnimatePresence } from 'framer-motion';
import { useLocation } from 'react-router-dom';
import { MYSTIC_VARIANTS, REDUCED_MOTION_VARIANTS } from '../styles/motion';
import { usePrefersReducedMotion } from '../hooks/useReducedMotion';

interface PageTransitionProps {
  children: React.ReactNode;
}

export function PageTransition({ children }: PageTransitionProps) {
  const location = useLocation();
  const prefersReducedMotion = usePrefersReducedMotion();

  const variants = prefersReducedMotion ? REDUCED_MOTION_VARIANTS : MYSTIC_VARIANTS;

  return (
    <AnimatePresence mode="wait" initial={false}>
      <motion.div
        key={location.pathname}
        variants={variants}
        initial="initial"
        animate="animate"
        exit="exit"
        transition={{ duration: 0.8, ease: [0.2, 0, 0.2, 1] }} // Slower duration (was 0.5-0.6)
        onAnimationStart={definition => {
          // Only scroll to top when the NEW page starts entering
          if (definition === 'animate') {
            window.scrollTo(0, 0);
          }
        }}
        style={{ width: '100%' }}
      >
        {children}
      </motion.div>
    </AnimatePresence>
  );
}

export default PageTransition;
