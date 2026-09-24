#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INFRA="${ROOT}/deploy/staging/infra.yaml"
APPLY="${ROOT}/scripts/staging/apply-infra.sh"
GUARD="${ROOT}/scripts/staging/guard-nats-pvc-migration.sh"
PREFLIGHT="${ROOT}/scripts/staging/preflight-nats-pvc.sh"
MIGRATE="${ROOT}/scripts/staging/migrate-nats-pvc.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }
grep -Fq 'name: voice-nats-pvc-candidate' "${INFRA}" || fail 'staging must define an isolated PVC-backed NATS candidate'
grep -Fq 'name: voice-nats-pvc-candidate' "${INFRA}" || fail 'candidate identity must remain distinct from source'
awk '/name: voice-nats$/{source=1} source && /emptyDir:/{ok=1} source && /^---/{source=0} END{exit !ok}' "${INFRA}" || fail 'original NATS source must remain on its pod-scoped data volume'
awk '/name: voice-nats-pvc-candidate$/{candidate=1} candidate && /persistentVolumeClaim:/{ok=1} candidate && /claimName: voice-nats-jsdata/{claim=1} candidate && /^---/{candidate=0} END{exit !(ok && claim)}' "${INFRA}" || fail 'candidate NATS must mount the dedicated PVC'
grep -Fq 'guard-nats-pvc-migration.sh' "${APPLY}" || fail 'apply-infra must gate NATS rollout on migration evidence'
guard_line="$(grep -n 'guard-nats-pvc-migration.sh.*--prepare' "${APPLY}" | cut -d: -f1)"
apply_line="$(grep -n 'render "${ROOT}/deploy/staging/infra.yaml"' "${APPLY}" | cut -d: -f1)"
[ "$guard_line" -lt "$apply_line" ] || fail 'migration guard must run before staging infra apply'
grep -Fq 'VOICE_NATS_STORAGE_CLASS' "${APPLY}" || fail 'storage class must be explicit'
grep -Fq 'VOICE_NATS_STORAGE_SIZE' "${APPLY}" || fail 'storage size must be explicit'
for executable in "$GUARD" "$PREFLIGHT" "$MIGRATE" "${ROOT}/scripts/staging/nats-source-census.sh"; do test -x "$executable" || fail "expected executable $executable"; done
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
grep -Fq 'claimName: voice-nats-jsdata' "${tmp}/infra-rendered.yaml" || fail 'candidate Deployment must mount the PVC claim'
grep -Fq 'voice-nats-pvc-candidate' "${APPLY}" || fail 'apply must prepare candidate without replacing source'
grep -Fq 'cutover' "${MIGRATE}" || fail 'migration tool must provide a separately gated cutover'
grep -Fq 'source-census.tsv' "${GUARD}" || fail 'guard must validate source census identity, not only decision row count'
awk '/kubectl (apply|create|delete|patch|rollout)/{if (!mutation) mutation=NR} /source_context=/{context=NR} /NATS_TARGET_CONTEXT.*source_context/{matchline=NR} END{exit !(context && matchline && mutation && context < mutation && matchline < mutation)}' "${MIGRATE}" || fail 'rollback context validation must precede every kubectl mutation'
grep -Fq 'RESTORE_ACCEPTED' "${MIGRATE}" || fail 'selector cutover must require recorded candidate restore acceptance'
grep -Fq 'voice-nats-pvc-candidate' "${MIGRATE}" || fail 'cutover target must be the isolated candidate'
grep -Fq 'cutover' "${APPLY}" && grep -Fq 'bootstrap jobs deferred' "${APPLY}" || fail 'infra apply must defer NATS bootstrap until migration acceptance'
! grep -Eq 'kubectl patch service voice-nats' "${APPLY}" || fail 'ordinary infra apply must never switch the NATS service selector'
grep -Fq 'candidate consumer identities do not exactly match' "${GUARD}" || fail 'acceptance must verify restored consumer identities against decisions'
grep -Fq 'message-hashes.tsv' "${GUARD}" || fail 'acceptance must require explicit retained-message hashes'
mkdir "${tmp}/evidence"
for index in 1 2 3 4 5 6 7; do
  printf 'archive-%s\n' "$index" > "${tmp}/evidence/STREAM_${index}.tgz"
  hash="$(sha256sum "${tmp}/evidence/STREAM_${index}.tgz" | awk '{print $1}')"
  printf 'STREAM_%s\t10\tSTREAM_%s.tgz\t%s\t%064d\n' "$index" "$index" "$hash" "$index" >> "${tmp}/evidence/streams.tsv"
