/**
 * INOS Compute Worker
 *
 * Simplified architecture: Loads Rust WASM modules in a dedicated Web Worker.
 * Main thread dispatches commands via postMessage, worker executes and writes to SAB.
 *
 * Architecture:
 * - Receives SAB reference from main thread
 * - Loads Rust WASM modules (compute, diagnostics)
 * - Exposes simple execute API
 * - Writes results to SAB, flips epochs
 * - Main thread reads via Atomics
 */

/// <reference lib="webworker" />

declare const self: DedicatedWorkerGlobalScope;

import { WasmHeap } from './heap';
import { createBaseEnv, createPlaceholders } from './bridge';
import { INOSBridge } from './bridge-state';
import { IDX_SYSTEM_PULSE } from './layout';

// Worker-scoped state
let _memory: WebAssembly.Memory | null = null;
let _modules: Record<string, ModuleExports> = {};
let _dispatcher: WorkerDispatcher | null = null;
let _isLooping = false;
let _loopParams: any = {};

interface ModuleExports {
  exports: WebAssembly.Exports;
  memory: WebAssembly.Memory;
}

// =============================================================================
// MODULE LOADING
// =============================================================================

const MODULE_IDS: Record<string, number> = {
  compute: 1,
  vault: 2,
  drivers: 3,
  diagnostics: 4,
};

const compiledModules = new Map<string, WebAssembly.Module>();
const moduleInstances = new Map<string, ModuleExports>();

async function loadModuleInWorker(
  name: string,
  sharedMemory: WebAssembly.Memory
): Promise<ModuleExports> {
  if (moduleInstances.has(name)) {
    return moduleInstances.get(name)!;
  }

  let compiledModule: WebAssembly.Module;

  if (compiledModules.has(name)) {
    compiledModule = compiledModules.get(name)!;
  } else {
    const url = `/modules/${name}.wasm`;
    console.log(`[ComputeWorker] Loading: ${name}`);

    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`Failed to fetch ${url}: ${response.status}`);
    }

    try {
      compiledModule = await WebAssembly.compileStreaming(response);
    } catch {
      const bytes = await response.arrayBuffer();
      compiledModule = await WebAssembly.compile(bytes);
    }

    compiledModules.set(name, compiledModule);
  }

  // Setup heap and imports
  const imports = WebAssembly.Module.imports(compiledModule);
  const heap = new WasmHeap();
  const addHeapObject = (obj: unknown) => heap.add(obj);
  const getObject = (idx: number) => heap.get(idx);

  let exports: WebAssembly.Exports;

  const getBuffer = () => {
    if (exports && (exports as any).memory) {
      return ((exports as any).memory as WebAssembly.Memory).buffer;
    }
    return sharedMemory.buffer;
  };

  const baseEnv = createBaseEnv(heap, getBuffer);
  const placeholders = createPlaceholders(heap, getBuffer);

  const linker: Record<string, Record<string, unknown>> = {
    env: {
      ...baseEnv,
      memory: sharedMemory,
    },
    __wbindgen_placeholder__: {},
  };

  // Dynamic import mapping
  imports.forEach((imp: WebAssembly.ModuleImportDescriptor) => {
    if (imp.module === '__wbindgen_placeholder__') {
      if ((placeholders as Record<string, unknown>)[imp.name]) {
        linker.__wbindgen_placeholder__[imp.name] = (placeholders as Record<string, unknown>)[
          imp.name
        ];
      } else if (imp.name.indexOf('__wbg_new') !== -1) {
        handleNewImport(imp.name, linker, getObject, addHeapObject, getBuffer);
      } else if (imp.name.indexOf('__wbg_') !== -1) {
        handleWbgImport(imp.name, linker, getObject, addHeapObject);
      } else {
        linker.__wbindgen_placeholder__[imp.name] = () => {};
      }
    } else if (imp.module === '__wbindgen_externref_xform__') {
      if (!linker.__wbindgen_externref_xform__) {
        linker.__wbindgen_externref_xform__ = {};
      }
      linker.__wbindgen_externref_xform__[imp.name] = (...args: unknown[]) => args[0];
    }
  });

  // Common stubs
  linker.__wbindgen_placeholder__.__wbg_new_8a6f238a6ece86ea = () => ({});
  linker.__wbindgen_placeholder__.__wbg_new_no_args_cb138f77cf6151ee = () => ({});
  linker.__wbindgen_placeholder__.__wbg_new_abda76e883ba8a5f = () => ({});
  linker.__wbindgen_placeholder__.__wbg_new_16b304a2cfa7ff4a = () => ({});

  // Instantiate
  const result = await WebAssembly.instantiate(compiledModule, linker as WebAssembly.Imports);
  exports = result.exports;

  // Initialize module
  (self as any).__INOS_MODULE_ID__ = MODULE_IDS[name] || 0;
  const initFn = (exports as any)[`${name}_init_with_sab`] || (exports as any).init_with_sab;

  if (typeof initFn === 'function') {
    initFn();
  }

  const moduleExports: ModuleExports = {
    exports,
    memory: (exports as any).memory || sharedMemory,
  };

  moduleInstances.set(name, moduleExports);
  console.log(`[ComputeWorker] Loaded: ${name}`);

  return moduleExports;
}

