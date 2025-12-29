use super::{Chunk, P2pConfig, P2pError, Result};
use blake3::Hasher;
use dashmap::DashMap;
use serde::{Deserialize, Serialize};
use std::sync::Arc;

/// Proof of Retrievability challenge
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct PorChallenge {
    pub chunk_id: String,
    pub nonce: [u8; 32],
    pub timestamp: u64,
    pub difficulty: u8,
}

/// Proof of Retrievability response
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct PorProof {
    pub chunk_id: String,
    pub response_hash: String,
    pub chunk_size: usize,
    pub timestamp: u64,
}

/// Detailed peer reputation tracking
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct PeerReputation {
    pub score: f32,
    pub total_challenges: u64,
    pub successful_challenges: u64,
    pub failed_challenges: u64,
    pub last_challenge_time: u64,
    pub consecutive_failures: u32,
    pub penalized_until: Option<u64>,
}

impl Default for PeerReputation {
    fn default() -> Self {
        Self {
            score: 1.0,
            total_challenges: 0,
            successful_challenges: 0,
            failed_challenges: 0,
            last_challenge_time: 0,
            consecutive_failures: 0,
            penalized_until: None,
        }
    }
}

/// Batch verification result
#[derive(Clone, Debug)]
pub struct BatchVerificationResult {
    pub valid: usize,
    pub invalid: usize,
    pub success_rate: f32,
    pub failed_chunks: Vec<String>,
}

/// Verification strategy
#[derive(Clone, Debug, PartialEq)]
pub enum VerificationStrategy {
    Quick,    // Just hash check
    Standard, // PoR challenge
    Thorough, // Multiple verifications
}

/// Proof of Retrievability verifier with challenge-response
pub struct PorVerifier {
    config: P2pConfig,
    peer_reputations: Arc<DashMap<String, PeerReputation>>,
    pending_challenges: Arc<DashMap<String, (PorChallenge, u64)>>,
}

impl PorVerifier {
    pub fn new(config: P2pConfig) -> Self {
        Self {
            config,
            peer_reputations: Arc::new(DashMap::new()),
            pending_challenges: Arc::new(DashMap::new()),
        }
    }

    /// Verify chunk integrity using BLAKE3
    pub fn verify_chunk(&self, chunk: &Chunk) -> Result<()> {
        if !chunk.is_valid() {
            return Err(P2pError::VerificationFailed {
                chunk_id: chunk.id.clone(),
                reason: "Hash mismatch".to_string(),
                proof_type: "BLAKE3".to_string(),
            });
        }
        Ok(())
    }

    /// Create a PoR challenge for a peer
    pub fn create_challenge(&self, peer_id: &str, chunk_id: &str) -> Result<PorChallenge> {
        let mut nonce = [0u8; 32];

        // Generate random nonce (WASM-compatible)
        #[cfg(target_arch = "wasm32")]
        {
            use js_sys::Math;
            for byte in &mut nonce {
                *byte = (Math::random() * 256.0) as u8;
            }
        }
        #[cfg(not(target_arch = "wasm32"))]
        {
            use rand::RngCore;
            rand::thread_rng().fill_bytes(&mut nonce);
        }

        let difficulty = self.calculate_difficulty(peer_id);

        let challenge = PorChallenge {
            chunk_id: chunk_id.to_string(),
            nonce,
            timestamp: current_timestamp(),
            difficulty,
        };

        // Store challenge with 30s expiry
        let challenge_id = self.challenge_id(&challenge);
        let expiry = challenge.timestamp + 30;
        self.pending_challenges
            .insert(challenge_id, (challenge.clone(), expiry));

        Ok(challenge)
    }

    /// Verify a PoR proof from a peer
    pub fn verify_por_proof(&self, peer_id: &str, proof: &PorProof) -> Result<bool> {
        // Find pending challenge
        let challenge_id = format!("{}:{}", proof.chunk_id, peer_id);
        let (challenge, expiry) = match self.pending_challenges.get(&challenge_id) {
            Some(entry) => entry.clone(),
            None => {
                return Err(P2pError::VerificationFailed {
                    chunk_id: proof.chunk_id.clone(),
                    reason: "Challenge not found or expired".to_string(),
                    proof_type: "PoR".to_string(),
                })
            }
        };

        // Check expiry
        if current_timestamp() > expiry {
            self.pending_challenges.remove(&challenge_id);
            return Ok(false);
        }

        // Verify response hash
        let expected_hash = self.compute_expected_response(&challenge);
        let is_valid = proof.response_hash == expected_hash;

        // Update reputation
        self.update_reputation_detailed(peer_id, is_valid, challenge.difficulty as f32);

        // Cleanup
        if is_valid {
            self.pending_challenges.remove(&challenge_id);
        }

        Ok(is_valid)
    }