done
cat >"${tmp}/evidence/evidence.env" <<'EOF'
SOURCE_STREAM_COUNT=7
SOURCE_CONSUMER_COUNT=566
SOURCE_CENSUS_SHA256=placeholder
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

: > "${tmp}/evidence/source-census.tsv"
awk 'BEGIN { for (i=1; i<=566; i++) printf "STREAM_1\tC_%d\t%064d\n", i, i }' > "${tmp}/evidence/source-census.tsv"
sed -i "s/SOURCE_CENSUS_SHA256=placeholder/SOURCE_CENSUS_SHA256=$(sha256sum "${tmp}/evidence/source-census.tsv" | awk '{print $1}')/" "${tmp}/evidence/evidence.env"
: > "${tmp}/evidence/consumer-decisions.tsv"
awk 'BEGIN { for (i=1; i<=566; i++) printf "STREAM_1\tC_%d\tdurable\t%064d\tRESTORE\t\n", i, i }' > "${tmp}/evidence/consumer-decisions.tsv"
if NATS_MIGRATION_EVIDENCE="${tmp}/evidence" VOICE_NATS_STORAGE_CLASS=confirmed-class VOICE_NATS_STORAGE_SIZE=20Gi bash "${GUARD}" --check-only >/dev/null 2>&1; then
  fail 'fabricated 566 TSV rows without live source context must not pass'
fi

: > "${tmp}/evidence/consumer-decisions.tsv"
awk 'BEGIN { for (i=1; i<=566; i++) if (i<=512) printf "STREAM_1\tC_%d\tdurable\t%064d\tRESTORE\t\n", i, i; else printf "STREAM_1\tC_%d\tdurable\t%064d\tDROP\tREVIEW-123\n", i, i }' > "${tmp}/evidence/consumer-decisions.tsv"
echo 'CUTOVER_ACCEPTED=false' >> "${tmp}/evidence/evidence.env"
grep -Fq 'live source identities/state differ' "${GUARD}" || fail 'guard must compare decisions to live source census before evidence acceptance'
grep -Fq 'candidate consumer identities do not exactly match' "${GUARD}" || fail 'guard must compare restored identities to approved RESTORE set'

# Wrong-context rollback must fail before invoking kubectl.
mkdir -p "${tmp}/bin" "${tmp}/rollback"
cat >"${tmp}/rollback/evidence.env" <<'EOF'
SOURCE_CONTEXT=staging-source
TARGET_CONTEXT=staging-candidate
EOF
cat >"${tmp}/bin/kubectl" <<EOF
#!/usr/bin/env bash
echo invoked >>"${tmp}/kubectl.log"
EOF
chmod +x "${tmp}/bin/kubectl"
if PATH="${tmp}/bin:${PATH}" NATS_MIGRATION_EVIDENCE="${tmp}/rollback" NATS_ROLLBACK_APPROVED=true NATS_TARGET_QUIESCED=true NATS_TARGET_CONTEXT=wrong-context VOICE_NATS_STORAGE_CLASS=confirmed-class VOICE_NATS_STORAGE_SIZE=20Gi bash "${MIGRATE}" rollback >/dev/null 2>&1; then
  fail 'rollback with the wrong target context must fail'
