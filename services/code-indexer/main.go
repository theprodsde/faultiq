// code-indexer connects the runtime fault layer (TechGraph) to the code layer (git repos).
//
// Lifecycle per repo:
//   1. git clone --depth=50 (shallow — only recent history, fast and small)
//   2. git log <lastHash>..HEAD — only NEW commits since last run (incremental)
//   3. parse changed files — extract function names by language
//   4. write CodeFunction, GitCommit, IMPLEMENTS nodes to graph-manager (Neo4j)
//   5. update last_commit_hash in Postgres
//   6. rm -rf clone dir — disk freed immediately after analysis
//
// This means disk is used ONLY during active analysis (seconds), then released.
// First run: ~20-50MB per repo for 50-commit shallow clone.
// Subsequent runs: only new commits — orders of magnitude faster.

package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

// ─── Service Map types (mirrors scripts/service-map.yaml) ──────────────────

type ServiceEntry struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	HealthURL   string   `yaml:"healthUrl"`
	Repo        string   `yaml:"repo"`
	Branch      string   `yaml:"branch"`
	CodePath    string   `yaml:"codePath"`
	Calls       []string `yaml:"calls"`
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

// ─── Graph-manager payloads ────────────────────────────────────────────────

type CodeNode struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`        // CODEFUNCTION | CODEFILE | GITCOMMIT
	ServiceID  string `json:"serviceId"`
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	CommitHash string `json:"commitHash,omitempty"`
	Author     string `json:"author,omitempty"`
	Message    string `json:"message,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"`
}

type CodeEdge struct {
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"` // IMPLEMENTS | CHANGED_IN
}

// ─── Commit represents one git commit with its changed files ───────────────

type Commit struct {
	Hash    string
	Author  string
	Message string
	Date    time.Time
	Files   []string // changed file paths
}

// ─── Indexer ───────────────────────────────────────────────────────────────

type Indexer struct {
	db             *pgxpool.Pool
	graphManagerURL string
	httpClient     *http.Client
	cloneBase      string
}

func NewIndexer(db *pgxpool.Pool, graphManagerURL string) *Indexer {
	return &Indexer{
		db:              db,
		graphManagerURL: graphManagerURL,
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		cloneBase:       os.TempDir(),
	}
}

// IndexRepoGroup clones a repo ONCE and indexes all services that share it.
// This is the key optimization for monoliths — one repo, N services, one clone.
func (idx *Indexer) IndexRepoGroup(ctx context.Context, repoURL, branch string, project ProjectEntry, services []ServiceEntry) error {
	log.Printf("code-indexer: repo %s — %d services to index", repoURL, len(services))

	// Use shared clone dir keyed by repo URL hash (safe path)
	cloneDir := filepath.Join(idx.cloneBase, fmt.Sprintf("ci-repo-%s", sanitizeID(repoURL)[:40]))

	// Clone or pull once for all services
	cloned, err := cloneOrPull(ctx, repoURL, branch, cloneDir)
	if err != nil {
		return fmt.Errorf("clone %s: %w", repoURL, err)
	}
	// Always clean up the clone dir after all services are processed
	defer func() {
		if rmErr := os.RemoveAll(cloneDir); rmErr != nil {
			log.Printf("code-indexer: cleanup %s: %v", cloneDir, rmErr)
		} else {
			log.Printf("code-indexer: cleaned up %s (disk freed, %d services processed)", cloneDir, len(services))
		}
	}()

	_ = cloned // used for logging if needed

	// Index each service from the same clone — different codePaths for monolith
	for _, svc := range services {
		if err := idx.indexFromClone(ctx, project, svc, cloneDir); err != nil {
			log.Printf("code-indexer: %s/%s: %v", project.Namespace, svc.ID, err)
		}
	}
	return nil
}

