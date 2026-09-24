#!/usr/bin/env bash
# Read-only JetStream identity census. No payloads, credentials, or config values
# are written: only stream/consumer identity and a canonical state/config digest.
set -euo pipefail
context="${1:-}"
output="${2:-}"
nats_bin="${NATS_CLI:-nats}"
jq_bin="${JQ_CLI:-jq}"
fail() { echo "ERROR: source census blocked: $*" >&2; exit 1; }
[ -n "$context" ] && [ -n "$output" ] || fail 'usage: nats-source-census.sh CONTEXT OUTPUT_TSV'
command -v "$nats_bin" >/dev/null 2>&1 || fail "NATS CLI not found: $nats_bin"
command -v "$jq_bin" >/dev/null 2>&1 || fail "jq not found: $jq_bin"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
"$nats_bin" --context "$context" stream ls --json >"$tmp/streams.json"
"$jq_bin" -er '[.streams[]?.config.name] | unique | .[]' "$tmp/streams.json" >"$tmp/streams"
[ "$(wc -l <"$tmp/streams" | tr -d ' ')" -gt 0 ] || fail 'source returned no stream identities'
: >"$tmp/census"
while IFS= read -r stream; do
  [ -n "$stream" ] || continue
  "$nats_bin" --context "$context" consumer ls "$stream" --json >"$tmp/consumers.json"
  "$jq_bin" -er '.consumers[]? | if type == "string" then . else .name end' "$tmp/consumers.json" >"$tmp/consumers"
  while IFS= read -r consumer; do
    [ -n "$consumer" ] || continue
    "$nats_bin" --context "$context" consumer info "$stream" "$consumer" --json >"$tmp/info.json"
    digest="$("$jq_bin" -cS '{config,state}' "$tmp/info.json" | sha256sum | awk '{print $1}')"
    printf '%s\t%s\t%s\n' "$stream" "$consumer" "$digest" >>"$tmp/census"
  done <"$tmp/consumers"
done <"$tmp/streams"
LC_ALL=C sort -u "$tmp/census" >"$output"
[ "$(wc -l <"$output" | tr -d ' ')" -eq "$(wc -l <"$tmp/census" | tr -d ' ')" ] || fail 'duplicate stream/consumer identity returned by source'
sha256sum "$output" | awk '{print $1}'
