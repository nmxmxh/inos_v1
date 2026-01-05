/**
 * Production-grade WASM Heap implementation.
 * Standardized at indices 0-3 for wasm-bindgen compatibility.
 * Manages object lifecycle with high-performance free-list allocation.
 *
 * CRITICAL: NO COMPACTION - Indices are used as handles by WASM modules.
 */
export class WasmHeap {
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

    // Initialize free list (linked in objects array - no separate buffer)
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

      // Copy old objects
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

  // Log heap stats for debugging memory issues
  logStats(label: string = 'WasmHeap') {
    const stats = this.getStats();
    console.log(`[${label}] size=${stats.current} peak=${stats.peak} allocs=${stats.allocations}`);
  }
}
