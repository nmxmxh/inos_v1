//go:build wasm

package threads

import (
	"context"
	"io"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh"
	syscall "github.com/nmxmxh/inos_v1/kernel/gen/system/v1"
	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/nmxmxh/inos_v1/kernel/threads/supervisor"
	"github.com/nmxmxh/inos_v1/kernel/utils"
	capnp "zombiezen.com/go/capnproto2"
)

// runSignalListener polls the SAB bridge for outgoing signals from modules
// and dispatches them to the Mesh.
// Refactored for Phase 15: DeepSeek Architecture (Reactive, Syscall-based)
func (s *Supervisor) runSignalListener(ctx context.Context) error {
	s.logger.Info("Signal Listener thread started (Phase 15: Reactive)")

	bridge := s.bridge
	if bridge == nil {
		s.logger.Error("SAB Bridge is nil in Signal Listener")
		return nil
	}

	// Cast MeshCoordinator
	var meshCoord *mesh.MeshCoordinator
	if m, ok := s.config.MeshCoordinator.(*mesh.MeshCoordinator); ok {
		meshCoord = m
	} else {
		s.logger.Warn("MeshCoordinator not available or invalid type")
	}

	// Atomic Reactive Signal Loop (Phase 15/16 → Phase 17: Signal-Based)
	// We monitor the global outbox sequence counter using Atomics.wait
	var lastSeq uint32 = bridge.ReadOutboxSequence()

	for {
		// Check for shutdown
		select {
		case <-ctx.Done():
			s.logger.Info("Signal Listener stopping")
			return nil
		default:
		}

		// 1. Wait for epoch change using Atomics.wait (zero CPU)
		// This blocks until the outbox sequence changes or timeout (100ms)
		currentSeq := bridge.ReadOutboxSequence()
		if currentSeq == lastSeq {
			// Use signal-based waiting (Atomics.wait) - yields CPU until signaled
			bridge.WaitForEpochChange(
				sab_layout.IDX_OUTBOX_DIRTY,
				int32(lastSeq),
				100.0, // 100ms timeout for shutdown checks
			)

			// Re-read after wait
			currentSeq = bridge.ReadOutboxSequence()
			if currentSeq == lastSeq {
				// Timed out or spurious wake, check again
				continue
			}
		}

		// Activity Detected!
		lastSeq = currentSeq

		// 2. Read Raw Bytes from Outbox
		data, err := bridge.ReadOutboxRaw()
		if err != nil {
			s.logger.Error("Failed to read outbox", utils.Err(err))
			continue
		}
		if len(data) == 0 {
			continue
		}

		// 3. Validate Cap'n Proto Message
		// Try to unmarshal as Syscall Message (Root)
		msg, err := capnp.Unmarshal(data)
		if err != nil {
			continue
		}

		// Try reading as Syscall Message
		env, err := syscall.ReadRootSyscall_Message(msg)
		if err == nil {
			// Check Header for Magic
			header, _ := env.Header()
			if header.Magic() == 0x53424142 {
				s.handleSyscall(ctx, meshCoord, env)
				continue
			}
		}

		// 4. Resolve Job if it's a JobResult
		result := bridge.DeserializeResult(data)
		if result.JobID != "" {
			s.logger.Debug("Resolving Job", utils.String("job_id", result.JobID), utils.Bool("success", result.Success))
			bridge.ResolveJob(result.JobID, result)
			continue
		}
	}
}

func (s *Supervisor) handleSyscall(ctx context.Context, m *mesh.MeshCoordinator, msg syscall.Syscall_Message) {
	header, _ := msg.Header()
	callId := header.CallId()

	// Switch on Body Union
	body, _ := msg.Body()

	if m == nil {
		s.logger.Warn("Syscall ignored: No Mesh Coordinator")
		return
	}

	switch body.Which() {
	case syscall.Syscall_Body_Which_fetchChunk:
		req, _ := body.FetchChunk()
		hash, _ := req.Hash()
		destOffset := req.DestinationOffset()
		destSize := req.DestinationSize()

		s.logger.Info("Syscall: Fetch Chunk",
			utils.String("hash", string(hash)),
			utils.Uint64("call_id", callId),
			utils.Uint64("dest_offset", destOffset),
		)

		// 1. Validate Offset (Security)
		if err := s.bridge.ValidateArenaOffset(uint32(destOffset), destSize); err != nil {
			s.logger.Warn("Syscall Invalid Offset", utils.Err(err))
			s.sendErrorResponse(callId, "Invalid Memory Access: "+err.Error())
			return
		}

		// Execute Mesh Fetch
		go func() {
			fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			// Zero-Copy Path:
			// Create SABWriter pointing to module's requested offset in Arena
			writer := &SABWriter{
				bridge: s.bridge,
				offset: uint32(destOffset),
				limit:  destSize,
			}

			// Stream from Mesh
			n, err := m.FetchChunkDirect(fetchCtx, string(hash), writer)

			// Return Typed Response
			if err != nil {
				s.logger.Error("Syscall Fetch Failed", utils.Err(err))
				s.sendErrorResponse(callId, err.Error())
			} else {
				s.logger.Info("Syscall Fetch Success", utils.Int64("bytes", n))
				// Success: Return FetchChunkResult
				s.sendFetchChunkResponse(callId, uint32(n))
			}
		}()

	case syscall.Syscall_Body_Which_storeChunk:
		req, _ := body.StoreChunk()
		hash, _ := req.Hash()
		srcOffset := req.SourceOffset()
		size := req.Size()

		s.logger.Info("Syscall: Store Chunk",
			utils.String("hash", string(hash)),
			utils.Uint64("src", srcOffset),
			utils.Uint64("size", uint64(size)),
		)

		// 1. Validate Offset (Security)
		if err := s.bridge.ValidateArenaOffset(uint32(srcOffset), size); err != nil {
			s.logger.Warn("Syscall Invalid Offset", utils.Err(err))
			s.sendErrorResponse(callId, "Invalid Memory Access: "+err.Error())
			return
		}

		// Execute Store
		go func() {
			storeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			// 2. Read Raw Data from SAB (One-Copy)
			data, err := s.bridge.ReadRaw(uint32(srcOffset), size)
			if err != nil {
				s.sendErrorResponse(callId, "Failed to read SAB: "+err.Error())
				return
			}

			// 3. Store to Mesh (Real)
			replicas, err := m.DistributeChunk(storeCtx, string(hash), data)
			if err != nil {
				s.logger.Error("Failed to distribute chunk", utils.Err(err))
				s.sendErrorResponse(callId, "Mesh Distribution Failed: "+err.Error())
				return
			}

			s.logger.Info("Stored Chunk Success",
				utils.Int("bytes", len(data)),
				utils.Int("replicas", replicas),
			)

			// Return Typed Response
			s.sendStoreChunkResponse(callId, uint8(replicas))
		}()

	case syscall.Syscall_Body_Which_sendMessage:
		req, _ := body.SendMessage()
		targetId, _ := req.TargetId()
		payload, _ := req.Payload()

		s.logger.Info("Syscall: Send Message",
			utils.String("target", targetId),
			utils.Int("payload_len", len(payload)),
			utils.Uint64("call_id", callId),
		)

		go func() {
			sendCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			// Route to Mesh
			err := m.SendMessage(sendCtx, targetId, payload)
			if err != nil {
				s.logger.Error("Syscall SendMessage Failed", utils.Err(err))
				s.sendErrorResponse(callId, "Mesh Deliver Failed: "+err.Error())
				return
			}

			// Return Success
			s.sendSendMessageResponse(callId, true)
		}()

	default:
		s.logger.Warn("Unknown Syscall Body", utils.String("which", body.Which().String()))
	}
}

