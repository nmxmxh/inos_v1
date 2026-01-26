package mesh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
)

const attestationVersion = "inos-attest-v1"
const attestationMethod = "mesh.Attest"
const attestationMaxSABBytes = 2048
const attestationKnownRegionMaxBytes = 4096

type AttestationChallenge struct {
	Version     string `json:"version"`
	Nonce       string `json:"nonce"`
	PeerID      string `json:"peer_id"`
	RequesterID string `json:"requester_id"`
	Timestamp   int64  `json:"timestamp"`
	SabOffset   uint32 `json:"sab_offset"`
	SabLength   uint32 `json:"sab_length"`
	KnownRegions []AttestationRegion `json:"known_regions"`
}

type AttestationResponse struct {
	Version     string `json:"version"`
	Nonce       string `json:"nonce"`
	PeerID      string `json:"peer_id"`
	RequesterID string `json:"requester_id"`
	Timestamp   int64  `json:"timestamp"`
	PublicKey   string `json:"public_key"`
	Signature   string `json:"signature"`
	SabHash     string `json:"sab_hash"`
	RegionHashes map[string]string `json:"region_hashes"`
}

type AttestationRegion struct {
	Name   string `json:"name"`
	Offset uint32 `json:"offset"`
	Length uint32 `json:"length"`
}

type AttestationRecord struct {
	PublicKey ed25519.PublicKey
	Attested  time.Time
}

func (m *MeshCoordinator) registerAttestationHandler() {
	m.transport.RegisterRPCHandler(attestationMethod, func(ctx context.Context, peerID string, args json.RawMessage) (interface{}, error) {
		var challenge AttestationChallenge
		if err := json.Unmarshal(args, &challenge); err != nil {
			return nil, fmt.Errorf("attestation challenge decode failed: %w", err)
		}
		if challenge.Version != attestationVersion {
			return nil, fmt.Errorf("unsupported attestation version: %s", challenge.Version)
		}
		if challenge.PeerID != "" && challenge.PeerID != m.nodeID {
			return nil, errors.New("attestation challenge not addressed to this node")
		}
		if m.gossip == nil {
			return nil, errors.New("gossip manager unavailable for attestation")
		}
		if m.bridge == nil {
			return nil, errors.New("SAB bridge unavailable for attestation")
		}
		if err := validateSabChallenge(challenge, m.bridge.Size()); err != nil {
			return nil, err
		}
		if err := validateKnownRegions(challenge.KnownRegions, m.bridge.Size()); err != nil {
			return nil, err
		}

		sabHash, err := hashSabRange(m.bridge, challenge.SabOffset, challenge.SabLength)
		if err != nil {
			return nil, err
		}

		regionHashes, err := hashKnownRegions(m.bridge, challenge.KnownRegions)
		if err != nil {
			return nil, err
		}

		payload := attestationPayload(challenge, sabHash, regionHashes)
		signature, publicKey, err := m.gossip.SignAttestation(payload)
		if err != nil {
			return nil, fmt.Errorf("attestation signing failed: %w", err)
		}

		return AttestationResponse{
			Version:     challenge.Version,
			Nonce:       challenge.Nonce,
			PeerID:      m.nodeID,
			RequesterID: challenge.RequesterID,
			Timestamp:   challenge.Timestamp,
			PublicKey:   base64.StdEncoding.EncodeToString(publicKey),
			Signature:   base64.StdEncoding.EncodeToString(signature),
			SabHash:     base64.StdEncoding.EncodeToString(sabHash),
			RegionHashes: encodeRegionHashes(regionHashes),
		}, nil
	})
}

func (m *MeshCoordinator) startPeerAttestation(peerID string) {
	if m.isPeerAttested(peerID) {
		m.acceptConnectedPeer(peerID)
		return
	}
	if !m.markAttesting(peerID) {
		return
	}

	go func() {
		defer m.unmarkAttesting(peerID)

		record, err := m.requestAttestation(peerID)
		if err != nil {
			m.logger.Warn("peer attestation failed", "peer", getShortID(peerID), "error", err)
			_ = m.transport.Disconnect(peerID)
			m.emitPeerUpdateEvent(&PeerCapability{
				PeerID:          peerID,
				ConnectionState: ConnectionStateFailed,
				LastSeen:        time.Now().UnixNano(),
			})
			return
		}

		m.attestationMu.Lock()
		m.attestedPeers[peerID] = record
		m.attestationMu.Unlock()

		m.acceptConnectedPeer(peerID)
	}()
}

