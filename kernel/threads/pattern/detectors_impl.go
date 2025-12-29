package pattern

import "time"

// Statistical Detector
type StatisticalDetector struct {
	minSamples    int
	minConfidence float32
}

func NewStatisticalDetector() *StatisticalDetector {
	return &StatisticalDetector{
		minSamples:    10,
		minConfidence: 0.7,
	}
}

func (sd *StatisticalDetector) Detect(obs []Observation) []*PatternCandidate {
	if len(obs) < sd.minSamples {
		return nil
	}

	// Calculate success rate
	successes := 0
	for _, o := range obs {
		if o.Success {
			successes++
		}
	}

	successRate := float32(successes) / float32(len(obs))
	if successRate < sd.minConfidence {
		return nil
	}

	// Create candidate
	candidate := &PatternCandidate{
		Type:       PatternTypeAtomic,
		Confidence: uint8(successRate * 100),
		Data:       []byte{},
		Evidence:   obs,
	}

	return []*PatternCandidate{candidate}
}

func (sd *StatisticalDetector) Confidence() float32 {
	return 0.8
}

func (sd *StatisticalDetector) Type() PatternType {
	return PatternTypeAtomic
}

func (sd *StatisticalDetector) Complexity() uint8 {
	return 1
}

// Temporal Detector
type TemporalDetector struct {
	timeWindow time.Duration
}

func NewTemporalDetector() *TemporalDetector {
	return &TemporalDetector{
		timeWindow: time.Hour,
	}
}

func (td *TemporalDetector) Detect(obs []Observation) []*PatternCandidate {
	if len(obs) < 10 {
		return nil
	}

	windows := td.groupByTimeWindow(obs)
	var candidates []*PatternCandidate

	for hour, windowObs := range windows {
		if len(windowObs) < 5 {
			continue
		}

		successes := 0
		for _, o := range windowObs {
			if o.Success {
				successes++
			}
		}

		successRate := float32(successes) / float32(len(windowObs))

		if successRate > 0.8 || successRate < 0.2 {
			candidate := &PatternCandidate{
				Type:       PatternTypeTemporal,
				Confidence: uint8(successRate * 100),
				Data:       td.encodeTimeWindow(hour),
				Evidence:   windowObs,
				Metadata: map[string]interface{}{
					"time_window": hour,
					"sample_size": len(windowObs),
				},
			}
			candidates = append(candidates, candidate)
		}
	}

	return candidates
}

func (td *TemporalDetector) groupByTimeWindow(obs []Observation) map[int][]Observation {
	windows := make(map[int][]Observation)
	for _, o := range obs {
		hour := o.Timestamp.Hour()
		windows[hour] = append(windows[hour], o)
	}
	return windows
}

func (td *TemporalDetector) encodeTimeWindow(hour int) []byte {
	return []byte{byte(hour), 0, 0, 0}
}

func (td *TemporalDetector) Confidence() float32 {
	return 0.7
}

func (td *TemporalDetector) Type() PatternType {
	return PatternTypeTemporal
}

func (td *TemporalDetector) Complexity() uint8 {
	return 3
}

// Sequence Detector
type SequenceDetector struct {
	minSequenceLength int
}

func NewSequenceDetector() *SequenceDetector {
	return &SequenceDetector{
		minSequenceLength: 3,
	}
}

func (sd *SequenceDetector) Detect(obs []Observation) []*PatternCandidate {
	if len(obs) < sd.minSequenceLength {
		return nil
	}

	sequences := sd.extractSequences(obs)
	if len(sequences) == 0 {
		return nil
	}

	frequentSequences := sd.findFrequentSequences(sequences)

	var candidates []*PatternCandidate
	for _, seq := range frequentSequences {
		candidate := &PatternCandidate{
			Type:       PatternTypeSequential,
			Confidence: seq.confidence,
			Data:       seq.encode(),
			Evidence:   seq.observations,
			Metadata: map[string]interface{}{
				"sequence_length": len(seq.steps),
				"frequency":       seq.frequency,
			},
		}
		candidates = append(candidates, candidate)
	}

	return candidates
}

type sequence struct {
	steps        []string
	frequency    int
	confidence   uint8
	observations []Observation
}

func (s *sequence) encode() []byte {
	var data []byte
	for _, step := range s.steps {
		data = append(data, []byte(step)...)
		data = append(data, 0)
	}
	return data
}

func (sd *SequenceDetector) extractSequences(obs []Observation) []sequence {
	var sequences []sequence

	for i := 0; i < len(obs)-sd.minSequenceLength+1; i++ {
		var steps []string
		var seqObs []Observation

		for j := 0; j < sd.minSequenceLength; j++ {
			if obs[i+j].Metadata != nil {
				if action, ok := obs[i+j].Metadata["action"].(string); ok {
					steps = append(steps, action)
					seqObs = append(seqObs, obs[i+j])
				}
			}
		}

		if len(steps) == sd.minSequenceLength {
			sequences = append(sequences, sequence{
				steps:        steps,
				frequency:    1,
				observations: seqObs,
			})
		}
	}

	return sequences
}

func (sd *SequenceDetector) findFrequentSequences(sequences []sequence) []sequence {
	seqMap := make(map[string]*sequence)

	for _, seq := range sequences {
		key := sd.sequenceKey(seq.steps)
		if existing, ok := seqMap[key]; ok {
			existing.frequency++
			existing.observations = append(existing.observations, seq.observations...)
		} else {
			seqCopy := seq
			seqMap[key] = &seqCopy
		}
	}

	var frequent []sequence
	for _, seq := range seqMap {
		if seq.frequency >= 3 {
			seq.confidence = uint8(min(seq.frequency*10, 100))
			frequent = append(frequent, *seq)
		}
	}

	return frequent
}

func (sd *SequenceDetector) sequenceKey(steps []string) string {
	key := ""
	for i, step := range steps {
		if i > 0 {
			key += "→"
		}
		key += step
	}
	return key
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (sd *SequenceDetector) Confidence() float32 {
	return 0.75
}

func (sd *SequenceDetector) Type() PatternType {
	return PatternTypeSequential
}

func (sd *SequenceDetector) Complexity() uint8 {
	return 5
}
