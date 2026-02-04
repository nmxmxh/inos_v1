package mesh

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
	"github.com/nmxmxh/inos_v1/kernel/gen/p2p/v1"
	"github.com/nmxmxh/inos_v1/kernel/gen/system/v1"
	"github.com/nmxmxh/inos_v1/kernel/threads/sab"
	capnp "zombiezen.com/go/capnproto2"
)

const meshEventHeaderSize = 16

type meshSubscription struct {
	id     string
	topics map[string]struct{}
}

type MeshEventQueue struct {
	bridge    SABWriter
	base      uint32
	slotSize  uint32
	slotCount uint32
}

type regionGuarder interface {
	AcquireRegionWriteGuard(region sab.RegionId, owner sab.RegionOwner) (sab.RegionWriteGuard, error)
}

func NewMeshEventQueue(bridge SABWriter) *MeshEventQueue {
	queue := &MeshEventQueue{
		bridge:    bridge,
		base:      sab.OFFSET_MESH_EVENT_QUEUE,
		slotSize:  sab.MESH_EVENT_SLOT_SIZE,
		slotCount: sab.MESH_EVENT_SLOT_COUNT,
	}
	queue.resetCounters()
	return queue
}

func (q *MeshEventQueue) resetCounters() {
	zero := []byte{0, 0, 0, 0}
	_ = q.bridge.WriteRaw(sab.OFFSET_ATOMIC_FLAGS+sab.IDX_MESH_EVENT_HEAD*4, zero)
	_ = q.bridge.WriteRaw(sab.OFFSET_ATOMIC_FLAGS+sab.IDX_MESH_EVENT_TAIL*4, zero)
	_ = q.bridge.WriteRaw(sab.OFFSET_ATOMIC_FLAGS+sab.IDX_MESH_EVENT_DROPPED*4, zero)
}

func (q *MeshEventQueue) Enqueue(topic string, payload []byte) error {
	if q == nil || q.bridge == nil {
		return errors.New("mesh event queue unavailable")
	}
	if len(payload) == 0 {
		return errors.New("mesh event payload empty")
	}

	maxPayload := int(q.slotSize) - meshEventHeaderSize
	if len(payload) > maxPayload {
		q.bridge.AtomicAdd(sab.IDX_MESH_EVENT_DROPPED, 1)
		return fmt.Errorf("mesh event payload too large (%d > %d)", len(payload), maxPayload)
	}

	var guard sab.RegionWriteGuard
	if guarder, ok := q.bridge.(regionGuarder); ok {
		g, err := guarder.AcquireRegionWriteGuard(sab.RegionMeshEventQueue, sab.RegionOwnerKernel)
		if err != nil {
			return err
		}
		guard = g
	}

	head := q.bridge.AtomicLoad(sab.IDX_MESH_EVENT_HEAD)
	tail := q.bridge.AtomicLoad(sab.IDX_MESH_EVENT_TAIL)
	inFlight := tail - head
	if inFlight >= q.slotCount {
		q.bridge.AtomicAdd(sab.IDX_MESH_EVENT_DROPPED, 1)
		return errors.New("mesh event queue full")
	}

	slot := tail % q.slotCount
	offset := q.base + slot*q.slotSize

	header := make([]byte, meshEventHeaderSize)
	binary.LittleEndian.PutUint32(header[0:4], uint32(len(payload)))
	binary.LittleEndian.PutUint32(header[4:8], crc32.ChecksumIEEE([]byte(topic)))
	binary.LittleEndian.PutUint32(header[8:12], crc32.ChecksumIEEE(payload))

	if err := q.bridge.WriteRaw(offset, header); err != nil {
		q.bridge.AtomicAdd(sab.IDX_MESH_EVENT_DROPPED, 1)
		return err
	}
	if err := q.bridge.WriteRaw(offset+meshEventHeaderSize, payload); err != nil {
		q.bridge.AtomicAdd(sab.IDX_MESH_EVENT_DROPPED, 1)
		return err
	}

	q.bridge.AtomicAdd(sab.IDX_MESH_EVENT_TAIL, 1)
	q.bridge.SignalEpoch(sab.IDX_MESH_EVENT_EPOCH)

	if guard != nil {
		if err := guard.EnsureEpochAdvanced(); err != nil {
			_ = guard.Release()
			return err
		}
		if err := guard.Release(); err != nil {
			return err
		}
	}
	return nil
}

