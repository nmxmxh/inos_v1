const { test, expect } = require('@playwright/test');

test('Debug economics API', async ({ page }) => {
  // Enable console logging
  page.on('console', msg => {
    if (msg.type() === 'error' || msg.type() === 'warning' || msg.text().includes('Kernel') || msg.text().includes('Worker')) {
      console.log(`[Browser ${msg.type()}] ${msg.text()}`);
    }
  });

  await page.goto('/');
  
  // Wait for kernel modules to be loaded
  await page.waitForFunction(() => {
    return window.inos?.ready && window.INOSBridge?.isReady?.() && window.__INOS_SAB__;
  }, { timeout: 30000 });
  
  console.log('[DEBUG] Modules loaded, checking economics SAB...');
  
  // Check if SAB bridge is defined
  const hasBridge = await page.evaluate(() => {
    return {
      hasBridge: typeof window.INOSBridge !== 'undefined',
      bridgeReady: !!window.INOSBridge?.isReady?.(),
      hasSAB: !!window.__INOS_SAB__,
    };
  });
  
  console.log('[DEBUG] Bridge check:', JSON.stringify(hasBridge));
  expect(hasBridge.hasBridge).toBe(true);
  expect(hasBridge.bridgeReady).toBe(true);
  
  // Try reading balance directly from SAB
  const balanceResult = await page.evaluate(() => {
    try {
      if (!window.INOSBridge?.isReady?.() || !window.__INOS_SAB__) {
        return { error: 'bridge not ready' };
      }

      const OFFSET_ECONOMICS = 0x004200;
      const ECONOMICS_METADATA_SIZE = 64;
      const accountOffset = OFFSET_ECONOMICS + ECONOMICS_METADATA_SIZE;
      const balance = window.INOSBridge.readU64AsNumber(accountOffset);

      return { success: true, balance };
    } catch (e) {
      return { error: e.message, stack: e.stack };
    }
  });
  
  console.log('[DEBUG] Balance result:', JSON.stringify(balanceResult));
  expect(balanceResult.success).toBe(true);
});
