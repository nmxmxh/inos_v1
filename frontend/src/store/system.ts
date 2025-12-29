import { create } from 'zustand';

// Declare global types for WASM runtime
declare global {
  interface Window {
    Go: any;
  }
}

// Types
const REGISTRY_OFFSET = 0x000100;
const MODULE_ENTRY_SIZE = 96;
const MAX_MODULES_INLINE = 64;
const CAPABILITY_ENTRY_SIZE = 36;
const ARENA_OFFSET_BASE = 0x150000;

class RegistryReader {
  private view: DataView;
  private memory: WebAssembly.Memory;
  private sabOffset: number;

  constructor(memory: WebAssembly.Memory, sabOffset: number = 0) {
    this.memory = memory;
    this.view = new DataView(memory.buffer);
    this.sabOffset = sabOffset;
  }

  readString(offset: number, length: number): string {
    const bytes = new Uint8Array(this.memory.buffer, offset, length);
    // Find null terminator
    let end = 0;
    while (end < length && bytes[end] !== 0) end++;
    // SharedArrayBuffer (SAB) decoding fix: TextDecoder.decode() does not support SAB views.
    // We must slice to create a copy in an unshared ArrayBuffer before decoding.
    return new TextDecoder().decode(bytes.slice(0, end));
  }

  readCapabilities(arenaOffset: number, count: number): string[] {
    const capabilities: string[] = [];
    if (arenaOffset === 0 || count === 0) return capabilities;

    // The arenaOffset read from the registry is relative to the start of the SAB.
    // To read it from the WASM linear memory, we MUST add the sabOffset.
    const absoluteOffset = this.sabOffset + arenaOffset;

    if (
      arenaOffset < ARENA_OFFSET_BASE ||
      absoluteOffset + count * CAPABILITY_ENTRY_SIZE > this.memory.buffer.byteLength
    ) {
      console.warn(
        `[System] Invalid capability table at 0x${arenaOffset.toString(16)} (Absolute: 0x${absoluteOffset.toString(16)})`
      );
      return capabilities;
    }

    for (let i = 0; i < count; i++) {
      const entryOffset = absoluteOffset + i * CAPABILITY_ENTRY_SIZE;
      const id = this.readString(entryOffset, 32);
      if (id) capabilities.push(id);
    }
    return capabilities;
  }

  scan(): Record<
    string,
    { id: string; active: boolean; version: string; capabilities: string[]; memoryUsage: number }
  > {
    const modules: Record<string, any> = {};

    for (let i = 0; i < MAX_MODULES_INLINE; i++) {
      const offset = this.sabOffset + REGISTRY_OFFSET + i * MODULE_ENTRY_SIZE;
      const idHash = this.view.getUint32(offset + 8, true);
      if (idHash === 0) continue;

      const flags = this.view.getUint8(offset + 15);
      const isActive = (flags & 0b0010) !== 0;

      if (!isActive) continue;

      // Offsets based on Rust EnhancedModuleEntry (96 bytes):
      // module_id: 64
      // cap_table_offset: 56
      // cap_count: 60

      const getHex = (off: number) => `0x${off.toString(16)}`;

      const moduleId = this.readString(offset + 64, 12);
      const capTableOffset = this.view.getUint32(offset + 56, true);
      const capCount = this.view.getUint16(offset + 60, true);

      console.log(
        `[Registry] ID: ${moduleId}, Offset: ${getHex(offset)}, CapTable: ${getHex(capTableOffset)}, Count: ${capCount}`
      );

      const capabilities = this.readCapabilities(capTableOffset, capCount);

      modules[moduleId] = {
        id: moduleId,
        active: isActive,
        version: `${this.view.getUint8(offset + 12)}.${this.view.getUint8(offset + 13)}.${this.view.getUint8(offset + 14)}`,
        capabilities: capabilities,
        memoryUsage: this.view.getUint16(offset + 34, true),
      };
    }
    return modules;
  }
}

let registryParser: RegistryReader | null = null;

/**
 * Production-grade WASM Heap implementation.
 * Standardized at indices 0-3 for wasm-bindgen compatibility.
 * Elevates architecture with high-performance free-list and telemetry.
 */
class WasmHeap {
  private objects: any[];
  private nextFree: number | undefined;
  private peakUsage: number = 0;
  private totalAllocations: number = 0;

  constructor(initialCapacity: number = 256) {
    // Standard wasm-bindgen primitives
    this.objects = new Array(initialCapacity);
    this.objects[0] = undefined;
    this.objects[1] = null;
    this.objects[2] = true;
    this.objects[3] = false;

    // Initialize free list
    for (let i = 4; i < initialCapacity - 1; i++) {
      this.objects[i] = i + 1;
    }
    this.objects[initialCapacity - 1] = undefined;
    this.nextFree = 4;
    this.peakUsage = 4;
  }

