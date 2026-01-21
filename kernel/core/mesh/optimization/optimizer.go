package optimization

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// EpochAwareOptimizer integrates all optimization features with epoch-based signaling
// This aligns with INOS's biological coherence model where updates happen on epoch boundaries
type EpochAwareOptimizer struct {
	mu sync.RWMutex

	// Optimization components
	tierDetector      *TierDetector
	schedulerScoring  *SchedulerScoring
	deltaReplication  *DeltaReplication
	priceDiscovery    *PriceDiscovery
	capabilityMatcher *CapabilityMatcher

	// Epoch tracking
	currentEpoch uint32
	lastUpdate   time.Time

	// Configuration
	EpochDuration time.Duration // How often to trigger epoch updates
	logger        *slog.Logger

	// Metrics
	epochCount      uint64
	optimizations   uint64
	tierTransitions uint64
}

// NewEpochAwareOptimizer creates a new epoch-aware optimizer
func NewEpochAwareOptimizer(epochDuration time.Duration, logger *slog.Logger) *EpochAwareOptimizer {
	if logger == nil {
		logger = slog.Default()
	}

	if epochDuration == 0 {
		epochDuration = 5 * time.Second // Default: 5 second epochs
	}

	return &EpochAwareOptimizer{
		tierDetector:      NewTierDetector(),
		schedulerScoring:  NewSchedulerScoring(),
		deltaReplication:  NewDeltaReplication(4096),
		priceDiscovery:    NewPriceDiscovery(),
		capabilityMatcher: NewCapabilityMatcher(),
		EpochDuration:     epochDuration,
		logger:            logger.With("component", "epoch_optimizer"),
		lastUpdate:        time.Now(),
	}
}

// OnEpochBoundary is called when an epoch boundary is crossed
// This is where all optimization decisions are made for biological coherence
func (eo *EpochAwareOptimizer) OnEpochBoundary(epoch uint32) {
	eo.mu.Lock()
	defer eo.mu.Unlock()

	eo.currentEpoch = epoch
	eo.epochCount++
	eo.lastUpdate = time.Now()

	eo.logger.Debug("epoch boundary crossed",
		"epoch", epoch,
		"duration_since_last", time.Since(eo.lastUpdate))

	// Perform all optimization updates atomically on epoch boundary
	eo.updateTiers()
	eo.adjustPrices()
	eo.cleanupOldData()
}

// updateTiers updates all content tiers based on access patterns
func (eo *EpochAwareOptimizer) updateTiers() {
	// This would iterate through tracked content
	// For now, this is a hook for the mesh coordinator to call
	eo.optimizations++
}

// adjustPrices recalculates prices based on current supply/demand
func (eo *EpochAwareOptimizer) adjustPrices() {
	// Market adjustments happen on epoch boundaries for consistency
	metrics := eo.priceDiscovery.GetMarketMetrics()
	eo.logger.Debug("market update",
		"tracked_content", metrics["tracked_content"],
		"avg_price", metrics["average_price"])
}

// cleanupOldData removes stale access history
func (eo *EpochAwareOptimizer) cleanupOldData() {
	// Cleanup every 100 epochs (configurable)
	if eo.epochCount%100 == 0 {
		removed := eo.tierDetector.Cleanup(24 * time.Hour)
		eo.logger.Info("cleanup completed",
			"removed_entries", removed,
			"epoch", eo.currentEpoch)
	}
}

// RecordAccess records content access (happens continuously, not on epoch boundary)
func (eo *EpochAwareOptimizer) RecordAccess(contentHash string) {
	eo.tierDetector.RecordAccess(contentHash)
	eo.priceDiscovery.RecordDemand(contentHash)
}

// UpdateSupply updates supply metrics (happens continuously)
func (eo *EpochAwareOptimizer) UpdateSupply(contentHash string, replicaCount int, nodes []string) {
	eo.priceDiscovery.UpdateSupply(contentHash, replicaCount, nodes)
}

