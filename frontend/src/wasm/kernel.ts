/**
 * Kernel initialization logic for INOS Go WASM kernel.
 * Handles loading wasm_exec.js, creating SharedArrayBuffer, and instantiating the kernel.
 */

declare global {
  interface Window {
    Go: any;
    __INOS_SAB__: SharedArrayBuffer;
    __INOS_MEM__: WebAssembly.Memory;
    __INOS_SAB_OFFSET__: number;
    __INOS_SAB_SIZE__: number;
    __INOS_CONTEXT_ID__: string;
    __INOS_INIT_PROMISE__?: Promise<KernelInitResult>;
    getSystemSABAddress?: () => number;
    getSystemSABSize?: () => number;
  }
}

export type ResourceTier = 'light' | 'moderate' | 'heavy' | 'dedicated';

export const TIER_CONFIG: Record<ResourceTier, { initial: number; maximum: number }> = {
  light: { initial: 512, maximum: 1024 }, // 32MB -> 64MB
  moderate: { initial: 1024, maximum: 2048 }, // 64MB -> 128MB
  heavy: { initial: 4096, maximum: 8192 }, // 256MB -> 512MB
  dedicated: { initial: 8192, maximum: 16384 }, // 512MB -> 1GB
};

export interface KernelInitResult {
  memory: WebAssembly.Memory;
  sabBase: SharedArrayBuffer;
  sabOffset: number;
  sabSize: number;
}

export async function initializeKernel(tier: ResourceTier = 'moderate'): Promise<KernelInitResult> {
  // 0. Update Context ID - Used to kill zombie loops
  const contextId = Math.random().toString(36).substring(2, 9);
  window.__INOS_CONTEXT_ID__ = contextId;
  console.log(`[Kernel] 🌐 New Context Instance: ${contextId} (Tier: ${tier})`);

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

    // 3. Load and instantiate Go kernel
    const go = new window.Go();
    const response = await fetch('/kernel.wasm');

    if (!response.ok) {
      throw new Error(`Failed to load kernel.wasm: ${response.statusText}`);
    }

    const wasmBytes = await response.arrayBuffer();
    const result = await WebAssembly.instantiate(wasmBytes, {
      ...go.importObject,
      env: {
        ...go.importObject.env,
        memory: sharedMemory,
      },
    });

    go.run(result.instance);

    // 4. Wait for Kernel to export SAB functions
    const maxWaitMs = 5000;
    const startTime = Date.now();

    while (!window.getSystemSABAddress || !window.getSystemSABSize) {
      if (Date.now() - startTime > maxWaitMs) {
        break;
      }
      await new Promise(resolve => setTimeout(resolve, 10));
    }

    // 5. Setup SharedArrayBuffer globals
    const memoryBuffer = sharedMemory.buffer;

    if (!(memoryBuffer instanceof SharedArrayBuffer)) {
      throw new Error('WebAssembly.Memory.buffer is not a SharedArrayBuffer');
    }

    const sabBase = memoryBuffer as SharedArrayBuffer;
    let sabOffset = 0;
    let sabSize = sabBase.byteLength;

    if (window.getSystemSABAddress && window.getSystemSABSize) {
      sabOffset = window.getSystemSABAddress();
      sabSize = window.getSystemSABSize();
    }

    window.__INOS_MEM__ = sharedMemory;
    window.__INOS_SAB__ = sabBase;
    window.__INOS_SAB_OFFSET__ = sabOffset;
    window.__INOS_SAB_SIZE__ = sabSize;
    (window as any).__INOS_SAB_INT32__ = new Int32Array(sabBase);

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
