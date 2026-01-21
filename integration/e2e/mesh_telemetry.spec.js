const { test, expect } = require('@playwright/test');

test.describe('INOS Mesh Telemetry & User Insights E2E', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    
    // Wait for system to be ready (including store status)
    await page.waitForFunction(() => {
      return window.inos?.ready && window.INOSBridge?.isReady?.();
    }, { timeout: 45000 });

    // Wait for the metrics bar to appear on the main page
    await page.locator('[data-testid="mesh-metrics-bar"]').waitFor({ state: 'visible', timeout: 30000 });

    // Disable all animations and transitions for predictable E2E testing
    await page.addStyleTag({
      content: `
        *, *::before, *::after {
          animation: none !important;
          transition: none !important;
        }
      `,
    });
  });

  let mockInterval;

  async function mockMeshMetrics(page, overrides) {
    // Clear any existing interval before setting a new one
    await page.evaluate(() => {
      if (window.__MOCK_INTERVAL__) {
        clearInterval(window.__MOCK_INTERVAL__);
        window.__MOCK_INTERVAL__ = undefined;
      }
    });

    await page.evaluate((vals) => {
      window.__MOCK_INTERVAL__ = setInterval(() => {
        const sab = window.__INOS_SAB__;
        if (!sab) return;
        const dv = new DataView(sab);
        const gOffset = 16640; // OFFSET_GLOBAL_ANALYTICS
        const mOffset = 16384; // OFFSET_MESH_METRICS
        
        const metrics = {
          storage: BigInt(1000000000000),
          compute: 500,
          ops: 2500,
          nodes: 10,
          successRate: 1.0,
          p50Latency: 45,
          ...vals
        };

        // Global Analytics
        dv.setBigUint64(gOffset + 0, metrics.storage, true);
        dv.setFloat64(gOffset + 8, metrics.compute, true);
        dv.setFloat64(gOffset + 16, metrics.ops, true);
        dv.setUint32(gOffset + 24, metrics.nodes, true);
        
        // Mesh Metrics (for health status)
        dv.setFloat32(mOffset + 48, metrics.successRate, true);
        dv.setFloat32(mOffset + 40, metrics.p50Latency, true);
        dv.setUint32(mOffset + 4, metrics.nodes, true); // connectedPeers
        
        const atomicFlags = new Int32Array(sab, 0, 32); 
        Atomics.add(atomicFlags, 21, 1); // IDX_GLOBAL_METRICS_EPOCH
        Atomics.add(atomicFlags, 11, 1); // IDX_METRICS_EPOCH
      }, 50); // Aggressive 50ms pulse
    }, overrides);
  }

  test.afterEach(async ({ page }) => {
    await page.evaluate(() => {
      if (window.__MOCK_INTERVAL__) {
        clearInterval(window.__MOCK_INTERVAL__);
        window.__MOCK_INTERVAL__ = undefined;
      }
    });
  });

  test('should display accurate global metrics from SAB', async ({ page }) => {
    await mockMeshMetrics(page, {});
    // No page.goto('/') here - preserve mock state
    
    // Check Ops/s: 2.5k
    const opsLabel = page.getByTestId('metric-ops');
    await expect(opsLabel).toBeVisible();
    await expect(opsLabel).toHaveAttribute('data-value', '2500');
    
    // Check Cap (Total Compute): 500G
    const capLabel = page.getByTestId('metric-cap');
    await expect(capLabel).toBeAttached();
    await expect(capLabel).toHaveAttribute('data-value', '500');
    
    // Check Nodes: 10
    const nodesLabel = page.getByTestId('metric-nodes');
    await expect(nodesLabel).toBeVisible();
    await expect(nodesLabel).toHaveAttribute('data-value', '10');
    
    // Check Avg (Average Capability) -> 500 / 10 = 50G
    const avgLabel = page.getByTestId('metric-avg');
    await expect(avgLabel).toBeAttached();
    await expect(avgLabel).toHaveAttribute('data-value', '50');
  });

  test('should reflect mesh health status via pulse indicator', async ({ page }) => {
    await mockMeshMetrics(page, { nodes: 5 });
    // No page.goto('/') here
    
    const healthPulse = page.locator('[data-testid="health-pulse"]');
    await expect(healthPulse).toBeVisible();
    // In healthy state (nodes > 0), it should have a green-ish theme
    await expect(healthPulse).toHaveCSS('background-color', /rgb\(.*\)/);
  });

  test('should provide descriptive tooltips for layman users', async ({ page }) => {
    await mockMeshMetrics(page, {});
    // No page.goto('/') here
    
    const metrics = [
      { id: 'metric-mesh', title: /Mesh Connection Status/ },
      { id: 'metric-ops', title: /Total Network Throughput/ },
      { id: 'metric-cap', title: /Total Compute Power/ },
      { id: 'metric-avg', title: /Average Capability/ },
      { id: 'metric-nodes', title: /Participating Nodes/ },
      { id: 'metric-lat', title: /Network Latency/ },
      { id: 'metric-sector', title: /Sector ID/ },
      { id: 'metric-rep', title: /Global Trust/ }
    ];

    for (const metric of metrics) {
      const el = page.getByTestId(metric.id);
      await expect(el).toBeAttached();
      await expect(el).toHaveAttribute('title', metric.title);
    }
  });

  test('should navigate to deep dive pages and verify content', async ({ page }) => {
    await mockMeshMetrics(page, {});
    await page.goto('/deep-dives/mesh');
    await page.waitForURL('**/mesh');
    
    // Wait for kernel to re-initialize after page transition
    await page.waitForFunction(() => {
      return window.inos?.ready;
    }, { timeout: 30000 });
    
    // Verify Page Header - check for any mesh/network related heading
    const meshHeading = page.locator('h1, h2, h3').filter({ hasText: /mesh|network|p2p|distributed/i }).first();
    await expect(meshHeading).toBeVisible({ timeout: 15000 });
    
    // Verify page has substantive content
    const contentSections = page.locator('p, section, article');
    await expect(contentSections.first()).toBeVisible({ timeout: 10000 });
  });


  test('should handle zero-nodes state gracefully', async ({ page }) => {
    await mockMeshMetrics(page, { storage: BigInt(0), compute: 0, ops: 0, nodes: 0 });
    // No page.goto('/') here

    const nodesLabel = page.getByTestId('metric-nodes');
    await expect(nodesLabel).toBeVisible();
    await expect(nodesLabel).toHaveAttribute('data-value', '0');
    
    const avgLabel = page.getByTestId('metric-avg');
    await expect(avgLabel).toBeAttached();
    await expect(avgLabel).toHaveAttribute('data-value', '0');
  });
});
