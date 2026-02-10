//go:build js && wasm
// +build js,wasm

package main

import (
	"context"
	"errors"
	"syscall/js"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/transport"
	"github.com/nmxmxh/inos_v1/kernel/threads/sab"
)

func jsMeshSetIdentity(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing identity argument"})
	}
	if kernelInstance == nil || kernelInstance.meshCoordinator == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	val := args[0]

	identity := kernelInstance.meshIdentity
	if did := val.Get("did"); did.Type() == js.TypeString {
		identity.DID = did.String()
	}
	if deviceID := val.Get("deviceId"); deviceID.Type() == js.TypeString {
		identity.DeviceID = deviceID.String()
	}
	if displayName := val.Get("displayName"); displayName.Type() == js.TypeString {
		identity.DisplayName = displayName.String()
	}
	if nodeID := val.Get("nodeId"); nodeID.Type() == js.TypeString {
		if nodeID.String() != "" && nodeID.String() != identity.NodeID {
			return js.ValueOf(map[string]interface{}{
				"error": "nodeId is immutable once the mesh is initialized",
			})
		}
	}

	kernelInstance.meshIdentity = identity
	kernelInstance.meshCoordinator.SetIdentity(identity.DID, identity.DeviceID, identity.DisplayName)
	return js.ValueOf(map[string]interface{}{
		"success": true,
		"did":     identity.DID,
		"nodeId":  identity.NodeID,
	})
}

func jsMeshConfigureTransport(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing transport config"})
	}
	if kernelInstance == nil || kernelInstance.meshCoordinator == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	if kernelInstance.state.Load() >= int32(StateRunning) {
		return js.ValueOf(map[string]interface{}{
			"error": "transport config must be set before mesh start",
		})
	}

	config := transport.DefaultTransportConfig()
	applyTransportConfigOverrides(&config, args[0])

	if err := kernelInstance.replaceTransport(config); err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}

	return js.ValueOf(map[string]interface{}{"success": true})
}

func jsMeshGetTelemetry(this js.Value, args []js.Value) interface{} {
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	return js.ValueOf(coord.GetTelemetry())
}

func jsMeshGetMetrics(this js.Value, args []js.Value) interface{} {
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	return js.ValueOf(coord.GetGlobalMetrics())
}

func jsMeshFindPeersWithChunk(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing chunk hash"})
	}
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	peers, err := coord.FindPeersWithChunk(ctx, args[0].String())
	if err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}
	return js.ValueOf(peers)
}

func jsMeshFindBestPeerForChunk(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing chunk hash"})
	}
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	peer, err := coord.FindBestPeerForChunk(ctx, args[0].String())
	if err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}
	return js.ValueOf(peer)
}

func jsMeshDelegateCompute(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing job"})
	}
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	job := args[0]
	operation := job.Get("operation").String()
	inputDigest := job.Get("inputDigest").String()

	var data []byte
	if dataVal := job.Get("data"); !dataVal.IsUndefined() && !dataVal.IsNull() {
		data = make([]byte, dataVal.Get("length").Int())
		js.CopyBytesToGo(data, dataVal)
	}

	if operation == "" || inputDigest == "" {
		return js.ValueOf(map[string]interface{}{"error": "operation and inputDigest are required"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := coord.DelegateCompute(ctx, operation, inputDigest, data)
	if err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}

	out := js.Global().Get("Uint8Array").New(len(result))
	js.CopyBytesToJS(out, result)
	return js.ValueOf(map[string]interface{}{
		"success": true,
		"data":    out,
	})
}

func jsMeshRegisterChunk(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return js.ValueOf(map[string]interface{}{"error": "missing chunk hash or size"})
	}
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	chunkHash := args[0].String()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := coord.RegisterChunk(ctx, chunkHash); err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}
	return js.ValueOf(map[string]interface{}{"success": true})
}

func jsMeshUnregisterChunk(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing chunk hash"})
	}
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := coord.UnregisterChunk(ctx, args[0].String()); err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}
	return js.ValueOf(map[string]interface{}{"success": true})
}

