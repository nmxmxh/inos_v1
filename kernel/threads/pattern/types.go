package pattern

import "time"

// Pattern types
type PatternType uint16

const (
	PatternTypeAtomic        PatternType = iota // Simple pattern
	PatternTypeComposite                        // Multiple patterns combined
	PatternTypeConditional                      // IF-THEN-ELSE pattern
	PatternTypeTemporal                         // Time-based pattern
	PatternTypeSequential                       // A→B→C sequence
	PatternTypeProbabilistic                    // Probabilistic decision
	PatternTypeAdaptive                         // Self-modifying pattern
	PatternTypeSecurity                         // Security policy pattern
)

// Pattern flags
type PatternFlags uint16

const (
	FlagActive     PatternFlags = 1 << 0 // Pattern is active
	FlagTrusted    PatternFlags = 1 << 1 // From trusted source
	FlagValidated  PatternFlags = 1 << 2 // Statistically validated
	FlagEvolved    PatternFlags = 1 << 3 // Result of evolution
	FlagComposite  PatternFlags = 1 << 4 // Composite pattern
	FlagTemporal   PatternFlags = 1 << 5 // Time-dependent
	FlagEncrypted  PatternFlags = 1 << 6 // Encrypted data
	FlagCompressed PatternFlags = 1 << 7 // Compressed data
)

// Data encoding types
type DataEncoding uint8

const (
	EncodingBinary DataEncoding = iota
	EncodingJSON
	EncodingProto
	EncodingExpression // DSL for patterns
	EncodingGraph      // Graph structure
)

// EnhancedPattern represents a complete pattern with metadata
type EnhancedPattern struct {
	Header PatternHeader
	Body   PatternBody
}

// PatternHeader contains pattern metadata (64 bytes)
type PatternHeader struct {
	Magic       uint64       // 0x5041545F45582D50 ("PAT_EX-P")
	ID          uint64       // Unique pattern ID
	Version     uint16       // Pattern version for evolution
	Type        PatternType  // Pattern type
	Complexity  uint8        // 1-10 scale
	Confidence  uint8        // 0-100%
	SourceHash  uint32       // Hash of source supervisor
	Timestamp   uint64       // Creation timestamp (Unix nano)
	Expiration  uint64       // Pattern expiration (0 = never)
	Weight      float32      // Dynamic weight (0.0-1.0)
	AccessCount uint32       // Times accessed
	SuccessRate float32      // Historical success rate
	Flags       PatternFlags // Pattern flags
}

// PatternBody contains pattern data and metadata
type PatternBody struct {
	Data     PatternData
	Metadata PatternMetadata
	Links    PatternLinks
}

// PatternData contains the actual pattern information
type PatternData struct {
	Encoding DataEncoding
	Size     uint16
	Payload  []byte // Variable length (up to 1KB)
}

// PatternMetadata contains additional pattern information
type PatternMetadata struct {
	Tags        []string     // For categorization
	Conditions  []Condition  // When pattern applies
	Constraints []Constraint // Constraints on application
	Metrics     PatternMetrics
}

// Condition represents a condition for pattern application
type Condition struct {
	Field    string
	Operator string // "=", ">", "<", "IN", "CONTAINS", etc.
	Value    interface{}
}

// Constraint represents a constraint on pattern application
type Constraint struct {
	Type  string // "resource", "time", "security", etc.
	Limit interface{}
}

// PatternMetrics tracks pattern performance
type PatternMetrics struct {
	Applications   uint32
	Successes      uint32
	Failures       uint32
	AvgImprovement float32
	AvgLatency     time.Duration
	CostSavings    float32
	LastApplied    time.Time
}

// PatternLinks tracks pattern relationships
type PatternLinks struct {
	Dependencies []uint64 // IDs of dependent patterns
	Alternatives []uint64 // Alternative patterns
	Contradicts  []uint64 // Contradictory patterns
	EvolvedFrom  []uint64 // Parent patterns
}

// Pattern constants
const (
	PATTERN_MAGIC          = 0x5041545F45582D50 // "PAT_EX-P"
	PATTERN_HEADER_SIZE    = 64
	PATTERN_MAX_PAYLOAD    = 1024
	PATTERN_MAX_TAGS       = 16
	PATTERN_MAX_CONDITIONS = 32
)

// Helper: Create new pattern
func NewPattern(patternType PatternType, sourceHash uint32) *EnhancedPattern {
	return &EnhancedPattern{
		Header: PatternHeader{
			Magic:       PATTERN_MAGIC,
			ID:          0, // Set by storage
			Version:     1,
			Type:        patternType,
			Complexity:  1,
			Confidence:  0,
			SourceHash:  sourceHash,
			Timestamp:   uint64(time.Now().UnixNano()),
			Expiration:  0,
			Weight:      1.0,
			AccessCount: 0,
			SuccessRate: 0.0,
			Flags:       FlagActive,
		},
		Body: PatternBody{
			Data: PatternData{
				Encoding: EncodingBinary,
				Size:     0,
				Payload:  make([]byte, 0),
			},
			Metadata: PatternMetadata{
				Tags:        make([]string, 0),
				Conditions:  make([]Condition, 0),
				Constraints: make([]Constraint, 0),
				Metrics: PatternMetrics{
					Applications: 0,
					Successes:    0,
					Failures:     0,
				},
			},
			Links: PatternLinks{
				Dependencies: make([]uint64, 0),
				Alternatives: make([]uint64, 0),
				Contradicts:  make([]uint64, 0),
				EvolvedFrom:  make([]uint64, 0),
			},
		},
	}
}

// Helper: Check if pattern is valid
func (p *EnhancedPattern) IsValid() bool {
	return p.Header.Magic == PATTERN_MAGIC &&
		p.Header.ID > 0 &&
		p.Body.Data.Size <= PATTERN_MAX_PAYLOAD
}

// Helper: Check if pattern is active
func (p *EnhancedPattern) IsActive() bool {
	return (p.Header.Flags & FlagActive) != 0
}

// Helper: Check if pattern is expired
func (p *EnhancedPattern) IsExpired() bool {
	if p.Header.Expiration == 0 {
		return false
	}
	return uint64(time.Now().UnixNano()) > p.Header.Expiration
}

// Helper: Update success rate
func (p *EnhancedPattern) UpdateSuccessRate(success bool) {
	p.Body.Metadata.Metrics.Applications++
	if success {
		p.Body.Metadata.Metrics.Successes++
	} else {
		p.Body.Metadata.Metrics.Failures++
	}

	if p.Body.Metadata.Metrics.Applications > 0 {
		p.Header.SuccessRate = float32(p.Body.Metadata.Metrics.Successes) /
			float32(p.Body.Metadata.Metrics.Applications)
	}
}
