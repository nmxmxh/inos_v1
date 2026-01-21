package optimization

import (
	"sync"
)

// SchedulerScoring provides multi-factor scoring for node selection
type SchedulerScoring struct {
	mu sync.RWMutex

	// Weights for scoring factors
	latencyWeight    float64
	costWeight       float64
	capabilityWeight float64
	reputationWeight float64
	geohashWeight    float64

	// Node performance history
	nodeLatencies map[string][]float64 // nodeID -> recent latencies
	nodeSuccesses map[string]int       // nodeID -> success count
	nodeFailures  map[string]int       // nodeID -> failure count
}

// NodeScore represents a scored node
type NodeScore struct {
	NodeID           string
	TotalScore       float64
	LatencyScore     float64
	CostScore        float64
	CapabilityScore  float64
	ReputationScore  float64
	GeohashScore     float64
	AverageLatencyMs float64
	SuccessRate      float64
}

// NewSchedulerScoring creates a new scheduler scoring system
func NewSchedulerScoring() *SchedulerScoring {
	return &SchedulerScoring{
		latencyWeight:    0.3,
		costWeight:       0.15,
		capabilityWeight: 0.25,
		reputationWeight: 0.1,
		geohashWeight:    0.2, // Increased for geographic proximity importance
		nodeLatencies:    make(map[string][]float64),
		nodeSuccesses:    make(map[string]int),
		nodeFailures:     make(map[string]int),
	}
}

// SetWeights updates the scoring weights
func (ss *SchedulerScoring) SetWeights(latency, cost, capability, reputation, geohash float64) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	total := latency + cost + capability + reputation + geohash
	if total == 0 {
		return
	}

	// Normalize weights
	ss.latencyWeight = latency / total
	ss.costWeight = cost / total
	ss.capabilityWeight = capability / total
	ss.reputationWeight = reputation / total
	ss.geohashWeight = geohash / total
}

// ScoreNode calculates a comprehensive score for a node
func (ss *SchedulerScoring) ScoreNode(
	nodeID string,
	latencyMs float64,
	costCredits uint64,
	capabilityMatch float64,
	reputation float64,
	nodeGeohash string,
	localGeohash string,
) NodeScore {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	score := NodeScore{
		NodeID: nodeID,
	}

	// 1. Latency score (inverse, lower is better)
	score.LatencyScore = ss.calculateLatencyScore(latencyMs)
	score.AverageLatencyMs = latencyMs

	// 2. Cost score (inverse, lower cost is better)
	score.CostScore = ss.calculateCostScore(costCredits)

	// 3. Capability match score (0.0-1.0)
	score.CapabilityScore = capabilityMatch

	// 4. Reputation score (0.0-1.0)
	score.ReputationScore = reputation

	// 5. Geohash proximity score
	score.GeohashScore = ss.calculateGeohashScore(nodeGeohash, localGeohash)

	// Calculate total weighted score
	score.TotalScore =
		score.LatencyScore*ss.latencyWeight +
			score.CostScore*ss.costWeight +
			score.CapabilityScore*ss.capabilityWeight +
			score.ReputationScore*ss.reputationWeight +
			score.GeohashScore*ss.geohashWeight

	// Calculate success rate
	successes := ss.nodeSuccesses[nodeID]
	failures := ss.nodeFailures[nodeID]
	total := successes + failures
	if total > 0 {
		score.SuccessRate = float64(successes) / float64(total)
	} else {
		score.SuccessRate = 0.5 // Neutral for unknown nodes
	}

	return score
}

// calculateLatencyScore converts latency to a 0.0-1.0 score
func (ss *SchedulerScoring) calculateLatencyScore(latencyMs float64) float64 {
	if latencyMs <= 0 {
		return 1.0
	}
	if latencyMs >= 1000 {
		return 0.01
	}
	// Exponential decay: score = e^(-latency/200)
	return 1.0 / (1.0 + latencyMs/200.0)
}

// calculateCostScore converts cost to a 0.0-1.0 score
func (ss *SchedulerScoring) calculateCostScore(costCredits uint64) float64 {
	if costCredits == 0 {
		return 1.0
	}
	if costCredits >= 1000 {
		return 0.01
	}
	// Inverse relationship
	return 1.0 / (1.0 + float64(costCredits)/100.0)
}

