import { useEffect, useMemo, useState } from 'react';
import {
  getOrCreateGuestIdentity,
  type MeshEventMessage,
  type MeshEventSubscription,
} from '../../../src/wasm/mesh';
import { INOSBridge } from '../../../src/wasm/bridge-state';
import {
  IDX_MESH_EVENT_DROPPED,
  IDX_MESH_EVENT_EPOCH,
  IDX_MESH_EVENT_HEAD,
  IDX_MESH_EVENT_TAIL,
  MESH_EVENT_SLOT_COUNT,
} from '../../../src/wasm/layout';
import { MeshEvent } from '../../../bridge/generated/protocols/schemas/p2p/v1/mesh';
import {
  DelegateRequest,
  DelegateResponse,
  DelegateRequest_Operation_Which,
  DelegateResponse_Status,
} from '../../../bridge/generated/protocols/schemas/p2p/v1/delegation';

export interface MeshEventFeedEntry {
  id: string;
  type: string;
  payloadType: string;
  timestamp: bigint;
  summary: string;
  size: number;
}

export interface MeshEventFeedStats {
  head: number;
  tail: number;
  depth: number;
  dropped: number;
  epoch: number;
}

const CONNECTION_STATE_LABELS = ['disconnected', 'connecting', 'connected', 'degraded', 'failed'];

function formatMeshEventSummary(event: MeshEventMessage): string {
  const parsed = event.parsed;
  if (parsed instanceof MeshEvent) {
    if (parsed._isPeerUpdate) {
      const peer = parsed.peerUpdate;
      const state = CONNECTION_STATE_LABELS[peer.connectionState] || 'unknown';
      return `peer ${peer.peerId} ${state}`;
    }
    if (parsed._isChunkDiscovered) {
      const chunk = parsed.chunkDiscovered;
      return `chunk ${chunk.chunkHash} via ${chunk.peerId}`;
    }
    if (parsed._isReputationChange) {
      const rep = parsed.reputationChange;
      return `reputation ${rep.peerId} ${rep.newScore.toFixed(2)}`;
    }
    if (parsed._isModelRegistered) {
      const model = parsed.modelRegistered;
      return `model ${model.modelId}`;
    }
  }
  if (parsed instanceof DelegateRequest) {
    const operation = parsed.operation.which();
    const opLabel =
      operation === DelegateRequest_Operation_Which.HASH
        ? 'hash'
        : operation === DelegateRequest_Operation_Which.COMPRESS
          ? 'compress'
          : operation === DelegateRequest_Operation_Which.ENCRYPT
            ? 'encrypt'
            : 'custom';
    return `delegate ${parsed.id} (${opLabel})`;
  }
  if (parsed instanceof DelegateResponse) {
    const status = parsed.status;
    const statusLabel =
      status === DelegateResponse_Status.SUCCESS
        ? 'success'
        : status === DelegateResponse_Status.INPUT_MISSING
          ? 'input_missing'
          : status === DelegateResponse_Status.CAPACITY_EXCEEDED
            ? 'capacity_exceeded'
            : status === DelegateResponse_Status.TIMEOUT
              ? 'timeout'
              : status === DelegateResponse_Status.VERIFICATION_FAILED
                ? 'verification_failed'
                : 'failed';
    return `delegate ${parsed.requestId} (${statusLabel})`;
  }
  return event.payloadType || event.type;
}

export function useMeshEvents(limit = 40) {
  const [events, setEvents] = useState<MeshEventFeedEntry[]>([]);
  const [stats, setStats] = useState<MeshEventFeedStats>({
    head: 0,
    tail: 0,
    depth: 0,
    dropped: 0,
    epoch: 0,
  });

  const identity = useMemo(() => {
    const identityConfig = (window as any).__INOS_IDENTITY__;
    if (identityConfig?.nodeId && identityConfig?.deviceId) {
      return identityConfig;
    }
    return getOrCreateGuestIdentity();
  }, []);

  useEffect(() => {
    let subscription: MeshEventSubscription | null = null;
    let active = true;

    const attach = async () => {
      const mesh = (window as any).mesh;
      if (!mesh?.subscribeToEvents) return;
      subscription = await mesh.subscribeToEvents(['mesh.*', 'delegation.*'], event => {
        if (!active) return;
        const summary = formatMeshEventSummary(event);
        setEvents(prev => {
          const next = [
            {
              id: event.id,
              type: event.type,
              payloadType: event.payloadType,
              timestamp: event.timestamp,
              summary,
              size: event.payload.length,
            },
            ...prev,
          ];
          return next.slice(0, limit);
        });
      });
    };

    attach();

    const interval = setInterval(() => {
      if (!INOSBridge.isReady()) return;
      const head = INOSBridge.atomicLoad(IDX_MESH_EVENT_HEAD);
      const tail = INOSBridge.atomicLoad(IDX_MESH_EVENT_TAIL);
      const dropped = INOSBridge.atomicLoad(IDX_MESH_EVENT_DROPPED);
      const epoch = INOSBridge.atomicLoad(IDX_MESH_EVENT_EPOCH);
      const depth = Math.max(0, Math.min(MESH_EVENT_SLOT_COUNT, tail - head));
      setStats({ head, tail, dropped, epoch, depth });
    }, 250);

    return () => {
      active = false;
      clearInterval(interval);
      if (subscription) {
        subscription.unsubscribe();
      }
    };
  }, [limit]);

  return { events, stats, identity };
}
