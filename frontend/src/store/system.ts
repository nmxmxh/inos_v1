import { create } from 'zustand';
import { initializeKernel, ResourceTier, shutdownKernel } from '../wasm/kernel';
import { resolveMeshBootstrapConfig } from '../wasm/mesh';
import { loadAllModules, loadModule } from '../wasm/module-loader';
import { RegistryReader } from '../wasm/registry';
import { dispatch } from '../wasm/dispatch';
import { IDX_REGISTRY_EPOCH } from '../wasm/layout';

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

export type SystemStatus = 'uninitialized' | 'initializing' | 'booting' | 'ready' | 'error';

export interface SystemStore {
  status: SystemStatus;
  units: Record<string, UnitState>;
  moduleExports: Record<string, any>;
  stats: KernelStats;
  error: Error | null;
  sab: SharedArrayBuffer | null;

  // Actions
  initialize: (tier?: ResourceTier) => Promise<void>;
  loadModule: (name: string) => Promise<void>;
  registerUnit: (unit: UnitState) => void;
  updateStats: (stats: Partial<KernelStats>) => void;
  setError: (error: Error) => void;
  scanRegistry: (buffer: ArrayBufferLike) => void;
  signalModule: (name: string) => void;
  setMetric: (name: keyof KernelStats, value: number) => void;
  cleanup: () => void;
}

let registryReader: RegistryReader | null = null;

