/**
 * INOS Kernel Worker
 *
 * Runs the Go WASM kernel in a dedicated Web Worker to prevent main thread blocking.
 * Communicates with main thread via SharedArrayBuffer (zero-copy) and postMessage (control).
 *
 * Architecture:
 * - Main thread creates SAB and sends reference to worker
 * - Worker loads Go WASM with SAB as linear memory
 * - Kernel runs supervisor loops in worker context
 * - Signaling via Atomics.notify/wait
 */

/// <reference lib="webworker" />

declare const self: DedicatedWorkerGlobalScope;
import { INOSBridge } from './bridge-state';
import {
  checkSharedMemoryCapability,
  fetchWasmWithFallback,
  instantiateWasm,
  loadGoRuntime,
} from './kernel.shared';

// Worker-scoped globals (no window.*)
let _sab: SharedArrayBuffer | null = null;
let _memory: WebAssembly.Memory | null = null;
let _go: any = null;

interface KernelWorkerMessage {
  type: 'init' | 'shutdown' | 'inject_sab' | 'kernel_call' | 'mesh_call';
  sab?: SharedArrayBuffer;
  sabOffset?: number;
  sabSize?: number;
  tier?: 'light' | 'moderate' | 'heavy' | 'dedicated';
  wasmUrl?: string;
  method?: string;
  args?: any[];
  requestId?: string;
}

interface KernelWorkerResponse {
  type:
    | 'ready'
    | 'error'
    | 'shutdown_complete'
    | 'sab_functions_ready'
    | 'kernel_response'
    | 'mesh_response';
  error?: string;
  sabOffset?: number;
  sabSize?: number;
  result?: any;
  requestId?: string;
}

// Memory page configurations (mirrored from layout.ts)
const MEMORY_PAGES: Record<string, { initial: number; maximum: number }> = {
  light: { initial: 512, maximum: 1024 }, // 32-64MB
  moderate: { initial: 1024, maximum: 2048 }, // 64-128MB
  heavy: { initial: 2048, maximum: 4096 }, // 128-256MB
  dedicated: { initial: 4096, maximum: 16384 }, // 256MB-1GB
};

/**
 * Initialize and run the Go kernel
 */
async function initializeKernel(
  tier: 'light' | 'moderate' | 'heavy' | 'dedicated',
  wasmUrl: string
): Promise<{ sabOffset: number; sabSize: number }> {
  // 0. Check shared memory capability FIRST (prevents iOS "body is distributed" error)
  const capability = checkSharedMemoryCapability();
  if (!capability.supported) {
    throw new Error(
      `INOS requires SharedArrayBuffer support.\n\n${capability.reason}\n\n` +
        'On iOS Safari, ensure the server sends:\n' +
        '  Cross-Origin-Opener-Policy: same-origin\n' +
        '  Cross-Origin-Embedder-Policy: require-corp'
    );
  }

  // 1. Load Go runtime
  await loadGoRuntime(self, '/wasm_exec.js', '[KernelWorker]');

  const config = MEMORY_PAGES[tier];

  // 2. Create shared memory
  _memory = new WebAssembly.Memory({
    initial: config.initial,
    maximum: config.maximum,
    shared: true,
  });

  // 3. Instantiate Go kernel
  _go = new (self as any).Go();

  let response: Response;
  try {
    response = await fetchWasmWithFallback(wasmUrl, '[KernelWorker]');
  } catch (err) {
    throw new Error(
      `Failed to fetch WASM from ${wasmUrl}: ${err instanceof Error ? err.message : String(err)}`
    );
  }

  let result: WebAssembly.WebAssemblyInstantiatedSource;

  try {
    result = await instantiateWasm(response, _go, _memory, '[KernelWorker]');
  } catch (err) {
    throw new Error(
      `Failed to instantiate WASM in worker: ${err instanceof Error ? err.message : String(err)}`
    );
  }

  // 4. Run Go kernel (this starts the Go runtime)
  _go.run(result.instance);

  // 5. Wait for SAB functions to be available
  const maxWaitMs = 5000;
  const startTime = Date.now();

  while (!(self as any).getSystemSABAddress || !(self as any).getSystemSABSize) {
    if (Date.now() - startTime > maxWaitMs) {
      console.warn('[KernelWorker] Timeout waiting for SAB functions');
      break;
    }
    await new Promise(r => setTimeout(r, 10));
  }

  // 6. Get SAB info from kernel
  // The memory.buffer IS a SharedArrayBuffer when created with shared: true
  const buffer = _memory.buffer as unknown as SharedArrayBuffer;
  _sab = buffer;

  let sabOffset = 0;
  let sabSize = buffer.byteLength;

  if ((self as any).getSystemSABAddress && (self as any).getSystemSABSize) {
    const kAddr = (self as any).getSystemSABAddress();
    const kSize = (self as any).getSystemSABSize();
    if (kSize > 0) {
      sabOffset = kAddr;
      sabSize = kSize;
    }
  }

  // Initialize centralized bridge for worker-local atomic access
  INOSBridge.initialize(buffer, sabOffset, sabSize, _memory);

  // Inject global view for Go's SABBridge (atomic signaling)
  (self as any).__INOS_SAB_INT32__ = INOSBridge.getFlagsView();

  return { sabOffset, sabSize };
}

