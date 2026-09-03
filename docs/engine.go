package detection

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/your-org/faultiq/packages/types"
)

// =============================================================================
// Constants
// =============================================================================

const (
	defaultMaxDepth    = 6
	defaultCacheKeyFmt = "graph:%s:%s" // projectSlug:environment
	defaultCacheTTL    = 5 * time.Minute

	// Evidence weights — must sum to 1.0
	weightDirectSignal    = 0.40
	weightSharedImpact    = 0.25
	weightBranchIsolation = 0.20
	weightLatencyAnomaly  = 0.10
	weightRecentDeploy    = 0.05

	// Confidence thresholds
	thresholdHigh   = 0.80
	thresholdMedium = 0.50
)

// =============================================================================
// Interfaces — keep engine decoupled from infrastructure
// =============================================================================

// GraphStore loads subgraphs for a given namespace.
type GraphStore interface {
	LoadSubgraph(ctx context.Context, namespace string) (*types.Subgraph, error)
}

// GraphCache is a fast read-through cache (Redis) for hot subgraphs.
type GraphCache interface {
	Get(ctx context.Context, key string) (*types.Subgraph, error)
	Set(ctx context.Context, key string, g *types.Subgraph, ttl time.Duration) error
}

// SignalIndex provides O(1) lookup of a FaultSignal by service name.
type SignalIndex map[string]*types.FaultSignal

// DeploymentChecker checks whether a service had a recent deployment.
// Wire to your CI/CD event log or Kubernetes rollout history.
type DeploymentChecker interface {
	RecentDeploy(ctx context.Context, namespace, service string, within time.Duration) (bool, error)
}

// =============================================================================
// Engine
// =============================================================================

// Engine is the core BFS detection engine.
// It is stateless — all state lives in the Subgraph and SignalIndex.
// Safe to use concurrently across multiple incidents.
type Engine struct {
	graph    GraphStore
	cache    GraphCache
	deploys  DeploymentChecker
	maxDepth int
	logger   *slog.Logger
}

// NewEngine constructs an Engine with required dependencies.
func NewEngine(
	graph GraphStore,
	cache GraphCache,
	deploys DeploymentChecker,
	logger *slog.Logger,
) *Engine {
	return &Engine{
		graph:    graph,
		cache:    cache,
		deploys:  deploys,
		maxDepth: defaultMaxDepth,
		logger:   logger,
	}
}

// WithMaxDepth overrides the default BFS traversal depth.
func (e *Engine) WithMaxDepth(depth int) *Engine {
	e.maxDepth = depth
	return e
}

// =============================================================================
// Analyse — public entry point
// =============================================================================

// Analyse runs the full detection pipeline for a given set of signals.
// Returns a TraversalResult ready for the Root Cause Ranker.
func (e *Engine) Analyse(
	ctx context.Context,
	req *types.AnalyzeRequest,
	projectSlug string,
) (*types.TraversalResult, error) {

	namespace := fmt.Sprintf("%s:%s", projectSlug, req.Environment)

	// 1. Load subgraph (cache-first)
	g, err := e.loadSubgraph(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("load subgraph: %w", err)
	}

	// 2. Build signal index for O(1) lookup during traversal
	idx := buildSignalIndex(req.Signals)

	// 3. Run BFS traversal
	result := e.traverse(ctx, g, idx, req.EntryService, namespace)

	e.logger.Info("traversal complete",
		"namespace", namespace,
		"nodesVisited", result.NodesVisited,
		"pathsPruned", result.PathsPruned,
		"suspects", len(result.SuspectChain),
	)

	return result, nil
}

// =============================================================================
// Subgraph loading — cache-first
// =============================================================================

func (e *Engine) loadSubgraph(ctx context.Context, namespace string) (*types.Subgraph, error) {
	key := fmt.Sprintf(defaultCacheKeyFmt, namespace, "current")

	// Try cache first
	if e.cache != nil {
		if cached, err := e.cache.Get(ctx, key); err == nil && cached != nil {
			e.logger.Debug("subgraph cache hit", "namespace", namespace)
			return cached, nil
		}
	}

	// Fall through to graph store
	e.logger.Debug("subgraph cache miss — loading from Neo4j", "namespace", namespace)
	g, err := e.graph.LoadSubgraph(ctx, namespace)
	if err != nil {
		return nil, err
	}

	// Populate cache async — do not block the traversal
	if e.cache != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if setErr := e.cache.Set(cacheCtx, key, g, defaultCacheTTL); setErr != nil {
				e.logger.Warn("failed to cache subgraph", "err", setErr)
			}
		}()
	}

	return g, nil
}

