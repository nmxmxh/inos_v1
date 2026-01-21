import { Message } from 'capnp-es';
import { Base_Envelope } from '../../bridge/generated/protocols/schemas/base/v1/base';
import {
  DelegateRequest,
  DelegateResponse,
} from '../../bridge/generated/protocols/schemas/p2p/v1/delegation';
import { MeshEvent } from '../../bridge/generated/protocols/schemas/p2p/v1/mesh';
import type {
  MeshMetrics,
  PeerCapability,
} from '../../bridge/generated/protocols/schemas/p2p/v1/mesh';
import EpochWatcherWorkerUrl from './epoch-watcher.worker.ts?worker&url';
import { getDataView, getFlagsView, getOffset, getSAB } from './bridge-state';
import type { MeshBootstrapConfig } from './kernel.shared';
import {
  IDX_MESH_EVENT_EPOCH,
  IDX_MESH_EVENT_HEAD,
  IDX_MESH_EVENT_TAIL,
  MESH_EVENT_SLOT_COUNT,
  MESH_EVENT_SLOT_SIZE,
  OFFSET_MESH_EVENT_QUEUE,
} from './layout';

export interface MeshIdentity {
  did: string;
  deviceId: string;
  nodeId: string;
  displayName?: string;
  hasGpu?: boolean;
  hasWebGpu?: boolean;
}

export interface MeshTransportConfig {
  webrtcEnabled?: boolean;
  iceServers?: string[];
  stunServers?: string[];
  turnServers?: string[];
  webSocketUrl?: string;
  signalingServers?: string[];
  maxConnections?: number;
  connectionTimeoutMs?: number;
  reconnectDelayMs?: number;
  keepAliveIntervalMs?: number;
  maxMessageSize?: number;
  rpcTimeoutMs?: number;
  maxRetries?: number;
  poolSize?: number;
  poolMaxIdleMs?: number;
  metricsIntervalMs?: number;
}

const MESH_EVENT_HEADER_SIZE = 16;

export interface MeshTelemetry {
  node_id?: string;
  did?: string;
  device_id?: string;
  display_name?: string;
  node_count?: number;
  active_peers?: number;
  avg_latency_ms?: number;
  bytes_sent?: number;
  bytes_received?: number;
  messages_sent?: number;
  messages_received?: number;
  region?: string;
}

export type MeshEventPayload = MeshEvent | DelegateRequest | DelegateResponse | null;

export interface MeshEventMessage {
  id: string;
  type: string;
  timestamp: bigint;
  payloadType: string;
  payload: Uint8Array;
  parsed: MeshEventPayload;
}

export type MeshEventHandler = (event: MeshEventMessage) => void;

export interface MeshEventSubscription {
  id: string;
  unsubscribe: () => Promise<void>;
}

export type MeshCall = (method: string, args?: any[]) => Promise<any>;

export interface MeshClient {
  delegateJob: (job: any) => Promise<any>;
  delegateCompute: (job: any) => Promise<any>;
  setIdentity: (identity: Partial<MeshIdentity>) => Promise<any>;
  configureTransport: (config: MeshTransportConfig) => Promise<any>;
  getTelemetry: () => Promise<MeshTelemetry>;
  getMetrics: () => Promise<MeshMetrics | Record<string, unknown>>;
  findPeersWithChunk: (chunkHash: string) => Promise<PeerCapability[] | Record<string, unknown>>;
  findBestPeerForChunk: (chunkHash: string) => Promise<PeerCapability | Record<string, unknown>>;
  registerChunk: (chunkHash: string, size: number, priority?: string) => Promise<any>;
  unregisterChunk: (chunkHash: string) => Promise<any>;
  scheduleChunkPrefetch: (chunkHashes: string[], priority?: string) => Promise<any>;
  reportPeerPerformance: (
    peerId: string,
    success: boolean,
    latencyMs: number,
    operation?: string
  ) => Promise<any>;
  getPeerReputation: (peerId: string) => Promise<any>;
  getTopPeers: (limit?: number) => Promise<any>;
  connectToPeer: (peerId: string, address?: string) => Promise<any>;
  disconnectFromPeer: (peerId: string) => Promise<any>;
  subscribeToEvents: (
    topics: string[],
    handler: MeshEventHandler
  ) => Promise<MeshEventSubscription>;
}

const CRC32_TABLE = (() => {
  const table = new Uint32Array(256);
  for (let i = 0; i < 256; i++) {
    let crc = i;
    for (let j = 0; j < 8; j++) {
      crc = (crc & 1) !== 0 ? 0xedb88320 ^ (crc >>> 1) : crc >>> 1;
    }
    table[i] = crc >>> 0;
  }
  return table;
})();

