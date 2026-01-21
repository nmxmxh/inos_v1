//go:build !js || !wasm

package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func (t *WebRTCTransport) isWebRTCSupported() bool {
	return true // Pion supports WebRTC on native
}

// connectViaWebSocket establishes a WebSocket connection as fallback
func (t *WebRTCTransport) connectViaWebSocket(ctx context.Context, peerID string) error {
	// Check if we have a direct WebSocket URL for the peer
	wsURL, err := t.getPeerWebSocketURL(peerID)
	if err != nil {
		return fmt.Errorf("failed to get peer WebSocket URL: %w", err)
	}

	// Connect to peer's WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: t.config.ConnectionTimeout,
		ReadBufferSize:   t.config.MaxMessageSize,
		WriteBufferSize:  t.config.MaxMessageSize,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to dial WebSocket: %w", err)
	}

	// Create WebSocket connection wrapper
	wsConn := &WebSocketConnection{
		peerID:   peerID,
		conn:     conn,
		stats:    ConnectionStats{OpenedAt: time.Now()},
		shutdown: make(chan struct{}),
	}

	// Store connection
	t.connMu.Lock()
	t.connections[peerID] = &PeerConnection{
		PeerID:      peerID,
		Connection:  wsConn,
		Connected:   true,
		LastContact: time.Now(),
	}
	t.connMu.Unlock()
	t.notifyPeerEvent(peerID, true)

	// Start receiving messages
	go wsConn.receiveLoop(t.handleIncomingMessage)

	return nil
}

// WebSocketConnection implements Connection for WebSocket
type WebSocketConnection struct {
	peerID   string
	conn     *websocket.Conn
	stats    ConnectionStats
	shutdown chan struct{}
	mu       sync.RWMutex
}

func (c *WebSocketConnection) Send(ctx context.Context, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return errors.New("connection not open")
	}

	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.stats.LastError = err.Error()
		return err
	}

	c.stats.BytesSent += uint64(len(data))
	c.stats.MessagesSent++
	return nil
}

func (c *WebSocketConnection) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.shutdown:
		return nil, errors.New("connection closed")
	default:
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return nil, err
		}

		c.stats.BytesReceived += uint64(len(message))
		c.stats.MessagesRecv++
		return message, nil
	}
}

func (c *WebSocketConnection) Close() error {
	defer func() {
		recover() // ignore close of closed channel
	}()
	close(c.shutdown)

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *WebSocketConnection) IsOpen() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.conn != nil
}

func (c *WebSocketConnection) GetStats() ConnectionStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

func (c *WebSocketConnection) receiveLoop(handler func(string, []byte)) {
	defer c.Close()

	for {
		select {
		case <-c.shutdown:
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				return
			}

			c.stats.BytesReceived += uint64(len(message))
			c.stats.MessagesRecv++

			// Handle message
			handler(c.peerID, message)
		}
	}
}
