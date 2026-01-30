import { create } from 'zustand';
import { initializeKernel, ResourceTier, shutdownKernel } from '../wasm/kernel';
import { resolveMeshBootstrapConfig } from '../wasm/mesh';
import { loadAllModules, loadModule } from '../wasm/module-loader';
import { RegistryReader } from '../wasm/registry';
import { dispatch } from '../wasm/dispatch';

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

      // 1. Initialize kernel
      const meshConfig = resolveMeshBootstrapConfig();
      const { memory, sabBase } = await initializeKernel(tier, meshConfig);

      // Memory might be null on main-thread if kernel is isolated in a worker (Split Memory Mode)
      // This is expected and should not halt initialization.
      if (memory) {
        (window as any).__INOS_MEM__ = memory;
      }

      const currentContext = window.__INOS_CONTEXT_ID__;
      console.log(
        `[System] ✅ Kernel initialized (Context: ${currentContext}, Mode: ${memory ? 'Unified' : 'Split'})`
      );

      // 2. Start registry scanning (using SAB if memory is null)
      const scanBuffer = memory ? memory.buffer : sabBase; // eslint-disable-line
      set({ sab: (window as any).__INOS_SAB__ || sabBase });

      const scannerId = setInterval(() => {
        if (window.__INOS_CONTEXT_ID__ !== currentContext) {
          console.log(`[System] 💀 Killing stale scanner: ${currentContext}`);
          clearInterval(scannerId);
          return;
        }
        get().scanRegistry(scanBuffer);
      }, 2000);

      get().scanRegistry(scanBuffer);

      // 3. Load modules (diagnostics only - compute is in worker)
      // If we don't have shared system memory, we skip loading on main thread or provide local memory
      let loadedModules = {};
      if (memory) {
        loadedModules = await loadAllModules(memory);
        console.log('[System] ✅ Modules loaded:', Object.keys(loadedModules));
      } else {
        console.log('[System] ⚠️ Split Memory Mode: Skipping main-thread module loading');
      }

      // 4. Initialize Dispatcher (will route to worker once spawned)
      // We spawn a compute worker that handles all physics/math
      const systemSAB = (window as any).__INOS_SAB__ || sabBase;
      const sabOffset = (window as any).__INOS_SAB_OFFSET__ || 0;

      // Import and spawn compute worker
      const ComputeWorker = await import('../wasm/compute.worker.ts?worker');
      const worker = new ComputeWorker.default();

      // Initialize worker with SAB (Memory is created inside worker)
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

      // Store worker reference for dispatch routing
      (window as any).__INOS_COMPUTE_WORKER__ = worker;

      // Initialize dispatcher without local exports - will create worker route
      dispatch.initialize(null as any, memory || ({} as any));

      // Register the worker route manually
      const dispatchInternal = dispatch.internal();
      if (dispatchInternal) {
        (dispatchInternal as any).workers.set('compute:main', {
          worker,
          unit: 'boids',
          role: 'main',
          ready: true,
        });
        // Also register math/boids units as available
        (dispatchInternal as any).workers.set('boids:main', {
          worker,
          unit: 'boids',
          role: 'main',
          ready: true,
        });
        (dispatchInternal as any).workers.set('math:main', {
          worker,
          unit: 'math',
          role: 'main',
          ready: true,
        });
        (dispatchInternal as any).workers.set('drone:main', {
          worker,
          unit: 'drone',
          role: 'main',
          ready: true,
        });

        // Register capabilities immediately so UI doesn't have to wait for 2s scan loop
        dispatch.register('boids', ['step_physics', 'init_population', 'evolve_batch']);
        dispatch.register('math', [
          'matrix_multiply',
          'fft',
          'interpolate',
          'compute_instance_matrices',
        ]);
        dispatch.register('drone', ['init', 'step_physics']);
      }
      console.log('[System] ✅ Dispatcher initialized (Worker mode)');

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
