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
    economics?: {
      getBalance: (did?: string) => Promise<number>;
      getAccountInfo: (did?: string) => Promise<{ offset: number; exists: boolean } | null>;
      getStats: () => Promise<any>;
      grantBonus: (did: string, bonus: number) => Promise<boolean>;
    };
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

    const isSafari = /^((?!chrome|android).)*safari/i.test(navigator.userAgent);
    const isIOS =
      /iPad|iPhone|iPod/.test(navigator.userAgent) ||
      (navigator.userAgent.includes('Mac') && navigator.maxTouchPoints > 1);

    const isDev = import.meta.env.DEV;
    // Default URL logic
    let wasmUrl = isDev ? '/kernel.wasm' : '/kernel.wasm.br?v=2.0';

    // Safari/iOS Fix: Force uncompressed WASM and aggressively bust cache
    if (isSafari || isIOS) {
      console.log('[Kernel] Safari/iOS detected: forcing uncompressed WASM and cache bust');
      // Strip .br extension
      wasmUrl = wasmUrl.replace('.br', '');
      // Append cache buster
      const separator = wasmUrl.includes('?') ? '&' : '?';
      wasmUrl = `${wasmUrl}${separator}t=${Date.now()}`;
    }

    if (isIOS || isSafari) {
      console.log('[Kernel] Safari/iOS detected, prioritizing main-thread initialization');
      return await initializeKernelOnMainThread(tier, wasmUrl, contextId);
    }

    // Try Worker-based initialization first, fall back to main thread
    try {
      return await initializeKernelInWorker(tier, wasmUrl, contextId);
    } catch (workerError) {
      console.warn(
        '[Kernel] Worker initialization failed, falling back to main thread:',
        workerError
      );
      return await initializeKernelOnMainThread(tier, wasmUrl, contextId);
    }
  };

  window.__INOS_INIT_PROMISE__ = init();
  return window.__INOS_INIT_PROMISE__;
}

/**
 * Initialize kernel in a dedicated Web Worker (preferred path)
 */
async function initializeKernelInWorker(
  tier: ResourceTier,
  wasmUrl: string,
  contextId: string
): Promise<KernelInitResult> {
  console.log('[Kernel] Spawning Kernel Worker...');
  const worker = new Worker(KernelWorkerUrl, { type: 'module' });
  window.__INOS_KERNEL_WORKER__ = worker;

  return new Promise<KernelInitResult>((resolve, reject) => {
    const timeoutId = setTimeout(() => {
      worker.terminate();
      window.__INOS_KERNEL_WORKER__ = undefined;
      reject(new Error('Kernel worker initialization timeout (10s)'));
    }, 10000);

    const messageHandler = (event: MessageEvent<any>) => {
      const { type, sab, sabOffset, sabSize, error } = event.data;

      if (type === 'error') {
        clearTimeout(timeoutId);
        console.error('[KernelWorker] Critical error:', error);
        worker.terminate();
        window.__INOS_KERNEL_WORKER__ = undefined;
        reject(new Error(error));
        return;
      }

      if (type === 'sab_functions_ready') {
        clearTimeout(timeoutId);
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

        // Economics data is read directly from SAB at OFFSET_ECONOMICS
        // Use the useEconomics() hook for zero-copy access - no worker messaging needed

        resolve({
          memory,
          sabBase: sab,
          sabOffset,
          sabSize,
        });
      }
    };

    worker.addEventListener('message', messageHandler);
    worker.addEventListener('error', e => {
      clearTimeout(timeoutId);
      worker.terminate();
      window.__INOS_KERNEL_WORKER__ = undefined;
      reject(new Error(`Worker error: ${e.message}`));
    });

    worker.postMessage({
      type: 'init',
      tier,
      wasmUrl,
    });
  });
}

/**
 * Initialize kernel on main thread (fallback for iOS/Safari)
 * This blocks the main thread during init but allows the kernel to run.
 */
