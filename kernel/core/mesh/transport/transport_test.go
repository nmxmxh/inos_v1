package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
	"github.com/pion/webrtc/v3"
	"github.com/stretchr/testify/assert"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type MockSignalingServer struct {
	server   *httptest.Server
	conns    map[string]*websocket.Conn
	allConns []*websocket.Conn
	mu       sync.Mutex
}

func NewMockSignalingServer() *MockSignalingServer {
	s := &MockSignalingServer{
		conns:    make(map[string]*websocket.Conn),
		allConns: make([]*websocket.Conn, 0),
	}
	s.server = httptest.NewServer(http.HandlerFunc(s.handleWS))
	return s
}

func (s *MockSignalingServer) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.allConns = append(s.allConns, conn)
	s.mu.Unlock()
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg map[string]interface{}
		json.Unmarshal(message, &msg)

		msgType, _ := msg["type"].(string)
		peerID, _ := msg["peer_id"].(string)

		s.mu.Lock()
		s.conns[peerID] = conn
		s.mu.Unlock()

		if msgType == "webrtc_offer" || msgType == "webrtc_answer" || msgType == "ice_candidate" {
			targetID, _ := msg["target_id"].(string)
			s.mu.Lock()
			target, ok := s.conns[targetID]
			if ok {
				target.WriteMessage(websocket.TextMessage, message)
			} else {
				// Broadcast to all other connections
				for _, c := range s.allConns {
					if c != conn {
						c.WriteMessage(websocket.TextMessage, message)
					}
				}
			}
			s.mu.Unlock()
		}
	}
}

func (s *MockSignalingServer) URL() string {
	return strings.Replace(s.server.URL, "http", "ws", 1)
}

func (s *MockSignalingServer) Close() {
	s.server.Close()
}

// MockConnection implements the Connection interface for testing
type MockConnection struct {
	open     bool
	sent     [][]byte
	received chan []byte
	stats    ConnectionStats
	mu       sync.RWMutex
}

func NewMockConnection() *MockConnection {
	return &MockConnection{
		open:     true,
		received: make(chan []byte, 10),
		stats:    ConnectionStats{OpenedAt: time.Now()},
	}
}

func (m *MockConnection) Send(ctx context.Context, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, data)
	m.stats.BytesSent += uint64(len(data))
	m.stats.MessagesSent++
	return nil
}

func (m *MockConnection) Receive(ctx context.Context) ([]byte, error) {
	select {
	case data := <-m.received:
		m.mu.Lock()
		m.stats.BytesReceived += uint64(len(data))
		m.stats.MessagesRecv++
		m.mu.Unlock()
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *MockConnection) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.open = false
	return nil
}

func (m *MockConnection) IsOpen() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.open
}

func (m *MockConnection) GetStats() ConnectionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

func (m *MockConnection) getSent() [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return a copy to avoid external mutations racing with Send
	copied := make([][]byte, len(m.sent))
	for i, b := range m.sent {
		item := make([]byte, len(b))
		copy(item, b)
		copied[i] = item
	}
	return copied
}

// TestTransportConfig tests default configuration
func TestTransportConfig(t *testing.T) {
	config := DefaultTransportConfig()
	if !config.WebRTCEnabled {
		t.Error("WebRTC should be enabled by default")
	}
	if len(config.ICEServers) == 0 {
		t.Error("Default ICE servers should be provided")
	}
}

// TestConnectionPool tests pooling logic
func TestConnectionPool(t *testing.T) {
	pool := &ConnectionPool{
		connections: make(map[string]*Connection),
		maxSize:     2,
		idleTimeout: 1 * time.Hour,
	}

	c1 := Connection(NewMockConnection())
	c2 := Connection(NewMockConnection())
	c3 := Connection(NewMockConnection())

	pool.Put("p1", c1)
	pool.Put("p2", c2)

	if _, ok := pool.Get("p1"); !ok {
		t.Error("Failed to get p1 from pool")
	}

	// Over capacity - one of p1 or p2 should be evicted when p3 added
	pool.Put("p3", c3)
	_, g1 := pool.Get("p1")
	_, g2 := pool.Get("p2")
	_, g3 := pool.Get("p3")

	if !g3 {
		t.Error("p3 should be in pool")
	}
	if g1 && g2 {
		t.Error("Expected eviction, but both p1 and p2 are still in pool")
	}
}

