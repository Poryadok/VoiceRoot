#!/usr/bin/env bash
# Root here belongs only to a disposable fixture container. No broker, host
# socket, staging connection, or production installer is invoked by these tests.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
docker run --rm --pids-limit 128 --memory 2g \
  --mount "type=bind,source=${ROOT},target=/work,readonly" \
  -e PYTHONPATH=/tmp/repo/scripts/staging/nats-known-baseline \
  python@sha256:1b668429b3511ab407d8e00648891631b0b1a4d7e15e3ca70f38ab5b91ad4ab4 \
  sh -eu -c '
    mkdir -p /tmp/repo/src/backend/auth/src/main/resources/db /tmp/repo/docker/clickhouse /etc/systemd/system
    cp -R /work/scripts /work/deploy /work/.github /tmp/repo/
    cp -R /work/src/backend/migrations /tmp/repo/src/backend/
    cp -R /work/src/backend/auth/src/main/resources/db/migration /tmp/repo/src/backend/auth/src/main/resources/db/
    cp -R /work/docker/clickhouse/init /tmp/repo/docker/clickhouse/
    apk add --no-cache bash openssl >/dev/null
    ln -s /usr/local/bin/python3 /usr/bin/python3
    python -m pip install --quiet --root-user-action=ignore --disable-pip-version-check PyYAML==6.0.3
    cd /tmp/repo/scripts/staging/nats-rollout-preservation
    python -m unittest discover -p "*_test.py"
    cd ../nats-known-baseline
    python -m unittest controller_test root_main_test docker_runtime_test stage_runtime_test
    bash /tmp/repo/scripts/staging/nats-rollout-preservation_test.sh
  '
