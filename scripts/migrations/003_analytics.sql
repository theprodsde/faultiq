-- ============================================================================
-- 003_analytics.sql — Operational analytics and code awareness
-- Signal history for SLO tracking, code indexer state, and team management.
-- These tables support observability, learning, and developer experience.
-- ============================================================================

-- ── Signal history ────────────────────────────────────────────────────────
-- Every health signal is recorded here.
-- Used for: SLO uptime computation, sparkline charts, error budget tracking.
-- Retention: 7 days (automated cleanup in detection-engine).

CREATE TABLE IF NOT EXISTS signal_history (
    id           BIGSERIAL PRIMARY KEY,
    tenant_id    TEXT,
    project_id   TEXT NOT NULL,
    service_id   TEXT NOT NULL,
    namespace    TEXT,
    status_class TEXT NOT NULL,
    error_rate   FLOAT DEFAULT 0,
    latency_p95  FLOAT DEFAULT 0,
    source       TEXT,                                  -- "health-poller" | "otel-receiver" | "manual"
    recorded_at  TIMESTAMPTZ DEFAULT NOW()
);

-- ── Code indexer state ────────────────────────────────────────────────────
-- Tracks the last git commit processed per (project, service).
-- Enables incremental indexing: only new commits are re-analyzed on each run.

CREATE TABLE IF NOT EXISTS code_index_state (
    project_id       TEXT NOT NULL,
    service_id       TEXT NOT NULL,
    last_commit_hash TEXT NOT NULL,
    last_indexed_at  TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (project_id, service_id)
);

-- ── Indexes ────────────────────────────────────────────────────────────────
-- Composite index covers the batch endpoint query pattern:
--   WHERE project_id=$1 AND service_id = ANY($2) AND recorded_at > ...

CREATE INDEX IF NOT EXISTS idx_signal_history_service
    ON signal_history(service_id, recorded_at DESC);

CREATE INDEX IF NOT EXISTS idx_signal_history_project
    ON signal_history(project_id, recorded_at DESC);

CREATE INDEX IF NOT EXISTS idx_signal_history_batch
    ON signal_history(project_id, service_id, recorded_at DESC);

CREATE INDEX IF NOT EXISTS idx_signal_history_faults
    ON signal_history(service_id, recorded_at DESC)
    WHERE status_class NOT IN ('2xx', 'degraded');

CREATE INDEX IF NOT EXISTS idx_code_index_project
    ON code_index_state(project_id);

INSERT INTO schema_migrations (version, applied_at) VALUES ('003_analytics', NOW())
    ON CONFLICT (version) DO NOTHING;
