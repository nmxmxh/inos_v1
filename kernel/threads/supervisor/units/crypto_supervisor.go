//go:build wasm

package units

import (
	"context"

	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
)

// CryptoSupervisor supervises the crypto module
type CryptoSupervisor struct {
	*supervisor.UnifiedSupervisor
	bridge *supervisor.SABBridge
}

func NewCryptoSupervisor(bridge *supervisor.SABBridge, patterns *pattern.TieredPatternStorage, knowledge *intelligence.KnowledgeGraph, capabilities []string) *CryptoSupervisor {
	if len(capabilities) == 0 {
		capabilities = []string{"crypto.hash", "crypto.sign", "crypto.verify", "crypto.encrypt", "crypto.decrypt", "crypto.keygen"}
	}
	return &CryptoSupervisor{
		UnifiedSupervisor: supervisor.NewUnifiedSupervisor("crypto", capabilities, patterns, knowledge),
		bridge:            bridge,
	}
}

func (s *CryptoSupervisor) Start(ctx context.Context) error {
	return s.UnifiedSupervisor.Start(ctx)
}
