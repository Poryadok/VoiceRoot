#!/bin/sh
set -eu

template="${NATS_ANALYTICS_CHAT_BOOTSTRAP_TEMPLATE:?NATS_ANALYTICS_CHAT_BOOTSTRAP_TEMPLATE is required}"

# Compose executes the exact ConfigMap script used by Kubernetes rather than a
# second handwritten consumer contract. The source file is repository-owned and
# mounted read-only; the extracted script never contains credentials.
sed -n '/^    #!\/bin\/sh$/,/^---$/ { /^---$/d; s/^    //; p }' "$template" | /bin/sh
