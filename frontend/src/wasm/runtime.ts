export interface RuntimeCapabilities {
  sharedMemory: boolean;
  waitAsync: boolean;
  reason?: string;
}

export function getRuntimeCapabilities(): RuntimeCapabilities {
  const waitAsync = typeof Atomics !== 'undefined' && typeof (Atomics as any).waitAsync === 'function';

  if (typeof SharedArrayBuffer === 'undefined') {
    return {
      sharedMemory: false,
      waitAsync,
      reason:
        'SharedArrayBuffer is not available. This may be due to missing COOP/COEP headers or an unsupported browser.',
    };
  }

  try {
    const testMemory = new WebAssembly.Memory({
      initial: 1,
      maximum: 1,
      shared: true,
    });
    if (!(testMemory.buffer instanceof SharedArrayBuffer)) {
      throw new Error('Shared memory buffer is not available.');
    }
  } catch {
    return {
      sharedMemory: false,
      waitAsync,
      reason:
        'Shared WebAssembly.Memory is not available. This may be due to missing COOP/COEP headers.',
    };
  }

  return { sharedMemory: true, waitAsync };
}

export function checkSharedMemoryCapability(): { supported: boolean; reason?: string } {
  const caps = getRuntimeCapabilities();
  if (!caps.sharedMemory) {
    return { supported: false, reason: caps.reason };
  }
  return { supported: true };
}
