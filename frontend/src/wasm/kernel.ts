import { MEMORY_PAGES, type ResourceTier } from './layout';
import { clearViewCache } from '../../app/features/scenes/SceneWrapper';
import { initializeBridge, clearBridge, INOSBridge } from './bridge-state';
import { fetchWasmWithFallback, instantiateWasm, loadGoRuntime } from './kernel.shared';
import {
  applyMeshBootstrapConfig,
  exposeBrowserApis,
  type MeshBootstrapConfig,
} from './kernel.shared';
import { createMeshClient } from './mesh';
import { pulseManager } from './pulse-manager';

// Vite worker import syntax
import KernelWorkerUrl from './kernel.worker?worker&url';

// Re-export IDX_CONTEXT_ID_HASH for other modules
export { IDX_CONTEXT_ID_HASH } from './layout';

console.log('!!! KERNEL TS LOADED !!!');

function ensureInosApi(): void {
  const global = window as any;
  if (!global.INOSBridge) {
    global.INOSBridge = INOSBridge;
  }
  if (!global.inos) {
    global.inos = {};
  }
  if (typeof global.inos.ready !== 'boolean') {
    global.inos.ready = false;
  }

  if (global.inos.invoke === undefined) {
    global.inos.invoke = undefined;
  }
}

function setInosReady(): void {
  const global = window as any;
  if (!global.inos) {
    global.inos = {};
  }
  global.inos.ready = true;
}

function clearKernelGlobals(): void {
  const global = window as any;
  global.__INOS_SAB__ = undefined;
  global.__INOS_SAB_OFFSET__ = undefined;
  global.__INOS_SAB_SIZE__ = undefined;
  global.__INOS_MEM__ = undefined;
  global.__INOS_TIER__ = undefined;
  global.__INOS_KERNEL_MODE__ = undefined;
  if (global.__INOS_EPOCH_LOGGER__) {
    clearInterval(global.__INOS_EPOCH_LOGGER__);
    global.__INOS_EPOCH_LOGGER__ = undefined;
  }
}

function terminateKernelWorker(): void {
  if (window.__INOS_KERNEL_WORKER__) {
    window.__INOS_KERNEL_WORKER__.terminate();
    window.__INOS_KERNEL_WORKER__ = undefined;
  }
  pulseManager.shutdown();
}

declare global {
  interface Window {
    Go: any;
    __INOS_SAB__: SharedArrayBuffer;
    __INOS_MEM__: WebAssembly.Memory;
    __INOS_SAB_OFFSET__: number;
    __INOS_SAB_SIZE__: number;
    __INOS_TIER__: ResourceTier;
    __INOS_CONTEXT_ID__: string;
    __INOS_INIT_PROMISE__?: Promise<KernelInitResult>;
    __INOS_KERNEL_WORKER__?: Worker;
    __INOS_KERNEL_MODE__?: 'worker' | 'main';
    __INOS_EPOCH_LOGGER__?: number;
    getSystemSABAddress?: () => number;
    getSystemSABSize?: () => number;
    kernel?: {
      submitJob: (job: any) => Promise<any>;
      deserializeResult: (data: Uint8Array) => Promise<any>;
    };
    mesh?: {
      delegateJob: (job: any) => Promise<any>;
    };
  }
}

export type { ResourceTier } from './layout';

// Re-export layout config for backward compatibility
export const TIER_CONFIG = MEMORY_PAGES;

export interface KernelInitResult {
  memory?: WebAssembly.Memory; // Might be unavailable on main thread if only worker has it
  sabBase: SharedArrayBuffer;
  sabOffset: number;
  sabSize: number;
}