/**
 * Inject SAB into Go kernel to start supervisor threads.
 */
async function injectSAB(sabOffset: number, sabSize: number): Promise<void> {
  const maxWaitMs = 5000;
  const startTime = Date.now();

  while (!(self as any).initializeSharedMemory) {
    if (Date.now() - startTime > maxWaitMs) {
      console.warn('[KernelWorker] Timeout waiting for initializeSharedMemory');
      return;
    }
    await new Promise(r => setTimeout(r, 10));
  }

  const injectStart = Date.now();
  while (true) {
    const result = (self as any).initializeSharedMemory(sabOffset, sabSize);
    if (!result?.error) {
      break;
    }
    if (typeof result.error === 'string' && result.error.includes('kernel not waiting')) {
      if (Date.now() - injectStart > maxWaitMs) {
        throw new Error(`Failed to inject SAB: ${result.error}`);
      }
      await new Promise(r => setTimeout(r, 25));
      continue;
    }
    throw new Error(`Failed to inject SAB: ${result.error}`);
  }
}

/**
 * Shutdown the kernel
 */
function shutdownKernel(): void {
  if (_sab) {
    const flags = new Int32Array(_sab, 0, 16);
    Atomics.store(flags, 0, 1); // Shutdown signal
    Atomics.notify(flags, 0);
  }
}

// =============================================================================
// MESSAGE HANDLER
// =============================================================================

self.onmessage = async (event: MessageEvent<KernelWorkerMessage>) => {
  const { type } = event.data;
  console.log(`[KernelWorker] Received message: type=${type}`);

  try {
    switch (type) {
      case 'init': {
        const tier = event.data.tier || 'light';
        const wasmUrl = event.data.wasmUrl || '/kernel.wasm';

        console.log(`[KernelWorker] Initializing kernel (tier: ${tier})`);

        const { sabOffset, sabSize } = await initializeKernel(tier, wasmUrl);

        // Send back the SAB for main thread and other workers
        const response = {
          type: 'sab_functions_ready' as const,
          sabOffset,
          sabSize,
          sab: _sab as SharedArrayBuffer,
          memory: _memory,
        };

        // Transfer the SAB reference (not ownership - it's shared)
        if (!_sab) throw new Error('SAB not initialized in kernel worker');
        self.postMessage(response);

        // Now inject SAB to start supervisors
        console.log('[KernelWorker] Injecting SAB...');
        await injectSAB(sabOffset, sabSize);
        console.log('[KernelWorker] SAB injection complete, signaling ready');

        const readyResponse: KernelWorkerResponse = { type: 'ready' };
        self.postMessage(readyResponse);
        break;
      }

      case 'shutdown': {
        shutdownKernel();
        const response: KernelWorkerResponse = { type: 'shutdown_complete' };
        self.postMessage(response);
        break;
      }

      case 'kernel_call': {
        const { method, args, requestId } = event.data;
        const kernel = (self as any).kernel;
        if (!kernel || !kernel[method!]) {
          self.postMessage({
            type: 'error',
            error: `Kernel method ${method} not available`,
            requestId,
          });
          return;
        }
        try {
          const result = kernel[method!](...(args || []));
          self.postMessage({ type: 'kernel_response', result, requestId });
        } catch (err) {
          self.postMessage({
            type: 'error',
            error: err instanceof Error ? err.message : String(err),
            requestId,
          });
        }
        break;
      }

      case 'mesh_call': {
        const { method, args, requestId } = event.data;
        const mesh = (self as any).mesh;
        if (!mesh || !mesh[method!]) {
          self.postMessage({
            type: 'error',
            error: `Mesh method ${method} not available`,
            requestId,
          });
          return;
        }
        try {
          const result = mesh[method!](...(args || []));
          self.postMessage({ type: 'mesh_response', result, requestId });
        } catch (err) {
          self.postMessage({
            type: 'error',
            error: err instanceof Error ? err.message : String(err),
            requestId,
          });
        }
        break;
      }

      default:
        console.warn(`[KernelWorker] Unknown message type: ${type}`);
    }
  } catch (error) {
    const response: KernelWorkerResponse = {
      type: 'error',
      error: error instanceof Error ? error.message : String(error),
    };
    self.postMessage(response);
  }
};

// Export for TypeScript module resolution
export {};
