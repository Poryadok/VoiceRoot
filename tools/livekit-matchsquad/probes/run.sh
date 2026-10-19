#!/bin/sh
set -eu
# Component-only probe: stock service_test.TestMain creates Docker containers
# with host ports. Select the actual build's production files and this probe,
# without invoking that unrelated integration harness or mounting a Docker socket.
files=$(go list -f '{{range .GoFiles}}{{printf "pkg/service/%s " .}}{{end}}' ./pkg/service)
# Pinned upstream filenames are trusted fixed source inputs without whitespace.
# shellcheck disable=SC2086
go test -race $files pkg/service/voice_missing_expiry_test.go -run TestVoice -count=1 -v
