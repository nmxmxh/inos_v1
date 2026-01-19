//go:build !js || !wasm

package transport

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/gorilla/websocket"
)

type nativeSignalingChannel struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (n *nativeSignalingChannel) Send(message interface{}) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.conn == nil {
		return errors.New("not connected")
	}
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return n.conn.WriteMessage(websocket.TextMessage, data)
}

func (n *nativeSignalingChannel) Receive() ([]byte, error) {
	if n.conn == nil {
		return nil, errors.New("not connected")
	}
	_, message, err := n.conn.ReadMessage()
	return message, err
}

func (n *nativeSignalingChannel) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.conn != nil {
		err := n.conn.Close()
		n.conn = nil
		return err
	}
	return nil
}

func (n *nativeSignalingChannel) IsConnected() bool {
	return n.conn != nil
}

func dialSignaling(url string) (SignalingChannel, error) {
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	return &nativeSignalingChannel{conn: conn}, nil
}
