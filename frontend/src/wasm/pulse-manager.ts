import PulseWorkerUrl from './pulse.worker.ts?worker&url';

let pulseWorker: Worker | null = null;

const epochHandlers = new Map<number, Set<(value: number, index: number) => void>>();

const pulseManager = {
  /**
   * Initialize the pulse system
   */
  start(sab: SharedArrayBuffer) {
    if (pulseWorker) return;

    console.log('[PulseManager] Starting high-precision pulse worker...');
    pulseWorker = new Worker(PulseWorkerUrl, { type: 'module' });

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

    // Handle visibility changes for the entire system
    document.addEventListener('visibilitychange', () => {
      const visible = document.visibilityState === 'visible';
      pulseWorker?.postMessage({
        type: 'SET_VISIBILITY',
        payload: { visible },
      });
    });

    // Initial visibility state
    pulseWorker.postMessage({
      type: 'SET_VISIBILITY',
      payload: { visible: document.visibilityState === 'visible' },
    });
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
  },
};

export default pulseManager;
