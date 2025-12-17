package wasm

import (
	"github.com/wasmerio/wasmer-go/wasmer"
)

// Execute runs a WASM module with the given input and returns the result.
func Execute(wasmBytes, input []byte) ([]byte, error) {
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)
	module, err := wasmer.NewModule(store, wasmBytes)
	if err != nil {
		return nil, err
	}
	instance, err := wasmer.NewInstance(module, wasmer.NewImportObject())
	if err != nil {
		return nil, err
	}
	mainFunc, err := instance.Exports.GetFunction("main")
	if err != nil {
		return nil, err
	}
	result, err := mainFunc(input)
	if err != nil {
		return nil, err
	}
	if bytes, ok := result.([]byte); ok {
		return bytes, nil
	}
	return nil, nil
}
