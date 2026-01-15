import { useCallback } from 'react';
import { INOSBridge } from '../../src/wasm/bridge-state';
import { OFFSET_ECONOMICS } from '../../src/wasm/layout';

// Economics region layout (from credits.go)
const ECONOMICS_METADATA_SIZE = 64;
const OFFSET_DEFAULT_ACCOUNT = OFFSET_ECONOMICS + ECONOMICS_METADATA_SIZE;

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
      // Balance is first 8 bytes (int64, little-endian) of default account
      return INOSBridge.readU64AsNumber(OFFSET_DEFAULT_ACCOUNT);
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
