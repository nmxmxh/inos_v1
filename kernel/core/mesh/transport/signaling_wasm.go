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
	ws        js.Value
	messages  chan []byte
	closed    chan struct{}
	onMessage js.Func
	onClose   js.Func
	onError   js.Func
	onOpen    js.Func
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

	ch.onMessage = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		data := args[0].Get("data").String()
		select {
		case ch.messages <- []byte(data):
		default:
			js.Global().Get("console").Call("warn", "[SignalingWS] message dropped: channel full")
		}
		return nil
	})

	ch.onClose = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		js.Global().Get("console").Call("warn", "[SignalingWS] closed")
		select {
		case <-ch.closed:
		default:
			close(ch.closed)
		}
		return nil
	})

	ch.onError = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		js.Global().Get("console").Call("error", "[SignalingWS] error", args[0])
		errMu.Lock()
		lastErr = errors.New("websocket error: " + args[0].String())
		errMu.Unlock()
		return nil
	})

	ws.Set("onmessage", ch.onMessage)
	ws.Set("onclose", ch.onClose)
	ws.Set("onerror", ch.onError)

	opened := make(chan bool)
	ch.onOpen = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		js.Global().Get("console").Call("info", "[SignalingWS] opened")
		select {
		case opened <- true:
		default:
		}
		return nil
	})
	ws.Set("onopen", ch.onOpen)

	// Single exit point for dial logic
	cleanup := func() {
		ws.Set("onopen", js.Null())
		ws.Set("onmessage", js.Null())
		ws.Set("onclose", js.Null())
		ws.Set("onerror", js.Null())
		ch.onOpen.Release()
		ch.onMessage.Release()
		ch.onClose.Release()
		ch.onError.Release()
	}

	select {
	case <-opened:
		// Successfully opened. onOpen is no longer needed, but others are.
		ws.Set("onopen", js.Null())
		ch.onOpen.Release()
		return ch, nil
	case <-ch.closed:
		errMu.Lock()
		defer errMu.Unlock()
		cleanup()
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, errors.New("websocket closed before opening")
	case <-ctx.Done():
		ws.Call("close")
		cleanup()
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
	// 1. Null out callbacks FIRST to prevent "call to released function"
	w.ws.Set("onmessage", js.Null())
	w.ws.Set("onclose", js.Null())
	w.ws.Set("onerror", js.Null())
	w.ws.Set("onopen", js.Null())

	// 2. Release Go functions
	w.onMessage.Release()
	w.onClose.Release()
	w.onError.Release()
	// onOpen might have been released already, but double-release is usually a panic in Go WASM
	// so we check if it was initialized or manage it better.
	// Actually, in dialSignaling we release it on success.

	w.ws.Call("close")
	return nil
}

func (w *wasmSignalingChannel) IsConnected() bool {
	return w.ws.Get("readyState").Int() == 1 // OPEN
}
