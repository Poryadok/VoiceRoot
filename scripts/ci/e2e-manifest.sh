#!/usr/bin/env bash
# Read .github/ci/e2e-features.yml (smoke_gateway / smoke_flutter / full_flutter / restart_proof_gateway / a1_multi_account_gateway / a1_flutter_profile_handoff sections).
set -euo pipefail

MANIFEST="${1:?manifest path}"
SECTION="${2:?section: smoke_gateway|smoke_flutter|full_flutter|restart_proof_gateway|a1_multi_account_gateway|a1_flutter_profile_handoff}"

case "${SECTION}" in
  smoke_gateway|smoke_flutter|full_flutter|restart_proof_gateway|a1_multi_account_gateway|a1_flutter_profile_handoff) ;;
  *)
    echo "ERROR: unknown section ${SECTION} (expected smoke_gateway, smoke_flutter, full_flutter, restart_proof_gateway, a1_multi_account_gateway, or a1_flutter_profile_handoff)" >&2
    exit 1
    ;;
esac

if [ ! -f "${MANIFEST}" ]; then
  echo "ERROR: manifest not found: ${MANIFEST}" >&2
  exit 1
fi

in_section=0
found_section=0
while IFS= read -r line || [ -n "${line}" ]; do
  # Strip trailing comments; keep leading whitespace for structure checks.
  line="${line%%#*}"
  trimmed="$(printf '%s' "${line}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  if [ -z "${trimmed}" ]; then
    continue
  fi
  if [ "${trimmed}" = "${SECTION}:" ]; then
    in_section=1
    found_section=1
    continue
  fi
  if [ "${in_section}" -eq 1 ]; then
    if printf '%s' "${trimmed}" | grep -Eq '^[a-z_]+:$'; then
      break
    fi
    if [[ "${trimmed}" == -* ]]; then
      entry="$(printf '%s' "${trimmed}" | sed 's/^- //;s/[[:space:]]*$//')"
      if [ -n "${entry}" ]; then
        printf '%s\n' "${entry}"
      fi
    fi
  fi
done < "${MANIFEST}"

if [ "${found_section}" -eq 0 ]; then
  echo "ERROR: section ${SECTION} not found in ${MANIFEST}" >&2
  exit 1
fi
