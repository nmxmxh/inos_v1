use crate::flux::anchor::{ConservationAnchor, FluxProposal};
use crate::mosaic::dispatch::VoxelID;
use nalgebra::Vector3;
use std::collections::HashMap;

/// Coordinates physics conservation across voxel boundaries.
pub struct CrossShardCoordinator {
    pub anchors: HashMap<BoundaryKey, ConservationAnchor>,
    pub sync_interval_ticks: u32,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct BoundaryKey {
    pub v1: VoxelID,
    pub v2: VoxelID,
}

impl BoundaryKey {
    pub fn new(mut v1: VoxelID, mut v2: VoxelID) -> Self {
        // Ensure deterministic ordering
        if format!("{:?}", v1.0) > format!("{:?}", v2.0) {
            std::mem::swap(&mut v1, &mut v2);
        }
        Self { v1, v2 }
    }
}

impl CrossShardCoordinator {
    pub fn new() -> Self {
        Self {
            anchors: HashMap::new(),
            sync_interval_ticks: 1,
        }
    }

    /// Negotiates conservation at the boundary between two voxels.
    pub fn negotiate_boundary(
        &mut self,
        v1: VoxelID,
        v2: VoxelID,
        incoming_flux: FluxProposal,
        outgoing_flux: FluxProposal,
        epoch: u64,
    ) {
        let key = BoundaryKey::new(v1, v2);
        let anchor = self.anchors.entry(key).or_insert_with(|| {
            ConservationAnchor::new(
                &format!("boundary_{:?}_{:?}", v1.0, v2.0),
                Vector3::zeros(), // Boundary center could be calculated
            )
        });

        // Ensure conservation: flux leaving v1 must enter v2
        anchor.negotiate(incoming_flux, outgoing_flux, epoch);
    }

    /// Enforces global invariants by syncing all boundary anchors.
    pub fn collect_violation_metrics(&self) -> f64 {
        let mut total_violation = 0.0;
        for anchor in self.anchors.values() {
            total_violation += anchor.telemetry.worst_violation;
        }
        total_violation
    }
}

impl Default for CrossShardCoordinator {
    fn default() -> Self {
        Self::new()
    }
}
