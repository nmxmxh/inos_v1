package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/gen/base/v1"
	p2p "github.com/nmxmxh/inos_v1/kernel/gen/p2p/v1"
	system "github.com/nmxmxh/inos_v1/kernel/gen/system/v1"
	capnp "zombiezen.com/go/capnproto2"
)

// PeerCapability represents a node's ability and status in the mesh.
type PeerCapability struct {
	PeerID          string                     `json:"peer_id"`
	AvailableChunks []string                   `json:"available_chunks"` // BLAKE3 Hashes
	BandwidthKbps   float32                    `json:"bandwidth_kbps"`
	LatencyMs       float32                    `json:"latency_ms"`
	Reputation      float32                    `json:"reputation"`
	Capabilities    []string                   `json:"capabilities"` // "gpu", "storage", etc.
	LastSeen        int64                      `json:"last_seen"`    // Unix Nanoseconds
	ConnectionState ConnectionState            `json:"connection_state"`
	Region          string                     `json:"region,omitempty"`
	Coordinates     *GeoCoordinates            `json:"coordinates,omitempty"`
	Role            system.Runtime_RuntimeRole `json:"role"`
	RuntimeCaps     *RuntimeCapabilities       `json:"runtime_caps,omitempty"`
}

type RuntimeCapabilities struct {
	ComputeScore    float32 `json:"compute_score"`
	NetworkLatency  float32 `json:"network_latency"`
	AtomicsOverhead float32 `json:"atomics_overhead"`
	HasSimd         bool    `json:"has_simd"`
	HasGpu          bool    `json:"has_gpu"`
	IsHeadless      bool    `json:"is_headless"`
	BatteryLevel    float32 `json:"battery_level"`
}

type GeoCoordinates struct {
	Latitude  float32 `json:"latitude"`
	Longitude float32 `json:"longitude"`
}

// ToCapnp converts PeerCapability to its Cap'n Proto representation.
func (p *PeerCapability) ToCapnp(seg *capnp.Segment) (p2p.PeerCapability, error) {
	cap, err := p2p.NewPeerCapability(seg)
	if err != nil {
		return p2p.PeerCapability{}, err
	}

	cap.SetPeerId(p.PeerID)

	chunks, err := cap.NewAvailableChunks(int32(len(p.AvailableChunks)))
	if err != nil {
		return p2p.PeerCapability{}, err
	}
	for i, chunk := range p.AvailableChunks {
		chunks.Set(i, chunk)
	}

	cap.SetBandwidthKbps(p.BandwidthKbps)
	cap.SetLatencyMs(p.LatencyMs)
	cap.SetReputation(p.Reputation)

	capabilities, err := cap.NewCapabilities(int32(len(p.Capabilities)))
	if err != nil {
		return p2p.PeerCapability{}, err
	}
	for i, c := range p.Capabilities {
		capabilities.Set(i, c)
	}

	cap.SetLastSeen(p.LastSeen)
	cap.SetRegion(p.Region)

	if p.Coordinates != nil {
		coords, err := cap.NewCoordinates()
		if err != nil {
			return p2p.PeerCapability{}, err
		}
		coords.SetLatitude(p.Coordinates.Latitude)
		coords.SetLongitude(p.Coordinates.Longitude)
	}

	cap.SetRole(p.Role)

	if p.RuntimeCaps != nil {
		rc, err := cap.NewRuntimeCaps()
		if err != nil {
			return p2p.PeerCapability{}, err
		}
		rc.SetComputeScore(p.RuntimeCaps.ComputeScore)
		rc.SetNetworkLatency(p.RuntimeCaps.NetworkLatency)
		rc.SetAtomicsOverhead(p.RuntimeCaps.AtomicsOverhead)
		rc.SetHasSimd(p.RuntimeCaps.HasSimd)
		rc.SetHasGpu(p.RuntimeCaps.HasGpu)
		rc.SetIsHeadless(p.RuntimeCaps.IsHeadless)
		rc.SetBatteryLevel(p.RuntimeCaps.BatteryLevel)
	}

	return cap, nil
}

