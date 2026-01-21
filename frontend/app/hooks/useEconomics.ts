import { useCallback } from 'react';
import { INOSBridge } from '../../src/wasm/bridge-state';
import { OFFSET_ECONOMICS, OFFSET_IDENTITY_REGISTRY } from '../../src/wasm/layout';

// Economics region layout (from credits.go)
const ECONOMICS_METADATA_SIZE = 64;
const OFFSET_DEFAULT_ACCOUNT = OFFSET_ECONOMICS + ECONOMICS_METADATA_SIZE;
const IDENTITY_METADATA_SIZE = 64;
const IDENTITY_META_DEFAULT_ACCOUNT_OFFSET = 12;

interface UseEconomicsResult {
  loading: boolean;
  getBalance: () => number;
  getFormattedBalance: () => string;
}

/**
 * useEconomics - Zero-Copy Hook for Economic Data
 *
 * Reads directly from the SAB economics region at OFFSET_ECONOMICS (0x004200).
 * Account structure (128 bytes):
 *   - Balance: int64 at offset 0 (first 8 bytes)
 *   - EarnedTotal: uint64 at offset 8
 *   - SpentTotal: uint64 at offset 16
 *   - ... (see credits.go for full layout)
 *
 * The default account ("did:inos:local") is at OFFSET_DEFAULT_ACCOUNT.
 */
export function useEconomics(): UseEconomicsResult {
  const getBalance = useCallback(() => {
    if (!INOSBridge.isReady()) return 0;

    try {
      const metadata = INOSBridge.getRegionDataView(OFFSET_IDENTITY_REGISTRY, IDENTITY_METADATA_SIZE);
      const accountOffset = metadata
        ? metadata.getUint32(IDENTITY_META_DEFAULT_ACCOUNT_OFFSET, true) || OFFSET_DEFAULT_ACCOUNT
        : OFFSET_DEFAULT_ACCOUNT;

      // Balance is first 8 bytes (int64, little-endian) of the default account slot
      return INOSBridge.readI64AsNumber(accountOffset);
    } catch {
      return 0;
    }
  }, []);

  const getFormattedBalance = useCallback(() => {
    return getBalance().toLocaleString();
  }, [getBalance]);

  return {
    loading: false, // No async loading - pure SAB read
    getBalance,
    getFormattedBalance,
  };
}
