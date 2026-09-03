-- ============================================================================
-- 001_core.sql — Platform foundation
-- Tenants, projects, environments, service topology, onboarding jobs.
-- These tables represent the structural configuration of a tenant's product.
-- ============================================================================

-- Migration tracking
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     TEXT PRIMARY KEY,
    applied_at  TIMESTAMPTZ DEFAULT NOW()
);

-- ── Multi-tenancy ──────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS tenants (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT UNIQUE NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    domain      TEXT,
    environment TEXT,
    metadata    JSONB,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS environments (
    id              TEXT PRIMARY KEY,
    project_id      TEXT REFERENCES projects(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    graph_namespace TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- ── Service topology (graph structure — Neo4j stores the runtime graph) ────

CREATE TABLE IF NOT EXISTS graph_versions (
    id              TEXT PRIMARY KEY,
    environment_id  TEXT REFERENCES environments(id) ON DELETE CASCADE,
    version_number  INT NOT NULL DEFAULT 1,
    status          TEXT,
    published_at    TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS service_nodes (
    id              TEXT PRIMARY KEY,
    environment_id  TEXT REFERENCES environments(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    type            TEXT,
    metadata        JSONB,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS dependency_edges (
    id           TEXT PRIMARY KEY,
    from_node_id TEXT REFERENCES service_nodes(id) ON DELETE CASCADE,
    to_node_id   TEXT REFERENCES service_nodes(id) ON DELETE CASCADE,
    edge_type    TEXT,
    confidence   DOUBLE PRECISION,
    last_observed TIMESTAMPTZ,
    metadata     JSONB
);

-- ── Onboarding ─────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS onboarding_jobs (
    id          TEXT PRIMARY KEY,
    project_id  TEXT,
    mode        TEXT,
    status      TEXT,
    payload     JSONB,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- ── Audit log ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS audit_events (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT,
    actor       TEXT,
    action      TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    metadata    JSONB
);

-- ── Backfill: ensure all columns exist on pre-existing tables ──────────────

ALTER TABLE projects ADD COLUMN IF NOT EXISTS metadata    JSONB;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS environment TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS domain      TEXT;

-- ── Indexes ────────────────────────────────────────────────────────────────

CREATE INDEX IF NOT EXISTS idx_projects_tenant      ON projects(tenant_id);
CREATE INDEX IF NOT EXISTS idx_environments_project ON environments(project_id);
CREATE INDEX IF NOT EXISTS idx_service_nodes_env    ON service_nodes(environment_id);

-- Mark this file as applied
INSERT INTO schema_migrations (version, applied_at) VALUES ('001_core', NOW())
    ON CONFLICT (version) DO NOTHING;
