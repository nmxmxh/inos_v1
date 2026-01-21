import { IDX_SYSTEM_PULSE, IDX_SYSTEM_VISIBILITY } from './layout';

declare const self: DedicatedWorkerGlobalScope;

let isRunning = false;
let isVisible = true;
let targetFPS = 60;
const backgroundFPS = 1;
let lastPulseTime = 0;
let flags: Int32Array | null = null;

self.onmessage = event => {
  const { type, payload } = event.data;

  switch (type) {
    case 'INIT':
      const { sab } = payload;
      flags = new Int32Array(sab);
      isRunning = true;
      runPulseLoop();
      break;

    case 'STOP':
      isRunning = false;
      break;

    case 'SET_TPS':
      targetFPS = payload.fps;
      break;

    case 'SET_VISIBILITY':
      isVisible = payload.visible;
      if (flags) {
        // Update visibility flag in SAB for workers to see
        Atomics.store(flags, IDX_SYSTEM_VISIBILITY, isVisible ? 1 : 0);
        Atomics.notify(flags, IDX_SYSTEM_VISIBILITY);
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
    if (flags) {
      // Increment the system pulse
      Atomics.add(flags, IDX_SYSTEM_PULSE, 1);
      Atomics.notify(flags, IDX_SYSTEM_PULSE);
    }
    lastPulseTime = now - (delta % interval); // Jitter compensation
  }

  // Tight loop for high precision timing without rAF overhead
  setTimeout(runPulseLoop, 0);
}

export {};
