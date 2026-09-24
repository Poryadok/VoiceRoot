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
    mapfile -t names < "$NATS_STREAMS_FILE"
    [ "${#names[@]}" -eq 7 ] || fail 'expected exactly 7 reviewed stream names'
    [ "$(printf '%s\n' "${names[@]}" | sort -u | wc -l | tr -d ' ')" -eq 7 ] || fail 'stream names must be unique'
    for name in "${names[@]}"; do
      [[ "$name" =~ ^[A-Za-z0-9_-]+$ ]] || fail "invalid stream name: $name"
      last="$(awk -F '\t' -v key="$name" '$1 == key {print $2}' "$NATS_LAST_SEQUENCE_FILE")"
      [[ "$last" =~ ^[0-9]+$ ]] || fail "no valid reviewed last sequence recorded for $name"
    done
    mkdir -p "$evidence"
    : > "${evidence}/streams.tsv"
    cat > "${evidence}/evidence.env" <<EOF
SOURCE_CONTEXT=${source_context}
TARGET_CONTEXT=
SOURCE_STREAM_COUNT=7
SOURCE_CONSUMER_COUNT=566
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
REPLAY_IDEMPOTENCY_APPROVED=false
CUTOVER_FENCE_APPROVED=false
EOF
    [ "$source_context" = "$(awk -F= '$1 == "SOURCE_CONTEXT" {sub(/^[^=]*=/, ""); print; found=1; exit} END {if (!found) exit 1}' "${evidence}/evidence.env")" ] || fail 'source context evidence initialization failed'
    for name in "${names[@]}"; do
      archive="${name}.tgz"
      "$nats_bin" --context "$source_context" stream backup "$name" "${evidence}/${archive}"
      hash="$(sha256sum "${evidence}/${archive}" | awk '{print $1}')"
      last="$(awk -F '\t' -v key="$name" '$1 == key {print $2}' "$NATS_LAST_SEQUENCE_FILE")"
      printf '%s\t%s\t%s\t%s\n' "$name" "$last" "$archive" "$hash" >> "${evidence}/streams.tsv"
    done
    echo 'Stream archives and hashes created. Complete evidence.env and consumer-decisions.tsv from reviewed source observations before restore or cutover.'
    ;;
  restore)
    bash "$(dirname "$0")/guard-nats-pvc-migration.sh" --prepare
    [ -n "${NATS_TARGET_CONTEXT:-}" ] || fail 'NATS_TARGET_CONTEXT must name the protected NATS CLI context for the target'
    expected_context="${NATS_EXPECTED_CONTEXT:-$(awk -F= '$1 == "TARGET_CONTEXT" {sub(/^[^=]*=/, ""); print; found=1; exit} END {if (!found) exit 1}' "${evidence}/evidence.env")}" || fail 'missing expected target context in evidence'
    [ "$NATS_TARGET_CONTEXT" = "$expected_context" ] || fail 'NATS_TARGET_CONTEXT does not match the reviewed target context'
    [ "${NATS_TARGET_QUIESCED:-false}" = true ] || fail 'keep the target fenced from publishers/consumers during restore'
    while IFS="$(printf '\t')" read -r stream last_sequence archive expected_hash rest; do
      [ -n "$stream" ] || continue
      "$nats_bin" --context "$NATS_TARGET_CONTEXT" stream restore "$stream" "${evidence}/${archive}"
    done < "${evidence}/streams.tsv"
    while IFS="$(printf '\t')" read -r stream consumer kind decision approval_ref rest; do
      [ "$decision" = DROP ] || continue
      echo "Applying explicitly reviewed DROP ${stream}/${consumer} (${approval_ref})"
      "$nats_bin" --context "$NATS_TARGET_CONTEXT" consumer rm "$stream" "$consumer" --force
    done < "${evidence}/consumer-decisions.tsv"
    echo 'Restore commands complete. Record target sequences, message hashes, consumer decisions/counts, replay/idempotency proof, then run guard-nats-pvc-migration.sh --acceptance.'
    ;;
  rollback)
    [ "${NATS_ROLLBACK_APPROVED:-false}" = true ] || fail 'rollback requires NATS_ROLLBACK_APPROVED=true after operator approval'
    [ "${NATS_TARGET_QUIESCED:-false}" = true ] || fail 'keep publishers and consumers fenced before rollback'
    [ -n "${NATS_TARGET_CONTEXT:-}" ] || fail 'NATS_TARGET_CONTEXT must name the protected NATS CLI context for the restored original'
    bash "$(dirname "$0")/guard-nats-pvc-migration.sh" --prepare
    kubectl rollout undo "deployment/voice-nats" -n "${VOICE_K8S_NAMESPACE:-voice-staging}"
    kubectl rollout status "deployment/voice-nats" -n "${VOICE_K8S_NAMESPACE:-voice-staging}" --timeout=180s
    source_context="$(awk -F= '$1 == "SOURCE_CONTEXT" {sub(/^[^=]*=/, ""); print; found=1; exit} END {if (!found) exit 1}' "${evidence}/evidence.env")"
    [ "$NATS_TARGET_CONTEXT" = "$source_context" ] || fail 'rollback target context must match the original source context'
    NATS_MIGRATION_EVIDENCE="$evidence" NATS_TARGET_CONTEXT="$NATS_TARGET_CONTEXT" NATS_EXPECTED_CONTEXT="$source_context" NATS_TARGET_QUIESCED=true bash "$0" restore
    echo 'Rollback restore complete. Keep the PVC and all snapshot artifacts until rollback validation is accepted.'
    ;;
  acceptance)
    bash "$(dirname "$0")/guard-nats-pvc-migration.sh" --acceptance
    ;;
  *)
    echo 'Usage: migrate-nats-pvc.sh backup|restore|rollback|acceptance' >&2
    exit 2
    ;;
esac
