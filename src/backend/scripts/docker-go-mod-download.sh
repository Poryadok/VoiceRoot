#!/bin/sh
# Retry go mod download for flaky Docker build networks (TLS timeouts to proxy.golang.org).
set -e

if [ -n "$1" ]; then
	cd "$1"
fi

for attempt in 1 2 3 4 5; do
	if go mod download; then
		exit 0
	fi
	echo "go mod download attempt ${attempt} failed, retrying in 15s..." >&2
	sleep 15
done

exit 1
