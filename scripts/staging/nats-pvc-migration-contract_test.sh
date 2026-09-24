#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INFRA="${ROOT}/deploy/staging/infra.yaml"
APPLY="${ROOT}/scripts/staging/apply-infra.sh"
GUARD="${ROOT}/scripts/staging/guard-nats-pvc-migration.sh"
PREFLIGHT="${ROOT}/scripts/staging/preflight-nats-pvc.sh"
MIGRATE="${ROOT}/scripts/staging/migrate-nats-pvc.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }
awk '/^        - name: jsdata$/{found=1; next} found && /persistentVolumeClaim:/{ok=1} found && /emptyDir:/{bad=1} found && /^        - name:/{found=0} END{exit !(ok && !bad)}' "${INFRA}" || fail 'staging NATS jsdata must use a PVC, not emptyDir'
grep -Fq 'guard-nats-pvc-migration.sh' "${APPLY}" || fail 'apply-infra must gate NATS rollout on migration evidence'
guard_line="$(grep -n 'guard-nats-pvc-migration.sh.*--prepare' "${APPLY}" | cut -d: -f1)"
apply_line="$(grep -n 'render "${ROOT}/deploy/staging/infra.yaml"' "${APPLY}" | cut -d: -f1)"
[ "$guard_line" -lt "$apply_line" ] || fail 'migration guard must run before staging infra apply'
grep -Fq 'VOICE_NATS_STORAGE_CLASS' "${APPLY}" || fail 'storage class must be explicit'
grep -Fq 'VOICE_NATS_STORAGE_SIZE' "${APPLY}" || fail 'storage size must be explicit'
for executable in "$GUARD" "$PREFLIGHT" "$MIGRATE"; do test -x "$executable" || fail "expected executable $executable"; done
! grep -Eq 'kubectl (apply|create|delete|patch|rollout)' "$PREFLIGHT" || fail 'storage preflight must remain read-only'
grep -Fq 'stream backup' "${MIGRATE}" || fail 'migration tool must export stream data'
grep -Fq 'stream restore' "${MIGRATE}" || fail 'migration tool must restore stream data'
grep -Fq 'requires an explicit review reference' "${GUARD}" || fail 'consumer drops must require an explicit review reference'
grep -Fq 'SOURCE_CONTEXT' "${MIGRATE}" || fail 'source context must be explicit'
grep -Fq 'NATS_EXPECTED_CONTEXT' "${MIGRATE}" || fail 'restore target context must be checked against evidence'

tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT
sed -e 's|__NATS_STORAGE_CLASS__|confirmed-class|g' -e 's|__NATS_STORAGE_SIZE__|20Gi|g' "$INFRA" > "${tmp}/infra-rendered.yaml"
grep -Fq 'storageClassName: confirmed-class' "${tmp}/infra-rendered.yaml" || fail 'rendered PVC must use the confirmed storage class'
grep -Fq 'storage: 20Gi' "${tmp}/infra-rendered.yaml" || fail 'rendered PVC must use the confirmed requested capacity'
grep -Fq 'claimName: voice-nats-jsdata' "${tmp}/infra-rendered.yaml" || fail 'NATS Deployment must mount the PVC claim'
mkdir "${tmp}/evidence"
for index in 1 2 3 4 5 6 7; do
  printf 'archive-%s\n' "$index" > "${tmp}/evidence/STREAM_${index}.tgz"
  hash="$(sha256sum "${tmp}/evidence/STREAM_${index}.tgz" | awk '{print $1}')"
  printf 'STREAM_%s\t10\tSTREAM_%s.tgz\t%s\n' "$index" "$index" "$hash" >> "${tmp}/evidence/streams.tsv"
done
cat >"${tmp}/evidence/evidence.env" <<'EOF'
SOURCE_STREAM_COUNT=7
SOURCE_CONSUMER_COUNT=566
RETAINED_MESSAGE_COUNT=8
APP_CONSUMER_CAP=512
SOURCE_CONTEXT=staging-source
TARGET_CONTEXT=staging-pvc-target
STORAGE_CLASS=confirmed-class
STORAGE_SIZE=20Gi
STORAGE_CLASS_CONFIRMED=true
CAPACITY_CONFIRMED=true
QUIESCE_CONFIRMED=true
BACKUP_VERIFIED=true
RESTORE_VALIDATED=true
SEQUENCE_HASHES_VERIFIED=true
REPLAY_IDEMPOTENCY_APPROVED=true
CUTOVER_FENCE_APPROVED=true
SOURCE_BACKUP_RETAINED=true
EOF

: > "${tmp}/evidence/consumer-decisions.tsv"
awk 'BEGIN { for (i=1; i<=566; i++) printf "STREAM_1\tC_%d\tdurable\tRESTORE\t\n", i }' > "${tmp}/evidence/consumer-decisions.tsv"
if NATS_MIGRATION_EVIDENCE="${tmp}/evidence" VOICE_NATS_STORAGE_CLASS=confirmed-class VOICE_NATS_STORAGE_SIZE=20Gi bash "${GUARD}" --check-only >/dev/null 2>&1; then
  fail '566 restores must be blocked by the 512 consumer cap'
fi

: > "${tmp}/evidence/consumer-decisions.tsv"
awk 'BEGIN { for (i=1; i<=566; i++) if (i<=512) printf "STREAM_1\tC_%d\tdurable\tRESTORE\t\n", i; else printf "STREAM_1\tC_%d\tdurable\tDROP\tREVIEW-123\n", i }' > "${tmp}/evidence/consumer-decisions.tsv"
NATS_MIGRATION_EVIDENCE="${tmp}/evidence" VOICE_NATS_STORAGE_CLASS=confirmed-class VOICE_NATS_STORAGE_SIZE=20Gi bash "${GUARD}" --check-only >/dev/null || fail 'complete reviewed evidence should pass check-only validation'
echo 'staging NATS PVC migration contract: OK'