function handleNewImport(
  name: string,
  linker: Record<string, Record<string, unknown>>,
  getObject: (idx: number) => unknown,
  addHeapObject: (obj: unknown) => number,
  getBuffer: () => ArrayBuffer
) {
  if (name.indexOf('new_with_byte_offset_and_length') !== -1) {
    linker.__wbindgen_placeholder__[name] = (bufferIdx: number, offset: number, len: number) => {
      const buffer = getObject(bufferIdx) as ArrayBuffer;
      return addHeapObject(new Uint8Array(buffer, offset, len));
    };
  } else if (name.indexOf('new_from_slice') !== -1) {
    linker.__wbindgen_placeholder__[name] = (ptr: number, len: number) => {
      return addHeapObject(new Uint8Array(getBuffer(), ptr, len));
    };
  } else if (name.match(/int32array_new/i)) {
    linker.__wbindgen_placeholder__[name] = (arg0: number, arg1: number, arg2: number) => {
      return addHeapObject(new Int32Array(getObject(arg0) as ArrayBuffer, arg1, arg2));
    };
  } else if (name.match(/uint8array_new/i)) {
    linker.__wbindgen_placeholder__[name] = (arg0: number) => {
      return addHeapObject(new Uint8Array(getObject(arg0) as ArrayBuffer));
    };
  } else {
    linker.__wbindgen_placeholder__[name] = () => addHeapObject(new Object());
  }
}

function handleWbgImport(
  name: string,
  linker: Record<string, Record<string, unknown>>,
  getObject: (idx: number) => unknown,
  addHeapObject: (obj: unknown) => number
) {
  if (name.indexOf('byteLength') !== -1) {
    linker.__wbindgen_placeholder__[name] = (idx: number) =>
      (getObject(idx) as ArrayBuffer).byteLength;
  } else if (name.indexOf('length') !== -1) {
    linker.__wbindgen_placeholder__[name] = (idx: number) => (getObject(idx) as unknown[]).length;
  } else if (name.indexOf('subarray') !== -1) {
    linker.__wbindgen_placeholder__[name] = (idx: number, a: number, b: number) => {
      return addHeapObject((getObject(idx) as Uint8Array).subarray(a, b));
    };
  } else if (name.indexOf('set') !== -1) {
    linker.__wbindgen_placeholder__[name] = (idx: number, valIdx: number, off: number) => {
      (getObject(idx) as Uint8Array).set(getObject(valIdx) as Uint8Array, off);
    };
  } else if (name.indexOf('load') !== -1) {
    linker.__wbindgen_placeholder__[name] = (bitsIdx: number, idx: number) => {
      return Atomics.load(getObject(bitsIdx) as Int32Array, idx);
    };
  } else if (name.indexOf('store') !== -1) {
    linker.__wbindgen_placeholder__[name] = (bitsIdx: number, idx: number, val: number) => {
      return Atomics.store(getObject(bitsIdx) as Int32Array, idx, val);
    };
  } else if (name.indexOf('add') !== -1) {
    linker.__wbindgen_placeholder__[name] = (bitsIdx: number, idx: number, val: number) => {
      return Atomics.add(getObject(bitsIdx) as Int32Array, idx, val);
    };
  } else {
    linker.__wbindgen_placeholder__[name] = () => {};
  }
}

// =============================================================================
// DISPATCHER
// =============================================================================

class WorkerDispatcher {
  private exports: WebAssembly.Exports;
  private memory: WebAssembly.Memory;
  private static encoder = new TextEncoder();
  private static decoder = new TextDecoder();
  private stringCache = new Map<string, Uint8Array>();

  constructor(exports: WebAssembly.Exports, memory: WebAssembly.Memory) {
    this.exports = exports;
    this.memory = memory;
  }

  private getEncoded(str: string): Uint8Array {
    let cached = this.stringCache.get(str);
    if (!cached) {
      cached = WorkerDispatcher.encoder.encode(str);
      this.stringCache.set(str, cached);
    }
    return cached;
  }

  execute(library: string, method: string, params: object = {}): any {
    const exp = this.exports as any;
    if (!exp.compute_execute) {
      throw new Error('compute_execute export missing');
    }

    const libBytes = this.getEncoded(library);
    const methodBytes = this.getEncoded(method);
    const paramsBytes = WorkerDispatcher.encoder.encode(JSON.stringify(params));

    const libPtr = exp.compute_alloc(libBytes.length);
    const methodPtr = exp.compute_alloc(methodBytes.length);
    const paramsPtr = exp.compute_alloc(paramsBytes.length);

    const heap = new Uint8Array(this.memory.buffer);
    heap.set(libBytes, libPtr);
    heap.set(methodBytes, methodPtr);
    heap.set(paramsBytes, paramsPtr);

    try {
      const resultPtr = exp.compute_execute(
        libPtr,
        libBytes.length,
        methodPtr,
        methodBytes.length,
        0,
        0, // No input buffer
        paramsPtr,
        paramsBytes.length
      );

      if (resultPtr === 0) return null;

      const resultView = new DataView(this.memory.buffer);
      const outputLen = resultView.getUint32(resultPtr, true);
      const output = new Uint8Array(this.memory.buffer, resultPtr + 4, outputLen);

      let result = null;
      try {
        result = JSON.parse(WorkerDispatcher.decoder.decode(output));
      } catch {
        result = new Uint8Array(output);
      }

      if (exp.compute_free) {
        exp.compute_free(resultPtr, 4 + outputLen);
      }

      return result;
    } finally {
      if (exp.compute_free) {
        exp.compute_free(libPtr, libBytes.length);
        exp.compute_free(methodPtr, methodBytes.length);
        exp.compute_free(paramsPtr, paramsBytes.length);
      }
    }
  }
}

