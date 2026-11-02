#!/usr/bin/env bash
# Root here belongs only to a disposable fixture container. No broker, host
# socket, staging connection, or production installer is invoked by these tests.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# These three suites create isolated PostgreSQL containers through the host
# Docker CLI. Run them as root on an immutable private source snapshot; never
# expose the host Docker socket to the untrusted inner fixture container.
[[ "$(uname -s)" == Linux ]] || { echo 'STAGING_PG_TESTS_REQUIRE_LINUX_HOST' >&2; exit 1; }
command -v docker >/dev/null || { echo 'STAGING_PG_TESTS_REQUIRE_DOCKER' >&2; exit 1; }
sudo -n true || { echo 'STAGING_PG_TESTS_REQUIRE_NONINTERACTIVE_SUDO' >&2; exit 1; }
[[ -z "$(git -C "$ROOT" status --porcelain --untracked-files=normal)" ]] || {
  echo 'STAGING_TEST_SOURCE_MUST_BE_CLEAN' >&2; exit 1;
}
pg_fixture="$(sudo -n mktemp -d /var/tmp/voice-staging-pg-ci.XXXXXXXX)"
case "$pg_fixture" in /var/tmp/voice-staging-pg-ci.*) ;; *) echo 'STAGING_PG_FIXTURE_INVALID' >&2; exit 1;; esac
qualified_fixture="$(mktemp -d /tmp/voice-staging-qualified.XXXXXXXX)"
case "$qualified_fixture" in /tmp/voice-staging-qualified.*) ;; *) echo 'STAGING_QUALIFIED_FIXTURE_INVALID' >&2; exit 1;; esac
cleanup_pg_fixture() { sudo -n rm -rf -- "$pg_fixture"; rm -rf -- "$qualified_fixture"; }
trap cleanup_pg_fixture EXIT
git -C "$ROOT" archive HEAD | sudo -n tar -xf - -C "$pg_fixture"
sudo -n chmod -R a-w "$pg_fixture"
pg_tmp="$(sudo -n mktemp -d "$pg_fixture/.ci-pg.XXXXXXXX")"
sudo -n sh -eu -c '
  cd "$1/scripts/staging/nats-rollout-preservation"
  command -v docker >/dev/null || { echo STAGING_PG_TESTS_REQUIRE_DOCKER >&2; exit 1; }
  export PYTHONPATH="$1/scripts/dev:$1/scripts/staging/nats-known-baseline:$1/scripts/staging/nats-rollout-preservation"
  export TMPDIR="$2"
  python3 -B -m unittest realtime_bootstrap_pg_test realtime_bootstrap_composition_test realtime_bootstrap_existing_pg_test
' sh "$pg_fixture" "$pg_tmp"
# Exercise the revision 31 -> 32 verifier on a separate, network-isolated
# disposable database. The container ID belongs only to this invocation.
sudo -n sh -eu -c '
  cidfile="$2/gis-rev32.cid"
  cleanup() {
    status=$?
    trap - EXIT
    if [ -s "$cidfile" ]; then
      id=$(cat "$cidfile")
      docker rm -f -v "$id" >/dev/null || status=1
    fi
    exit "$status"
  }
  trap cleanup EXIT
  docker run -d --cidfile "$cidfile" --network none \
    -e POSTGRES_PASSWORD=fixture-password postgres:16-alpine >/dev/null
  id=$(cat "$cidfile")
  ready=0
  for n in $(seq 1 60); do
    if docker exec "$id" pg_isready -U postgres >/dev/null 2>&1; then ready=1; break; fi
    sleep 1
  done
  [ "$ready" = 1 ] || { echo GIS_REV32_POSTGRES_NOT_READY >&2; exit 1; }
  docker exec "$id" mkdir /fixture
  docker cp "$1/src/backend/migrations/game_integration_db" "$id:/fixture/migrations"
  docker cp "$1/docker/postgres/provision-gameintegration-runtime-role.sql" "$id:/fixture/provision.sql"
  docker cp "$1/docker/postgres/verify-gameintegration-runtime-role.sql" "$id:/fixture/verify.sql"
  docker cp "$1/scripts/dev/gis_rev32_verifier_test.sh" "$id:/fixture/test.sh"
  docker exec -e GIS_REV32_FIXTURE=/fixture "$id" sh /fixture/test.sh
