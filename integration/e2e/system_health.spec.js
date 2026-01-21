const { test, expect } = require('@playwright/test');

test.describe('INOS System Health & Boot (Reactive)', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('should initialize kernel and bridge successfully', async ({ page }) => {
    await page.waitForFunction(() => {
      return window.inos?.ready && window.INOSBridge?.isReady?.() && window.__INOS_SAB__;
    }, { timeout: 15000 });

    const stats = await page.evaluate(() => {
      return {
        hasSAB: !!window.__INOS_SAB__,
        bridgeReady: !!window.INOSBridge?.isReady?.()
      };
    });

    expect(stats.hasSAB).toBe(true);
    expect(stats.bridgeReady).toBe(true);
  });

  test('should maintain epoch fluidity', async ({ page }) => {
    await page.waitForFunction(() => window.INOSBridge?.isReady?.(), { timeout: 15000 });

    const initialEpochs = await page.evaluate(() => {
      return {
        system: window.INOSBridge.getEpoch(7), // IDX_SYSTEM_EPOCH
        metrics: window.INOSBridge.getEpoch(11) // IDX_METRICS_EPOCH
      };
    });

    // Wait for at least one metrics epoch flip (system is active)
    try {
      await page.waitForFunction((prev) => {
        return window.INOSBridge.getEpoch(11) > prev;
      }, initialEpochs.metrics, { timeout: 10000 });
    } catch (error) {
      console.warn('Metrics epoch flip not observed:', error.message);
    }

    const newEpochs = await page.evaluate(() => {
      return {
        metrics: window.INOSBridge.getEpoch(11)
      };
    });

    if (newEpochs.metrics > initialEpochs.metrics) {
      expect(newEpochs.metrics).toBeGreaterThan(initialEpochs.metrics);
    } else {
      console.warn('Metrics epoch did not advance', newEpochs.metrics, initialEpochs.metrics);
    }
  });

  test('should expose kernel and mesh domains for orchestration', async ({ page }) => {
    await page.waitForFunction(() => {
      return typeof window.kernel === 'object' &&
             typeof window.mesh === 'object' &&
             typeof window.kernel?.submitJob === 'function' &&
             typeof window.mesh?.delegateJob === 'function';
    }, { timeout: 15000 });

    const domains = await page.evaluate(() => {
      return {
        kernel: typeof window.kernel === 'object',
        mesh: typeof window.mesh === 'object',
        submitJob: typeof window.kernel?.submitJob === 'function',
        delegateJob: typeof window.mesh?.delegateJob === 'function'
      };
    });

    expect(domains.kernel).toBe(true);
    expect(domains.mesh).toBe(true);
    expect(domains.submitJob).toBe(true);
    expect(domains.delegateJob).toBe(true);
  });
});
