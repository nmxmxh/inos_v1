package optimization

import (
	"math"
	"sync"
	"time"
)

// TierDetector manages hot/cold tier detection with time-decay
type TierDetector struct {
	mu sync.RWMutex

	// Access history: contentHash -> []accessTime
	accessHistory map[string][]int64

	// Current tiers: contentHash -> tier
	tiers map[string]ReplicationTier

	// Configuration
	decayHalfLife time.Duration // Time for access weight to decay by half
	hotThreshold  float64       // Accesses/sec to promote to Hot
	warmThreshold float64       // Accesses/sec to promote to Warm
	coldThreshold float64       // Accesses/sec to demote to Cold
}

// ReplicationTier represents storage tier
type ReplicationTier int

const (
	TierArchive ReplicationTier = iota
	TierCold
	TierWarm
	TierHot
)

func (t ReplicationTier) String() string {
	switch t {
	case TierHot:
		return "hot"
	case TierWarm:
		return "warm"
	case TierCold:
		return "cold"
	case TierArchive:
		return "archive"
	default:
		return "unknown"
	}
}

// ReplicaCount returns the number of replicas for this tier
func (t ReplicationTier) ReplicaCount() int {
	switch t {
	case TierHot:
		return 10
	case TierWarm:
		return 5
	case TierCold:
		return 2
	case TierArchive:
		return 1
	default:
		return 1
	}
}

// AccessCost returns the credit cost per access
func (t ReplicationTier) AccessCost() uint64 {
	switch t {
	case TierHot:
		return 1
	case TierWarm:
		return 5
	case TierCold:
		return 20
	case TierArchive:
		return 100
	default:
		return 100
	}
}

// NewTierDetector creates a new tier detector
func NewTierDetector() *TierDetector {
	return &TierDetector{
		accessHistory: make(map[string][]int64),
		tiers:         make(map[string]ReplicationTier),
		decayHalfLife: 1 * time.Hour,
		hotThreshold:  0.5,   // >0.5 access/sec
		warmThreshold: 0.05,  // >0.05 access/sec
		coldThreshold: 0.005, // >0.005 access/sec
	}
}

// RecordAccess records an access to content
func (td *TierDetector) RecordAccess(contentHash string) {
	td.mu.Lock()
	defer td.mu.Unlock()

	now := time.Now().UnixNano()
	td.accessHistory[contentHash] = append(td.accessHistory[contentHash], now)

	// Limit history size (keep last 1000 accesses)
	if len(td.accessHistory[contentHash]) > 1000 {
		td.accessHistory[contentHash] = td.accessHistory[contentHash][len(td.accessHistory[contentHash])-1000:]
	}
}

// GetTier returns the current tier for content
func (td *TierDetector) GetTier(contentHash string) ReplicationTier {
	td.mu.RLock()
	defer td.mu.RUnlock()

	if tier, exists := td.tiers[contentHash]; exists {
		return tier
	}
	return TierCold // Default tier
}

// CalculateAccessRate calculates time-decayed access rate (accesses/sec)
func (td *TierDetector) CalculateAccessRate(contentHash string) float64 {
	td.mu.RLock()
	defer td.mu.RUnlock()

	history, exists := td.accessHistory[contentHash]
	if !exists || len(history) == 0 {
		return 0.0
	}

	now := time.Now().UnixNano()
	decayConstant := math.Log(2) / float64(td.decayHalfLife.Nanoseconds())

	var weightedAccesses float64
	for _, accessTime := range history {
		age := float64(now - accessTime)
		weight := math.Exp(-decayConstant * age)
		weightedAccesses += weight
	}

	// Convert to accesses per second
	// Weighted accesses over the decay window
	windowSeconds := td.decayHalfLife.Seconds() * 3 // 3x half-life window
	return weightedAccesses / windowSeconds
}

