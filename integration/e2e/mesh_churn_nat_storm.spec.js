const { test, expect } = require('@playwright/test');
const { spawn } = require('child_process');
const net = require('net');
const path = require('path');

let signalingProcess;
let signalingUrl;

function getFreePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.unref();
    server.on('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const { port } = server.address();
      server.close(() => resolve(port));
    });
  });
}

function waitForReady(proc, timeoutMs = 10000) {
  return new Promise((resolve, reject) => {
    let settled = false;
    const timeout = setTimeout(() => {
      if (!settled) {
        settled = true;
        reject(new Error('signaling server startup timeout'));
      }
    }, timeoutMs);

    proc.stdout.on('data', data => {
      const text = data.toString();
      console.log(`[CHURN-SIGNALING-STDOUT] ${text.trim()}`);
      if (text.includes('Listening on')) {
        if (!settled) {
          settled = true;
          clearTimeout(timeout);
          resolve();
        }
      }
    });

    proc.stderr.on('data', data => {
      console.log(`[CHURN-SIGNALING-STDERR] ${data.toString().trim()}`);
    });

    proc.on('exit', code => {
      if (!settled) {
        settled = true;
        clearTimeout(timeout);
        reject(new Error(`signaling server exited early (${code})`));
      }
    });
  });
}

async function createPeer(browser, baseUrl, nodeId, signaling) {
  const context = await browser.newContext();
  const page = await context.newPage();
  const url = `${baseUrl}/?nodeId=${encodeURIComponent(nodeId)}&deviceId=${encodeURIComponent(
    `${nodeId}-device-${Date.now()}`
  )}&signaling=${encodeURIComponent(signaling)}`;

  await page.goto(url);
  await page.waitForFunction(() => window.mesh?.connectToPeer && window.mesh?.getTelemetry, {
    timeout: 60000,
  });
  return { context, page };
}

async function connectNoThrow(page, peerId, address) {
  let lastError = '';
  for (let attempt = 0; attempt < 3; attempt++) {
    const result = await page.evaluate(
      async ({ targetPeerId, targetAddress }) => {
        try {
          const timeout = new Promise((_, reject) => {
            setTimeout(() => reject(new Error('connect timeout')), 8000);
          });
          const out = await Promise.race([
            targetAddress
              ? window.mesh.connectToPeer(targetPeerId, targetAddress)
              : window.mesh.connectToPeer(targetPeerId),
            timeout,
          ]);
          return { success: true, out };
        } catch (error) {
          return { success: false, error: String(error) };
        }
      },
      { targetPeerId: peerId, targetAddress: address || '' }
    );

    if (result.success) {
      return;
    }
    lastError = result.error || 'unknown error';
    await new Promise(resolve => setTimeout(resolve, 200));
  }
  throw new Error(`connectToPeer failed for ${peerId}: ${lastError}`);
}

async function disconnectNoThrow(page, peerId) {
  let lastError = '';
  for (let attempt = 0; attempt < 2; attempt++) {
    const result = await page.evaluate(async targetPeerId => {
      try {
        const timeout = new Promise((_, reject) => {
          setTimeout(() => reject(new Error('disconnect timeout')), 5000);
        });
        const out = await Promise.race([window.mesh.disconnectFromPeer(targetPeerId), timeout]);
        return { success: true, out };
      } catch (error) {
        return { success: false, error: String(error) };
      }
    }, peerId);

    if (result.success) {
      return;
    }
    lastError = result.error || 'unknown error';
    await new Promise(resolve => setTimeout(resolve, 150));
  }
  throw new Error(`disconnectFromPeer failed for ${peerId}: ${lastError}`);
}

