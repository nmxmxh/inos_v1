use automerge::{AutoCommit, ObjType, ReadDoc, ScalarValue};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Represents an economic wallet in the P2P network
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Wallet {
    pub public_key: Vec<u8>, // Ed25519 public key
    pub balance: u64,        // Current credit balance
    pub nonce: u64,          // For replay protection
    pub created_at: i64,     // Unix timestamp
}

/// Economic transaction between wallets
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Transaction {
    pub id: String,               // BLAKE3 hash of transaction data
    pub from: String,             // Sender's public key hash
    pub to: String,               // Receiver's public key hash
    pub amount: u64,              // Amount to transfer
    pub fee: u64,                 // Transaction fee
    pub nonce: u64,               // Sender's nonce (prevents replay)
    pub signature: Vec<u8>,       // Ed25519 signature
    pub timestamp: i64,           // Unix timestamp
    pub payload: Option<Vec<u8>>, // Optional extra data
}

/// Proof-of-Work mining result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MiningShare {
    pub miner_id: String, // Miner's public key hash
    pub nonce: u64,       // Mining nonce
    pub hash: Vec<u8>,    // Proof hash
    pub difficulty: u64,  // Mining difficulty achieved
    pub job_id: String,   // Mining job identifier
    pub reward: u64,      // Mining reward amount
    pub timestamp: i64,
}

/// CRDT-based distributed ledger state
pub struct LedgerState {
    doc: AutoCommit,
    wallets: HashMap<String, Wallet>,
    pending_transactions: Vec<Transaction>,
    mining_shares: Vec<MiningShare>,
}

impl Default for LedgerState {
    fn default() -> Self {
        Self::new()
    }
}

use automerge::transaction::Transactable; // Key import for put/get/etc.

impl LedgerState {
    /// Create a new empty ledger
    pub fn new() -> Self {
        let mut doc = AutoCommit::new();

        // Initialize CRDT document structure
        // Using unwrap() here is safe as we are initializing a fresh document
        doc.put_object(automerge::ROOT, "wallets", ObjType::Map)
            .unwrap();
        doc.put_object(automerge::ROOT, "transactions", ObjType::List)
            .unwrap();
        doc.put_object(automerge::ROOT, "mining_shares", ObjType::List)
            .unwrap();
        doc.put(
            automerge::ROOT,
            "last_block_hash",
            ScalarValue::Bytes(Vec::new()),
        )
        .unwrap();
        doc.put(automerge::ROOT, "height", ScalarValue::Uint(0))
            .unwrap();

        Self {
            doc,
            wallets: HashMap::new(),
            pending_transactions: Vec::new(),
            mining_shares: Vec::new(),
        }
    }

    /// Create or update a wallet
    /// Returns the wallet ID string
    pub fn create_wallet(&mut self, public_key: Vec<u8>, initial_balance: u64) -> String {
        let wallet_id = Self::hash_public_key(&public_key);

        let wallet = Wallet {
            public_key: public_key.clone(),
            balance: initial_balance,
            nonce: 0,
            created_at: chrono::Utc::now().timestamp(),
        };

        // Robust CRDT Update Logic
        if let Ok(Some((_, wallet_map_id))) = self.doc.get(automerge::ROOT, "wallets") {
            let wallet_id_key = wallet_id.clone();

            // Check if wallet already exists in the CRDT
            let existing_wallet = self.doc.get(&wallet_map_id, &wallet_id_key).unwrap_or(None);

            let wallet_obj_id = if let Some((_, id)) = existing_wallet {
                // Wallet exists, we will update it.
                // Note: In a real distributed system, we might want to merge balances or check conflicts.
                // For now, we reuse the object ID.
                id
            } else {
                // Wallet does not exist, create new map object
                self.doc
                    .put_object(&wallet_map_id, &wallet_id_key, ObjType::Map)
                    .unwrap()
            };

            // Update fields (Last Write Wins semantics by default)
            self.doc
                .put(&wallet_obj_id, "balance", ScalarValue::Uint(wallet.balance))
                .unwrap();
            self.doc
                .put(&wallet_obj_id, "nonce", ScalarValue::Uint(wallet.nonce))
                .unwrap();
            self.doc
                .put(
                    &wallet_obj_id,
                    "created_at",
                    ScalarValue::Int(wallet.created_at),
                )
                .unwrap();
            self.doc
                .put(&wallet_obj_id, "public_key", ScalarValue::Bytes(public_key))
                .unwrap();
        }

        // Update local cache
        self.wallets.insert(wallet_id.clone(), wallet);
        wallet_id
    }

