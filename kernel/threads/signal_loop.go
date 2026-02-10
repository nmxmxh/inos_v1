//go:build wasm

package threads

import (
	"context"
	"errors"
	"io"
	"syscall/js"
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
				sab_layout.IDX_OUTBOX_KERNEL_DIRTY,
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
			reportedReplicas := replicas
			const maxWireReplicas = int(^uint16(0))
			if reportedReplicas > maxWireReplicas {
				s.logger.Warn("Replica count exceeds syscall wire range; clamping to UInt16 max",
					utils.Int("requested_replicas", replicas),
					utils.Int("reported_replicas", maxWireReplicas),
				)
				reportedReplicas = maxWireReplicas
			}
			s.sendStoreChunkResponse(callId, uint16(reportedReplicas))
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

	case syscall.Syscall_Body_Which_hostCall:
		req, _ := body.HostCall()
		service, _ := req.Service()
		payload, _ := req.Payload()

		go func() {
			requestValue, err := resourceToJS(payload, s.bridge)
			if err != nil {
				s.sendErrorResponse(callId, "Invalid host payload: "+err.Error())
				return
			}

			responseValue, err := callHost(service, requestValue)
			if err != nil {
				s.sendErrorResponse(callId, "Host call failed: "+err.Error())
				return
			}

			s.sendHostCallResponse(callId, responseValue)
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
func (s *Supervisor) sendStoreChunkResponse(callId uint64, replicas uint16) {
	s.sendResponse(callId, func(resp syscall.Syscall_Response) error {
		resp.SetStatus(syscall.Syscall_Status_success)
		res, _ := resp.Result()
		storeRes, _ := res.NewStoreChunk()
		storeRes.SetReplicas(replicas)
		return nil
	})
}

func (s *Supervisor) sendHostCallResponse(callId uint64, response js.Value) {
	s.sendResponse(callId, func(resp syscall.Syscall_Response) error {
		resp.SetStatus(syscall.Syscall_Status_success)
		res, _ := resp.Result()
		hostRes, _ := res.NewHostCall()
		payload, err := hostRes.NewPayload()
		if err != nil {
			return err
		}
		if err := fillResourceFromJS(payload, response); err != nil {
			return err
		}
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

func callHost(service string, request js.Value) (js.Value, error) {
	fn := js.Global().Get("inosHostCall")
	if !fn.Truthy() {
		return js.Value{}, errors.New("inosHostCall is not available")
	}

	res := fn.Invoke(service, request)
	if res.Type() == js.TypeObject && res.InstanceOf(js.Global().Get("Promise")) {
		var err error
		res, err = awaitPromise(res)
		if err != nil {
			return js.Value{}, err
		}
	}

	return res, nil
}

func awaitPromise(promise js.Value) (js.Value, error) {
	done := make(chan struct{})
	var result js.Value
	var err error

	then := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) > 0 {
			result = args[0]
		}
		close(done)
		return nil
	})
	catchFn := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) > 0 {
			err = errors.New(args[0].String())
		} else {
			err = errors.New("promise rejected")
		}
		close(done)
		return nil
	})

	promise.Call("then", then).Call("catch", catchFn)
	<-done
	then.Release()
	catchFn.Release()

	if err != nil {
		return js.Value{}, err
	}
	return result, nil
}

func resourceToJS(payload syscall.Resource, bridge *supervisor.SABBridge) (js.Value, error) {
	switch payload.Which() {
	case syscall.Resource_Which_inline:
		data, err := payload.Inline()
		if err != nil {
			return js.Value{}, err
		}
		array := js.Global().Get("Uint8Array").New(len(data))
		js.CopyBytesToJS(array, data)
		custom := resourceCustomBytes(payload)
		if len(custom) == 0 {
			return array, nil
		}
		customArray := js.Global().Get("Uint8Array").New(len(custom))
		js.CopyBytesToJS(customArray, custom)
		return js.ValueOf(map[string]interface{}{
			"kind":   "inline",
			"data":   array,
			"custom": customArray,
		}), nil
	case syscall.Resource_Which_sabRef:
		ref, err := payload.SabRef()
		if err != nil {
			return js.Value{}, err
		}
		if err := bridge.ValidateArenaOffset(ref.Offset(), ref.Size()); err != nil {
			return js.Value{}, err
		}
		custom := resourceCustomBytes(payload)
		req := map[string]interface{}{
			"kind":   "sab",
			"offset": ref.Offset(),
			"size":   ref.Size(),
		}
		if len(custom) > 0 {
			customArray := js.Global().Get("Uint8Array").New(len(custom))
			js.CopyBytesToJS(customArray, custom)
			req["custom"] = customArray
		}
		return js.ValueOf(req), nil
	case syscall.Resource_Which_shards:
		return js.Value{}, errors.New("sharded payloads not supported for host calls")
	default:
		return js.Value{}, errors.New("unknown resource payload")
	}
}

