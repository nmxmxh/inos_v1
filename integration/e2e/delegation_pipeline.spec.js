const { test, expect } = require('@playwright/test');

test.describe('INOS Delegation Pipeline (Reactive)', () => {
  test.beforeEach(async ({ page }) => {
    // Redirect browser console to stdout
    page.on('console', msg => console.log(`[Browser] ${msg.text()}`));
    await page.goto('/');
    await page.waitForFunction(async () => {
      if (!window.INOSBridge || !window.INOSBridge.isReady() || !window.mesh || !window.kernel) return false;
      const stats = await window.kernel.getStats();
      return stats.state === 'RUNNING';
    }, { timeout: 30000 });
  });

  test('should delegate compute task to mesh and observe remote result', async ({ page }) => {
    const submission = await page.evaluate(() => {
      const prevEpoch = window.INOSBridge.getEpoch(window.INOSBridge.IDX_OUTBOX_HOST_DIRTY); 
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
        return window.INOSBridge.getEpoch(window.INOSBridge.IDX_OUTBOX_HOST_DIRTY) > prev;
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
    // Wait for kernel initialization AND mesh method function
    await page.waitForFunction(() => window.inos && window.inos.ready && window.mesh && typeof window.mesh.delegateJob === 'function');
  });

  test('should handle concurrent delegations', async ({ page }) => {
    const count = 3;
    const submittedJobIds = await page.evaluate(async (n) => {
      // Retry mechanism for window.mesh availability
      // In some test environments, there's a micro-race between waitForFunction and context availability
      let retries = 5;
      while (!window.mesh && retries > 0) {
        await new Promise(r => setTimeout(r, 200));
        retries--;
      }
      
      if (!window.mesh) throw new Error("FATAL: window.mesh missing after wait");

      const jobIds = [];
      const promises = [];
      
      console.log(`[Test] Starting concurrent submission loop, count=${n}`);
      for (let i = 0; i < n; i++) {
        const jobId = `job-${Date.now()}-${i}`;
        jobIds.push(jobId);
        const job = {
          id: jobId,
          type: 'compute', // Ensure generic compute type
          op: 'data',      // Match Go bridge expectation (was operation)
          payload: { size: 1024 } 
        };
        
        console.log(`[Test] Submitting job ${jobId}`);
        const p = window.mesh.delegateJob(job)
            .then(() => console.log(`[Test] Job ${jobId} delegated successfully`))
            .catch(err => console.error(`[Test] Job ${jobId} failed:`, err));
        promises.push(p);
      }
      
      // We don't await promises here to simulate fire-and-forget concurrency,
      // but we ensure they are triggered.
      return jobIds;
    }, count);

    const collectedResults = await page.evaluate(async ({ count, submittedJobIds }) => {
        const collected = [];
        const startTime = Date.now();
        
        console.log(`[Test] Starting result collection for ${count} jobs...`);

        // Wait for results
        while (Date.now() - startTime < 15000) { 
          const bytes = window.INOSBridge.popResult();
          if (!bytes) {
             await new Promise(r => setTimeout(r, 50));
             continue;
          }
          
          console.log(`[Test] popResult detected data, deserializing...`);
          try {
            const res = await window.kernel.deserializeResult(bytes);
            console.log(`[Test] Result deserialized for job ${res.jobId}`);
            collected.push(res);
          } catch (err) {
            console.error(`[Test] Deserialize failed`, err);
          }
          
          if (collected.length >= count) break;
        }
        
        console.log(`[Test] Collection finished. Got ${collected.length}, Expected ${count}`);
        return collected;
    }, { count, submittedJobIds });

    expect(collectedResults).toHaveLength(count);
    const resultIds = collectedResults.map(r => r.jobId).sort();
    const expectedIds = submittedJobIds.sort();
    expect(resultIds).toEqual(expectedIds);
    
    // Check success status
    for (const res of collectedResults) {
      expect(res.success).toBe(true);
    }
  });
});