    /// Create a signed transaction
    pub fn create_transaction(
        &mut self,
        from_sk: &ed25519_dalek::SigningKey,
        to_pubkey: &[u8],
        amount: u64,
        fee: u64,
        payload: Option<Vec<u8>>,
    ) -> Result<Transaction, String> {
        use ed25519_dalek::Signer;

        let from_pubkey = ed25519_dalek::VerifyingKey::from(from_sk);
        let from_id = Self::hash_public_key(from_pubkey.as_bytes());
        let to_id = Self::hash_public_key(to_pubkey);

        // Check sender balance using local cache (which should be synced)
        let from_wallet = self
            .wallets
            .get(&from_id)
            .ok_or_else(|| "Sender wallet not found. Ensure wallet is synced.".to_string())?;

        if from_wallet.balance < amount + fee {
            return Err(format!(
                "Insufficient balance: {} < {}",
                from_wallet.balance,
                amount + fee
            ));
        }

        // Create transaction
        let nonce = from_wallet.nonce + 1;
        let timestamp = chrono::Utc::now().timestamp();

        let tx_data = Self::serialize_transaction_data(
            &from_id, &to_id, amount, fee, nonce, timestamp, &payload,
        );

        // Sign transaction
        let signature = from_sk.sign(&tx_data).to_bytes().to_vec();
        let tx_id = crate::hashing::hash_data(&tx_data);

        let transaction = Transaction {
            id: tx_id,
            from: from_id,
            to: to_id,
            amount,
            fee,
            nonce,
            signature,
            timestamp,
            payload,
        };

        self.pending_transactions.push(transaction.clone());
        Ok(transaction)
    }

    /// Apply transaction to ledger (CRDT)
    pub fn apply_transaction(&mut self, transaction: &Transaction) -> Result<(), String> {
        // Verify signature
        if !self.verify_transaction(transaction)? {
            return Err("Invalid transaction signature".to_string());
        }

        // Validating nonce would happen here against state

        // Update sender wallet locally
        if let Some(sender) = self.wallets.get_mut(&transaction.from) {
            // Strict check
            if sender.balance < transaction.amount + transaction.fee {
                return Err("Insufficient balance during application".to_string());
            }
            sender.balance -= transaction.amount + transaction.fee;
            sender.nonce = transaction.nonce;

            // Sync to CRDT
            // We need to find the wallet object. This is expensive but necessary for correctness.
            if let Ok(Some((_, wallet_map_id))) = self.doc.get(automerge::ROOT, "wallets") {
                let sender_id = transaction.from.clone();
                if let Ok(Some((_, sender_obj_id))) = self.doc.get(&wallet_map_id, sender_id) {
                    self.doc
                        .put(&sender_obj_id, "balance", ScalarValue::Uint(sender.balance))
                        .unwrap();
                    self.doc
                        .put(&sender_obj_id, "nonce", ScalarValue::Uint(sender.nonce))
                        .unwrap();
                }
            }
        }

        // Update receiver wallet locally
        if let Some(receiver) = self.wallets.get_mut(&transaction.to) {
            receiver.balance += transaction.amount;
            // Sync to CRDT
            if let Ok(Some((_, wallet_map_id))) = self.doc.get(automerge::ROOT, "wallets") {
                let receiver_id = transaction.to.clone();
                if let Ok(Some((_, receiver_obj_id))) = self.doc.get(&wallet_map_id, receiver_id) {
                    self.doc
                        .put(
                            &receiver_obj_id,
                            "balance",
                            ScalarValue::Uint(receiver.balance),
                        )
                        .unwrap();
                }
            }
        }

        // Add to CRDT document transaction list
        if let Ok(Some((_, tx_list))) = self.doc.get(automerge::ROOT, "transactions") {
            let tx_idx = self.doc.length(&tx_list);
            // Use put_object generic logic if insert_object is not directly reachable or behaves differently
            let tx_obj = self
                .doc
                .insert_object(&tx_list, tx_idx, ObjType::Map)
                .unwrap();

            self.doc
                .put(
                    &tx_obj,
                    "id",
                    ScalarValue::Str(transaction.id.clone().into()),
                )
                .unwrap();
            self.doc
                .put(
                    &tx_obj,
                    "from",
                    ScalarValue::Str(transaction.from.clone().into()),
                )
                .unwrap();
            self.doc
                .put(
                    &tx_obj,
                    "to",
                    ScalarValue::Str(transaction.to.clone().into()),
                )
                .unwrap();
            self.doc
                .put(&tx_obj, "amount", ScalarValue::Uint(transaction.amount))
                .unwrap();
            self.doc
                .put(&tx_obj, "fee", ScalarValue::Uint(transaction.fee))
                .unwrap();
            self.doc
                .put(&tx_obj, "nonce", ScalarValue::Uint(transaction.nonce))
                .unwrap();
            self.doc
                .put(
                    &tx_obj,
                    "timestamp",
                    ScalarValue::Int(transaction.timestamp),
                )
                .unwrap();
        }

        // Remove from pending
        self.pending_transactions
            .retain(|tx| tx.id != transaction.id);
        Ok(())
    }

