-- 004_demo_seed.sql
--
-- Seeds the demo tenants and projects declared in scripts/service-map.yaml so a
-- fresh `docker compose up` has a working demo with no manual SQL required.
--
-- The graph itself (Neo4j nodes/edges) is built by the onboarding worker from
-- the same service map, so this only covers the relational side.
--
-- Every statement is idempotent: re-running is a no-op, and a real onboarding
-- run that creates its own rows is unaffected.

-- ── Tenants ────────────────────────────────────────────────────────────────
-- The Keycloak realm in scripts/keycloak/realm.json ships users for these two
-- tenants, so they must exist for login to resolve to a tenant.
INSERT INTO tenants (id, name, slug) VALUES
  ('acme-corp', 'ACME Corp',  'acme-corp'),
  ('zen-inc',   'Zen Inc',    'zen-inc')
ON CONFLICT (id) DO NOTHING;

-- ── Projects ───────────────────────────────────────────────────────────────
-- id/slug pairs match the namespace that scripts/service-map.yaml declares
-- (namespace is "<slug>:<environment>"), which is how the health poller,
-- detection engine and graph manager agree on where a graph lives.
INSERT INTO projects (id, tenant_id, name, slug, domain, environment) VALUES
  ('proj-payments-prod', 'acme-corp', 'Payments Platform', 'payments-platform', NULL, 'prod'),
  ('proj-orders-prod',   'acme-corp', 'Orders Platform',   'orders-platform',   NULL, 'prod')
ON CONFLICT (id) DO NOTHING;

-- Record this migration (run_migrations.sh also records it as a fallback).
INSERT INTO schema_migrations (version, applied_at)
VALUES ('004_demo_seed', NOW())
ON CONFLICT (version) DO NOTHING;
