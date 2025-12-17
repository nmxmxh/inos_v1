package core

// Packet represents the universal protocol message.
type Packet struct {
	WASM   []byte // Code to run
	Input  []byte // Data to process
	Result []byte // Output (if returning)
	Cost   int64  // Credits to earn/spend
}
