package main

import (
	"container/heap"
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

// candidateHeap is a min-heap of RankedCandidate ordered by ascending Confidence.
// Used by TopKCandidates to maintain the k highest-confidence candidates in O(n log k).
type candidateHeap []RankedCandidate

func (h candidateHeap) Len() int           { return len(h) }
func (h candidateHeap) Less(i, j int) bool { return h[i].Confidence < h[j].Confidence }
func (h candidateHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *candidateHeap) Push(x interface{}) {
	*h = append(*h, x.(RankedCandidate))
}
func (h *candidateHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// TopKCandidates returns the top k highest-confidence candidates in O(n log k).
// Falls back to Rank when k <= 0 or k >= len(candidates).
func TopKCandidates(candidates []Candidate, k int) []Candidate {
	if k <= 0 || k >= len(candidates) {
		return Rank(candidates)
	}
	h := &candidateHeap{}
	heap.Init(h)
	for _, c := range candidates {
		heap.Push(h, RankedCandidate{
			Service:    c.ID,
			Confidence: Score(c),
			Evidence:   c.Evidence,
			Impact:     c.Impact,
		})
		if h.Len() > k {
			heap.Pop(h) // remove the lowest-confidence entry
		}
	}
	result := make([]Candidate, h.Len())
	for i := h.Len() - 1; i >= 0; i-- {
		rc := heap.Pop(h).(RankedCandidate)
		result[i] = Candidate{ID: rc.Service, Evidence: rc.Evidence, Impact: rc.Impact}
	}
	return result
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