// FromCapnp updates PeerCapability from its Cap'n Proto representation.
func (p *PeerCapability) FromCapnp(cap p2p.PeerCapability) error {
	peerID, err := cap.PeerId()
	if err != nil {
		return err
	}
	p.PeerID = peerID

	chunks, err := cap.AvailableChunks()
	if err == nil {
		p.AvailableChunks = make([]string, chunks.Len())
		for i := 0; i < chunks.Len(); i++ {
			p.AvailableChunks[i], _ = chunks.At(i)
		}
	}

	p.BandwidthKbps = cap.BandwidthKbps()
	p.LatencyMs = cap.LatencyMs()
	p.Reputation = cap.Reputation()

	capabilities, err := cap.Capabilities()
	if err == nil {
		p.Capabilities = make([]string, capabilities.Len())
		for i := 0; i < capabilities.Len(); i++ {
			p.Capabilities[i], _ = capabilities.At(i)
		}
	}

	p.LastSeen = cap.LastSeen()

	region, _ := cap.Region()
	p.Region = region

	p.Role = cap.Role()

	if cap.HasRuntimeCaps() {
		rc, _ := cap.RuntimeCaps()
		p.RuntimeCaps = &RuntimeCapabilities{
			ComputeScore:    rc.ComputeScore(),
			NetworkLatency:  rc.NetworkLatency(),
			AtomicsOverhead: rc.AtomicsOverhead(),
			HasSimd:         rc.HasSimd(),
			HasGpu:          rc.HasGpu(),
			IsHeadless:      rc.IsHeadless(),
			BatteryLevel:    rc.BatteryLevel(),
		}
	}

	return nil
}

// Envelope wraps payloads with metadata and DNA.
type Envelope struct {
	ID        string           `json:"id"`
	Type      string           `json:"type"`
	Timestamp int64            `json:"timestamp"`
	Version   string           `json:"version"`
	Metadata  EnvelopeMetadata `json:"metadata"`
	Payload   []byte           `json:"payload"`
}

type EnvelopeMetadata struct {
	UserID         string `json:"user_id,omitempty"`
	DeviceID       string `json:"device_id,omitempty"`
	TraceParent    string `json:"trace_parent,omitempty"`
	TraceState     string `json:"trace_state,omitempty"`
	AuthToken      string `json:"auth_token,omitempty"`
	CreditLedgerID string `json:"credit_ledger_id,omitempty"`
}

// ToCapnp converts Envelope to base.Base_Envelope.
func (e *Envelope) ToCapnp(seg *capnp.Segment) (base.Base_Envelope, error) {
	env, err := base.NewBase_Envelope(seg)
	if err != nil {
		return base.Base_Envelope{}, err
	}

	env.SetId(e.ID)
	env.SetType(e.Type)
	env.SetTimestamp(e.Timestamp)
	env.SetVersion(e.Version)

	meta, err := env.NewMetadata()
	if err == nil {
		meta.SetUserId(e.Metadata.UserID)
		meta.SetDeviceId(e.Metadata.DeviceID)
		meta.SetTraceParent(e.Metadata.TraceParent)
		meta.SetTraceState(e.Metadata.TraceState)
		meta.SetAuthToken(e.Metadata.AuthToken)
		meta.SetCreditLedgerId(e.Metadata.CreditLedgerID)
	}

	payload, err := env.NewPayload()
	if err == nil {
		payload.SetData(e.Payload)
		payload.SetTypeId(e.Type)
	}

	return env, nil
}

// Marshal serializes the Envelope to Cap'n Proto binary format.
func (e *Envelope) Marshal() ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	env, err := e.ToCapnp(seg)
	if err != nil {
		return nil, err
	}

	if err := msg.SetRootPtr(env.Struct.ToPtr()); err != nil {
		return nil, err
	}

	return msg.Marshal()
}

// Unmarshal populates the Envelope from Cap'n Proto binary data.
func (e *Envelope) Unmarshal(data []byte) error {
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		return err
	}

	env, err := base.ReadRootBase_Envelope(msg)
	if err != nil {
		return err
	}

	return e.FromCapnp(env)
}

