#!/usr/bin/env bash
# Apply ClickHouse DDL on production (idempotent IF NOT EXISTS in 001_events.sql).
set -euo pipefail

export VOICE_K8S_NAMESPACE="${VOICE_K8S_NAMESPACE:-voice-prod}"
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
exec "${ROOT}/scripts/staging/apply-clickhouse-init.sh"