// GetOptimalNode returns the best node for content based on all factors
// This is epoch-aware: decisions are stable within an epoch
func (eo *EpochAwareOptimizer) GetOptimalNode(
	contentHash string,
	required []string,
	candidates []NodeCandidate,
	localGeohash string,
) *NodeCandidate {
	eo.mu.RLock()
	defer eo.mu.RUnlock()

	if len(candidates) == 0 {
		return nil
	}

	// Get current tier and price (stable within epoch)
	tier := eo.tierDetector.GetTier(contentHash)
	price := eo.priceDiscovery.CalculatePrice(contentHash)

	var bestNode *NodeCandidate
	var bestScore float64

	for i := range candidates {
		candidate := &candidates[i]

		// Calculate capability match
		capScore := eo.capabilityMatcher.MatchScore(required, candidate.Capabilities)

		// Calculate overall score
		nodeScore := eo.schedulerScoring.ScoreNode(
			candidate.NodeID,
			candidate.LatencyMs,
			tier.AccessCost(),
			capScore,
			candidate.Reputation,
			candidate.Geohash,
			localGeohash,
		)

		// Adjust score based on price (economic pressure)
		score := nodeScore.TotalScore * (1.0 / (1.0 + price/10.0))

		if score > bestScore {
			bestScore = score
			bestNode = candidate
		}
	}

	eo.logger.Debug("node selection",
		"content", contentHash[:8],
		"tier", tier,
		"price", price,
		"selected", bestNode.NodeID[:8],
		"score", bestScore,
		"epoch", eo.currentEpoch)

	return bestNode
}

// ShouldReplicate determines if content should be replicated based on tier
// Decision is stable within an epoch
func (eo *EpochAwareOptimizer) ShouldReplicate(contentHash string, currentReplicas int) (bool, ReplicationTier) {
	eo.mu.RLock()
	defer eo.mu.RUnlock()

	tier := eo.tierDetector.GetTier(contentHash)
	targetReplicas := tier.ReplicaCount()

	shouldReplicate := currentReplicas < targetReplicas

	if shouldReplicate {
		eo.logger.Debug("replication needed",
			"content", contentHash[:8],
			"tier", tier,
			"current", currentReplicas,
			"target", targetReplicas,
			"epoch", eo.currentEpoch)
	}

	return shouldReplicate, tier
}

// CalculateDelta calculates delta for efficient replication
func (eo *EpochAwareOptimizer) CalculateDelta(contentHash string, localData, remoteData []byte) ([]ContentMerkleLeaf, float64) {
	tree1 := eo.deltaReplication.BuildMerkleTree(contentHash+"_local", localData)
	tree2 := eo.deltaReplication.BuildMerkleTree(contentHash+"_remote", remoteData)

	diff := eo.deltaReplication.DiffTrees(tree1, tree2)
	chunks := eo.deltaReplication.GetChunks(tree2, diff)

	_, savings := eo.deltaReplication.CalculateBandwidthSavings(len(localData), diff)

	return chunks, savings
}

// GetMetrics returns comprehensive optimizer metrics
func (eo *EpochAwareOptimizer) GetMetrics() map[string]interface{} {
	eo.mu.RLock()
	defer eo.mu.RUnlock()

	return map[string]interface{}{
		"current_epoch":     eo.currentEpoch,
		"epoch_count":       eo.epochCount,
		"optimizations":     eo.optimizations,
		"tier_transitions":  eo.tierTransitions,
		"epoch_duration_ms": eo.EpochDuration.Milliseconds(),
		"last_update":       eo.lastUpdate.Format(time.RFC3339),

		// Component metrics
		"tier_detector":      eo.tierDetector.GetMetrics(),
		"scheduler_scoring":  eo.schedulerScoring.GetMetrics(),
		"delta_replication":  eo.deltaReplication.GetMetrics(),
		"price_discovery":    eo.priceDiscovery.GetMarketMetrics(),
		"capability_matcher": eo.capabilityMatcher.GetMetrics(),
	}
}

// NodeCandidate represents a candidate node for selection
type NodeCandidate struct {
	NodeID       string
	LatencyMs    float64
	Capabilities []string
	Reputation   float64
	Geohash      string
}

// EpochTicker manages epoch boundaries
type EpochTicker struct {
	ticker    *time.Ticker
	optimizer *EpochAwareOptimizer
	stop      chan struct{}
	logger    *slog.Logger
}

// NewEpochTicker creates a new epoch ticker
func NewEpochTicker(optimizer *EpochAwareOptimizer, logger *slog.Logger) *EpochTicker {
	return &EpochTicker{
		optimizer: optimizer,
		stop:      make(chan struct{}),
		logger:    logger,
	}
}

// Start begins the epoch ticker
func (et *EpochTicker) Start(ctx context.Context) {
	et.ticker = time.NewTicker(et.optimizer.EpochDuration)

	go func() {
		var epoch uint32
		for {
			select {
			case <-et.ticker.C:
				epoch++
				et.optimizer.OnEpochBoundary(epoch)

			case <-et.stop:
				et.ticker.Stop()
				return

			case <-ctx.Done():
				et.ticker.Stop()
				return
			}
		}
	}()

	et.logger.Info("epoch ticker started",
		"duration", et.optimizer.EpochDuration)
}

// Stop stops the epoch ticker
func (et *EpochTicker) Stop() {
	close(et.stop)
}