// indexFromClone indexes one service using an already-cloned repo directory.
func (idx *Indexer) indexFromClone(ctx context.Context, project ProjectEntry, svc ServiceEntry, cloneDir string) error {
	log.Printf("code-indexer: indexing %s/%s (codePath=%q)", project.Namespace, svc.ID, svc.CodePath)

	lastHash := idx.getLastHash(ctx, project.ID, svc.ID)
	commits, err := gitLogFiltered(ctx, cloneDir, lastHash, 50, svc.CodePath)
	if err != nil {
		return fmt.Errorf("git log: %w", err)
	}
	if len(commits) == 0 && lastHash != "" {
		log.Printf("code-indexer: %s/%s — no new commits", project.Namespace, svc.ID)
		return nil
	}
	log.Printf("code-indexer: %s/%s — processing %d commits", project.Namespace, svc.ID, len(commits))

	var nodes []CodeNode
	var edges []CodeEdge
	edgeIdx := 0

	for _, commit := range commits {
		commitNodeID := fmt.Sprintf("commit_%s_%s", svc.ID, commit.Hash[:min(12, len(commit.Hash))])
		nodes = append(nodes, CodeNode{
			ID:        commitNodeID,
			Name:      commit.Message,
			Type:      "GITCOMMIT",
			ServiceID: svc.ID,
			CommitHash: commit.Hash,
			Author:    commit.Author,
			Message:   commit.Message,
			Timestamp: commit.Date.UTC().Format(time.RFC3339),
		})

		for _, filePath := range commit.Files {
			if !isCodeFile(filePath) {
				continue
			}
			// For monolith: only files within the service's codePath scope
			if svc.CodePath != "" {
				cleanPath := strings.TrimPrefix(svc.CodePath, "/")
				if !strings.HasPrefix(filePath, cleanPath) {
					continue
				}
			}
			fullPath := filepath.Join(cloneDir, filePath)
			funcs := extractFunctions(fullPath, filePath)
			for _, fn := range funcs {
				fnNodeID := sanitizeID(fmt.Sprintf("fn_%s_%s_%s", svc.ID, filePath, fn.name))
				nodes = append(nodes, CodeNode{
					ID:        fnNodeID,
					Name:      fn.name,
					Type:      "CODEFUNCTION",
					ServiceID: svc.ID,
					File:      filePath,
					Line:      fn.line,
					CommitHash: commit.Hash,
				})
				edges = append(edges, CodeEdge{ID: fmt.Sprintf("impl-%d", edgeIdx), From: fnNodeID, To: svc.ID, Type: "IMPLEMENTS"})
				edgeIdx++
				edges = append(edges, CodeEdge{ID: fmt.Sprintf("chg-%d", edgeIdx), From: fnNodeID, To: commitNodeID, Type: "CHANGED_IN"})
				edgeIdx++
			}
		}
	}

	if err := idx.writeToGraph(ctx, project.Namespace, nodes, edges); err != nil {
		return err
	}
	if len(commits) > 0 {
		idx.saveLastHash(ctx, project.ID, svc.ID, commits[0].Hash)
	}
	log.Printf("code-indexer: ✓ %s/%s — %d nodes, %d edges", project.Namespace, svc.ID, len(nodes), len(edges))
	return nil
}

// gitLogFiltered returns commits that touch files in the given codePath (for monolith scoping).
func gitLogFiltered(ctx context.Context, repoDir, afterHash string, maxN int, codePath string) ([]Commit, error) {
	args := []string{"-C", repoDir, "log",
		"--format=%H|%ae|%s|%ai",
		"--name-only",
		fmt.Sprintf("-n%d", maxN),
	}
	if afterHash != "" {
		args = append(args, fmt.Sprintf("%s..HEAD", afterHash))
	}
	// For monolith: scope git log to the subdirectory — drastically reduces noise
	if codePath != "" {
		args = append(args, "--", strings.TrimPrefix(codePath, "/"))
	}
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	return parseGitLog(string(out)), nil
}

