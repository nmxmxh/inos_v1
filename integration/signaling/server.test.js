const test = require('node:test');
const assert = require('node:assert/strict');
const { spawn } = require('child_process');
const net = require('net');
const path = require('path');
const WebSocket = require('ws');

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
      if (text.includes('Listening on')) {
        if (!settled) {
          settled = true;
          clearTimeout(timeout);
          resolve();
        }
      }
    });

    proc.stderr.on('data', data => {
      // Keep stderr consumed to avoid blocking; surfaced only on failures.
      void data;
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

async function startServer() {
  const port = await getFreePort();
  const serverPath = path.resolve(__dirname, 'server.js');
  const proc = spawn('node', [serverPath], {
    env: {
      ...process.env,
      INOS_SIGNALING_PORT: String(port),
      INOS_SIGNALING_HOST: '127.0.0.1',
      INOS_SIGNALING_PATH: '/ws',
      INOS_SIGNALING_HEARTBEAT_MS: '5000',
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  await waitForReady(proc);

  return {
    url: `ws://127.0.0.1:${port}/ws`,
    stop: () =>
      new Promise(resolve => {
        if (proc.exitCode !== null) {
          resolve();
          return;
        }

        let settled = false;
        const done = () => {
          if (!settled) {
            settled = true;
            resolve();
          }
        };

        const killTimer = setTimeout(() => {
          if (proc.exitCode === null) {
            proc.kill('SIGKILL');
          }
          done();
        }, 3000);

        proc.once('exit', () => {
          clearTimeout(killTimer);
          done();
        });

        proc.kill('SIGINT');
      }),
  };
}

function connectWS(url) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(url);
    ws.once('open', () => resolve(ws));
    ws.once('error', reject);
  });
}

function sendJSON(ws, payload) {
  ws.send(JSON.stringify(payload));
}

function waitForMessage(ws, predicate, timeoutMs = 3000) {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      cleanup();
      reject(new Error('message timeout'));
    }, timeoutMs);

    const onMessage = data => {
      let parsed;
      try {
        parsed = JSON.parse(data.toString());
      } catch {
        return;
      }
      if (!predicate || predicate(parsed)) {
        cleanup();
        resolve(parsed);
      }
    };

    const onClose = () => {
      cleanup();
      reject(new Error('socket closed before expected message'));
    };

    function cleanup() {
      clearTimeout(timeout);
      ws.off('message', onMessage);
      ws.off('close', onClose);
    }

    ws.on('message', onMessage);
    ws.on('close', onClose);
  });
}

async function expectNoMatchingMessage(ws, predicate, timeoutMs = 500) {
  try {
    await waitForMessage(ws, predicate, timeoutMs);
    return false;
  } catch {
    return true;
  }
}

function drainMatchingMessages(ws, predicate, durationMs = 1200) {
  return new Promise(resolve => {
    const messages = [];
    const onMessage = data => {
      let parsed;
      try {
        parsed = JSON.parse(data.toString());
      } catch {
        return;
      }
      if (!predicate || predicate(parsed)) {
        messages.push(parsed);
      }
    };

    ws.on('message', onMessage);
    setTimeout(() => {
      ws.off('message', onMessage);
      resolve(messages);
    }, durationMs);
  });
}

test('queues and replays targeted signaling messages for offline peers', async t => {
  const server = await startServer();
  t.after(async () => {
    await server.stop();
  });

  const sender = await connectWS(server.url);
  t.after(() => sender.terminate());

  sendJSON(sender, { type: 'peer_discovery', peer_id: 'peer-a' });
  sendJSON(sender, {
    type: 'webrtc_offer',
    peer_id: 'peer-a',
    target_id: 'peer-b',
    offer: { type: 'offer', sdp: 'v=0\r\n' },
  });

  const receiver = await connectWS(server.url);
  t.after(() => receiver.terminate());

  sendJSON(receiver, { type: 'peer_discovery', peer_id: 'peer-b' });

  const replayed = await waitForMessage(
    receiver,
    msg => msg.type === 'webrtc_offer' && msg.peer_id === 'peer-a' && msg.target_id === 'peer-b',
    3000
  );

  assert.equal(replayed.type, 'webrtc_offer');
  assert.equal(replayed.target_id, 'peer-b');
});

