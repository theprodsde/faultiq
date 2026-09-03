package main

import (
    "testing"
)

func TestScoreAndRank(t *testing.T) {
    cand := Candidate{ID: "a", Evidence: 10, Impact: 1.0}
    s := Score(cand)
    if s <= 0 {
        t.Fatalf("expected positive score, got %v", s)
    }

    candidates := []Candidate{
        {ID: "a", Evidence: 10, Impact: 1.0},
        {ID: "b", Evidence: 5, Impact: 5.0},
        {ID: "c", Evidence: 8, Impact: 0.5},
    }
    ranked := Rank(candidates)
    if len(ranked) != 3 {
        t.Fatalf("expected 3 candidates, got %d", len(ranked))
    }
    // ensure descending order
    for i := 1; i < len(ranked); i++ {
        if Score(ranked[i]) > Score(ranked[i-1]) {
            t.Fatalf("candidates not sorted: %v before %v", ranked[i-1], ranked[i])
        }
    }
}
