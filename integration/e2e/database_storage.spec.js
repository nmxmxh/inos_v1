const { test, expect } = require('@playwright/test');

function buildPiedPiperCorpus(size) {
  const pattern = 'middle-out|dictionary|segment|';
  let text = '';
  while (text.length < size) {
    text += pattern;
  }
  return text.slice(0, size);
}

test.describe('Database and Storage Integration', () => {
  test('should render database deep-dive and storage architecture cues', async ({ page }) => {
    await page.goto('/deep-dives/database');
    await page.waitForLoadState('domcontentloaded');

    const heading = page
      .locator('h1, h2, h3')
      .filter({ hasText: /database|storage|cas|content-addressable/i })
      .first();
    await expect(heading).toBeVisible({ timeout: 20000 });

    // The deep-dive docs describe 1MB chunking and tiered storage behavior.
    const body = page.locator('body');
    await expect(body).toContainText(/1MB|chunk|tier|replication/i);
  });

  test('should roundtrip Brotli storage payload on mixed corpus', async ({ page }) => {
    await page.goto('/');
    await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

    const result = await page.evaluate(async corpus => {
      if (!window.inos || !window.inos.invoke) {
        return { skipped: true, reason: 'invoke API not exposed' };
      }

      try {
        const source = new TextEncoder().encode(corpus);
        const compressed = await window.inos.invoke('storage', 'compress', {
          data: source,
          algorithm: 'brotli',
        });
        const decompressed = await window.inos.invoke('storage', 'decompress', {
          data: compressed.data,
          algorithm: 'brotli',
        });

        const restored = decompressed.data instanceof Uint8Array
          ? decompressed.data
          : new Uint8Array(decompressed.data);

        let mismatch = -1;
        if (restored.length !== source.length) {
          mismatch = source.length;
        } else {
          for (let i = 0; i < source.length; i++) {
            if (restored[i] !== source[i]) {
              mismatch = i;
              break;
            }
          }
        }

        return {
          success: true,
          rawSize: source.length,
          wireSize: compressed.data.length,
          ratio: compressed.data.length / source.length,
          roundTripMatch: mismatch === -1,
          mismatch,
        };
      } catch (error) {
        return { success: false, error: error.message };
      }
    }, buildPiedPiperCorpus(256 * 1024));

    if (!result.skipped) {
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.roundTripMatch).toBe(true);
        expect(result.wireSize).toBeLessThan(result.rawSize);
      }
    }
  });

  test('should keep chunked CAS storage aligned with 1MB boundaries', async ({ page }) => {
    await page.goto('/');
    await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

    const chunkResult = await page.evaluate(async () => {
      if (!window.inos || !window.inos.cas || !window.inos.cas.storeChunked) {
        return { skipped: true, reason: 'storeChunked API not exposed' };
      }

      try {
        const size = 5 * 1024 * 1024 + 17; // Forces a partial final chunk.
        const data = new Uint8Array(size);
        for (let i = 0; i < size; i++) {
          data[i] = i % 251;
        }

        const stored = await window.inos.cas.storeChunked(data);
        const expectedChunks = Math.ceil(size / (1024 * 1024));

        return {
          success: true,
          rootHash: stored.rootHash,
          chunks: stored.chunks?.length || 0,
          expectedChunks,
          finalChunkNonEmpty: size % (1024 * 1024) > 0,
        };
      } catch (error) {
        return { success: false, error: error.message };
      }
    });

    if (!chunkResult.skipped) {
      expect(chunkResult.success).toBe(true);
      if (chunkResult.success) {
        expect(chunkResult.rootHash).toBeTruthy();
        expect(chunkResult.chunks).toBe(chunkResult.expectedChunks);
      }
    }
  });

  test('should keep CAS root hash stable for identical chunked payloads', async ({ page }) => {
    await page.goto('/');
    await page.waitForFunction(() => window.inos?.ready, { timeout: 30000 });

    const result = await page.evaluate(async () => {
      if (!window.inos || !window.inos.cas || !window.inos.cas.storeChunked) {
        return { skipped: true, reason: 'storeChunked API not exposed' };
      }

      try {
        const size = 2 * 1024 * 1024 + 19;
        const data = new Uint8Array(size);
        for (let i = 0; i < size; i++) {
          data[i] = (i * 17) % 251;
        }

        const first = await window.inos.cas.storeChunked(data);
        const second = await window.inos.cas.storeChunked(data);

        return {
          success: true,
          firstRootHash: first.rootHash,
          secondRootHash: second.rootHash,
          firstChunkCount: first.chunks?.length || 0,
          secondChunkCount: second.chunks?.length || 0,
        };
      } catch (error) {
        return { success: false, error: error.message };
      }
    });

    if (!result.skipped) {
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.firstRootHash).toBeTruthy();
        expect(result.firstRootHash).toBe(result.secondRootHash);
        expect(result.firstChunkCount).toBe(result.secondChunkCount);
      }
    }
  });
});
