const http = require('http');
const WebSocket = require('ws');

const port = Number(process.env.INOS_SIGNALING_PORT || process.env.SIGNALING_PORT || 8787);
const host = process.env.INOS_SIGNALING_HOST || '127.0.0.1';
const path = process.env.INOS_SIGNALING_PATH || '/ws';
const heartbeatIntervalMs = Number(process.env.INOS_SIGNALING_HEARTBEAT_MS || 25000);

const peers = new Map();
const pending = new Map();

function send(ws, payload) {
  if (ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(payload));
  }
}

function broadcast(payload, exclude) {
  for (const ws of peers.values()) {
    if (ws !== exclude) {
      send(ws, payload);
    }
  }
}

function registerPeer(peerId, ws) {
  if (!peerId) return;
  if (ws.peerId && ws.peerId !== peerId) {
    peers.delete(ws.peerId);
  }
  ws.peerId = peerId;
  peers.set(peerId, ws);

  const queued = pending.get(peerId);
  if (queued) {
    queued.forEach(message => send(ws, message));
    pending.delete(peerId);
  }
}

function queueMessage(peerId, message) {
  if (!peerId) return;
  const queue = pending.get(peerId) || [];
  queue.push(message);
  pending.set(peerId, queue.slice(-64));
}

const server = http.createServer((req, res) => {
  res.writeHead(200, { 'Content-Type': 'text/plain' });
  res.end('INOS local signaling server\n');
});

const wss = new WebSocket.Server({ noServer: true });

wss.on('connection', (ws, req) => {
  console.log(`[Signaling] New connection from ${req.url}`);
  ws.isAlive = true;

  ws.on('pong', () => {
    ws.isAlive = true;
  });

  ws.on('message', data => {
    let message;
    try {
      message = JSON.parse(data.toString());
      console.log(`[Signaling] Message from ${ws.peerId || 'unknown'}: ${message.type} to ${message.target_id || message.targetId || 'broadcast'}`);
    } catch {
      console.log(`[Signaling] Invalid message from ${ws.peerId || 'unknown'}: ${data.toString().substring(0, 50)}`);
      return;
    }

    const peerId = message.peer_id || message.peerId;
    const targetId = message.target_id || message.targetId;
    const msgType = message.type;

    if (peerId) {
      registerPeer(peerId, ws);
    }

    if (msgType === 'ping') {
      send(ws, { type: 'pong', peer_id: peerId || '' });
      return;
    }

    if (targetId) {
      const target = peers.get(targetId);
      if (target) {
        send(target, message);
      } else {
        queueMessage(targetId, message);
      }
      return;
    }

    if (msgType === 'peer_discovery') {
      broadcast(message, ws);
      return;
    }
  });

  ws.on('close', () => {
    console.log(`[Signaling] Client disconnected: ${ws.peerId || 'unknown'}`);
    if (ws.peerId) {
      peers.delete(ws.peerId);
    }
  });
});

server.on('upgrade', (req, socket, head) => {
  const urlPath = req.url.split('?')[0];
  if (urlPath !== path) {
    console.log(`[Signaling] Rejecting upgrade: ${req.url} (expected path: ${path})`);
    socket.write('HTTP/1.1 400 Bad Request\r\n\r\n');
    socket.destroy();
    return;
  }
  console.log(`[Signaling] Accepting upgrade: ${req.url}`);

  wss.handleUpgrade(req, socket, head, ws => {
    wss.emit('connection', ws, req);
  });
});

const heartbeat = setInterval(() => {
  wss.clients.forEach(ws => {
    if (!ws.isAlive) {
      ws.terminate();
      return;
    }
    ws.isAlive = false;
    ws.ping();
  });
}, heartbeatIntervalMs);

server.listen(port, host, () => {
  console.log(`[Signaling] Listening on ws://${host}:${port}${path}`);
});

process.on('SIGINT', () => {
  clearInterval(heartbeat);
  wss.close();
  server.close(() => process.exit(0));
});
