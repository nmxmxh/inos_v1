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
    server.listen(0, () => {
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
      console.log(`[SIGNALING-STDOUT] ${text.trim()}`);
      if (text.includes('Listening on')) {
        if (!settled) {
          settled = true;
          clearTimeout(timeout);
          resolve();
        }
      }
    });

    proc.stderr.on('data', data => {
      console.log(`[SIGNALING-STDERR] ${data.toString().trim()}`);
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

test.describe('INOS Mesh Local Signaling', () => {
  test.beforeAll(async () => {
    const port = await getFreePort();
    signalingUrl = `ws://localhost:${port}/ws`;
    const serverPath = path.resolve(__dirname, '../signaling/server.js');
    signalingProcess = spawn('node', [serverPath], {
      env: {
        ...process.env,
        INOS_SIGNALING_PORT: String(port),
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

  test('should connect peers via local signaling and emit mesh events', async ({ browser }) => {
    const baseUrl = process.env.INOS_APP_URL || 'http://localhost:5173';
    const signaling =
      process.env.INOS_SIGNALING_URL || signalingUrl || 'ws://localhost:8787/ws';

    const contextA = await browser.newContext();
    const contextB = await browser.newContext();

    const pageA = await contextA.newPage();
    const pageB = await contextB.newPage();

    // Capture console logs
    pageA.on('console', msg => console.log(`[PAGE-A] ${msg.text()}`));
    pageB.on('console', msg => console.log(`[PAGE-B] ${msg.text()}`));

    pageA.on('worker', worker => {
      worker.on('console', msg => console.log(`[WORKER-A] ${msg.text()}`));
    });
    pageB.on('worker', worker => {
      worker.on('console', msg => console.log(`[WORKER-B] ${msg.text()}`));
    });

    const peerA = `${baseUrl}/?nodeId=node-a&deviceId=device-a&signaling=${encodeURIComponent(
      signaling
    )}`;
    const peerB = `${baseUrl}/?nodeId=node-b&deviceId=device-b&signaling=${encodeURIComponent(
      signaling
    )}`;

    await pageA.goto(peerA);
    await pageB.goto(peerB);

    await pageA.waitForFunction(() => (window).mesh?.connectToPeer, { timeout: 60000 });
    await pageB.waitForFunction(() => (window).mesh?.subscribeToEvents, { timeout: 60000 });

    await pageB.evaluate(() => {
      window.__mesh_events__ = [];
      return window.mesh.subscribeToEvents(['mesh.*'], event => {
        window.__mesh_events__.push({ type: event.type, payloadType: event.payloadType });
      });
    });

    await pageA.evaluate(() => window.mesh.connectToPeer('node-b'));

    await pageB.waitForFunction(
      () => (window.__mesh_events__ || []).some(evt => evt.type === 'mesh.peer_update'),
      { timeout: 20000 }
    );

    const eventSummary = await pageB.evaluate(() => window.__mesh_events__?.slice(-3) || []);
    expect(eventSummary.length).toBeGreaterThan(0);

    await contextA.close();
    await contextB.close();
  });
});
