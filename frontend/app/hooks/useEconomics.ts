import { useState, useEffect, useCallback, useRef } from 'react';
import { INOSBridge } from '../../src/wasm/bridge-state';

interface UseEconomicsResult {
  loading: boolean;
  offset: number | null;
  getBalance: () => number;
  getFormattedBalance: () => string;
}

/**
 * useEconomics - Zero-Copy Hook for Economic Data
 *
 * Instead of polling the async bridge, this hook resolves the SAB offset once
 * and allows the component to read directly from memory in its render loop.
 */
export function useEconomics(did?: string): UseEconomicsResult {
  const [offset, setOffset] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);

  // Use a ref to track if we're currently fetching to prevent double-firing
  const fetchingRef = useRef(false);

  useEffect(() => {
    // Wait for kernel to be ready
    if (!window.economics?.getAccountInfo) {
      const checkInterval = setInterval(() => {
        if (window.economics?.getAccountInfo) {
          clearInterval(checkInterval);
          loadAccount();
        }
      }, 100);
      return () => clearInterval(checkInterval);
    } else {
      loadAccount();
    }

    async function loadAccount() {
      if (fetchingRef.current) return;
      fetchingRef.current = true;

      try {
        const info = await window.economics!.getAccountInfo(did);
        if (info && info.exists) {
          console.log(`[Economics] Zero-Copy attached to account at offset ${info.offset}`);
          setOffset(info.offset);
        }
      } catch (err) {
        console.warn('[Economics] Failed to resolving account offset:', err);
      } finally {
        setLoading(false);
        fetchingRef.current = false;
      }
    }
  }, [did]);

  const getBalance = useCallback(() => {
    if (offset === null || !INOSBridge.isReady()) return 0;

    // Balance is the first 8 bytes (int64) of the account struct
    // We read as BigUint64 and convert to Number (safe for < 9 quadrillion)
    return INOSBridge.readU64AsNumber(offset);
  }, [offset]);

  const getFormattedBalance = useCallback(() => {
    return getBalance().toLocaleString();
  }, [getBalance]);

  return {
    loading,
    offset,
    getBalance,
    getFormattedBalance,
  };
}
