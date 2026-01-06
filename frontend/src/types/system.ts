// ========== SAB Memory Layout Constants ==========
// Re-exported from auto-generated sab_layout.consts.ts
// Source of truth: protocols/schemas/system/v1/sab_layout.capnp

import {
  OFFSET_MODULE_REGISTRY,
  SIZE_MODULE_REGISTRY,
  MODULE_ENTRY_SIZE,
  MAX_MODULES_INLINE,
  OFFSET_ARENA,
  OFFSET_ATOMIC_FLAGS,
  IDX_SYSTEM_EPOCH,
  IDX_ARENA_ALLOCATOR,
} from '../wasm/layout';

export const SAB_LAYOUT = {
  OFFSET_MODULE_REGISTRY,
  SIZE_MODULE_REGISTRY,
  MODULE_ENTRY_SIZE,
  MAX_MODULES_INLINE,
  OFFSET_ARENA,
  OFFSET_ATOMIC_FLAGS,
  IDX_SYSTEM_EPOCH,
  IDX_ARENA_ALLOCATOR,
} as const;

// ========== Module Registry Entry (96 bytes) ==========
// Matches EnhancedModuleEntry from modules/sdk/src/registry.rs

export interface ModuleRegistryEntry {
  // Header (32 bytes)
  signature: bigint; // 0x494E4F5352454749 ("INOSREGI")
  idHash: number; // CRC32C of module ID
  versionMajor: number;
  versionMinor: number;
  versionPatch: number;
  flags: number;

  // Metadata (16 bytes)
  timestamp: bigint;
  dataOffset: number;
  dataSize: number;

  // Resource profile (8 bytes)
  resourceFlags: number;
  minMemoryMb: number;
  minGpuMemoryMb: number;
  minCpuCores: number;

  // Cost model (8 bytes)
  baseCost: number;
  perMbCost: number;
  perSecondCost: number;

  // Dependency/capability pointers (24 bytes)
  depTableOffset: number;
  depCount: number;
  maxVersionMajor: number;
  minVersionMajor: number;
  capTableOffset: number;
  capCount: number;

  // Module ID (12 bytes, null-terminated)
  moduleId: string;

  // Quick hash (4 bytes)
  quickHash: number;
}

// ========== Capability Entry (36 bytes) ==========
// Matches CapabilityEntry from modules/sdk/src/registry.rs

export interface CapabilityEntry {
  id: string; // 32 bytes, null-terminated
  minMemoryMb: number;
  flags: number;
  requiresGpu: boolean;
}

// ========== Resource Flags ==========

export const RESOURCE_FLAGS = {
  CPU_INTENSIVE: 0b0001,
  GPU_INTENSIVE: 0b0010,
  MEMORY_INTENSIVE: 0b0100,
  IO_INTENSIVE: 0b1000,
  NETWORK_INTENSIVE: 0b10000,
} as const;

export const MODULE_FLAGS = {
  HAS_EXTENDED_DATA: 0b0001,
  IS_ACTIVE: 0b0010,
  HAS_OVERFLOW: 0b0100,
} as const;

export const CAPABILITY_FLAGS = {
  REQUIRES_GPU: 0b0001,
} as const;

// ========== Magic Constants ==========

export const REGISTRY_SIGNATURE = 0x494e4f5352454749n; // "INOSREGI"

// ========== Legacy Types (for backward compatibility) ==========

export interface KernelStats {
  nodes: number;
  particles: number;
  sector: number;
  fps: number;
  epochPlane: number;
  sabCommits: number;
  meshNodes: number;
  wasmUnits: number;
  sabUsage: number;
}

export interface UnitState {
  id: string;
  active: boolean;
  capabilities: string[];
  config?: Record<string, any>;
  // Enhanced with registry data
  registryEntry?: ModuleRegistryEntry;
}

export interface KernelInstance {
  memory: WebAssembly.Memory;
  dispatch: (eventType: string, payload?: any) => void;
  getStats: () => KernelStats;
}

export type SystemStatus = 'initializing' | 'booting' | 'ready' | 'error';