    /// Create batch challenge for multiple chunks
    pub fn create_batch_challenge(
        &self,
        peer_id: &str,
        chunk_ids: &[String],
        sample_rate: f32,
    ) -> Result<Vec<PorChallenge>> {
        let sample_count = (chunk_ids.len() as f32 * sample_rate).ceil() as usize;
        let sample_count = sample_count.clamp(1, chunk_ids.len().min(10));

        // Simple sampling (deterministic for WASM compatibility)
        let step = chunk_ids.len() / sample_count;
        let sampled: Vec<String> = chunk_ids
            .iter()
            .step_by(step.max(1))
            .take(sample_count)
            .cloned()
            .collect();

        sampled
            .iter()
            .map(|chunk_id| self.create_challenge(peer_id, chunk_id))
            .collect()
    }

    /// Verify batch of proofs
    pub fn verify_batch_proofs(
        &self,
        peer_id: &str,
        proofs: &[PorProof],
    ) -> Result<BatchVerificationResult> {
        let mut valid = 0;
        let mut invalid = 0;
        let mut failed_chunks = Vec::new();

        for proof in proofs {
            match self.verify_por_proof(peer_id, proof) {
                Ok(true) => valid += 1,
                Ok(false) => {
                    invalid += 1;
                    failed_chunks.push(proof.chunk_id.clone());
                }
                Err(_) => {
                    invalid += 1;
                    failed_chunks.push(proof.chunk_id.clone());
                }
            }
        }

        let success_rate = if valid + invalid > 0 {
            valid as f32 / (valid + invalid) as f32
        } else {
            0.0
        };

        Ok(BatchVerificationResult {
            valid,
            invalid,
            success_rate,
            failed_chunks,
        })
    }

    /// Update peer reputation with detailed tracking
    pub fn update_reputation_detailed(
        &self,
        peer_id: &str,
        success: bool,
        challenge_difficulty: f32,
    ) {
        let mut entry = self
            .peer_reputations
            .entry(peer_id.to_string())
            .or_default();

        let rep = entry.value_mut();
        rep.total_challenges += 1;
        rep.last_challenge_time = current_timestamp();

        if success {
            rep.successful_challenges += 1;
            rep.consecutive_failures = 0;

            // Weighted increase based on difficulty
            let increase = 0.1 * challenge_difficulty;
            rep.score = (rep.score + increase).min(1.0);
        } else {
            rep.failed_challenges += 1;
            rep.consecutive_failures += 1;

            // Exponential penalty for consecutive failures
            let penalty = 0.3 * (2.0f32).powi(rep.consecutive_failures as i32 - 1);
            rep.score = (rep.score * self.config.reputation_decay - penalty).max(0.0);

            // Penalize peer if they fail too many times
            if rep.consecutive_failures >= 3 {
                let penalty_duration = 5 * 60; // 5 minutes in seconds
                rep.penalized_until = Some(current_timestamp() + penalty_duration);
            }
        }
    }

    /// Update peer reputation (simple version)
    pub fn update_reputation(&mut self, peer_id: &str, success: bool) {
        self.update_reputation_detailed(peer_id, success, 1.0);
    }

    /// Get peer reputation score
    pub fn get_reputation(&self, peer_id: &str) -> f32 {
        self.peer_reputations
            .get(peer_id)
            .map(|entry| entry.score)
            .unwrap_or(1.0)
    }

    /// Get detailed peer reputation
    pub fn get_peer_reputation(&self, peer_id: &str) -> Option<PeerReputation> {
        self.peer_reputations
            .get(peer_id)
            .map(|entry| entry.clone())
    }

    /// Check if peer is trusted
    pub fn is_peer_trusted(&self, peer_id: &str) -> bool {
        let reputation = self.get_reputation(peer_id);
        self.config.is_peer_trusted(reputation)
    }

    /// Check if peer is currently penalized
    pub fn is_peer_penalized(&self, peer_id: &str) -> bool {
        if let Some(rep) = self.peer_reputations.get(peer_id) {
            if let Some(penalized_until) = rep.penalized_until {
                return current_timestamp() < penalized_until;
            }
        }
        false
    }

    /// Calculate challenge difficulty based on peer reputation
    fn calculate_difficulty(&self, peer_id: &str) -> u8 {
        let reputation = self.get_reputation(peer_id);

        if reputation > 0.8 {
            1 // Easy for trusted peers
        } else if reputation > 0.5 {
            2 // Medium
        } else if reputation > 0.2 {
            3 // Hard for low-reputation
        } else {
            4 // Very hard for untrusted
        }
    }

    /// Determine if peer should be verified
    pub fn should_verify_peer(&self, peer_id: &str) -> bool {
        if let Some(rep) = self.peer_reputations.get(peer_id) {
            // Verify less frequently for trusted peers
            let verification_probability: f32 = match rep.score {
                s if s > 0.9 => 0.1, // 10% chance
                s if s > 0.7 => 0.2, // 20% chance
                s if s > 0.5 => 0.3, // 30% chance
                _ => 0.5,            // 50% for low-reputation
            };

            // Simple random check (WASM-compatible)
            #[cfg(target_arch = "wasm32")]
            {
                js_sys::Math::random() < verification_probability as f64
            }
            #[cfg(not(target_arch = "wasm32"))]
            {
                rand::random::<f32>() < verification_probability
            }
        } else {
            true // Always verify new peers
        }
    }