// =============================================================================
// BFS traversal
// =============================================================================

// nodeState tracks per-node traversal state.
type nodeState struct {
	suspect types.SuspectLevel
	depth   int
	pruned  bool
}

func (e *Engine) traverse(
	ctx context.Context,
	g *types.Subgraph,
	idx SignalIndex,
	entryService string,
	namespace string,
) *types.TraversalResult {

	state := make(map[string]*nodeState, len(g.Nodes))
	visited := make(map[string]bool, len(g.Nodes))
	evaluations := make([]types.NodeEvaluation, 0, len(g.Nodes))

	// BFS queue holds (nodeID, depth) pairs
	type queueItem struct {
		nodeID string
		depth  int
	}
	queue := make([]queueItem, 0, len(g.Nodes))

	// Seed queue — start from entry node if provided, else all signalled nodes
	seeds := e.seedNodes(g, idx, entryService)
	for _, s := range seeds {
		queue = append(queue, queueItem{s, 0})
		visited[s] = true
		state[s] = &nodeState{suspect: types.SuspectLevelUnknown}
	}

	pathsPruned := 0
	maxDepthReached := 0

	for len(queue) > 0 {
		// Dequeue
		item := queue[0]
		queue = queue[1:]

		nodeID := item.nodeID
		depth := item.depth

		if depth > maxDepthReached {
			maxDepthReached = depth
		}

		node, exists := g.Nodes[nodeID]
		if !exists {
			continue
		}

		// Evaluate this node
		signal := idx[node.Name]
		eval, ns := e.evaluateNode(ctx, node, signal, namespace)

		state[nodeID] = &nodeState{suspect: ns, pruned: eval.Pruned}
		evaluations = append(evaluations, eval)

		if eval.Pruned {
			// Healthy branch — prune. Do not expand neighbors.
			pathsPruned++
			continue
		}

		// Depth limit check
		if depth >= e.maxDepth {
			continue
		}

		// Expand downstream neighbors
		for _, edge := range g.Edges[nodeID] {
			if !visited[edge.To] {
				visited[edge.To] = true
				queue = append(queue, queueItem{edge.To, depth + 1})
			}
		}

		// Expand upstream neighbors (reverse edges) for shared-impact scoring
		for _, edge := range g.Reverse[nodeID] {
			if !visited[edge.From] {
				visited[edge.From] = true
				queue = append(queue, queueItem{edge.From, depth + 1})
			}
		}
	}

	// Collect results
	var suspects, blastRadius, pruned []string
	for id, s := range state {
		node := g.Nodes[id]
		if node == nil {
			continue
		}
		switch s.suspect {
		case types.SuspectLevelHigh, types.SuspectLevelMedium, types.SuspectLevelLow:
			suspects = append(suspects, id)
		case types.SuspectLevelBlastRad:
			blastRadius = append(blastRadius, node.Name)
		case types.SuspectLevelHealthy:
			if s.pruned {
				pruned = append(pruned, node.Name)
			}
		}
	}

	return &types.TraversalResult{
		Namespace:      namespace,
		NodesVisited:   len(visited),
		PathsPruned:    pathsPruned,
		TraversalDepth: maxDepthReached,
		Evaluations:    evaluations,
		SuspectChain:   suspects,
		PrunedServices: pruned,
		BlastRadius:    blastRadius,
	}
}

// =============================================================================
// Node evaluation — maps signal to SuspectLevel
// =============================================================================