fi
[ ! -f "${tmp}/kubectl.log" ] || fail 'wrong-context rollback must not invoke kubectl'
grep -Fq 'VOICE_NATS_STORAGE_CLASS="$VOICE_NATS_STORAGE_CLASS"' "${MIGRATE}" || fail 'rollback must pass storage class to prepare validation'
grep -Fq 'VOICE_NATS_STORAGE_SIZE="$VOICE_NATS_STORAGE_SIZE"' "${MIGRATE}" || fail 'rollback must pass storage size to prepare validation'
cat >"${tmp}/rollback/evidence.env" <<'EOF'
SOURCE_CONTEXT=staging-source
CUTOVER_ACCEPTED=false
EOF
cat >"${tmp}/bin/bash" <<EOF
#!${BASH}
echo "guard:\$*:\$VOICE_NATS_STORAGE_CLASS:\$VOICE_NATS_STORAGE_SIZE" >>"${tmp}/actions.log"
EOF
cat >"${tmp}/bin/kubectl" <<EOF
#!${BASH}
echo "kubectl:\$*" >>"${tmp}/actions.log"
EOF
chmod +x "${tmp}/bin/bash" "${tmp}/bin/kubectl"
PATH="${tmp}/bin:${PATH}" NATS_MIGRATION_EVIDENCE="${tmp}/rollback" NATS_ROLLBACK_APPROVED=true NATS_TARGET_QUIESCED=true NATS_TARGET_CONTEXT=staging-source NATS_SOURCE_CONTEXT=staging-source VOICE_NATS_STORAGE_CLASS=confirmed-class VOICE_NATS_STORAGE_SIZE=20Gi "${BASH:-C:/Program Files/Git/bin/bash.exe}" "${MIGRATE}" rollback >/dev/null || fail 'approved rollback with matching contexts should reach selector restore'
grep -Fq 'guard:' "${tmp}/actions.log" || fail 'rollback must invoke prepare validation'
grep -Fq 'guard:' "${tmp}/actions.log" && grep -Fq 'confirmed-class:20Gi' "${tmp}/actions.log" || fail 'rollback must pass storage class and size to prepare validation'
guard_action="$(grep -n '^guard:' "${tmp}/actions.log" | cut -d: -f1)"
kubectl_action="$(grep -n '^kubectl:' "${tmp}/actions.log" | cut -d: -f1)"
[ "$guard_action" -lt "$kubectl_action" ] || fail 'rollback must validate evidence before selector mutation'

# A protected source identity/state change must alter the fresh census.
mkdir -p "${tmp}/census-bin"
cat >"${tmp}/census-bin/nats" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  *"stream ls"*) printf '{"streams":[{"config":{"name":"S1"}}]}\n' ;;
  *"consumer ls"*) printf '{"consumers":[{"name":"C1"}]}\n' ;;
  *"consumer info"*) printf '{"config":{"durable_name":"C1"},"state":{"delivered":%s}}\n' "${NATS_FAKE_STATE:-1}" ;;
  *) exit 2 ;;
esac
EOF
cat >"${tmp}/census-bin/jq" <<'EOF'
#!/usr/bin/env bash
query="$*"
case "$query" in
  *"streams[]?"*) printf 'S1\n' ;;
  *"consumers[]?"*) printf 'C1\n' ;;
  *"{config,state}"*) printf '{"config":{"durable_name":"C1"},"state":{"delivered":%s}}\n' "${NATS_FAKE_STATE:-1}" ;;
  *) exit 2 ;;
esac
EOF
chmod +x "${tmp}/census-bin/nats" "${tmp}/census-bin/jq"
NATS_CLI=nats JQ_CLI=jq PATH="${tmp}/census-bin:${PATH}" NATS_FAKE_STATE=1 bash "${ROOT}/scripts/staging/nats-source-census.sh" fake-source "${tmp}/census-before.tsv" >/dev/null
NATS_CLI=nats JQ_CLI=jq PATH="${tmp}/census-bin:${PATH}" NATS_FAKE_STATE=2 bash "${ROOT}/scripts/staging/nats-source-census.sh" fake-source "${tmp}/census-after.tsv" >/dev/null
! cmp -s "${tmp}/census-before.tsv" "${tmp}/census-after.tsv" || fail 'source config/state identity change must change census'
grep -Fq 'cmp -s "$census" "$fresh_census"' "${GUARD}" || fail 'guard must fail closed on a source census mismatch'
echo 'staging NATS PVC migration contract: OK'