function crc32(data: Uint8Array): number {
  let crc = 0xffffffff;
  for (let i = 0; i < data.length; i++) {
    crc = CRC32_TABLE[(crc ^ data[i]) & 0xff] ^ (crc >>> 8);
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function decodeEnvelope(bytes: Uint8Array): MeshEventMessage | null {
  try {
    const msg = new Message(bytes.buffer as any, false);
    const envelope = msg.getRoot(Base_Envelope);
    const payload = envelope._hasPayload() ? envelope.payload.data : new Uint8Array(0);
    const payloadBytes = new Uint8Array(
      (payload as any).byteLength || (payload as any).length || 0
    );
    payloadBytes.set(payload as any);
    const payloadType = envelope._hasPayload() ? envelope.payload.typeId : envelope.type;
    let parsed: MeshEventPayload = null;

    if (payloadBytes.length > 0) {
      const payloadMsg = new Message(payloadBytes.buffer, false);
      if (payloadType.startsWith('mesh.')) {
        parsed = payloadMsg.getRoot(MeshEvent);
      } else if (payloadType === 'delegation.request') {
        parsed = payloadMsg.getRoot(DelegateRequest);
      } else if (payloadType === 'delegation.response') {
        parsed = payloadMsg.getRoot(DelegateResponse);
      }
    }

    return {
      id: envelope.id,
      type: envelope.type,
      timestamp: envelope.timestamp,
      payloadType,
      payload: payloadBytes,
      parsed,
    };
  } catch (error) {
    console.warn('[MeshEvents] Failed to decode envelope', error);
    return null;
  }
}

class MeshEventStream {
  private worker: Worker | null = null;
  private handlers = new Map<string, MeshEventHandler>();

  subscribe(id: string, handler: MeshEventHandler): void {
    this.handlers.set(id, handler);
    this.ensureWorker();
  }

  unsubscribe(id: string): void {
    this.handlers.delete(id);
    if (this.handlers.size === 0) {
      this.stopWorker();
    }
  }

  private ensureWorker(): void {
    if (this.worker) {
      return;
    }
    const sab = getSAB();
    if (!sab) {
      console.warn('[MeshEvents] SharedArrayBuffer not ready');
      return;
    }

    this.worker = new Worker(EpochWatcherWorkerUrl, { type: 'module' });
    this.worker.onmessage = event => {
      if (event.data?.type !== 'epoch_change') {
        return;
      }
      this.drainQueue();
    };
    this.worker.postMessage({
      type: 'init',
      sab,
      sabOffset: getOffset(),
      index: IDX_MESH_EVENT_EPOCH,
    });
  }

  private stopWorker(): void {
    if (!this.worker) {
      return;
    }
    this.worker.postMessage({ type: 'shutdown' });
    this.worker.terminate();
    this.worker = null;
  }

  private drainQueue(): void {
    const sab = getSAB();
    const view = getDataView();
    const flags = getFlagsView();
    if (!sab || !view || !flags) {
      return;
    }

    let head = Atomics.load(flags, IDX_MESH_EVENT_HEAD);
    const tail = Atomics.load(flags, IDX_MESH_EVENT_TAIL);
    const baseOffset = getOffset() + OFFSET_MESH_EVENT_QUEUE;

    while (head !== tail) {
      const slotIndex = head % MESH_EVENT_SLOT_COUNT;
      const slotOffset = baseOffset + slotIndex * MESH_EVENT_SLOT_SIZE;
      const size = view.getUint32(slotOffset, true);
      const expectedCrc = view.getUint32(slotOffset + 8, true);

      if (size > 0 && size <= MESH_EVENT_SLOT_SIZE - MESH_EVENT_HEADER_SIZE) {
        const payloadOffset = slotOffset + MESH_EVENT_HEADER_SIZE;
        const slice = new Uint8Array(sab, payloadOffset, size);
        const payload = new Uint8Array(size);
        payload.set(slice);
        if (crc32(payload) === expectedCrc) {
          const decoded = decodeEnvelope(payload);
          if (decoded) {
            for (const handler of this.handlers.values()) {
              handler(decoded);
            }
          }
        }
      }

      head += 1;
    }

    Atomics.store(flags, IDX_MESH_EVENT_HEAD, head);
  }
}

const meshEventStream = new MeshEventStream();

export function createMeshClient(call: MeshCall): MeshClient {
  return {
    delegateJob: job => call('delegateJob', [job]),
    delegateCompute: job => call('delegateCompute', [job]),
    setIdentity: identity => call('setIdentity', [identity]),
    configureTransport: config => call('configureTransport', [config]),
    getTelemetry: () => call('getTelemetry', []),
    getMetrics: () => call('getMetrics', []),
    findPeersWithChunk: chunkHash => call('findPeersWithChunk', [chunkHash]),
    findBestPeerForChunk: chunkHash => call('findBestPeerForChunk', [chunkHash]),
    registerChunk: (chunkHash, size, priority) =>
      call('registerChunk', [chunkHash, size, priority]),
    unregisterChunk: chunkHash => call('unregisterChunk', [chunkHash]),
    scheduleChunkPrefetch: (chunkHashes, priority) =>
      call('scheduleChunkPrefetch', [chunkHashes, priority]),
    reportPeerPerformance: (peerId, success, latencyMs, operation) =>
      call('reportPeerPerformance', [peerId, success, latencyMs, operation]),
    getPeerReputation: peerId => call('getPeerReputation', [peerId]),
    getTopPeers: limit => call('getTopPeers', [limit]),
    connectToPeer: (peerId, address) => call('connectToPeer', [peerId, address]),
    disconnectFromPeer: peerId => call('disconnectFromPeer', [peerId]),
    subscribeToEvents: async (topics, handler) => {
      const response = await call('subscribeToEvents', [topics]);
      const subId =
        response?.subscriptionId ||
        response?.subscription_id ||
        response?.id ||
        `mesh_sub_${Date.now()}`;
      meshEventStream.subscribe(subId, handler);
      return {
        id: subId,
        unsubscribe: async () => {
          meshEventStream.unsubscribe(subId);
          await call('unsubscribeFromEvents', [subId]);
        },
      };
    },
  };
}

export function getStoredGuestIdentity(storageKey = 'inos.mesh.identity'): MeshIdentity | null {
  if (typeof localStorage === 'undefined') {
    return null;
  }
  const raw = localStorage.getItem(storageKey);
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw) as MeshIdentity;
  } catch {
    return null;
  }
}

