package core

import (
	"crypto/rand"
	"encoding/hex"
)

// Identity represents a persistent cryptographic node identity.
type Identity struct {
	ID string
}

// NewIdentity generates a new persistent identity (for demo, random hex).
func NewIdentity() *Identity {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return &Identity{ID: hex.EncodeToString(b)}
}