export async function initializeKernel(
  tier: ResourceTier = 'light',
  meshConfig?: MeshBootstrapConfig
): Promise<KernelInitResult> {
  // 0. Ensure API is available early for tests
  ensureInosApi();
  const contextId = Math.random().toString(36).substring(2, 9);
  window.__INOS_CONTEXT_ID__ = contextId;

  // 1. Atomic Locking - Prevent concurrent initialization spawns
  if (window.__INOS_INIT_PROMISE__) {
    console.log('[Kernel] Waiting for existing initialization to complete...');
    return window.__INOS_INIT_PROMISE__;
  }

  applyMeshBootstrapConfig(window, meshConfig);
  exposeBrowserApis(window, '[Kernel]');

  // Reuse existing kernel (avoid dual main-thread + worker execution)
  if (window.__INOS_SAB__ && window.__INOS_KERNEL_MODE__ === 'main') {
    console.log('[Kernel] Reusing main-thread kernel singleton');
    return {
      sabBase: window.__INOS_SAB__,
      sabOffset: window.__INOS_SAB_OFFSET__ || 0,
      sabSize: window.__INOS_SAB_SIZE__ || 0,
    };
  }

  if (window.__INOS_SAB__ && window.__INOS_KERNEL_WORKER__) {
    console.log('[Kernel] Reusing worker kernel singleton');
    return {
      sabBase: window.__INOS_SAB__,
      sabOffset: window.__INOS_SAB_OFFSET__ || 0,
      sabSize: window.__INOS_SAB_SIZE__ || 0,
    };
  }

  // Ensure we never run both main-thread and worker kernels
  if (window.__INOS_KERNEL_WORKER__) {
    terminateKernelWorker();
    clearKernelGlobals();
  }

  // Clear stale SAB views (Fixes memory leak on HMR/Re-init)
  clearViewCache();
  clearBridge();

  // Define the init logic as a single promise
  const init = async (): Promise<KernelInitResult> => {
    const isSafari = /^((?!chrome|android).)*safari/i.test(navigator.userAgent);
    const isIOS =
      /iPad|iPhone|iPod/.test(navigator.userAgent) ||
      (navigator.userAgent.includes('Mac') && navigator.maxTouchPoints > 1);

    const isDev = import.meta.env.DEV;
    // Default URL logic
    let wasmUrl = isDev ? '/kernel.wasm' : '/kernel.wasm.br?v=2.0';

    // Safari/iOS Fix: Force uncompressed WASM and aggressively bust cache
    if (isSafari || isIOS) {
      console.log('[Kernel] Safari/iOS detected: forcing uncompressed WASM and cache bust');
      // Strip .br extension
      wasmUrl = wasmUrl.replace('.br', '');
      // Append cache buster
      const separator = wasmUrl.includes('?') ? '&' : '?';
      wasmUrl = `${wasmUrl}${separator}t=${Date.now()}`;
    }

    if (isIOS || isSafari) {
      console.log('[Kernel] Safari/iOS detected, prioritizing main-thread initialization');
      return await initializeKernelOnMainThread(tier, wasmUrl);
    }

    // Try Worker-based initialization first, fall back to main thread
    try {
      // Create SAB on Main Thread (One SAB to rule them all)
      if (!window.__INOS_SAB__) {
        const config = MEMORY_PAGES[tier];
        // We create a Shared Memory to get the buffer, but we don't necessarily use it as WASM memory
        // if we are in "Split Memory" mode. But for now, we just need the buffer.
        const mem = new WebAssembly.Memory({
          initial: config.initial,
          maximum: config.maximum,
          shared: true,
        });
        window.__INOS_SAB__ = mem.buffer as unknown as SharedArrayBuffer;
        window.__INOS_SAB_SIZE__ = mem.buffer.byteLength;
        window.__INOS_SAB_OFFSET__ = 0;
        window.__INOS_MEM__ = mem;

        console.log(`[Kernel] allocated Shared SAB: ${mem.buffer.byteLength} bytes`);
      }

      return await initializeKernelInWorker(tier, wasmUrl, meshConfig);
    } catch (workerError) {
      console.warn(
        '[Kernel] Worker initialization failed, falling back to main thread:',
        workerError
      );
      return await initializeKernelOnMainThread(tier, wasmUrl);
    }
  };

  window.__INOS_INIT_PROMISE__ = init().then(result => {
    // 2. Initialize the Pulse Manager (Dedicated heartbeat worker)
    pulseManager.initialize(result.sabBase);
    return result;
  });
  return window.__INOS_INIT_PROMISE__;
}

