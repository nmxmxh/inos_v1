import { WasmHeap } from './heap';

type GetBufferFn = () => ArrayBuffer;

// Cached decoder to avoid allocation on every call
const textDecoder = new TextDecoder();

export function createBaseEnv(heap: WasmHeap, getBuffer: GetBufferFn) {
  const addHeapObject = (obj: any) => heap.add(obj);
  const getObject = (idx: number) => heap.get(idx);

  return {
    // Logging
    host_log: (ptr: number, len: number, level: number) => {
      const buffer = getBuffer();
      if (!buffer || buffer.byteLength === 0) return;
      const view = new Uint8Array(buffer, ptr, len);
      const msg = textDecoder.decode(view); // Reuse cached decoder
      const methods = ['error', 'warn', 'info', 'debug', 'trace'];
      (console as any)[methods[level] || 'log'](msg);
    },

    inos_log: (ptr: number, len: number, level: number) => {
      const buffer = getBuffer();
      if (!buffer || buffer.byteLength === 0) return;
      const view = new Uint8Array(buffer, ptr, len);
      const msg = textDecoder.decode(view); // Reuse cached decoder
      const prefix = ['ERROR', 'WARN', 'INFO', 'DEBUG', 'TRACE'][level] || 'LOG';

      if (level === 0) console.warn(`[WASM-${prefix}] ${msg}`);
      else if (level <= 2) console.log(`[WASM-${prefix}] ${msg}`);
      else console.debug(`[WASM-${prefix}] ${msg}`);
    },

    // Array creation
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
      const buffer = getBuffer();
      if (!buffer || buffer.byteLength === 0) return addHeapObject('');
      const view = new Uint8Array(buffer, ptr, len);
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
      const res = Atomics.wait(arr, index, value, timeout === -1 ? undefined : timeout);
      return res === 'ok' ? 0 : res === 'not-equal' ? 1 : 2;
    },

    inos_atomic_compare_exchange: (
      typedArrayIdx: number,
      index: number,
      expected: number,
      replacement: number
    ) => {
      return Atomics.compareExchange(getObject(typedArrayIdx), index, expected, replacement);
    },

    // Math
    inos_math_random: () => Math.random(),

    // Memory operations
    inos_copy_to_sab: (
      targetBufferIdx: number,
      targetOffset: number,
      srcPtr: number,
      len: number
    ) => {
      const buffer = getBuffer();
      const targetBuffer = getObject(targetBufferIdx);
      if (!buffer || !targetBuffer) return;
      const src = new Uint8Array(buffer, srcPtr, len);
      const dest = new Uint8Array(targetBuffer, targetOffset, len);
      dest.set(src);
    },

    inos_copy_from_sab: (srcBufferIdx: number, srcOffset: number, destPtr: number, len: number) => {
      const buffer = getBuffer();
      const srcBuffer = getObject(srcBufferIdx);
      if (!buffer || !srcBuffer) return;
      const src = new Uint8Array(srcBuffer, srcOffset, len);
      const dest = new Uint8Array(buffer, destPtr, len);
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
      const { written } = new TextEncoder().encodeInto(obj, dest);
      return written;
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
