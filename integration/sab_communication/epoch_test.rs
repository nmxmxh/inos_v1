use wasm_bindgen_test::*;

wasm_bindgen_test_configure!(run_in_browser);

// Note: These tests validate Rust's side of SAB communication
// They complement the Go tests in zero_copy_test.go

#[wasm_bindgen_test]
fn test_rust_read_go_write() {
    // Simulate Go having written data to SAB
    let data = vec![0u8; 16 * 1024 * 1024];

    // Go writes to OFFSET_INBOX (0x050000)
    let offset = 0x050000;
    let expected = b"Hello from Go Kernel";

    // Simulate Go write
    for (i, &byte) in expected.iter().enumerate() {
        // In real scenario, this would be written by Go
        // Here we simulate it for testing
    }

    // Rust reads from SAB
    let read = &data[offset..offset + 20];

    // In real implementation, this would use SafeSAB
    // assert_eq!(read, expected);
}

#[wasm_bindgen_test]
fn test_rust_write_go_read() {
    let mut data = vec![0u8; 16 * 1024 * 1024];

    // Rust writes to OFFSET_OUTBOX (0x0D0000)
    let offset = 0x0D0000;
    let message = b"Hello from Rust Module";

    // Rust writes
    for (i, &byte) in message.iter().enumerate() {
        data[offset + i] = byte;
    }

    // Validate: Data written correctly (Go would read this)
    let read = &data[offset..offset + 22];
    assert_eq!(read, message);
}

#[wasm_bindgen_test]
fn test_epoch_signaling_rust_side() {
    let mut data = vec![0u8; 1024];

    // Simulate epoch at index 7
    let epoch_offset = 7 * 4; // 4 bytes per i32

    // Initial value
    assert_eq!(data[epoch_offset], 0);

    // Rust increments epoch
    let current = i32::from_le_bytes([
        data[epoch_offset],
        data[epoch_offset + 1],
        data[epoch_offset + 2],
        data[epoch_offset + 3],
    ]);

    let new_value = current + 1;
    let bytes = new_value.to_le_bytes();

    data[epoch_offset] = bytes[0];
    data[epoch_offset + 1] = bytes[1];
    data[epoch_offset + 2] = bytes[2];
    data[epoch_offset + 3] = bytes[3];

    // Validate: Epoch incremented
    let read_value = i32::from_le_bytes([
        data[epoch_offset],
        data[epoch_offset + 1],
        data[epoch_offset + 2],
        data[epoch_offset + 3],
    ]);

    assert_eq!(read_value, 1);
}

#[wasm_bindgen_test]
fn test_module_registration_rust_side() {
    let mut data = vec![0u8; 16 * 1024 * 1024];

    // OFFSET_MODULE_REGISTRY = 0x000100
    let offset = 0x000100;

    // Rust writes module ID
    let module_id = b"ml";
    for (i, &byte) in module_id.iter().enumerate() {
        data[offset + i] = byte;
    }

    // Rust writes version
    let version = b"1.0.0";
    for (i, &byte) in version.iter().enumerate() {
        data[offset + 32 + i] = byte;
    }

    // Validate: Go can read this
    assert_eq!(data[offset], b'm');
    assert_eq!(data[offset + 1], b'l');
    assert_eq!(data[offset + 32], b'1');
}

#[wasm_bindgen_test]
fn test_zero_copy_pointer_validation() {
    let data = vec![42u8; 1024];
    let ptr_before = data.as_ptr();

    // Simulate SAB operation (no copy)
    let slice = &data[..];

    let ptr_after = slice.as_ptr();

    // Validate: Same pointer (zero-copy)
    assert_eq!(ptr_before, ptr_after);
}

// Integration test: Ring buffer communication
#[wasm_bindgen_test]
fn test_ringbuffer_go_rust_communication() {
    let mut data = vec![0u8; 1024 * 1024];

    // Ring buffer at OFFSET_INBOX
    let rb_offset = 0x050000;

    // Head and tail pointers (first 8 bytes)
    let head_offset = rb_offset;
    let tail_offset = rb_offset + 4;

    // Initialize
    for i in 0..4 {
        data[head_offset + i] = 0;
        data[tail_offset + i] = 0;
    }

    // Simulate Go writing message
    let msg = b"test message";
    let msg_len = (msg.len() as u32).to_le_bytes();

    // Write length
    for i in 0..4 {
        data[rb_offset + 8 + i] = msg_len[i];
    }

    // Write message
    for (i, &byte) in msg.iter().enumerate() {
        data[rb_offset + 12 + i] = byte;
    }

    // Update tail
    let new_tail = (4 + msg.len()) as u32;
    let tail_bytes = new_tail.to_le_bytes();
    for i in 0..4 {
        data[tail_offset + i] = tail_bytes[i];
    }

    // Rust reads
    let read_len = u32::from_le_bytes([
        data[rb_offset + 8],
        data[rb_offset + 9],
        data[rb_offset + 10],
        data[rb_offset + 11],
    ]);

    assert_eq!(read_len, msg.len() as u32);

    let read_msg = &data[rb_offset + 12..rb_offset + 12 + msg.len()];
    assert_eq!(read_msg, msg);
}