func jsMeshScheduleChunkPrefetch(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing chunk list"})
	}
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	chunkHashes := jsValueToStringSlice(args[0])
	priority := ""
	if len(args) > 1 && args[1].Type() == js.TypeString {
		priority = args[1].String()
	}
	ctx := context.Background()
	if err := coord.ScheduleChunkPrefetch(ctx, chunkHashes, priority); err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}
	return js.ValueOf(map[string]interface{}{"success": true})
}

func jsMeshReportPeerPerformance(this js.Value, args []js.Value) interface{} {
	if len(args) < 3 {
		return js.ValueOf(map[string]interface{}{"error": "missing arguments"})
	}
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	peerID := args[0].String()
	success := args[1].Bool()
	latencyMs := float32(args[2].Float())
	operation := ""
	if len(args) > 3 && args[3].Type() == js.TypeString {
		operation = args[3].String()
	}
	if err := coord.ReportPeerPerformance(peerID, success, latencyMs, operation); err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}
	return js.ValueOf(map[string]interface{}{"success": true})
}

func jsMeshGetPeerReputation(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing peer ID"})
	}
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	score, confidence, err := coord.GetPeerReputation(args[0].String())
	if err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}
	return js.ValueOf(map[string]interface{}{
		"score":      score,
		"confidence": confidence,
	})
}

func jsMeshGetTopPeers(this js.Value, args []js.Value) interface{} {
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	limit := 10
	if len(args) > 0 && args[0].Type() == js.TypeNumber {
		limit = args[0].Int()
	}
	return js.ValueOf(coord.GetTopPeers(limit))
}

func jsMeshConnectToPeer(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing peer ID"})
	}
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}

	address := ""
	if len(args) > 1 && args[1].Type() == js.TypeString {
		address = args[1].String()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := coord.ConnectToPeer(ctx, args[0].String(), address); err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}
	return js.ValueOf(map[string]interface{}{"success": true})
}

func jsMeshDisconnectFromPeer(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing peer ID"})
	}
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	if err := coord.DisconnectFromPeer(args[0].String()); err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}
	return js.ValueOf(map[string]interface{}{"success": true})
}

func jsMeshSubscribeToEvents(this js.Value, args []js.Value) interface{} {
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	topics := []string{}
	if len(args) > 0 {
		topics = jsValueToStringSlice(args[0])
	}
	subID, err := coord.SubscribeToEvents(topics)
	if err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}
	return js.ValueOf(map[string]interface{}{
		"success":        true,
		"subscriptionId": subID,
		"epochIndex":     sab.IDX_MESH_EVENT_EPOCH,
		"headIndex":      sab.IDX_MESH_EVENT_HEAD,
		"tailIndex":      sab.IDX_MESH_EVENT_TAIL,
		"droppedIndex":   sab.IDX_MESH_EVENT_DROPPED,
		"queueOffset":    sab.OFFSET_MESH_EVENT_QUEUE,
		"slotSize":       sab.MESH_EVENT_SLOT_SIZE,
		"slotCount":      sab.MESH_EVENT_SLOT_COUNT,
	})
}

func jsMeshUnsubscribeFromEvents(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{"error": "missing subscription ID"})
	}
	coord := kernelInstance.meshCoordinator
	if coord == nil {
		return js.ValueOf(map[string]interface{}{"error": "mesh not initialized"})
	}
	success := coord.UnsubscribeFromEvents(args[0].String())
	return js.ValueOf(map[string]interface{}{"success": success})
}

func jsValueToStringSlice(val js.Value) []string {
	if val.IsUndefined() || val.IsNull() {
		return nil
	}
	length := val.Length()
	out := make([]string, 0, length)
	for i := 0; i < length; i++ {
		item := val.Index(i)
		if item.Type() == js.TypeString {
			out = append(out, item.String())
		}
	}
	return out
}

func (k *Kernel) replaceTransport(cfg transport.TransportConfig) error {
	if k.meshCoordinator == nil {
		return errors.New("mesh coordinator missing")
	}
	nodeID := k.meshIdentity.NodeID
	tr, err := transport.NewWebRTCTransport(nodeID, cfg, nil)
	if err != nil {
		return err
	}
	k.meshCoordinator.ReplaceTransport(tr)
	return nil
}
