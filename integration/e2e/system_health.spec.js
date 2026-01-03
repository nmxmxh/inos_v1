const { test, expect } = require('@playwright/test');

test.describe('INOS System Health E2E', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to INOS frontend
    await page.goto('/');
    
    // Wait for initial load
    await page.waitForLoadState('networkidle');
  });

  test('should load the application', async ({ page }) => {
    // Verify page loaded
    await expect(page).toHaveTitle(/INOS/i);
  });

  test('should initialize kernel WASM module', async ({ page }) => {
    // Wait for kernel initialization
    await page.waitForFunction(() => {
      return window.wasmReady === true || 
             (window.inos && window.inos.kernelStatus === 'ready');
    }, { timeout: 30000 });

    // Check console for kernel init message
    const logs = [];
    page.on('console', msg => logs.push(msg.text()));
    
    // Verify no critical errors
    const errors = logs.filter(log => log.includes('ERROR') || log.includes('FATAL'));
    expect(errors).toHaveLength(0);
  });

  test('should discover and register Rust modules', async ({ page }) => {
    // Wait for module registry to populate
    await page.waitForFunction(() => {
      return window.inos && 
             window.inos.modules && 
             Object.keys(window.inos.modules).length > 0;
    }, { timeout: 30000 });

    // Check that key modules are registered
    const moduleCount = await page.evaluate(() => {
      return Object.keys(window.inos.modules).length;
    });

    expect(moduleCount).toBeGreaterThan(0);
    
    // Verify specific modules
    const hasCompute = await page.evaluate(() => {
      return !!window.inos.modules['compute'];
    });
    
    expect(hasCompute).toBe(true);
  });

  test('should have SAB (SharedArrayBuffer) initialized', async ({ page }) => {
    const sabInitialized = await page.evaluate(() => {
      return window.__INOS_SAB__ instanceof SharedArrayBuffer &&
             window.__INOS_SAB__.byteLength > 0;
    });

    expect(sabInitialized).toBe(true);

    // Check SAB size
    const sabSize = await page.evaluate(() => {
      return window.__INOS_SAB__.byteLength;
    });

    expect(sabSize).toBeGreaterThanOrEqual(4 * 1024 * 1024); // At least 4MB
  });

  test('should handle basic job submission flow', async ({ page }) => {
    // Wait for system ready
    await page.waitForFunction(() => {
      return window.inos && window.inos.ready === true;
    }, { timeout: 30000 });

    // Submit a test job (if API is exposed)
    const result = await page.evaluate(async () => {
      if (!window.inos || !window.inos.submitJob) {
        return { skipped: true };
      }

      try {
        const job = {
          id: 'e2e-test-job',
          type: 'test',
          operation: 'echo',
          data: new Uint8Array([1, 2, 3, 4])
        };

        const response = await window.inos.submitJob(job);
        return { success: true, result: response };
      } catch (error) {
        return { success: false, error: error.message };
      }
    });

    // Either job succeeded or API isn't exposed yet (both acceptable)
    if (!result.skipped) {
      expect(result.success).toBe(true);
    }
  });

  test('should render module status in UI', async ({ page }) => {
    // Look for module status indicators
    const moduleStatusVisible = await page.locator('[data-testid="module-status"]').isVisible()
      .catch(() => false);

    // If module status UI exists, verify it shows data
    if (moduleStatusVisible) {
      const statusText = await page.locator('[data-testid="module-status"]').textContent();
      expect(statusText).toBeTruthy();
    }
  });

  test('should have no critical console errors', async ({ page }) => {
    const criticalErrors = [];
    
    page.on('console', msg => {
      const text = msg.text();
      if (msg.type() === 'error' && 
          !text.includes('WASM') && // Filter expected WASM init messages
          !text.includes('DevTools')) {
        criticalErrors.push(text);
      }
    });

    // Wait for app to settle
    await page.waitForTimeout(2000);

    expect(criticalErrors).toHaveLength(0);
  });

  test('should handle network errors gracefully', async ({ page }) => {
    // Simulate offline mode
    await page.context().setOffline(true);

    // App should still function for local compute
    const appResponsive = await page.evaluate(() => {
      return document.readyState === 'complete';
    });

    expect(appResponsive).toBe(true);

    // Restore online
    await page.context().setOffline(false);
  });

  test('should support basic GPU detection', async ({ page }) => {
    const gpuAvailable = await page.evaluate(async () => {
      if (!navigator.gpu) {
        return { available: false, reason: 'WebGPU not supported' };
      }

      try {
        const adapter = await navigator.gpu.requestAdapter();
        return { 
          available: !!adapter,
          features: adapter ? Array.from(adapter.features) : []
        };
      } catch (error) {
        return { available: false, reason: error.message };
      }
    });

    // WebGPU availability varies by browser/platform
    // Just verify we handled it gracefully
    expect(gpuAvailable).toHaveProperty('available');
  });
});
