#!/usr/bin/env bash
# Operator-run JetStream snapshot/restore. Requires a quiesced source for backup.
set -euo pipefail

mode="${1:-}"
evidence="${NATS_MIGRATION_EVIDENCE:?set VOICE_NATS_MIGRATION_EVIDENCE to an offline protected evidence directory}"
nats_bin="${NATS_CLI:-nats}"
source_context="${NATS_SOURCE_CONTEXT:-}"

fail() { echo "ERROR: $*" >&2; exit 1; }
case "$mode" in
  backup)
    [ "${NATS_SOURCE_QUIESCED:-false}" = true ] || fail 'freeze writers/consumers and set NATS_SOURCE_QUIESCED=true only after confirmation'
    [ -f "${NATS_STREAMS_FILE:-}" ] || fail 'NATS_STREAMS_FILE must list the reviewed source stream names'
    [ -f "${NATS_LAST_SEQUENCE_FILE:-}" ] || fail 'NATS_LAST_SEQUENCE_FILE must contain reviewed name<TAB>last_sequence records'
    [ -n "$source_context" ] || fail 'NATS_SOURCE_CONTEXT must name the protected NATS CLI context for the source'
    command -v "$nats_bin" >/dev/null 2>&1 || fail "NATS CLI not found: $nats_bin"
    [ ! -e "$evidence" ] || fail "evidence directory already exists; refusing to overwrite $evidence"
    [ ! -e "${evidence}.census.tmp" ] && [ ! -e "${evidence}.sequences.tmp" ] || fail 'stale census scratch file exists; inspect/remove it before backup'
    trap 'rm -f "${evidence}.census.tmp" "${evidence}.sequences.tmp"' EXIT
    "$(dirname "$0")/nats-source-census.sh" "$source_context" "${evidence}.census.tmp" >/dev/null
    census_count="$(wc -l <"${evidence}.census.tmp" | tr -d ' ')"
    [ "$census_count" -eq 566 ] || fail "live source census returned ${census_count} consumers; expected exactly 566"
    mapfile -t names < "$NATS_STREAMS_FILE"
    [ "${#names[@]}" -eq 7 ] || fail 'expected exactly 7 reviewed stream names'
    [ "$(printf '%s\n' "${names[@]}" | sort -u | wc -l | tr -d ' ')" -eq 7 ] || fail 'stream names must be unique'
    live_streams="$("$nats_bin" --context "$source_context" stream ls --json | jq -er '[.streams[]?.config.name] | unique | sort | .[]')"
    declared_streams="$(printf '%s\n' "${names[@]}" | LC_ALL=C sort -u)"
    [ "$live_streams" = "$declared_streams" ] || fail 'reviewed stream names do not exactly match the live source inventory'
    : > "${evidence}.sequences.tmp"
    for name in "${names[@]}"; do
      [[ "$name" =~ ^[A-Za-z0-9_-]+$ ]] || fail "invalid stream name: $name"
      last="$(awk -F '\t' -v key="$name" '$1 == key {print $2}' "$NATS_LAST_SEQUENCE_FILE")"
      [[ "$last" =~ ^[0-9]+$ ]] || fail "no valid reviewed last sequence recorded for $name"
      live_info="$("$nats_bin" --context "$source_context" stream info "$name" --json)"
      live_sequence="$(printf '%s' "$live_info" | jq -er '.state.last_seq')"
      [ "$last" = "$live_sequence" ] || fail "reviewed source sequence for $name differs from live source"
      config_hash="$(printf '%s' "$live_info" | jq -cS '.config' | sha256sum | awk '{print $1}')"
      printf '%s\t%s\t%s\n' "$name" "$live_sequence" "$config_hash" >> "${evidence}.sequences.tmp"
    done
    mkdir -p "$evidence"
    mv "${evidence}.census.tmp" "${evidence}/source-census.tsv"
    census_hash="$(sha256sum "${evidence}/source-census.tsv" | awk '{print $1}')"
    : > "${evidence}/streams.tsv"
    cat > "${evidence}/evidence.env" <<EOF
