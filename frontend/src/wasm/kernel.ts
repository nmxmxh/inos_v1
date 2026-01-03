/**
 * Kernel initialization logic for INOS Go WASM kernel.
 * Handles loading wasm_exec.js, creating SharedArrayBuffer, and instantiating the kernel.
 */

declare global {
  interface Window {
    Go: any;
    __INOS_SAB__: SharedArrayBuffer;
    __INOS_SAB_OFFSET__: number;
    __INOS_SAB_SIZE__: number;
    getSystemSABAddress?: () => number;
    getSystemSABSize?: () => number;
  }
}

export interface KernelInitResult {
  memory: WebAssembly.Memory;
  sabBase: SharedArrayBuffer;
  sabOffset: number;
  sabSize: number;
}

export async function initializeKernel(): Promise<KernelInitResult> {
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
  const sharedMemory = new WebAssembly.Memory({
    initial: 256, // 16MB
    maximum: 1024, // 64MB max
    shared: true,
  });

  // 3. Load and instantiate Go kernel
  const go = new window.Go();
  const response = await fetch('/kernel.wasm');

  if (!response.ok) {
    throw new Error(`Failed to load kernel.wasm: ${response.statusText}`);
  }

  const wasmBytes = await response.arrayBuffer();
  const result = await WebAssembly.instantiate(wasmBytes, {
    ...go.importObject,
    env: {
      ...go.importObject.env,
      memory: sharedMemory,
    },
  });

  go.run(result.instance);

  // 4. Wait for Kernel to export SAB functions
  const maxWaitMs = 5000;
  const startTime = Date.now();

  while (!window.getSystemSABAddress || !window.getSystemSABSize) {
    if (Date.now() - startTime > maxWaitMs) {
      break;
    }
    await new Promise(resolve => setTimeout(resolve, 10));
  }

  // 5. Setup SharedArrayBuffer globals
  const memoryBuffer = sharedMemory.buffer;

  if (!(memoryBuffer instanceof SharedArrayBuffer)) {
    throw new Error('WebAssembly.Memory.buffer is not a SharedArrayBuffer');
  }

  const sabBase = memoryBuffer as SharedArrayBuffer;
  let sabOffset = 0;
  let sabSize = sabBase.byteLength;

  if (window.getSystemSABAddress && window.getSystemSABSize) {
    sabOffset = window.getSystemSABAddress();
    sabSize = window.getSystemSABSize();
  }

  window.__INOS_SAB__ = sabBase;
  window.__INOS_SAB_OFFSET__ = sabOffset;
  window.__INOS_SAB_SIZE__ = sabSize;

  return {
    memory: sharedMemory,
    sabBase,
    sabOffset,
    sabSize,
  };
}
