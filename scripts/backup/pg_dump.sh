#!/usr/bin/env sh
set -euo pipefail

CONTAINER=${1:-techgraph-postgres-1}
OUTDIR=${2:-./backups}
TIMESTAMP=$(date +%Y%m%dT%H%M%S)
mkdir -p "$OUTDIR"

echo "Creating Postgres dump from container $CONTAINER"
docker exec -i "$CONTAINER" pg_dump -U postgres -d postgres > "$OUTDIR/pg_dump_$TIMESTAMP.sql"
echo "Dump written to $OUTDIR/pg_dump_$TIMESTAMP.sql"
