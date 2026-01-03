package transport

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

// WebRTCTransport implements Transport using WebRTC with WebSocket fallback
type WebRTCTransport struct {
	nodeID          string
	localCapability *common.PeerCapability

	// Connections
	connections    map[string]*PeerConnection
	connMu         sync.RWMutex
	connectionPool *ConnectionPool

	// WebRTC configuration
	webrtcConfig    webrtc.Configuration
	peerConnections map[string]*webrtc.PeerConnection
	pcMu            sync.RWMutex

	// WebSocket signaling
	signalingURL     string
	signalingConn    *websocket.Conn
	signalingMu      sync.RWMutex
	signalingWriteMu sync.Mutex
	signalingStatus  atomic.Value // "connected", "connecting", "disconnected"

	// STUN/TURN servers
	// iceServers []string // Removed per lint warning (unused, present in config)

	// Channels
	rpcResponses map[string]chan RPCResponse
	rpcMu        sync.RWMutex
	rpcHandlers  map[string]func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error)
	handlerMu    sync.RWMutex

	messageQueue chan QueuedMessage
	shutdown     chan struct{}

	// Metrics
	metrics   common.ConnectionMetrics
	metricsMu sync.RWMutex

	// Health monitoring
	health       common.TransportHealth
	healthTicker *time.Ticker

	// Configuration
	config TransportConfig

	// Logger
	logger *slog.Logger

	// State
	startTime time.Time
	started   atomic.Bool
}

// RPCRequest represents a remote procedure call
type RPCRequest struct {
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	Timeout int64       `json:"timeout"` // Milliseconds
}

// RPCResponse represents an RPC response
type RPCResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *RPCError   `json:"error,omitempty"`
}

// RPCError represents an RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// PeerConnection represents a connection to a specific peer
type PeerConnection struct {
	PeerID      string
	Connection  Connection
	Capability  *common.PeerCapability
	LastContact time.Time
	Latency     time.Duration
	Connected   bool
	mu          sync.RWMutex
}

// Connection is an interface for a specific transport connection
type Connection interface {
	Send(ctx context.Context, data []byte) error
	Receive(ctx context.Context) ([]byte, error)
	Close() error
	IsOpen() bool
	GetStats() ConnectionStats
}

// ConnectionStats holds connection statistics
type ConnectionStats struct {
	BytesSent     uint64        `json:"bytes_sent"`
	BytesReceived uint64        `json:"bytes_received"`
	MessagesSent  uint64        `json:"messages_sent"`
	MessagesRecv  uint64        `json:"messages_received"`
	Latency       time.Duration `json:"latency"`
	LastError     string        `json:"last_error,omitempty"`
	OpenedAt      time.Time     `json:"opened_at"`
}