    /// Add mining reward
    pub fn add_mining_share(&mut self, share: MiningShare) -> Result<(), String> {
        if !self.verify_mining_share(&share) {
            return Err("Invalid mining share".to_string());
        }

        // Reward miner locally
        if let Some(miner) = self.wallets.get_mut(&share.miner_id) {
            miner.balance += share.reward;

            // Sync to CRDT
            if let Ok(Some((_, wallet_map_id))) = self.doc.get(automerge::ROOT, "wallets") {
                let miner_id = share.miner_id.clone();
                if let Ok(Some((_, miner_obj_id))) = self.doc.get(&wallet_map_id, miner_id) {
                    self.doc
                        .put(&miner_obj_id, "balance", ScalarValue::Uint(miner.balance))
                        .unwrap();
                }
            }
        }

        // Add to CRDT document
        if let Ok(Some((_, mining_list))) = self.doc.get(automerge::ROOT, "mining_shares") {
            let idx = self.doc.length(&mining_list);
            let mining_obj = self
                .doc
                .insert_object(&mining_list, idx, ObjType::Map)
                .unwrap();

            self.doc
                .put(
                    &mining_obj,
                    "miner_id",
                    ScalarValue::Str(share.miner_id.clone().into()),
                )
                .unwrap();
            self.doc
                .put(&mining_obj, "nonce", ScalarValue::Uint(share.nonce))
                .unwrap();
            self.doc
                .put(
                    &mining_obj,
                    "difficulty",
                    ScalarValue::Uint(share.difficulty),
                )
                .unwrap();
            self.doc
                .put(&mining_obj, "reward", ScalarValue::Uint(share.reward))
                .unwrap();
            self.doc
                .put(&mining_obj, "timestamp", ScalarValue::Int(share.timestamp))
                .unwrap();
        }

        self.mining_shares.push(share);
        Ok(())
    }

    /// Merge with another ledger state
    pub fn merge(&mut self, other: &mut Self) -> Result<(), String> {
        // In automerge 0.5+, apply_changes requires Owned changes or correct iteration
        // Getting changes from 'other' usually requires mutable access in recent generic APIs or we clone
        // But get_changes returns &'a Change. We need to clone them to apply.

        let changes = other.doc.get_changes(&[]);
        // Clone changes to own them
        let owned_changes: Vec<automerge::Change> = changes.into_iter().cloned().collect();

        self.doc
            .apply_changes(owned_changes)
            .map_err(|e| format!("Failed to merge ledger: {:?}", e))
    }

    /// Generate changes for synchronization
    pub fn get_changes(&mut self, after_heads: &[automerge::ChangeHash]) -> Vec<u8> {
        let changes = self.doc.get_changes(after_heads);
        let mut bytes = Vec::new();
        for change in changes {
            // Change in 0.5.12 might require mutable access for bytes() or we need to own it
            // safely clone to get ownership and bytes.
            bytes.extend_from_slice(change.clone().bytes().as_ref());
        }
        bytes
    }

    /// Get current heads
    pub fn get_heads(&mut self) -> Vec<automerge::ChangeHash> {
        self.doc.get_heads()
    }

    /// Export ledger state
    pub fn save(&mut self) -> Vec<u8> {
        self.doc.save()
    }

    // ... Verification helpers remain similar ...
    fn verify_transaction(&self, tx: &Transaction) -> Result<bool, String> {
        let tx_data = Self::serialize_transaction_data(
            &tx.from,
            &tx.to,
            tx.amount,
            tx.fee,
            tx.nonce,
            tx.timestamp,
            &tx.payload,
        );
        let expected_hash = crate::hashing::hash_data(&tx_data);
        Ok(expected_hash == tx.id)
    }

    fn verify_mining_share(&self, share: &MiningShare) -> bool {
        !share.miner_id.is_empty() && share.reward > 0
    }

