//go:build !js || !wasm
// +build !js !wasm

package utils

// redirectLogToBridge is a no-op on native platforms
func (l *Logger) redirectLogToBridge(level LogLevel, logLine string) bool {
	// Native tests use stdout/stderr which is already handled by l.output.Write
	return false
}
