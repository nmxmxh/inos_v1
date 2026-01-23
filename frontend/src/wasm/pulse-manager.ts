import PulseWorkerUrl from './pulse.worker.ts?worker&url';
import { IDX_SYSTEM_PULSE } from './layout';

let pulseWorker: Worker | null = null;
let mainThreadPulseId: number | null = null;
let mainThreadFlags: Int32Array | null = null;

const epochHandlers = new Map<number, Set<(value: number, index: number) => void>>();

/**
 * Fallback: Main-thread pulse loop for browsers where workers fail
 * Less efficient but ensures compatibility
 */
function startMainThreadPulse(sab: SharedArrayBuffer) {
  console.log('[PulseManager] Starting main-thread pulse fallback...');
  mainThreadFlags = new Int32Array(sab, 0, 128);
  let lastPulseTime = 0;
  const targetFPS = 60;
  const interval = 1000 / targetFPS;

  const pulse = () => {
    const now = performance.now();
    const delta = now - lastPulseTime;

    if (delta >= interval && mainThreadFlags) {
      Atomics.add(mainThreadFlags, IDX_SYSTEM_PULSE, 1);
      Atomics.notify(mainThreadFlags, IDX_SYSTEM_PULSE);
      lastPulseTime = now - (delta % interval);
    }

    mainThreadPulseId = requestAnimationFrame(pulse);
  };

  mainThreadPulseId = requestAnimationFrame(pulse);
}

const pulseManager = {
  /**
   * Initialize the pulse system
   */
  start(sab: SharedArrayBuffer) {
    if (pulseWorker) return;

    console.log('[PulseManager] Starting high-precision pulse worker...');

    try {
      pulseWorker = new Worker(PulseWorkerUrl, { type: 'module' });
    } catch (err) {
      console.error('[PulseManager] Failed to create pulse worker:', err);
      console.warn('[PulseManager] Falling back to main-thread pulse (degraded performance)');
      startMainThreadPulse(sab);
      return;
    }

    pulseWorker.onerror = event => {
      console.error('[PulseManager] Worker error:', event.message, event);
    };

    pulseWorker.onmessage = event => {
      const { type, payload } = event.data;
      if (type === 'EPOCH_CHANGE') {
        const { index, value } = payload;
        const handlers = epochHandlers.get(index);
        if (handlers) {
          handlers.forEach(h => h(value, index));
        }
      }
    };

    pulseWorker.postMessage({
      type: 'INIT',
      payload: { sab },
    });

    // Helper to update visibility state
    const setVisibility = (visible: boolean, source: string) => {
      console.log(`[PulseManager] Visibility: ${visible} (source: ${source})`);
      pulseWorker?.postMessage({
        type: 'SET_VISIBILITY',
        payload: { visible },
      });
    };

    // Handle visibility changes - multiple events for robust mobile support
    document.addEventListener('visibilitychange', () => {
      setVisibility(document.visibilityState === 'visible', 'visibilitychange');
    });

    // Additional events for iOS Safari robustness
    window.addEventListener('focus', () => setVisibility(true, 'focus'));
    window.addEventListener('blur', () => setVisibility(false, 'blur'));
    window.addEventListener('pageshow', () => setVisibility(true, 'pageshow'));

    // Initial visibility state
    const initialVisibility = document.visibilityState === 'visible';
    console.log('[PulseManager] Initial visibility state:', initialVisibility);
    setVisibility(initialVisibility, 'init');

    console.log('[PulseManager] Pulse worker initialized successfully');
  },

  /**
   * Watch one or more epoch indices
   */
  watchEpochs(indices: number[], handler: (value: number, index: number) => void) {
    indices.forEach(index => {
      let handlers = epochHandlers.get(index);
      if (!handlers) {
        handlers = new Set();
        epochHandlers.set(index, handlers);
      }
      handlers.add(handler);
    });

    pulseWorker?.postMessage({
      type: 'WATCH_INDICES',
      payload: { indices },
    });
  },

  /**
   * Stop watching an epoch index
   */
  unwatchEpoch(index: number, handler: (value: number, index: number) => void) {
    const handlers = epochHandlers.get(index);
    if (handlers) {
      handlers.delete(handler);
      if (handlers.size === 0) {
        epochHandlers.delete(index);
      }
    }
  },

  /**
   * Set target Ticks Per Second (TPS)
   */
  setTPS(tps: number) {
    pulseWorker?.postMessage({ type: 'SET_TPS', payload: { fps: tps } });
  },

  /**
   * Stop the pulse loop
   */
  stop() {
    pulseWorker?.postMessage({ type: 'STOP' });
  },

  /**
   * Cleanup resources
   */
  shutdown() {
    if (pulseWorker) {
      pulseWorker.terminate();
      pulseWorker = null;
    }
    if (mainThreadPulseId !== null) {
      cancelAnimationFrame(mainThreadPulseId);
      mainThreadPulseId = null;
    }
    mainThreadFlags = null;
  },
};

export default pulseManager;
