use crate::sab::SafeSAB;
use std::sync::atomic::{AtomicU32, Ordering};

/// Global atomic to store the module ID assigned by the kernel
static MODULE_ID: AtomicU32 = AtomicU32::new(0);

const IDENTITY_ENTRY_SIZE: usize = 128;

pub struct IdentityContext {
    node_id: String,
    module_id: u32,
}

pub struct IdentityEntry {
    pub did: String,
    pub public_key: Vec<u8>,
    pub status: u8,
}

impl IdentityContext {
    pub fn new(node_id: String, module_id: u32) -> Self {
        Self { node_id, module_id }
    }

    pub fn node_id(&self) -> &str {
        &self.node_id
    }

    pub fn module_id(&self) -> u32 {
        self.module_id
    }
}

pub struct IdentityRegistry {
    sab: SafeSAB,
}

impl IdentityRegistry {
    pub fn new(sab: SafeSAB) -> Self {
        Self { sab }
    }

    pub fn get_entry(&self, index: usize) -> Result<IdentityEntry, String> {
        let offset = SafeSAB::OFFSET_IDENTITY_REGISTRY + (index * IDENTITY_ENTRY_SIZE);
        let data = self.sab.read(offset, IDENTITY_ENTRY_SIZE)?;

        let did = Self::parse_did(&data[0..64]);
        let public_key = data[64..97].to_vec();
        let status = data[97];

        Ok(IdentityEntry {
            did,
            public_key,
            status,
        })
    }

    fn parse_did(data: &[u8]) -> String {
        let len = data.iter().position(|&b| b == 0).unwrap_or(data.len());
        String::from_utf8_lossy(&data[..len]).to_string()
    }
}

pub fn set_module_id(id: u32) {
    MODULE_ID.store(id, Ordering::SeqCst);
}

pub fn get_module_id() -> u32 {
    MODULE_ID.load(Ordering::SeqCst)
}
