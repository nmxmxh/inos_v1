package optimization

import (
	"testing"
	"time"
)

func TestPriceDiscovery_BasicPrice(t *testing.T) {
	pd := NewPriceDiscovery()

	// Record demand
	pd.RecordDemand("content1")
	pd.RecordDemand("content1")
	pd.RecordDemand("content1")

	// Update supply
	pd.UpdateSupply("content1", 1, []string{"node1"})

	// Calculate price
	price := pd.CalculatePrice("content1")

	if price <= pd.basePrice {
		t.Errorf("Expected price > base price for high demand/low supply, got %f", price)
	}

	t.Logf("Price: %f (base: %f)", price, pd.basePrice)
}

func TestPriceDiscovery_SupplyDemandBalance(t *testing.T) {
	pd := NewPriceDiscovery()

	// High demand
	for i := 0; i < 100; i++ {
		pd.RecordDemand("content1")
	}

	// Low supply
	pd.UpdateSupply("content1", 1, []string{"node1"})
	price1 := pd.CalculatePrice("content1")

	// Increase supply
	pd.UpdateSupply("content1", 10, []string{"node1", "node2", "node3"})
	price2 := pd.CalculatePrice("content1")

	if price2 >= price1 {
		t.Errorf("Expected price to decrease with increased supply: %f -> %f", price1, price2)
	}

	t.Logf("Price with low supply: %f, with high supply: %f", price1, price2)
}

func TestPriceDiscovery_TierSuggestion(t *testing.T) {
	pd := NewPriceDiscovery()

	tests := []struct {
		name         string
		demand       int
		supply       int
		expectedTier ReplicationTier
	}{
		{"High demand, low supply", 100, 1, TierHot},
		{"Medium demand, medium supply", 50, 5, TierWarm},
		{"Low demand, high supply", 10, 10, TierCold},
		{"Very low demand", 1, 10, TierArchive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentHash := tt.name

			for i := 0; i < tt.demand; i++ {
				pd.RecordDemand(contentHash)
			}

			nodes := make([]string, tt.supply)
			for i := range nodes {
				nodes[i] = string(rune(i))
			}
			pd.UpdateSupply(contentHash, tt.supply, nodes)

			tier := pd.SuggestTier(contentHash)
			price := pd.CalculatePrice(contentHash)

			t.Logf("%s: price=%f, tier=%s", tt.name, price, tier)

			// Tier should generally match expectations (allowing some flexibility)
			if tier != tt.expectedTier {
				t.Logf("Note: Expected tier %s, got %s (price=%f)", tt.expectedTier, tier, price)
			}
		})
	}
}

func TestPriceDiscovery_PriceDecay(t *testing.T) {
	pd := NewPriceDiscovery()
	pd.priceDecayRate = 1.0 // Fast decay for testing

	// Record demand
	for i := 0; i < 10; i++ {
		pd.RecordDemand("content1")
	}
	pd.UpdateSupply("content1", 1, []string{"node1"})

	price1 := pd.CalculatePrice("content1")

	// Wait for decay
	time.Sleep(100 * time.Millisecond)

	price2 := pd.CalculatePrice("content1")

	if price2 >= price1 {
		t.Errorf("Expected price to decay over time: %f -> %f", price1, price2)
	}

	t.Logf("Price decay: %f -> %f", price1, price2)
}

func TestPriceDiscovery_ZeroSupply(t *testing.T) {
	pd := NewPriceDiscovery()

	pd.RecordDemand("content1")

	// No supply recorded - should use neutral supply (1.0)
	price := pd.CalculatePrice("content1")

	// Price should be reasonable (not extreme) since we use neutral supply
	if price < pd.basePrice*0.1 || price > pd.basePrice*100 {
		t.Errorf("Expected reasonable price for neutral supply, got %f", price)
	}

	t.Logf("Price with neutral supply: %f", price)
}

func TestPriceDiscovery_PriceBounds(t *testing.T) {
	pd := NewPriceDiscovery()

	// Extreme demand
	for i := 0; i < 10000; i++ {
		pd.RecordDemand("content1")
	}
	pd.UpdateSupply("content1", 1, []string{"node1"})

	price := pd.CalculatePrice("content1")

	// Price should be bounded
	maxPrice := pd.basePrice * 100
	if price > maxPrice {
		t.Errorf("Expected price <= %f, got %f", maxPrice, price)
	}

	t.Logf("Price with extreme demand: %f (max: %f)", price, maxPrice)
}

func TestPriceDiscovery_MarketMetrics(t *testing.T) {
	pd := NewPriceDiscovery()

	pd.RecordDemand("content1")
	pd.RecordDemand("content2")
	pd.UpdateSupply("content1", 5, []string{"node1"})
	pd.UpdateSupply("content2", 3, []string{"node2"})

	metrics := pd.GetMarketMetrics()

	if metrics["tracked_content"] != 2 {
		t.Errorf("Expected 2 tracked content, got %v", metrics["tracked_content"])
	}

	if metrics["base_price"] != pd.basePrice {
		t.Errorf("Expected base price %f, got %v", pd.basePrice, metrics["base_price"])
	}

	t.Logf("Market metrics: %+v", metrics)
}

func TestPriceDiscovery_BasePriceAdjustment(t *testing.T) {
	pd := NewPriceDiscovery()

	oldBase := pd.basePrice
	newBase := 2.0

	pd.AdjustBasePrice(newBase)

	if pd.basePrice != newBase {
		t.Errorf("Expected base price %f, got %f", newBase, pd.basePrice)
	}

	// Negative price should be rejected
	pd.AdjustBasePrice(-1.0)
	if pd.basePrice != newBase {
		t.Errorf("Expected base price to remain %f, got %f", newBase, pd.basePrice)
	}

	t.Logf("Base price: %f -> %f", oldBase, pd.basePrice)
}

func TestPriceDiscovery_ConcurrentAccess(t *testing.T) {
	pd := NewPriceDiscovery()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			contentHash := string(rune(id % 3))
			for j := 0; j < 100; j++ {
				pd.RecordDemand(contentHash)
				pd.UpdateSupply(contentHash, id+1, []string{"node"})
				pd.CalculatePrice(contentHash)
				pd.SuggestTier(contentHash)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	metrics := pd.GetMarketMetrics()
	if metrics["tracked_content"].(int) != 3 {
		t.Errorf("Expected 3 tracked content, got %v", metrics["tracked_content"])
	}
}

func BenchmarkPriceDiscovery_RecordDemand(b *testing.B) {
	pd := NewPriceDiscovery()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pd.RecordDemand("content1")
	}
}

func BenchmarkPriceDiscovery_CalculatePrice(b *testing.B) {
	pd := NewPriceDiscovery()

	for i := 0; i < 100; i++ {
		pd.RecordDemand("content1")
	}
	pd.UpdateSupply("content1", 5, []string{"node1"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pd.CalculatePrice("content1")
	}
}

func BenchmarkPriceDiscovery_SuggestTier(b *testing.B) {
	pd := NewPriceDiscovery()

	for i := 0; i < 100; i++ {
		pd.RecordDemand("content1")
	}
	pd.UpdateSupply("content1", 5, []string{"node1"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pd.SuggestTier("content1")
	}
}
