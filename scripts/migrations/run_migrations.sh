#!/bin/sh
set -euo pipefail

echo "db-migrate: waiting for Postgres..."
PG_HOST=${PG_HOST:-postgres}
PG_PORT=${PG_PORT:-5432}
until pg_isready -h "$PG_HOST" -p "$PG_PORT" >/dev/null 2>&1; do
  echo "waiting for postgres at $PG_HOST:$PG_PORT..."
  sleep 1
done

if [ -z "${PG_DSN:-}" ]; then
  PG_DSN="postgres://postgres:postgres@postgres:5432/postgres?sslmode=disable"
fi

echo "db-migrate: ensuring schema_migrations table"
psql "$PG_DSN" -v ON_ERROR_STOP=1 -c "
  CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ DEFAULT NOW()
  );
"

echo "db-migrate: applying migrations from /migrations/"
for f in $(ls /migrations/*.sql 2>/dev/null | sort); do
  # Use the basename (without extension) as the version key for tracking.
  # This means renaming a file is treated as a new migration — intentional.
  ver=$(basename "$f" .sql)
  applied=$(psql "$PG_DSN" -tAc "SELECT 1 FROM schema_migrations WHERE version = '$ver' LIMIT 1;") || true
  if [ "x$applied" = "x1" ]; then
    echo "  skipping $ver (already applied)"
    continue
  fi
  echo "  applying $ver..."
  psql "$PG_DSN" -v ON_ERROR_STOP=1 -f "$f"
  # Record the version (each SQL file inserts its own row, but also record here as fallback)
  psql "$PG_DSN" -v ON_ERROR_STOP=1 -c "
    INSERT INTO schema_migrations (version, applied_at)
    VALUES ('$ver', NOW())
    ON CONFLICT (version) DO NOTHING;
  "
  echo "  applied $ver"
done

echo "db-migrate: completed ($(ls /migrations/*.sql 2>/dev/null | wc -l) migration files)"