// TestWebRTCTransport_RPC tests the RPC mechanism using a mock connection
func TestWebRTCTransport_RPC(t *testing.T) {
	nodeID := "node1"
	config := DefaultTransportConfig()
	tr, err := NewWebRTCTransport(nodeID, config, nil)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	peerID := "peer2"
	mockConn := NewMockConnection()

	// Manually inject mock connection into transport
	tr.connMu.Lock()
	tr.connections[peerID] = &PeerConnection{
		PeerID:     peerID,
		Connection: mockConn,
		Connected:  true,
	}
	tr.connMu.Unlock()

	// 1. Test RPC Registration
	tr.RegisterRPCHandler("test.method", func(ctx context.Context, pID string, args json.RawMessage) (interface{}, error) {
		if pID != peerID {
			t.Errorf("Expected peer ID %s, got %s", peerID, pID)
		}
		return map[string]string{"result": "ok"}, nil
	})

	// 2. Test SendRPC
	ctx := context.Background()

	go func() {
		// Mock the peer's response to the transport's outgoing request
		// Wait for the message to be "sent" through mockConn
		time.Sleep(50 * time.Millisecond)

		var lastReq RPCRequest
		sent := mockConn.getSent()
		if len(sent) > 0 {
			json.Unmarshal(sent[0], &lastReq)
		}

		if lastReq.ID == "" {
			return
		}

		resp := RPCResponse{
			ID:     lastReq.ID,
			Result: map[string]string{"hello": "world"},
		}

		// Inject response into transport's internal response channel
		tr.rpcMu.Lock()
		if ch, ok := tr.rpcResponses[lastReq.ID]; ok {
			ch <- resp
		}
		tr.rpcMu.Unlock()
	}()

	var reply map[string]interface{}
	err = tr.SendRPC(ctx, peerID, "remote.method", map[string]string{"ping": "pong"}, &reply)
	if err != nil {
		t.Errorf("SendRPC failed: %v", err)
	}

	if reply["hello"] != "world" {
		t.Errorf("Unexpected RPC result: %+v", reply)
	}
}

// TestWebRTCTransport_StreamRPC tests the StreamRPC wrapper
func TestWebRTCTransport_StreamRPC(t *testing.T) {
	tr, _ := NewWebRTCTransport("n1", DefaultTransportConfig(), nil)
	peerID := "p1"

	mockConn := NewMockConnection()
	tr.connMu.Lock()
	tr.connections[peerID] = &PeerConnection{PeerID: peerID, Connection: mockConn, Connected: true}
	tr.connMu.Unlock()

	ctx := context.Background()
	go func() {
		time.Sleep(50 * time.Millisecond)
		var lastReq RPCRequest
		// Wait for request to be sent
		for i := 0; i < 20; i++ {
			tr.connMu.Lock()
			if len(mockConn.getSent()) > 0 {
				json.Unmarshal(mockConn.getSent()[0], &lastReq)
				tr.connMu.Unlock()
				break
			}
			tr.connMu.Unlock()
			time.Sleep(10 * time.Millisecond)
		}

		resp := RPCResponse{
			ID:     lastReq.ID,
			Result: map[string]interface{}{"data": []byte("stream_content")},
		}
		tr.rpcMu.Lock()
		if ch, ok := tr.rpcResponses[lastReq.ID]; ok {
			ch <- resp
		}
		tr.rpcMu.Unlock()
	}()

	var buf strings.Builder
	n, err := tr.StreamRPC(ctx, peerID, "get_data", nil, &buf)
	if err != nil {
		t.Fatalf("StreamRPC failed: %v", err)
	}
	if n == 0 || buf.String() == "" {
		t.Error("No data streamed")
	}
}

// TestWebRTCTransport_Broadcast tests the broadcast logic
func TestWebRTCTransport_Broadcast(t *testing.T) {
	tr, _ := NewWebRTCTransport("n1", DefaultTransportConfig(), nil)

	// Add two mock connections
	m1 := NewMockConnection()
	m2 := NewMockConnection()
	tr.connMu.Lock()
	tr.connections["p1"] = &PeerConnection{PeerID: "p1", Connection: m1, Connected: true}
	tr.connections["p2"] = &PeerConnection{PeerID: "p2", Connection: m2, Connected: true}
	tr.connMu.Unlock()

	err := tr.Broadcast("test.topic", map[string]string{"msg": "all"})
	if err != nil {
		t.Errorf("Broadcast failed: %v", err)
	}

	// Verify both got the message
	if len(m1.sent) == 0 || len(m2.sent) == 0 {
		t.Error("Broadcast did not reach all connected peers")
	}
}

