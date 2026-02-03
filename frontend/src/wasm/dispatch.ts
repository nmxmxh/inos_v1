import { getSAB, getMemory } from './bridge-state';
import { IDX_BIRD_EPOCH, IDX_MATRIX_EPOCH, IDX_REGISTRY_EPOCH } from './layout';

// Vite worker import syntax
import ComputeWorkerUrl from './compute.worker.ts?worker&url';

export interface DispatchResult<T = any> {
  success: boolean;
  data: T;
  error?: string;
}

export interface WorkerRef {
  worker: Worker;
  unit: string;
  role: string;
  ready: boolean;
}

export class Dispatcher {
  private exports: any;
  private memory: WebAssembly.Memory | null = null;
  private capabilities: Map<string, string[]> = new Map();
  private static encoder = new TextEncoder();
  private static decoder = new TextDecoder();

  // Worker Pool
  private workers = new Map<string, WorkerRef>();
  private pendingRequests = new Map<number, { resolve: Function; reject: Function }>();
  private messageIdCounter = 0;

  constructor(exports?: any, memory?: WebAssembly.Memory) {
    this.exports = exports;
    this.memory = memory || null;
  }

  /**
   * Register or update capabilities for a unit
   */
  registerUnit(unit: string, methods: string[]) {
    const existing = this.capabilities.get(unit);
    if (
      !existing ||
      existing.length !== methods.length ||
      !existing.every((m, i) => m === methods[i])
    ) {
      this.capabilities.set(unit, methods);
      console.log(`[Dispatch] Registered unit '${unit}' with ${methods.length} methods`);
    }
  }

  /**
   * Check if a capability/method exists
   */
  hasCapability(unit: string, method?: string): boolean {
    const methods = this.capabilities.get(unit);
    if (methods && (!method || methods.includes(method))) return true;

    // Cross-module discovery: check if this unit:method is provided by generic module
    if (method) {
      const fullCap = `${unit}:${method}`;
      for (const m of this.capabilities.values()) {
        if (m.includes(fullCap)) return true;
      }
    } else {
      // Namespace-only lookup: check if ANY capability starts with "unit:"
      const prefix = `${unit}:`;
      for (const m of this.capabilities.values()) {
        if (m.some(cap => cap.startsWith(prefix))) return true;
      }
    }

    return false;
  }

  /**
   * Wait for a specific capability to become available via registry epochs.
   * This uses zero-CPU signaling (Atomics.waitAsync) to avoid polling.
   */
  async waitForCapability(
    unit: string,
    method?: string,
    timeoutMs: number = 10000
  ): Promise<boolean> {
    if (this.hasCapability(unit, method)) return true;

    const sab = getSAB() || (window as any).__INOS_SAB__;
    if (!sab) {
      console.warn(
        '[Dispatch] waitForCapability: SAB not available yet, falling back to brief sleep'
      );
      await new Promise(resolve => setTimeout(resolve, 100));
      return this.hasCapability(unit, method);
    }

    const flags = new Int32Array(sab, 0, 32);
    const start = Date.now();

    while (Date.now() - start < timeoutMs) {
      if (this.hasCapability(unit, method)) return true;

      // Small initial yield to allow registry reader to finish first pass
      await new Promise(resolve => setTimeout(resolve, 50));
      if (this.hasCapability(unit, method)) return true;

      const currentEpoch = Atomics.load(flags, IDX_REGISTRY_EPOCH);

      // Using Atomics.waitAsync (Stage 3/4 JS, supported in modern Chrome/Safari/Firefox)
      // This allows the main thread to "wait" without blocking.
      if (typeof (Atomics as any).waitAsync === 'function') {
        const result = (Atomics as any).waitAsync(
          flags,
          IDX_REGISTRY_EPOCH,
          currentEpoch,
          timeoutMs - (Date.now() - start)
        );
        await result.value;
      } else {
        // Fallback for older browsers (standard promise sleep)
        await new Promise(resolve => setTimeout(resolve, 50));
      }

      // After a signal, the registry epoch changed. The SystemStore should have updated us.
      // Small yield to allow SystemStore's scan to complete if it's on the same thread.
      await new Promise(resolve => setTimeout(resolve, 10));
    }

    const success = this.hasCapability(unit, method);
    if (!success) {
      console.warn(
        `[Dispatch] Timeout waiting for capability: ${unit}${method ? ':' + method : ''}`
      );
    }
    return success;
  }

