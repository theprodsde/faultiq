-- ============================================================================
-- 002_incidents.sql — Incident tracking + SOP engine
-- Complete final state of every incident-related table.
-- Covers: detection, RCA scoring, SOP phases, playbooks, audit trail,
-- incident assignment, maintenance windows, and deployment correlation.
-- ============================================================================

-- ── Core incident tracking ─────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS incidents (
    id                    TEXT PRIMARY KEY,
    tenant_id             TEXT,
    project_id            TEXT,
    environment_id        TEXT,
    environment           TEXT DEFAULT 'prod',
    service               TEXT,
    status                TEXT DEFAULT 'OPEN',          -- OPEN | ACKNOWLEDGED | RESOLVED
    phase                 TEXT DEFAULT 'DETECTING',     -- SOP phase state machine
    detected_at           TIMESTAMPTZ DEFAULT NOW(),
    updated_at            TIMESTAMPTZ DEFAULT NOW(),
    resolved_at           TIMESTAMPTZ,                  -- set exactly once at resolution
    evidence              JSONB DEFAULT '[]',           -- array of signal snapshots
    root_cause_candidate  TEXT,
    confidence            FLOAT DEFAULT 0.0,
    expected_signal       TEXT,                         -- condition for VERIFYING auto-resolve
    verifying_step_order  INT,                          -- which SOP step is in VERIFYING
    assigned_to           TEXT,                         -- operator email or user ID
    assigned_at           TIMESTAMPTZ,
    metadata              JSONB
);

