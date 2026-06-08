#!/usr/bin/env bash
# List Go functions below coverage threshold from .local/coverage/go-*.out profiles.
set -euo pipefail

ROOT="${1:-}"
THRESH="${2:-100}"
if [[ -z "${ROOT}" ]]; then
  ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
fi

OUT="${ROOT}/.local/coverage"
mkdir -p "${OUT}"

GO_SERVICES=(
  pkg analytics bot chat federation file gateway matchmaking messaging moderation
  notification realtime role search social space story subscription user voice
)

report="${OUT}/uncovered-functions.txt"
: >"${report}"

for svc in "${GO_SERVICES[@]}"; do
  prof="${OUT}/go-${svc}.out"
  mod_dir="${ROOT}/src/backend/${svc}"
  if [[ ! -s "${prof}" ]]; then
    mod_name="$(awk '/^module / { print $2; exit }' "${mod_dir}/go.mod")"
    (
      cd "${mod_dir}"
      export CGO_ENABLED=0
      go test ./... -count=1 -coverprofile="${prof}" -coverpkg="${mod_name}/..." >/dev/null 2>&1
    ) || {
      echo "=== ${svc} (TEST FAIL) ===" >>"${report}"
      continue
    }
  fi
  echo "=== ${svc} ===" >>"${report}"
  (
    cd "${mod_dir}"
    go tool cover -func "${prof}" | awk -v t="${THRESH}" '
      /^total:/ { next }
      {
        fn = $1
        pct = $3
        gsub(/%/, "", pct)
        if (pct + 0 < t + 0) print pct "%", fn
      }
    ' | sort -n >>"${report}"
  ) || echo "(cover parse failed)" >>"${report}"
  echo "" >>"${report}"
done

echo "Wrote ${report}"
