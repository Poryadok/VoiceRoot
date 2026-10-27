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
grep -Fq '      nats_probe_only:' <<<"$dispatch" || fail "missing nats_probe_only input"
grep -Fq '        type: boolean' <<<"$dispatch" || fail "probe input must be boolean"
grep -Fq '        default: false' <<<"$dispatch" || fail "probe input must default false"
deploy="$(sed -n '/^  deploy:$/,/^  nats-search-jetstream-probe:$/p' "$WORKFLOW")"
grep -Fq -- "!inputs.accept_provisional_operation && (github.event_name != 'workflow_dispatch' || (inputs.nats_probe_only != true && inputs.validate_app_secret_only != true && inputs.mail_only != true))" <<<"$deploy" \
  || fail "deploy must skip acceptance/diagnostics-only dispatches and preserve workflow_call"
probe="$(sed -n '/^  nats-search-jetstream-probe:$/,/^  validate-app-secret:$/p' "$WORKFLOW")"
[ -n "$probe" ] || fail "diagnostic job missing"
test_bin="$(mktemp -d)"
trap 'rm -rf "$test_bin"' EXIT
SOURCE_STEP="$test_bin/download-source.sh"
awk '
  /^      - name: Download exact staging source archive$/ { step = 1; next }
  step && /^      - / { exit }
  step && /run: .*\|/ { run = 1; next }
  run { sub(/^          /, ""); print }
' "$WORKFLOW" >"$SOURCE_STEP"
[[ -s "$SOURCE_STEP" ]] || fail "inline pre-checkout source acquisition missing"
deploy_source_line="$(printf '%s\n' "$deploy" | grep -nF 'Download exact staging source archive' | cut -d: -f1)"
deploy_checkout_line="$(grep -nF -m 1 'uses: actions/checkout@' <<<"$deploy" | cut -d: -f1 || true)"
[[ -n "$deploy_source_line" && -z "$deploy_checkout_line" ]] || fail "deploy must acquire its source archive without git checkout"
source_line="$(printf '%s\n' "$probe" | grep -nF 'Download exact staging source archive' | cut -d: -f1)"
probe_checkout_line="$(grep -nF -m 1 'uses: actions/checkout@' <<<"$probe" | cut -d: -f1 || true)"
[[ -n "$source_line" && -z "$probe_checkout_line" ]] || fail "probe must acquire its source archive without git checkout"
grep -Fq 'STAGING_SOURCE_SHA: ${{ github.sha }}' <<<"$probe" || fail "probe must use its immutable workflow commit"
grep -Fq 'STAGING_SOURCE_SHA: ${{ inputs.image_tag }}' <<<"$deploy" || fail "deploy must use the selected source image tag"
for requirement in \
  'source_sha" =~ ^[[:xdigit:]]{40}$' \
  'api="https://api.github.com/repos/Poryadok/VoiceRoot"' \
  '"$api/commits/$source_sha"' \
  'https://codeload.github.com/Poryadok/VoiceRoot/tar.gz/$source_sha' \
  'curl --fail --silent --show-error --proto' \
  'tar -tzf' \
  'git hash-object -w --no-filters' \
  'update-index -z --index-info' \
  'write-tree' \
  'STAGING_SOURCE_ARCHIVE=FAIL_TREE_MISMATCH' \
  'GITHUB_WORKSPACE'; do
  grep -Fq "$requirement" "$SOURCE_STEP" || fail "source acquisition missing $requirement"
done
if grep -Fq 'git add' "$SOURCE_STEP"; then fail "tree verification must not run source-configured Git clean filters"; fi
if grep -Eq -- '--location|--proto-redir' "$SOURCE_STEP"; then fail "archive acquisition must remain on its fixed HTTPS host"; fi
if grep -Eqi 'secrets\.GITHUB_TOKEN|Authorization:|github_pat_|gh[pousr]_' "$SOURCE_STEP"; then
  fail "source acquisition must not attach credentials to API/archive redirects"
fi
if grep -Eq 'uses: actions/checkout@' <<<"$deploy"$'\n'"$probe" || grep -Fq 'https://github.com/' "$SOURCE_STEP"; then
  fail "staging jobs must not require blocked github.com git transport"
fi
CONNECTIVITY="$test_bin/check-source-connectivity.sh"
awk '
  /^      - name: Check staging runner can reach GitHub before checkout$/ { step = 1; next }
  step && /^      - / { exit }
  step && /run: \|/ { run = 1; next }
  run { sub(/^          /, ""); print }
' "$WORKFLOW" >"$CONNECTIVITY"
[ -s "$CONNECTIVITY" ] || fail "inline pre-acquisition connectivity script missing"
grep -Fq 'if [[ "$label" == API || "$label" == SOURCE_ARCHIVE ]]' "$CONNECTIVITY" \
  || fail "only the required API and archive routes may gate acquisition"
for endpoint in GITHUB_DEFAULT GITHUB_IPV4 API RAW SOURCE_ARCHIVE; do
  grep -Fq "check_endpoint ${endpoint}" "$CONNECTIVITY" \
    || fail "endpoint ${endpoint} must be checked"
