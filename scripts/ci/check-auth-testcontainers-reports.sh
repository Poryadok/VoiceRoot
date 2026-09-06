#!/usr/bin/env bash
# Fail closed when Maven reports that Auth Testcontainers suites did not run.
set -euo pipefail

reports_dir="${1:-}"

if [[ -z "${reports_dir}" || ! -d "${reports_dir}" ]]; then
  echo "usage: $0 <surefire-reports-directory>" >&2
  exit 2
fi

required_suites=(
  "voice.backend.auth.AccountDeletionEpochFloorJdbcIntegrationTest"
  "voice.backend.auth.AccountDeletionOperationRepositoryJdbcIntegrationTest"
  "voice.backend.auth.AuthJdbcRedisIntegrationTest"
  "voice.backend.auth.E2EKeyBackupJdbcIntegrationTest"
  "voice.backend.auth.RegistrationSessionEpochJdbcIntegrationTest"
  "voice.backend.auth.GuestAccountSweeperJdbcIntegrationTest"
  "voice.backend.auth.GuestConversionDurabilityJdbcIntegrationTest"
  "voice.backend.auth.GuestConversionLocalPromotionJdbcIntegrationTest"
  "voice.backend.auth.GuestConversionOperationRepositoryJdbcIntegrationTest"
  "voice.backend.auth.GuestConversionOtpAcceptanceJdbcIntegrationTest"
  "voice.backend.auth.ProfilesVerificationIntegrationTest"
  "voice.backend.auth.SessionEpochRepositoryJdbcIntegrationTest"
  "voice.backend.auth.SetAccountStatusIntegrationTest"
  "voice.backend.auth.service.RedisAccountRestoreTokenStoreIntegrationTest"
  "voice.backend.auth.sessionepoch.RedisSessionEpochFloorStoreIntegrationTest"
  "voice.backend.auth.sessionepoch.SessionEpochFloorStartupLifecycleJdbcIntegrationTest"
  "voice.backend.auth.oauth.RedisOAuthAuthorizationCodeStoreIntegrationTest"
)

attribute_value() {
  local report_line="$1"
  local attribute="$2"
  sed -nE "s/.*[[:space:]]${attribute}=\"([0-9]+)\".*/\1/p" <<<"${report_line}"
}

failed=0

for suite in "${required_suites[@]}"; do
  report="${reports_dir}/TEST-${suite}.xml"
  if [[ ! -f "${report}" ]]; then
    echo "missing required Testcontainers report: ${report}" >&2
    failed=1
    continue
  fi

  suite_line="$(grep -m 1 '<testsuite ' "${report}" || true)"
  if [[ "${suite_line}" != *"name=\"${suite}\""* ]]; then
    echo "invalid Testcontainers suite name in ${report}: expected ${suite}" >&2
    failed=1
    continue
  fi

  tests="$(attribute_value "${suite_line}" tests)"
  skipped="$(attribute_value "${suite_line}" skipped)"
  failures="$(attribute_value "${suite_line}" failures)"
  errors="$(attribute_value "${suite_line}" errors)"

  if [[ -z "${tests}" || -z "${skipped}" || -z "${failures}" || -z "${errors}" ]]; then
    echo "missing numeric test counters in ${report}" >&2
    failed=1
    continue
  fi

  if (( tests <= 0 || skipped != 0 || failures != 0 || errors != 0 )); then
    echo "invalid Testcontainers result in ${report}: tests=${tests}, skipped=${skipped}, failures=${failures}, errors=${errors}" >&2
    failed=1
  fi
done

if (( failed != 0 )); then
  exit 1
fi

echo "Auth Testcontainers Surefire reports passed."
