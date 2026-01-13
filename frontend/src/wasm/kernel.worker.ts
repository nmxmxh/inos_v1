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

// Worker-scoped globals (no window.*)
let _sab: SharedArrayBuffer | null = null;
let _memory: WebAssembly.Memory | null = null;
let _go: any = null;

interface KernelWorkerMessage {
  type: 'init' | 'shutdown' | 'inject_sab';
  sab?: SharedArrayBuffer;
  sabOffset?: number;
  sabSize?: number;
  tier?: 'light' | 'moderate' | 'heavy' | 'dedicated';
  wasmUrl?: string;
}

interface KernelWorkerResponse {
  type: 'ready' | 'error' | 'shutdown_complete' | 'sab_functions_ready';
  error?: string;
  sabOffset?: number;
  sabSize?: number;
}

// Memory page configurations (mirrored from layout.ts)
const MEMORY_PAGES: Record<string, { initial: number; maximum: number }> = {
  light: { initial: 512, maximum: 1024 }, // 32-64MB
  moderate: { initial: 1024, maximum: 2048 }, // 64-128MB
  heavy: { initial: 2048, maximum: 4096 }, // 128-256MB
  dedicated: { initial: 4096, maximum: 16384 }, // 256MB-1GB
};

/**
 * Load Go runtime (wasm_exec.js) in worker context
 */
async function loadGoRuntime(): Promise<void> {
  // In worker context, we import the script differently
  // Vite will bundle this, or we fetch and eval
  const response = await fetch('/wasm_exec.js');
  const script = await response.text();

  // Execute in worker global scope
  const fn = new Function(script);
  fn.call(self);

  if (!(self as any).Go) {
    throw new Error('Go runtime failed to load in worker');
  }
}

/**
 * Check if SharedArrayBuffer and shared WebAssembly.Memory are available.
 * iOS Safari and some contexts lack support for these features.
 */
function checkSharedMemoryCapability(): { supported: boolean; reason?: string } {
  // 1. Check if SharedArrayBuffer exists
  if (typeof SharedArrayBuffer === 'undefined') {
    return {
      supported: false,
      reason:
        'SharedArrayBuffer is not available. This may be due to missing COOP/COEP headers or an unsupported browser.',
    };
  }

  // 2. Check if we can create shared WebAssembly.Memory
  try {
    const testMemory = new WebAssembly.Memory({
      initial: 1,
      maximum: 1,
      shared: true,
    });
    // Verify buffer is actually a SharedArrayBuffer
    if (!(testMemory.buffer instanceof SharedArrayBuffer)) {
      return {
        supported: false,
        reason: 'WebAssembly.Memory does not produce SharedArrayBuffer. Check COOP/COEP headers.',
      };
    }
  } catch (e) {
    return {
      supported: false,
      reason: `Shared WebAssembly.Memory is not supported: ${e instanceof Error ? e.message : String(e)}`,
    };
  }

  return { supported: true };
}

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
  await loadGoRuntime();

  const config = MEMORY_PAGES[tier];

  // 2. Create shared memory
  _memory = new WebAssembly.Memory({
    initial: config.initial,
    maximum: config.maximum,
    shared: true,
  });

  // 3. Instantiate Go kernel
  _go = new (self as any).Go();

  const response = await fetch(wasmUrl);
  let result: WebAssembly.WebAssemblyInstantiatedSource;

  try {
    const fallbackResponse = response.clone();
    try {
      result = await WebAssembly.instantiateStreaming(response, {
        ..._go.importObject,
        env: { ..._go.importObject.env, memory: _memory },
      });
    } catch (streamingError) {
      console.warn(
        '[KernelWorker] instantiateStreaming failed, falling back to arrayBuffer:',
        streamingError
      );
      const bytes = await fallbackResponse.arrayBuffer();
      result = await WebAssembly.instantiate(bytes, {
        ..._go.importObject,
        env: { ..._go.importObject.env, memory: _memory },
      });
    }
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
 * Inject SAB into Go kernel to start supervisor threads
 */
async function injectSAB(sabOffset: number, sabSize: number): Promise<void> {
  // Wait for initializeSharedMemory to be available
  const maxWaitMs = 5000;
  const startTime = Date.now();

  while (!(self as any).initializeSharedMemory) {
    if (Date.now() - startTime > maxWaitMs) {
      console.warn('[KernelWorker] Timeout waiting for initializeSharedMemory');
      return;
    }
    await new Promise(r => setTimeout(r, 10));
  }

  const result = (self as any).initializeSharedMemory(sabOffset, sabSize);
  if (result?.error) {
    throw new Error(`Failed to inject SAB: ${result.error}`);
  }

  console.log('[KernelWorker] ✅ Supervisor threads started');
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
        await injectSAB(sabOffset, sabSize);

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
