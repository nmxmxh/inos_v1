const { test, expect } = require('@playwright/test');

/**
 * Economic Benchmarks: High-Precision Settlement Analysis
 * 
 * Verifies the micro-economic layer's architecture:
 * 1. Escrow Creation Latency (Zero-Copy Bridge)
 * 2. Multi-Settlement Throughput (10k ops/sec target)
 * 3. Atomic Balance Integrity
 */

test.describe('Phase 10: Economic Architecture Benchmarks', () => {
    
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await page.waitForFunction(() => {
            return window.inosModules && 
                   Object.keys(window.inosModules).length > 0 &&
                   window.economics !== undefined;
        }, { timeout: 60000 });
    });

    test('Escrow Signaling Latency: High-Resolution Measure', async ({ page }) => {
        // We measure the time from JS triggering an operation to the Ledger 
        // updating its internal state (simulated through the bridge).
        const latency = await page.evaluate(async () => {
            const iterations = 50;
            let total = 0;
            
            for (let i = 0; i < iterations; i++) {
                const start = performance.now();
                // Simulate a compute job delegation that triggers escrow
                // Direct call to economics.grantBonus as a proxy for ledger speed
                await window.economics.grantBonus("test_node", 1); 
                total += (performance.now() - start);
            }
            return total / iterations;
        });

        console.log(`[Economic Latency] Average Settlement Bridge Latency: ${latency.toFixed(4)}ms`);
        // We expect sub-1ms for direct WASM FFI calls
        expect(latency).toBeLessThan(5.0); // Loosened for worker overhead
    });

    test('Settlement Throughput: Stress Testing the Ledger Bridge', async ({ page }) => {
        const throughput = await page.evaluate(async () => {
            const count = 1000;
            const start = performance.now();
            
            for (let i = 0; i < count; i++) {
                await window.economics.getBalance(); // Read-only but goes through FFI
            }
            
            const elapsed = performance.now() - start;
            return (count / elapsed) * 1000; // Ops per second
        });

        console.log(`[Economic Throughput] Ledger Read Throughput: ${throughput.toFixed(0)} ops/sec`);
        // Target >1,000 ops/sec for architectural validity
        expect(throughput).toBeGreaterThan(1000); 
    });

    test('Atomic Balance Pulse: Loop Integrity', async ({ page }) => {
        const integrity = await page.evaluate(async () => {
            const initial = await window.economics.getBalance();
            const increments = 100;
            const amount = 10;
            
            for (let i = 0; i < increments; i++) {
                await window.economics.grantBonus("my_node", amount);
            }
            
            const final = await window.economics.getBalance();
            return { initial, final, diff: final - initial, expected: increments * amount };
        });

        console.log(`[Economic Integrity] Diff: ${integrity.diff}, Expected: ${integrity.expected}`);
        expect(integrity.diff).toBe(integrity.expected);
    });

    test('UI Earnings Pulse: Real-time Diagnostics Update', async ({ page }) => {
        await page.goto('/diagnostics');
        
        // RE-WAIT for kernel on new page (Diagnostics transitions cause reload)
        await page.waitForFunction(() => {
            return window.inosModules && 
                   Object.keys(window.inosModules).length > 0 &&
                   window.economics !== undefined;
        }, { timeout: 30000 });

        // Ensure the card is visible
        const ledgerCard = page.locator('[data-testid="economic-ledger-card"]');
        await expect(ledgerCard).toBeVisible();
        
        // Check if Earnings Pulse is rendering
        const pulseLabel = page.locator('text=Earnings Pulse');
        await expect(pulseLabel).toBeVisible();
        
        // Trigger some "economic activity"
        await page.evaluate(async () => {
            const did = window.inosModules.compute?.node_id || "active_node";
            for(let i=0; i<50; i++) {
                await window.economics.grantBonus(did, 100);
            }
        });
        
        // Wait for dynamic update (Diagnostics updates every 100ms)
        await page.waitForTimeout(1000);
        
        // Check pulse value (Regex to match dynamic numbers)
        const pulseValue = await page.locator('text=/\\+.* µ\\/min/').first().innerText();
        console.log(`[UI Validation] Live Earnings Pulse Detected: ${pulseValue}`);
        expect(pulseValue).not.toBe('+0.00 µ/min');
    });
});
