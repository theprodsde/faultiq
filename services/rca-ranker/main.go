package main

import (
    "encoding/json"
    "io"
    "log"
    "net"
    "net/http"
    "os"
    "sort"
    "time"
)

type EvidencePayload struct {
    EvidenceCount int       `json:"evidence_count"`
    Impact        float64   `json:"impact"`
    ErrorRate     float64   `json:"error_rate"`
    CallerScores  []float64 `json:"caller_scores,omitempty"`
}

type RankRequest struct {
    IncidentID string                     `json:"incident_id"`
    Services   map[string]EvidencePayload `json:"services"` // service -> evidence
    TopK       int                        `json:"top_k,omitempty"`
}

type RankedCandidate struct {
    Service    string  `json:"service"`
    Confidence float64 `json:"confidence"`
    Rank       int     `json:"rank"`
    Evidence   int     `json:"evidence"`
    Impact     float64 `json:"impact"`
}

type RankResponse struct {
    IncidentID string             `json:"incident_id"`
    Candidates []RankedCandidate  `json:"candidates"`
    RootCause  string             `json:"root_cause"`
    Timestamp  int64              `json:"timestamp"`
}

func main() {
    http.HandleFunc("/health", healthHandler)
    http.HandleFunc("/rank", rankHandler)

    port := os.Getenv("RCA_RANKER_PORT")
    if port == "" {
        port = ":8087"
    }

    // Use net.Listen with tcp4 to avoid IPv6 dual-stack epoll issues in Docker
    ln, err := net.Listen("tcp4", port)
    if err != nil {
        log.Fatalf("rca-ranker: failed to listen on %s: %v", port, err)
    }
    log.Printf("rca-ranker: serving on %s", port)
    log.Fatal(http.Serve(ln, nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func rankHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "failed to read body", http.StatusBadRequest)
        return
    }

    var req RankRequest
    if err := json.Unmarshal(body, &req); err != nil {
        http.Error(w, "invalid JSON", http.StatusBadRequest)
        return
    }

    if req.IncidentID == "" {
        http.Error(w, "incident_id required", http.StatusBadRequest)
        return
    }

    if len(req.Services) == 0 {
        http.Error(w, "services required", http.StatusBadRequest)
        return
    }

    // Score each service
    var candidates []RankedCandidate
    for service, evidence := range req.Services {
        c := Candidate{
            ID:           service,
            Evidence:     evidence.EvidenceCount,
            Impact:       evidence.Impact,
            ErrorRate:    evidence.ErrorRate,
            CallerScores: evidence.CallerScores,
        }
        score := Score(c)
        candidates = append(candidates, RankedCandidate{
            Service:    service,
            Confidence: score,
            Evidence:   evidence.EvidenceCount,
            Impact:     evidence.Impact,
        })
    }

    // Sort by confidence descending
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].Confidence > candidates[j].Confidence
    })

    // Assign ranks
    for i := range candidates {
        candidates[i].Rank = i + 1
    }

    // Apply top-k truncation if requested
    if req.TopK > 0 && req.TopK < len(candidates) {
        candidates = candidates[:req.TopK]
    }

    // Root cause is the top-ranked service
    rootCause := ""
    if len(candidates) > 0 {
        rootCause = candidates[0].Service
    }

    resp := RankResponse{
        IncidentID: req.IncidentID,
        Candidates: candidates,
        RootCause:  rootCause,
        Timestamp:  time.Now().UnixMilli(),
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(resp)
}
