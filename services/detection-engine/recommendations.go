package main

import "sort"

// SOPStep represents one ordered step in a Sequenced Fault Automation playbook.
type SOPStep struct {
    Order          int    `json:"order"`
    Title          string `json:"title"`
    Action         string `json:"action"`          // what the operator does
    ExpectedSignal string `json:"expectedSignal"`  // "" means no auto-verify
    AutoVerify     bool   `json:"autoVerify"`      // system watches for signal
    Status         string `json:"status"`          // PENDING / ACTIVE / DONE / FAILED
}

// GenerateSOPPlaybook returns an ordered sequence of SOPSteps for a given fault type.
// Step 1 is always ACTIVE; the rest are PENDING. AutoVerify=true steps are resolved
// automatically by the detection engine when the expected signal arrives.
func GenerateSOPPlaybook(faultType FaultType, serviceName string) []SOPStep {
    switch faultType {
    case FaultServiceDown:
        return []SOPStep{
            {Order: 1, Title: "Verify service is down", Action: "Check container/pod status for " + serviceName + ". Confirm it is not running.", ExpectedSignal: "", AutoVerify: false, Status: "ACTIVE"},
            {Order: 2, Title: "Check resource limits", Action: "Review CPU, memory, and disk utilization before the failure. Look for OOMKilled or CrashLoopBackOff.", ExpectedSignal: "", AutoVerify: false, Status: "PENDING"},
            {Order: 3, Title: "Restart service", Action: "Restart " + serviceName + " via your orchestrator (kubectl rollout restart / ECS force-deploy).", ExpectedSignal: "2xx", AutoVerify: true, Status: "PENDING"},
            {Order: 4, Title: "Confirm traffic normalizes", Action: "Watch error rate for 2 minutes post-restart.", ExpectedSignal: "errorRate<0.01", AutoVerify: true, Status: "PENDING"},
        }
    case FaultTimeout:
        return []SOPStep{
            {Order: 1, Title: "Identify bottleneck", Action: "Check connection pool utilization and thread pool saturation for " + serviceName + ".", ExpectedSignal: "", AutoVerify: false, Status: "ACTIVE"},
            {Order: 2, Title: "Inspect downstream dependencies", Action: "Identify slow downstream calls via distributed traces. Look for database lock contention.", ExpectedSignal: "", AutoVerify: false, Status: "PENDING"},
            {Order: 3, Title: "Enable circuit breaker", Action: "Configure circuit breaker with 50% failure rate threshold and fallback behavior.", ExpectedSignal: "", AutoVerify: false, Status: "PENDING"},
            {Order: 4, Title: "Scale upstream service", Action: "Increase instance count or connection pool size for " + serviceName + ".", ExpectedSignal: "latency<500ms", AutoVerify: true, Status: "PENDING"},
            {Order: 5, Title: "Confirm latency normalized", Action: "Verify P95 latency returns below SLO threshold.", ExpectedSignal: "2xx", AutoVerify: true, Status: "PENDING"},
        }
    case FaultHighErrorRate:
        return []SOPStep{
            {Order: 1, Title: "Check recent deployments", Action: "List deployments for " + serviceName + " in the last 30 minutes. Correlate deploy timestamp with error spike.", ExpectedSignal: "", AutoVerify: false, Status: "ACTIVE"},
            {Order: 2, Title: "Review application logs", Action: "Scan logs for new exception types, panic traces, or configuration errors.", ExpectedSignal: "", AutoVerify: false, Status: "PENDING"},
            {Order: 3, Title: "Roll back deployment", Action: "If a recent deploy correlates, trigger rollback to last known good version.", ExpectedSignal: "2xx", AutoVerify: true, Status: "PENDING"},
            {Order: 4, Title: "Verify external dependencies", Action: "Check database connection health, cache hit rates, and external API status pages.", ExpectedSignal: "", AutoVerify: false, Status: "PENDING"},
            {Order: 5, Title: "Confirm error rate normalized", Action: "Watch error rate for 3 minutes after rollback/fix.", ExpectedSignal: "errorRate<0.01", AutoVerify: true, Status: "PENDING"},
        }
    case FaultLatencyAnomaly:
        return []SOPStep{
            {Order: 1, Title: "Check resource saturation", Action: "Review CPU (>80% sustained), GC pause times, and network I/O for " + serviceName + ".", ExpectedSignal: "", AutoVerify: false, Status: "ACTIVE"},
            {Order: 2, Title: "Identify slow endpoints", Action: "Check if latency is across all endpoints or specific paths. Use request-level traces.", ExpectedSignal: "", AutoVerify: false, Status: "PENDING"},
            {Order: 3, Title: "Scale horizontally", Action: "Add instances to distribute load if CPU saturation is the root cause.", ExpectedSignal: "latency<200ms", AutoVerify: true, Status: "PENDING"},
            {Order: 4, Title: "Confirm latency normalized", Action: "Verify P95 latency returns below SLO.", ExpectedSignal: "2xx", AutoVerify: true, Status: "PENDING"},
        }
    case FaultCascading:
        return []SOPStep{
            {Order: 1, Title: "Identify root dependency", Action: "Follow the dependency graph downstream from " + serviceName + ". Find the deepest failing node.", ExpectedSignal: "", AutoVerify: false, Status: "ACTIVE"},
            {Order: 2, Title: "Isolate root service", Action: "Apply fix to the identified root dependency first — do not attempt to fix " + serviceName + " directly.", ExpectedSignal: "", AutoVerify: false, Status: "PENDING"},
            {Order: 3, Title: "Implement bulkhead", Action: "Add timeout + circuit breaker on " + serviceName + "'s calls to the failing dependency to limit blast radius.", ExpectedSignal: "", AutoVerify: false, Status: "PENDING"},
            {Order: 4, Title: "Confirm cascade cleared", Action: "Watch upstream services recover as root dependency stabilizes.", ExpectedSignal: "2xx", AutoVerify: true, Status: "PENDING"},
        }
    case FaultConnectionError:
        return []SOPStep{
            {Order: 1, Title: "Check network connectivity", Action: "Verify DNS resolution, security groups/firewall rules, and service mesh config for " + serviceName + ".", ExpectedSignal: "", AutoVerify: false, Status: "ACTIVE"},
            {Order: 2, Title: "Test direct connectivity", Action: "Run a connectivity test from " + serviceName + " to its dependency (curl, telnet, nc).", ExpectedSignal: "", AutoVerify: false, Status: "PENDING"},
            {Order: 3, Title: "Restore connectivity", Action: "Fix the identified network/config issue. Re-deploy if config change required.", ExpectedSignal: "2xx", AutoVerify: true, Status: "PENDING"},
        }
    default: // FaultTransient or unknown
        return []SOPStep{
            {Order: 1, Title: "Monitor — likely transient", Action: "Error rate is low. Set an alert for >50% error rate in the next 5 minutes.", ExpectedSignal: "", AutoVerify: false, Status: "ACTIVE"},
            {Order: 2, Title: "Check retry patterns", Action: "Verify retries are succeeding in logs. Look for retry-after headers in downstream responses.", ExpectedSignal: "", AutoVerify: false, Status: "PENDING"},
            {Order: 3, Title: "Confirm self-healing", Action: "If error rate stays below 5% for 10 minutes, mark resolved.", ExpectedSignal: "errorRate<0.05", AutoVerify: true, Status: "PENDING"},
        }
    }
}

