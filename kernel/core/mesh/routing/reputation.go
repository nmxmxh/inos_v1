package routing

import (
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"
)

// ReputationStore defines persistence for trust scores.
type ReputationStore interface {
	SaveScores(scores map[string]ReputationScore) error
	LoadScores() (map[string]ReputationScore, error)
}

// PenaltyReason defines types of failures that affect reputation.
type PenaltyReason int

const (
	PenaltyTimeout PenaltyReason = iota
	PenaltyInvalidData
	PenaltyPoRFailure
	PenaltyMaliciousBehavior
	PenaltyCongestion
)

// ReputationScore matches Rust's PeerReputation and extends it.
type ReputationScore struct {
	PeerID                 string  `json:"peer_id"`
	Score                  float64 `json:"score"`
	Confidence             float64 `json:"confidence"`
	SuccessfulInteractions uint64  `json:"successful_interactions"`
	FailedInteractions     uint64  `json:"failed_interactions"`
	ChallengesIssued       uint64  `json:"challenges_issued"`
	TotalLatencyMs         float64 `json:"total_latency_ms"`
	LastInteraction        int64   `json:"last_interaction"` // Unix Nano
	LastUpdated            int64   `json:"last_updated"`     // Unix Nano
}

// ReputationManager implements an EMA-based trust system.
type ReputationManager struct {
	scores   map[string]ReputationScore
	scoresMu sync.RWMutex

	decayHalfLife time.Duration // Time for score to decay by half
	minScore      float64       // 0.0
	maxScore      float64       // 1.0
	defaultScore  float64       // 0.5
	alpha         float64       // EMA smoothing factor (0.1 - 0.2)

	store  ReputationStore
	logger *slog.Logger
}

func NewReputationManager(decayHalflife time.Duration, store ReputationStore, logger *slog.Logger) *ReputationManager {
	if logger == nil {
		logger = slog.Default()
	}

	rm := &ReputationManager{
		scores:        make(map[string]ReputationScore),
		decayHalfLife: decayHalflife,
		minScore:      0.0,
		maxScore:      1.0,
		defaultScore:  0.5,
		alpha:         0.15,
		store:         store,
		logger:        logger.With("component", "reputation"),
	}

	// Try to restore state
	if store != nil {
		if loaded, err := store.LoadScores(); err == nil && len(loaded) > 0 {
			rm.scores = loaded
			rm.logger.Info("restored reputation scores", "count", len(loaded))
		}
	}

	return rm
}

// Snapshot persists current scores.
func (r *ReputationManager) Snapshot() error {
	if r.store == nil {
		return nil
	}

	r.scoresMu.RLock()
	snapshot := make(map[string]ReputationScore, len(r.scores))
	for k, v := range r.scores {
		snapshot[k] = v
	}
	r.scoresMu.RUnlock()

	return r.store.SaveScores(snapshot)
}

// Report ingests a new interaction result.
func (r *ReputationManager) Report(peerID string, success bool, latencyMs float64) {
	r.scoresMu.Lock()
	defer r.scoresMu.Unlock()

	score := r.getOrCreateScore(peerID)
	r.applyDecay(&score)

	// Update interaction counts
	if success {
		score.SuccessfulInteractions++

		// Latency Score: 1.0 for <50ms, 0.0 for >2000ms
		latScore := 1.0
		if latencyMs > 50 {
			latScore = math.Max(0, 1.0-(latencyMs-50)/1950.0)
		}

		// EMA update for success
		score.Score = (1-r.alpha)*score.Score + r.alpha*latScore
		score.TotalLatencyMs += latencyMs
	} else {
		score.FailedInteractions++
		// Default failure penalty
		score.Score = math.Max(r.minScore, score.Score-0.05)
	}

	r.updateConfidence(&score)
	score.LastInteraction = time.Now().UnixNano()
	score.LastUpdated = score.LastInteraction
	r.scores[peerID] = score
}

// PoRReport specifically handles Proof of Retrievability results.
func (r *ReputationManager) PoRReport(peerID string, success bool, difficulty float64) {
	r.scoresMu.Lock()
	defer r.scoresMu.Unlock()

	score := r.getOrCreateScore(peerID)
	r.applyDecay(&score)

	score.ChallengesIssued++

	if success {
		score.SuccessfulInteractions++
		// PoR success gives a larger boost based on difficulty
		boost := 0.05 * (1.0 + difficulty)
		score.Score = math.Min(r.maxScore, score.Score+boost)
	} else {
		score.FailedInteractions++
		// PoR failure is a major penalty
		penalty := 0.3
		score.Score = math.Max(r.minScore, score.Score-penalty)
		r.logger.Warn("peer failed PoR challenge", "peer_id", peerID, "penalty", penalty)
	}

	r.updateConfidence(&score)
	score.LastInteraction = time.Now().UnixNano()
	score.LastUpdated = score.LastInteraction
	r.scores[peerID] = score
}