// ShouldPromote checks if content should be promoted to a higher tier
func (td *TierDetector) ShouldPromote(contentHash string) bool {
	currentTier := td.GetTier(contentHash)
	accessRate := td.CalculateAccessRate(contentHash)

	switch currentTier {
	case TierArchive:
		return accessRate > td.coldThreshold
	case TierCold:
		return accessRate > td.warmThreshold
	case TierWarm:
		return accessRate > td.hotThreshold
	case TierHot:
		return false // Already at highest tier
	}
	return false
}

// ShouldDemote checks if content should be demoted to a lower tier
func (td *TierDetector) ShouldDemote(contentHash string) bool {
	currentTier := td.GetTier(contentHash)
	accessRate := td.CalculateAccessRate(contentHash)

	// Don't demote if no access history
	td.mu.RLock()
	_, hasHistory := td.accessHistory[contentHash]
	td.mu.RUnlock()

	if !hasHistory {
		return false
	}

	switch currentTier {
	case TierHot:
		return accessRate < td.warmThreshold
	case TierWarm:
		return accessRate < td.coldThreshold
	case TierCold:
		return accessRate < (td.coldThreshold / 10) // Very low access
	case TierArchive:
		return false // Already at lowest tier
	}
	return false
}

// UpdateTier updates the tier for content based on access patterns
func (td *TierDetector) UpdateTier(contentHash string) (ReplicationTier, bool) {
	td.mu.Lock()
	defer td.mu.Unlock()

	currentTier, exists := td.tiers[contentHash]
	if !exists {
		currentTier = TierCold
	}

	accessRate := td.calculateAccessRateUnlocked(contentHash)

	var newTier ReplicationTier
	if accessRate > td.hotThreshold {
		newTier = TierHot
	} else if accessRate > td.warmThreshold {
		newTier = TierWarm
	} else if accessRate > td.coldThreshold {
		newTier = TierCold
	} else {
		newTier = TierArchive
	}

	changed := newTier != currentTier
	if changed {
		td.tiers[contentHash] = newTier
	}

	return newTier, changed
}

// calculateAccessRateUnlocked is the unlocked version for internal use
func (td *TierDetector) calculateAccessRateUnlocked(contentHash string) float64 {
	history, exists := td.accessHistory[contentHash]
	if !exists || len(history) == 0 {
		return 0.0
	}

	now := time.Now().UnixNano()
	decayConstant := math.Log(2) / float64(td.decayHalfLife.Nanoseconds())

	var weightedAccesses float64
	for _, accessTime := range history {
		age := float64(now - accessTime)
		weight := math.Exp(-decayConstant * age)
		weightedAccesses += weight
	}

	windowSeconds := td.decayHalfLife.Seconds() * 3
	return weightedAccesses / windowSeconds
}

// GetMetrics returns tier detection metrics
func (td *TierDetector) GetMetrics() map[string]interface{} {
	td.mu.RLock()
	defer td.mu.RUnlock()

	tierCounts := make(map[string]int)
	for _, tier := range td.tiers {
		tierCounts[tier.String()]++
	}

	return map[string]interface{}{
		"total_content":  len(td.tiers),
		"hot_count":      tierCounts["hot"],
		"warm_count":     tierCounts["warm"],
		"cold_count":     tierCounts["cold"],
		"archive_count":  tierCounts["archive"],
		"tracked_access": len(td.accessHistory),
	}
}

// Cleanup removes old access history
func (td *TierDetector) Cleanup(maxAge time.Duration) int {
	td.mu.Lock()
	defer td.mu.Unlock()

	cutoff := time.Now().Add(-maxAge).UnixNano()
	removed := 0

	for contentHash, history := range td.accessHistory {
		// Filter out old accesses
		newHistory := make([]int64, 0, len(history))
		for _, accessTime := range history {
			if accessTime > cutoff {
				newHistory = append(newHistory, accessTime)
			}
		}

		if len(newHistory) == 0 {
			delete(td.accessHistory, contentHash)
			delete(td.tiers, contentHash)
			removed++
		} else {
			td.accessHistory[contentHash] = newHistory
		}
	}

	return removed
}