function startEpochLogger(label: string, sabOffset: number): void {
  if (!(window as any).__INOS_DEBUG_EPOCH__) return;
  if ((window as any).__INOS_EPOCH_LOGGER__) return;
  (window as any).__INOS_EPOCH_LOGGER__ = window.setInterval(() => {
    const flags = INOSBridge.getFlagsView();
    if (!flags) return;
    const birdEpoch = Atomics.load(flags, 12);
    const evoEpoch = Atomics.load(flags, 16);
    const systemEpoch = Atomics.load(flags, 7);
    const birdCount = Atomics.load(flags, 20);
    console.log(`[Kernel] ${label} epoch snapshot`, {
      sabOffset,
      birdEpoch,
      evolutionEpoch: evoEpoch,
      systemEpoch,
      birdCount,
    });
  }, 1000);
}

/**
 * Initialize kernel in a dedicated Web Worker (preferred path)
 */
async function initializeKernelInWorker(
  tier: ResourceTier,
  wasmUrl: string,
  meshConfig?: MeshBootstrapConfig
): Promise<KernelInitResult> {
  console.log('[Kernel] Spawning Kernel Worker...');
  window.__INOS_KERNEL_MODE__ = 'worker';
  const worker = new Worker(KernelWorkerUrl, { type: 'module' });
  window.__INOS_KERNEL_WORKER__ = worker;
  setupWebRTCProxyHost(worker);

  let workerReadyResolve: (() => void) | null = null;
  let workerReadyResolved = false;
  const workerReady = new Promise<void>(resolve => {
    workerReadyResolve = resolve;
  });

  return new Promise<KernelInitResult>((resolve, reject) => {
    const timeoutId = setTimeout(() => {
      worker.terminate();
      window.__INOS_KERNEL_WORKER__ = undefined;
      window.__INOS_KERNEL_MODE__ = undefined;
      reject(new Error('Kernel worker initialization timeout (10s)'));
    }, 10000);
    const messageHandler = (event: MessageEvent<any>) => {
      const { type, sab, sabSize, error } = event.data;

      if (type === 'error') {
        clearTimeout(timeoutId);
        console.error('[KernelWorker] Critical error:', error);
        worker.terminate();
        window.__INOS_KERNEL_WORKER__ = undefined;
        window.__INOS_KERNEL_MODE__ = undefined;
        reject(new Error(error));
        return;
      }

      if (type === 'sab_functions_ready') {
        clearTimeout(timeoutId);
        console.log('[Kernel] Kernel Worker SAB ready');
        if (!workerReadyResolved) {
          workerReadyResolved = true;
          workerReadyResolve?.();
        }

        const { memory } = event.data;
        window.__INOS_SAB__ = sab;
        window.__INOS_MEM__ = memory;

        // Use absolute 0-based offsets (authoritative schema)
        const sabOffset = 0;
        window.__INOS_SAB_OFFSET__ = sabOffset;
        window.__INOS_SAB_SIZE__ = sabSize;
        window.__INOS_TIER__ = tier;

        // Initialize centralized SAB bridge (if not already done)
        if (!INOSBridge.isReady()) {
          initializeBridge(
            window.__INOS_SAB__,
            sabOffset,
            window.__INOS_SAB_SIZE__,
            window.__INOS_MEM__
          );
          (window as any).INOSBridge = INOSBridge;
          startEpochLogger('worker', sabOffset);
        }

        // Initialize API proxies to worker
        if (!(window as any).kernel) {
          (window as any).kernel = {
            submitJob: (job: any) => {
              return new Promise((res, rej) => {
                const reqId = Math.random().toString(36).substring(7);
                const handler = (e: MessageEvent) => {
                  if (e.data.type === 'kernel_response' && e.data.requestId === reqId) {
                    worker.removeEventListener('message', handler);
                    if (e.data.error) rej(new Error(e.data.error));
                    else res(e.data.result);
                  }
                };
                worker.addEventListener('message', handler);
                worker.postMessage({
                  type: 'kernel_call',
                  method: 'submitJob',
                  args: [job],
                  requestId: reqId,
                });
              });
            },
            deserializeResult: (data: Uint8Array) => {
              return new Promise((res, rej) => {
                const reqId = Math.random().toString(36).substring(7);
                const handler = (e: MessageEvent) => {
                  if (e.data.type === 'kernel_response' && e.data.requestId === reqId) {
                    worker.removeEventListener('message', handler);
                    if (e.data.error) rej(new Error(e.data.error));
                    else res(e.data.result);
                  }
                };
                worker.addEventListener('message', handler);
                worker.postMessage({
                  type: 'kernel_call',
                  method: 'deserializeResult',
                  args: [data],
                  requestId: reqId,
                });
              });
            },
            getStats: () => {
              return new Promise((res, rej) => {
                const reqId = Math.random().toString(36).substring(7);
                const handler = (e: MessageEvent) => {
                  if (e.data.type === 'kernel_response' && e.data.requestId === reqId) {
                    worker.removeEventListener('message', handler);
                    if (e.data.error) rej(new Error(e.data.error));
                    else res(e.data.result);
                  }
                };
                worker.addEventListener('message', handler);
                worker.postMessage({
                  type: 'kernel_call',
                  method: 'getStats',
                  args: [],
                  requestId: reqId,
                });
              });
            },
          };
        }

        if (!(window as any).mesh) {
          (window as any).mesh = createMeshClient((method, args = []) => {
            return new Promise((res, rej) => {
              const reqId = Math.random().toString(36).substring(7);
              const handler = (e: MessageEvent) => {
                if (e.data.type === 'mesh_response' && e.data.requestId === reqId) {
                  worker.removeEventListener('message', handler);
                  if (e.data.error) rej(new Error(e.data.error));
                  else res(e.data.result);
                }
              };
              worker.addEventListener('message', handler);
              worker.postMessage({ type: 'mesh_call', method, args, requestId: reqId });
            });
          });
        }

        ensureInosApi();
        setInosReady();

        // Resolve the initialization promise
        resolve({
          memory,
          sabBase: sab,
          sabOffset: 0,
          sabSize,
        });
        // ...
        // Intentionally skipped duplicate postMessage

        const readyHandler = (event: MessageEvent<any>) => {
          if (event.data?.type === 'ready') {
            if (!workerReadyResolved) {
              workerReadyResolved = true;
              workerReadyResolve?.();
            }
            console.log('[Kernel] Kernel Worker ready');
            worker.removeEventListener('message', readyHandler);
          }
        };
        worker.addEventListener('message', readyHandler);

        const readyFallback = setTimeout(() => {
          if (!workerReadyResolved) {
            console.warn('[Kernel] Worker ready signal not received; continuing');
            workerReadyResolved = true;
            workerReadyResolve?.();
          }
        }, 5000);

        // Moved init message to end of function to avoid deadlock

        workerReady.then(() => {
          clearTimeout(readyFallback);
        });
      } // End if (sab_functions_ready)
    }; // End messageHandler

    worker.addEventListener('message', messageHandler);
    worker.addEventListener('error', e => {
      clearTimeout(timeoutId);
      worker.terminate();
      window.__INOS_KERNEL_WORKER__ = undefined;
      reject(new Error(`Worker error: ${e.message}`));
    });

    // Send INIT immediately (break deadlock)
    console.log('[Kernel] Sending INIT to worker...', { tier, hasSab: !!window.__INOS_SAB__ });
    worker.postMessage({
      type: 'init',
      tier,
      wasmUrl,
      meshConfig,
      sab: window.__INOS_SAB__,
    });

    // We already attach specific readyHandler inside, so maybe we don't need this global one,
    // but the original code had it. Let's keep the structure clean.

    // Actually, looking at the previous code, there was a secondary readyHandler attached at the top level
    // Let's restore the basic error handling and event attachment.
  });
}