func (e *Envelope) FromCapnp(env base.Base_Envelope) error {
	id, _ := env.Id()
	e.ID = id

	t, _ := env.Type()
	e.Type = t

	e.Timestamp = env.Timestamp()

	v, _ := env.Version()
	e.Version = v

	if env.HasMetadata() {
		meta, _ := env.Metadata()
		u, _ := meta.UserId()
		e.Metadata.UserID = u
		d, _ := meta.DeviceId()
		e.Metadata.DeviceID = d
		tp, _ := meta.TraceParent()
		e.Metadata.TraceParent = tp
		ts, _ := meta.TraceState()
		e.Metadata.TraceState = ts
		at, _ := meta.AuthToken()
		e.Metadata.AuthToken = at
		cl, _ := meta.CreditLedgerId()
		e.Metadata.CreditLedgerID = cl
	}

	if env.HasPayload() {
		payload, _ := env.Payload()
		data, _ := payload.Data()
		e.Payload = data
	}

	return nil
}

// StorageProvider defines the interface for local chunk storage
type StorageProvider interface {
	StoreChunk(ctx context.Context, hash string, data []byte) error
	FetchChunk(ctx context.Context, hash string) ([]byte, error)
	HasChunk(ctx context.Context, hash string) (bool, error)
}

// Transport defines the interface for peer-to-peer communication
type Transport interface {
	Start(ctx context.Context) error
	Stop() error
	Connect(ctx context.Context, peerID string) error
	Disconnect(peerID string) error
	IsConnected(peerID string) bool
	GetConnectedPeers() []string
	Advertise(ctx context.Context, key string, value string) error
	FindPeers(ctx context.Context, key string) ([]PeerInfo, error)
	SendRPC(ctx context.Context, peerID string, method string, args interface{}, reply interface{}) error
	StreamRPC(ctx context.Context, peerID string, method string, args interface{}, writer io.Writer) (int64, error)
	SendMessage(ctx context.Context, peerID string, msg interface{}) error
	Broadcast(topic string, message interface{}) error
	RegisterRPCHandler(method string, handler func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error))
	FindNode(ctx context.Context, peerID, targetID string) ([]PeerInfo, error)
	FindValue(ctx context.Context, peerID, chunkHash string) ([]string, []PeerInfo, error)
	Store(ctx context.Context, peerID string, key string, value []byte) error
	Ping(ctx context.Context, peerID string) error
	GetPeerCapabilities(peerID string) (*PeerCapability, error)
	UpdateLocalCapabilities(capabilities *PeerCapability) error
	GetConnectionMetrics() ConnectionMetrics
	GetHealth() TransportHealth
	GetStats() map[string]interface{}
}

// ConnectionMetrics tracks transport-level statistics
type ConnectionMetrics struct {
	ActiveConnections  uint32  `json:"active_connections"`
	TotalConnections   uint64  `json:"total_connections"`
	BytesSent          uint64  `json:"bytes_sent"`
	BytesReceived      uint64  `json:"bytes_received"`
	MessagesSent       uint64  `json:"messages_sent"`
	MessagesReceived   uint64  `json:"messages_received"`
	LatencyP50         float32 `json:"latency_p50_ms"`
	LatencyP95         float32 `json:"latency_p95_ms"`
	ErrorRate          float32 `json:"error_rate"`
	SuccessRate        float32 `json:"success_rate"`
	FailedMessages     uint64  `json:"failed_messages"`
	WebRTCCandidates   uint32  `json:"webrtc_candidates"`
	WebSocketFallbacks uint32  `json:"websocket_fallbacks"`
}

// TransportHealth represents transport system health
type TransportHealth struct {
	Status          string  `json:"status"`
	Score           float32 `json:"score"`
	WebRTCSupported bool    `json:"webrtc_supported"`
	IceServers      int     `json:"ice_servers"`
	SignalingActive bool    `json:"signaling_active"`
	LastError       string  `json:"last_error,omitempty"`
	Uptime          string  `json:"uptime"`
}

func (p *PeerCapability) Validate() error {
	if p.PeerID == "" {
		return errors.New("peer ID is required")
	}
	if p.BandwidthKbps < 0 {
		return errors.New("bandwidth cannot be negative")
	}
	if p.LatencyMs < 0 {
		return errors.New("latency cannot be negative")
	}
	if p.Reputation < 0 || p.Reputation > 1 {
		return errors.New("reputation must be between 0 and 1")
	}
	return nil
}

