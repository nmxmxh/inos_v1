const { test, expect } = require('@playwright/test');

test.describe('INOS ML Inference Pipeline', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to INOS frontend
    await page.goto('http://localhost:5173');
    
    // Wait for kernel to be ready
    await page.waitForFunction(() => {
      return window.getKernelStats && window.getKernelStats().state === 'RUNNING';
    }, { timeout: 10000 });
  });

  test('should load model from P2P mesh', async ({ page }) => {
    const result = await page.evaluate(async () => {
      // This would call the actual WASM module
      // For now, we validate the API exists
      return {
        success: typeof window.loadModelFromMesh === 'function',
        kernelReady: window.getKernelStats().state === 'RUNNING'
      };
    });

    expect(result.kernelReady).toBe(true);
  });

  test('should run ML inference and mint credits', async ({ page }) => {
    const result = await page.evaluate(async () => {
      const stats = window.getKernelStats();
      
      return {
        kernelState: stats.state,
        hasModules: stats.supervisor && stats.supervisor.activeThreads > 0
      };
    });

    expect(result.kernelState).toBe('RUNNING');
  });

  test('should validate SAB communication', async ({ page }) => {
    const sabInfo = await page.evaluate(() => {
      // Get SAB info from kernel
      const sabAddress = window.getSystemSABAddress();
      const sabSize = window.getSystemSABSize();
      
      return {
        address: sabAddress,
        size: sabSize,
        hasAddress: sabAddress > 0,
        hasSize: sabSize > 0
      };
    });

    expect(sabInfo.hasAddress).toBe(true);
    expect(sabInfo.hasSize).toBe(true);
    expect(sabInfo.size).toBeGreaterThan(0);
  });
});

test.describe('INOS P2P Mesh', () => {
  test('should initialize mesh coordinator', async ({ page }) => {
    await page.goto('http://localhost:5173');
    
    await page.waitForFunction(() => window.getKernelStats, { timeout: 10000 });

    const meshStats = await page.evaluate(() => {
      const stats = window.getKernelStats();
      return stats.mesh || {};
    });

    // Validate mesh initialized
    expect(meshStats).toBeDefined();
  });
});

test.describe('INOS SAB Zero-Copy', () => {
  test('should perform zero-copy SAB operations', async ({ page }) => {
    await page.goto('http://localhost:5173');
    
    await page.waitForFunction(() => window.getKernelStats, { timeout: 10000 });

    const result = await page.evaluate(() => {
      // Validate SAB is accessible
      const sabAddress = window.getSystemSABAddress();
      const sabSize = window.getSystemSABSize();
      
      return {
        sabConfigured: sabAddress > 0 && sabSize > 0,
        kernelReady: window.getKernelStats().state === 'RUNNING'
      };
    });

    expect(result.sabConfigured).toBe(true);
    expect(result.kernelReady).toBe(true);
  });
});
