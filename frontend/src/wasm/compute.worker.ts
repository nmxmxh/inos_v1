/**
 * INOS Compute Worker
 *
 * Runs Rust WASM modules in a dedicated Web Worker for parallel execution.
 * Physics and matrix generation happen here, writing directly to SAB.
 * Main thread only reads SAB epochs - no blocking.
 *
 * Architecture:
 * - Receives SAB reference from main thread
 * - Loads Rust WASM modules (compute, diagnostics)
 * - Runs physics loop independently at ~60fps
 * - Writes results to SAB and increments epochs
 * - Main thread reads via Atomics polling
 */

/// <reference lib="webworker" />

declare const self: DedicatedWorkerGlobalScope;

import { WasmHeap } from './heap';
import { createBaseEnv, createPlaceholders } from './bridge';
import { INOSBridge } from './bridge-state';
import { IDX_PHYSICS_BARRIER, IDX_PHYSICS_PULSE, IDX_MATH_BARRIER } from './layout';

// Worker-scoped state
let _sab: SharedArrayBuffer | null = null;
let _memory: WebAssembly.Memory | null = null;
let _modules: Record<string, ModuleExports> = {};
let _dispatcher: WorkerDispatcher | null = null;
let _running = false;

// Layout constants are unused in this worker - physics writes to SAB directly
// The main thread reads epochs from layout.ts constants

interface ModuleExports {
  exports: WebAssembly.Exports;
  memory: WebAssembly.Memory;
}

// =============================================================================
// MODULE LOADING (Worker Context)
// =============================================================================

const MODULE_IDS: Record<string, number> = {
  compute: 1,
  vault: 2,
  drivers: 3,
  diagnostics: 4,
};

// Module compilation cache (worker-scoped)
const compiledModules = new Map<string, WebAssembly.Module>();
const moduleInstances = new Map<string, ModuleExports>();

async function loadModuleInWorker(
  name: string,
  sharedMemory: WebAssembly.Memory
): Promise<ModuleExports> {
  // Check cache
  if (moduleInstances.has(name)) {
    return moduleInstances.get(name)!;
  }

  let compiledModule: WebAssembly.Module;

  if (compiledModules.has(name)) {
    compiledModule = compiledModules.get(name)!;
  } else {
    // Fetch and compile
    const url = `/modules/${name}.wasm`;
    console.log(`[ComputeWorker] Loading module: ${name}`);

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

  // Add common stubs
  linker.__wbindgen_placeholder__.__wbg_new_8a6f238a6ece86ea = () => ({});
  linker.__wbindgen_placeholder__.__wbg_new_no_args_cb138f77cf6151ee = () => ({});
  linker.__wbindgen_placeholder__.__wbg_new_abda76e883ba8a5f = () => ({});
  linker.__wbindgen_placeholder__.__wbg_new_16b304a2cfa7ff4a = () => ({});

  // Instantiate
  const result = await WebAssembly.instantiate(compiledModule, linker as WebAssembly.Imports);
  exports = result.exports;

  // Initialize module
  // Use worker-global for module ID (no window in worker)
  (self as any).__INOS_MODULE_ID__ = MODULE_IDS[name] || 0;
  const initFn = (exports as any)[`${name}_init_with_sab`] || (exports as any).init_with_sab;

  if (typeof initFn === 'function') {
    const success = initFn();
    if (!success) {
      console.warn(`[ComputeWorker] ${name} initialization reported failure`);
    }
  }

  const moduleExports: ModuleExports = {
    exports,
    memory: (exports as any).memory || sharedMemory,
  };

  moduleInstances.set(name, moduleExports);
  console.log(`[ComputeWorker] Module loaded: ${name}`);

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
// DISPATCHER (Worker Context)
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

  executeSync(
    library: string,
    method: string,
    params: object | Uint8Array = {}
  ): Uint8Array | null {
    const exp = this.exports as any;
    if (!exp.compute_execute) {
      throw new Error('compute_execute export missing');
    }

    const libBytes = this.getEncoded(library);
    const methodBytes = this.getEncoded(method);
    const paramsBytes =
      params instanceof Uint8Array
        ? params
        : WorkerDispatcher.encoder.encode(JSON.stringify(params));

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
      const finalResult = new Uint8Array(output);

      if (exp.compute_free) {
        exp.compute_free(resultPtr, 4 + outputLen);
      }

      return finalResult;
    } finally {
      if (exp.compute_free) {
        exp.compute_free(libPtr, libBytes.length);
        exp.compute_free(methodPtr, methodBytes.length);
        exp.compute_free(paramsPtr, paramsBytes.length);
      }
    }
  }

  executeJson<T = unknown>(library: string, method: string, params: object = {}): T | null {
    const res = this.executeSync(library, method, params);
    if (!res) return null;
    try {
      return JSON.parse(WorkerDispatcher.decoder.decode(res)) as T;
    } catch {
      return null;
    }
  }
}

