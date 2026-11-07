#!/usr/bin/env bash
# Standalone key-generation qualification only; not the outer broker integration.
# No private files leave the container writable layer. If this caller receives
# SIGKILL, each container's timeout with kill-after bounds its command and Docker
# --rm removes the layer when the daemon completes cleanup. Cidfiles in the
# printed metadata directory permit reconciliation if the daemon is unavailable;
# its volume stores Go dependencies only. Never remove an unverified resource.
set -euo pipefail
repo=$(cd "$(dirname "$0")/../.." && pwd -P)
meta=$(mktemp -d "${TMPDIR:-/tmp}/voice-owned-genesis.XXXXXXXX")
token=$(basename "$meta" | tr '.' '-')
volume="$token-modcache"
go_image='golang@sha256:8ac98ca534ac3f51e1f420a1dd2c15e74c75cfa0f23f3ad27eb5d7236c349a0c'
cidfile="$meta/container.id"
fetch_cidfile="$meta/fetch.id"
mount_repo="$repo"
docker_cidfile="$cidfile"
docker_fetch_cidfile="$fetch_cidfile"
if command -v cygpath >/dev/null 2>&1; then
    mount_repo=$(cygpath -m "$repo")
    docker_cidfile=$(cygpath -m "$cidfile")
    docker_fetch_cidfile=$(cygpath -m "$fetch_cidfile")
    export MSYS_NO_PATHCONV=1
fi
created_volume=0
cleanup() {
    result=$?
    trap - EXIT
    for receipt in "$fetch_cidfile" "$cidfile"; do
        [[ -s "$receipt" ]] || continue
        cid=$(cat "$receipt")
        if [[ ! "$cid" =~ ^[a-f0-9]{64}$ ]]; then echo "invalid container receipt; reconcile $receipt" >&2; exit 1; fi
        if ! present=$(docker ps -aq --no-trunc --filter "id=$cid"); then echo "daemon unavailable; reconcile $meta" >&2; exit 1; fi
        if [[ -n "$present" ]]; then
            if [[ "$present" != "$cid" ]]; then echo "ambiguous container; reconcile $receipt" >&2; exit 1; fi
            label=$(docker inspect --format '{{ index .Config.Labels "voice.fixture.owner" }}' "$cid") || { echo "inspect failed; reconcile $meta" >&2; exit 1; }
            if [[ "$label" == "$token" ]]; then docker rm -f "$cid" >/dev/null; else echo "foreign container; reconcile $receipt" >&2; exit 1; fi
            if ! present=$(docker ps -aq --no-trunc --filter "id=$cid") || [[ -n "$present" ]]; then echo "container cleanup unconfirmed; reconcile $meta" >&2; exit 1; fi
        fi
    done
    if [[ "$created_volume" == 1 ]]; then
        label=$(docker volume inspect --format '{{ index .Labels "voice.fixture.owner" }}' "$volume") || { echo "daemon or volume inspection failed; reconcile $meta" >&2; exit 1; }
        if [[ "$label" != "$token" ]]; then echo "foreign dependency volume; reconcile $volume" >&2; exit 1; fi
        docker volume rm "$volume" >/dev/null || { echo "reconcile dependency volume $volume" >&2; exit 1; }
    fi
    rm -f -- "$cidfile" "$fetch_cidfile"
    rmdir -- "$meta"
    exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
echo "owned fixture metadata: $meta"
if docker volume inspect "$volume" >/dev/null 2>&1; then echo "dependency volume already exists" >&2; exit 1; fi
docker volume create --label "voice.fixture.owner=$token" "$volume" >/dev/null
created_volume=1
# Download only dependencies before any fixture key exists.
docker run --rm --cidfile "$docker_fetch_cidfile" --label "voice.fixture.owner=$token" \
    -v "$volume:/go/pkg/mod" -v "$mount_repo:/workspace:ro" -w /workspace/src/backend/pkg \
    "$go_image" timeout -k 10s 120s go mod download github.com/nats-io/jwt/v2 github.com/nats-io/nkeys golang.org/x/crypto golang.org/x/sys gopkg.in/yaml.v3
docker run --rm --cidfile "$docker_cidfile" --label "voice.fixture.owner=$token" --network none -e TMPDIR=/root \
    -v "$volume:/go/pkg/mod:ro" -v "$mount_repo:/workspace:ro" -w /workspace/src/backend/pkg \
    "$go_image" timeout -k 10s 120s go test /workspace/scripts/ci/nats-owned-genesis/main.go /workspace/scripts/ci/nats-owned-genesis/main_test.go
