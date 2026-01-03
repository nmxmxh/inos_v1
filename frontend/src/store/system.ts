import { create } from 'zustand';
import { initializeKernel } from '../wasm/kernel';
import { loadAllModules, loadModule } from '../wasm/module-loader';
import { RegistryReader } from '../wasm/registry';

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
}

export interface SystemStore {
  status: 'initializing' | 'booting' | 'ready' | 'error';
  units: Record<string, UnitState>;
  moduleExports: Record<string, any>;
  stats: KernelStats;
  error: Error | null;

  // Actions
  initialize: () => Promise<void>;
  loadModule: (name: string) => Promise<void>;
  registerUnit: (unit: UnitState) => void;
  updateStats: (stats: Partial<KernelStats>) => void;
  setError: (error: Error) => void;
  scanRegistry: (memory: WebAssembly.Memory) => void;
  signalModule: (name: string) => void;
  pollAll: () => void;
  setMetric: (name: keyof KernelStats, value: number) => void;
}

let registryReader: RegistryReader | null = null;

export const useSystemStore = create<SystemStore>((set, get) => ({
  status: 'initializing',
  units: {},
  moduleExports: {},
  stats: {
    nodes: 1,
    particles: 1000,
    sector: 0,
    fps: 0,
    epochPlane: 0,
    sabCommits: 0,
    meshNodes: 0,
    wasmUnits: 0,
    sabUsage: 0,
  },
  error: null,

  scanRegistry: (memory: WebAssembly.Memory) => {
    if (!registryReader || (registryReader as any)['memory'] !== memory) {
      registryReader = new RegistryReader(memory, (window as any).__INOS_SAB_OFFSET__ || 0);
    }

    const modules = registryReader.scan();

    set(state => {
      const updatedUnits = { ...state.units };

      Object.values(modules).forEach(data => {
        updatedUnits[data.id] = {
          id: data.id,
          active: data.active,
          capabilities: data.capabilities,
        };
      });

      return { units: updatedUnits };
    });
  },

  initialize: async () => {
    if (get().status !== 'initializing' && get().status !== 'error') return;
    set({ status: 'booting' });

    try {
      console.log('[System] Initializing INOS...');

      // 1. Initialize kernel
      const { memory } = await initializeKernel();
      (window as any).__INOS_MEM__ = memory;
      console.log('[System] ✅ Kernel initialized');

      // 2. Start registry scanning
      setInterval(() => {
        get().scanRegistry(memory);
      }, 2000);

      get().scanRegistry(memory);

      // 3. Load modules
      const loadedModules = await loadAllModules(memory);
      console.log('[System] ✅ Modules loaded:', Object.keys(loadedModules));

      // 4. Update state
      set({
        status: 'ready',
        moduleExports: loadedModules,
        units: {
          kernel: {
            id: 'kernel',
            active: true,
            capabilities: ['orchestration', 'mesh', 'gossip'],
          },
          ...Object.keys(loadedModules).reduce(
            (acc, name) => ({
              ...acc,
              [name]: { id: name, active: true, capabilities: [] },
            }),
            {}
          ),
        },
      });

      // 5. Start polling loop
      let lastTime = performance.now();
      let frames = 0;

      const loop = () => {
        const now = performance.now();
        frames++;

        get().pollAll();

        if (now > lastTime + 1000) {
          get().updateStats({ fps: frames });
          frames = 0;
          lastTime = now;
        }

        requestAnimationFrame(loop);
      };

      loop();

      console.log('[System] ✅ INOS ready');
    } catch (error) {
      console.error('[System] Initialization failed:', error);
      set({ status: 'error', error: error as Error });
    }
  },

  loadModule: async (name: string) => {
    const { status, moduleExports } = get();
    if (status !== 'ready') return;
    if (moduleExports[name]) return; // Already loaded

    try {
      console.log(`[System] Lazy loading module: ${name}...`);
      const mem = (window as any).__INOS_MEM__; // The WebAssembly.Memory instance

      if (!mem) throw new Error('Kernel memory not found for module loading');

      const result = await loadModule(name, mem);

      set(state => ({
        moduleExports: { ...state.moduleExports, [name]: result.exports },
        units: {
          ...state.units,
          [name]: { id: name, active: true, capabilities: [] },
        },
      }));

      console.log(`[System] ✅ Module ${name} loaded dynamically`);
    } catch (error) {
      console.error(`[System] Failed to lazy load ${name}:`, error);
    }
  },

  registerUnit: (unit: UnitState) => {
    set(state => ({
      units: {
        ...state.units,
        [unit.id]: unit,
      },
    }));
  },

  updateStats: (stats: Partial<KernelStats>) => {
    set(state => ({
      stats: {
        ...state.stats,
        ...stats,
      },
    }));
  },

  setError: (error: Error) => {
    set({ status: 'error', error });
  },

  signalModule: () => {
    const sab = (window as any).__INOS_SAB__;
    if (!sab) return;

    const flags = new Int32Array(sab, 0, 16);
    Atomics.add(flags, 1, 1);
    Atomics.notify(flags, 1);
  },

  pollAll: () => {
    const { moduleExports } = get();

    for (const moduleName in moduleExports) {
      const exports = moduleExports[moduleName];
      if (exports && typeof exports.poll === 'function') {
        try {
          exports.poll();
        } catch (e) {
          console.error(`[System] Poll failed for ${moduleName}:`, e);
        }
      }
    }
  },

  setMetric: (name: keyof KernelStats, value: number) => {
    set(state => ({
      stats: {
        ...state.stats,
        [name]: value,
      },
    }));
  },
}));
