package socialprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"os"
	"path/filepath"
	"testing"
	"time"
	catalogv1 "voice/backend/pkg/pb/voice/catalogactor/v1"
	"voice/backend/pkg/principal"
)

func TestCatalogActorCapabilityIsSeparateAndMutualTLS(t *testing.T) {
	method := "/voice.catalogactor.v1.CatalogActorAuthorityService/GetCatalogActorStates"
	require.True(t, AllowsMethod("catalog", method))
	require.True(t, RequiresMutualTLS("catalog"))
	for _, other := range []string{"messaging", "user", "file", "auth", "search", "notification"} {
		require.False(t, AllowsMethod(other, method), other)
	}
	require.False(t, AllowsMethod("catalog", "/voice.user.v1.UserService/GetProfileDeletionStates"))
}

func TestCatalogActorConfigRequiresBothIssuersAndCompleteIdentity(t *testing.T) {
	const prefix = "USER_CATALOG_PRINCIPAL_"
	t.Setenv("S2S_JWKS_URLS_JSON", `{"chat":"https://chat/jwks","file":"https://file/jwks","messaging":"https://messaging/jwks"}`)
	_, enabled, err := LoadFromEnvWithAudience("user", "catalog", prefix, ":9098")
	require.NoError(t, err)
	require.False(t, enabled)
	t.Setenv(prefix+"TLS_CERT_FILE", "cert")
	_, enabled, err = LoadFromEnvWithAudience("user", "catalog", prefix, ":9098")
	require.Error(t, err)
	require.True(t, enabled)
	_, _, pem := testMessagingClientCA(t)
	path := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(path, pem, 0600))
	for name, value := range map[string]string{"TLS_KEY_FILE": "key", "REPLAY_REDIS_ADDR": "redis:6379", "CLIENT_CA_FILE": path, "JWKS_CLIENT_CERT_FILE": "outbound-cert", "JWKS_CLIENT_KEY_FILE": "outbound-key"} {
		t.Setenv(prefix+name, value)
	}
	cfg, enabled, err := LoadFromEnvWithAudience("user", "catalog", prefix, ":9098")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, map[string]string{"chat": "https://chat/jwks", "file": "https://file/jwks"}, cfg.JWKSURLs)
	for _, issuer := range []string{"chat", "file"} {
		broken := cfg
		broken.JWKSURLs = map[string]string{"chat": "https://chat/jwks", "file": "https://file/jwks"}
		delete(broken.JWKSURLs, issuer)
		require.Error(t, broken.validate())
	}
	for _, name := range []string{"CLIENT_CA_FILE", "JWKS_CLIENT_CERT_FILE", "JWKS_CLIENT_KEY_FILE", "REPLAY_REDIS_ADDR", "TLS_CERT_FILE", "TLS_KEY_FILE"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(prefix+name, "")
			_, enabled, err := LoadFromEnvWithAudience("user", "catalog", prefix, ":9098")
			require.Error(t, err)
			require.True(t, enabled)
		})
	}
}

func TestCatalogActorSignedPrincipalMatrix(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	wrong, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	req := &catalogv1.GetCatalogActorStatesRequest{ProtocolVersion: 1, Actors: []*catalogv1.CatalogActor{{AccountId: uuid.NewString(), ProfileId: uuid.NewString()}}}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	for _, tc := range []struct {
		name, issuer, audience, rpc, binding string
		raw, forged, replayDown              bool
		code                                 codes.Code
	}{
		{name: "chat", issuer: "chat", code: codes.OK}, {name: "file", issuer: "file", code: codes.OK},
		{name: "messaging", issuer: "messaging", code: codes.PermissionDenied},
		{name: "wrong audience", issuer: "chat", audience: "file", code: codes.Unauthenticated},
		{name: "wrong rpc", issuer: "chat", rpc: "/other", code: codes.Unauthenticated},
		{name: "wrong body", issuer: "chat", binding: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", code: codes.Unauthenticated},
		{name: "forged", issuer: "chat", forged: true, code: codes.Unauthenticated},
		{name: "raw marker", issuer: "chat", raw: true, code: codes.Unauthenticated},
		{name: "replay down", issuer: "chat", replayDown: true, code: codes.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			signing := key
			if tc.forged {
				signing = wrong
			}
			issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: tc.issuer, KeyID: "current", PrivateKey: signing})
			require.NoError(t, err)
			audience := tc.audience
			if audience == "" {
				audience = "user"
			}
			rpc := tc.rpc
			if rpc == "" {
				rpc = Method("catalog")
			}
			binding := tc.binding
			if binding == "" {
				binding = hash
			}
			rid := uuid.NewString()
			token, err := issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: rpc, RequestID: rid, RequestHash: binding})
			require.NoError(t, err)
			seen := false
			verifier := &Verifier{Target: "user", Capability: "catalog", Issuers: map[string]bool{"chat": true, "file": true, "messaging": true}, Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error {
				if tc.replayDown {
					return Unavailable(errors.New("down"))
				}
				if seen {
					return errors.New("replay")
				}
				seen = true
				return nil
			}}
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", rid)
			if tc.raw {
				md.Set("x-voice-internal-caller", "chat")
			}
			ctx := metadata.NewIncomingContext(context.Background(), md)
			called := false
			handler := func(ctx context.Context, _ any) (any, error) {
				called = true
				return nil, RequireCatalogActor(ctx, req)
			}
			_, err = StrictUnaryInterceptor(verifier)(ctx, req, &grpc.UnaryServerInfo{FullMethod: Method("catalog")}, handler)
			require.Equal(t, tc.code, status.Code(err))
			require.Equal(t, tc.code == codes.OK, called)
			if tc.code == codes.OK {
				_, err = StrictUnaryInterceptor(verifier)(ctx, req, &grpc.UnaryServerInfo{FullMethod: Method("catalog")}, handler)
				require.Equal(t, codes.Unauthenticated, status.Code(err))
			}
		})
	}
}
