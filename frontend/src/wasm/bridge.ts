import { WasmHeap } from './heap';
import { getDataView, getFlagsView, getOffset, isReady } from './bridge-state';

type GetBufferFn = () => ArrayBuffer;

// Cached encoder/decoder to avoid allocation on every call
const textDecoder = new TextDecoder();
const textEncoder = new TextEncoder();

const LOG_METHODS = ['error', 'warn', 'info', 'debug', 'trace'];
const LOG_PREFIXES = ['ERROR', 'WARN', 'INFO', 'DEBUG', 'TRACE'];

let globalLogLevel = 1; // Default to INFO (1)

export function setGlobalLogLevel(level: number) {
  globalLogLevel = level;
}

export function createBaseEnv(heap: WasmHeap, getBuffer: GetBufferFn) {
  const addHeapObject = (obj: any) => heap.add(obj);
  const getObject = (idx: number) => heap.get(idx);

  // View cache to prevent object churn
  const viewCache = new Map<string, any>();
  const viewCacheKeys: string[] = [];
  const MAX_VIEW_CACHE = 500;
  let lastBuffer: ArrayBuffer | null = null;

  function getCachedView(type: any, offset: number, len: number) {
    const buffer = getBuffer();
    if (buffer !== lastBuffer) {
      viewCache.clear();
      lastBuffer = buffer;
    }
    // Optimization: Use a combined numeric key for common types to avoid string concatenation
    // Assuming offset < 2^32 and len < 2^32, we can't easily fit in 53-bit integer if concatenated.
    // But we can use nested Maps or a simple string for now, but pre-calculating type index.
    const typeIdx =
      type === Uint8Array ? 0 : type === Int32Array ? 1 : type === Float32Array ? 2 : 3;
    const key = `${typeIdx}:${offset}:${len}`;
    let view = viewCache.get(key);
    if (!view) {
      // CAUTION: Cap viewCache to prevent memory leaks from excessive sub-views
      if (viewCacheKeys.length >= MAX_VIEW_CACHE) {
        const oldest = viewCacheKeys.shift()!;
        viewCache.delete(oldest);
      }
      view = new type(buffer, offset, len);
      viewCache.set(key, view);
      viewCacheKeys.push(key);
    } else {
      // LRU Update: Move to end
      const idx = viewCacheKeys.indexOf(key);
      if (idx > -1) {
        viewCacheKeys.splice(idx, 1);
        viewCacheKeys.push(key);
      }
    }
    return view;
  }

  return {
    // Logging
    host_log: (ptr: number, len: number, level: number) => {
      if (level > globalLogLevel) return; // Skip if level is too high (verbose)
      const view = getCachedView(Uint8Array, ptr, len);
      const msg = textDecoder.decode(view);
      (console as any)[LOG_METHODS[level] || 'log'](msg);
    },

    inos_log: (ptr: number, len: number, level: number) => {
      if (level > globalLogLevel) return; // Skip filtered logs
      const view = getCachedView(Uint8Array, ptr, len);
      const msg = textDecoder.decode(view);
      const prefix = LOG_PREFIXES[level] || 'LOG';

      if (level === 0) console.warn(`[WASM-${prefix}] ${msg}`);
      else if (level <= 2) console.log(`[WASM-${prefix}] ${msg}`);
      else console.debug(`[WASM-${prefix}] ${msg}`);
    },

    // Array creation
    inos_create_u8_array: (ptr: number, len: number) => {
      return addHeapObject(getCachedView(Uint8Array, ptr, len));
    },

    inos_wrap_u8_array: (valIdx: number) => {
      const val = getObject(valIdx);
      return addHeapObject(new Uint8Array(val));
    },

    inos_create_u8_view: (bufferIdx: number, offset: number, len: number) => {
      const buffer = getObject(bufferIdx);
      return addHeapObject(new Uint8Array(buffer, offset, len)); // Not cached as it's a dynamic buffer object
    },

    inos_create_i32_view: (bufferIdx: number, offset: number, len: number) => {
      const buffer = getObject(bufferIdx);
      return addHeapObject(new Int32Array(buffer, offset, len)); // Not cached
    },

    inos_create_sab: (len: number) => {
      return addHeapObject(new SharedArrayBuffer(len));
    },

    // Global access
    inos_get_global: () => addHeapObject(globalThis),

    inos_reflect_get: (targetIdx: number, keyIdx: number) => {
      const target = getObject(targetIdx);
      const key = getObject(keyIdx);

      if (target === undefined || target === null) {
        return 0; // Return primitive heap index for undefined
      }

      try {
        const result = Reflect.get(target, key);
        if (result === undefined) return 0; // Primitive index for undefined
        if (result === null) return 1; // Primitive index for null
        return addHeapObject(result);
      } catch (e) {
        return 0; // Primitive index for undefined
      }
    },

    inos_as_f64: (valIdx: number) => {
      const val = getObject(valIdx);
      return typeof val === 'number' ? val : Number(val);
    },

    // Strings
    inos_create_string: (ptr: number, len: number) => {
      const view = getCachedView(Uint8Array, ptr, len);
      const str = textDecoder.decode(view); // Reuse cached decoder
      return addHeapObject(str);
    },

    // Time
    inos_get_now: () => Date.now(),
    inos_get_performance_now: () => performance.now(),

    // Atomics
    inos_atomic_add: (typedArrayIdx: number, index: number, value: number) => {
      return Atomics.add(getObject(typedArrayIdx), index, value);
    },

    inos_atomic_load: (typedArrayIdx: number, index: number) => {
      return Atomics.load(getObject(typedArrayIdx), index);
    },

    inos_atomic_store: (typedArrayIdx: number, index: number, value: number) => {
      return Atomics.store(getObject(typedArrayIdx), index, value);
    },

    inos_atomic_wait: (typedArrayIdx: number, index: number, value: number, timeout: number) => {
      const arr = getObject(typedArrayIdx);
      try {
        // NOTE: Atomics.wait() is NOT allowed on the main thread in Safari/iOS.
        // If we're running on main thread fallback, this will throw TypeError.
        const res = Atomics.wait(arr, index, value, timeout === -1 ? undefined : timeout);
        return res === 'ok' ? 0 : res === 'not-equal' ? 1 : 2;
      } catch (e) {
        // Safari/iOS main thread: "TypeError: Atomics.wait cannot be called in this context"
        // Return 2 (timeout) to trigger polling fallback in Go kernel
        console.warn('[Bridge] Atomics.wait not available (main thread?), using polling fallback');
        return 2;
      }
    },

    inos_atomic_compare_exchange: (
      typedArrayIdx: number,
      index: number,
      expected: number,
      replacement: number
    ) => {
      return Atomics.compareExchange(getObject(typedArrayIdx), index, expected, replacement);
    },

    inos_atomic_notify: (typedArrayIdx: number, index: number, count: number) => {
      return Atomics.notify(getObject(typedArrayIdx), index, count);
    },

    // Math
    inos_math_random: () => Math.random(),

    inos_fill_random: (ptr: number, len: number) => {
      const view = getCachedView(Uint8Array, ptr, len);
      const cryptoObj = (globalThis as any).crypto;
      if (cryptoObj && typeof cryptoObj.getRandomValues === 'function') {
        cryptoObj.getRandomValues(view);
      } else {
        for (let i = 0; i < len; i++) {
          view[i] = Math.floor(Math.random() * 256);
        }
      }
    },

    // Memory operations
    inos_copy_to_sab: (
      targetBufferIdx: number,
      targetOffset: number,
      srcPtr: number,
      len: number
    ) => {
      const src = getCachedView(Uint8Array, srcPtr, len);
      const targetBuffer = getObject(targetBufferIdx);
      if (!targetBuffer) return;
      const dest = new Uint8Array(targetBuffer, targetOffset, len);
      dest.set(src);
    },

    inos_copy_from_sab: (srcBufferIdx: number, srcOffset: number, destPtr: number, len: number) => {
      const srcBuffer = getObject(srcBufferIdx);
      if (!srcBuffer) return;
      const src = new Uint8Array(srcBuffer, srcOffset, len);
      const dest = getCachedView(Uint8Array, destPtr, len);
      dest.set(src);
    },

    inos_get_byte_length: (idx: number) => {
      const obj = getObject(idx);
      if (obj && typeof obj.byteLength === 'number') return obj.byteLength;
      if (obj && typeof obj.length === 'number') return obj.length;
      return 0;
    },

    inos_js_to_string: (idx: number, ptr: number, maxLen: number) => {
      const obj = getObject(idx);
      if (typeof obj !== 'string') return 0;
      const buffer = getBuffer();
      // Use encodeInto for zero-copy write directly to WASM memory
      const dest = new Uint8Array(buffer, ptr, maxLen);
      const { written } = textEncoder.encodeInto(obj, dest);
      return written;
    },

    // =============================================================================
    // TYPED SAB ACCESSORS (Zero-Copy via INOSBridge)
    // =============================================================================

    inos_sab_read_i32: (byteOffset: number) => {
      if (!isReady()) return 0;
      const view = getDataView();
      const offset = getOffset();
      if (!view) return 0;
      return view.getInt32(offset + byteOffset, true);
    },

    inos_sab_read_u32: (byteOffset: number) => {
      if (!isReady()) return 0;
      const view = getDataView();
      const offset = getOffset();
      if (!view) return 0;
      return view.getUint32(offset + byteOffset, true);
    },

    inos_sab_read_f32: (byteOffset: number) => {
      if (!isReady()) return 0;
      const view = getDataView();
      const offset = getOffset();
      if (!view) return 0;
      return view.getFloat32(offset + byteOffset, true);
    },

    inos_sab_atomic_load: (index: number) => {
      if (!isReady()) return 0;
      const flags = getFlagsView();
      if (!flags) return 0;
      return Atomics.load(flags, index);
    },
  };
}