func fillResourceFromJS(payload syscall.Resource, val js.Value) error {
	initResourceDefaults(payload)

	if !val.Truthy() {
		payload.SetInline([]byte{})
		return nil
	}

	if val.Type() == js.TypeString {
		payload.SetInline([]byte(val.String()))
		payload.SetRawSize(uint32(len(val.String())))
		payload.SetWireSize(uint32(len(val.String())))
		return nil
	}

	if val.Type() == js.TypeObject {
		offset := val.Get("offset")
		size := val.Get("size")
		if offset.Truthy() && size.Truthy() {
			ref, err := payload.NewSabRef()
			if err != nil {
				return err
			}
			ref.SetOffset(uint32(offset.Int()))
			ref.SetSize(uint32(size.Int()))
			payload.SetRawSize(uint32(size.Int()))
			payload.SetWireSize(uint32(size.Int()))
			if alloc, err := payload.Allocation(); err == nil {
				alloc.SetType(syscall.Resource_Allocation_Type_sab)
			}
			setResourceCustom(payload, val.Get("custom"))
			return nil
		}
		data := val.Get("data")
		if data.Truthy() {
			bytes, err := jsValueToBytes(data)
			if err != nil {
				return err
			}
			payload.SetInline(bytes)
			payload.SetRawSize(uint32(len(bytes)))
			payload.SetWireSize(uint32(len(bytes)))
			setResourceCustom(payload, val.Get("custom"))
			return nil
		}
	}

	bytes, err := jsValueToBytes(val)
	if err != nil {
		return err
	}
	payload.SetInline(bytes)
	payload.SetRawSize(uint32(len(bytes)))
	payload.SetWireSize(uint32(len(bytes)))
	return nil
}

func resourceCustomBytes(payload syscall.Resource) []byte {
	meta, err := payload.Metadata()
	if err != nil {
		return nil
	}
	custom, err := meta.Custom()
	if err != nil {
		return nil
	}
	return custom
}

func initResourceDefaults(payload syscall.Resource) {
	payload.SetCompression(syscall.Resource_Compression_none)
	payload.SetEncryption(syscall.Resource_Encryption_none)
	alloc, err := payload.NewAllocation()
	if err != nil {
		return
	}
	alloc.SetType(syscall.Resource_Allocation_Type_heap)
	alloc.SetLifetime(syscall.Resource_Allocation_Lifetime_ephemeral)
}

func setResourceCustom(payload syscall.Resource, val js.Value) {
	if !val.Truthy() {
		return
	}
	bytes, err := jsValueToBytes(val)
	if err != nil {
		return
	}
	meta, err := payload.NewMetadata()
	if err != nil {
		return
	}
	meta.SetCustom(bytes)
}

func jsValueToBytes(val js.Value) ([]byte, error) {
	if !val.Truthy() {
		return []byte{}, nil
	}
	if val.Type() == js.TypeString {
		return []byte(val.String()), nil
	}

	uint8Array := js.Global().Get("Uint8Array")
	if val.InstanceOf(uint8Array) {
		out := make([]byte, val.Length())
		js.CopyBytesToGo(out, val)
		return out, nil
	}

	arrayBuffer := js.Global().Get("ArrayBuffer")
	if val.InstanceOf(arrayBuffer) {
		buf := uint8Array.New(val)
		out := make([]byte, buf.Length())
		js.CopyBytesToGo(out, buf)
		return out, nil
	}

	return nil, errors.New("unsupported host response type")
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
