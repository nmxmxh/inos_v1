import { MEMORY_PAGES, type ResourceTier } from './layout';
import { clearViewCache } from '../../app/features/scenes/SceneWrapper';
import { initializeBridge, clearBridge, INOSBridge } from './bridge-state';

// Vite worker import syntax
import KernelWorkerUrl from './kernel.worker?worker&url';

// Re-export IDX_CONTEXT_ID_HASH for other modules
export { IDX_CONTEXT_ID_HASH } from './layout';

/**
 * Hash a string to a 32-bit integer for zero-copy comparison in SAB.
 */
function stringHash(s: string): number {
  let hash = 0;
  for (let i = 0; i < s.length; i++) {
    hash = (hash << 5) - hash + s.charCodeAt(i);
    hash |= 0; // Convert to 32bit integer
  }
  return hash;
}

declare global {
  interface Window {
    Go: any;
    __INOS_SAB__: SharedArrayBuffer;
    __INOS_MEM__: WebAssembly.Memory;
    __INOS_SAB_OFFSET__: number;
    __INOS_SAB_SIZE__: number;
    __INOS_TIER__: ResourceTier;
    __INOS_CONTEXT_ID__: string;
    __INOS_INIT_PROMISE__?: Promise<KernelInitResult>;
    __INOS_KERNEL_WORKER__?: Worker;
    getSystemSABAddress?: () => number;
    getSystemSABSize?: () => number;
  }
}

export type { ResourceTier } from './layout';

// Re-export layout config for backward compatibility
export const TIER_CONFIG = MEMORY_PAGES;

export interface KernelInitResult {
  memory?: WebAssembly.Memory; // Might be unavailable on main thread if only worker has it
  sabBase: SharedArrayBuffer;
  sabOffset: number;
  sabSize: number;
}

export async function initializeKernel(tier: ResourceTier = 'light'): Promise<KernelInitResult> {
  // 0. Update Context ID - Used to kill zombie loops
  const contextId = Math.random().toString(36).substring(2, 9);
  window.__INOS_CONTEXT_ID__ = contextId;
  console.log(`[Kernel] 🌐 New Context Instance: ${contextId} (Tier: ${tier})`);

  // Clear stale SAB views (Fixes memory leak on HMR/Re-init)
  clearViewCache();
  clearBridge();

  // 1. Atomic Locking - Prevent concurrent initialization spawns
  if (window.__INOS_INIT_PROMISE__) {
    console.log('[Kernel] Waiting for existing initialization to complete...');
    return window.__INOS_INIT_PROMISE__;
  }

  // Define the init logic as a single promise
  const init = async (): Promise<KernelInitResult> => {
    // 1. Singleton Check - Reuse existing memory if already initialized
    if (window.__INOS_SAB__ && window.__INOS_KERNEL_WORKER__) {
      console.log('[Kernel] Reusing existing SharedArrayBuffer and Worker singleton');
      return {
        sabBase: window.__INOS_SAB__,
        sabOffset: window.__INOS_SAB_OFFSET__ || 0,
        sabSize: window.__INOS_SAB_SIZE__ || 0,
      };
    }

    // 2. Spawn Kernel Worker
    console.log('[Kernel] Spawning Kernel Worker...');
    const worker = new Worker(KernelWorkerUrl, { type: 'module' });
    window.__INOS_KERNEL_WORKER__ = worker;

    const isDev = import.meta.env.DEV;
    const wasmUrl = isDev ? '/kernel.wasm' : '/kernel.wasm.br?v=2.0';

    return new Promise<KernelInitResult>((resolve, reject) => {
      const messageHandler = (event: MessageEvent<any>) => {
        const { type, sab, sabOffset, sabSize, error } = event.data;

        if (type === 'error') {
          console.error('[KernelWorker] Critical error:', error);
          worker.terminate();
          window.__INOS_KERNEL_WORKER__ = undefined;
          reject(new Error(error));
          return;
        }

        if (type === 'sab_functions_ready') {
          console.log('[Kernel] Kernel Worker SAB received');

          const { memory } = event.data;
          window.__INOS_SAB__ = sab;
          window.__INOS_MEM__ = memory;
          window.__INOS_SAB_OFFSET__ = sabOffset;
          window.__INOS_SAB_SIZE__ = sabSize;
          window.__INOS_TIER__ = tier;

          // Initialize centralized SAB bridge
          initializeBridge(sab, sabOffset, sabSize, memory);

          // Write Context ID Hash
          const contextHash = stringHash(contextId);
          const flags = INOSBridge.getFlagsView();
          if (flags) {
            flags[31] = contextHash; // IDX_CONTEXT_ID_HASH
          }

          console.log(`[Kernel] Worker SAB initialized. Context hash: ${contextHash}`);

          worker.removeEventListener('message', messageHandler);
          resolve({
            memory,
            sabBase: sab,
            sabOffset,
            sabSize,
          });
        }
      };

      worker.addEventListener('message', messageHandler);

      worker.postMessage({
        type: 'init',
        tier,
        wasmUrl,
      });
    });
  };

  window.__INOS_INIT_PROMISE__ = init();
  return window.__INOS_INIT_PROMISE__;
}

/**
 * Stop signal for the Go kernel.
 */
export function shutdownKernel() {
  if (window.__INOS_SAB__) {
    const flags = new Int32Array(window.__INOS_SAB__, 0, 16);
    // Standard INOS shutdown signal (IDX 0)
    Atomics.store(flags, 0, 1);
    Atomics.notify(flags, 0);
    console.log('[Kernel] 🛑 Sent shutdown signal to Go process');
  }
  (window as any).__INOS_INIT_PROMISE__ = undefined;
}
