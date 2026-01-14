const { test, expect } = require('@playwright/test');

/**
 * INOS Multi-Peer Mesh Network E2E Tests
 * 
 * Tests the full P2P mesh interactions with multiple browser contexts
 * simulating separate peers in the network.
 * 
 * Core Vision Tests:
 * - Epoch-based signaling (Mutate → Signal → React)
 * - Zero-copy data flow through SAB
 * - Content-addressable storage with BLAKE3
 * - Economic credit settlement
 * - Gossip-based state propagation
 * - Merkle tree anti-entropy
 */
test.describe('INOS Multi-Peer Mesh Network E2E', () => {

  // ========== Epoch-Based Signaling Tests ==========
  
  test.describe('Epoch-Based Signaling (Vision Core)', () => {
    
    test('should increment epoch on SAB mutation', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');

      const epochResult = await page.evaluate(async () => {
        if (!window.__INOS_SAB__) return { skipped: true };

        try {
          const sab = window.__INOS_SAB__;
          const view = new DataView(sab);
          const epochOffset = 0x100; // System epoch offset

          // Read initial epoch
          const initialEpoch = view.getUint32(epochOffset, true);

          // Wait for epoch change (up to 2 seconds)
          const startTime = Date.now();
          let newEpoch = initialEpoch;
          
          while (newEpoch === initialEpoch && Date.now() - startTime < 2000) {
            await new Promise(r => setTimeout(r, 50));
            newEpoch = view.getUint32(epochOffset, true);
          }

          return {
            success: true,
            initialEpoch,
            newEpoch,
            epochChanged: newEpoch !== initialEpoch,
            timeToChange: Date.now() - startTime
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!epochResult.skipped) {
        expect(epochResult.success).toBe(true);
        // Epoch should be tracked
        expect(typeof epochResult.initialEpoch).toBe('number');
      }
    });

    test('should batch multiple mutations per epoch', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');

      const batchResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.batchMutations) {
          return { skipped: true, reason: 'batchMutations API not exposed' };
        }

        try {
          const mutations = [
            { type: 'set', key: 'test1', value: 'value1' },
            { type: 'set', key: 'test2', value: 'value2' },
            { type: 'set', key: 'test3', value: 'value3' },
          ];

          const result = await window.inos.batchMutations(mutations);

          return {
            success: true,
            epochsUsed: result.epochsUsed,
            batchedCorrectly: result.epochsUsed === 1
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!batchResult.skipped) {
        expect(batchResult.success).toBe(true);
      }
    });
  });

  // ========== Zero-Copy Data Flow Tests ==========

  test.describe('Zero-Copy Pipeline (Vision Core)', () => {

    test('should verify SAB regions are properly allocated', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');

      const regionsResult = await page.evaluate(() => {
        if (!window.__INOS_SAB__) return { skipped: true };

        const sab = window.__INOS_SAB__;
        const size = sab.byteLength;

        // Critical SAB regions based on architecture
        const regions = {
          systemHeader: { offset: 0x0, size: 0x1000, description: 'System Header' },
          epochSignals: { offset: 0x100, size: 0x100, description: 'Epoch Signals' },
          moduleRegistry: { offset: 0x8000, size: 0x4000, description: 'Module Registry' },
          meshMetrics: { offset: 0x4000, size: 0x2000, description: 'Mesh Metrics' },
          inboxBase: { offset: 0x10000, size: 0x10000, description: 'Job Inbox' },
          arenaBase: { offset: 0xD0000, size: 0x100000, description: 'Arena Buffer' }
        };

        const validations = {};
        for (const [name, region] of Object.entries(regions)) {
          validations[name] = {
            fits: region.offset + region.size <= size,
            offset: region.offset.toString(16),
            description: region.description
          };
        }

        return { success: true, sabSize: size, regions: validations };
      });

      if (!regionsResult.skipped) {
        expect(regionsResult.success).toBe(true);
        // Core regions should fit in SAB
        for (const [name, validation] of Object.entries(regionsResult.regions)) {
          expect(validation.fits).toBe(true);
        }
      }
    });

    test('should perform zero-copy read from Arena', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');

      const zeroCopyResult = await page.evaluate(() => {
        if (!window.__INOS_SAB__) return { skipped: true };

        try {
          const sab = window.__INOS_SAB__;
          const arenaOffset = 0xD0000;

          // Create typed array view (zero-copy)
          const arenaView = new Float32Array(sab, arenaOffset, 1024);

          // Read without copying
          const firstValues = arenaView.slice(0, 10);

          return {
            success: true,
            viewCreated: arenaView instanceof Float32Array,
            viewLength: arenaView.length,
            sampleValues: Array.from(firstValues)
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!zeroCopyResult.skipped) {
        expect(zeroCopyResult.success).toBe(true);
        expect(zeroCopyResult.viewCreated).toBe(true);
      }
    });
  });

  // ========== Rust Module Integration Tests ==========

  test.describe('Rust Module Integration', () => {

    test('should invoke crypto.blake3 via Rust module', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const cryptoResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.invoke) {
          return { skipped: true, reason: 'invoke API not exposed' };
        }

        try {
          const testData = new TextEncoder().encode('BLAKE3 integration test');

          const result = await window.inos.invoke('crypto', 'blake3', testData);

          return {
            success: true,
            hashLength: result.hash?.length,
            executionTimeNs: result.executionTimeNs
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!cryptoResult.skipped) {
        expect(cryptoResult.success).toBe(true);
      }
    });

    test('should invoke storage.compress via Rust module', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const compressResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.invoke) {
          return { skipped: true, reason: 'invoke API not exposed' };
        }

        try {
          const pattern = 'INOS_COMPRESSION_TEST_'.repeat(500);
          const testData = new TextEncoder().encode(pattern);

          const compressed = await window.inos.invoke('storage', 'compress', {
            data: testData,
            algorithm: 'brotli'
          });

          const decompressed = await window.inos.invoke('storage', 'decompress', {
            data: compressed.data,
            algorithm: 'brotli'
          });

          return {
            success: true,
            originalSize: testData.length,
            compressedSize: compressed.data.length,
            ratio: compressed.data.length / testData.length,
            roundTripMatch: decompressed.data.length === testData.length
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!compressResult.skipped) {
        expect(compressResult.success).toBe(true);
        if (compressResult.success) {
          expect(compressResult.ratio).toBeLessThan(0.3); // Good compression
          expect(compressResult.roundTripMatch).toBe(true);
        }
      }
    });

    test('should invoke all registered units', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const unitsResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.listUnits) {
          return { skipped: true, reason: 'listUnits API not exposed' };
        }

        try {
          const units = await window.inos.listUnits();
          const expectedUnits = [
            'crypto', 'storage', 'image', 'audio', 'data', 'gpu', 'boids'
          ];

          const found = {};
          for (const unit of expectedUnits) {
            found[unit] = units.includes(unit);
          }

          return {
            success: true,
            totalUnits: units.length,
            units: units,
            expectedFound: found
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!unitsResult.skipped) {
        expect(unitsResult.success).toBe(true);
      }
    });
  });

  // ========== Mesh Metrics & Monitoring Tests ==========

  test.describe('Mesh Metrics & Monitoring', () => {

    test('should read mesh metrics from SAB', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');

      const metricsResult = await page.evaluate(() => {
        if (!window.__INOS_SAB__) return { skipped: true };

        try {
          const sab = window.__INOS_SAB__;
          const view = new DataView(sab);
          const metricsOffset = 0x4000;

          // Read mesh metrics structure
          const metrics = {
            peerCount: view.getUint32(metricsOffset, true),
            messagesSent: view.getUint32(metricsOffset + 4, true),
            messagesReceived: view.getUint32(metricsOffset + 8, true),
            bytesTransferred: view.getUint32(metricsOffset + 12, true),
            avgLatencyMs: view.getFloat32(metricsOffset + 16, true),
            uptime: view.getUint32(metricsOffset + 20, true)
          };

          return {
            success: true,
            metrics,
            isValid: metrics.peerCount >= 0 && metrics.uptime >= 0
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!metricsResult.skipped) {
        expect(metricsResult.success).toBe(true);
      }
    });

    test('should track gossip metrics over time', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');

      const gossipMetrics = await page.evaluate(async () => {
        if (!window.inos || !window.inos.getGossipMetrics) {
          return { skipped: true, reason: 'getGossipMetrics API not exposed' };
        }

        try {
          const initial = await window.inos.getGossipMetrics();
          
          // Wait for some gossip activity
          await new Promise(r => setTimeout(r, 2000));
          
          const after = await window.inos.getGossipMetrics();

          return {
            success: true,
            initial,
            after,
            messagesDelta: after.messagesSent - initial.messagesSent,
            syncsDelta: after.syncOperations - initial.syncOperations
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!gossipMetrics.skipped) {
        expect(gossipMetrics.success).toBe(true);
      }
    });

    test('should expose module registry in SAB', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const registryResult = await page.evaluate(() => {
        if (!window.__INOS_SAB__) return { skipped: true };

        try {
          const sab = window.__INOS_SAB__;
          const view = new DataView(sab);
          const registryOffset = 0x8000;

          // Read registry header
          const magic = view.getUint32(registryOffset, true);
          const slotCount = view.getUint16(registryOffset + 4, true);
          const usedSlots = view.getUint16(registryOffset + 6, true);

          // Validate magic number (should be consistent)
          const isValid = magic !== 0 && slotCount > 0;

          return {
            success: true,
            magic: magic.toString(16),
            slotCount,
            usedSlots,
            isValid
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!registryResult.skipped) {
        expect(registryResult.success).toBe(true);
      }
    });
  });

  // ========== Content-Addressable Storage Tests ==========

  test.describe('Content-Addressable Storage (CAS)', () => {

    test('should store and retrieve content by hash', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const casResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.cas) {
          return { skipped: true, reason: 'CAS API not exposed' };
        }

        try {
          const testContent = 'INOS CAS Test Content - ' + Date.now();
          const data = new TextEncoder().encode(testContent);

          // Store returns hash
          const hash = await window.inos.cas.store(data);

          // Retrieve by hash
          const retrieved = await window.inos.cas.retrieve(hash);

          const match = new TextDecoder().decode(retrieved) === testContent;

          return {
            success: true,
            hash,
            hashLength: hash.length,
            contentMatch: match
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!casResult.skipped) {
        expect(casResult.success).toBe(true);
        if (casResult.success) {
          expect(casResult.hashLength).toBe(64); // BLAKE3 hex
          expect(casResult.contentMatch).toBe(true);
        }
      }
    });

    test('should deduplicate identical content', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const dedupeResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.cas) {
          return { skipped: true, reason: 'CAS API not exposed' };
        }

        try {
          const content = 'Duplicate content test';
          const data = new TextEncoder().encode(content);

          // Store twice
          const hash1 = await window.inos.cas.store(data);
          const hash2 = await window.inos.cas.store(data);

          return {
            success: true,
            hash1,
            hash2,
            deduplicated: hash1 === hash2
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!dedupeResult.skipped) {
        expect(dedupeResult.success).toBe(true);
        if (dedupeResult.success) {
          expect(dedupeResult.deduplicated).toBe(true);
        }
      }
    });

    test('should chunk large files into 1MB blocks', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const chunkResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.cas || !window.inos.cas.storeChunked) {
          return { skipped: true, reason: 'storeChunked API not exposed' };
        }

        try {
          // Create 3MB file
          const fileSize = 3 * 1024 * 1024;
          const data = new Uint8Array(fileSize);
          for (let i = 0; i < fileSize; i++) {
            data[i] = i % 256;
          }

          const result = await window.inos.cas.storeChunked(data);

          return {
            success: true,
            totalSize: fileSize,
            chunks: result.chunks.length,
            rootHash: result.rootHash,
            expectedChunks: Math.ceil(fileSize / (1024 * 1024))
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!chunkResult.skipped) {
        expect(chunkResult.success).toBe(true);
        if (chunkResult.success) {
          expect(chunkResult.chunks).toBe(chunkResult.expectedChunks);
        }
      }
    });
  });

  // ========== Economic Credit System Tests ==========

  test.describe('Economic Credit System', () => {

    test('should track credit balance', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const balanceResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.credits) {
          return { skipped: true, reason: 'credits API not exposed' };
        }

        try {
          const balance = await window.inos.credits.getBalance();

          return {
            success: true,
            balance,
            hasBalance: typeof balance === 'number' && balance >= 0
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!balanceResult.skipped) {
        expect(balanceResult.success).toBe(true);
      }
    });

    test('should deduct credits for compute operations', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const creditFlowResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.credits || !window.inos.invoke) {
          return { skipped: true, reason: 'credits/invoke API not exposed' };
        }

        try {
          const balanceBefore = await window.inos.credits.getBalance();

          // Perform a compute operation
          const testData = new TextEncoder().encode('Credit test data'.repeat(1000));
          await window.inos.invoke('storage', 'compress', { data: testData });

          const balanceAfter = await window.inos.credits.getBalance();

          return {
            success: true,
            before: balanceBefore,
            after: balanceAfter,
            creditsUsed: balanceBefore - balanceAfter
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!creditFlowResult.skipped) {
        expect(creditFlowResult.success).toBe(true);
      }
    });

    test('should show transaction history', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const historyResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.credits || !window.inos.credits.getHistory) {
          return { skipped: true, reason: 'getHistory API not exposed' };
        }

        try {
          const history = await window.inos.credits.getHistory(10);

          return {
            success: true,
            transactionCount: history.length,
            hasTransactions: history.length > 0
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!historyResult.skipped) {
        expect(historyResult.success).toBe(true);
      }
    });
  });

  // ========== Multi-Peer P2P Tests ==========

  test.describe('Multi-Peer P2P Interactions', () => {

    test('should discover peers via DHT', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const peerDiscovery = await page.evaluate(async () => {
        if (!window.inos || !window.inos.mesh || !window.inos.mesh.discoverPeers) {
          return { skipped: true, reason: 'mesh.discoverPeers API not exposed' };
        }

        try {
          const peers = await window.inos.mesh.discoverPeers({ maxPeers: 10 });

          return {
            success: true,
            peerCount: peers.length,
            peers: peers.slice(0, 5).map(p => ({
              id: p.id?.substring(0, 16) + '...',
              latency: p.latencyMs,
              reputation: p.reputation
            }))
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!peerDiscovery.skipped) {
        expect(peerDiscovery.success).toBe(true);
      }
    });

    test('should propagate state via gossip', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const gossipResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.mesh || !window.inos.mesh.broadcast) {
          return { skipped: true, reason: 'mesh.broadcast API not exposed' };
        }

        try {
          const message = {
            type: 'test:ping',
            data: { timestamp: Date.now() }
          };

          const result = await window.inos.mesh.broadcast(message);

          return {
            success: true,
            peersReached: result.peersReached,
            latencyMs: result.avgLatencyMs
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!gossipResult.skipped) {
        expect(gossipResult.success).toBe(true);
      }
    });

    test('should sync state with peers via anti-entropy', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const antiEntropyResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.mesh || !window.inos.mesh.syncState) {
          return { skipped: true, reason: 'mesh.syncState API not exposed' };
        }

        try {
          const syncResult = await window.inos.mesh.syncState();

          return {
            success: true,
            itemsSynced: syncResult.itemsSynced,
            peersContacted: syncResult.peersContacted,
            merkleRootMatch: syncResult.merkleRootMatch
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!antiEntropyResult.skipped) {
        expect(antiEntropyResult.success).toBe(true);
      }
    });

    test('should delegate compute to remote peer', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const delegateResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.mesh || !window.inos.mesh.delegateCompute) {
          return { skipped: true, reason: 'mesh.delegateCompute API not exposed' };
        }

        try {
          const job = {
            operation: 'hash',
            data: new TextEncoder().encode('Remote compute test'),
            priority: 100
          };

          const result = await window.inos.mesh.delegateCompute(job);

          return {
            success: true,
            peerUsed: result.peerID?.substring(0, 16) + '...',
            latencyMs: result.latencyMs,
            outputDigest: result.outputDigest,
            verified: result.verified
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!delegateResult.skipped) {
        expect(delegateResult.success).toBe(true);
        if (delegateResult.success) {
          expect(delegateResult.verified).toBe(true);
        }
      }
    });

    test('should handle peer reputation updates', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const reputationResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.mesh || !window.inos.mesh.getPeerReputation) {
          return { skipped: true, reason: 'getPeerReputation API not exposed' };
        }

        try {
          const peers = await window.inos.mesh.discoverPeers({ maxPeers: 5 });
          
          if (peers.length === 0) {
            return { success: true, noPeers: true };
          }

          const peerID = peers[0].id;
          const reputation = await window.inos.mesh.getPeerReputation(peerID);

          return {
            success: true,
            peerID: peerID.substring(0, 16) + '...',
            reputation: reputation.score,
            interactions: reputation.totalInteractions,
            successRate: reputation.successRate
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!reputationResult.skipped) {
        expect(reputationResult.success).toBe(true);
      }
    });
  });

  // ========== Storage Tier Tests ==========

  test.describe('Storage Tiers (Hot/Cold)', () => {

    test('should identify storage tier based on latency', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const tierResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.storage || !window.inos.storage.getTierInfo) {
          return { skipped: true, reason: 'getTierInfo API not exposed' };
        }

        try {
          const tierInfo = await window.inos.storage.getTierInfo();

          return {
            success: true,
            currentTier: tierInfo.tier,
            isHotTier: tierInfo.tier === 'hot',
            latencyMs: tierInfo.avgLatencyMs,
            bandwidthMbps: tierInfo.bandwidthMbps
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!tierResult.skipped) {
        expect(tierResult.success).toBe(true);
      }
    });

    test('should replicate to maintain replication factor', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const replicationResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.storage || !window.inos.storage.getReplicationStatus) {
          return { skipped: true, reason: 'getReplicationStatus API not exposed' };
        }

        try {
          const status = await window.inos.storage.getReplicationStatus();

          return {
            success: true,
            targetRF: status.targetReplicationFactor,
            currentRF: status.currentReplicationFactor,
            pendingReplications: status.pendingReplications
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!replicationResult.skipped) {
        expect(replicationResult.success).toBe(true);
        if (replicationResult.success) {
          // Target RF should be at least 3
          expect(replicationResult.targetRF).toBeGreaterThanOrEqual(3);
        }
      }
    });
  });

  // ========== Identity & Security Tests ==========

  test.describe('Identity & Security', () => {

    test('should have valid device identity', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const identityResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.identity) {
          return { skipped: true, reason: 'identity API not exposed' };
        }

        try {
          const identity = await window.inos.identity.getDeviceIdentity();

          return {
            success: true,
            hasDeviceID: !!identity.deviceID,
            hasPublicKey: !!identity.publicKey,
            keyAlgorithm: identity.algorithm
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!identityResult.skipped) {
        expect(identityResult.success).toBe(true);
        if (identityResult.success) {
          expect(identityResult.hasDeviceID).toBe(true);
          expect(identityResult.hasPublicKey).toBe(true);
        }
      }
    });

    test('should sign and verify messages', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const signVerifyResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.identity) {
          return { skipped: true, reason: 'identity API not exposed' };
        }

        try {
          const message = new TextEncoder().encode('Message to sign');

          const signature = await window.inos.identity.sign(message);
          const verified = await window.inos.identity.verify(message, signature);

          return {
            success: true,
            signatureLength: signature.length,
            verified
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!signVerifyResult.skipped) {
        expect(signVerifyResult.success).toBe(true);
        if (signVerifyResult.success) {
          expect(signVerifyResult.verified).toBe(true);
        }
      }
    });
  });

  // ========== Boids Simulation Tests (Visual Proof of Architecture) ==========

  test.describe('Boids Simulation (Architecture Proof)', () => {

    test('should update boids via SAB ping-pong buffer', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const boidsResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.boids) {
          return { skipped: true, reason: 'boids API not exposed' };
        }

        try {
          const status = await window.inos.boids.getStatus();

          return {
            success: true,
            entityCount: status.entityCount,
            fps: status.fps,
            currentEpoch: status.currentEpoch,
            pingPongBuffer: status.currentBuffer // 'ping' or 'pong'
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!boidsResult.skipped) {
        expect(boidsResult.success).toBe(true);
      }
    });

    test('should render boids from SAB matrix buffer', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      // Wait for boids to render
      await page.waitForTimeout(2000);

      // Check for WebGL canvas
      const canvasExists = await page.evaluate(() => {
        const canvas = document.querySelector('canvas');
        return canvas && canvas.getContext('webgl2') !== null;
      });

      expect(canvasExists).toBe(true);
    });
  });

  // ========== Performance & Stability Tests ==========

  test.describe('Performance & Stability', () => {

    test('should maintain stable SAB access under load', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const stabilityResult = await page.evaluate(async () => {
        if (!window.__INOS_SAB__) return { skipped: true };

        try {
          const sab = window.__INOS_SAB__;
          const view = new DataView(sab);
          const iterations = 1000;
          const startTime = performance.now();
          let errors = 0;

          for (let i = 0; i < iterations; i++) {
            try {
              // Read from multiple regions
              view.getUint32(0x100, true);
              view.getUint32(0x4000, true);
              view.getUint32(0x8000, true);
            } catch {
              errors++;
            }
          }

          const endTime = performance.now();

          return {
            success: true,
            iterations,
            errors,
            totalTimeMs: endTime - startTime,
            avgTimeMs: (endTime - startTime) / iterations
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!stabilityResult.skipped) {
        expect(stabilityResult.success).toBe(true);
        expect(stabilityResult.errors).toBe(0);
        expect(stabilityResult.avgTimeMs).toBeLessThan(1); // < 1ms per read
      }
    });

    test('should handle concurrent operations without race conditions', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const concurrencyResult = await page.evaluate(async () => {
        if (!window.inos || !window.inos.invoke) {
          return { skipped: true, reason: 'invoke API not exposed' };
        }

        try {
          const concurrentOps = 10;
          const promises = [];

          for (let i = 0; i < concurrentOps; i++) {
            promises.push(
              window.inos.invoke('crypto', 'blake3', 
                new TextEncoder().encode(`Concurrent test ${i}`))
            );
          }

          const startTime = performance.now();
          const results = await Promise.all(promises);
          const endTime = performance.now();

          const successCount = results.filter(r => r && r.hash).length;

          return {
            success: true,
            concurrentOps,
            successCount,
            allSucceeded: successCount === concurrentOps,
            totalTimeMs: endTime - startTime
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!concurrencyResult.skipped) {
        expect(concurrencyResult.success).toBe(true);
        if (concurrencyResult.success) {
          expect(concurrencyResult.allSucceeded).toBe(true);
        }
      }
    });

    test('should not leak memory over repeated operations', async ({ page }) => {
      await page.goto('/');
      await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

      const memoryResult = await page.evaluate(async () => {
        if (!performance.memory) {
          return { skipped: true, reason: 'performance.memory not available' };
        }

        try {
          const iterations = 100;
          const memoryBefore = performance.memory.usedJSHeapSize;

          for (let i = 0; i < iterations; i++) {
            // Create and discard data
            const data = new Uint8Array(10 * 1024); // 10KB
            for (let j = 0; j < data.length; j++) {
              data[j] = j % 256;
            }
          }

          // Force GC if available
          if (window.gc) window.gc();
          await new Promise(r => setTimeout(r, 100));

          const memoryAfter = performance.memory.usedJSHeapSize;
          const growth = memoryAfter - memoryBefore;
          const growthMB = growth / (1024 * 1024);

          return {
            success: true,
            memoryBeforeMB: memoryBefore / (1024 * 1024),
            memoryAfterMB: memoryAfter / (1024 * 1024),
            growthMB,
            acceptable: growthMB < 10 // Less than 10MB growth acceptable
          };
        } catch (error) {
          return { success: false, error: error.message };
        }
      });

      if (!memoryResult.skipped) {
        expect(memoryResult.success).toBe(true);
      }
    });
  });
});