func (m *MeshCoordinator) acceptConnectedPeer(peerID string) {
	_ = m.dht.AddPeer(PeerInfo{ID: peerID})
	m.gossip.AddPeer(peerID)
	m.emitPeerUpdateEvent(&PeerCapability{
		PeerID:          peerID,
		ConnectionState: ConnectionStateConnected,
		LastSeen:        time.Now().UnixNano(),
	})
}

func (m *MeshCoordinator) clearPeerAttestation(peerID string) {
	m.attestationMu.Lock()
	delete(m.attestedPeers, peerID)
	m.attestationMu.Unlock()
	m.unmarkAttesting(peerID)
}

func (m *MeshCoordinator) isPeerAttested(peerID string) bool {
	m.attestationMu.RLock()
	_, ok := m.attestedPeers[peerID]
	m.attestationMu.RUnlock()
	return ok
}

func (m *MeshCoordinator) markAttesting(peerID string) bool {
	m.attestingPeersMu.Lock()
	defer m.attestingPeersMu.Unlock()
	if _, exists := m.attestingPeers[peerID]; exists {
		return false
	}
	m.attestingPeers[peerID] = struct{}{}
	return true
}

func (m *MeshCoordinator) unmarkAttesting(peerID string) {
	m.attestingPeersMu.Lock()
	delete(m.attestingPeers, peerID)
	m.attestingPeersMu.Unlock()
}

func (m *MeshCoordinator) requestAttestation(peerID string) (AttestationRecord, error) {
	if m.transport == nil {
		return AttestationRecord{}, errors.New("transport unavailable for attestation")
	}
	if m.bridge == nil {
		return AttestationRecord{}, errors.New("SAB bridge unavailable for attestation")
	}

	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return AttestationRecord{}, fmt.Errorf("nonce generation failed: %w", err)
	}

	sabSize := m.bridge.Size()
	if sabSize == 0 {
		return AttestationRecord{}, errors.New("SAB size unavailable for attestation")
	}

	knownRegions := defaultAttestationRegions(sabSize)
	if len(knownRegions) == 0 {
		return AttestationRecord{}, errors.New("no attestation regions available")
	}

	randomRegion, err := pickRandomRegion(knownRegions)
	if err != nil {
		return AttestationRecord{}, fmt.Errorf("attestation region selection failed: %w", err)
	}

	sabLength := uint32(attestationMaxSABBytes)
	if sabLength > randomRegion.Length {
		sabLength = randomRegion.Length
	}
	if sabLength == 0 {
		return AttestationRecord{}, errors.New("attestation SAB length unavailable")
	}

	maxOffset := randomRegion.Length - sabLength
	regionOffset, err := randomOffset(maxOffset)
	if err != nil {
		return AttestationRecord{}, fmt.Errorf("attestation offset selection failed: %w", err)
	}
	sabOffset := randomRegion.Offset + regionOffset

	challenge := AttestationChallenge{
		Version:     attestationVersion,
		Nonce:       base64.StdEncoding.EncodeToString(nonceBytes),
		PeerID:      peerID,
		RequesterID: m.nodeID,
		Timestamp:   time.Now().UnixNano(),
		SabOffset:   sabOffset,
		SabLength:   sabLength,
		KnownRegions: knownRegions,
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.config.AttestationTimeout)
	defer cancel()

	var response AttestationResponse
	if err := m.transport.SendRPC(ctx, peerID, attestationMethod, challenge, &response); err != nil {
		return AttestationRecord{}, fmt.Errorf("attestation RPC failed: %w", err)
	}

	if err := validateAttestationResponse(challenge, response); err != nil {
		return AttestationRecord{}, err
	}

	pubKeyBytes, err := base64.StdEncoding.DecodeString(response.PublicKey)
	if err != nil {
		return AttestationRecord{}, fmt.Errorf("invalid attestation public key: %w", err)
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return AttestationRecord{}, errors.New("attestation public key has invalid size")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(response.Signature)
	if err != nil {
		return AttestationRecord{}, fmt.Errorf("invalid attestation signature encoding: %w", err)
	}

	responseHash, err := base64.StdEncoding.DecodeString(response.SabHash)
	if err != nil {
		return AttestationRecord{}, fmt.Errorf("invalid attestation SAB hash: %w", err)
	}

	decodedRegionHashes, err := decodeRegionHashes(challenge.KnownRegions, response.RegionHashes)
	if err != nil {
		return AttestationRecord{}, err
	}

	payload := attestationPayload(challenge, responseHash, decodedRegionHashes)
	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), payload, sigBytes) {
		return AttestationRecord{}, errors.New("attestation signature verification failed")
	}

	return AttestationRecord{
		PublicKey: ed25519.PublicKey(pubKeyBytes),
		Attested:  time.Now(),
	}, nil
}

