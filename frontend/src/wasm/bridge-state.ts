/**
 * INOS Bridge State Manager
 *
 * Centralized, zero-allocation access to SharedArrayBuffer.
 * All views are cached once at initialization and reused across the application.
 *
 * This eliminates per-hook DataView/TypedArray allocations that cause GC pressure.
 */

// =============================================================================
// SINGLETON STATE
// =============================================================================

let _sab: SharedArrayBuffer | null = null;
let _memory: WebAssembly.Memory | null = null;
let _offset: number = 0;
let _size: number = 0;

// Cached views - created once, reused forever
let _dataView: DataView | null = null;
let _flagsView: Int32Array | null = null;
let _floatsView: Float32Array | null = null;

// Region-specific views (cached on first access)
const _regionViews = new Map<string, DataView>();
const _regionFloat32Views = new Map<string, Float32Array>();

// =============================================================================
// INITIALIZATION
// =============================================================================

/**
 * Initialize the bridge with the SharedArrayBuffer.
 * Called once from kernel.ts after SAB is established.
 */
export function initializeBridge(
  sab: SharedArrayBuffer,
  offset: number,
  size: number,
  memory?: WebAssembly.Memory
): void {
  if (_sab === sab && _offset === offset && _size === size && _memory === memory) {
    return; // Already initialized with same params
  }

  _sab = sab;
  _memory = memory || null;
  _offset = offset;
  _size = size;

  // Create master views
  // CRITICAL: flagsView MUST start at byte 0 to match Rust SafeSAB::barrier_view()
  // Rust writes epoch flips to absolute Int32 indices (e.g., IDX_MATRIX_EPOCH=13)
  // If we use _offset here, we'd read from wrong memory location
  _dataView = new DataView(sab);
  _flagsView = new Int32Array(sab, 0, 128); // Absolute byte 0, matches Rust
  _floatsView = new Float32Array(sab);

  // Clear cached region views (they'll be recreated on demand)
  _regionViews.clear();
  _regionFloat32Views.clear();

  console.log('[INOSBridge] Initialized with cached views');
}

/**
 * Clear all cached views (used during HMR or kernel reboot)
 */
export function clearBridge(): void {
  _sab = null;
  _memory = null;
  _offset = 0;
  _size = 0;
  _dataView = null;
  _flagsView = null;
  _floatsView = null;
  _regionViews.clear();
  _regionFloat32Views.clear();
}

// =============================================================================
// ACCESSORS
// =============================================================================

/**
 * Check if the bridge is ready
 */
export function isReady(): boolean {
  return _sab !== null && _dataView !== null && _flagsView !== null;
}

/**
 * Get the raw SharedArrayBuffer (for advanced use cases)
 */
export function getSAB(): SharedArrayBuffer | null {
  return _sab;
}

/**
 * Get the WebAssembly.Memory instance (if available)
 */
export function getMemory(): WebAssembly.Memory | null {
  return _memory;
}

/**
 * Get the kernel offset
 */
export function getOffset(): number {
  return _offset;
}

/**
 * Get the cached master DataView
 */
export function getDataView(): DataView | null {
  return _dataView;
}

/**
 * Get the cached atomic flags view (Int32Array)
 */
export function getFlagsView(): Int32Array | null {
  return _flagsView;
}

/**
 * Get the cached floats view
 */
export function getFloatsView(): Float32Array | null {
  return _floatsView;
}

// =============================================================================
// TYPED READS (Zero-Allocation)
// =============================================================================

/**
 * Read a 32-bit signed integer at the given byte offset (relative to kernel offset)
 */
export function readI32(byteOffset: number): number {
  if (!_dataView) return 0;
  return _dataView.getInt32(_offset + byteOffset, true);
}

/**
 * Read a 32-bit unsigned integer at the given byte offset (relative to kernel offset)
 */
export function readU32(byteOffset: number): number {
  if (!_dataView) return 0;
  return _dataView.getUint32(_offset + byteOffset, true);
}

/**
 * Read a 32-bit float at the given byte offset (relative to kernel offset)
 */
export function readF32(byteOffset: number): number {
  if (!_dataView) return 0;
  return _dataView.getFloat32(_offset + byteOffset, true);
}

/**
 * Read a 64-bit unsigned integer at the given byte offset (relative to kernel offset)
 */
export function readU64(byteOffset: number): bigint {
  if (!_dataView) return BigInt(0);
  return _dataView.getBigUint64(_offset + byteOffset, true);
}

/**
 * Read a 64-bit unsigned integer as a Number (loses precision for large values)
 */