  /**
   * Plug a unit into a dedicated background worker for maximum performance
   * Supports 'parallel: n' to spawn a pool for multi-core processing.
   */
  async plug(unit: string, role: string, params: any = {}): Promise<WorkerRef[]> {
    const memory = getMemory();
    const sab = getSAB();

    if (!sab) {
      // Fallback to global if bridge-state hasn't synchronized yet
      const globalSab = (window as any).__INOS_SAB__;
      if (!globalSab) {
        throw new Error('[Dispatch] Cannot plug unit: SAB or Memory not initialized');
      }
    }

    const parallel = params.parallel || 1;
    const workerRefs: WorkerRef[] = [];

    for (let i = 0; i < parallel; i++) {
      const workerId = parallel > 1 ? `${unit}:${role}:${i}` : `${unit}:${role}`;

      // Intelligent Reuse: Check if we already have a worker that provides this unit/role
      // Or if the unit 'compute' already provides the capability and is available for this role.
      let workerRef = this.getWorkerForRole(unit, role);

      if (workerRef) {
        workerRefs.push(workerRef);
        // Force parameter update if role params changed
        if (workerRef.ready) {
          workerRef.worker.postMessage({
            type: 'start_role_loop',
            role,
            params: { ...params, index: i, parallel },
          });
        }
        // If this was the first worker in a parallel set, we still need to "resolve"
        // the conceptual wait for it to move the loop along.
        continue;
      }

      console.log(`[Dispatch] Spawning worker ${i + 1}/${parallel} for ${unit} (role: ${role})`);
      const worker = new Worker(ComputeWorkerUrl, { type: 'module' });
      const newWorkerRef: WorkerRef = { worker, unit, role, ready: false };
      this.workers.set(workerId, newWorkerRef);
      workerRefs.push(newWorkerRef);

      // Initialize each worker
      const initPromise = new Promise<void>((resolve, reject) => {
        worker.onmessage = event => {
          const { type, id, result, error } = event.data;

          switch (type) {
            case 'ready':
              newWorkerRef.ready = true;
              // Transition to the requested role with partition info
              worker.postMessage({
                type: 'start_role_loop',
                role,
                params: { ...params, index: i, parallel },
              });
              resolve();
              break;

            case 'result': {
              const pending = this.pendingRequests.get(id);
              if (pending) {
                this.pendingRequests.delete(id);
                if (error) pending.reject(new Error(error));
                else pending.resolve(result);
              }
              break;
            }

            case 'error':
              console.error(`[Dispatch:Worker:${workerId}] Error:`, error);
              if (id !== undefined) {
                const pending = this.pendingRequests.get(id);
                if (pending) {
                  this.pendingRequests.delete(id);
                  pending.reject(new Error(error));
                }
              }
              break;
          }
        };

        worker.onerror = err => {
          console.error(`[Dispatch:Worker:${workerId}] Fatal error:`, err);
          reject(err);
        };
      });

      worker.postMessage({
        type: 'init',
        sab: sab || (window as any).__INOS_SAB__,
        memory: memory,
        sabOffset: (sab ? 0 : (window as any).__INOS_SAB_OFFSET__) || 0,
        sabSize: (sab || (window as any).__INOS_SAB__)?.byteLength || 0,
        role,
      });

      // Wait for the worker to be ready before moving to the next one in the pool
      // This ensures we don't return from plug until at least the first worker is operational.
      if (i === 0 || !workerRefs[0].ready) {
        await initPromise;
      }
    }

    return workerRefs;
  }

  /**
   * Execute a compute operation.
   * Automatically routes to a background worker if one is plugged for this unit.
   */
  async execute(
    library: string,
    method: string,
    params: object = {},
    input: Uint8Array | null = null,
    forceSync: boolean = false
  ): Promise<Uint8Array | null> {
    // Priority:
    // 1. Worker specifically assigned this role (unit-mode)
    // 2. Generic worker providing the capability (capability-mode)
    let workerRef = this.getWorkerForRole(library, method.split('_')[0] || ''); // Try to infer role

    if (!workerRef) {
      workerRef = Array.from(this.workers.values()).find(
        w => w.unit === library || this.hasCapability(w.unit, `${library}:${method}`)
      );
    }

    if (workerRef && workerRef.ready && !forceSync) {
      return this.executeOnWorker(workerRef.worker, library, method, params);
    }

    // Split Memory Mode Protection:
    // If we have no local exports, we CANNOT fall back to executeSync.
    // We must wait for a worker or throw a specific error.
    if (!this.exports) {
      console.warn(
        `[Dispatch] '${library}:${method}' requested but no worker ready and no local exports.`
      );

      // Try for 2 seconds to see if a worker pops up (e.g. during boot)
      const start = Date.now();
      while (Date.now() - start < 2000) {
        await new Promise(resolve => setTimeout(resolve, 100));
        const retryWorker = Array.from(this.workers.values()).find(
          w => w.unit === library || this.hasCapability(w.unit, `${library}:${method}`)
        );
        if (retryWorker && retryWorker.ready) {
          return this.executeOnWorker(retryWorker.worker, library, method, params);
        }
      }

      throw new Error(
        `[Dispatch] Cannot execute '${library}:${method}': No workers ready and local execution unavailable.`
      );
    }

    // Fallback to local synchronous execution
    return this.executeSync(library, method, params, input);
  }