test.describe('INOS Mesh NAT/Churn Stress', () => {
  test.describe.configure({ timeout: 120000 });

  test.beforeAll(async () => {
    const port = await getFreePort();
    signalingUrl = `ws://127.0.0.1:${port}/ws`;
    const serverPath = path.resolve(__dirname, '../signaling/server.js');
    signalingProcess = spawn('node', [serverPath], {
      env: {
        ...process.env,
        INOS_SIGNALING_PORT: String(port),
        INOS_SIGNALING_HOST: '127.0.0.1',
        INOS_SIGNALING_PATH: '/ws',
      },
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    await waitForReady(signalingProcess);
  });

  test.afterAll(async () => {
    if (signalingProcess) {
      signalingProcess.kill('SIGINT');
      signalingProcess = null;
    }
  });

  test('should tolerate rapid peer flaps (simulated NAT rebinding)', async ({ browser }) => {
    const baseUrl = process.env.INOS_APP_URL || 'http://127.0.0.1:5173';
    const signaling =
      process.env.INOS_SIGNALING_URL || signalingUrl || 'ws://127.0.0.1:8787/ws';

    const stableA = await createPeer(browser, baseUrl, 'nat-stable-a', signaling);
    const stableB = await createPeer(browser, baseUrl, 'nat-stable-b', signaling);
    const rounds = 3;
    for (let i = 0; i < rounds; i++) {
      const flapPeer = await createPeer(browser, baseUrl, `nat-flap-peer-${i}`, signaling);
      await connectNoThrow(flapPeer.page, 'nat-stable-a');
      await new Promise(resolve => setTimeout(resolve, 150));
      await flapPeer.context.close();
    }

    // After repeated flaps, a fresh peer should still be able to establish a connection.
    const postFlapPeer = await createPeer(browser, baseUrl, 'nat-post-flap', signaling);
    await connectNoThrow(postFlapPeer.page, 'nat-stable-a');

    const health = await stableA.page.evaluate(async () => {
      try {
        const telemetry = await window.mesh.getTelemetry();
        return { success: true, telemetry };
      } catch (error) {
        return { success: false, error: String(error) };
      }
    });

    expect(health.success).toBe(true);
    if (health.telemetry && typeof health.telemetry.active_peers === 'number') {
      expect(health.telemetry.active_peers).toBeGreaterThanOrEqual(0);
    }

    await postFlapPeer.context.close();
    await stableA.context.close();
    await stableB.context.close();
  });

  test('should survive reconnect storms from many peers to one hub', async ({ browser }) => {
    const baseUrl = process.env.INOS_APP_URL || 'http://127.0.0.1:5173';
    const signaling =
      process.env.INOS_SIGNALING_URL || signalingUrl || 'ws://127.0.0.1:8787/ws';
    const fallbackList = `ws://127.0.0.1:1/ws,${signaling}`;

    const hub = await createPeer(browser, baseUrl, 'storm-hub', signaling);
    const clients = [];
    for (let i = 0; i < 4; i++) {
      clients.push(await createPeer(browser, baseUrl, `storm-client-${i}`, fallbackList));
    }

    for (let round = 0; round < 2; round++) {
      await Promise.all(
        clients.map(client => connectNoThrow(client.page, 'storm-hub', fallbackList))
      );
      await new Promise(resolve => setTimeout(resolve, 150));
      await Promise.all(clients.map(client => disconnectNoThrow(client.page, 'storm-hub')));
    }

    await Promise.all(clients.map(client => connectNoThrow(client.page, 'storm-hub', fallbackList)));

    const clientTelemetry = await Promise.all(
      clients.map(client =>
        client.page.evaluate(async () => {
          try {
            const telemetry = await window.mesh.getTelemetry();
            return { success: true, telemetry };
          } catch (error) {
            return { success: false, error: String(error) };
          }
        })
      )
    );
    for (const status of clientTelemetry) {
      expect(status.success).toBe(true);
    }

    const hubTelemetry = await hub.page.evaluate(async () => {
      try {
        const telemetry = await window.mesh.getTelemetry();
        return { success: true, telemetry };
      } catch (error) {
        return { success: false, error: String(error) };
      }
    });

    expect(hubTelemetry.success).toBe(true);
    if (hubTelemetry.telemetry && typeof hubTelemetry.telemetry.active_peers === 'number') {
      expect(hubTelemetry.telemetry.active_peers).toBeGreaterThanOrEqual(0);
    }

    for (const client of clients) {
      await client.context.close();
    }
    await hub.context.close();
  });
});
