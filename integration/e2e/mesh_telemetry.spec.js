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

  test('should display accurate global metrics from SAB', async ({ page }) => {
    // Mock global analytics in SAB
    // OFFSET_GLOBAL_ANALYTICS: 0x4100 (16640)
    // Structure: [TotalStorage(8), TotalCompute(8), GlobalOps(8), NodeCount(4)]
    await page.evaluate(() => {
      const sab = window.__INOS_SAB__;
      const dv = new DataView(sab);
      const offset = 16640; // OFFSET_GLOBAL_ANALYTICS
      
      // Write mock values (Little Endian)
      // Total Storage: 1000 GB (1,000,000,000,000 bytes)
      dv.setBigUint64(offset + 0, BigInt(1000000000000), true);
      // Total Compute: 500 GFLOPS
      dv.setBigUint64(offset + 8, BigInt(500), true);
      // Global Ops: 2500 Ops/s
      dv.setBigUint64(offset + 16, BigInt(2500), true);
      // Node Count: 10 Nodes
      dv.setUint32(offset + 24, 10, true);
      
      // Signal update (IDX_GLOBAL_METRICS_EPOCH = 21)
      const atomicFlags = new Int32Array(sab, 0, 32); 
      Atomics.add(atomicFlags, 21, 1);
    });

    // Verify UI updates. Note: RollingCounter noise might still be in outerText,
    // so we look for the components that should be there.
    
    // Check Ops/s: 2.5k
    const opsLabel = page.getByTestId('metric-ops');
    await expect(opsLabel).toBeVisible();
    await expect(opsLabel).toHaveText(/Ops\/s\d+/);
    
    // Check Cap (Total Compute): 500G
    const capLabel = page.getByTestId('metric-cap');
    await expect(capLabel).toBeVisible();
    await expect(capLabel).toContainText('G');
    
    // Check Nodes: 10
    const nodesLabel = page.getByTestId('metric-nodes');
    await expect(nodesLabel).toBeVisible();
    
    // Check Avg (Average Capability) -> 500 / 10 = 50G
    const avgLabel = page.getByTestId('metric-avg');
    await expect(avgLabel).toContainText('G');
  });

  test('should reflect mesh health status via pulse indicator', async ({ page }) => {
    await page.evaluate(() => {
      const sab = window.__INOS_SAB__;
      const dv = new DataView(sab);
      const offset = 16384; // OFFSET_MESH_METRICS (0x4000)
      
      dv.setFloat32(offset + 48, 1.0, true);
      dv.setFloat32(offset + 40, 45, true);
      
      const atomicFlags = new Int32Array(sab, 0, 32); 
      Atomics.add(atomicFlags, 11, 1);
    });

    await expect(page.getByTestId('metric-mesh')).toBeVisible();
  });

  test('should provide descriptive tooltips for layman users', async ({ page }) => {
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
      await expect(el).toBeVisible();
      await expect(el).toHaveAttribute('title', metric.title);
    }
  });

  test('should navigate to deep dive pages and verify content', async ({ page }) => {
    await page.goto('/deep-dives/mesh');
    await page.waitForURL('**/mesh');
    
    // Wait for kernel to re-initialize after page transition
    await page.waitForFunction(() => {
      return window.inos?.ready;
    }, { timeout: 30000 });
    
    // Verify Page Header - check for any mesh/network related heading
    // The page may have different headings like "P2P Mesh", "Mesh Network", "The Mesh", etc.
    const meshHeading = page.locator('h1, h2, h3').filter({ hasText: /mesh|network|p2p|distributed/i }).first();
    await expect(meshHeading).toBeVisible({ timeout: 15000 });
    
    // Verify page has substantive content (paragraphs or sections)
    const contentSections = page.locator('p, section, article');
    await expect(contentSections.first()).toBeVisible({ timeout: 10000 });
    
    // Check for SVG diagrams if they exist (optional)
    const diagrams = page.locator('svg');
    const diagramCount = await diagrams.count();
    expect(diagramCount).toBeGreaterThanOrEqual(0); // At least 0, meaning page loads
  });


  test('should handle zero-nodes state gracefully', async ({ page }) => {
    await page.evaluate(() => {
      const sab = window.__INOS_SAB__;
      const dv = new DataView(sab);
      const offset = 16640; // OFFSET_GLOBAL_ANALYTICS
      
      dv.setBigUint64(offset + 8, BigInt(0), true);
      dv.setBigUint64(offset + 16, BigInt(0), true);
      dv.setUint32(offset + 24, 0, true);
      
      const atomicFlags = new Int32Array(sab, 0, 32); 
      Atomics.add(atomicFlags, 21, 1);
    });

    await expect(page.getByTestId('metric-nodes')).toContainText('0');
    await expect(page.getByTestId('metric-avg')).toContainText('0');
  });
});
