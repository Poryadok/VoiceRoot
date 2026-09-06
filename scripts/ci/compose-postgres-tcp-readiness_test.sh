#!/usr/bin/env bash
# Prove Compose's PostgreSQL healthcheck does not report healthy while the
# official image is still running its init-only Unix-socket server.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
FIXTURE="$ROOT/scripts/ci/testdata/compose-postgres-tcp-readiness/01-hold-init.sh"
readonly label_key="voice.t104.postgres-tcp-readiness"
readonly run_id="$(date +%s)-${RANDOM}${RANDOM}"
readonly resource_prefix="voice-t104-pg-readiness-${run_id}"
readonly network="${resource_prefix}-network"
readonly volume="${resource_prefix}-data"
readonly container="${resource_prefix}-postgres"

created_network=false
created_volume=false
created_container=false

cleanup() {
  local status=$?
  set +e
  if [[ "$created_container" == true ]] \
    && [[ "$(docker inspect --format "{{index .Config.Labels \"${label_key}\"}}" "$container" 2>/dev/null)" == "$run_id" ]]; then
    docker rm -f "$container" >/dev/null
  fi
  if [[ "$created_network" == true ]] \
    && [[ "$(docker network inspect --format "{{index .Labels \"${label_key}\"}}" "$network" 2>/dev/null)" == "$run_id" ]]; then
    docker network rm "$network" >/dev/null
  fi
  if [[ "$created_volume" == true ]] \
    && [[ "$(docker volume inspect --format "{{index .Labels \"${label_key}\"}}" "$volume" 2>/dev/null)" == "$run_id" ]]; then
    docker volume rm "$volume" >/dev/null
  fi
  return "$status"
}
trap cleanup EXIT

fail() {
  echo "compose-postgres-tcp-readiness: $*" >&2
  if [[ "$created_container" == true ]]; then
    docker logs --timestamps "$container" >&2 || true
  fi
  exit 1
}

# Git Bash otherwise rewrites container-only paths such as /tmp and /bin/sh
# before docker exec receives them.
docker_exec() {
  MSYS_NO_PATHCONV=1 docker exec "$@"
}

[[ -f "$FIXTURE" ]] || fail "missing held-init fixture"
docker image inspect postgres:16-alpine >/dev/null \
  || fail "postgres:16-alpine is required locally"
docker image inspect ghcr.io/jqlang/jq:1.7 >/dev/null \
  || fail "ghcr.io/jqlang/jq:1.7 is required locally"

# Compose preserves '$$' in rendered JSON. Convert only that Compose escape
# before asking a shell in the container to execute the configured CMD-SHELL.
healthcheck="$(
  cd "$ROOT"
  docker compose config --format json \
    | docker run --rm -i ghcr.io/jqlang/jq:1.7 -er '
        .services.postgres.healthcheck.test
        | if type == "array" and length == 2 and .[0] == "CMD-SHELL"
          then .[1]
          else error("postgres healthcheck must be CMD-SHELL with one command")
          end
      '
)" || fail "could not load the configured postgres healthcheck"
healthcheck="${healthcheck//\$\$/\$}"
[[ -n "$healthcheck" ]] || fail "configured postgres healthcheck is empty"

if command -v cygpath >/dev/null 2>&1; then
  fixture_mount="$(cygpath -am "$FIXTURE")"
else
  fixture_mount="$FIXTURE"
fi

docker network create --label "${label_key}=${run_id}" "$network" >/dev/null
created_network=true
docker volume create --label "${label_key}=${run_id}" "$volume" >/dev/null
created_volume=true
docker run -d --name "$container" --network "$network" \
  --label "${label_key}=${run_id}" \
  -e POSTGRES_USER=voice -e POSTGRES_PASSWORD=voice-t104-test-only -e POSTGRES_DB=voice \
  -v "${volume}:/var/lib/postgresql/data" \
  -v "${fixture_mount}:/docker-entrypoint-initdb.d/01-hold-init.sh:ro" \
  postgres:16-alpine >/dev/null
created_container=true

deadline=$((SECONDS + 60))
until docker_exec "$container" test -f /tmp/voice-t104-init-held >/dev/null 2>&1; do
  (( SECONDS < deadline )) || fail "timed out waiting for the init-only server barrier"
  sleep 1
done

docker_exec "$container" pg_isready -U voice -d voice >/dev/null \
  || fail "expected Unix-socket readiness during held init"
if docker_exec "$container" pg_isready -h 127.0.0.1 -p 5432 -U voice -d voice >/dev/null 2>&1; then
  fail "expected TCP readiness to remain unavailable during held init"
fi
if docker_exec "$container" /bin/sh -ec "$healthcheck" >/dev/null 2>&1; then
  fail "configured healthcheck succeeded during socket-only init; it must require TCP"
fi

docker_exec "$container" touch /tmp/voice-t104-init-release
deadline=$((SECONDS + 60))
until docker_exec "$container" pg_isready -h 127.0.0.1 -p 5432 -U voice -d voice >/dev/null 2>&1 \
  && docker_exec "$container" /bin/sh -ec "$healthcheck" >/dev/null 2>&1; do
  (( SECONDS < deadline )) || fail "timed out waiting for final TCP readiness and configured healthcheck"
  sleep 1
done

echo "compose-postgres-tcp-readiness: PASS"
