#!/usr/bin/env bash
# Global Testcontainers labels do not identify a single local run. Refuse to
# delete resources until the run-owned cleanup mechanism is available.
set -euo pipefail
echo 'testcontainers-prune: refusing unscoped cleanup; use run-owned cleanup' >&2
exit 2
