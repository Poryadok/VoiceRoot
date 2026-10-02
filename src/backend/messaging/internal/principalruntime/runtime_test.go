package principalruntime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModerationPrincipalRuntimeRequiresJWKSAndReplayConfiguration(t *testing.T) {
	_, err := New(context.Background(), Config{})
	require.Error(t, err)
	runtime := &Runtime{}
	_, err = runtime.Verify(context.Background(), "token", "/voice.messaging.v1.MessagingService/TombstoneGameMessage", "request", "hash")
	require.Error(t, err)
}

func TestPrincipalJWKSURLIsPinnedByIssuer(t *testing.T) {
	for _, test := range []struct {
		issuer string
		url    string
		valid  bool
	}{
		{"moderation", "https://moderation/.well-known/principal-jwks.json", true},
		{"gateway", "https://gateway/.well-known/principal-jwks.json", true},
		{"bot", "https://bot/.well-known/principal-jwks.json", true},
		{"gameintegration", "https://gameintegration/internal/v1/principal/jwks.json", true},
		{"chat", "https://chat/.well-known/principal-jwks.json", true},
		{"space", "https://space/.well-known/jwks.json", true},
		{"space", "https://space/.well-known/principal-jwks.json", false},
		{"chat", "https://chat/internal/v1/principal/jwks.json", false},
		{"gameintegration", "https://gameintegration/.well-known/principal-jwks.json", false},
		{"moderation", "https://moderation/internal/v1/principal/jwks.json", false},
		{"gameintegration", "http://gameintegration/internal/v1/principal/jwks.json", false},
		{"gameintegration", "https://user:pass@gameintegration/internal/v1/principal/jwks.json", false},
		{"gameintegration", "https://gameintegration/internal/v1/principal/jwks.json?issuer=gameintegration", false},
	} {
		t.Run(test.issuer+"/"+test.url, func(t *testing.T) {
			require.Equal(t, test.valid, validPrincipalJWKSURL(test.url, test.issuer))
		})
	}
}