func (m *MeshCoordinator) SubscribeToEvents(topics []string) (string, error) {
	if m.bridge == nil {
		return "", errors.New("mesh SAB bridge unavailable")
	}
	if m.eventQueue == nil {
		m.eventQueue = NewMeshEventQueue(m.bridge)
	}

	sub := &meshSubscription{
		id:     fmt.Sprintf("mesh_sub_%d", time.Now().UnixNano()),
		topics: make(map[string]struct{}, len(topics)),
	}
	for _, topic := range topics {
		trimmed := strings.TrimSpace(topic)
		if trimmed == "" {
			continue
		}
		sub.topics[trimmed] = struct{}{}
	}

	m.subscriptionsMu.Lock()
	if m.subscriptions == nil {
		m.subscriptions = make(map[string]*meshSubscription)
	}
	m.subscriptions[sub.id] = sub
	m.subscriptionsMu.Unlock()

	return sub.id, nil
}

func (m *MeshCoordinator) UnsubscribeFromEvents(subscriptionID string) bool {
	if subscriptionID == "" {
		return false
	}
	m.subscriptionsMu.Lock()
	_, ok := m.subscriptions[subscriptionID]
	if ok {
		delete(m.subscriptions, subscriptionID)
	}
	m.subscriptionsMu.Unlock()
	return ok
}

func (m *MeshCoordinator) emitMeshEvent(topic string, payload []byte) {
	if m.eventQueue == nil || m.bridge == nil {
		return
	}
	if !m.shouldEmitTopic(topic) {
		return
	}

	env := &common.Envelope{
		ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:      topic,
		Timestamp: time.Now().UnixNano(),
		Version:   "1.0",
		Metadata: common.EnvelopeMetadata{
			UserID:   m.did,
			DeviceID: m.device,
		},
		Payload: payload,
	}

	data, err := env.Marshal()
	if err != nil {
		return
	}
	_ = m.eventQueue.Enqueue(topic, data)
}

func (m *MeshCoordinator) shouldEmitTopic(topic string) bool {
	m.subscriptionsMu.RLock()
	defer m.subscriptionsMu.RUnlock()

	if len(m.subscriptions) == 0 {
		return false
	}
	for _, sub := range m.subscriptions {
		if topicMatches(sub.topics, topic) {
			return true
		}
	}
	return false
}

func topicMatches(topics map[string]struct{}, topic string) bool {
	if len(topics) == 0 {
		return true
	}
	if _, ok := topics["*"]; ok {
		return true
	}
	if _, ok := topics[topic]; ok {
		return true
	}
	for t := range topics {
		if strings.HasSuffix(t, ".*") {
			prefix := strings.TrimSuffix(t, ".*")
			if strings.HasPrefix(topic, prefix+".") {
				return true
			}
		}
	}
	return false
}

func (m *MeshCoordinator) emitPeerUpdateEvent(capability *PeerCapability) {
	if capability == nil {
		return
	}
	payload, err := marshalMeshEvent(func(ev p2p.MeshEvent) error {
		capnpCap, err := capability.ToCapnp(ev.Struct.Segment())
		if err != nil {
			return err
		}
		return ev.SetPeerUpdate(capnpCap)
	})
	if err != nil {
		return
	}
	m.emitMeshEvent("mesh.peer_update", payload)
}

