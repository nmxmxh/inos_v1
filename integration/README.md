# INOS Integration Tests

## Overview

This directory contains **polyglot integration tests** that validate cross-language interactions between:
- **Go** (Kernel)
- **Rust** (Modules)  
- **JavaScript** (Frontend)

## Directory Structure

```
integration/
├── go.mod                     # Go module (imports kernel)
├── Cargo.toml                 # Rust crate (imports SDK)
├── package.json               # JS test runner (Playwright)
├── sab_communication/         # SAB communication tests
│   ├── zero_copy_test.go      # Go ↔ Rust SAB tests
│   └── epoch_test.rs          # Rust epoch signaling tests
└── e2e/                       # End-to-end tests
    └── ml_pipeline.spec.js    # Full stack ML pipeline test
```

## Running Tests

### SAB Communication Tests (Go)

```bash
cd integration
go test ./sab_communication/... -v
```

**Tests**:
- `TestGoWriteRustRead` - Go writes to SAB, validates Rust can read
- `TestRustWriteGoRead` - Rust writes to SAB, Go reads
- `TestZeroCopyValidation` - Ensures no data copying
- `TestModuleRegistration` - Module registry via SAB
- `TestEpochSignaling` - Epoch-based reactive mutation

**Benchmarks**:
```bash
go test ./sab_communication/... -bench=. -benchmem
```

Targets:
- SAB Read: < 10ns
- SAB Write: < 20ns
- Epoch Increment: < 5ns

### SAB Communication Tests (Rust)

```bash
cd integration
cargo test
```

Or for WASM browser tests:
```bash
wasm-pack test --headless --firefox
```

### End-to-End Tests (Playwright)

```bash
cd integration
npm install
npm test
```

**Tests**:
- ML Inference Pipeline
- P2P Mesh Initialization
- SAB Zero-Copy Validation

## Test Scenarios

### 1. Go → Rust Communication

**Flow**:
1. Go kernel creates SAB
2. Go writes data to `OFFSET_INBOX`
3. Go increments `IDX_INBOX_DIRTY` epoch
4. Rust module detects epoch change
5. Rust reads data from SAB (zero-copy)

**Validated**:
- ✅ Zero-copy data transfer
- ✅ Epoch signaling
- ✅ Correct memory offsets

### 2. Rust → Go Communication

**Flow**:
1. Rust module writes to `OFFSET_OUTBOX`
2. Rust increments `IDX_OUTBOX_DIRTY` epoch
3. Go kernel detects epoch change
4. Go reads data from SAB (zero-copy)

**Validated**:
- ✅ Reactive mutation pattern
- ✅ Ring buffer communication
- ✅ Module registration

### 3. Full Stack E2E

**Flow**:
1. Frontend loads
2. Kernel initializes
3. Modules register via SAB
4. ML model loads from P2P mesh
5. Inference runs
6. Credits minted

**Validated**:
- ✅ Complete system integration
- ✅ P2P mesh functionality
- ✅ Economic system

## CI/CD Integration

These tests run automatically on every commit via GitHub Actions.

See: `.github/workflows/tests.yml`

## Adding New Tests

### Go Tests

Add to `sab_communication/`:
```go
func TestMyFeature(t *testing.T) {
    // Test implementation
}
```

### Rust Tests

Add to `sab_communication/epoch_test.rs`:
```rust
#[wasm_bindgen_test]
fn test_my_feature() {
    // Test implementation
}
```

### E2E Tests

Add to `e2e/`:
```javascript
test('my feature', async ({ page }) => {
    // Test implementation
});
```

## Troubleshooting

### Go Tests Fail

```bash
# Ensure kernel module is accessible
cd integration
go mod tidy
```

### Rust Tests Fail

```bash
# Ensure SDK is built
cd ../modules
cargo build -p sdk
```

### E2E Tests Fail

```bash
# Ensure frontend is running
cd ../frontend
yarn dev

# In another terminal
cd ../integration
npm test
```

## Performance Targets

| Operation | Target | Test |
|-----------|--------|------|
| SAB Read | < 10ns | `BenchmarkSABRead` |
| SAB Write | < 20ns | `BenchmarkSABWrite` |
| Epoch Increment | < 5ns | `BenchmarkEpochIncrement` |
| Module Load | < 100ms | E2E test |
| ML Inference | < 500ms/token | E2E test |