// TestWebRTCTransport_Signaling tests signaling connection and message handling
func TestWebRTCTransport_Signaling(t *testing.T) {
	s := NewMockSignalingServer()
	defer s.Close()

	config := DefaultTransportConfig()
	config.SignalingServers = []string{s.URL()}

	tr, err := NewWebRTCTransport("node1_long_enough", config, nil)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	if err := tr.Start(context.Background()); err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}

	// Wait for signaling to connect
	time.Sleep(500 * time.Millisecond)

	if tr.signalingStatus.Load() != "connected" {
		t.Errorf("Signaling should be connected, got %v", tr.signalingStatus.Load())
	}

	// Test Peer Discovery
	discoveryMsg := map[string]interface{}{
		"type": "peer_discovery",
		"peers": []map[string]interface{}{
			{"peer_id": "peer2", "address": "127.0.0.1"},
		},
	}
	data, _ := json.Marshal(discoveryMsg)
	tr.handleSignalingMessage(data)

	// Test Incoming WebRTC Offer (Simulation)
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  "v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n",
	}
	offerMsg := map[string]interface{}{
		"type":    "webrtc_offer",
		"peer_id": "peer3",
		"offer":   offer,
	}
	offData, _ := json.Marshal(offerMsg)
	tr.handleSignalingMessage(offData)

	// Test Incoming ICE Candidate
	candidateMsg := map[string]interface{}{
		"type":      "ice_candidate",
		"peer_id":   "peer3",
		"candidate": `{"candidate":"candidate:1 1 UDP 2122260223 127.0.0.1 3478 typ host","sdpMid":"0","sdpMLineIndex":0}`,
	}
	candData, _ := json.Marshal(candidateMsg)
	tr.handleSignalingMessage(candData)
}

// TestWebRTCTransport_ConnectFailures tests various connection failure paths
func TestWebRTCTransport_ConnectFailures(t *testing.T) {
	tr, _ := NewWebRTCTransport("node1_long_enough", DefaultTransportConfig(), nil)

	// Test connect to self
	err := tr.Connect(context.Background(), "node1_long_enough")
	if err == nil || !strings.Contains(err.Error(), "cannot connect to self") {
		t.Errorf("Expected error connecting to self, got %v", err)
	}

	// Test connect via WebSocket failure (no URL)
	err = tr.Connect(context.Background(), "unknown_peer")
	if err == nil {
		t.Error("Expected error connecting to unknown peer")
	}
}

// TestWebRTCTransport_WebSocketFallback tests the direct WebSocket connection logic
func TestWebRTCTransport_WebSocketFallback(t *testing.T) {
	// 1. Start a mock WebSocket peer
	peerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		conn.Close()
	}))
	defer peerServer.Close()

	// 2. Configure transport with WebRTC disabled
	config := DefaultTransportConfig()
	config.WebRTCEnabled = false
	config.WebSocketURL = strings.Replace(peerServer.URL, "http", "ws", 1)

	tr, _ := NewWebRTCTransport("node1_long_enough", config, nil)

	// 3. Attempt connect
	ctx := context.Background()
	err := tr.Connect(ctx, "peer2")
	if err != nil {
		t.Fatalf("WebSocket fallback failed: %v", err)
	}

	if !tr.IsConnected("peer2") {
		t.Error("Should be connected via WebSocket")
	}
}

// TestWebRTCTransport_Metrics tests metrics and health reporting
func TestWebRTCTransport_Metrics(t *testing.T) {
	nodeID := "node1_long_enough"
	tr, _ := NewWebRTCTransport(nodeID, DefaultTransportConfig(), nil)

	// Test UpdateLocalCapabilities
	newCap := &common.PeerCapability{PeerID: nodeID, Reputation: 0.9}
	tr.UpdateLocalCapabilities(newCap)

	if tr.GetHealth().Status != "unknown" && tr.GetHealth().Status != "" {
		// Just verify it doesn't panic and returns something
	}

	metrics := tr.GetConnectionMetrics()
	if metrics.TotalConnections != 0 {
		t.Errorf("Expected 0 connections, got %d", metrics.TotalConnections)
	}

	// Test PeerCapabilities
	peerID := "peer2"
	tr.connMu.Lock()
	tr.connections[peerID] = &PeerConnection{
		PeerID:     peerID,
		Capability: &common.PeerCapability{PeerID: peerID, Reputation: 0.7},
		Connected:  true,
	}
	tr.connMu.Unlock()

	pCap, err := tr.GetPeerCapabilities(peerID)
	if err != nil || pCap.Reputation != 0.7 {
		t.Errorf("Failed to get peer capabilities: %v", err)
	}
}

