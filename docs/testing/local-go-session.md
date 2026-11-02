# Go Testcontainers session diagnostics

From `src/backend/pkg`, build `go build -o <private-output> ./cmd/ci-go-session`,
then run the executable with `--pid <live-parent-pid>`. It reports that
process's creation time and the session ID predicted for direct children using
the repository's pinned Testcontainers Go 0.35.0 and gopsutil 3.23.12. The
executable rejects missing or replaced versions and unavailable process data.

This is a prediction, not an ownership or cleanup authorization. A consumer
must verify the actual test binary uses the same dependency versions, that the
parent identity remains unchanged, and that the observed Docker resources have
the expected identity. Testcontainers may fall back to a random session when
process discovery fails; nested processes may have different parents. Do not
release a scheduler slot or delete resources based solely on this prediction.
Java Testcontainers uses a different session mechanism.
