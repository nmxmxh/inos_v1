package optimization

import (
	"testing"
)

func TestCapabilityMatcher_ExactMatch(t *testing.T) {
	cm := NewCapabilityMatcher()

	required := []string{"gpu", "storage"}
	available := []string{"gpu", "storage", "cpu"}

	score := cm.MatchScore(required, available)

	if score != 1.0 {
		t.Errorf("Expected perfect score for exact match, got %f", score)
	}
}

func TestCapabilityMatcher_FuzzyAlias(t *testing.T) {
	cm := NewCapabilityMatcher()

	required := []string{"graphics"} // Alias for "gpu"
	available := []string{"gpu"}

	score := cm.MatchScore(required, available)

	if score < 0.8 {
		t.Errorf("Expected high score for alias match, got %f", score)
	}

	t.Logf("Fuzzy alias score: %f", score)
}

func TestCapabilityMatcher_GraphInference(t *testing.T) {
	cm := NewCapabilityMatcher()

	required := []string{"gpu"}
	available := []string{"cuda"} // cuda is related to gpu

	score := cm.MatchScore(required, available)

	if score < 0.6 {
		t.Errorf("Expected decent score for graph inference, got %f", score)
	}

	t.Logf("Graph inference score: %f", score)
}

func TestCapabilityMatcher_NoMatch(t *testing.T) {
	cm := NewCapabilityMatcher()

	required := []string{"quantum"}
	available := []string{"gpu", "cpu"}

	score := cm.MatchScore(required, available)

	if score > 0.1 {
		t.Errorf("Expected low score for no match, got %f", score)
	}
}

func TestCapabilityMatcher_PartialMatch(t *testing.T) {
	cm := NewCapabilityMatcher()

	required := []string{"gpu", "quantum", "storage"}
	available := []string{"gpu", "storage"}

	score := cm.MatchScore(required, available)

	// 2 out of 3 match
	expectedScore := 2.0 / 3.0
	if score < expectedScore-0.1 || score > expectedScore+0.1 {
		t.Errorf("Expected score ~%f for partial match, got %f", expectedScore, score)
	}

	t.Logf("Partial match score: %f", score)
}

func TestCapabilityMatcher_RelatedCapabilities(t *testing.T) {
	cm := NewCapabilityMatcher()

	related := cm.GetRelatedCapabilities("gpu")

	if len(related) == 0 {
		t.Error("Expected related capabilities for gpu")
	}

	// Should include cuda, opencl, webgpu
	hasRelated := false
	for _, r := range related {
		if r == "cuda" || r == "opencl" || r == "webgpu" {
			hasRelated = true
			break
		}
	}

	if !hasRelated {
		t.Errorf("Expected gpu to have cuda/opencl/webgpu as related, got %v", related)
	}

	t.Logf("GPU related capabilities: %v", related)
}

func TestCapabilityMatcher_HasCapability(t *testing.T) {
	cm := NewCapabilityMatcher()

	available := []string{"gpu", "storage"}

	tests := []struct {
		required string
		expected bool
	}{
		{"gpu", true},
		{"graphics", true}, // Alias
		{"cuda", true},     // Related
		{"quantum", false},
	}

	for _, tt := range tests {
		t.Run(tt.required, func(t *testing.T) {
			has := cm.HasCapability(tt.required, available)
			if has != tt.expected {
				t.Errorf("HasCapability(%s): expected %v, got %v", tt.required, tt.expected, has)
			}
		})
	}
}

func TestCapabilityMatcher_CustomGraph(t *testing.T) {
	cm := NewCapabilityMatcher()

	// Add custom relationship
	cm.AddCapabilityRelation("ml", "tensor")
	cm.AddCapabilityRelation("ml", "neural")

	required := []string{"ml"}
	available := []string{"tensor"}

	score := cm.MatchScore(required, available)

	if score < 0.6 {
		t.Errorf("Expected decent score for custom graph relation, got %f", score)
	}

	t.Logf("Custom graph score: %f", score)
}

func TestCapabilityMatcher_CustomAlias(t *testing.T) {
	cm := NewCapabilityMatcher()

	cm.AddAlias("ai", "ml")
	cm.AddCapabilityRelation("ml", "tensor")

	required := []string{"ai"}
	available := []string{"ml"}

	score := cm.MatchScore(required, available)

	if score < 0.8 {
		t.Errorf("Expected high score for custom alias, got %f", score)
	}

	t.Logf("Custom alias score: %f", score)
}

func TestCapabilityMatcher_CaseInsensitive(t *testing.T) {
	cm := NewCapabilityMatcher()

	required := []string{"GPU", "Storage"}
	available := []string{"gpu", "STORAGE"}

	score := cm.MatchScore(required, available)

	if score != 1.0 {
		t.Errorf("Expected perfect score for case-insensitive match, got %f", score)
	}
}

func TestCapabilityMatcher_EmptyRequirements(t *testing.T) {
	cm := NewCapabilityMatcher()

	required := []string{}
	available := []string{"gpu", "cpu"}

	score := cm.MatchScore(required, available)

	if score != 1.0 {
		t.Errorf("Expected perfect score for no requirements, got %f", score)
	}
}

func TestCapabilityMatcher_Metrics(t *testing.T) {
	cm := NewCapabilityMatcher()

	metrics := cm.GetMetrics()

	if metrics["graph_nodes"].(int) == 0 {
		t.Error("Expected non-zero graph nodes")
	}

	if metrics["aliases"].(int) == 0 {
		t.Error("Expected non-zero aliases")
	}

	t.Logf("Metrics: %+v", metrics)
}

func TestCapabilityMatcher_ConcurrentAccess(t *testing.T) {
	cm := NewCapabilityMatcher()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				cm.MatchScore([]string{"gpu"}, []string{"cuda"})
				cm.GetRelatedCapabilities("gpu")
				cm.HasCapability("storage", []string{"disk"})
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkCapabilityMatcher_ExactMatch(b *testing.B) {
	cm := NewCapabilityMatcher()
	required := []string{"gpu", "storage"}
	available := []string{"gpu", "storage", "cpu"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.MatchScore(required, available)
	}
}

func BenchmarkCapabilityMatcher_GraphInference(b *testing.B) {
	cm := NewCapabilityMatcher()
	required := []string{"gpu"}
	available := []string{"cuda"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.MatchScore(required, available)
	}
}

func BenchmarkCapabilityMatcher_ComplexMatch(b *testing.B) {
	cm := NewCapabilityMatcher()
	required := []string{"gpu", "simd", "storage", "compute"}
	available := []string{"cuda", "avx", "ssd", "cpu"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.MatchScore(required, available)
	}
}
