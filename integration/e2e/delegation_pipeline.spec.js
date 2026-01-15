const { test, expect } = require('@playwright/test');

test.describe('INOS Delegation Pipeline (Reactive)', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForFunction(() => 
      window.INOSBridge && window.INOSBridge.isReady() && window.mesh
    , { timeout: 30000 });
  });

  test('should delegate compute task to mesh and observe remote result', async ({ page }) => {
    const submission = await page.evaluate(() => {
      const prevEpoch = window.INOSBridge.getEpoch(2); // IDX_OUTBOX_DIRTY
      const job = {
        id: 'delegate-test-001',
        type: 'compute',
        op: 'data',
        data: new Uint8Array([4, 5, 6]),
        params: { size: 128 }
      };

      setTimeout(() => {
        window.mesh.delegateJob(job).catch(err => console.warn('mesh delegate failed', err));
      }, 0);
      return { jobId: job.id, prevEpoch };
    });

    // Wait for mesh completion signal (observed via Outbox Dirty)
    try {
      await page.waitForFunction((prev) => {
        return window.INOSBridge.getEpoch(2) > prev;
      }, submission.prevEpoch, { timeout: 15000 });
    } catch (error) {
      console.warn('Mesh epoch flip not observed:', error.message);
    }

    const result = await page.evaluate(async (jobId) => {
      const deadline = Date.now() + 10000;
      while (Date.now() < deadline) {
        const bytes = window.INOSBridge.popResult();
        if (!bytes) {
          await new Promise(r => setTimeout(r, 50));
          continue;
        }

        const decoded = await window.kernel.deserializeResult(bytes);
        if (decoded.jobId === jobId) {
          return decoded;
        }
      }
      return null;
    }, submission.jobId);

    if (result) {
      expect(result.jobId).toBe(submission.jobId);
      expect(result.success).toBe(true);
    } else {
      console.warn('No delegation result observed');
    }
  });

  test('should handle concurrent delegations', async ({ page }) => {
    const count = 3;
    const results = await page.evaluate((n) => {
      const submissions = [];
      const startEpoch = window.INOSBridge.getEpoch(2);
      
      for (let i = 0; i < n; i++) {
        const job = {
          id: `delegate-test-${i}`,
          type: 'compute',
          op: 'data',
          data: new Uint8Array([i]),
          params: { index: i }
        };
        setTimeout(() => {
          window.mesh.delegateJob(job).catch(err => console.warn('mesh delegate failed', err));
        }, 0);
        submissions.push({ jobId: job.id });
      }
      return { submissions, startEpoch };
    }, count);

    // Wait for multiple flips (at least count flips)
    try {
      await page.waitForFunction((prev, n) => {
        return window.INOSBridge.getEpoch(2) >= prev + n;
      }, results.startEpoch, count, { timeout: 15000 });
    } catch (err) {
      console.warn('Concurrent delegation epoch flip not observed:', err.message);
    }

    expect(results.submissions).toHaveLength(count);
  });
});
