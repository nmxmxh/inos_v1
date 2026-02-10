//go:build !js || !wasm

package transport

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
	"github.com/pion/webrtc/v3"
)

func BenchmarkWebRTCTransport_HandleJSONPayloadChunkStore(b *testing.B) {
	tr, err := NewWebRTCTransport("bench-node", DefaultTransportConfig(), nil)
	if err != nil {
		b.Fatalf("failed to create transport: %v", err)
	}

	chunkData := make([]byte, 256*1024)
	for i := range chunkData {
		chunkData[i] = byte(i % 251)
	}

	tr.RegisterRPCHandler("chunk.store", func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error) {
		var req struct {
			ChunkHash string `json:"chunk_hash"`
			Data      []byte `json:"data"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		return map[string]any{
			"stored": req.ChunkHash != "" && len(req.Data) > 0,
		}, nil
	})

	payload, err := json.Marshal(map[string]interface{}{
		"type":       "chunk_store",
		"chunk_hash": "bench-hash",
		"data":       chunkData,
		"size":       len(chunkData),
	})
	if err != nil {
		b.Fatalf("failed to marshal payload: %v", err)
	}
	env := &common.Envelope{
		ID:        "bench-msg",
		Type:      "json_payload",
		Timestamp: time.Now().UnixNano(),
		Payload:   payload,
	}
	wire, err := env.Marshal()
	if err != nil {
		b.Fatalf("failed to marshal envelope: %v", err)
	}

	b.SetBytes(int64(len(chunkData)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tr.handleIncomingMessage("peer-bench", wire)
	}
}

func BenchmarkWebRTCTransport_HandleSignalingMessage_TargetFiltering(b *testing.B) {
	tr, err := NewWebRTCTransport("node-self", DefaultTransportConfig(), nil)
	if err != nil {
		b.Fatalf("failed to create transport: %v", err)
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  "v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n",
	}
	msg, err := json.Marshal(map[string]interface{}{
		"type":      "webrtc_offer",
		"peer_id":   "peer-a",
		"target_id": "someone-else",
		"offer":     offer,
	})
	if err != nil {
		b.Fatalf("failed to marshal signaling message: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.handleSignalingMessage(msg)
	}
}
