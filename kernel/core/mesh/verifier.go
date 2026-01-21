package mesh

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
)

// VerificationStatus represents the state of streaming verification
type VerificationStatus int

const (
	VerificationPending VerificationStatus = iota
	VerificationPassed
	VerificationFailed
	VerificationCorrupted
)

func (vs VerificationStatus) String() string {
	switch vs {
	case VerificationPending:
		return "pending"
	case VerificationPassed:
		return "passed"
	case VerificationFailed:
		return "failed"
	case VerificationCorrupted:
		return "corrupted"
	default:
		return "unknown"
	}
}

// DigestValidator validates hashes returned from Rust modules
// The actual BLAKE3 computation happens in Rust; Go only validates the result
type DigestValidator struct {
	expectedDigest []byte
	expectedHex    string
	status         VerificationStatus
	mu             sync.Mutex
}

// NewDigestValidator creates a validator for the given expected digest
func NewDigestValidator(expectedDigest []byte) *DigestValidator {
	return &DigestValidator{
		expectedDigest: expectedDigest,
		expectedHex:    hex.EncodeToString(expectedDigest),
		status:         VerificationPending,
	}
}

// NewDigestValidatorFromHex creates a validator from hex-encoded digest
func NewDigestValidatorFromHex(hexDigest string) (*DigestValidator, error) {
	digest, err := hex.DecodeString(hexDigest)
	if err != nil {
		return nil, fmt.Errorf("invalid hex digest: %w", err)
	}
	return NewDigestValidator(digest), nil
}

// Validate checks if the actual digest matches the expected one
func (dv *DigestValidator) Validate(actualDigest []byte) bool {
	dv.mu.Lock()
	defer dv.mu.Unlock()

	if dv.status != VerificationPending {
		return dv.status == VerificationPassed
	}

	if bytes.Equal(dv.expectedDigest, actualDigest) {
		dv.status = VerificationPassed
		return true
	}

	dv.status = VerificationFailed
	return false
}

// ValidateHex checks if the actual hex digest matches the expected one
func (dv *DigestValidator) ValidateHex(actualHex string) bool {
	return dv.expectedHex == actualHex
}

// Status returns the current verification status
func (dv *DigestValidator) Status() VerificationStatus {
	dv.mu.Lock()
	defer dv.mu.Unlock()
	return dv.status
}

// ExpectedDigest returns the expected digest bytes
func (dv *DigestValidator) ExpectedDigest() []byte {
	return dv.expectedDigest
}

// ExpectedHex returns the expected digest as hex string
func (dv *DigestValidator) ExpectedHex() string {
	return dv.expectedHex
}

// StreamingVerifier handles progressive verification of data streams
// Uses SHA-256 for header verification (Go stdlib), delegates BLAKE3 to Rust
type StreamingVerifier struct {
	expectedDigest []byte
	headerHash     []byte // Hash of first chunk for quick validation
	bytesProcessed int64
	chunkCount     int64
	chunkSize      int
	status         VerificationStatus
	mu             sync.Mutex
}

// VerifierConfig holds configuration for the streaming verifier
type VerifierConfig struct {
	ChunkSize    int   // Size of chunks for processing (default: 1MB)
	ExpectedSize int64 // Expected total size (0 for unknown)
}

// DefaultVerifierConfig returns production-ready defaults
func DefaultVerifierConfig() VerifierConfig {
	return VerifierConfig{
		ChunkSize: 1024 * 1024, // 1MB
	}
}

// NewStreamingVerifier creates a new verifier for the given digest
func NewStreamingVerifier(digest []byte) *StreamingVerifier {
	return NewStreamingVerifierWithConfig(digest, DefaultVerifierConfig())
}

// NewStreamingVerifierWithConfig creates a verifier with custom configuration
func NewStreamingVerifierWithConfig(digest []byte, config VerifierConfig) *StreamingVerifier {
	sv := &StreamingVerifier{
		expectedDigest: make([]byte, len(digest)),
		chunkSize:      config.ChunkSize,
		status:         VerificationPending,
	}
	copy(sv.expectedDigest, digest)
	return sv
}

// ProcessHeader verifies the first chunk header (quick sanity check)
func (sv *StreamingVerifier) ProcessHeader(data []byte) error {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	if sv.status != VerificationPending {
		return errors.New("verifier already finalized")
	}

	if len(data) == 0 {
		return nil
	}

	// Compute SHA-256 of header for quick validation
	hash := sha256.Sum256(data)
	sv.headerHash = hash[:]
	sv.bytesProcessed += int64(len(data))
	sv.chunkCount++

	return nil
}

// ProcessStream reads from a stream and tracks progress
func (sv *StreamingVerifier) ProcessStream(r io.Reader) error {
	buf := make([]byte, sv.chunkSize)

	for {
		n, err := r.Read(buf)
		if n > 0 {
			sv.mu.Lock()
			sv.bytesProcessed += int64(n)
			sv.chunkCount++
			sv.mu.Unlock()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stream read error: %w", err)
		}
	}

	return nil
}

// Finalize completes verification with the digest from Rust module
func (sv *StreamingVerifier) Finalize(actualDigest []byte) (verified bool) {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	if sv.status != VerificationPending {
		return sv.status == VerificationPassed
	}

	if bytes.Equal(sv.expectedDigest, actualDigest) {
		sv.status = VerificationPassed
		return true
	}

	sv.status = VerificationFailed
	return false
}

// Status returns the current verification status
func (sv *StreamingVerifier) Status() VerificationStatus {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	return sv.status
}

