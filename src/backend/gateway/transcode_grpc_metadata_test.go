package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGRPCMetadataFromRequestForwardsAuthenticatedSessionEpoch(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Voice-User-Id", "account-1")
	req.Header.Set("X-Voice-Profile-Id", "profile-1")
	req.Header.Set("X-Voice-Session-Epoch", "17")
	md := grpcMetadataFromRequest(req)

	require.Equal(t, []string{"account-1"}, md.Get("x-voice-user-id"))
	require.Equal(t, []string{"profile-1"}, md.Get("x-voice-profile-id"))
	require.Equal(t, []string{"17"}, md.Get("x-voice-session-epoch"))
}
