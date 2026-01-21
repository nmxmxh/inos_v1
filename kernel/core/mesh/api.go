package mesh

import (
	"context"
	"errors"
	"time"

	p2p "github.com/nmxmxh/inos_v1/kernel/gen/p2p/v1"
)

// FindPeersWithChunk resolves peer capabilities that advertise the chunk.
func (m *MeshCoordinator) FindPeersWithChunk(ctx context.Context, chunkHash string) ([]*PeerCapability, error) {
	if chunkHash == "" {
		return nil, errors.New("chunk hash is required")
	}
	peerIDs, err := m.dht.FindPeers(chunkHash)
	if err != nil {
		return nil, err
	}
	return m.fetchPeerCapabilities(ctx, peerIDs)
}

// RegisterChunk advertises local chunk availability to the mesh.
func (m *MeshCoordinator) RegisterChunk(ctx context.Context, chunkHash string) error {
	if chunkHash == "" {
		return errors.New("chunk hash is required")
	}
	m.localChunksMu.Lock()
	m.localChunks[chunkHash] = struct{}{}
	m.localChunksMu.Unlock()
	if err := m.dht.Store(chunkHash, m.nodeID, 3600); err != nil {
		return err
	}
	if err := m.gossip.AnnounceChunk(chunkHash); err != nil {
		return err
	}
	m.emitChunkDiscoveredEvent(chunkHash, m.nodeID, p2p.ChunkPriority_medium)
	return nil
}

// UnregisterChunk removes local chunk availability from the mesh view.
func (m *MeshCoordinator) UnregisterChunk(ctx context.Context, chunkHash string) error {
	if chunkHash == "" {
		return errors.New("chunk hash is required")
	}
	m.localChunksMu.Lock()
	delete(m.localChunks, chunkHash)
	m.localChunksMu.Unlock()
	return m.dht.RemoveChunkPeer(chunkHash, m.nodeID)
}

// ScheduleChunkPrefetch fetches chunks in the background to warm local caches.
func (m *MeshCoordinator) ScheduleChunkPrefetch(ctx context.Context, chunkHashes []string, priority string) error {
	if len(chunkHashes) == 0 {
		return errors.New("chunk hashes required")
	}

	timeout := 20 * time.Second
	switch priority {
	case "background":
		timeout = 45 * time.Second
	case "aggressive":
		timeout = 10 * time.Second
	}

	for _, hash := range chunkHashes {
		chunkHash := hash
		go func() {
			pctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			data, err := m.FetchChunk(pctx, chunkHash)
			if err != nil {
				m.logger.Debug("prefetch failed", "chunk", getShortID(chunkHash), "error", err)
				return
			}
			if m.storage != nil {
				_ = m.storage.StoreChunk(pctx, chunkHash, data)
			}
		}()
	}
	return nil
}

// ReportPeerPerformance updates reputation and circuit breakers based on outcomes.
func (m *MeshCoordinator) ReportPeerPerformance(peerID string, success bool, latencyMs float32, _ string) error {
	if peerID == "" {
		return errors.New("peer ID is required")
	}
	m.reputation.Report(peerID, success, float64(latencyMs))
	m.updateCircuitBreaker(peerID, success)
	score, _ := m.reputation.GetTrustScore(peerID)
	reason := "peer_performance"
	m.emitReputationUpdate(peerID, score, reason)
	return nil
}

// GetPeerReputation returns score and confidence for a peer.
func (m *MeshCoordinator) GetPeerReputation(peerID string) (float64, float64, error) {
	if peerID == "" {
		return 0, 0, errors.New("peer ID is required")
	}
	score, confidence := m.reputation.GetTrustScore(peerID)
	return score, confidence, nil
}

// GetTopPeers returns the top peer IDs by reputation.
func (m *MeshCoordinator) GetTopPeers(limit int) []string {
	if limit <= 0 {
		limit = 10
	}
	return m.reputation.GetTopPeers(limit)
}

// ConnectToPeer establishes a transport connection and updates routing.
func (m *MeshCoordinator) ConnectToPeer(ctx context.Context, peerID string) error {
	if peerID == "" {
		return errors.New("peer ID is required")
	}
	m.emitPeerUpdateEvent(&PeerCapability{
		PeerID:          peerID,
		ConnectionState: ConnectionStateConnecting,
		LastSeen:        time.Now().UnixNano(),
	})

	// Perform connection asynchronously to avoid blocking the caller (critical for JS/WASM)
	go func() {
		// Use a detached context with timeout to ensure the connection attempt persists
		// even if the original request context is cancelled quickly.
		connCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := m.transport.Connect(connCtx, peerID); err != nil {
			m.logger.Error("async connection failed", "peer", peerID, "error", err)
			m.emitPeerUpdateEvent(&PeerCapability{
				PeerID:          peerID,
				ConnectionState: ConnectionStateFailed,
				LastSeen:        time.Now().UnixNano(),
			})
		}
	}()

	return nil
}

// DisconnectFromPeer tears down a transport connection and removes routing entries.
func (m *MeshCoordinator) DisconnectFromPeer(peerID string) error {
	if peerID == "" {
		return errors.New("peer ID is required")
	}
	if err := m.transport.Disconnect(peerID); err != nil {
		return err
	}
	return nil
}