func (p *PeerCapability) IsOnline() bool {
	now := time.Now().UnixNano()
	lastSeen := time.Duration(now - p.LastSeen)
	return lastSeen < 5*time.Minute &&
		p.ConnectionState == ConnectionStateConnected
}

func (p *PeerCapability) HasCapability(cap string) bool {
	for _, c := range p.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

type ConnectionState int

const (
	ConnectionStateDisconnected ConnectionState = 0
	ConnectionStateConnecting   ConnectionState = 1
	ConnectionStateConnected    ConnectionState = 2
	ConnectionStateDegraded     ConnectionState = 3
	ConnectionStateFailed       ConnectionState = 4
)

func (c ConnectionState) String() string {
	switch c {
	case ConnectionStateDisconnected:
		return "disconnected"
	case ConnectionStateConnecting:
		return "connecting"
	case ConnectionStateConnected:
		return "connected"
	case ConnectionStateDegraded:
		return "degraded"
	case ConnectionStateFailed:
		return "failed"
	default:
		return fmt.Sprintf("unknown(%d)", c)
	}
}

// ModelMetadata matches registry.rs and mesh.capnp
type ModelMetadata struct {
	SchemaVersion int      `json:"schema_version,omitempty"` // Default to 1
	ModelID       string   `json:"model_id"`
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	TotalChunks   uint32   `json:"total_chunks"`
	TotalSize     uint64   `json:"total_size"`
	ChunkIDs      []string `json:"chunk_ids"`
	CreatedAt     uint64   `json:"created_at"`
	LastAccessed  uint64   `json:"last_accessed"`

	// Rich Metadata
	ModelType ModelType   `json:"model_type"`
	Format    ModelFormat `json:"format"`
	Tags      []string    `json:"tags"`

	// Advanced Specs
	Architecture   string            `json:"architecture"`
	ParameterCount uint64            `json:"parameter_count"`
	Quantization   QuantizationLevel `json:"quantization"`
	License        string            `json:"license"`
	Author         string            `json:"author"`
	Description    string            `json:"description"`

	// Resource Requirements
	EstimatedInferenceTimeMs float32 `json:"estimated_inference_time_ms"`
	MemoryRequiredMb         uint32  `json:"memory_required_mb"`
	GPURequired              bool    `json:"gpu_required"`

	LayerChunks []LayerChunkMapping `json:"layer_chunks"`
}

type LayerChunkMapping struct {
	LayerIndex   uint32   `json:"layer_index"`
	ChunkIndices []uint32 `json:"chunk_indices"`
}

func (m *ModelMetadata) Validate() error {
	if m.ModelID == "" {
		return errors.New("model ID is required")
	}
	if m.TotalChunks == 0 {
		return errors.New("total chunks must be positive")
	}
	if m.TotalSize == 0 {
		return errors.New("total size must be positive")
	}

	for _, layer := range m.LayerChunks {
		for _, chunkIdx := range layer.ChunkIndices {
			if chunkIdx >= m.TotalChunks {
				return errors.New("chunk index out of bounds")
			}
		}
	}
	return nil
}

type ModelType int

const (
	ModelTypeLLM        ModelType = 0
	ModelTypeVision     ModelType = 1
	ModelTypeAudio      ModelType = 2
	ModelTypeMultimodal ModelType = 3
	ModelTypeEmbedding  ModelType = 4
	ModelTypeDiffusion  ModelType = 5
)

type ModelFormat int

const (
	ModelFormatSafetensors ModelFormat = 0
	ModelFormatGGUF        ModelFormat = 1
	ModelFormatONNX        ModelFormat = 2
	ModelFormatPyTorch     ModelFormat = 3
	ModelFormatTensorFlow  ModelFormat = 4
)

type QuantizationLevel int

const (
	QuantizationLevelFP32 QuantizationLevel = 0
	QuantizationLevelFP16 QuantizationLevel = 1
	QuantizationLevelINT8 QuantizationLevel = 2
	QuantizationLevelINT4 QuantizationLevel = 3
)

// DHTEntry represents a value stored in the distributed hash table.
type DHTEntry struct {
	Key       string   `json:"key"`       // Chunk Hash
	Value     []string `json:"value"`     // List of Peer IDs
	Timestamp int64    `json:"timestamp"` // Unix Nano
	TTL       int64    `json:"ttl"`       // Seconds
}

// ToCapnp converts DHTEntry to p2p.DHTEntry.
func (d *DHTEntry) ToCapnp(seg *capnp.Segment) (p2p.DHTEntry, error) {
	entry, err := p2p.NewDHTEntry(seg)
	if err != nil {
		return p2p.DHTEntry{}, err
	}

	entry.SetKey(d.Key)

	val, err := entry.NewValue(int32(len(d.Value)))
	if err == nil {
		for i, v := range d.Value {
			val.Set(i, v)
		}
	}

	entry.SetTimestamp(d.Timestamp)
	entry.SetTtl(d.TTL)

	return entry, nil
}

// FromCapnp updates DHTEntry from p2p.DHTEntry.
func (d *DHTEntry) FromCapnp(entry p2p.DHTEntry) error {
	k, _ := entry.Key()
	d.Key = k

	val, err := entry.Value()
	if err == nil {
		d.Value = make([]string, val.Len())
		for i := 0; i < val.Len(); i++ {
			d.Value[i], _ = val.At(i)
		}
	}

	d.Timestamp = entry.Timestamp()
	d.TTL = entry.Ttl()

	return nil
}

// PeerInfo is the internal routing table representation of a peer.
type PeerInfo struct {
	ID           string          `json:"id"`
	Address      string          `json:"address"` // WebRTC/WebSocket Addr
	LastContact  time.Time       `json:"last_contact"`
	BucketIndex  int             `json:"bucket_index"`
	Capabilities *PeerCapability `json:"capabilities,omitempty"`
}

// ToCapnp converts PeerInfo to p2p.PeerInfo (internal schema if available, otherwise just use as is).
// Note: p2p.Gossip_PeerInfo is available in gossip.capnp.
func (p *PeerInfo) ToCapnp(seg *capnp.Segment) (p2p.Gossip_PeerInfo, error) {
	info, err := p2p.NewGossip_PeerInfo(seg)
	if err != nil {
		return p2p.Gossip_PeerInfo{}, err
	}

	info.SetId(p.ID)
	info.SetAddress(p.Address)
	info.SetLastSeen(p.LastContact.UnixNano())

	if p.Capabilities != nil {
		caps, _ := info.NewCapabilities(int32(len(p.Capabilities.Capabilities)))
		for i, c := range p.Capabilities.Capabilities {
			caps.Set(i, c)
		}
		info.SetReputation(p.Capabilities.Reputation)
	}

	return info, nil
}

// FromCapnp updates PeerInfo from p2p.Gossip_PeerInfo.
func (p *PeerInfo) FromCapnp(info p2p.Gossip_PeerInfo) error {
	id, _ := info.Id()
	p.ID = id
	addr, _ := info.Address()
	p.Address = addr
	p.LastContact = time.Unix(0, info.LastSeen())

	caps, err := info.Capabilities()
	if err == nil {
		p.Capabilities = &PeerCapability{
			PeerID:     id,
			Reputation: info.Reputation(),
		}
		p.Capabilities.Capabilities = make([]string, caps.Len())
		for i := 0; i < caps.Len(); i++ {
			p.Capabilities.Capabilities[i], _ = caps.At(i)
		}
	}

	return nil
}

// MeshMetrics for observability.
type MeshMetrics struct {
	TotalPeers       uint32  `json:"total_peers"`
	ConnectedPeers   uint32  `json:"connected_peers"`
	DHTEntries       uint32  `json:"dht_entries"`
	GossipRatePerSec float32 `json:"gossip_rate_per_sec"`
	AvgReputation    float32 `json:"avg_reputation"`
	BytesSent        uint64  `json:"bytes_sent"`
	BytesReceived    uint64  `json:"bytes_received"`
	RegionID         uint32  `json:"region_id"`
	// Latency
	P50LatencyMs float32 `json:"p50_latency_ms"`
	P95LatencyMs float32 `json:"p95_latency_ms"`

	// Success Rates
	ConnectionSuccessRate float32 `json:"connection_success_rate"`
	ChunkFetchSuccessRate float32 `json:"chunk_fetch_success_rate"`

	// Storage
	LocalChunks          uint32 `json:"local_chunks"`
	TotalChunksAvailable uint32 `json:"total_chunks_available"`

	// Global Analytics Aggregations
	TotalStorageBytes  uint64  `json:"total_storage_bytes"`
	TotalComputeGFLOPS float32 `json:"total_compute_gflops"`
	GlobalOpsPerSec    float32 `json:"global_ops_per_sec"`
	ActiveNodeCount    uint32  `json:"active_node_count"`
}

// GossipMessage represents a message propagated through the gossip protocol
type GossipMessage struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Sender    string      `json:"sender"`
	Timestamp int64       `json:"timestamp"`
	TTL       int         `json:"ttl"`
	HopCount  int         `json:"hop_count"`
	MaxHops   int         `json:"max_hops"`
	Payload   interface{} `json:"payload"`
	PublicKey []byte      `json:"public_key,omitempty"`
	Signature []byte      `json:"signature,omitempty"`
}