  add(obj: any): number {
    if (this.nextFree === undefined) {
      // Exponential growth for P2P scaling
      const oldLen = this.objects.length;
      const newLen = oldLen * 2;
      const newObjects = new Array(newLen);
      // Copy old (faster than push spread in many JS engines)
      for (let i = 0; i < oldLen; i++) newObjects[i] = this.objects[i];
      // Build new free list
      for (let i = oldLen; i < newLen - 1; i++) newObjects[i] = i + 1;
      newObjects[newLen - 1] = undefined;
      this.objects = newObjects;
      this.nextFree = oldLen;
    }

    const idx = this.nextFree;
    this.nextFree = this.objects[idx];
    this.objects[idx] = obj;
    this.totalAllocations++;
    if (idx + 1 > this.peakUsage) this.peakUsage = idx + 1;
    return idx;
  }

  get(idx: number): any {
    return this.objects[idx];
  }

  drop(idx: number): void {
    if (idx < 4) return; // Primitives are immortal
    this.objects[idx] = this.nextFree;
    this.nextFree = idx;
  }

  getStats() {
    return {
      current: this.objects.length,
      peak: this.peakUsage,
      allocations: this.totalAllocations,
    };
  }
}

export interface KernelStats {
  nodes: number;
  particles: number;
  sector: number;
  fps: number;
}

export interface UnitState {
  id: string;
  active: boolean;
  capabilities: string[];
}

export interface SystemStore {
  status: 'initializing' | 'booting' | 'ready' | 'error';
  kernel: any | null;
  units: Record<string, UnitState>;
  stats: KernelStats;
  error: Error | null;

  // Actions
  initialize: () => Promise<void>;
  registerUnit: (unit: UnitState) => void;
  updateStats: (stats: Partial<KernelStats>) => void;
  setError: (error: Error) => void;
  scanRegistry: (memory: WebAssembly.Memory) => void;
}