// IndexService runs the full lifecycle for one service repo:
// clone → log → parse → write to Neo4j → update state → cleanup
func (idx *Indexer) IndexService(ctx context.Context, project ProjectEntry, svc ServiceEntry) error {
	if svc.Repo == "" {
		return nil // no repo configured for this service
	}

	branch := svc.Branch
	if branch == "" {
		branch = "main"
	}

	log.Printf("code-indexer: indexing %s/%s → %s@%s", project.Namespace, svc.ID, svc.Repo, branch)

	// 1. Get last indexed commit from Postgres
	lastHash := idx.getLastHash(ctx, project.ID, svc.ID)

	// 2. Clone or pull the repo into a temp dir
	cloneDir := filepath.Join(idx.cloneBase, fmt.Sprintf("ci-%s-%s", project.ID, svc.ID))
	cloned, err := cloneOrPull(ctx, svc.Repo, branch, cloneDir)
	if err != nil {
		return fmt.Errorf("git clone/pull %s: %w", svc.Repo, err)
	}

	// IMPORTANT: Always delete the clone dir when done, regardless of outcome
	defer func() {
		if removeErr := os.RemoveAll(cloneDir); removeErr != nil {
			log.Printf("code-indexer: cleanup %s: %v", cloneDir, removeErr)
		} else {
			log.Printf("code-indexer: cleaned up %s (disk freed)", cloneDir)
		}
	}()

	// Scope to codePath for monolith (e.g. "src/payments/")
	analysisDir := cloneDir
	if svc.CodePath != "" {
		analysisDir = filepath.Join(cloneDir, svc.CodePath)
	}

	// 3. Get list of commits newer than lastHash (incremental)
	commits, err := gitLog(ctx, cloneDir, lastHash, 50)
	if err != nil {
		return fmt.Errorf("git log %s: %w", svc.ID, err)
	}
	if len(commits) == 0 && !cloned {
		log.Printf("code-indexer: %s/%s — no new commits since %s", project.Namespace, svc.ID, lastHash[:min(8, len(lastHash))])
		return nil
	}
	log.Printf("code-indexer: %s/%s — %d new commits to process", project.Namespace, svc.ID, len(commits))

	// 4. For each commit, parse changed files and extract functions
	var nodes []CodeNode
	var edges []CodeEdge
	edgeIdx := 0

	for _, commit := range commits {
		// Write GitCommit node
		commitNodeID := fmt.Sprintf("commit_%s", commit.Hash[:12])
		nodes = append(nodes, CodeNode{
			ID:        commitNodeID,
			Name:      commit.Message,
			Type:      "GITCOMMIT",
			ServiceID: svc.ID,
			CommitHash: commit.Hash,
			Author:    commit.Author,
			Message:   commit.Message,
			Timestamp: commit.Date.UTC().Format(time.RFC3339),
		})

		// For each changed file, extract function names
		for _, filePath := range commit.Files {
			if !isCodeFile(filePath) {
				continue
			}
			// Only include files in the codePath scope (for monolith)
			if svc.CodePath != "" && !strings.HasPrefix(filePath, strings.TrimPrefix(svc.CodePath, "/")) {
				continue
			}
			fullPath := filepath.Join(analysisDir, filepath.Base(filePath))
			if svc.CodePath == "" {
				fullPath = filepath.Join(cloneDir, filePath)
			}

			funcs := extractFunctions(fullPath, filePath)
			for _, fn := range funcs {
				fnNodeID := sanitizeID(fmt.Sprintf("fn_%s_%s_%s", svc.ID, filePath, fn.name))
				nodes = append(nodes, CodeNode{
					ID:        fnNodeID,
					Name:      fn.name,
					Type:      "CODEFUNCTION",
					ServiceID: svc.ID,
					File:      filePath,
					Line:      fn.line,
					CommitHash: commit.Hash,
				})
				// IMPLEMENTS: function → service
				edges = append(edges, CodeEdge{
					ID:   fmt.Sprintf("impl-%d", edgeIdx),
					From: fnNodeID,
					To:   svc.ID,
					Type: "IMPLEMENTS",
				})
				edgeIdx++
				// CHANGED_IN: function → commit
				edges = append(edges, CodeEdge{
					ID:   fmt.Sprintf("changed-%d", edgeIdx),
					From: fnNodeID,
					To:   commitNodeID,
					Type: "CHANGED_IN",
				})
				edgeIdx++
			}
		}
	}

	// 5. Write nodes and edges to graph-manager (Neo4j)
	if err := idx.writeToGraph(ctx, project.Namespace, nodes, edges); err != nil {
		return fmt.Errorf("write to graph %s: %w", svc.ID, err)
	}

	// 6. Update last processed commit hash in Postgres
	if len(commits) > 0 {
		idx.saveLastHash(ctx, project.ID, svc.ID, commits[0].Hash)
	}

	log.Printf("code-indexer: ✓ %s/%s — wrote %d nodes, %d edges", project.Namespace, svc.ID, len(nodes), len(edges))
	return nil
	// defer above runs: rm -rf cloneDir
}