/**
 * Initialize kernel on main thread (fallback for iOS/Safari)
 * This blocks the main thread during init but allows the kernel to run.
 */
async function initializeKernelOnMainThread(
  tier: ResourceTier,
  wasmUrl: string
): Promise<KernelInitResult> {
  console.log('[Kernel] 🔄 Initializing kernel on MAIN THREAD (fallback mode)');
  window.__INOS_KERNEL_MODE__ = 'main';
  terminateKernelWorker();
  clearKernelGlobals();

  // 1. Load Go runtime
  await loadGoRuntime(window, '/wasm_exec.js', '[Kernel]');
  exposeBrowserApis(window, '[Kernel]');

  // 2. Create shared memory (or fallback to non-shared if unavailable)
  const config = MEMORY_PAGES[tier];
  let memory: WebAssembly.Memory;
  let isShared = true;

  try {
    memory = new WebAssembly.Memory({
      initial: config.initial,
      maximum: config.maximum,
      shared: true,
    });
  } catch {
    console.warn('[Kernel] Shared memory unavailable, using non-shared (limited functionality)');
    memory = new WebAssembly.Memory({
      initial: config.initial,
      maximum: config.maximum,
    });
    isShared = false;
  }

  // 3. Load and instantiate Go kernel (with Retry Logic)
  const go = new window.Go();
  let result: WebAssembly.WebAssemblyInstantiatedSource | undefined;

  const MAX_RETRIES = 2;
  let lastError: Error | null = null;

  for (let attempt = 0; attempt < MAX_RETRIES; attempt++) {
    try {
      const currentUrl =
        attempt > 0 ? `${wasmUrl}${wasmUrl.includes('?') ? '&' : '?'}retry=${Date.now()}` : wasmUrl;

      if (attempt > 0) {
        console.warn(`[Kernel] Retrying WASM fetch (attempt ${attempt + 1}/${MAX_RETRIES})...`);
      }

      const wasmResponse = await fetchWasmWithFallback(currentUrl, '[Kernel]');
      // FIX: Pass undefined for memory to ensure Go uses its own private memory (Split Memory Architecture)
      result = await instantiateWasm(wasmResponse, go, undefined, '[Kernel]');

      // If we got here, success
      break;
    } catch (err) {
      lastError = err instanceof Error ? err : new Error(String(err));

      // Only retry on specific magic number mismatch or network errors (optional)
      const isMagicMismatch = lastError.message.includes('MAGIC_MISMATCH_85FF1F');

      if (isMagicMismatch && attempt < MAX_RETRIES - 1) {
        console.warn(
          '[Kernel] Detected corrupted cache (magic 85 ff 1f). Triggering retry with cache bust...'
        );
        continue;
      }

      // For other errors, or if retries exhausted, rethrow
      throw lastError;
    }
  }

  if (!result) {
    throw lastError || new Error('Failed to instantiate WASM after retries');
  }

  // 4. Run Go kernel (non-blocking - runs async via goroutines)
  const buffer = memory.buffer as unknown as SharedArrayBuffer;
  const sabOffset = 0;
  const sabSize = buffer.byteLength;

  window.__INOS_SAB__ = buffer;
  window.__INOS_SAB_OFFSET__ = sabOffset;
  window.__INOS_SAB_SIZE__ = sabSize;

  if (isShared) {
    (window as any).__INOS_SAB_INT32__ = new Int32Array(buffer, sabOffset, 128);
  }

  go.run(result.instance);

  // 5. Wait for SAB functions
  const maxWaitMs = 5000;
  const startTime = Date.now();
  while (!window.getSystemSABAddress || !window.getSystemSABSize) {
    if (Date.now() - startTime > maxWaitMs) {
      console.warn('[Kernel] Timeout waiting for SAB functions on main thread');
      break;
    }
    await new Promise(r => setTimeout(r, 10));
  }

  // 6. Final initialization and bridge setup
  window.__INOS_MEM__ = memory;
  window.__INOS_TIER__ = tier;

  if (isShared) {
    initializeBridge(buffer, sabOffset, sabSize, memory);
    (window as any).INOSBridge = INOSBridge;
    startEpochLogger('main', sabOffset);

    // FIX: Inject SAB to start supervisors (Grounding)
    // This is required to signal 'sabReady' in the Go kernel
    console.log('[Kernel] ⚡ Grounding SAB to Go kernel...');
    const injectResult = (window as any).initializeSharedMemory(sabOffset, sabSize);
    if (injectResult?.error) {
      console.error('[Kernel] ❌ SAB Grounding failed:', injectResult.error);
    } else {
      console.log('[Kernel] ✨ SAB Grounded successfully');
    }
  }

  console.log(`[Kernel] ✅ Main thread kernel initialized (shared: ${isShared})`);

  // Direct access to Kernel and Mesh APIs
  if (!window.kernel) {
    (window as any).kernel = {
      submitJob: (job: any) => (window as any).jsSubmitJob?.(job),
      deserializeResult: (data: Uint8Array) => (window as any).jsDeserializeResult?.(data),
    };
  }
  if (!window.mesh) {
    const nativeMesh = (window as any).mesh;
    if (nativeMesh) {
      (window as any).__INOS_MESH_NATIVE__ = nativeMesh;
      (window as any).mesh = createMeshClient((method, args = []) => {
        const fn = (window as any).__INOS_MESH_NATIVE__?.[method];
        if (!fn) {
          return Promise.reject(new Error(`Mesh method ${method} not available`));
        }
        try {
          return Promise.resolve(fn(...args));
        } catch (err) {
          return Promise.reject(err instanceof Error ? err : new Error(String(err)));
        }
      });
    } else {
      (window as any).mesh = createMeshClient((method, args = []) => {
        const fn = (window as any).jsDelegateJob;
        if (method !== 'delegateJob' || !fn) {
          return Promise.reject(new Error(`Mesh method ${method} not available`));
        }
        return Promise.resolve(fn(...args));
      });
    }
  }

  ensureInosApi();
  setInosReady();

  return {
    memory,
    sabBase: buffer,
    sabOffset,
    sabSize,
  };
}

