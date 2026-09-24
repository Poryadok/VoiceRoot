#!/usr/bin/env bash
set -euo pipefail

phase="${1:---prepare}"
evidence="${NATS_MIGRATION_EVIDENCE:?VOICE_NATS_MIGRATION_EVIDENCE must point to reviewed migration evidence}"
manifest="${evidence}/evidence.env"
streams="${evidence}/streams.tsv"
consumers="${evidence}/consumer-decisions.tsv"

fail() { echo "ERROR: NATS PVC migration blocked: $*" >&2; exit 1; }
value() { awk -F= -v key="$1" '$1 == key {sub(/^[^=]*=/, ""); print; found=1; exit} END {if (!found) exit 1}' "$manifest"; }
required_true() { [ "$(value "$1")" = true ] || fail "$1 must be reviewed and set to true"; }
count_rows() { awk 'NF {n++} END {print n+0}' "$1"; }

[ -f "$manifest" ] || fail "missing $manifest"
[ -f "$streams" ] || fail "missing $streams (stream name, last sequence, archive path, archive SHA-256)"
[ -f "$consumers" ] || fail "missing $consumers (every source consumer needs a reviewed RESTORE or DROP decision)"
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
while IFS="$(printf '\t')" read -r stream consumer kind decision approval_ref rest; do
  [ -n "$stream" ] || continue
  [ -n "$consumer" ] && [ -n "$kind" ] || fail 'consumer decision row has missing stream, consumer, or classification'
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
  [ -z "${rest:-}" ] || fail 'consumer decision rows must have exactly five tab-separated columns'
done < "$consumers"
cap="$(value APP_CONSUMER_CAP)"
case "$cap" in ''|*[!0-9]*) fail 'APP_CONSUMER_CAP must be an integer' ;; esac
[ "$restore_count" -le "$cap" ] || fail "${restore_count} consumers selected for restore exceeds APP_CONSUMER_CAP=${cap}; classify and approve explicit drops"
[ $((restore_count + drop_count)) -eq "$(value SOURCE_CONSUMER_COUNT)" ] || fail 'consumer decisions do not account for all source consumers'

declare -A seen_streams=()
while IFS="$(printf '\t')" read -r name last_sequence archive expected_hash rest; do
  [ -n "$name" ] || continue
  [[ "$name" =~ ^[A-Za-z0-9_-]+$ ]] || fail "invalid stream name ${name}"
  [ -z "${seen_streams[$name]:-}" ] || fail "duplicate stream record for ${name}"
  seen_streams["$name"]=1
  [[ "$last_sequence" =~ ^[0-9]+$ ]] || fail "invalid last sequence for ${name}"
  [ -n "$last_sequence" ] && [ -n "$archive" ] && [ -n "$expected_hash" ] || fail "incomplete stream record for ${name}"
  [ -z "${rest:-}" ] || fail "stream record for ${name} must have exactly four tab-separated columns"
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
    required_true RESTORE_VALIDATED
    required_true SEQUENCE_HASHES_VERIFIED
    required_true REPLAY_IDEMPOTENCY_APPROVED
    required_true CUTOVER_FENCE_APPROVED
    echo 'Post-restore acceptance evidence validated; keep source backups until explicit acceptance.'
    ;;
  *) fail 'phase must be --prepare or --acceptance' ;;
esac