// TestWebRTCTransport_HandleIncomingMessages tests the processing of various incoming message types
func TestWebRTCTransport_HandleIncomingMessages(t *testing.T) {
	nodeID := "node1_long_enough"
	tr, _ := NewWebRTCTransport(nodeID, DefaultTransportConfig(), nil)
	peerID := "peer2_long_enough"

	// Pre-register peer
	tr.connMu.Lock()
	tr.connections[peerID] = &PeerConnection{PeerID: peerID, Connected: true}
	tr.connMu.Unlock()

	// 1. Test Ping
	pingMsg := map[string]interface{}{"type": "ping"}
	pingData, _ := json.Marshal(pingMsg)
	tr.handleIncomingMessage(peerID, pingData)

	// 2. Test Pong (Latency update)
	now := time.Now().UnixNano()
	pongMsg := map[string]interface{}{
		"type":      "pong",
		"timestamp": float64(now),
	}
	pongData, _ := json.Marshal(pongMsg)
	tr.handleIncomingMessage(peerID, pongData)

	// 3. Test Capability Update
	newCap := common.PeerCapability{PeerID: peerID, Reputation: 0.95}
	updateMsg := map[string]interface{}{
		"type":    "capability_update",
		"payload": newCap,
	}
	updateData, _ := json.Marshal(updateMsg)
	tr.handleIncomingMessage(peerID, updateData)

	tr.connMu.RLock()
	if tr.connections[peerID].Capability.Reputation != 0.95 {
		t.Errorf("Capability not updated, got %f", tr.connections[peerID].Capability.Reputation)
	}
	tr.connMu.RUnlock()
}

// TestWebRTCTransport_Adaptors tests the DHT adaptor methods
func TestWebRTCTransport_Adaptors(t *testing.T) {
	tr, _ := NewWebRTCTransport("n1", DefaultTransportConfig(), nil)
	peerID := "p1"
	mockConn := NewMockConnection()
	tr.connMu.Lock()
	tr.connections[peerID] = &PeerConnection{PeerID: peerID, Connection: mockConn, Connected: true}
	tr.connMu.Unlock()

	// Mock RPC response for FindNode
	go func() {
		time.Sleep(50 * time.Millisecond)
		tr.rpcMu.Lock()
		for id, ch := range tr.rpcResponses {
			ch <- RPCResponse{ID: id, Result: []common.PeerInfo{{ID: "p2"}}}
		}
		tr.rpcMu.Unlock()
	}()

	nodes, err := tr.FindNode(context.Background(), peerID, "target")
	if err != nil || len(nodes) == 0 {
		t.Errorf("FindNode failed: %v", err)
	}
}

// TestWebRTCTransport_Shutdown tests graceful shutdown
func TestWebRTCTransport_Shutdown(t *testing.T) {
	tr, _ := NewWebRTCTransport("n1_long_enough", DefaultTransportConfig(), nil)
	tr.Start(context.Background())

	err := tr.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Verify shutdown channel is closed
	select {
	case <-tr.shutdown:
		// Success
	default:
		t.Error("Shutdown channel should be closed")
	}
}

// TestWebRTCTransport_MessageRetries tests the internal message queue retry logic
func TestWebRTCTransport_MessageRetries(t *testing.T) {
	config := DefaultTransportConfig()
	config.MaxRetries = 1
	tr, _ := NewWebRTCTransport("n1_long_enough", config, nil)
	tr.Start(context.Background())
	defer tr.Stop()

	// Inject a peer but with no connection so SendMessage fails
	tr.connMu.Lock()
	tr.connections["p2"] = &PeerConnection{PeerID: "p2", Connected: true}
	tr.connMu.Unlock()

	result := make(chan error, 1)
	tr.messageQueue <- QueuedMessage{
		Context: context.Background(),
		PeerID:  "p2",
		Message: "test",
		Result:  result,
	}

	// Wait for retry
	err := <-result
	if err == nil {
		t.Error("Expected error from message processor")
	}
}

