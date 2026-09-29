#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/staging/nats-bootstrap-policy.sh
source "${ROOT}/scripts/staging/nats-bootstrap-policy.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }
nats_bootstrap_on_active_pvc clean-install voice-nats-pvc-candidate false || fail 'accepted PVC hub must refresh fixed consumers'
[[ "$(nats_bootstrap_action '' voice-nats false false)" == prepare ]] || fail 'documented direct infra apply must prepare the source-selector PVC candidate'
grep -Fq 'bash scripts/staging/apply-infra.sh' "${ROOT}/deploy/staging/NATS_PVC_MIGRATION.md" || fail 'documented PVC preparation entrypoint changed'
if nats_bootstrap_action '' voice-nats false true >/dev/null; then
  fail 'full deploy must reject the source-selector PVC preparation state'
fi
[[ "$(nats_bootstrap_action clean-install voice-nats-pvc-candidate false true)" == bootstrap ]] || fail 'accepted PVC hub must permit full bootstrap'
nats_acl_proof_valid abc abc || fail 'matching reviewed ACL proof must permit full deploy'
if nats_acl_proof_valid abc stale || nats_acl_proof_valid abc ''; then
  fail 'missing or stale reviewed ACL proof must block full deploy'
fi
for state in 'clean-install voice-nats false' 'clean-install voice-nats-pvc-candidate true' 'missing voice-nats-pvc-candidate false'; do
  read -r marker selector fresh <<<"${state}"
  if nats_bootstrap_on_active_pvc "${marker}" "${selector}" "${fresh}"; then
    fail "unsafe bootstrap state accepted: ${state}"
  fi
done

APPLY="${ROOT}/scripts/staging/apply-infra.sh"
RENDER="${ROOT}/scripts/staging/render-and-apply.sh"
WORKFLOW="${ROOT}/.github/workflows/staging-deploy.yml"
grep -Fq 'nats_bootstrap_action "${clean_install_mode_value}" "${nats_service_selector}" "${fresh_install}"' "${APPLY}" || fail 'ordinary PVC deploy must use bootstrap policy'
grep -Fq 'run_nats_bootstrap_jobs' "${APPLY}" || fail 'accepted PVC must reconcile fixed consumers'
grep -Fq 'VOICE_NATS_FRESH_INSTALL: "false"' "${WORKFLOW}" || fail 'ordinary deploy must preserve NATS data'
infra_line="$(grep -nF 'apply-infra.sh' "${RENDER}" | head -1 | cut -d: -f1)"
app_line="$(grep -nF 'apply-app-manifests.sh' "${RENDER}" | tail -1 | cut -d: -f1)"
[ -n "${infra_line}" ] && [ -n "${app_line}" ] && [ "${infra_line}" -lt "${app_line}" ] || fail 'NATS bootstrap must precede app rollout'
grep -Fq 'consumer social_events rt_realtime1_friend_request social.friend_request _INBOX.voice.realtime1.friend_request' "${ROOT}/deploy/templates/nats-realtime-bootstrap.yaml" || fail 'friend durable is missing from bootstrap template'

PREFLIGHT="${ROOT}/deploy/templates/nats-realtime-permissions-preflight.yaml"
test -f "${PREFLIGHT}" || fail 'Realtime credential preflight Job is missing'
grep -Fq 'run_nats_realtime_preflight' "${APPLY}" || fail 'bootstrap must verify Realtime permissions before app rollout'
grep -Fq 'job/voice-nats-realtime-permissions-preflight' "${APPLY}" || fail 'infra apply must wait for Realtime credential preflight'
grep -Fq 'REALTIME_NATS_FRIEND_REQUEST_PREFLIGHT' "${PREFLIGHT}" || fail 'preflight must execute Realtime bind logic'
grep -Fq 'restartPolicy: Always' "${PREFLIGHT}" || fail 'preflight leaf must be a native Job sidecar'
grep -Fq 'realtime.creds' "${PREFLIGHT}" || fail 'preflight leaf must use Realtime credential'
grep -Fq 'voice-nats-realtime-permissions-preflight' "${ROOT}/deploy/templates/network-policy-nats-hub.yaml" || fail 'preflight leaf must have scoped hub network access'
grep -Fq '[ "${NS}" = voice-staging ]' "${APPLY}" || fail 'preflight must be staging-scoped'
grep -Fq 'kubectl delete job voice-nats-realtime-permissions-preflight' "${APPLY}" || fail 'preflight must replace any stale completed Job'
grep -Fq 'ttlSecondsAfterFinished: 3600' "${PREFLIGHT}" || fail 'preflight Job must clean up after completion'
grep -Fq 'activeDeadlineSeconds: 180' "${PREFLIGHT}" || fail 'preflight Job must have a hard deadline'
preflight_source="$(sed -n '/^func preflightFriendRequestConsumer(/,/^}/p' "${ROOT}/src/backend/realtime/social_events_consumer.go")"
[[ "${preflight_source}" == *'validateRealtimeConsumerConfig('* ]] || fail 'preflight must inspect exact durable configuration'
[[ "${preflight_source}" != *'.Subscribe('* && "${preflight_source}" != *'nats.Bind('* && "${preflight_source}" != *'.Ack('* ]] || fail 'preflight must never bind to or consume from the live durable'
grep -Fq 'set -euo pipefail' "${APPLY}" || fail 'preflight failure must abort infra apply'
grep -Fq 'kubectl wait --for=condition=complete job/voice-nats-realtime-permissions-preflight' "${APPLY}" || fail 'preflight must block app rollout on credential failure'
grep -Fq 'values: [voice-auth, voice-analytics' "${ROOT}/deploy/templates/network-policy-nats-hub.yaml" || fail 'hub leaf network policy is missing'
grep -Fq 'VOICE_NATS_REQUIRE_APP_READY=true bash "${ROOT}/scripts/staging/apply-infra.sh"' "${RENDER}" || fail 'full deploy must require accepted NATS state before app rollout'
grep -Fq 'nats_acl_proof_valid "${acl_intent_sha}" "${VOICE_NATS_ACL_PROOF_SHA:-}"' "${APPLY}" || fail 'full deploy must reject absent or stale ACL activation proof'
grep -Fq 'VOICE_NATS_ACL_PROOF_SHA: ${{ vars.VOICE_NATS_ACL_PROOF_SHA }}' "${WORKFLOW}" || fail 'staging workflow must pass reviewed ACL proof'

echo 'staging NATS bootstrap policy: OK'
