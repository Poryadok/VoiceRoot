#!/usr/bin/env bash
set -euo pipefail

phase="${1:---prepare}"
evidence="${NATS_MIGRATION_EVIDENCE:?VOICE_NATS_MIGRATION_EVIDENCE must point to reviewed migration evidence}"
manifest="${evidence}/evidence.env"
streams="${evidence}/streams.tsv"
consumers="${evidence}/consumer-decisions.tsv"
census="${evidence}/source-census.tsv"

fail() { echo "ERROR: NATS PVC migration blocked: $*" >&2; exit 1; }
value() { awk -F= -v key="$1" '$1 == key {sub(/^[^=]*=/, ""); print; found=1; exit} END {if (!found) exit 1}' "$manifest"; }
required_true() { [ "$(value "$1")" = true ] || fail "$1 must be reviewed and set to true"; }
count_rows() { awk 'NF {n++} END {print n+0}' "$1"; }

[ -f "$manifest" ] || fail "missing $manifest"
[ -f "$streams" ] || fail "missing $streams (stream name, last sequence, archive path, archive SHA-256)"
[ -f "$consumers" ] || fail "missing $consumers (every source consumer needs a reviewed RESTORE or DROP decision)"
[ -f "$census" ] || fail "missing $census (live source census required; TSV decisions alone are not evidence)"
[ "$(value SOURCE_STREAM_COUNT)" = 7 ] || fail 'source stream count must match the observed 7 streams'
[ "$(value SOURCE_CONSUMER_COUNT)" = 566 ] || fail 'source consumer count must match the observed 566 consumers'
[ "$(value RETAINED_MESSAGE_COUNT)" = 8 ] || fail 'retained message count must match the observed 8 messages'
required_true STORAGE_CLASS_CONFIRMED
required_true CAPACITY_CONFIRMED
storage_class="${VOICE_NATS_STORAGE_CLASS:-}"
storage_size="${VOICE_NATS_STORAGE_SIZE:-}"
if [ "$phase" = --prepare ] || [ -n "$storage_class" ]; then
  [ -n "$storage_class" ] && [ "$(value STORAGE_CLASS)" = "$storage_class" ] || fail 'evidence STORAGE_CLASS must match VOICE_NATS_STORAGE_CLASS'
fi
if [ "$phase" = --prepare ] || [ -n "$storage_size" ]; then
  [ -n "$storage_size" ] && [ "$(value STORAGE_SIZE)" = "$storage_size" ] || fail 'evidence STORAGE_SIZE must match VOICE_NATS_STORAGE_SIZE'
fi
source_context="$(value SOURCE_CONTEXT)" || fail 'SOURCE_CONTEXT must identify the reviewed source NATS CLI context'
target_context="$(value TARGET_CONTEXT)" || fail 'TARGET_CONTEXT must identify the reviewed PVC-backed target context'
[ -n "$source_context" ] && [ -n "$target_context" ] || fail 'source and target NATS contexts must be distinct, named contexts'
[ "$source_context" != "$target_context" ] || fail 'source and PVC-backed target must use distinct NATS CLI contexts'
required_true QUIESCE_CONFIRMED
required_true BACKUP_VERIFIED
required_true SOURCE_BACKUP_RETAINED
[ "$(sha256sum "$census" | awk '{print $1}')" = "$(value SOURCE_CENSUS_SHA256)" ] || fail 'source census hash differs from reviewed evidence'
source_context_env="${NATS_SOURCE_CONTEXT:-}"
[ -n "$source_context_env" ] && [ "$source_context_env" = "$source_context" ] || fail 'set NATS_SOURCE_CONTEXT to the exact reviewed source context for live census verification'
fresh_census="$(mktemp)"
trap 'rm -f "$fresh_census"' EXIT
"$(dirname "$0")/nats-source-census.sh" "$source_context_env" "$fresh_census" >/dev/null || fail 'live source census failed'
cmp -s "$census" "$fresh_census" || fail 'live source identities/state differ from the archived census; re-quiesce and re-export'
[ "$(count_rows "$census")" = "$(value SOURCE_CONSUMER_COUNT)" ] || fail 'census identity count differs from the reviewed 566 source consumers'

