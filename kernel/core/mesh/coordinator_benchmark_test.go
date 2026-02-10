package mesh

import (
	"bytes"
	"context"
	"fmt"
	mrand "math/rand"
	"testing"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
)

func makePiedPiperCorpus(size int, noisePercent int) []byte {
	if size <= 0 {
		return nil
	}
	if noisePercent < 0 {
		noisePercent = 0
	}
	if noisePercent > 100 {
		noisePercent = 100
	}

	base := []byte("middle-out::dictionary::compression::")
	buf := bytes.Repeat(base, (size/len(base))+1)
	buf = buf[:size]

	noiseBytes := size * noisePercent / 100
	rng := mrand.New(mrand.NewSource(int64(size*31 + noisePercent)))
	for i := 0; i < noiseBytes; i++ {
		idx := rng.Intn(size)
		buf[idx] = byte(rng.Intn(256))
	}

	return buf
}

func BenchmarkMeshCoordinator_EncodePayloadForWire_PiedPiperCorpus(b *testing.B) {
	coord := NewMeshCoordinator("bench-node", "us-east", &MockTransport{nodeID: "bench-node"}, nil)

	cases := []struct {
		name  string
		size  int
		noise int
	}{
		{name: "64KB_low_noise", size: 64 * 1024, noise: 2},
		{name: "1MB_medium_noise", size: 1024 * 1024, noise: 8},
		{name: "4MB_medium_noise", size: 4 * 1024 * 1024, noise: 8},
	}

	for _, tc := range cases {
		payload := makePiedPiperCorpus(tc.size, tc.noise)
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(payload)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				encoded, err := coord.encodePayloadForWire(payload, meshCompressionMinBytes, meshBrotliCompressionLevel)
				if err != nil {
					b.Fatalf("encodePayloadForWire failed: %v", err)
				}
				if encoded.RawSize != len(payload) {
					b.Fatalf("raw size mismatch: got=%d want=%d", encoded.RawSize, len(payload))
				}
			}
		})
	}
}

func BenchmarkMeshCoordinator_ChunkRPCRoundTrip_Brotli(b *testing.B) {
	tr := &MockTransport{
		nodeID:      "bench-node",
		rpcHandlers: make(map[string]func(args interface{}) (interface{}, error)),
	}
	coord := NewMeshCoordinator("bench-node", "us-east", tr, nil)
	storage := &MockStorage{chunks: make(map[string][]byte)}
	coord.SetStorage(storage)

	chunkHash := "bench-chunk"
	payload := makePiedPiperCorpus(1024*1024, 6)
	peer := &common.PeerCapability{PeerID: "peer-1"}
	ctx := context.Background()

	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hash := fmt.Sprintf("%s-%d", chunkHash, i)
		if err := coord.sendChunkToPeer(ctx, peer.PeerID, hash, payload); err != nil {
			b.Fatalf("sendChunkToPeer failed: %v", err)
		}
		if _, err := coord.fetchFromPeer(ctx, hash, peer); err != nil {
			b.Fatalf("fetchFromPeer failed: %v", err)
		}
	}
}

func BenchmarkMeshCoordinator_DecodePayloadFromWire_Brotli(b *testing.B) {
	coord := NewMeshCoordinator("bench-node", "us-east", &MockTransport{nodeID: "bench-node"}, nil)
	original := makePiedPiperCorpus(1024*1024, 8)
	encoded, err := coord.encodePayloadForWire(original, meshCompressionMinBytes, meshBrotliCompressionLevel)
	if err != nil {
		b.Fatalf("encodePayloadForWire failed: %v", err)
	}
	if encoded.Compression != "brotli" {
		b.Skip("payload did not compress; benchmark requires brotli path")
	}

	b.SetBytes(int64(len(original)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		decoded, err := coord.decodePayloadFromWire(encoded.Data, encoded.Compression, encoded.RawSize)
		if err != nil {
			b.Fatalf("decodePayloadFromWire failed: %v", err)
		}
		if len(decoded) != len(original) {
			b.Fatalf("decoded length mismatch: got=%d want=%d", len(decoded), len(original))
		}
	}
}