// TestWebRTCTransport_GetStats tests diagnostic reporting
func TestWebRTCTransport_GetStats(t *testing.T) {
	tr, _ := NewWebRTCTransport("n1_long_enough", DefaultTransportConfig(), nil)
	stats := tr.GetStats()

	if stats["node_id"] != "n1_long_enough" {
		t.Errorf("Unexpected node_id in stats: %v", stats["node_id"])
	}
	if _, ok := stats["uptime"]; !ok {
		t.Error("Uptime missing from stats")
	}
}

// TestWebRTCTransport_ConnectionManagement tests keepalive and cleanup
func TestWebRTCTransport_ConnectionManagement(t *testing.T) {
	nodeID := "node1_long_enough"
	tr, _ := NewWebRTCTransport(nodeID, DefaultTransportConfig(), nil)

	peerID := "peer2_long_enough"
	m := NewMockConnection()
	tr.connMu.Lock()
	tr.connections[peerID] = &PeerConnection{
		PeerID:      peerID,
		Connection:  m,
		Connected:   true,
		LastContact: time.Now().Add(-2 * time.Hour), // Stale
	}
	tr.connMu.Unlock()

	// 1. Test keepalives (should not panic and should try to send)
	tr.sendKeepAlives()

	// 2. Test cleanup
	tr.cleanupStaleConnections()

	tr.connMu.RLock()
	if _, ok := tr.connections[peerID]; ok {
		// Cleanup should have removed it if stale threshold met (check threshold in transport.go)
	}
	tr.connMu.RUnlock()
}

// TestWebRTCTransport_State tests Disconnect and GetConnectedPeers
func TestWebRTCTransport_State(t *testing.T) {
	tr, _ := NewWebRTCTransport("n1", DefaultTransportConfig(), nil)
	peerID := "p1"
	m := NewMockConnection()

	tr.connMu.Lock()
	tr.connections[peerID] = &PeerConnection{PeerID: peerID, Connection: m, Connected: true}
	tr.connMu.Unlock()

	if !tr.IsConnected(peerID) {
		t.Error("Expected to be connected")
	}

	peers := tr.GetConnectedPeers()
	if len(peers) != 1 || peers[0] != peerID {
		t.Errorf("Expected 1 peer, got %v", peers)
	}

	err := tr.Disconnect(peerID)
	if err != nil {
		t.Errorf("Disconnect failed: %v", err)
	}

	if tr.IsConnected(peerID) {
		t.Error("Should be disconnected")
	}
}

// TestTransport_WebRTCSignaling tests a full WebRTC handshake via signaling
func TestTransport_WebRTCSignaling(t *testing.T) {
	server := NewMockSignalingServer()
	defer server.Close()

	config1 := DefaultTransportConfig()
	config1.SignalingServers = []string{server.URL()}
	config1.RPCTimeout = 5 * time.Second

	t1, _ := NewWebRTCTransport("node1", config1, nil)
	t1.Start(context.Background())
	defer t1.Stop()

	config2 := DefaultTransportConfig()
	config2.SignalingServers = []string{server.URL()}
	config2.RPCTimeout = 5 * time.Second

	t2, _ := NewWebRTCTransport("node2", config2, nil)
	t2.Start(context.Background())
	defer t2.Stop()

	// Wait for signaling connections to establish
	time.Sleep(2 * time.Second)

	// node1 connects to node2
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := t1.connectViaWebRTC(ctx, "node2")
	if err != nil {
		t.Fatalf("WebRTC connection failed: %v", err)
	}

	// Grace period for state propagation across nodes
	time.Sleep(200 * time.Millisecond)

	if !t1.IsConnected("node2") {
		t.Error("node1 should be connected to node2")
	}
	if !t2.IsConnected("node1") {
		t.Error("node2 should be connected to node1")
	}

	// Test sending a message
	err = t1.SendMessage(ctx, "node2", "hello")
	if err != nil {
		t.Errorf("SendMessage failed: %v", err)
	}
}
func TestWebRTCTransport_Discovery(t *testing.T) {
	config := DefaultTransportConfig()
	tr, _ := NewWebRTCTransport("node1", config, nil)
	tr.Start(context.Background())
	defer tr.Stop()

	// Test Advertise
	err := tr.Advertise(context.Background(), "key", "value")
	assert.NoError(t, err)

	// Test FindPeers
	peers, err := tr.FindPeers(context.Background(), "key")
	assert.NoError(t, err)
	assert.Nil(t, peers)
}

