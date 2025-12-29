use crate::model_capnp::model::{brain_request, BrainOp};
use capnp::serialize_packed;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::RwLock;

pub struct CyberneticBrain {
    // Sub-models
    bayes: RwLock<NaiveBayes>,
    // rl: RwLock<ReinforcementLearner>, // Future
    // causal: RwLock<CausalGraph>, // Future
}

#[derive(Serialize, Deserialize, Debug)]
pub struct BrainResult {
    pub decision: String,
    pub confidence: f32,
    pub latency_ms: u64,
    pub explanation: Vec<String>,
}

impl Default for CyberneticBrain {
    fn default() -> Self {
        Self::new()
    }
}

impl CyberneticBrain {
    pub fn new() -> Self {
        Self {
            bayes: RwLock::new(NaiveBayes::new()),
        }
    }

    pub fn process(&self, _method: &str, input: &[u8], _params: &str) -> Result<Vec<u8>, String> {
        let start = std::time::Instant::now();

        // -----------------------------------------------------------
        // Zero-Copy Cap'n Proto Deserialization (Production Grade)
        // -----------------------------------------------------------

        let reader =
            serialize_packed::read_message(&mut &input[..], capnp::message::ReaderOptions::new())
                .map_err(|e| format!("Capnp decode failed: {}", e))?;

        let request = reader
            .get_root::<brain_request::Reader>()
            .map_err(|e| format!("Get root failed: {}", e))?;

        let op = request.get_op().map_err(|_| "Unknown OP".to_string())?;

        match op {
            BrainOp::Predict => {
                let features_list = request.get_features().map_err(|e| e.to_string())?;
                let mut features = HashMap::new();
                for i in 0..features_list.len() {
                    let entry = features_list.get(i);
                    let key = entry
                        .get_key()
                        .map_err(|e| e.to_string())?
                        .to_str()
                        .map_err(|e| e.to_string())?
                        .to_string();
                    features.insert(key, entry.get_value());
                }

                let decisions = self.bayes.read().unwrap().predict(&features);

                // Get the top decision safely (cloning string to avoid borrow issues)
                let (decision, confidence) = if let Some((d, c)) = decisions.first() {
                    (d.clone(), *c)
                } else {
                    ("uncertain".to_string(), 0.0)
                };

                let result = BrainResult {
                    decision,
                    confidence,
                    latency_ms: start.elapsed().as_millis() as u64,
                    explanation: vec!["Bayesian inference".to_string()],
                };

                // Return JSON for JS compatibility layer
                serde_json::to_vec(&result).map_err(|e| e.to_string())
            }
            BrainOp::Learn => {
                let features_list = request.get_features().map_err(|e| e.to_string())?;
                let context = request
                    .get_context()
                    .map_err(|e| e.to_string())?
                    .to_str()
                    .map_err(|e| e.to_string())?;

                // In a real generic learn op, 'context' might be the label or encoded data.
                // For Naive Bayes, we treat context as label.
                let label = context;

                let mut features = HashMap::new();
                for i in 0..features_list.len() {
                    let entry = features_list.get(i);
                    let key = entry
                        .get_key()
                        .map_err(|e| e.to_string())?
                        .to_str()
                        .map_err(|e| e.to_string())?
                        .to_string();
                    features.insert(key, entry.get_value());
                }

                self.bayes.write().unwrap().train(&features, label);
                Ok(vec![1])
            }
            _ => Err("Unsupported OP".to_string()),
        }
    }
}

// ----------------------------------------------------------------------------
// ALGORITHM 1: Naive Bayes (Incremental)
// ----------------------------------------------------------------------------

struct NaiveBayes {
    // Map<Label, Map<Feature, Count>>
    match_counts: HashMap<String, HashMap<String, u64>>,
    // Map<Label, TotalCount>
    class_counts: HashMap<String, u64>,
    total_samples: u64,
}

impl NaiveBayes {
    fn new() -> Self {
        Self {
            match_counts: HashMap::new(),
            class_counts: HashMap::new(),
            total_samples: 0,
        }
    }

    fn train(&mut self, features: &HashMap<String, f32>, label: &str) {
        self.total_samples += 1;
        *self.class_counts.entry(label.to_string()).or_insert(0) += 1;

        for (feature, &value) in features {
            // Bucketize implementation: "low", "med", "high"
            // We suffix the feature name with the bucket
            let bucket = if value < 0.3 {
                "low"
            } else if value < 0.7 {
                "med"
            } else {
                "high"
            };
            let token = format!("{}:{}", feature, bucket);

            let label_map = self.match_counts.entry(label.to_string()).or_default();
            *label_map.entry(token).or_insert(0) += 1;
        }
    }

    fn predict(&self, features: &HashMap<String, f32>) -> Vec<(String, f32)> {
        let mut scores = HashMap::new();

        for (label, count) in &self.class_counts {
            // P(Class)
            let prior = (*count as f64) / (self.total_samples as f64);
            let mut log_prob = prior.ln();

            let label_features = self.match_counts.get(label);

            for (feature, &value) in features {
                let bucket = if value < 0.3 {
                    "low"
                } else if value < 0.7 {
                    "med"
                } else {
                    "high"
                };
                let token = format!("{}:{}", feature, bucket);

                // P(Feature|Class)
                // Laplacian smoothing
                let feature_count = label_features
                    .and_then(|m| m.get(&token))
                    .copied()
                    .unwrap_or(0);
                let prob = (feature_count as f64 + 1.0) / (*count as f64 + 10.0); // +10 for vocab size approx
                log_prob += prob.ln();
            }

            scores.insert(label, log_prob);
        }

        // Normalize scores to Softmax-ish (simplified)
        let mut predictions: Vec<(String, f32)> = scores
            .into_iter()
            .map(|(k, v)| (k.clone(), (v as f32).exp()))
            .collect();
        predictions.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap());

        predictions
    }
}
