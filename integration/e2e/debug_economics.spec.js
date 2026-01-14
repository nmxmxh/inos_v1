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
    return window.inosModules && Object.keys(window.inosModules).length > 0;
  }, { timeout: 30000 });
  
  console.log('[DEBUG] Modules loaded, checking economics...');
  
  // Check if economics is defined
  const hasEconomics = await page.evaluate(() => {
    return {
      hasEconomics: typeof window.economics !== 'undefined',
      economicsType: typeof window.economics,
      hasGetBalance: typeof window.economics?.getBalance === 'function',
      hasGrantBonus: typeof window.economics?.grantBonus === 'function',
    };
  });
  
  console.log('[DEBUG] Economics check:', JSON.stringify(hasEconomics));
  expect(hasEconomics.hasEconomics).toBe(true);
  expect(hasEconomics.hasGetBalance).toBe(true);
  
  // Try calling getBalance directly with timeout
  const balanceResult = await page.evaluate(async () => {
    try {
      if (!window.economics) return { error: 'economics not defined' };
      const timeoutPromise = new Promise((_, reject) => 
        setTimeout(() => reject(new Error('Timeout after 5s')), 5000)
      );
      const balance = await Promise.race([
        window.economics.getBalance(),
        timeoutPromise
      ]);
      return { success: true, balance };
    } catch (e) {
      return { error: e.message, stack: e.stack };
    }
  });
  
  console.log('[DEBUG] Balance result:', JSON.stringify(balanceResult));
  expect(balanceResult.success).toBe(true);
});
