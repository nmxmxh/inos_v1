import { useEffect } from 'react';
import { useSystemStore } from '../store/system';
import { KernelStats, KernelInstance } from '../types/system';

// Re-export types for compatibility
export type { KernelStats, KernelInstance as Kernel };

export function useWasmBridge() {
  const { kernel, status, error, stats, initialize } = useSystemStore();

  useEffect(() => {
    // Auto-initialize on first use if not already done
    if (status === 'initializing') {
      initialize();
    }
  }, [status, initialize]);

  return {
    kernel,
    loading: status === 'booting' || status === 'initializing',
    error,
    stats,
  };
}
