package main

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMessagingStartupRejectsMissingAndShortThreadCursorHMAC documents the
// process-start contract. Validation belongs at startup, before a service with
// DATABASE_URL can register ListThreads; RPC-level fallback is not sufficient.
func TestMessagingStartupRejectsMissingAndShortThreadCursorHMAC(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	require.NoError(t, err)
	require.True(t, regexp.MustCompile(`len\(threadCursorSecret\)\s*<\s*32`).Match(raw), "startup must reject absent and fewer-than-32-byte HMAC keys")
	require.True(t, regexp.MustCompile(`MESSAGING_THREAD_CURSOR_HMAC_SECRET must be at least 32 bytes`).Match(raw), "startup error must explain the minimum key length")
}