export function createGuestIdentity(storageKey = 'inos.mesh.identity'): MeshIdentity {
  const deviceId = `device:${crypto.randomUUID().replace(/-/g, '')}`;
  const nodeId = `node:${crypto.randomUUID().replace(/-/g, '')}`;
  const identity: MeshIdentity = {
    did: 'did:inos:system',
    deviceId,
    nodeId,
    displayName: 'Guest',
  };

  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(storageKey, JSON.stringify(identity));
  }
  return identity;
}

export function getOrCreateGuestIdentity(storageKey = 'inos.mesh.identity'): MeshIdentity {
  const stored = getStoredGuestIdentity(storageKey);
  if (stored && stored.deviceId && stored.nodeId) {
    return stored;
  }
  return createGuestIdentity(storageKey);
}

function getSearchParams(): URLSearchParams | null {
  if (typeof window === 'undefined' || !window.location?.search) {
    return null;
  }
  try {
    return new URLSearchParams(window.location.search);
  } catch {
    return null;
  }
}

function getFirstParam(params: URLSearchParams | null, keys: string[]): string | null {
  if (!params) {
    return null;
  }
  for (const key of keys) {
    const value = params.get(key);
    if (value) {
      return value;
    }
  }
  return null;
}

function getAllParams(params: URLSearchParams | null, keys: string[]): string[] {
  if (!params) {
    return [];
  }
  const values: string[] = [];
  keys.forEach(key => {
    params.getAll(key).forEach(value => {
      if (value) {
        values.push(value);
      }
    });
  });
  return values;
}

export function getIdentityOverridesFromUrl(): Partial<MeshIdentity> {
  const params = getSearchParams();
  if (!params) {
    return {};
  }

  const overrides: Partial<MeshIdentity> = {};
  const nodeId = getFirstParam(params, ['nodeId', 'node_id', 'peer']);
  const deviceId = getFirstParam(params, ['deviceId', 'device_id']);
  const did = getFirstParam(params, ['did']);
  const displayName = getFirstParam(params, ['name', 'displayName', 'display_name']);

  if (nodeId) overrides.nodeId = nodeId;
  if (deviceId) overrides.deviceId = deviceId;
  if (did) overrides.did = did;
  if (displayName) overrides.displayName = displayName;

  return overrides;
}

export function getSignalingServersFromUrl(): string[] {
  const params = getSearchParams();
  const values = getAllParams(params, [
    'signaling',
    'signal',
    'signalingServer',
    'signaling_server',
  ]);
  const parsed: string[] = [];
  values.forEach(value => {
    value
      .split(',')
      .map(v => v.trim())
      .filter(Boolean)
      .forEach(v => parsed.push(v));
  });
  return parsed;
}

export function resolveMeshBootstrapConfig(storageKey = 'inos.mesh.identity'): MeshBootstrapConfig {
  const identity = getOrCreateGuestIdentity(storageKey);
  const overrides = getIdentityOverridesFromUrl();
  const signalingServers = getSignalingServersFromUrl();

  const mergedIdentity: MeshIdentity = {
    ...identity,
    ...overrides,
    hasGpu: !!(typeof navigator !== 'undefined' && (navigator as any).gpu),
    hasWebGpu: !!(typeof navigator !== 'undefined' && (navigator as any).gpu),
  };

  if (typeof localStorage !== 'undefined' && Object.keys(overrides).length === 0) {
    localStorage.setItem(storageKey, JSON.stringify(mergedIdentity));
  }

  let transport: MeshTransportConfig | undefined =
    signalingServers.length > 0
      ? { signalingServers, webSocketUrl: signalingServers[0] }
      : undefined;

  // Applying universal signaling strategy (Local + Global)

  if (!transport) {
    console.log('[Mesh] Applying universal signaling strategy (Local + Global)');
    transport = {
      signalingServers: ['ws://localhost:8787/ws', 'wss://signaling.inos.ai/ws'],
      webSocketUrl: 'ws://localhost:8787/ws', // Legacy primary
    };
  }

  return {
    identity: mergedIdentity as any,
    transport: transport as any,
  } as MeshBootstrapConfig;
}
