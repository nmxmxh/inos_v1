use crate::sab::SafeSAB;

pub struct SocialGraph {
    sab: SafeSAB,
}

const SOCIAL_ACCOUNT_SIZE: usize = 1248;

pub struct SocialEntry {
    pub owner_did: String,
    pub referrer_did: String,
    pub close_ids: Vec<String>,
}

impl SocialGraph {
    pub fn new(sab: SafeSAB) -> Self {
        Self { sab }
    }

    pub fn get_entry(&self, index: usize) -> Result<SocialEntry, String> {
        let offset = SafeSAB::OFFSET_SOCIAL_GRAPH + (index * SOCIAL_ACCOUNT_SIZE);
        let data = self.sab.read(offset, SOCIAL_ACCOUNT_SIZE)?;

        let owner_did = Self::parse_did(&data[0..64]);
        let referrer_did = Self::parse_did(&data[64..128]);

        let mut close_ids = Vec::new();
        for i in 0..15 {
            let start = 128 + (i * 64);
            let end = 128 + ((i + 1) * 64);
            let cid = Self::parse_did(&data[start..end]);
            if !cid.is_empty() {
                close_ids.push(cid);
            }
        }

        Ok(SocialEntry {
            owner_did,
            referrer_did,
            close_ids,
        })
    }

    fn parse_did(data: &[u8]) -> String {
        let len = data.iter().position(|&b| b == 0).unwrap_or(data.len());
        String::from_utf8_lossy(&data[..len]).to_string()
    }
}
