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

console.log('[KernelWorker] 🚀 SCRIPT LOADED - Top Level Executing');

self.addEventListener('error', (e: ErrorEvent) => {
  console.error('[KernelWorker] 💥 UNCAUGHT ERROR:', e.message, e.error);
});

declare const self: DedicatedWorkerGlobalScope;
import { INOSBridge } from './bridge-state';
import {
  checkSharedMemoryCapability,
  exposeBrowserApis,
  registerHostCall,
  fetchWasmWithFallback,
  instantiateWasm,
  loadGoRuntime,
} from './kernel.shared';
import {
  IDX_BIRD_EPOCH,
  IDX_EVOLUTION_EPOCH,
  IDX_REGISTRY_EPOCH,
  IDX_SYSTEM_EPOCH,
  IDX_INBOX_DIRTY,
  IDX_OUTBOX_KERNEL_DIRTY,
  IDX_ECONOMY_EPOCH,
} from './layout';

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
  meshConfig?: any;
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
  wasmUrl: string,
  meshConfig?: any,
  injectedSab?: SharedArrayBuffer
): Promise<{ sabOffset: number; sabSize: number }> {
  if (meshConfig) {
    (self as any).__INOS_MESH_CONFIG__ = meshConfig;
    if (meshConfig.identity) {
      (self as any).__INOS_IDENTITY__ = meshConfig.identity;
      if (typeof meshConfig.identity.nodeId === 'string') {
        (self as any).__INOS_NODE_ID__ = meshConfig.identity.nodeId;
      }
      if (typeof meshConfig.identity.deviceId === 'string') {
        (self as any).__INOS_DEVICE_ID__ = meshConfig.identity.deviceId;
      }
      if (typeof meshConfig.identity.did === 'string') {
        (self as any).__INOS_DID__ = meshConfig.identity.did;
      }
    }
  }
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

  exposeBrowserApis(self, '[KernelWorker]');
  registerHostCall(self, '[KernelWorker]');

  const config = MEMORY_PAGES[tier];

  // 2. Setup Memory (Either Injected or Created)
  if (injectedSab) {
    console.log('[KernelWorker] Using injected SharedArrayBuffer (Single Source of Truth)');
    _sab = injectedSab;
    _memory = null; // Go uses private memory in split architecture
  } else {
    console.warn('[KernelWorker] Creating NEW SharedArrayBuffer (Split Brain Risk!)');
    _memory = new WebAssembly.Memory({
      initial: config.initial,
      maximum: config.maximum,
      shared: true,
    });
    _sab = _memory.buffer as unknown as SharedArrayBuffer;
  }

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
    // FIX: Pass undefined for memory to ensure Go uses its own private memory (Split Memory Architecture)
    // This prevents Go's runtime (Stack/Heap) from overwriting the SAB at offset 0.
    result = await instantiateWasm(response, _go, undefined, '[KernelWorker]');
  } catch (err) {
    throw new Error(
      `Failed to instantiate WASM in worker: ${err instanceof Error ? err.message : String(err)}`
    );
  }

  // 4. Pre-initialize __INOS_SAB_INT32__ BEFORE Go runs (critical for Go's SABBridge)
  const buffer = _sab!;

  // Expose SAB to Go Bridge (Pre-initialization)
  (self as any).__INOS_SAB__ = buffer;
  (self as any).__INOS_SAB_SIZE__ = buffer.byteLength;
  (self as any).__INOS_SAB_INT32__ = new Int32Array(buffer, 0, buffer.byteLength / 4);

  // 5. Run Go kernel (this starts the Go runtime)
  // This must happen AFTER __INOS_SAB_INT32__ is set for Go's SABBridge to work
  _go.run(result.instance);
  console.log('[KernelWorker] Go runtime started in background');

  // 6. Wait for SAB functions to be available
  const maxWaitMs = 5000;
  const startTime = Date.now();

  while (
    !(self as any).getSystemSABAddress ||
    !(self as any).getSystemSABSize ||
    !(self as any).jsGetKernelStats ||
    !(self as any).jsSubmitJob ||
    !(self as any).jsDeserializeResult ||
    !(self as any).jsDelegateJob
  ) {
    if (Date.now() - startTime > maxWaitMs) {
      console.warn('[KernelWorker] Timeout waiting for Go WASM exports');
      break;
    }
    await new Promise(r => setTimeout(r, 10));
  }

  // 7. Get SAB info (Authoritative Schema: 0-based)
  const sabOffset = 0;
  const sabSize = buffer.byteLength;

  (self as any).__INOS_SAB_OFFSET__ = sabOffset;
  (self as any).__INOS_SAB_SIZE__ = sabSize;
  (self as any).__INOS_SAB_INT32__ = new Int32Array(
    buffer,
    sabOffset,
    (buffer.byteLength - sabOffset) / 4
  );

  // 8. Initialize centralized bridge for worker-local atomic access
  INOSBridge.initialize(buffer, sabOffset, sabSize, _memory as WebAssembly.Memory);
  startEpochWatchers(buffer, sabOffset);

  // 9. Expose Go exports as mesh/kernel APIs for Worker proxy
  const global = self as any;
  if (!global.mesh) global.mesh = {};
  if (!global.kernel) global.kernel = {};

  // Map/Alias key functions for Mesh
  global.mesh.delegateJob = global.mesh.delegateJob || global.jsDelegateJob;
  global.mesh.delegateCompute = global.mesh.delegateCompute || global.mesh.delegateJob;
  global.mesh.subscribeToEvents = global.mesh.subscribeToEvents || global.subscribeToEvents;
  global.mesh.unsubscribeFromEvents =
    global.mesh.unsubscribeFromEvents || global.unsubscribeFromEvents;
  global.mesh.connectToPeer = global.mesh.connectToPeer || global.jsMeshConnectToPeer;

  // Map/Alias key functions for Kernel
  global.kernel.submitJob = global.kernel.submitJob || global.jsSubmitJob;
  global.kernel.deserializeResult = global.kernel.deserializeResult || global.jsDeserializeResult;
  global.kernel.getStats =
    global.kernel.getStats || global.jsGetKernelStats || global.getKernelStats;

  if (!(self as any).jsDelegateJob) {
    console.warn('[KernelWorker] ⚠️ jsDelegateJob not found in global scope - Go exports missing?');
  }

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

