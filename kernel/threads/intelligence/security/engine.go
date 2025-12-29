package security

import (
	"math"
	"sync"
	"time"
)

// SecurityEngine coordinates adaptive security
type SecurityEngine struct {
	anomalyDetector *AnomalyDetector
	threatScorer    *ThreatScorer
	validator       *InputValidator
	rateLimiter     *RateLimiter

	// Statistics
	threatsDetected uint64
	threatsBlocked  uint64

	mu sync.RWMutex
}

type ThreatEvent struct {
	ID          string
	Type        ThreatType
	Severity    float64
	Source      string
	Timestamp   time.Time
	Blocked     bool
	Description string
}

type ThreatType int

const (
	ThreatAnomaly ThreatType = iota
	ThreatRateLimit
	ThreatInvalidInput
	ThreatSuspiciousPattern
)

func NewSecurityEngine() *SecurityEngine {
	return &SecurityEngine{
		anomalyDetector: NewAnomalyDetector(),
		threatScorer:    NewThreatScorer(),
		validator:       NewInputValidator(),
		rateLimiter:     NewRateLimiter(),
	}
}

// Analyze analyzes request for security threats
func (se *SecurityEngine) Analyze(request *SecurityRequest) *SecurityDecision {
	decision := &SecurityDecision{
		Allow:   true,
		Threats: make([]*ThreatEvent, 0),
	}

	// Check rate limit
	if !se.rateLimiter.Allow(request.Source) {
		threat := &ThreatEvent{
			Type:        ThreatRateLimit,
			Severity:    0.7,
			Source:      request.Source,
			Timestamp:   time.Now(),
			Blocked:     true,
			Description: "Rate limit exceeded",
		}
		decision.Threats = append(decision.Threats, threat)
		decision.Allow = false
		se.recordThreat(threat)
		return decision
	}

	// Validate input
	if !se.validator.Validate(request.Data) {
		threat := &ThreatEvent{
			Type:        ThreatInvalidInput,
			Severity:    0.8,
			Source:      request.Source,
			Timestamp:   time.Now(),
			Blocked:     true,
			Description: "Invalid input detected",
		}
		decision.Threats = append(decision.Threats, threat)
		decision.Allow = false
		se.recordThreat(threat)
		return decision
	}

	// Detect anomalies
	anomalyScore := se.anomalyDetector.Detect(request.Features)
	if anomalyScore > 0.7 {
		threat := &ThreatEvent{
			Type:        ThreatAnomaly,
			Severity:    anomalyScore,
			Source:      request.Source,
			Timestamp:   time.Now(),
			Blocked:     anomalyScore > 0.9,
			Description: "Anomalous behavior detected",
		}
		decision.Threats = append(decision.Threats, threat)
		if threat.Blocked {
			decision.Allow = false
		}
		se.recordThreat(threat)
	}

	// Calculate overall threat score
	decision.ThreatScore = se.threatScorer.Score(decision.Threats)

	return decision
}

type SecurityRequest struct {
	Source   string
	Data     interface{}
	Features map[string]float64
}

type SecurityDecision struct {
	Allow       bool
	ThreatScore float64
	Threats     []*ThreatEvent
}

func (se *SecurityEngine) recordThreat(threat *ThreatEvent) {
	se.mu.Lock()
	defer se.mu.Unlock()

	se.threatsDetected++
	if threat.Blocked {
		se.threatsBlocked++
	}
}

// GetStats returns security statistics
func (se *SecurityEngine) GetStats() SecurityStats {
	se.mu.RLock()
	defer se.mu.RUnlock()

	blockRate := float64(0)
	if se.threatsDetected > 0 {
		blockRate = float64(se.threatsBlocked) / float64(se.threatsDetected)
	}

	return SecurityStats{
		ThreatsDetected: se.threatsDetected,
		ThreatsBlocked:  se.threatsBlocked,
		BlockRate:       blockRate,
	}
}

