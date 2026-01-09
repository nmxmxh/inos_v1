/**
 * INOS Technical Codex — Design System Theme
 *
 * Da Vinci manuscript aesthetic with blueprint variant.
 * Used with styled-components ThemeProvider.
 */

export const theme = {
  // ═══════════════════════════════════════════════════════════════════
  // COLORS
  // ═══════════════════════════════════════════════════════════════════
  colors: {
    // Paper tones
    paperCream: '#f4f1ea',
    paperWhite: '#ffffff',
    paperAged: '#e8e2d6',
    paperOffWhite: '#fafaf9',

    // Ink tones
    inkDark: '#1a1a1a',
    inkMedium: '#404040',
    inkLight: '#737373',
    inkFaded: '#a3a3a3',

    // Accent (purple - the royal color of manuscripts)
    accent: '#6d28d9',
    accentHover: '#5b21b6',
    accentDim: '#ede9fe',
    accentLight: '#f5f3ff',

    // Blueprint variant
    blueprint: '#1e40af',
    blueprintLight: '#dbeafe',
    blueprintGrid: 'rgba(30, 64, 175, 0.1)',

    // Semantic
    success: '#10b981',
    successDim: '#d1fae5',
    warning: '#f59e0b',
    warningDim: '#fef3c7',
    error: '#ef4444',
    errorDim: '#fee2e2',

    // Borders
    borderSubtle: '#e5e5e5',
    borderMedium: '#d4d4d4',
    borderAccent: '#6d28d9',

    // Shadows
    shadowLight: 'rgba(0, 0, 0, 0.05)',
    shadowMedium: 'rgba(0, 0, 0, 0.1)',
  },

  // ═══════════════════════════════════════════════════════════════════
  // TYPOGRAPHY
  // ═══════════════════════════════════════════════════════════════════
  fonts: {
    main: "'Inter', -apple-system, system-ui, sans-serif",
    typewriter: "'IBM Plex Mono', 'SF Mono', ui-monospace, monospace",
    display: "'Inter', -apple-system, system-ui, sans-serif",
  },

  fontSizes: {
    xs: '0.75rem', // 12px
    sm: '0.875rem', // 14px
    base: '1rem', // 16px
    lg: '1.125rem', // 18px
    xl: '1.25rem', // 20px
    '2xl': '1.5rem', // 24px
    '3xl': '1.875rem', // 30px
    '4xl': '2.25rem', // 36px
    '5xl': '3rem', // 48px
    '6xl': '3.75rem', // 60px
  },

  fontWeights: {
    normal: 400,
    medium: 500,
    semibold: 600,
    bold: 700,
    extrabold: 800,
  },

  lineHeights: {
    tight: 1.25,
    normal: 1.5,
    relaxed: 1.75,
    loose: 2,
  },

  letterSpacing: {
    tighter: '-0.04em',
    tight: '-0.02em',
    normal: '0',
    wide: '0.05em',
    wider: '0.1em',
  },

  // ═══════════════════════════════════════════════════════════════════
  // SPACING (8px base grid)
  // ═══════════════════════════════════════════════════════════════════
  spacing: {
    px: '1px',
    0: '0',
    0.5: '0.125rem', // 2px
    1: '0.25rem', // 4px
    2: '0.5rem', // 8px
    3: '0.75rem', // 12px
    4: '1rem', // 16px
    5: '1.25rem', // 20px
    6: '1.5rem', // 24px
    8: '2rem', // 32px
    10: '2.5rem', // 40px
    12: '3rem', // 48px
    16: '4rem', // 64px
    20: '5rem', // 80px
    24: '6rem', // 96px
    32: '8rem', // 128px
  },

  // ═══════════════════════════════════════════════════════════════════
  // LAYOUT
  // ═══════════════════════════════════════════════════════════════════
  layout: {
    maxWidth: '800px',
    wideWidth: '1200px',
    sidebarWidth: '280px',
    navHeight: '64px',
    footerHeight: '80px',
    contentPadding: '2rem',
  },

  // ═══════════════════════════════════════════════════════════════════
  // SHADOWS (manuscript-style, subtle)
  // ═══════════════════════════════════════════════════════════════════
  shadows: {
    sm: '0 1px 2px rgba(0, 0, 0, 0.05)',
    md: '0 4px 6px rgba(0, 0, 0, 0.05)',
    lg: '0 10px 15px rgba(0, 0, 0, 0.05)',
    xl: '0 20px 25px rgba(0, 0, 0, 0.05)',
    // Manuscript "lifted page" effect
    page: '4px 4px 0 #ede9fe',
    // Inset for focus
    focusRing: '0 0 0 3px rgba(109, 40, 217, 0.3)',
  },

  // ═══════════════════════════════════════════════════════════════════
  // BORDERS
  // ═══════════════════════════════════════════════════════════════════
  borders: {
    radius: {
      none: '0',
      sm: '2px',
      md: '4px',
      lg: '8px',
      xl: '12px',
      full: '9999px',
    },
    width: {
      thin: '1px',
      medium: '2px',
      thick: '4px',
    },
  },

  // ═══════════════════════════════════════════════════════════════════
  // Z-INDEX SCALE
  // ═══════════════════════════════════════════════════════════════════
  zIndex: {
    base: 0,
    content: 10,
    dropdown: 100,
    sticky: 200,
    fixed: 300,
    modal: 400,
    toast: 500,
    tooltip: 600,
    overlay: 1000,
  },

  // ═══════════════════════════════════════════════════════════════════
  // BREAKPOINTS
  // ═══════════════════════════════════════════════════════════════════
  breakpoints: {
    sm: '640px',
    md: '768px',
    lg: '1024px',
    xl: '1280px',
    '2xl': '1536px',
  },
} as const;

export type Theme = typeof theme;

// Blueprint colors are available as theme.colors.blueprint, theme.colors.blueprintLight, etc.
// To use a full blueprint theme, create a separate theme object or use CSS variables.
