package pattern

import (
	"context"
	"testing"
	"time"

	"unsafe"

	sab_layout "github.com/nmxmxh/inos_v1/kernel/threads/sab"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatternSecurity_Validate(t *testing.T) {
	ps := NewPatternSecurity()
	require.NotNil(t, ps)

	// Create a mock pattern
	p := &EnhancedPattern{
		Header: PatternHeader{
			ID:         1,
			Magic:      PATTERN_MAGIC,
			SourceHash: 0x12345678,
			Complexity: 1,
			Type:       PatternTypeAtomic,
		},
		Body: PatternBody{
			Data: PatternData{
				Size:    10,
				Payload: make([]byte, 10),
			},
		},
	}

	// 1. Untrusted source (default)
	valid, threats := ps.ValidatePattern(p)
	assert.False(t, valid)
	assert.Len(t, threats, 1)
	assert.Equal(t, ThreatPrivilegeEscalation, threats[0].Type)

	// 2. Trust the source
	ps.trustStore.AddTrusted(0x12345678)
	valid, threats = ps.ValidatePattern(p)
	assert.True(t, valid)
	assert.Len(t, threats, 0)

	// 3. Test Logic Bomb (short expiration)
	p.Header.Expiration = uint64(time.Now().Add(10 * time.Minute).UnixNano())
	valid, threats = ps.ValidatePattern(p)
	assert.False(t, valid)
	assert.Contains(t, threats[0].Message, "logic bomb")

	// 4. Test Resource Exhaustion (high complexity)
	p.Header.Expiration = 0
	p.Header.Complexity = 10
	valid, threats = ps.ValidatePattern(p)
	assert.False(t, valid)
	assert.Contains(t, threats[0].Message, "resource exhaustion")
}

func TestTrustStore(t *testing.T) {
	ts := &TrustStore{trusted: make(map[uint32]bool)}

	ts.AddTrusted(1)
	assert.True(t, ts.IsTrusted(1))

	ts.RemoveTrusted(1)
	assert.False(t, ts.IsTrusted(1))
}

func TestPatternTypes(t *testing.T) {
	p := NewPattern(PatternTypeSequential, 0x1)
	assert.NotNil(t, p)
	assert.Equal(t, PatternTypeSequential, p.Header.Type)

	p.Header.Expiration = uint64(time.Now().Add(1 * time.Hour).UnixNano())
	p.Header.ID = 1
	p.Body.Data.Size = 10

	assert.True(t, p.IsActive())
	assert.False(t, p.IsExpired())

	p.Header.Expiration = uint64(time.Now().Add(-1 * time.Hour).UnixNano())
	assert.True(t, p.IsExpired())
	assert.True(t, p.IsActive())
}

func TestPattern_IsExpired_Zero(t *testing.T) {
	p := NewPattern(PatternTypeAtomic, 1)
	p.Header.Expiration = 0
	assert.False(t, p.IsExpired())
}

func TestPattern_UpdateSuccessRate(t *testing.T) {
	p := NewPattern(PatternTypeAtomic, 1)
	p.UpdateSuccessRate(true)
	p.UpdateSuccessRate(true)
	p.UpdateSuccessRate(false)

	assert.Equal(t, uint32(3), p.Body.Metadata.Metrics.Applications)
	assert.InDelta(t, 0.666, float64(p.Header.SuccessRate), 0.01)
}

func TestPatternSubscriber_Basic(t *testing.T) {
	sabSize := uint32(sab_layout.SAB_SIZE_DEFAULT)
	sab := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&sab[0])
	storage := NewTieredPatternStorage(sabPtr, sabSize, sab_layout.OFFSET_PATTERN_EXCHANGE, sab_layout.SIZE_PATTERN_EXCHANGE)
	sub := NewPatternSubscriber(storage)
	require.NotNil(t, sub)

	callback := func(p *EnhancedPattern) {}

	query := NewPatternQuery().WithType(PatternTypeAtomic)
	err := sub.Subscribe("sub1", query, callback)
	assert.NoError(t, err)

	sub.Unsubscribe("sub1")
	assert.Empty(t, sub.subscriptions)

	ctx, cancel := context.WithCancel(context.Background())
	go sub.Watch(ctx)
	time.Sleep(10 * time.Millisecond)
	cancel()
}

