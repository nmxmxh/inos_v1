package main

import (
	"context"
	"fmt"
	"os"

	"github.com/nmxmxh/inos_v1/internal/core"
	"github.com/nmxmxh/inos_v1/internal/network"
	proto_v1 "github.com/nmxmxh/inos_v1/proto/packet/v1"
	"github.com/nmxmxh/inos_v1/wasm"
	"google.golang.org/protobuf/proto"
)

func handlePacket(data []byte) []byte {
	var packet proto_v1.Packet
	if err := proto.Unmarshal(data, &packet); err != nil {
		fmt.Println("Failed to unmarshal packet:", err)
		return nil
	}
	result, err := wasm.Execute(packet.Wasm, packet.Input)
	if err != nil {
		packet.Result = []byte("error")
	} else {
		packet.Result = result
	}
	packet.Cost += 10 // Simulate earning credits
	response, err := proto.Marshal(&packet)
	if err != nil {
		fmt.Println("Failed to marshal response:", err)
		return nil
	}
	return response
}

func main() {
	fmt.Println("INOS Node starting...")
	ctx := context.Background()

	// Persistent identity
	identity := core.NewIdentity()
	fmt.Println("Node identity:", identity.ID)

	// Start libp2p node
	if err := network.StartNodeWithStreams(ctx, handlePacket); err != nil {
		fmt.Println("Network error:", err)
		os.Exit(1)
	}

	// Initialize credits
	credits := core.Credits{Balance: 100}

	// Simulate real packet exchange between two nodes
	sender := core.NewIdentity()
	receiver := core.NewIdentity()

	packet := proto_v1.Packet{
		Wasm:  []byte("(wasm binary placeholder)"),
		Input: []byte("input data"),
		Cost:  10,
	}

	// Sender marshals packet
	data, err := proto.Marshal(&packet)
	if err != nil {
		fmt.Println("Sender failed to marshal packet:", err)
		os.Exit(1)
	}

	// Receiver unmarshals packet
	var received proto_v1.Packet
	if err := proto.Unmarshal(data, &received); err != nil {
		fmt.Println("Receiver failed to unmarshal packet:", err)
		os.Exit(1)
	}

	// Receiver executes WASM
	result, err := wasm.Execute(received.Wasm, received.Input)
	if err != nil {
		received.Result = []byte("error")
	} else {
		received.Result = result
	}

	// Receiver earns credits
	credits.Add(received.Cost)

	fmt.Printf("Packet exchange: sender=%s, receiver=%s, input=%s, result=%s, credits=%d\n", sender.ID, receiver.ID, received.Input, received.Result, credits.Balance)
	os.Exit(0)
}