CREATE TABLE IF NOT EXISTS rca_candidates (
    id                TEXT PRIMARY KEY,
    incident_id       TEXT REFERENCES incidents(id) ON DELETE CASCADE,
    candidate_node_id TEXT,
    confidence_score  FLOAT,
    evidence_count    INT DEFAULT 1,                    -- increments with each new signal
    evidence          JSONB DEFAULT '{}',
    operator_confirmed BOOLEAN DEFAULT FALSE,
    created_at        TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS recommendations (
    id             TEXT PRIMARY KEY,
    incident_id    TEXT REFERENCES incidents(id) ON DELETE CASCADE,
    rank           INT,
    playbook_title TEXT,
    steps          JSONB,                               -- human-readable steps
    sop_steps      JSONB DEFAULT '[]',                  -- structured SOPStep[] with auto-verify
    created_at     TIMESTAMPTZ DEFAULT NOW()
);

-- ── SOP audit trail ────────────────────────────────────────────────────────
-- Records every phase transition, step completion, and auto-resolution.
-- Used for operator accountability, MTTR analysis, and retrospectives.

CREATE TABLE IF NOT EXISTS sop_audit (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    incident_id     TEXT NOT NULL,
    step_order      INT,
    action          TEXT NOT NULL,                      -- phase_change | step_done | step_failed | auto_resolved | sla_escalation
    actor           TEXT DEFAULT 'system',              -- "system" or operator email/UUID
    phase_from      TEXT,
    phase_to        TEXT,
    signal_snapshot JSONB DEFAULT '{}',                 -- signal values at time of action
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- ── Custom SOP playbook versioning ────────────────────────────────────────
-- Stores versioned history of custom playbooks per service.
-- Each PUT /sop-playbook saves the previous version before overwriting.

CREATE TABLE IF NOT EXISTS runbook_versions (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    project_id  TEXT NOT NULL,
    service_id  TEXT NOT NULL,
    version     INT NOT NULL DEFAULT 1,
    steps       JSONB NOT NULL,
    fault_type  TEXT,
    created_by  TEXT DEFAULT 'system',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_runbook_version_unique
    ON runbook_versions(project_id, service_id, version);

-- ── Playbook learning ─────────────────────────────────────────────────────
-- Tracks which SOP steps resolve incidents of each fault type.
-- Used to reorder playbook steps by historical success rate.

CREATE TABLE IF NOT EXISTS playbook_learning (
    service_id    TEXT,
    fault_type    TEXT,
    step_order    INT,
    step_title    TEXT,
    success_count INT DEFAULT 0,
    total_count   INT DEFAULT 0,
    last_updated  TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (service_id, fault_type, step_order)
);

-- ── Maintenance windows ───────────────────────────────────────────────────
-- Suppresses signals for a service or whole project during planned maintenance.
-- Detection engine checks this before creating incidents.

CREATE TABLE IF NOT EXISTS maintenance_windows (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    project_id  TEXT NOT NULL,
    service_id  TEXT,                                   -- NULL = whole project
    reason      TEXT,
    created_by  TEXT DEFAULT 'operator',
    starts_at   TIMESTAMPTZ DEFAULT NOW(),
    ends_at     TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- ── Deployments ───────────────────────────────────────────────────────────
-- CI/CD calls POST /api/v1/deployments after each deploy.
-- 5-minute health gate monitors the service after deploy.
-- Correlated with incidents: fault within 30 min of deploy = likely cause.

CREATE TABLE IF NOT EXISTS deployments (
    id                TEXT PRIMARY KEY,
    tenant_id         TEXT,
    project_id        TEXT NOT NULL,
    service_id        TEXT NOT NULL,
    service_name      TEXT,
    version           TEXT,
    commit_hash       TEXT,
    deployed_by       TEXT,
    environment       TEXT DEFAULT 'prod',
    status            TEXT DEFAULT 'deploying',         -- deploying | healthy | degraded | rolled_back
    deployed_at       TIMESTAMPTZ DEFAULT NOW(),
    health_gate_until TIMESTAMPTZ,                      -- watch window: deployed_at + 5 min
    health_checked_at TIMESTAMPTZ,
    health_status     TEXT,
    notes             TEXT
);

-- ── Backfill: ensure all columns exist on pre-existing incidents table ─────
-- These ALTER TABLE statements are safe on both fresh and upgraded databases.
-- They add columns that may have been missing from an older schema.

ALTER TABLE incidents ADD COLUMN IF NOT EXISTS environment           TEXT DEFAULT 'prod';
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS environment_id        TEXT;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS status                TEXT DEFAULT 'OPEN';
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS phase                 TEXT DEFAULT 'DETECTING';
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS updated_at            TIMESTAMPTZ DEFAULT NOW();
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS resolved_at           TIMESTAMPTZ;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS evidence              JSONB DEFAULT '[]';
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS root_cause_candidate  TEXT;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS confidence            FLOAT DEFAULT 0.0;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS expected_signal       TEXT;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS verifying_step_order  INT;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS assigned_to           TEXT;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS assigned_at           TIMESTAMPTZ;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS metadata              JSONB;
ALTER TABLE rca_candidates ADD COLUMN IF NOT EXISTS candidate_node_id  TEXT;
ALTER TABLE rca_candidates ADD COLUMN IF NOT EXISTS confidence_score   FLOAT;
ALTER TABLE rca_candidates ADD COLUMN IF NOT EXISTS evidence_count     INT DEFAULT 1;
ALTER TABLE rca_candidates ADD COLUMN IF NOT EXISTS evidence           JSONB DEFAULT '{}';
ALTER TABLE rca_candidates ADD COLUMN IF NOT EXISTS operator_confirmed BOOLEAN DEFAULT FALSE;
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS sop_steps JSONB DEFAULT '[]';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS health_gate_until   TIMESTAMPTZ;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS health_checked_at   TIMESTAMPTZ;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS health_status       TEXT;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS notes               TEXT;

-- ── Indexes ────────────────────────────────────────────────────────────────

CREATE INDEX IF NOT EXISTS idx_incidents_tenant           ON incidents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_incidents_project          ON incidents(project_id);
CREATE INDEX IF NOT EXISTS idx_incidents_service          ON incidents(service);
CREATE INDEX IF NOT EXISTS idx_incidents_phase            ON incidents(phase)        WHERE status = 'OPEN';
CREATE INDEX IF NOT EXISTS idx_incidents_open             ON incidents(detected_at DESC) WHERE status = 'OPEN';
CREATE INDEX IF NOT EXISTS idx_incidents_assignee         ON incidents(assigned_to)  WHERE assigned_to IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_incidents_status_detected  ON incidents(status, detected_at DESC);
CREATE INDEX IF NOT EXISTS idx_sop_audit_incident         ON sop_audit(incident_id);
CREATE INDEX IF NOT EXISTS idx_sop_audit_created          ON sop_audit(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_maintenance_active         ON maintenance_windows(project_id, ends_at);
CREATE INDEX IF NOT EXISTS idx_deployments_service        ON deployments(service_id, deployed_at DESC);
CREATE INDEX IF NOT EXISTS idx_deployments_project        ON deployments(project_id, deployed_at DESC);
CREATE INDEX IF NOT EXISTS idx_deployments_active         ON deployments(deployed_at DESC) WHERE status IN ('deploying','healthy');

-- ── Operator RCA feedback ─────────────────────────────────────────────────
-- Operators confirm or deny the system's root cause prediction.
-- Used to calibrate per-service confidence thresholds over time.

CREATE TABLE IF NOT EXISTS rca_feedback (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    incident_id     TEXT NOT NULL,
    project_id      TEXT,
    predicted_root  TEXT NOT NULL,   -- what system predicted
    actual_root     TEXT,            -- what operator says was real root (NULL if prediction was correct)
    was_correct     BOOLEAN NOT NULL,
    operator_note   TEXT,
    created_by      TEXT DEFAULT 'operator',
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rca_feedback_incident ON rca_feedback(incident_id);
CREATE INDEX IF NOT EXISTS idx_rca_feedback_project  ON rca_feedback(project_id);

-- Missing FK indexes (Postgres does not auto-index foreign keys)
CREATE INDEX IF NOT EXISTS idx_rca_candidates_incident    ON rca_candidates(incident_id);
CREATE INDEX IF NOT EXISTS idx_rca_candidates_node        ON rca_candidates(candidate_node_id);
CREATE INDEX IF NOT EXISTS idx_recommendations_incident   ON recommendations(incident_id);

-- JSONB expression index for project list filtering
CREATE INDEX IF NOT EXISTS idx_projects_graph_status      ON projects((metadata->>'graphStatus'));

-- Partial composite index for the high-frequency VERIFYING-phase lookup
CREATE INDEX IF NOT EXISTS idx_incidents_verifying
    ON incidents(service, detected_at DESC)
    WHERE phase = 'VERIFYING' AND status IN ('OPEN', 'ACKNOWLEDGED');

INSERT INTO schema_migrations (version, applied_at) VALUES ('002_incidents', NOW())
    ON CONFLICT (version) DO NOTHING;