// ─── Git operations ─────────────────────────────────────────────────────────

// cloneOrPull clones the repo shallowly if it doesn't exist, or fetches new commits.
// Returns true if this was a fresh clone (full index needed), false for incremental fetch.
func cloneOrPull(ctx context.Context, repoURL, branch, cloneDir string) (bool, error) {
	if _, err := os.Stat(filepath.Join(cloneDir, ".git")); os.IsNotExist(err) {
		// Fresh clone — shallow (depth 50) to save disk
		cmd := exec.CommandContext(ctx, "git", "clone",
			"--depth", "50",
			"--branch", branch,
			"--single-branch",
			"--no-tags",
			repoURL, cloneDir)
		cmd.Env = append(os.Environ(),
			"GIT_TERMINAL_PROMPT=0",   // never prompt for credentials
			"GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=no -o BatchMode=yes",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return false, fmt.Errorf("git clone: %w\noutput: %s", err, out)
		}
		return true, nil
	}

	// Already cloned — fetch only new commits (no full re-download)
	cmd := exec.CommandContext(ctx, "git", "-C", cloneDir, "fetch",
		"--depth", "50",
		"--no-tags",
		"origin", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If fetch fails, wipe and re-clone on next run
		_ = os.RemoveAll(cloneDir)
		return false, fmt.Errorf("git fetch: %w\noutput: %s", err, out)
	}
	// Reset to fetched origin
	exec.CommandContext(ctx, "git", "-C", cloneDir, "reset", "--hard", "FETCH_HEAD").Run()
	return false, nil
}

// gitLog returns commits newer than afterHash.
// If afterHash is empty, returns the last maxN commits.
func gitLog(ctx context.Context, repoDir, afterHash string, maxN int) ([]Commit, error) {
	args := []string{"-C", repoDir, "log",
		"--format=%H|%ae|%s|%ai",
		"--name-only",
		fmt.Sprintf("-n%d", maxN),
	}
	if afterHash != "" {
		args = append(args, fmt.Sprintf("%s..HEAD", afterHash))
	}

	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	return parseGitLog(string(out)), nil
}

// parseGitLog parses the output of `git log --format=%H|%ae|%s|%ai --name-only`
func parseGitLog(raw string) []Commit {
	var commits []Commit
	var current *Commit

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Header line: hash|email|subject|date
		if parts := strings.SplitN(line, "|", 4); len(parts) == 4 {
			if current != nil {
				commits = append(commits, *current)
			}
			t, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[3])
			current = &Commit{
				Hash:    parts[0],
				Author:  parts[1],
				Message: parts[2],
				Date:    t,
			}
		} else if current != nil {
			// Changed file line
			current.Files = append(current.Files, line)
		}
	}
	if current != nil {
		commits = append(commits, *current)
	}
	return commits
}

// ─── Function extraction ────────────────────────────────────────────────────

type funcInfo struct {
	name string
	line int
}

// extractFunctions extracts function/method names from a file using language-specific patterns.
// Returns a deduplicated list of function names.
func extractFunctions(fullPath, relPath string) []funcInfo {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil
	}
	content := string(data)
	ext := strings.ToLower(filepath.Ext(relPath))

	switch ext {
	case ".go":
		return extractGoFunctions(content)
	case ".ts", ".tsx", ".js", ".jsx":
		return extractJSFunctions(content)
	case ".py":
		return extractPyFunctions(content)
	case ".java", ".kt":
		return extractJavaFunctions(content)
	case ".rs":
		return extractRustFunctions(content)
	default:
		return nil
	}
}

