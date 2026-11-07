#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd)"
scratch="$(mktemp -d)"
trap 'rm -f "$scratch/docker" "$scratch/output" "$scratch/docker-calls"; rmdir "$scratch"' EXIT
cat > "$scratch/docker" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "invoked" >> "$DOCKER_CALL_LOG"
exit 1
EOF
chmod +x "$scratch/docker"
export DOCKER_CALL_LOG="$scratch/docker-calls"
if PATH="$scratch:$PATH" bash "$root/scripts/ci/testcontainers-prune.sh" >"$scratch/output" 2>&1; then
  echo 'unscoped pruning must fail closed' >&2
  exit 1
fi
if [[ -e "$DOCKER_CALL_LOG" ]]; then
  echo 'unscoped pruning must not contact Docker' >&2
  exit 1
fi
for target in backend-test-ci backend-test-ci-short; do
  line="$(grep -E "^${target}:" "$root/Makefile")"
  if [[ "$line" == *testcontainers-prune* ]]; then
    echo "$target must not invoke global pruning" >&2
    exit 1
  fi
done
echo 'unscoped Testcontainers cleanup fails closed'
