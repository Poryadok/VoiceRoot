#!/usr/bin/env bash
# Compare streams.tsv with a live stream-name list, or assert an empty candidate.
set -euo pipefail
expected_file="${1:-}"
actual_file="${2:-}"
mode="${3:-exact}"
fail() { echo "ERROR: candidate stream inventory: $*" >&2; exit 1; }
[ -f "$expected_file" ] && [ -f "$actual_file" ] || fail 'expected stream TSV and actual-name file are required'
case "$mode" in
  empty)
    [ ! -s "$actual_file" ] || fail 'candidate already contains streams; refusing restore into a stale PVC'
    ;;
  exact)
    expected="$(mktemp)"
    actual="$(mktemp)"
    trap 'rm -f "$expected" "$actual"' EXIT
    awk -F '\t' 'NF {print $1}' "$expected_file" | LC_ALL=C sort -u > "$expected"
    LC_ALL=C sort -u "$actual_file" > "$actual"
    cmp -s "$expected" "$actual" || fail 'live candidate stream set does not exactly match the reviewed source set'
    ;;
  *) fail 'mode must be exact or empty' ;;
esac
