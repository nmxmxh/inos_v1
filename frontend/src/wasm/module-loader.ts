/**
 * Module loader for INOS Rust WASM modules.
 * Handles compilation, import bridge construction, and initialization.
 */

import { WasmHeap } from './heap';
import { createBaseEnv, createPlaceholders } from './bridge';

declare global {
  interface Window {
    __INOS_MODULE_ID__: number;
    inosModules?: Record<string, any>;
  }
}

const MODULE_IDS: Record<string, number> = {
  compute: 1,
  vault: 2,
  drivers: 3,
  diagnostics: 4,
};

export interface ModuleLoadResult {
  exports: any;
  capabilities: string[];
}

export async function loadModule(
  name: string,
  sharedMemory: WebAssembly.Memory
): Promise<ModuleLoadResult> {
  const response = await fetch(`/modules/${name}.wasm?t=${Date.now()}`);
  if (!response.ok) throw new Error(`Failed to load ${name}.wasm`);

  const bytes = await response.arrayBuffer();
  const compiledModule = await WebAssembly.compile(bytes);
  const imports = WebAssembly.Module.imports(compiledModule);

  // Setup heap and memory access
  const heap = new WasmHeap();
  const addHeapObject = (obj: any) => heap.add(obj);
  const getObject = (idx: number) => heap.get(idx);

  let exports: any;

  const getBuffer = () => {
    if (exports && exports.memory) {
      return exports.memory.buffer;
    }
    return sharedMemory.buffer;
  };

  // Build import object
  const baseEnv = createBaseEnv(heap, getBuffer);
  const placeholders = createPlaceholders(heap, getBuffer);

  const linker: any = {
    env: {
      ...baseEnv,
      memory: sharedMemory,
    },
    __wbindgen_placeholder__: {},
  };

  // Dynamic import mapping
  imports.forEach(imp => {
    if (imp.module === '__wbindgen_placeholder__') {
      if (placeholders[imp.name as keyof typeof placeholders]) {
        linker.__wbindgen_placeholder__[imp.name] =
          placeholders[imp.name as keyof typeof placeholders];
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
      linker.__wbindgen_externref_xform__[imp.name] = (...args: any[]) => args[0];
    }
  });

  // Add common stubs
  linker.__wbindgen_placeholder__.__wbg_new_8a6f238a6ece86ea = () => ({});
  linker.__wbindgen_placeholder__.__wbg_new_no_args_cb138f77cf6151ee = () => ({});
  linker.__wbindgen_placeholder__.__wbg_new_abda76e883ba8a5f = () => ({});
  linker.__wbindgen_placeholder__.__wbg_new_16b304a2cfa7ff4a = () => ({});

  // Instantiate module
  const result = await WebAssembly.instantiate(compiledModule, linker);
  exports = result.exports as any;

  // Initialize module
  window.__INOS_MODULE_ID__ = MODULE_IDS[name] || 0;
  const initFn = exports[`${name}_init_with_sab`] || exports.init_with_sab;

  if (typeof initFn === 'function') {
    const success = initFn();
    if (!success) {
      console.warn(`[ModuleLoader] ${name} initialization reported failure`);
    }
  }

  return {
    exports,
    capabilities: [], // Capabilities will be read from registry
  };
}

function handleNewImport(
  name: string,
  linker: any,
  getObject: (idx: number) => any,
  addHeapObject: (obj: any) => number,
  getBuffer: () => ArrayBuffer
) {
  if (name.indexOf('new_with_byte_offset_and_length') !== -1) {
    linker.__wbindgen_placeholder__[name] = (bufferIdx: number, offset: number, len: number) => {
      const buffer = getObject(bufferIdx);
      return addHeapObject(new Uint8Array(buffer, offset, len));
    };
  } else if (name.indexOf('new_from_slice') !== -1) {
    linker.__wbindgen_placeholder__[name] = (ptr: number, len: number) => {
      return addHeapObject(new Uint8Array(getBuffer(), ptr, len));
    };
  } else if (name.match(/int32array_new/i)) {
    linker.__wbindgen_placeholder__[name] = (arg0: any, arg1: any, arg2: any) => {
      return addHeapObject(new Int32Array(getObject(arg0), arg1, arg2));
    };
  } else if (name.match(/uint8array_new/i)) {
    linker.__wbindgen_placeholder__[name] = (arg0: any) => {
      return addHeapObject(new Uint8Array(getObject(arg0)));
    };
  } else {
    linker.__wbindgen_placeholder__[name] = () => addHeapObject(new Object());
  }
}

function handleWbgImport(
  name: string,
  linker: any,
  getObject: (idx: number) => any,
  addHeapObject: (obj: any) => number
) {
  if (name.indexOf('byteLength') !== -1) {
    linker.__wbindgen_placeholder__[name] = (idx: number) => getObject(idx).byteLength;
  } else if (name.indexOf('length') !== -1) {
    linker.__wbindgen_placeholder__[name] = (idx: number) => getObject(idx).length;
  } else if (name.indexOf('subarray') !== -1) {
    linker.__wbindgen_placeholder__[name] = (idx: number, a: number, b: number) => {
      return addHeapObject(getObject(idx).subarray(a, b));
    };
  } else if (name.indexOf('set') !== -1) {
    linker.__wbindgen_placeholder__[name] = (idx: number, valIdx: number, off: number) => {
      getObject(idx).set(getObject(valIdx), off);
    };
  } else if (name.indexOf('load') !== -1) {
    linker.__wbindgen_placeholder__[name] = (bitsIdx: number, idx: number) => {
      return Atomics.load(getObject(bitsIdx), idx);
    };
  } else if (name.indexOf('store') !== -1) {
    linker.__wbindgen_placeholder__[name] = (bitsIdx: number, idx: number, val: number) => {
      return Atomics.store(getObject(bitsIdx), idx, val);
    };
  } else if (name.indexOf('add') !== -1) {
    linker.__wbindgen_placeholder__[name] = (bitsIdx: number, idx: number, val: number) => {
      return Atomics.add(getObject(bitsIdx), idx, val);
    };
  } else {
    linker.__wbindgen_placeholder__[name] = () => {};
  }
}

export async function loadAllModules(
  sharedMemory: WebAssembly.Memory
): Promise<Record<string, any>> {
  // Singleton check
  if (window.inosModules) {
    console.log('[ModuleLoader] Reusing existing modules singleton');
    return window.inosModules;
  }

  const moduleNames = ['compute', 'diagnostics']; // drivers and vault are now lazy-loaded
  const loadedModules: Record<string, any> = {};

  for (const name of moduleNames) {
    try {
      const result = await loadModule(name, sharedMemory);
      loadedModules[name] = result.exports;
    } catch (err) {
      console.error(`[ModuleLoader] Failed to load ${name}:`, err);
      throw err;
    }
  }

  window.inosModules = loadedModules;
  return loadedModules;
}