// sendSendMessageResponse sends a typed SendMessageResult
func (s *Supervisor) sendSendMessageResponse(callId uint64, delivered bool) {
	s.sendResponse(callId, func(resp syscall.Syscall_Response) error {
		resp.SetStatus(syscall.Syscall_Status_success)
		res, _ := resp.Result()
		sendRes, _ := res.NewSendMessage()
		sendRes.SetDelivered(delivered)
		return nil
	})
}

// sendFetchChunkResponse sends a typed FetchChunkResult
func (s *Supervisor) sendFetchChunkResponse(callId uint64, bytesTransferred uint32) {
	s.sendResponse(callId, func(resp syscall.Syscall_Response) error {
		resp.SetStatus(syscall.Syscall_Status_success)
		res, _ := resp.Result()
		fetchRes, _ := res.NewFetchChunk()
		fetchRes.SetBytesTransferred(bytesTransferred)
		fetchRes.SetHashVerified(true) // Implicitly verified by Mesh
		return nil
	})
}

// sendStoreChunkResponse sends a typed StoreChunkResult
func (s *Supervisor) sendStoreChunkResponse(callId uint64, replicas uint8) {
	s.sendResponse(callId, func(resp syscall.Syscall_Response) error {
		resp.SetStatus(syscall.Syscall_Status_success)
		res, _ := resp.Result()
		storeRes, _ := res.NewStoreChunk()
		storeRes.SetReplicas(replicas)
		return nil
	})
}

// sendErrorResponse sends a generic error response
func (s *Supervisor) sendErrorResponse(callId uint64, errorMsg string) {
	s.sendResponse(callId, func(resp syscall.Syscall_Response) error {
		resp.SetStatus(syscall.Syscall_Status_internalError) // Or invalidRequest based on context
		errStruct, _ := resp.Error()
		errStruct.SetMessage(errorMsg)
		return nil
	})
}

// sendResponse is a generic helper that handles allocation and signaling
func (s *Supervisor) sendResponse(callId uint64, builder func(syscall.Syscall_Response) error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		s.logger.Error("Failed to create response message", utils.Err(err))
		return
	}

	resp, err := syscall.NewRootSyscall_Response(seg)
	if err != nil {
		s.logger.Error("Failed to create root response", utils.Err(err))
		return
	}

	resp.SetCallId(callId)

	// Apply specific builder logic
	if err := builder(resp); err != nil {
		s.logger.Error("Failed to build response payload", utils.Err(err))
		return
	}

	// Serialize
	out, err := msg.Marshal()
	if err != nil {
		s.logger.Error("Failed to marshal response", utils.Err(err))
		return
	}

	// Write to Inbox
	if err := s.bridge.WriteInbox(out); err != nil {
		s.logger.Error("Failed to write response to inbox", utils.Err(err))
		return
	}

	// Signal Module
	s.bridge.SignalInbox()
}

// SABWriter implements io.Writer for direct SAB writing (Zero-Copy Return Path)
type SABWriter struct {
	bridge *supervisor.SABBridge
	offset uint32
	limit  uint32
	cursor uint32
}

func (w *SABWriter) Write(p []byte) (n int, err error) {
	if w.cursor >= w.limit {
		return 0, io.ErrShortBuffer
	}

	toWrite := len(p)
	remaining := int(w.limit - w.cursor)
	if toWrite > remaining {
		toWrite = remaining
	}

	// WriteRaw to SAB (One-Copy)
	if err := w.bridge.WriteRaw(w.offset+w.cursor, p[:toWrite]); err != nil {
		return 0, err
	}

	w.cursor += uint32(toWrite)

	if toWrite < len(p) {
		return toWrite, io.ErrShortBuffer
	}
	return toWrite, nil
}