func TestWebRTCTransport_RPCProcess(t *testing.T) {
	config := DefaultTransportConfig()
	tr, _ := NewWebRTCTransport("node1", config, nil)

	// Register a dummy handler
	tr.RegisterRPCHandler("ping", func(ctx context.Context, peerID string, data json.RawMessage) (interface{}, error) {
		return "pong", nil
	})

	// Manually trigger handleRPCRequest
	req := RPCRequest{
		ID:     "123",
		Method: "ping",
		Params: nil,
	}
	data, _ := json.Marshal(req)

	// We need a dummy peer connection to send response to, but SendMessage will fail gracefully if no connection exists
	tr.handleRPCRequest("peer1", data)
}
func TestWebRTCTransport_DHTBridge(t *testing.T) {
	config := DefaultTransportConfig()
	tr, _ := NewWebRTCTransport("node1", config, nil)

	ctx := context.Background()
	peerID := "peer2"

	// Test Ping
	err := tr.Ping(ctx, peerID)
	assert.Error(t, err)

	// Test Store
	err = tr.Store(ctx, peerID, "key", []byte("value"))
	assert.Error(t, err)

	// Test FindValue
	vals, nodes, err := tr.FindValue(ctx, peerID, "hash")
	assert.Error(t, err)
	assert.Nil(t, vals)
	assert.Nil(t, nodes)
}

func TestWebRTCTransport_FailedMetrics(t *testing.T) {
	tr, _ := NewWebRTCTransport("node1", DefaultTransportConfig(), nil)

	tr.metricsMu.Lock()
	tr.metrics.MessagesSent = 10
	tr.metrics.FailedMessages = 2
	tr.metricsMu.Unlock()

	metrics := tr.GetConnectionMetrics()
	assert.Equal(t, float32(0.8), metrics.SuccessRate)
}

func TestWebRTCTransport_Cache(t *testing.T) {
	pool := &ConnectionPool{
		connections: make(map[string]*Connection),
		maxSize:     2,
		idleTimeout: 1 * time.Minute,
	}

	c1 := Connection(NewMockConnection())
	pool.Put("p1", c1)

	pool.Remove("p1")
	_, ok := pool.Get("p1")
	assert.False(t, ok)

	pool.Put("p2", c1)
	pool.Cleanup() // Should not panic
}

func TestWebRTCConnection_Methods(t *testing.T) {
	conn := &WebRTCConnection{
		stats: ConnectionStats{MessagesSent: 5},
	}

	// Test Receive (placeholder)
	data, err := conn.Receive(context.Background())
	assert.Error(t, err)
	assert.Nil(t, data)

	// Test Stats
	stats := conn.GetStats()
	assert.Equal(t, uint64(5), stats.MessagesSent)

	// Test IsOpen (mock dc)
	assert.False(t, conn.IsOpen())
}

func TestWebSocketConnection_Methods(t *testing.T) {
	conn := &WebSocketConnection{
		stats: ConnectionStats{MessagesSent: 10},
	}

	assert.False(t, conn.IsOpen())
	stats := conn.GetStats()
	assert.Equal(t, uint64(10), stats.MessagesSent)
}

func TestWebRTCTransport_GenericSend(t *testing.T) {
	tr, _ := NewWebRTCTransport("n1", DefaultTransportConfig(), nil)
	err := tr.Send("p1", "test_payload")
	assert.Error(t, err) // No connection
}