func (e *Engine) evaluateNode(
	ctx context.Context,
	node *types.ServiceNode,
	signal *types.FaultSignal,
	namespace string,
) (types.NodeEvaluation, types.SuspectLevel) {

	eval := types.NodeEvaluation{
		NodeID: node.ID,
		Name:   node.Name,
		Signal: signal,
	}

	// No signal for this node — treat as unknown, expand cautiously
	if signal == nil {
		eval.SuspectLevel = types.SuspectLevelLow
		return eval, types.SuspectLevelLow
	}

	switch signal.StatusClass {
	case types.StatusClass2xx:
		slaOK := node.SLAMs == 0 || signal.LatencyP95 <= node.SLAMs
		if slaOK {
			eval.SuspectLevel = types.SuspectLevelHealthy
			eval.Pruned = true
			return eval, types.SuspectLevelHealthy
		}
		// 2xx but latency breach
		eval.SuspectLevel = types.SuspectLevelLow
		return eval, types.SuspectLevelLow

	case types.StatusClass4xx:
		// Delegate to per-project classification — default is LOW
		level := e.classifyFourXX(signal)
		eval.SuspectLevel = level
		return eval, level

	case types.StatusClass5xx:
		eval.SuspectLevel = types.SuspectLevelHigh
		return eval, types.SuspectLevelHigh

	case types.StatusClassTimeout:
		eval.SuspectLevel = types.SuspectLevelHigh
		return eval, types.SuspectLevelHigh

	default:
		eval.SuspectLevel = types.SuspectLevelLow
		return eval, types.SuspectLevelLow
	}
}

// classifyFourXX applies configurable 4xx interpretation rules.
// In MVP this is a simple heuristic. Post-MVP: load per-project rules from DB.
func (e *Engine) classifyFourXX(signal *types.FaultSignal) types.SuspectLevel {
	// High error rate 4xx across many callers → likely auth or rate-limit cascade
	if signal.ErrorRate > 0.30 {
		return types.SuspectLevelMedium
	}
	// Low rate 4xx → client error, not platform fault
	return types.SuspectLevelHealthy
}

// =============================================================================
// Root cause ranking
// =============================================================================

// Rank scores all suspect nodes and returns them sorted by confidence descending.
// Called by the RCA layer after traversal completes.
func (e *Engine) Rank(
	ctx context.Context,
	result *types.TraversalResult,
	g *types.Subgraph,
	idx SignalIndex,
	namespace string,
) ([]types.RCACandidate, error) {

	candidates := make([]types.RCACandidate, 0, len(result.SuspectChain))

	// Count how many callers of each suspect node are also in suspect chain
	sharedImpact := e.computeSharedImpact(result.SuspectChain, g)

	// Count healthy neighbors per suspect node
	branchIsolation := e.computeBranchIsolation(result.SuspectChain, g, result.PrunedServices)

	for _, nodeID := range result.SuspectChain {
		node, ok := g.Nodes[nodeID]
		if !ok {
			continue
		}

		signal := idx[node.Name]
		if signal == nil {
			continue
		}

		// Compute evidence components
		ev := types.EvidenceBreakdown{}

		// Direct signal weight
		switch signal.StatusClass {
		case types.StatusClass5xx, types.StatusClassTimeout:
			ev.DirectSignal = 1.0
		case types.StatusClass4xx:
			ev.DirectSignal = 0.50
		default:
			ev.DirectSignal = 0.20
		}

		// Shared downstream impact — normalise by max possible callers
		if total := float64(len(g.Reverse[nodeID])); total > 0 {
			ev.SharedDownstreamImpact = float64(sharedImpact[nodeID]) / total
		}

		// Branch isolation — ratio of healthy neighbors to total neighbors
		ev.BranchIsolation = branchIsolation[nodeID]

		// Latency anomaly — p95 vs SLA threshold (simple ratio)
		if node.SLAMs > 0 && signal.LatencyP95 > node.SLAMs {
			ratio := float64(signal.LatencyP95) / float64(node.SLAMs)
			// Cap contribution at 1.0
			if ratio > 2.0 {
				ev.LatencyAnomaly = 1.0
			} else {
				ev.LatencyAnomaly = (ratio - 1.0)
			}
		}

		// Recent deployment flag
		if e.deploys != nil {
			if recent, _ := e.deploys.RecentDeploy(ctx, namespace, node.Name, 30*time.Minute); recent {
				ev.RecentDeployment = 1.0
			}
		}

		candidates = append(candidates, types.RCACandidate{
			NodeID:    nodeID,
			Name:      node.Name,
			Confidence: ev.CompositeScore(),
			FaultType: e.inferFaultType(signal),
			Evidence:  ev,
		})
	}

	// Sort by confidence descending, then name ascending for determinism
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		return candidates[i].Name < candidates[j].Name
	})

	// Assign ranks
	for i := range candidates {
		candidates[i].Rank = i + 1
	}

	return candidates, nil
}

// =============================================================================
// Scoring helpers
// =============================================================================