export function readU64AsNumber(byteOffset: number): number {
  if (!_dataView) return 0;
  return Number(_dataView.getBigUint64(_offset + byteOffset, true));
}

// =============================================================================
// ATOMIC OPERATIONS (Zero-Allocation)
// =============================================================================

/**
 * Atomically load a value from the flags region
 */
export function atomicLoad(index: number): number {
  if (!_flagsView) return 0;
  return Atomics.load(_flagsView, index);
}

// =============================================================================
// REGION VIEWS (Cached)
// =============================================================================

/**
 * Get a cached DataView for a specific region.
 * Region views are created once and reused.
 */
export function getRegionDataView(regionOffset: number, regionSize: number): DataView | null {
  if (!_sab) return null;

  const key = `${regionOffset}:${regionSize}`;
  let view = _regionViews.get(key);
  if (!view) {
    view = new DataView(_sab, _offset + regionOffset, regionSize);
    _regionViews.set(key, view);
  }
  return view;
}

/**
 * Get a cached Float32Array view for a region (used for matrix data)
 */
export function getRegionFloat32View(byteOffset: number, floatCount: number): Float32Array | null {
  if (!_sab) return null;

  const key = `f32:${byteOffset}:${floatCount}`;
  let view = _regionFloat32Views.get(key);
  if (!view) {
    view = new Float32Array(_sab, byteOffset, floatCount);
    _regionFloat32Views.set(key, view);
  }
  return view;
}

// =============================================================================
// CONVENIENCE EXPORTS
// =============================================================================

/**
 * The INOSBridge namespace object for cleaner imports
 */
export const INOSBridge = {
  initialize: initializeBridge,
  clear: clearBridge,
  isReady,
  getSAB,
  getMemory,
  getOffset,
  getDataView,
  getFlagsView,
  getFloatsView,
  readI32,
  readU32,
  readF32,
  readU64,
  readU64AsNumber,
  atomicLoad,
  /**
   * Observe an epoch change (polling-friendly version of atomicLoad)
   */
  getEpoch: (index: number) => atomicLoad(index),
  /**
   * Peek at the Outbox without advancing the head pointer.
   */
  peekOutbox: (len: number = 4096) => {
    // Data starts after 8-byte header (head, tail)
    return getRegionDataView(0x0d0008, len);
  },
  /**
   * Pop a result from the Outbox ringbuffer.
   * This handles the head/tail pointers and wrap-around.
   */
  popResult: (): Uint8Array | null => {
    if (!_dataView || !_sab) return null;

    const outboxBase = _offset + 0x0d0000;
    const regionSize = 0x080000; // SIZE_OUTBOX_TOTAL
    const headerSize = 8;
    const dataCapacity = regionSize - headerSize;

    // 1. Read metadata
    const head = _dataView.getUint32(outboxBase, true);
    const tail = _dataView.getUint32(outboxBase + 4, true);

    if (head === tail) return null;

    // 2. Read message length
    // We need to handle potential wrap-around for the length field itself
    // but the system ensures message length (4 bytes) is never split if possible,
    // or we handle it via a helper.
    const readRaw = (idx: number, len: number): Uint8Array => {
      const res = new Uint8Array(len);
      const dataBase = outboxBase + headerSize;
      const firstChunk = dataCapacity - idx;
      if (len <= firstChunk) {
        res.set(new Uint8Array(_sab!, dataBase + idx, len));
      } else {
        res.set(new Uint8Array(_sab!, dataBase + idx, firstChunk));
        res.set(new Uint8Array(_sab!, dataBase, len - firstChunk), firstChunk);
      }
      return res;
    };

    const lenBytes = readRaw(head, 4);
    const msgLen = new DataView(lenBytes.buffer).getUint32(0, true);

    if (msgLen === 0 || msgLen > dataCapacity) {
      console.warn('[INOSBridge] Invalid msgLen in outbox:', msgLen);
      return null;
    }

    // 3. Read payload
    const dataHead = (head + 4) % dataCapacity;
    const payload = readRaw(dataHead, msgLen);

    // 4. Advance head
    const nextHead = (dataHead + msgLen) % dataCapacity;
    Atomics.store(_flagsView!, 16, 1); // IDX_OUTBOX_MUTEX (Simple lock)
    _dataView.setUint32(outboxBase, nextHead, true);
    Atomics.store(_flagsView!, 16, 0);

    return payload;
  },
  getRegionDataView,
  getRegionFloat32View,
};

export default INOSBridge;
