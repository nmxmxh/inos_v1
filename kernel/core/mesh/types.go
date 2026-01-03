package mesh

import (
	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
)

// Types defined here are mesh-specific or re-exports if needed

// Re-export common types for convenience within the mesh package
type PeerCapability = common.PeerCapability
type GeoCoordinates = common.GeoCoordinates
type Envelope = common.Envelope
type EnvelopeMetadata = common.EnvelopeMetadata
type StorageProvider = common.StorageProvider
type Transport = common.Transport
type ConnectionMetrics = common.ConnectionMetrics
type TransportHealth = common.TransportHealth
type ConnectionState = common.ConnectionState
type ModelMetadata = common.ModelMetadata
type LayerChunkMapping = common.LayerChunkMapping
type ModelType = common.ModelType
type ModelFormat = common.ModelFormat
type QuantizationLevel = common.QuantizationLevel
type DHTEntry = common.DHTEntry
type PeerInfo = common.PeerInfo
type MeshMetrics = common.MeshMetrics
type GossipMessage = common.GossipMessage
type ContentMerkleTree = common.ContentMerkleTree
type ContentMerkleLeaf = common.ContentMerkleLeaf

const (
	EventTypeDHTLookup   = "dht_lookup"
	EventTypeChunkFetch  = "chunk_fetch"
	EventTypePeerConnect = "peer_connect"
	EventTypeGossipProp  = "gossip_propagation"
	EventTypeReputation  = "reputation_update"
)
