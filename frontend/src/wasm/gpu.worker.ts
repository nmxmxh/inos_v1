/**
 * INOS GPU Worker
 *
 * Autonomous WebGPU execution engine.
 * Environment: Dedicated Web Worker
 */

import { WebGpuExecutor } from './gpu/WebGpuExecutor';
import { WebGpuRequest } from './gpu/ShaderPipelineManager';
import {
  IDX_BIRD_EPOCH,
  IDX_MATRIX_EPOCH,
  IDX_SYSTEM_VISIBILITY,
  OFFSET_BIRD_BUFFER_A,
  OFFSET_BIRD_BUFFER_B,
} from './layout';

let executor: WebGpuExecutor | null = null;
let sab: SharedArrayBuffer;
let flags: Int32Array;
let isLooping = false;
let currentRequest: WebGpuRequest | null = null;

self.onmessage = async (e: MessageEvent) => {
  const { type, payload } = e.data;

  switch (type) {
    case 'INIT':
      sab = payload.sab;
      flags = new Int32Array(sab, 0, 256);
      executor = new WebGpuExecutor();
      await executor.initialize();
      console.log('[GpuWorker] Initialized and ready');
      break;

    case 'START_AUTONOMOUS':
      currentRequest = payload.request;
      if (!isLooping) {
        isLooping = true;
        runAutonomousGpuLoop();
      }
      break;

    case 'STOP':
      isLooping = false;
      break;

    case 'EXECUTE_TASK':
      if (!executor) break;
      try {
        const result = await executor.execute(payload.request, sab);
        self.postMessage({
          type: 'TASK_RESULT',
          payload: {
            taskId: payload.taskId,
            result,
          },
        });
      } catch (err: any) {
        self.postMessage({
          type: 'TASK_ERROR',
          payload: {
            taskId: payload.taskId,
            error: err.message || String(err),
          },
        });
      }
      break;
  }
};

async function runAutonomousGpuLoop() {
  if (!executor || !currentRequest || !isLooping) return;

  const request = currentRequest;
  console.log(`[GpuWorker] Starting autonomous loop for: ${request.method}`);

  // Wait for Device
  // @ts-ignore - reaching into internal for optimization
  const device = executor['device'] as GPUDevice;
  if (!device) return;

  let lastSeenEpoch = Atomics.load(flags, IDX_BIRD_EPOCH);

  while (isLooping) {
    // 1. PARK until Physics is ready (Wait for epoch change)
    Atomics.wait(flags, IDX_BIRD_EPOCH, lastSeenEpoch);
    lastSeenEpoch = Atomics.load(flags, IDX_BIRD_EPOCH);

    // 2. CHECK visibility - if hidden, we can throttle or pause
    const visibility = Atomics.load(flags, IDX_SYSTEM_VISIBILITY);
    if (visibility === 0) {
      // Deep sleep if hidden
      Atomics.wait(flags, IDX_SYSTEM_VISIBILITY, 0);
      continue;
    }

    // 3. Execute the Compute Pass
    try {
      // Calculate active bird buffer offset based on epoch
      const birdsOffset = lastSeenEpoch % 2 === 0 ? OFFSET_BIRD_BUFFER_A : OFFSET_BIRD_BUFFER_B;

      // Pass SAB and offset for direct writeBuffer updates
      await executor.execute(
        {
          ...request,
          birdsOffset,
        } as any,
        sab
      );
    } catch (err) {
      console.error('[GpuWorker] Execution error:', err);
      isLooping = false;
      break;
    }

    // 4. SIGNAL Matrix Readiness - align with physics epoch
    Atomics.store(flags, IDX_MATRIX_EPOCH, lastSeenEpoch);
    Atomics.notify(flags, IDX_MATRIX_EPOCH);
  }

  console.log('[GpuWorker] Autonomous loop stopped');
}

export {};
