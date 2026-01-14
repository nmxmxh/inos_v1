package mesh

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== DigestValidator Tests ==========

func TestNewDigestValidator(t *testing.T) {
	digest := []byte{0x01, 0x02, 0x03, 0x04}
	dv := NewDigestValidator(digest)

	require.NotNil(t, dv)
	assert.Equal(t, digest, dv.ExpectedDigest())
	assert.Equal(t, "01020304", dv.ExpectedHex())
	assert.Equal(t, VerificationPending, dv.Status())
}

func TestNewDigestValidatorFromHex(t *testing.T) {
	hexDigest := "deadbeef"
	dv, err := NewDigestValidatorFromHex(hexDigest)

	require.NoError(t, err)
	require.NotNil(t, dv)
	assert.Equal(t, hexDigest, dv.ExpectedHex())
}

func TestNewDigestValidatorFromHex_Invalid(t *testing.T) {
	_, err := NewDigestValidatorFromHex("not-valid-hex")
	assert.Error(t, err)
}

func TestDigestValidator_Validate_Success(t *testing.T) {
	digest := []byte{0xde, 0xad, 0xbe, 0xef}
	dv := NewDigestValidator(digest)

	result := dv.Validate(digest)
	assert.True(t, result)
	assert.Equal(t, VerificationPassed, dv.Status())
}

func TestDigestValidator_Validate_Failure(t *testing.T) {
	expected := []byte{0xde, 0xad, 0xbe, 0xef}
	actual := []byte{0xba, 0xad, 0xca, 0xfe}
	dv := NewDigestValidator(expected)

	result := dv.Validate(actual)
	assert.False(t, result)
	assert.Equal(t, VerificationFailed, dv.Status())
}

func TestDigestValidator_ValidateHex(t *testing.T) {
	dv, _ := NewDigestValidatorFromHex("deadbeef")

	assert.True(t, dv.ValidateHex("deadbeef"))
	assert.False(t, dv.ValidateHex("baadcafe"))
}

func TestDigestValidator_DoubleValidate(t *testing.T) {
	digest := []byte{0x01, 0x02}
	dv := NewDigestValidator(digest)

	// First validation passes
	assert.True(t, dv.Validate(digest))

	// Second validation with different value should still return "passed"
	// because status is already finalized
	assert.True(t, dv.Validate([]byte{0xFF, 0xFF}))
}

func TestDigestValidator_ConcurrentAccess(t *testing.T) {
	digest := []byte{0xca, 0xfe, 0xba, 0xbe}
	dv := NewDigestValidator(digest)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dv.Validate(digest)
			_ = dv.Status()
			_ = dv.ExpectedHex()
		}()
	}
	wg.Wait()

	assert.Equal(t, VerificationPassed, dv.Status())
}

// ========== StreamingVerifier Tests ==========

func TestNewStreamingVerifier(t *testing.T) {
	digest := make([]byte, 32)
	sv := NewStreamingVerifier(digest)

	require.NotNil(t, sv)
	assert.Equal(t, VerificationPending, sv.Status())
	assert.Equal(t, int64(0), sv.BytesProcessed())
	assert.Equal(t, int64(0), sv.ChunkCount())
}

func TestStreamingVerifier_ProcessHeader(t *testing.T) {
	digest := make([]byte, 32)
	sv := NewStreamingVerifier(digest)

	err := sv.ProcessHeader([]byte("header data"))
	require.NoError(t, err)
	assert.Equal(t, int64(11), sv.BytesProcessed())
	assert.Equal(t, int64(1), sv.ChunkCount())
}

func TestStreamingVerifier_ProcessStream(t *testing.T) {
	digest := make([]byte, 32)
	sv := NewStreamingVerifierWithConfig(digest, VerifierConfig{ChunkSize: 10})

	data := strings.Repeat("x", 100)
	err := sv.ProcessStream(strings.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, int64(100), sv.BytesProcessed())
}

func TestStreamingVerifier_Finalize_Success(t *testing.T) {
	digest := []byte{0x01, 0x02, 0x03}
	sv := NewStreamingVerifier(digest)

	verified := sv.Finalize(digest)
	assert.True(t, verified)
	assert.Equal(t, VerificationPassed, sv.Status())
}

func TestStreamingVerifier_Finalize_Failure(t *testing.T) {
	expected := []byte{0x01, 0x02, 0x03}
	actual := []byte{0xFF, 0xFF, 0xFF}
	sv := NewStreamingVerifier(expected)

	verified := sv.Finalize(actual)
	assert.False(t, verified)
	assert.Equal(t, VerificationFailed, sv.Status())
}

func TestStreamingVerifier_VerifyHash(t *testing.T) {
	digest := []byte{0xab, 0xcd, 0xef}
	sv := NewStreamingVerifier(digest)

	assert.True(t, sv.VerifyHash(digest))
	assert.False(t, sv.VerifyHash([]byte{0x00, 0x00, 0x00}))
}