// =============================================================================
// PERFORMANCE LOOPS (Multi-Core Pipelined)
// =============================================================================

function runPhysicsRole(params: any): void {
  const {
    birdCount = 1000,
    parallel = 1,
    index = 0,
    targetHz = 240, // Target frequency for AR/VR consistency
  } = params;

  if (!_dispatcher || !_sab) return;

  const isLeader = index === 0;
  const sliceSize = Math.floor(birdCount / parallel);
  const sliceStart = index * sliceSize;
  const sliceCount = index === parallel - 1 ? birdCount - sliceStart : sliceSize;

  console.log(
    `[ComputeWorker:Physics:${index}] Starting ${isLeader ? 'Leader' : 'Worker'} (${sliceCount} birds) at ${targetHz}Hz`
  );
  _running = true;

  const encoder = new TextEncoder();
  const flags = INOSBridge.getFlagsView()!;

  // Barrier Indices (defined in sab_layout.capnp / layout.ts)
  // IDX_PHYSICS_BARRIER (23) - End-of-frame count
  // IDX_PHYSICS_PULSE (24) - Start-of-frame signal

  let lastTime = performance.now();
  let frames = 0;
  let lastReport = performance.now();
  let frameTime = 1000 / targetHz;
  let nextFrameTime = performance.now();
  let lastPulse = 0;

  const loop = () => {
    if (!_running) return;

    if (isLeader) {
      const now = performance.now();
      // PACER: Only leader manages the clock
      if (now < nextFrameTime) {
        self.setTimeout(loop, 1);
        return;
      }

      const dt = Math.min((now - lastTime) / 1000, 0.05);
      lastTime = now;
      nextFrameTime = now + frameTime;

      // START FRAME: Increment pulse to release workers
      Atomics.add(flags, IDX_PHYSICS_PULSE, 1);
      Atomics.notify(flags, IDX_PHYSICS_PULSE, parallel - 1);

      const stepParams = encoder.encode(
        JSON.stringify({
          bird_count: birdCount,
          dt,
          slice_start: sliceStart,
          slice_count: sliceCount,
          skip_flip: parallel > 1, // We'll manually flip after barrier if parallel
          worker_index: index,
        })
      );

      try {
        _dispatcher!.executeSync('boids', 'step_physics', stepParams);

        if (parallel > 1) {
          // GATHER: Wait for workers
          let expected = parallel - 1;
          while (Atomics.load(flags, IDX_PHYSICS_BARRIER) < expected) {
            // Use Atomics.wait without fixed timeout; notify will wake us
            Atomics.wait(flags, IDX_PHYSICS_BARRIER, Atomics.load(flags, IDX_PHYSICS_BARRIER));
            if (!_running) return;
          }
          // Reset barrier
          Atomics.store(flags, IDX_PHYSICS_BARRIER, 0);

          // MANUAL FLIP: Finalize frame after all slices are written
          const flipParams = encoder.encode(
            JSON.stringify({ bird_count: birdCount, dt, slice_count: 0, skip_flip: false })
          );
          _dispatcher!.executeSync('boids', 'step_physics', flipParams);
        } else {
          // Single core flip
          const flipParams = encoder.encode(
            JSON.stringify({
              bird_count: birdCount,
              dt,
              slice_count: 0,
              skip_flip: false,
              worker_index: index,
            })
          );
          _dispatcher!.executeSync('boids', 'step_physics', flipParams);
        }

        frames++;
      } catch (err) {
        console.error(`[ComputeWorker:Physics:${index}] Error:`, err);
        _running = false;
        return;
      }
    } else {
      // WORKER: Wait for pulse to CHANGE from what we last saw
      let currentPulse = Atomics.load(flags, IDX_PHYSICS_PULSE);
      while (currentPulse === lastPulse) {
        Atomics.wait(flags, IDX_PHYSICS_PULSE, lastPulse);
        currentPulse = Atomics.load(flags, IDX_PHYSICS_PULSE);
        if (!_running) return;
      }
      lastPulse = currentPulse;

      const dt = 1 / targetHz; // Nominal DT for workers matching pacer target
      const stepParams = encoder.encode(
        JSON.stringify({
          bird_count: birdCount,
          dt,
          slice_start: sliceStart,
          slice_count: sliceCount,
          skip_flip: true,
          worker_index: index,
        })
      );

      try {
        _dispatcher!.executeSync('boids', 'step_physics', stepParams);
        // SIGNAL COMPLETION
        Atomics.add(flags, IDX_PHYSICS_BARRIER, 1);
        Atomics.notify(flags, IDX_PHYSICS_BARRIER, 1);
      } catch (err) {
        console.error(`[ComputeWorker:Physics:${index}] Error:`, err);
        _running = false;
        return;
      }
    }

    // Report performance
    const now = performance.now();
    if (isLeader && now - lastReport > 1000) {
      const fps = (frames * 1000) / (now - lastReport);
      console.log(
        `[ComputeWorker:Physics:Pool] Consistent Throughput: ${fps.toFixed(1)} steps/sec`
      );
      frames = 0;
      lastReport = now;
    }

    self.setTimeout(loop, 0);
  };

  loop();
}

