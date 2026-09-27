#!/bin/sh
# Applies every scripts/migrations/*.sql file in filename order, tracking what
# has run in schema_migrations so re-running is a no-op.
#
# This is deliberately POSIX sh, not bash: docker compose runs it via
# /bin/sh (BusyBox ash) and CI runs it via dash on Ubuntu. `set -o pipefail` is
# a bashism that dash rejects outright, so the file must not rely on it.
set -eu

MIGRATIONS_DIR=${MIGRATIONS_DIR:-/migrations}

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

# Collect migration files by glob rather than `ls | sort`, so an empty or
# missing directory degrades cleanly instead of depending on pipeline status.
count=0
applied_count=0
for f in "$MIGRATIONS_DIR"/*.sql; do
  [ -e "$f" ] || continue
  count=$((count + 1))

  # Use the basename (without extension) as the version key for tracking.
  # This means renaming a file is treated as a new migration — intentional.
  ver=$(basename "$f" .sql)
  already=$(psql "$PG_DSN" -tAc "SELECT 1 FROM schema_migrations WHERE version = '$ver' LIMIT 1;")
  if [ "x$already" = "x1" ]; then
    echo "  skipping $ver (already applied)"
    continue
  fi

  echo "  applying $ver..."
  psql "$PG_DSN" -v ON_ERROR_STOP=1 -f "$f"
  # Record the version (each SQL file inserts its own row, but also record here
  # as a fallback so a migration without that INSERT is still tracked).
  psql "$PG_DSN" -v ON_ERROR_STOP=1 -c "
    INSERT INTO schema_migrations (version, applied_at)
    VALUES ('$ver', NOW())
    ON CONFLICT (version) DO NOTHING;
  "
  echo "  applied $ver"
  applied_count=$((applied_count + 1))
done

echo "db-migrate: completed ($applied_count applied, $count total migration files)"
