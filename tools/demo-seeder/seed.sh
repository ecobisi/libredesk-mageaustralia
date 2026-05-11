#!/usr/bin/env bash
# seed.sh — build and run the demo seeder.
#
# Usage:
#   ./seed.sh                              # default: hit http://localhost:9000
#   ./seed.sh --reset                      # wipe demo data first
#   ./seed.sh --url http://localhost:9001  # custom URL
#   ./seed.sh --user System --pass mypass  # custom System creds
#
# Refuses to run against non-loopback hosts unless --allow-non-localhost is
# passed through to the binary. See main.go safetyCheck().

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if ! command -v go >/dev/null 2>&1; then
  echo "ERROR: Go toolchain not found in PATH. Install Go >= 1.25 and try again." >&2
  exit 1
fi

BIN="./demo-seeder.bin"

echo "==> Building demo-seeder ($BIN)"
go build -o "$BIN" .

echo "==> Running demo-seeder"
exec "$BIN" "$@"
