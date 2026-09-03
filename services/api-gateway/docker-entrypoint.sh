#!/bin/sh
set -e
# write runtime env for SPA (ensure target dir exists)
mkdir -p /usr/share/ui
cat > /usr/share/ui/env-config.js <<EOF
window.__ENV = {
  "API_BASE": "${API_BASE:-/api}",
  "KEYCLOAK_URL": "${KEYCLOAK_URL:-http://keycloak:8080}",
  "KEYCLOAK_REALM": "${KEYCLOAK_REALM:-faultiq}",
  "KEYCLOAK_CLIENT": "${KEYCLOAK_CLIENT:-faultiq-ui}"
}
EOF

exec "$@"
