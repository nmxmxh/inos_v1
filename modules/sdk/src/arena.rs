use crate::registry::crc32c_hash;
use crate::sab::SafeSAB;
use crate::signal::Epoch;

// Arena allocator interface for Rust modules
// Communicates with Go-side hybrid allocator via Epoch signaling

const ARENA_REQUEST_QUEUE: usize = crate::layout::OFFSET_ARENA_REQUEST_QUEUE;
const ARENA_RESPONSE_QUEUE: usize = crate::layout::OFFSET_ARENA_RESPONSE_QUEUE;
const REQUEST_ENTRY_SIZE: usize = crate::layout::ARENA_QUEUE_ENTRY_SIZE;
const MAX_REQUESTS: usize = crate::layout::MAX_ARENA_REQUESTS;

// Epoch indices
const IDX_ARENA_REQUEST: u32 = 12; // Dedicated System Reserved index

pub struct ArenaAllocator {
    sab: SafeSAB,
    request_epoch: Epoch,
    next_request_id: u64,
}

impl ArenaAllocator {
    pub fn new(sab: SafeSAB) -> Self {
        Self {
            request_epoch: Epoch::new(sab.clone(), IDX_ARENA_REQUEST),
            sab,
            next_request_id: 1,
        }
    }

    /// Allocate memory from arena
    pub fn allocate(&mut self, size: u32, owner: &str) -> Result<u32, ArenaError> {
        let request = AllocationRequest {
            id: self.next_request_id,
            size,
            owner_hash: crc32c_hash(owner.as_bytes()),
            priority: 0,
            flags: 0,
            reserved: [0; 2],
        };

        self.next_request_id += 1;

        // Write request
        self.write_request(&request)?;

        // Signal Go
        self.request_epoch.increment();

        // Wait for response (1 second timeout)
        self.wait_for_response(request.id, 1000)
    }

    /// Allocate with flags
    pub fn allocate_with_flags(
        &mut self,
        size: u32,
        owner: &str,
        flags: u8,
    ) -> Result<u32, ArenaError> {
        let request = AllocationRequest {
            id: self.next_request_id,
            size,
            owner_hash: crc32c_hash(owner.as_bytes()),
            priority: 0,
            flags,
            reserved: [0; 2],
        };

        self.next_request_id += 1;

        self.write_request(&request)?;
        self.request_epoch.increment();
        self.wait_for_response(request.id, 1000)
    }

    /// Free memory
    pub fn free(&mut self, offset: u32) -> Result<(), ArenaError> {
        // Write free request (use size=0 to indicate free)
        let request = AllocationRequest {
            id: self.next_request_id,
            size: 0,
            owner_hash: offset, // Reuse field for offset
            priority: 0,
            flags: 0xFF, // Special flag for free
            reserved: [0; 2],
        };

        self.next_request_id += 1;

        self.write_request(&request)?;
        self.request_epoch.increment();

        Ok(())
    }

    fn write_request(&self, request: &AllocationRequest) -> Result<(), ArenaError> {
        // Find next slot in circular queue
        let slot = (request.id % MAX_REQUESTS as u64) as usize;
        let offset = ARENA_REQUEST_QUEUE + slot * REQUEST_ENTRY_SIZE;

        // Write request struct
        let bytes = unsafe {
            std::slice::from_raw_parts(
                request as *const _ as *const u8,
                std::mem::size_of::<AllocationRequest>(),
            )
        };

        self.sab
            .write(offset, bytes)
            .map_err(ArenaError::WriteError)?;

        Ok(())
    }

    fn wait_for_response(&self, request_id: u64, timeout_ms: u32) -> Result<u32, ArenaError> {
        let slot = (request_id % MAX_REQUESTS as u64) as usize;
        let offset = ARENA_RESPONSE_QUEUE + slot * REQUEST_ENTRY_SIZE;

        // Poll for response
        for _ in 0..timeout_ms {
            // Check if response ID matches
            let response_bytes = self.sab.read(offset, 16).map_err(ArenaError::ReadError)?;

            let response_id = u64::from_le_bytes([
                response_bytes[0],
                response_bytes[1],
                response_bytes[2],
                response_bytes[3],
                response_bytes[4],
                response_bytes[5],
                response_bytes[6],
                response_bytes[7],
            ]);

            if response_id == request_id {
                // Read offset
                let result_offset = u32::from_le_bytes([
                    response_bytes[8],
                    response_bytes[9],
                    response_bytes[10],
                    response_bytes[11],
                ]);

                if result_offset == 0 {
                    return Err(ArenaError::OutOfMemory);
                }

                return Ok(result_offset);
            }

            // Yield
            std::hint::spin_loop();
        }

        Err(ArenaError::Timeout)
    }
}

#[repr(C, packed)]
pub struct AllocationRequest {
    pub id: u64,
    pub size: u32,
    pub owner_hash: u32,
    pub priority: u8,
    pub flags: u8,
    pub reserved: [u8; 2],
}

#[derive(Debug)]
pub enum ArenaError {
    WriteError(String),
    ReadError(String),
    OutOfMemory,
    Timeout,
}

impl std::fmt::Display for ArenaError {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        match self {
            ArenaError::WriteError(e) => write!(f, "Write error: {}", e),
            ArenaError::ReadError(e) => write!(f, "Read error: {}", e),
            ArenaError::OutOfMemory => write!(f, "Out of memory"),
            ArenaError::Timeout => write!(f, "Request timeout"),
        }
    }
}

impl std::error::Error for ArenaError {}