func extractGoFunctions(content string) []funcInfo {
	var funcs []funcInfo
	seen := map[string]bool{}
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// Match: func Name( or func (recv) Name(
		if !strings.HasPrefix(trimmed, "func ") {
			continue
		}
		// Extract name after "func " or "func (receiver) "
		rest := strings.TrimPrefix(trimmed, "func ")
		// Skip receiver: (r *Type) MethodName(
		if strings.HasPrefix(rest, "(") {
			if idx := strings.Index(rest, ")"); idx >= 0 {
				rest = strings.TrimSpace(rest[idx+1:])
			}
		}
		// Name is everything up to "("
		if idx := strings.Index(rest, "("); idx > 0 {
			name := rest[:idx]
			if !seen[name] {
				seen[name] = true
				funcs = append(funcs, funcInfo{name: name, line: i + 1})
			}
		}
	}
	return funcs
}

func extractJSFunctions(content string) []funcInfo {
	var funcs []funcInfo
	seen := map[string]bool{}
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		var name string
		// export function Foo(  or  function foo(
		if idx := strings.Index(trimmed, "function "); idx >= 0 {
			rest := trimmed[idx+9:]
			if end := strings.IndexAny(rest, "(<"); end > 0 {
				name = strings.TrimSpace(rest[:end])
			}
		}
		// const foo = ( or const foo = async (
		if strings.HasPrefix(trimmed, "const ") || strings.HasPrefix(trimmed, "export const ") {
			rest := strings.TrimPrefix(strings.TrimPrefix(trimmed, "export "), "const ")
			if idx := strings.Index(rest, " ="); idx > 0 {
				candidate := rest[:idx]
				if !strings.Contains(candidate, " ") {
					name = candidate
				}
			}
		}
		if name != "" && !seen[name] && isValidIdentifier(name) {
			seen[name] = true
			funcs = append(funcs, funcInfo{name: name, line: i + 1})
		}
	}
	return funcs
}

func extractPyFunctions(content string) []funcInfo {
	var funcs []funcInfo
	seen := map[string]bool{}
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "def ") && !strings.HasPrefix(trimmed, "async def ") {
			continue
		}
		rest := strings.TrimPrefix(strings.TrimPrefix(trimmed, "async "), "def ")
		if idx := strings.Index(rest, "("); idx > 0 {
			name := rest[:idx]
			if !seen[name] {
				seen[name] = true
				funcs = append(funcs, funcInfo{name: name, line: i + 1})
			}
		}
	}
	return funcs
}

func extractJavaFunctions(content string) []funcInfo {
	var funcs []funcInfo
	seen := map[string]bool{}
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// Look for: public/private/protected ... Type name(
		for _, mod := range []string{"public ", "private ", "protected "} {
			if !strings.Contains(trimmed, mod) {
				continue
			}
			// Find the method name before "("
			if idx := strings.Index(trimmed, "("); idx > 0 {
				before := trimmed[:idx]
				parts := strings.Fields(before)
				if len(parts) >= 2 {
					name := parts[len(parts)-1]
					if isValidIdentifier(name) && !seen[name] {
						seen[name] = true
						funcs = append(funcs, funcInfo{name: name, line: i + 1})
					}
				}
			}
		}
	}
	return funcs
}

func extractRustFunctions(content string) []funcInfo {
	var funcs []funcInfo
	seen := map[string]bool{}
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "fn ") {
			continue
		}
		for _, prefix := range []string{"pub fn ", "pub async fn ", "fn ", "async fn "} {
			if idx := strings.Index(trimmed, prefix); idx >= 0 {
				rest := trimmed[idx+len(prefix):]
				if end := strings.IndexAny(rest, "(<"); end > 0 {
					name := rest[:end]
					if isValidIdentifier(name) && !seen[name] {
						seen[name] = true
						funcs = append(funcs, funcInfo{name: name, line: i + 1})
					}
				}
				break
			}
		}
	}
	return funcs
}

// ─── Graph-manager writer ───────────────────────────────────────────────────

