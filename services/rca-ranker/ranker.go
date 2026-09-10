package main

import (
	"math"
	"sort"
)

type Candidate struct {
	ID           string
	Evidence     int
	Impact       float64   // callerCount / totalNodes
	ErrorRate    float64
	CallerScores []float64 // the confidence scores of failing callers
}

// Score computes a PageRank-inspired weighted score for an RCA candidate.
// A service that MANY FAILING callers all depend on scores higher than one with few callers.
func Score(c Candidate) float64 {
	// Direct evidence (time-normalized, saturates at 10)
	evidenceNorm := math.Min(float64(c.Evidence)/10.0, 1.0)

	// Propagated fault signal: average of failing callers' raw scores
	// If 3 failing callers all depend on this service, their combined failure propagates
	propagation := 0.0
	if len(c.CallerScores) > 0 {
		sum := 0.0
		for _, s := range c.CallerScores {
			sum += s
		}
		propagation = math.Min(sum/float64(len(c.CallerScores)), 1.0)
	}

	// Weighted formula: evidence 40%, propagation 35%, impact 15%, errorRate 10%
	return evidenceNorm*0.40 + propagation*0.35 + c.Impact*0.15 + c.ErrorRate*0.10
}

// Rank returns candidates sorted by descending score.
func Rank(candidates []Candidate) []Candidate {
	if len(candidates) == 0 {
		return candidates
	}
	out := make([]Candidate, len(candidates))
	copy(out, candidates)
	scores := make([]float64, len(out))
	for i, c := range out {
		scores[i] = Score(c)
	}
	sort.Slice(out, func(i, j int) bool {
		return scores[i] > scores[j]
	})
	return out
}
