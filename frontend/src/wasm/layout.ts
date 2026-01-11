/**
 * INOS SAB Layout - TypeScript
 *
 * Re-exports auto-generated constants from Cap'n Proto schema
 * and adds tier-aware helper functions.
 *
 * Constants are auto-generated from: protocols/schemas/system/v1/sab_layout.capnp
 * Run: make proto
 */

// Re-export all generated constants
export * from '../../bridge/generated/protocols/schemas/system/v1/sab_layout.consts';

// Import specific constants for helpers
import {
  SAB_SIZE_LIGHT,
  SAB_SIZE_MODERATE,
  SAB_SIZE_HEAVY,
  SAB_SIZE_DEDICATED,
  OFFSET_ARENA,
  SIZE_ARENA_METADATA,
  OFFSET_BIRD_BUFFER_A,
  OFFSET_BIRD_BUFFER_B,
  OFFSET_MATRIX_BUFFER_A,
  OFFSET_MATRIX_BUFFER_B,
  BIRD_STRIDE,
  MATRIX_STRIDE,
} from '../../bridge/generated/protocols/schemas/system/v1/sab_layout.consts';

// ========== RESOURCE TIERS ==========

export type ResourceTier = 'light' | 'moderate' | 'heavy' | 'dedicated';

/** SAB size in bytes for each tier */
export const SAB_SIZE = {
  light: SAB_SIZE_LIGHT,
  moderate: SAB_SIZE_MODERATE,
  heavy: SAB_SIZE_HEAVY,
  dedicated: SAB_SIZE_DEDICATED,
} as const;

/** Memory pages (64KB each) for WebAssembly.Memory
 * Sizes are now ACTUAL SAB size (no Go reservation)
 * Light: 32MB, Moderate: 64MB, Heavy: 128MB, Dedicated: 256MB
 */
export const MEMORY_PAGES = {
  light: { initial: 512, maximum: 1024 }, // 32-64MB
  moderate: { initial: 1024, maximum: 2048 }, // 64-128MB
  heavy: { initial: 2048, maximum: 4096 }, // 128-256MB
  dedicated: { initial: 4096, maximum: 16384 }, // 256MB-1GB
} as const;

// ========== TIER-AWARE HELPERS ==========

/**
 * Compute tier-specific entity limits based on available SAB space.
 */
export function computeTierLimits(tier: ResourceTier) {
  const sabSize = SAB_SIZE[tier];
  const fixedOverhead = OFFSET_ARENA + SIZE_ARENA_METADATA;
  const perEntity = BIRD_STRIDE * 2 + MATRIX_STRIDE * 8 * 2;
  const available = sabSize - fixedOverhead;
  const maxEntities = Math.floor(available / perEntity);

  const defaults: Record<ResourceTier, { recommended: number; maximum: number }> = {
    light: { recommended: 1000, maximum: 5000 },
    moderate: { recommended: 5000, maximum: 15000 },
    heavy: { recommended: 15000, maximum: 50000 },
    dedicated: { recommended: 50000, maximum: 100000 },
  };

  return {
    sabSize,
    sabSizeMB: sabSize / (1024 * 1024),
    maxPossible: maxEntities,
    ...defaults[tier],
    memory: MEMORY_PAGES[tier],
  };
}

/**
 * Get layout configuration for a specific tier.
 */
export function getLayoutConfig(tier: ResourceTier = 'light') {
  return {
    tier,
    ...computeTierLimits(tier),
  };
}

/**
 * Get the correct buffer offset based on current epoch (ping-pong).
 */
export function getActiveBirdBuffer(epoch: number): number {
  return epoch % 2 === 0 ? OFFSET_BIRD_BUFFER_A : OFFSET_BIRD_BUFFER_B;
}

export function getActiveMatrixBuffer(epoch: number): number {
  return epoch % 2 === 0 ? OFFSET_MATRIX_BUFFER_A : OFFSET_MATRIX_BUFFER_B;
}

/** Default layout configuration (Light tier) */
export const DEFAULT_CONFIG = getLayoutConfig('light');