    /// Select verification strategy based on risk
    pub fn select_verification_strategy(
        &self,
        peer_id: &str,
        chunk_value: f32,
    ) -> VerificationStrategy {
        let reputation = self.get_reputation(peer_id);

        match (reputation, chunk_value) {
            (r, _) if r > 0.8 => VerificationStrategy::Quick,
            (r, v) if r > 0.5 && v < 0.3 => VerificationStrategy::Standard,
            _ => VerificationStrategy::Thorough,
        }
    }

    /// Apply reputation decay to all peers
    pub fn apply_decay_to_all(&mut self) {
        for mut entry in self.peer_reputations.iter_mut() {
            let rep = entry.value_mut();
            rep.score = self.config.apply_reputation_decay(rep.score);
        }
    }

    /// Generate challenge ID
    fn challenge_id(&self, challenge: &PorChallenge) -> String {
        format!("{}:{}", challenge.chunk_id, challenge.timestamp)
    }

    /// Compute expected response hash
    fn compute_expected_response(&self, challenge: &PorChallenge) -> String {
        let mut hasher = Hasher::new();
        hasher.update(challenge.chunk_id.as_bytes());
        hasher.update(&challenge.nonce);
        hasher.update(&challenge.timestamp.to_le_bytes());
        hasher.finalize().to_hex().to_string()
    }
}

/// Get current timestamp (WASM-compatible)
fn current_timestamp() -> u64 {
    #[cfg(target_arch = "wasm32")]
    {
        (js_sys::Date::now() / 1000.0) as u64
    }
    #[cfg(not(target_arch = "wasm32"))]
    {
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn create_test_chunk() -> Chunk {
        let data = b"test data";
        let mut hasher = Hasher::new();
        hasher.update(data);
        let hash = hasher.finalize().to_hex().to_string();

        Chunk {
            id: "test-chunk".to_string(),
            model_id: "test-model".to_string(),
            index: 0,
            data: data.to_vec(),
            hash,
            size: data.len(),
        }
    }

    #[test]
    fn test_chunk_verification() {
        let config = P2pConfig::default();
        let verifier = PorVerifier::new(config);
        let chunk = create_test_chunk();

        assert!(verifier.verify_chunk(&chunk).is_ok());
    }

    #[test]
    fn test_challenge_creation() {
        let config = P2pConfig::default();
        let verifier = PorVerifier::new(config);

        let challenge = verifier.create_challenge("peer-1", "chunk-1").unwrap();
        assert_eq!(challenge.chunk_id, "chunk-1");
        assert!(challenge.difficulty > 0);
    }

    #[test]
    fn test_reputation_updates() {
        let config = P2pConfig::default();
        let mut verifier = PorVerifier::new(config);

        // Initial reputation
        assert_eq!(verifier.get_reputation("peer-1"), 1.0);

        // Success increases reputation
        verifier.update_reputation("peer-1", true);
        let rep_after_success = verifier.get_reputation("peer-1");
        assert!(rep_after_success >= 1.0);

        // Failure decreases reputation
        verifier.update_reputation("peer-1", false);
        assert!(verifier.get_reputation("peer-1") < rep_after_success);
    }

    #[test]
    fn test_peer_penalty() {
        let config = P2pConfig::default();
        let mut verifier = PorVerifier::new(config);

        // Initially not penalized
        assert!(!verifier.is_peer_penalized("peer-1"));

        // After 3 consecutive failures, should be penalized
        for _ in 0..3 {
            verifier.update_reputation("peer-1", false);
        }
        assert!(verifier.is_peer_penalized("peer-1"));
    }

    #[test]
    fn test_batch_verification() {
        let config = P2pConfig::default();
        let verifier = PorVerifier::new(config);

        let chunk_ids = vec![
            "chunk-1".to_string(),
            "chunk-2".to_string(),
            "chunk-3".to_string(),
        ];

        let challenges = verifier
            .create_batch_challenge("peer-1", &chunk_ids, 0.5)
            .unwrap();

        assert!(!challenges.is_empty());
        assert!(challenges.len() <= chunk_ids.len());
    }

    #[test]
    fn test_verification_strategy() {
        let config = P2pConfig::default();
        let mut verifier = PorVerifier::new(config);

        // High reputation -> Quick strategy
        verifier.update_reputation("peer-1", true);
        verifier.update_reputation("peer-1", true);
        let strategy = verifier.select_verification_strategy("peer-1", 0.1);
        assert_eq!(strategy, VerificationStrategy::Quick);

        // Low reputation -> Thorough strategy
        let strategy = verifier.select_verification_strategy("peer-2", 0.9);
        assert_eq!(strategy, VerificationStrategy::Thorough);
    }
}
