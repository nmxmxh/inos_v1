use crate::protocols::resource;
use crate::protocols::syscall;
use crate::sab::SafeSAB;

use capnp::message::{Builder, ReaderOptions};
use capnp::serialize_packed;

use futures_timer::Delay;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Duration;

/// Global atomic counter for unique Call IDs within this module instance
static CALL_ID_COUNTER: AtomicU64 = AtomicU64::new(1);

/// Production Syscall Client
pub struct SyscallClient;

pub enum HostPayload<'a> {
    Inline(&'a [u8]),
    SabRef { offset: u32, size: u32 },
}

pub enum HostResponse {
    Inline {
        data: Vec<u8>,
        custom: Vec<u8>,
    },
    SabRef {
        offset: u32,
        size: u32,
        custom: Vec<u8>,
    },
}

impl SyscallClient {
    /// Send a fetch_chunk request and await the response (Async)
    pub async fn fetch_chunk(
        sab: &SafeSAB,
        hash: &str,
        dest_offset: u64,
        dest_size: u32,
    ) -> Result<Vec<u8>, String> {
        let call_id = CALL_ID_COUNTER.fetch_add(1, Ordering::Relaxed);

        // 1. Build Request
        let mut message = Builder::new_default();
        {
            let mut root = message.init_root::<syscall::syscall::message::Builder>();
            let mut header = root.reborrow().init_header();
            header.set_magic(0x53424142); // "SABS"
            header.set_call_id(call_id);
            header.set_source_module_id(crate::identity::get_module_id());
            header.set_opcode(syscall::syscall::Opcode::FetchChunk);

            // Populate Metadata
            let mut meta = header.init_metadata();
            meta.set_module_id(crate::identity::get_module_id());
            if let Some(device_id) = crate::identity::get_device_id() {
                meta.set_device_id(device_id);
            }
            if let Some(did) = crate::identity::get_did() {
                meta.set_user_id(did);
            } else if let Some(node_id) = crate::identity::get_node_id() {
                meta.set_user_id(node_id);
            }
            meta.set_version(1);
            // Other fields (user_id, device_id, etc.) will be populated when available in context

            let body = root.init_body();
            let mut fetch = body.init_fetch_chunk();
            fetch.set_hash(hash.as_bytes());
            fetch.set_destination_offset(dest_offset);
            fetch.set_destination_size(dest_size);
        }

        // 2. Serialize to Bytes
        let mut request_bytes = Vec::new();
        serialize_packed::write_message(&mut request_bytes, &message).map_err(|e| e.to_string())?;

        // 3. Send to Outbox & Signal
        Self::send_raw(sab, &request_bytes)?;

        // 4. Await Response (Poll Inbox)
        // In a real reactor model, we would register a waker.
        // For v2.0, we use an efficient async sleep-poll loop.
        Self::poll_response(sab, call_id).await
    }

    /// Send a store_chunk request and await response
    pub async fn store_chunk(
        sab: &SafeSAB,
        hash: &str,
        src_offset: u64,
        size: u32,
    ) -> Result<u16, String> {
        let call_id = CALL_ID_COUNTER.fetch_add(1, Ordering::Relaxed);

        let mut message = Builder::new_default();
        {
            let mut root = message.init_root::<syscall::syscall::message::Builder>();
            let mut header = root.reborrow().init_header();
            header.set_magic(0x53424142);
            header.set_call_id(call_id);
            header.set_source_module_id(crate::identity::get_module_id());
            header.set_opcode(syscall::syscall::Opcode::StoreChunk);

            // Populate Metadata
            let mut meta = header.init_metadata();
            meta.set_module_id(crate::identity::get_module_id());
            if let Some(device_id) = crate::identity::get_device_id() {
                meta.set_device_id(device_id);
            }
            if let Some(did) = crate::identity::get_did() {
                meta.set_user_id(did);
            } else if let Some(node_id) = crate::identity::get_node_id() {
                meta.set_user_id(node_id);
            }
            meta.set_version(1);

            let body = root.init_body();
            let mut store = body.init_store_chunk();
            store.set_hash(hash.as_bytes());
            store.set_source_offset(src_offset);
            store.set_size(size);
        }

        let mut request_bytes = Vec::new();
        serialize_packed::write_message(&mut request_bytes, &message).map_err(|e| e.to_string())?;

        Self::send_raw(sab, &request_bytes)?;

        // Response should contain StoreChunkResult with replicas count
        let response_bytes = Self::poll_response(sab, call_id).await?;

        // Parse Response
        let reader = serialize_packed::read_message(&mut &response_bytes[..], ReaderOptions::new())
            .map_err(|e| format!("Invalid response format: {}", e))?;

        let root = reader
            .get_root::<syscall::syscall::response::Reader>()
            .map_err(|e| e.to_string())?;

        // Correctly handle the result union
        // get_result() returns CapnpResult<result::Reader>
        let result_reader = root.get_result().map_err(|e| e.to_string())?;

        match result_reader.which().map_err(|e| e.to_string())? {
            syscall::syscall::result::Which::StoreChunk(res) => {
                let reader = res.map_err(|e| e.to_string())?;
                Ok(reader.get_replicas())
            }
            _ => {
                Err("Unexpected result type for StoreChunk (expected StoreChunkResult)".to_string())
            }
        }
    }