export const useSystemStore = create<SystemStore>((set, get) => ({
  status: 'initializing',
  kernel: null,
  units: {},
  stats: {
    nodes: 1,
    particles: 1000,
    sector: 0,
    fps: 0,
  },
  error: null,

  scanRegistry: (memory: WebAssembly.Memory) => {
    if (!registryParser || registryParser['memory'] !== memory) {
      registryParser = new RegistryReader(memory, (window as any).__INOS_SAB_OFFSET__ || 0);
    }

    const realModules = registryParser.scan();

    set(state => {
      const updatedUnits = { ...state.units };

      Object.values(realModules).forEach(data => {
        updatedUnits[data.id] = {
          id: data.id,
          active: data.active,
          capabilities: data.capabilities,
        };
      });

      return { units: updatedUnits };
    });
  },

  initialize: async () => {
    if (get().status !== 'initializing' && get().status !== 'error') return;
    set({ status: 'booting' });

    try {
      console.log('[SystemStore] Starting INOS initialization...');

      // 1. Load wasm_exec.js (Go runtime)
      if (!window.Go) {
        const wasmExecScript = document.createElement('script');
        wasmExecScript.src = '/wasm_exec.js';
        await new Promise((resolve, reject) => {
          wasmExecScript.onload = resolve;
          wasmExecScript.onerror = reject;
          document.head.appendChild(wasmExecScript);
        });
      }

      // 2. Create SharedArrayBuffer for zero-copy architecture
      // JavaScript creates the SAB and provides it to both kernel and modules
      const sharedMemory = new WebAssembly.Memory({
        initial: 256, // 16MB (256 * 64KB pages)
        maximum: 32768, // 2GB max (32768 * 64KB pages)
        shared: true, // Enable SharedArrayBuffer
      });

      console.log(
        `[SystemStore] 🔗 Created SharedArrayBuffer: ${(sharedMemory.buffer.byteLength / 1024 / 1024).toFixed(1)}MB`
      );

      // 3. Load and instantiate Go kernel with provided SharedArrayBuffer
      // @ts-ignore
      const go = new Go();

      const response = await fetch('/kernel.wasm');
      if (!response.ok) throw new Error(`Failed to load kernel.wasm: ${response.statusText}`);

      const wasmBytes = await response.arrayBuffer();

      // Provide SharedArrayBuffer as memory to kernel
      const result = await WebAssembly.instantiate(wasmBytes, {
        ...go.importObject,
        env: {
          ...go.importObject.env,
          memory: sharedMemory, // Kernel uses our SharedArrayBuffer
        },
      });

      // Verify memory
      const memory = sharedMemory;
      console.log('[SystemStore] ✅ Kernel using provided memory');

      go.run(result.instance);
      console.log('[SystemStore] ✅ Kernel loaded and running (Memory captured)');

      // 4. Wait for Kernel to export zero-copy SAB functions
      // Go's main() runs asynchronously, so we poll until functions are available
      const maxWaitMs = 5000;
      const startTime = Date.now();
      while (!(window as any).getSystemSABAddress || !(window as any).getSystemSABSize) {
        if (Date.now() - startTime > maxWaitMs) {
          console.warn('[SystemStore] Kernel SAB export timeout - using full SAB');
          break;
        }
        await new Promise(resolve => setTimeout(resolve, 10));
      }

      // 5. Setup SharedArrayBuffer for modules
      // The kernel may export a sub-region, or we use the full SharedArrayBuffer
      const memoryBuffer = sharedMemory.buffer;

      // Runtime verification that it's actually a SharedArrayBuffer
      if (!(memoryBuffer instanceof SharedArrayBuffer)) {
        throw new Error(
          'WebAssembly.Memory.buffer is not a SharedArrayBuffer - shared flag may not have worked'
        );
      }

      // TypeScript doesn't narrow the type after instanceof check, so we assert
      const sabBase: SharedArrayBuffer = memoryBuffer as SharedArrayBuffer;
      let sabOffset = 0;
      let sabSize = sabBase.byteLength;

      if ((window as any).getSystemSABAddress && (window as any).getSystemSABSize) {
        // Kernel exports a specific region within the SharedArrayBuffer
        sabOffset = (window as any).getSystemSABAddress();
        sabSize = (window as any).getSystemSABSize();
        console.log(
          `[SystemStore] 🔗 Kernel sub-region at offset 0x${sabOffset.toString(16)} (Size: ${sabSize / 1024 / 1024}MB)`
        );
      } else {
        // Use the full SharedArrayBuffer
        console.log(
          `[SystemStore] 🔗 Using full SharedArrayBuffer (Size: ${sabSize / 1024 / 1024}MB)`
        );
      }

      (window as any).__INOS_SAB__ = sabBase;
      (window as any).__INOS_SAB_OFFSET__ = sabOffset;
      (window as any).__INOS_SAB_SIZE__ = sabSize;

      // Start periodic registry scan (every 2 seconds)
      setInterval(() => {
        get().scanRegistry(memory);
      }, 2000);

      // Initial scan
      get().scanRegistry(memory);

      // Module ID mapping for syscall authentication
      const MODULE_IDS: Record<string, number> = {
        compute: 1,
        science: 2,
        ml: 3,
        mining: 4,
        vault: 5,
        drivers: 6,
      };

      const loadModule = async (name: string) => {
        let moduleCapabilities: string[] = [];
        console.log(`[SystemStore] Loading module: ${name}`);
        const response = await fetch(`/modules/${name}.wasm?t=${Date.now()}`);
        if (!response.ok) throw new Error(`Failed to load ${name}.wasm`);
        const bytes = await response.arrayBuffer();

        // 1. Compile the module first (compile-time)
        const compiledModule = await WebAssembly.compile(bytes);

        // 2. Inspect requested imports to build dynamic bridge
        const imports = WebAssembly.Module.imports(compiledModule);
        // imports.forEach(i => console.log(`[SystemStore] Import requested: ${i.module} :: ${i.name}`));

        // --- Production WasmHeap Implementation ---
        const heap = new WasmHeap();
        const addHeapObject = (obj: any) => heap.add(obj);
        const getObject = (idx: number) => heap.get(idx);
        const dropObject = (idx: number) => heap.drop(idx);

        // 0. Declare exports early to avoid TDZ in closures
        let exports: any;

        // Helper to always get the current, non-detached memory buffer
        const getBuffer = () => {
          // Modules use imported memory (sharedMemory), not exports.memory
          return sharedMemory.buffer;
        };

        // Define base stable imports
        const baseEnv = {
          host_log: (ptr: number, len: number, level: number) => {
            const memoryBuffer = getBuffer();
            if (!memoryBuffer || memoryBuffer.byteLength === 0) return;
            const view = new Uint8Array(memoryBuffer, ptr, len);
            const msg = new TextDecoder().decode(view.slice());
            const methods = ['error', 'warn', 'info', 'debug', 'trace'];
            (console as any)[methods[level] || 'log'](msg);
          },
          // Stable ABI Implementation
          inos_create_u8_array: (ptr: number, len: number) => {
            return addHeapObject(new Uint8Array(getBuffer(), ptr, len));
          },
          inos_wrap_u8_array: (valIdx: number) => {
            const val = getObject(valIdx);
            return addHeapObject(new Uint8Array(val));
          },
          inos_create_u8_view: (bufferIdx: number, offset: number, len: number) => {
            const buffer = getObject(bufferIdx);
            return addHeapObject(new Uint8Array(buffer, offset, len));
          },
          inos_create_i32_view: (bufferIdx: number, offset: number, len: number) => {
            const buffer = getObject(bufferIdx);
            return addHeapObject(new Int32Array(buffer, offset, len));
          },
          // Fixed Global Access using Heap
          inos_get_global: () => {
            return addHeapObject(globalThis);
          },
          // Fixed Reflect.get using Heap (direct indices)
          inos_reflect_get: (targetIdx: number, keyIdx: number) => {
            const target = getObject(targetIdx);
            const key = getObject(keyIdx);

            if (target === undefined || target === null) {
              return addHeapObject(undefined);
            }

            try {
              const result = Reflect.get(target, key);
              return addHeapObject(result);
            } catch (e) {
              console.error(`[WASM-Heap] Reflect.get failed: ${e}`, {
                target,
                key,
                targetIdx,
                keyIdx,
              });
              return addHeapObject(undefined);
            }
          },
          inos_as_f64: (valIdx: number) => {
            const val = getObject(valIdx);
            return typeof val === 'number' ? val : Number(val);
          },
          // Stable ABI: Logging
          inos_log: (ptr: number, len: number, level: number) => {
            const memoryBuffer = getBuffer();
            if (!memoryBuffer || memoryBuffer.byteLength === 0) return;
            const view = new Uint8Array(memoryBuffer, ptr, len);
            const msg = new TextDecoder().decode(view.slice());
            const prefix = ['ERROR', 'WARN', 'INFO', 'DEBUG', 'TRACE'][level] || 'LOG';

            if (level === 0) console.warn(`[WASM-${prefix}] ${msg}`);
            else if (level <= 2) console.log(`[WASM-${prefix}] ${msg}`);
            else console.debug(`[WASM-${prefix}] ${msg}`);
          },

          // Stable ABI: Strings
          inos_create_string: (ptr: number, len: number) => {
            const memoryBuffer = getBuffer();
            if (!memoryBuffer || memoryBuffer.byteLength === 0) return addHeapObject('');
            const view = new Uint8Array(memoryBuffer, ptr, len);
            const str = new TextDecoder().decode(view.slice());
            // console.debug(`[WASM-ABI] Create String: "${str}"`);
            return addHeapObject(str);
          },

          // Stable ABI: Time
          inos_get_now: () => {
            return Date.now();
          },

          // Stable ABI: Atomics
          inos_atomic_add: (typedArrayIdx: number, index: number, value: number) => {
            const arr = getObject(typedArrayIdx);
            return Atomics.add(arr, index, value);
          },
          inos_atomic_load: (typedArrayIdx: number, index: number) => {
            const arr = getObject(typedArrayIdx);
            return Atomics.load(arr, index);
          },
          inos_atomic_store: (typedArrayIdx: number, index: number, value: number) => {
            const arr = getObject(typedArrayIdx);
            return Atomics.store(arr, index, value);
          },
          inos_atomic_wait: (
            typedArrayIdx: number,
            index: number,
            value: number,
            timeout: number
          ) => {
            const arr = getObject(typedArrayIdx);
            const res = Atomics.wait(arr, index, value, timeout === -1 ? undefined : timeout);
            return res === 'ok' ? 0 : res === 'not-equal' ? 1 : 2;
          },
          inos_atomic_compare_exchange: (
            typedArrayIdx: number,
            index: number,
            expected: number,
            replacement: number
          ) => {
            const arr = getObject(typedArrayIdx);
            return Atomics.compareExchange(arr, index, expected, replacement);
          },

          // Stable ABI: Math
          inos_math_random: () => {
            return Math.random();
          },

          // Stable ABI: Memory Copy
          inos_copy_to_sab: (
            targetBufferIdx: number,
            targetOffset: number,
            srcPtr: number,
            len: number
          ) => {
            const memoryBuffer = getBuffer();
            const targetBuffer = getObject(targetBufferIdx);
            const globalSAB = (window as any).__INOS_SAB__;
            console.log(
              `[inos_copy_to_sab] targetOffset=0x${targetOffset.toString(16)}, len=${len}, targetBuffer type:`,
              targetBuffer?.constructor?.name,
              'Same as global?',
              targetBuffer === globalSAB
            );
            if (!memoryBuffer || !targetBuffer) return;
            const src = new Uint8Array(memoryBuffer, srcPtr, len);
            const dest = new Uint8Array(targetBuffer, targetOffset, len);
            dest.set(src);
            console.log(
              `[inos_copy_to_sab] Wrote ${len} bytes to offset 0x${targetOffset.toString(16)}`
            );
            // Verify write
            const verify = new Uint8Array(globalSAB, targetOffset, Math.min(len, 64));
            console.log(
              `[inos_copy_to_sab] Verify first 64 bytes:`,
              Array.from(verify)
                .map(b => '0x' + b.toString(16).padStart(2, '0'))
                .join(' ')
            );
          },

          inos_copy_from_sab: (
            srcBufferIdx: number,
            srcOffset: number,
            destPtr: number,
            len: number
          ) => {
            const memoryBuffer = getBuffer();
            const srcBuffer = getObject(srcBufferIdx);
            if (!memoryBuffer || !srcBuffer) return;
            const src = new Uint8Array(srcBuffer, srcOffset, len);
            const dest = new Uint8Array(memoryBuffer, destPtr, len);
            dest.set(src);
          },

          inos_get_byte_length: (idx: number) => {
            const obj = getObject(idx);
            if (obj && typeof obj.byteLength === 'number') return obj.byteLength;
            if (obj && typeof obj.length === 'number') return obj.length;
            return 0;
          },
        };

        const placeholders = {
          // Error handling
          __wbindgen_throw: (ptr: number, len: number) => {
            const memoryBuffer = getBuffer();
            if (!memoryBuffer || memoryBuffer.byteLength === 0)
              throw new Error('WASM memory not ready');
            const view = new Uint8Array(memoryBuffer, ptr, len);
            const msg = new TextDecoder().decode(view.slice());
            throw new Error(`WASM panic: ${msg}`);
          },
          // Number handling
          // Note: __wbindgen_number_get usually takes a handle (index) NOT a pointer
          __wbindgen_number_get: (idx: number) => {
            const val = getObject(idx);
            return typeof val === 'number' ? val : NaN;
          },
          // Strings
          __wbindgen_string_new: (ptr: number, len: number) => {
            const memoryBuffer = getBuffer();
            if (!memoryBuffer || memoryBuffer.byteLength === 0) return addHeapObject('');
            const view = new Uint8Array(memoryBuffer, ptr, len);
            const str = new TextDecoder().decode(view.slice());
            const idx = addHeapObject(str);
            console.debug(`[WASM-Heap] string_new: "${str}" -> idx=${idx}`);
            return idx;
          },
          // Basic types
          __wbindgen_object_drop_ref: (idx: number) => dropObject(idx),
          __wbindgen_object_clone_ref: (idx: number) => {
            const obj = getObject(idx);
            return addHeapObject(obj);
          },
          __wbindgen_number_new: (n: number) => addHeapObject(n),
          __wbindgen_bigint_from_u64: (n: number) => addHeapObject(BigInt(n)),
          __wbindgen_jsval_eq: (aIdx: number, bIdx: number) => getObject(aIdx) === getObject(bIdx),
          __wbindgen_is_undefined: (idx: number) => getObject(idx) === undefined,
          __wbindgen_is_null: (idx: number) => getObject(idx) === null,
          __wbindgen_is_object: (idx: number) =>
            typeof getObject(idx) === 'object' && getObject(idx) !== null,
          __wbindgen_is_function: (idx: number) => typeof getObject(idx) === 'function',
          __wbindgen_is_string: (idx: number) => typeof getObject(idx) === 'string',
          __wbindgen_boolean_get: (idx: number) => (getObject(idx) ? 1 : 0),
          __wbindgen_externref_xform__: {
            __wbindgen_externref_table_grow: (delta: number) => delta,
            __wbindgen_externref_table_set_null: (_idx: number) => {},
          },

          // Describe stubs (for debug builds)
          __wbindgen_describe: (_v: number) => {},
          __wbindgen_describe_cast: (_a: number, _b: number) => {},
          __wbindgen_debug_string: (vIdx: number, _lenPtr: number) => {
            const val = getObject(vIdx);
            const debugStr = JSON.stringify(val); // Simple debug representation
            // const encoder = new TextEncoder();
            // const _view = encoder.encode(debugStr);
            // Write to memory? This requires allocating in Rust or writing to a pointer.
            // Usually this function returns [ptr, len] via retptr or similar.
            // But the signature in signature list is often (val, len_ptr).
            // Let's assume for now it's just logging or we can stub it safely.
            console.log(`[WASM-Debug] ${debugStr}`);
            // If we need to return it to Rust, we need to allocate.
            // For now, let's keep it as a safe stub that logs.
          },
        };

        // 3. Construct specific link object
        const linker: any = {
          env: {
            ...baseEnv,
            memory: memory, // CRITICAL: Inject the Kernel's shared memory
          },
          __wbindgen_placeholder__: {},
        };

        // 4. Dynamic Mapping Loop
        imports.forEach(imp => {
          console.debug(`[SystemStore] ${name} requested: ${imp.module} :: ${imp.name}`);
          if (imp.module === '__wbindgen_placeholder__') {
            if (placeholders[imp.name as keyof typeof placeholders]) {
              // Exact match
              linker.__wbindgen_placeholder__[imp.name] =
                placeholders[imp.name as keyof typeof placeholders];
            } else if (imp.name.indexOf('__wbg_new') !== -1) {
              // Handle generic 'new' constructors and slice allocators
              if (imp.name.indexOf('new_with_byte_offset_and_length') !== -1) {
                linker.__wbindgen_placeholder__[imp.name] = (
                  bufferIdx: number,
                  offset: number,
                  len: number
                ) => {
                  const buffer = getObject(bufferIdx);
                  return addHeapObject(new Uint8Array(buffer, offset, len));
                };
              } else if (imp.name.indexOf('new_from_slice') !== -1) {
                // Correct implementation using memoryProvider
                linker.__wbindgen_placeholder__[imp.name] = (ptr: number, len: number) => {
                  return addHeapObject(new Uint8Array(getBuffer(), ptr, len));
                };
              } else if (imp.name.match(/int32array_new/i)) {
                linker.__wbindgen_placeholder__[imp.name] = (arg0: any, arg1: any, arg2: any) => {
                  // Support variable args if possible, but bindgen usually names them specific.
                  // If name is just 'new', it's 1 arg.
                  return addHeapObject(new Int32Array(getObject(arg0), arg1, arg2));
                };
              } else if (imp.name.match(/uint8array_new/i)) {
                linker.__wbindgen_placeholder__[imp.name] = (arg0: any) => {
                  return addHeapObject(new Uint8Array(getObject(arg0)));
                };
              } else {
                console.warn(`[SystemStore] Stubbing unknown new: ${imp.name}`);
                linker.__wbindgen_placeholder__[imp.name] = () => addHeapObject(new Object());
              }
            } else if (imp.name.indexOf('__wbg_byteLength') !== -1) {
              linker.__wbindgen_placeholder__[imp.name] = (idx: number) => {
                return getObject(idx).byteLength;
              };
            } else if (imp.name.indexOf('__wbg_length') !== -1) {
              linker.__wbindgen_placeholder__[imp.name] = (idx: number) => {
                return getObject(idx).length;
              };
            } else if (imp.name.indexOf('__wbg_subarray') !== -1) {
              linker.__wbindgen_placeholder__[imp.name] = (idx: number, a: number, b: number) => {
                return addHeapObject(getObject(idx).subarray(a, b));
              };
            } else if (imp.name.indexOf('__wbg_set') !== -1) {
              linker.__wbindgen_placeholder__[imp.name] = (
                idx: number,
                valIdx: number,
                off: number
              ) => {
                getObject(idx).set(getObject(valIdx), off);
              };
            } else if (imp.name.indexOf('__wbg_load') !== -1) {
              linker.__wbindgen_placeholder__[imp.name] = (bitsIdx: number, idx: number) => {
                return Atomics.load(getObject(bitsIdx), idx);
              };
            } else if (imp.name.indexOf('__wbg_store') !== -1) {
              linker.__wbindgen_placeholder__[imp.name] = (
                bitsIdx: number,
                idx: number,
                val: number
              ) => {
                return Atomics.store(getObject(bitsIdx), idx, val);
              };
            } else if (imp.name.indexOf('__wbg_add') !== -1) {
              linker.__wbindgen_placeholder__[imp.name] = (
                bitsIdx: number,
                idx: number,
                val: number
              ) => {
                return Atomics.add(getObject(bitsIdx), idx, val);
              };
            } else if (imp.name.indexOf('__wbg_prototypesetcall') !== -1) {
              linker.__wbindgen_placeholder__[imp.name] = (
                fnIdx: number,
                thisArgIdx: number,
                ...args: any[]
              ) => {
                const fn = getObject(fnIdx);
                const obj = getObject(thisArgIdx);
                if (typeof fn !== 'function') {
                  console.error(
                    `[SystemStore] ${name}: prototypesetcall target is NOT a function (fnIdx: ${fnIdx}, objIdx: ${thisArgIdx})`,
                    { fn, obj, args, heapStats: heap.getStats() }
                  );
                  return addHeapObject(undefined);
                }
                try {
                  return addHeapObject(fn.call(obj, ...args));
                } catch (e) {
                  console.error(`[SystemStore] ${name}: prototypesetcall failed`, e);
                  return addHeapObject(undefined);
                }
              };
            } else if (imp.name.indexOf('__wbindgen_number_get') !== -1) {
              // Loose match for number_get (handles __wbg_ prefix)
              linker.__wbindgen_placeholder__[imp.name] = placeholders.__wbindgen_number_get;
            } else if (imp.name.indexOf('__wbindgen_throw') !== -1) {
              // Loose match for throw
              linker.__wbindgen_placeholder__[imp.name] = placeholders.__wbindgen_throw;
            } else if (imp.name.indexOf('__wbindgen_string_new') !== -1) {
              linker.__wbindgen_placeholder__[imp.name] = placeholders.__wbindgen_string_new;
            } else if (imp.name.indexOf('__wbindgen_is_undefined') !== -1) {
              linker.__wbindgen_placeholder__[imp.name] = placeholders.__wbindgen_is_undefined;
            } else if (imp.name.indexOf('__wbindgen_is_null') !== -1) {
              linker.__wbindgen_placeholder__[imp.name] = placeholders.__wbindgen_is_null;
            }
            // Log unhandled imports for awareness
            else if (!linker.__wbindgen_placeholder__[imp.name]) {
              // Check if we can find it in placeholders by relaxed suffix match
              // e.g. __wbg___wbindgen_is_null_dfda... -> __wbindgen_is_null
              const match = Object.keys(placeholders).find(k => imp.name.indexOf(k) !== -1);
              if (match) {
                linker.__wbindgen_placeholder__[imp.name] =
                  placeholders[match as keyof typeof placeholders];
              } else {
                console.warn(
                  `[SystemStore] Unhandled wasm-bindgen import: ${imp.name}. Stubbing with no-op.`
                );
                linker.__wbindgen_placeholder__[imp.name] = () => {};
              }
            }
          } else if (imp.module === '__wbindgen_externref_xform__') {
            // Ensure the module object exists in linker
            if (!linker.__wbindgen_externref_xform__) {
              linker.__wbindgen_externref_xform__ = {};
            }
            // Map externref functions
            if (
              placeholders.__wbindgen_externref_xform__ &&
              imp.name in placeholders.__wbindgen_externref_xform__
            ) {
              linker.__wbindgen_externref_xform__[imp.name] = (
                placeholders.__wbindgen_externref_xform__ as any
              )[imp.name];
            } else {
              // Default fallback for table ops
              linker.__wbindgen_externref_xform__[imp.name] = (...args: any[]) => args[0];
            }
          }
        });

        // Add stubs for constructors
        linker.__wbindgen_placeholder__.__wbg_new_8a6f238a6ece86ea = () => ({}); // Object
        linker.__wbindgen_placeholder__.__wbg_new_no_args_cb138f77cf6151ee = () => ({}); // Object
        linker.__wbindgen_placeholder__.__wbg_new_abda76e883ba8a5f = () => ({}); // Stack
        linker.__wbindgen_placeholder__.__wbg_new_16b304a2cfa7ff4a = () => ({}); // Error

        // 5. Provide SharedArrayBuffer to module
        // Modules are built with --import-memory and expect env.memory
        if (!linker.env) {
          linker.env = {};
        }
        linker.env.memory = sharedMemory; // Same SAB as kernel

        // 6. Instantiate with dynamic linker
        try {
          const result = await WebAssembly.instantiate(compiledModule, linker);
          exports = result.exports as any;
        } catch (error) {
          console.error(`[SystemStore] ❌ WASM instantiation failed for ${name}:`, error);
          console.error(`[SystemStore] Error details:`, {
            message: error instanceof Error ? error.message : String(error),
            stack: error instanceof Error ? error.stack : undefined,
          });
          throw error;
        }

        // Initialize module
        (window as any).__INOS_MODULE_ID__ = MODULE_IDS[name] || 0;
        console.log(`[SystemStore] Looking for init function: ${name}_init_with_sab`);
        const initFn = exports[`${name}_init_with_sab`] || exports.init_with_sab;
        console.log(`[SystemStore] Init function found:`, typeof initFn);
        if (typeof initFn === 'function') {
          const stats = heap.getStats();
          console.log(
            `[SystemStore] 🚀 Booting ${name} (Heap: ${stats.current}, Peak: ${stats.peak})`
          );
          console.log(`[SystemStore] Calling ${name}_init_with_sab()...`);
          console.log(`[SystemStore] Globals:`, {
            SAB: typeof (window as any).__INOS_SAB__,
            OFFSET: (window as any).__INOS_SAB_OFFSET__,
            SIZE: (window as any).__INOS_SAB_SIZE__,
            MODULE_ID: (window as any).__INOS_MODULE_ID__,
          });
          let success = 0;
          try {
            success = initFn();
          } catch (error) {
            console.error(`[SystemStore] ❌ Init threw exception for ${name}:`, error);
            success = 0;
          }
          console.log(`[SystemStore] Init function returned:`, success);
          if (!success) {
            console.warn(`[SystemStore] Module ${name} initialization reported failure`);
          } else {
            console.log(`[SystemStore] ✅ Module ${name} initialized with ID ${MODULE_IDS[name]}`);

            // Log capabilities after successful init
            // Architecture: Hash-based registry with CRC32C (threads.md line 148)
            try {
              const sabBase = (window as any).__INOS_SAB__;
              if (sabBase) {
                const view = new DataView(sabBase);

                // CRC32C hash (Castagnoli polynomial) - matches registry.rs:200-214
                const crc32c = (str: string): number => {
                  let crc = 0xffffffff;
                  for (let i = 0; i < str.length; i++) {
                    const byte = str.charCodeAt(i);
                    crc ^= byte;
                    for (let j = 0; j < 8; j++) {
                      if (crc & 1) {
                        crc = (crc >>> 1) ^ 0x82f63b78; // Castagnoli polynomial
                      } else {
                        crc >>>= 1;
                      }
                    }
                  }
                  return (crc ^ 0xffffffff) >>> 0;
                };

                const moduleHash = crc32c(name);
                console.log(
                  `[SystemStore] 📋 Scanning registry for ${name} (hash: 0x${moduleHash.toString(16)})`
                );

                // Scan registry (0x000100 - 0x001000, 64-byte entries, 60 inline capacity)
                // CRITICAL: Registry is at ABSOLUTE offset 0x000100 in the SAB
                // Modules write with global_sab (base_offset=0), so they write to absolute offsets
                const OFFSET_MODULE_REGISTRY = 0x000100;
                const MODULE_ENTRY_SIZE = 96; // Must match Rust: layout.rs line 28
                const MAX_MODULES_INLINE = 60;

                console.log(
                  `[SystemStore]   Registry at absolute: 0x${OFFSET_MODULE_REGISTRY.toString(16)}`
                );

                let foundSlot = -1;

                // Debug: Log first 10 slots and the claimed slot
                console.log(`[SystemStore]   Debug: First 10 registry slots:`);
                for (let i = 0; i < 10; i++) {
                  const offset = OFFSET_MODULE_REGISTRY + i * MODULE_ENTRY_SIZE;
                  const hash = view.getUint32(offset, true);
                  if (hash !== 0) {
                    console.log(`[SystemStore]     Slot ${i}: hash=0x${hash.toString(16)}`);
                  }
                }

                // Also check the slot where module claims to be (from log message)
                const claimedSlot = name === 'compute' ? 5 : name === 'science' ? 53 : -1;
                if (claimedSlot >= 0) {
                  const offset = OFFSET_MODULE_REGISTRY + claimedSlot * MODULE_ENTRY_SIZE;
                  const hash = view.getUint32(offset, true);
                  console.log(
                    `[SystemStore]   Debug: Claimed slot ${claimedSlot}: hash=0x${hash.toString(16)}`
                  );
                }

                for (let slot = 0; slot < MAX_MODULES_INLINE; slot++) {
                  const offset = OFFSET_MODULE_REGISTRY + slot * MODULE_ENTRY_SIZE;
                  // EnhancedModuleEntry: signature(8 bytes) + id_hash(4 bytes) at offset 8
                  const entryHash = view.getUint32(offset + 8, true); // id_hash at byte 8

                  if (entryHash === moduleHash) {
                    foundSlot = slot;
                    break;
                  }
                }

                if (foundSlot >= 0) {
                  const offset = OFFSET_MODULE_REGISTRY + foundSlot * MODULE_ENTRY_SIZE;
                  console.log(
                    `[SystemStore]   Found at slot ${foundSlot} (offset: 0x${offset.toString(16)})`
                  );

                  // Read entry structure - EnhancedModuleEntry field offsets:
                  // Verified from binary dump: cap_table_offset at byte 56, cap_count at byte 60
                  const capTableOffset = view.getUint32(offset + 56, true);
                  const capCount = view.getUint16(offset + 60, true);

                  console.log(
                    `[SystemStore]   Cap table offset: 0x${capTableOffset.toString(16)}, count: ${capCount}`
                  );

                  // Read capabilities from the table
                  const capabilities: string[] = [];
                  if (capTableOffset > 0 && capCount > 0) {
                    const globalSAB = (window as any).__INOS_SAB__;
                    const CAP_ENTRY_SIZE = 36; // CapabilityEntry: id[32] + min_memory_mb[2] + flags[1] + reserved[1]
                    console.log(
                      `[SystemStore]   Reading ${capCount} capabilities from 0x${capTableOffset.toString(16)}`
                    );
                    for (let i = 0; i < capCount; i++) {
                      // Memory barrier fixed - read from correct offset
                      const capOffset = capTableOffset + i * CAP_ENTRY_SIZE;
                      console.log(`[SystemStore]     Cap ${i}: offset=0x${capOffset.toString(16)}`);
                      // Read capability name (32 bytes, null-terminated)
                      const nameBytes = new Uint8Array(globalSAB, capOffset, 32);
                      console.log(
                        `[SystemStore]     Cap ${i}: hex:`,
                        Array.from(nameBytes.slice(0, 16))
                          .map(b => '0x' + b.toString(16).padStart(2, '0'))
                          .join(' ')
                      );
                      console.log(
                        `[SystemStore]     Cap ${i}: first 16 bytes:`,
                        Array.from(nameBytes.slice(0, 16))
                          .map(b => String.fromCharCode(b))
                          .join('')
                      );
                      let nameLen = 0;
                      while (nameLen < 32 && nameBytes[nameLen] !== 0) nameLen++;
                      const capName = new TextDecoder().decode(nameBytes.slice(0, nameLen));
                      console.log(`[SystemStore]     Cap ${i}: name="${capName}" (len=${nameLen})`);
                      if (capName) capabilities.push(capName);
                    }
                    console.log(`[SystemStore]   Capabilities:`, capabilities);
                    moduleCapabilities = capabilities; // Store for return
                  }
                } else {
                  console.warn(
                    `[SystemStore]   Module ${name} not found in registry (hash mismatch)`
                  );
                }
              }
            } catch (e) {
              console.warn(`[SystemStore] Could not read capabilities:`, e);
            }
          }
        } else {
          console.error(`[SystemStore] Init function not found for ${name}!`);
        }

        return { exports, capabilities: moduleCapabilities };
      };

      // Load all modules in parallel
      const modules = ['compute', 'science', 'ml', 'mining', 'vault', 'drivers'];
      const loadedModules: Record<string, any> = {};
      const moduleCapabilities: Record<string, string[]> = {}; // Store capabilities here

      // Load modules sequentially to avoid OOM/contention on shared memory
      for (const name of modules) {
        try {
          const result = await loadModule(name);
          loadedModules[name] = result.exports;
          moduleCapabilities[name] = result.capabilities || [];
        } catch (err) {
          console.error(`[SystemStore] Critical failure loading ${name}:`, err);
          throw err;
        }
      }

      // 5. Update state
      set({
        status: 'ready',
        units: {
          kernel: { id: 'kernel', active: true, capabilities: ['orchestration', 'mesh', 'gossip'] },
          ...Object.keys(loadedModules).reduce(
            (acc, name) => ({
              ...acc,
              [name]: { id: name, active: true, capabilities: moduleCapabilities[name] || [] },
            }),
            {}
          ),
        },
      });

      console.log('[SystemStore] ✅ INOS initialized successfully with all modules');
    } catch (error) {
      console.error('[SystemStore] Boot Error:', error);
      set({ status: 'error', error: error as Error });
    }
  },

  registerUnit: (unit: UnitState) => {
    set(state => ({
      units: {
        ...state.units,
        [unit.id]: unit,
      },
    }));
  },

  updateStats: (stats: Partial<KernelStats>) => {
    set(state => ({
      stats: {
        ...state.stats,
        ...stats,
      },
    }));
  },

  setError: (error: Error) => {
    set({ status: 'error', error });
  },
}));
