package internal

import (
	"sync"
	"time"
)

// DemandTracker tracks access patterns for chunks to calculate demand scores
type DemandTracker struct {
	mu            sync.RWMutex
	accessCounts  map[string]*AccessStats
	decayInterval time.Duration
	lastDecay     time.Time
}

// AccessStats tracks access statistics for a chunk
type AccessStats struct {
	TotalAccesses  uint64
	RecentAccesses uint64
	LastAccess     time.Time
	DemandScore    float64 // 0.0-1.0
}

// NewDemandTracker creates a new demand tracker
func NewDemandTracker() *DemandTracker {
	return &DemandTracker{
		accessCounts:  make(map[string]*AccessStats),
		decayInterval: 1 * time.Hour,
		lastDecay:     time.Now(),
	}
}

// RecordAccess records an access to a chunk
func (dt *DemandTracker) RecordAccess(chunkHash string) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	stats, exists := dt.accessCounts[chunkHash]
	if !exists {
		stats = &AccessStats{
			LastAccess: time.Now(),
		}
		dt.accessCounts[chunkHash] = stats
	}

	stats.TotalAccesses++
	stats.RecentAccesses++
	stats.LastAccess = time.Now()

	// Update demand score based on access frequency
	dt.updateDemandScore(stats)

	// Perform decay if needed
	if time.Since(dt.lastDecay) > dt.decayInterval {
		dt.decayAll()
	}
}

// GetDemandScore returns the demand score for a chunk (0.0-1.0)
func (dt *DemandTracker) GetDemandScore(chunkHash string) float64 {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	stats, exists := dt.accessCounts[chunkHash]
	if !exists {
		return 0.0
	}

	// Adjust score based on recency
	timeSinceAccess := time.Since(stats.LastAccess)
	recencyFactor := 1.0
	if timeSinceAccess > 24*time.Hour {
		recencyFactor = 0.5
	} else if timeSinceAccess > time.Hour {
		recencyFactor = 0.8
	}

	return stats.DemandScore * recencyFactor
}

// updateDemandScore calculates demand score based on access patterns
func (dt *DemandTracker) updateDemandScore(stats *AccessStats) {
	// Score based on recent access frequency
	// 1-5 accesses = 0.2
	// 6-20 accesses = 0.5
	// 21-100 accesses = 0.8
	// 100+ accesses = 1.0

	switch {
	case stats.RecentAccesses >= 100:
		stats.DemandScore = 1.0
	case stats.RecentAccesses >= 21:
		stats.DemandScore = 0.8
	case stats.RecentAccesses >= 6:
		stats.DemandScore = 0.5
	case stats.RecentAccesses >= 1:
		stats.DemandScore = 0.2
	default:
		stats.DemandScore = 0.0
	}
}

// decayAll applies decay to all tracked chunks
func (dt *DemandTracker) decayAll() {
	for _, stats := range dt.accessCounts {
		// Decay recent accesses by 50%
		stats.RecentAccesses = stats.RecentAccesses / 2
		dt.updateDemandScore(stats)
	}
	dt.lastDecay = time.Now()
}

// GetStats returns demand statistics
func (dt *DemandTracker) GetStats() map[string]interface{} {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	highDemand := 0
	mediumDemand := 0
	lowDemand := 0

	for hash := range dt.accessCounts {
		score := dt.GetDemandScore(hash)
		if score >= 0.7 {
			highDemand++
		} else if score >= 0.3 {
			mediumDemand++
		} else {
			lowDemand++
		}
	}

	return map[string]interface{}{
		"total_tracked": len(dt.accessCounts),
		"high_demand":   highDemand,
		"medium_demand": mediumDemand,
		"low_demand":    lowDemand,
	}
}

// Cleanup removes old entries
func (dt *DemandTracker) Cleanup(maxAge time.Duration) int {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	removed := 0
	now := time.Now()

	for hash, stats := range dt.accessCounts {
		if now.Sub(stats.LastAccess) > maxAge {
			delete(dt.accessCounts, hash)
			removed++
		}
	}

	return removed
}