// ToCapnp converts GossipMessage to its Cap'n Proto Envelope representation.
func (g *GossipMessage) ToCapnp(seg *capnp.Segment) (base.Base_Envelope, error) {
	env, err := base.NewBase_Envelope(seg)
	if err != nil {
		return base.Base_Envelope{}, err
	}

	env.SetId(g.ID)
	env.SetType(g.Type)
	env.SetTimestamp(g.Timestamp)

	// Create GossipPayload
	payloadMsg, payloadSeg, _ := capnp.NewMessage(capnp.SingleSegment(nil))
	gop, err := p2p.NewGossip_GossipPayload(payloadSeg)
	if err == nil {
		// Set metadata
		meta, _ := gop.NewMetadata()
		meta.SetDeviceId(g.Sender)

		// Set union based on payload type
		switch p := g.Payload.(type) {
		case string:
			_ = gop.SetCustom([]byte(p))
		case []byte:
			_ = gop.SetCustom(p)
		}

		// Marshal GossipPayload and set in Envelope
		data, _ := payloadMsg.Marshal()
		ep, _ := env.NewPayload()
		ep.SetData(data)
		ep.SetTypeId("gossip.payload")
	}

	return env, nil
}

// Marshal serializes the GossipMessage to Cap'n Proto binary format.
func (g *GossipMessage) Marshal() ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	env, err := g.ToCapnp(seg)
	if err != nil {
		return nil, err
	}

	if err := msg.SetRootPtr(env.Struct.ToPtr()); err != nil {
		return nil, err
	}

	return msg.Marshal()
}