SOURCE_CONTEXT=${source_context}
TARGET_CONTEXT=
SOURCE_STREAM_COUNT=7
SOURCE_CONSUMER_COUNT=566
SOURCE_CENSUS_SHA256=${census_hash}
RETAINED_MESSAGE_COUNT=8
APP_CONSUMER_CAP=512
STORAGE_CLASS=
STORAGE_SIZE=
STORAGE_CLASS_CONFIRMED=false
CAPACITY_CONFIRMED=false
QUIESCE_CONFIRMED=true
BACKUP_VERIFIED=false
SOURCE_BACKUP_RETAINED=false
RESTORE_VALIDATED=false
SEQUENCE_HASHES_VERIFIED=false
MESSAGE_HASHES_VERIFIED=false
REPLAY_IDEMPOTENCY_APPROVED=false
CUTOVER_FENCE_APPROVED=false
CANDIDATE_TLS_IDENTITY_CONFIRMED=false
RESTORE_ACCEPTED=false
CUTOVER_ACCEPTED=false
EOF
    [ "$source_context" = "$(awk -F= '$1 == "SOURCE_CONTEXT" {sub(/^[^=]*=/, ""); print; found=1; exit} END {if (!found) exit 1}' "${evidence}/evidence.env")" ] || fail 'source context evidence initialization failed'
    for name in "${names[@]}"; do
      archive="${name}.tgz"
      "$nats_bin" --context "$source_context" stream backup "$name" "${evidence}/${archive}"
      hash="$(sha256sum "${evidence}/${archive}" | awk '{print $1}')"
      last="$(awk -F '\t' -v key="$name" '$1 == key {print $2}' "$NATS_LAST_SEQUENCE_FILE")"
      config_hash="$(awk -F '\t' -v key="$name" '$1 == key {print $3}' "${evidence}.sequences.tmp")"
      printf '%s\t%s\t%s\t%s\t%s\n' "$name" "$last" "$archive" "$hash" "$config_hash" >> "${evidence}/streams.tsv"
    done
    rm -f "${evidence}.sequences.tmp"
    echo 'Stream archives and hashes created. Complete evidence.env and consumer-decisions.tsv from reviewed source observations before restore or cutover.'
    ;;
  restore)
    bash "$(dirname "$0")/guard-nats-pvc-migration.sh" --prepare
    [ -n "${NATS_TARGET_CONTEXT:-}" ] || fail 'NATS_TARGET_CONTEXT must name the protected NATS CLI context for the target'
    expected_context="${NATS_EXPECTED_CONTEXT:-$(awk -F= '$1 == "TARGET_CONTEXT" {sub(/^[^=]*=/, ""); print; found=1; exit} END {if (!found) exit 1}' "${evidence}/evidence.env")}" || fail 'missing expected target context in evidence'
    [ "$NATS_TARGET_CONTEXT" = "$expected_context" ] || fail 'NATS_TARGET_CONTEXT does not match the reviewed target context'
    [ "${NATS_TARGET_QUIESCED:-false}" = true ] || fail 'keep the target fenced from publishers/consumers during restore'
    target_streams="$(mktemp)"
    trap 'rm -f "$target_streams"' EXIT
    "$nats_bin" --context "$NATS_TARGET_CONTEXT" stream ls --json | jq -r '.streams[]?.config.name' > "$target_streams"
    bash "$(dirname "$0")/verify-nats-stream-inventory.sh" "${evidence}/streams.tsv" "$target_streams" empty
    while IFS="$(printf '\t')" read -r stream last_sequence archive expected_hash config_hash rest; do
      [ -n "$stream" ] || continue
      "$nats_bin" --context "$NATS_TARGET_CONTEXT" stream restore "$stream" "${evidence}/${archive}"
    done < "${evidence}/streams.tsv"
    while IFS="$(printf '\t')" read -r stream consumer kind state_hash decision approval_ref rest; do
      [ "$decision" = DROP ] || continue
      echo "Applying explicitly reviewed DROP ${stream}/${consumer} (${approval_ref})"
      "$nats_bin" --context "$NATS_TARGET_CONTEXT" consumer rm "$stream" "$consumer" --force
    done < "${evidence}/consumer-decisions.tsv"
    while IFS="$(printf '\t')" read -r stream last_sequence archive expected_hash config_hash rest; do
      [ -n "$stream" ] || continue
      target_info="$("$nats_bin" --context "$NATS_TARGET_CONTEXT" stream info "$stream" --json)"
      actual_sequence="$(printf '%s' "$target_info" | jq -er '.state.last_seq')"
      [ "$actual_sequence" = "$last_sequence" ] || fail "target sequence mismatch for $stream (expected $last_sequence, got $actual_sequence)"
      target_config_hash="$(printf '%s' "$target_info" | jq -cS '.config' | sha256sum | awk '{print $1}')"
      [ "$target_config_hash" = "$config_hash" ] || fail "target stream config mismatch for $stream"
    done < "${evidence}/streams.tsv"
    echo 'Restore commands complete. Record target sequences, message hashes, consumer decisions/counts, replay/idempotency proof, then run guard-nats-pvc-migration.sh --acceptance.'
    ;;
  rollback)
    [ "${NATS_ROLLBACK_APPROVED:-false}" = true ] || fail 'rollback requires NATS_ROLLBACK_APPROVED=true after operator approval'
    [ "${NATS_TARGET_QUIESCED:-false}" = true ] || fail 'keep publishers and consumers fenced before rollback'
    [ -n "${NATS_TARGET_CONTEXT:-}" ] || fail 'NATS_TARGET_CONTEXT must name the protected NATS CLI context for the restored original'
    [ -n "${VOICE_NATS_STORAGE_CLASS:-}" ] || fail 'VOICE_NATS_STORAGE_CLASS is required for rollback evidence validation'
    [ -n "${VOICE_NATS_STORAGE_SIZE:-}" ] || fail 'VOICE_NATS_STORAGE_SIZE is required for rollback evidence validation'
    source_context="$(awk -F= '$1 == "SOURCE_CONTEXT" {sub(/^[^=]*=/, ""); print; found=1; exit} END {if (!found) exit 1}' "${evidence}/evidence.env")"
    [ "$NATS_TARGET_CONTEXT" = "$source_context" ] || fail 'rollback target context must match the original source context'
    [ "$(awk -F= '$1 == "CUTOVER_ACCEPTED" {print $2}' "${evidence}/evidence.env")" != true ] || fail 'rollback after acceptance requires a separately reviewed reconciliation plan'
    NATS_MIGRATION_EVIDENCE="$evidence" VOICE_NATS_STORAGE_CLASS="$VOICE_NATS_STORAGE_CLASS" VOICE_NATS_STORAGE_SIZE="$VOICE_NATS_STORAGE_SIZE" bash "$(dirname "$0")/guard-nats-pvc-migration.sh" --prepare
    kubectl patch service voice-nats -n "${VOICE_K8S_NAMESPACE:-voice-staging}" --type=merge -p '{"spec":{"selector":{"app":"voice-nats"}}}'
    echo 'Rollback selector restored to the retained source. Keep the PVC and snapshot artifacts until rollback validation is accepted.'
    ;;
  acceptance)
    bash "$(dirname "$0")/guard-nats-pvc-migration.sh" --acceptance
    ;;
  cutover)
    [ "${NATS_CUTOVER_APPROVED:-false}" = true ] || fail 'cutover requires NATS_CUTOVER_APPROVED=true after operator approval'
    [ "${NATS_TARGET_QUIESCED:-false}" = true ] || fail 'keep publishers and consumers fenced through validation and cutover'
    [ -n "${NATS_TARGET_CONTEXT:-}" ] || fail 'NATS_TARGET_CONTEXT must identify the candidate service context'
    [ -n "${VOICE_NATS_STORAGE_CLASS:-}" ] || fail 'VOICE_NATS_STORAGE_CLASS is required for cutover evidence validation'
    [ -n "${VOICE_NATS_STORAGE_SIZE:-}" ] || fail 'VOICE_NATS_STORAGE_SIZE is required for cutover evidence validation'
    expected_context="$(awk -F= '$1 == "TARGET_CONTEXT" {sub(/^[^=]*=/, ""); print; found=1; exit} END {if (!found) exit 1}' "${evidence}/evidence.env")" || fail 'missing expected candidate context'
    [ "$NATS_TARGET_CONTEXT" = "$expected_context" ] || fail 'target context does not match evidence'
    [ "$(awk -F= '$1 == "RESTORE_ACCEPTED" {print $2}' "${evidence}/evidence.env")" = true ] || fail 'candidate restore acceptance must be recorded before selector mutation'
    NATS_MIGRATION_EVIDENCE="$evidence" VOICE_NATS_STORAGE_CLASS="$VOICE_NATS_STORAGE_CLASS" VOICE_NATS_STORAGE_SIZE="$VOICE_NATS_STORAGE_SIZE" bash "$(dirname "$0")/guard-nats-pvc-migration.sh" --acceptance
    kubectl patch service voice-nats -n "${VOICE_K8S_NAMESPACE:-voice-staging}" --type=merge -p '{"spec":{"selector":{"app":"voice-nats-pvc-candidate"}}}'
    echo 'Service selector cutover complete; keep source and archive retained until post-cutover acceptance.'
    ;;
  *)
    echo 'Usage: migrate-nats-pvc.sh backup|restore|rollback|acceptance|cutover' >&2
    exit 2
    ;;
esac