done
grep -Fq 'api.github.com/repos/Poryadok/VoiceRoot/commits/${GITHUB_SHA}' "$CONNECTIVITY" \
  || fail "preflight must test the immutable commit API URL used by acquisition"
grep -Fq 'codeload.github.com/Poryadok/VoiceRoot/tar.gz/${GITHUB_SHA}' "$CONNECTIVITY" \
  || fail "preflight must test the fixed-host immutable archive URL used by acquisition"
grep -Fq 'STAGING_RUNNER_SOURCE_ARCHIVE_CONNECTIVITY=PASS' "$CONNECTIVITY" \
  || fail "archive connectivity success must be visible"
grep -Fq 'STAGING_RUNNER_SOURCE_ARCHIVE_CONNECTIVITY=FAIL' "$CONNECTIVITY" \
  || fail "archive connectivity failure must be visible"
if grep -Eq 'printenv|env \|' "$CONNECTIVITY"; then
  fail "connectivity step must not dump environment or credential values"
fi
cat >"$test_bin/curl" <<'CURL_STUB'
#!/usr/bin/env bash
args=" $* "
if [[ "${CURL_TEST_MODE:-}" == optional-fails && "$args" == *"https://github.com/ "* ]]; then
  echo 'curl: (7) Failed to connect: optional github.com route unavailable' >&2
  exit 7
fi
if [[ "${CURL_TEST_MODE:-}" == required-fails && "$args" == *"codeload.github.com"* ]]; then
  echo 'curl: (28) Operation timed out' >&2
  exit 28
fi
if [[ "${CURL_TEST_MODE:-}" == api-fails && "$args" == *"api.github.com/repos/Poryadok/VoiceRoot/commits/"* ]]; then
  echo 'curl: (28) Operation timed out' >&2
  exit 28
fi
printf '200'
CURL_STUB
chmod +x "$test_bin/curl"
optional_output="$(PATH="$test_bin:$PATH" CURL_TEST_MODE=optional-fails GITHUB_SHA=0123456789012345678901234567890123456789 bash "$CONNECTIVITY")" \
  || fail "blocked github.com must remain informational when the archive endpoint works"
grep -Fq 'STAGING_RUNNER_ENDPOINT_GITHUB_DEFAULT=FAIL_TCP_CONNECT' <<<"$optional_output" \
  || fail "blocked github.com route failure must remain visible"
grep -Fq 'STAGING_RUNNER_SOURCE_ARCHIVE_CONNECTIVITY=PASS' <<<"$optional_output" \
  || fail "reachable immutable archive route must pass"
if PATH="$test_bin:$PATH" CURL_TEST_MODE=required-fails GITHUB_SHA=0123456789012345678901234567890123456789 bash "$CONNECTIVITY" >/dev/null 2>&1; then
  fail "required source archive route failure must block acquisition"
fi
if PATH="$test_bin:$PATH" CURL_TEST_MODE=api-fails GITHUB_SHA=0123456789012345678901234567890123456789 bash "$CONNECTIVITY" >/dev/null 2>&1; then
  fail "required source API route failure must block acquisition"
fi
ci_target="$(make --no-print-directory -n -C "${ROOT}" ci-script-tests)"
grep -Fq 'staging-source-acquisition-workflow_test.sh' <<<"$ci_target" \
  || fail "CI script test target does not run source acquisition fixtures"
grep -Fq "if: github.event_name == 'workflow_dispatch' && inputs.nats_probe_only == true" <<<"$probe" \
  || fail "diagnostic job gate is too broad"
grep -Fq 'timeout-minutes: 15' <<<"$probe" || fail "job timeout missing"
grep -Fq 'kctl() { kubectl --request-timeout=8s "$@"; }' <<<"$probe" || fail "kubectl timeout wrapper missing"
if grep -Eq 'kubectl (get|create|delete|logs) ' <<<"$probe"; then fail "unbounded direct Kubernetes request"; fi
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
  grep -Fq "$required" <<<"$probe" || fail "probe job missing $required"
done
grep -Fq 'app: voice-nats-search-bootstrap' <<<"$probe" \
  || fail "probe label must be admitted on hub client port"
if grep -Fq 'app: voice-search' <<<"$probe"; then
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
  grep -Fq "$forbidden" <<<"$probe" && fail "diagnostic job contains forbidden access/mutation: $forbidden"
done
for raw_output in \
  'echo "$response"' \
  'echo "$probe_output"' \
  'printf "%s" "$response"' \
  'printf "%s" "$stderr"'; do
  grep -Fq "$raw_output" <<<"$probe" && fail "diagnostic job may print raw output: $raw_output"
done
ci_target="$(make --no-print-directory -n -C "${ROOT}" ci-script-tests)"
grep -Fq 'nats-search-jetstream-workflow-contract_test.sh' <<<"$ci_target" \
  || fail "CI script test target does not run workflow contract"
grep -Fq "vars.STAGING_DEPLOY_ENABLED == 'true'" "$CI_WORKFLOW" \
  || fail "automatic staging deployment must remain behind its explicit repository switch"
echo "staging NATS diagnostic workflow contract passed"
