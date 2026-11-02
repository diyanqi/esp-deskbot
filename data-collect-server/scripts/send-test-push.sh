#!/usr/bin/env bash
# Send an IoTDA-shaped, correctly signed property report to the local service.
#
# Usage:
#   IOTDA_PUSH_TOKEN=... ./scripts/send-test-push.sh
#   BASE_URL=https://127.0.0.1:11294 CURL_INSECURE=1 ./scripts/send-test-push.sh
set -euo pipefail

cd "$(dirname "$0")/.."

if [ -z "${IOTDA_PUSH_TOKEN:-}" ] && [ -f .env ]; then
    set -a
    # shellcheck disable=SC1091
    . ./.env
    set +a
fi

TOKEN="${IOTDA_PUSH_TOKEN:-}"
if [ -z "$TOKEN" ]; then
    echo "IOTDA_PUSH_TOKEN is required (or put it in .env)" >&2
    exit 1
fi

BASE_URL="${BASE_URL:-http://127.0.0.1:11294}"
DEVICE_ID="${DEVICE_ID:-6ae74bcd7f2e6c302fa9194e_hbot}"
TIMESTAMP="${TIMESTAMP:-$(python3 -c 'import time; print(int(time.time() * 1000))')}"
NONCE="${NONCE:-$(openssl rand -hex 16)}"
EVENT_TIME="${EVENT_TIME:-$(date -u +%Y%m%dT%H%M%SZ)}"
EVENT_TIME_MS="${EVENT_TIME_MS:-$(date -u +%Y-%m-%dT%H:%M:%S.000Z)}"

SIGNATURE="$(
    printf '%s\n' "$TOKEN" "$TIMESTAMP" "$NONCE" |
        LC_ALL=C sort |
        tr -d '\n' |
        openssl dgst -sha256 -hex |
        awk '{print $NF}'
)"

PAYLOAD="$(
    sed -e "s/__EVENT_TIME_MS__/${EVENT_TIME_MS}/g" \
        -e "s/__EVENT_TIME__/${EVENT_TIME}/g" \
        -e "s/__DEVICE_ID__/${DEVICE_ID}/g" \
        scripts/payload-property.json
)"

CURL_OPTS=(
    -sS
    -X POST "${BASE_URL}/data/push"
    -H "Content-Type: application/json"
    -H "timestamp: ${TIMESTAMP}"
    -H "nonce: ${NONCE}"
    -H "signature: ${SIGNATURE}"
    --data-binary "${PAYLOAD}"
    -w "\nHTTP %{http_code}\n"
)
if [ "${CURL_INSECURE:-0}" = "1" ]; then
    CURL_OPTS=(-k "${CURL_OPTS[@]}")
fi

echo "POST ${BASE_URL}/data/push"
echo "device_id=${DEVICE_ID} timestamp=${TIMESTAMP} event_time=${EVENT_TIME}"
curl "${CURL_OPTS[@]}"