type SecurityStats struct {
	ThreatsDetected uint64
	ThreatsBlocked  uint64
	BlockRate       float64
}

// AnomalyDetector implements isolation forest for anomaly detection
type AnomalyDetector struct {
	trees      []*IsolationTree
	numTrees   int
	sampleSize int
	mu         sync.RWMutex
}

type IsolationTree struct {
	root *TreeNode
}

type TreeNode struct {
	feature   string
	threshold float64
	left      *TreeNode
	right     *TreeNode
	size      int
}

func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{
		trees:      make([]*IsolationTree, 0),
		numTrees:   10,
		sampleSize: 256,
	}
}

// Detect detects anomalies using isolation forest
func (ad *AnomalyDetector) Detect(features map[string]float64) float64 {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	if len(ad.trees) == 0 {
		return 0.0 // Not trained
	}

	// Calculate average path length across all trees
	avgPathLength := 0.0
	for _, tree := range ad.trees {
		pathLength := ad.pathLength(tree.root, features, 0)
		avgPathLength += pathLength
	}
	avgPathLength /= float64(len(ad.trees))

	// Normalize to anomaly score [0, 1]
	// Lower path length = more anomalous
	c := ad.averagePathLength(ad.sampleSize)
	anomalyScore := math.Pow(2, -avgPathLength/c)

	return anomalyScore
}

// Calculate path length in tree
func (ad *AnomalyDetector) pathLength(node *TreeNode, features map[string]float64, depth int) float64 {
	if node.left == nil && node.right == nil {
		// Leaf node
		return float64(depth) + ad.averagePathLength(node.size)
	}

	featureValue := features[node.feature]
	if featureValue < node.threshold {
		return ad.pathLength(node.left, features, depth+1)
	}
	return ad.pathLength(node.right, features, depth+1)
}

// Average path length for unsuccessful search in BST
func (ad *AnomalyDetector) averagePathLength(n int) float64 {
	if n <= 1 {
		return 0
	}
	// c(n) = 2H(n-1) - 2(n-1)/n where H(n) is harmonic number
	h := math.Log(float64(n-1)) + 0.5772156649 // Euler's constant
	return 2*h - 2*float64(n-1)/float64(n)
}

// ThreatScorer scores threats
type ThreatScorer struct{}

func NewThreatScorer() *ThreatScorer {
	return &ThreatScorer{}
}

func (ts *ThreatScorer) Score(threats []*ThreatEvent) float64 {
	if len(threats) == 0 {
		return 0.0
	}

	// Aggregate threat scores
	maxSeverity := 0.0
	totalSeverity := 0.0

	for _, threat := range threats {
		totalSeverity += threat.Severity
		if threat.Severity > maxSeverity {
			maxSeverity = threat.Severity
		}
	}

	// Combined score: max + average
	avgSeverity := totalSeverity / float64(len(threats))
	score := 0.7*maxSeverity + 0.3*avgSeverity

	return score
}

// InputValidator validates inputs
type InputValidator struct{}

func NewInputValidator() *InputValidator {
	return &InputValidator{}
}

func (iv *InputValidator) Validate(data interface{}) bool {
	// Simple validation - in production use schema validation
	if data == nil {
		return false
	}
	return true
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	buckets map[string]*TokenBucket
	mu      sync.RWMutex
}

type TokenBucket struct {
	tokens     float64
	capacity   float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*TokenBucket),
	}
}

func (rl *RateLimiter) Allow(source string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.buckets[source]
	if !exists {
		// Create new bucket
		bucket = &TokenBucket{
			tokens:     100,
			capacity:   100,
			refillRate: 10, // 10 requests per second
			lastRefill: time.Now(),
		}
		rl.buckets[source] = bucket
	}

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * bucket.refillRate
	if bucket.tokens > bucket.capacity {
		bucket.tokens = bucket.capacity
	}
	bucket.lastRefill = now

	// Check if request allowed
	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true
	}

	return false
}
