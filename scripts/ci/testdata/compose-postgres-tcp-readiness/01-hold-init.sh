#!/bin/sh
# The official postgres entrypoint has started its init-only Unix-socket server
# before it sources this file. Keep that server alive until the harness samples
# both socket and TCP readiness, then releases it for the final TCP server.
set -eu

touch /tmp/voice-t104-init-held
while [ ! -f /tmp/voice-t104-init-release ]; do
  sleep 1
done
