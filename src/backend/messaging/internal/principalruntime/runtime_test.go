package principalruntime

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestComposeSpaceJWKSURLMatchesPinnedIssuerRoute(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../../"))
	compose, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	require.NoError(t, err)
	service := strings.SplitN(string(compose), "\n  messaging:\n", 2)
	require.Len(t, service, 2)
	section := strings.SplitN(service[1], "\n  file:\n", 2)[0]
	var endpoint string
	for _, line := range strings.Split(section, "\n") {
		if value, found := strings.CutPrefix(strings.TrimSpace(line), "SPACE_PRINCIPAL_JWKS_URL: "); found {
			endpoint = value
			break
		}
	}
	require.NotEmpty(t, endpoint)
	require.True(t, validPrincipalJWKSURL(endpoint, "space"), "Compose must use the issuer-pinned public JWKS route")
}
