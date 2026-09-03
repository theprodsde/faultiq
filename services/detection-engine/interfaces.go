package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/faultiq/graphclient"
	"github.com/jackc/pgx/v5/pgxpool"
)

// reasoningLog emits a structured JSON log line for machine-parseable reasoning traces.
func reasoningLog(fields map[string]interface{}) {
	b, _ := json.Marshal(fields)
	log.Printf("REASONING: %s", b)
}

// GraphProvider retrieves a Graph for a given namespace.
type GraphProvider interface {
	GetSubgraph(ctx context.Context, namespace string) (*graphclient.Graph, error)
}

// Store persists incidents and candidates.
type Store interface {
	PersistIncident(ctx context.Context, id, project string, detectedAt time.Time) error
	PersistCandidates(ctx context.Context, incidentID string, candidates []string) error
}

// pgStore implements Store using pgxpool.Pool (goroutine-safe)
type pgStore struct {
	conn *pgxpool.Pool
}

func NewPGStore(conn *pgxpool.Pool) Store {
	return &pgStore{conn: conn}
}

func (p *pgStore) PersistIncident(ctx context.Context, id, project string, detectedAt time.Time) error {
	_, err := p.conn.Exec(ctx, `INSERT INTO incidents (id, project_id, detected_at) VALUES ($1,$2,$3) ON CONFLICT (id) DO NOTHING`, id, project, detectedAt)
	return err
}

func (p *pgStore) PersistCandidates(ctx context.Context, incidentID string, candidates []string) error {
	for i, node := range candidates {
		if _, err := p.conn.Exec(ctx, `INSERT INTO rca_candidates (incident_id, rank, candidate_node_id, confidence_score, evidence) VALUES ($1,$2,$3,$4,$5)`, incidentID, i+1, node, 0.5, "{}"); err != nil {
			return err
		}
	}
	return nil
}

func fmtErr(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%v", err)
}