  private executeOnWorker(
    worker: Worker,
    library: string,
    method: string,
    params: object
  ): Promise<Uint8Array | null> {
    return new Promise((resolve, reject) => {
      const id = this.messageIdCounter++;
      this.pendingRequests.set(id, { resolve, reject });
      worker.postMessage({
        type: 'execute',
        id,
        library,
        method,
        params,
      });
    });
  }

  private static stringCache = new Map<string, Uint8Array>();
  private static stringCacheKeys: string[] = [];
  private static readonly MAX_STRING_CACHE = 100;

  private static getEncoded(str: string): Uint8Array {
    let cached = this.stringCache.get(str);
    if (!cached) {
      cached = this.encoder.encode(str);
      if (this.stringCacheKeys.length >= this.MAX_STRING_CACHE) {
        const oldest = this.stringCacheKeys.shift()!;
        this.stringCache.delete(oldest);
      }
      this.stringCache.set(str, cached);
      this.stringCacheKeys.push(str);
    } else {
      const idx = this.stringCacheKeys.indexOf(str);
      if (idx > -1) {
        this.stringCacheKeys.splice(idx, 1);
        this.stringCacheKeys.push(str);
      }
    }
    return cached;
  }

  /**
   * Execute a compute operation synchronously on the current thread
   */
  executeSync(
    library: string,
    method: string,
    params: object = {},
    input: Uint8Array | null = null
  ): Uint8Array | null {
    if (!this.exports || !this.exports.compute_execute) {
      if (!this.exports)
        throw new Error('[Dispatch] No local exports available. Initialize first.');
      throw new Error('[Dispatch] compute_execute export missing');
    }

    const encoder = Dispatcher.encoder;
    const libBytes = Dispatcher.getEncoded(library);
    const methodBytes = Dispatcher.getEncoded(method);
    const paramsBytes = encoder.encode(JSON.stringify(params));

    const libPtr = this.exports.compute_alloc(libBytes.length);
    const methodPtr = this.exports.compute_alloc(methodBytes.length);
    const paramsPtr = this.exports.compute_alloc(paramsBytes.length);
    let inputPtr = 0;

    if (input) {
      inputPtr = this.exports.compute_alloc(input.length);
    }

    const heap = new Uint8Array(this.memory!.buffer);
    heap.set(libBytes, libPtr);
    heap.set(methodBytes, methodPtr);
    heap.set(paramsBytes, paramsPtr);
    if (input && inputPtr) {
      heap.set(input, inputPtr);
    }

    try {
      const resultPtr = this.exports.compute_execute(
        libPtr,
        libBytes.length,
        methodPtr,
        methodBytes.length,
        inputPtr,
        input ? input.length : 0,
        paramsPtr,
        paramsBytes.length
      );

      console.log(`[Dispatch] executeSync '${library}:${method}' returned ptr: ${resultPtr}`);

      if (resultPtr === 0) return null;

      if (!this.memory) {
        console.error('[DISPATCH-CRITICAL] Memory reference lost! Attempting recovery...');
        this.memory = (window as any).__INOS_MEM__;
        if (!this.memory) throw new Error('[DISPATCH-CRITICAL] Fatal: Compute memory unavailable');
      }

      const resultView = new DataView(this.memory!.buffer);
      const outputLen = resultView.getUint32(resultPtr, true);
      console.log(`[Dispatch] Output len: ${outputLen}`);

      const output = new Uint8Array(this.memory!.buffer, resultPtr + 4, outputLen);
      const finalResult = new Uint8Array(output);

      // --- ARCHITECTURE ALIGNMENT: Direct Feedback for Safari Unified Mode ---
      // If we are on the main thread, notify the Go kernel immediately of the epoch change.
      // This eliminates the 50ms polling latency.
      if (typeof (window as any).notifyEpochChange === 'function') {
        try {
          // Heuristic: map library/method to epoch index
          let index = -1;
          if (library === 'boids') {
            if (method === 'step_physics') index = IDX_BIRD_EPOCH;
            else if (method === 'generate_matrices') index = IDX_MATRIX_EPOCH;
          }

          if (index !== -1) {
            // Parse result to get the new epoch value if possible (WASM returns JSON { epoch: N })
            const str = Dispatcher.decoder.decode(finalResult);
            const json = JSON.parse(str);
            if (json && typeof json.epoch === 'number') {
              (window as any).notifyEpochChange(index, json.epoch);
            }
          }
        } catch (e) {
          // Silent fail for notification - don't break the main loop
          console.warn('[Dispatch] notifyEpochChange error:', e);
        }
      }

      if (this.exports.compute_free) {
        console.log(`[Dispatch] Freeing result ptr: ${resultPtr}`);
        this.exports.compute_free(resultPtr, 4 + outputLen);
      } else {
        console.warn('[Dispatch] compute_free missing!');
      }

      return finalResult;
    } finally {
      if (this.exports.compute_free) {
        this.exports.compute_free(libPtr, libBytes.length);
        this.exports.compute_free(methodPtr, methodBytes.length);
        this.exports.compute_free(paramsPtr, paramsBytes.length);
        if (input && inputPtr) {
          this.exports.compute_free(inputPtr, input.length);
        }
      }
    }
  }

