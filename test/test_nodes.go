package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	libp2p_host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

const numNodes = 2
const numPackets = 5
const protocolID = "/packet/1.0.0"

type TestNode struct {
	Host libp2p_host.Host
	ID   int
}

func main() {
	ctx := context.Background()
	fmt.Println("[INFO] Creating mocknet...")
	mn := mocknet.New()
	var nodes []*TestNode

	// Create nodes
	for i := 0; i < numNodes; i++ {
		fmt.Printf("[INFO] Creating node %d...\n", i)
		h, err := mn.GenPeer()
		if err != nil {
			panic(err)
		}
		fmt.Printf("[INFO] Node %d PeerID: %s\n", i, h.ID().String())
		n := &TestNode{Host: h, ID: i}
		h.SetStreamHandler(protocolID, func(s network.Stream) {
			fmt.Printf("[NODE %d] Stream handler triggered!\n", n.ID)
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[NODE %d] Stream handler panic: %v\n", n.ID, r)
				}
			}()
			defer s.Close()
			data, err := io.ReadAll(s)
			if err != nil {
				fmt.Printf("[NODE %d] Error reading packet: %v\n", n.ID, err)
				return
			}
			fmt.Printf("[NODE %d] Received packet: %s\n", n.ID, string(data))
			resp := append([]byte(fmt.Sprintf("ACK%d:", n.ID)), data...)
			_, err = s.Write(resp)
			if err != nil {
				fmt.Printf("[NODE %d] Error writing response: %v\n", n.ID, err)
			} else {
				fmt.Printf("[NODE %d] Response sent.\n", n.ID)
			}
		})
		nodes = append(nodes, n)
	}

	fmt.Println("[INFO] Linking all nodes...")
	if err := mn.LinkAll(); err != nil {
		panic(err)
	}
	fmt.Println("[INFO] Connecting all nodes...")
	if err := mn.ConnectAllButSelf(); err != nil {
		panic(err)
	}
	fmt.Println("[INFO] All nodes linked and connected.")

	// Force mocknet tick to process events
	mn.Peers()
	time.Sleep(500 * time.Millisecond)

	for i, n := range nodes {
		fmt.Printf("[DEBUG] Node %d HostID: %s\n", i, n.Host.ID().String())
	}

	var wg sync.WaitGroup

	fmt.Println("[INFO] Starting load test...")
	// Load test: each node sends packets to all others
	for i := 0; i < numNodes; i++ {
		for j := 0; j < numNodes; j++ {
			if i == j {
				continue
			}
			for k := 0; k < numPackets; k++ {
				wg.Add(1)
				go func(src, dst, pkt int) {
					defer func() {
						if r := recover(); r != nil {
							fmt.Printf("[GOROUTINE] Panic: %v\n", r)
						}
					}()
					msg := []byte(fmt.Sprintf("Hello from %d to %d #%d", src, dst, pkt))
					fmt.Printf("[SEND] Node %d -> Node %d: %s\n", src, dst, string(msg))
					resp, err := sendPacket(ctx, nodes[src].Host, nodes[dst].Host.ID(), msg)
					if err != nil {
						fmt.Printf("[ERROR] Node %d -> Node %d: %v\n", src, dst, err)
					} else {
						fmt.Printf("[RECV] Node %d <- Node %d: %s\n", dst, src, string(resp))
					}
					fmt.Printf("[GOROUTINE] Node %d -> Node %d finished.\n", src, dst)
					wg.Done()
				}(i, j, k)
			}
		}
	}

	fmt.Println("[INFO] Starting penetration test...")
	// Penetration test: send malformed packets
	for i := 0; i < numNodes; i++ {
		for j := 0; j < numNodes; j++ {
			if i == j {
				continue
			}
			wg.Add(1)
			go func(src, dst int) {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("[GOROUTINE] PenTest Panic: %v\n", r)
					}
				}()
				malformed := make([]byte, rand.Intn(100)+1)
				for k := range malformed {
					malformed[k] = byte(rand.Intn(256))
				}
				fmt.Printf("[SEND] Node %d -> Node %d: [malformed %d bytes]\n", src, dst, len(malformed))
				resp, err := sendPacket(ctx, nodes[src].Host, nodes[dst].Host.ID(), malformed)
				if err != nil {
					fmt.Printf("[PenTest ERROR] Node %d -> Node %d: %v\n", src, dst, err)
				} else {
					fmt.Printf("[PenTest RECV] Node %d <- Node %d: %s\n", dst, src, string(resp))
				}
				fmt.Printf("[GOROUTINE] PenTest Node %d -> Node %d finished.\n", src, dst)
				wg.Done()
			}(i, j)
		}
	}

	wg.Wait()
	fmt.Println("[INFO] All goroutines finished. Waiting for logs to flush...")
	time.Sleep(3 * time.Second)
	fmt.Println("[INFO] Mocknet test complete.")
}

// sendPacket opens a stream and sends a packet
func sendPacket(ctx context.Context, host libp2p_host.Host, peerID peer.ID, packet []byte) ([]byte, error) {
	stream, err := host.NewStream(ctx, peerID, protocolID)
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