func TestPatternSubscriber_Updates(t *testing.T) {
	sabSize := uint32(sab_layout.SAB_SIZE_DEFAULT)
	sab := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&sab[0])
	storage := NewTieredPatternStorage(sabPtr, sabSize, sab_layout.OFFSET_PATTERN_EXCHANGE, sab_layout.SIZE_PATTERN_EXCHANGE)
	sub := NewPatternSubscriber(storage)

	received := make(chan *EnhancedPattern, 1)
	callback := func(p *EnhancedPattern) {
		received <- p
	}

	query := NewPatternQuery().WithType(PatternTypeAtomic)
	sub.Subscribe("sub1", query, callback)

	p := NewPattern(PatternTypeAtomic, 1)
	p.Header.ID = 100
	p.Body.Data.Size = 4
	p.Body.Data.Payload = []byte("test")
	storage.WritePattern(p)

	sub.checkUpdates()

	select {
	case pRec := <-received:
		assert.Equal(t, uint64(100), pRec.Header.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for pattern update")
	}
}

func TestPatternSubscriber_ApplyPattern(t *testing.T) {
	sabSize := uint32(sab_layout.SAB_SIZE_DEFAULT)
	sab := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&sab[0])
	storage := NewTieredPatternStorage(sabPtr, sabSize, sab_layout.OFFSET_PATTERN_EXCHANGE, sab_layout.SIZE_PATTERN_EXCHANGE)
	sub := NewPatternSubscriber(storage)

	p := NewPattern(PatternTypeAtomic, 1)
	p.Header.ID = 700
	p.Header.Flags |= FlagActive
	storage.WritePattern(p)

	success := sub.ApplyPattern(p, nil)
	assert.True(t, success)

	// Non-existent
	nonExistentP := NewPattern(PatternTypeAtomic, 999)
	success = sub.ApplyPattern(nonExistentP, nil)
	assert.False(t, success)
}

func TestPatternEngine_Apply(t *testing.T) {
	pe := &PatternEngine{}

	p1 := &EnhancedPattern{Header: PatternHeader{Type: PatternTypeAtomic, Magic: PATTERN_MAGIC, ID: 1, Flags: FlagActive}}
	assert.True(t, pe.Apply(p1, nil))

	p2 := &EnhancedPattern{
		Header: PatternHeader{Type: PatternTypeConditional, Magic: PATTERN_MAGIC, ID: 2, Flags: FlagActive},
		Body: PatternBody{
			Metadata: PatternMetadata{
				Conditions: []Condition{
					{Field: "os", Operator: "=", Value: "inos"},
				},
			},
		},
	}
	assert.True(t, pe.Apply(p2, map[string]interface{}{"os": "inos"}))
	assert.False(t, pe.Apply(p2, map[string]interface{}{"os": "linux"}))

	p3 := &EnhancedPattern{Header: PatternHeader{Type: PatternTypeSecurity, Magic: PATTERN_MAGIC, ID: 3, Flags: FlagActive}}
	assert.False(t, pe.Apply(p3, nil))
	p3.Header.Flags |= FlagTrusted
	assert.True(t, pe.Apply(p3, nil))
}

func TestPatternQuery_Helpers(t *testing.T) {
	q := NewPatternQuery().
		WithType(PatternTypeAtomic).
		WithMinConfidence(90).
		WithTag("test").
		WithTimeRange(time.Now(), time.Now().Add(time.Hour)).
		WithLimit(10)

	assert.Equal(t, []PatternType{PatternTypeAtomic}, q.Types)
	assert.Equal(t, uint8(90), q.MinConfidence)
	assert.Equal(t, []string{"test"}, q.Tags)
	assert.NotNil(t, q.TimeRange)
	assert.Equal(t, 10, q.Limit)
}

func TestObservationStore(t *testing.T) {
	os := NewObservationStore()
	obs := Observation{Timestamp: time.Now(), Success: true}
	os.Add("test", obs)

	all := os.GetAll()
	assert.Len(t, all, 1)
	assert.Equal(t, obs.Timestamp, all[0].Timestamp)

	assert.Len(t, os.Get("test"), 1)
	assert.Nil(t, os.Get("unknown"))
}

func TestPatternValidator_Significance(t *testing.T) {
	pv := NewPatternValidator()
	obs := make([]Observation, 5)
	assert.False(t, pv.CheckStatisticalSignificance(obs))

	obs = make([]Observation, 10)
	assert.True(t, pv.CheckStatisticalSignificance(obs))
}

func TestPatternDetector_Basic(t *testing.T) {
	sabSize := uint32(sab_layout.SAB_SIZE_DEFAULT)
	sab := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&sab[0])
	storage := NewTieredPatternStorage(sabPtr, sabSize, sab_layout.OFFSET_PATTERN_EXCHANGE, sab_layout.SIZE_PATTERN_EXCHANGE)
	publisher := NewPatternPublisher(storage)
	validator := NewPatternValidator()
	pd := NewPatternDetector(validator, publisher)
	require.NotNil(t, pd)

	pd.Observe("test", Observation{Success: true, Latency: time.Millisecond})
	patterns := pd.DetectPatterns()
	assert.Empty(t, patterns)
}