  /**
   * Execute and parse JSON result. Async to support worker routing.
   */
  async json<T = any>(library: string, method: string, params: object = {}): Promise<T | null> {
    const res = await this.execute(library, method, params);
    if (!res) return null;
    try {
      const str = Dispatcher.decoder.decode(res);
      return JSON.parse(str) as T;
    } catch (e) {
      console.error('[Dispatch] JSON Parse error:', e);
      return null;
    }
  }

  /**
   * Shutdown all workers
   */
  shutdown() {
    for (const ref of this.workers.values()) {
      ref.worker.postMessage({ type: 'shutdown' });
      ref.worker.terminate();
    }
    this.workers.clear();
  }

  initialize(exports: any, memory: WebAssembly.Memory) {
    this.exports = exports;
    this.memory = memory;
    console.log('[Dispatch] Instance setup complete (memory attached)');
  }

  private getWorkerForRole(unit: string, role: string): WorkerRef | undefined {
    return Array.from(this.workers.values()).find(
      w =>
        (w.unit === unit && w.role === role) ||
        (this.hasCapability(w.unit, unit) && w.role === role)
    );
  }

  /**
   * Binds an existing worker to the dispatcher's message handling logic.
   * Useful for workers spawned outside of plug (e.g. system boot worker).
   */
  bindWorker(id: string, ref: Omit<WorkerRef, 'ready'> & { ready: boolean }) {
    const workerRef = { ...ref };
    this.workers.set(id, workerRef);

    workerRef.worker.onmessage = event => {
      const { type, id: msgId, result, error } = event.data;
      if (type === 'result') {
        const pending = this.pendingRequests.get(msgId);
        if (pending) {
          this.pendingRequests.delete(msgId);
          if (error) pending.reject(new Error(error));
          else pending.resolve(result);
        }
      } else if (type === 'error') {
        console.error(`[Dispatch:Worker:${id}] Error:`, error);
        if (msgId !== undefined) {
          const pending = this.pendingRequests.get(msgId);
          if (pending) {
            this.pendingRequests.delete(msgId);
            pending.reject(new Error(error));
          }
        }
      }
    };

    workerRef.worker.onerror = err => {
      console.error(`[Dispatch:Worker:${id}] Fatal error:`, err);
    };

    return workerRef;
  }
}

// Global dispatcher instance
let instance = new Dispatcher();

export const dispatch = {
  internal: () => instance,

  initialize: (exports: any, memory: WebAssembly.Memory) => {
    instance.initialize(exports, memory);
    return instance;
  },

  register: (unit: string, methods: string[]) => {
    instance.registerUnit(unit, methods);
  },

  has: (unit: string, method?: string) => instance.hasCapability(unit, method) || false,

  waitUntilReady: (unit: string, method?: string, timeout?: number) =>
    instance.waitForCapability(unit, method, timeout),

  /**
   * Dedicated background unit registration
   */
  plug: (unit: string, role: string, params: object = {}) => {
    return instance.plug(unit, role, params);
  },

  execute: (
    library: string,
    method: string,
    params: object = {},
    input: Uint8Array | null = null,
    forceSync = false
  ) => {
    return instance.execute(library, method, params, input, forceSync);
  },

  json: <T = any>(library: string, method: string, params: object = {}) => {
    return instance.json<T>(library, method, params);
  },

  bind: (id: string, ref: any) => instance.bindWorker(id, ref),

  shutdown: () => instance.shutdown(),
};

export default dispatch;
