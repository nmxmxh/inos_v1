//go:build js && wasm

package transport

import (
	"context"
	"errors"
	"sync"
	"syscall/js"
	"time"
)

// connectViaWebSocket establishes a WebSocket connection as fallback
func (t *WebRTCTransport) isWebRTCSupported() bool {
	pc := js.Global().Get("RTCPeerConnection")
	return !pc.IsUndefined()
}

func (t *WebRTCTransport) connectViaWebSocket(ctx context.Context, peerID string) error {
	wsURL, err := t.getPeerWebSocketURL(peerID)
	if err != nil {
		return err
	}

	ws := js.Global().Get("WebSocket").New(wsURL)
	conn := &WebSocketConnection{
		peerID:   peerID,
		ws:       ws,
		messages: make(chan []byte, 100),
		shutdown: make(chan struct{}),
		stats:    ConnectionStats{OpenedAt: time.Now()},
	}

	opened := make(chan bool)
	ws.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		opened <- true
		return nil
	}))
	ws.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		data := args[0].Get("data").String()
		conn.messages <- []byte(data)
		return nil
	}))
	ws.Set("onclose", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		close(conn.shutdown)
		return nil
	}))

	select {
	case <-opened:
		// success
	case <-ctx.Done():
		ws.Call("close")
		return ctx.Err()
	case <-time.After(t.config.ConnectionTimeout):
		ws.Call("close")
		return errors.New("timeout connecting to websocket")
	}

	// Store connection
	t.connMu.Lock()
	t.connections[peerID] = &PeerConnection{
		PeerID:      peerID,
		Connection:  conn,
		Connected:   true,
		LastContact: time.Now(),
	}
	t.connMu.Unlock()
	t.notifyPeerEvent(peerID, true)

	// Start receiving messages
	go conn.receiveLoop(t.handleIncomingMessage)

	return nil
}

type WebSocketConnection struct {
	peerID   string
	ws       js.Value
	messages chan []byte
	shutdown chan struct{}
	stats    ConnectionStats
	mu       sync.RWMutex
}

func (c *WebSocketConnection) Send(ctx context.Context, data []byte) error {
	c.ws.Call("send", string(data))
	c.mu.Lock()
	c.stats.BytesSent += uint64(len(data))
	c.stats.MessagesSent++
	c.mu.Unlock()
	return nil
}

func (c *WebSocketConnection) Receive(ctx context.Context) ([]byte, error) {
	select {
	case msg := <-c.messages:
		c.mu.Lock()
		c.stats.BytesReceived += uint64(len(msg))
		c.stats.MessagesRecv++
		c.mu.Unlock()
		return msg, nil
	case <-c.shutdown:
		return nil, errors.New("websocket closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *WebSocketConnection) Close() error {
	c.ws.Call("close")
	return nil
}

func (c *WebSocketConnection) IsOpen() bool {
	return c.ws.Get("readyState").Int() == 1
}

func (c *WebSocketConnection) GetStats() ConnectionStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

func (c *WebSocketConnection) receiveLoop(handler func(string, []byte)) {
	for {
		select {
		case msg := <-c.messages:
			handler(c.peerID, msg)
		case <-c.shutdown:
			return
		}
	}
}
