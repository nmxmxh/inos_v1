//go:build js && wasm

package transport

import (
	"encoding/json"
	"errors"
	"syscall/js"
)

type wasmSignalingChannel struct {
	ws       js.Value
	messages chan []byte
	closed   chan struct{}
}

func dialSignaling(url string) (SignalingChannel, error) {
	ws := js.Global().Get("WebSocket").New(url)
	ch := &wasmSignalingChannel{
		ws:       ws,
		messages: make(chan []byte, 100),
		closed:   make(chan struct{}),
	}

	ws.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		data := args[0].Get("data").String()
		js.Global().Get("console").Call("debug", "[SignalingWS] received:", data)
		ch.messages <- []byte(data)
		return nil
	}))

	ws.Set("onclose", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		js.Global().Get("console").Call("warn", "[SignalingWS] closed")
		close(ch.closed)
		return nil
	}))

	ws.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		js.Global().Get("console").Call("error", "[SignalingWS] error", args[0])
		return nil
	}))

	// Wait for open (blocking for Dial simplicity)
	opened := make(chan bool)
	ws.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		js.Global().Get("console").Call("info", "[SignalingWS] opened")
		opened <- true
		return nil
	}))

	select {
	case <-opened:
		return ch, nil
	case <-ch.closed:
		return nil, errors.New("websocket closed before opening")
	}
}

func (w *wasmSignalingChannel) Send(message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	w.ws.Call("send", string(data))
	return nil
}

func (w *wasmSignalingChannel) Receive() ([]byte, error) {
	select {
	case msg := <-w.messages:
		return msg, nil
	case <-w.closed:
		return nil, errors.New("websocket closed")
	}
}

func (w *wasmSignalingChannel) Close() error {
	w.ws.Call("close")
	return nil
}

func (w *wasmSignalingChannel) IsConnected() bool {
	return w.ws.Get("readyState").Int() == 1 // OPEN
}
