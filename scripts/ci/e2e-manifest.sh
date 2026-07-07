#!/usr/bin/env bash
# Read .github/ci/e2e-features.yml (smoke_gateway / smoke_flutter sections).
set -euo pipefail

MANIFEST="${1:?manifest path}"
SECTION="${2:?section: smoke_gateway|smoke_flutter}"

awk -v section="${SECTION}:" '
  $0 == section { in_section=1; next }
  in_section && /^[a-z_]+:/ { exit }
  in_section && /^  - / {
    gsub(/^  - /, "")
    gsub(/#.*/, "")
    gsub(/^[ \t]+|[ \t]+$/, "")
    if (length($0) > 0) print $0
  }
' "${MANIFEST}"
