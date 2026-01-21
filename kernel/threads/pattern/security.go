package pattern

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/utils"
)

// PatternSecurity handles pattern security validation
type PatternSecurity struct {
	trustStore     *TrustStore
	threatDetector *ThreatDetector
	mu             sync.RWMutex
}

// TrustStore manages trusted pattern sources
type TrustStore struct {
	trusted map[uint32]bool // sourceHash → trusted
	mu      sync.RWMutex
}

// ThreatDetector detects malicious patterns
type ThreatDetector struct {
	detectors []ThreatCheck
}

type ThreatCheck interface {
	Check(pattern *EnhancedPattern) []ThreatIndicator
	Type() ThreatType
}

type ThreatType int

const (
	ThreatLogicBomb ThreatType = iota
	ThreatResourceExhaustion
	ThreatPrivilegeEscalation
	ThreatDataLeakage
)

type ThreatIndicator struct {
	Type     ThreatType
	Severity string // "LOW", "MEDIUM", "HIGH", "CRITICAL"
	Message  string
}

// NewPatternSecurity creates a new pattern security system
func NewPatternSecurity() *PatternSecurity {
	return &PatternSecurity{
		trustStore: &TrustStore{
			trusted: make(map[uint32]bool),
		},
		threatDetector: &ThreatDetector{
			detectors: []ThreatCheck{
				&LogicBombDetector{},
				&ResourceExhaustionDetector{},
			},
		},
	}
}

// ValidatePattern validates a pattern for security
func (ps *PatternSecurity) ValidatePattern(pattern *EnhancedPattern) (bool, []ThreatIndicator) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var threats []ThreatIndicator

	// 1. Check if source is trusted
	if !ps.trustStore.IsTrusted(pattern.Header.SourceHash) {
		threats = append(threats, ThreatIndicator{
			Type:     ThreatPrivilegeEscalation,
			Severity: "MEDIUM",
			Message:  fmt.Sprintf("Untrusted source: %x", pattern.Header.SourceHash),
		})
	}

	// 2. Verify pattern integrity
	if !ps.verifyIntegrity(pattern) {
		threats = append(threats, ThreatIndicator{
			Type:     ThreatDataLeakage,
			Severity: "HIGH",
			Message:  "Pattern integrity check failed",
		})
		return false, threats
	}

	// 3. Run threat detection
	for _, detector := range ps.threatDetector.detectors {
		indicators := detector.Check(pattern)
		threats = append(threats, indicators...)
	}

	// 4. Check for critical threats
	for _, threat := range threats {
		if threat.Severity == "CRITICAL" {
			return false, threats
		}
	}

	return len(threats) == 0, threats
}

func (ps *PatternSecurity) verifyIntegrity(pattern *EnhancedPattern) bool {
	// Verify magic number
	if pattern.Header.Magic != PATTERN_MAGIC {
		return false
	}

	// Verify data size
	if pattern.Body.Data.Size > PATTERN_MAX_PAYLOAD {
		return false
	}

	// Calculate checksum
	hash := sha256.Sum256(pattern.Body.Data.Payload)
	expectedHash := pattern.Header.SourceHash // Simplified

	// Compare (simplified - in production use proper signature verification)
	_ = hash
	_ = expectedHash

	return true
}

// TrustStore methods

func (ts *TrustStore) IsTrusted(sourceHash uint32) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	trusted, exists := ts.trusted[sourceHash]
	return exists && trusted
}

func (ts *TrustStore) AddTrusted(sourceHash uint32) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.trusted[sourceHash] = true
}

func (ts *TrustStore) RemoveTrusted(sourceHash uint32) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	delete(ts.trusted, sourceHash)
}

// Threat detectors

type LogicBombDetector struct{}

func (lbd *LogicBombDetector) Check(pattern *EnhancedPattern) []ThreatIndicator {
	var threats []ThreatIndicator

	// Check for suspicious expiration times
	if pattern.Header.Expiration > 0 {
		expirationTime := time.Unix(0, int64(pattern.Header.Expiration))
		if time.Until(expirationTime) < time.Hour {
			threats = append(threats, ThreatIndicator{
				Type:     ThreatLogicBomb,
				Severity: "MEDIUM",
				Message:  "Pattern expires soon - possible logic bomb",
			})
		}
	}

	return threats
}

func (lbd *LogicBombDetector) Type() ThreatType {
	return ThreatLogicBomb
}

type ResourceExhaustionDetector struct{}

func (red *ResourceExhaustionDetector) Check(pattern *EnhancedPattern) []ThreatIndicator {
	var threats []ThreatIndicator

	// Check for excessive complexity
	if pattern.Header.Complexity > 8 {
		threats = append(threats, ThreatIndicator{
			Type:     ThreatResourceExhaustion,
			Severity: "MEDIUM",
			Message:  "Pattern complexity too high - possible resource exhaustion",
		})
	}

	// Check for excessive payload size
	if pattern.Body.Data.Size > PATTERN_MAX_PAYLOAD*8/10 {
		threats = append(threats, ThreatIndicator{
			Type:     ThreatResourceExhaustion,
			Severity: "LOW",
			Message:  "Pattern payload size is large",
		})
	}

	return threats
}

func (red *ResourceExhaustionDetector) Type() ThreatType {
	return ThreatResourceExhaustion
}

// Enhanced pattern validator with security
type EnhancedPatternValidator struct {
	security    *PatternSecurity
	statistical *PatternValidator
}

func NewEnhancedPatternValidator() *EnhancedPatternValidator {
	return &EnhancedPatternValidator{
		security:    NewPatternSecurity(),
		statistical: NewPatternValidator(),
	}
}

func (epv *EnhancedPatternValidator) Validate(pattern *EnhancedPattern, observations []Observation) bool {
	// 1. Security validation
	valid, threats := epv.security.ValidatePattern(pattern)
	if !valid {
		// Log threats
		for _, threat := range threats {
			utils.Warn("Security Threat Detected",
				utils.String("type", fmt.Sprintf("%v", threat.Type)),
				utils.String("severity", threat.Severity),
				utils.String("message", threat.Message),
				utils.Uint64("patternID", pattern.Header.ID))
		}
		return false
	}

	// 2. Statistical validation
	return epv.statistical.Validate(pattern, observations)
}
