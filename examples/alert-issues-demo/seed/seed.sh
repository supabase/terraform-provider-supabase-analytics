#!/usr/bin/env bash
# Ingests the demo log events into the local Logflare: 6 hours of Edge Function requests
# and the advisor lint findings. `seed.sh fnlogs` adds Edge Function console logs from the
# last few minutes, for the custom log rule demo.
set -euo pipefail
cd "$(dirname "$0")"
LOGFLARE_URL=${LOGFLARE_URL:-http://localhost:4000}
TOKEN=${LOGFLARE_ACCESS_TOKEN:-my-cool-api-key-123}

ingest() {
  python3 gen.py "$1" | curl -sS --fail-with-body -H "x-api-key: $TOKEN" \
    -H "Content-Type: application/json" -X POST "$LOGFLARE_URL/api/logs?source_name=$2" -d @-
  echo
}

if [ "${1:-}" = fnlogs ]; then
  ingest fnlogs function_logs
  exit
fi
ingest edge function_edge_logs
ingest advisors advisor_lints
