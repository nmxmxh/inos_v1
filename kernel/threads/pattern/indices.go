package pattern

import "sync"

// PatternIndices provides multi-dimensional indexing for patterns
type PatternIndices struct {
	byType        map[PatternType][]uint64
	byConfidence  *ConfidenceIndex
	byAccessTime  *TimeIndex
	bySuccessRate *SuccessIndex
	byComplexity  *ComplexityIndex
	bySource      map[uint32][]uint64
	byTag         map[string][]uint64
	mu            sync.RWMutex
}

// ConfidenceIndex indexes patterns by confidence level
type ConfidenceIndex struct {
	buckets [11][]uint64 // 0-10, 10-20, ..., 90-100
	mu      sync.RWMutex
}

// TimeIndex indexes patterns by access time
type TimeIndex struct {
	recent []uint64 // Recently accessed
	mu     sync.RWMutex
}

// SuccessIndex indexes patterns by success rate
type SuccessIndex struct {
	buckets [11][]uint64 // 0-10%, 10-20%, ..., 90-100%
	mu      sync.RWMutex
}

// ComplexityIndex indexes patterns by complexity
type ComplexityIndex struct {
	buckets [11][]uint64 // Complexity 0-10
	mu      sync.RWMutex
}

// PatternMetadataStore stores pattern metadata
type PatternMetadataStore struct {
	metadata map[uint64]*PatternMetadata
	mu       sync.RWMutex
}

// NewPatternIndices creates a new pattern indices
func NewPatternIndices() *PatternIndices {
	return &PatternIndices{
		byType:        make(map[PatternType][]uint64),
		byConfidence:  &ConfidenceIndex{},
		byAccessTime:  &TimeIndex{recent: make([]uint64, 0)},
		bySuccessRate: &SuccessIndex{},
		byComplexity:  &ComplexityIndex{},
		bySource:      make(map[uint32][]uint64),
		byTag:         make(map[string][]uint64),
	}
}

// Add adds a pattern to all indices
func (pi *PatternIndices) Add(pattern *EnhancedPattern) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	id := pattern.Header.ID

	// Index by type
	pi.byType[pattern.Header.Type] = append(pi.byType[pattern.Header.Type], id)

	// Index by confidence
	pi.byConfidence.Add(id, pattern.Header.Confidence)

	// Index by success rate
	pi.bySuccessRate.Add(id, pattern.Header.SuccessRate)

	// Index by complexity
	pi.byComplexity.Add(id, pattern.Header.Complexity)

	// Index by source
	pi.bySource[pattern.Header.SourceHash] = append(pi.bySource[pattern.Header.SourceHash], id)

	// Index by tags
	for _, tag := range pattern.Body.Metadata.Tags {
		pi.byTag[tag] = append(pi.byTag[tag], id)
	}
}

// FindByType finds patterns by type
func (pi *PatternIndices) FindByType(patternType PatternType) []uint64 {
	pi.mu.RLock()
	defer pi.mu.RUnlock()

	return pi.byType[patternType]
}

// FindByConfidence finds patterns with confidence >= minConfidence
func (pi *PatternIndices) FindByConfidence(minConfidence uint8) []uint64 {
	return pi.byConfidence.Find(minConfidence)
}

// FindBySource finds patterns from a specific source
func (pi *PatternIndices) FindBySource(sourceHash uint32) []uint64 {
	pi.mu.RLock()
	defer pi.mu.RUnlock()

	return pi.bySource[sourceHash]
}

// FindByTag finds patterns with a specific tag
func (pi *PatternIndices) FindByTag(tag string) []uint64 {
	pi.mu.RLock()
	defer pi.mu.RUnlock()

	return pi.byTag[tag]
}

// ConfidenceIndex methods

func (ci *ConfidenceIndex) Add(id uint64, confidence uint8) {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	bucket := confidence / 10
	if bucket > 10 {
		bucket = 10
	}

	ci.buckets[bucket] = append(ci.buckets[bucket], id)
}

func (ci *ConfidenceIndex) Find(minConfidence uint8) []uint64 {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	minBucket := minConfidence / 10
	var result []uint64

	for i := minBucket; i <= 10; i++ {
		result = append(result, ci.buckets[i]...)
	}

	return result
}

// SuccessIndex methods

func (si *SuccessIndex) Add(id uint64, successRate float32) {
	si.mu.Lock()
	defer si.mu.Unlock()

	bucket := int(successRate * 10)
	if bucket > 10 {
		bucket = 10
	}

	si.buckets[bucket] = append(si.buckets[bucket], id)
}

// ComplexityIndex methods

func (ci *ComplexityIndex) Add(id uint64, complexity uint8) {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if complexity > 10 {
		complexity = 10
	}

	ci.buckets[complexity] = append(ci.buckets[complexity], id)
}

// PatternMetadataStore methods

func NewPatternMetadataStore() *PatternMetadataStore {
	return &PatternMetadataStore{
		metadata: make(map[uint64]*PatternMetadata),
	}
}

func (pms *PatternMetadataStore) Set(id uint64, metadata *PatternMetadata) {
	pms.mu.Lock()
	defer pms.mu.Unlock()

	pms.metadata[id] = metadata
}

func (pms *PatternMetadataStore) Get(id uint64) *PatternMetadata {
	pms.mu.RLock()
	defer pms.mu.RUnlock()

	return pms.metadata[id]
}