// TransportConfig holds transport configuration
type TransportConfig struct {
	// WebRTC settings
	WebRTCEnabled bool     `json:"webrtc_enabled"`
	ICEServers    []string `json:"ice_servers"`
	STUNServers   []string `json:"stun_servers"`
	TURNServers   []string `json:"turn_servers"`

	// WebSocket settings
	WebSocketURL     string   `json:"websocket_url"`
	SignalingServers []string `json:"signaling_servers"`

	// Connection settings
	MaxConnections    int           `json:"max_connections"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
	ReconnectDelay    time.Duration `json:"reconnect_delay"`
	KeepAliveInterval time.Duration `json:"keepalive_interval"`
	MaxMessageSize    int           `json:"max_message_size"`

	// RPC settings
	RPCTimeout time.Duration `json:"rpc_timeout"`
	MaxRetries int           `json:"max_retries"`

	// Pool settings
	PoolSize    int           `json:"pool_size"`
	PoolMaxIdle time.Duration `json:"pool_max_idle"`

	// Metrics settings
	MetricsInterval time.Duration `json:"metrics_interval"`
}

// QueuedMessage represents a message in the send queue
type QueuedMessage struct {
	PeerID  string
	Message interface{}
	Retries int
	Context context.Context
	Result  chan error
}

// ConnectionPool manages a pool of connections
type ConnectionPool struct {
	connections map[string]*Connection
	mu          sync.RWMutex
	maxSize     int
	idleTimeout time.Duration
}

// DefaultTransportConfig returns sensible production defaults
func DefaultTransportConfig() TransportConfig {
	return TransportConfig{
		WebRTCEnabled: true,
		ICEServers: []string{
			"stun:stun.l.google.com:19302",
			"stun:global.stun.twilio.com:3478",
		},
		STUNServers: []string{
			"stun:stun.l.google.com:19302",
			"stun:stun1.l.google.com:19302",
			"stun:stun2.l.google.com:19302",
			"stun:stun3.l.google.com:19302",
			"stun:stun4.l.google.com:19302",
		},
		TURNServers: []string{},

		WebSocketURL: "wss://signaling.inos.ai/ws",
		SignalingServers: []string{
			"wss://signaling1.inos.ai/ws",
			"wss://signaling2.inos.ai/ws",
			"wss://signaling3.inos.ai/ws",
		},

		MaxConnections:    100,
		ConnectionTimeout: 10 * time.Second,
		ReconnectDelay:    5 * time.Second,
		KeepAliveInterval: 30 * time.Second,
		MaxMessageSize:    1024 * 1024 * 10, // 10MB

		RPCTimeout: 30 * time.Second,
		MaxRetries: 3,

		PoolSize:    50,
		PoolMaxIdle: 5 * time.Minute,

		MetricsInterval: 10 * time.Second,
	}
}

// NewWebRTCTransport creates a new WebRTC transport with WebSocket fallback
func NewWebRTCTransport(nodeID string, config TransportConfig, logger *slog.Logger) (*WebRTCTransport, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Generate default ICE servers if none provided
	if len(config.ICEServers) == 0 {
		config.ICEServers = DefaultTransportConfig().ICEServers
	}

	transport := &WebRTCTransport{
		nodeID:          nodeID,
		connections:     make(map[string]*PeerConnection),
		peerConnections: make(map[string]*webrtc.PeerConnection),
		rpcResponses:    make(map[string]chan RPCResponse),
		rpcHandlers:     make(map[string]func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error)),
		messageQueue:    make(chan QueuedMessage, 1000),
		shutdown:        make(chan struct{}),
		config:          config,
		logger:          logger.With("component", "transport", "node_id", getShortID(nodeID)),
		startTime:       time.Now(),
	}

	// Initialize WebRTC configuration
	transport.webrtcConfig = transport.createWebRTCConfig()

	// Initialize connection pool
	transport.connectionPool = &ConnectionPool{
		connections: make(map[string]*Connection),
		maxSize:     config.PoolSize,
		idleTimeout: config.PoolMaxIdle,
	}

	// Initialize health
	transport.health = common.TransportHealth{
		Status:          "initializing",
		Score:           0.0,
		WebRTCSupported: config.WebRTCEnabled,
		IceServers:      len(config.ICEServers),
	}

	transport.signalingStatus.Store("disconnected")

	return transport, nil
}

// Start initializes and starts the transport
func (t *WebRTCTransport) Start(ctx context.Context) error {
	if t.started.Load() {
		return errors.New("transport already started")
	}

	t.logger.Info("starting transport")

	// Connect to signaling server
	if err := t.connectSignaling(); err != nil {
		t.logger.Warn("failed to connect to signaling server, will retry", "error", err)
		go t.reconnectSignaling()
	}

	// Start background workers
	go t.messageProcessor()
	go t.connectionManager()
	go t.healthMonitor()
	go t.metricsCollector()

	t.started.Store(true)
	t.health.Status = "running"
	t.logger.Info("transport started successfully")

	return nil
}

// Stop gracefully shuts down the transport
func (t *WebRTCTransport) Stop() error {
	if !t.started.Load() {
		return nil
	}

	t.logger.Info("stopping transport")
	close(t.shutdown)

	// Close signaling connection
	t.signalingMu.Lock()
	if t.signalingConn != nil {
		t.signalingConn.Close()
		t.signalingConn = nil
	}
	t.signalingMu.Unlock()

	// Close all peer connections
	t.connMu.Lock()
	for peerID, conn := range t.connections {
		if conn.Connection != nil {
			conn.Connection.Close()
		}
		delete(t.connections, peerID)
	}
	t.connMu.Unlock()

	// Close WebRTC connections
	t.pcMu.Lock()
	for peerID, pc := range t.peerConnections {
		pc.Close()
		delete(t.peerConnections, peerID)
	}
	t.pcMu.Unlock()

	t.started.Store(false)
	t.health.Status = "stopped"
	t.logger.Info("transport stopped")

	return nil
}

// Connect establishes a connection to a peer
func (t *WebRTCTransport) Connect(ctx context.Context, peerID string) error {
	if peerID == t.nodeID {
		return errors.New("cannot connect to self")
	}

	// Check if already connected
	if t.IsConnected(peerID) {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, t.config.ConnectionTimeout)
	defer cancel()

	t.logger.Debug("connecting to peer", "peer", getShortID(peerID))

	// Try WebRTC first if enabled
	if t.config.WebRTCEnabled {
		if err := t.connectViaWebRTC(ctx, peerID); err == nil {
			t.logger.Debug("connected via WebRTC", "peer", getShortID(peerID))
			t.metricsMu.Lock()
			t.metrics.WebRTCCandidates++
			t.metricsMu.Unlock()
			return nil
		} else {
			t.logger.Debug("WebRTC connection failed", "peer", getShortID(peerID), "error", err)
		}
	}

	// Fall back to WebSocket
	if err := t.connectViaWebSocket(ctx, peerID); err != nil {
		return fmt.Errorf("failed to connect via WebSocket: %w", err)
	}

	t.logger.Debug("connected via WebSocket fallback", "peer", getShortID(peerID))
	t.metricsMu.Lock()
	t.metrics.WebSocketFallbacks++
	t.metricsMu.Unlock()

	return nil
}

// connectViaWebRTC attempts to establish a WebRTC connection
func (t *WebRTCTransport) connectViaWebRTC(ctx context.Context, peerID string) error {
	// Create WebRTC peer connection
	config := t.webrtcConfig
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Store peer connection immediately so signaling can find it
	t.pcMu.Lock()
	t.peerConnections[peerID] = peerConnection
	t.pcMu.Unlock()

	// Create data channel
	dataChannel, err := peerConnection.CreateDataChannel("mesh", nil)
	if err != nil {
		peerConnection.Close()
		return fmt.Errorf("failed to create data channel: %w", err)
	}

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		// Send candidate to peer via signaling
		msg := map[string]interface{}{
			"type":      "ice_candidate",
			"peer_id":   t.nodeID,
			"target_id": peerID,
			"candidate": candidate.ToJSON(), // Send object directly
		}

		if err := t.sendSignalingMessage(msg); err != nil {
			t.logger.Error("failed to send ICE candidate", "error", err)
		}
	})

	// Wait for connection with timeout
	connected := make(chan struct{}, 1)

	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		t.logger.Debug("WebRTC connection state changed",
			"peer", getShortID(peerID),
			"state", state.String())

		switch state {
		case webrtc.PeerConnectionStateConnected:
			// Connection established
			conn := &WebRTCConnection{
				peerID:   peerID,
				dc:       dataChannel,
				pc:       peerConnection,
				stats:    ConnectionStats{OpenedAt: time.Now()},
				shutdown: make(chan struct{}),
			}

			// Store connection
			t.connMu.Lock()
			t.connections[peerID] = &PeerConnection{
				PeerID:     peerID,
				Connection: conn,
				Connected:  true,
			}
			t.connMu.Unlock()

			t.logger.Info("WebRTC connection established", "peer", getShortID(peerID))

			// Signal success if waiting
			select {
			case connected <- struct{}{}:
			default:
			}

			// Start receiving messages
			go conn.receiveLoop()

		case webrtc.PeerConnectionStateDisconnected,
			webrtc.PeerConnectionStateFailed,
			webrtc.PeerConnectionStateClosed:

			// Clean up
			t.connMu.Lock()
			delete(t.connections, peerID)
			t.connMu.Unlock()

			t.pcMu.Lock()
			delete(t.peerConnections, peerID)
			t.pcMu.Unlock()
		}
	})

	// Create offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		peerConnection.Close()
		return fmt.Errorf("failed to create offer: %w", err)
	}

	// Set local description
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		peerConnection.Close()
		return fmt.Errorf("failed to set local description: %w", err)
	}

	msg := map[string]interface{}{
		"type":      "webrtc_offer",
		"peer_id":   t.nodeID,
		"target_id": peerID,
		"offer":     offer, // Send object directly
	}

	if err := t.sendSignalingMessage(msg); err != nil {
		peerConnection.Close()
		return fmt.Errorf("failed to send offer: %w", err)
	}

	// Wait for answer/connection with timeout
	select {
	case <-ctx.Done():
		peerConnection.Close()
		return ctx.Err()
	case <-connected:
		return nil
	}
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

	// Start receiving messages
	go wsConn.receiveLoop(t.handleIncomingMessage)

	return nil
}

// Disconnect closes connection to a peer
func (t *WebRTCTransport) Disconnect(peerID string) error {
	t.connMu.Lock()
	conn, exists := t.connections[peerID]
	if exists {
		if conn.Connection != nil {
			conn.Connection.Close()
		}
		delete(t.connections, peerID)
	}
	t.connMu.Unlock()

	t.pcMu.Lock()
	if pc, exists := t.peerConnections[peerID]; exists {
		pc.Close()
		delete(t.peerConnections, peerID)
	}
	t.pcMu.Unlock()

	t.logger.Debug("disconnected from peer", "peer", getShortID(peerID))
	return nil
}

// IsConnected checks if connected to a peer
func (t *WebRTCTransport) IsConnected(peerID string) bool {
	t.connMu.RLock()
	defer t.connMu.RUnlock()

	if conn, exists := t.connections[peerID]; exists {
		return conn.Connected && conn.Connection != nil && conn.Connection.IsOpen()
	}
	return false
}

// GetConnectedPeers returns list of connected peer IDs
func (t *WebRTCTransport) GetConnectedPeers() []string {
	t.connMu.RLock()
	defer t.connMu.RUnlock()

	peers := make([]string, 0, len(t.connections))
	for peerID, conn := range t.connections {
		if conn.Connected && conn.Connection != nil && conn.Connection.IsOpen() {
			peers = append(peers, peerID)
		}
	}
	return peers
}

// SendRPC sends an RPC request and waits for response
func (t *WebRTCTransport) SendRPC(ctx context.Context, peerID string, method string, args interface{}, reply interface{}) error {
	start := time.Now()

	// Check connection
	if !t.IsConnected(peerID) {
		if err := t.Connect(ctx, peerID); err != nil {
			return fmt.Errorf("failed to connect to peer: %w", err)
		}
	}

	// Generate RPC ID
	rpcID, err := generateRPCID()
	if err != nil {
		return fmt.Errorf("failed to generate RPC ID: %w", err)
	}

	// Create RPC request
	request := RPCRequest{
		ID:      rpcID,
		Method:  method,
		Params:  args,
		Timeout: t.config.RPCTimeout.Milliseconds(),
	}

	// Marshal request
	_, err = json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal RPC request: %w", err)
	}

	// Create response channel
	responseChan := make(chan RPCResponse, 1)
	t.rpcMu.Lock()
	t.rpcResponses[rpcID] = responseChan
	t.rpcMu.Unlock()

	// Clean up response channel when done
	defer func() {
		t.rpcMu.Lock()
		delete(t.rpcResponses, rpcID)
		t.rpcMu.Unlock()
		close(responseChan)
	}()

	// Send the request
	ctx, cancel := context.WithTimeout(ctx, t.config.RPCTimeout)
	defer cancel()

	if err := t.SendMessage(ctx, peerID, request); err != nil {
		return fmt.Errorf("failed to send RPC request: %w", err)
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return ctx.Err()
	case response := <-responseChan:
		// Update metrics
		latency := time.Since(start)
		t.recordRPCLatency(latency)

		if response.Error != nil {
			return fmt.Errorf("RPC error: %s (code: %d)", response.Error.Message, response.Error.Code)
		}

		// Unmarshal response into reply
		if reply != nil && response.Result != nil {
			resultBytes, err := json.Marshal(response.Result)
			if err != nil {
				return fmt.Errorf("failed to marshal result: %w", err)
			}

			if err := json.Unmarshal(resultBytes, reply); err != nil {
				return fmt.Errorf("failed to unmarshal response: %w", err)
			}
		}

		return nil
	}
}

// StreamRPC pipes the response directly to the writer for zero-copy efficiency
func (t *WebRTCTransport) StreamRPC(ctx context.Context, peerID string, method string, args interface{}, writer io.Writer) (int64, error) {
	// For now, implement as a wrapper around SendRPC
	// In production, this would use a dedicated binary stream channel
	var result struct {
		Data []byte `json:"data"`
	}

	if err := t.SendRPC(ctx, peerID, method, args, &result); err != nil {
		return 0, err
	}

	n, err := writer.Write(result.Data)
	return int64(n), err
}

// SendMessage sends a message to a peer
func (t *WebRTCTransport) SendMessage(ctx context.Context, peerID string, message interface{}) error {
	if !t.IsConnected(peerID) {
		return errors.New("not connected to peer")
	}

	t.connMu.RLock()
	conn, exists := t.connections[peerID]
	t.connMu.RUnlock()

	if !exists || conn.Connection == nil {
		return errors.New("connection not found")
	}

	// Marshal message
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Send via connection
	if err := conn.Connection.Send(ctx, messageBytes); err != nil {
		// Mark as disconnected on send error
		conn.mu.Lock()
		conn.Connected = false
		conn.mu.Unlock()

		t.metricsMu.Lock()
		t.metrics.FailedMessages++
		t.metricsMu.Unlock()

		return fmt.Errorf("failed to send message: %w", err)
	}

	// Update metrics
	t.recordMessageSent(len(messageBytes))

	// Update last contact
	conn.mu.Lock()
	conn.LastContact = time.Now()
	conn.mu.Unlock()

	return nil
}

// Broadcast sends a message to all connected peers
func (t *WebRTCTransport) Broadcast(topic string, message interface{}) error {
	peers := t.GetConnectedPeers()

	var wg sync.WaitGroup
	errs := make(chan error, len(peers))

	for _, peerID := range peers {
		wg.Add(1)
		go func(pid string) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), t.config.RPCTimeout)
			defer cancel()

			broadcastMsg := map[string]interface{}{
				"type":    "broadcast",
				"topic":   topic,
				"message": message,
			}

			if err := t.SendMessage(ctx, pid, broadcastMsg); err != nil {
				errs <- fmt.Errorf("failed to broadcast to %s: %w", pid[:8], err)
			}
		}(peerID)
	}

	wg.Wait()
	close(errs)

	// Collect errors
	var broadcastErr error
	for err := range errs {
		if broadcastErr == nil {
			broadcastErr = err
		} else {
			broadcastErr = fmt.Errorf("%v; %v", broadcastErr, err)
		}
	}

	return broadcastErr
}

// GetPeerCapabilities retrieves capabilities of a peer
func (t *WebRTCTransport) GetPeerCapabilities(peerID string) (*common.PeerCapability, error) {
	// Check cache first
	t.connMu.RLock()
	conn, exists := t.connections[peerID]
	t.connMu.RUnlock()

	if exists && conn.Capability != nil {
		return conn.Capability, nil
	}

	// Fetch via RPC
	var capability common.PeerCapability
	ctx, cancel := context.WithTimeout(context.Background(), t.config.RPCTimeout)
	defer cancel()

	if err := t.SendRPC(ctx, peerID, "get_capabilities", nil, &capability); err != nil {
		return nil, fmt.Errorf("failed to get peer capabilities: %w", err)
	}

	// Cache the capability
	if conn != nil {
		conn.mu.Lock()
		conn.Capability = &capability
		conn.mu.Unlock()
	}

	return &capability, nil
}

// UpdateLocalCapabilities updates and broadcasts local capabilities
func (t *WebRTCTransport) UpdateLocalCapabilities(capabilities *common.PeerCapability) error {
	t.localCapability = capabilities
	return t.Broadcast("capability_update", capabilities)
}

// Advertise implements Discovery interface
func (t *WebRTCTransport) Advertise(ctx context.Context, key string, value string) error {
	// For now, broadcast to all peers or use signaling server
	return t.Broadcast("discovery.advertise", map[string]string{
		"key":   key,
		"value": value,
	})
}

// FindPeers implements Discovery interface
func (t *WebRTCTransport) FindPeers(ctx context.Context, key string) ([]common.PeerInfo, error) {
	// In production, this would query DHT and signaling server
	return nil, nil // Placeholder
}

// GetConnectionMetrics returns current transport metrics
func (t *WebRTCTransport) GetConnectionMetrics() common.ConnectionMetrics {
	t.metricsMu.RLock()
	defer t.metricsMu.RUnlock()

	// Calculate success rate if we have data
	if t.metrics.MessagesSent > 0 {
		// SuccessRate is calculated as (MessagesSent - FailedMessages) / MessagesSent
		successCount := float64(t.metrics.MessagesSent)
		if t.metrics.FailedMessages > 0 {
			successCount -= float64(t.metrics.FailedMessages)
		}
		t.metrics.SuccessRate = float32(successCount / float64(t.metrics.MessagesSent))
	}

	return t.metrics
}

// GetHealth returns transport health status
func (t *WebRTCTransport) GetHealth() common.TransportHealth {
	t.health.Uptime = time.Since(t.startTime).String()

	// Calculate health score
	var score float32

	// Check signaling connection
	if t.signalingStatus.Load().(string) == "connected" {
		score += 0.3
		t.health.SignalingActive = true
	} else {
		t.health.SignalingActive = false
	}

	// Check WebRTC support
	if t.config.WebRTCEnabled {
		score += 0.2
		t.health.WebRTCSupported = true
	}

	// Check ICE servers
	if len(t.config.ICEServers) > 0 {
		score += 0.2
		t.health.IceServers = len(t.config.ICEServers)
	}

	// Check active connections
	connectedPeers := len(t.GetConnectedPeers())
	if connectedPeers > 0 {
		score += 0.3
	}

	t.health.Score = score

	// Determine status
	if score > 0.8 {
		t.health.Status = "healthy"
	} else if score > 0.5 {
		t.health.Status = "degraded"
	} else {
		t.health.Status = "unhealthy"
	}

	return t.health
}

// GetStats returns detailed transport statistics
func (t *WebRTCTransport) GetStats() map[string]interface{} {
	metrics := t.GetConnectionMetrics()
	health := t.GetHealth()

	return map[string]interface{}{
		"node_id":         t.nodeID,
		"uptime":          t.startTime.Format(time.RFC3339),
		"connected_peers": t.GetConnectedPeers(),
		"total_peers":     len(t.connections),
		"metrics":         metrics,
		"health":          health,
		"config": map[string]interface{}{
			"webrtc_enabled":  t.config.WebRTCEnabled,
			"ice_servers":     len(t.config.ICEServers),
			"max_connections": t.config.MaxConnections,
		},
		"signaling_status":  t.signalingStatus.Load(),
		"message_queue_len": len(t.messageQueue),
		"rpc_pending":       len(t.rpcResponses),
	}
}

// ========== Internal Methods ==========

// connectSignaling connects to signaling server
func (t *WebRTCTransport) connectSignaling() error {
	t.signalingMu.Lock()
	defer t.signalingMu.Unlock()

	if t.signalingConn != nil {
		return nil // Already connected
	}

	// Try servers in order
	var lastErr error
	for _, server := range t.config.SignalingServers {
		dialer := websocket.Dialer{
			HandshakeTimeout: t.config.ConnectionTimeout,
		}

		conn, _, err := dialer.Dial(server, nil)
		if err != nil {
			lastErr = err
			t.logger.Debug("failed to connect to signaling server", "server", server, "error", err)
			continue
		}

		t.signalingConn = conn
		t.signalingURL = server
		t.signalingStatus.Store("connected")

		// Start receiving signaling messages
		go t.receiveSignalingMessages()

		t.logger.Info("connected to signaling server", "server", server)
		return nil
	}

	return fmt.Errorf("failed to connect to any signaling server: %w", lastErr)
}

// reconnectSignaling attempts to reconnect to signaling server
func (t *WebRTCTransport) reconnectSignaling() {
	for {
		select {
		case <-t.shutdown:
			return
		case <-time.After(t.config.ReconnectDelay):
			if err := t.connectSignaling(); err == nil {
				return // Successfully reconnected
			}
			t.logger.Warn("signaling reconnect failed, will retry")
		}
	}
}

// sendSignalingMessage sends a message to signaling server
func (t *WebRTCTransport) sendSignalingMessage(message interface{}) error {
	t.signalingMu.RLock()
	conn := t.signalingConn
	t.signalingMu.RUnlock()

	if conn == nil {
		return errors.New("not connected to signaling server")
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal signaling message: %w", err)
	}

	t.signalingWriteMu.Lock()
	defer t.signalingWriteMu.Unlock()
	return conn.WriteMessage(websocket.TextMessage, messageBytes)
}

// receiveSignalingMessages handles incoming signaling messages
func (t *WebRTCTransport) receiveSignalingMessages() {
	defer func() {
		t.signalingStatus.Store("disconnected")
		t.signalingMu.Lock()
		t.signalingConn = nil
		t.signalingMu.Unlock()

		// Attempt reconnect
		go t.reconnectSignaling()
	}()

	for {
		select {
		case <-t.shutdown:
			return
		default:
			t.signalingMu.RLock()
			conn := t.signalingConn
			t.signalingMu.RUnlock()

			if conn == nil {
				return
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				t.logger.Error("failed to read signaling message", "error", err)
				return
			}

			t.handleSignalingMessage(message)
		}
	}
}

// handleSignalingMessage processes signaling server messages
func (t *WebRTCTransport) handleSignalingMessage(message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		t.logger.Error("failed to unmarshal signaling message", "error", err)
		return
	}

	msgType, _ := msg["type"].(string)
	senderID, _ := msg["peer_id"].(string)

	switch msgType {
	case "webrtc_offer":
		t.handleWebRTCOffer(senderID, msg)
	case "webrtc_answer":
		t.handleWebRTCAnswer(senderID, msg)
	case "ice_candidate":
		t.handleICECandidate(senderID, msg)
	case "peer_discovery":
		t.handlePeerDiscovery(msg)
	case "ping":
		// Respond to ping
		t.sendSignalingMessage(map[string]interface{}{
			"type":    "pong",
			"peer_id": t.nodeID,
		})
	}
}

// handleWebRTCOffer processes incoming WebRTC offer
func (t *WebRTCTransport) handleWebRTCOffer(senderID string, msg map[string]interface{}) {
	offerRaw, ok := msg["offer"]
	if !ok {
		t.logger.Error("missing WebRTC offer")
		return
	}

	var offer webrtc.SessionDescription
	offerData, _ := json.Marshal(offerRaw)
	if err := json.Unmarshal(offerData, &offer); err != nil {
		t.logger.Error("failed to unmarshal WebRTC offer", "error", err)
		return
	}

	// Create peer connection
	peerConnection, err := webrtc.NewPeerConnection(t.webrtcConfig)
	if err != nil {
		t.logger.Error("failed to create peer connection", "error", err)
		return
	}

	// Store peer connection immediately so signaling (candidates) can find it
	t.pcMu.Lock()
	t.peerConnections[senderID] = peerConnection
	t.pcMu.Unlock()

	// Set up data channel
	peerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
		dc.OnOpen(func() {
			conn := &WebRTCConnection{
				peerID:   senderID,
				dc:       dc,
				pc:       peerConnection,
				stats:    ConnectionStats{OpenedAt: time.Now()},
				shutdown: make(chan struct{}),
			}

			t.connMu.Lock()
			t.connections[senderID] = &PeerConnection{
				PeerID:      senderID,
				Connection:  conn,
				Connected:   true,
				LastContact: time.Now(),
			}
			t.connMu.Unlock()

			go conn.receiveLoop()
		})

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			t.handleIncomingMessage(senderID, msg.Data)
		})
	})

	// Set remote description
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		peerConnection.Close()
		t.logger.Error("failed to set remote description", "error", err)
		return
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		peerConnection.Close()
		t.logger.Error("failed to create answer", "error", err)
		return
	}

	// Set local description
	if err := peerConnection.SetLocalDescription(answer); err != nil {
		peerConnection.Close()
		t.logger.Error("failed to set local description", "error", err)
		return
	}

	response := map[string]interface{}{
		"type":      "webrtc_answer",
		"peer_id":   t.nodeID,
		"target_id": senderID,
		"answer":    answer, // Send object directly
	}

	if err := t.sendSignalingMessage(response); err != nil {
		t.logger.Error("failed to send WebRTC answer", "error", err)
	}
}

// handleWebRTCAnswer processes incoming WebRTC answer
func (t *WebRTCTransport) handleWebRTCAnswer(senderID string, msg map[string]interface{}) {
	answerRaw, ok := msg["answer"]
	if !ok {
		t.logger.Error("missing WebRTC answer")
		return
	}

	var answer webrtc.SessionDescription
	answerData, _ := json.Marshal(answerRaw)
	if err := json.Unmarshal(answerData, &answer); err != nil {
		t.logger.Error("failed to unmarshal WebRTC answer", "error", err)
		return
	}

	t.pcMu.RLock()
	peerConnection, exists := t.peerConnections[senderID]
	t.pcMu.RUnlock()

	if !exists {
		t.logger.Error("no peer connection found for answer")
		return
	}

	// Set remote description
	if err := peerConnection.SetRemoteDescription(answer); err != nil {
		t.logger.Error("failed to set remote description", "error", err)
	}
}

// handleICECandidate processes incoming ICE candidate
func (t *WebRTCTransport) handleICECandidate(senderID string, msg map[string]interface{}) {
	candidateRaw, ok := msg["candidate"]
	if !ok {
		t.logger.Error("missing ICE candidate")
		return
	}

	var candidate webrtc.ICECandidateInit
	candidateData, _ := json.Marshal(candidateRaw)
	if err := json.Unmarshal(candidateData, &candidate); err != nil {
		t.logger.Error("failed to unmarshal ICE candidate", "error", err)
		return
	}

	t.pcMu.RLock()
	peerConnection, exists := t.peerConnections[senderID]
	t.pcMu.RUnlock()

	if !exists {
		t.logger.Error("no peer connection found for ICE candidate")
		return
	}

	// Add ICE candidate
	if err := peerConnection.AddICECandidate(candidate); err != nil {
		t.logger.Error("failed to add ICE candidate", "error", err)
	}
}

// handlePeerDiscovery processes peer discovery messages
func (t *WebRTCTransport) handlePeerDiscovery(msg map[string]interface{}) {
	peers, ok := msg["peers"].([]interface{})
	if !ok {
		return
	}

	for _, peer := range peers {
		if peerInfo, ok := peer.(map[string]interface{}); ok {
			peerID, _ := peerInfo["id"].(string)
			capabilities, _ := peerInfo["capabilities"].(string)

			if peerID != "" && peerID != t.nodeID {
				// Store peer info for later connection
				t.logger.Debug("discovered peer", "peer", peerID[:8])

				// Parse capabilities if available
				if capabilities != "" {
					var cap common.PeerCapability
					if err := json.Unmarshal([]byte(capabilities), &cap); err == nil {
						t.connMu.Lock()
						if conn, exists := t.connections[peerID]; exists {
							conn.Capability = &cap
						} else {
							t.connections[peerID] = &PeerConnection{
								PeerID:     peerID,
								Capability: &cap,
								Connected:  false,
							}
						}
						t.connMu.Unlock()
					}
				}
			}
		}
	}
}

// handleIncomingMessage processes incoming messages from peers
func (t *WebRTCTransport) handleIncomingMessage(peerID string, data []byte) {
	// Update metrics
	t.recordMessageReceived(len(data))

	// Parse message
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		t.logger.Error("failed to unmarshal incoming message", "error", err)
		return
	}

	// Check if it's an RPC response
	if id, ok := msg["id"].(string); ok {
		// It's an RPC response
		t.rpcMu.RLock()
		responseChan, exists := t.rpcResponses[id]
		t.rpcMu.RUnlock()

		if exists {
			var response RPCResponse
			if err := json.Unmarshal(data, &response); err == nil {
				responseChan <- response
			}
		}
		return
	}

	// Handle other message types
	msgType, _ := msg["type"].(string)
	switch msgType {
	case "rpc_request":
		t.handleRPCRequest(peerID, data)
	case "ping":
		// Respond to ping
		t.SendMessage(context.Background(), peerID, map[string]interface{}{
			"type": "pong",
		})
	case "pong":
		// Update latency
		if timestamp, ok := msg["timestamp"].(float64); ok {
			latency := time.Since(time.Unix(0, int64(timestamp)))
			t.updatePeerLatency(peerID, latency)
		}
	case "capability_update":
		// Update peer capabilities
		if payload, ok := msg["payload"].(map[string]interface{}); ok {
			var capability common.PeerCapability
			capBytes, _ := json.Marshal(payload)
			if err := json.Unmarshal(capBytes, &capability); err == nil {
				t.connMu.Lock()
				if conn, exists := t.connections[peerID]; exists {
					conn.Capability = &capability
				}
				t.connMu.Unlock()
			}
		}
	case "chunk_request":
		// Handle chunk request (forward to mesh coordinator)
		// This would be handled by the mesh layer
		t.logger.Debug("received chunk request", "peer", peerID[:8])
	}
}

// handleRPCRequest processes an incoming RPC request
func (t *WebRTCTransport) handleRPCRequest(peerID string, data []byte) {
	var request RPCRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return
	}

	t.handlerMu.RLock()
	handler, exists := t.rpcHandlers[request.Method]
	t.handlerMu.RUnlock()

	var result interface{}
	var err error

	if !exists {
		err = fmt.Errorf("method not found: %s", request.Method)
	} else {
		// Convert params to RawMessage for the handler
		paramsBytes, _ := json.Marshal(request.Params)
		result, err = handler(context.Background(), peerID, json.RawMessage(paramsBytes))
	}

	// Send response
	response := RPCResponse{
		ID:     request.ID,
		Result: result,
	}
	if err != nil {
		response.Error = &RPCError{
			Code:    -32601,
			Message: err.Error(),
		}
	}

	t.SendMessage(context.Background(), peerID, response)
}

// getPeerWebSocketURL returns WebSocket URL for a peer
func (t *WebRTCTransport) getPeerWebSocketURL(peerID string) (string, error) {
	// In production, this would query a signaling server or DHT
	// For now, use a simple pattern based on peer ID
	baseURL := t.config.WebSocketURL
	if baseURL == "" {
		baseURL = "wss://relay.inos.ai/ws"
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("peer_id", peerID)
	q.Set("node_id", t.nodeID)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// messageProcessor handles queued messages
func (t *WebRTCTransport) messageProcessor() {
	for {
		select {
		case <-t.shutdown:
			return
		case queued := <-t.messageQueue:
			// Try to send the message
			err := t.SendMessage(queued.Context, queued.PeerID, queued.Message)
			if err != nil && queued.Retries < t.config.MaxRetries {
				// Retry with backoff
				queued.Retries++
				go func(q QueuedMessage) {
					backoff := time.Duration(math.Pow(2, float64(q.Retries))) * 100 * time.Millisecond
					time.Sleep(backoff)
					t.messageQueue <- q
				}(queued)
			}

			// Notify sender
			if queued.Result != nil {
				queued.Result <- err
			}
		}
	}
}

// RegisterRPCHandler registers a handler for an RPC method
func (t *WebRTCTransport) RegisterRPCHandler(method string, handler func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error)) {
	t.handlerMu.Lock()
	defer t.handlerMu.Unlock()
	t.rpcHandlers[method] = handler
}

// connectionManager manages connection lifecycle
func (t *WebRTCTransport) connectionManager() {
	keepAliveTicker := time.NewTicker(t.config.KeepAliveInterval)
	defer keepAliveTicker.Stop()

	cleanupTicker := time.NewTicker(1 * time.Minute)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-t.shutdown:
			return
		case <-keepAliveTicker.C:
			t.sendKeepAlives()
		case <-cleanupTicker.C:
			t.cleanupStaleConnections()
		}
	}
}

// sendKeepAlives sends ping messages to all connected peers
func (t *WebRTCTransport) sendKeepAlives() {
	peers := t.GetConnectedPeers()

	for _, peerID := range peers {
		go func(pid string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			pingMsg := map[string]interface{}{
				"type":      "ping",
				"timestamp": time.Now().UnixNano(),
			}

			if err := t.SendMessage(ctx, pid, pingMsg); err != nil {
				t.logger.Debug("keep-alive failed", "peer", pid[:8], "error", err)
			}
		}(peerID)
	}
}

// cleanupStaleConnections removes stale connections
func (t *WebRTCTransport) cleanupStaleConnections() {
	t.connMu.Lock()
	defer t.connMu.Unlock()

	now := time.Now()
	for peerID, conn := range t.connections {
		if !conn.Connected {
			continue
		}

		// Check last contact
		if now.Sub(conn.LastContact) > 2*t.config.KeepAliveInterval {
			t.logger.Debug("cleaning up stale connection", "peer", peerID[:8])
			if conn.Connection != nil {
				conn.Connection.Close()
			}
			conn.Connected = false
		}
	}
}

// healthMonitor monitors transport health
func (t *WebRTCTransport) healthMonitor() {
	t.healthTicker = time.NewTicker(t.config.MetricsInterval)
	defer t.healthTicker.Stop()

	for {
		select {
		case <-t.shutdown:
			return
		case <-t.healthTicker.C:
			t.checkHealth()
		}
	}
}

// checkHealth performs health checks
func (t *WebRTCTransport) checkHealth() {
	// Check signaling connection
	if t.signalingStatus.Load().(string) != "connected" {
		t.logger.Warn("signaling connection lost")
		go t.reconnectSignaling()
	}

	// Check connection count
	connected := len(t.GetConnectedPeers())
	if connected == 0 && t.started.Load() {
		t.logger.Warn("no active connections")
	}

	// Log health status
	health := t.GetHealth()
	if health.Score < 0.5 {
		t.logger.Error("transport health degraded", "score", health.Score)
	}
}

// metricsCollector collects and updates metrics
func (t *WebRTCTransport) metricsCollector() {
	ticker := time.NewTicker(t.config.MetricsInterval)
	defer ticker.Stop()

	// Store latency samples for percentile calculation
	latencySamples := make([]float64, 0, 1000)

	for {
		select {
		case <-t.shutdown:
			return
		case <-ticker.C:
			// Update active connections
			connected := len(t.GetConnectedPeers())

			t.metricsMu.Lock()
			t.metrics.ActiveConnections = uint32(connected)

			// Calculate latency percentiles if we have samples
			if len(latencySamples) > 0 {
				sort.Float64s(latencySamples)
				p50Idx := int(float64(len(latencySamples)) * 0.5)
				p95Idx := int(float64(len(latencySamples)) * 0.95)

				if p50Idx < len(latencySamples) {
					t.metrics.LatencyP50 = float32(latencySamples[p50Idx])
				}
				if p95Idx < len(latencySamples) {
					t.metrics.LatencyP95 = float32(latencySamples[p95Idx])
				}

				// Reset samples for next interval
				latencySamples = make([]float64, 0, 1000)
			}
			t.metricsMu.Unlock()
		}
	}
}

// recordMessageSent updates sent message metrics
func (t *WebRTCTransport) recordMessageSent(bytes int) {
	t.metricsMu.Lock()
	t.metrics.BytesSent += uint64(bytes)
	t.metrics.MessagesSent++
	t.metricsMu.Unlock()
}

// recordMessageReceived updates received message metrics
func (t *WebRTCTransport) recordMessageReceived(bytes int) {
	t.metricsMu.Lock()
	t.metrics.BytesReceived += uint64(bytes)
	t.metrics.MessagesReceived++
	t.metricsMu.Unlock()
}

// recordRPCLatency records RPC latency for percentile calculation
// recordRPCLatency records RPC latency for percentile calculation
func (t *WebRTCTransport) recordRPCLatency(_ time.Duration) {
	// t.metricsMu.Lock()
	// t.metrics.SuccessRate++ // Removed: SuccessRate is calculated from MessagesSent and FailedMessages
	// t.metricsMu.Unlock()

	// Store latency sample (implement circular buffer in production)
	// latencyMs := float64(latency.Milliseconds())

	// In production, use a proper circular buffer
	// For simplicity, we'll just add to slice and let metrics collector handle it
	go func() {
		// This would add to a thread-safe buffer in production
	}()
}

// updatePeerLatency updates latency for a peer
func (t *WebRTCTransport) updatePeerLatency(peerID string, latency time.Duration) {
	t.connMu.Lock()
	if conn, exists := t.connections[peerID]; exists {
		conn.Latency = latency
	}
	t.connMu.Unlock()
}

// createWebRTCConfig creates WebRTC configuration
func (t *WebRTCTransport) createWebRTCConfig() webrtc.Configuration {
	var iceServers []webrtc.ICEServer

	for _, server := range t.config.ICEServers {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs: []string{server},
		})
	}

	for _, server := range t.config.STUNServers {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs: []string{server},
		})
	}

	for _, server := range t.config.TURNServers {
		// Parse TURN server credentials (username:password@server)
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:       []string{server},
			Username:   "", // Would be parsed from server string
			Credential: "", // Would be parsed from server string
		})
	}

	return webrtc.Configuration{
		ICEServers:         iceServers,
		ICETransportPolicy: webrtc.ICETransportPolicyAll,
		BundlePolicy:       webrtc.BundlePolicyMaxCompat,
		RTCPMuxPolicy:      webrtc.RTCPMuxPolicyRequire,
	}
}

// ========== WebRTCConnection Implementation ==========

// WebRTCConnection implements Connection for WebRTC DataChannel
type WebRTCConnection struct {
	peerID   string
	dc       *webrtc.DataChannel
	pc       *webrtc.PeerConnection
	stats    ConnectionStats
	shutdown chan struct{}
	mu       sync.RWMutex
}

func (c *WebRTCConnection) Send(ctx context.Context, data []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		c.mu.Lock()
		defer c.mu.Unlock()

		if c.dc == nil {
			return errors.New("data channel not open")
		}

		if err := c.dc.Send(data); err != nil {
			c.stats.LastError = err.Error()
			return err
		}

		c.stats.BytesSent += uint64(len(data))
		c.stats.MessagesSent++
		return nil
	}
}

func (c *WebRTCConnection) Receive(ctx context.Context) ([]byte, error) {
	// WebRTC DataChannel uses callbacks, so we need to implement
	// a different pattern. This is a simplified version.
	// In production, we'd use a channel-based approach.
	return nil, errors.New("not implemented for WebRTC")
}

func (c *WebRTCConnection) Close() error {
	close(c.shutdown)

	if c.dc != nil {
		if err := c.dc.Close(); err != nil {
			return err
		}
	}

	if c.pc != nil {
		if err := c.pc.Close(); err != nil {
			return err
		}
	}

	return nil
}

func (c *WebRTCConnection) IsOpen() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.dc != nil && c.dc.ReadyState() == webrtc.DataChannelStateOpen
}

func (c *WebRTCConnection) GetStats() ConnectionStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

func (c *WebRTCConnection) receiveLoop() {
	// WebRTC uses callbacks, so this just keeps the connection alive
	<-c.shutdown
}

// ========== WebSocketConnection Implementation ==========

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
				if websocket.IsUnexpectedCloseError(err,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure) {
					// Log unexpected closure
				}
				return
			}

			c.stats.BytesReceived += uint64(len(message))
			c.stats.MessagesRecv++

			// Handle message
			handler(c.peerID, message)
		}
	}
}

// ========== Helper Functions ==========

// generateRPCID generates a unique RPC ID
func generateRPCID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// ConnectionPool implementation
func (p *ConnectionPool) Get(peerID string) (Connection, bool) {
	p.mu.RLock()
	conn, exists := p.connections[peerID]
	p.mu.RUnlock()
	if !exists {
		return nil, false
	}
	return *conn, exists
}

func (p *ConnectionPool) Put(peerID string, conn Connection) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.connections) >= p.maxSize {
		// Evict oldest connection
		p.evictOldest()
	}

	p.connections[peerID] = &conn
}

func (p *ConnectionPool) Remove(peerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.connections, peerID)
}

func (p *ConnectionPool) evictOldest() {
	// In production, implement LRU eviction
	// For now, just remove first connection
	for peerID := range p.connections {
		delete(p.connections, peerID)
		break
	}
}

func (p *ConnectionPool) Cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for peerID, conn := range p.connections {
		if now.Sub((*conn).GetStats().OpenedAt) > p.idleTimeout {
			(*conn).Close()
			delete(p.connections, peerID)
		}
	}
}

// MockTransport for testing
// Implement adaptor methods for SendRPC mapped logic for legacy DHT support
func (t *WebRTCTransport) FindNode(ctx context.Context, peerID, targetID string) ([]common.PeerInfo, error) {
	var nodes []common.PeerInfo
	err := t.SendRPC(ctx, peerID, "find_node", map[string]string{"target_id": targetID}, &nodes)
	return nodes, err
}

func (t *WebRTCTransport) FindValue(ctx context.Context, peerID, chunkHash string) ([]string, []common.PeerInfo, error) {
	var result struct {
		Values []string          `json:"values"`
		Nodes  []common.PeerInfo `json:"nodes"`
	}
	err := t.SendRPC(ctx, peerID, "find_value", map[string]string{"key": chunkHash}, &result)
	return result.Values, result.Nodes, err
}

func (t *WebRTCTransport) Store(ctx context.Context, peerID string, key string, value []byte) error {
	return t.SendRPC(ctx, peerID, "store", map[string]interface{}{"key": key, "value": value}, nil)
}

func (t *WebRTCTransport) Ping(ctx context.Context, peerID string) error {
	return t.SendRPC(ctx, peerID, "ping", nil, nil)
}

func (t *WebRTCTransport) Send(toPeerID string, payload interface{}) error {
	return t.SendMessage(context.Background(), toPeerID, payload)
}

func getShortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