[ "$(count_rows "$streams")" -eq 7 ] || fail 'streams.tsv must contain exactly 7 stream records'
[ "$(count_rows "$consumers")" = "$(value SOURCE_CONSUMER_COUNT)" ] || fail 'every one of the 566 source consumers must have a decision row'

declare -A declared_streams=()
while IFS="$(printf '\t')" read -r stream_name rest_fields; do
  [ -n "$stream_name" ] || continue
  declared_streams["$stream_name"]=1
done < "$streams"

restore_count=0
drop_count=0
declare -A seen_consumers=()
declare -A census_hashes=()
while IFS="$(printf '\t')" read -r stream consumer state_hash rest; do
  [ -n "$stream" ] || continue
  [ -n "$consumer" ] && [[ "$state_hash" =~ ^[a-fA-F0-9]{64}$ ]] || fail 'source census row has missing identity or invalid state hash'
  key="${stream}/${consumer}"
  [ -z "${census_hashes[$key]:-}" ] || fail "duplicate source census identity ${key}"
  census_hashes["$key"]="$state_hash"
done < "$census"

while IFS="$(printf '\t')" read -r stream consumer kind state_hash decision approval_ref rest; do
  [ -n "$stream" ] || continue
  [ -n "$consumer" ] && [ -n "$kind" ] || fail 'consumer decision row has missing stream, consumer, or classification'
  [ -n "${census_hashes[${stream}/${consumer}]:-}" ] || fail "decision identity is absent from source census: ${stream}/${consumer}"
  [ "${census_hashes[${stream}/${consumer}]}" = "$state_hash" ] || fail "source state identity mismatch for ${stream}/${consumer}"
  [ -n "${declared_streams[$stream]:-}" ] || fail "consumer ${consumer} references unknown stream ${stream}"
  key="${stream}/${consumer}"
  [ -z "${seen_consumers[$key]:-}" ] || fail "duplicate consumer decision for ${key}"
  seen_consumers["$key"]=1
  case "$decision" in
    RESTORE) restore_count=$((restore_count + 1)) ;;
    DROP)
      [ -n "$approval_ref" ] || fail "DROP for ${stream}/${consumer} requires an explicit review reference"
      drop_count=$((drop_count + 1))
      ;;
    *) fail "consumer ${stream}/${consumer} must be explicitly RESTORE or DROP" ;;
  esac
  [ -z "${rest:-}" ] || fail 'consumer decision rows must have exactly six tab-separated columns'
done < "$consumers"
cap="$(value APP_CONSUMER_CAP)"
case "$cap" in ''|*[!0-9]*) fail 'APP_CONSUMER_CAP must be an integer' ;; esac
[ "$restore_count" -le "$cap" ] || fail "${restore_count} consumers selected for restore exceeds APP_CONSUMER_CAP=${cap}; classify and approve explicit drops"
[ $((restore_count + drop_count)) -eq "$(value SOURCE_CONSUMER_COUNT)" ] || fail 'consumer decisions do not account for all source consumers'
[ "${#seen_consumers[@]}" -eq "${#census_hashes[@]}" ] || fail 'consumer decision identities are not an exact match for the source census'

declare -A seen_streams=()
while IFS="$(printf '\t')" read -r name last_sequence archive expected_hash config_hash rest; do
  [ -n "$name" ] || continue
  [[ "$name" =~ ^[A-Za-z0-9_-]+$ ]] || fail "invalid stream name ${name}"
  [ -z "${seen_streams[$name]:-}" ] || fail "duplicate stream record for ${name}"
  seen_streams["$name"]=1
  [[ "$last_sequence" =~ ^[0-9]+$ ]] || fail "invalid last sequence for ${name}"
  [ -n "$last_sequence" ] && [ -n "$archive" ] && [ -n "$expected_hash" ] && [[ "$config_hash" =~ ^[a-fA-F0-9]{64}$ ]] || fail "incomplete stream record for ${name}"
  [ -z "${rest:-}" ] || fail "stream record for ${name} must have exactly five tab-separated columns"
  [ "$(basename "$archive")" = "$archive" ] || fail "archive path must be a file name for ${name}"
  [[ "$expected_hash" =~ ^[a-fA-F0-9]{64}$ ]] || fail "invalid SHA-256 for ${name}"
  [ -f "${evidence}/${archive}" ] || fail "missing stream archive ${archive}"
  actual_hash="$(sha256sum "${evidence}/${archive}" | awk '{print $1}')"
  [ "$actual_hash" = "$expected_hash" ] || fail "SHA-256 mismatch for ${archive}"