func (idx *Indexer) writeToGraph(ctx context.Context, namespace string, nodes []CodeNode, edges []CodeEdge) error {
	// Write nodes concurrently — up to 8 in-flight requests at a time
	eg, egCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, 8)
	for _, n := range nodes {
		n := n
		sem <- struct{}{}
		eg.Go(func() error {
			defer func() { <-sem }()
			// Convert CodeNode to the graphclient.Node format graph-manager expects
			gNode := map[string]interface{}{
				"id":          n.ID,
				"name":        n.Name,
				"type":        n.Type,
				"serviceId":   n.ServiceID,
				"statusClass": "2xx", // code nodes don't have runtime status
				"latencyP95":  0,
				"tags":        []string{n.Type},
			}
			if n.File != "" {
				gNode["file"] = n.File
			}
			if n.CommitHash != "" {
				gNode["commitHash"] = n.CommitHash
			}
			if n.Author != "" {
				gNode["author"] = n.Author
			}
			if n.Message != "" {
				gNode["message"] = n.Message
			}
			if n.Timestamp != "" {
				gNode["timestamp"] = n.Timestamp
			}
			body, _ := json.Marshal(gNode)
			nodeURL := fmt.Sprintf("%s/nodes?namespace=%s", idx.graphManagerURL, namespace)
			req, _ := http.NewRequestWithContext(egCtx, "POST", nodeURL, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := idx.httpClient.Do(req)
			if err != nil {
				log.Printf("code-indexer: write node %s: %v", n.ID, err)
				return nil // node failures are logged but non-fatal
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return nil
		})
	}
	eg.Wait() //nolint:errcheck — goroutines always return nil

	// Write edges concurrently — up to 8 in-flight requests at a time
	eg2, egCtx2 := errgroup.WithContext(ctx)
	sem2 := make(chan struct{}, 8)
	for _, e := range edges {
		e := e
		sem2 <- struct{}{}
		eg2.Go(func() error {
			defer func() { <-sem2 }()
			gEdge := map[string]interface{}{
				"id":           e.ID,
				"from":         e.From,
				"to":           e.To,
				"type":         e.Type,
				"confidence":   1.0,
				"successRatio": 1.0,
			}
			body, _ := json.Marshal(gEdge)
			edgeURL := fmt.Sprintf("%s/edges?namespace=%s", idx.graphManagerURL, namespace)
			req, _ := http.NewRequestWithContext(egCtx2, "POST", edgeURL, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := idx.httpClient.Do(req)
			if err != nil {
				log.Printf("code-indexer: write edge %s: %v", e.ID, err)
				return nil // edge failures are logged but non-fatal
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return nil
		})
	}
	eg2.Wait() //nolint:errcheck — goroutines always return nil
	return nil
}

// ─── State management (incremental indexing) ───────────────────────────────

func (idx *Indexer) getLastHash(ctx context.Context, projectID, serviceID string) string {
	var hash string
	err := idx.db.QueryRow(ctx,
		`SELECT last_commit_hash FROM code_index_state WHERE project_id=$1 AND service_id=$2`,
		projectID, serviceID).Scan(&hash)
	if err != nil {
		return ""
	}
	return hash
}

func (idx *Indexer) saveLastHash(ctx context.Context, projectID, serviceID, hash string) {
	_, _ = idx.db.Exec(ctx,
		`INSERT INTO code_index_state (project_id, service_id, last_commit_hash, last_indexed_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (project_id, service_id)
		 DO UPDATE SET last_commit_hash=$3, last_indexed_at=NOW()`,
		projectID, serviceID, hash)
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func isCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	codeExts := map[string]bool{
		".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".py": true, ".java": true, ".kt": true, ".rs": true, ".rb": true,
		".cs": true, ".cpp": true, ".c": true, ".h": true,
	}
	return codeExts[ext]
}

func isValidIdentifier(s string) bool {
	if len(s) == 0 || len(s) > 80 {
		return false
	}
	for i, c := range s {
		if i == 0 && !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_') {
			return false
		}
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

func sanitizeID(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteRune('_')
		}
	}
	r := b.String()
	if len(r) > 120 {
		r = r[:120]
	}
	return r
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── Schema ─────────────────────────────────────────────────────────────────

func ensureSchema(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS code_index_state (
			project_id       TEXT NOT NULL,
			service_id       TEXT NOT NULL,
			last_commit_hash TEXT NOT NULL,
			last_indexed_at  TIMESTAMPTZ DEFAULT NOW(),
			PRIMARY KEY (project_id, service_id)
		)`)
	return err
}

// ─── GitHub webhook helpers ──────────────────────────────────────────────────

// validateGitHubSignature validates the X-Hub-Signature-256 header from GitHub.
func validateGitHubSignature(secret, sig string, body []byte) bool {
	if !strings.HasPrefix(sig, "sha256=") {
		return false
	}
	sig = strings.TrimPrefix(sig, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

// runIndexByRepo loads the service map and indexes all services matching the given repo URL.
func runIndexByRepo(ctx context.Context, indexer *Indexer, serviceMapFile, repoURL string) {
	sm, err := loadServiceMap(serviceMapFile)
	if err != nil {
		log.Printf("code-indexer: runIndexByRepo: load service map: %v", err)
		return
	}
	normalizedURL := strings.TrimSuffix(repoURL, ".git")
	for _, project := range sm.Projects {
		var matching []ServiceEntry
		for _, svc := range project.Services {
			if strings.TrimSuffix(svc.Repo, ".git") == normalizedURL {
				matching = append(matching, svc)
			}
		}
		if len(matching) == 0 {
			continue
		}
		branch := matching[0].Branch
		if branch == "" {
			branch = "main"
		}
		if err := indexer.IndexRepoGroup(ctx, matching[0].Repo, branch, project, matching); err != nil {
			log.Printf("code-indexer: runIndexByRepo %s: %v", repoURL, err)
		}
	}
}

// ─── Main ────────────────────────────────────────────────────────────────────

func main() {
	ctx := context.Background()

	serviceMapFile := os.Getenv("SERVICE_MAP_FILE")
	if serviceMapFile == "" {
		serviceMapFile = "/etc/code-indexer/service-map.yaml"
	}
	pg := os.Getenv("PG_DSN")
	if pg == "" {
		pg = "postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable"
	}
	graphManagerURL := os.Getenv("GRAPH_MANAGER_URL")
	if graphManagerURL == "" {
		graphManagerURL = "http://graph-manager:8086/api/v1/graphs"
	}
	intervalStr := os.Getenv("INDEX_INTERVAL_MINUTES")
	interval := 60 * time.Minute // default: index every 60 minutes
	if intervalStr != "" {
		var mins int
		if _, err := fmt.Sscan(intervalStr, &mins); err == nil && mins > 0 {
			interval = time.Duration(mins) * time.Minute
		}
	}

	log.Printf("code-indexer: starting (map=%s interval=%s)", serviceMapFile, interval)

	db, err := pgxpool.New(ctx, pg)
	if err != nil {
		log.Fatalf("code-indexer: pg connect: %v", err)
	}
	defer db.Close()

	if err := ensureSchema(ctx, db); err != nil {
		log.Fatalf("code-indexer: ensure schema: %v", err)
	}

	indexer := NewIndexer(db, graphManagerURL)

	// HTTP server for on-demand trigger and webhook
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// POST /index — trigger immediate re-index (called by CI/CD webhook on push)
	mux.HandleFunc("/index", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		go runIndex(context.Background(), indexer, serviceMapFile)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"indexing started"}`))
	})
	// POST /index/service/{id} — index one specific service immediately
	mux.HandleFunc("/index/service/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		serviceID := strings.TrimPrefix(r.URL.Path, "/index/service/")
		go runIndexService(context.Background(), indexer, serviceMapFile, serviceID)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"indexing %s"}`, serviceID)))
	})
	// POST /webhook/github — receives GitHub push events and triggers re-index
	mux.HandleFunc("/webhook/github", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", 405)
			return
		}

		// Validate HMAC signature if secret is configured
		secret := os.Getenv("GITHUB_WEBHOOK_SECRET")
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		if secret != "" {
			sig := r.Header.Get("X-Hub-Signature-256")
			if !validateGitHubSignature(secret, sig, body) {
				http.Error(w, "invalid signature", 401)
				return
			}
		}

		// Parse push event to get repo URL and branch
		var event struct {
			Ref        string `json:"ref"` // "refs/heads/main"
			Repository struct {
				CloneURL string `json:"clone_url"` // "https://github.com/org/repo.git"
			} `json:"repository"`
		}
		json.NewDecoder(r.Body).Decode(&event)

		branch := strings.TrimPrefix(event.Ref, "refs/heads/")
		repoURL := strings.TrimSuffix(event.Repository.CloneURL, ".git")

		log.Printf("webhook: push to %s@%s — triggering re-index", repoURL, branch)
		go runIndexByRepo(context.Background(), indexer, serviceMapFile, repoURL)

		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"indexing triggered"}`))
	})

	port := os.Getenv("CODE_INDEXER_PORT")
	if port == "" {
		port = "8092"
	}
	go func() {
		log.Printf("code-indexer: HTTP trigger endpoint on :%s", port)
		log.Fatal(http.ListenAndServe(":"+port, mux))
	}()

	// Run immediately on startup, then on schedule
	runIndex(ctx, indexer, serviceMapFile)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runIndex(ctx, indexer, serviceMapFile)
		}
	}
}

