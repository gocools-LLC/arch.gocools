#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${ARCH_SMOKE_PORT:-18081}"
BASE_URL="http://127.0.0.1:${PORT}"
TMP_DIR="$(mktemp -d)"
LOG_PATH="${TMP_DIR}/arch.log"

cleanup() {
  if [[ -n "${ARCH_PID:-}" ]]; then
    kill -TERM "${ARCH_PID}" >/dev/null 2>&1 || true
    wait "${ARCH_PID}" >/dev/null 2>&1 || true
  fi
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

cd "${ROOT_DIR}"

echo "Starting Arch smoke target on ${BASE_URL}..."
ARCH_HTTP_ADDR=":${PORT}" go run ./cmd/arch >"${LOG_PATH}" 2>&1 &
ARCH_PID=$!

READY=0
for _ in $(seq 1 60); do
  if curl -fsS "${BASE_URL}/healthz" >"${TMP_DIR}/healthz.json" 2>/dev/null; then
    READY=1
    break
  fi
  sleep 0.25
done

if [[ "${READY}" -ne 1 ]]; then
  echo "Arch failed readiness checks."
  echo "Arch logs:"
  cat "${LOG_PATH}"
  exit 1
fi

cat >"${TMP_DIR}/graph-diff.json" <<'JSON'
{
  "stack_id": "dev-stack",
  "environment": "dev",
  "before": {
    "schema_version": "arch.gocools/v1alpha1",
    "generated_at": "2026-03-05T00:00:00Z",
    "nodes": [],
    "edges": []
  },
  "after": {
    "schema_version": "arch.gocools/v1alpha1",
    "generated_at": "2026-03-05T00:01:00Z",
    "nodes": [],
    "edges": []
  }
}
JSON

cat >"${TMP_DIR}/create-stack.json" <<'JSON'
{
  "action": "create",
  "stack_id": "dev-stack",
  "environment": "dev",
  "actor": "alice",
  "tags": {
    "gocools:stack-id": "dev-stack",
    "gocools:environment": "dev",
    "gocools:owner": "alice"
  }
}
JSON

cat >"${TMP_DIR}/update-stack-missing-owner.json" <<'JSON'
{
  "action": "update",
  "stack_id": "dev-stack",
  "environment": "dev",
  "actor": "alice",
  "tags": {
    "gocools:stack-id": "dev-stack",
    "gocools:environment": "dev"
  }
}
JSON

cat >"${TMP_DIR}/drift.json" <<'JSON'
{
  "desired": [
    {
      "id": "i-1",
      "type": "aws.ec2.instance",
      "state": "running"
    }
  ],
  "actual": [
    {
      "id": "i-1",
      "type": "aws.ec2.instance",
      "state": "stopped"
    }
  ]
}
JSON

healthz="$(cat "${TMP_DIR}/healthz.json")"
graph="$(curl -fsS "${BASE_URL}/api/v1/graph?stack_id=dev-stack&environment=dev")"
graphDiff="$(curl -fsS -X POST "${BASE_URL}/api/v1/graph/diff" -H 'content-type: application/json' --data @"${TMP_DIR}/graph-diff.json")"
createStack="$(curl -fsS -X POST "${BASE_URL}/api/v1/stacks/operations" -H 'content-type: application/json' --data @"${TMP_DIR}/create-stack.json")"
drift="$(curl -fsS -X POST "${BASE_URL}/api/v1/drift" -H 'content-type: application/json' --data @"${TMP_DIR}/drift.json")"

updateStatus="$(curl -sS -o "${TMP_DIR}/update-error.json" -w '%{http_code}' -X POST "${BASE_URL}/api/v1/stacks/operations" -H 'content-type: application/json' --data @"${TMP_DIR}/update-stack-missing-owner.json")"
updateError="$(cat "${TMP_DIR}/update-error.json")"

echo "${healthz}" | grep -q '"status":"ok"' || {
  echo "unexpected /healthz response: ${healthz}"
  exit 1
}
echo "${graph}" | grep -q '"schema_version":"arch.gocools/v1alpha1"' || {
  echo "unexpected graph API response: ${graph}"
  exit 1
}
echo "${graphDiff}" | grep -q '"changes"' || {
  echo "unexpected graph diff response: ${graphDiff}"
  exit 1
}
echo "${createStack}" | grep -q '"executed":true' || {
  echo "unexpected stack create response: ${createStack}"
  exit 1
}
echo "${drift}" | grep -q '"changed":' || {
  echo "unexpected drift response: ${drift}"
  exit 1
}
if [[ "${updateStatus}" != "400" ]]; then
  echo "expected owner-tag update guardrail to return HTTP 400, got ${updateStatus}"
  echo "response: ${updateError}"
  exit 1
fi
echo "${updateError}" | grep -q 'gocools:owner' || {
  echo "expected owner-tag remediation in guardrail response, got: ${updateError}"
  exit 1
}

echo "Arch smoke checks passed."
