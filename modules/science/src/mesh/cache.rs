use crate::types::{CacheEntry, SimulationScale};
use std::collections::{HashMap, VecDeque};

pub struct ComputationCache {
    entries: HashMap<String, CacheEntry>,
    order: VecDeque<String>,
    max_size: usize,
}

impl Default for ComputationCache {
    fn default() -> Self {
        Self::new()
    }
}

impl ComputationCache {
    pub fn new() -> Self {
        Self {
            entries: HashMap::new(),
            order: VecDeque::new(),
            max_size: 1000,
        }
    }

    /// Retrieve an entry if it exists and is compatible with the requested scale.
    pub fn get(
        &mut self,
        key: &str,
        requested_scale: Option<&SimulationScale>,
    ) -> Option<CacheEntry> {
        if let Some(entry) = self.entries.get(key) {
            // Check scale compatibility if requested
            if let Some(req_scale) = requested_scale {
                if !self.is_compatible(&entry.scale, req_scale) {
                    return None;
                }
            }

            // Move to back of LRU order
            if let Some(pos) = self.order.iter().position(|k| k == key) {
                self.order.remove(pos);
            }
            self.order.push_back(key.to_string());

            return Some(entry.clone());
        }
        None
    }

    pub fn put(&mut self, key: String, entry: CacheEntry) {
        if self.entries.contains_key(&key) {
            // Update existing: remove from order to move to back later
            if let Some(pos) = self.order.iter().position(|k| k == &key) {
                self.order.remove(pos);
            }
        } else if self.entries.len() >= self.max_size {
            // Evict oldest (front)
            if let Some(oldest_key) = self.order.pop_front() {
                self.entries.remove(&oldest_key);
            }
        }

        self.order.push_back(key.clone());
        self.entries.insert(key, entry);
    }

    /// Simple heuristic for scale compatibility.
    /// In a production scenario, this would involve epsilon checks and fidelity downgrading logic.
    fn is_compatible(&self, cached: &SimulationScale, requested: &SimulationScale) -> bool {
        // Fidelity must be equal or higher than requested
        if cached.fidelity < requested.fidelity {
            return false;
        }

        // Spatial and temporal resolution must match exactly (for now)
        // Note: Using direct f64 comparison is risky, but given this is likely discrete scales
        // from a configuration, it might be acceptable. In practice, use epsilons.
        let epsilon = 1e-9;
        (cached.spatial - requested.spatial).abs() < epsilon
            && (cached.temporal - requested.temporal).abs() < epsilon
            && (cached.energy - requested.energy).abs() < epsilon
    }

    pub fn clear(&mut self) {
        self.entries.clear();
        self.order.clear();
    }

    pub fn len(&self) -> usize {
        self.entries.len()
    }

    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }
}
