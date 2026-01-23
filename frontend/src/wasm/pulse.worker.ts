import { IDX_SYSTEM_PULSE, IDX_SYSTEM_VISIBILITY } from './layout';

declare const self: DedicatedWorkerGlobalScope;

let isRunning = false;
let isVisible = true;
let targetFPS = 60;
const backgroundFPS = 30; // 30 FPS when backgrounded - prevents severe throttling on mobile
let lastPulseTime = 0;
let flags: Int32Array | null = null;

interface PulseMessage {
  type: 'INIT' | 'STOP' | 'SET_TPS' | 'SET_VISIBILITY' | 'WATCH_INDICES';
  payload: {
    sab?: SharedArrayBuffer;
    fps?: number;
    visible?: boolean;
    indices?: number[];
  };
}

const watchers = new Set<number>();

self.onmessage = (event: MessageEvent<PulseMessage>) => {
  const { type, payload } = event.data;

  switch (type) {
    case 'INIT':
      const { sab } = payload;
      if (!sab) return;
      flags = new Int32Array(sab, 0, 128); // Standard 128-byte flags region
      isRunning = true;
      runPulseLoop();
      break;

    case 'WATCH_INDICES':
      if (!flags || !payload.indices) return;
      for (const index of payload.indices) {
        if (!watchers.has(index)) {
          watchers.add(index);
          watchIndex(index);
        }
      }
      break;

    case 'STOP':
      isRunning = false;
      break;

    case 'SET_TPS':
      if (payload.fps !== undefined) targetFPS = payload.fps;
      break;

    case 'SET_VISIBILITY':
      if (payload.visible !== undefined) {
        console.log('[PulseWorker] Visibility changed:', payload.visible);
        isVisible = payload.visible;
      }
      if (flags) {
        Atomics.store(flags, IDX_SYSTEM_VISIBILITY, isVisible ? 1 : 0);
        Atomics.notify(flags, IDX_SYSTEM_VISIBILITY);
      }
      break;
  }
};

/**
 * Check if Atomics.waitAsync is available (Safari 16.4+, Chrome 87+)
 */
const hasWaitAsync =
  typeof Atomics !== 'undefined' && typeof (Atomics as any).waitAsync === 'function';

/**
 * Microsecond-latency watcher using non-blocking Atomics.waitAsync
 * Falls back to polling on older browsers (iOS Safari < 16.4)
 */
function watchIndex(index: number) {
  if (!isRunning || !flags) return;

  const current = Atomics.load(flags, index);

  if (hasWaitAsync) {
    // Modern path: use Atomics.waitAsync for zero-CPU waiting
    const result = (Atomics as any).waitAsync(flags, index, current);

    if (result.async) {
      result.value.then(() => {
        if (isRunning && flags) {
          const newValue = Atomics.load(flags, index);
          self.postMessage({ type: 'EPOCH_CHANGE', payload: { index, value: newValue } });
          watchIndex(index); // Re-arm
        }
      });
    } else {
      // Value already changed synchronously
      const newValue = Atomics.load(flags, index);
      self.postMessage({ type: 'EPOCH_CHANGE', payload: { index, value: newValue } });
      setTimeout(() => watchIndex(index), 0); // Avoid stack overflow
    }
  } else {
    // Fallback path for older browsers: poll at 60Hz
    // This is less efficient but ensures compatibility with iOS Safari < 16.4
    const pollInterval = 16; // ~60fps
    const poll = () => {
      if (!isRunning || !flags) return;
      const newValue = Atomics.load(flags, index);
      if (newValue !== current) {
        self.postMessage({ type: 'EPOCH_CHANGE', payload: { index, value: newValue } });
        watchIndex(index); // Re-arm with new current value
      } else {
        setTimeout(poll, pollInterval);
      }
    };
    setTimeout(poll, pollInterval);
  }
}

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