// StepLearning holds historical outcome data for a single playbook step.
type StepLearning struct {
	StepOrder   int
	SuccessRate float64
	TotalCount  int
}

// GenerateSOPPlaybookWithLearning generates steps and reorders based on historical success rates.
// Steps with higher success rates AND sufficient sample size (> 5) are moved up.
// Steps with low sample size keep their original position.
func GenerateSOPPlaybookWithLearning(faultType FaultType, serviceName string, learning []StepLearning) []SOPStep {
	steps := GenerateSOPPlaybook(faultType, serviceName)
	if len(learning) == 0 {
		return steps
	}

	// Build success rate map — only include steps with sufficient sample size
	successByOrder := make(map[int]float64)
	for _, l := range learning {
		if l.TotalCount >= 5 {
			successByOrder[l.StepOrder] = l.SuccessRate
		}
	}
	if len(successByOrder) == 0 {
		return steps // not enough data to reorder
	}

	// Precompute scores to avoid repeated map lookups during sort
	type indexedStep struct {
		step  SOPStep
		score float64
	}
	scored := make([]indexedStep, len(steps))
	for i, s := range steps {
		v, ok := successByOrder[s.Order]
		if ok {
			scored[i] = indexedStep{s, v}
		} else {
			scored[i] = indexedStep{s, -1}
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		si, sj := scored[i].score, scored[j].score
		if si < 0 && sj < 0 {
			return false
		}
		if si < 0 {
			return false
		}
		if sj < 0 {
			return true
		}
		return si > sj
	})
	result := make([]SOPStep, len(scored))
	for i, s := range scored {
		result[i] = s.step
	}

	// Re-assign order numbers after reordering and set first step as ACTIVE
	for i := range result {
		result[i].Order = i + 1
		if result[i].Status == "" {
			result[i].Status = "PENDING"
		}
	}
	if len(result) > 0 {
		result[0].Status = "ACTIVE" // first step is always active
	}
	return result
}

