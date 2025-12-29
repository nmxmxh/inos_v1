# TypeScript Proto Generation for INOS Frontend

This directory contains TypeScript types and utilities generated from Cap'n Proto schemas.

## Generation

TypeScript code is generated using `ts-proto`:

```bash
yarn proto
```

This will:
1. Read all `.capnp` schemas from `../protocols/schemas`
2. Generate TypeScript interfaces and utilities
3. Output to `./bridge/generated`

## Usage

```typescript
import { Envelope, Metadata } from './bridge/generated/base/v1/base';

// Type-safe message creation
const envelope: Envelope = {
  id: 'event-123',
  type: 'compute:physics:v1:request',
  timestamp: Date.now() * 1000000, // nanoseconds
  metadata: {
    userId: 'user-456',
    deviceId: 'device-789',
    creditLedgerId: 'ledger-abc',
  },
  payload: new Uint8Array(),
};
```

## Benefits

1. **Type Safety**: Full TypeScript types for all protocol messages
2. **Autocomplete**: IDE support for all fields
3. **Validation**: Compile-time checking of message structure
4. **Zero-Copy**: Direct Uint8Array handling for performance

## Integration with Kernel

The bridge uses these types to communicate with the Go WASM kernel:

```typescript
// Send typed message to kernel
function sendToKernel(envelope: Envelope) {
  const bytes = Envelope.encode(envelope).finish();
  kernel.dispatch(envelope.type, bytes);
}

// Receive typed message from kernel
function receiveFromKernel(bytes: Uint8Array): Envelope {
  return Envelope.decode(bytes);
}
```

## Schema Updates

When protocol schemas change:

1. Update `.capnp` files in `protocols/schemas/`
2. Run `yarn proto` to regenerate TypeScript
3. Fix any type errors in the codebase
4. Commit both schema and generated code

## Files

- `base/v1/base.ts` - Core envelope and metadata types
- `system/v1/orchestration.ts` - Lifecycle and health types
- `compute/v1/capsule.ts` - Job and result types
- `io/v1/sensor.ts` - Sensor data types
- `io/v1/actor.ts` - Actor command types
- `economy/v1/ledger.ts` - Credit and transaction types
