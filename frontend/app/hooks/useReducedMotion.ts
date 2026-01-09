/**
 * INOS Technical Codex — Reduced Motion Hook
 *
 * Accessibility hook for respecting user motion preferences.
 * Supports both system preference and user override via localStorage.
 */

import { useReducedMotion as useFramerReducedMotion } from 'framer-motion';
import { useEffect, useState, useCallback } from 'react';

const STORAGE_KEY = 'inos-reduce-motion';

/**
 * Hook to detect and manage reduced motion preference.
 *
 * Priority:
 * 1. User override (localStorage)
 * 2. System preference (prefers-reduced-motion)
 *
 * @returns Object with current preference and setter
 */
export function useReducedMotion() {
  const systemPreference = useFramerReducedMotion();
  const [userOverride, setUserOverride] = useState<boolean | null>(null);

  // Load user preference from localStorage
  useEffect(() => {
    if (typeof window === 'undefined') return;

    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored !== null) {
      setUserOverride(stored === 'true');
    }
  }, []);

  // Set user preference
  const setPreference = useCallback((value: boolean | null) => {
    if (typeof window === 'undefined') return;

    if (value === null) {
      localStorage.removeItem(STORAGE_KEY);
      setUserOverride(null);
    } else {
      localStorage.setItem(STORAGE_KEY, String(value));
      setUserOverride(value);
    }
  }, []);

  // User override takes precedence over system preference
  const prefersReducedMotion = userOverride ?? systemPreference ?? false;

  return {
    prefersReducedMotion,
    setPreference,
    isSystemPreference: userOverride === null,
    systemValue: systemPreference,
  };
}

/**
 * Simple hook that just returns the boolean value
 */
export function usePrefersReducedMotion(): boolean {
  const { prefersReducedMotion } = useReducedMotion();
  return prefersReducedMotion;
}