func TestStreamingVerifier_ProcessAfterFinalize(t *testing.T) {
	digest := make([]byte, 32)
	sv := NewStreamingVerifier(digest)
	sv.Finalize(digest)

	err := sv.ProcessHeader([]byte("more data"))
	assert.Error(t, err)
}

func TestStreamingVerifier_StreamReadError(t *testing.T) {
	digest := make([]byte, 32)
	sv := NewStreamingVerifier(digest)

	err := sv.ProcessStream(&errorReader{err: io.ErrUnexpectedEOF})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stream read error")
}

// ========== DelegationVerifier Tests ==========

func TestNewDelegationVerifier(t *testing.T) {
	dv := NewDelegationVerifier("abc123", "compress")

	require.NotNil(t, dv)
	assert.Equal(t, "abc123", dv.InputDigest())
	assert.Equal(t, "", dv.OutputDigest())
	assert.False(t, dv.IsVerified())
}

func TestDelegationVerifier_SetResult(t *testing.T) {
	dv := NewDelegationVerifier("input", "hash")

	dv.SetResult("output123", 1000000)

	assert.Equal(t, "output123", dv.OutputDigest())
	assert.Equal(t, int64(1000000), dv.ExecutionTime())
}

func TestDelegationVerifier_Verify_Success(t *testing.T) {
	dv := NewDelegationVerifier("input", "hash")
	dv.SetResult("expected_output", 500)

	result := dv.Verify("expected_output")
	assert.True(t, result)
	assert.True(t, dv.IsVerified())
}

func TestDelegationVerifier_Verify_Failure(t *testing.T) {
	dv := NewDelegationVerifier("input", "compress")
	dv.SetResult("actual_output", 500)

	result := dv.Verify("different_output")
	assert.False(t, result)
	assert.False(t, dv.IsVerified())
}

func TestDelegationVerifier_ConcurrentAccess(t *testing.T) {
	dv := NewDelegationVerifier("input", "encrypt")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			dv.SetResult("output", int64(idx))
			dv.Verify("output")
			_ = dv.IsVerified()
			_ = dv.OutputDigest()
		}(i)
	}
	wg.Wait()

	// Should be verified since we always set and verify "output"
	assert.True(t, dv.IsVerified())
}

// ========== VerificationStatus Tests ==========

func TestVerificationStatus_String(t *testing.T) {
	assert.Equal(t, "pending", VerificationPending.String())
	assert.Equal(t, "passed", VerificationPassed.String())
	assert.Equal(t, "failed", VerificationFailed.String())
	assert.Equal(t, "corrupted", VerificationCorrupted.String())
	assert.Equal(t, "unknown", VerificationStatus(99).String())
}

// ========== Integration Test ==========

func TestDelegationFlow_EndToEnd(t *testing.T) {
	// Simulate a full delegation flow:
	// 1. Local node has data, Rust computes BLAKE3 hash
	// 2. Remote Rust module processes data and returns hash
	// 3. Local Go code validates the returned hash

	// Simulate: Rust computed input hash
	inputDigest := "abc123def456"

	// Create delegation verifier
	dv := NewDelegationVerifier(inputDigest, "compress")

	// Simulate: Remote Rust module processes and returns output hash
	outputDigest := "789xyz"
	executionTimeNs := int64(50000000) // 50ms

	dv.SetResult(outputDigest, executionTimeNs)

	// Verify the output matches what we expect
	// (In real flow, we'd recompute or have secondary verification)
	assert.True(t, dv.Verify(outputDigest))
	assert.True(t, dv.IsVerified())
	assert.Equal(t, int64(50000000), dv.ExecutionTime())
}

func TestStreamingVerifier_FullFlow(t *testing.T) {
	// Simulate streaming verification where data is streamed
	// and Rust returns final BLAKE3 hash

	// Expected hash (would come from prior computation or spec)
	expectedHash := []byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
	}

	sv := NewStreamingVerifier(expectedHash)

	// Process some data (header check)
	sv.ProcessHeader([]byte("file header"))

	// Stream the rest
	sv.ProcessStream(bytes.NewReader(make([]byte, 1024*1024)))

	// Rust module returns hash after processing
	rustComputedHash := expectedHash // Same in this test

	// Finalize with Rust's hash
	verified := sv.Finalize(rustComputedHash)
	assert.True(t, verified)
	assert.Equal(t, VerificationPassed, sv.Status())
}

// Helper for error tests
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

// ========== Benchmarks ==========

func BenchmarkDigestValidator_Validate(b *testing.B) {
	digest := make([]byte, 32)
	for i := range digest {
		digest[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dv := NewDigestValidator(digest)
		dv.Validate(digest)
	}
}

func BenchmarkStreamingVerifier_ProcessStream_1MB(b *testing.B) {
	data := make([]byte, 1024*1024)
	digest := make([]byte, 32)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sv := NewStreamingVerifier(digest)
		sv.ProcessStream(bytes.NewReader(data))
		sv.Finalize(digest)
	}
}

func BenchmarkDelegationVerifier_FullFlow(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dv := NewDelegationVerifier("input_hash", "compress")
		dv.SetResult("output_hash", 1000)
		dv.Verify("output_hash")
	}
}
