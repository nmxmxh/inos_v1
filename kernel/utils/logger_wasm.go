//go:build js && wasm
// +build js,wasm

package utils

import "syscall/js"

// redirectLogToBridge redirects kernel logs to the browser's JS console
func (l *Logger) redirectLogToBridge(level LogLevel, logLine string) {
	console := js.Global().Get("console")
	if !isValueNil(console) {
		method := "log"
		switch level {
		case DEBUG:
			method = "debug"
		case INFO:
			method = "info"
		case WARN:
			method = "warn"
		case ERROR, FATAL:
			method = "error"
		}
		console.Call(method, logLine)
	}
}

// isValueNil helper for js.Value
func isValueNil(v js.Value) bool {
	return v.Type() == js.TypeNull || v.Type() == js.TypeUndefined
}