func TestCreatePatternFromObservations(t *testing.T) {
	obs := []Observation{
		{Success: true, Latency: 10 * time.Millisecond, Cost: 1.0},
		{Success: false, Latency: 20 * time.Millisecond, Cost: 2.0},
	}

	p, err := CreatePatternFromObservations(PatternTypeAtomic, obs)
	assert.NoError(t, err)
	assert.Equal(t, uint8(50), p.Header.Confidence)
	assert.Equal(t, float32(0.5), p.Header.SuccessRate)
	assert.Equal(t, 15*time.Millisecond, p.Body.Metadata.Metrics.AvgLatency)
}

func TestPatternIndices_Extended(t *testing.T) {
	indices := NewPatternIndices()
	p := &EnhancedPattern{
		Header: PatternHeader{
			ID:         1,
			Type:       PatternTypeAtomic,
			Confidence: 95,
			SourceHash: 0x1234,
		},
		Body: PatternBody{
			Metadata: PatternMetadata{
				Tags: []string{"test", "fast"},
			},
		},
	}
	indices.Add(p)

	// Test FindByConfidence
	highConf := indices.FindByConfidence(90)
	assert.Len(t, highConf, 1)
	assert.Empty(t, indices.FindByConfidence(100))

	// Test FindBySource
	srcMatch := indices.FindBySource(0x1234)
	assert.Len(t, srcMatch, 1)
	assert.Empty(t, indices.FindBySource(0x5678))

	// Test FindByTag
	tagMatch := indices.FindByTag("test")
	assert.Len(t, tagMatch, 1)
	assert.Empty(t, indices.FindByTag("slow"))
}

func TestPatternMetadataStore(t *testing.T) {
	ms := NewPatternMetadataStore()
	md := &PatternMetadata{Tags: []string{"test"}}
	ms.Set(1, md)

	val := ms.Get(1)
	assert.NotNil(t, val)
	assert.Equal(t, "test", val.Tags[0])

	val = ms.Get(99)
	assert.Nil(t, val)
}

func TestPatternSecurity_Extended(t *testing.T) {
	ps := NewPatternSecurity()

	// Test ThreatIndicator Types
	ti1 := ThreatIndicator{Type: ThreatLogicBomb}
	assert.Equal(t, ThreatLogicBomb, ti1.Type)

	// Test extended validation logic
	p := &EnhancedPattern{
		Header: PatternHeader{
			Magic:      PATTERN_MAGIC,
			SourceHash: 0x1234,
		},
	}
	// Untrusted source
	valid, threats := ps.ValidatePattern(p)
	assert.False(t, valid)
	assert.Contains(t, threats[0].Message, "Untrusted")

	// Verify integrity check
	p.Header.Magic = 0x0
	valid, threats = ps.ValidatePattern(p)
	assert.False(t, valid)
	assert.GreaterOrEqual(t, len(threats), 1)
	// One of the threats should be the integrity failure
	found := false
	for _, t := range threats {
		if t.Type == ThreatDataLeakage {
			found = true
			break
		}
	}
	assert.True(t, found, "Should have found an integrity threat")
}

func TestTieredPatternStorage_Extended(t *testing.T) {
	sabSize := uint32(sab_layout.SAB_SIZE_DEFAULT)
	sab := make([]byte, sabSize)
	sabPtr := unsafe.Pointer(&sab[0])
	storage := NewTieredPatternStorage(sabPtr, sabSize, sab_layout.OFFSET_PATTERN_EXCHANGE, sab_layout.SIZE_PATTERN_EXCHANGE)

	p := NewPattern(PatternTypeAtomic, 1)
	p.Header.ID = 500
	p.Header.Confidence = 80
	p.Body.Data.Size = 4
	p.Body.Data.Payload = []byte("tier")
	p.Body.Metadata.Tags = []string{"query"}

	err := storage.WritePattern(p)
	require.NoError(t, err)

	// Test Read
	pRec, err := storage.ReadPattern(500)
	require.NoError(t, err)
	assert.Equal(t, uint64(500), pRec.Header.ID)

	// Test Query combinations
	q := NewPatternQuery().WithType(PatternTypeAtomic).WithMinConfidence(50).WithTag("query")
	matches, err := storage.Query(q)
	require.NoError(t, err)
	assert.NotEmpty(t, matches)

	// Test query with limit
	q2 := NewPatternQuery().WithLimit(1)
	matches2, err := storage.Query(q2)
	assert.NoError(t, err)
	assert.Len(t, matches2, 1)

	// Test Stats
	stats := storage.GetStats()
	assert.Greater(t, stats.TotalPatterns, uint64(0))
}
