import PulseWorkerUrl from './pulse.worker.ts?worker&url';

let pulseWorker: Worker | null = null;

const pulseManager = {
  /**
   * Initialize the pulse system
   */
  start(sab: SharedArrayBuffer) {
    if (pulseWorker) return;

    console.log('[PulseManager] Starting high-precision pulse worker...');
    pulseWorker = new Worker(PulseWorkerUrl, { type: 'module' });

    pulseWorker.postMessage({
      type: 'INIT',
      payload: { sab },
    });

    // Handle visibility changes for the entire system
    document.addEventListener('visibilitychange', () => {
      const visible = document.visibilityState === 'visible';
      console.log(
        `[PulseManager] Visibility changed: ${visible ? 'VISIBLE' : 'HIDDEN'} (Throttling active)`
      );

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