// Track active loop generation to prevent zombie loops on restart
let _activeLoopId = 0;

function runMathRole(params: any): void {
  const currentLoopId = ++_activeLoopId;

  if (!_dispatcher || !_sab) return;

  const { birdCount = 1000, index = 0, parallel = 1 } = params;

  // Partition the workload
  const sliceSize = Math.floor(birdCount / parallel);
  const sliceStart = index * sliceSize;
  const sliceCount = index === parallel - 1 ? birdCount - sliceStart : sliceSize;

  console.log(
    `[ComputeWorker:Math:${index}] Starting pipelined matrix loop (${sliceCount} birds, offset ${sliceStart})`
  );
  _running = true;

  const flags = INOSBridge.getFlagsView();

  if (!flags) {
    console.error(`[ComputeWorker:Math:${index}] Flags view not available`);
    return;
  }

  const IDX_BIRD_EPOCH = 12;
  let lastProcessedEpoch = Atomics.load(flags, IDX_BIRD_EPOCH);
  let frames = 0;
  let lastReport = performance.now();

  const loop = () => {
    // 1. Check if we should stop (global flag)
    if (!_running) return;

    // 2. Check if we are stale (new loop started)
    if (currentLoopId !== _activeLoopId) {
      // console.log(`[ComputeWorker:Math:${index}] Stale loop ${currentLoopId} exiting`);
      return;
    }

    // Wait for BIRD_EPOCH to change
    let currentEpoch = Atomics.load(flags, IDX_BIRD_EPOCH);
    if (currentEpoch === lastProcessedEpoch) {
      Atomics.wait(flags, IDX_BIRD_EPOCH, lastProcessedEpoch);
      currentEpoch = Atomics.load(flags, IDX_BIRD_EPOCH);
    }

    if (currentEpoch !== lastProcessedEpoch) {
      // Generic Atomic Barrier (Last Man Standing) for Generic Parallelism
      // Compute with skip_flip=true (Generic Standard)
      try {
        // Pass parameters as an object to executeSync, allowing it to handle encoding.
        // We must construct the object here or reuse a pre-built one.
        // Since we need skip_flip: true, and previously mathParams was encoded without it (likely),
        // let's pass the raw object.
        // Note: sliceStart, sliceCount are captured from closure.
        _dispatcher!.executeSync('math', 'compute_instance_matrices', {
          count: birdCount,
          slice_start: sliceStart,
          slice_count: sliceCount,
          skip_flip: true,
        });
        frames++;
      } catch (err) {
        console.error(`[ComputeWorker:Math:${index}] Error:`, err);
        _running = false;
        return;
      }

      // Barrier Logic
      // Ticket-based barrier that survives race conditions
      const ticket = Atomics.add(flags, IDX_MATH_BARRIER, 1);
      const count = ticket + 1;

      if (count === parallel) {
        // I am the Last Worker to finish this frame.
        // 1. Flip the epoch (making the buffer visible)
        try {
          _dispatcher!.executeSync('math', 'flip_matrix_epoch', {});
          // console.log(`[ComputeWorker:Math:${index}] FLIPPED EPOCH`);
        } catch (e) {
          console.error(e);
        }

        // 2. Reset the barrier for the next frame
        // We subtract 'parallel' instead of storing 0 to handle race conditions
        // where fast workers have already incremented for the next frame.
        Atomics.sub(flags, IDX_MATH_BARRIER, parallel);
      }

      // All workers update their local tracker to the CURRENT epoch they just processed
      lastProcessedEpoch = currentEpoch;
    } // Close the 'if (currentEpoch !== lastProcessedEpoch)' block

    const now = performance.now();
    if (now - lastReport > 1000) {
      const fps = (frames * 1000) / (now - lastReport);
      console.log(`[ComputeWorker:Math:${index}] Throughput: ${fps.toFixed(1)} ops/sec`);
      frames = 0;
      lastReport = now;
    }

    self.setTimeout(loop, 0);
  };

  loop();
}