func validateAttestationResponse(challenge AttestationChallenge, response AttestationResponse) error {
	if response.Version != attestationVersion {
		return fmt.Errorf("unsupported attestation response version: %s", response.Version)
	}
	if response.Nonce != challenge.Nonce {
		return errors.New("attestation nonce mismatch")
	}
	if response.PeerID != challenge.PeerID {
		return errors.New("attestation peer mismatch")
	}
	if response.RequesterID != challenge.RequesterID {
		return errors.New("attestation requester mismatch")
	}
	if response.Timestamp != challenge.Timestamp {
		return errors.New("attestation timestamp mismatch")
	}
	if strings.TrimSpace(response.PublicKey) == "" || strings.TrimSpace(response.Signature) == "" {
		return errors.New("attestation response missing signature material")
	}
	if strings.TrimSpace(response.SabHash) == "" {
		return errors.New("attestation response missing SAB hash")
	}
	if len(response.RegionHashes) == 0 {
		return errors.New("attestation response missing region hashes")
	}
	return nil
}

func attestationPayload(challenge AttestationChallenge, sabHash []byte, regionHashes map[string][]byte) []byte {
	builder := strings.Builder{}
	builder.Grow(len(attestationVersion) + len(challenge.Nonce) + len(challenge.PeerID) + len(challenge.RequesterID) + 64)
	builder.WriteString(attestationVersion)
	builder.WriteString(":")
	builder.WriteString(challenge.Nonce)
	builder.WriteString(":")
	builder.WriteString(challenge.PeerID)
	builder.WriteString(":")
	builder.WriteString(challenge.RequesterID)
	builder.WriteString(":")
	builder.WriteString(fmt.Sprintf("%d", challenge.Timestamp))
	builder.WriteString(":")
	builder.WriteString(fmt.Sprintf("%d", challenge.SabOffset))
	builder.WriteString(":")
	builder.WriteString(fmt.Sprintf("%d", challenge.SabLength))
	builder.WriteString(":")
	builder.WriteString(base64.StdEncoding.EncodeToString(sabHash))
	for _, region := range challenge.KnownRegions {
		builder.WriteString(":")
		builder.WriteString(region.Name)
		builder.WriteString(":")
		builder.WriteString(fmt.Sprintf("%d", region.Offset))
		builder.WriteString(":")
		builder.WriteString(fmt.Sprintf("%d", region.Length))
		builder.WriteString(":")
		builder.WriteString(base64.StdEncoding.EncodeToString(regionHashes[region.Name]))
	}
	return []byte(builder.String())
}

func validateSabChallenge(challenge AttestationChallenge, sabSize uint32) error {
	if sabSize == 0 {
		return errors.New("SAB size unavailable for attestation")
	}
	if challenge.SabLength == 0 {
		return errors.New("attestation SAB length must be non-zero")
	}
	if challenge.SabLength > attestationMaxSABBytes {
		return fmt.Errorf("attestation SAB length exceeds limit: %d", challenge.SabLength)
	}
	if challenge.SabOffset+challenge.SabLength > sabSize {
		return errors.New("attestation SAB range out of bounds")
	}
	return nil
}

func validateKnownRegions(regions []AttestationRegion, sabSize uint32) error {
	if len(regions) == 0 {
		return errors.New("attestation known regions missing")
	}
	seen := make(map[string]struct{}, len(regions))
	for _, region := range regions {
		if strings.TrimSpace(region.Name) == "" {
			return errors.New("attestation region name missing")
		}
		if region.Length == 0 {
			return errors.New("attestation region length must be non-zero")
		}
		if region.Offset+region.Length > sabSize {
			return errors.New("attestation region out of bounds")
		}
		if _, exists := seen[region.Name]; exists {
			return errors.New("attestation region name duplicated")
		}
		seen[region.Name] = struct{}{}
	}
	return nil
}

