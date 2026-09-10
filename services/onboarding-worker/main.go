// onboarding-worker processes onboarding jobs from Redis.
// It reads a service-map.yaml, writes graph nodes + edges to graph-manager,
// and marks the project graph as PUBLISHED in Postgres.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

// ─── Service Map types (mirrors scripts/service-map.yaml) ────────────────────

type ServiceEntry struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	HealthURL   string   `yaml:"healthUrl"`
	Calls       []string `yaml:"calls"`
	Tags        []string `yaml:"tags"`
	TimeoutSecs int      `yaml:"timeoutSeconds"`
}

type ProjectEntry struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Namespace   string         `yaml:"namespace"`
	Tenant      string         `yaml:"tenant"`
	Environment string         `yaml:"environment"`
	Services    []ServiceEntry `yaml:"services"`
}

type ServiceMap struct {
	Version  string         `yaml:"version"`
	Projects []ProjectEntry `yaml:"projects"`
}

// ─── Graph-manager API payloads ───────────────────────────────────────────────

type GraphNode struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Tags        []string `json:"tags,omitempty"`
	StatusClass string   `json:"statusClass"`
	LatencyP95  int      `json:"latencyP95"`
	ErrorRate   float64  `json:"errorRate"`
}

type GraphEdge struct {
	ID           string  `json:"id"`
	From         string  `json:"from"`
	To           string  `json:"to"`
	Type         string  `json:"type"`
	Confidence   float64 `json:"confidence"`
	SuccessRatio float64 `json:"successRatio"`
}

// ─── Schema ───────────────────────────────────────────────────────────────────

