#!/usr/bin/env bash
# Collect test coverage summary for Go backend modules, Auth (JaCoCo), and Flutter (lcov).
# Output: .local/coverage/summary.txt (and per-module Go profiles when merge succeeds).
# Requires: Go, Maven/Java, Flutter SDK, Docker daemon (Go integration tests).
set -euo pipefail

ROOT="${1:-}"
if [[ -z "${ROOT}" ]]; then
  ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
fi

OUT="${ROOT}/.local/coverage"
mkdir -p "${OUT}"

GO_SERVICES=(
  pkg analytics bot chat federation file gateway matchmaking messaging moderation
  notification realtime role search social space story subscription user voice
)

SUMMARY="${OUT}/summary.txt"
: >"${SUMMARY}"

log() {
  echo "$*" | tee -a "${SUMMARY}"
}

pct() {
  local covered="$1"
  local total="$2"
  if [[ "${total}" -eq 0 ]]; then
    echo "N/A"
    return
  fi
  awk -v c="${covered}" -v t="${total}" 'BEGIN { printf "%.1f%%", 100 * c / t }'
}

go_module_coverage() {
  local svc="$1"
  local mod_dir="${ROOT}/src/backend/${svc}"
  local mod_name
  mod_name="$(awk '/^module / { print $2; exit }' "${mod_dir}/go.mod")"
  local profile="${OUT}/go-${svc}.out"

  (
    cd "${mod_dir}"
    export CGO_ENABLED=0
    if ! go test ./... -count=1 -coverprofile="${profile}" -coverpkg="${mod_name}/..." >/dev/null 2>&1; then
      echo "FAIL"
      return 1
    fi
    if [[ ! -s "${profile}" ]]; then
      echo "N/A"
      return 0
    fi
    go tool cover -func "${profile}" | awk '/^total:/ { print $3; exit }'
  )
}

auth_jacoco_coverage() {
  local jacoco_xml="${ROOT}/src/backend/auth/target/site/jacoco/jacoco.xml"
  if [[ ! -f "${jacoco_xml}" ]]; then
    echo "N/A"
    return 0
  fi
  local line missed covered
  line="$(grep -oE '<counter type="LINE" missed="[0-9]+" covered="[0-9]+"' "${jacoco_xml}" | tail -1)"
  if [[ -z "${line}" ]]; then
    echo "N/A"
    return 0
  fi
  missed="$(sed -n 's/.*missed="\([0-9]*\)".*/\1/p' <<<"${line}")"
  covered="$(sed -n 's/.*covered="\([0-9]*\)".*/\1/p' <<<"${line}")"
  pct "${covered}" "$((missed + covered))"
}

flutter_lcov_coverage() {
  local lcov="${ROOT}/src/frontend/coverage/lcov.info"
  if [[ ! -f "${lcov}" ]]; then
    echo "N/A"
    return 0
  fi
  local lf=0 lh=0
  while IFS= read -r line; do
    case "${line}" in
      LF:*) lf=$((lf + ${line#LF:})) ;;
      LH:*) lh=$((lh + ${line#LH:})) ;;
    esac
  done <"${lcov}"
  pct "${lh}" "${lf}"
}

log "Voice coverage report"
log "Generated: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
log ""

log "== Go backend =="
go_failed=0
for svc in "${GO_SERVICES[@]}"; do
  printf "  %-14s " "${svc}" | tee -a "${SUMMARY}"
  if cov="$(go_module_coverage "${svc}")"; then
    printf "%s\n" "${cov}" | tee -a "${SUMMARY}"
  else
    printf "FAIL\n" | tee -a "${SUMMARY}"
    go_failed=1
  fi
done
log ""

log "== Auth (Java / JaCoCo) =="
printf "  %-14s " "auth" | tee -a "${SUMMARY}"
(
  cd "${ROOT}/src/backend/auth"
  mvn -B -q test >/dev/null 2>&1
)
auth_cov="$(auth_jacoco_coverage)"
printf "%s\n" "${auth_cov}" | tee -a "${SUMMARY}"
log "  HTML report: src/backend/auth/target/site/jacoco/index.html"
log ""

log "== Flutter =="
printf "  %-14s " "frontend" | tee -a "${SUMMARY}"
(
  cd "${ROOT}/src/frontend"
  flutter pub get >/dev/null
  flutter test --coverage >/dev/null
)
flutter_cov="$(flutter_lcov_coverage)"
printf "%s\n" "${flutter_cov}" | tee -a "${SUMMARY}"
log "  lcov: src/frontend/coverage/lcov.info"
log ""

log "Full summary: ${SUMMARY}"

if [[ "${go_failed}" -ne 0 ]]; then
  exit 1
fi