// FaultType classifies the kind of failure detected.
type FaultType string

const (
	FaultHighErrorRate    FaultType = "high-error-rate"
	FaultTimeout         FaultType = "timeout-burst"
	FaultConnectionError FaultType = "connection-refused"
	FaultServiceDown     FaultType = "service-down"
	FaultLatencyAnomaly  FaultType = "latency-anomaly"
	FaultCascading       FaultType = "cascading-failure"
	FaultTransient       FaultType = "transient-error"
)

// Recommendation represents a suggested fix for a detected fault.
type Recommendation struct {
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Priority string   `json:"priority"` // critical, high, medium, low
	Steps    []string `json:"steps"`
	Reason   string   `json:"reason"`
}

// ClassifyFault determines the fault type from signal characteristics.
func ClassifyFault(statusClass string, errorRate float64, latencyP95 int) FaultType {
	switch {
	case statusClass == "down" || statusClass == "connection_error":
		return FaultServiceDown
	case statusClass == "timeout" || statusClass == "504":
		if errorRate > 0.8 {
			return FaultTimeout
		}
		return FaultLatencyAnomaly
	case len(statusClass) > 0 && statusClass[0] == '5':
		if errorRate > 0.5 {
			return FaultHighErrorRate
		}
		return FaultCascading
	case errorRate > 0.1 && errorRate <= 0.3:
		return FaultTransient
	default:
		return FaultLatencyAnomaly
	}
}