// Unmarshal populates the GossipMessage from Cap'n Proto binary data.
func (g *GossipMessage) Unmarshal(data []byte) error {
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		return err
	}

	env, err := base.ReadRootBase_Envelope(msg)
	if err != nil {
		return err
	}

	return g.FromCapnp(env)
}

func (g *GossipMessage) FromCapnp(env base.Base_Envelope) error {
	id, _ := env.Id()
	g.ID = id

	t, _ := env.Type()
	g.Type = t

	g.Timestamp = env.Timestamp()

	if env.HasPayload() {
		payload, _ := env.Payload()
		data, _ := payload.Data()
		g.Payload = data // Store raw for now, GossipManager will decode
	}

	return nil
}

// ContentMerkleTree represents a merkle tree for content verification and delta replication
type ContentMerkleTree struct {
	Root   string              `json:"root"`
	Leaves []ContentMerkleLeaf `json:"leaves"`
	Depth  int                 `json:"depth"`
}

// ContentMerkleLeaf represents a leaf node in the content merkle tree
type ContentMerkleLeaf struct {
	Index int    `json:"index"`
	Hash  string `json:"hash"`
	Data  []byte `json:"data"`
}

// Resource represents a compute or storage resource
type Resource struct {
	Size         uint64  `json:"size"`          // Size in bytes
	Type         string  `json:"type"`          // "chunk", "model", "compute"
	DemandScore  float64 `json:"demand_score"`  // 0.0 to 1.0, higher = more demand
	CreditBudget float64 `json:"credit_budget"` // Available credits for replication
}
