/**
 * INOS Pulse Manager
 *
 * Orchestrates the dedicated Pulse Worker.
 * Separated from Kernel (Go) to focus on timing and visibility authority.
 */

import PulseWorkerUrl from './pulse.worker?worker&url';

let pulseWorker: Worker | null = null;

export const pulseManager = {
  /**
   * Initialize the Pulse Worker with the SharedArrayBuffer.
   */
  async initialize(sab: SharedArrayBuffer): Promise<void> {
    if (pulseWorker) {
      console.warn('[PulseManager] Already initialized');
      return;
    }

    console.log('[PulseManager] Spawning Precise Pulse Worker...');
    pulseWorker = new Worker(PulseWorkerUrl, { type: 'module' });

    pulseWorker.postMessage({ type: 'INIT', payload: { sab } });
    pulseWorker.postMessage({ type: 'START' });

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