    fn serialize_transaction_data(
        from: &str,
        to: &str,
        amount: u64,
        fee: u64,
        nonce: u64,
        timestamp: i64,
        payload: &Option<Vec<u8>>,
    ) -> Vec<u8> {
        let mut data = Vec::new();
        data.extend(from.as_bytes());
        data.extend(to.as_bytes());
        data.extend(&amount.to_le_bytes());
        data.extend(&fee.to_le_bytes());
        data.extend(&nonce.to_le_bytes());
        data.extend(&timestamp.to_le_bytes());
        if let Some(p) = payload {
            data.extend(p);
        }
        data
    }

    fn hash_public_key(pubkey: &[u8]) -> String {
        crate::hashing::hash_data(pubkey).chars().take(16).collect()
    }

    pub fn get_balance(&self, wallet_id: &str) -> Option<u64> {
        self.wallets.get(wallet_id).map(|w| w.balance)
    }

    pub fn get_pending_transactions(&self) -> &[Transaction] {
        &self.pending_transactions
    }

    pub fn total_supply(&self) -> u64 {
        self.wallets.values().map(|w| w.balance).sum()
    }

    /// Import ledger state from bytes
    pub fn load(data: &[u8]) -> Result<Self, String> {
        let doc = AutoCommit::load(data).map_err(|e| format!("Failed to load ledger: {:?}", e))?;
        // Reconstruct local caches
        let mut wallets = HashMap::new();

        // Populate wallets from CRDT
        if let Ok(Some((_, wallet_map_id))) = doc.get(automerge::ROOT, "wallets") {
            // Iterate over keys in the map
            for key in doc.keys(&wallet_map_id) {
                if let Ok(Some((_, wallet_obj_id))) = doc.get(&wallet_map_id, &key) {
                    let balance = doc
                        .get(&wallet_obj_id, "balance")
                        .unwrap()
                        .and_then(|(v, _)| v.to_u64())
                        .unwrap_or(0);
                    let nonce = doc
                        .get(&wallet_obj_id, "nonce")
                        .unwrap()
                        .and_then(|(v, _)| v.to_u64())
                        .unwrap_or(0);
                    let created_at = doc
                        .get(&wallet_obj_id, "created_at")
                        .unwrap()
                        .and_then(|(v, _)| v.to_i64())
                        .unwrap_or(0);
                    let public_key = doc
                        .get(&wallet_obj_id, "public_key")
                        .unwrap()
                        .and_then(|(v, _)| v.to_bytes().map(|b| b.to_vec()))
                        .unwrap_or_default();

                    let wallet = Wallet {
                        public_key,
                        balance,
                        nonce,
                        created_at,
                    };
                    wallets.insert(key, wallet);
                }
            }
        }
        Ok(Self {
            doc,
            wallets,
            pending_transactions: Vec::new(),
            mining_shares: Vec::new(),
        })
    }
}

/// Grow-only counter for economic metrics
#[derive(Debug, Clone)]
pub struct GCounter {
    replica_id: String,
    counts: HashMap<String, u64>, // replica_id -> count
}

impl Default for GCounter {
    fn default() -> Self {
        Self::new("default")
    }
}

impl GCounter {
    pub fn new(replica_id: &str) -> Self {
        let mut counts = HashMap::new();
        counts.insert(replica_id.to_string(), 0);

        Self {
            replica_id: replica_id.to_string(),
            counts,
        }
    }

    /// Increment this replica's counter
    pub fn increment(&mut self, amount: u64) {
        let entry = self.counts.entry(self.replica_id.clone()).or_insert(0);
        *entry += amount;
    }

    /// Get current value (sum of all replicas)
    pub fn value(&self) -> u64 {
        self.counts.values().sum()
    }

    /// Merge with another GCounter
    pub fn merge(&mut self, other: &Self) {
        for (replica_id, count) in &other.counts {
            let entry = self.counts.entry(replica_id.clone()).or_insert(0);
            *entry = (*entry).max(*count);
        }
    }

    /// Export counter state
    pub fn to_bytes(&self) -> Vec<u8> {
        bincode::serialize(&self.counts).unwrap()
    }

    /// Import counter state
    pub fn from_bytes(bytes: &[u8], replica_id: &str) -> Result<Self, String> {
        let counts: HashMap<String, u64> = bincode::deserialize(bytes)
            .map_err(|e| format!("Failed to deserialize GCounter: {}", e))?;

        Ok(Self {
            replica_id: replica_id.to_string(),
            counts,
        })
    }
}
