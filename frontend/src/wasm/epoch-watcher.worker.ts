/// <reference lib="webworker" />

interface EpochWatcherInit {
  type: 'init';
  sab: SharedArrayBuffer;
  sabOffset: number;
  index: number;
}

interface EpochWatcherShutdown {
  type: 'shutdown';
}

type EpochWatcherMessage = EpochWatcherInit | EpochWatcherShutdown;

let running = true;
let active: { sab: SharedArrayBuffer; sabOffset: number; index: number } | null = null;

function isValidInit(data: EpochWatcherInit): boolean {
  if (!(data.sab instanceof SharedArrayBuffer)) return false;
  if (!Number.isFinite(data.sabOffset) || data.sabOffset < 0) return false;
  if (!Number.isFinite(data.index) || data.index < 0) return false;
  return true;
}

function watchEpoch(sab: SharedArrayBuffer, sabOffset: number, index: number): void {
  const flags = new Int32Array(sab, sabOffset, 128);
  if (index >= flags.length) {
    self.postMessage({ type: 'error', error: `index ${index} out of range` });
    return;
  }

  let expected = Atomics.load(flags, index);

  while (running) {
    Atomics.wait(flags, index, expected);
    const current = Atomics.load(flags, index);
    if (current !== expected) {
      expected = current;
      self.postMessage({ type: 'epoch_change', index, value: current });
    }
  }
}

self.onmessage = (event: MessageEvent<EpochWatcherMessage>) => {
  const { data } = event;
  if (data.type === 'shutdown') {
    running = false;
    self.close();
    return;
  }
  if (!isValidInit(data)) {
    self.postMessage({ type: 'error', error: 'invalid init payload' });
    return;
  }
  if (active) {
    self.postMessage({ type: 'error', error: 'watcher already initialized' });
    return;
  }
  active = { sab: data.sab, sabOffset: data.sabOffset, index: data.index };
  watchEpoch(data.sab, data.sabOffset, data.index);
};