    /// Send a message to a target peer via the kernel
    pub async fn send_message(
        sab: &SafeSAB,
        target_id: &str,
        payload: &[u8],
    ) -> Result<bool, String> {
        let call_id = CALL_ID_COUNTER.fetch_add(1, Ordering::Relaxed);

        let mut message = Builder::new_default();
        {
            let mut root = message.init_root::<syscall::syscall::message::Builder>();
            let mut header = root.reborrow().init_header();
            header.set_magic(0x53424142);
            header.set_call_id(call_id);
            header.set_source_module_id(crate::identity::get_module_id());
            header.set_opcode(syscall::syscall::Opcode::SendMessage);

            // Populate Metadata
            let mut meta = header.init_metadata();
            meta.set_module_id(crate::identity::get_module_id());
            if let Some(device_id) = crate::identity::get_device_id() {
                meta.set_device_id(device_id);
            }
            if let Some(did) = crate::identity::get_did() {
                meta.set_user_id(did);
            } else if let Some(node_id) = crate::identity::get_node_id() {
                meta.set_user_id(node_id);
            }
            meta.set_version(1);

            let body = root.init_body();
            let mut send = body.init_send_message();
            send.set_target_id(target_id);
            send.set_payload(payload);
        }

        let mut request_bytes = Vec::new();
        serialize_packed::write_message(&mut request_bytes, &message).map_err(|e| e.to_string())?;

        Self::send_raw(sab, &request_bytes)?;

        let response_bytes = Self::poll_response(sab, call_id).await?;

        // Parse Response
        let reader = serialize_packed::read_message(&mut &response_bytes[..], ReaderOptions::new())
            .map_err(|e| format!("Invalid response format: {}", e))?;

        let root = reader
            .get_root::<syscall::syscall::response::Reader>()
            .map_err(|e| e.to_string())?;

        let result_reader = root.get_result().map_err(|e| e.to_string())?;

        match result_reader.which().map_err(|e| e.to_string())? {
            syscall::syscall::result::Which::SendMessage(res) => {
                let reader = res.map_err(|e| e.to_string())?;
                Ok(reader.get_delivered())
            }
            _ => Err("Unexpected result type for SendMessage".to_string()),
        }
    }

    /// Internal: Write bytes to SAB Outbox and Signal Kernel
    /// Internal: Write bytes to SAB Outbox and Signal Kernel
    /// This method is protected by an Atomic Swapping logic on the SAB to ensure thread safety
    /// Matches Kernel 'sab_bridge.go' expectation of a single slotted message.
    pub fn send_raw(sab: &SafeSAB, message_bytes: &[u8]) -> Result<(), String> {
        if message_bytes.len() > crate::layout::SIZE_OUTBOX {
            return Err("Message too large for Outbox".to_string());
        }

        // ACQUIRE OUTBOX LOCK
        // We use index IDX_OUTBOX_MUTEX in AtomicFlags as a Mutex for the Outbox
        let flags = sab.int32_view(
            crate::layout::OFFSET_ATOMIC_FLAGS,
            crate::layout::SIZE_ATOMIC_FLAGS / 4,
        )?;

        let mut backoff = 1;
        loop {
            // compare_exchange(expected=0, replacement=1)
            if crate::js_interop::atomic_compare_exchange(
                &flags,
                crate::layout::IDX_OUTBOX_MUTEX,
                0,
                1,
            ) == 0
            {
                break; // Acquired
            }
            // Locked. Spin.
            std::hint::spin_loop();
            for _ in 0..backoff {
                std::hint::spin_loop();
            }
            backoff = std::cmp::min(backoff * 2, 64);
        }

        // Write to Outbox Slot
        let write_result = sab.write(crate::layout::OFFSET_SAB_OUTBOX, message_bytes);

        if let Err(e) = write_result {
            // Unlock and return error
            crate::js_interop::atomic_store(&flags, crate::layout::IDX_OUTBOX_MUTEX, 0);
            return Err(e);
        }

        // Signal Kernel (Consistency: Kernel watches IDX_OUTBOX_KERNEL_DIRTY at index 22)
        // See kernel/threads/supervisor/sab_bridge.go: ReadOutboxSequence -> offset 8
        crate::js_interop::atomic_add(&flags, crate::layout::IDX_OUTBOX_KERNEL_DIRTY, 1);

        // RELEASE OUTBOX LOCK
        crate::js_interop::atomic_store(&flags, crate::layout::IDX_OUTBOX_MUTEX, 0);

        Ok(())
    }

