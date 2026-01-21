use crate::sab::SafeSAB;
use once_cell::sync::OnceCell;
use std::sync::atomic::{AtomicU32, Ordering};

/// Global atomic to store the module ID assigned by the kernel
static MODULE_ID: AtomicU32 = AtomicU32::new(0);
static NODE_ID: OnceCell<String> = OnceCell::new();
static DEVICE_ID: OnceCell<String> = OnceCell::new();
static DID: OnceCell<String> = OnceCell::new();

const IDENTITY_ENTRY_SIZE: usize = 128;

pub struct IdentityContext {
    node_id: String,
    module_id: u32,
}

pub struct IdentityEntry {
    pub did: String,
    pub public_key: Vec<u8>,
    pub status: u8,
    pub account_offset: u32,
    pub social_offset: u32,
    pub recovery_threshold: u8,
    pub total_shares: u8,
    pub tier: u8,
    pub flags: u8,
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
        let account_offset = u32::from_le_bytes([data[98], data[99], data[100], data[101]]);
        let social_offset = u32::from_le_bytes([data[102], data[103], data[104], data[105]]);
        let recovery_threshold = data[106];
        let total_shares = data[107];
        let tier = data[108];
        let flags = data[109];

        Ok(IdentityEntry {
            did,
            public_key,
            status,
            account_offset,
            social_offset,
            recovery_threshold,
            total_shares,
            tier,
            flags,
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

pub fn set_node_id(id: &str) {
    let _ = NODE_ID.set(id.to_string());
}

pub fn set_device_id(id: &str) {
    let _ = DEVICE_ID.set(id.to_string());
}

pub fn set_did(id: &str) {
    let _ = DID.set(id.to_string());
}

pub fn get_node_id() -> Option<&'static str> {
    NODE_ID.get().map(String::as_str)
}

pub fn get_device_id() -> Option<&'static str> {
    DEVICE_ID.get().map(String::as_str)
}

pub fn get_did() -> Option<&'static str> {
    DID.get().map(String::as_str)
}

pub fn init_identity_from_js() {
    if let Some(node_id) = crate::js_interop::get_global_string("__INOS_NODE_ID__") {
        set_node_id(&node_id);
    }
    if let Some(device_id) = crate::js_interop::get_global_string("__INOS_DEVICE_ID__") {
        set_device_id(&device_id);
    }
    if let Some(did) = crate::js_interop::get_global_string("__INOS_DID__") {
        set_did(&did);
    }
}