/**
 * WebRTC Proxy Host (Main Thread)
 * Handles WebRTC requests from the worker and manages real PeerConnections.
 */
function setupWebRTCProxyHost(worker: Worker): void {
  const peerConnections = new Map<string, RTCPeerConnection>();
  const dataChannels = new Map<string, RTCDataChannel>();

  worker.addEventListener('message', async (event: MessageEvent) => {
    const { type, proxyId, method, args, channelId } = event.data;
    if (type !== 'webrtc_proxy') return;

    console.log(
      `[WebRTCProxyHost] Received ${method} for proxy ${proxyId}`,
      JSON.stringify(args, null, 2)
    );
    let pc = peerConnections.get(proxyId);

    try {
      switch (method) {
        case 'create': {
          const configuration = args.configuration;
          pc = new RTCPeerConnection(configuration);
          peerConnections.set(proxyId, pc);

          // Standard events
          const events = [
            'icecandidate',
            'connectionstatechange',
            'iceconnectionstatechange',
            'signalingstatechange',
            'track',
          ];
          events.forEach(evtType => {
            pc!.addEventListener(evtType, (e: any) => {
              console.log(`[WebRTCProxyHost] Event fired: ${evtType} for ${proxyId}`);
              const data: any = {};
              // Explicitly handle null for end-of-candidates
              if (evtType === 'icecandidate') {
                data.candidate = e.candidate;
              }
              data.connectionState = pc!.connectionState;
              data.iceConnectionState = pc!.iceConnectionState;
              data.signalingState = pc!.signalingState;
              data.localDescription = pc!.localDescription;
              data.remoteDescription = pc!.remoteDescription;

              try {
                // Ensure data is plain object to avoid DataCloneError
                const safeData = JSON.parse(JSON.stringify(data));
                worker.postMessage({
                  type: 'webrtc_event',
                  proxyId,
                  eventType: evtType,
                  data: safeData,
                });
              } catch (err) {
                console.error(`[WebRTCProxyHost] Failed to post message to worker:`, err);
              }
            });
          });

          // DataChannel event (incoming)
          pc.ondatachannel = (e: RTCDataChannelEvent) => {
            const incomingChannelId = Math.random().toString(36).substring(7);
            const channel = e.channel;
            dataChannels.set(incomingChannelId, channel);
            setupDataChannel(channel, incomingChannelId, proxyId, worker);

            worker.postMessage({
              type: 'webrtc_datachannel_created',
              proxyId,
              channelId: incomingChannelId,
              label: channel.label,
            });
          };
          break;
        }

        case 'createOffer': {
          console.log(`[WebRTCProxyHost] createOffer for ${proxyId}`, args.options);
          const offer = await pc!.createOffer(args.options || undefined);
          worker.postMessage({
            type: 'webrtc_response',
            proxyId,
            reqId: args.requestId,
            result: offer,
          });
          break;
        }

        case 'createAnswer': {
          console.log(`[WebRTCProxyHost] createAnswer for ${proxyId}`, args.options);
          const answer = await pc!.createAnswer(args.options || undefined);
          worker.postMessage({
            type: 'webrtc_response',
            proxyId,
            reqId: args.requestId,
            result: answer,
          });
          break;
        }

        case 'setLocalDescription': {
          console.log(`[WebRTCProxyHost] setLocalDescription for ${proxyId}`, args.description);
          if (!args.description) throw new Error('setLocalDescription: description is required');
          await pc!.setLocalDescription(args.description);
          worker.postMessage({ type: 'webrtc_response', proxyId, reqId: args.requestId });
          break;
        }

        case 'setRemoteDescription': {
          console.log(`[WebRTCProxyHost] setRemoteDescription for ${proxyId}`, args.description);
          if (!args.description) throw new Error('setRemoteDescription: description is required');
          await pc!.setRemoteDescription(args.description);
          worker.postMessage({ type: 'webrtc_response', proxyId, reqId: args.requestId });
          break;
        }

        case 'addIceCandidate': {
          console.log(`[WebRTCProxyHost] addIceCandidate for ${proxyId}`, args.candidate);
          await pc!.addIceCandidate(args.candidate || undefined);
          worker.postMessage({ type: 'webrtc_response', proxyId, reqId: args.requestId });
          break;
        }

        case 'createDataChannel': {
          const channel = pc!.createDataChannel(args.label, args.dataChannelDict);
          dataChannels.set(args.channelId, channel);
          setupDataChannel(channel, args.channelId, proxyId, worker);
          break;
        }

        case 'send': {
          const channel = dataChannels.get(channelId);
          if (channel && channel.readyState === 'open') {
            channel.send(args.data);
          }
          break;
        }

        case 'close': {
          pc?.close();
          peerConnections.delete(proxyId);
          break;
        }
      }
    } catch (error: any) {
      console.error(`[WebRTCProxyHost] Error in ${method}:`, error);
      worker.postMessage({
        type: 'webrtc_response',
        proxyId,
        reqId: args.requestId,
        error: error.message || String(error),
      });
    }
  });

  function setupDataChannel(
    channel: RTCDataChannel,
    channelId: string,
    proxyId: string,
    worker: Worker
  ) {
    const events = ['open', 'message', 'close', 'error'];
    events.forEach(evtType => {
      channel.addEventListener(evtType, (e: any) => {
        const data: any = { readyState: channel.readyState };
        if (evtType === 'message') data.data = e.data;

        worker.postMessage({
          type: 'webrtc_datachannel_event',
          proxyId,
          channelId,
          eventType: evtType,
          data,
        });
      });
    });
  }
}

export function shutdownKernel() {
  if (window.__INOS_SAB__) {
    const flags = new Int32Array(window.__INOS_SAB__, 0, 16);
    // Standard INOS shutdown signal (IDX 0)
    Atomics.store(flags, 0, 1);
    Atomics.notify(flags, 0);
    console.log('[Kernel] 🛑 Sent shutdown signal to Go process');
  }
  (window as any).__INOS_INIT_PROMISE__ = undefined;
}