test('routes targeted payloads only to target peer and supports ping/pong', async t => {
  const server = await startServer();
  t.after(async () => {
    await server.stop();
  });

  const peerA = await connectWS(server.url);
  const peerB = await connectWS(server.url);
  const peerC = await connectWS(server.url);
  t.after(() => peerA.terminate());
  t.after(() => peerB.terminate());
  t.after(() => peerC.terminate());

  sendJSON(peerA, { type: 'peer_discovery', peer_id: 'peer-a' });
  sendJSON(peerB, { type: 'peer_discovery', peer_id: 'peer-b' });
  sendJSON(peerC, { type: 'peer_discovery', peer_id: 'peer-c' });

  sendJSON(peerA, {
    type: 'webrtc_answer',
    peer_id: 'peer-a',
    target_id: 'peer-b',
    answer: { type: 'answer', sdp: 'v=0\r\n' },
  });

  const targeted = await waitForMessage(
    peerB,
    msg => msg.type === 'webrtc_answer' && msg.peer_id === 'peer-a' && msg.target_id === 'peer-b',
    3000
  );
  assert.equal(targeted.target_id, 'peer-b');

  const cDidNotReceive = await expectNoMatchingMessage(
    peerC,
    msg => msg.type === 'webrtc_answer' && msg.peer_id === 'peer-a',
    600
  );
  assert.equal(cDidNotReceive, true);

  sendJSON(peerA, { type: 'ping', peer_id: 'peer-a' });
  const pong = await waitForMessage(peerA, msg => msg.type === 'pong', 2000);
  assert.equal(pong.type, 'pong');
});

test('keeps only latest targeted messages in offline queue', async t => {
  const server = await startServer();
  t.after(async () => {
    await server.stop();
  });

  const sender = await connectWS(server.url);
  t.after(() => sender.terminate());
  sendJSON(sender, { type: 'peer_discovery', peer_id: 'peer-a' });

  const total = 70;
  for (let i = 0; i < total; i += 1) {
    sendJSON(sender, {
      type: 'webrtc_offer',
      peer_id: 'peer-a',
      target_id: 'peer-b',
      sequence: i,
      offer: { type: 'offer', sdp: `v=0\r\ns=${i}\r\n` },
    });
  }

  const receiver = await connectWS(server.url);
  t.after(() => receiver.terminate());
  sendJSON(receiver, { type: 'peer_discovery', peer_id: 'peer-b' });

  const replayed = await drainMatchingMessages(
    receiver,
    msg => msg.type === 'webrtc_offer' && msg.target_id === 'peer-b',
    1500
  );

  assert.equal(replayed.length, 64);
  assert.equal(replayed[0].sequence, 6);
  assert.equal(replayed[replayed.length - 1].sequence, 69);
});

test('routes to the latest socket when a peer reconnects with same peer_id', async t => {
  const server = await startServer();
  t.after(async () => {
    await server.stop();
  });

  const peerA = await connectWS(server.url);
  const peerBOld = await connectWS(server.url);
  const peerBNew = await connectWS(server.url);
  t.after(() => peerA.terminate());
  t.after(() => peerBOld.terminate());
  t.after(() => peerBNew.terminate());

  sendJSON(peerA, { type: 'peer_discovery', peer_id: 'peer-a' });
  sendJSON(peerA, { type: 'ping', peer_id: 'peer-a' });
  await waitForMessage(peerA, msg => msg.type === 'pong', 2000);

  sendJSON(peerBOld, { type: 'peer_discovery', peer_id: 'peer-b' });
  sendJSON(peerBOld, { type: 'ping', peer_id: 'peer-b' });
  await waitForMessage(peerBOld, msg => msg.type === 'pong', 2000);

  // Re-register peer-b on a newer socket; this one must become authoritative.
  sendJSON(peerBNew, { type: 'peer_discovery', peer_id: 'peer-b' });
  sendJSON(peerBNew, { type: 'ping', peer_id: 'peer-b' });
  await waitForMessage(peerBNew, msg => msg.type === 'pong', 2000);

  sendJSON(peerA, {
    type: 'webrtc_answer',
    peer_id: 'peer-a',
    target_id: 'peer-b',
    answer: { type: 'answer', sdp: 'v=0\r\n' },
  });

  const routed = await waitForMessage(
    peerBNew,
    msg => msg.type === 'webrtc_answer' && msg.target_id === 'peer-b',
    3000
  );
  assert.equal(routed.target_id, 'peer-b');

  const oldSocketSilent = await expectNoMatchingMessage(
    peerBOld,
    msg => msg.type === 'webrtc_answer' && msg.target_id === 'peer-b',
    700
  );
  assert.equal(oldSocketSilent, true);
});
