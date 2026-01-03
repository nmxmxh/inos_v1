package optimization

import (
	"sync"
	"time"
)

// PriceDiscovery manages market-driven tier adjustment based on supply and demand
type PriceDiscovery struct {
	mu sync.RWMutex

	// Demand tracking: contentHash -> access count
	demand map[string]*DemandMetrics

	// Supply tracking: contentHash -> replica count
	supply map[string]*SupplyMetrics

	// Price history: contentHash -> price
	prices map[string]float64

	// Configuration
	basePrice      float64
	priceDecayRate float64 // How fast prices adjust
}

// DemandMetrics tracks demand for content
type DemandMetrics struct {
	AccessCount    uint64
	LastAccessTime int64
	AccessRate     float64 // Accesses per second
}

// SupplyMetrics tracks supply of content
type SupplyMetrics struct {
	ReplicaCount   int
	AvailableNodes []string
	LastUpdated    int64
}

// NewPriceDiscovery creates a new price discovery system
func NewPriceDiscovery() *PriceDiscovery {
	return &PriceDiscovery{
		demand:         make(map[string]*DemandMetrics),
		supply:         make(map[string]*SupplyMetrics),
		prices:         make(map[string]float64),
		basePrice:      1.0,
		priceDecayRate: 0.1,
	}
}

// RecordDemand records an access to content
func (pd *PriceDiscovery) RecordDemand(contentHash string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	metrics, exists := pd.demand[contentHash]
	if !exists {
		metrics = &DemandMetrics{}
		pd.demand[contentHash] = metrics
	}

	metrics.AccessCount++
	metrics.LastAccessTime = time.Now().UnixNano()
}

// UpdateSupply updates the supply metrics for content
func (pd *PriceDiscovery) UpdateSupply(contentHash string, replicaCount int, nodes []string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.supply[contentHash] = &SupplyMetrics{
		ReplicaCount:   replicaCount,
		AvailableNodes: nodes,
		LastUpdated:    time.Now().UnixNano(),
	}
}

// CalculatePrice calculates the market price for content based on supply/demand
func (pd *PriceDiscovery) CalculatePrice(contentHash string) float64 {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	demand := pd.getDemandScore(contentHash)
	supply := pd.getSupplyScore(contentHash)

	if supply == 0 {
		return pd.basePrice * 10 // High price for scarce content
	}

	// Price = basePrice * (demand / supply)
	price := pd.basePrice * (demand / supply)

	// Apply bounds
	if price < pd.basePrice*0.1 {
		price = pd.basePrice * 0.1
	}
	if price > pd.basePrice*100 {
		price = pd.basePrice * 100
	}

	return price
}

// getDemandScore calculates demand score (unlocked version)
func (pd *PriceDiscovery) getDemandScore(contentHash string) float64 {
	metrics, exists := pd.demand[contentHash]
	if !exists {
		return 1.0 // Neutral demand
	}

	// Calculate time-weighted demand
	now := time.Now().UnixNano()
	age := float64(now-metrics.LastAccessTime) / float64(time.Hour)

	// Decay demand over time
	decayFactor := 1.0 / (1.0 + age*pd.priceDecayRate)

	return float64(metrics.AccessCount) * decayFactor
}

// getSupplyScore calculates supply score (unlocked version)
func (pd *PriceDiscovery) getSupplyScore(contentHash string) float64 {
	metrics, exists := pd.supply[contentHash]
	if !exists {
		return 1.0 // Neutral supply
	}

	return float64(metrics.ReplicaCount)
}

// SuggestTier suggests a tier based on current price
func (pd *PriceDiscovery) SuggestTier(contentHash string) ReplicationTier {
	price := pd.CalculatePrice(contentHash)

	// Higher price = higher demand = higher tier
	if price >= pd.basePrice*10 {
		return TierHot
	} else if price >= pd.basePrice*5 {
		return TierWarm
	} else if price >= pd.basePrice {
		return TierCold
	}
	return TierArchive
}

// GetMarketMetrics returns market metrics
func (pd *PriceDiscovery) GetMarketMetrics() map[string]interface{} {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	var totalDemand, totalSupply, avgPrice float64
	priceCount := 0

	for contentHash := range pd.demand {
		totalDemand += pd.getDemandScore(contentHash)
		totalSupply += pd.getSupplyScore(contentHash)

		price := pd.CalculatePrice(contentHash)
		avgPrice += price
		priceCount++
	}

	if priceCount > 0 {
		avgPrice /= float64(priceCount)
	}

	return map[string]interface{}{
		"tracked_content": len(pd.demand),
		"total_demand":    totalDemand,
		"total_supply":    totalSupply,
		"average_price":   avgPrice,
		"base_price":      pd.basePrice,
	}
}

// AdjustBasePrice adjusts the base price (for market regulation)
func (pd *PriceDiscovery) AdjustBasePrice(newBasePrice float64) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	if newBasePrice > 0 {
		pd.basePrice = newBasePrice
	}
}