function stopLoops(): void {
  _running = false;
}

// =============================================================================
// MESSAGE HANDLER
// =============================================================================

self.onmessage = async (event: MessageEvent<any>) => {
  const { type, id, role, params } = event.data;

  try {
    switch (type) {
      case 'init': {
        const { sab, memory } = event.data;
        if (!sab) throw new Error('SAB is required for init');

        _sab = sab;
        _memory =
          memory ||
          new WebAssembly.Memory({
            initial: Math.ceil(sab.byteLength / 65536),
            maximum: Math.ceil(sab.byteLength / 65536) * 2,
            shared: true,
          });

        INOSBridge.initialize(
          sab,
          event.data.sabOffset || 0,
          event.data.sabSize || sab.byteLength,
          _memory!
        );

        (self as any).__INOS_SAB__ = sab;
        (self as any).__INOS_SAB_OFFSET__ = event.data.sabOffset || 0;
        (self as any).__INOS_SAB_SIZE__ = event.data.sabSize || sab.byteLength;
        (self as any).__INOS_SAB_INT32__ = INOSBridge.getFlagsView();

        const moduleNames = ['compute', 'diagnostics'];
        for (const name of moduleNames) {
          const mod = await loadModuleInWorker(name, _memory!);
          _modules[name] = mod;
        }

        if (_modules.compute) {
          _dispatcher = new WorkerDispatcher(_modules.compute.exports, _modules.compute.memory);
        }

        self.postMessage({ type: 'ready' });
        break;
      }

      case 'start_role_loop': {
        const { birdCount = 1000 } = params || {};

        // Initialize population if needed (Only Leader or single worker does this)
        if (
          _dispatcher &&
          role === 'physics' &&
          (params.index === 0 || params.index === undefined)
        ) {
          _dispatcher.executeSync('boids', 'init_population', { bird_count: birdCount });
        }

        if (role === 'physics') runPhysicsRole(params || { birdCount });
        else if (role === 'math') runMathRole(params || { birdCount });
        break;
      }

      case 'execute': {
        if (!_dispatcher) throw new Error('Dispatcher not initialized');
        const { library, method, params } = event.data;
        const result = _dispatcher.executeJson(library, method, params || {});
        self.postMessage({ type: 'result', id, result });
        break;
      }

      case 'shutdown': {
        stopLoops();
        _modules = {};
        _dispatcher = null;
        _sab = null;
        _memory = null;
        break;
      }

      default:
        console.warn(`[ComputeWorker] Unknown type: ${type}`);
    }
  } catch (error) {
    self.postMessage({
      type: 'error',
      id,
      error: error instanceof Error ? error.message : String(error),
    });
  }
};

// Export for TypeScript
export {};

// Export for TypeScript module resolution
export {};
