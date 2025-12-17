package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	libp2p_host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

const identityFile = "node_identity.json"

// PersistentIdentity holds the private key and peer ID.
type PersistentIdentity struct {
	PrivKey []byte `json:"priv_key"`
	PeerID  string `json:"peer_id"`
}

// SaveIdentity saves identity to disk.
func SaveIdentity(id *PersistentIdentity) error {
	data, err := json.Marshal(id)
	if err != nil {
		return err
	}
	return os.WriteFile(identityFile, data, 0600)
}

// LoadIdentity loads identity from disk.
func LoadIdentity() (*PersistentIdentity, error) {
	data, err := os.ReadFile(identityFile)
	if err != nil {
		return nil, err
	}
	var id PersistentIdentity
	if err := json.Unmarshal(data, &id); err != nil {
		return nil, err
	}
	return &id, nil
}

// StartNodeWithStreams starts a libp2p node and sets up a stream handler for packets.
func StartNodeWithStreams(ctx context.Context, handlePacket func([]byte) []byte) error {
	// Try to load identity
	var priv crypto.PrivKey
	var pid peer.ID
	id, err := LoadIdentity()
	if err == nil {
		priv, err = crypto.UnmarshalPrivateKey(id.PrivKey)
		if err != nil {
			return err
		}
		pid, err = peer.Decode(id.PeerID)
		if err != nil {
			return err
		}
	} else {
		priv, _, err = crypto.GenerateEd25519Key(nil)
		if err != nil {
			return err
		}
		pid, err = peer.IDFromPrivateKey(priv)
		if err != nil {
			return err
		}
		privBytes, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			return err
		}
		SaveIdentity(&PersistentIdentity{PrivKey: privBytes, PeerID: pid.String()})
	}

	host, err := libp2p.New(
		libp2p.Identity(priv),
	)
	if err != nil {
		return err
	}
	fmt.Println("Libp2p node started. Peer ID:", host.ID())

	host.SetStreamHandler("/packet/1.0.0", func(s network.Stream) {
		defer s.Close()
		data, _ := io.ReadAll(s)
		response := handlePacket(data)
		if response != nil {
			s.Write(response)
		}
	})

	select {} // Keep running
}

// SendPacket connects to a remote peer and sends a packet over a stream.
func SendPacket(ctx context.Context, host libp2p_host.Host, peerAddr string, packet []byte) ([]byte, error) {
	maddr, err := ma.NewMultiaddr(peerAddr)
	if err != nil {
		return nil, err
	}
	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return nil, err
	}
	if err := host.Connect(ctx, *info); err != nil {
		return nil, err
	}
	stream, err := host.NewStream(ctx, info.ID, "/packet/1.0.0")
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	_, err = stream.Write(packet)
	if err != nil {
		return nil, err
	}
	response, err := io.ReadAll(stream)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// TestNode wraps a libp2p node for testing
// Addr is the node's multiaddress
// SendPacket sends a packet to another node
type TestNode struct {
	Host libp2p_host.Host
	Addr string
}

// NewTestNode creates and starts a TestNode
func NewTestNode(ctx context.Context, idx int) *TestNode {
	handlePacket := func(data []byte) []byte {
		return append([]byte("ACK:"), data...)
	}

	host, err := libp2p.New()
	if err != nil {
		panic(err)
	}

	host.SetStreamHandler("/packet/1.0.0", func(s network.Stream) {
		defer s.Close()
		data, _ := io.ReadAll(s)
		resp := handlePacket(data)
		if resp != nil {
			s.Write(resp)
		}
	})

	addrs := host.Addrs()
	addr := ""
	if len(addrs) > 0 {
		addr = fmt.Sprintf("%s/p2p/%s", addrs[0].String(), host.ID().String())
	}

	return &TestNode{Host: host, Addr: addr}
}

// SendPacket sends a packet to a peer
func (n *TestNode) SendPacket(ctx context.Context, peerAddr string, packet []byte) ([]byte, error) {
	maddr, err := ma.NewMultiaddr(peerAddr)
	if err != nil {
		return nil, err
	}
	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return nil, err
	}
	if err := n.Host.Connect(ctx, *info); err != nil {
		return nil, err
	}
	stream, err := n.Host.NewStream(ctx, info.ID, "/packet/1.0.0")
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	_, err = stream.Write(packet)
	if err != nil {
		return nil, err
	}
	response, err := io.ReadAll(stream)
	if err != nil {
		return nil, err
	}
	return response, nil
}