// calculateGeohashScore calculates proximity score based on geohash distance
func (ss *SchedulerScoring) calculateGeohashScore(nodeGeohash, localGeohash string) float64 {
	if nodeGeohash == "" || localGeohash == "" {
		return 0.5 // Neutral for unknown
	}

	// Calculate approximate distance
	distanceKm := GeohashDistance(nodeGeohash, localGeohash)

	// Convert distance to score (0.0-1.0)
	// <10km = 1.0, <50km = 0.8, <200km = 0.6, <1000km = 0.3, >1000km = 0.1
	if distanceKm < 10 {
		return 1.0
	} else if distanceKm < 50 {
		return 0.9
	} else if distanceKm < 200 {
		return 0.7
	} else if distanceKm < 500 {
		return 0.5
	} else if distanceKm < 1000 {
		return 0.3
	} else if distanceKm < 2000 {
		return 0.2
	}
	return 0.1
}

// RecordLatency records a latency measurement for a node
func (ss *SchedulerScoring) RecordLatency(nodeID string, latencyMs float64) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	latencies := ss.nodeLatencies[nodeID]
	latencies = append(latencies, latencyMs)

	// Keep last 100 measurements
	if len(latencies) > 100 {
		latencies = latencies[len(latencies)-100:]
	}

	ss.nodeLatencies[nodeID] = latencies
}

// RecordSuccess records a successful interaction with a node
func (ss *SchedulerScoring) RecordSuccess(nodeID string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.nodeSuccesses[nodeID]++
}

// RecordFailure records a failed interaction with a node
func (ss *SchedulerScoring) RecordFailure(nodeID string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.nodeFailures[nodeID]++
}

// GetAverageLatency returns the average latency for a node
func (ss *SchedulerScoring) GetAverageLatency(nodeID string) float64 {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	latencies := ss.nodeLatencies[nodeID]
	if len(latencies) == 0 {
		return 0
	}

	var sum float64
	for _, lat := range latencies {
		sum += lat
	}
	return sum / float64(len(latencies))
}

// GetSuccessRate returns the success rate for a node
func (ss *SchedulerScoring) GetSuccessRate(nodeID string) float64 {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	successes := ss.nodeSuccesses[nodeID]
	failures := ss.nodeFailures[nodeID]
	total := successes + failures

	if total == 0 {
		return 0.5 // Neutral for unknown
	}

	return float64(successes) / float64(total)
}

// GetMetrics returns scoring metrics
func (ss *SchedulerScoring) GetMetrics() map[string]interface{} {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	return map[string]interface{}{
		"tracked_nodes":     len(ss.nodeLatencies),
		"latency_weight":    ss.latencyWeight,
		"cost_weight":       ss.costWeight,
		"capability_weight": ss.capabilityWeight,
		"reputation_weight": ss.reputationWeight,
		"geohash_weight":    ss.geohashWeight,
	}
}

// GeohashFromLocation generates a production-grade geohash from lat/lon
// Geohash provides hierarchical spatial indexing where nearby locations
// share common prefixes, making it ideal for proximity-based routing
func GeohashFromLocation(lat, lon float64, precision int) string {
	if precision <= 0 || precision > 12 {
		precision = 8 // Default precision (~19m x 19m)
	}

	const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"

	latMin, latMax := -90.0, 90.0
	lonMin, lonMax := -180.0, 180.0

	var geohash []byte
	var bits uint
	var bit uint

	for len(geohash) < precision {
		if bit%2 == 0 {
			// Even bit: longitude
			mid := (lonMin + lonMax) / 2
			if lon > mid {
				bits |= (1 << (4 - (bit / 2)))
				lonMin = mid
			} else {
				lonMax = mid
			}
		} else {
			// Odd bit: latitude
			mid := (latMin + latMax) / 2
			if lat > mid {
				bits |= (1 << (4 - (bit / 2)))
				latMin = mid
			} else {
				latMax = mid
			}
		}

		bit++

		if bit == 10 {
			geohash = append(geohash, base32[bits])
			bits = 0
			bit = 0
		}
	}

	return string(geohash)
}

// GeohashDistance calculates approximate distance in km between two geohashes
func GeohashDistance(hash1, hash2 string) float64 {
	// Count matching prefix
	minLen := len(hash1)
	if len(hash2) < minLen {
		minLen = len(hash2)
	}

	matchingChars := 0
	for i := 0; i < minLen; i++ {
		if hash1[i] == hash2[i] {
			matchingChars++
		} else {
			break
		}
	}

	// Approximate distance based on precision
	// Each geohash character adds ~5x precision
	distances := []float64{
		5000.0, // 0 chars: ~5000 km
		1250.0, // 1 char: ~1250 km
		156.0,  // 2 chars: ~156 km
		39.0,   // 3 chars: ~39 km
		4.9,    // 4 chars: ~4.9 km
		1.2,    // 5 chars: ~1.2 km
		0.15,   // 6 chars: ~150 m
		0.038,  // 7 chars: ~38 m
		0.0047, // 8 chars: ~4.7 m
	}

	if matchingChars >= len(distances) {
		return 0.001 // Very close
	}

	return distances[matchingChars]
}