// GenerateRecommendations produces fix suggestions based on fault classification.
// These are returned immediately with the incident for near-zero-latency advice.
func GenerateRecommendations(faultType FaultType, serviceName string, nodeType string) []Recommendation {
	switch faultType {
	case FaultServiceDown:
		return []Recommendation{
			{
				Title:    "Verify service health and restart if needed",
				Category: "Infrastructure",
				Priority: "critical",
				Steps: []string{
					"Check if the container/pod for " + serviceName + " is running",
					"Review container logs for OOMKilled, CrashLoopBackOff, or exit codes",
					"Verify network reachability (DNS, security groups, service mesh)",
					"If crashed, trigger a restart via orchestrator (kubectl rollout restart, ECS force-deploy)",
					"Check if the failure is caused by a dependency being unavailable",
				},
				Reason: "Service is unreachable. This is typically caused by a crash, resource exhaustion, or network partition.",
			},
			{
				Title:    "Check resource limits and scaling",
				Category: "Capacity",
				Priority: "high",
				Steps: []string{
					"Review CPU and memory utilization before the failure",
					"Check if autoscaler limits have been reached",
					"Verify no disk space exhaustion on the node",
					"Consider temporarily increasing resource limits",
				},
				Reason: "Resource exhaustion is a common cause of service unavailability.",
			},
		}

	case FaultTimeout:
		return []Recommendation{
			{
				Title:    "Identify and resolve downstream bottleneck",
				Category: "Performance",
				Priority: "critical",
				Steps: []string{
					"Check connection pool utilization for " + serviceName,
					"Identify slow downstream calls using distributed traces",
					"Review database query performance (slow queries, lock contention)",
					"Check thread pool saturation and async queue depth",
					"Consider increasing timeout thresholds or enabling circuit breakers",
				},
				Reason: "Timeouts indicate " + serviceName + " is waiting on a blocked resource or saturated downstream service.",
			},
			{
				Title:    "Enable circuit breaker pattern",
				Category: "Resilience",
				Priority: "high",
				Steps: []string{
					"Configure circuit breaker with appropriate threshold (e.g., 50% failure rate over 10s)",
					"Set fallback behavior (cached response, default value, or graceful degradation)",
					"Monitor circuit breaker state transitions",
				},
				Reason: "Circuit breakers prevent cascade failures by fast-failing when a dependency is unhealthy.",
			},
		}

	case FaultHighErrorRate:
		return []Recommendation{
			{
				Title:    "Identify and rollback bad deployment",
				Category: "Deployment",
				Priority: "critical",
				Steps: []string{
					"Check if a deployment occurred in the last 30 minutes for " + serviceName,
					"Compare error rates before and after the deployment",
					"If correlated, initiate rollback to last known good version",
					"Review application logs for new exception patterns or panic traces",
					"Verify configuration changes (env vars, feature flags) made recently",
				},
				Reason: "High error rate (>50%) strongly correlates with a bad deployment or configuration change.",
			},
			{
				Title:    "Check external dependencies",
				Category: "Dependencies",
				Priority: "high",
				Steps: []string{
					"Verify database connection health and connection pool stats",
					"Check external API partner status pages",
					"Review cache hit rates — a cold cache can cause cascading failures",
					"Verify message queue connectivity and consumer lag",
				},
				Reason: "If no deployment correlates, the root cause is likely a dependency failure propagating upstream.",
			},
		}

	case FaultLatencyAnomaly:
		return []Recommendation{
			{
				Title:    "Investigate resource saturation",
				Category: "Performance",
				Priority: "high",
				Steps: []string{
					"Check CPU utilization — sustained >80% causes latency spikes",
					"Review garbage collection metrics (GC pause times)",
					"Check network I/O and bandwidth utilization",
					"Identify if latency is across all endpoints or specific paths",
					"Review if this correlates with traffic spikes (check request rate)",
				},
				Reason: "Latency anomalies without errors typically indicate resource contention or increased load.",
			},
		}

	case FaultCascading:
		return []Recommendation{
			{
				Title:    "Isolate the root cause in the dependency chain",
				Category: "Root Cause Analysis",
				Priority: "critical",
				Steps: []string{
					"Follow the dependency graph downstream from " + serviceName,
					"Identify the deepest failing node — that's likely the true root cause",
					"Check if the failure is propagating because of missing retry/circuit-breaker patterns",
					"Consider implementing bulkhead isolation to contain the blast radius",
				},
				Reason: "This service is failing due to a downstream dependency failure. Fixing the root node will resolve this.",
			},
		}

	case FaultTransient:
		return []Recommendation{
			{
				Title:    "Monitor — likely self-healing",
				Category: "Observability",
				Priority: "low",
				Steps: []string{
					"Error rate is low (<30%) — this may resolve without intervention",
					"Set up an alert if error rate exceeds 50% within the next 5 minutes",
					"Check if retries are succeeding (look for retry-after patterns in logs)",
					"Verify this isn't caused by a specific client or endpoint",
				},
				Reason: "Low error rates often indicate transient network issues or brief resource contention that self-resolves.",
			},
		}

	default:
		return []Recommendation{
			{
				Title:    "Investigate anomaly in " + serviceName,
				Category: "General",
				Priority: "medium",
				Steps: []string{
					"Review application metrics and logs for " + serviceName,
					"Check recent changes (deployments, config updates, scaling events)",
					"Correlate with upstream and downstream service health",
				},
				Reason: "An anomaly was detected but doesn't match a specific pattern. Manual investigation recommended.",
			},
		}
	}
}