done < "$streams"
[ "${#seen_streams[@]}" -eq 7 ] || fail 'stream inventory must contain 7 distinct streams'

case "$phase" in
  --prepare|--check-only)
    echo "NATS PVC cutover prerequisites validated: 7 stream backups, ${restore_count} consumers selected for restore, ${drop_count} explicitly approved drops."
    ;;
  --acceptance)
    required_true CANDIDATE_TLS_IDENTITY_CONFIRMED
    target_context_env="${NATS_TARGET_CONTEXT:-}"
    [ -n "$target_context_env" ] && [ "$target_context_env" = "$target_context" ] || fail 'set NATS_TARGET_CONTEXT to the exact reviewed candidate context for live restore validation'
    target_census="$(mktemp)"
    target_expected="$(mktemp)"
    target_streams="$(mktemp)"
    trap 'rm -f "$fresh_census" "$target_census" "$target_expected" "$target_streams"' EXIT
    "${NATS_CLI:-nats}" --context "$target_context_env" stream ls --json | jq -r '.streams[]?.config.name' > "$target_streams"
    bash "$(dirname "$0")/verify-nats-stream-inventory.sh" "$streams" "$target_streams" exact
    "$(dirname "$0")/nats-source-census.sh" "$target_context_env" "$target_census" >/dev/null || fail 'candidate census failed'
    awk -F '\t' '$5 == "RESTORE" {print $1 "\t" $2 "\t" $4}' "$consumers" | LC_ALL=C sort -u > "$target_expected"
    LC_ALL=C sort -u "$target_census" > "${target_expected}.actual"
    cmp -s "$target_expected" "${target_expected}.actual" || fail 'candidate consumer identities do not exactly match approved RESTORE decisions'
    rm -f "${target_expected}.actual"
    required_true RESTORE_VALIDATED
    required_true SEQUENCE_HASHES_VERIFIED
    required_true MESSAGE_HASHES_VERIFIED
    message_hashes="${evidence}/message-hashes.tsv"
    [ -f "$message_hashes" ] || fail 'message-hashes.tsv must contain each retained message source/target digest comparison'
    [ "$(count_rows "$message_hashes")" = "$(value RETAINED_MESSAGE_COUNT)" ] || fail 'message-hashes.tsv must contain exactly 8 retained-message records'
    declare -A seen_messages=()
    while IFS="$(printf '\t')" read -r message_stream sequence source_hash target_hash proof_ref rest; do
      [ -n "$message_stream" ] || continue
      [[ "$sequence" =~ ^[0-9]+$ ]] || fail 'retained message sequence must be an integer'
      [[ "$source_hash" =~ ^[a-fA-F0-9]{64}$ ]] && [[ "$target_hash" =~ ^[a-fA-F0-9]{64}$ ]] || fail 'retained message source/target digests must be SHA-256'
      [ "$source_hash" = "$target_hash" ] || fail "retained message hash mismatch for ${message_stream}/${sequence}"
      [ -n "$proof_ref" ] || fail "retained message ${message_stream}/${sequence} requires an owner verification reference"
      key="${message_stream}/${sequence}"
      [ -z "${seen_messages[$key]:-}" ] || fail "duplicate retained message verification for ${key}"
      seen_messages["$key"]=1
      [ -z "${rest:-}" ] || fail 'message hash rows must have exactly five columns'
    done < "$message_hashes"
    [ "${#seen_messages[@]}" -eq "$(value RETAINED_MESSAGE_COUNT)" ] || fail 'retained message hash proof identities are incomplete'
    required_true REPLAY_IDEMPOTENCY_APPROVED
    required_true CUTOVER_FENCE_APPROVED
    echo 'Post-restore acceptance evidence validated; keep source backups until explicit acceptance.'
    ;;
  *) fail 'phase must be --prepare or --acceptance' ;;
esac