func (m *MeshCoordinator) emitChunkDiscoveredEvent(chunkHash, peerID string, priority p2p.ChunkPriority) {
	payload, err := marshalMeshEvent(func(ev p2p.MeshEvent) error {
		chunk, err := ev.NewChunkDiscovered()
		if err != nil {
			return err
		}
		chunk.SetChunkHash(chunkHash)
		chunk.SetPeerId(peerID)
		chunk.SetPriority(priority)
		return nil
	})
	if err != nil {
		return
	}
	m.emitMeshEvent("mesh.chunk_discovered", payload)
}

func (m *MeshCoordinator) emitReputationUpdate(peerID string, score float64, reason string) {
	payload, err := marshalMeshEvent(func(ev p2p.MeshEvent) error {
		update, err := ev.NewReputationChange()
		if err != nil {
			return err
		}
		update.SetPeerId(peerID)
		update.SetNewScore(float32(score))
		update.SetReason(reason)
		return nil
	})
	if err != nil {
		return
	}
	m.emitMeshEvent("mesh.reputation_update", payload)
}

func (m *MeshCoordinator) emitDelegationRequestEvent(operation string, id string, digest []byte, rawSize uint32) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return
	}

	req, err := p2p.NewDelegateRequest(seg)
	if err != nil {
		return
	}
	req.SetId(id)
	op, _ := req.NewOperation()
	op.SetCustom(operation)
	meta, _ := req.NewMetadata()
	meta.SetUserId(m.did)
	meta.SetDeviceId(m.device)
	resource, err := buildResourceStub(seg, id, digest, rawSize)
	if err != nil {
		return
	}
	req.SetResource(resource)

	_ = msg.SetRootPtr(req.Struct.ToPtr())
	payload, err := msg.Marshal()
	if err != nil {
		return
	}
	m.emitMeshEvent("delegation.request", payload)
}

func (m *MeshCoordinator) emitDelegationResponseEvent(status p2p.DelegateResponse_Status, id string, digest []byte, rawSize uint32, latencyMs float32, errMsg string) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return
	}

	resp, err := p2p.NewDelegateResponse(seg)
	if err != nil {
		return
	}
	resp.SetStatus(status)
	resp.SetError(errMsg)
	resource, err := buildResourceStub(seg, id, digest, rawSize)
	if err != nil {
		return
	}
	resp.SetResult(resource)
	metrics, _ := resp.NewMetrics()
	metrics.SetExecutionTimeNs(uint64(latencyMs * 1000000))

	_ = msg.SetRootPtr(resp.Struct.ToPtr())
	payload, err := msg.Marshal()
	if err != nil {
		return
	}
	m.emitMeshEvent("delegation.response", payload)
}

func marshalMeshEvent(fill func(p2p.MeshEvent) error) ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	ev, err := p2p.NewRootMeshEvent(seg)
	if err != nil {
		return nil, err
	}
	if err := fill(ev); err != nil {
		return nil, err
	}
	return msg.Marshal()
}

func buildResourceStub(seg *capnp.Segment, id string, digest []byte, rawSize uint32) (system.Resource, error) {
	res, err := system.NewResource(seg)
	if err != nil {
		return system.Resource{}, err
	}
	res.SetId(id)
	res.SetDigest(digest)
	res.SetRawSize(rawSize)
	res.SetWireSize(rawSize)
	res.SetCompression(system.Resource_Compression_none)
	res.SetEncryption(system.Resource_Encryption_none)
	res.SetTimestamp(uint64(time.Now().UnixNano()))
	res.SetPriority(128)
	alloc, _ := res.NewAllocation()
	alloc.SetType(system.Resource_Allocation_Type_heap)
	alloc.SetLifetime(system.Resource_Allocation_Lifetime_ephemeral)
	meta, _ := res.NewMetadata()
	meta.SetContentType("mesh/delegation")
	_, _ = res.NewShards(0)
	return res, nil
}
