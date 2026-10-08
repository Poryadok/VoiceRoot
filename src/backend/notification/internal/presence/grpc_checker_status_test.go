package presence

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
)

type fakeUserServiceClient struct {
	userv1.UserServiceClient
	response *userv1.GetNotificationRoutingPresenceResponse
	err      error
	request  *userv1.GetNotificationRoutingPresenceRequest
	ctx      context.Context
}

func (f *fakeUserServiceClient) GetNotificationRoutingPresence(ctx context.Context, request *userv1.GetNotificationRoutingPresenceRequest, _ ...grpc.CallOption) (*userv1.GetNotificationRoutingPresenceResponse, error) {
	f.ctx = ctx
	f.request = request
	return f.response, f.err
}

func TestGRPCCheckerIsOnlineUsesBoundAuthenticatedRoutingRPC(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "notification", KeyID: "test-key", PrivateKey: key})
	require.NoError(t, err)
	profileID := uuid.New()
	client := &fakeUserServiceClient{response: &userv1.GetNotificationRoutingPresenceResponse{HasActiveSession: true}}
	checker := &GRPCChecker{client: client, issuer: issuer}

	active, err := checker.IsOnline(context.Background(), profileID)
	require.NoError(t, err)
	require.True(t, active)
	require.Equal(t, profileID.String(), client.request.GetProfileId())

	md, ok := metadata.FromOutgoingContext(client.ctx)
	require.True(t, ok)
	require.Len(t, md.Get("authorization"), 1)
	require.Len(t, md.Get("x-request-id"), 1)
	requestID := md.Get("x-request-id")[0]
	authorization := md.Get("authorization")[0]
	require.True(t, len(authorization) > len("Bearer "))
	require.Equal(t, "Bearer ", authorization[:len("Bearer ")])

	hash, err := principal.RequestHash(client.request)
	require.NoError(t, err)
	verified, err := principal.VerifyService(context.Background(), authorization[len("Bearer "):], principal.VerifyConfig{
		ExpectedIssuer:      "notification",
		ExpectedAudience:    "user",
		ExpectedRPC:         notificationPresenceMethod,
		ExpectedRequestID:   requestID,
		ExpectedRequestHash: hash,
		KeyResolver: func(_ context.Context, issuer, keyID string) (*rsa.PublicKey, error) {
			if issuer != "notification" || keyID != "test-key" {
				return nil, errors.New("unexpected signer")
			}
			return &key.PublicKey, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, "service:notification", verified.Subject)
	require.Empty(t, verified.AccountID)
	require.Empty(t, verified.ProfileID)
	require.Zero(t, verified.SessionEpoch)
}

func TestGRPCCheckerFailsClosedAtPresenceBoundary(t *testing.T) {
	profileID := uuid.New()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "notification", KeyID: "test-key", PrivateKey: testSigningKey(t)})
	require.NoError(t, err)

	tests := []struct {
		name     string
		response *userv1.GetNotificationRoutingPresenceResponse
		want     bool
		wantErr  bool
	}{
		{name: "active session", response: &userv1.GetNotificationRoutingPresenceResponse{HasActiveSession: true}, want: true},
		{name: "offline is push eligible", response: &userv1.GetNotificationRoutingPresenceResponse{HasActiveSession: false}},
		{name: "nil response is unavailable", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checker := &GRPCChecker{client: &fakeUserServiceClient{response: test.response}, issuer: issuer}
			got, err := checker.IsOnline(context.Background(), profileID)
			if test.wantErr {
				require.Error(t, err)
				require.False(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}

	t.Run("RPC error propagates", func(t *testing.T) {
		expected := errors.New("User service unavailable")
		checker := &GRPCChecker{client: &fakeUserServiceClient{err: expected}, issuer: issuer}
		got, err := checker.IsOnline(context.Background(), profileID)
		require.False(t, got)
		require.ErrorIs(t, err, expected)
	})

	t.Run("missing signer does not fall back to offline", func(t *testing.T) {
		checker := &GRPCChecker{client: &fakeUserServiceClient{response: &userv1.GetNotificationRoutingPresenceResponse{}}}
		got, err := checker.IsOnline(context.Background(), profileID)
		require.False(t, got)
		require.Error(t, err)
	})
}

func TestAuthenticatedCheckerConfigurationIsAllOrNothing(t *testing.T) {
	values := map[string]string{}
	lookupEnv := func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}

	checker, server, err := LoadAuthenticatedGRPCCheckerFromEnv(lookupEnv)
	require.NoError(t, err)
	require.Nil(t, checker)
	require.Nil(t, server)

	values["NOTIFICATION_PRINCIPAL_ACTIVE_KID"] = "current"
	checker, server, err = LoadAuthenticatedGRPCCheckerFromEnv(lookupEnv)
	require.Error(t, err)
	require.Nil(t, checker)
	require.Nil(t, server)

	values["NOTIFICATION_PRINCIPAL_ACTIVE_KID"] = ""
	checker, server, err = LoadAuthenticatedGRPCCheckerFromEnv(lookupEnv)
	require.Error(t, err, "an explicitly present but empty setting is a partial configuration")
	require.Nil(t, checker)
	require.Nil(t, server)
}

func TestNotificationJWKSPublishesOnlyPublicKeyMaterial(t *testing.T) {
	key := testSigningKey(t)
	document, err := marshalNotificationJWKS(map[string]*rsa.PrivateKey{"current": key})
	require.NoError(t, err)
	var decoded struct {
		Keys []map[string]json.RawMessage `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(document, &decoded))
	require.Len(t, decoded.Keys, 1)
	require.JSONEq(t, `{"alg":"RS256","e":"AQAB","kid":"current","kty":"RSA","n":"`+base64.RawURLEncoding.EncodeToString(key.N.Bytes())+`","use":"sig"}`, string(document))
	for _, privateField := range []string{"d", "p", "q", "dp", "dq", "qi"} {
		_, exists := decoded.Keys[0][privateField]
		require.False(t, exists, "JWKS must never publish private RSA key field %q", privateField)
	}
}

func testSigningKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}