let epochWatchersRunning = true;

/**
 * Shutdown the kernel
 */
function shutdownKernel(): void {
  stopEpochWatchers();
  if (_sab) {
    const flags = new Int32Array(_sab, 0, 16);
    Atomics.store(flags, 0, 1); // Shutdown signal
    Atomics.notify(flags, 0);
  }
}

/**
 * Stop epoch watchers
 */
function stopEpochWatchers(): void {
  epochWatchersRunning = false;
}

/**
 * Check if Atomics.waitAsync is available (Safari 16.4+, Chrome 87+)
 */
const hasWaitAsync =
  typeof Atomics !== 'undefined' && typeof (Atomics as any).waitAsync === 'function';

/**
 * Start epoch watchers that notify Go kernel when epochs change.
 * Uses Atomics.waitAsync when available, falls back to polling otherwise.
 */
function startEpochWatchers(sab: SharedArrayBuffer, sabOffset: number): void {
  const indices = [
    IDX_SYSTEM_EPOCH,
    IDX_BIRD_EPOCH,
    IDX_EVOLUTION_EPOCH,
    IDX_REGISTRY_EPOCH,
    IDX_INBOX_DIRTY,
    IDX_OUTBOX_KERNEL_DIRTY,
    IDX_ECONOMY_EPOCH,
  ];

  const flags = new Int32Array(sab, sabOffset, 128);
  console.log('[KernelWorker] Starting epoch watchers (hasWaitAsync:', hasWaitAsync, ')');

  indices.forEach(index => {
    watchEpochIndex(flags, index);
  });
}

/**
 * Watch a single epoch index and notify Go when it changes.
 * Uses Atomics.waitAsync for zero-CPU idling when available.
 */
function watchEpochIndex(flags: Int32Array, index: number): void {
  if (!epochWatchersRunning) return;

  const current = Atomics.load(flags, index);

  if (hasWaitAsync) {
    // Modern path: non-blocking async wait
    const result = (Atomics as any).waitAsync(flags, index, current);
    if (result.async) {
      result.value.then(() => {
        if (!epochWatchersRunning) return;
        const newValue = Atomics.load(flags, index);
        notifyGoEpochChange(index, newValue);
        watchEpochIndex(flags, index); // Re-arm
      });
    } else {
      // Value already changed
      const newValue = Atomics.load(flags, index);
      notifyGoEpochChange(index, newValue);
      setTimeout(() => watchEpochIndex(flags, index), 0);
    }
  } else {
    // Fallback: poll at 60Hz for older browsers
    const pollInterval = 16;
    const poll = () => {
      if (!epochWatchersRunning) return;
      const newValue = Atomics.load(flags, index);
      if (newValue !== current) {
        notifyGoEpochChange(index, newValue);
        watchEpochIndex(flags, index); // Re-arm with new value
      } else {
        setTimeout(poll, pollInterval);
      }
    };
    setTimeout(poll, pollInterval);
  }
}

/**
 * Notify Go kernel of epoch change
 */
function notifyGoEpochChange(index: number, value: number): void {
  const notify = (self as any).notifyEpochChange;
  if (typeof notify === 'function') {
    notify(index, value);
  }
}

// =============================================================================
// MESSAGE HANDLER
// =============================================================================

console.log('[KernelWorker] 🛠️ Setting up MESSAGE HANDLER...');
self.onmessage = async (event: MessageEvent<KernelWorkerMessage>) => {
  // console.log('[KernelWorker] 📨 RAW MESSAGE RECEIVED', event.data);
  const { type, method, args, requestId } = event.data;
  // console.log(`[KernelWorker] 📨 Received message: type=${type} method=${method || 'N/A'}`, { ... });

  try {
    switch (type as any) {
      case 'ping':
        console.log('[KernelWorker] 🏓 PONG! Communication channel is open.');
        return;
      case 'init': {
        const tier = event.data.tier || 'light';
        const wasmUrl = event.data.wasmUrl || '/kernel.wasm';

        console.log(`[KernelWorker] Initializing kernel (tier: ${tier})`);

        const { sabOffset, sabSize } = await initializeKernel(
          tier,
          wasmUrl,
          event.data.meshConfig,
          event.data.sab // Pass injected SAB
        );

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

      case 'webrtc_event':
      case 'webrtc_datachannel_created':
      case 'webrtc_datachannel_event':
      case 'webrtc_response':
        // Handled by WebRTCProxy in kernel.shared.ts
        break;

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
