export async function loadGoRuntime(
  target: any,
  wasmExecUrl: string,
  contextLabel: string
): Promise<void> {
  const response = await fetch(wasmExecUrl);
  const script = await response.text();
  const fn = new Function(script);
  fn.call(target);

  if (!target.Go) {
    throw new Error(`${contextLabel} Go runtime failed to load`);
  }
}

export async function fetchWasmWithFallback(
  wasmUrl: string,
  logPrefix: string
): Promise<Response> {
  let response = await fetch(wasmUrl);

  if (!response.ok && wasmUrl.endsWith('.br')) {
    const fallbackUrl = wasmUrl.replace('.wasm.br', '.wasm').split('?')[0];
    console.warn(
      `${logPrefix} Failed to load compressed WASM from ${wasmUrl}, trying fallback: ${fallbackUrl}`
    );
    response = await fetch(fallbackUrl);
  }

  if (!response.ok) {
    throw new Error(`HTTP ${response.status} ${response.statusText}`);
  }

  const contentType = response.headers.get('Content-Type');
  if (contentType && contentType.includes('text/html')) {
    throw new Error('Received HTML instead of WASM (check server SPA fallback)');
  }

  return response;
}

export async function instantiateWasm(
  response: Response,
  go: any,
  memory: WebAssembly.Memory,
  logPrefix: string
): Promise<WebAssembly.WebAssemblyInstantiatedSource> {
  const importObject = {
    ...go.importObject,
    env: { ...(go.importObject?.env || {}), memory },
  };

  const fallbackResponse = response.clone();
  try {
    return await WebAssembly.instantiateStreaming(response, importObject);
  } catch (streamingError) {
    console.warn(
      `${logPrefix} instantiateStreaming failed, falling back to arrayBuffer:`,
      streamingError
    );
  }

  const bytes = await fallbackResponse.arrayBuffer();
  const view = new Uint8Array(bytes);
  const hex = Array.from(view.slice(0, 16))
    .map(b => b.toString(16).padStart(2, '0'))
    .join(' ');
  const text = new TextDecoder().decode(view.slice(0, 50)).replace(/\0/g, '.');

  const isWasm = view[0] === 0x00 && view[1] === 0x61 && view[2] === 0x73 && view[3] === 0x6d;
  if (!isWasm) {
    if (hex.startsWith('85 ff 1f')) {
      throw new Error(`MAGIC_MISMATCH_85FF1F: Received hex: ${hex}`);
    }
    if (view[0] === 0x1f && view[1] === 0x8b) {
      throw new Error(
        'WASM is Gzip-compressed but the server is missing Content-Encoding: gzip'
      );
    }
    if (text.toLowerCase().includes('<!doctype html') || text.toLowerCase().includes('<html')) {
      throw new Error('Received HTML error page instead of WASM. Hex: ' + hex);
    }
    throw new Error(`WASM magic number mismatch ('\\0asm' expected). Received hex: ${hex}`);
  }

  return await WebAssembly.instantiate(bytes, importObject);
}

export function checkSharedMemoryCapability(): { supported: boolean; reason?: string } {
  if (typeof SharedArrayBuffer === 'undefined') {
    return {
      supported: false,
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
      supported: false,
      reason:
        'Shared WebAssembly.Memory is not available. This may be due to missing COOP/COEP headers.',
    };
  }

  return { supported: true };
}