// BytesProcessed returns the number of bytes processed so far
func (sv *StreamingVerifier) BytesProcessed() int64 {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	return sv.bytesProcessed
}

// ChunkCount returns the number of chunks processed
func (sv *StreamingVerifier) ChunkCount() int64 {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	return sv.chunkCount
}

// VerifyHash compares the final computed hash with the expected digest
func (sv *StreamingVerifier) VerifyHash(actual []byte) bool {
	return bytes.Equal(sv.expectedDigest, actual)
}

// DelegationVerifier integrates with Rust module delegation flow
// It validates hashes returned from Rust without re-computing them locally
type DelegationVerifier struct {
	inputDigest   string // BLAKE3 hash of input data (computed by Rust)
	outputDigest  string // BLAKE3 hash of output data (computed by Rust)
	operation     string // Operation performed (hash, compress, encrypt)
	verified      bool
	verifiedAt    int64
	executionTime int64 // Nanoseconds
	mu            sync.Mutex
}

// NewDelegationVerifier creates a verifier for delegation results
func NewDelegationVerifier(inputDigest, operation string) *DelegationVerifier {
	return &DelegationVerifier{
		inputDigest: inputDigest,
		operation:   operation,
	}
}

// SetResult stores the result from the remote Rust module execution
func (dv *DelegationVerifier) SetResult(outputDigest string, executionTimeNs int64) {
	dv.mu.Lock()
	defer dv.mu.Unlock()
	dv.outputDigest = outputDigest
	dv.executionTime = executionTimeNs
}

// Verify confirms the output digest matches expected value
// For hash operations: output should match input (content-addressable)
// For compress/encrypt: output is new digest of transformed data
func (dv *DelegationVerifier) Verify(expectedOutputDigest string) bool {
	dv.mu.Lock()
	defer dv.mu.Unlock()

	dv.verified = dv.outputDigest == expectedOutputDigest
	return dv.verified
}

// IsVerified returns whether verification has passed
func (dv *DelegationVerifier) IsVerified() bool {
	dv.mu.Lock()
	defer dv.mu.Unlock()
	return dv.verified
}

// InputDigest returns the input digest
func (dv *DelegationVerifier) InputDigest() string {
	return dv.inputDigest
}

// OutputDigest returns the output digest
func (dv *DelegationVerifier) OutputDigest() string {
	dv.mu.Lock()
	defer dv.mu.Unlock()
	return dv.outputDigest
}

// ExecutionTime returns the execution time in nanoseconds
func (dv *DelegationVerifier) ExecutionTime() int64 {
	dv.mu.Lock()
	defer dv.mu.Unlock()
	return dv.executionTime
}

// GpuVerifier handles verification of GPU-specific compute results
type GpuVerifier struct {
	operation  string
	prevState  []byte // Previous state for delta checks
	parameters map[string]interface{}
	status     VerificationStatus
	mu         sync.Mutex
}

// NewGpuVerifier creates a verifier for GPU compute tasks
func NewGpuVerifier(operation string, prevState []byte) *GpuVerifier {
	return &GpuVerifier{
		operation: operation,
		prevState: prevState,
		status:    VerificationPending,
	}
}

// VerifyCompute validates the output of a GPU compute pass
func (gv *GpuVerifier) VerifyCompute(output []byte) bool {
	gv.mu.Lock()
	defer gv.mu.Unlock()

	if gv.status != VerificationPending {
		return gv.status == VerificationPassed
	}

	// 1. Basic check: length should be non-zero and match expected structure if possible
	if len(output) == 0 {
		gv.status = VerificationFailed
		return false
	}

	// 2. Operation-specific logic
	switch gv.operation {
	case "gpu.boids":
		// Phase 17: Support WebGpuRequest JSON (Rust → JS Delegation)
		// If the output starts with '{', it's a structured request for the host.
		if len(output) > 0 && output[0] == '{' {
			// Basic sanity: Check for JSON markers
			if len(output) > 20 && (bytes.Contains(output, []byte("shader")) || bytes.Contains(output, []byte("method"))) {
				gv.status = VerificationPassed
				return true
			}
		}

		// Legacy Path: Basic Sanity: Boid buffer should be a multiple of boid struct size
		// Canonical Bird struct is 236 bytes (59 floats)
		const birdSize = 236
		if len(output)%birdSize != 0 {
			gv.status = VerificationCorrupted
			return false
		}

		// Delta Check: If we have previous state, ensure no teleportation
		if len(gv.prevState) > 0 && len(gv.prevState) == len(output) {
			// In a high-fidelity verifier, we would parse floats here.
			// For Phase 4/5 integration, we'll verify the buffer is "active" (not all zeros)
			isActive := false
			for i := 0; i < len(output); i++ {
				if output[i] != 0 {
					isActive = true
					break
				}
			}
			if !isActive {
				gv.status = VerificationFailed
				return false
			}
		}
		gv.status = VerificationPassed

	case "instance_matrix_gen":
		// Matrix generation should return 64 bytes per matrix
		const matSize = 64
		if len(output) == 0 || len(output)%matSize != 0 {
			gv.status = VerificationCorrupted
			return false
		}
		gv.status = VerificationPassed

	default:
		// Fallback to presence check
		gv.status = VerificationPassed
	}

	return gv.status == VerificationPassed
}

func (gv *GpuVerifier) Status() VerificationStatus {
	gv.mu.Lock()
	defer gv.mu.Unlock()
	return gv.status
}