// ReportPenalty applies a specific penalty to a peer.
func (r *ReputationManager) ReportPenalty(peerID string, reason PenaltyReason) {
	r.scoresMu.Lock()
	defer r.scoresMu.Unlock()

	score := r.getOrCreateScore(peerID)
	r.applyDecay(&score)

	var penalty float64
	switch reason {
	case PenaltyTimeout:
		penalty = 0.02
	case PenaltyInvalidData:
		penalty = 0.15
	case PenaltyPoRFailure:
		penalty = 0.30
	case PenaltyMaliciousBehavior:
		penalty = 1.0 // Blacklist
	case PenaltyCongestion:
		penalty = 0.01
	default:
		penalty = 0.05
	}

	score.Score = math.Max(r.minScore, score.Score-penalty)
	score.FailedInteractions++

	r.logger.Debug("applied reputation penalty", "peer_id", peerID, "reason", reason, "penalty", penalty)

	r.updateConfidence(&score)
	score.LastUpdated = time.Now().UnixNano()
	r.scores[peerID] = score
}

func (r *ReputationManager) GetTrustScore(peerID string) (float64, float64) {
	r.scoresMu.RLock()
	defer r.scoresMu.RUnlock()

	score, exists := r.scores[peerID]
	if !exists {
		return r.defaultScore, 0.0
	}

	// Apply decay for read (shallow copy)
	r.applyDecay(&score)
	return score.Score, score.Confidence
}

func (r *ReputationManager) IsTrusted(peerID string) bool {
	score, confidence := r.GetTrustScore(peerID)
	// Must have decent score and some confidence
	return score > 0.4 && confidence > 0.2
}

func (r *ReputationManager) GetTopPeers(n int) []string {
	r.scoresMu.RLock()
	defer r.scoresMu.RUnlock()

	type pSort struct {
		id    string
		score float64
	}
	var list []pSort
	for id, s := range r.scores {
		r.applyDecay(&s)
		list = append(list, pSort{id, s.Confidence * s.Score}) // Weight score by confidence
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].score > list[j].score
	})

	limit := n
	if len(list) < n {
		limit = len(list)
	}

	res := make([]string, limit)
	for i := 0; i < limit; i++ {
		res[i] = list[i].id
	}
	return res
}

// Internal helpers

func (r *ReputationManager) getOrCreateScore(peerID string) ReputationScore {
	if s, exists := r.scores[peerID]; exists {
		return s
	}
	return ReputationScore{
		PeerID:      peerID,
		Score:       r.defaultScore,
		Confidence:  0.0,
		LastUpdated: time.Now().UnixNano(),
	}
}

func (r *ReputationManager) applyDecay(s *ReputationScore) {
	now := time.Now().UnixNano()
	dt := now - s.LastUpdated
	if dt <= 0 {
		return
	}

	// Score decays toward defaultScore
	// confidence decays toward 0

	hours := float64(dt) / float64(time.Hour)
	decayFactor := math.Pow(0.5, hours/(float64(r.decayHalfLife)/float64(time.Hour)))

	// EMA-like decay toward neutral
	s.Score = r.defaultScore + (s.Score-r.defaultScore)*decayFactor
	s.Confidence = s.Confidence * decayFactor
}

func (r *ReputationManager) updateConfidence(s *ReputationScore) {
	total := s.SuccessfulInteractions + s.FailedInteractions
	if total == 0 {
		s.Confidence = 0
		return
	}
	// Asymptotic approach to 1.0
	// 5 interactions -> ~0.8 confidence
	// 20 interactions -> ~0.95 confidence
	s.Confidence = 1.0 - (1.0 / float64(total/2+1))
}

func (r *ReputationManager) GetMetrics() map[string]interface{} {
	r.scoresMu.RLock()
	defer r.scoresMu.RUnlock()

	var sumScore float64
	var sumConfidence float64
	for _, s := range r.scores {
		sumScore += s.Score
		sumConfidence += s.Confidence
	}

	avgScore := 0.0
	avgConfidence := 0.0
	if len(r.scores) > 0 {
		avgScore = sumScore / float64(len(r.scores))
		avgConfidence = sumConfidence / float64(len(r.scores))
	}

	return map[string]interface{}{
		"total_peers":    len(r.scores),
		"avg_score":      avgScore,
		"avg_confidence": avgConfidence,
	}
}

// GetAverageScore returns the average reputation score across all peers
func (r *ReputationManager) GetAverageScore() float64 {
	r.scoresMu.RLock()
	defer r.scoresMu.RUnlock()

	if len(r.scores) == 0 {
		return r.defaultScore
	}

	var sum float64
	for _, s := range r.scores {
		sum += s.Score
	}

	return sum / float64(len(r.scores))
}
