#!/bin/sh
set -eu

template="${NATS_ANALYTICS_CHAT_BOOTSTRAP_TEMPLATE:?NATS_ANALYTICS_CHAT_BOOTSTRAP_TEMPLATE is required}"

# Compose executes the exact ConfigMap script used by Kubernetes rather than a
# second handwritten consumer contract. Materialize it before invoking sh: the
# bootstrap uses `nats req`, whose client may read stdin for a reply body. A
# pipeline would let that client consume later shell source bytes and silently
# alter the contract. The source file is repository-owned and mounted read-only;
# the extracted script never contains credentials.
script="$(mktemp "${TMPDIR:-/tmp}/nats-analytics-chat-bootstrap.XXXXXX")"
trap 'rm -f "$script"' EXIT HUP INT TERM
sed -n '/^    #!\/bin\/sh$/,/^---$/ { /^---$/d; s/^    //; p }' "$template" >"$script"
test -s "$script"
/bin/sh "$script" </dev/null
