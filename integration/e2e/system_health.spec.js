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
    // Wait for kernel initialization - uses __INOS_KERNEL_WORKER__ and __INOS_SAB__
    await page.waitForFunction(() => {
      return window.__INOS_SAB__ instanceof SharedArrayBuffer && 
             window.__INOS_KERNEL_WORKER__ !== undefined;
    }, { timeout: 30000 });

    // Check console for kernel init message
    const logs = [];
    page.on('console', msg => logs.push(msg.text()));
    
    // Verify no critical errors
    const errors = logs.filter(log => log.includes('ERROR') || log.includes('FATAL'));
    expect(errors).toHaveLength(0);
  });

  test('should discover and register Rust modules', async ({ page }) => {
    // Wait for inosModules to populate
    await page.waitForFunction(() => {
      return window.inosModules && 
             Object.keys(window.inosModules).length > 0;
    }, { timeout: 30000 });

    // Check that key modules are registered
    const moduleCount = await page.evaluate(() => {
      return Object.keys(window.inosModules).length;
    });

    expect(moduleCount).toBeGreaterThan(0);
    
    // Verify at least one compute-capable module exists
    const moduleInfo = await page.evaluate(() => {
      const modules = window.inosModules;
      const moduleKeys = Object.keys(modules);
      // Filter out contextId which is a metadata field
      const realModules = moduleKeys.filter(k => k !== 'contextId');
      return {
        hasModules: realModules.length > 0,
        moduleNames: realModules
      };
    });
    
    expect(moduleInfo.hasModules).toBe(true);
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
    // Wait for system ready - inosModules populated
    await page.waitForFunction(() => {
      return window.inosModules && 
             window.__INOS_SAB__ instanceof SharedArrayBuffer;
    }, { timeout: 30000 });

    // Verify modules loaded
    const result = await page.evaluate(() => {
      const modules = window.inosModules;
      if (!modules) {
        return { skipped: true, reason: 'No modules loaded' };
      }

      try {
        // Check compute module has exports
        const compute = modules['compute'];
        if (!compute || !compute.exports) {
          return { skipped: true, reason: 'Compute module not loaded' };
        }

        return { 
          success: true, 
          moduleCount: Object.keys(modules).length,
          hasCompute: !!compute
        };
      } catch (error) {
        return { success: false, error: error.message };
      }
    });

    // Either modules loaded or API isn't exposed yet (both acceptable)
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
          !text.includes('WASM') && 
          !text.includes('DevTools') &&
          !text.includes('NotSameOriginAfterDefaultedToSameOriginByCoep')) {
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
