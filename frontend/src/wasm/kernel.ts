import { MEMORY_PAGES, type ResourceTier } from './layout';
import { clearViewCache } from '../../app/features/scenes/SceneWrapper';

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
    getSystemSABAddress?: () => number;
    getSystemSABSize?: () => number;
  }
}

export type { ResourceTier } from './layout';

// Re-export layout config for backward compatibility
export const TIER_CONFIG = MEMORY_PAGES;

export interface KernelInitResult {
  memory: WebAssembly.Memory;
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

  // 1. Atomic Locking - Prevent concurrent initialization spawns
  if (window.__INOS_INIT_PROMISE__) {
    console.log('[Kernel] Waiting for existing initialization to complete...');
    return window.__INOS_INIT_PROMISE__;
  }

  // Define the init logic as a single promise
  const init = async (): Promise<KernelInitResult> => {
    // 1. Singleton Check - Reuse existing memory if already initialized
    if (window.__INOS_SAB__ && (window as any).__INOS_MEM__) {
      console.log('[Kernel] Reusing existing SharedArrayBuffer and Memory singleton');
      return {
        memory: (window as any).__INOS_MEM__ as WebAssembly.Memory,
        sabBase: window.__INOS_SAB__,
        sabOffset: window.__INOS_SAB_OFFSET__ || 0,
        sabSize: window.__INOS_SAB_SIZE__ || 0,
      };
    }

    // 1b. Load wasm_exec.js (Go runtime)
    if (!window.Go) {
      const wasmExecScript = document.createElement('script');
      wasmExecScript.src = '/wasm_exec.js';
      await new Promise((resolve, reject) => {
        wasmExecScript.onload = resolve;
        wasmExecScript.onerror = reject;
        document.head.appendChild(wasmExecScript);
      });
    }

    const config = TIER_CONFIG[tier];

    // 2. Create SharedArrayBuffer for zero-copy architecture
    const sharedMemory = new WebAssembly.Memory({
      initial: config.initial,
      maximum: config.maximum,
      shared: true,
    });

    // 3. Load and instantiate Go kernel using streaming (Optimized)
    const go = new window.Go();
    const response = fetch('/kernel.wasm.br');

    const result = await WebAssembly.instantiateStreaming(response, {
      ...go.importObject,
      env: {
        ...go.importObject.env,
        memory: sharedMemory,
      },
    });

    console.log('[Kernel] Starting Go WASM...');
    go.run(result.instance);
    console.log('[Kernel] go.run() returned (Go WASM started)');

    // 4. Wait for Kernel to export SAB functions
    const maxWaitMs = 5000;
    const startTime = Date.now();

    console.log('[Kernel] Waiting for getSystemSABAddress/getSystemSABSize...');
    while (!window.getSystemSABAddress || !window.getSystemSABSize) {
      if (Date.now() - startTime > maxWaitMs) {
        console.warn('[Kernel] Timeout waiting for SAB functions');
        break;
      }
      await new Promise(resolve => setTimeout(resolve, 10));
    }
    console.log('[Kernel] SAB functions available');

    // 5. Setup SharedArrayBuffer globals
    const memoryBuffer = sharedMemory.buffer;
    // Guard against SharedArrayBuffer being undefined (ReferenceError) or hidden
    const isSAB =
      typeof SharedArrayBuffer !== 'undefined' && memoryBuffer instanceof SharedArrayBuffer;
    const isLikelySAB = (memoryBuffer as any).constructor?.name === 'SharedArrayBuffer';

    if (!isSAB && !isLikelySAB) {
      console.error(
        '[Kernel] ❌ SharedArrayBuffer is not available. This site must be cross-origin isolated (COOP/COEP) to use shared memory.'
      );
      throw new Error('SharedArrayBuffer is missing. Check COOP/COEP headers.');
    }

    const sabBase = memoryBuffer as unknown as SharedArrayBuffer;
    let sabOffset = 0;
    let sabSize = sabBase.byteLength;

    // Only overwrite if Kernel provides NON-ZERO values (meaning it's already initialized)
    // During boot, we MUST use the full buffer length.
    if (window.getSystemSABAddress && window.getSystemSABSize) {
      const kAddr = window.getSystemSABAddress();
      const kSize = window.getSystemSABSize();
      if (kSize > 0) {
        sabOffset = kAddr;
        sabSize = kSize;
      }
    }

    window.__INOS_MEM__ = sharedMemory;
    window.__INOS_SAB__ = sabBase;
    window.__INOS_SAB_OFFSET__ = sabOffset;
    window.__INOS_SAB_SIZE__ = sabSize;
    (window as any).__INOS_SAB_INT32__ = new Int32Array(sabBase);
    (window as any).__INOS_TIER__ = tier;

    // 5b. Write Context ID Hash to SAB for zero-copy validation
    const contextHash = stringHash(contextId);
    const contextHashIndex = 31; // IDX_CONTEXT_ID_HASH from layout
    (window as any).__INOS_SAB_INT32__[contextHashIndex] = contextHash;
    console.log(`[Kernel] Context hash written to SAB[${contextHashIndex}]: ${contextHash}`);

    // 6. Wait for Kernel to be ready for SAB injection
    // Wait for initializeSharedMemory function to be registered by Go kernel
    const waitForKernelReady = async (): Promise<void> => {
      const maxAttempts = 500; // 5 seconds max
      for (let i = 0; i < maxAttempts; i++) {
        if ((window as any).initializeSharedMemory) {
          console.log(`[Kernel] initializeSharedMemory available after ${i * 10}ms`);
          // Small delay to ensure kernel state has transitioned
          await new Promise(resolve => setTimeout(resolve, 50));
          return;
        }
        await new Promise(resolve => setTimeout(resolve, 10));
      }
      console.warn('[Kernel] initializeSharedMemory timeout - proceeding anyway');
    };

    await waitForKernelReady();

    // 7. Inject SAB into Go Kernel to start Supervisor threads
    // This calls InjectSAB which starts discovery loop, signal listener, economy loop
    if ((window as any).initializeSharedMemory) {
      console.log('[Kernel] Injecting SAB into kernel (starting supervisor threads)...');
      const result = (window as any).initializeSharedMemory(sabOffset, sabSize);
      if (result?.error) {
        console.error('[Kernel] Failed to inject SAB:', result.error);
      } else {
        console.log('[Kernel] ✅ Supervisor threads started');
      }
    } else {
      console.warn(
        '[Kernel] initializeSharedMemory not available - supervisor threads will not start'
      );
    }

    return {
      memory: sharedMemory,
      sabBase,
      sabOffset,
      sabSize,
    };
  };

  window.__INOS_INIT_PROMISE__ = init();

  try {
    const result = await window.__INOS_INIT_PROMISE__;
    return result;
  } finally {
    // Clean up promise ref once settled so it can be re-run if error
    // but keep SAB/MEM on window for singleton persistence.
    // Actually, we want to keep it around to serve future calls immediately.
    // But if we want to allow RE-INIT after error, we clear it on failure.
  }
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