// computeSharedImpact counts how many upstream callers of each suspect node
// are also suspect — indicating the node is a shared point of failure.
func (e *Engine) computeSharedImpact(
	suspects []string,
	g *types.Subgraph,
) map[string]int {
	suspectSet := make(map[string]bool, len(suspects))
	for _, id := range suspects {
		suspectSet[id] = true
	}

	impact := make(map[string]int, len(suspects))
	for _, nodeID := range suspects {
		count := 0
		for _, edge := range g.Reverse[nodeID] {
			if suspectSet[edge.From] {
				count++
			}
		}
		impact[nodeID] = count
	}
	return impact
}

// computeBranchIsolation returns a 0–1 score per suspect node:
// 1.0 = all its neighbors are healthy (strongly isolated failure)
// 0.0 = all its neighbors are also suspect
func (e *Engine) computeBranchIsolation(
	suspects []string,
	g *types.Subgraph,
	prunedServices []string,
) map[string]float64 {
	suspectSet := make(map[string]bool, len(suspects))
	for _, id := range suspects {
		suspectSet[id] = true
	}

	prunedSet := make(map[string]bool, len(prunedServices))
	for _, name := range prunedServices {
		prunedSet[name] = true
	}

	isolation := make(map[string]float64, len(suspects))
	for _, nodeID := range suspects {
		neighbors := append(g.Edges[nodeID], g.Reverse[nodeID]...)
		if len(neighbors) == 0 {
			isolation[nodeID] = 0.5 // no neighbors — neutral score
			continue
		}
		healthy := 0
		for _, edge := range neighbors {
			neighborID := edge.To
			if edge.From != nodeID {
				neighborID = edge.From
			}
			neighbor, ok := g.Nodes[neighborID]
			if !ok {
				continue
			}
			if prunedSet[neighbor.Name] || !suspectSet[neighborID] {
				healthy++
			}
		}
		isolation[nodeID] = float64(healthy) / float64(len(neighbors))
	}
	return isolation
}

// =============================================================================
// Helpers
// =============================================================================

// seedNodes determines the BFS starting points.
// If entryService is specified, start there.
// Otherwise start from all nodes that have a signal.
func (e *Engine) seedNodes(
	g *types.Subgraph,
	idx SignalIndex,
	entryService string,
) []string {
	if entryService != "" {
		for id, node := range g.Nodes {
			if node.Name == entryService {
				return []string{id}
			}
		}
	}
	// Fall back to all signalled nodes
	seeds := make([]string, 0, len(idx))
	for id, node := range g.Nodes {
		if _, ok := idx[node.Name]; ok {
			seeds = append(seeds, id)
		}
	}
	return seeds
}

// inferFaultType maps the observed signal to a named fault pattern.
func (e *Engine) inferFaultType(signal *types.FaultSignal) types.FaultType {
	switch signal.StatusClass {
	case types.StatusClassTimeout:
		return types.FaultTypeTimeoutBurst
	case types.StatusClass5xx:
		return types.FaultTypeFiveXXPropagation
	case types.StatusClass4xx:
		return types.FaultTypeAuthFailure
	default:
		return types.FaultTypeLatencyAnomaly
	}
}

// buildSignalIndex converts a slice of FaultSignals to a name-keyed map.
func buildSignalIndex(signals []types.FaultSignal) SignalIndex {
	idx := make(SignalIndex, len(signals))
	for i := range signals {
		idx[signals[i].Service] = &signals[i]
	}
	return idx
}

// =============================================================================
// Parallel traversal (optional — for large graphs)
// =============================================================================

// TraverseParallel runs independent subgraph traversals concurrently.
// Use when one incident involves multiple disconnected graph components.
func (e *Engine) TraverseParallel(
	ctx context.Context,
	namespaces []string,
	req *types.AnalyzeRequest,
	projectSlug string,
) ([]*types.TraversalResult, error) {

	results := make([]*types.TraversalResult, len(namespaces))
	errs := make([]error, len(namespaces))
	var wg sync.WaitGroup

	for i, ns := range namespaces {
		wg.Add(1)
		go func(idx int, namespace string) {
			defer wg.Done()
			g, err := e.loadSubgraph(ctx, namespace)
			if err != nil {
				errs[idx] = err
				return
			}
			sigIdx := buildSignalIndex(req.Signals)
			results[idx] = e.traverse(ctx, g, sigIdx, req.EntryService, namespace)
		}(i, ns)
	}

	wg.Wait()

	// Collect errors
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}
