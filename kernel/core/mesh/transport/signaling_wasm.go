//go:build js && wasm

package transport

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"syscall/js"
)

type wasmSignalingChannel struct {
	ws       js.Value
	messages chan []byte
	closed   chan struct{}
}

func dialSignaling(ctx context.Context, url string) (SignalingChannel, error) {
	ws := js.Global().Get("WebSocket").New(url)
	ch := &wasmSignalingChannel{
		ws:       ws,
		messages: make(chan []byte, 100),
		closed:   make(chan struct{}),
	}

	var errMu sync.Mutex
	var lastErr error

	onMessage := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		data := args[0].Get("data").String()
		select {
		case ch.messages <- []byte(data):
		default:
			js.Global().Get("console").Call("warn", "[SignalingWS] message dropped: channel full")
		}
		return nil
	})

	onClose := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		js.Global().Get("console").Call("warn", "[SignalingWS] closed")
		select {
		case <-ch.closed:
		default:
			close(ch.closed)
		}
		return nil
	})

	onError := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		js.Global().Get("console").Call("error", "[SignalingWS] error", args[0])
		errMu.Lock()
		lastErr = errors.New("websocket error: " + args[0].String())
		errMu.Unlock()
		return nil
	})

	ws.Set("onmessage", onMessage)
	ws.Set("onclose", onClose)
	ws.Set("onerror", onError)

	// Clean up functions on exit
	defer func() {
		if lastErr != nil || ctx.Err() != nil {
			onMessage.Release()
			onClose.Release()
			onError.Release()
		}
	}()

	opened := make(chan bool)
	onOpen := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		js.Global().Get("console").Call("info", "[SignalingWS] opened")
		select {
		case opened <- true:
		default:
		}
		return nil
	})
	ws.Set("onopen", onOpen)
	defer onOpen.Release()

	select {
	case <-opened:
		// Successfully opened, the receiver loop will handle closing
		return ch, nil
	case <-ch.closed:
		errMu.Lock()
		defer errMu.Unlock()
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, errors.New("websocket closed before opening")
	case <-ctx.Done():
		ws.Call("close")
		return nil, ctx.Err()
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