func ensureSchema(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS onboarding_jobs (
			id          TEXT PRIMARY KEY,
			project_id  TEXT,
			mode        TEXT,
			status      TEXT,
			payload     JSONB,
			created_at  TIMESTAMP DEFAULT now()
		)`)
	return err
}

// ─── Graph builder ────────────────────────────────────────────────────────────

func loadServiceMap(path string) (*ServiceMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var sm ServiceMap
	if err := yaml.Unmarshal(data, &sm); err != nil {
		return nil, fmt.Errorf("parse service map: %w", err)
	}
	return &sm, nil
}

func buildGraph(ctx context.Context, graphManagerURL string, project ProjectEntry) error {
	ns := project.Namespace
	client := &http.Client{Timeout: 10 * time.Second}

	log.Printf("onboarding: building graph for %s (%d services)", ns, len(project.Services))

	// Write nodes concurrently — up to 8 in-flight requests at a time
	eg, egCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, 8)
	for _, svc := range project.Services {
		svc := svc
		sem <- struct{}{}
		eg.Go(func() error {
			defer func() { <-sem }()
			node := GraphNode{
				ID:          svc.ID,
				Name:        svc.Name,
				Type:        strings.ToUpper(svc.Type),
				Tags:        svc.Tags,
				StatusClass: "2xx",
				LatencyP95:  50,
				ErrorRate:   0.001,
			}
			if node.Type == "" {
				node.Type = "SERVICE"
			}
			nodeJSON, _ := json.Marshal(node)
			nodeURL := fmt.Sprintf("%s/nodes?namespace=%s", graphManagerURL, ns)
			req, _ := http.NewRequestWithContext(egCtx, "POST", nodeURL, bytes.NewReader(nodeJSON))
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("write node %s: %w", svc.ID, err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
				log.Printf("onboarding: node %s returned %d", svc.ID, resp.StatusCode)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// Pre-build edge list with stable IDs before fan-out (avoids shared counter race)
	type edgeTask struct {
		from string
		to   string
		idx  int
	}
	var edgeTasks []edgeTask
	for _, svc := range project.Services {
		for _, target := range svc.Calls {
			edgeTasks = append(edgeTasks, edgeTask{from: svc.ID, to: target, idx: len(edgeTasks)})
		}
	}

	// Write edges concurrently — errors logged, not fatal (edge failures don't abort the graph)
	eg2, egCtx2 := errgroup.WithContext(ctx)
	sem2 := make(chan struct{}, 8)
	for _, et := range edgeTasks {
		et := et
		sem2 <- struct{}{}
		eg2.Go(func() error {
			defer func() { <-sem2 }()
			edge := GraphEdge{
				ID:           fmt.Sprintf("edge-%d", et.idx),
				From:         et.from,
				To:           et.to,
				Type:         "CALLS",
				Confidence:   0.95,
				SuccessRatio: 0.99,
			}
			edgeJSON, _ := json.Marshal(edge)
			edgeURL := fmt.Sprintf("%s/edges?namespace=%s", graphManagerURL, ns)
			req, _ := http.NewRequestWithContext(egCtx2, "POST", edgeURL, bytes.NewReader(edgeJSON))
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("onboarding: write edge %s→%s: %v", et.from, et.to, err)
				return nil // edge failures are non-fatal
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return nil
		})
	}
	eg2.Wait() //nolint:errcheck — goroutines always return nil

	log.Printf("onboarding: graph built for %s — %d nodes, %d edges",
		ns, len(project.Services), len(edgeTasks))
	return nil
}

// markProjectPublished updates the project's graphStatus in Postgres.
func markProjectPublished(ctx context.Context, conn *pgx.Conn, projectID, namespace string) {
	_, err := conn.Exec(ctx,
		`UPDATE projects SET metadata = jsonb_set(
			COALESCE(metadata, '{}'),
			'{graphStatus}', '"PUBLISHED"'
		) WHERE id = $1`, projectID)
	if err != nil {
		log.Printf("onboarding: mark published %s: %v", projectID, err)
	} else {
		log.Printf("onboarding: project %s marked PUBLISHED (namespace=%s)", projectID, namespace)
	}
}

// ─── Job processing ───────────────────────────────────────────────────────────

func processJob(ctx context.Context, conn *pgx.Conn, graphManagerURL, serviceMapFile string, job map[string]interface{}) {
	jobID, _ := job["job_id"].(string)
	projectID, _ := job["project_id"].(string)
	mode, _ := job["mode"].(string)

	if jobID == "" {
		log.Printf("onboarding: missing job_id, skipping")
		return
	}

	// Insert job record
	payload, _ := json.Marshal(job["sources"])
	_, _ = conn.Exec(ctx,
		`INSERT INTO onboarding_jobs (id, project_id, mode, status, payload)
		 VALUES ($1,$2,$3,'processing',$4) ON CONFLICT(id) DO UPDATE SET status='processing'`,
		jobID, projectID, mode, payload)

	log.Printf("onboarding: processing job=%s project=%s mode=%s", jobID, projectID, mode)

	// Extract sources once for use by both mode-specific and service-map code paths
	sources, _ := job["sources"].(map[string]interface{})

	// ── MODE-SPECIFIC DISCOVERY ──────────────────────────────────────────────
	// kubernetes and openapi modes build the graph directly without a service-map file.
	// hybrid tries kubernetes first and falls back to service-map on failure.
	// All other modes (default: "service-map") fall through to the existing logic below.

	// helper: resolve environment from sources or default
	modeEnv := ""
	if sources != nil {
		modeEnv, _ = sources["environment"].(string)
	}
	if modeEnv == "" {
		modeEnv = "default"
	}

	switch mode {
	case "kubernetes":
		kcURL := os.Getenv("K8S_CONNECTOR_URL")
		if kcURL == "" {
			kcURL = "http://k8s-connector:8093"
		}
		ns := modeEnv // use project environment as K8s namespace
		kcResp, kcErr := http.Get(kcURL + "/discover?namespace=" + ns)
		if kcErr != nil {
			log.Printf("onboarding: k8s discover %s: %v", kcURL, kcErr)
			_, _ = conn.Exec(ctx, `UPDATE onboarding_jobs SET status='failed' WHERE id=$1`, jobID)
			return
		}
		defer kcResp.Body.Close()
		var svcEntries []ServiceEntry
		if decErr := json.NewDecoder(kcResp.Body).Decode(&svcEntries); decErr != nil || len(svcEntries) == 0 {
			log.Printf("onboarding: k8s discover empty or decode error for ns=%s: %v", ns, decErr)
			_, _ = conn.Exec(ctx, `UPDATE onboarding_jobs SET status='failed' WHERE id=$1`, jobID)
			return
		}
		projNS := projectID + ":" + ns
		proj := ProjectEntry{ID: projectID, Namespace: projNS, Environment: ns, Services: svcEntries}
		if bErr := buildGraph(ctx, graphManagerURL, proj); bErr != nil {
			log.Printf("onboarding: k8s build graph: %v", bErr)
			_, _ = conn.Exec(ctx, `UPDATE onboarding_jobs SET status='failed' WHERE id=$1`, jobID)
			return
		}
		markProjectPublished(ctx, conn, projectID, projNS)
		_, _ = conn.Exec(ctx, `UPDATE onboarding_jobs SET status='completed' WHERE id=$1`, jobID)
		log.Printf("onboarding: job=%s kubernetes mode completed namespace=%s services=%d", jobID, projNS, len(svcEntries))
		return

	case "openapi":
		openapiSrcURL := ""
		if sources != nil {
			openapiSrcURL, _ = sources["openapi"].(string)
		}
		if openapiSrcURL == "" {
			log.Printf("onboarding: openapi mode but no 'openapi' URL in sources for job=%s", jobID)
			_, _ = conn.Exec(ctx, `UPDATE onboarding_jobs SET status='failed' WHERE id=$1`, jobID)
			return
		}
		// fetch + parse OpenAPI spec → extract servers + paths → create service nodes from server URLs
		opResp, fetchErr := http.Get(openapiSrcURL)
		if fetchErr != nil {
			log.Printf("onboarding: fetch openapi %s: %v", openapiSrcURL, fetchErr)
			_, _ = conn.Exec(ctx, `UPDATE onboarding_jobs SET status='failed' WHERE id=$1`, jobID)
			return
		}
		defer opResp.Body.Close()
		var spec struct {
			Info    struct{ Title string `json:"title"` } `json:"info"`
			Servers []struct{ URL string `json:"url"` }  `json:"servers"`
		}
		if decErr := json.NewDecoder(opResp.Body).Decode(&spec); decErr != nil {
			log.Printf("onboarding: parse openapi spec: %v", decErr)
			_, _ = conn.Exec(ctx, `UPDATE onboarding_jobs SET status='failed' WHERE id=$1`, jobID)
			return
		}
		var openapiSvcs []ServiceEntry
		for i, srv := range spec.Servers {
			svcName := "service"
			if parsed, pErr := url.Parse(srv.URL); pErr == nil && parsed.Hostname() != "" {
				hostParts := strings.Split(parsed.Hostname(), ".")
				if len(hostParts) > 0 && hostParts[0] != "" {
					svcName = hostParts[0]
				}
			}
			if svcName == "service" {
				svcName = fmt.Sprintf("service-%d", i+1)
			}
			svcID := "svc_" + strings.ReplaceAll(strings.ToLower(svcName), "-", "_")
			openapiSvcs = append(openapiSvcs, ServiceEntry{ID: svcID, Name: svcName, Type: "SERVICE"})
		}
		if len(openapiSvcs) == 0 && spec.Info.Title != "" {
			svcID := "svc_" + strings.ToLower(strings.ReplaceAll(spec.Info.Title, " ", "_"))
			openapiSvcs = []ServiceEntry{{ID: svcID, Name: spec.Info.Title, Type: "SERVICE"}}
		}
		projNS := projectID + ":prod"
		proj := ProjectEntry{ID: projectID, Name: spec.Info.Title, Namespace: projNS, Environment: "prod", Services: openapiSvcs}
		if bErr := buildGraph(ctx, graphManagerURL, proj); bErr != nil {
			log.Printf("onboarding: openapi build graph: %v", bErr)
			_, _ = conn.Exec(ctx, `UPDATE onboarding_jobs SET status='failed' WHERE id=$1`, jobID)
			return
		}
		markProjectPublished(ctx, conn, projectID, projNS)
		_, _ = conn.Exec(ctx, `UPDATE onboarding_jobs SET status='completed' WHERE id=$1`, jobID)
		log.Printf("onboarding: job=%s openapi mode completed project=%s services=%d", jobID, projectID, len(openapiSvcs))
		return

	case "hybrid":
		// Try kubernetes first, fall back to service-map.yaml on failure
		kcURL := os.Getenv("K8S_CONNECTOR_URL")
		if kcURL == "" {
			kcURL = "http://k8s-connector:8093"
		}
		ns := modeEnv
		k8sOK := false
		if kcResp, kcErr := http.Get(kcURL + "/discover?namespace=" + ns); kcErr == nil {
			var svcEntries []ServiceEntry
			decErr := json.NewDecoder(kcResp.Body).Decode(&svcEntries)
			kcResp.Body.Close()
			if decErr == nil && len(svcEntries) > 0 {
				projNS := projectID + ":" + ns
				proj := ProjectEntry{ID: projectID, Namespace: projNS, Environment: ns, Services: svcEntries}
				if bErr := buildGraph(ctx, graphManagerURL, proj); bErr == nil {
					markProjectPublished(ctx, conn, projectID, projNS)
					_, _ = conn.Exec(ctx, `UPDATE onboarding_jobs SET status='completed' WHERE id=$1`, jobID)
					log.Printf("onboarding: job=%s hybrid mode completed via kubernetes namespace=%s", jobID, projNS)
					k8sOK = true
				}
			}
		}
		if k8sOK {
			return
		}
		log.Printf("onboarding: hybrid mode: kubernetes unavailable or empty, falling back to service-map for job=%s", jobID)
		// fall through to service-map logic below
	}

	// Determine service map — prefer inline YAML, then file path, then default
	mapFile := serviceMapFile
	if sources != nil {
		if f, ok := sources["serviceMapFile"].(string); ok && f != "" {
			mapFile = f
		}
		// Inline YAML: write to a temp file and use that
		if inlineYAML, ok := sources["serviceMapYaml"].(string); ok && inlineYAML != "" {
			tmp, err := os.CreateTemp("", "service-map-*.yaml")
			if err == nil {
				if _, werr := tmp.WriteString(inlineYAML); werr == nil {
					tmp.Close()
					mapFile = tmp.Name()
					defer os.Remove(mapFile) // clean up temp file after use
					log.Printf("onboarding: using inline YAML (%d bytes), temp=%s", len(inlineYAML), mapFile)
				} else {
					tmp.Close()
				}
			}
		}
	}

	// Load service map
	sm, err := loadServiceMap(mapFile)
	if err != nil {
		log.Printf("onboarding: load service map: %v", err)
		_, _ = conn.Exec(ctx, `UPDATE onboarding_jobs SET status='failed' WHERE id=$1`, jobID)
		return
	}

	// Find matching project(s) from the map
	built := 0
	for _, proj := range sm.Projects {
		// Match by project ID or by namespace prefix
		if proj.ID == projectID || strings.HasPrefix(proj.Namespace, projectID) || projectID == "" {
			if err := buildGraph(ctx, graphManagerURL, proj); err != nil {
				log.Printf("onboarding: build graph %s: %v", proj.Namespace, err)
				continue
			}
			// Mark project published
			pid := proj.ID
			if projectID != "" {
				pid = projectID
			}
			markProjectPublished(ctx, conn, pid, proj.Namespace)
			built++
		}
	}

	// If no match found by ID, build all projects in the map
	if built == 0 && len(sm.Projects) > 0 {
		for _, proj := range sm.Projects {
			if err := buildGraph(ctx, graphManagerURL, proj); err != nil {
				log.Printf("onboarding: build graph %s: %v", proj.Namespace, err)
				continue
			}
			markProjectPublished(ctx, conn, proj.ID, proj.Namespace)
			built++
		}
	}

	status := "completed"
	if built == 0 {
		status = "failed"
		log.Printf("onboarding: no projects matched for job=%s", jobID)
	} else {
		log.Printf("onboarding: job=%s completed — built %d project graph(s)", jobID, built)
	}
	_, _ = conn.Exec(ctx, `UPDATE onboarding_jobs SET status=$1 WHERE id=$2`, status, jobID)
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	ctx := context.Background()

	raddr := os.Getenv("REDIS_ADDR")
	if raddr == "" {
		raddr = "redis:6379"
	}
	pg := os.Getenv("PG_DSN")
	if pg == "" {
		pg = "postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable"
	}
	graphManagerURL := os.Getenv("GRAPH_MANAGER_URL")
	if graphManagerURL == "" {
		graphManagerURL = "http://graph-manager:8086/api/v1/graphs"
	}
	serviceMapFile := os.Getenv("SERVICE_MAP_FILE")
	if serviceMapFile == "" {
		serviceMapFile = "/etc/onboarding/service-map.yaml"
	}

	conn, err := pgx.Connect(ctx, pg)
	if err != nil {
		log.Fatalf("onboarding: pg connect: %v", err)
	}
	defer conn.Close(ctx)

	if err := ensureSchema(ctx, conn); err != nil {
		log.Fatalf("onboarding: ensure schema: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: raddr})
	log.Printf("onboarding-worker: ready (redis=%s graphManager=%s mapFile=%s)",
		raddr, graphManagerURL, serviceMapFile)

	for {
		res, err := rdb.BLPop(ctx, 0, "onboarding_jobs").Result()
		if err != nil {
			log.Printf("onboarding: redis blpop error: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if len(res) < 2 {
			continue
		}
		var job map[string]interface{}
		if err := json.Unmarshal([]byte(res[1]), &job); err != nil {
			log.Printf("onboarding: invalid job payload: %v", err)
			continue
		}
		processJob(ctx, conn, graphManagerURL, serviceMapFile, job)
	}
}
