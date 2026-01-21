use serde::{Deserialize, Serialize};
use web_sys::Performance;

/// Cost tracker for measuring compute time
pub struct CostTracker {
    perf: Performance,
    start_time: f64,
}

impl CostTracker {
    pub fn new() -> Option<Self> {
        let window = web_sys::window()?;
        let perf = window.performance()?;
        Some(Self {
            perf,
            start_time: 0.0,
        })
    }

    pub fn start(&mut self) {
        self.start_time = self.perf.now();
    }

    pub fn stop(&self) -> f64 {
        // Returns ms duration
        self.perf.now() - self.start_time
    }
}

/// Budget verifier for credit consumption
pub struct BudgetVerifier {
    budget: u64,
    consumed: u64,
}

impl BudgetVerifier {
    pub fn new(budget: u64) -> Self {
        Self {
            budget,
            consumed: 0,
        }
    }

    pub fn consume(&mut self, amount: u64) -> Result<(), &'static str> {
        self.consumed += amount;
        if self.consumed > self.budget {
            Err("OutOfCredits")
        } else {
            Ok(())
        }
    }

    pub fn remaining(&self) -> u64 {
        self.budget.saturating_sub(self.consumed)
    }
}

/// Replication tier for economic regulation
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ReplicationTier {
    /// Hot: Many replicas, low latency, high cost
    Hot,
    /// Warm: Some replicas, medium latency, medium cost
    Warm,
    /// Cold: Few replicas, high latency, low cost
    Cold,
    /// Archive: Minimal replicas, restoration required
    Archive,
}

impl ReplicationTier {
    /// Cost per access (in credits)
    pub fn access_cost(&self) -> u64 {
        match self {
            ReplicationTier::Hot => 1,
            ReplicationTier::Warm => 5,
            ReplicationTier::Cold => 20,
            ReplicationTier::Archive => 100,
        }
    }

    /// Cost per byte stored per hour (in credits)
    pub fn storage_cost(&self) -> u64 {
        match self {
            ReplicationTier::Hot => 10,
            ReplicationTier::Warm => 5,
            ReplicationTier::Cold => 2,
            ReplicationTier::Archive => 1,
        }
    }

    /// Number of replicas
    pub fn replica_count(&self) -> usize {
        match self {
            ReplicationTier::Hot => 10,
            ReplicationTier::Warm => 5,
            ReplicationTier::Cold => 2,
            ReplicationTier::Archive => 1,
        }
    }

    /// Promote to higher tier
    pub fn promote(&self) -> Option<Self> {
        match self {
            ReplicationTier::Archive => Some(ReplicationTier::Cold),
            ReplicationTier::Cold => Some(ReplicationTier::Warm),
            ReplicationTier::Warm => Some(ReplicationTier::Hot),
            ReplicationTier::Hot => None,
        }
    }

    /// Demote to lower tier
    pub fn demote(&self) -> Option<Self> {
        match self {
            ReplicationTier::Hot => Some(ReplicationTier::Warm),
            ReplicationTier::Warm => Some(ReplicationTier::Cold),
            ReplicationTier::Cold => Some(ReplicationTier::Archive),
            ReplicationTier::Archive => None,
        }
    }
}

/// Economic incentives for replication
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ReplicationIncentive {
    /// Reward for serving bandwidth (credits per MB)
    pub bandwidth_reward: u64,

    /// Reward for storing data (credits per GB per hour)
    pub storage_reward: u64,

    /// Multiplier based on demand
    pub demand_multiplier: f64,
}

impl ReplicationIncentive {
    pub fn new() -> Self {
        Self {
            bandwidth_reward: 10, // 10 credits per MB served
            storage_reward: 100,  // 100 credits per GB per hour
            demand_multiplier: 1.0,
        }
    }

    /// Calculate reward for serving data
    pub fn calculate_bandwidth_reward(&self, bytes_served: u64) -> u64 {
        let mb_served = bytes_served / (1024 * 1024);
        (mb_served * self.bandwidth_reward) as u64
    }

    /// Calculate reward for storing data
    pub fn calculate_storage_reward(&self, bytes_stored: u64, hours: u64) -> u64 {
        let gb_stored = bytes_stored / (1024 * 1024 * 1024);
        (gb_stored * hours * self.storage_reward) as u64
    }

    /// Adjust multiplier based on demand
    pub fn adjust_for_demand(&mut self, access_frequency: f64) {
        // Higher demand = higher multiplier
        self.demand_multiplier = (access_frequency / 100.0).max(0.1).min(10.0);
    }
}

impl Default for ReplicationIncentive {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_tier_costs() {
        assert_eq!(ReplicationTier::Hot.access_cost(), 1);
        assert_eq!(ReplicationTier::Archive.access_cost(), 100);
    }

    #[test]
    fn test_tier_promotion() {
        let tier = ReplicationTier::Cold;
        assert_eq!(tier.promote(), Some(ReplicationTier::Warm));
        assert_eq!(ReplicationTier::Hot.promote(), None);
    }

    #[test]
    fn test_bandwidth_reward() {
        let incentive = ReplicationIncentive::new();
        let reward = incentive.calculate_bandwidth_reward(10 * 1024 * 1024); // 10 MB
        assert_eq!(reward, 100); // 10 MB * 10 credits/MB
    }
}
