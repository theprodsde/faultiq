#!/usr/bin/env sh
set -euo pipefail

# Simple Neo4j dump script that uses neo4j-admin inside the running container
CONTAINER=${1:-techgraph-neo4j-1}
OUTDIR=${2:-./backups}
TIMESTAMP=$(date +%Y%m%dT%H%M%S)
mkdir -p "$OUTDIR"
TMP_PATH=/tmp/neo4j_dump_$TIMESTAMP.dump

echo "Creating Neo4j dump in container $CONTAINER -> $TMP_PATH"
docker exec "$CONTAINER" neo4j-admin dump --database=neo4j --to="$TMP_PATH"
echo "Copying dump to host: $OUTDIR/neo4j_dump_$TIMESTAMP.dump"
docker cp "$CONTAINER":"$TMP_PATH" "$OUTDIR/neo4j_dump_$TIMESTAMP.dump"
echo "Cleaning up container temp file"
docker exec "$CONTAINER" rm -f "$TMP_PATH" || true
echo "Done"