    /// Internal: Poll SAB Inbox for matching Call ID
    /// Uses exponential backoff to be friendly to the CPU/Runtime.
    async fn poll_response(sab: &SafeSAB, expected_call_id: u64) -> Result<Vec<u8>, String> {
        let mut attempts = 0;
        let max_attempts = 5000; // 5000 * 1ms = 5s timeout
        let base_delay_micros = 1000;

        loop {
            if attempts >= max_attempts {
                return Err("Syscall timed out".to_string());
            }

            // 1. Peek Inbox for Header
            // We assume the kernel writes the response at the start of the Inbox for now (Simple Ring/Slot later)
            // Ideally syscall.capnp Request/Response are symmetric.

            // Try to read generic response structure
            match Self::try_read_inbox(sab, expected_call_id) {
                Ok(Some(data)) => return Ok(data),
                Ok(None) => {} // Not found yet or different ID
                Err(e) => return Err(e),
            }

            // 2. Wait
            Delay::new(Duration::from_micros(base_delay_micros)).await;
            attempts += 1;
        }
    }

    /// Check Inbox for our response
    fn try_read_inbox(sab: &SafeSAB, expected_call_id: u64) -> Result<Option<Vec<u8>>, String> {
        // Read headers first? Or just try to decode?
        // Reading full 512KB is expensive.
        // Let's read first 4KB which should cover most headers.
        let peek_size = 4096;
        let bytes = sab.read(crate::layout::OFFSET_SAB_INBOX, peek_size)?;

        // Attempt to read Cap'n Proto message from stream
        let mut slice = &bytes[..];

        // We use try_read_message because the message might be incomplete or empty (all zeros)
        // But serialize_packed::read_message will error on zeros.
        // We need to check if the first word is non-zero (primitive check)
        if bytes[0] == 0 && bytes[1] == 0 && bytes[2] == 0 && bytes[3] == 0 {
            return Ok(None); // Empty inbox
        }

        let reader = match serialize_packed::read_message(&mut slice, ReaderOptions::new()) {
            Ok(r) => r,
            Err(_) => return Ok(None), // Valid message not yet ready
        };

        let response = reader
            .get_root::<syscall::syscall::response::Reader>()
            .map_err(|e| e.to_string())?;

        if response.get_call_id() == expected_call_id {
            // Found it!
            // If the message was larger than 4KB, we would need to re-read.
            // But for v2.0, syscall responses are small (status, or small result).
            // Large data is zero-copied to destinationOffset.

            // Return validation
            match response.get_status().map_err(|e| e.to_string())? {
                syscall::syscall::Status::Success => {
                    // We return the raw bytes so the caller can re-parse the specific result union
                    // Optimization: Pass the reader up? Can't due to lifetime.
                    // Just return valid bytes.
                    Ok(Some(bytes)) // Return the 4KB chunk, caller will re-parse.
                }
                syscall::syscall::Status::Pending => Ok(None),
                start => Err(format!("Syscall failed with status: {:?}", start)),
            }
        } else {
            Ok(None) // Inbox contains someone else's message or old message
        }
    }

