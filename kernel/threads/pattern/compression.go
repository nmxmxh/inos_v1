package pattern

import "fmt"

// CompressionType defines compression algorithm
type CompressionType int

const (
	CompressionNone CompressionType = iota
	CompressionBrotli
	CompressionSnappy
	CompressionLZ4
)

// PatternCompressor handles pattern compression metadata
// Actual compression is handled by Rust modules
type PatternCompressor struct {
	algorithm CompressionType
}

// NewPatternCompressor creates a new pattern compressor
func NewPatternCompressor(algorithm CompressionType) *PatternCompressor {
	return &PatternCompressor{
		algorithm: algorithm,
	}
}

// Compress returns error as Kernel should not compress
func (pc *PatternCompressor) Compress(pattern *EnhancedPattern) ([]byte, error) {
	return nil, fmt.Errorf("compression handled by Rust modules")
}

// Decompress returns error as Kernel should not decompress
func (pc *PatternCompressor) Decompress(data []byte) (*EnhancedPattern, error) {
	return nil, fmt.Errorf("decompression handled by Rust modules")
}
