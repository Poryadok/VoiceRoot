#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORKFLOW="$ROOT/.github/workflows/staging-deploy.yml"
CI_WORKFLOW="$ROOT/.github/workflows/ci.yml"
MAKEFILE="$ROOT/Makefile"
fail() { echo "FAIL: $*" >&2; exit 1; }
[ -f "$WORKFLOW" ] || fail "workflow missing"
[ -f "$CI_WORKFLOW" ] || fail "CI workflow missing"
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
connectivity_line="$(printf '%s\n' "$probe" | grep -nF 'https://github.com/' | head -n 1 | cut -d: -f1)"
checkout_line="$(printf '%s\n' "$probe" | grep -nF 'uses: actions/checkout@v4' | head -n 1 | cut -d: -f1)"
[[ -n "$connectivity_line" && -n "$checkout_line" && "$connectivity_line" -lt "$checkout_line" ]] \
  || fail "GitHub connectivity must be checked before checkout on the staging runner"
printf '%s\n' "$probe" | grep -Fq 'STAGING_RUNNER_GITHUB_CONNECTIVITY=PASS' \
  || fail "connectivity success must be visible"
for failure in PROXY_DNS GITHUB_DNS TCP_CONNECT TIMEOUT TLS_OR_CURL; do
  printf '%s\n' "$probe" | grep -Fq "failure=${failure}" \
    || fail "connectivity failure class ${failure} must be classified"
done
printf '%s\n' "$probe" | grep -Fq 'STAGING_RUNNER_GITHUB_CONNECTIVITY=FAIL' \
  || fail "connectivity failure class must be emitted"
for endpoint in GITHUB_DEFAULT GITHUB_IPV4 API RAW CODELOAD; do
  printf '%s\n' "$probe" | grep -Fq "check_endpoint ${endpoint}" \
    || fail "endpoint ${endpoint} must be checked"
done
printf '%s\n' "$probe" | grep -Fq 'STAGING_RUNNER_ENDPOINT_${label}=HTTP_${status}' \
  || fail "endpoint HTTP statuses must be emitted without response bodies"
printf '%s\n' "$probe" | grep -Fq 'STAGING_RUNNER_ENDPOINT_${label}=FAIL_${failure}' \
  || fail "endpoint-specific network failures must have sanitized status"
printf '%s\n' "$probe" | grep -Fq 'api.github.com/repos/Poryadok/VoiceRoot' \
  || fail "preflight must test the GitHub API used by checkout"
printf '%s\n' "$probe" | grep -Fq 'raw.githubusercontent.com/Poryadok/VoiceRoot/${GITHUB_SHA}/.gitignore' \
  || fail "preflight must test pinned raw content availability"
printf '%s\n' "$probe" | grep -Fq 'codeload.github.com/Poryadok/VoiceRoot/tar.gz/${GITHUB_SHA}' \
  || fail "preflight must test pinned archive availability"
if printf '%s\n' "$probe" | grep -Eq 'printenv|env \|'; then
  fail "connectivity step must not dump environment or credential values"
fi
printf '%s\n' "$probe" | grep -Fq "if: github.event_name == 'workflow_dispatch' && inputs.nats_probe_only == true" \
  || fail "diagnostic job gate is too broad"
printf '%s\n' "$probe" | grep -Fq 'timeout-minutes: 8' || fail "job timeout missing"
printf '%s\n' "$probe" | grep -Fq 'kctl() { kubectl --request-timeout=8s "$@"; }' || fail "kubectl timeout wrapper missing"
if printf '%s\n' "$probe" | grep -Eq 'kubectl (get|create|delete|logs) '; then fail "unbounded direct Kubernetes request"; fi
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
grep -Fq "vars.STAGING_DEPLOY_ENABLED == 'true'" "$CI_WORKFLOW" \
  || fail "automatic staging deployment must remain behind its explicit repository switch"
echo "staging NATS diagnostic workflow contract passed"
