import { useEffect, useState } from 'react';
import { INOSBridge } from '../../src/wasm/bridge-state';
import { OFFSET_IDENTITY_REGISTRY } from '../../src/wasm/layout';

const IDENTITY_METADATA_SIZE = 64;
const IDENTITY_ENTRY_SIZE = 128;
const SOCIAL_ENTRY_SIZE = 1248;

const IDENTITY_META_DEFAULT_IDENTITY_OFFSET = 8;
const IDENTITY_META_DEFAULT_SOCIAL_OFFSET = 16;

const IDENTITY_STATUS_UNDER_RECOVERY = 1;

const DID_BYTES = 64;

function parseDid(bytes: Uint8Array) {
  const end = bytes.findIndex(byte => byte === 0);
  const slice = end === -1 ? bytes : bytes.slice(0, end);
  return new TextDecoder().decode(slice);
}

export interface IdentitySnapshot {
  did: string;
  status: number;
  tier: number;
  recoveryThreshold: number;
  totalShares: number;
  socialOffset: number;
  closeIds: string[];
  verifiedCloseIds: number;
}

export function useIdentitySnapshot() {
  const [snapshot, setSnapshot] = useState<IdentitySnapshot | null>(null);

  useEffect(() => {
    const interval = setInterval(() => {
      try {
        if (!INOSBridge.isReady()) return;

        const metadata = INOSBridge.getRegionDataView(OFFSET_IDENTITY_REGISTRY, IDENTITY_METADATA_SIZE);
        if (!metadata) return;

        const identityOffset = metadata.getUint32(IDENTITY_META_DEFAULT_IDENTITY_OFFSET, true);
        if (!identityOffset) return;

        const identityBytes = INOSBridge.getRegionDataView(identityOffset, IDENTITY_ENTRY_SIZE);
        if (!identityBytes) return;

        const identityBuffer = new Uint8Array(
          identityBytes.buffer,
          identityBytes.byteOffset,
          IDENTITY_ENTRY_SIZE
        );

        const did = parseDid(identityBuffer.slice(0, DID_BYTES));
        const status = identityBuffer[97];
        const socialOffset = new DataView(
          identityBuffer.buffer,
          identityBuffer.byteOffset + 102,
          4
        ).getUint32(0, true);
        const recoveryThreshold = identityBuffer[106];
        const totalShares = identityBuffer[107];
        const tier = identityBuffer[108];

        const resolvedSocialOffset =
          socialOffset || metadata.getUint32(IDENTITY_META_DEFAULT_SOCIAL_OFFSET, true);

        const closeIds: string[] = [];
        let verifiedCloseIds = 0;

        if (resolvedSocialOffset) {
          const socialView = INOSBridge.getRegionDataView(resolvedSocialOffset, SOCIAL_ENTRY_SIZE);
          if (socialView) {
            const socialBuffer = new Uint8Array(
              socialView.buffer,
              socialView.byteOffset,
              SOCIAL_ENTRY_SIZE
            );

            const closeBase = 128;
            for (let i = 0; i < 15; i++) {
              const start = closeBase + i * DID_BYTES;
              const id = parseDid(socialBuffer.slice(start, start + DID_BYTES));
              if (id) {
                closeIds.push(id);
              }
            }

            const verifiedBase = closeBase + DID_BYTES * 15 + 4 * 15;
            for (let i = 0; i < 15; i++) {
              const offset = verifiedBase + i * 4;
              const ts =
                socialBuffer[offset] |
                (socialBuffer[offset + 1] << 8) |
                (socialBuffer[offset + 2] << 16) |
                (socialBuffer[offset + 3] << 24);
              if (ts > 0) verifiedCloseIds += 1;
            }
          }
        }

        setSnapshot({
          did,
          status,
          tier,
          recoveryThreshold,
          totalShares,
          socialOffset: resolvedSocialOffset,
          closeIds,
          verifiedCloseIds,
        });
      } catch {
        // SAB not ready
      }
    }, 500);

    return () => clearInterval(interval);
  }, []);

  return snapshot;
}

export function getIdentityStatusLabel(status: number) {
  if (status === IDENTITY_STATUS_UNDER_RECOVERY) return 'UNDER_RECOVERY';
  if (status === 3) return 'SYSTEM_WALLET';
  if (status === 2) return 'REVOKED';
  return 'ACTIVE';
}

export function getTierLabel(tier: number) {
  if (tier === 3) return 'DEDICATED';
  if (tier === 2) return 'HEAVY';
  if (tier === 1) return 'MODERATE';
  return 'LIGHT';
}
