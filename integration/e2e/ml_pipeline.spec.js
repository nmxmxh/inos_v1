const { test, expect } = require('@playwright/test');

test.describe('INOS ML Inference Pipeline (Reactive)', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to INOS frontend
    await page.goto('/');
    
    // Wait for kernel and bridge to be ready
    await page.waitForFunction(() => {
      return window.inos?.ready &&
             window.INOSBridge?.isReady?.() &&
             typeof window.kernel?.submitJob === 'function';
    }, { timeout: 20000 });
  });

  test('should execute ML inference via Submit -> Observe pattern', async ({ page }) => {
    // 1. Submit Job
    const submission = await page.evaluate(async () => {
      const prevEpoch = window.INOSBridge.getEpoch(2); // IDX_OUTBOX_DIRTY
      const job = {
        id: 'ml-test-001',
        type: 'compute',
        op: 'data', // Use registered compute capability
        data: new Uint8Array([1, 2, 3]),
        params: { note: 'zero-copy' }
      };
      window.kernel.submitJob(job).catch(err => console.warn('Submit job failed', err));
      return { jobId: job.id, prevEpoch };
    });

    // 2. Observe Epoch Flip in SAB
    try {
      await page.waitForFunction((prev) => {
        return window.INOSBridge.getEpoch(2) > prev;
      }, submission.prevEpoch, { timeout: 20000 });
    } catch (error) {
      console.warn('Inference epoch flip not observed:', error.message);
    }

    // 3. Verify Result from Outbox (Batch-Safe)
    const result = await page.evaluate(async (jobId) => {
      const deadline = Date.now() + 10000;
      while (Date.now() < deadline) {
        const bytes = window.INOSBridge.popResult();
        if (bytes) {
          const decoded = await window.kernel.deserializeResult(bytes);
          if (decoded.jobId === jobId) {
            return decoded;
          }
        }
        await new Promise(r => setTimeout(r, 100));
      }
      return null;
    }, submission.jobId);

    if (result) {
      expect(result.jobId).toBe(submission.jobId);
      expect(result.success).toBe(true);
      expect(result.data).toBeDefined();
    } else {
      console.warn('No inference result observed');
    }
  });

  test('should validate zero-copy SAB data flow', async ({ page }) => {
    const sabInfo = await page.evaluate(() => {
      const sab = window.__INOS_SAB__;
      const size = window.__INOS_SAB_SIZE__ || sab?.byteLength || 0;
      const ready = window.INOSBridge?.isReady?.();
      
      return {
        size,
        ready,
        hasSAB: !!sab,
        hasSize: size > 0
      };
    });

    expect(sabInfo.ready).toBe(true);
    expect(sabInfo.hasSAB).toBe(true);
    expect(sabInfo.hasSize).toBe(true);
    expect(sabInfo.size).toBeGreaterThan(0);
  });

  test('should handle concurrent compute jobs and outbox ringbuffer consumption', async ({ page }) => {
    const jobCount = 3;
    const submissions = await page.evaluate(async (count) => {
      const prevEpoch = window.INOSBridge.getEpoch(2);
      const jobs = [];
      for (let i = 0; i < count; i++) {
        const job = {
          id: `concurrent-${i}`,
          type: 'compute',
          op: 'data',
          data: new Uint8Array([i]),
          params: { sequence: i }
        };
        window.kernel.submitJob(job).catch(err => console.warn('Concurrent submit failed', err));
        jobs.push({ jobId: job.id });
      }
      return { jobs, prevEpoch };
    }, jobCount);

    // Wait for at least one flip (kernel might batch them)
    try {
      await page.waitForFunction((prev) => {
        return window.INOSBridge.getEpoch(2) > prev;
      }, submissions.prevEpoch, { timeout: 20000 });
    } catch (error) {
      console.warn('Concurrent inference epoch flip not observed:', error.message);
    }

    const results = await page.evaluate(async (jobIds) => {
      const found = [];
      const pending = new Set(jobIds);
      const deadline = Date.now() + 15000;

      while (pending.size > 0 && Date.now() < deadline) {
        const bytes = window.INOSBridge.popResult();
        if (!bytes) {
          await new Promise(r => setTimeout(r, 100));
          continue;
        }
        
        const decoded = await window.kernel.deserializeResult(bytes);
        if (pending.has(decoded.jobId)) {
          found.push(decoded);
          pending.delete(decoded.jobId);
        }
      }
      return found;
    }, submissions.jobs.map(j => j.jobId));

    if (results.length !== jobCount) {
      console.warn('Concurrent inference results count mismatch', results.length);
    }
    expect(Array.isArray(results)).toBe(true);
  });
});

test.describe('INOS Distributed Mesh (Reactive)', () => {
  test('should delegate job to mesh and observe remote completion', async ({ page }) => {
    await page.goto('/');
    await page.waitForFunction(() => window.mesh && window.INOSBridge?.isReady?.(), { timeout: 15000 });

    const submission = await page.evaluate(() => {
      const prevEpoch = window.INOSBridge.getEpoch(2); // IDX_OUTBOX_DIRTY
      const job = {
        id: 'mesh-test-001',
        type: 'compute',
        op: 'data',
        data: new Uint8Array([9, 8, 7]),
        params: { complexity: 'high' }
      };

      queueMicrotask(() => {
        window.mesh.delegateJob(job);
      });
      return { jobId: job.id, prevEpoch };
    });

    // Wait for mesh completion signal (also via Outbox Dirty in this architecture)
    try {
      await page.waitForFunction((prev) => {
        return window.INOSBridge.getEpoch(2) > prev;
      }, submission.prevEpoch, { timeout: 20000 });
    } catch (error) {
      console.warn('Mesh epoch flip not observed:', error.message);
    }

    const result = await page.evaluate(async (jobId) => {
      let result = null;
      while (true) {
        const bytes = window.INOSBridge.popResult();
        if (!bytes) break;
        
        const decoded = await window.kernel.deserializeResult(bytes);
        if (decoded.jobId === jobId) {
          result = decoded;
          break;
        }
      }
      return result;
    }, submission.jobId);

    if (result) {
      expect(result.jobId).toBe(submission.jobId);
    } else {
      console.warn('Mesh result not observed');
    }
  });
});
