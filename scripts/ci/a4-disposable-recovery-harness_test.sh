#!/usr/bin/env bash
# Contract for the A4 recovery-evidence harness. It does not need Docker or a cluster.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
HARNESS="${ROOT}/scripts/dev/a4-disposable-recovery-harness.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

[[ -x "${HARNESS}" ]] || fail 'A4 harness must be executable'

for name in config-validation admission observability durable-store-manifest; do
  printf '%s evidence\n' "${name}" >"${TMP_DIR}/${name}.txt"
done

"${HARNESS}" --dry-run \
  --source 'postgres://source-user:source-pass@source.example:5432/voice' \
  --target 'postgres://target-user:target-pass@target.example:5432/voice-restore' \
  --artifact-dir "${TMP_DIR}/bundle" \
  --config-validation "${TMP_DIR}/config-validation.txt" \
  --admission "${TMP_DIR}/admission.txt" \
  --observability "${TMP_DIR}/observability.txt" \
  --durable-store-manifest "${TMP_DIR}/durable-store-manifest.txt"

[[ -f "${TMP_DIR}/bundle/recovery-evidence.json" ]] || fail 'missing evidence manifest'
grep -Fq '"mode": "dry-run"' "${TMP_DIR}/bundle/recovery-evidence.json" || fail 'manifest must record dry-run mode'
grep -Fq '"source": "source.example:5432/voice"' "${TMP_DIR}/bundle/recovery-evidence.json" || fail 'source endpoint was not redacted correctly'
grep -Fq '"target": "target.example:5432/voice-restore"' "${TMP_DIR}/bundle/recovery-evidence.json" || fail 'target endpoint was not redacted correctly'
if grep -Fq 'source-pass' "${TMP_DIR}/bundle/recovery-evidence.json" || grep -Fq 'target-pass' "${TMP_DIR}/bundle/recovery-evidence.json"; then
  fail 'credentials must not enter the evidence manifest'
fi
grep -Fq 'config-validation.txt' "${TMP_DIR}/bundle/recovery-evidence.json" || fail 'config evidence absent'
grep -Fq 'admission.txt' "${TMP_DIR}/bundle/recovery-evidence.json" || fail 'admission evidence absent'
grep -Fq 'observability.txt' "${TMP_DIR}/bundle/recovery-evidence.json" || fail 'observability evidence absent'
grep -Fq 'durable-store-manifest.txt' "${TMP_DIR}/bundle/recovery-evidence.json" || fail 'durable-store evidence absent'

if "${HARNESS}" --dry-run \
  --source 'postgres://same:secret@same.example:5432/voice' \
  --target 'postgres://other:secret@same.example:5432/voice' \
  --artifact-dir "${TMP_DIR}/same" \
  --config-validation "${TMP_DIR}/config-validation.txt" \
  --admission "${TMP_DIR}/admission.txt" \
  --observability "${TMP_DIR}/observability.txt" \
  --durable-store-manifest "${TMP_DIR}/durable-store-manifest.txt"; then
  fail 'same source and target authority must be rejected'
fi

echo 'A4 disposable recovery harness contract: PASS'
