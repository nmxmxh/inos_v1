/**
 * Production-grade WASM Heap implementation.
 * Standardized at indices 0-3 for wasm-bindgen compatibility.
 * Manages object lifecycle with high-performance free-list allocation.
 *
 * CRITICAL: NO COMPACTION - Indices are used as handles by WASM modules.
 * INOS Optimization: Universal Object Interning with reference counting.
 */
export class WasmHeap {
  private objects: any[];
  private nextFree: number | undefined;
  private peakUsage: number = 0;
  private totalAllocations: number = 0;
  private totalDrops: number = 0;
  private lastGcTime: number = 0;

  // Universal Object Cache for reference counting and deduplication
  private objectCache = new Map<any, { idx: number; refs: number }>();

  constructor(initialCapacity: number = 256) {
    // Standard wasm-bindgen primitives
    this.objects = new Array(initialCapacity);
    this.objects[0] = undefined;
    this.objects[1] = null;
    this.objects[2] = true;
    this.objects[3] = false;

    // Initialize free list (linked in objects array - no separate buffer)
    for (let i = 4; i < initialCapacity - 1; i++) {
      this.objects[i] = i + 1;
    }
    this.objects[initialCapacity - 1] = undefined;
    this.nextFree = 4;
    this.peakUsage = 4;
  }

  /**
   * Add object to heap with Universal Interning.
   * Non-primitives are deduplicated and reference-counted.
   */
  add(obj: any): number {
    // 1. Handle primitives (Immortal in INOS)
    if (obj === undefined) return 0;
    if (obj === null) return 1;
    if (obj === true) return 2;
    if (obj === false) return 3;

    // 2. Interning for all non-primitive types
    const type = typeof obj;
    const isObject = type === 'object' || type === 'function' || type === 'string';

    if (isObject && obj !== null) {
      const entry = this.objectCache.get(obj);
      if (entry) {
        entry.refs++;
        return entry.idx;
      }
    }

    // 3. Allocate new index
    if (this.nextFree === undefined) {
      this.grow();
    }

    const idx = this.nextFree!;
    this.nextFree = this.objects[idx];
    this.objects[idx] = obj;
    this.totalAllocations++;

    if (idx + 1 > this.peakUsage) this.peakUsage = idx + 1;

    // 4. Register in interning cache
    if (isObject && obj !== null) {
      this.objectCache.set(obj, { idx, refs: 1 });
    }

    return idx;
  }

  /**
   * Periodic GC for the interning cache.
   * Removes objects that have 0 references but haven't been reused.
   */
  gc(): void {
    const now = Date.now();
    if (now - this.lastGcTime < 2000) return; // Throttled GC

    let collected = 0;
    for (const [obj, entry] of this.objectCache.entries()) {
      if (entry.refs <= 0) {
        this.objectCache.delete(obj);
        collected++;
      }
    }

    if (collected > 0) {
      console.log(`[WasmHeap] GC collected ${collected} interned objects`);
    }
    this.lastGcTime = now;
  }

  private grow(): void {
    const oldLen = this.objects.length;
    const newLen = oldLen * 2;

    // Copy logic
    const newObjects = new Array(newLen);
    for (let i = 0; i < oldLen; i++) newObjects[i] = this.objects[i];

    // Build new free list
    for (let i = oldLen; i < newLen - 1; i++) newObjects[i] = i + 1;
    newObjects[newLen - 1] = undefined;

    this.objects = newObjects;
    this.nextFree = oldLen;
  }

  get(idx: number): any {
    return this.objects[idx];
  }

  /**
   * Release object from heap with reference counting.
   */
  drop(idx: number): void {
    if (idx < 4) return; // Primitives are immortal

    const obj = this.objects[idx];

    // Ref-counted cleanup for interned items
    const type = typeof obj;
    if ((type === 'object' || type === 'function' || type === 'string') && obj !== null) {
      const entry = this.objectCache.get(obj);
      if (entry) {
        entry.refs--;
        if (entry.refs > 0) return; // Still has other handles in use
        this.objectCache.delete(obj);
      }
    }

    // Release reference for GC and link to free list
    this.objects[idx] = this.nextFree;
    this.nextFree = idx;
    this.totalDrops++;

    // Trigger throttled GC on large drops
    if (this.totalDrops % 100 === 0) {
      this.gc();
    }
  }

  getStats() {
    return {
      current: this.objects.length,
      peak: this.peakUsage,
      allocations: this.totalAllocations,
      drops: this.totalDrops,
      interned: this.objectCache.size,
    };
  }

  logStats(label: string = 'WasmHeap') {
    const s = this.getStats();
    console.log(
      `[${label}] size=${s.current} peak=${s.peak} allocs=${s.allocations} drops=${s.drops} interned=${s.interned}`
    );
  }
}
