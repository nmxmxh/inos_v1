/**
 * INOS Pulse Worker
 * Dedicated Precise Timing Authority (Zero-rAF Pilot)
 */

import { IDX_SYSTEM_PULSE, IDX_SYSTEM_VISIBILITY, IDX_SYSTEM_POWER_STATE } from './layout';

let sab: SharedArrayBuffer;
let flags: Int32Array;
let isRunning = false;
let isVisible = true;
let targetFPS = 60;
let backgroundFPS = 10; // Throttle to 10 TPS in background for continuation
let lastPulseTime = 0;

self.onmessage = (e: MessageEvent) => {
  const { type, payload } = e.data;

  switch (type) {
    case 'INIT':
      sab = payload.sab;
      flags = new Int32Array(sab);
      console.log('[PulseWorker] Initialized');
      break;

    case 'START':
      isRunning = true;
      lastPulseTime = performance.now();
      runPulseLoop();
      break;

    case 'STOP':
      isRunning = false;
      break;

    case 'SET_TPS':
      targetFPS = payload.fps;
      break;

    case 'SET_VISIBILITY':
      isVisible = !!payload.visible;
      if (flags) {
        // IDX_SYSTEM_VISIBILITY: 1 = Visible (Rendering Active), 0 = Hidden (Rendering Paused)
        Atomics.store(flags, IDX_SYSTEM_VISIBILITY, isVisible ? 1 : 0);
        Atomics.notify(flags, IDX_SYSTEM_VISIBILITY);

        // IDX_SYSTEM_POWER_STATE: 1 = High Perf (Visible), 0 = Throttled (Background)
        // This allows computation to continue but with reduced frequency
        Atomics.store(flags, IDX_SYSTEM_POWER_STATE, isVisible ? 1 : 0);
        Atomics.notify(flags, IDX_SYSTEM_POWER_STATE);
      }
      break;
  }
};

function runPulseLoop() {
  if (!isRunning) return;

  const now = performance.now();
  const delta = now - lastPulseTime;

  // Adaptive frequency based on visibility
  const currentTPS = isVisible ? targetFPS : backgroundFPS;
  const interval = 1000 / currentTPS;

  if (delta >= interval) {
    // Increment the system pulse
    Atomics.add(flags, IDX_SYSTEM_PULSE, 1);
    Atomics.notify(flags, IDX_SYSTEM_PULSE);

    lastPulseTime = now - (delta % interval); // Jitter compensation
  }

  // Tight loop for high precision timing
  setTimeout(runPulseLoop, 0);
}
