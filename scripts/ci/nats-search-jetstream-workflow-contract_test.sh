#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORKFLOW="$ROOT/.github/workflows/staging-deploy.yml"
MAKEFILE="$ROOT/Makefile"
fail() { echo "FAIL: $*" >&2; exit 1; }
[ -f "$WORKFLOW" ] || fail "workflow missing"
[ -f "$MAKEFILE" ] || fail "Makefile missing"
dispatch="$(sed -n '/^  workflow_dispatch:/,/^permissions:/p' "$WORKFLOW")"
printf '%s\n' "$dispatch" | grep -Fq '      nats_probe_only:' || fail "missing nats_probe_only input"
printf '%s\n' "$dispatch" | grep -Fq '        type: boolean' || fail "probe input must be boolean"
printf '%s\n' "$dispatch" | grep -Fq '        default: false' || fail "probe input must default false"
deploy="$(sed -n '/^  deploy:$/,/^  nats-search-jetstream-probe:$/p' "$WORKFLOW")"
printf '%s\n' "$deploy" | grep -Fq "if: github.event_name != 'workflow_dispatch' || inputs.nats_probe_only != true" \
  || fail "deploy must skip diagnostics-only dispatches and preserve workflow_call"
probe="$(sed -n '/^  nats-search-jetstream-probe:$/,$p' "$WORKFLOW")"
[ -n "$probe" ] || fail "diagnostic job missing"
printf '%s\n' "$probe" | grep -Fq "if: github.event_name == 'workflow_dispatch' && inputs.nats_probe_only == true" \
  || fail "diagnostic job gate is too broad"
for required in \
  'environment: staging' \
  'voice-nats-service-credentials' \
  'search.creds' \
  'natsio/nats-box:0.18.0' \
  'voice-nats-hub-ingress' \
  'voice-nats-search-bootstrap' \
  'voice-nats:4222' \
  '$JS.API.CONSUMER.INFO.message_events.search-indexer-message-v1' \
  '_INBOX.voice.search' \
  'ttlSecondsAfterFinished:' \
  'trap ' \
  'JETSTREAM_DISABLED' \
  'PERMISSION_DENIED' \
  'TIMEOUT' \
  'OTHER'; do
  printf '%s\n' "$probe" | grep -Fq "$required" || fail "probe job missing $required"
done
printf '%s\n' "$probe" | grep -Fq 'app: voice-nats-search-bootstrap' \
  || fail "probe label must be admitted on hub client port"
if printf '%s\n' "$probe" | grep -Fq 'app: voice-search'; then
  fail "probe must not match the live Search Service selector"
fi
for forbidden in \
  'STAGING_NATS_SECRETS_B64' \
  'base64 -d' \
  'kubectl get secret -o' \
  'kubectl delete namespace' \
  'kubectl rollout' \
  'kubectl scale' \
  'kubectl apply -f deploy/'; do
  printf '%s\n' "$probe" | grep -Fq "$forbidden" && fail "diagnostic job contains forbidden access/mutation: $forbidden"
done
for raw_output in \
  'echo "$response"' \
  'echo "$probe_output"' \
  'printf "%s" "$response"' \
  'printf "%s" "$stderr"'; do
  printf '%s\n' "$probe" | grep -Fq "$raw_output" && fail "diagnostic job may print raw output: $raw_output"
done
ci_target="$(sed -n '/^ci-script-tests:/,/^[[:alnum:]_.-]*:/p' "$MAKEFILE")"
printf '%s\n' "$ci_target" | grep -Fq 'nats-search-jetstream-workflow-contract_test.sh' \
  || fail "CI script test target does not run workflow contract"
echo "staging NATS diagnostic workflow contract passed"
