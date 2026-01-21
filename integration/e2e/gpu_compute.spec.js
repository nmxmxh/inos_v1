const { test, expect } = require('@playwright/test');

test.describe('INOS GPU Compute Pipeline', () => {
  test.beforeEach(async ({ page }) => {
    // Redirect browser console to stdout for debugging
    page.on('console', msg => {
      if (msg.type() === 'error') {
        console.error(`[Browser Error] ${msg.text()}`);
      } else {
        console.log(`[Browser] ${msg.text()}`);
      }
    });

    await page.goto('/');
    
    // Wait for kernel, bridge, and mesh to be ready
    await page.waitForFunction(() => {
      return window.inos?.ready &&
             window.INOSBridge?.isReady?.() &&
             window.mesh &&
             typeof window.mesh.delegateJob === 'function';
    }, { timeout: 30000 });
  });

  test('should execute GPU boids simulation via mesh delegation', async ({ page }) => {
    const submission = await page.evaluate(async () => {
      const prevEpoch = window.INOSBridge.getEpoch(window.INOSBridge.IDX_OUTBOX_HOST_DIRTY); 
      
      // 236 bytes per bird (matching GpuVerifier expectation)
      const birdSize = 236;
      const birdCount = 10;
      const boidData = new Uint8Array(birdSize * birdCount);
      for (let i = 0; i < boidData.length; i++) boidData[i] = Math.random() * 255;

      const job = {
        id: 'gpu-boids-test-001',
        type: 'gpu',
        op: 'gpu.boids',
        data: boidData,
        params: { 
          count: birdCount,
          dt: 0.1
        }
      };

      console.log('[Test] Delegating GPU Boids job...');
      window.mesh.delegateJob(job).catch(err => console.error('[Test] GPU delegate failed', err));
      
      return { jobId: job.id, prevEpoch };
    });

    // 2. Wait for completion signal
    await page.waitForFunction((prev) => {
      return window.INOSBridge.getEpoch(window.INOSBridge.IDX_OUTBOX_HOST_DIRTY) > prev;
    }, submission.prevEpoch, { timeout: 20000 });

    // 3. Collect and verify result
    const result = await page.evaluate(async (jobId) => {
      const startTime = Date.now();
      while (Date.now() - startTime < 10000) {
        const bytes = window.INOSBridge.popResult();
        if (!bytes) {
          await new Promise(r => setTimeout(r, 100));
          continue;
        }

        const decoded = await window.kernel.deserializeResult(bytes);
        if (decoded.jobId === jobId) {
          return decoded;
        }
      }
      return null;
    }, submission.jobId);

    expect(result).not.toBeNull();
    if (!result.success) {
      console.error(`[Test] Job failed: ${JSON.stringify(result, null, 2)}`);
    }
    expect(result.success).toBe(true);
    
    // Verify data integrity
    expect(result.data).toBeDefined();
    expect(result.data.length).toBe(236 * 10);

    
    // Basic verification: result should not be all zeros if input wasn't (or at least it shifted)
    const isAllZeros = Array.from(result.data).every(x => x === 0);
    expect(isAllZeros).toBe(false);
  });

  test('should execute GPU matrix generation (instance_matrix_gen)', async ({ page }) => {
    const submission = await page.evaluate(async () => {
      const prevEpoch = window.INOSBridge.getEpoch(window.INOSBridge.IDX_OUTBOX_HOST_DIRTY);
      
      const boidData = new Uint8Array(32 * 5); // 5 boids
      const job = {
        id: 'gpu-matrix-test-001',
        type: 'gpu',
        op: 'instance_matrix_gen',
        data: boidData,
        params: { count: 5 }
      };

      window.mesh.delegateJob(job).catch(err => console.error('[Test] GPU matrix delegate failed', err));
      return { jobId: job.id, prevEpoch };
    });

    await page.waitForFunction((prev) => {
      return window.INOSBridge.getEpoch(window.INOSBridge.IDX_OUTBOX_HOST_DIRTY) > prev;
    }, submission.prevEpoch, { timeout: 15000 });

    const result = await page.evaluate(async (jobId) => {
      const startTime = Date.now();
      while (Date.now() - startTime < 5000) {
        const bytes = window.INOSBridge.popResult();
        if (bytes) {
          const decoded = await window.kernel.deserializeResult(bytes);
          if (decoded.jobId === jobId) return decoded;
        }
        await new Promise(r => setTimeout(r, 50));
      }
      return null;
    }, submission.jobId);

    expect(result).not.toBeNull();
    expect(result.success).toBe(true);
    // Matrix gen returns 64 bytes per matrix
    expect(result.data.length).toBe(64 * 5);
  });
});
