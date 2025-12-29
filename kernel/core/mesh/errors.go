package mesh

import (
	"fmt"
)

// Error codes for mesh operations
const (
	// Discovery errors
	ErrCodeChunkNotFound   = "CHUNK_NOT_FOUND"
	ErrCodePeerNotFound    = "PEER_NOT_FOUND"
	ErrCodeDHTLookupFailed = "DHT_LOOKUP_FAILED"

	// Connection errors
	ErrCodePeerUnreachable  = "PEER_UNREACHABLE"
	ErrCodeConnectionFailed = "CONNECTION_FAILED"
	ErrCodeTimeout          = "TIMEOUT"

	// Circuit breaker errors
	ErrCodeCircuitOpen     = "CIRCUIT_OPEN"
	ErrCodeCircuitHalfOpen = "CIRCUIT_HALF_OPEN"

	// Validation errors
	ErrCodeInvalidChunkHash = "INVALID_CHUNK_HASH"
	ErrCodeInvalidPeerID    = "INVALID_PEER_ID"
	ErrCodeInvalidProof     = "INVALID_PROOF"

	// Resource errors
	ErrCodeInsufficientPeers = "INSUFFICIENT_PEERS"
	ErrCodeCapacityExceeded  = "CAPACITY_EXCEEDED"
	ErrCodeQuotaExceeded     = "QUOTA_EXCEEDED"

	// Gossip errors
	ErrCodeGossipFailed     = "GOSSIP_FAILED"
	ErrCodeSignatureInvalid = "SIGNATURE_INVALID"
	ErrCodeMessageExpired   = "MESSAGE_EXPIRED"

	// Reputation errors
	ErrCodeLowReputation = "LOW_REPUTATION"
	ErrCodePeerBanned    = "PEER_BANNED"
)

// MeshError is a production-grade error type with context
type MeshError struct {
	Code    string                 // Error code for programmatic handling
	Message string                 // Human-readable message
	Context map[string]interface{} // Additional context
	Cause   error                  // Underlying error
}

// Error implements the error interface
func (e *MeshError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *MeshError) Unwrap() error {
	return e.Cause
}

// WithContext adds context to the error
func (e *MeshError) WithContext(key string, value interface{}) *MeshError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewMeshError creates a new mesh error
func NewMeshError(code, message string) *MeshError {
	return &MeshError{
		Code:    code,
		Message: message,
		Context: make(map[string]interface{}),
	}
}

// WrapError wraps an existing error with mesh error context
func WrapError(code, message string, cause error) *MeshError {
	return &MeshError{
		Code:    code,
		Message: message,
		Cause:   cause,
		Context: make(map[string]interface{}),
	}
}

// Common error constructors

func ErrChunkNotFound(chunkHash string) *MeshError {
	return NewMeshError(ErrCodeChunkNotFound, "chunk not found").
		WithContext("chunk_hash", chunkHash)
}

func ErrPeerUnreachable(peerID string, cause error) *MeshError {
	return WrapError(ErrCodePeerUnreachable, "peer unreachable", cause).
		WithContext("peer_id", peerID)
}

func ErrCircuitOpen(peerID string) *MeshError {
	return NewMeshError(ErrCodeCircuitOpen, "circuit breaker open").
		WithContext("peer_id", peerID)
}

func ErrTimeout(operation string, duration string) *MeshError {
	return NewMeshError(ErrCodeTimeout, "operation timed out").
		WithContext("operation", operation).
		WithContext("duration", duration)
}

func ErrInsufficientPeers(required, available int) *MeshError {
	return NewMeshError(ErrCodeInsufficientPeers, "insufficient peers").
		WithContext("required", required).
		WithContext("available", available)
}

func ErrSignatureInvalid(messageID string) *MeshError {
	return NewMeshError(ErrCodeSignatureInvalid, "message signature invalid").
		WithContext("message_id", messageID)
}

func ErrLowReputation(peerID string, score float64, threshold float64) *MeshError {
	return NewMeshError(ErrCodeLowReputation, "peer reputation too low").
		WithContext("peer_id", peerID).
		WithContext("score", score).
		WithContext("threshold", threshold)
}
