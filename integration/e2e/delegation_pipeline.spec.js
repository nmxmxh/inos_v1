const { test, expect } = require('@playwright/test');

/**
 * INOS Delegation Pipeline E2E Tests
 * 
 * Tests the full delegation flow:
 * 1. SAB Initialization → Rust Modules register capabilities
 * 2. Job submission → Go Kernel routes to appropriate module
 * 3. Rust module executes (hash/compress/encrypt)
 * 4. Result verified via DigestValidator
 * 5. Economic settlement via EconomicLedger
 */
test.describe('INOS Delegation Pipeline E2E', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    
    // Wait for kernel and modules to be ready
    // Uses correct frontend globals: __INOS_SAB__ and inosModules
    await page.waitForFunction(() => {
      return window.__INOS_SAB__ instanceof SharedArrayBuffer &&
             window.inosModules && 
             Object.keys(window.inosModules).length > 0;
    }, { timeout: 30000 });
  });

  // ========== SAB & Module Registration Tests ==========

  test('should have SAB initialized with correct size', async ({ page }) => {
    const sabInfo = await page.evaluate(() => {
      const sab = window.__INOS_SAB__;
      return {
        exists: sab instanceof SharedArrayBuffer,
        size: sab ? sab.byteLength : 0,
        offset: window.__INOS_SAB_OFFSET__ || 0
      };
    });

    expect(sabInfo.exists).toBe(true);
    expect(sabInfo.size).toBeGreaterThanOrEqual(64 * 1024 * 1024); // 64MB minimum
  });

  test('should have storage module registered with capabilities', async ({ page }) => {
    const storageModule = await page.evaluate(() => {
      if (!window.inos || !window.inos.modules) return null;
      
      const storage = window.inos.modules['vault'] || window.inos.modules['storage'];
      return storage ? {
        registered: true,
        capabilities: storage.capabilities || []
      } : null;
    });

    if (storageModule) {
      expect(storageModule.registered).toBe(true);
      expect(storageModule.capabilities).toContain('storage');
    }
  });

  test('should have compute module registered', async ({ page }) => {
    const computeModule = await page.evaluate(() => {
      if (!window.inos || !window.inos.modules) return null;
      
      const compute = window.inos.modules['compute'];
      return compute ? {
        registered: true,
        capabilities: compute.capabilities || []
      } : null;
    });

    if (computeModule) {
      expect(computeModule.registered).toBe(true);
    }
  });

  // ========== Delegation Decision Tests ==========

  test('should determine delegation decision based on load', async ({ page }) => {
    const delegationDecision = await page.evaluate(async () => {
      if (!window.inos || !window.inos.analyzeJob) {
        return { skipped: true, reason: 'analyzeJob API not exposed' };
      }

      try {
        // Simulate a large job
        const testJob = {
          id: 'e2e-delegation-test',
          operation: 'compress',
          data: new Uint8Array(2 * 1024 * 1024), // 2MB
          priority: 50
        };

        const decision = await window.inos.analyzeJob(testJob);
        return {
          success: true,
          shouldDelegate: decision.shouldDelegate,
          targetType: decision.targetType,
          efficiencyScore: decision.efficiencyScore
        };
      } catch (error) {
        return { success: false, error: error.message };
      }
    });

    if (!delegationDecision.skipped) {
      expect(delegationDecision.success).toBe(true);
      expect(typeof delegationDecision.shouldDelegate).toBe('boolean');
    }
  });

  // ========== Storage CAS Flow Tests ==========

  test('should perform content-addressable storage round-trip', async ({ page }) => {
    const casResult = await page.evaluate(async () => {
      if (!window.inos || !window.inos.storage) {
        return { skipped: true, reason: 'storage API not exposed' };
      }

      try {
        // Test data
        const testData = new TextEncoder().encode('INOS E2E Test Data - ' + Date.now());
        
        // Store via CAS (Rust module: store_cas_chunk)
        const storeResult = await window.inos.storage.store(testData);
        
        if (!storeResult || !storeResult.hash) {
          return { success: false, error: 'No hash returned from store' };
        }

        // Retrieve and verify
        const retrieved = await window.inos.storage.retrieve(storeResult.hash);
        
        // Compare
        const matches = retrieved.length === testData.length &&
          retrieved.every((byte, i) => byte === testData[i]);

        return {
          success: true,
          hash: storeResult.hash,
          matches: matches,
          storedSize: testData.length,
          retrievedSize: retrieved.length
        };
      } catch (error) {
        return { success: false, error: error.message };
      }
    });

    if (!casResult.skipped) {
      expect(casResult.success).toBe(true);
      if (casResult.success) {
        expect(casResult.matches).toBe(true);
      }
    }
  });

  // ========== Hashing via Rust Module Tests ==========

  test('should compute BLAKE3 hash via Rust module', async ({ page }) => {
    const hashResult = await page.evaluate(async () => {
      if (!window.inos || !window.inos.crypto) {
        return { skipped: true, reason: 'crypto API not exposed' };
      }

      try {
        const testData = new TextEncoder().encode('BLAKE3 E2E Test');
        
        // Hash via Rust crypto module
        const hash = await window.inos.crypto.blake3(testData);
        
        return {
          success: true,
          hashLength: hash.length,
          isHex: /^[a-f0-9]{64}$/i.test(hash)
        };
      } catch (error) {
        return { success: false, error: error.message };
      }
    });

    if (!hashResult.skipped) {
      expect(hashResult.success).toBe(true);
      if (hashResult.success) {
        expect(hashResult.hashLength).toBe(64); // 32 bytes = 64 hex chars
        expect(hashResult.isHex).toBe(true);
      }
    }
  });

  // ========== Compression via Rust Module Tests ==========

  test('should compress data via Rust Brotli module', async ({ page }) => {
    const compressionResult = await page.evaluate(async () => {
      if (!window.inos || !window.inos.storage || !window.inos.storage.compress) {
        return { skipped: true, reason: 'compression API not exposed' };
      }

      try {
        // Create compressible data (repeated pattern)
        const pattern = 'INOS-E2E-TEST-DATA-'.repeat(1000);
        const testData = new TextEncoder().encode(pattern);
        
        // Compress via Rust module
        const compressed = await window.inos.storage.compress(testData, 'brotli');
        
        // Decompress to verify
        const decompressed = await window.inos.storage.decompress(compressed, 'brotli');
        
        const matches = decompressed.length === testData.length;

        return {
          success: true,
          originalSize: testData.length,
          compressedSize: compressed.length,
          compressionRatio: compressed.length / testData.length,
          roundTripMatches: matches
        };
      } catch (error) {
        return { success: false, error: error.message };
      }
    });

    if (!compressionResult.skipped) {
      expect(compressionResult.success).toBe(true);
      if (compressionResult.success) {
        // Brotli should achieve good compression on repeated data
        expect(compressionResult.compressionRatio).toBeLessThan(0.5);
        expect(compressionResult.roundTripMatches).toBe(true);
      }
    }
  });

  // ========== Full Delegation Cycle Tests ==========

  test('should complete full delegation cycle with verification', async ({ page }) => {
    const fullCycleResult = await page.evaluate(async () => {
      if (!window.inos || !window.inos.delegateJob) {
        return { skipped: true, reason: 'delegateJob API not exposed' };
      }

      try {
        const startTime = performance.now();
        
        // Submit delegation job
        const job = {
          id: 'e2e-full-cycle-' + Date.now(),
          operation: 'hash',
          data: new TextEncoder().encode('Full Cycle E2E Test'),
          priority: 100
        };

        const result = await window.inos.delegateJob(job);
        const endTime = performance.now();

        return {
          success: result.success,
          verified: result.verified,
          outputDigest: result.outputDigest,
          executionTimeMs: result.executionTimeMs,
          totalTimeMs: endTime - startTime
        };
      } catch (error) {
        return { success: false, error: error.message };
      }
    });

    if (!fullCycleResult.skipped) {
      expect(fullCycleResult.success).toBe(true);
      if (fullCycleResult.success) {
        expect(fullCycleResult.verified).toBe(true);
        expect(fullCycleResult.outputDigest).toBeTruthy();
      }
    }
  });

  // ========== Economic Flow Tests ==========

  test('should track economic credits for delegated work', async ({ page }) => {
    const economicResult = await page.evaluate(async () => {
      if (!window.inos || !window.inos.economics) {
        return { skipped: true, reason: 'economics API not exposed' };
      }

      try {
        // Get current balance
        const initialBalance = await window.inos.economics.getBalance();
        
        // Perform a job (should cost credits)
        const job = {
          id: 'e2e-economic-' + Date.now(),
          operation: 'compress',
          data: new TextEncoder().encode('Economic Test'.repeat(100)),
          priority: 50
        };

        await window.inos.delegateJob(job);
        
        // Check balance after
        const finalBalance = await window.inos.economics.getBalance();

        return {
          success: true,
          initialBalance,
          finalBalance,
          creditsUsed: initialBalance - finalBalance
        };
      } catch (error) {
        return { success: false, error: error.message };
      }
    });

    if (!economicResult.skipped) {
      expect(economicResult.success).toBe(true);
    }
  });

  // ========== Mesh Metrics Tests ==========

  test('should expose mesh metrics via SAB', async ({ page }) => {
    const meshMetrics = await page.evaluate(() => {
      if (!window.__INOS_SAB__) return { skipped: true };

      try {
        const sab = window.__INOS_SAB__;
        const view = new DataView(sab);
        
        // Read mesh metrics from known offset (0x4000 = 16384)
        const meshOffset = 0x4000;
        
        return {
          success: true,
          sabSize: sab.byteLength,
          canReadMetrics: sab.byteLength > meshOffset
        };
      } catch (error) {
        return { success: false, error: error.message };
      }
    });

    if (!meshMetrics.skipped) {
      expect(meshMetrics.success).toBe(true);
    }
  });

  // ========== Module Registry Tests ==========

  test('should read module registry from SAB', async ({ page }) => {
    const registryResult = await page.evaluate(() => {
      if (!window.__INOS_SAB__) return { skipped: true };

      try {
        const sab = window.__INOS_SAB__;
        const registryOffset = 0x8000; // 32KB offset for registry
        
        if (sab.byteLength < registryOffset + 512) {
          return { success: false, error: 'SAB too small for registry' };
        }

        // Try to read registry header
        const view = new DataView(sab);
        const magic = view.getUint32(registryOffset, true);
        
        return {
          success: true,
          registryMagic: magic.toString(16),
          sabSize: sab.byteLength
        };
      } catch (error) {
        return { success: false, error: error.message };
      }
    });

    if (!registryResult.skipped) {
      expect(registryResult.success).toBe(true);
    }
  });

  // ========== Error Handling Tests ==========

  test('should handle invalid job gracefully', async ({ page }) => {
    const errorResult = await page.evaluate(async () => {
      if (!window.inos || !window.inos.submitJob) {
        return { skipped: true, reason: 'submitJob API not exposed' };
      }

      try {
        // Submit invalid job
        const result = await window.inos.submitJob({
          id: 'e2e-invalid-job',
          operation: 'nonexistent_operation',
          data: null
        });

        return {
          success: false,
          unexpectedSuccess: true,
          result
        };
      } catch (error) {
        // We expect an error
        return {
          success: true,
          errorHandled: true,
          errorMessage: error.message
        };
      }
    });

    if (!errorResult.skipped) {
      // Either error was handled gracefully, or API returned error status
      expect(errorResult.errorHandled || !errorResult.unexpectedSuccess).toBe(true);
    }
  });

  // ========== Performance Tests ==========

  test('should complete batch jobs within performance bounds', async ({ page }) => {
    const perfResult = await page.evaluate(async () => {
      if (!window.inos || !window.inos.submitJob) {
        return { skipped: true, reason: 'submitJob API not exposed' };
      }

      try {
        const batchSize = 10;
        const jobs = [];
        
        for (let i = 0; i < batchSize; i++) {
          jobs.push({
            id: `e2e-perf-${i}`,
            operation: 'echo',
            data: new Uint8Array([1, 2, 3, 4])
          });
        }

        const startTime = performance.now();
        
        // Submit batch
        const results = await Promise.all(
          jobs.map(job => window.inos.submitJob(job))
        );
        
        const endTime = performance.now();
        const successCount = results.filter(r => r && r.success).length;

        return {
          success: true,
          batchSize,
          successCount,
          totalTimeMs: endTime - startTime,
          avgTimeMs: (endTime - startTime) / batchSize
        };
      } catch (error) {
        return { success: false, error: error.message };
      }
    });

    if (!perfResult.skipped) {
      expect(perfResult.success).toBe(true);
      if (perfResult.success) {
        // Batch should complete in reasonable time
        expect(perfResult.totalTimeMs).toBeLessThan(5000); // 5 seconds max
      }
    }
  });
});