    /// Send a host call request (browser API proxy) and await response.
    pub async fn host_call(
        sab: &SafeSAB,
        service: &str,
        payload: HostPayload<'_>,
        custom: Option<&[u8]>,
    ) -> Result<HostResponse, String> {
        let call_id = CALL_ID_COUNTER.fetch_add(1, Ordering::Relaxed);

        let mut message = Builder::new_default();
        {
            let mut root = message.init_root::<syscall::syscall::message::Builder>();
            let mut header = root.reborrow().init_header();
            header.set_magic(0x53424142);
            header.set_call_id(call_id);
            header.set_source_module_id(crate::identity::get_module_id());
            header.set_opcode(syscall::syscall::Opcode::HostCall);

            let mut meta = header.init_metadata();
            meta.set_module_id(crate::identity::get_module_id());
            if let Some(device_id) = crate::identity::get_device_id() {
                meta.set_device_id(device_id);
            }
            if let Some(did) = crate::identity::get_did() {
                meta.set_user_id(did);
            } else if let Some(node_id) = crate::identity::get_node_id() {
                meta.set_user_id(node_id);
            }
            meta.set_version(1);

            let body = root.init_body();
            let mut host_call = body.init_host_call();
            host_call.set_service(service);
            let mut req_payload = host_call.init_payload();
            fill_resource_payload(&mut req_payload, payload, custom)?;
        }

        let mut request_bytes = Vec::new();
        serialize_packed::write_message(&mut request_bytes, &message).map_err(|e| e.to_string())?;

        Self::send_raw(sab, &request_bytes)?;

        let response_bytes = Self::poll_response(sab, call_id).await?;

        let reader = serialize_packed::read_message(&mut &response_bytes[..], ReaderOptions::new())
            .map_err(|e| format!("Invalid response format: {}", e))?;

        let root = reader
            .get_root::<syscall::syscall::response::Reader>()
            .map_err(|e| e.to_string())?;

        let result_reader = root.get_result().map_err(|e| e.to_string())?;
        match result_reader.which().map_err(|e| e.to_string())? {
            syscall::syscall::result::Which::HostCall(res) => {
                let reader = res.map_err(|e| e.to_string())?;
                let payload = reader.get_payload().map_err(|e| e.to_string())?;
                parse_resource_payload(payload)
            }
            _ => Err("Unexpected result type for HostCall".to_string()),
        }
    }
}

fn fill_resource_payload(
    payload: &mut resource::resource::Builder,
    data: HostPayload<'_>,
    custom: Option<&[u8]>,
) -> Result<(), String> {
    payload.set_compression(resource::resource::Compression::None);
    payload.set_encryption(resource::resource::Encryption::None);

    let mut alloc = payload.reborrow().init_allocation();
    alloc.set_lifetime(resource::resource::allocation::Lifetime::Ephemeral);

    match data {
        HostPayload::Inline(bytes) => {
            alloc.set_type(resource::resource::allocation::Type::Heap);
            payload.set_inline(bytes);
            payload.set_raw_size(bytes.len() as u32);
            payload.set_wire_size(bytes.len() as u32);
        }
        HostPayload::SabRef { offset, size } => {
            alloc.set_type(resource::resource::allocation::Type::Sab);
            let mut sab_ref = payload.reborrow().init_sab_ref();
            sab_ref.set_offset(offset);
            sab_ref.set_size(size);
            payload.set_raw_size(size);
            payload.set_wire_size(size);
        }
    }

    if let Some(custom_bytes) = custom {
        let mut meta = payload.reborrow().init_metadata();
        meta.set_custom(custom_bytes);
    }

    Ok(())
}

fn parse_resource_payload(payload: resource::resource::Reader) -> Result<HostResponse, String> {
    let custom = payload
        .get_metadata()
        .and_then(|m| m.get_custom())
        .map(|v| v.to_vec())
        .unwrap_or_default();

    match payload.which().map_err(|e| e.to_string())? {
        resource::resource::Which::Inline(data) => Ok(HostResponse::Inline {
            data: data.map_err(|e| e.to_string())?.to_vec(),
            custom,
        }),
        resource::resource::Which::SabRef(ref_reader) => {
            let ref_reader = ref_reader.map_err(|e| e.to_string())?;
            Ok(HostResponse::SabRef {
                offset: ref_reader.get_offset(),
                size: ref_reader.get_size(),
                custom,
            })
        }
        resource::resource::Which::Shards(_) => {
            Err("HostCall response does not support shards".to_string())
        }
    }
}