// repoKey uniquely identifies a (repoURL, branch) pair for deduplication.
type repoKey struct{ url, branch string }

// repoEntry groups all services that share the same repository.
// For a monolith, multiple services share one repo but have different codePaths.
// We clone ONCE and index all services from the same clone, then delete.
type repoEntry struct {
	project  ProjectEntry
	services []ServiceEntry
}

func runIndex(ctx context.Context, indexer *Indexer, serviceMapFile string) {
	sm, err := loadServiceMap(serviceMapFile)
	if err != nil {
		log.Printf("code-indexer: load service map: %v", err)
		return
	}
	log.Printf("code-indexer: starting index run — %d projects", len(sm.Projects))
	start := time.Now()

	// Group services by (repoURL, branch) to avoid cloning the same repo multiple times.
	// Monolith pattern: 5 services all pointing to github.com/org/monolith → clone once.
	grouped := map[repoKey]*repoEntry{}
	for _, project := range sm.Projects {
		for _, svc := range project.Services {
			if svc.Repo == "" {
				continue
			}
			branch := svc.Branch
			if branch == "" {
				branch = "main"
			}
			k := repoKey{url: svc.Repo, branch: branch}
			if _, ok := grouped[k]; !ok {
				grouped[k] = &repoEntry{project: project}
			}
			grouped[k].services = append(grouped[k].services, svc)
		}
	}

	// Process each unique repo concurrently, up to maxConcurrent parallel clones.
	// This prevents hammering GitHub with 20 simultaneous clone requests.
	const maxConcurrent = 3
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for k, entry := range grouped {
		wg.Add(1)
		sem <- struct{}{}
		go func(k repoKey, e *repoEntry) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := indexer.IndexRepoGroup(ctx, k.url, k.branch, e.project, e.services); err != nil {
				log.Printf("code-indexer: error indexing repo %s: %v", k.url, err)
			}
		}(k, entry)
	}
	wg.Wait()
	log.Printf("code-indexer: run complete — %d unique repos, elapsed=%s",
		len(grouped), time.Since(start).Round(time.Millisecond))
}

func runIndexService(ctx context.Context, indexer *Indexer, serviceMapFile, serviceID string) {
	sm, err := loadServiceMap(serviceMapFile)
	if err != nil {
		return
	}
	// Build a flat O(1) index of service ID → (project, service) to avoid O(P×S) scan
	type svcProjectPair struct {
		project ProjectEntry
		svc     ServiceEntry
	}
	svcByID := make(map[string]svcProjectPair)
	for _, project := range sm.Projects {
		for _, svc := range project.Services {
			if svc.Repo != "" {
				svcByID[svc.ID] = svcProjectPair{project: project, svc: svc}
			}
		}
	}
	if pair, ok := svcByID[serviceID]; ok {
		if err := indexer.IndexService(ctx, pair.project, pair.svc); err != nil {
			log.Printf("code-indexer: error indexing %s: %v", serviceID, err)
		}
	}
}

func loadServiceMap(path string) (*ServiceMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var sm ServiceMap
	if err := yaml.Unmarshal(data, &sm); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &sm, nil
}
