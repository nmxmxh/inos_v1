package core

// Processor is the core universal component.
type Processor struct {
	ID      string      // Cryptographic identity
	Runtime interface{} // WASM executor (placeholder)
	Network interface{} // P2P mesh (placeholder)
	Credits int64       // Simple economy
}