func hashSabRange(bridge SABWriter, offset uint32, size uint32) ([]byte, error) {
	data, err := bridge.ReadRaw(offset, size)
	if err != nil {
		return nil, fmt.Errorf("attestation SAB read failed: %w", err)
	}
	sum := sha256.Sum256(data)
	return sum[:], nil
}

func hashKnownRegions(bridge SABWriter, regions []AttestationRegion) (map[string][]byte, error) {
	hashes := make(map[string][]byte, len(regions))
	for _, region := range regions {
		sum, err := hashSabRange(bridge, region.Offset, region.Length)
		if err != nil {
			return nil, err
		}
		hashes[region.Name] = sum
	}
	return hashes, nil
}

func encodeRegionHashes(hashes map[string][]byte) map[string]string {
	encoded := make(map[string]string, len(hashes))
	for name, sum := range hashes {
		encoded[name] = base64.StdEncoding.EncodeToString(sum)
	}
	return encoded
}

func decodeRegionHashes(regions []AttestationRegion, encoded map[string]string) (map[string][]byte, error) {
	if len(encoded) == 0 {
		return nil, errors.New("attestation response missing region hashes")
	}
	hashes := make(map[string][]byte, len(regions))
	for _, region := range regions {
		encodedHash, ok := encoded[region.Name]
		if !ok {
			return nil, fmt.Errorf("attestation response missing hash for region: %s", region.Name)
		}
		decoded, err := base64.StdEncoding.DecodeString(encodedHash)
		if err != nil {
			return nil, fmt.Errorf("invalid attestation region hash for %s: %w", region.Name, err)
		}
		if len(decoded) != sha256.Size {
			return nil, fmt.Errorf("invalid attestation region hash size for %s", region.Name)
		}
		hashes[region.Name] = decoded
	}
	return hashes, nil
}

func defaultAttestationRegions(sabSize uint32) []AttestationRegion {
	regions := []AttestationRegion{
		{Name: "atomic_flags", Offset: sab_layout.OFFSET_ATOMIC_FLAGS, Length: sab_layout.SIZE_ATOMIC_FLAGS},
		{Name: "module_registry", Offset: sab_layout.OFFSET_MODULE_REGISTRY, Length: sab_layout.SIZE_MODULE_REGISTRY},
		{Name: "supervisor_headers", Offset: sab_layout.OFFSET_SUPERVISOR_HEADERS, Length: sab_layout.SIZE_SUPERVISOR_HEADERS},
		{Name: "economics", Offset: sab_layout.OFFSET_ECONOMICS, Length: sab_layout.SIZE_ECONOMICS},
		{Name: "identity_registry", Offset: sab_layout.OFFSET_IDENTITY_REGISTRY, Length: sab_layout.SIZE_IDENTITY_REGISTRY},
		{Name: "social_graph", Offset: sab_layout.OFFSET_SOCIAL_GRAPH, Length: sab_layout.SIZE_SOCIAL_GRAPH},
	}

	filtered := make([]AttestationRegion, 0, len(regions))
	for _, region := range regions {
		if region.Offset >= sabSize {
			continue
		}
		if region.Offset+region.Length > sabSize {
			region.Length = sabSize - region.Offset
		}
		if region.Length > attestationKnownRegionMaxBytes {
			region.Length = attestationKnownRegionMaxBytes
		}
		if region.Length == 0 {
			continue
		}
		filtered = append(filtered, region)
	}
	return filtered
}

func pickRandomRegion(regions []AttestationRegion) (AttestationRegion, error) {
	if len(regions) == 0 {
		return AttestationRegion{}, errors.New("attestation region list empty")
	}
	idx, err := randomIndex(len(regions))
	if err != nil {
		return AttestationRegion{}, err
	}
	return regions[idx], nil
}

func randomOffset(max uint32) (uint32, error) {
	if max == 0 {
		return 0, nil
	}
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, err
	}
	value := binary.LittleEndian.Uint32(buf[:])
	return value % (max + 1), nil
}

func randomIndex(max int) (int, error) {
	if max <= 1 {
		return 0, nil
	}
	idx, err := randomOffset(uint32(max - 1))
	if err != nil {
		return 0, err
	}
	return int(idx), nil
}
