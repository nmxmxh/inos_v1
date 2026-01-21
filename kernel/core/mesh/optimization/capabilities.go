package optimization

import (
	"strings"
	"sync"
)

// CapabilityMatcher provides fuzzy matching and graph-based capability inference
type CapabilityMatcher struct {
	mu sync.RWMutex

	// Capability graph: capability -> related capabilities
	graph map[string][]string

	// Fuzzy aliases: alias -> canonical capability
	aliases map[string]string
}

// NewCapabilityMatcher creates a new capability matcher
func NewCapabilityMatcher() *CapabilityMatcher {
	cm := &CapabilityMatcher{
		graph:   make(map[string][]string),
		aliases: make(map[string]string),
	}

	// Initialize default capability graph
	cm.initializeDefaultGraph()

	return cm
}

// initializeDefaultGraph sets up common capability relationships
func (cm *CapabilityMatcher) initializeDefaultGraph() {
	// GPU capabilities
	cm.AddCapabilityRelation("gpu", "cuda")
	cm.AddCapabilityRelation("gpu", "opencl")
	cm.AddCapabilityRelation("gpu", "webgpu")
	cm.AddCapabilityRelation("cuda", "gpu")
	cm.AddCapabilityRelation("opencl", "gpu")

	// SIMD capabilities
	cm.AddCapabilityRelation("simd", "sse")
	cm.AddCapabilityRelation("simd", "avx")
	cm.AddCapabilityRelation("simd", "neon")
	cm.AddCapabilityRelation("avx", "sse")

	// Storage capabilities
	cm.AddCapabilityRelation("storage", "disk")
	cm.AddCapabilityRelation("storage", "ssd")
	cm.AddCapabilityRelation("ssd", "disk")

	// Compute capabilities
	cm.AddCapabilityRelation("compute", "cpu")
	cm.AddCapabilityRelation("compute", "gpu")

	// Fuzzy aliases
	cm.AddAlias("graphics", "gpu")
	cm.AddAlias("vector", "simd")
	cm.AddAlias("parallel", "simd")
	cm.AddAlias("nvid", "cuda")
	cm.AddAlias("nvidia", "cuda")
}

// AddCapabilityRelation adds a relationship in the capability graph
func (cm *CapabilityMatcher) AddCapabilityRelation(from, to string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	from = strings.ToLower(from)
	to = strings.ToLower(to)

	if cm.graph[from] == nil {
		cm.graph[from] = []string{}
	}

	// Avoid duplicates
	for _, existing := range cm.graph[from] {
		if existing == to {
			return
		}
	}

	cm.graph[from] = append(cm.graph[from], to)
}

// AddAlias adds a fuzzy alias for a capability
func (cm *CapabilityMatcher) AddAlias(alias, canonical string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.aliases[strings.ToLower(alias)] = strings.ToLower(canonical)
}

// MatchScore calculates a match score between required and available capabilities
func (cm *CapabilityMatcher) MatchScore(required []string, available []string) float64 {
	if len(required) == 0 {
		return 1.0 // No requirements = perfect match
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var totalScore float64
	for _, req := range required {
		req = strings.ToLower(req)
		score := cm.matchSingleCapability(req, available)
		totalScore += score
	}

	return totalScore / float64(len(required))
}

// matchSingleCapability matches a single required capability against available ones
func (cm *CapabilityMatcher) matchSingleCapability(required string, available []string) float64 {
	// 1. Exact match
	for _, avail := range available {
		if strings.ToLower(avail) == required {
			return 1.0
		}
	}

	// 2. Fuzzy match via aliases
	canonical := cm.aliases[required]
	if canonical != "" {
		for _, avail := range available {
			if strings.ToLower(avail) == canonical {
				return 0.9 // Slightly lower than exact
			}
		}
	}

	// 3. Graph-based inference
	related := cm.graph[required]
	for _, rel := range related {
		for _, avail := range available {
			if strings.ToLower(avail) == rel {
				return 0.7 // Lower score for inferred match
			}
		}
	}

	// 4. Substring match (fuzzy)
	for _, avail := range available {
		availLower := strings.ToLower(avail)
		if strings.Contains(availLower, required) || strings.Contains(required, availLower) {
			return 0.5 // Weak fuzzy match
		}
	}

	return 0.0 // No match
}

// GetRelatedCapabilities returns capabilities related to the given one
func (cm *CapabilityMatcher) GetRelatedCapabilities(capability string) []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	capability = strings.ToLower(capability)

	// Check aliases first
	if canonical := cm.aliases[capability]; canonical != "" {
		capability = canonical
	}

	related := cm.graph[capability]
	if related == nil {
		return []string{}
	}

	// Return a copy to avoid race conditions
	result := make([]string, len(related))
	copy(result, related)
	return result
}

// HasCapability checks if a capability is available (with fuzzy matching)
func (cm *CapabilityMatcher) HasCapability(required string, available []string) bool {
	score := cm.matchSingleCapability(strings.ToLower(required), available)
	return score > 0.5 // Threshold for "has capability"
}

// GetMetrics returns capability matcher metrics
func (cm *CapabilityMatcher) GetMetrics() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return map[string]interface{}{
		"graph_nodes": len(cm.graph),
		"aliases":     len(cm.aliases),
	}
}
