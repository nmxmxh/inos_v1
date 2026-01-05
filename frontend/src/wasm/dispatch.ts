/**
 * INOS Compute Dispatcher
 *
 * Standardized utility for calling WASM compute units via the unified dispatcher.
 * Handles memory allocation, string marshalling, and capability discovery.
 *
 * Architecture:
 * - marshalling: JS strings/objects -> WASM heap
 * - execution: Calls compute_execute (generic dispatcher)
 * - unmarshalling: WASM pointer -> JS Uint8Array/JSON
 * - cleanup: Reclaims heap memory via compute_free
 */

export interface DispatchResult<T = any> {
  success: boolean;
  data: T;
  error?: string;
}

export class Dispatcher {
  private exports: any;
  private memory: WebAssembly.Memory;
  private capabilities: Map<string, string[]> = new Map();

  constructor(exports: any, memory: WebAssembly.Memory) {
    this.exports = exports;
    this.memory = memory;
  }

  /**
   * Register or update capabilities for a unit
   */
  registerUnit(unit: string, methods: string[]) {
    this.capabilities.set(unit, methods);
    console.log(`[Dispatch] Registered unit '${unit}' with ${methods.length} methods`);
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
   * Execute a compute operation synchronously
   */
  executeSync(
    library: string,
    method: string,
    params: object = {},
    input: Uint8Array | null = null
  ): Uint8Array | null {
    if (!this.exports.compute_execute) {
      throw new Error('compute_execute export missing');
    }

    const encoder = new TextEncoder();

    // 1. Prepare strings and JSON
    const libBytes = encoder.encode(library);
    const methodBytes = encoder.encode(method);
    const paramsBytes = encoder.encode(JSON.stringify(params));

    // 2. Allocate on WASM heap
    const libPtr = this.exports.compute_alloc(libBytes.length);
    const methodPtr = this.exports.compute_alloc(methodBytes.length);
    const paramsPtr = this.exports.compute_alloc(paramsBytes.length);
    let inputPtr = 0;

    if (input) {
      inputPtr = this.exports.compute_alloc(input.length);
    }

    // 3. Copy data to heap
    const heap = new Uint8Array(this.memory.buffer);
    heap.set(libBytes, libPtr);
    heap.set(methodBytes, methodPtr);
    heap.set(paramsBytes, paramsPtr);
    if (input && inputPtr) {
      heap.set(input, inputPtr);
    }

    try {
      // 4. Run execution
      // console.log(`[Dispatch] Executing ${library}::${method}...`);
      // const start = performance.now();
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
      // const end = performance.now();
      // console.log(`[Dispatch] Execution ${library}::${method} took ${(end - start).toFixed(2)}ms`);

      if (resultPtr === 0) {
        console.warn(`[Dispatch] ${library}::${method} returned NULL result`);
        return null;
      }

      // 5. Read result
      // Format: [len: 4 bytes (u32, little-endian)] + [data: len bytes]
      const resultView = new DataView(this.memory.buffer);
      const outputLen = resultView.getUint32(resultPtr, true);
      const output = new Uint8Array(this.memory.buffer, resultPtr + 4, outputLen);

      // Copy to new array so we can free the WASM buffer
      const finalResult = new Uint8Array(output);

      // 6. Free result buffer in WASM
      if (this.exports.compute_free) {
        this.exports.compute_free(resultPtr, 4 + outputLen);
      }

      return finalResult;
    } finally {
      // 7. Cleanup input allocations
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
   * Execute a compute operation asynchronously (placeholder for future async WASM)
   */
  async execute(
    library: string,
    method: string,
    params: object = {},
    input: Uint8Array | null = null
  ): Promise<Uint8Array | null> {
    return this.executeSync(library, method, params, input);
  }

  /**
   * Execute and parse JSON result
   */
  executeJson<T = any>(library: string, method: string, params: object = {}): T | null {
    const res = this.executeSync(library, method, params);
    if (!res) return null;
    try {
      const str = new TextDecoder().decode(res);
      return JSON.parse(str) as T;
    } catch (e) {
      console.error('[Dispatch] JSON Parse error:', e);
      return null;
    }
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

  execute: (
    library: string,
    method: string,
    params: object = {},
    input: Uint8Array | null = null
  ) => {
    if (!instance) throw new Error('Dispatcher not initialized');
    return instance.executeSync(library, method, params, input);
  },

  json: <T = any>(library: string, method: string, params: object = {}) => {
    if (!instance) throw new Error('Dispatcher not initialized');
    return instance.executeJson<T>(library, method, params);
  },
};