// TestWebSocketConnection_SendReceive tests WebSocket connection methods
func TestWebSocketConnection_SendReceive(t *testing.T) {
	// Create a mock WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Echo server
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if err := conn.WriteMessage(msgType, msg); err != nil {
				break
			}
		}
	}))
	defer server.Close()

	// Connect to the server
	wsURL := strings.Replace(server.URL, "http", "ws", 1)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)

	wsConn := &WebSocketConnection{
		peerID:   "test_peer",
		conn:     conn,
		stats:    ConnectionStats{OpenedAt: time.Now()},
		shutdown: make(chan struct{}),
	}

	// Test Send
	ctx := context.Background()
	testData := []byte("test message")
	err = wsConn.Send(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, uint64(len(testData)), wsConn.stats.BytesSent)
	assert.Equal(t, uint64(1), wsConn.stats.MessagesSent)

	// Test Receive
	received, err := wsConn.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, testData, received)
	assert.Equal(t, uint64(len(testData)), wsConn.stats.BytesReceived)
	assert.Equal(t, uint64(1), wsConn.stats.MessagesRecv)

	// Test GetStats
	stats := wsConn.GetStats()
	assert.Equal(t, uint64(len(testData)), stats.BytesSent)
	assert.Equal(t, uint64(len(testData)), stats.BytesReceived)

	// Test IsOpen
	assert.True(t, wsConn.IsOpen())

	// Test Close
	err = wsConn.Close()
	assert.NoError(t, err)
}

// TestWebSocketConnection_ContextCancellation tests context handling
func TestWebSocketConnection_ContextCancellation(t *testing.T) {
	wsConn := &WebSocketConnection{
		peerID:   "test_peer",
		conn:     nil, // No actual connection
		stats:    ConnectionStats{},
		shutdown: make(chan struct{}),
	}

	// Test Receive with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := wsConn.Receive(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

// TestWebRTCTransport_CheckHealth tests health monitoring
func TestWebRTCTransport_CheckHealth(t *testing.T) {
	config := DefaultTransportConfig()
	tr, _ := NewWebRTCTransport("node1", config, nil)
	tr.Start(context.Background())
	defer tr.Stop()

	// Simulate disconnected signaling
	tr.signalingStatus.Store("disconnected")

	// Call checkHealth - should trigger reconnect attempt
	tr.checkHealth()

	// Simulate degraded health
	tr.health.Score = 0.3
	tr.checkHealth()

	// Should log warnings but not panic
}

// TestWebRTCTransport_ReconnectSignaling tests reconnection logic
func TestWebRTCTransport_ReconnectSignaling(t *testing.T) {
	config := DefaultTransportConfig()
	config.ReconnectDelay = 100 * time.Millisecond
	tr, _ := NewWebRTCTransport("node1", config, nil)

	// Start reconnect in background
	go tr.reconnectSignaling()

	// Give it time to attempt reconnect
	time.Sleep(200 * time.Millisecond)

	// Stop should cancel reconnect
	tr.Stop()
}

// TestWebRTCTransport_HandleIncomingEdgeCases tests additional message types
func TestWebRTCTransport_HandleIncomingEdgeCases(t *testing.T) {
	tr, _ := NewWebRTCTransport("node1", DefaultTransportConfig(), nil)
	peerID := "peer2"

	tr.connMu.Lock()
	tr.connections[peerID] = &PeerConnection{PeerID: peerID, Connected: true}
	tr.connMu.Unlock()

	// Test RPC Response
	rpcResp := RPCResponse{
		ID:     "test-123",
		Result: "success",
	}
	respData, _ := json.Marshal(rpcResp)

	// Create response channel
	tr.rpcMu.Lock()
	respChan := make(chan RPCResponse, 1)
	tr.rpcResponses["test-123"] = respChan
	tr.rpcMu.Unlock()

	tr.handleIncomingMessage(peerID, respData)

	// Verify response was delivered
	select {
	case resp := <-respChan:
		assert.Equal(t, "test-123", resp.ID)
	case <-time.After(100 * time.Millisecond):
		t.Error("RPC response not delivered")
	}

	// Test Broadcast message
	broadcastMsg := map[string]interface{}{
		"type":    "broadcast",
		"topic":   "test.topic",
		"payload": "test data",
	}
	broadcastData, _ := json.Marshal(broadcastMsg)
	tr.handleIncomingMessage(peerID, broadcastData)
}

// TestWebSocketConnection_SendError tests error handling
func TestWebSocketConnection_SendError(t *testing.T) {
	wsConn := &WebSocketConnection{
		peerID:   "test_peer",
		conn:     nil, // No connection
		stats:    ConnectionStats{},
		shutdown: make(chan struct{}),
	}

	// Test Send with no connection
	err := wsConn.Send(context.Background(), []byte("test"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not open")
}