' sh "$pg_fixture" "$pg_tmp"
# Build exact qualified helpers before the isolated snapshot loses Git history.
python3 -B "$ROOT/scripts/ci/staging_story_target8_helper_test.py"
python3 "$ROOT/scripts/ci/staging_qualified_helpers.py" --root "$ROOT" --output "$qualified_fixture/binaries"
python3 "$ROOT/scripts/ci/staging_story_target8_helper.py" --root "$ROOT" --output "$qualified_fixture/target8"
# The pinned official Go toolchain supplies mandatory server-helper parity;
# the Python fixture image intentionally has no Go installation.
docker run --rm --network none --entrypoint tar \
  golang@sha256:8ac98ca534ac3f51e1f420a1dd2c15e74c75cfa0f23f3ad27eb5d7236c349a0c \
  -C /usr/local -cf - go | docker run --rm -i --pids-limit 128 --memory 2g \
  --mount "type=bind,source=${pg_fixture},target=/work,readonly" \
  --mount "type=bind,source=${qualified_fixture}/binaries,target=/qualified,readonly" \
  --mount "type=bind,source=${qualified_fixture}/target8,target=/qualified-target8,readonly" \
  -e PYTHONPATH=/tmp/repo/scripts/dev:/tmp/repo/scripts/staging/nats-known-baseline:/tmp/repo/scripts/staging/nats-rollout-preservation \
  python@sha256:1b668429b3511ab407d8e00648891631b0b1a4d7e15e3ca70f38ab5b91ad4ab4 \
  sh -eu -c '
    tar -xf - -C /usr/local
    export PATH=/usr/local/go/bin:$PATH GOMAXPROCS=2 GOFLAGS=-p=2
    mkdir -p /tmp/repo/src/backend/auth/src/main/resources/db /tmp/repo/docker/clickhouse /etc/systemd/system
    mkdir -m 700 /fixture
    export TMPDIR=/fixture
    mkdir -m 700 /fixture/qualified-binaries
    cp /qualified/kernel /qualified/bootstrap-renewer /qualified/story-current4-probe /qualified/provenance.json /fixture/qualified-binaries/
    cp /qualified-target8/story-restored8-probe /fixture/qualified-binaries/
    cp /qualified-target8/provenance.json /fixture/qualified-binaries/target8-provenance.json
    chmod 400 /fixture/qualified-binaries/*
    cp -R /work/scripts /work/deploy /work/.github /tmp/repo/
    cp /work/docker-compose.voice-space-media.yml /tmp/repo/
    cp -R /work/src/backend/migrations /tmp/repo/src/backend/
    cp -R /work/src/backend/auth/src/main/resources/db/migration /tmp/repo/src/backend/auth/src/main/resources/db/
    cp -R /work/docker/clickhouse/init /tmp/repo/docker/clickhouse/
    mkdir -p /tmp/repo/docker/postgres
    cp /work/docker/postgres/provision-gameintegration-runtime-role.sql /work/docker/postgres/verify-gameintegration-runtime-role.sql /tmp/repo/docker/postgres/
    apk add --no-cache bash git jq openssl >/dev/null
    # RootJournal exercises the real operator-group ownership contract here.
    addgroup -S pmd
    ln -s /usr/local/bin/python3 /usr/bin/python3
    python -m pip install --quiet --root-user-action=ignore --disable-pip-version-check PyYAML==6.0.3
    cd /tmp/repo/scripts/staging/nats-rollout-preservation
    set --
    for path in *_test.py; do
      case "$path" in
        realtime_bootstrap_pg_test.py|realtime_bootstrap_composition_test.py|realtime_bootstrap_existing_pg_test.py) continue ;;
      esac
      set -- "$@" "${path%.py}"
    done
    python -m unittest "$@"
    cd /tmp/repo/scripts/deploy
    python -m unittest staging_p3_auxiliary_image_test
    cd /tmp/repo/scripts/staging/nats-known-baseline
    python -m unittest controller_test root_main_test docker_runtime_test stage_runtime_test
    bash /tmp/repo/scripts/staging/nats-rollout-preservation_test.sh
  '
