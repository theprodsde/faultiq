#!/usr/bin/env bash
#
# Generate scripts/certs/corporate-ca.crt from the host OS trust store.
#
# Only needed on machines behind a TLS-intercepting proxy (Zscaler, corporate
# MITM), where `docker compose build` fails with:
#     tls: failed to verify certificate: x509: certificate signed by unknown authority
#
# On a normal machine you do not need to run this — the builds use the public
# Go module proxy over standard TLS.
#
# Usage: ./scripts/generate-corporate-ca.sh
set -euo pipefail

OUT_DIR="$(cd "$(dirname "$0")" && pwd)/certs"
OUT_FILE="$OUT_DIR/corporate-ca.crt"
mkdir -p "$OUT_DIR"

os="$(uname -s)"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

case "$os" in
  Darwin)
    # Admin-installed roots (corporate CAs land here) + system roots.
    security find-certificate -a -p /Library/Keychains/System.keychain >> "$tmp" 2>/dev/null || true
    security find-certificate -a -p /System/Library/Keychains/SystemRootCertificates.keychain >> "$tmp" 2>/dev/null || true
    ;;
  Linux)
    # Common bundle locations.
    for f in /etc/ssl/certs/ca-certificates.crt /etc/pki/tls/certs/ca-bundle.crt; do
      [ -f "$f" ] && cat "$f" >> "$tmp"
    done
    ;;
  *)
    echo "Unsupported OS: $os. Export your corporate root CA(s) to $OUT_FILE manually." >&2
    exit 1
    ;;
esac

# Keep only the corporate/intercepting CA certs so the bundle stays small and
# obvious. Adjust the pattern if your organisation differs.
python3 - "$tmp" "$OUT_FILE" <<'PY'
import re, subprocess, sys
src, out = sys.argv[1], sys.argv[2]
data = open(src, encoding="utf-8", errors="ignore").read()
blocks = re.findall(r'-----BEGIN CERTIFICATE-----.*?-----END CERTIFICATE-----', data, re.S)
KEYWORDS = ("Zscaler", "GOJCI", "GO JCI", "Johnson Controls", "JCI Private",
            "Corporate", "Proxy", "MITM", "Netskope", "Palo Alto", "Blue Coat")
kept = []
for b in blocks:
    p = subprocess.run(["openssl", "x509", "-noout", "-subject", "-issuer"],
                       input=b, capture_output=True, text=True)
    if any(k in p.stdout for k in KEYWORDS):
        kept.append(b.strip())
if not kept:
    print("No corporate/intercepting CA certs matched. If your build still "
          "fails on TLS, copy your root CA into", out, "manually.", file=sys.stderr)
    sys.exit(2)
open(out, "w").write("\n".join(kept) + "\n")
print(f"Wrote {len(kept)} corporate CA cert(s) to {out}")
PY
