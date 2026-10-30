#!/usr/bin/env bash
# Synthetic authority/transport, real encrypted PostgreSQL dump and restore.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
[[ "$(docker info --format '{{.OSType}}')" == linux ]]

# Fresh runners must materialize the exact linux/amd64 image IDs in the fixture;
# a local image cache is not a dependency declaration.
postgres='postgres@sha256:cf78e76683b9ca8c5733cbbdce6c9262b45b6767934dd0a95e671f9a0fc20685'
docker pull --platform linux/amd64 "$postgres"
[[ "$(docker image inspect "$postgres" --format '{{.Id}}')" == sha256:75f5a96988cdf694a215073c3e9c001b706b371e2f94df3967f2efdec2787f6b ]]

scratch="$(mktemp -d)"
trap 'rm -rf -- "$scratch"' EXIT
# This image copies only from another image; send no repository build context.
docker build --platform linux/amd64 --iidfile "$scratch/runner" - \
  < "$ROOT/scripts/ci/story-media-cohort-chain.Dockerfile"
runner="$(cat "$scratch/runner")"
# The test verifies and removes only its randomly labelled containers/volumes.
# /repo is the original fixture contract; no environment credentials are passed.
docker run --rm --platform linux/amd64 --network none --read-only \
  --tmpfs /tmp:rw,nosuid,nodev,size=268435456 \
  -v /var/run/docker.sock:/var/run/docker.sock -v "$ROOT:/repo:ro" \
  -w /repo/scripts/dev "$runner" \
  python3 -B -m unittest -v story_media_backup_cohort_chain_test