export function createPlaceholders(heap: WasmHeap, getBuffer: GetBufferFn) {
  const addHeapObject = (obj: any) => heap.add(obj);
  const getObject = (idx: number) => heap.get(idx);
  const dropObject = (idx: number) => heap.drop(idx);

  return {
    __wbindgen_throw: (ptr: number, len: number) => {
      const buffer = getBuffer();
      if (!buffer || buffer.byteLength === 0) throw new Error('WASM memory not ready');
      const view = new Uint8Array(buffer, ptr, len);
      const msg = textDecoder.decode(view); // Reuse cached decoder
      throw new Error(`WASM panic: ${msg}`);
    },

    __wbindgen_number_get: (idx: number) => {
      const val = getObject(idx);
      return typeof val === 'number' ? val : NaN;
    },

    __wbindgen_string_new: (ptr: number, len: number) => {
      const buffer = getBuffer();
      if (!buffer || buffer.byteLength === 0) return addHeapObject('');
      const view = new Uint8Array(buffer, ptr, len);
      const str = textDecoder.decode(view); // Reuse cached decoder
      return addHeapObject(str);
    },

    __wbindgen_object_drop_ref: (idx: number) => dropObject(idx),
    __wbindgen_object_clone_ref: (idx: number) => addHeapObject(getObject(idx)),
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

    __wbindgen_describe: (_v: number) => {},
    __wbindgen_describe_cast: (_a: number, _b: number) => {},
    __wbindgen_debug_string: (vIdx: number, _lenPtr: number) => {
      const val = getObject(vIdx);
      console.log(`[WASM-Debug] ${JSON.stringify(val)}`);
    },
  };
}
