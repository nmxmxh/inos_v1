import { getSAB, getMemory, getOffset } from './bridge-state';
import { IDX_BIRD_EPOCH, IDX_MATRIX_EPOCH } from './layout';

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
    if (!methods) return false;
    if (method && !methods.includes(method)) return false;
    return true;
  }

  /**
   * Plug a unit into a dedicated background worker for maximum performance
   * Supports 'parallel: n' to spawn a pool for multi-core processing.
   */
  async plug(unit: string, role: string, params: any = {}): Promise<WorkerRef[]> {
    const sab = getSAB();
    const memory = getMemory();
    const offset = getOffset();

    if (!sab || !memory) {
      throw new Error('[Dispatch] Cannot plug unit: SAB or Memory not initialized');
    }

    const parallel = params.parallel || 1;
    const workerRefs: WorkerRef[] = [];

    for (let i = 0; i < parallel; i++) {
      const workerId = parallel > 1 ? `${unit}:${role}:${i}` : `${unit}:${role}`;

      if (this.workers.has(workerId)) {
        const ref = this.workers.get(workerId)!;
        workerRefs.push(ref);
        // FORCE PARAMETER UPDATE FOR EXISTING WORKER
        // This stops the old loop and starts a new one with fresh slicing params.
        if (ref.ready) {
          ref.worker.postMessage({
            type: 'start_role_loop',
            role,
            params: { ...params, index: i, parallel },
          });
        }
        continue;
      }

      console.log(`[Dispatch] Spawning worker ${i + 1}/${parallel} for ${unit} (role: ${role})`);
      const worker = new Worker(ComputeWorkerUrl, { type: 'module' });
      const workerRef: WorkerRef = { worker, unit, role, ready: false };
      this.workers.set(workerId, workerRef);
      workerRefs.push(workerRef);

      // Initialize each worker
      const initPromise = new Promise<void>((resolve, reject) => {
        worker.onmessage = event => {
          const { type, id, result, error } = event.data;

          switch (type) {
            case 'ready':
              workerRef.ready = true;
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
        sab,
        memory,
        sabOffset: offset,
        sabSize: sab.byteLength,
        role,
      });

      if (i === 0) await initPromise; // Wait for leader to be ready before spawning others?
      // Actually, they can all spawn in parallel.
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
    input: Uint8Array | null = null
  ): Promise<Uint8Array | null> {
    // Check if we have a plugged worker for this library
    // Priority: role-specific worker (if any) > generic unit worker > local execution
    const workerRef = Array.from(this.workers.values()).find(w => w.unit === library);

    if (workerRef && workerRef.ready) {
      return this.executeOnWorker(workerRef.worker, library, method, params);
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

      if (resultPtr === 0) return null;

      const resultView = new DataView(this.memory!.buffer);
      const outputLen = resultView.getUint32(resultPtr, true);
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
        }
      }

      if (this.exports.compute_free) {
        this.exports.compute_free(resultPtr, 4 + outputLen);
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
}

// Global dispatcher instance
let instance: Dispatcher | null = null;

export const dispatch = {
  internal: () => instance,

  initialize: (exports: any, memory: WebAssembly.Memory) => {
    instance = new Dispatcher(exports, memory);
    return instance;
  },

  register: (unit: string, methods: string[]) => {
    instance?.registerUnit(unit, methods);
  },

  has: (unit: string, method?: string) => instance?.hasCapability(unit, method) || false,

  /**
   * Dedicated background unit registration
   */
  plug: (unit: string, role: string, params: object = {}) => {
    if (!instance) instance = new Dispatcher(); // Lazy init if no local exports
    return instance.plug(unit, role, params);
  },

  execute: (
    library: string,
    method: string,
    params: object = {},
    input: Uint8Array | null = null
  ) => {
    if (!instance) throw new Error('Dispatcher not initialized');
    return instance.execute(library, method, params, input);
  },

  json: <T = any>(library: string, method: string, params: object = {}) => {
    if (!instance) throw new Error('Dispatcher not initialized');
    return instance.json<T>(library, method, params);
  },

  shutdown: () => instance?.shutdown(),
};

export default dispatch;