async function initializeKernelOnMainThread(
  tier: ResourceTier,
  wasmUrl: string,
  contextId: string
): Promise<KernelInitResult> {
  console.log('[Kernel] 🔄 Initializing kernel on MAIN THREAD (fallback mode)');

  // 1. Load Go runtime
  const response = await fetch('/wasm_exec.js');
  const script = await response.text();
  const fn = new Function(script);
  fn.call(window);

  if (!window.Go) {
    throw new Error('Go runtime failed to load on main thread');
  }

  // 2. Create shared memory (or fallback to non-shared if unavailable)
  const config = MEMORY_PAGES[tier];
  let memory: WebAssembly.Memory;
  let isShared = true;

  try {
    memory = new WebAssembly.Memory({
      initial: config.initial,
      maximum: config.maximum,
      shared: true,
    });
  } catch {
    console.warn('[Kernel] Shared memory unavailable, using non-shared (limited functionality)');
    memory = new WebAssembly.Memory({
      initial: config.initial,
      maximum: config.maximum,
    });
    isShared = false;
  }

  // 3. Load and instantiate Go kernel (with Retry Logic)
  const go = new window.Go();
  let result: WebAssembly.WebAssemblyInstantiatedSource | undefined;

  const MAX_RETRIES = 2;
  let lastError: Error | null = null;

  for (let attempt = 0; attempt < MAX_RETRIES; attempt++) {
    try {
      const currentUrl =
        attempt > 0 ? `${wasmUrl}${wasmUrl.includes('?') ? '&' : '?'}retry=${Date.now()}` : wasmUrl;

      if (attempt > 0) {
        console.warn(`[Kernel] Retrying WASM fetch (attempt ${attempt + 1}/${MAX_RETRIES})...`);
      }

      let wasmResponse = await fetch(currentUrl);

      // If .br fails or is not found, try the uncompressed version (only on first attempt logic preserverd)
      if (!wasmResponse.ok && currentUrl.endsWith('.br')) {
        const fallbackUrl = currentUrl.replace('.wasm.br', '.wasm').split('?')[0];
        console.warn(
          `[Kernel] Failed to load compressed WASM from ${currentUrl}, trying fallback: ${fallbackUrl}`
        );
        wasmResponse = await fetch(fallbackUrl);
      }

      if (!wasmResponse.ok) {
        throw new Error(`HTTP ${wasmResponse.status} ${wasmResponse.statusText}`);
      }

      const contentType = wasmResponse.headers.get('Content-Type');
      if (contentType && contentType.includes('text/html')) {
        throw new Error('Received HTML instead of WASM (check server SPA fallback)');
      }

      // Clone for fallback
      const fallbackResponse = wasmResponse.clone();

      try {
        // Try streaming first
        result = await WebAssembly.instantiateStreaming(wasmResponse, {
          ...go.importObject,
          env: { ...go.importObject.env, memory },
        });
      } catch (streamingError) {
        console.warn(
          '[Kernel] instantiateStreaming failed, falling back to arrayBuffer:',
          streamingError
        );
        const bytes = await fallbackResponse.arrayBuffer();

        // Diagnostics
        const view = new Uint8Array(bytes);
        const hex = Array.from(view.slice(0, 16))
          .map(b => b.toString(16).padStart(2, '0'))
          .join(' ');

        const isWasm = view[0] === 0x00 && view[1] === 0x61 && view[2] === 0x73 && view[3] === 0x6d;

        if (!isWasm) {
          // Check for the specific Safari ghost bytes
          if (hex.startsWith('85 ff 1f')) {
            throw new Error(`MAGIC_MISMATCH_85FF1F: Received hex: ${hex}`);
          }

          const text = new TextDecoder().decode(view.slice(0, 50)).replace(/\0/g, '.');
          console.log(`[Kernel] WASM Diagnostics - First 16 bytes (hex): ${hex}`);
          console.log(`[Kernel] WASM Diagnostics - First 50 bytes (text): ${text}`);

          if (view[0] === 0x1f && view[1] === 0x8b) {
            throw new Error(
              'WASM is Gzip-compressed but the server is missing Content-Encoding: gzip'
            );
          }

          if (
            text.toLowerCase().includes('<!doctype html') ||
            text.toLowerCase().includes('<html')
          ) {
            throw new Error('Received HTML error page instead of WASM. Hex: ' + hex);
          }

          throw new Error(`WASM magic number mismatch ('\\0asm' expected). Received hex: ${hex}`);
        }

        result = await WebAssembly.instantiate(bytes, {
          ...go.importObject,
          env: { ...go.importObject.env, memory },
        });
      }

      // If we got here, success
      break;
    } catch (err) {
      lastError = err instanceof Error ? err : new Error(String(err));

      // Only retry on specific magic number mismatch or network errors (optional)
      const isMagicMismatch = lastError.message.includes('MAGIC_MISMATCH_85FF1F');

      if (isMagicMismatch && attempt < MAX_RETRIES - 1) {
        console.warn(
          '[Kernel] Detected corrupted cache (magic 85 ff 1f). Triggering retry with cache bust...'
        );
        continue;
      }

      // For other errors, or if retries exhausted, rethrow
      throw lastError;
    }
  }

  if (!result) {
    throw lastError || new Error('Failed to instantiate WASM after retries');
  }

  // 4. Run Go kernel (non-blocking - runs async via goroutines)
  go.run(result.instance);

  // 5. Wait for SAB functions
  const maxWaitMs = 5000;
  const startTime = Date.now();
  while (!window.getSystemSABAddress || !window.getSystemSABSize) {
    if (Date.now() - startTime > maxWaitMs) {
      console.warn('[Kernel] Timeout waiting for SAB functions on main thread');
      break;
    }
    await new Promise(r => setTimeout(r, 10));
  }

  // 6. Get SAB info
  const buffer = memory.buffer as unknown as SharedArrayBuffer;
  let sabOffset = 0;
  let sabSize = buffer.byteLength;

  if (window.getSystemSABAddress && window.getSystemSABSize) {
    const kAddr = window.getSystemSABAddress();
    const kSize = window.getSystemSABSize();
    if (kSize > 0) {
      sabOffset = kAddr;
      sabSize = kSize;
    }
  }

  // 7. Set globals and initialize bridge
  window.__INOS_SAB__ = buffer;
  window.__INOS_MEM__ = memory;
  window.__INOS_SAB_OFFSET__ = sabOffset;
  window.__INOS_SAB_SIZE__ = sabSize;
  window.__INOS_TIER__ = tier;

  if (isShared) {
    initializeBridge(buffer, sabOffset, sabSize, memory);
  }

  // Write Context ID Hash
  const contextHash = stringHash(contextId);
  try {
    const flags = INOSBridge.getFlagsView();
    if (flags) {
      flags[31] = contextHash;
    }
  } catch {
    console.warn('[Kernel] Could not write context hash (non-shared memory mode)');
  }

  console.log(`[Kernel] ✅ Main thread kernel initialized (shared: ${isShared})`);

  // Direct Economic local access (for main thread mode)
  if (!window.economics) {
    (window as any).economics = {
      getBalance: (did?: string) => (window as any).getEconomicBalance?.(did),
      getAccountInfo: (did?: string) => (window as any).getAccountInfo?.(did),
      getStats: () => (window as any).getEconomicStats?.(),
      grantBonus: (did: string, bonus: number) => (window as any).grantEconomicBonus?.(did, bonus),
    };
  }

  return {
    memory,
    sabBase: buffer,
    sabOffset,
    sabSize,
  };
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