export const useSystemStore = create<SystemStore>((set, get) => ({
  status: 'initializing',
  units: {},
  moduleExports: {},
  sab: null,
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

  scanRegistry: (buffer: ArrayBufferLike) => {
    if (!registryReader || (registryReader as any)['buffer'] !== buffer) {
      registryReader = new RegistryReader(buffer, (window as any).__INOS_SAB_OFFSET__ || 0);
    }

    const currentUnits = get().units;
    const modules = registryReader.scan();
    let hasChanges = false;

    // Fast check for changes
    if (Object.keys(modules).length !== Object.keys(currentUnits).length - 1) {
      // -1 for kernel which is manual
      hasChanges = true;
    }

    if (!hasChanges) {
      for (const id in modules) {
        const data = modules[id];
        const existing = currentUnits[id];
        if (
          !existing ||
          existing.active !== data.active ||
          existing.capabilities.length !== data.capabilities.length
        ) {
          hasChanges = true;
          break;
        }
      }
    }

    if (!hasChanges) return;

    set(state => {
      const updatedUnits = { ...state.units };

      Object.values(modules).forEach(data => {
        updatedUnits[data.id] = {
          id: data.id,
          active: data.active,
          capabilities: data.capabilities,
        };
        // Also register with the dispatcher store
        dispatch.register(data.id, data.capabilities);
      });

      return { units: updatedUnits };
    });
  },

  initialize: async (tier: ResourceTier = 'moderate') => {
    if (get().status !== 'initializing' && get().status !== 'error') return;
    set({ status: 'booting' });

    try {
      console.log(`[System] Initializing INOS (Tier: ${tier})...`);

      // 1. Parallelize Kernel & Compute Worker Spawning
      const meshConfig = resolveMeshBootstrapConfig();

      console.log('[System] Launching parallel boot sequence...');

      const bootTasks = async () => {
        // Task A: Kernel Initialization
        const kernelPromise = initializeKernel(tier, meshConfig);

        // Task B: Compute Worker Spawning (Concurrently)
        const workerPromise = (async () => {
          const ComputeWorker = await import('../wasm/compute.worker.ts?worker');
          const worker = new ComputeWorker.default();
          return worker;
        })();

        const [kernelResult, worker] = await Promise.all([kernelPromise, workerPromise]);
        const { memory, sabBase } = kernelResult;
        const systemSAB = (window as any).__INOS_SAB__ || sabBase;
        const sabOffset = (window as any).__INOS_SAB_OFFSET__ || 0;
        const contextId = window.__INOS_CONTEXT_ID__;

        if (memory) {
          (window as any).__INOS_MEM__ = memory;
        }
        set({ sab: systemSAB });

        console.log(`[System] ✅ Kernel & Worker spawned (Context: ${contextId})`);

        // 2. Immediate Dispatcher Setup (Stabilize early!)
        dispatch.initialize(null as any, memory || ({} as any));

        // 3. Worker Initialization
        await new Promise<void>((resolve, reject) => {
          worker.onmessage = (event: MessageEvent) => {
            if (event.data.type === 'ready') {
              console.log('[System] ✅ Compute Worker ready');
              resolve();
            } else if (event.data.type === 'error') {
              reject(new Error(event.data.error));
            }
          };
          worker.onerror = reject;
          worker.postMessage({
            type: 'init',
            sab: systemSAB,
            sabOffset,
            sabSize: systemSAB.byteLength,
            identity: meshConfig.identity,
          });
        });

        // 4. specialized worker role registration
        const di = dispatch.internal();
        if (di) {
          dispatch.bind('compute:main', {
            worker,
            unit: 'compute',
            role: 'main',
            ready: true,
          });
        }

        // 6. Diagnostics Module (Main Thread)
        let loadedModules = {};
        if (memory) {
          loadedModules = await loadAllModules(memory);
          console.log('[System] ✅ Modules loaded:', Object.keys(loadedModules));
        }

        return { memory, sabBase, worker, loadedModules, contextId };
      };

      const { memory, sabBase, loadedModules, contextId: currentContext } = await bootTasks();
      const scanBuffer = memory ? memory.buffer : sabBase;

      // Efficient Registry Watcher: Replaces 2s polling with Atomics.waitAsync signaling
      // CRITICAL: Must be started before we wait for capabilities!
      const startRegistryWatcher = async () => {
        const sab = (window as any).__INOS_SAB__ || sabBase;
        const flags = new Int32Array(sab, 0, 32);

        // Initial scan immediately
        get().scanRegistry(scanBuffer);

        while (window.__INOS_CONTEXT_ID__ === currentContext) {
          const currentEpoch = Atomics.load(flags, IDX_REGISTRY_EPOCH);

          if (typeof (Atomics as any).waitAsync === 'function') {
            const result = (Atomics as any).waitAsync(flags, IDX_REGISTRY_EPOCH, currentEpoch);
            await result.value;
          } else {
            await new Promise(resolve => setTimeout(resolve, 1000));
          }

          // Scan after signal
          get().scanRegistry(scanBuffer);
        }
      };

      // Start the watcher background loop immediately
      startRegistryWatcher();

      // Ensure holistic readiness before proceeding to 'ready' status
      const di = dispatch.internal();
      if (di) {
        console.log('[System] Verifying holistic readiness...');
        await di.waitForCapability('compute');
      }

      console.log('[System] ✅ System is READY.');

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
        if (window.__INOS_CONTEXT_ID__ !== currentContext) {
          console.log(`[System] 💀 Killing stale loop: ${currentContext}`);
          (window as any).__INOS_LOOP_ACTIVE__ = false;
          return;
        }
        if (!get().moduleExports) return; // Cleanup check
        const now = performance.now();
        frames++;

        if (now > lastTime + 1000) {
          get().updateStats({ fps: frames });
          frames = 0;
          lastTime = now;
        }

        (window as any).__INOS_RAF_ID__ = requestAnimationFrame(loop);
      };

      if (!(window as any).__INOS_LOOP_ACTIVE__) {
        (window as any).__INOS_LOOP_ACTIVE__ = true;
        loop();
      }

      console.log('[System] ✅ INOS ready');
    } catch (error) {
      console.error('[System] Initialization failed:', error);
      set({ status: 'error', error: error as Error });
    }
  },

  loadModule: async (name: string) => {
    const { status, moduleExports } = get();
    if (status !== 'ready') return;
    if (moduleExports && moduleExports[name]) return; // Already loaded

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

  setMetric: (name: keyof KernelStats, value: number) => {
    set(state => ({
      stats: {
        ...state.stats,
        [name]: value,
      },
    }));
  },

  cleanup: () => {
    if ((window as any).__INOS_RAF_ID__) {
      cancelAnimationFrame((window as any).__INOS_RAF_ID__);
      (window as any).__INOS_RAF_ID__ = null;
    }
    (window as any).__INOS_LOOP_ACTIVE__ = false;
    // Signal shutdown to Go kernel
    shutdownKernel();
    // Clear modules to allow GC
    set({ moduleExports: undefined, status: 'uninitialized' });
  },
}));
