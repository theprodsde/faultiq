package graphclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Graph is the wire-format service dependency graph shared between graph-manager and consumers.
type Graph struct {
	Nodes       map[string]*Node    `json:"nodes"`
	Edges       map[string][]string `json:"edges"`
	EdgeDetails map[string]*Edge    `json:"edge_details,omitempty"`
}

// Node represents a service (or other entity) in the graph.
type Node struct {
	ID          string   `json:"id"`
	Name        string   `json:"name,omitempty"`
	Type        string   `json:"type,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	StatusClass string   `json:"status_class,omitempty"`
	LatencyP95  int      `json:"latency_p95,omitempty"`
	ErrorRate   float64  `json:"error_rate,omitempty"`
}

// Edge represents a directed relationship between two nodes.
type Edge struct {
	ID           string  `json:"id"`
	From         string  `json:"from"`
	To           string  `json:"to"`
	Type         string  `json:"type,omitempty"`
	SuccessRatio float64 `json:"success_ratio,omitempty"`
	Confidence   float64 `json:"confidence,omitempty"`
	LastObserved int64   `json:"last_observed,omitempty"`
}

// HTTPProvider fetches graphs from a graph-manager HTTP endpoint.
type HTTPProvider struct {
	baseURL string
	client  *http.Client
}

// NewHTTPProvider returns an HTTPProvider that calls baseURL/api/v1/graphs?namespace=<ns>.
func NewHTTPProvider(baseURL string) *HTTPProvider {
	return &HTTPProvider{baseURL: baseURL, client: &http.Client{}}
}

// GetSubgraph fetches the graph for the given namespace.
func (p *HTTPProvider) GetSubgraph(ctx context.Context, namespace string) (*Graph, error) {
	url := fmt.Sprintf("%s/api/v1/graphs?namespace=%s", p.baseURL, namespace)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graph-manager returned %d", resp.StatusCode)
	}
	var g Graph
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return nil, err
	}
	return &g, nil
}