// =============================================================================
// MESSAGE HANDLER
// =============================================================================

self.onmessage = async (event: MessageEvent<any>) => {
  const { type, id } = event.data;

  try {
    switch (type) {
      case 'init': {
        const { sab, memory, sabOffset, sabSize, identity } = event.data;
        if (!sab) throw new Error('SAB is required');
        _memory =
          memory ||
          new WebAssembly.Memory({
            initial: Math.ceil(sab.byteLength / 65536),
            maximum: Math.ceil(sab.byteLength / 65536) * 2,
            shared: true,
          });

        INOSBridge.initialize(sab, sabOffset || 0, sabSize || sab.byteLength, _memory!);

        (self as any).__INOS_SAB__ = sab;
        (self as any).__INOS_SAB_OFFSET__ = sabOffset || 0;
        (self as any).__INOS_SAB_SIZE__ = sabSize || sab.byteLength;
        (self as any).__INOS_SAB_INT32__ = INOSBridge.getFlagsView();
        if (identity) {
          (self as any).__INOS_IDENTITY__ = identity;
          if (typeof identity.nodeId === 'string') {
            (self as any).__INOS_NODE_ID__ = identity.nodeId;
          }
          if (typeof identity.deviceId === 'string') {
            (self as any).__INOS_DEVICE_ID__ = identity.deviceId;
          }
          if (typeof identity.did === 'string') {
            (self as any).__INOS_DID__ = identity.did;
          }
        }

        // Load modules
        for (const name of ['compute', 'diagnostics']) {
          const mod = await loadModuleInWorker(name, _memory!);
          _modules[name] = mod;
        }

        if (_modules.compute) {
          _dispatcher = new WorkerDispatcher(_modules.compute.exports, _modules.compute.memory);
        }

        self.postMessage({ type: 'ready' });
        break;
      }

      case 'execute': {
        if (!_dispatcher) throw new Error('Dispatcher not initialized');
        const { library, method, params } = event.data;
        const result = _dispatcher.execute(library, method, params || {});
        if (method === 'step_physics' && (self as any).__INOS_DEBUG_COMPUTE__) {
          console.log(`[ComputeWorker] ✅ ${method} finished (id: ${id})`);
        }
        self.postMessage({ type: 'result', id, result });
        break;
      }

      case 'start_role_loop': {
        if (!_dispatcher) throw new Error('Dispatcher not initialized');
        const { params } = event.data;
        _loopParams = params || {};

        if (!_isLooping) {
          _isLooping = true;
          runAutonomousLoop();
        }
        break;
      }

      case 'stop_role_loop': {
        _isLooping = false;
        break;
      }

      case 'shutdown': {
        _isLooping = false;
        _modules = {};
        _dispatcher = null;
        _memory = null;
        self.postMessage({ type: 'shutdown_complete' });
        break;
      }

      default:
        console.warn(`[ComputeWorker] Unknown message type: ${type}`);
    }
  } catch (error) {
    self.postMessage({
      type: 'error',
      id,
      error: error instanceof Error ? error.message : String(error),
    });
  }
};

async function runAutonomousLoop() {
  const flags = (self as any).__INOS_SAB_INT32__ as Int32Array;
  if (!flags || !_dispatcher || !_isLooping) return;

  const library = _loopParams.library || 'compute';
  const method = _loopParams.method || 'step_physics';

  console.log(`[ComputeWorker] Starting autonomous loop for ${library}:${method}`);

  while (_isLooping) {
    // 1. Get current pulse
    const lastPulse = Atomics.load(flags, IDX_SYSTEM_PULSE);

    // 2. Execute the unit logic
    // The dispatcher handles SAB mutation and epoch flipping
    try {
      _dispatcher.execute(library, method, _loopParams);
    } catch (e) {
      console.error(`[ComputeWorker] Loop error in ${library}:${method}:`, e);
      _isLooping = false;
      break;
    }

    // 3. PARK until the next pulse (Zero-CPU idling)
    // Atomics.wait blocks the thread, which is fine for a dedicated background worker.
    // We wait for the pulse to change from lastPulse.
    // If the pulse worker is throttled, THIS wait will naturally take longer.
    Atomics.wait(flags, IDX_SYSTEM_PULSE, lastPulse);
  }

  console.log(`[ComputeWorker] Autonomous loop stopped`);
}

export {};
