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

export async function fetchWasmWithFallback(wasmUrl: string, logPrefix: string): Promise<Response> {
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
  memory: WebAssembly.Memory | undefined,
  logPrefix: string
): Promise<WebAssembly.WebAssemblyInstantiatedSource> {
  const importObject = {
    ...go.importObject,
    env: { ...(go.importObject?.env || {}) },
  };

  if (memory) {
    importObject.env.memory = memory;
  }

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
      throw new Error('WASM is Gzip-compressed but the server is missing Content-Encoding: gzip');
    }
    if (text.toLowerCase().includes('<!doctype html') || text.toLowerCase().includes('<html')) {
      throw new Error('Received HTML error page instead of WASM. Hex: ' + hex);
    }
    throw new Error(`WASM magic number mismatch ('\\0asm' expected). Received hex: ${hex}`);
  }

  return await WebAssembly.instantiate(bytes, importObject);
}

export interface MeshBootstrapConfig {
  identity?: Record<string, unknown>;
  transport?: Record<string, unknown>;
  region?: string;
}

export function applyMeshBootstrapConfig(target: any, config?: MeshBootstrapConfig): void {
  if (!config) {
    return;
  }
  if (config.identity) {
    target.__INOS_IDENTITY__ = config.identity;
    const identity = config.identity as Record<string, unknown>;
    if (typeof identity.nodeId === 'string') {
      target.__INOS_NODE_ID__ = identity.nodeId;
    }
    if (typeof identity.deviceId === 'string') {
      target.__INOS_DEVICE_ID__ = identity.deviceId;
    }
    if (typeof identity.did === 'string') {
      target.__INOS_DID__ = identity.did;
    }
  }
  target.__INOS_MESH_CONFIG__ = config;
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

/**
 * WebRTC Proxy DataChannel for Worker Contexts
 */
class WebRTCProxyDataChannel extends EventTarget {
  public onopen: ((ev: any) => any) | null = null;
  public onmessage: ((ev: any) => any) | null = null;
  public onclose: ((ev: any) => any) | null = null;
  public onerror: ((ev: any) => any) | null = null;

  public readyState: RTCDataChannelState = 'connecting';
  public label: string;

  private proxyId: string;
  private channelId: string;

  constructor(proxyId: string, channelId: string, label: string) {
    super();
    this.proxyId = proxyId;
    this.channelId = channelId;
    this.label = label;
    console.log(`[WebRTCProxy] DataChannel created: ${label} (id: ${channelId})`);

    globalThis.addEventListener('message', (event: MessageEvent) => {
      const { type, proxyId, channelId, eventType, data } = event.data;
      if (
        type === 'webrtc_datachannel_event' &&
        proxyId === this.proxyId &&
        channelId === this.channelId
      ) {
        console.log(`[WebRTCProxy] DataChannel event: ${eventType} for ${this.label}`, data);
        this.handleChannelEvent(eventType, data);
      }
    });
  }

  private handleChannelEvent(eventType: string, data: any): void {
    if (data?.readyState) this.readyState = data.readyState;

    let event: any;
    if (eventType === 'message') {
      event = new MessageEvent('message', {
        data: data.data instanceof ArrayBuffer ? data.data : data.data,
      });
    } else {
      event = new Event(eventType);
    }

    this.dispatchEvent(event);

    const handlerName = `on${eventType}` as keyof this;
    const handler = this[handlerName];
    if (typeof handler === 'function') {
      (handler as Function)(event);
    }
  }

  public send(data: string | Blob | ArrayBuffer | ArrayBufferView): void {
    console.log(`[WebRTCProxy] DataChannel send: ${this.label}`, typeof data);
    (globalThis as any).postMessage({
      type: 'webrtc_proxy',
      proxyId: this.proxyId,
      channelId: this.channelId,
      method: 'send',
      args: { data },
    });
  }

  public close(): void {
    (globalThis as any).postMessage({
      type: 'webrtc_proxy',
      proxyId: this.proxyId,
      channelId: this.channelId,
      method: 'close_channel',
    });
  }
}

/**
 * WebRTC Proxy PeerConnection for Worker Contexts
 * Delegates WebRTC calls to the main thread.
 */
class WebRTCProxyPeerConnection extends EventTarget {
  public onicecandidate: ((ev: any) => any) | null = null;
  public onconnectionstatechange: ((ev: any) => any) | null = null;
  public oniceconnectionstatechange: ((ev: any) => any) | null = null;
  public onsignalingstatechange: ((ev: any) => any) | null = null;
  public ondatachannel: ((ev: any) => any) | null = null;
  public ontrack: ((ev: any) => any) | null = null;

  public connectionState: RTCPeerConnectionState = 'new';
  public iceConnectionState: RTCIceConnectionState = 'new';
  public signalingState: RTCSignalingState = 'stable';
  public localDescription: RTCSessionDescription | null = null;
  public remoteDescription: RTCSessionDescription | null = null;

  private id: string;
  private dataChannels = new Map<string, WebRTCProxyDataChannel>();

  constructor(configuration?: RTCConfiguration) {
    super();
    this.id = Math.random().toString(36).substring(7);
    console.log(`[WebRTCProxy] PeerConnection created (id: ${this.id})`, configuration);
    this.postToMain('create', { configuration });

    // Listen for events from main thread
    globalThis.addEventListener('message', (event: MessageEvent) => {
      const { type, proxyId, eventType, data, channelId, label } = event.data;
      if (proxyId !== this.id) return;

      if (type === 'webrtc_event') {
        console.log(`[WebRTCProxy] PeerConnection event: ${eventType}`, data);
        this.handleProxyEvent(eventType, data);
      } else if (type === 'webrtc_datachannel_created') {
        console.log(`[WebRTCProxy] Remote DataChannel created: ${label} (id: ${channelId})`);
        const channel = new WebRTCProxyDataChannel(this.id, channelId, label);
        // ...
        this.dataChannels.set(channelId, channel);

        const ev = new Event('datachannel') as any;
        ev.channel = channel;
        this.dispatchEvent(ev);
        if (typeof this.ondatachannel === 'function') {
          this.ondatachannel(ev);
        }
      }
    });
  }

  private postToMain(method: string, args: any = {}): void {
    (globalThis as any).postMessage({
      type: 'webrtc_proxy',
      proxyId: this.id,
      method,
      args,
    });
  }

  private handleProxyEvent(eventType: string, data: any): void {
    if (data?.connectionState) this.connectionState = data.connectionState;
    if (data?.iceConnectionState) this.iceConnectionState = data.iceConnectionState;
    if (data?.signalingState) this.signalingState = data.signalingState;
    if (data?.localDescription) this.localDescription = data.localDescription;
    if (data?.remoteDescription) this.remoteDescription = data.remoteDescription;

    let event: any;
    if (eventType === 'icecandidate') {
      event = new Event('icecandidate') as any;
      if (data.candidate) {
        // Re-hydrate the candidate to ensure methods like toJSON() are available
        // This is critical for Pion's JS bindings which may expect a real RTCIceCandidate object
        event.candidate = new RTCIceCandidate(data.candidate);
      } else {
        event.candidate = null;
      }
    } else if (eventType === 'track') {
      event = new Event('track') as any;
      event.track = data.track;
      event.streams = data.streams;
    } else {
      event = new Event(eventType);
      Object.assign(event, data);
    }

    this.dispatchEvent(event);

    // Call shorthand handlers
    const handlerName = `on${String(eventType)}` as keyof this;
    const handler = this[handlerName];
    if (typeof handler === 'function') {
      try {
        (handler as Function)(event);
      } catch (err) {
        console.error(`[WebRTCProxy] Error in ${String(handlerName)} handler:`, err);
      }
    }
  }

  public async createOffer(options?: RTCOfferOptions): Promise<RTCSessionDescriptionInit> {
    return this.proxyCall('createOffer', { options });
  }

  public async createAnswer(options?: RTCAnswerOptions): Promise<RTCSessionDescriptionInit> {
    return this.proxyCall('createAnswer', { options });
  }

  public async setLocalDescription(description?: RTCSessionDescriptionInit): Promise<void> {
    this.localDescription = description as RTCSessionDescription;
    return this.proxyCall('setLocalDescription', { description });
  }

  public async setRemoteDescription(description: RTCSessionDescriptionInit): Promise<void> {
    this.remoteDescription = description as RTCSessionDescription;
    return this.proxyCall('setRemoteDescription', { description });
  }

  public async addIceCandidate(candidate?: RTCIceCandidateInit | RTCIceCandidate): Promise<void> {
    return this.proxyCall('addIceCandidate', { candidate });
  }

  public createDataChannel(label: string, dataChannelDict?: RTCDataChannelInit): RTCDataChannel {
    const channelId = Math.random().toString(36).substring(7);
    const channel = new WebRTCProxyDataChannel(this.id, channelId, label);
    this.dataChannels.set(channelId, channel);

    this.postToMain('createDataChannel', { label, dataChannelDict, channelId });
    return channel as unknown as RTCDataChannel;
  }

  public close(): void {
    this.postToMain('close');
  }

  private proxyCall(method: string, args: any): Promise<any> {
    return new Promise((resolve, reject) => {
      const requestId = Math.random().toString(36).substring(7);
      const handler = (event: MessageEvent) => {
        const { type, proxyId, reqId, result, error } = event.data;
        if (type === 'webrtc_response' && proxyId === this.id && reqId === requestId) {
          globalThis.removeEventListener('message', handler);
          if (error) reject(new Error(error));
          else resolve(result);
        }
      };
      globalThis.addEventListener('message', handler);
      this.postToMain(method, { ...args, requestId });
    });
  }
}

/**
 * Ensure browser APIs (WebSocket, RTCPeerConnection) are available to Go WASM
 */
export function exposeBrowserApis(target: any, logPrefix: string): void {
  const g = globalThis as any;
  console.log(`${logPrefix} [EXPOSE] Starting browser API exposure...`);

  if (typeof g.window === 'undefined') {
    g.window = g;
  }

  // 1. Navigator
  if (typeof target.navigator === 'undefined') {
    if (g.navigator) {
      // Use native worker navigator
      target.navigator = g.navigator;
    } else {
      // Fallback for non-browser environments (e.g. tests)
      target.navigator = {
        userAgent: 'INOS-Worker/1.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36',
        hardwareConcurrency: 4,
        language: 'en-US',
        onLine: true,
      };
    }
  }

  // 2. Location
  if (typeof target.location === 'undefined') {
    if (g.location) {
      // Use native worker location (may be restricted)
      target.location = g.location;
    } else {
      // Fallback
      target.location = {
        protocol: 'https:',
        hostname: 'localhost',
        port: '3000',
        href: 'https://localhost:3000/worker',
        origin: 'https://localhost:3000',
      };
    }
  }

  // WebSocket
  if (typeof target.WebSocket === 'undefined') {
    if (typeof g.WebSocket !== 'undefined') {
      target.WebSocket = g.WebSocket;
    }
  }

  // RTCPeerConnection and related types
  const isWorker = typeof g.postMessage === 'function' && typeof g.importScripts === 'function';

  const rtcTypes = [
    'RTCPeerConnection',
    'RTCIceCandidate',
    'RTCSessionDescription',
    'RTCDataChannel',
    'RTCRtpSender',
    'RTCRtpReceiver',
    'RTCRtpTransceiver',
  ];
  rtcTypes.forEach(type => {
    if (typeof target[type] === 'undefined') {
      const native = g[type] || g[`webkit${type}`] || g[`moz${type}`];
      if (native && (!isWorker || type !== 'RTCPeerConnection')) {
        target[type] = native;
      } else if (isWorker && type === 'RTCPeerConnection') {
        console.log(`${logPrefix} Enabling WebRTC Proxy in Worker.`);
        target[type] = WebRTCProxyPeerConnection;
      } else if (isWorker) {
        // For helper types like IceCandidate, if missing, we can use a simple object constructor
        // though usually they are present if the browser supports WebRTC at all.
        target[type] = class {
          constructor(init: any) {
            Object.assign(this, init);
          }
        };
      }
    }
  });

  if (typeof g.INOSBridge !== 'undefined' && typeof target.INOSBridge === 'undefined') {
    target.INOSBridge = g.INOSBridge;
  }

  console.log(`${logPrefix} [EXPOSE] Browser APIs exposure check:`, {
    targetType: target === g ? 'globalThis (self/window)' : 'custom object',
    WebSocket: typeof target.WebSocket,
    RTCPeerConnection: typeof target.RTCPeerConnection,
    RTCIceCandidate: typeof target.RTCIceCandidate,
    RTCSessionDescription: typeof target.RTCSessionDescription,
    RTCDataChannel: typeof target.RTCDataChannel,
    isProxy: target.RTCPeerConnection === WebRTCProxyPeerConnection,
  });
}
